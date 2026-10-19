package recipe

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const base = "name: demo\nkind: harness\nimage: example/demo:1\nvolumes: {data: /opt/demo}\n"

func TestRecipeValidation(t *testing.T) {
	cases := []struct{ name, input, want string }{
		{"valid", base, ""},
		{"unknown", base + "unknown: true\n", "unknown field"},
		{"kind", strings.Replace(base, "harness", "root-shell", 1), "allowed choice"},
		{"hook string", base + "hooks: {post_install: 'sh -c evil'}\n", "expected array"},
		{"hook args", base + "hooks: {post_install: [demo, setup]}\n", ""},
		{"traversal", base + "templates: [{src: hi.tmpl, dst: /opt/demo/../outside}]\n", "inside a tool volume"},
		{"outside", base + "supports: {mcps: {file: /etc/config.yaml, format: yaml, key: servers}}\n", "inside a tool volume"},
		{"prefix", base + "templates: [{src: hi.tmpl, dst: /opt/demo-other/config}]\n", "inside a tool volume"},
		{"source traversal", base + "templates: [{src: ../hi.tmpl, dst: /opt/demo/hi}]\n", "confined relative"},
		{"volume root", strings.Replace(base, "/opt/demo", "/", 1), "invalid volume"},
		{"volume traversal", strings.Replace(base, "/opt/demo", "/opt/../demo", 1), "invalid volume"},
		{"overlap", strings.Replace(base, "{data: /opt/demo}", "{data: /opt/demo, sub: /opt/demo/sub}", 1), "must not overlap"},
		{"variable in image", strings.Replace(base, "example/demo:1", "${IMAGE}", 1), "secret-capable"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, d := Parse("recipe.yaml", []byte(tc.input))
			var messages []string
			for _, e := range d.Errors {
				if e.Line < 1 || e.Column < 1 {
					t.Fatal(e)
				}
				messages = append(messages, e.Message)
			}
			got := strings.Join(messages, "\n")
			if tc.want == "" && got != "" || tc.want != "" && !strings.Contains(got, tc.want) {
				t.Fatalf("want %q, got %q", tc.want, got)
			}
		})
	}
}
func TestConfinement(t *testing.T) {
	r := &Recipe{Volumes: map[string]string{"data": "/opt/demo"}}
	for _, p := range []string{"../escape", "/opt/demo/../escape", "/etc/passwd", "/opt/demo-other/file", "/opt/demo", "/opt/demo/sub/../../escape", "/opt/demo/\\escape"} {
		if r.Confined(p) {
			t.Fatalf("accepted %q", p)
		}
	}
	if !r.Confined("/opt/demo/config.yaml") {
		t.Fatal("valid destination rejected")
	}
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside")
	if e := os.WriteFile(outside, []byte("do not copy"), 0600); e != nil {
		t.Fatal(e)
	}
	if e := os.Symlink(outside, filepath.Join(root, "link")); e != nil {
		t.Fatal(e)
	}
	if _, e := ReadFile(root, "link"); e == nil {
		t.Fatal("read symlink outside recipe")
	}
	if _, e := ReadFile(root, "../outside"); e == nil {
		t.Fatal("read traversal")
	}
}
func TestMissingRecipeSuggestion(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "observatory"), 0700); err != nil {
		t.Fatal(err)
	}
	_, d := Load(root, "observatry")
	if len(d.Errors) != 1 || !strings.Contains(d.Errors[0].Message, "closest recipe: observatory") {
		t.Fatal(d.Errors)
	}
}
