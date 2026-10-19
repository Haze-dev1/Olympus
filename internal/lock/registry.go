package lock

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

const (
	ociIndex   = "application/vnd.oci.image.index.v1+json"
	dockerList = "application/vnd.docker.distribution.manifest.list.v2+json"

	maxManifest = 8 << 20
	maxToken    = 1 << 20
)

var (
	digestRE   = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
	repoNameRE = regexp.MustCompile(`^[a-z0-9][a-z0-9._:/-]*$`)
	tagRE      = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.-]{0,127}$`)
	acceptAll  = strings.Join([]string{
		ociIndex,
		dockerList,
		"application/vnd.oci.image.manifest.v1+json",
		"application/vnd.docker.distribution.manifest.v2+json",
	}, ", ")
)

// Resolver resolves image references through a registry. Its zero value is
// usable: it uses the default HTTP client and HTTPS. Scheme exists so tests can
// point at an HTTP test server.
type Resolver struct {
	Client *http.Client
	Scheme string
}

// Resolve turns "repo/name:tag" into "repo/name@sha256:<index digest>". A
// reference that already carries a sha256 digest is returned unchanged. Only
// multi-architecture indexes and manifest lists are accepted, because the lock
// must work on both amd64 and arm64.
func (r Resolver) Resolve(ctx context.Context, ref string) (string, error) {
	ref = strings.TrimSpace(ref)
	if name, digest, ok := strings.Cut(ref, "@"); ok {
		if _, _, err := splitTag(name); err != nil || !digestRE.MatchString(digest) {
			return "", fmt.Errorf("lock: invalid image reference %q", ref)
		}
		return name + "@" + digest, nil
	}

	name, tag, err := splitTag(ref)
	if err != nil {
		return "", err
	}
	registry, repo := splitRegistry(name)
	scheme := r.Scheme
	if scheme == "" {
		scheme = "https"
	}
	endpoint := (&url.URL{
		Scheme: scheme,
		Host:   registry,
		Path:   "/v2/" + repo + "/manifests/" + tag,
	}).String()

	body, mediaType, err := r.manifest(ctx, endpoint, repo)
	if err != nil {
		return "", err
	}
	if mediaType != ociIndex && mediaType != dockerList {
		return "", fmt.Errorf("lock: %s is not a multi-arch index (media type %q)", ref, mediaType)
	}
	var index struct {
		Manifests []json.RawMessage `json:"manifests"`
	}
	if err := json.Unmarshal(body, &index); err != nil || len(index.Manifests) == 0 {
		return "", fmt.Errorf("lock: %s: response is not a valid image index", ref)
	}

	sum := sha256.Sum256(body)
	return name + "@sha256:" + hex.EncodeToString(sum[:]), nil
}

func splitTag(ref string) (name, tag string, err error) {
	name, tag = ref, "latest"
	if i := strings.LastIndex(ref, ":"); i > strings.LastIndex(ref, "/") {
		name, tag = ref[:i], ref[i+1:]
	}
	if !validRepoName(name) || !tagRE.MatchString(tag) {
		return "", "", fmt.Errorf("lock: invalid image reference %q", ref)
	}
	return name, tag, nil
}

func validRepoName(name string) bool {
	if !repoNameRE.MatchString(name) {
		return false
	}
	for _, part := range strings.Split(name, "/") {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}
	return true
}

// splitRegistry returns the API host and repository path. Docker Hub short
// names get the library/ prefix.
func splitRegistry(name string) (registry, repo string) {
	if i := strings.Index(name, "/"); i > 0 {
		first := name[:i]
		if strings.ContainsAny(first, ".:") || first == "localhost" {
			if first == "docker.io" || first == "index.docker.io" {
				return "registry-1.docker.io", name[i+1:]
			}
			return first, name[i+1:]
		}
		return "registry-1.docker.io", name
	}
	return "registry-1.docker.io", "library/" + name
}

func (r Resolver) manifest(ctx context.Context, endpoint, repo string) ([]byte, string, error) {
	resp, err := r.get(ctx, endpoint, "")
	if err != nil {
		return nil, "", err
	}
	if resp.StatusCode == http.StatusUnauthorized {
		challenge := resp.Header.Get("WWW-Authenticate")
		resp.Body.Close()
		token, err := r.token(ctx, challenge, repo)
		if err != nil {
			return nil, "", err
		}
		resp, err = r.get(ctx, endpoint, token)
		if err != nil {
			return nil, "", err
		}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("lock: registry returned %s for %s", resp.Status, endpoint)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxManifest+1))
	if err != nil {
		return nil, "", fmt.Errorf("lock: read manifest: %w", err)
	}
	if len(body) > maxManifest {
		return nil, "", fmt.Errorf("lock: manifest for %s exceeds %d bytes", repo, maxManifest)
	}
	sum := sha256.Sum256(body)
	digest := "sha256:" + hex.EncodeToString(sum[:])
	if got := resp.Header.Get("Docker-Content-Digest"); got != "" && got != digest {
		return nil, "", fmt.Errorf("lock: registry digest %s does not match the manifest (%s)", got, digest)
	}
	mediaType := resp.Header.Get("Content-Type")
	if i := strings.IndexByte(mediaType, ';'); i >= 0 {
		mediaType = mediaType[:i]
	}
	return body, strings.TrimSpace(mediaType), nil
}

func (r Resolver) token(ctx context.Context, challenge, repo string) (string, error) {
	if !strings.HasPrefix(challenge, "Bearer ") {
		return "", fmt.Errorf("lock: registry requires authentication (%s)", challenge)
	}
	params := parseChallenge(strings.TrimPrefix(challenge, "Bearer "))
	realm := params["realm"]
	if realm == "" {
		return "", fmt.Errorf("lock: registry auth challenge has no realm")
	}
	u, err := url.Parse(realm)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") {
		return "", fmt.Errorf("lock: invalid registry auth realm %q", realm)
	}
	q := u.Query()
	if service := params["service"]; service != "" {
		q.Set("service", service)
	}
	scope := params["scope"]
	if scope == "" {
		scope = "repository:" + repo + ":pull"
	}
	q.Set("scope", scope)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return "", fmt.Errorf("lock: build token request: %w", err)
	}
	resp, err := r.client().Do(req)
	if err != nil {
		return "", fmt.Errorf("lock: fetch registry token: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("lock: token endpoint returned %s", resp.Status)
	}
	var tok struct {
		Token       string `json:"token"`
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxToken)).Decode(&tok); err != nil {
		return "", fmt.Errorf("lock: parse registry token: %w", err)
	}
	if tok.Token != "" {
		return tok.Token, nil
	}
	if tok.AccessToken != "" {
		return tok.AccessToken, nil
	}
	return "", fmt.Errorf("lock: token endpoint returned no token")
}

func parseChallenge(s string) map[string]string {
	params := map[string]string{}
	for _, part := range strings.Split(s, ",") {
		key, value, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			continue
		}
		params[strings.ToLower(strings.TrimSpace(key))] = strings.Trim(strings.TrimSpace(value), `"`)
	}
	return params
}

func (r Resolver) get(ctx context.Context, endpoint, token string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("lock: build manifest request: %w", err)
	}
	req.Header.Set("Accept", acceptAll)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := r.client().Do(req)
	if err != nil {
		return nil, fmt.Errorf("lock: fetch manifest: %w", err)
	}
	return resp, nil
}

func (r Resolver) client() *http.Client {
	if r.Client != nil {
		return r.Client
	}
	return http.DefaultClient
}
