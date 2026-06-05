package external

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"syscall"
	"text/template"
	"time"

	"github.com/google/go-jsonnet"
	"github.com/google/go-jsonnet/ast"
	"github.com/kayac/ecspresso/v2/duration"
)

const (
	ModeExec    = "exec"
	ModeJSONRPC = "jsonrpc"
)

type Config struct {
	Name    string            `json:"name" yaml:"name"`
	Command []string          `json:"command" yaml:"command"`
	NumArgs int               `json:"num_args" yaml:"num_args"`
	Parser  string            `json:"parser" yaml:"parser"`
	Timeout duration.Duration `json:"timeout" yaml:"timeout"`
	Mode    string            `json:"mode" yaml:"mode"`
}

type Plugin struct {
	Config *Config
	proc   *rpcProcess
	mu     sync.Mutex
}

type rpcProcess struct {
	cmd    *exec.Cmd
	enc    *json.Encoder
	dec    *json.Decoder
	stdin  io.WriteCloser
	stdout io.ReadCloser
	mu     sync.Mutex
	id     atomic.Int64
}

type rpcRequest struct {
	JSONRPC string   `json:"jsonrpc"`
	Method  string   `json:"method"`
	Params  []string `json:"params"`
	ID      int64    `json:"id"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
	ID      int64           `json:"id"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func NewPlugin(ctx context.Context, cfg *Config) (*Plugin, error) {
	if len(cfg.Command) == 0 {
		return nil, fmt.Errorf("command is required")
	}
	if cfg.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if cfg.Parser == "" {
		cfg.Parser = "json"
	}
	if cfg.Parser != "json" && cfg.Parser != "string" {
		return nil, fmt.Errorf("unsupported parser: %s", cfg.Parser)
	}
	if cfg.Mode == "" {
		cfg.Mode = ModeExec
	}
	if cfg.Mode != ModeExec && cfg.Mode != ModeJSONRPC {
		return nil, fmt.Errorf("unsupported mode: %s", cfg.Mode)
	}
	return &Plugin{Config: cfg}, nil
}

func (p *Plugin) Exec(ctx context.Context, extraArgs []string) (any, error) {
	switch p.Config.Mode {
	case ModeJSONRPC:
		return p.callRPC(ctx, extraArgs)
	default:
		return p.execOnce(ctx, extraArgs)
	}
}

func (p *Plugin) execOnce(ctx context.Context, extraArgs []string) (any, error) {
	cmd, args := p.Config.Command[0], p.Config.Command[1:]
	args = append(args, extraArgs...)
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)

	if p.Config.Timeout.Duration > 0 {
		_ctx, cancel := context.WithTimeout(ctx, p.Config.Timeout.Duration)
		defer cancel()
		ctx = _ctx
	}
	c := exec.CommandContext(ctx, cmd, args...)
	c.Stdout = stdout
	c.Stderr = stderr
	var timedOut bool
	c.Cancel = func() error {
		timedOut = true
		return c.Process.Signal(syscall.SIGTERM)
	}
	c.WaitDelay = 5 * time.Second
	if err := c.Run(); err != nil {
		return nil, fmt.Errorf("failed to run command: %w stdout:%s stderr:%s", err, stdout.String(), stderr.String())
	}
	if timedOut {
		return nil, fmt.Errorf("command timed out: %s %v", cmd, args)
	}
	switch p.Config.Parser {
	case "json", "":
		var result any
		if err := json.NewDecoder(stdout).Decode(&result); err != nil {
			return nil, fmt.Errorf("failed to decode json: %w", err)
		}
		return result, nil
	case "string":
		return stdout.String(), nil
	default:
		return nil, fmt.Errorf("unsupported parser: %s", p.Config.Parser)
	}
}

func (p *Plugin) startProcess() (*rpcProcess, error) {
	cmd := exec.Command(p.Config.Command[0], p.Config.Command[1:]...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close() //nolint:errcheck
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		stdin.Close()  //nolint:errcheck
		stdout.Close() //nolint:errcheck
		return nil, fmt.Errorf("failed to start jsonrpc process: %w", err)
	}
	return &rpcProcess{
		cmd:    cmd,
		enc:    json.NewEncoder(stdin),
		dec:    json.NewDecoder(stdout),
		stdin:  stdin,
		stdout: stdout,
	}, nil
}

func (p *Plugin) getOrStartProcess() (*rpcProcess, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.proc != nil {
		return p.proc, nil
	}
	proc, err := p.startProcess()
	if err != nil {
		return nil, err
	}
	p.proc = proc
	return proc, nil
}

// resetProcess clears p.proc and tears down the running process. The
// pipes are closed synchronously so any in-flight Encode/Decode unblocks
// immediately; the process itself is reaped in the background (SIGTERM,
// then SIGKILL after a grace period). Safe to call concurrently with
// callRPC.
func (p *Plugin) resetProcess() {
	p.mu.Lock()
	if p.proc == nil {
		p.mu.Unlock()
		return
	}
	proc := p.proc
	p.proc = nil
	p.mu.Unlock()

	// Closing the pipes unblocks a goroutine blocked in dec.Decode (or a
	// blocked enc.Encode) right away, instead of waiting for the process
	// to notice EOF and exit.
	proc.stdin.Close()  //nolint:errcheck
	proc.stdout.Close() //nolint:errcheck

	// Send SIGTERM synchronously so it is delivered even if the program
	// exits right after Close(); reaping the process (and escalating to
	// SIGKILL) happens in the background.
	proc.cmd.Process.Signal(syscall.SIGTERM) //nolint:errcheck

	go func() {
		done := make(chan struct{})
		go func() {
			proc.cmd.Wait() //nolint:errcheck
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			proc.cmd.Process.Kill() //nolint:errcheck
			<-done                  // let the single Wait above return
		}
	}()
}

func (p *Plugin) callRPC(ctx context.Context, extraArgs []string) (any, error) {
	if p.Config.Timeout.Duration > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, p.Config.Timeout.Duration)
		defer cancel()
	}

	params := extraArgs
	if params == nil {
		params = []string{}
	}

	// Retry once if the process we obtained was retired (by another
	// call's reset) between getOrStartProcess and acquiring proc.mu, so
	// the caller transparently uses the freshly started process instead
	// of failing on a dead pipe.
	for range 2 {
		proc, err := p.getOrStartProcess()
		if err != nil {
			return nil, err
		}
		proc.mu.Lock()
		p.mu.Lock()
		stale := p.proc != proc
		p.mu.Unlock()
		if stale {
			proc.mu.Unlock()
			continue
		}
		result, err := p.rpcRoundtrip(ctx, proc, params)
		proc.mu.Unlock()
		return result, err
	}
	return nil, fmt.Errorf("jsonrpc process repeatedly restarted: %s", p.Config.Name)
}

// rpcRoundtrip sends a single request and waits for its response. The
// caller must hold proc.mu so requests are serialized and responses are
// read in order. On any I/O error, timeout, or response-id mismatch it
// returns the error and kills the process (the stream can no longer be
// trusted), matching exec mode where a timed-out call fails immediately.
func (p *Plugin) rpcRoundtrip(ctx context.Context, proc *rpcProcess, params []string) (any, error) {
	req := rpcRequest{
		JSONRPC: "2.0",
		Method:  p.Config.Name,
		Params:  params,
		ID:      proc.id.Add(1),
	}
	if err := proc.enc.Encode(req); err != nil {
		p.resetProcess()
		return nil, fmt.Errorf("failed to send jsonrpc request: %w", err)
	}

	type decodeResult struct {
		resp rpcResponse
		err  error
	}
	ch := make(chan decodeResult, 1)
	go func() {
		var resp rpcResponse
		err := proc.dec.Decode(&resp)
		ch <- decodeResult{resp, err}
	}()

	select {
	case r := <-ch:
		if r.err != nil {
			p.resetProcess()
			return nil, fmt.Errorf("failed to read jsonrpc response: %w", r.err)
		}
		if r.resp.ID != req.ID {
			// Responses are out of sync with requests; the stream is no
			// longer trustworthy, so discard the process.
			p.resetProcess()
			return nil, fmt.Errorf("jsonrpc response id mismatch: got %d, want %d", r.resp.ID, req.ID)
		}
		if r.resp.Error != nil {
			return nil, fmt.Errorf("jsonrpc error: %s (code: %d)", r.resp.Error.Message, r.resp.Error.Code)
		}
		var value any
		if err := json.Unmarshal(r.resp.Result, &value); err != nil {
			return nil, fmt.Errorf("failed to decode jsonrpc result: %w", err)
		}
		return value, nil
	case <-ctx.Done():
		p.resetProcess()
		return nil, fmt.Errorf("jsonrpc call timed out: %s", p.Config.Name)
	}
}

// Close terminates the long-running jsonrpc process if one is running.
func (p *Plugin) Close() error {
	p.resetProcess()
	return nil
}

func (p *Plugin) FuncMap(ctx context.Context) template.FuncMap {
	return template.FuncMap{
		p.Config.Name: func(args ...string) (any, error) {
			return p.Exec(ctx, args)
		},
	}
}

func (p *Plugin) JsonnetNativeFuncs(ctx context.Context) []*jsonnet.NativeFunction {
	params := make([]ast.Identifier, p.Config.NumArgs)
	for i := range params {
		params[i] = ast.Identifier(fmt.Sprintf("arg%d", i))
	}
	return []*jsonnet.NativeFunction{
		{
			Name:   p.Config.Name,
			Params: params,
			Func: func(args []any) (any, error) {
				pArgs := make([]string, len(args))
				for i, arg := range args {
					s, ok := arg.(string)
					if !ok {
						return nil, fmt.Errorf("arg%d must be string", i)
					}
					pArgs[i] = s
				}
				return p.Exec(ctx, pArgs)
			},
		},
	}
}
