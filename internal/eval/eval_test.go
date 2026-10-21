package eval

import (
	"bytes"
	"flag"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Haze-dev1/Olympus/internal/encode"
	"github.com/Haze-dev1/Olympus/internal/recipe"
	"github.com/Haze-dev1/Olympus/internal/stack"
)

var updateGolden = flag.Bool("update", false, "regenerate committed expected generation")

func sample(t *testing.T) Inputs {
	t.Helper()
	file := "../../testdata/sample/stack.yaml"
	s, d := stack.Load(file)
	if len(d.Errors) > 0 {
		t.Fatal(d.Errors)
	}
	in, errors := Load(s, "../../testdata/sample/recipes", "../../testdata/sample")
	if len(errors) > 0 {
		t.Fatal(errors)
	}
	return in
}
func TestGoldenAndPurity(t *testing.T) {
	in := sample(t)
	g, err := Build(in)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Build(in)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(g, second) {
		t.Fatal("builds differ")
	}
	if *updateGolden {
		for p, b := range g.Files {
			target := filepath.Join("../../testdata/golden", p)
			if e := os.MkdirAll(filepath.Dir(target), 0700); e != nil {
				t.Fatal(e)
			}
			if e := os.WriteFile(target, b, 0600); e != nil {
				t.Fatal(e)
			}
		}
	}
	count := 0
	err = filepath.WalkDir("../../testdata/golden", func(p string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.IsDir() {
			return nil
		}
		rel, e := filepath.Rel("../../testdata/golden", p)
		if e != nil {
			return e
		}
		expected, e := os.ReadFile(p)
		if e != nil {
			return e
		}
		if !bytes.Equal(expected, g.Files[filepath.ToSlash(rel)]) {
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
	shuffled := []byte("agents:\n  research:\n    secrets: {github-token: '${GH_TOKEN}'}\n    mcps: [github]\n    recipe: observatory\nmcps:\n  github:\n    env: {GITHUB_TOKEN: '${GH_TOKEN}'}\n    args: [stdio]\n    command: github-mcp\nserver: {host: offline.example.test}\n")
	s, d := stack.Parse("stack.yaml", shuffled)
	if len(d.Errors) > 0 {
		t.Fatal(d.Errors)
	}
	in.Stack = s
	reordered, e := Build(in)
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(g, reordered) {
		t.Fatal("reordering stack keys changes generation")
	}
	dir := t.TempDir()
	file := filepath.Join(dir, "stack.yaml")
	for _, value := range []string{"OLYM_SENTINEL_first_private_value", "OLYM_SENTINEL_second_private_value"} {
		if e := os.WriteFile(filepath.Join(dir, ".env"), []byte("GH_TOKEN="+value+"\n"), 0600); e != nil {
			t.Fatal(e)
		}
		_, d := stack.Parse(file, shuffled)
		stack.CheckEnv(d, file)
		if len(d.Errors) > 0 {
			t.Fatal(d.Errors)
		}
		built, e := Build(in)
		if e != nil {
			t.Fatal(e)
		}
		if !reflect.DeepEqual(g, built) {
			t.Fatal("secret value changes build")
		}
		for p, b := range built.Files {
			if bytes.Contains(b, []byte(value)) || bytes.Contains(b, []byte("OLYM_SENTINEL_")) {
				t.Fatalf("secret appears in %s", p)
			}
		}
	}
	config := g.Files["store/"+g.Manifest.Tools["research"].Hash+"/files/opt/observatory/config.yaml"]
	if !bytes.Contains(config, []byte("${GITHUB_TOKEN}")) {
		t.Fatal("MCP runtime variable reference missing")
	}
	compose := g.Files["compose.yaml"]
	if !bytes.Contains(compose, []byte("127.0.0.1:8642:8642")) || !bytes.Contains(compose, []byte("/var/lib/olympus/secrets/research/environment")) {
		t.Fatal("private port or env_file missing")
	}
}
func TestManagedFormatsPreserveBase(t *testing.T) {
	for _, format := range []string{"yaml", "json", "toml"} {
		t.Run(format, func(t *testing.T) {
			in := sample(t)
			r := in.Recipes["observatory"]
			r.Supports.MCPs.Format = format
			r.Supports.MCPs.File = "/opt/observatory/config." + format
			r.Templates[0].Dst = r.Supports.MCPs.File
			seed, e := encode.Marshal(format, map[string]any{"model": "local-test-model", "mcp_servers": map[string]any{"stale": "gone"}})
			if e != nil {
				t.Fatal(e)
			}
			in.Templates["observatory"][r.Templates[0].Src] = seed
			g, e := Build(in)
			if e != nil {
				t.Fatal(e)
			}
			b := g.Files["store/"+g.Manifest.Tools["research"].Hash+"/files/opt/observatory/config."+format]
			doc, e := encode.Unmarshal(format, b)
			if e != nil {
				t.Fatal(e)
			}
			if doc["model"] != "local-test-model" {
				t.Fatal("base config lost")
			}
			servers := doc["mcp_servers"].(map[string]any)
			if _, ok := servers["stale"]; ok {
				t.Fatal("managed key not replaced")
			}
			if servers["github"] == nil {
				t.Fatal("missing MCP entry")
			}
		})
	}
}
func TestBuildRejectsUnsafeDestination(t *testing.T) {
	in := sample(t)
	in.Recipes["observatory"].Templates[0].Dst = "/opt/observatory/../escape"
	if _, err := Build(in); err == nil {
		t.Fatal("builder trusted unsafe destination")
	}
}
func TestRenderingFailuresAndWarnings(t *testing.T) {
	cases := []struct {
		name   string
		change func(Inputs)
		want   string
	}{
		{"unsupported MCP", func(in Inputs) { in.Recipes["observatory"].Supports.MCPs = nil }, "warning"},
		{"undeclared secret", func(in Inputs) { in.Recipes["observatory"].Secrets = map[string]recipe.Secret{} }, "not declared"},
		{"missing MCP secret mapping", func(in Inputs) { v := in.Stack.Agents["research"]; v.Secrets = nil; in.Stack.Agents["research"] = v }, "environment mapping"},
		{"template error", func(in Inputs) { in.Templates["observatory"]["SOUL.md.tmpl"] = []byte("{{.Secrets.login}}") }, "template execution failed"},
		{"empty channel allow", func(in Inputs) {
			v := in.Stack.Agents["research"]
			v.Channels = map[string]stack.Channel{"telegram": {Token: "${TG}"}}
			in.Stack.Agents["research"] = v
		}, "allow-list"},
		{"unsupported channel", func(in Inputs) {
			v := in.Stack.Agents["research"]
			v.Channels = map[string]stack.Channel{"telegram": {Token: "${TG}", Allow: []string{"owner"}}}
			in.Stack.Agents["research"] = v
		}, "not supported"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := sample(t)
			tc.change(in)
			g, e := Build(in)
			if tc.want == "warning" {
				if e != nil || len(g.Warnings) != 1 {
					t.Fatalf("%v %v", g, e)
				}
				return
			}
			if e == nil || !strings.Contains(e.Error(), tc.want) {
				t.Fatalf("want %q, got %v", tc.want, e)
			}
		})
	}
}
func TestDiagnosticsLoadedRecipe(t *testing.T) {
	s := sample(t).Stack
	_, errors := Load(s, t.TempDir(), ".")
	if len(errors) == 0 {
		t.Fatal("missing recipe accepted")
	}
	for _, e := range errors {
		if e.Line < 1 || e.Column < 1 {
			t.Fatal(e)
		}
	}
}

func TestConfinedSkillsAndUnsupportedWarning(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "skills", "research")
	if e := os.MkdirAll(skillDir, 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# Research\nCite sources.\n"), 0600); e != nil {
		t.Fatal(e)
	}
	in := sample(t)
	in.Stack.Skills = []string{"./skills/research"}
	tool := in.Stack.Agents["research"]
	tool.Skills = "all"
	in.Stack.Agents["research"] = tool
	loaded, diagnostics := Load(in.Stack, "../../testdata/sample/recipes", dir)
	if len(diagnostics) > 0 {
		t.Fatal(diagnostics)
	}
	g, e := Build(loaded)
	if e != nil || len(g.Warnings) != 1 {
		t.Fatalf("missing unsupported skills warning: %v %v", g, e)
	}
	loaded.Recipes["observatory"].Supports.Skills = &recipe.Skills{Dir: "/opt/observatory/skills", Mode: "mount"}
	g, e = Build(loaded)
	if e != nil {
		t.Fatal(e)
	}
	if len(g.Files["store/"+g.Manifest.Tools["research"].Hash+"/files/opt/observatory/skills/research/SKILL.md"]) == 0 {
		t.Fatal("missing skill")
	}
	if e := os.Symlink(filepath.Join(skillDir, "SKILL.md"), filepath.Join(skillDir, "link")); e != nil {
		t.Fatal(e)
	}
	_, diagnostics = Load(in.Stack, "../../testdata/sample/recipes", dir)
	if len(diagnostics) == 0 {
		t.Fatal("skill symlink accepted")
	}
}

func TestVolumeNamesCannotCollideAcrossTools(t *testing.T) {
	in := sample(t)
	r := in.Recipes["observatory"]
	r.Volumes = map[string]string{"c": "/opt/observatory"}
	r.Ports = nil
	other := *r
	other.Name = "alternate"
	other.Volumes = map[string]string{"b-c": "/opt/observatory"}
	in.Recipes["alternate"] = &other
	in.Templates["alternate"] = in.Templates["observatory"]
	tool := in.Stack.Agents["research"]
	otherTool := tool
	otherTool.Recipe = "alternate"
	in.Stack.Agents = map[string]stack.Tool{"a-b": tool, "a": otherTool}
	g, e := Build(in)
	if e != nil {
		t.Fatal(e)
	}
	compose, e := encode.Unmarshal("yaml", g.Files["compose.yaml"])
	if e != nil {
		t.Fatal(e)
	}
	volumes := compose["volumes"].(map[string]any)
	if len(volumes) != 2 || volumes["a-b_c"] == nil || volumes["a_b-c"] == nil {
		t.Fatal("volume identity collided across tools")
	}
}

func TestTOMLSeedRetainsInteger(t *testing.T) {
	in := sample(t)
	r := in.Recipes["observatory"]
	r.Supports.MCPs.Format = "toml"
	r.Supports.MCPs.File = "/opt/observatory/config.toml"
	r.Templates[0].Dst = r.Supports.MCPs.File
	in.Templates["observatory"][r.Templates[0].Src] = []byte("port = 8642\nlarge = 9007199254740993\n")
	g, err := Build(in)
	if err != nil {
		t.Fatal(err)
	}
	output := g.Files["store/"+g.Manifest.Tools["research"].Hash+"/files/opt/observatory/config.toml"]
	if !bytes.Contains(output, []byte("port = 8642\n")) || !bytes.Contains(output, []byte("large = 9007199254740993\n")) {
		t.Fatalf("seed integers changed: %s", output)
	}
}

func TestUnchangedToolKeepsMountSource(t *testing.T) {
	in := sample(t)
	in.Stack.Agents["second"] = in.Stack.Agents["research"]
	first, err := Build(in)
	if err != nil {
		t.Fatal(err)
	}
	delete(in.Stack.Agents, "second")
	second, err := Build(in)
	if err != nil {
		t.Fatal(err)
	}
	if first.Manifest.ID == second.Manifest.ID {
		t.Fatal("generation did not change")
	}
	if first.Manifest.Tools["research"].Hash != second.Manifest.Tools["research"].Hash {
		t.Fatal("unchanged tool hash changed")
	}
	a, _ := encode.Unmarshal("yaml", first.Files["compose.yaml"])
	b, _ := encode.Unmarshal("yaml", second.Files["compose.yaml"])
	if !reflect.DeepEqual(a["services"].(map[string]any)["research"], b["services"].(map[string]any)["research"]) {
		t.Fatal("unchanged service changed")
	}
	hash := first.Manifest.Tools["research"].Hash
	if !bytes.Contains(first.Files["compose.yaml"], []byte("/var/lib/olympus/store/"+hash+"/files/")) {
		t.Fatal("mount is not content addressed")
	}
}
