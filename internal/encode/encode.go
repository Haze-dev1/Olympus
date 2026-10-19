// Package encode provides deterministic structured encoders.
package encode

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/Haze-dev1/Olympus/internal/schema"
	"github.com/goccy/go-yaml"
	"github.com/pelletier/go-toml/v2"
)

func Canonical(v any) ([]byte, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(b))
	decoder.UseNumber()
	var normalized any
	if err = decoder.Decode(&normalized); err != nil {
		return nil, err
	}
	return json.Marshal(normalized)
}
func Marshal(format string, v any) ([]byte, error) {
	// Normalize through JSON so field tags, omissions, and map ordering agree.
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var data any
	if err = json.Unmarshal(b, &data); err != nil {
		return nil, err
	}
	switch format {
	case "json":
		b, err = json.MarshalIndent(data, "", "  ")
		if err != nil {
			return nil, err
		}
		return append(b, '\n'), nil
	case "yaml":
		return yaml.Marshal(data)
	case "toml":
		return toml.Marshal(data)
	default:
		return nil, fmt.Errorf("unsupported structured format")
	}
}
func Unmarshal(format string, b []byte) (map[string]any, error) {
	out := map[string]any{}
	var err error
	switch format {
	case "json":
		err = json.Unmarshal(b, &out)
	case "yaml":
		d := schema.ParseData("config.yaml", b)
		if len(d.Errors) > 0 {
			return nil, fmt.Errorf("invalid structured YAML")
		}
		var ok bool
		out, ok = d.Value.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("expected YAML object")
		}
	case "toml":
		err = toml.Unmarshal(b, &out)
	default:
		return nil, fmt.Errorf("unsupported structured format")
	}
	if err != nil || out == nil {
		return nil, fmt.Errorf("invalid structured config document")
	}
	return out, nil
}
