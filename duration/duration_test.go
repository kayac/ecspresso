package duration_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/kayac/ecspresso/v2/duration"
)

func TestDurationRoundTrip(t *testing.T) {
	cases := []struct {
		str string
		dur time.Duration
	}{
		{"10s", 10 * time.Second},
		{"10m0s", 10 * time.Minute},
		{"5m10s", 310 * time.Second},
		{"1h0m0s", time.Hour},
		{"1h10m0s", 70 * time.Minute},
	}
	for _, c := range cases {
		t.Run(c.str, func(t *testing.T) {
			d := duration.Duration{Duration: c.dur}
			if d.String() != c.str {
				t.Errorf("String: want %s, got %s", c.str, d.String())
			}
			b := new(bytes.Buffer)
			if err := json.NewEncoder(b).Encode(&d); err != nil {
				t.Fatal(err)
			}
			want := `"` + c.str + `"` + "\n"
			if b.String() != want {
				t.Errorf("json encode: want %s, got %s", want, b.String())
			}
			var d2 duration.Duration
			if err := json.Unmarshal(b.Bytes(), &d2); err != nil {
				t.Fatal(err)
			}
			if d2.Duration != c.dur {
				t.Errorf("json decode: want %s, got %s", c.dur, d2.Duration)
			}
			b.Reset()
			if err := yaml.NewEncoder(b).Encode(&d); err != nil {
				t.Fatal(err)
			}
			var d3 duration.Duration
			if err := yaml.Unmarshal(b.Bytes(), &d3); err != nil {
				t.Fatal(err)
			}
			if d3.Duration != c.dur {
				t.Errorf("yaml decode: want %s, got %s", c.dur, d3.Duration)
			}
		})
	}
}

// TestDurationUnmarshalSemantics pins the v3 semantics: bare numbers
// (JSON / YAML number, or pure-digit string) are interpreted as
// seconds, anything else goes through time.ParseDuration.
func TestDurationUnmarshalSemantics(t *testing.T) {
	cases := []struct {
		name  string
		input string // JSON literal
		want  time.Duration
	}{
		{"number is seconds", "30", 30 * time.Second},
		{"digit string is seconds", `"30"`, 30 * time.Second},
		{"parseduration string", `"5m"`, 5 * time.Minute},
		{"compound duration string", `"1h30m"`, 90 * time.Minute},
		{"zero number", "0", 0},
		{"float seconds", "1.5", 1500 * time.Millisecond},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var d duration.Duration
			if err := json.Unmarshal([]byte(c.input), &d); err != nil {
				t.Fatal(err)
			}
			if d.Duration != c.want {
				t.Errorf("want %s, got %s", c.want, d.Duration)
			}
		})
	}
}

// TestDurationUnmarshalWarnsOnLargeValues pins the 30-day warning
// guard: an input that decodes to a duration above 30 days emits a
// slog.Warn so accidental nanosecond-shaped numbers (e.g.
// 30_000_000_000 meant as "30 seconds" under v2 semantics) surface at
// config-load time. The value is still honoured.
func TestDurationUnmarshalWarnsOnLargeValues(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		wantWarn bool
	}{
		{"v2-style nanosecond literal warns (overflows but still warns)", "30000000000", true},
		{"realistic 1h timeout", `"1h"`, false},
		{"realistic 600s timeout", "600", false},
		{"30 days exactly does not warn", "2592000", false},
		{"slightly over 30 days warns", "2592001", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			buf := new(bytes.Buffer)
			orig := slog.Default()
			slog.SetDefault(slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelWarn})))
			defer slog.SetDefault(orig)

			var d duration.Duration
			if err := json.Unmarshal([]byte(c.input), &d); err != nil {
				t.Fatal(err)
			}
			gotWarn := strings.Contains(buf.String(), "unusually large")
			if gotWarn != c.wantWarn {
				t.Errorf("warn: want %v, got %v (log=%s)", c.wantWarn, gotWarn, buf.String())
			}
		})
	}
}

// TestDurationUnmarshalInvalid pins that malformed strings still
// produce an error (not silently a zero Duration).
func TestDurationUnmarshalInvalid(t *testing.T) {
	cases := []string{
		`"not-a-duration"`,
		`"30x"`,
		`true`,
		`null`,
	}
	for _, in := range cases {
		t.Run(in, func(t *testing.T) {
			var d duration.Duration
			if err := json.Unmarshal([]byte(in), &d); err == nil {
				t.Errorf("expected error for %s, got %s", in, d.Duration)
			}
		})
	}
}
