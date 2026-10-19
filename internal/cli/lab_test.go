//go:build lab

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Haze-dev1/Olympus/internal/activate"
	"github.com/Haze-dev1/Olympus/internal/lock"
	"github.com/Haze-dev1/Olympus/internal/remote"
	"github.com/Haze-dev1/Olympus/internal/stack"
)

// Reset the canonical VM before running. All credentials/cache are explicit lab
// settings; no developer SSH files or real secrets are read.
func TestLabM1(t *testing.T) {
	if os.Getenv("OLYMPUS_LAB") != "1" {
		t.Skip("set OLYMPUS_LAB=1 with lab.sh env on a reset VM")
	}
	for _, key := range []string{"OLYMPUS_LAB_HOST", "OLYMPUS_LAB_PORT", "OLYMPUS_LAB_USER", "OLYMPUS_LAB_SSH_KEY", "OLYMPUS_LAB_KNOWN_HOSTS", "XDG_CACHE_HOME"} {
		if os.Getenv(key) == "" {
			t.Fatalf("%s required", key)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	dir := t.TempDir()
	binary := filepath.Join(dir, "olym")
	build := exec.CommandContext(ctx, "go", "build", "-o", binary, "../../cmd/olym")
	build.Env = append(os.Environ(), "GOOS=linux", "GOARCH=amd64", "CGO_ENABLED=0")
	if b, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, b)
	}
	recipes := filepath.Join(dir, "recipes")
	for _, name := range []string{"lab-one", "lab-two"} {
		if err := os.MkdirAll(filepath.Join(recipes, name), 0700); err != nil {
			t.Fatal(err)
		}
		for _, file := range []string{"recipe.yaml", "default.conf.tmpl"} {
			b, err := os.ReadFile(filepath.Join("testdata", name, file))
			if err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(filepath.Join(recipes, name, file), b, 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	file := filepath.Join(dir, "stack.yaml")
	body := fmt.Sprintf("server:\n  host: %s\n  port: %s\n  user: %s\n  ssh_key: %s\n  confirm_timeout: 5s\n  keep: 2\napps:\n  one:\n    recipe: lab-one\n    secrets: {credential: '${LAB_ONE}'}\n  two:\n    recipe: lab-two\n    secrets: {credential: '${LAB_TWO}'}\n", os.Getenv("OLYMPUS_LAB_HOST"), os.Getenv("OLYMPUS_LAB_PORT"), os.Getenv("OLYMPUS_LAB_USER"), os.Getenv("OLYMPUS_LAB_SSH_KEY"))
	if err := os.WriteFile(file, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	secretOne := "OLYM_SENTINEL_one_$literal\\end'quote\""
	writeEnv := func(first string) {
		t.Helper()
		b := fmt.Sprintf("LAB_ONE=%q\nLAB_TWO=OLYM_SENTINEL_two\n", first)
		if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(b), 0600); err != nil {
			t.Fatal(err)
		}
	}
	writeEnv(secretOne)
	s, d := stack.Load(file)
	if len(d.Errors) > 0 {
		t.Fatal(d.Errors)
	}
	cfg, err := connectionConfig(s, os.Getenv("OLYMPUS_LAB_KNOWN_HOSTS"), &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	c, err := remote.Dial(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	runRemote := func(args ...string) []byte {
		t.Helper()
		b, _, e := c.Run(ctx, remote.Sudo(args), nil)
		if e != nil {
			t.Fatalf("remote %s failed: %v", args[0], e)
		}
		return b
	}
	if _, _, e := c.Run(ctx, remote.Sudo([]string{"docker", "info"}), nil); e == nil {
		t.Fatal("reset the VM first; Docker already installed")
	}
	// Pinned multi-architecture nginx index verified during the first lab run.
	const image = "nginx@sha256:65645c7bb6a0661892a8b03b89d0743208a18dd2f3f17a54ef4b76fb8e2f2a10"
	pinned := &lock.File{Version: 1, Inputs: map[string]lock.Input{"recipes": {URL: "github:lab/recipes", Rev: strings.Repeat("a", 40), Hash: "sha256-" + strings.Repeat("a", 64)}}, Tools: map[string]lock.Tool{"one": {Image: image}, "two": {Image: image}}}
	if err = lock.Write(filepath.Join(dir, "stack.lock"), pinned); err != nil {
		t.Fatal(err)
	}
	argsFor := func(command string, extra ...string) []string {
		a := []string{command, "-f", file, "-recipes", recipes, "-known-hosts", os.Getenv("OLYMPUS_LAB_KNOWN_HOSTS")}
		return append(a, extra...)
	}
	run := func(command string, want int, extra ...string) string {
		t.Helper()
		cmd := exec.CommandContext(ctx, binary, argsFor(command, extra...)...)
		cmd.Stdin = strings.NewReader("y\n")
		b, e := cmd.CombinedOutput()
		code := 0
		if e != nil {
			if exit, ok := e.(*exec.ExitError); ok {
				code = exit.ExitCode()
			} else {
				t.Fatal(e)
			}
		}
		if bytes.Contains(b, []byte("OLYM_SENTINEL_")) {
			t.Fatal("secret leaked in CLI output")
		}
		if code != want {
			t.Fatalf("%s: exit %d, want %d: %s", command, code, want, b)
		}
		t.Logf("%s: exit %d", command, code)
		return string(b)
	}
	ids := func() string {
		return string(runRemote("docker", "ps", "--filter", "label=com.docker.compose.project=olympus", "--format", "{{.Names}} {{.ID}}"))
	}
	run("check", 0)
	saved := filepath.Join(dir, "plan.json")
	run("plan", 2, "--detailed-exitcode", "-out", saved)
	firstPlan, e := os.ReadFile(saved)
	if e != nil {
		t.Fatal(e)
	}
	var initial savedPlan
	if json.Unmarshal(firstPlan, &initial) != nil {
		t.Fatal("invalid saved plan")
	}
	for _, fp := range initial.Fingerprints {
		if len(fp) != 0 {
			t.Fatal("first-server plan persisted a publicly keyed hash")
		}
	}
	run("apply", 0, saved, "--yes", "-activator", binary)
	firstIDs := ids()
	var status []activate.ToolStatus
	if err = json.Unmarshal([]byte(run("status", 0, "--json")), &status); err != nil || len(status) != 2 {
		t.Fatalf("status: %v", err)
	}
	for _, row := range status {
		if row.State != "running" || row.Health != "healthy" || row.Generation != 1 || !strings.Contains(row.Image, "@sha256:") {
			t.Fatalf("status: %+v", row)
		}
	}
	// Check exact delivery without returning the value to test diagnostics.
	delivered := runRemote("docker", "compose", "-p", "olympus", "-f", "/var/lib/olympus/current/compose.yaml", "exec", "-T", "one", "printenv", "LAB_CREDENTIAL")
	if string(delivered) != secretOne+"\n" {
		t.Fatal("environment delivery changed secret bytes")
	}
	run("apply", 0, "--yes", "-activator", binary)
	if ids() != firstIDs {
		t.Fatal("no-op apply restarted containers")
	}
	run("apply", 1, saved, "--yes", "-activator", binary)
	config := filepath.Join(recipes, "lab-one", "default.conf.tmpl")
	if err = os.WriteFile(config, []byte("server { listen 80; location / { return 200 'changed'; } }\n"), 0600); err != nil {
		t.Fatal(err)
	}
	planText := run("plan", 2, "--detailed-exitcode")
	if !strings.Contains(planText, "1 unchanged. 1 to restart") {
		t.Fatalf("unexpected config diff: %s", planText)
	}
	run("apply", 0, "--yes", "-activator", binary)
	changedIDs := ids()
	idFor := func(all, name string) string {
		for _, line := range strings.Split(all, "\n") {
			if strings.HasPrefix(line, "olympus-"+name+"-") {
				return line
			}
		}
		return ""
	}
	if idFor(firstIDs, "one") == idFor(changedIDs, "one") || idFor(firstIDs, "two") != idFor(changedIDs, "two") {
		t.Fatal("config change restarted wrong tools")
	}
	run("rollback", 0, "-activator", binary)
	rollbackIDs := ids()
	if idFor(rollbackIDs, "two") != idFor(firstIDs, "two") {
		t.Fatal("rollback restarted unchanged tool")
	}
	run("apply", 0, "--yes", "-activator", binary)
	writeEnv("OLYM_SENTINEL_rotated")
	planText = run("plan", 2, "--detailed-exitcode")
	if !strings.Contains(planText, "secret changed") || !strings.Contains(planText, "1 unchanged. 1 to restart") {
		t.Fatal("secret-only diff incorrect")
	}
	beforeSecret := ids()
	run("apply", 0, "--yes", "-activator", binary)
	if after := ids(); idFor(after, "one") == idFor(beforeSecret, "one") || idFor(after, "two") != idFor(beforeSecret, "two") {
		t.Fatal("secret change recreated wrong services")
	}
	// A failing readiness check returns exit 3 and restores current.
	failedRecipe := filepath.Join(recipes, "lab-one", "recipe.yaml")
	goodRecipe, e := os.ReadFile(failedRecipe)
	if e != nil {
		t.Fatal(e)
	}
	badRecipe := bytes.ReplaceAll(goodRecipe, []byte("command: [nginx, -t]"), []byte("command: [\"false\"]"))
	badRecipe = bytes.ReplaceAll(badRecipe, []byte("timeout: 10s"), []byte("timeout: 1s"))
	if e = os.WriteFile(failedRecipe, badRecipe, 0600); e != nil {
		t.Fatal(e)
	}
	beforeFailure := string(runRemote("readlink", "-f", "/var/lib/olympus/current"))
	run("apply", 3, "--yes", "-activator", binary)
	if string(runRemote("readlink", "-f", "/var/lib/olympus/current")) != beforeFailure {
		t.Fatal("health failure changed current")
	}
	if e = os.WriteFile(failedRecipe, goodRecipe, 0600); e != nil {
		t.Fatal(e)
	}
	// Give the test time to kill the real CLI before confirmation is published.
	recipePath := filepath.Join(recipes, "lab-one", "recipe.yaml")
	recipeBody, err := os.ReadFile(recipePath)
	if err != nil {
		t.Fatal(err)
	}
	recipeBody = bytes.ReplaceAll(recipeBody, []byte("command: [nginx, -t]"), []byte("command: [sh, -c, 'sleep 4; nginx -t']"))
	if err = os.WriteFile(recipePath, recipeBody, 0600); err != nil {
		t.Fatal(err)
	}
	before := string(runRemote("readlink", "-f", "/var/lib/olympus/current"))
	var oldState activate.State
	if json.Unmarshal(runRemote("cat", "/var/lib/olympus/state.json"), &oldState) != nil {
		t.Fatal("invalid prior state")
	}
	cmd := exec.CommandContext(ctx, binary, argsFor("apply", "--yes", "-activator", binary)...)
	var killedOutput bytes.Buffer
	cmd.Stdout = &killedOutput
	cmd.Stderr = &killedOutput
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(45 * time.Second)
	started := false
	for time.Now().Before(deadline) {
		b := runRemote("systemctl", "list-units", "--type=service", "--state=running", "--no-legend", "olympus-activate-*")
		if strings.Contains(string(b), "olympus-activate-") {
			started = true
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !started {
		cmd.Process.Kill()
		cmd.Wait()
		t.Fatal("activation never started")
	}
	if err = cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait()
	// A contender must neither consume the first run's payload nor confirm it.
	run("apply", 4, "--yes", "-activator", binary)
	deadline = time.Now().Add(30 * time.Second)
	for {
		var state activate.State
		b := runRemote("cat", "/var/lib/olympus/state.json")
		if json.Unmarshal(b, &state) == nil && state.Status == activate.StatusRolledBack && state.Request != oldState.Request {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("server did not roll back after CLI kill: %s", b)
		}
		time.Sleep(200 * time.Millisecond)
	}
	if string(runRemote("readlink", "-f", "/var/lib/olympus/current")) != before {
		t.Fatal("CLI kill changed current")
	}
	t.Log("killed CLI after detached start; autonomous rollback verified")
	run("gc", 0, "-keep", "2", "-activator", binary)
	if b, _, e := c.Run(ctx, remote.Sudo([]string{"grep", "-R", "-l", "OLYM_SENTINEL_", "/var/lib/olympus/store", "/var/lib/olympus/log"}), nil); e == nil || len(b) > 0 {
		t.Fatal("sentinel in store or activation logs")
	}

	// No sentinel in durable artifacts, logs, or container logs.
	for _, name := range []string{"one", "two"} {
		b := runRemote("docker", "compose", "-p", "olympus", "-f", "/var/lib/olympus/current/compose.yaml", "logs", "--no-color", name)
		if bytes.Contains(b, []byte("OLYM_SENTINEL_")) {
			t.Fatal("sentinel in container logs")
		}
	}
	mode := string(runRemote("stat", "-c", "%a", "/var/lib/olympus/secrets/one/credential"))
	if strings.TrimSpace(mode) != "600" {
		t.Fatal("wrong secret mode")
	}
	runRemote("docker", "run", "-d", "--name", "olympus-lab-unrelated", image)
	runRemote("docker", "volume", "create", "olympus-lab-unrelated")
	run("destroy", 0, "-activator", binary)
	if strings.TrimSpace(ids()) != "" {
		t.Fatal("destroy left project containers")
	}
	runRemote("docker", "volume", "inspect", "olympus_one_data")
	runRemote("test", "-d", "/var/lib/olympus")
	run("destroy", 0, "--delete-data", "-activator", binary)
	if _, _, e := c.Run(ctx, remote.Sudo([]string{"test", "-e", "/var/lib/olympus"}), nil); e == nil {
		t.Fatal("delete-data kept server state")
	}
	runRemote("docker", "inspect", "olympus-lab-unrelated")
	runRemote("docker", "volume", "inspect", "olympus-lab-unrelated")
	runRemote("docker", "rm", "-f", "olympus-lab-unrelated")
	runRemote("docker", "volume", "rm", "olympus-lab-unrelated")
	t.Log("full cycle passed with pinned image " + image)
}
