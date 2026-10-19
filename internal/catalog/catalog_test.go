package catalog

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const testRev = "0123456789abcdef0123456789abcdef01234567"

type entry struct {
	name string
	body string
	mode int64
	typ  byte
}

func tarball(t *testing.T, entries []entry) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, e := range entries {
		mode := e.mode
		if mode == 0 {
			mode = 0o644
		}
		hdr := &tar.Header{Name: e.name, Mode: mode, Size: int64(len(e.body))}
		if e.typ != 0 {
			hdr.Typeflag = e.typ
			hdr.Size = 0
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("write header %q: %v", e.name, err)
		}
		if e.typ == 0 || e.typ == tar.TypeReg {
			if _, err := tw.Write([]byte(e.body)); err != nil {
				t.Fatalf("write body %q: %v", e.name, err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func serve(t *testing.T, data []byte) string {
	t.Helper()
	server := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/tar.gz/"+testRev) {
			http.NotFound(w, r)
			return
		}
		w.Write(data)
	})
	return server
}

func newTestServer(t *testing.T, handler http.HandlerFunc) string {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv.URL
}

func writeFile(t *testing.T, path, body string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), mode); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
}

func TestHashDeterministic(t *testing.T) {
	first := t.TempDir()
	writeFile(t, filepath.Join(first, "a.txt"), "hello\n", 0o644)
	writeFile(t, filepath.Join(first, "bin", "run.sh"), "#!/bin/sh\n", 0o755)

	second := t.TempDir()
	writeFile(t, filepath.Join(second, "bin", "run.sh"), "#!/bin/sh\n", 0o755)
	writeFile(t, filepath.Join(second, "a.txt"), "hello\n", 0o644)
	if err := os.Chtimes(filepath.Join(second, "a.txt"), time.Unix(1, 0), time.Unix(1, 0)); err != nil {
		t.Fatal(err)
	}

	got, err := HashDir(first)
	if err != nil {
		t.Fatal(err)
	}
	again, err := HashDir(second)
	if err != nil {
		t.Fatal(err)
	}
	if got != again {
		t.Fatalf("hash depends on file order or timestamps: %s != %s", got, again)
	}
	const want = "sha256-84136af7b7750199ac0db0025f59773ee647333632d8477ef065b775bcf9ca25"
	if got != want {
		t.Fatalf("hash format changed: got %s, want %s", got, want)
	}

	if err := os.Chmod(filepath.Join(second, "a.txt"), 0o600); err != nil {
		t.Fatal(err)
	}
	changed, err := HashDir(second)
	if err != nil {
		t.Fatal(err)
	}
	if changed == got {
		t.Fatal("hash ignores file modes")
	}
}

func TestFetchRejectsTamperedTarball(t *testing.T) {
	src := Source{Owner: "haze", Repo: "olympus", Path: "recipes"}
	original := tarball(t, []entry{
		{name: "olympus-" + testRev + "/recipes/hermes/recipe.yaml", body: "name: hermes\n"},
	})
	tampered := tarball(t, []entry{
		{name: "olympus-" + testRev + "/recipes/hermes/recipe.yaml", body: "name: evil\n"},
	})

	// Hash the tree the lock should expect.
	unpacked := t.TempDir()
	gz, err := gzip.NewReader(bytes.NewReader(original))
	if err != nil {
		t.Fatal(err)
	}
	if err := extract(gz, unpacked, src.Path); err != nil {
		t.Fatal(err)
	}
	want, err := HashDir(unpacked)
	if err != nil {
		t.Fatal(err)
	}

	base := serve(t, tampered)
	f := Fetcher{CacheDir: t.TempDir(), BaseURL: base}
	if _, _, err := f.Fetch(context.Background(), src, testRev, want); err == nil {
		t.Fatal("expected a hash mismatch for a tampered tarball")
	} else if !strings.Contains(err.Error(), "hash mismatch") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFetchRejectsPathEscape(t *testing.T) {
	cases := []struct {
		name  string
		entry entry
		want  string
	}{
		{"parent traversal", entry{name: "top/../../evil.txt", body: "pwned"}, "escapes"},
		{"absolute path", entry{name: "/tmp/evil.txt", body: "pwned"}, "invalid tar path"},
		{"symlink", entry{name: "top/recipes/link", typ: tar.TypeSymlink}, "unsupported tar entry"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := Source{Owner: "haze", Repo: "olympus", Path: "recipes"}
			data := tarball(t, []entry{
				{name: "top/recipes/ok.txt", body: "ok"},
				tc.entry,
			})
			root := t.TempDir()
			f := Fetcher{CacheDir: root, BaseURL: serve(t, data)}
			_, _, err := f.Fetch(context.Background(), src, testRev, "")
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("got error %v, want one containing %q", err, tc.want)
			}
			if _, statErr := os.Stat(filepath.Join(root, "evil.txt")); !os.IsNotExist(statErr) {
				t.Fatal("a file escaped the extraction directory")
			}
		})
	}
}

func TestFetchUsesVerifiedCache(t *testing.T) {
	src := Source{Owner: "haze", Repo: "olympus", Path: "recipes"}
	data := tarball(t, []entry{
		{name: "top/recipes/ok.txt", body: "ok"},
	})
	cache := t.TempDir()
	base := serve(t, data)
	f := Fetcher{CacheDir: cache, BaseURL: base}
	dir, hash, err := f.Fetch(context.Background(), src, testRev, "")
	if err != nil {
		t.Fatal(err)
	}

	// The cache alone is enough after the first fetch.
	offline := Fetcher{CacheDir: cache, BaseURL: "http://127.0.0.1:1"}
	again, sameHash, err := offline.Fetch(context.Background(), src, testRev, hash)
	if err != nil {
		t.Fatalf("cached fetch failed: %v", err)
	}
	if again != dir || sameHash != hash {
		t.Fatalf("cache returned %s %s, want %s %s", again, sameHash, dir, hash)
	}

	// A modified cache entry is rejected and refetched, not trusted.
	if err := os.WriteFile(filepath.Join(dir, "ok.txt"), []byte("tampered"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := offline.Fetch(context.Background(), src, testRev, hash); err == nil {
		t.Fatal("expected a network error after the damaged cache entry was discarded")
	}
}

func TestParseSource(t *testing.T) {
	cases := []struct {
		in      string
		want    Source
		wantErr bool
	}{
		{"github:haze/olympus/recipes", Source{Owner: "haze", Repo: "olympus", Path: "recipes"}, false},
		{"github:Haze-dev1/Olympus", Source{Owner: "Haze-dev1", Repo: "Olympus"}, false},
		{"https://github.com/haze/olympus", Source{}, true},
		{"github:haze", Source{}, true},
		{"github:haze/olympus/../secrets", Source{}, true},
		{"github:haze/olympus//recipes", Source{}, true},
		{"github:../olympus", Source{}, true},
	}
	for _, tc := range cases {
		got, err := ParseSource(tc.in)
		if (err != nil) != tc.wantErr {
			t.Errorf("%q: got error %v, wantErr %v", tc.in, err, tc.wantErr)
			continue
		}
		if got != tc.want {
			t.Errorf("%q: got %+v, want %+v", tc.in, got, tc.want)
		}
	}
}

func TestLatestAndCredentialConfinement(t *testing.T) {
	const credential = "sentinel-private-github-token"
	var foreignRequests int
	foreign := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		foreignRequests++
		if r.Header.Get("Authorization") != "" {
			t.Error("credential followed redirect to another origin")
		}
		http.Error(w, credential, http.StatusForbidden)
	}))
	defer foreign.Close()
	var mode string
	github := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		want := "Bearer " + credential
		if mode == "public" {
			want = ""
		}
		if r.Header.Get("Authorization") != want {
			t.Error("unexpected GitHub authorization")
		}
		switch mode {
		case "redirect":
			http.Redirect(w, r, foreign.URL, http.StatusFound)
		case "invalid":
			w.Write([]byte(`{"sha":"bad"}`))
		case "error":
			http.Error(w, credential, http.StatusUnauthorized)
		default:
			w.Write([]byte(`{"sha":"` + testRev + `"}`))
		}
	}))
	defer github.Close()
	src := Source{Owner: "owner", Repo: "repo"}
	f := Fetcher{APIURL: github.URL, BaseURL: github.URL, Token: credential}
	for _, modeValue := range []string{"private", "public", "invalid", "error", "redirect"} {
		mode = modeValue
		f.Token = credential
		if mode == "public" {
			f.Token = ""
		}
		rev, err := f.Latest(context.Background(), src)
		if mode == "private" || mode == "public" {
			if err != nil || rev != testRev {
				t.Fatalf("lookup: %s %v", rev, err)
			}
		} else if err == nil || strings.Contains(err.Error(), credential) {
			t.Fatal("lookup failed to reject or leaked credential")
		}
	}
	mode = "redirect"
	f.Token = credential
	if _, _, err := f.Fetch(context.Background(), src, testRev, ""); err == nil || strings.Contains(err.Error(), credential) {
		t.Fatal("download failed to reject or leaked credential")
	}
	if foreignRequests != 2 {
		t.Fatalf("foreign requests: %d", foreignRequests)
	}
	github.Close()
	if _, err := f.Latest(context.Background(), src); err == nil || strings.Contains(err.Error(), credential) {
		t.Fatal("transport error leaked credential")
	}
}
