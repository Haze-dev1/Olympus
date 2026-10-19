//go:build lab

package deploy

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Haze-dev1/Olympus/internal/activate"
	"github.com/Haze-dev1/Olympus/internal/eval"
	"github.com/Haze-dev1/Olympus/internal/generation"
	"github.com/Haze-dev1/Olympus/internal/lock"
	"github.com/Haze-dev1/Olympus/internal/remote"
)

// The caller starts/resets the canonical VM. This test never manages a VM or
// falls back to the developer's SSH credentials or cache.
func TestLabDeploy(t *testing.T) {
	if os.Getenv("OLYMPUS_LAB") != "1" {
		t.Skip("set OLYMPUS_LAB=1 and pass lab.sh env settings")
	}
	for _, name := range []string{"OLYMPUS_LAB_HOST", "OLYMPUS_LAB_PORT", "OLYMPUS_LAB_USER", "OLYMPUS_LAB_SSH_KEY", "OLYMPUS_LAB_KNOWN_HOSTS", "XDG_CACHE_HOME"} {
		if os.Getenv(name) == "" {
			t.Fatalf("%s is required", name)
		}
	}
	cfg := remote.Config{HostKeyAlgorithms: []string{"ssh-ed25519"}, Host: net.JoinHostPort(os.Getenv("OLYMPUS_LAB_HOST"), os.Getenv("OLYMPUS_LAB_PORT")), User: os.Getenv("OLYMPUS_LAB_USER"), KeyFile: os.Getenv("OLYMPUS_LAB_SSH_KEY"), KnownHosts: os.Getenv("OLYMPUS_LAB_KNOWN_HOSTS"), Timeout: 10 * time.Second}
	c, err := Dial(cfg, eval.Version)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	if _, _, err = c.Conn.Run(ctx, remote.Sudo([]string{"docker", "info"}), nil); err == nil {
		t.Fatal("reset the lab VM first; Docker already exists")
	}
	if err = c.PrepareServer(ctx); err != nil {
		t.Fatal(err)
	}
	t.Log("reset VM: Docker and Compose installed through sudo")
	binary := filepath.Join(t.TempDir(), "olym")
	build := exec.CommandContext(ctx, "go", "build", "-o", binary, "../../cmd/olym")
	build.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS=linux", "GOARCH=amd64")
	if output, e := build.CombinedOutput(); e != nil {
		t.Fatalf("build: %v %s", e, output)
	}
	if err = c.UploadActivator(ctx, binary); err != nil {
		t.Fatal(err)
	}
	image, err := (lock.Resolver{}).Resolve(ctx, "busybox:1.37.0")
	if err != nil {
		t.Fatal(err)
	}
	makeGeneration := func(marker string) *generation.Generation {
		g := &generation.Generation{Manifest: generation.Manifest{Version: 1, Olym: eval.Version, Inputs: map[string]string{"lab": marker}, Tools: map[string]generation.Tool{"tiny": {Hash: generation.Hash([]byte("tiny")), Image: image, Files: map[string]string{}}}}, Files: map[string][]byte{"compose.yaml": []byte("services:\n  tiny:\n    image: " + image + "\n    command: [sleep, '3600']\n")}}
		if e := g.Finish(); e != nil {
			t.Fatal(e)
		}
		return g
	}
	first := makeGeneration("first")
	if err = c.UploadGeneration(ctx, first); err != nil {
		t.Fatal(err)
	}
	if err = c.SendSecrets(ctx, first.Manifest, map[string]ToolSecrets{}); err != nil {
		t.Fatal(err)
	}
	if code, e := c.Activate(ctx, first.Manifest.ID); e != nil || code != 0 {
		t.Fatalf("first activation: %d %v", code, e)
	}
	current, e := c.run(ctx, []string{"readlink", "-f", root + "/current"}, nil)
	if e != nil || strings.TrimSpace(string(current)) != root+"/store/gen-"+first.Manifest.ID {
		t.Fatalf("current: %s %v", current, e)
	}
	t.Log("generation uploaded, activated and confirmed; current verified")
	second := makeGeneration("second")
	if err = c.UploadGeneration(ctx, second); err != nil {
		t.Fatal(err)
	}
	c.Reconnect = func() (Connection, error) { return nil, errors.New("simulated client disconnect") }
	if code, e := c.Activate(ctx, second.Manifest.ID); code != 3 || e == nil {
		t.Fatalf("unconfirmed activation: %d %v", code, e)
	}
	deadline := time.Now().Add(90 * time.Second)
	for {
		b, e := c.run(ctx, []string{"cat", root + "/state.json"}, nil)
		var state activate.State
		if e == nil && json.Unmarshal(b, &state) == nil && state.ID == second.Manifest.ID && state.Status == activate.StatusRolledBack {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("server did not roll back: %s %v", b, e)
		}
		time.Sleep(time.Second)
	}
	current, e = c.run(ctx, []string{"readlink", "-f", root + "/current"}, nil)
	if e != nil || strings.TrimSpace(string(current)) != root+"/store/gen-"+first.Manifest.ID {
		t.Fatal("rollback did not preserve current")
	}
	t.Log("second activation was never confirmed; server rolled back autonomously")
}
