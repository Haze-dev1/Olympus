package activate

import (
	"fmt"
	"time"

	"github.com/Haze-dev1/Olympus/internal/generation"
)

// ManifestHealthSource loads both the active and previous generation's manifest.
// The caller supplies the same validated store loader used for activation.
type ManifestHealthSource func(id string) (generation.Manifest, error)

var _ HealthSource = ManifestHealthSource(nil)

func (load ManifestHealthSource) Health(id string) (map[string]Health, error) {
	m, err := load(id)
	if err != nil {
		return nil, err
	}
	if m.ID != id || m.Version != 1 {
		return nil, fmt.Errorf("invalid health manifest")
	}
	result := map[string]Health{}
	for name, tool := range m.Tools {
		if tool.Health == nil {
			continue
		}
		h := Health{Command: tool.Health.Command}
		if tool.Health.HTTP != nil {
			h.HTTP = &HTTPCheck{Port: tool.Health.HTTP.Port, Path: tool.Health.HTTP.Path}
		}
		if tool.Health.Timeout != "" {
			h.Timeout, err = time.ParseDuration(tool.Health.Timeout)
			if err != nil {
				return nil, fmt.Errorf("invalid health timeout")
			}
		}
		if err := h.validate(); err != nil {
			return nil, fmt.Errorf("invalid manifest health check")
		}
		result[name] = h
	}
	return result, nil
}
