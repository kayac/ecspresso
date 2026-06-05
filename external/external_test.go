package external_test

import (
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/kayac/ecspresso/v2/duration"
	"github.com/kayac/ecspresso/v2/external"
)

const jsonrpcServerBin = "testdata/jsonrpc_server/jsonrpc_server"

func TestMain(m *testing.M) {
	cmd := exec.Command("go", "build", "-o", jsonrpcServerBin, "./testdata/jsonrpc_server/")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		panic("failed to build jsonrpc_server: " + err.Error())
	}
	code := m.Run()
	os.Remove(jsonrpcServerBin)
	os.Exit(code)
}

func TestExternalPlugin(t *testing.T) {
	ctx := t.Context()
	config := external.Config{
		Name:    "test",
		Command: []string{"jq", "-n"},
		NumArgs: 1,
	}
	p, err := external.NewPlugin(ctx, &config)
	if err != nil {
		t.Fatal(err)
	}
	result, err := p.Exec(ctx, []string{`{Now: now}`})
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Fatal("result is nil")
	}
	if m, ok := result.(map[string]any); ok {
		unix, ok := m["Now"]
		if !ok {
			t.Fatal("Now is not found")
		}
		ts, ok := unix.(float64)
		if !ok {
			t.Fatalf("Now is not float64: %T", unix)
		}
		now := time.Unix(int64(ts), 0)
		goNow := time.Now()
		if now.Before(goNow.Add(-1*time.Second)) || now.After(goNow.Add(1*time.Second)) {
			t.Fatalf("Now is not current time: %s expected: %s", now, goNow)
		}
	} else {
		t.Fatalf("result is not map: %T", result)
	}
}

func TestExternalPluginTimeout(t *testing.T) {
	ctx := t.Context()
	config := external.Config{
		Name:    "test",
		Command: []string{"sh", "-c", "sleep 2; echo 123"},
		Timeout: duration.Duration{Duration: time.Second},
	}
	p, err := external.NewPlugin(ctx, &config)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.Exec(ctx, nil); err == nil {
		t.Fatal("timeout is not working")
	} else {
		t.Log(err)
	}
}

func TestExternalPluginString(t *testing.T) {
	ctx := t.Context()
	config := external.Config{
		Name:    "echo",
		Command: []string{"echo", "-n"},
		NumArgs: 1,
		Parser:  "string",
	}
	p, err := external.NewPlugin(ctx, &config)
	if err != nil {
		t.Fatal(err)
	}
	result, err := p.Exec(ctx, []string{`Hello World`})
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Fatal("result is nil")
	}
	if m, ok := result.(string); ok {
		if m != "Hello World" {
			t.Fatalf("unexpected result: %s", m)
		}
	} else {
		t.Fatalf("result is not a string: %T", result)
	}
}

func TestExternalPluginJSONRPC(t *testing.T) {
	ctx := t.Context()
	config := external.Config{
		Name:    "myfunc",
		Command: []string{jsonrpcServerBin},
		NumArgs: 1,
		Mode:    external.ModeJSONRPC,
	}
	p, err := external.NewPlugin(ctx, &config)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	result, err := p.Exec(ctx, []string{"hello"})
	if err != nil {
		t.Fatal(err)
	}
	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result is not map: %T", result)
	}
	if m["param"] != "hello" {
		t.Errorf("unexpected param: %v", m["param"])
	}
	if m["count"] != float64(1) {
		t.Errorf("unexpected count: %v", m["count"])
	}
}

func TestExternalPluginJSONRPCMultipleCalls(t *testing.T) {
	ctx := t.Context()
	config := external.Config{
		Name:    "myfunc",
		Command: []string{jsonrpcServerBin},
		NumArgs: 1,
		Mode:    external.ModeJSONRPC,
	}
	p, err := external.NewPlugin(ctx, &config)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	for i := 1; i <= 3; i++ {
		result, err := p.Exec(ctx, []string{"arg"})
		if err != nil {
			t.Fatal(err)
		}
		m, ok := result.(map[string]any)
		if !ok {
			t.Fatalf("call %d: result is not map: %T", i, result)
		}
		// count increments across calls, proving the same process is reused
		if m["count"] != float64(i) {
			t.Errorf("call %d: expected count=%d, got %v", i, i, m["count"])
		}
	}
}

func TestExternalPluginJSONRPCTimeout(t *testing.T) {
	ctx := t.Context()
	config := external.Config{
		Name:    "myfunc",
		Command: []string{jsonrpcServerBin, "-delay", "2s"},
		NumArgs: 1,
		Mode:    external.ModeJSONRPC,
		Timeout: duration.Duration{Duration: 200 * time.Millisecond},
	}
	p, err := external.NewPlugin(ctx, &config)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	// The server delays 2s while the timeout is 200ms, so every call times
	// out. The first timeout kills the process; the same plugin instance
	// must restart it for the second call (rather than hanging or erroring
	// on a dead pipe), so the second call reaches a fresh process and times
	// out the same way.
	for i := range 2 {
		if _, err := p.Exec(ctx, []string{"hello"}); err == nil {
			t.Fatalf("call %d: timeout did not trigger", i+1)
		} else {
			t.Logf("call %d: %v", i+1, err)
		}
	}
}

func TestExternalPluginJSONRPCIDMismatch(t *testing.T) {
	ctx := t.Context()
	config := external.Config{
		Name:    "myfunc",
		Command: []string{jsonrpcServerBin, "-bad-id"},
		NumArgs: 1,
		Mode:    external.ModeJSONRPC,
	}
	p, err := external.NewPlugin(ctx, &config)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	if _, err := p.Exec(ctx, []string{"hello"}); err == nil {
		t.Fatal("expected id mismatch error")
	} else {
		t.Log(err)
	}
}

func TestExternalPluginJSONRPCError(t *testing.T) {
	ctx := t.Context()
	config := external.Config{
		Name:    "myfunc",
		Command: []string{jsonrpcServerBin, "-error"},
		NumArgs: 1,
		Mode:    external.ModeJSONRPC,
	}
	p, err := external.NewPlugin(ctx, &config)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	if _, err := p.Exec(ctx, []string{"hello"}); err == nil {
		t.Fatal("expected jsonrpc error")
	} else {
		t.Log(err)
	}
}

func TestExternalPluginJSONRPCRestart(t *testing.T) {
	ctx := t.Context()
	config := external.Config{
		Name:    "myfunc",
		Command: []string{jsonrpcServerBin},
		NumArgs: 1,
		Mode:    external.ModeJSONRPC,
	}
	p, err := external.NewPlugin(ctx, &config)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	// First call starts the process (count=1).
	result, err := p.Exec(ctx, []string{"first"})
	if err != nil {
		t.Fatal(err)
	}
	m := result.(map[string]any)
	if m["count"] != float64(1) {
		t.Errorf("expected count=1, got %v", m["count"])
	}

	// Close restarts the process; the next call starts a fresh process (count=1 again).
	p.Close()

	result, err = p.Exec(ctx, []string{"after-close"})
	if err != nil {
		t.Fatal(err)
	}
	m = result.(map[string]any)
	if m["count"] != float64(1) {
		t.Errorf("after restart expected count=1, got %v", m["count"])
	}
}
