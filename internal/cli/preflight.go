package cli

import (
	"github.com/Haze-dev1/Olympus/internal/deploy"
	"github.com/Haze-dev1/Olympus/internal/eval"
	"github.com/Haze-dev1/Olympus/internal/recipe"
	"github.com/Haze-dev1/Olympus/internal/schema"
)

// preflightRequest derives the server checks from the same inputs the
// generation was built from. Recipe sizes are already validated at load time;
// an unreadable value is skipped rather than invented.
func preflightRequest(in eval.Inputs) deploy.PreflightRequest {
	req := deploy.PreflightRequest{
		Images:    in.Images,
		Platforms: in.Platforms,
		MinRAM:    map[string]uint64{},
	}
	tools := in.Stack.Tools()
	for _, name := range schema.Keys(tools) {
		tool := tools[name]
		r := in.Recipes[tool.Recipe]
		if r == nil {
			continue
		}
		public := map[int]bool{}
		for _, port := range tool.Ports.Public {
			public[port] = true
		}
		for _, port := range r.Ports {
			req.Ports = append(req.Ports, deploy.Port{Tool: name, Number: port, Public: public[port]})
		}
		if r.Resources.MinRAM != "" {
			if need, err := recipe.ParseSize(r.Resources.MinRAM); err == nil {
				req.MinRAM[name] = need
			}
		}
	}
	return req
}
