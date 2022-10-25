package ecspresso

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/goccy/go-yaml"
)

type Duration struct {
	time.Duration
}

func (d *Duration) UnmarshalJSON(b []byte) error {
	return d.unmarshal(b, json.Unmarshal)
}

func (d *Duration) MarshalJSON() ([]byte, error) {
	return d.marshal()
}

func (d *Duration) UnmarshalYAML(b []byte) error {
	return d.unmarshal(b, yaml.Unmarshal)
}

func (d *Duration) MarshalYAML() ([]byte, error) {
	return d.marshal()
}

func (d *Duration) unmarshal(b []byte, unmarshaler func([]byte, interface{}) error) error {
	var unmarshalled interface{}

	err := unmarshaler(b, &unmarshalled)
	if err != nil {
		return err
	}
	switch value := unmarshalled.(type) {
	case string:
		d.Duration, err = time.ParseDuration(value)
		if err != nil {
			return err
		}
	case float64:
		d.Duration = time.Duration(value)
	default:
		return fmt.Errorf("invalid duration format: %v", value)
	}
	return nil
}

func (d *Duration) marshal() ([]byte, error) {
	return []byte(fmt.Sprintf(`"%s"`, d.Duration.String())), nil
}
