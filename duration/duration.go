// Package duration provides a time.Duration wrapper with permissive
// JSON / YAML unmarshalling tuned for ecspresso configuration files.
//
// Behaviour:
//
//   - A pure-digit string ("30") and a JSON / YAML number (30) are both
//     interpreted as seconds — i.e. "30" and 30 produce 30s. This
//     differs from v2, which interpreted JSON numbers as nanoseconds
//     under the standard time.Duration cast.
//   - Any other string ("30s", "5m", "1h30m") is delegated to
//     time.ParseDuration.
//   - For bare-number inputs, if the seconds-interpreted result exceeds
//     30 days a slog.Warn is emitted so accidental nanosecond-shaped
//     inputs (e.g. 30000000000 meant as "30 seconds" under v2
//     semantics) surface at config-load time instead of silently
//     producing absurd timeouts. Values that would overflow
//     time.Duration (int64 ns) when multiplied by time.Second are
//     clamped to math.MaxInt64 rather than wrapping to garbage.
//     Explicit duration strings ("1000h" etc.) are trusted as-is.
package duration

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"time"

	"github.com/goccy/go-yaml"
)

// warnThreshold is the upper bound for a "normal" ecspresso-ish
// timeout. A bare-number input larger than this is almost certainly a
// v2 nanosecond value being reinterpreted as seconds.
const warnThreshold = 30 * 24 * time.Hour

// maxSafeSeconds is the largest integer seconds value that fits in a
// time.Duration (int64 ns) without overflowing when multiplied by
// time.Second. Roughly 9_223_372_036 seconds (~292 years).
const maxSafeSeconds = int64(math.MaxInt64 / int64(time.Second))

// Duration wraps time.Duration with JSON / YAML marshalling that
// treats bare numbers as seconds. See the package comment.
type Duration struct {
	time.Duration
}

func (d *Duration) UnmarshalJSON(b []byte) error {
	return d.unmarshal(b, json.Unmarshal)
}

func (d *Duration) MarshalJSON() ([]byte, error) {
	return d.marshal(), nil
}

func (d *Duration) UnmarshalYAML(b []byte) error {
	return d.unmarshal(b, yaml.Unmarshal)
}

func (d *Duration) MarshalYAML() ([]byte, error) {
	return d.marshal(), nil
}

func (d *Duration) unmarshal(b []byte, unmarshaler func([]byte, any) error) error {
	var v any
	if err := unmarshaler(b, &v); err != nil {
		return err
	}
	// numericInput is true for the seconds-reinterpretation paths
	// (JSON / YAML number, or pure-digit string). Used to scope the
	// warn message to those inputs only — explicit duration strings
	// like "1000h" are trusted and not warned about.
	var numericInput bool
	switch value := v.(type) {
	case string:
		if n, err := strconv.ParseInt(value, 10, 64); err == nil {
			numericInput = true
			d.Duration = secondsToDuration(n)
		} else {
			parsed, err := time.ParseDuration(value)
			if err != nil {
				return err
			}
			d.Duration = parsed
		}
	case float64:
		numericInput = true
		d.Duration = floatSecondsToDuration(value)
	case int:
		numericInput = true
		d.Duration = secondsToDuration(int64(value))
	case int64:
		numericInput = true
		d.Duration = secondsToDuration(value)
	case uint64:
		numericInput = true
		if value > math.MaxInt64 {
			d.Duration = math.MaxInt64
		} else {
			d.Duration = secondsToDuration(int64(value))
		}
	default:
		return fmt.Errorf("invalid duration format: %v", value)
	}
	if numericInput && d.Duration > warnThreshold {
		slog.Warn(
			"duration value is unusually large for a timeout; interpreted as seconds (v2 interpreted plain numbers as nanoseconds)",
			"input", string(b),
			"interpreted", d.Duration.String(),
			"hint", `use "30s" / "5m" / "1h" string notation to make the unit explicit`,
		)
	}
	return nil
}

// secondsToDuration converts an integer seconds value to time.Duration,
// clamping to math.MaxInt64 when the multiplication by time.Second
// would overflow int64.
func secondsToDuration(n int64) time.Duration {
	if n > maxSafeSeconds {
		return math.MaxInt64
	}
	if n < -maxSafeSeconds {
		return math.MinInt64
	}
	return time.Duration(n) * time.Second
}

// floatSecondsToDuration is the float64 equivalent of
// secondsToDuration: any positive value above maxSafeSeconds clamps to
// math.MaxInt64.
func floatSecondsToDuration(v float64) time.Duration {
	if v >= float64(maxSafeSeconds) {
		return math.MaxInt64
	}
	if v <= -float64(maxSafeSeconds) {
		return math.MinInt64
	}
	return time.Duration(v * float64(time.Second))
}

func (d *Duration) marshal() []byte {
	return fmt.Appendf(nil, `"%s"`, d.Duration.String())
}
