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
	// An unowned directory is preserved even when non-empty.
	unrelated := filepath.Join(dir, "unrelated")
	if e = os.Mkdir(unrelated, 0700); e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(unrelated, "keep"), []byte("keep"), 0600); e != nil {
		t.Fatal(e)
	}
	args[len(args)-1] = unrelated
	if code := Run(args, &stdout, &stderr); code != 1 {
		t.Fatal("overwrote unrelated output")
	}
	b, e := os.ReadFile(filepath.Join(unrelated, "keep"))
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

func TestBuildReplacesDefaultOutputAfterStackChange(t *testing.T) {
	dir := t.TempDir()
	recipeRoot, err := filepath.Abs("../../testdata/sample/recipes")
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	body := []byte("server: {host: offline.test}\nagents: {research: {recipe: observatory}}\n")
	if err = os.WriteFile("stack.yaml", body, 0600); err != nil {
		t.Fatal(err)
	}
	args := []string{"build", "-recipes", recipeRoot}
	var out, stderr bytes.Buffer
	if code := Run(args, &out, &stderr); code != 0 {
		t.Fatalf("first build: %s", &stderr)
	}
	first, err := os.ReadFile(".olympus/build/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	body = bytes.ReplaceAll(body, []byte("research:"), []byte("assistant:"))
	if err = os.WriteFile("stack.yaml", body, 0600); err != nil {
		t.Fatal(err)
	}
	if code := Run(args, &out, &stderr); code != 0 {
		t.Fatalf("changed build: %s", &stderr)
	}
	second, err := os.ReadFile(".olympus/build/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(first, second) {
		t.Fatal("changed stack did not replace generation")
	}
	if _, err = os.Stat(".olympus/build/tools/research"); !os.IsNotExist(err) {
		t.Fatal("stale tool output survived replacement")
	}
	if _, err = os.Stat(".olympus/build/tools/assistant"); err != nil {
		t.Fatal(err)
	}
	siblings, err := os.ReadDir(".olympus")
	if err != nil || len(siblings) != 1 {
		t.Fatalf("staging directory left behind: %v %v", siblings, err)
	}
}
