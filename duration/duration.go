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
//   - If the resulting Duration exceeds 30 days, a slog.Warn is emitted
//     so accidental nanosecond-shaped inputs (e.g. 30000000000 meant as
//     "30 seconds" under v2 semantics) surface at config-load time
//     instead of silently producing absurd timeouts.
package duration

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/goccy/go-yaml"
)

// warnThreshold is the upper bound for a "normal" ecspresso-ish
// timeout. Anything larger is almost certainly a v2 nanosecond value
// being reinterpreted as seconds, so we warn but still honour it.
const warnThreshold = 30 * 24 * time.Hour

// warnSeconds is warnThreshold expressed in seconds. Used to flag
// suspect raw-number inputs before the multiplication that could
// otherwise overflow time.Duration (int64 ns) past int64 max.
const warnSeconds = int64(warnThreshold / time.Second)

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
	suspectLarge := false
	switch value := v.(type) {
	case string:
		if n, err := strconv.ParseInt(value, 10, 64); err == nil {
			suspectLarge = n > warnSeconds
			d.Duration = time.Duration(n) * time.Second
		} else {
			parsed, err := time.ParseDuration(value)
			if err != nil {
				return err
			}
			d.Duration = parsed
		}
	case float64:
		suspectLarge = value > float64(warnSeconds)
		d.Duration = time.Duration(value * float64(time.Second))
	default:
		return fmt.Errorf("invalid duration format: %v", value)
	}
	// suspectLarge catches the raw-number paths whose multiplication
	// can overflow time.Duration; the bottom check catches the
	// time.ParseDuration path (which can't overflow but can still
	// produce an unusually long Duration).
	if suspectLarge || d.Duration > warnThreshold {
		slog.Warn(
			"duration value is unusually large for a timeout; interpreted as seconds (v2 interpreted plain numbers as nanoseconds)",
			"input", string(b),
			"interpreted", d.Duration.String(),
			"hint", `use "30s" / "5m" / "1h" string notation to make the unit explicit`,
		)
	}
	return nil
}

func (d *Duration) marshal() []byte {
	return fmt.Appendf(nil, `"%s"`, d.Duration.String())
}
