package cli

import (
	"os/exec"
	"path/filepath"
	"testing"
)

func TestReleaseVersionLinkerFlag(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "olym")
	cmd := exec.Command("go", "build", "-ldflags=-X github.com/Haze-dev1/Olympus/internal/eval.Version=9.9.9", "-o", binary, "../../cmd/olym")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build: %v: %s", err, out)
	}
	out, err := exec.Command(binary, "version").CombinedOutput()
	if err != nil || string(out) != "9.9.9\n" {
		t.Fatalf("version: %q: %v", out, err)
	}
}
