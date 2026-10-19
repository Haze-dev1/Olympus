// Package catalog fetches the recipe catalog at a pinned git revision, caches
// the extracted tree under ~/.cache/olympus/inputs/<hash>/, and computes the
// content hash recorded in stack.lock.
package catalog

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const (
	// HashPrefix prefixes every catalog hash, matching the lock file format.
	HashPrefix = "sha256-"

	hashDomain = "olympus-catalog-v1\x00"

	maxTarball = 64 << 20
	maxTree    = 256 << 20
	maxFiles   = 10000
)

var (
	githubName = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
	revPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)
	hashRE     = regexp.MustCompile(`^sha256-[0-9a-f]{64}$`)
)

// Source identifies the catalog inside a GitHub repository.
type Source struct {
	Owner string
	Repo  string
	Path  string // subdirectory, empty for the whole repository
}

// ParseSource parses the stack file's recipe input, such as
// "github:haze/olympus/recipes". Only GitHub is accepted in v1.
func ParseSource(s string) (Source, error) {
	rest, ok := strings.CutPrefix(s, "github:")
	if !ok {
		return Source{}, fmt.Errorf("catalog: source %q: only github:owner/repo[/path] is supported", s)
	}
	parts := strings.Split(rest, "/")
	if len(parts) < 2 {
		return Source{}, fmt.Errorf("catalog: source %q: want github:owner/repo[/path]", s)
	}
	src := Source{Owner: parts[0], Repo: parts[1], Path: strings.Join(parts[2:], "/")}
	if !githubName.MatchString(src.Owner) || !githubName.MatchString(src.Repo) ||
		src.Owner == "." || src.Owner == ".." || src.Repo == "." || src.Repo == ".." {
		return Source{}, fmt.Errorf("catalog: source %q: invalid owner or repository", s)
	}
	if src.Path != "" {
		if strings.HasPrefix(src.Path, "/") || path.Clean(src.Path) != src.Path || strings.HasPrefix(src.Path, "..") {
			return Source{}, fmt.Errorf("catalog: source %q: invalid catalog path", s)
		}
		for _, part := range strings.Split(src.Path, "/") {
			if part == "" || part == "." || part == ".." {
				return Source{}, fmt.Errorf("catalog: source %q: invalid catalog path", s)
			}
		}
	}
	return src, nil
}

// Fetcher downloads and caches catalogs. Its zero value is usable: it uses the
// default HTTP client, the GitHub tarball host and DefaultCacheDir.
type Fetcher struct {
	Client   *http.Client
	CacheDir string // root for extracted trees; empty means DefaultCacheDir
	BaseURL  string // tarball host; empty means https://codeload.github.com
	APIURL   string // empty means https://api.github.com
	Token    string // GitHub credential; never persisted or included in errors
}

// Fetch downloads the tarball for rev, extracts its source path, computes the
// content hash and stores the tree under the cache. If wantHash is not empty,
// the computed hash must match it; a mismatch is an error. The returned hash is
// the "sha256-<hex>" value to record in stack.lock.
func (f Fetcher) Fetch(ctx context.Context, src Source, rev, wantHash string) (dir, hash string, err error) {
	if !revPattern.MatchString(rev) {
		return "", "", fmt.Errorf("catalog: revision %q is not a full 40-character commit hash", rev)
	}
	if wantHash != "" && !hashRE.MatchString(wantHash) {
		return "", "", fmt.Errorf("catalog: invalid hash %q", wantHash)
	}
	cacheRoot := f.CacheDir
	if cacheRoot == "" {
		cacheRoot, err = DefaultCacheDir()
		if err != nil {
			return "", "", err
		}
	}
	if err := os.MkdirAll(cacheRoot, 0o700); err != nil {
		return "", "", fmt.Errorf("catalog: create cache: %w", err)
	}

	if wantHash != "" {
		cached := filepath.Join(cacheRoot, wantHash)
		if info, statErr := os.Stat(cached); statErr == nil && info.IsDir() {
			if got, hashErr := HashDir(cached); hashErr == nil && got == wantHash {
				return cached, wantHash, nil
			}
			// A damaged cache entry is never used; refetch below.
			if err := os.RemoveAll(cached); err != nil {
				return "", "", fmt.Errorf("catalog: remove damaged cache entry: %w", err)
			}
		}
	}

	tmp, err := os.MkdirTemp(cacheRoot, ".tmp-")
	if err != nil {
		return "", "", fmt.Errorf("catalog: create temporary directory: %w", err)
	}
	defer os.RemoveAll(tmp)

	if err := f.download(ctx, src, rev, tmp); err != nil {
		return "", "", err
	}
	got, err := HashDir(tmp)
	if err != nil {
		return "", "", err
	}
	if wantHash != "" && got != wantHash {
		return "", "", fmt.Errorf("catalog: hash mismatch for %s at %s: got %s, want %s", src, rev, got, wantHash)
	}

	final := filepath.Join(cacheRoot, got)
	if err := os.Rename(tmp, final); err != nil {
		// Another process may have populated the same content first.
		if existing, hashErr := HashDir(final); hashErr != nil || existing != got {
			return "", "", fmt.Errorf("catalog: store catalog: %w", err)
		}
	}
	return final, got, nil
}

// DefaultCacheDir returns ~/.cache/olympus/inputs.
func DefaultCacheDir() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("catalog: locate user cache directory: %w", err)
	}
	return filepath.Join(dir, "olympus", "inputs"), nil
}

// request keeps credentials on the configured GitHub origins, including redirects.
func (f Fetcher) request(ctx context.Context, endpoint string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, errors.New("catalog: invalid GitHub endpoint")
	}
	allowed := func(u *url.URL) bool {
		for _, base := range []string{f.apiURL(), f.baseURL()} {
			origin, e := url.Parse(base)
			if e == nil && u.Scheme == origin.Scheme && u.Host == origin.Host {
				return true
			}
		}
		return false
	}
	if f.Token != "" && allowed(req.URL) {
		req.Header.Set("Authorization", "Bearer "+f.Token)
	}
	client := *http.DefaultClient
	if f.Client != nil {
		client = *f.Client
	}
	previous := client.CheckRedirect
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		req.Header.Del("Authorization")
		if len(via) >= 10 {
			return errors.New("too many redirects")
		}
		if previous != nil {
			if err := previous(req, via); err != nil {
				return err
			}
		}
		req.Header.Del("Authorization")
		if f.Token != "" && allowed(req.URL) {
			req.Header.Set("Authorization", "Bearer "+f.Token)
		}
		return nil
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.New("catalog: GitHub request failed")
	}
	return resp, nil
}

func (f Fetcher) apiURL() string {
	if f.APIURL != "" {
		return strings.TrimSuffix(f.APIURL, "/")
	}
	return "https://api.github.com"
}

// Latest resolves HEAD, GitHub's default-branch commit, to a full revision.
func (f Fetcher) Latest(ctx context.Context, src Source) (string, error) {
	resp, err := f.request(ctx, f.apiURL()+"/repos/"+src.Owner+"/"+src.Repo+"/commits/HEAD")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("catalog: commit lookup returned HTTP %d", resp.StatusCode)
	}
	var commit struct {
		SHA string `json:"sha"`
	}
	if json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&commit) != nil || !revPattern.MatchString(commit.SHA) {
		return "", errors.New("catalog: invalid default-branch commit response")
	}
	return commit.SHA, nil
}

func (f Fetcher) baseURL() string {
	if f.BaseURL != "" {
		return strings.TrimSuffix(f.BaseURL, "/")
	}
	return "https://codeload.github.com"
}

func (f Fetcher) download(ctx context.Context, src Source, rev, dst string) error {
	endpoint := f.baseURL() + "/" + src.Owner + "/" + src.Repo + "/tar.gz/" + rev
	resp, err := f.request(ctx, endpoint)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("catalog: tarball download returned HTTP %d", resp.StatusCode)
	}

	gz, err := gzip.NewReader(io.LimitReader(resp.Body, maxTarball))
	if err != nil {
		return fmt.Errorf("catalog: read tarball: %w", err)
	}
	defer gz.Close()
	if err := extract(gz, dst, src.Path); err != nil {
		return fmt.Errorf("catalog: extract %s: %w", src, err)
	}
	return nil
}

// extract writes the tarball's single top-level directory into dst, keeping
// only files below sub. It rejects entries that leave dst or the subdirectory.
func extract(r io.Reader, dst, sub string) error {
	tr := tar.NewReader(r)
	var (
		total   int64
		files   int
		matched bool
	)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		rel, ok, err := catalogPath(hdr.Name, sub)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		target := filepath.Join(dst, filepath.FromSlash(rel))
		if target != dst && !strings.HasPrefix(target, dst+string(os.PathSeparator)) {
			return fmt.Errorf("path %q escapes the extraction directory", hdr.Name)
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			files++
			if files > maxFiles {
				return fmt.Errorf("tarball has more than %d files", maxFiles)
			}
			if hdr.Size < 0 || total+hdr.Size > maxTree {
				return fmt.Errorf("tarball exceeds %d bytes", maxTree)
			}
			perm := hdr.FileInfo().Mode().Perm()
			if perm == 0 {
				perm = 0o644
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
			if err != nil {
				return err
			}
			n, copyErr := io.Copy(out, tr)
			closeErr := out.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
			if n != hdr.Size {
				return fmt.Errorf("short read for %q", hdr.Name)
			}
			if err := os.Chmod(target, perm); err != nil {
				return err
			}
			total += n
			matched = true
		default:
			return fmt.Errorf("unsupported tar entry %q (type %d)", hdr.Name, hdr.Typeflag)
		}
	}
	if sub != "" && !matched {
		return fmt.Errorf("catalog path %q not found in tarball", sub)
	}
	if files == 0 {
		return errors.New("tarball contains no files")
	}
	return nil
}

// catalogPath strips the tarball's top-level directory and, when sub is set,
// returns only paths inside it. It rejects absolute paths and traversal.
func catalogPath(name, sub string) (string, bool, error) {
	if name == "" || strings.ContainsRune(name, 0) || strings.Contains(name, `\`) || path.IsAbs(name) {
		return "", false, fmt.Errorf("invalid tar path %q", name)
	}
	clean := path.Clean(name)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", false, fmt.Errorf("tar path %q escapes the extraction directory", name)
	}
	parts := strings.Split(clean, "/")
	if len(parts) < 2 {
		return "", false, nil // inside the top-level directory only
	}
	rel := strings.Join(parts[1:], "/")
	if sub == "" {
		return rel, true, nil
	}
	if rel == sub {
		return "", false, nil
	}
	if !strings.HasPrefix(rel, sub+"/") {
		return "", false, nil
	}
	return strings.TrimPrefix(rel, sub+"/"), true, nil
}

// HashDir hashes a catalog tree: for each regular file in sorted path order,
// the relative slash-separated path, its permission bits in octal, its byte
// length and its bytes, each NUL-terminated. The stream starts with a version
// domain separator.
func HashDir(dir string) (string, error) {
	var paths []string
	err := filepath.WalkDir(dir, func(p string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&fs.ModeSymlink != 0 {
			return fmt.Errorf("catalog: %s is a symbolic link", p)
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("catalog: %s is not a regular file", p)
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		paths = append(paths, rel)
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(paths)

	h := sha256.New()
	io.WriteString(h, hashDomain)
	for _, rel := range paths {
		full := filepath.Join(dir, rel)
		info, err := os.Stat(full)
		if err != nil {
			return "", err
		}
		io.WriteString(h, filepath.ToSlash(rel))
		h.Write([]byte{0})
		io.WriteString(h, strconv.FormatUint(uint64(info.Mode().Perm()), 8))
		h.Write([]byte{0})
		io.WriteString(h, strconv.FormatInt(info.Size(), 10))
		h.Write([]byte{0})
		f, err := os.Open(full)
		if err != nil {
			return "", err
		}
		n, copyErr := io.Copy(h, f)
		closeErr := f.Close()
		if copyErr != nil {
			return "", copyErr
		}
		if closeErr != nil {
			return "", closeErr
		}
		if n != info.Size() {
			return "", fmt.Errorf("catalog: %s changed while hashing", full)
		}
		h.Write([]byte{0})
	}
	return HashPrefix + hex.EncodeToString(h.Sum(nil)), nil
}
