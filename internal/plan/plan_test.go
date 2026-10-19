package plan

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/Haze-dev1/Olympus/internal/generation"
)

func TestCompare(t *testing.T) {
	base := generation.Tool{Hash: "old", Image: "image:1", Files: map[string]string{"/a": "a", "/removed": "r"}}
	current := generation.Manifest{Tools: map[string]generation.Tool{"demo": base}}
	actual := map[string]map[string]string{"demo": base.Files}
	for _, tc := range []struct {
		name      string
		want      generation.Manifest
		actual    map[string]map[string]string
		kind      string
		files     int
		image     bool
		drift     int
		unchanged int
	}{
		{"new", generation.Manifest{Tools: map[string]generation.Tool{"demo": base, "new": base}}, actual, "+", 0, false, 0, 1},
		{"removed", generation.Manifest{}, actual, "-", 0, false, 0, 0},
		{"files", generation.Manifest{Tools: map[string]generation.Tool{"demo": {Hash: "new", Image: "image:1", Files: map[string]string{"/a": "changed", "/added": "new"}}}}, actual, "~", 3, false, 0, 0},
		{"image", generation.Manifest{Tools: map[string]generation.Tool{"demo": {Hash: "new", Image: "image:2", Files: base.Files}}}, actual, "~", 0, true, 0, 0},
		{"unchanged", current, actual, "", 0, false, 0, 1},
		{"drift", current, map[string]map[string]string{"demo": {"/a": "edited", "/removed": "r"}}, "~", 0, false, 1, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := Compare(tc.want, current, tc.actual)
			if d.Unchanged != tc.unchanged {
				t.Fatal(d)
			}
			if tc.kind == "" {
				if len(d.Changes) != 0 {
					t.Fatal(d)
				}
				return
			}
			if len(d.Changes) != 1 {
				t.Fatal(d)
			}
			c := d.Changes[0]
			if c.Kind != tc.kind || len(c.Files) != tc.files || c.Image != tc.image || len(c.Drift) != tc.drift {
				t.Fatal(c)
			}
		})
	}
}

type fakeReader struct {
	t        *testing.T
	calls    int
	manifest []byte
	id       string
	hash     string
	fail     bool
	missing  bool
}

func (f *fakeReader) Run(_ context.Context, args []string, _ io.Reader) ([]byte, []byte, error) {
	f.calls++
	if len(args) < 3 || args[0] != "sudo" || args[1] != "-n" {
		f.t.Fatalf("missing sudo: %v", args)
	}
	args = args[2:]
	if f.fail {
		return nil, nil, fmt.Errorf("sensitive remote diagnostic")
	}
	switch args[0] {
	case "id":
		return []byte("0\n"), nil, nil
	case "readlink":
		if args[1] == "--" {
			return []byte("generations/1\n"), nil, nil
		}
		return []byte("/var/lib/olympus/store/gen-" + f.id + "\n"), nil, nil
	case "head":
		return f.manifest, nil, nil
	case "test":
		return nil, nil, exitCode(1)
	case "sha256sum":
		if f.missing {
			return nil, nil, exitCode(1)
		}
		if !strings.Contains(args[2], "/tools/demo/files/a") {
			f.t.Fatal(args)
		}
		return []byte(strings.TrimPrefix(f.hash, "sha256-") + "  file\n"), nil, nil
	default:
		f.t.Fatalf("unexpected command: %v", args)
	}
	return nil, nil, nil
}
func TestReadCurrent(t *testing.T) {
	hash := generation.Hash([]byte("contents"))
	g := generation.Generation{Manifest: generation.Manifest{Version: 1, Olym: "test", Tools: map[string]generation.Tool{"demo": {Files: map[string]string{"/a": hash}}}}, Files: map[string][]byte{}}
	if err := g.Finish(); err != nil {
		t.Fatal(err)
	}
	f := &fakeReader{t: t, manifest: g.Files["manifest.json"], id: g.Manifest.ID, hash: hash}
	m, actual, err := ReadCurrent(context.Background(), f, false)
	if err != nil || m.ID != g.Manifest.ID || actual["demo"]["/a"] != hash || f.calls != 5 {
		t.Fatalf("%v %v", actual, err)
	}
	f.missing = true
	_, actual, err = ReadCurrent(context.Background(), f, false)
	if err != nil || actual["demo"]["/a"] != "" {
		t.Fatalf("missing file not recorded as drift: %v", err)
	}
	f.missing = false
	f.manifest = []byte("{}")
	if _, _, err := ReadCurrent(context.Background(), f, false); err == nil {
		t.Fatal("accepted invalid manifest")
	}
	f.fail = true
	if _, _, err := ReadCurrent(context.Background(), f, false); err == nil || strings.Contains(err.Error(), "sensitive") {
		t.Fatal(err)
	}
}

type exitCode int

func (e exitCode) Error() string   { return "command failed" }
func (e exitCode) ExitStatus() int { return int(e) }

type absentReader struct {
	code  int
	calls int
}

func (r *absentReader) Run(_ context.Context, args []string, _ io.Reader) ([]byte, []byte, error) {
	r.calls++
	return nil, nil, exitCode(r.code)
}
func TestAbsentCurrentAndPermissionFailure(t *testing.T) {
	r := &absentReader{code: 1}
	m, actual, err := ReadCurrent(context.Background(), r, true)
	if err != nil || m.ID != "" || len(actual) != 0 || r.calls != 3 {
		t.Fatalf("%v %v", m, err)
	}
	r = &absentReader{code: 2}
	if _, _, err = ReadCurrent(context.Background(), r, true); err == nil {
		t.Fatal("treated permission/command failure as empty")
	}
}
