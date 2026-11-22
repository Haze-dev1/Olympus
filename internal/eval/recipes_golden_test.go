package eval

import (
	"bytes"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Haze-dev1/Olympus/internal/stack"
)

// TestRecipeBuildInputPins checks that both Pi recipes' generated build context
// contains the pinned install inputs required by ADR 0051.
func TestRecipeBuildInputPins(t *testing.T) {
	for _, tc := range []struct{ dir, tool, file, want string }{
		{"pi", "pi", "package.json", `"@earendil-works/pi-coding-agent": "1.5.0"`},
		{"pi", "pi", "package-lock.json", `"lockfileVersion": 3`},
		{"oh-my-pi", "oh-my-pi", "Dockerfile", "COPY --from=fetch /usr/local/bin/omp /usr/local/bin/omp"},
	} {
		t.Run(tc.dir+"/"+tc.file, func(t *testing.T) {
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
			var got []byte
			for p, b := range g.Files {
				if path.Base(p) == tc.file {
					got = b
					break
				}
			}
			if !bytes.Contains(got, []byte(tc.want)) {
				t.Errorf("generated %s does not contain %q", tc.file, tc.want)
			}
		})
	}
}

// TestRecipeNpmInstallScriptsDisabled checks that npm-built recipes install
// with lifecycle scripts off, so only audited build steps can execute (R5).
func TestRecipeNpmInstallScriptsDisabled(t *testing.T) {
	for _, tool := range []string{"claude-code", "codex", "gemini-cli"} {
		t.Run(tool, func(t *testing.T) {
			b, err := os.ReadFile(filepath.Join("../../recipes", tool, "Dockerfile"))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Contains(b, []byte("npm ci --omit=dev --ignore-scripts")) {
				t.Errorf("recipes/%s/Dockerfile: npm ci without --ignore-scripts", tool)
			}
		})
	}
	// Claude Code's bin is an error stub until its postinstall links the
	// platform binary, so run only that audited script after npm ci.
	b, err := os.ReadFile("../../recipes/claude-code/Dockerfile")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(b, []byte("node node_modules/@anthropic-ai/claude-code/install.cjs")) {
		t.Error("claude-code Dockerfile does not run the audited install.cjs")
	}
}

// TestRecipeGoldens renders each single-recipe fixture stack with a fake digest
// for the recipe's own image and compares it with <dir>/golden.
func TestRecipeGoldens(t *testing.T) {
	for _, tc := range []struct{ dir, tool string }{
		{"claude-code", "claude"},
		{"gemini-cli", "gemini"},
		{"goose", "goose"},
		{"aider", "aider"},
		{"openclaw", "openclaw"},
		{"pi", "pi"},
		{"oh-my-pi", "oh-my-pi"},
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
