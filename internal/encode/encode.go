// Package encode provides deterministic structured encoders.
package encode

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

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
	data, err := decodeJSON(b)
	if err != nil {
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
		var data any
		data, err = decodeJSON(b)
		if err == nil {
			var ok bool
			out, ok = data.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("expected JSON object")
			}
		}
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

// Decode JSON numbers before passing data to encoders: json.Number is a string
// type that YAML/TOML encoders would otherwise quote instead of emitting numbers.
func decodeJSON(b []byte) (any, error) {
	decoder := json.NewDecoder(bytes.NewReader(b))
	decoder.UseNumber()
	var data any
	if err := decoder.Decode(&data); err != nil {
		return nil, err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, fmt.Errorf("expected one JSON document")
	}
	return nativeNumbers(data)
}
func nativeNumbers(v any) (any, error) {
	switch v := v.(type) {
	case json.Number:
		if strings.ContainsAny(string(v), ".eE") {
			return strconv.ParseFloat(string(v), 64)
		}
		if i, err := v.Int64(); err == nil {
			return i, nil
		}
		return strconv.ParseUint(string(v), 10, 64)
	case map[string]any:
		for k, item := range v {
			value, err := nativeNumbers(item)
			if err != nil {
				return nil, err
			}
			v[k] = value
		}
	case []any:
		for i, item := range v {
			value, err := nativeNumbers(item)
			if err != nil {
				return nil, err
			}
			v[i] = value
		}
	}
	return v, nil
}
