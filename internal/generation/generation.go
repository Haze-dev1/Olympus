// Package generation defines offline artifacts and content hashes.
package generation

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
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

// Write stages a complete generation beside the destination, then atomically
// publishes it. Only a directory with a valid Olympus manifest may be replaced.
func (g *Generation) Write(dir string) (result error) {
	for p := range g.Files {
		if !recipe.SafeRelative(p) {
			return fmt.Errorf("unsafe generated path")
		}
	}
	abs, err := filepath.Abs(dir)
	if err != nil || filepath.Dir(abs) == abs {
		return fmt.Errorf("invalid build directory")
	}
	for ancestor := abs; ; ancestor = filepath.Dir(ancestor) {
		info, e := os.Lstat(ancestor)
		if e == nil && (info.Mode()&os.ModeSymlink != 0 || !info.IsDir()) {
			return fmt.Errorf("unsafe build directory ancestor")
		}
		if e != nil && !os.IsNotExist(e) {
			return fmt.Errorf("cannot inspect build directory")
		}
		if filepath.Dir(ancestor) == ancestor {
			break
		}
	}
	parentPath, name := filepath.Dir(abs), filepath.Base(abs)
	if err = os.MkdirAll(parentPath, 0700); err != nil {
		return fmt.Errorf("cannot create build parent")
	}
	parent, err := os.OpenRoot(parentPath)
	if err != nil {
		return fmt.Errorf("cannot open build parent")
	}
	defer parent.Close()
	old, err := parent.Lstat(name)
	exists := err == nil
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("cannot inspect build directory")
	}
	if exists && !ownedDirectory(parent, name) {
		return fmt.Errorf("build directory has no valid Olympus manifest; refusing to replace it")
	}
	stagePath, err := os.MkdirTemp(parentPath, "."+name+"-")
	if err != nil {
		return fmt.Errorf("cannot stage build directory")
	}
	stage := filepath.Base(stagePath)
	defer func() {
		if err := parent.RemoveAll(stage); err != nil && result == nil {
			result = fmt.Errorf("generation published, but previous output cleanup failed")
		}
	}()
	root, err := parent.OpenRoot(stage)
	if err != nil {
		return fmt.Errorf("cannot open staging directory")
	}
	err = g.writeFiles(root)
	closeErr := root.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return fmt.Errorf("cannot close staging directory")
	}
	if !ownedDirectory(parent, stage) {
		return fmt.Errorf("generation has no valid Olympus manifest")
	}
	if exists {
		current, e := parent.Lstat(name)
		if e != nil || !os.SameFile(old, current) || !ownedDirectory(parent, name) {
			return fmt.Errorf("build directory changed during staging")
		}
	}
	fd, err := parent.Open(".")
	if err != nil {
		return fmt.Errorf("cannot open build parent descriptor")
	}
	defer fd.Close()
	if err = publishDirectory(fd, stage, name, exists); err != nil {
		return fmt.Errorf("cannot atomically publish build directory: %w", err)
	}
	return nil
}
func (g *Generation) writeFiles(root *os.Root) error {
	for _, p := range schema.Keys(g.Files) {
		if err := root.MkdirAll(filepath.Dir(p), 0700); err != nil {
			return fmt.Errorf("cannot create output directories")
		}
		f, err := root.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return fmt.Errorf("cannot create output file")
		}
		_, err = f.Write(g.Files[p])
		closeErr := f.Close()
		if err != nil || closeErr != nil {
			return fmt.Errorf("cannot write output file")
		}
	}
	return nil
}
func ownedDirectory(parent *os.Root, name string) bool {
	info, err := parent.Lstat(name)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return false
	}
	root, err := parent.OpenRoot(name)
	if err != nil {
		return false
	}
	defer root.Close()
	info, err = root.Lstat("manifest.json")
	if err != nil || !info.Mode().IsRegular() || info.Size() > 4<<20 {
		return false
	}
	b, err := root.ReadFile("manifest.json")
	if err != nil {
		return false
	}
	var m Manifest
	if json.Unmarshal(b, &m) != nil || m.Version != 1 || m.Olym == "" || m.Inputs == nil || m.Tools == nil {
		return false
	}
	id := m.ID
	m.ID = ""
	canonical, err := encode.Canonical(m)
	return err == nil && id == Hash(canonical)
}
