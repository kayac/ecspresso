package external

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"syscall"
	"text/template"
	"time"

	"github.com/google/go-jsonnet"
	"github.com/google/go-jsonnet/ast"
	"github.com/kayac/ecspresso/v2/duration"
)

type Plugin struct {
	Config *Config
}

type Config struct {
	Name    string            `json:"name" yaml:"name"`
	Command []string          `json:"command" yaml:"command"`
	NumArgs int               `json:"num_args" yaml:"num_args"`
	Parser  string            `json:"parser" yaml:"parser"`
	Timeout duration.Duration `json:"timeout" yaml:"timeout"`
}

func NewPlugin(ctx context.Context, cfg *Config) (*Plugin, error) {
	if len(cfg.Command) == 0 {
		return nil, fmt.Errorf("command is required")
	}
	if cfg.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if cfg.Parser == "" {
		cfg.Parser = "json" // default parser
	}
	if cfg.Parser != "json" && cfg.Parser != "string" {
		return nil, fmt.Errorf("unsupported parser: %s", cfg.Parser)
	}
	return &Plugin{Config: cfg}, nil
}

func (p *Plugin) Exec(ctx context.Context, extraArgs []string) (any, error) {
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
	c.WaitDelay = 5 * time.Second // SIGKILL after 5 seconds
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

func (p *Plugin) FuncMap(ctx context.Context) template.FuncMap {
	funcs := template.FuncMap{
		p.Config.Name: func(args ...string) (any, error) {
			return p.Exec(ctx, args)
		},
	}
	return funcs
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
					if s, ok := arg.(string); ok {
						pArgs[i] = s
					} else {
						return nil, fmt.Errorf("arg%d must be string", i)
					}
				}
				return p.Exec(ctx, pArgs)
			},
		},
	}
}
