package ecspresso_test

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/kayac/ecspresso/v2"
)

var testDurations = []struct {
	str string
	dur time.Duration
}{
	{"10s", 10 * time.Second},
	{"10m0s", 10 * time.Minute},
	{"5m10s", 310 * time.Second},
	{"1h0m0s", time.Hour},
	{"1h10m0s", 70 * time.Minute},
}

func TestDuration(t *testing.T) {
	b := bytes.NewBuffer(nil)
	for _, ds := range testDurations {
		b.Reset()
		d := ecspresso.Duration{Duration: ds.dur}
		if d.String() != ds.str {
			t.Errorf("expected %s, got %s", ds.str, d.String())
		}
		json.NewEncoder(b).Encode(&d)
		if bytes.Equal(b.Bytes(), []byte(`"`+ds.str+`"`)) {
			t.Errorf("json expected \"%s\", got %s", ds.str, b.String())
		}
		b.Reset()
		yaml.NewEncoder(b).Encode(&d)
		if bytes.Equal(b.Bytes(), []byte(`"`+ds.str+`"`)) {
			t.Errorf("yaml expected \"%s\", got %s", ds.str, b.String())
		}
		json.Unmarshal(b.Bytes(), &d)
		if d.String() != ds.str {
			t.Errorf("json expected %s, got %s", ds.str, d.String())
		}
		yaml.Unmarshal(b.Bytes(), &d)
		if d.String() != ds.str {
			t.Errorf("yaml expected %s, got %s", ds.str, d.String())
		}
	}
}
