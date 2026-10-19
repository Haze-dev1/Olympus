package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOfflineCommands(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "stack.yaml")
	stackBytes, e := os.ReadFile("../../testdata/sample/stack.yaml")
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(file, stackBytes, 0600); e != nil {
		t.Fatal(e)
	}
	sentinel := "OLYM_SENTINEL_cli_never_print_this"
	if e = os.WriteFile(filepath.Join(dir, ".env"), []byte("GH_TOKEN="+sentinel+"\n"), 0600); e != nil {
		t.Fatal(e)
	}
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"check", "-f", file}, &stdout, &stderr); code != 0 {
		t.Fatalf("check returned %d: %s", code, &stderr)
	}
	output := filepath.Join(dir, "build")
	args := []string{"build", "-f", file, "-recipes", "../../testdata/sample/recipes", "-o", output}
	for i := 0; i < 2; i++ {
		stdout.Reset()
		stderr.Reset()
		if code := Run(args, &stdout, &stderr); code != 0 {
			t.Fatalf("build returned %d: %s", code, &stderr)
		}
		if !strings.Contains(stdout.String(), "Built sha256-") {
			t.Fatal("missing build hash")
		}
	}
	if strings.Contains(stdout.String()+stderr.String(), sentinel) {
		t.Fatal("secret leaked to CLI")
	}
	// An existing directory with unrelated content must never be overwritten.
	if e = os.WriteFile(filepath.Join(output, "unrelated"), []byte("keep"), 0600); e != nil {
		t.Fatal(e)
	}
	if code := Run(args, &stdout, &stderr); code != 1 {
		t.Fatal("overwrote unrelated output")
	}
	b, e := os.ReadFile(filepath.Join(output, "unrelated"))
	if e != nil || string(b) != "keep" {
		t.Fatal("unrelated file changed")
	}
}
func TestFailuresNeverEchoInputValues(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "stack.yaml")
	value := "OLYM_SENTINEL_must_not_appear"
	body := "server: {host: test}\nagents: {demo: {recipe: observatory, secrets: {login: " + value + "}}}\n"
	if e := os.WriteFile(file, []byte(body), 0600); e != nil {
		t.Fatal(e)
	}
	for _, args := range [][]string{{"check", "-f", file}, {"check", "-invalid", value}, {"apply"}, nil} {
		var out, err bytes.Buffer
		if Run(args, &out, &err) != 1 {
			t.Fatal("expected failure")
		}
		if strings.Contains(out.String()+err.String(), value) {
			t.Fatal("diagnostic leaked scalar input")
		}
	}
}
