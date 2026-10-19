package activate

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/Haze-dev1/Olympus/internal/generation"
)

func TestManifestHealthRoundTrip(t *testing.T) {
	m := generation.Manifest{Version: 1, ID: "generation", Tools: map[string]generation.Tool{
		"http":    {Health: &generation.Health{HTTP: &generation.HTTPCheck{Port: 8080, Path: "/health"}, Timeout: "30s"}},
		"command": {Health: &generation.Health{Command: []string{"check", "ready"}}},
		"legacy":  {},
	}}
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	var decoded generation.Manifest
	if err = json.Unmarshal(b, &decoded); err != nil {
		t.Fatal(err)
	}
	source := ManifestHealthSource(func(string) (generation.Manifest, error) { return decoded, nil })
	checks, err := source.Health("generation")
	if err != nil || len(checks) != 2 || checks["http"].Timeout != 30*time.Second || checks["http"].HTTP.Path != "/health" || checks["command"].Command[1] != "ready" {
		t.Fatalf("%v %v", checks, err)
	}
	if _, err = source.Health("wrong"); err == nil {
		t.Fatal("accepted mismatched generation")
	}
	decoded.Tools["bad"] = generation.Tool{Health: &generation.Health{Timeout: "invalid", Command: []string{"check"}}}
	if _, err = source.Health("generation"); err == nil {
		t.Fatal("accepted invalid timeout")
	}
}
