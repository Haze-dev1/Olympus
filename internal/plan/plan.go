// Package plan compares generations without changing the server.
package plan

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"reflect"
	"regexp"
	"strings"

	"github.com/Haze-dev1/Olympus/internal/encode"
	"github.com/Haze-dev1/Olympus/internal/generation"
	"github.com/Haze-dev1/Olympus/internal/recipe"
	"github.com/Haze-dev1/Olympus/internal/remote"
	"github.com/Haze-dev1/Olympus/internal/schema"
	"github.com/Haze-dev1/Olympus/internal/stack"
)

type Reader interface {
	Run(context.Context, []string, io.Reader) ([]byte, []byte, error)
}

type Change struct {
	Name, Kind   string
	Files, Drift []string
	Image        bool
	Secret       bool
}
type Diff struct {
	Changes   []Change
	Unchanged int
}

func Compare(want, current generation.Manifest, actual map[string]map[string]string) Diff {
	d := Diff{}
	names := map[string]bool{}
	for n := range want.Tools {
		names[n] = true
	}
	for n := range current.Tools {
		names[n] = true
	}
	for _, n := range schema.Keys(names) {
		w, exists := want.Tools[n]
		c, had := current.Tools[n]
		change := Change{Name: n}
		switch {
		case !had:
			change.Kind = "+"
		case !exists:
			change.Kind = "-"
		default:
			change.Kind = "~"
			change.Image = w.Image != c.Image
			files := map[string]bool{}
			for f := range w.Files {
				files[f] = true
			}
			for f := range c.Files {
				files[f] = true
			}
			for _, f := range schema.Keys(files) {
				if w.Files[f] != c.Files[f] {
					change.Files = append(change.Files, f)
				}
			}
		}
		if had {
			for _, f := range schema.Keys(c.Files) {
				if actual[n][f] != c.Files[f] {
					change.Drift = append(change.Drift, f)
				}
			}
		}
		if exists && had && reflect.DeepEqual(w, c) && len(change.Drift) == 0 {
			d.Unchanged++
			continue
		}
		d.Changes = append(d.Changes, change)
	}
	return d
}

var hashPattern = regexp.MustCompile(`^sha256-[0-9a-f]{64}$`)

// ReadCurrent resolves current once, then reads only that immutable generation.
func ReadCurrent(ctx context.Context, r Reader, root bool) (generation.Manifest, map[string]map[string]string, error) {
	var m generation.Manifest
	actual := map[string]map[string]string{}
	run := func(args ...string) ([]byte, error) {
		if !root {
			args = remote.Sudo(args)
		}
		b, _, err := r.Run(ctx, args, nil)
		return b, err
	}
	if !root {
		uid, err := run("id", "-u")
		if err != nil || strings.TrimSpace(string(uid)) != "0" {
			return m, nil, fmt.Errorf("plan requires passwordless sudo: sudo -n id -u")
		}
	}
	target, err := run("readlink", "--", "/var/lib/olympus/current")
	if err != nil {
		var exit interface{ ExitStatus() int }
		if !errors.As(err, &exit) || exit.ExitStatus() != 1 {
			return m, nil, fmt.Errorf("cannot read current generation")
		}
		// A dangling or malformed current must not be mistaken for an empty server.
		_, linkErr := run("test", "-L", "/var/lib/olympus/current")
		_, existsErr := run("test", "-e", "/var/lib/olympus/current")
		if isAbsent(linkErr) && isAbsent(existsErr) {
			return m, actual, nil
		}
		return m, nil, fmt.Errorf("invalid current generation")
	}
	link := strings.TrimSuffix(string(target), "\n")
	if !regexp.MustCompile(`^generations/[1-9][0-9]*$`).MatchString(link) {
		return m, nil, fmt.Errorf("invalid current link")
	}
	resolved, err := run("readlink", "-f", "--", path.Join("/var/lib/olympus", link))
	if err != nil {
		return m, nil, fmt.Errorf("cannot resolve current generation")
	}
	dir := strings.TrimSuffix(string(resolved), "\n")
	const prefix = "/var/lib/olympus/store/gen-"
	if !strings.HasPrefix(dir, prefix) || !hashPattern.MatchString(strings.TrimPrefix(dir, prefix)) {
		return m, nil, fmt.Errorf("invalid generation store path")
	}
	b, err := run("head", "-c", "4194305", "--", dir+"/manifest.json")
	if err != nil || len(b) > 4<<20 || json.Unmarshal(b, &m) != nil || m.Version != 1 || m.Tools == nil || m.ID != strings.TrimPrefix(dir, prefix) {
		return m, nil, fmt.Errorf("invalid current manifest")
	}
	canonical := m
	canonical.ID = ""
	encoded, err := encode.Canonical(canonical)
	if err != nil || generation.Hash(encoded) != m.ID {
		return m, nil, fmt.Errorf("current manifest hash mismatch")
	}
	for _, n := range schema.Keys(m.Tools) {
		if !stack.ValidName(n) {
			return m, nil, fmt.Errorf("invalid manifest tool name")
		}
		if !hashPattern.MatchString(m.Tools[n].Hash) {
			return m, nil, fmt.Errorf("invalid manifest tool hash")
		}
		actual[n] = map[string]string{}
		for _, f := range schema.Keys(m.Tools[n].Files) {
			if !recipe.CleanAbsolute(f) || f == "/" || strings.ContainsAny(f, "\x00\r\n\x1b") || !hashPattern.MatchString(m.Tools[n].Files[f]) {
				return m, nil, fmt.Errorf("invalid manifest file")
			}
			file := path.Join("/var/lib/olympus/store", m.Tools[n].Hash, "files", strings.TrimPrefix(f, "/"))
			b, err := run("sha256sum", "--", file)
			if err != nil {
				_, missingErr := run("test", "-e", file)
				_, symlinkErr := run("test", "-L", file)
				if isAbsent(missingErr) && isAbsent(symlinkErr) {
					continue
				}
				return m, nil, fmt.Errorf("cannot hash current generation file")
			}
			fields := strings.Fields(string(b))
			if len(fields) < 1 || !hashPattern.MatchString("sha256-"+fields[0]) {
				return m, nil, fmt.Errorf("invalid server file hash")
			}
			actual[n][f] = "sha256-" + fields[0]
		}
	}
	return m, actual, nil
}
func isAbsent(err error) bool {
	var exit interface{ ExitStatus() int }
	return errors.As(err, &exit) && exit.ExitStatus() == 1
}

func (d Diff) Print(out io.Writer) {
	restart, add, remove := 0, 0, 0
	for _, c := range d.Changes {
		switch c.Kind {
		case "+":
			add++
			fmt.Fprintf(out, "  + %-16s new tool\n", c.Name)
		case "-":
			remove++
			fmt.Fprintf(out, "  - %-16s removed\n", c.Name)
		default:
			restart++
			if c.Secret {
				fmt.Fprintf(out, "  ~ %-16s secret changed\n", c.Name)
			}
			if c.Image {
				fmt.Fprintf(out, "  ~ %-16s image changed\n", c.Name)
			}
			for _, f := range c.Files {
				fmt.Fprintf(out, "  ~ %-16s config: %s\n", c.Name, f)
			}
			if !c.Secret && !c.Image && len(c.Files) == 0 && len(c.Drift) == 0 {
				fmt.Fprintf(out, "  ~ %-16s configuration changed\n", c.Name)
			}
		}
		for _, f := range c.Drift {
			fmt.Fprintf(out, "  ! %-16s drift: %s changed on server\n", c.Name, f)
		}
	}
	fmt.Fprintf(out, "\n%d unchanged. %d to restart, %d to add, %d to remove. Volumes are kept.\n", d.Unchanged, restart, add, remove)
}

// SecretChanges merges fingerprint changes without counting a tool twice.
func (d *Diff) SecretChanges(changed map[string]bool) {
	for _, name := range schema.Keys(changed) {
		if !changed[name] {
			continue
		}
		found := false
		for i := range d.Changes {
			if d.Changes[i].Name == name {
				d.Changes[i].Secret = true
				found = true
				break
			}
		}
		if !found {
			d.Changes = append(d.Changes, Change{Name: name, Kind: "~", Secret: true})
			d.Unchanged--
		}
	}
}
