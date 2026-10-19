// Package generation defines offline artifacts and content hashes.
package generation

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"

	"github.com/Haze-dev1/Olympus/internal/encode"
	"github.com/Haze-dev1/Olympus/internal/recipe"
	"github.com/Haze-dev1/Olympus/internal/schema"
)

type Tool struct {
	Hash    string            `json:"hash"`
	Image   string            `json:"image"`
	Files   map[string]string `json:"files"`
	Secrets []string          `json:"secrets"`
}
type Manifest struct {
	Version int               `json:"version"`
	ID      string            `json:"id,omitempty"`
	Inputs  map[string]string `json:"inputs"`
	Tools   map[string]Tool   `json:"tools"`
	Olym    string            `json:"olym"`
}
type Generation struct {
	Manifest Manifest
	Files    map[string][]byte
	Warnings []string
}

func Hash(b []byte) string { return fmt.Sprintf("sha256-%x", sha256.Sum256(b)) }
func ToolHash(files map[string][]byte, service any) (string, error) {
	h := sha256.New()
	for _, p := range schema.Keys(files) {
		h.Write([]byte(p))
		h.Write([]byte{0})
		h.Write([]byte(strconv.Itoa(len(files[p]))))
		h.Write([]byte{0})
		h.Write(files[p])
	}
	b, err := encode.Canonical(service)
	if err != nil {
		return "", err
	}
	h.Write(b)
	return fmt.Sprintf("sha256-%x", h.Sum(nil)), nil
}
func (g *Generation) Finish() error {
	manifest := g.Manifest
	manifest.ID = ""
	b, err := encode.Canonical(manifest)
	if err != nil {
		return err
	}
	g.Manifest.ID = Hash(b)
	b, err = encode.Marshal("json", g.Manifest)
	if err != nil {
		return err
	}
	g.Files["manifest.json"] = b
	return nil
}

// Write requires a fresh or empty directory. It never deletes prior output.
// os.Root and exclusive file creation prevent escaping through existing links.
func (g *Generation) Write(dir string) error {
	for p := range g.Files {
		if !recipe.SafeRelative(p) {
			return fmt.Errorf("unsafe generated path")
		}
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("invalid build directory")
	}
	for parent := abs; ; parent = filepath.Dir(parent) {
		info, e := os.Lstat(parent)
		if e == nil && (info.Mode()&os.ModeSymlink != 0 || !info.IsDir()) {
			return fmt.Errorf("unsafe build directory ancestor")
		}
		if e != nil && !os.IsNotExist(e) {
			return fmt.Errorf("cannot inspect build directory")
		}
		if filepath.Dir(parent) == parent {
			break
		}
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("cannot create build directory")
	}
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("unsafe build directory")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("cannot read build directory")
	}
	if len(entries) > 0 {
		count := 0
		err = filepath.WalkDir(dir, func(p string, d fs.DirEntry, e error) error {
			if e != nil {
				return e
			}
			if d.Type()&os.ModeSymlink != 0 {
				return fmt.Errorf("symlink in output")
			}
			if d.IsDir() {
				return nil
			}
			rel, e := filepath.Rel(dir, p)
			if e != nil {
				return e
			}
			expected, ok := g.Files[filepath.ToSlash(rel)]
			if !ok {
				return fmt.Errorf("unexpected output file")
			}
			actual, e := recipe.ReadFile(dir, rel)
			if e != nil || !bytes.Equal(actual, expected) {
				return fmt.Errorf("output differs")
			}
			count++
			return nil
		})
		if err == nil && count == len(g.Files) {
			return nil
		}
		return fmt.Errorf("build directory contains different files; use -o with a fresh directory")
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return fmt.Errorf("cannot open build directory")
	}
	defer root.Close()
	for _, p := range schema.Keys(g.Files) {
		if !recipe.SafeRelative(p) {
			return fmt.Errorf("unsafe generated path")
		}
		if err = root.MkdirAll(filepath.Dir(p), 0700); err != nil {
			return fmt.Errorf("cannot create output directories")
		}
		f, e := root.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if e != nil {
			return fmt.Errorf("cannot create output file")
		}
		_, e = f.Write(g.Files[p])
		closeErr := f.Close()
		if e != nil || closeErr != nil {
			return fmt.Errorf("cannot write output file")
		}
	}
	return nil
}
