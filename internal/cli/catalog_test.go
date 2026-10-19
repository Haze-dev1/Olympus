package cli

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Haze-dev1/Olympus/internal/catalog"
	"github.com/Haze-dev1/Olympus/internal/generation"
	"github.com/Haze-dev1/Olympus/internal/lock"
)

func TestCatalogCommands(t *testing.T) {
	const token = "sentinel-github-credential"
	rev := strings.Repeat("a", 40)
	var githubRequests, registryRequests int
	registry := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		registryRequests++
		if r.Header.Get("Authorization") != "" {
			t.Error("GitHub credential reached registry")
		}
		w.Header().Set("Content-Type", "application/vnd.oci.image.index.v1+json")
		fmt.Fprint(w, `{"schemaVersion":2,"manifests":[{}]}`)
	}))
	defer registry.Close()
	var archive bytes.Buffer
	gz := gzip.NewWriter(&archive)
	tw := tar.NewWriter(gz)
	recipe := "name: demo\nkind: harness\nimage: " + strings.TrimPrefix(registry.URL, "http://") + "/demo:latest\n"
	if err := tw.WriteHeader(&tar.Header{Name: "repo/recipes/demo/recipe.yaml", Mode: 0644, Size: int64(len(recipe))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write([]byte(recipe)); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	github := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		githubRequests++
		if r.Header.Get("Authorization") != "Bearer "+token {
			t.Error("missing GitHub credential")
		}
		if r.URL.Path == "/repos/owner/repo/commits/HEAD" {
			fmt.Fprintf(w, `{"sha":%q}`, rev)
			return
		}
		if r.URL.Path != "/owner/repo/tar.gz/"+rev {
			t.Errorf("unexpected GitHub path %s", r.URL.Path)
		}
		w.Write(archive.Bytes())
	}))
	defer github.Close()
	dir := t.TempDir()
	file := filepath.Join(dir, "stack.yaml")
	body := []byte("inputs: {recipes: github:owner/repo/recipes}\nserver: {host: offline.test}\nagents: {assistant: {recipe: demo}}\n")
	if err := os.WriteFile(file, body, 0600); err != nil {
		t.Fatal(err)
	}
	fetch := catalog.Fetcher{APIURL: github.URL, BaseURL: github.URL, CacheDir: t.TempDir(), Token: token}
	resolver := lock.Resolver{Scheme: "http"}
	invoke := func(command string, extra ...string) (int, string) {
		t.Helper()
		var out, stderr bytes.Buffer
		args := append([]string{command, "-f", file}, extra...)
		code := run(args, &out, &stderr, fetch, resolver)
		output := out.String() + stderr.String()
		if strings.Contains(output, token) {
			t.Fatal("credential leaked in output")
		}
		return code, output
	}
	lockPath := filepath.Join(dir, "stack.lock")
	if code, output := invoke("build"); code != 1 || !strings.Contains(output, "stack.lock is missing") {
		t.Fatalf("missing lock build: %d %s", code, output)
	}

	for _, command := range []string{"check", "update"} {
		rev = strings.Repeat(map[string]string{"check": "a", "update": "b"}[command], 40)
		if code, output := invoke(command); code != 0 {
			t.Fatalf("%s: %s", command, output)
		}
		pinned, err := lock.Read(lockPath)
		if err != nil {
			t.Fatal(err)
		}
		if pinned.Inputs["recipes"].Rev != rev {
			t.Fatal("did not lock newest revision")
		}
	}
	if githubRequests != 4 || registryRequests != 2 {
		t.Fatalf("requests: github %d registry %d", githubRequests, registryRequests)
	}
	pinned, err := lock.Read(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	outputDir := filepath.Join(dir, "build")
	if code, output := invoke("build", "-o", outputDir); code != 0 {
		t.Fatal(output)
	}
	manifestBytes, err := os.ReadFile(filepath.Join(outputDir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(manifestBytes, []byte(token)) {
		t.Fatal("credential persisted in manifest")
	}
	var manifest generation.Manifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Inputs["recipes"] != rev || manifest.Tools["assistant"].Image != pinned.Tools["assistant"].Image {
		t.Fatal("build ignored lock")
	}
	compose, err := os.ReadFile(filepath.Join(outputDir, "compose.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(compose, []byte(pinned.Tools["assistant"].Image)) {
		t.Fatal("compose lacks locked digest")
	}
	if githubRequests != 4 || registryRequests != 2 {
		t.Fatal("locked build accessed network")
	}

	lockBytes, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	if code, output := invoke("check"); code != 0 {
		t.Fatal(output)
	}
	if bytes.Contains(lockBytes, []byte(token)) {
		t.Fatal("credential persisted in lock")
	}
	after, err := os.ReadFile(lockPath)
	if err != nil || !bytes.Equal(lockBytes, after) {
		t.Fatal("check changed existing lock")
	}
	local := filepath.Join(fetch.CacheDir, pinned.Inputs["recipes"].Hash)
	if code, output := invoke("build", "-recipes", local, "-o", outputDir); code != 0 || !strings.Contains(output, "skipping the lock's catalog pin") {
		t.Fatalf("local override: %d %s", code, output)
	}
	saved := pinned.Tools["assistant"]
	delete(pinned.Tools, "assistant")
	if err := lock.Write(lockPath, pinned); err != nil {
		t.Fatal(err)
	}
	for _, command := range []string{"check", "build"} {
		if code, output := invoke(command); code != 1 || !strings.Contains(output, "lacks tool assistant") {
			t.Fatalf("missing tool: %d %s", code, output)
		}
	}
	pinned.Tools["assistant"] = saved
	in := pinned.Inputs["recipes"]
	in.Hash = "sha256-" + strings.Repeat("0", 64)
	pinned.Inputs["recipes"] = in
	if err := lock.Write(lockPath, pinned); err != nil {
		t.Fatal(err)
	}
	if code, output := invoke("build"); code != 1 || !strings.Contains(output, "hash mismatch") {
		t.Fatalf("tampered hash: %d %s", code, output)
	}
	if _, err := os.Stat(filepath.Join(dir, ".olympus")); !os.IsNotExist(err) {
		t.Fatal("update/check wrote build output")
	}
}
