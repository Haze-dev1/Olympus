package generation

import (
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
