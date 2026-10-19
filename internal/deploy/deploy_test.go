package deploy

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Haze-dev1/Olympus/internal/activate"
	"github.com/Haze-dev1/Olympus/internal/generation"
)

type fakeConn struct {
	run    func([]string, io.Reader) ([]byte, []byte, error)
	calls  [][]string
	closed bool
}

func (f *fakeConn) Run(_ context.Context, a []string, r io.Reader) ([]byte, []byte, error) {
	f.calls = append(f.calls, append([]string{}, a...))
	return f.run(a, r)
}
func (f *fakeConn) Close() error                { f.closed = true; return nil }
func okOutput(s string) ([]byte, []byte, error) { return []byte(s), nil, nil }
func testGeneration(t *testing.T) *generation.Generation {
	t.Helper()
	g := &generation.Generation{Manifest: generation.Manifest{Version: 1, Olym: "test", Inputs: map[string]string{}, Tools: map[string]generation.Tool{}}, Files: map[string][]byte{"compose.yaml": []byte("services: {}\n")}}
	if err := g.Finish(); err != nil {
		t.Fatal(err)
	}
	return g
}
func TestPrepareServer(t *testing.T) {
	for _, tc := range []struct {
		name, os, uid string
		sudo, docker  bool
		wantErr       string
	}{
		{"unsupported", "ID=fedora\nVERSION_ID=40", "0", true, true, "unsupported OS"},
		{"no sudo", "ID=ubuntu\nVERSION_ID=24.04", "1000", false, true, "sudo -n true"},
		{"root installed", "ID=debian\nVERSION_ID=12", "0", true, true, ""},
		{"sudo install", "ID=ubuntu\nVERSION_ID=24.04", "1000", true, false, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &fakeConn{}
			f.run = func(a []string, _ io.Reader) ([]byte, []byte, error) {
				if a[0] == "sudo" {
					if !tc.sudo {
						return nil, nil, errors.New("denied")
					}
					a = a[2:]
				}
				switch a[0] {
				case "cat":
					return okOutput(tc.os)
				case "id":
					return okOutput(tc.uid)
				case "docker":
					if !tc.docker {
						return nil, nil, errors.New("missing")
					}
				}
				return okOutput("")
			}
			c := Client{Conn: f}
			err := c.PrepareServer(context.Background())
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("got %v", err)
				}
				for _, a := range f.calls {
					if strings.Contains(strings.Join(a, " "), "install") {
						t.Fatal("mutated before refusal")
					}
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			installed := false
			for _, a := range f.calls {
				if tc.uid != "0" && a[0] != "cat" && a[0] != "id" && a[0] != "sudo" {
					t.Fatalf("missing sudo: %v", a)
				}
				if strings.Contains(strings.Join(a, " "), "docker-compose-plugin") {
					installed = true
				}
			}
			if installed == tc.docker {
				t.Fatal("incorrect Docker installation decision")
			}
		})
	}
}

// filesystemConn runs the transfer argv against a temporary fake server root.
func filesystemConn(t *testing.T, corrupt bool) (*fakeConn, string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "store/.incoming"), 0700); err != nil {
		t.Fatal(err)
	}
	f := &fakeConn{}
	f.run = func(a []string, in io.Reader) ([]byte, []byte, error) {
		mapped := append([]string{}, a...)
		for i, v := range mapped {
			if strings.HasPrefix(v, root) {
				mapped[i] = dir + strings.TrimPrefix(v, root)
			}
		}
		if corrupt && a[0] == "sha256sum" {
			return okOutput(strings.Repeat("0", 64) + "  file\n")
		}
		cmd := exec.Command(mapped[0], mapped[1:]...)
		cmd.Stdin = in
		b, e := cmd.Output()
		return b, nil, e
	}
	return f, dir
}
func TestUploadGeneration(t *testing.T) {
	for _, corrupt := range []bool{false, true} {
		t.Run(fmt.Sprint(corrupt), func(t *testing.T) {
			f, dir := filesystemConn(t, corrupt)
			c := Client{Conn: f, Version: "test"}
			g := testGeneration(t)
			err := c.UploadGeneration(context.Background(), g)
			if corrupt {
				if err == nil {
					t.Fatal("accepted mismatch")
				}
				entries, _ := os.ReadDir(filepath.Join(dir, "store"))
				if len(entries) != 1 {
					t.Fatal("half-written store entry")
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				if err = c.UploadGeneration(context.Background(), g); err != nil {
					t.Fatal(err)
				}
				count := 0
				for _, a := range f.calls {
					if a[0] == "tar" {
						count++
					}
				}
				if count != 1 {
					t.Fatal("verified entry reuploaded")
				}
			}
			entries, _ := os.ReadDir(filepath.Join(dir, "store/.incoming"))
			if len(entries) != 0 {
				t.Fatal("staging not cleaned")
			}
		})
	}
}
func TestUploadGenerationRejectsChangedManifestFile(t *testing.T) {
	g := testGeneration(t)
	g.Files["manifest.json"] = []byte(`{}`)
	c := Client{Version: "test"}
	if c.UploadGeneration(context.Background(), g) == nil {
		t.Fatal("accepted changed manifest")
	}
}

func TestSendSecretsNeverUsesArgumentsOrOutput(t *testing.T) {
	sentinel := "OLYM_SENTINEL_never_print_me"
	var payload []byte
	f := &fakeConn{run: func(a []string, in io.Reader) ([]byte, []byte, error) {
		if strings.Contains(strings.Join(a, " "), sentinel) {
			t.Fatal("secret in argv")
		}
		payload, _ = io.ReadAll(in)
		return []byte(sentinel), []byte(sentinel), errors.New(sentinel)
	}}
	c := Client{Conn: f}
	m := generation.Manifest{Tools: map[string]generation.Tool{"tool": {Secrets: []string{"credential"}}}}
	err := c.SendSecrets(context.Background(), m, map[string]ToolSecrets{"tool": {Secrets: map[string]string{"credential": sentinel}}})
	if err == nil || strings.Contains(err.Error(), sentinel) {
		t.Fatalf("unsafe error: %v", err)
	}
	if !bytes.Contains(payload, []byte(sentinel)) {
		t.Fatal("missing stdin secret")
	}
	if !strings.Contains(strings.Join(f.calls[0], " "), "0600") {
		t.Fatal("missing mode")
	}
	var decoded map[string]any
	if json.Unmarshal(payload, &decoded) != nil || decoded["version"] != float64(1) {
		t.Fatal("invalid payload")
	}
}
func TestActivatorMismatchNeverRuns(t *testing.T) {
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	f := &fakeConn{run: func(a []string, _ io.Reader) ([]byte, []byte, error) {
		if a[0] == "uname" {
			return okOutput("x86_64")
		}
		if a[0] == "sha256sum" {
			return okOutput(strings.Repeat("0", 64))
		}
		return okOutput("")
	}}
	c := Client{Conn: f, Version: "test"}
	if c.UploadActivator(context.Background(), binary) == nil {
		t.Fatal("accepted mismatch")
	}
	for _, a := range f.calls {
		if strings.HasPrefix(a[0], root) || a[0] == "mv" || a[0] == "ln" {
			t.Fatalf("ran/published unverified binary: %v", a)
		}
	}
}
func TestActivateFreshConnectionAndExitCodes(t *testing.T) {
	for _, code := range []int{0, 1, 3, 4} {
		t.Run(fmt.Sprint(code), func(t *testing.T) {
			g := testGeneration(t)
			poll := 0
			freshCalls := 0
			fresh := &fakeConn{run: func(a []string, _ io.Reader) ([]byte, []byte, error) {
				freshCalls++
				if a[1] != "confirm" || a[2] != g.Manifest.ID || a[3] != "nonce" {
					t.Fatal(a)
				}
				return okOutput("")
			}}
			f := &fakeConn{run: func(a []string, _ io.Reader) ([]byte, []byte, error) {
				switch a[0] {
				case "sha256sum":
					return okOutput("digest  binary")
				case "systemctl":
					poll++
					if poll == 1 {
						return okOutput("ActiveState=active\nExecMainStatus=0\nExecMainCode=0")
					}
					return okOutput(fmt.Sprintf("ActiveState=inactive\nExecMainStatus=%d\nExecMainCode=1", code))
				case "cat":
					if poll == 0 {
						return nil, nil, errors.New("no state")
					}
					b, _ := json.Marshal(activate.State{Version: 1, ID: g.Manifest.ID, Status: activate.StatusAwaiting, Nonce: "nonce"})
					return b, nil, nil
				}
				return okOutput("test")
			}}
			c := Client{Conn: f, Version: "test", binary: root + "/bin/olym-test", checksum: "digest", PollInterval: time.Nanosecond, Reconnect: func() (Connection, error) { return fresh, nil }}
			got, err := c.Activate(context.Background(), g.Manifest.ID)
			if got != code || (err == nil) != (code == 0) {
				t.Fatalf("%d %v", got, err)
			}
			if freshCalls != 1 || !fresh.closed {
				t.Fatal("did not confirm on fresh connection")
			}
		})
	}
}
func TestActivateRechecksBinary(t *testing.T) {
	f := &fakeConn{run: func([]string, io.Reader) ([]byte, []byte, error) { return okOutput("wrong") }}
	c := Client{Conn: f, Version: "test", binary: root + "/bin/olym-test", checksum: fmt.Sprintf("%x", sha256.Sum256(nil)), Reconnect: func() (Connection, error) { t.Fatal("unexpected reconnect"); return nil, nil }}
	code, err := c.Activate(context.Background(), testGeneration(t).Manifest.ID)
	if code != 1 || err == nil || len(f.calls) != 1 {
		t.Fatal("ran mismatched activator")
	}
}

func TestUploadActivatorVersionAndPublication(t *testing.T) {
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, version := range []string{"test", "wrong"} {
		t.Run(version, func(t *testing.T) {
			var uploaded []byte
			f := &fakeConn{run: func(a []string, in io.Reader) ([]byte, []byte, error) {
				switch a[0] {
				case "uname":
					return okOutput("x86_64")
				case "install":
					uploaded, _ = io.ReadAll(in)
				case "sha256sum":
					return okOutput(fmt.Sprintf("%x  binary", sha256.Sum256(uploaded)))
				}
				if strings.HasPrefix(a[0], root) {
					return okOutput(version)
				}
				return okOutput("")
			}}
			c := Client{Conn: f, Version: "test"}
			err = c.UploadActivator(context.Background(), binary)
			if (err == nil) != (version == "test") {
				t.Fatal(err)
			}
			published := false
			for _, a := range f.calls {
				if a[0] == "ln" {
					published = true
				}
			}
			if published != (version == "test") {
				t.Fatal("incorrect publication")
			}
		})
	}
}

func TestSendSecretsRejectsUndeclared(t *testing.T) {
	c := Client{}
	m := generation.Manifest{Tools: map[string]generation.Tool{"tool": {}}}
	for _, tools := range []map[string]ToolSecrets{{"unknown": {}}, {"tool": {Secrets: map[string]string{"unknown": "value"}}}} {
		if c.SendSecrets(context.Background(), m, tools) == nil {
			t.Fatal("accepted undeclared secret")
		}
	}
}
