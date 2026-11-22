package activate

import (
	"context"
	"os"
	"path/filepath"
	"slices"
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

func TestComposeLifecycleArguments(t *testing.T) {
	for _, tc := range []struct {
		name string
		run  func(Compose) error
		want []string
	}{
		{
			name: "recreate includes init dependencies",
			run:  func(c Compose) error { return c.Recreate(context.Background(), "generation.yaml", []string{"one"}) },
			want: []string{"up", "-d", "--no-build", "--pull", "never", "--force-recreate", "one"},
		},
		{
			name: "down includes orphans without removing volumes",
			run:  func(c Compose) error { return c.Down(context.Background(), "generation.yaml") },
			want: []string{"down", "--remove-orphans"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var output strings.Builder
			if err := tc.run(Compose{Bin: "/bin/echo", Log: &output}); err != nil {
				t.Fatal(err)
			}
			got := strings.Fields(output.String())
			want := append([]string{"compose", "-p", "olympus", "-f", "generation.yaml"}, tc.want...)
			if !slices.Equal(got, want) {
				t.Errorf("Compose argv = %q, want %q", got, want)
			}
		})
	}
}
