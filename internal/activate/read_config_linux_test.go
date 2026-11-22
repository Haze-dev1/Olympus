package activate

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/Haze-dev1/Olympus/internal/generation"
)

func TestReadConfig(t *testing.T) {
	a := Activator{Root: t.TempDir()}
	hash := generation.Hash([]byte("tool"))
	body := []byte("{}")
	digest := generation.Hash(body)
	base := filepath.Join(a.Root, "store", hash, "files")
	if err := os.MkdirAll(filepath.Join(base, "real"), 0700); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(base, "real", "config.json")
	if err := os.WriteFile(target, body, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Dir(target), filepath.Join(base, "linked")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(base, "link.json")); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, path, hash, digest string
		limit                    int
		ok                       bool
	}{
		{"regular", "real/config.json", hash, digest, len(body), true},
		{"symlinked parent", "linked/config.json", hash, digest, len(body), false},
		{"symlinked leaf", "link.json", hash, digest, len(body), false},
		{"directory", "real", hash, digest, len(body), false},
		{"over limit", "real/config.json", hash, digest, len(body) - 1, false},
		{"digest mismatch", "real/config.json", hash, generation.Hash(nil), len(body), false},
		{"parent path", "real/../real/config.json", hash, digest, len(body), false},
		{"absolute path", target, hash, digest, len(body), false},
		{"invalid hash", "real/config.json", "..", digest, len(body), false},
		{"invalid digest", "real/config.json", hash, "invalid", len(body), false},
		{"invalid limit", "real/config.json", hash, digest, MaxConfigBytes + 1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := a.ReadConfig(tc.hash, tc.path, tc.limit, tc.digest)
			if tc.ok {
				if err != nil || !bytes.Equal(got, body) {
					t.Fatalf("got %q, %v, want %q", got, err, body)
				}
				return
			}
			if err != errReadConfig || len(got) != 0 {
				t.Fatalf("got %q, %v, want empty body and generic error", got, err)
			}
		})
	}
}
