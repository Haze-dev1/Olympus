//go:build lab

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Haze-dev1/Olympus/internal/activate"
	"github.com/Haze-dev1/Olympus/internal/encode"
	"github.com/Haze-dev1/Olympus/internal/remote"
	"github.com/Haze-dev1/Olympus/internal/stack"
)

// TestLabRealStack uses the private development catalog to resolve real image
// pins, then the worktree recipes so unmerged recipe fixes can be exercised.
// Run on a reset canonical VM with 6144 MiB RAM and lab.sh env.
func TestLabRealStack(t *testing.T) {
	if os.Getenv("OLYMPUS_LAB") != "1" {
		t.Skip("set OLYMPUS_LAB=1 with lab.sh env on a reset 6144 MiB VM")
	}
	for _, k := range []string{"OLYMPUS_LAB_HOST", "OLYMPUS_LAB_PORT", "OLYMPUS_LAB_USER", "OLYMPUS_LAB_SSH_KEY", "OLYMPUS_LAB_KNOWN_HOSTS", "XDG_CACHE_HOME"} {
		if os.Getenv(k) == "" {
			t.Fatalf("%s required", k)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Minute)
	defer cancel()
	dir := t.TempDir()
	binary := filepath.Join(dir, "olym")
	build := exec.CommandContext(ctx, "go", "build", "-o", binary, "../../cmd/olym")
	build.Env = append(os.Environ(), "GOOS=linux", "GOARCH=amd64", "CGO_ENABLED=0")
	if b, e := build.CombinedOutput(); e != nil {
		t.Fatalf("build: %v %s", e, b)
	}
	write := func(p, body string) {
		t.Helper()
		if e := os.WriteFile(p, []byte(body), 0600); e != nil {
			t.Fatal(e)
		}
	}
	if e := os.Mkdir(filepath.Join(dir, "lab-skill"), 0700); e != nil {
		t.Fatal(e)
	}
	write(filepath.Join(dir, "lab-skill", "SKILL.md"), "---\nname: lab-skill\ndescription: Inspect deployment health before changing a tool.\n---\nCheck health before changing a deployment.\n")
	recipes := filepath.Join(dir, "recipes")
	if err := filepath.WalkDir("../../recipes", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel("../../recipes", p)
		if err != nil {
			return err
		}
		dst := filepath.Join(recipes, rel)
		if d.IsDir() {
			return os.MkdirAll(dst, 0700)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(dst, b, 0600)
	}); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "stack.yaml")
	mcpScript, e := os.ReadFile("testdata/lab-mcp.sh")
	if e != nil {
		t.Fatal(e)
	}
	body := fmt.Sprintf(`inputs:
  recipes: github:Haze-dev1/Olympus/recipes?ref=development
server:
  host: %s
  port: %s
  user: %s
  ssh_key: %s
  confirm_timeout: 30s
mcps:
  lab:
    command: /bin/sh
    args: ["-c", %q]
    env: {GITHUB_TOKEN: '${LAB_MCP}'}
skills: [./lab-skill]
agents:
  hermes:
    recipe: hermes
    mcps: [lab]
    skills: all
    secrets: {model-key: '${LAB_MODEL}', github-token: '${LAB_MCP}'}
  codex:
    recipe: codex
    mcps: [lab]
    skills: all
    secrets: {api-key: '${LAB_MODEL}', github-token: '${LAB_MCP}'}
  opencode:
    recipe: opencode
    mcps: [lab]
    skills: all
    secrets: {model-key: '${LAB_MODEL}', github-token: '${LAB_MCP}'}
`, os.Getenv("OLYMPUS_LAB_HOST"), os.Getenv("OLYMPUS_LAB_PORT"), os.Getenv("OLYMPUS_LAB_USER"), os.Getenv("OLYMPUS_LAB_SSH_KEY"), string(mcpScript))
	write(file, body)
	// Values are generated test data, never user credentials.
	write(filepath.Join(dir, ".env"), "LAB_MODEL=OLYM_SENTINEL_model\nLAB_MCP=OLYM_SENTINEL_mcp\n")
	s, d := stack.Load(file)
	if len(d.Errors) > 0 {
		t.Fatal(d.Errors)
	}
	cfg, e := connectionConfig(s, os.Getenv("OLYMPUS_LAB_KNOWN_HOSTS"), &bytes.Buffer{})
	if e != nil {
		t.Fatal(e)
	}
	c, e := remote.Dial(cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer c.Close()
	remoteRun := func(args ...string) []byte {
		t.Helper()
		b, _, e := c.Run(ctx, remote.Sudo(args), nil)
		if e != nil {
			t.Fatalf("remote %s: %v", args[0], e)
		}
		return b
	}
	run := func(command string, extra ...string) string {
		t.Helper()
		args := []string{command, "-f", file, "-known-hosts", os.Getenv("OLYMPUS_LAB_KNOWN_HOSTS")}
		if command != "update" {
			args = append(args, "-recipes", recipes)
		}
		args = append(args, extra...)
		cmd := exec.CommandContext(ctx, binary, args...)
		cmd.Stdin = strings.NewReader("y\n")
		b, e := cmd.CombinedOutput()
		if bytes.Contains(b, []byte("OLYM_SENTINEL_")) {
			t.Fatal("sentinel in CLI output")
		}
		if e != nil {
			t.Fatalf("%s: %v %s", command, e, b)
		}
		t.Logf("%s: exit 0", command)
		return string(b)
	}
	compose := func(args ...string) []byte {
		return remoteRun(append([]string{"docker", "compose", "-p", "olympus", "-f", "/var/lib/olympus/current/compose.yaml"}, args...)...)
	}
	ids := func() map[string]string {
		b := remoteRun("docker", "ps", "-a", "--filter", "label=com.docker.compose.project=olympus", "--format", "{{.Label \"com.docker.compose.service\"}} {{.ID}}")
		out := map[string]string{}
		for _, line := range strings.Split(string(b), "\n") {
			f := strings.Fields(line)
			if len(f) == 2 {
				out[f[0]] = f[1]
			}
		}
		return out
	}
	run("update")
	run("check")
	run("plan")
	run("apply", "--yes", "-activator", binary)
	t.Log(strings.TrimSpace(string(remoteRun("docker", "compose", "version"))))
	for _, cmd := range []string{"pull", "up"} {
		help := string(remoteRun("docker", "compose", cmd, "--help"))
		for _, flag := range map[string][]string{"pull": {"--ignore-buildable"}, "up": {"--no-build", "--pull"}}[cmd] {
			if !strings.Contains(help, flag) {
				t.Fatalf("Compose lacks %s", flag)
			}
		}
	}
	verify := func() {
		t.Helper()
		var status []activate.ToolStatus
		if e := json.Unmarshal([]byte(run("status", "--json", "-activator", binary)), &status); e != nil || len(status) != 3 {
			t.Fatalf("status: %v", e)
		}
		for _, r := range status {
			if r.State != "running" || r.Health != "healthy" {
				t.Fatalf("unhealthy: %+v", r)
			}
		}
	}
	verify()
	if b := remoteRun("curl", "-fsS", "http://127.0.0.1:4096/global/health"); !bytes.Contains(b, []byte(`"healthy":true`)) {
		t.Fatalf("OpenCode health: %s", b)
	}
	t.Log(strings.TrimSpace(string(compose("exec", "-T", "codex", "codex", "--version"))))
	compose("exec", "-T", "hermes", "python", "-c", `import os
from hermes_cli.config import _env_ref_lookup
assert _env_ref_lookup("ANTHROPIC_API_KEY") == os.environ["ANTHROPIC_API_KEY"]
assert _env_ref_lookup("GITHUB_TOKEN") == os.environ["GITHUB_TOKEN"]
from gateway.config import _getenv
os.environ["TELEGRAM_BOT_TOKEN"] = os.environ["GITHUB_TOKEN"]
assert _getenv("TELEGRAM_BOT_TOKEN") == os.environ["GITHUB_TOKEN"]
from pathlib import Path
assert "OLYM_SENTINEL_" not in Path("/opt/data/.env").read_text()

`)
	configs := map[string]string{"hermes": "/opt/data/config.yaml", "codex": "/root/.codex/config.toml", "opencode": "/root/.config/opencode/opencode.json"}
	for name, p := range configs {
		b := compose("exec", "-T", name, "cat", p)
		if bytes.Contains(b, []byte("OLYM_SENTINEL_")) {
			t.Fatalf("sentinel in %s config", name)
		}
		format := map[string]string{"hermes": "yaml", "codex": "toml", "opencode": "json"}[name]
		doc, e := encode.Unmarshal(format, b)
		if e != nil {
			t.Fatal(e)
		}
		key := map[string]string{"hermes": "mcp_servers", "codex": "mcp_servers", "opencode": "mcp"}[name]
		mcps, ok := doc[key].(map[string]any)
		if !ok || mcps["lab"] == nil {
			t.Fatalf("missing %s MCP", name)
		}
		reference := map[string]string{"hermes": "${GITHUB_TOKEN}", "codex": "env_vars", "opencode": "{env:GITHUB_TOKEN}"}[name]
		if !bytes.Contains(b, []byte(reference)) {
			t.Fatalf("missing %s reference", name)
		}
		compose("exec", "-T", name, "/bin/sh", "-c", "command -v cp >/dev/null")
		if bytes.Contains(compose("logs", "--no-color", name), []byte("OLYM_SENTINEL_")) {
			t.Fatalf("sentinel in %s logs", name)
		}
		if strings.TrimSpace(string(remoteRun("stat", "-c", "%a", "/var/lib/olympus/secrets/"+name+"/environment"))) != "600" {
			t.Fatalf("wrong secret mode for %s", name)
		}

		mcpOut, _, err := c.Run(ctx, remote.Sudo([]string{"docker", "compose", "-p", "olympus", "-f", "/var/lib/olympus/current/compose.yaml", "exec", "-T", name, "/bin/sh", "-c", string(mcpScript)}), strings.NewReader("{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"initialize\",\"params\":{}}\n{\"jsonrpc\":\"2.0\",\"id\":2,\"method\":\"tools/list\",\"params\":{}}\n"))
		if err != nil || !bytes.Contains(mcpOut, []byte(`"tools":[]`)) || bytes.Contains(mcpOut, []byte("OLYM_SENTINEL_")) {
			t.Fatalf("%s MCP fixture failed", name)
		}

		skill := map[string]string{"hermes": "/opt/data/skills/lab-skill/SKILL.md", "codex": "/root/.agents/skills/lab-skill/SKILL.md", "opencode": "/root/.config/opencode/skills/lab-skill/SKILL.md"}[name]
		if !bytes.Contains(compose("exec", "-T", name, "cat", skill), []byte("Check health")) {
			t.Fatalf("missing %s skill", name)
		}
	}
	if b, _, err := c.Run(ctx, remote.Sudo([]string{"grep", "-R", "-l", "OLYM_SENTINEL_", "/var/lib/olympus/store", "/var/lib/olympus/log"}), nil); err == nil || len(b) > 0 {
		t.Fatal("sentinel in store or activation logs")
	}
	first := ids()
	if len(first) != 6 {
		t.Fatalf("want 6 services, got %d", len(first))
	}
	imageBefore := string(remoteRun("docker", "inspect", "--format", "{{.Image}}", first["codex"]))
	if out := run("apply", "--yes", "-activator", binary); !strings.Contains(out, "No changes.") {
		t.Fatalf("expected no-op: %s", out)
	}
	if !reflect.DeepEqual(first, ids()) {
		t.Fatal("no-op apply recreated a service")
	}
	// Only the OpenCode MCP list changes. Leave its secret declaration intact.
	changed := strings.Replace(body, "  opencode:\n    recipe: opencode\n    mcps: [lab]", "  opencode:\n    recipe: opencode\n    mcps: []", 1)
	write(file, changed)
	run("plan")
	run("apply", "--yes", "-activator", binary)
	second := ids()
	for name, id := range first {
		wantChange := name == "opencode" || name == "opencode-init"
		if (second[name] != id) != wantChange {
			t.Fatalf("incorrect recreation: %s", name)
		}
	}
	run("rollback", "-activator", binary)
	verify()
	third := ids()
	for _, name := range []string{"hermes", "hermes-init", "codex", "codex-init"} {
		if third[name] != first[name] {
			t.Fatalf("rollback recreated %s", name)
		}
	}
	if imageBefore != string(remoteRun("docker", "inspect", "--format", "{{.Image}}", third["codex"])) {
		t.Fatal("rollback changed built image")
	}
	run("destroy", "--delete-data", "-activator", binary)
	if len(ids()) != 0 {
		t.Fatal("destroy left containers")
	}
	if b := remoteRun("docker", "volume", "ls", "--filter", "label=com.docker.compose.project=olympus", "-q"); len(bytes.TrimSpace(b)) != 0 {
		t.Fatal("destroy left volumes")
	}
	if _, _, e := c.Run(ctx, remote.Sudo([]string{"test", "-e", "/var/lib/olympus"}), nil); e == nil {
		t.Fatal("destroy left server data")
	}
	t.Log("real stack lifecycle passed; model inference and chat were not attempted")
}
