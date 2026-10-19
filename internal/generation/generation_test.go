package generation

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/Haze-dev1/Olympus/internal/encode"
)

func TestHashesFollowCanonicalStreams(t *testing.T) {
	service := map[string]any{"image": "demo:1"}
	files := map[string][]byte{"/b": []byte("two"), "/a": []byte("one")}
	b, e := encode.Canonical(service)
	if e != nil {
		t.Fatal(e)
	}
	expected := Hash(append([]byte("/a\x003\x00one/b\x003\x00two"), b...))
	actual, e := ToolHash(files, service)
	if e != nil || actual != expected {
		t.Fatalf("wrong tool hash: %s, %v", actual, e)
	}
	g := &Generation{Manifest: Manifest{Version: 1, Tools: map[string]Tool{}, Inputs: map[string]string{}, Olym: "test"}, Files: map[string][]byte{}}
	b, e = encode.Canonical(g.Manifest)
	if e != nil {
		t.Fatal(e)
	}
	if e = g.Finish(); e != nil {
		t.Fatal(e)
	}
	if g.Manifest.ID != Hash(b) {
		t.Fatal("generation hash includes ID or differs from canonical manifest")
	}
	first := g.Manifest.ID
	if e = g.Finish(); e != nil || first != g.Manifest.ID {
		t.Fatal("finishing twice changed hash")
	}
}
func TestWriteConfinement(t *testing.T) {
	parent := t.TempDir()
	outside := t.TempDir()
	link := filepath.Join(parent, "link")
	if e := os.Symlink(outside, link); e != nil {
		t.Fatal(e)
	}
	g := &Generation{Files: map[string][]byte{"safe": []byte("data")}}
	if e := g.Write(link); e == nil {
		t.Fatal("accepted output symlink")
	}
	if e := g.Write(filepath.Join(link, "nested")); e == nil {
		t.Fatal("accepted symlink ancestor")
	}
	if _, e := os.Stat(filepath.Join(outside, "safe")); !os.IsNotExist(e) {
		t.Fatal("wrote outside output")
	}
	g.Files = map[string][]byte{"../escape": []byte("data")}
	if e := g.Write(filepath.Join(parent, "build")); e == nil {
		t.Fatal("accepted output traversal")
	}
}

func testGeneration(t *testing.T, value string) *Generation {
	t.Helper()
	g := &Generation{Manifest: Manifest{Version: 1, Olym: "test", Inputs: map[string]string{}, Tools: map[string]Tool{}}, Files: map[string][]byte{"value": []byte(value)}}
	// Give each test generation a distinct identity, just as rendered tools do.
	g.Manifest.Inputs["test"] = Hash([]byte(value))
	if err := g.Finish(); err != nil {
		t.Fatal(err)
	}
	return g
}
func TestReplacementIsCompleteAndFailedStagingPreservesOutput(t *testing.T) {
	parent := t.TempDir()
	destination := filepath.Join(parent, "build")
	first := testGeneration(t, "old")
	if err := first.Write(destination); err != nil {
		t.Fatal(err)
	}
	opened, err := os.Open(filepath.Join(destination, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer opened.Close()
	// A conflicting pair of file paths forces a staging failure before publish.
	broken := testGeneration(t, "broken")
	broken.Files["collision"] = []byte("file")
	broken.Files["collision/child"] = []byte("file")
	if err = broken.Write(destination); err == nil {
		t.Fatal("staging failure accepted")
	}
	b, err := os.ReadFile(filepath.Join(destination, "value"))
	if err != nil || string(b) != "old" {
		t.Fatal("failed staging changed prior build")
	}
	replacement := testGeneration(t, "new")
	if err = replacement.Write(destination); err != nil {
		t.Fatal(err)
	}
	b, err = os.ReadFile(filepath.Join(destination, "value"))
	if err != nil || string(b) != "new" {
		t.Fatal("replacement incomplete")
	}
	oldManifest, err := io.ReadAll(opened)
	if err != nil || !bytes.Equal(oldManifest, first.Files["manifest.json"]) {
		t.Fatal("publication changed previously opened manifest")
	}
	b, err = os.ReadFile(filepath.Join(destination, "manifest.json"))
	if err != nil || !bytes.Equal(b, replacement.Files["manifest.json"]) {
		t.Fatal("new manifest not published")
	}
	entries, err := os.ReadDir(parent)
	if err != nil || len(entries) != 1 {
		t.Fatalf("staging or old output left behind: %v %v", entries, err)
	}
}
func TestUnownedBuildDirectoriesArePreserved(t *testing.T) {
	for _, marker := range []string{"", `{"version":1}`, `{"version":1,"olym":"test","id":"fake","inputs":{},"tools":{}}`} {
		t.Run(marker, func(t *testing.T) {
			destination := t.TempDir()
			unrelated := filepath.Join(destination, "keep")
			if err := os.WriteFile(unrelated, []byte("untouched"), 0600); err != nil {
				t.Fatal(err)
			}
			if marker != "" {
				if err := os.WriteFile(filepath.Join(destination, "manifest.json"), []byte(marker), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if err := testGeneration(t, "new").Write(destination); err == nil {
				t.Fatal("replaced directory without valid Olympus manifest")
			}
			b, err := os.ReadFile(unrelated)
			if err != nil || string(b) != "untouched" {
				t.Fatal("unrelated file changed")
			}
		})
	}
}
func TestReplacementDoesNotFollowOldOutputSymlinks(t *testing.T) {
	parent := t.TempDir()
	destination := filepath.Join(parent, "build")
	outside := filepath.Join(t.TempDir(), "keep")
	if err := os.WriteFile(outside, []byte("untouched"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := testGeneration(t, "old").Write(destination); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Dir(outside), filepath.Join(destination, "link")); err != nil {
		t.Fatal(err)
	}
	if err := testGeneration(t, "new").Write(destination); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(outside)
	if err != nil || string(b) != "untouched" {
		t.Fatal("cleanup followed old output symlink")
	}
}

func TestEmptyAndSymlinkManifestDirectoriesAreUnowned(t *testing.T) {
	parent := t.TempDir()
	destination := filepath.Join(parent, "build")
	if err := os.Mkdir(destination, 0700); err != nil {
		t.Fatal(err)
	}
	replacement := testGeneration(t, "new")
	if err := replacement.Write(destination); err == nil {
		t.Fatal("replaced empty unowned directory")
	}
	outside := filepath.Join(parent, "manifest.json")
	if err := os.WriteFile(outside, replacement.Files["manifest.json"], 0600); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(destination, "manifest.json")
	if err := os.Symlink(outside, marker); err != nil {
		t.Fatal(err)
	}
	if err := replacement.Write(destination); err == nil {
		t.Fatal("accepted symlinked ownership manifest")
	}
	if info, err := os.Lstat(marker); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("unowned directory changed")
	}
}
