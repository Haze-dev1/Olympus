package catalog

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"io"
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
		{"github:haze/olympus/recipes?ref=development", Source{Owner: "haze", Repo: "olympus", Path: "recipes", Ref: "development"}, false},
		{"github:haze/olympus?ref=feat%2Ftest", Source{Owner: "haze", Repo: "olympus", Ref: "feat/test"}, false},
		{"github:haze/olympus?ref=", Source{}, true},
		{"github:haze/olympus?ref=one&ref=two", Source{}, true},
		{"github:haze/olympus?token=x", Source{}, true},
		{"github:haze/olympus?ref=..%2Fbad", Source{}, true},
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

func TestRedirectNeverSendsTokenToDifferentHost(t *testing.T) {
	const token = "sentinel-token-12345"
	var server2ReceivedAuth string

	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server2ReceivedAuth = r.Header.Get("Authorization")
		w.Write([]byte(`{"sha":"` + testRev + `"}`))
	}))
	defer server2.Close()

	server1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, server2.URL, http.StatusFound)
	}))
	defer server1.Close()

	f := Fetcher{
		APIURL:  server1.URL,
		BaseURL: server2.URL,
		Token:   token,
	}

	_, err := f.Latest(context.Background(), Source{Owner: "owner", Repo: "repo"})
	if err != nil {
		t.Fatalf("Latest failed: %v", err)
	}
	if server2ReceivedAuth != "" {
		t.Fatalf("expected no Authorization header on redirected host, got %q", server2ReceivedAuth)
	}
}

func TestLatestPreservesRefSlashes(t *testing.T) {
	for _, ref := range []string{"", "development", "feat/real-stack", "refs/heads/feature/x", "release+candidate/v1"} {
		t.Run(ref, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				want := ref
				if want == "" {
					want = "HEAD"
				}
				if r.URL.Path != "/repos/owner/repo/commits/"+want || strings.Contains(r.URL.EscapedPath(), "%2F") || r.URL.RawQuery != "" {
					t.Errorf("lookup URL %s", r.URL)
				}
				w.Write([]byte(`{"sha":"` + testRev + `"}`))
			}))
			defer server.Close()
			rev, err := (Fetcher{APIURL: server.URL}).Latest(context.Background(), Source{Owner: "owner", Repo: "repo", Ref: ref})
			if err != nil || rev != testRev {
				t.Fatalf("lookup: %s %v", rev, err)
			}
		})
	}
}

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 0
	}
	return len(p), nil
}

func tarHeaderBytes(t *testing.T, name string, size int64) []byte {
	t.Helper()
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	if err := tw.WriteHeader(&tar.Header{
		Name:     name,
		Mode:     0o644,
		Size:     size,
		Typeflag: tar.TypeReg,
	}); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestExtractDecompressionBombCaps(t *testing.T) {
	t.Run("per-file cap", func(t *testing.T) {
		var buf bytes.Buffer
		tw := tar.NewWriter(&buf)
		hdr := &tar.Header{
			Name:     "recipes/big.txt",
			Mode:     0o644,
			Size:     maxFile + 1,
			Typeflag: tar.TypeReg,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		_ = tw.Close()

		dst := t.TempDir()
		err := extract(&buf, dst, "")
		if err == nil || !strings.Contains(err.Error(), "exceeds") {
			t.Fatalf("expected error exceeding file cap, got: %v", err)
		}
	})

	t.Run("total tree cap", func(t *testing.T) {
		// 4 files of maxFile (4 * 64MB = 256MB = maxTree), plus 1 byte in a 5th file
		var readers []io.Reader
		for i := 1; i <= 4; i++ {
			hdr := tarHeaderBytes(t, filepath.Join("recipes", strings.Repeat("a", i)+".txt"), maxFile)
			readers = append(readers, bytes.NewReader(hdr), io.LimitReader(zeroReader{}, maxFile))
		}
		hdr5 := tarHeaderBytes(t, "recipes/overflow.txt", 1)
		readers = append(readers, bytes.NewReader(hdr5))

		dst := t.TempDir()
		err := extract(io.MultiReader(readers...), dst, "")
		if err == nil || !strings.Contains(err.Error(), "tarball exceeds") {
			t.Fatalf("expected error exceeding total tree cap, got: %v", err)
		}
	})
}

func TestExpandedArchiveBudget(t *testing.T) {
	header := tarHeaderBytes(t, "root/outside/data", maxExpanded)
	r := io.MultiReader(bytes.NewReader(header), io.LimitReader(zeroReader{}, maxExpanded))
	if err := extract(r, t.TempDir(), "recipes"); err == nil || !strings.Contains(err.Error(), "expanded tarball exceeds") {
		t.Fatalf("got %v, want expanded archive limit", err)
	}
}

func TestArchiveEntryBudget(t *testing.T) {
	var archive bytes.Buffer
	tw := tar.NewWriter(&archive)
	for range maxFiles + 1 {
		if err := tw.WriteHeader(&tar.Header{Name: "root/outside/", Typeflag: tar.TypeDir}); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := extract(&archive, t.TempDir(), "recipes"); err == nil || !strings.Contains(err.Error(), "entries") {
		t.Fatalf("got %v, want archive entry limit", err)
	}
}
