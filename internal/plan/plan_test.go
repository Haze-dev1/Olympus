package plan

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/Haze-dev1/Olympus/internal/generation"
)

func TestCompare(t *testing.T) {
	base := generation.Tool{Hash: "old", Image: "image:1", Files: map[string]string{"/a": "a", "/removed": "r"}}
	current := generation.Manifest{Tools: map[string]generation.Tool{"demo": base}}
	actual := map[string]map[string]string{"demo": base.Files}
	for _, tc := range []struct {
		name      string
		want      generation.Manifest
		actual    map[string]map[string]string
		kind      string
		files     int
		image     bool
		drift     int
		unchanged int
	}{
		{"new", generation.Manifest{Tools: map[string]generation.Tool{"demo": base, "new": base}}, actual, "+", 0, false, 0, 1},
		{"removed", generation.Manifest{}, actual, "-", 0, false, 0, 0},
		{"files", generation.Manifest{Tools: map[string]generation.Tool{"demo": {Hash: "new", Image: "image:1", Files: map[string]string{"/a": "changed", "/added": "new"}}}}, actual, "~", 3, false, 0, 0},
		{"image", generation.Manifest{Tools: map[string]generation.Tool{"demo": {Hash: "new", Image: "image:2", Files: base.Files}}}, actual, "~", 0, true, 0, 0},
		{"unchanged", current, actual, "", 0, false, 0, 1},
		{"drift", current, map[string]map[string]string{"demo": {"/a": "edited", "/removed": "r"}}, "~", 0, false, 1, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := Compare(tc.want, current, tc.actual)
			if d.Unchanged != tc.unchanged {
				t.Fatal(d)
			}
			if tc.kind == "" {
				if len(d.Changes) != 0 {
					t.Fatal(d)
				}
				return
			}
			if len(d.Changes) != 1 {
				t.Fatal(d)
			}
			c := d.Changes[0]
			if c.Kind != tc.kind || len(c.Files) != tc.files || c.Image != tc.image || len(c.Drift) != tc.drift {
				t.Fatal(c)
			}
		})
	}
}

type fakeReader struct {
	t        *testing.T
	calls    int
	manifest []byte
	id       string
	hash     string
	fail     bool
	missing  bool
}

func (f *fakeReader) Run(_ context.Context, args []string, _ io.Reader) ([]byte, []byte, error) {
	f.calls++
	if len(args) < 3 || args[0] != "sudo" || args[1] != "-n" {
		f.t.Fatalf("missing sudo: %v", args)
	}
	args = args[2:]
	if f.fail {
		return nil, nil, fmt.Errorf("sensitive remote diagnostic")
	}
	switch args[0] {
	case "id":
		return []byte("0\n"), nil, nil
	case "readlink":
		if args[1] == "--" {
			return []byte("generations/1\n"), nil, nil
		}
		return []byte("/var/lib/olympus/store/gen-" + f.id + "\n"), nil, nil
	case "head":
		return f.manifest, nil, nil
	case "test":
		return nil, nil, exitCode(1)
	case "sha256sum":
		if f.missing {
			return nil, nil, exitCode(1)
		}
		if !strings.Contains(args[2], "/store/"+f.hash+"/files/a") {
			f.t.Fatal(args)
		}
		return []byte(strings.TrimPrefix(f.hash, "sha256-") + "  file\n"), nil, nil
	default:
		f.t.Fatalf("unexpected command: %v", args)
	}
	return nil, nil, nil
}
func TestReadCurrent(t *testing.T) {
	hash := generation.Hash([]byte("contents"))
	g := generation.Generation{Manifest: generation.Manifest{Version: 1, Olym: "test", Tools: map[string]generation.Tool{"demo": {Hash: hash, Files: map[string]string{"/a": hash}}}}, Files: map[string][]byte{}}
	if err := g.Finish(); err != nil {
		t.Fatal(err)
	}
	f := &fakeReader{t: t, manifest: g.Files["manifest.json"], id: g.Manifest.ID, hash: hash}
	m, actual, err := ReadCurrent(context.Background(), f, false)
	if err != nil || m.ID != g.Manifest.ID || actual["demo"]["/a"] != hash || f.calls != 5 {
		t.Fatalf("%v %v", actual, err)
	}
	f.missing = true
	_, actual, err = ReadCurrent(context.Background(), f, false)
	if err != nil || actual["demo"]["/a"] != "" {
		t.Fatalf("missing file not recorded as drift: %v", err)
	}
	f.missing = false
	f.manifest = []byte("{}")
	if _, _, err := ReadCurrent(context.Background(), f, false); err == nil {
		t.Fatal("accepted invalid manifest")
	}
	f.fail = true
	if _, _, err := ReadCurrent(context.Background(), f, false); err == nil || strings.Contains(err.Error(), "sensitive") {
		t.Fatal(err)
	}
}

type exitCode int

func (e exitCode) Error() string   { return "command failed" }
func (e exitCode) ExitStatus() int { return int(e) }

type absentReader struct {
	code  int
	calls int
}

func (r *absentReader) Run(_ context.Context, args []string, _ io.Reader) ([]byte, []byte, error) {
	r.calls++
	return nil, nil, exitCode(r.code)
}
func TestAbsentCurrentAndPermissionFailure(t *testing.T) {
	r := &absentReader{code: 1}
	m, actual, err := ReadCurrent(context.Background(), r, true)
	if err != nil || m.ID != "" || len(actual) != 0 || r.calls != 3 {
		t.Fatalf("%v %v", m, err)
	}
	r = &absentReader{code: 2}
	if _, _, err = ReadCurrent(context.Background(), r, true); err == nil {
		t.Fatal("treated permission/command failure as empty")
	}
}

func TestSecretChangeCountsToolOnce(t *testing.T) {
	d := Diff{Changes: []Change{{Name: "changed", Kind: "~", Image: true}}, Unchanged: 2}
	d.SecretChanges(map[string]bool{"changed": true, "same": true, "other": false})
	if len(d.Changes) != 2 || d.Unchanged != 1 || !d.Changes[0].Secret || !d.Changes[1].Secret {
		t.Fatal(d)
	}
}

func TestDiffConfig(t *testing.T) {
	t.Run("yaml", func(t *testing.T) {
		oldYaml := []byte(`
gateway: {}
model:
  name: gpt-4
  provider: openai
mcp_servers:
  slack:
    command: python
`)
		newYaml := []byte(`
gateway: {}
model:
  name: gpt-4
  provider: anthropic
mcp_servers:
  github:
    command: npx
`)
		keys, err := DiffConfig("/opt/data/config.yaml", oldYaml, newYaml)
		if err != nil {
			t.Fatal(err)
		}
		want := []KeyChange{
			{Path: "mcp_servers.github", Kind: "added"},
			{Path: "mcp_servers.slack", Kind: "removed"},
			{Path: "model.provider", Kind: "changed"},
		}
		if !reflect.DeepEqual(keys, want) {
			t.Fatalf("got %+v, want %+v", keys, want)
		}
	})

	t.Run("toml", func(t *testing.T) {
		oldToml := []byte(`
[model]
name = "gpt-4"

[mcp_servers.slack]
command = "python"
`)
		newToml := []byte(`
[model]
name = "claude-3"

[mcp_servers.github]
command = "npx"
`)
		keys, err := DiffConfig("/root/.codex/config.toml", oldToml, newToml)
		if err != nil {
			t.Fatal(err)
		}
		want := []KeyChange{
			{Path: "mcp_servers.github", Kind: "added"},
			{Path: "mcp_servers.slack", Kind: "removed"},
			{Path: "model.name", Kind: "changed"},
		}
		if !reflect.DeepEqual(keys, want) {
			t.Fatalf("got %+v, want %+v", keys, want)
		}
	})

	t.Run("json", func(t *testing.T) {
		oldJSON := []byte(`{"theme":"light","mcp":{"slack":{"command":"python"}}}`)
		newJSON := []byte(`{"theme":"dark","mcp":{"github":{"command":"npx"}}}`)
		keys, err := DiffConfig("/root/.config/opencode/opencode.json", oldJSON, newJSON)
		if err != nil {
			t.Fatal(err)
		}
		want := []KeyChange{
			{Path: "mcp.github", Kind: "added"},
			{Path: "mcp.slack", Kind: "removed"},
			{Path: "theme", Kind: "changed"},
		}
		if !reflect.DeepEqual(keys, want) {
			t.Fatalf("got %+v, want %+v", keys, want)
		}
	})

	t.Run("array change", func(t *testing.T) {
		oldJSON := []byte(`{"channels":{"telegram":{"allow":[1,2]}}}`)
		newJSON := []byte(`{"channels":{"telegram":{"allow":[1,2,3]}}}`)
		keys, err := DiffConfig("config.json", oldJSON, newJSON)
		if err != nil {
			t.Fatal(err)
		}
		want := []KeyChange{
			{Path: "channels.telegram.allow", Kind: "changed"},
		}
		if !reflect.DeepEqual(keys, want) {
			t.Fatalf("got %+v, want %+v", keys, want)
		}
	})

	t.Run("unparsable fallback", func(t *testing.T) {
		for _, tc := range []struct {
			path string
			old  []byte
			new  []byte
		}{
			{"config.txt", []byte("a"), []byte("b")},
			{"config.yaml", []byte("invalid: yaml: :"), []byte("ok: true")},
			{"config.toml", []byte("invalid toml ="), []byte("ok = true")},
			{"config.json", []byte("{not json"), []byte("{}")},
		} {
			if _, err := DiffConfig(tc.path, tc.old, tc.new); err == nil {
				t.Fatalf("expected error for %s", tc.path)
			}
		}
	})

	t.Run("unchanged keys", func(t *testing.T) {
		a := []byte("# comment\nfoo: bar\n")
		b := []byte("foo: bar\n")
		keys, err := DiffConfig("config.yaml", a, b)
		if err != nil || len(keys) != 0 {
			t.Fatalf("expected 0 key changes, got %+v, err %v", keys, err)
		}
	})
}

func TestCompareWithFileContents(t *testing.T) {
	current := generation.Manifest{
		Tools: map[string]generation.Tool{
			"hermes": {
				Hash:  "hash1",
				Image: "image:1",
				Files: map[string]string{"/opt/data/config.yaml": "h1"},
			},
		},
	}
	want := generation.Manifest{
		Tools: map[string]generation.Tool{
			"hermes": {
				Hash:  "hash2",
				Image: "image:1",
				Files: map[string]string{"/opt/data/config.yaml": "h2"},
			},
		},
	}
	actual := map[string]map[string]string{"hermes": {"/opt/data/config.yaml": "h1"}}

	contents := FileContents{
		Want: map[string]map[string][]byte{
			"hermes": {"/opt/data/config.yaml": []byte("mcp_servers:\n  github:\n    command: npx\n")},
		},
		Current: map[string]map[string][]byte{
			"hermes": {"/opt/data/config.yaml": []byte("mcp_servers: {}\n")},
		},
	}

	diff := Compare(want, current, actual, contents)
	var buf bytes.Buffer
	diff.Print(&buf)
	out := buf.String()
	if !strings.Contains(out, "config: /opt/data/config.yaml (mcp_servers.github added)") {
		t.Fatalf("unexpected output: %s", out)
	}

	// Unparsable fallback
	unparsableContents := FileContents{
		Want: map[string]map[string][]byte{
			"hermes": {"/opt/data/config.yaml": []byte("invalid: : yaml")},
		},
		Current: map[string]map[string][]byte{
			"hermes": {"/opt/data/config.yaml": []byte("mcp_servers: {}\n")},
		},
	}
	diff = Compare(want, current, actual, unparsableContents)
	buf.Reset()
	diff.Print(&buf)
	out = buf.String()
	if !strings.Contains(out, "config: /opt/data/config.yaml\n") || strings.Contains(out, "(") {
		t.Fatalf("unexpected unparsable fallback output: %s", out)
	}
}

func TestSecretValueNeverAppearsInPlanOutput(t *testing.T) {
	const sentinelOld = "SENTINEL_OLD_SECRET_VAL_9999"
	const sentinelNew = "SENTINEL_NEW_SECRET_VAL_8888"

	current := generation.Manifest{
		Tools: map[string]generation.Tool{
			"codex": {
				Hash:  "hash1",
				Image: "image:1",
				Files: map[string]string{"/root/.codex/config.toml": "h1"},
			},
		},
	}
	want := generation.Manifest{
		Tools: map[string]generation.Tool{
			"codex": {
				Hash:  "hash2",
				Image: "image:1",
				Files: map[string]string{"/root/.codex/config.toml": "h2"},
			},
		},
	}
	actual := map[string]map[string]string{"codex": {"/root/.codex/config.toml": "h1"}}

	contents := FileContents{
		Want: map[string]map[string][]byte{
			"codex": {"/root/.codex/config.toml": []byte(fmt.Sprintf("[mcp_servers.github.env]\nTOKEN = %q\n", sentinelNew))},
		},
		Current: map[string]map[string][]byte{
			"codex": {"/root/.codex/config.toml": []byte(fmt.Sprintf("[mcp_servers.github.env]\nTOKEN = %q\n", sentinelOld))},
		},
	}

	diff := Compare(want, current, actual, contents)
	var buf bytes.Buffer
	diff.Print(&buf)
	out := buf.String()

	if strings.Contains(out, sentinelOld) {
		t.Fatalf("secret leak in plan output! Found old sentinel secret %q in:\n%s", sentinelOld, out)
	}
	if strings.Contains(out, sentinelNew) {
		t.Fatalf("secret leak in plan output! Found new sentinel secret %q in:\n%s", sentinelNew, out)
	}
	if !strings.Contains(out, "config: /root/.codex/config.toml (mcp_servers.github.env.TOKEN changed)") {
		t.Fatalf("expected key-level change notification without secret value, got:\n%s", out)
	}
}

type fakeFileContentReader struct {
	files map[string][]byte
}

func (f *fakeFileContentReader) Run(_ context.Context, args []string, _ io.Reader) ([]byte, []byte, error) {
	if len(args) >= 5 && args[0] == "head" && args[1] == "-c" {
		filePath := args[4]
		if b, ok := f.files[filePath]; ok {
			return b, nil, nil
		}
	}
	return nil, nil, exitCode(1)
}

func TestReadCurrentFiles(t *testing.T) {
	current := generation.Manifest{
		Tools: map[string]generation.Tool{
			"demo": {
				Hash:  "sha256-" + strings.Repeat("a", 64),
				Files: map[string]string{"/opt/config.yaml": "sha256-" + strings.Repeat("b", 64)},
			},
		},
	}
	storePath := "/var/lib/olympus/store/" + current.Tools["demo"].Hash + "/files/opt/config.yaml"
	reader := &fakeFileContentReader{
		files: map[string][]byte{
			storePath: []byte("foo: bar\n"),
		},
	}

	changed := map[string][]string{
		"demo": {"/opt/config.yaml"},
	}
	result := ReadCurrentFiles(context.Background(), reader, true, current, changed)
	if string(result["demo"]["/opt/config.yaml"]) != "foo: bar\n" {
		t.Fatalf("unexpected read current files: %v", result)
	}
}

func TestToolFilesAndChangedFiles(t *testing.T) {
	hash := "sha256-" + strings.Repeat("1", 64)
	g := &generation.Generation{
		Manifest: generation.Manifest{
			Tools: map[string]generation.Tool{
				"demo": {
					Hash:  hash,
					Files: map[string]string{"/cfg.yaml": "h1"},
				},
			},
		},
		Files: map[string][]byte{
			"store/" + hash + "/files/cfg.yaml": []byte("key: value\n"),
		},
	}
	extracted := ToolFiles(g)
	if string(extracted["demo"]["/cfg.yaml"]) != "key: value\n" {
		t.Fatalf("unexpected tool files: %v", extracted)
	}

	want := g.Manifest
	current := generation.Manifest{
		Tools: map[string]generation.Tool{
			"demo": {
				Hash:  "sha256-" + strings.Repeat("2", 64),
				Files: map[string]string{"/cfg.yaml": "h0"},
			},
		},
	}
	changed := ChangedFiles(want, current)
	if len(changed["demo"]) != 1 || changed["demo"][0] != "/cfg.yaml" {
		t.Fatalf("unexpected changed files: %v", changed)
	}
}
