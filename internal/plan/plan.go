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
	"sort"
	"strconv"
	"strings"

	"github.com/Haze-dev1/Olympus/internal/activate"
	"github.com/Haze-dev1/Olympus/internal/encode"
	"github.com/Haze-dev1/Olympus/internal/generation"
	"github.com/Haze-dev1/Olympus/internal/recipe"
	"github.com/Haze-dev1/Olympus/internal/remote"
	"github.com/Haze-dev1/Olympus/internal/schema"
	"github.com/Haze-dev1/Olympus/internal/stack"
)

// Reader runs bounded commands against a server.
type Reader interface {
	Run(context.Context, []string, io.Reader) ([]byte, []byte, error)
}

// KeyChange represents an added, changed, or removed configuration key.
type KeyChange struct {
	Path string
	Kind string
}

// FileChange holds the key-level changes for a single configuration file.
type FileChange struct {
	Path string
	Keys []KeyChange
}

// FileContents maps tool names to file paths and their contents.
type FileContents struct {
	Want    map[string]map[string][]byte
	Current map[string]map[string][]byte
}

// Change describes modifications to a single tool.
type Change struct {
	Name, Kind string
	Files      []string
	FileDiffs  []FileChange
	Drift      []string
	Image      bool
	Secret     bool
}

// Diff holds the overall differences between two generations.
type Diff struct {
	Changes   []Change
	Unchanged int
}

// Compare returns the difference between the desired and current manifests.
func Compare(want, current generation.Manifest, actual map[string]map[string]string, contents ...FileContents) Diff {
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
					fc := FileChange{Path: f}
					if len(contents) > 0 && contents[0].Want != nil && contents[0].Current != nil {
						wantBytes := contents[0].Want[n][f]
						curBytes := contents[0].Current[n][f]
						if len(wantBytes) > 0 && len(curBytes) > 0 {
							if keys, err := DiffConfig(f, curBytes, wantBytes); err == nil && len(keys) > 0 {
								fc.Keys = keys
							}
						}
					}
					change.FileDiffs = append(change.FileDiffs, fc)
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
			if len(c.FileDiffs) > 0 {
				for _, f := range c.FileDiffs {
					if len(f.Keys) > 0 {
						parts := make([]string, 0, len(f.Keys))
						for _, k := range f.Keys {
							parts = append(parts, fmt.Sprintf("%s %s", k.Path, k.Kind))
						}
						fmt.Fprintf(out, "  ~ %-16s config: %s (%s)\n", c.Name, f.Path, strings.Join(parts, ", "))
					} else {
						fmt.Fprintf(out, "  ~ %-16s config: %s\n", c.Name, f.Path)
					}
				}
			} else {
				for _, f := range c.Files {
					fmt.Fprintf(out, "  ~ %-16s config: %s\n", c.Name, f)
				}
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

const (
	maxConfigBytes = activate.MaxConfigBytes
	maxPlanBytes   = 16 << 20
	maxPlanReads   = 32
)

// DiffConfig parses oldBytes and newBytes according to filePath's format and returns sorted key-level changes.
// It falls back to an error for unsupported extensions or invalid structured content.
func DiffConfig(filePath string, oldBytes, newBytes []byte) ([]KeyChange, error) {
	if len(oldBytes) > maxConfigBytes || len(newBytes) > maxConfigBytes {
		return nil, errors.New("config exceeds semantic diff size limit")
	}
	format := configFormat(filePath)
	if format == "" {
		return nil, fmt.Errorf("unsupported config format: %s", filePath)
	}
	oldDoc, err := encode.Unmarshal(format, oldBytes)
	if err != nil {
		return nil, fmt.Errorf("parsing current config: %w", err)
	}
	newDoc, err := encode.Unmarshal(format, newBytes)
	if err != nil {
		return nil, fmt.Errorf("parsing desired config: %w", err)
	}
	keys, err := diffKeys("", oldDoc, newDoc)
	if err != nil {
		return nil, err
	}
	displayBytes := 0
	for _, key := range keys {
		displayBytes += len(key.Path) + len(key.Kind) + len(" , ")
		if displayBytes > 4096 {
			return nil, errors.New("config changes exceed display limit")
		}
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].Path == keys[j].Path {
			return keys[i].Kind < keys[j].Kind
		}
		return keys[i].Path < keys[j].Path
	})
	return keys, nil
}

func configFormat(filePath string) string {
	ext := strings.ToLower(path.Ext(filePath))
	switch ext {
	case ".yaml", ".yml":
		return "yaml"
	case ".toml":
		return "toml"
	case ".json":
		return "json"
	default:
		return ""
	}
}

func diffKeys(prefix string, oldMap, newMap map[string]any) ([]KeyChange, error) {
	keys := map[string]bool{}
	for k := range oldMap {
		keys[k] = true
	}
	for k := range newMap {
		keys[k] = true
	}
	var changes []KeyChange
	for _, k := range schema.Keys(keys) {
		p := k
		if len(k) > 256 {
			return nil, errors.New("config key path exceeds display limit")
		}
		if k == "" || strings.ContainsAny(k, ".,\\\"") || strings.ContainsFunc(k, func(r rune) bool { return !strconv.IsPrint(r) }) {
			p = strconv.QuoteToASCII(k)
		}
		if prefix != "" {
			p = prefix + "." + p
		}
		if len(p) > 256 {
			return nil, errors.New("config key path exceeds display limit")
		}
		vOld, hadOld := oldMap[k]
		vNew, hadNew := newMap[k]
		switch {
		case !hadOld:
			changes = append(changes, KeyChange{Path: p, Kind: "added"})
		case !hadNew:
			changes = append(changes, KeyChange{Path: p, Kind: "removed"})
		default:
			mOld, okOld := vOld.(map[string]any)
			mNew, okNew := vNew.(map[string]any)
			if okOld && okNew {
				nested, err := diffKeys(p, mOld, mNew)
				if err != nil {
					return nil, err
				}
				changes = append(changes, nested...)
			} else if !reflect.DeepEqual(vOld, vNew) {
				changes = append(changes, KeyChange{Path: p, Kind: "changed"})
			}
		}
	}
	return changes, nil
}

// ChangedFiles returns the file paths that changed between want and current for each tool.
func ChangedFiles(want, current generation.Manifest) map[string][]string {
	out := map[string][]string{}
	for n, w := range want.Tools {
		c, ok := current.Tools[n]
		if !ok {
			continue
		}
		for f, h := range w.Files {
			if c.Files[f] != h && c.Files[f] != "" {
				out[n] = append(out[n], f)
			}
		}
	}
	for n := range out {
		sort.Strings(out[n])
	}
	return out
}

// ToolFiles extracts the generated tool files for want.
func ToolFiles(g *generation.Generation) map[string]map[string][]byte {
	out := map[string]map[string][]byte{}
	if g == nil {
		return out
	}
	for name, tool := range g.Manifest.Tools {
		out[name] = map[string][]byte{}
		for f := range tool.Files {
			rel := path.Join("store", tool.Hash, "files", strings.TrimPrefix(f, "/"))
			if b, ok := g.Files[rel]; ok {
				out[name][f] = b
			}
		}
	}
	return out
}

// ReadCurrentFiles reads the store files for the specified changed files from the server.
func ReadCurrentFiles(ctx context.Context, read func(context.Context, string, string, int, string) ([]byte, error), current generation.Manifest, changed map[string][]string) map[string]map[string][]byte {
	out := map[string]map[string][]byte{}
	remaining, reads := maxPlanBytes, 0
	for _, toolName := range schema.Keys(changed) {
		tool, ok := current.Tools[toolName]
		if !ok || !hashPattern.MatchString(tool.Hash) {
			continue
		}
		out[toolName] = map[string][]byte{}
		for _, f := range changed[toolName] {
			if reads == maxPlanReads || remaining <= 0 {
				return out
			}
			if configFormat(f) == "" || !hashPattern.MatchString(tool.Files[f]) || !recipe.CleanAbsolute(f) || f == "/" || strings.ContainsAny(f, "\x00\r\n\x1b") {
				continue
			}
			limit := min(maxConfigBytes, remaining)
			reads++
			b, err := read(ctx, tool.Hash, strings.TrimPrefix(f, "/"), limit, tool.Files[f])
			remaining -= len(b)
			if err == nil && len(b) <= limit {
				out[toolName][f] = b
			}
		}
	}
	return out
}
