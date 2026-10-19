package activate

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// GCResult reports what a collection removed.
type GCResult struct {
	Generations []int    // numbers whose links were removed
	Entries     []string // store entries that were removed
	Freed       int64    // bytes freed
}

// GC keeps the newest keep generations plus the current one and deletes only
// the store entries no kept generation references (TRD 8.6). Volumes and
// secrets are never touched.
func (a *Activator) GC(ctx context.Context, keep int) (GCResult, error) {
	var result GCResult
	if keep < 1 {
		return result, fmt.Errorf("activate: keep must be at least 1, got %d", keep)
	}
	if err := a.ensureRoot(); err != nil {
		return result, err
	}
	lock, err := a.lock(fmt.Sprintf("gc %d", keep))
	if err != nil {
		return result, err
	}
	defer lock.Close()
	endLog, err := a.beginLog("gc")
	if err != nil {
		return result, err
	}
	defer endLog()

	numbers, err := a.generations()
	if err != nil {
		return result, err
	}
	_, current, haveCurrent, err := a.current()
	if err != nil {
		return result, err
	}
	kept := make(map[int]bool, keep+1)
	for i, n := range numbers {
		if i < keep || (haveCurrent && n == current) {
			kept[n] = true
		}
	}

	// Everything the kept generations still need: their own store entry and
	// every store path their Compose file points at.
	referenced := make(map[string]bool)
	for _, n := range numbers {
		if !kept[n] {
			continue
		}
		dst, err := filepath.EvalSymlinks(filepath.Join(a.generationsDir(), strconv.Itoa(n)))
		if err != nil {
			a.logf("generations/%d cannot be resolved: %v", n, err)
			continue
		}
		referenced[filepath.Base(dst)] = true
		compose, err := os.ReadFile(filepath.Join(dst, "compose.yaml"))
		if err != nil {
			a.logf("generations/%d has no readable compose file: %v", n, err)
			continue
		}
		for _, entry := range storeRefs(compose) {
			referenced[entry] = true
		}
	}

	entries, err := os.ReadDir(filepath.Join(a.root(), "store"))
	if err != nil {
		return result, fmt.Errorf("activate: read store: %w", err)
	}
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		// In-progress uploads live in dot directories and are never collected.
		if strings.HasPrefix(entry.Name(), ".") || referenced[entry.Name()] {
			continue
		}
		path := filepath.Join(a.root(), "store", entry.Name())
		size := dirSize(path)
		if err := os.RemoveAll(path); err != nil {
			return result, fmt.Errorf("activate: delete store entry %s: %w", entry.Name(), err)
		}
		a.logf("deleted store entry %s (%d bytes)", entry.Name(), size)
		result.Entries = append(result.Entries, entry.Name())
		result.Freed += size
	}
	for _, n := range numbers {
		if kept[n] {
			continue
		}
		if err := ctx.Err(); err != nil {
			return result, err
		}
		if err := os.Remove(filepath.Join(a.generationsDir(), strconv.Itoa(n))); err != nil {
			return result, fmt.Errorf("activate: delete generations/%d: %w", n, err)
		}
		a.logf("deleted generations/%d", n)
		result.Generations = append(result.Generations, n)
	}
	sort.Ints(result.Generations)
	sort.Strings(result.Entries)
	return result, nil
}

// storeRefPattern matches the store entries a generated Compose file mounts.
var storeRefPattern = regexp.MustCompile(`store/([^/'"\\\s]+)/`)

// storeRefs collects the store entries a Compose file references.
func storeRefs(compose []byte) []string {
	var entries []string
	for _, match := range storeRefPattern.FindAllSubmatch(compose, -1) {
		entries = append(entries, string(match[1]))
	}
	return entries
}

// dirSize is the number of regular-file bytes under path.
func dirSize(path string) int64 {
	var total int64
	filepath.WalkDir(path, func(_ string, entry fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if info, err := entry.Info(); err == nil && info.Mode().IsRegular() {
			total += info.Size()
		}
		return nil
	})
	return total
}
