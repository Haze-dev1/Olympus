package cli

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

var envVarRef = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)

func TestExamples(t *testing.T) {
	root := filepath.Join("..", "..")
	examples := filepath.Join(root, "examples")
	recipes := filepath.Join(root, "recipes")

	var stacks []string
	err := filepath.WalkDir(examples, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && d.Name() == "stack.yaml" {
			stacks = append(stacks, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(stacks) == 0 {
		t.Fatal("no example stacks found")
	}

	for _, stackPath := range stacks {
		stackPath := stackPath
		name, err := filepath.Rel(examples, filepath.Dir(stackPath))
		if err != nil {
			t.Fatal(err)
		}
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			if err := copyExampleDir(filepath.Dir(stackPath), dir); err != nil {
				t.Fatal(err)
			}
			if err := writeExampleEnv(dir); err != nil {
				t.Fatal(err)
			}

			file := filepath.Join(dir, "stack.yaml")
			outDir := filepath.Join(dir, "build")

			var stdout, stderr bytes.Buffer
			if code := Run([]string{"check", "-f", file, "-recipes", recipes}, &stdout, &stderr); code != 0 {
				t.Fatalf("check failed (%d):\n%s\n%s", code, stdout.String(), stderr.String())
			}
			if !strings.Contains(stdout.String(), "Stack is valid.") {
				t.Fatalf("check output missing success message: %q", stdout.String())
			}

			stdout.Reset()
			stderr.Reset()
			if code := Run([]string{"build", "-f", file, "-recipes", recipes, "-o", outDir}, &stdout, &stderr); code != 0 {
				t.Fatalf("build failed (%d):\n%s\n%s", code, stdout.String(), stderr.String())
			}
			if !strings.Contains(stdout.String(), "Built sha256-") {
				t.Fatalf("build output missing hash: %q", stdout.String())
			}
		})
	}
}

func writeExampleEnv(dir string) error {
	envExample := filepath.Join(dir, ".env.example")
	envFile := filepath.Join(dir, ".env")
	if b, err := os.ReadFile(envExample); err == nil {
		return os.WriteFile(envFile, b, 0600)
	}
	stack, err := os.ReadFile(filepath.Join(dir, "stack.yaml"))
	if err != nil {
		return err
	}
	seen := map[string]bool{}
	var names []string
	for _, m := range envVarRef.FindAllSubmatch(stack, -1) {
		name := string(m[1])
		if !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		return nil
	}
	sort.Strings(names)
	var b strings.Builder
	for _, name := range names {
		b.WriteString(name)
		b.WriteString("=replace-me\n")
	}
	return os.WriteFile(envFile, []byte(b.String()), 0600)
}

func copyExampleDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0700)
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, b, 0600)
	})
}
