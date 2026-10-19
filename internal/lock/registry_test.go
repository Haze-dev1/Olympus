package lock

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

const indexBody = `{"schemaVersion":2,"mediaType":"application/vnd.oci.image.index.v1+json","manifests":[{"mediaType":"application/vnd.oci.image.manifest.v1+json","digest":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","size":123,"platform":{"architecture":"amd64","os":"linux"}}]}`

func digestOf(body string) string {
	sum := sha256.Sum256([]byte(body))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func hostOf(t *testing.T, raw string) string {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return u.Host
}

func TestResolveImageDigest(t *testing.T) {
	var accept string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/acme/tool/manifests/1.2" {
			http.NotFound(w, r)
			return
		}
		accept = r.Header.Get("Accept")
		w.Header().Set("Content-Type", ociIndex)
		w.Header().Set("Docker-Content-Digest", digestOf(indexBody))
		w.Write([]byte(indexBody))
	}))
	defer srv.Close()

	host := hostOf(t, srv.URL)
	r := Resolver{Scheme: "http"}
	got, err := r.Resolve(context.Background(), host+"/acme/tool:1.2")
	if err != nil {
		t.Fatal(err)
	}
	want := host + "/acme/tool@" + digestOf(indexBody)
	if got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
	if !strings.Contains(accept, ociIndex) {
		t.Fatalf("Accept header %q does not ask for an OCI index", accept)
	}
}

func TestResolveImageUsesBearerToken(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			if r.URL.Query().Get("service") != "registry.test" {
				t.Errorf("token service = %q", r.URL.Query().Get("service"))
			}
			if r.URL.Query().Get("scope") != "repository:acme/tool:pull" {
				t.Errorf("token scope = %q", r.URL.Query().Get("scope"))
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"token":"test-token"}`))
		case "/v2/acme/tool/manifests/latest":
			if r.Header.Get("Authorization") != "Bearer test-token" {
				w.Header().Set("WWW-Authenticate", `Bearer realm="`+srv.URL+`/token",service="registry.test",scope="repository:acme/tool:pull"`)
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.Header().Set("Content-Type", dockerList)
			w.Write([]byte(`{"schemaVersion":2,"mediaType":"` + dockerList + `","manifests":[{"digest":"sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	host := hostOf(t, srv.URL)
	r := Resolver{Scheme: "http"}
	got, err := r.Resolve(context.Background(), host+"/acme/tool")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got, host+"/acme/tool@sha256:") {
		t.Fatalf("unexpected result %s", got)
	}
}

func TestResolveImageRejectsSingleManifest(t *testing.T) {
	const single = `{"schemaVersion":2,"mediaType":"application/vnd.docker.distribution.manifest.v2+json","layers":[]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.docker.distribution.manifest.v2+json")
		w.Write([]byte(single))
	}))
	defer srv.Close()

	r := Resolver{Scheme: "http"}
	_, err := r.Resolve(context.Background(), hostOf(t, srv.URL)+"/acme/tool:1.2")
	if err == nil || !strings.Contains(err.Error(), "not a multi-arch index") {
		t.Fatalf("got error %v, want a multi-arch rejection", err)
	}
}

func TestResolveImageRejectsBadDigestHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", ociIndex)
		w.Header().Set("Docker-Content-Digest", "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc")
		w.Write([]byte(indexBody))
	}))
	defer srv.Close()

	r := Resolver{Scheme: "http"}
	_, err := r.Resolve(context.Background(), hostOf(t, srv.URL)+"/acme/tool:1.2")
	if err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("got error %v, want a digest mismatch", err)
	}
}

func TestResolvePinnedImageNeedsNoNetwork(t *testing.T) {
	const pinned = "ghcr.io/acme/tool@sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
	got, err := Resolver{}.Resolve(context.Background(), pinned)
	if err != nil {
		t.Fatal(err)
	}
	if got != pinned {
		t.Fatalf("got %s, want %s", got, pinned)
	}
}

func TestSplitRegistry(t *testing.T) {
	cases := []struct {
		name, registry, repo string
	}{
		{"alpine", "registry-1.docker.io", "library/alpine"},
		{"nousresearch/hermes-agent", "registry-1.docker.io", "nousresearch/hermes-agent"},
		{"docker.io/library/alpine", "registry-1.docker.io", "library/alpine"},
		{"ghcr.io/acme/tool", "ghcr.io", "acme/tool"},
		{"localhost:5000/tool", "localhost:5000", "tool"},
	}
	for _, tc := range cases {
		registry, repo := splitRegistry(tc.name)
		if registry != tc.registry || repo != tc.repo {
			t.Errorf("%q: got %s/%s, want %s/%s", tc.name, registry, repo, tc.registry, tc.repo)
		}
	}
}
