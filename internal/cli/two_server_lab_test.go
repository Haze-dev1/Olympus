//go:build lab

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Haze-dev1/Olympus/internal/catalog"
	"github.com/Haze-dev1/Olympus/internal/eval"
	"github.com/Haze-dev1/Olympus/internal/generation"
	"github.com/Haze-dev1/Olympus/internal/lock"
	"github.com/Haze-dev1/Olympus/internal/remote"
	"github.com/Haze-dev1/Olympus/internal/stack"
)

// TestLabTwoServers uses lab.sh env for A and the same connection variables
// with a _B suffix for B. Run it against two freshly reset amd64 lab VMs.
func TestLabTwoServers(t *testing.T) {
	if os.Getenv("OLYMPUS_LAB") != "1" || os.Getenv("OLYMPUS_LAB_PORT_B") == "" {
		t.Skip("set OLYMPUS_LAB=1 and load lab.sh env for two reset VMs (suffix B settings with _B)")
	}
	env := func(name string) string {
		t.Helper()
		value := os.Getenv(name)
		if value == "" {
			t.Fatalf("%s required", name)
		}
		return value
	}
	env("XDG_CACHE_HOME")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	dir := t.TempDir()
	binary := filepath.Join(dir, "olym")
	build := exec.CommandContext(ctx, "go", "build", "-o", binary, "../../cmd/olym")
	build.Env = append(os.Environ(), "GOOS=linux", "GOARCH=amd64", "CGO_ENABLED=0")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build activator: %v\n%s", err, output)
	}
	file := filepath.Join(dir, "stack.yaml")
	const body = `server:
  host: 127.0.0.1
apps:
  one:
    recipe: lab-one
    secrets: {credential: '${LAB_ONE}'}
  two:
    recipe: lab-two
    secrets: {credential: '${LAB_TWO}'}
`
	for name, contents := range map[string]string{
		"stack.yaml": body,
		".env":       "LAB_ONE=OLYM_SENTINEL_one\nLAB_TWO=OLYM_SENTINEL_two\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0600); err != nil {
			t.Fatal(err)
		}
	}
	pinned := &lock.File{Version: 1,
		Inputs: map[string]lock.Input{"recipes": {URL: "github:lab/recipes", Rev: strings.Repeat("a", 40), Hash: "sha256-" + strings.Repeat("a", 64)}},
		Tools:  map[string]lock.Tool{"one": {Image: labNginxImage}, "two": {Image: labNginxImage}},
	}
	if err := lock.Write(filepath.Join(dir, "stack.lock"), pinned); err != nil {
		t.Fatal(err)
	}

	var firstManifest []byte
	var firstHost string
	firstConfigs := make(map[string]string)
	type imageDigests struct{ id, platform string }
	firstImages := make(map[string]imageDigests)
	for i, suffix := range []string{"", "_B"} {
		s, diagnostics := stack.Load(file)
		stack.CheckEnv(diagnostics, file)
		if len(diagnostics.Errors) > 0 {
			t.Fatal(diagnostics.Errors)
		}
		// The CLI has no connection override flags. Keep both input files
		// byte-for-byte identical and supply each VM's transport settings
		// in memory before using the CLI's evaluation and apply path.
		port, err := strconv.Atoi(env("OLYMPUS_LAB_PORT" + suffix))
		if err != nil || port < 1 || port > 65535 {
			t.Fatalf("invalid lab SSH port for VM %d", i+1)
		}
		s.Server.Host = env("OLYMPUS_LAB_HOST" + suffix)
		s.Server.Port = port
		s.Server.User = env("OLYMPUS_LAB_USER" + suffix)
		s.Server.SSHKey = env("OLYMPUS_LAB_SSH_KEY" + suffix)
		known := env("OLYMPUS_LAB_KNOWN_HOSTS" + suffix)

		var output bytes.Buffer
		in, err := prepare(ctx, s, file, "testdata", "apply", &output, catalog.Fetcher{}, lock.Resolver{})
		if err != nil {
			t.Fatal(err)
		}
		g, err := eval.Build(in)
		if err != nil {
			t.Fatal(err)
		}
		cfg, err := connectionConfig(s, known, &output)
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Host == firstHost {
			t.Fatal("both lab targets point to the same SSH endpoint")
		}
		firstHost = cfg.Host
		conn, err := remote.Dial(cfg)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = conn.Close() })
		run := func(args ...string) []byte {
			t.Helper()
			out, _, err := conn.Run(ctx, remote.Sudo(args), nil)
			if err != nil {
				t.Fatalf("VM %d %s: %v", i+1, args[0], err)
			}
			return out
		}
		const root = "/var/lib/olympus"
		if _, _, err := conn.Run(ctx, remote.Sudo([]string{"docker", "info"}), nil); err == nil {
			t.Fatalf("VM %d already has Docker; reset it first", i+1)
		}
		code := manageGeneration("apply", in, g, file, known, binary, "", "", true, false, true, &output, &output)
		if strings.Contains(output.String(), "OLYM_SENTINEL_") {
			t.Fatalf("VM %d apply output leaked a sentinel", i+1)
		}
		if code != 0 {
			t.Fatalf("VM %d apply: exit %d\n%s", i+1, code, &output)
		}
		t.Logf("VM %d apply exit %d", i+1, code)

		manifestBytes := run("cat", root+"/current/manifest.json")
		var manifest generation.Manifest
		if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
			t.Fatalf("VM %d manifest: %v", i+1, err)
		}
		current := strings.TrimSpace(string(run("readlink", "-f", root+"/current")))
		if manifest.ID != g.Manifest.ID || current != root+"/store/gen-"+manifest.ID {
			t.Errorf("VM %d current %q with manifest ID %q, want store entry for %q", i+1, current, manifest.ID, g.Manifest.ID)
		}
		hash := generation.Hash(manifestBytes)
		t.Logf("VM %d generation %s, manifest %s", i+1, manifest.ID, hash)
		if i == 0 {
			firstManifest = manifestBytes
		} else if !bytes.Equal(firstManifest, manifestBytes) {
			t.Errorf("VM B manifest %s differs from VM A %s", hash, generation.Hash(firstManifest))
		}
		if !bytes.Equal(manifestBytes, g.Files["manifest.json"]) {
			t.Errorf("VM %d stored manifest differs from the local build", i+1)
		}

		for _, name := range []string{"one", "two"} {
			container := strings.TrimSpace(string(run("docker", "compose", "-p", "olympus", "-f", root+"/current/compose.yaml", "ps", "-q", name)))
			state := strings.Fields(string(run("docker", "inspect", "--format", "{{.State.Running}} {{.Config.Image}} {{.Image}} {{.ImageManifestDescriptor.Digest}}", container)))
			pinnedID := strings.TrimSpace(string(run("docker", "image", "inspect", "--format", "{{.Id}}", labNginxImage)))
			if len(state) != 4 || state[0] != "true" || state[1] != labNginxImage || state[2] != pinnedID ||
				!strings.HasPrefix(pinnedID, "sha256:") || !strings.HasPrefix(state[3], "sha256:") {
				t.Fatalf("VM %d %s inspect got %q, want running %s at %s with a SHA-256 platform digest", i+1, name, state, labNginxImage, pinnedID)
			}
			image := imageDigests{id: state[2], platform: state[3]}
			t.Logf("VM %d %s pinned image %s, image ID %s, platform manifest %s", i+1, name, labNginxImage, image.id, image.platform)
			if i == 0 {
				firstImages[name] = image
			} else if firstImages[name] != image {
				t.Errorf("VM B %s image digests %+v differ from VM A %+v", name, image, firstImages[name])
			}

			const config = "/etc/nginx/conf.d/default.conf"
			tool := g.Manifest.Tools[name]
			want := tool.Files[config]
			got := generation.Hash(run("cat", root+"/store/"+tool.Hash+"/files"+config))
			t.Logf("VM %d %s %s %s", i+1, name, config, got)
			if i == 0 {
				firstConfigs[name] = got
			} else if firstConfigs[name] != got {
				t.Errorf("VM B %s config hash %s differs from VM A %s", name, got, firstConfigs[name])
			}
			if got != want {
				t.Errorf("VM %d %s config hash %s, want %s", i+1, name, got, want)
			}
		}
	}
}
