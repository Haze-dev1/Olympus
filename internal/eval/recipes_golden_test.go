package eval

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Haze-dev1/Olympus/internal/stack"
)

// TestRecipeGoldens renders each single-recipe fixture stack with a fake digest
// for the recipe's own image and compares it with <dir>/golden.
func TestRecipeGoldens(t *testing.T) {
	for _, tc := range []struct{ dir, tool string }{
		{"claude-code", "claude"},
		{"gemini-cli", "gemini"},
		{"goose", "goose"},
		{"aider", "aider"},
		{"openclaw", "openclaw"},
	} {
		t.Run(tc.dir, func(t *testing.T) {
			dir := filepath.Join("../../testdata", tc.dir)
			s, d := stack.Load(filepath.Join(dir, "stack.yaml"))
			if len(d.Errors) > 0 {
				t.Fatal(d.Errors)
			}
			in, errs := Load(s, "../../recipes", dir)
			if len(errs) > 0 {
				t.Fatal(errs)
			}
			image := in.Recipes[s.Agents[tc.tool].Recipe].Image
			in.Images = map[string]string{tc.tool: image + "@sha256:" + strings.Repeat("d", 64)}
			g, err := Build(in)
			if err != nil {
				t.Fatal(err)
			}
			golden := filepath.Join(dir, "golden")
			if *updateGolden {
				if e := os.RemoveAll(golden); e != nil {
					t.Fatal(e)
				}
				for p, b := range g.Files {
					target := filepath.Join(golden, p)
					if e := os.MkdirAll(filepath.Dir(target), 0700); e != nil {
						t.Fatal(e)
					}
					if e := os.WriteFile(target, b, 0600); e != nil {
						t.Fatal(e)
					}
				}
			}
			count := 0
			err = filepath.WalkDir(golden, func(p string, d fs.DirEntry, e error) error {
				if e != nil || d.IsDir() {
					return e
				}
				rel, e := filepath.Rel(golden, p)
				if e != nil {
					return e
				}
				want, e := os.ReadFile(p)
				if e != nil {
					return e
				}
				if !bytes.Equal(want, g.Files[filepath.ToSlash(rel)]) {
					t.Errorf("golden differs: %s", rel)
				}
				count++
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if count != len(g.Files) {
				t.Fatalf("golden has %d files, build has %d", count, len(g.Files))
			}
		})
	}
}
