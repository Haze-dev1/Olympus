package activate

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildDeduplicatesImagesAndReusesExisting(t *testing.T) {
	dir := t.TempDir()
	calls := filepath.Join(dir, "calls")
	bin := filepath.Join(dir, "docker")
	file := filepath.Join(dir, "compose.yaml")
	script := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> '" + calls + "'\ncase \"$*\" in\n'image inspect existing') exit 0;;\n'image inspect missing') exit 1;;\nesac\n"
	if e := os.WriteFile(bin, []byte(script), 0700); e != nil {
		t.Fatal(e)
	}
	data := "services:\n  a:\n    image: missing\n    build: {context: /store/a}\n  a-init:\n    image: missing\n    build: {context: /store/a}\n  b:\n    image: existing\n    build: {context: /store/b}\n  c:\n    image: registry/image@sha256:abc\n"
	if e := os.WriteFile(file, []byte(data), 0600); e != nil {
		t.Fatal(e)
	}
	if e := (Compose{Bin: bin}).Build(context.Background(), file); e != nil {
		t.Fatal(e)
	}
	got, e := os.ReadFile(calls)
	if e != nil {
		t.Fatal(e)
	}
	want := "image inspect missing\ncompose -p olympus -f " + file + " build --build-arg SOURCE_DATE_EPOCH=0 a\nimage inspect existing\n"
	if string(got) != want {
		t.Fatalf("commands: %s", got)
	}
	if strings.Contains(string(got), "build a-init") {
		t.Fatal("duplicate build")
	}
}
