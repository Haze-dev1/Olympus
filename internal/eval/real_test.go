package eval

import (
	"bytes"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Haze-dev1/Olympus/internal/encode"
	"github.com/Haze-dev1/Olympus/internal/generation"
	"github.com/Haze-dev1/Olympus/internal/stack"
)

const (
	hermesPinned   = "nousresearch/hermes-agent@sha256:" + "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	codexPinned    = "node:22-bookworm-slim@sha256:" + "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	opencodePinned = "ghcr.io/anomalyco/opencode@sha256:" + "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
)

func realInputs(t *testing.T) Inputs {
	t.Helper()
	s, d := stack.Load("../../testdata/real/stack.yaml")
	if len(d.Errors) > 0 {
		t.Fatal(d.Errors)
	}
	in, errs := Load(s, "../../recipes", "../../testdata/real")
	if len(errs) > 0 {
		t.Fatal(errs)
	}
	in.Images = map[string]string{"hermes": hermesPinned, "codex": codexPinned, "opencode": opencodePinned}
	return in
}

func toolFile(t *testing.T, g *generation.Generation, tool, containerPath string) []byte {
	t.Helper()
	entry, ok := g.Manifest.Tools[tool]
	if !ok {
		t.Fatalf("tool %s missing from manifest", tool)
	}
	rel := path.Join("store", entry.Hash, "files", strings.TrimPrefix(containerPath, "/"))
	b, ok := g.Files[rel]
	if !ok {
		t.Fatalf("generation lacks %s", rel)
	}
	return b
}

func object(t *testing.T, value any, name string) map[string]any {
	t.Helper()
	m, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("%s is %T, want object", name, value)
	}
	return m
}

func TestRealStackGolden(t *testing.T) {
	in := realInputs(t)
	g, err := Build(in)
	if err != nil {
		t.Fatal(err)
	}
	if len(g.Warnings) > 0 {
		t.Fatalf("unexpected warnings: %v", g.Warnings)
	}
	if *updateGolden {
		for p, b := range g.Files {
			target := filepath.Join("../../testdata/golden-real", p)
			if e := os.MkdirAll(filepath.Dir(target), 0700); e != nil {
				t.Fatal(e)
			}
			if e := os.WriteFile(target, b, 0600); e != nil {
				t.Fatal(e)
			}
		}
	}
	count := 0
	err = filepath.WalkDir("../../testdata/golden-real", func(p string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.IsDir() {
			return nil
		}
		rel, e := filepath.Rel("../../testdata/golden-real", p)
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
}

func TestRealStackConfigFormats(t *testing.T) {
	g, err := Build(realInputs(t))
	if err != nil {
		t.Fatal(err)
	}

	hermes := toolFile(t, g, "hermes", "/opt/data/config.yaml")
	hermesDoc := object(t, mustUnmarshal(t, "yaml", hermes), "hermes config")
	hermesMCP := object(t, hermesDoc["mcp_servers"], "hermes mcp_servers")
	github := object(t, hermesMCP["github"], "hermes github")
	githubEnv := object(t, github["env"], "hermes github env")
	if githubEnv["GITHUB_TOKEN"] != "${GITHUB_TOKEN}" {
		t.Errorf("hermes token = %v, want variable reference", githubEnv["GITHUB_TOKEN"])
	}

	codex := toolFile(t, g, "codex", "/root/.codex/config.toml")
	if bytes.Contains(codex, []byte("${")) {
		t.Errorf("codex config contains an interpolated secret: %s", codex)
	}
	codexDoc := object(t, mustUnmarshal(t, "toml", codex), "codex config")
	codexMCP := object(t, codexDoc["mcp_servers"], "codex mcp_servers")
	codexGithub := object(t, codexMCP["github"], "codex github")
	names, ok := codexGithub["env_vars"].([]any)
	if !ok || len(names) != 1 || names[0] != "GITHUB_TOKEN" {
		t.Errorf("codex env_vars = %v, want [GITHUB_TOKEN]", codexGithub["env_vars"])
	}

	opencode := toolFile(t, g, "opencode", "/root/.config/opencode/opencode.json")
	opencodeDoc := object(t, mustUnmarshal(t, "json", opencode), "opencode config")
	opencodeMCP := object(t, opencodeDoc["mcp"], "opencode mcp")
	opencodeGithub := object(t, opencodeMCP["github"], "opencode github")
	if opencodeGithub["type"] != "local" {
		t.Errorf("opencode type = %v, want local", opencodeGithub["type"])
	}
	command, ok := opencodeGithub["command"].([]any)
	if !ok || len(command) != 2 || command[0] != "npx" {
		t.Errorf("opencode command = %v", opencodeGithub["command"])
	}
	environment := object(t, opencodeGithub["environment"], "opencode environment")
	if environment["GITHUB_TOKEN"] != "{env:GITHUB_TOKEN}" {
		t.Errorf("opencode token = %v, want environment reference", environment["GITHUB_TOKEN"])
	}

	for p, b := range g.Files {
		if bytes.Contains(b, []byte("OLYM_SENTINEL_")) {
			t.Fatalf("secret value appears in %s", p)
		}
	}
}

func TestRealStackCompose(t *testing.T) {
	g, err := Build(realInputs(t))
	if err != nil {
		t.Fatal(err)
	}
	compose := object(t, mustUnmarshal(t, "yaml", g.Files["compose.yaml"]), "compose")
	services := object(t, compose["services"], "services")

	codex := object(t, services["codex"], "codex service")
	build := object(t, codex["build"], "codex build")
	codexHash := g.Manifest.Tools["codex"].Hash
	if want := "/var/lib/olympus/store/" + codexHash + "/files/root/.olympus-build"; build["context"] != want {
		t.Errorf("codex build context = %v, want %s", build["context"], want)
	}
	if build["dockerfile"] != "Dockerfile" {
		t.Errorf("codex dockerfile = %v", build["dockerfile"])
	}
	args := object(t, build["args"], "codex build args")
	if args["BASE_IMAGE"] != codexPinned {
		t.Errorf("codex base = %v, want pinned digest", args["BASE_IMAGE"])
	}
	if image, _ := codex["image"].(string); !strings.HasPrefix(image, "olympus/codex:sha256-") {
		t.Errorf("codex image = %v, want content-addressed local tag", codex["image"])
	}

	opencode := object(t, services["opencode"], "opencode service")
	if ports, ok := opencode["ports"].([]any); !ok || len(ports) != 1 || ports[0] != "127.0.0.1:4096:4096" {
		t.Errorf("opencode ports = %v", opencode["ports"])
	}
	if _, ok := services["hermes"].(map[string]any)["ports"]; ok {
		t.Error("hermes publishes a port")
	}
	if _, ok := codex["ports"]; ok {
		t.Error("codex publishes a port")
	}

	hermesService := object(t, services["hermes"], "hermes service")
	env := object(t, hermesService["environment"], "hermes environment")
	if env["TELEGRAM_ALLOWED_USERS"] != "@alice,@bob" {
		t.Errorf("allow-list = %v", env["TELEGRAM_ALLOWED_USERS"])
	}

	for _, name := range []string{"hermes", "codex", "opencode"} {
		init := object(t, services[name+"-init"], name+"-init service")
		depends := object(t, object(t, services[name], name+" service")["depends_on"], name+" depends_on")
		initDepends := object(t, depends[name+"-init"], name+" init dependency")
		if initDepends["condition"] != "service_completed_successfully" {
			t.Errorf("%s init condition = %v", name, initDepends["condition"])
		}
		if init["restart"] != "no" {
			t.Errorf("%s init restart = %v", name, init["restart"])
		}
	}
	hermesInit := object(t, services["hermes-init"], "hermes-init")
	entrypoint := hermesInit["entrypoint"].([]any)
	script, _ := entrypoint[2].(string)
	for _, want := range []string{"/opt/data/config.yaml", "/opt/data/skills/deploy-checklist/SKILL.md"} {
		if !strings.Contains(script, want) {
			t.Errorf("hermes init script lacks %s: %s", want, script)
		}
	}
}

func TestRealStackEmptyTelegramAllowListFails(t *testing.T) {
	in := realInputs(t)
	tool := in.Stack.Agents["hermes"]
	tool.Channels["telegram"] = stack.Channel{Token: "${TELEGRAM_BOT_TOKEN}"}
	in.Stack.Agents["hermes"] = tool
	if _, err := Build(in); err == nil || !strings.Contains(err.Error(), "allow-list") {
		t.Fatalf("error = %v, want allow-list rejection", err)
	}
}

func mustUnmarshal(t *testing.T, format string, b []byte) any {
	t.Helper()
	v, err := encode.Unmarshal(format, b)
	if err != nil {
		t.Fatalf("cannot decode %s: %v", format, err)
	}
	return v
}
