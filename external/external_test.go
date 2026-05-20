package external_test

import (
	"testing"
	"time"

	"github.com/kayac/ecspresso/v2/duration"
	"github.com/kayac/ecspresso/v2/external"
)

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
