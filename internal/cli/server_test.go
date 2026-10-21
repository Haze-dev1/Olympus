package cli

import (
	"bytes"
	"context"
	"github.com/Haze-dev1/Olympus/internal/activate"
	"github.com/Haze-dev1/Olympus/internal/eval"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestServerDispatch(t *testing.T) {
	for _, args := range [][]string{{"activate"}, {"confirm", "id"}, {"version", "extra"}} {
		var out, err bytes.Buffer
		if Run(args, &out, &err) != 1 {
			t.Fatal(args)
		}
	}
	var out, err bytes.Buffer
	if Run([]string{"version"}, &out, &err) != 0 || out.String() != eval.Version+"\n" {
		t.Fatalf("version: %s %s", &out, &err)
	}
}

func TestBuildDiagnosticsDoNotLeak(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "docker")
	file := filepath.Join(dir, "compose.yaml")
	if e := os.WriteFile(bin, []byte("#!/bin/sh\necho OLYM_SENTINEL_build_output >&2\nexit 1\n"), 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(file, []byte("services:\n  codex:\n    image: olympus/codex:hash\n    build: {context: /store/codex}\n"), 0600); e != nil {
		t.Fatal(e)
	}
	err := (quietCompose{Compose: activate.Compose{Bin: bin}}).Build(context.Background(), file)
	if err == nil || strings.Contains(err.Error(), "OLYM_SENTINEL_") {
		t.Fatalf("unsafe build diagnostic: %v", err)
	}
}
