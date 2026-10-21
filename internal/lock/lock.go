// Package lock reads and writes stack.lock and resolves image tags to index
// digests through the container registry API.
package lock

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
)

// File is the contents of stack.lock. Map keys are written in sorted order by
// encoding/json.
type File struct {
	Version int              `json:"version"`
	Inputs  map[string]Input `json:"inputs"`
	Tools   map[string]Tool  `json:"tools"`
}

// Input pins one catalog input by revision and content hash.
type Input struct {
	URL  string `json:"url"`
	Rev  string `json:"rev"`
	Hash string `json:"hash"`
}

// Tool pins one tool's image by index digest.
type Tool struct {
	Image    string `json:"image"`
	ImageTag string `json:"image_tag,omitempty"`
}

var (
	revRE   = regexp.MustCompile(`^[0-9a-f]{40}$`)
	hashRE  = regexp.MustCompile(`^sha256-[0-9a-f]{64}$`)
	imageRE = regexp.MustCompile(`^[^\s@]+@sha256:[0-9a-f]{64}$`)
)

// Validate checks the parts of the format the CLI depends on: one known
// version, full revisions, sha256 content hashes and digest-pinned images.
func (f *File) Validate() error {
	if f == nil {
		return errors.New("lock: nil file")
	}
	if f.Version != 1 {
		return fmt.Errorf("lock: unsupported version %d", f.Version)
	}
	if len(f.Inputs) == 0 {
		return errors.New("lock: no inputs")
	}
	for name, in := range f.Inputs {
		if name == "" || in.URL == "" {
			return fmt.Errorf("lock: input %q has an empty url", name)
		}
		if !revRE.MatchString(in.Rev) {
			return fmt.Errorf("lock: input %q has an invalid rev %q", name, in.Rev)
		}
		if !hashRE.MatchString(in.Hash) {
			return fmt.Errorf("lock: input %q has an invalid hash %q", name, in.Hash)
		}
	}
	for name, tool := range f.Tools {
		if name == "" || !imageRE.MatchString(tool.Image) {
			return fmt.Errorf("lock: tool %q is not pinned to a digest: %q", name, tool.Image)
		}
	}
	return nil
}

// Read parses a lock file, rejecting unknown fields and trailing data.
func Read(path string) (*File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("lock: read: %w", err)
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var f File
	if err := dec.Decode(&f); err != nil {
		return nil, fmt.Errorf("lock: parse %s: %w", path, err)
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("lock: parse %s: trailing data", path)
	}
	if err := f.Validate(); err != nil {
		return nil, fmt.Errorf("lock: %s: %w", path, err)
	}
	return &f, nil
}

// Marshal returns the canonical encoding: two-space indent, sorted keys and
// one trailing newline.
func Marshal(f *File) ([]byte, error) {
	if err := f.Validate(); err != nil {
		return nil, err
	}
	c := *f
	if c.Inputs == nil {
		c.Inputs = map[string]Input{}
	}
	if c.Tools == nil {
		c.Tools = map[string]Tool{}
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("lock: encode: %w", err)
	}
	return append(data, '\n'), nil
}

// Write atomically replaces path with the canonical encoding.
func Write(path string, f *File) error {
	data, err := Marshal(f)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".stack.lock-*")
	if err != nil {
		return fmt.Errorf("lock: create temporary file: %w", err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("lock: write: %w", err)
	}
	if err := tmp.Chmod(0o644); err != nil {
		tmp.Close()
		return fmt.Errorf("lock: set mode: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("lock: close: %w", err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("lock: replace %s: %w", path, err)
	}
	return nil
}
