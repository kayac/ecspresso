package ecspresso_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/kayac/ecspresso/v3"
)

var logLevels = []slog.Level{slog.LevelDebug, slog.LevelInfo, slog.LevelWarn, slog.LevelError}

func TestCommonLogger(t *testing.T) {
	for _, format := range []string{"text", "json"} {
		for _, level := range logLevels {
			b := new(bytes.Buffer)
			ecspresso.SetLogFormat(format)
			logger := ecspresso.NewLogger(b)
			ecspresso.LogLevel.Set(level)
			ecspresso.SetLogger(logger)

			ecspresso.LogDebug("test %s", level)
			ecspresso.LogInfo("test message", "level", level.String())
			ecspresso.LogWarn("test message", "level", level.String())
			ecspresso.LogError("test %s", level)
			t.Log(b.String())
		}
	}
}

func TestLogger(t *testing.T) {
	app := &ecspresso.App{}
	for _, format := range []string{"text", "json"} {
		for _, level := range logLevels {
			b := new(bytes.Buffer)
			ecspresso.SetLogFormat(format)
			logger := ecspresso.NewLogger(b)
			ecspresso.LogLevel.Set(level)
			app.SetLogger(logger)

			app.LogDebug("test %s", "test")
			app.LogInfo("test message", "key", "value")
			app.LogWarn("test message", "key", "value")
			app.LogError("test %s", "test")
			t.Log(b.String())
		}
	}
}

func TestStructuredLogOutput(t *testing.T) {
	t.Run("json format has structured fields", func(t *testing.T) {
		b := new(bytes.Buffer)
		ecspresso.SetLogFormat("json")
		logger := ecspresso.NewLogger(b)
		ecspresso.LogLevel.Set(slog.LevelInfo)

		app := &ecspresso.App{}
		app.SetLogger(logger.With("cluster", "test-cluster", "service", "test-svc"))

		app.LogInfo("deployment created", "deployment_id", "d-ABC123", "url", "https://example.com")

		var m map[string]any
		if err := json.Unmarshal(b.Bytes(), &m); err != nil {
			t.Fatalf("failed to parse JSON log: %s\noutput: %s", err, b.String())
		}
		if diff := cmp.Diff("deployment created", m["msg"]); diff != "" {
			t.Errorf("msg mismatch (-want +got):\n%s", diff)
		}
		if diff := cmp.Diff("d-ABC123", m["deployment_id"]); diff != "" {
			t.Errorf("deployment_id mismatch (-want +got):\n%s", diff)
		}
		if diff := cmp.Diff("https://example.com", m["url"]); diff != "" {
			t.Errorf("url mismatch (-want +got):\n%s", diff)
		}
		if diff := cmp.Diff("test-cluster", m["cluster"]); diff != "" {
			t.Errorf("cluster mismatch (-want +got):\n%s", diff)
		}
		if diff := cmp.Diff("test-svc", m["service"]); diff != "" {
			t.Errorf("service mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("text format has key:value attrs", func(t *testing.T) {
		b := new(bytes.Buffer)
		ecspresso.SetLogFormat("text")
		logger := ecspresso.NewLogger(b)
		ecspresso.LogLevel.Set(slog.LevelInfo)

		app := &ecspresso.App{}
		app.SetLogger(logger)

		app.LogInfo("starting deploy", "dry_run", true)

		output := b.String()
		if !strings.Contains(output, "[dry_run:true]") {
			t.Errorf("expected [dry_run:true] in text output: %s", output)
		}
		if !strings.Contains(output, "starting deploy") {
			t.Errorf("expected message in text output: %s", output)
		}
	})

	t.Run("plain message without attrs", func(t *testing.T) {
		b := new(bytes.Buffer)
		ecspresso.SetLogFormat("json")
		logger := ecspresso.NewLogger(b)
		ecspresso.LogLevel.Set(slog.LevelInfo)

		app := &ecspresso.App{}
		app.SetLogger(logger)

		app.LogInfo("DRY RUN OK")

		var m map[string]any
		if err := json.Unmarshal(b.Bytes(), &m); err != nil {
			t.Fatalf("failed to parse JSON log: %s\noutput: %s", err, b.String())
		}
		if diff := cmp.Diff("DRY RUN OK", m["msg"]); diff != "" {
			t.Errorf("msg mismatch (-want +got):\n%s", diff)
		}
	})
}
