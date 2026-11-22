package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Haze-dev1/Olympus/internal/eval"
	"github.com/Haze-dev1/Olympus/internal/recipe"
	"github.com/Haze-dev1/Olympus/internal/stack"
)

func TestPreflightRequestUsesStackAndRecipes(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "stack.yaml")
	body := []byte("server: {host: example.test}\nagents: {tool: {recipe: demo, ports: {public: [8080]}}}\n")
	if err := os.WriteFile(file, body, 0600); err != nil {
		t.Fatal(err)
	}
	s, d := stack.Load(file)
	if len(d.Errors) > 0 {
		t.Fatal(d.Errors)
	}
	r, d := recipe.Parse("recipe.yaml", []byte("name: demo\nkind: harness\nimage: example/demo:1\nports: [8080, 9090]\nresources: {min_ram: 4GB}\n"))
	if len(d.Errors) > 0 {
		t.Fatal(d.Errors)
	}
	in := eval.Inputs{
		Images:    map[string]string{"tool": "example/demo@sha256:" + strings.Repeat("a", 64)},
		Platforms: map[string][]string{"tool": {"linux/amd64"}},
		Stack:     s,
		Recipes:   map[string]*recipe.Recipe{"demo": r},
	}

	req := preflightRequest(in)
	if len(req.Ports) != 2 {
		t.Fatalf("ports = %v", req.Ports)
	}
	if req.Ports[0].Tool != "tool" || req.Ports[0].Number != 8080 || !req.Ports[0].Public {
		t.Fatalf("public port not marked: %+v", req.Ports[0])
	}
	if req.Ports[1].Number != 9090 || req.Ports[1].Public {
		t.Fatalf("private port marked public: %+v", req.Ports[1])
	}
	if req.MinRAM["tool"] != 4_000_000_000 {
		t.Fatalf("min RAM = %d", req.MinRAM["tool"])
	}
	if len(req.Platforms["tool"]) != 1 || req.Images["tool"] != in.Images["tool"] {
		t.Fatalf("lock data not carried: %+v", req)
	}
}
