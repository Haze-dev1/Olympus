// Package deploy implements the client half of the activation protocol.
package deploy

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"debug/elf"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/Haze-dev1/Olympus/internal/activate"
	"github.com/Haze-dev1/Olympus/internal/encode"
	"github.com/Haze-dev1/Olympus/internal/generation"
	"github.com/Haze-dev1/Olympus/internal/remote"
	"github.com/Haze-dev1/Olympus/internal/schema"
	"github.com/Haze-dev1/Olympus/internal/stack"
)

const root = activate.DefaultRoot

var idRE = regexp.MustCompile(`^sha256-[0-9a-f]{64}$`)
var versionRE = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,127}$`)

// Connection is implemented by remote.Client. All commands use quoted argv.
type Connection interface {
	Run(context.Context, []string, io.Reader) ([]byte, []byte, error)
	Close() error
}

// Client owns one deployment connection. Reconnect must open a fresh SSH
// connection using the same verified host-key policy.
type Client struct {
	Conn         Connection
	Reconnect    func() (Connection, error)
	Version      string
	PollInterval time.Duration
	sudo         bool
	binary       string
	checksum     string
}

// Dial uses remote's host-key verification for both initial and confirm SSH.
func Dial(cfg remote.Config, version string) (*Client, error) {
	conn, err := remote.Dial(cfg)
	if err != nil {
		return nil, err
	}
	return &Client{Conn: conn, Version: version, Reconnect: func() (Connection, error) { return remote.Dial(cfg) }}, nil
}

func (c *Client) run(ctx context.Context, args []string, in io.Reader) ([]byte, error) {
	if c.sudo {
		args = remote.Sudo(args)
	}
	out, _, err := c.Conn.Run(ctx, args, in)
	// Never expose remote output or input through an error, especially for secrets.
	if err != nil {
		return nil, fmt.Errorf("deploy: %s failed", args[0])
	}
	return out, nil
}

// PrepareServer refuses unsupported systems before making changes.
func (c *Client) PrepareServer(ctx context.Context) error {
	b, err := c.run(ctx, []string{"cat", "/etc/os-release"}, nil)
	if err != nil {
		return err
	}
	osInfo, err := remote.ParseOSRelease(b)
	if err != nil {
		return err
	}
	b, err = c.run(ctx, []string{"id", "-u"}, nil)
	if err != nil {
		return err
	}
	c.sudo = strings.TrimSpace(string(b)) != "0"
	if c.sudo {
		if _, err = c.run(ctx, []string{"true"}, nil); err != nil {
			return errors.New("deploy: passwordless sudo required: sudo -n true")
		}
	}
	_, dockerErr := c.run(ctx, []string{"docker", "info"}, nil)
	_, composeErr := c.run(ctx, []string{"docker", "compose", "version"}, nil)
	if dockerErr != nil || composeErr != nil {
		for _, step := range remote.DockerInstallSteps(osInfo) {
			if _, err = c.run(ctx, step.Args, bytes.NewReader(step.Stdin)); err != nil {
				return err
			}
		}
	}
	_, err = c.run(ctx, []string{"install", "-d", "-m", "0700", "-o", "root", "-g", "root", root, root + "/store", root + "/store/.incoming", root + "/bin"}, nil)
	return err
}

func randomName() (string, error) {
	var b [16]byte
	_, err := rand.Read(b[:])
	return hex.EncodeToString(b[:]), err
}

// UploadGeneration stages a regular-file-only tar and publishes only after
// checking every file remotely. A failed transfer never becomes a store entry.
func (c *Client) UploadGeneration(ctx context.Context, g *generation.Generation) (result error) {
	if g == nil || !idRE.MatchString(g.Manifest.ID) || g.Manifest.Version != 1 || g.Manifest.Olym != c.Version {
		return errors.New("deploy: invalid generation or version")
	}
	m := g.Manifest
	m.ID = ""
	canonical, err := encode.Canonical(m)
	if err != nil || generation.Hash(canonical) != g.Manifest.ID {
		return errors.New("deploy: manifest id mismatch")
	}
	var uploaded generation.Manifest
	if json.Unmarshal(g.Files["manifest.json"], &uploaded) != nil {
		return errors.New("deploy: invalid manifest file")
	}
	a, _ := json.Marshal(uploaded)
	b, _ := json.Marshal(g.Manifest)
	if !bytes.Equal(a, b) || len(g.Files["compose.yaml"]) == 0 {
		return errors.New("deploy: generation files disagree with manifest")
	}
	for name, tool := range g.Manifest.Tools {
		if !stack.ValidName(name) {
			return errors.New("deploy: invalid tool name")
		}
		for p, hash := range tool.Files {
			rel := "tools/" + name + "/files/" + strings.TrimPrefix(p, "/")
			data, ok := g.Files[rel]
			if !ok || generation.Hash(data) != hash {
				return errors.New("deploy: manifest file hash mismatch")
			}
		}
	}
	var archive bytes.Buffer
	tw := tar.NewWriter(&archive)
	for _, p := range schema.Keys(g.Files) {
		if p == "." || path.IsAbs(p) || path.Clean(p) != p || strings.HasPrefix(p, "../") || strings.ContainsAny(p, "\n\r\\") {
			return errors.New("deploy: unsafe generation path")
		}
		data := g.Files[p]
		if err = tw.WriteHeader(&tar.Header{Name: p, Mode: 0600, Size: int64(len(data)), Typeflag: tar.TypeReg}); err != nil {
			return err
		}
		if _, err = tw.Write(data); err != nil {
			return err
		}
	}
	if err = tw.Close(); err != nil {
		return err
	}
	destination := root + "/store/gen-" + g.Manifest.ID
	if _, err = c.run(ctx, []string{"test", "-e", destination}, nil); err == nil {
		return c.verifyFiles(ctx, destination, g.Files)
	}
	suffix, err := randomName()
	if err != nil {
		return err
	}
	stage := root + "/store/.incoming/" + suffix
	if _, err = c.run(ctx, []string{"mkdir", "-m", "0700", stage}, nil); err != nil {
		return err
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, e := c.run(cleanup, []string{"rm", "-rf", "--", stage}, nil)
		if result == nil {
			result = e
		}
	}()
	if _, err = c.run(ctx, []string{"tar", "-x", "-f", "-", "--no-same-owner", "-C", stage}, &archive); err != nil {
		return err
	}
	if err = c.verifyFiles(ctx, stage, g.Files); err != nil {
		return err
	}
	_, err = c.run(ctx, []string{"mv", "-T", "-n", "--", stage, destination}, nil)
	if err != nil {
		return err
	}
	return c.verifyFiles(ctx, destination, g.Files)
}

func (c *Client) verifyFiles(ctx context.Context, dir string, files map[string][]byte) error {
	for _, p := range schema.Keys(files) {
		out, err := c.run(ctx, []string{"sha256sum", "--", dir + "/" + p}, nil)
		want := fmt.Sprintf("%x", sha256.Sum256(files[p]))
		if err != nil || len(strings.Fields(string(out))) == 0 || strings.Fields(string(out))[0] != want {
			return errors.New("deploy: uploaded file checksum mismatch")
		}
	}
	return nil
}

// UploadActivator accepts a local Linux binary, including an explicit development
// build. Release-checksum embedding and downloading are deliberately deferred.
func (c *Client) UploadActivator(ctx context.Context, localPath string) (result error) {
	if !versionRE.MatchString(c.Version) {
		return errors.New("deploy: invalid activator version")
	}
	if localPath == "" {
		var err error
		localPath, err = os.Executable()
		if err != nil {
			return err
		}
	}
	data, err := os.ReadFile(localPath)
	if err != nil {
		return errors.New("deploy: cannot read activator")
	}
	file, err := elf.NewFile(bytes.NewReader(data))
	if err != nil {
		return errors.New("deploy: activator must be a Linux ELF binary")
	}
	defer file.Close()
	machine, err := c.run(ctx, []string{"uname", "-m"}, nil)
	if err != nil {
		return err
	}
	arch := strings.TrimSpace(string(machine))
	if !(arch == "x86_64" && file.Machine == elf.EM_X86_64 || arch == "aarch64" && file.Machine == elf.EM_AARCH64) {
		return errors.New("deploy: activator architecture mismatch")
	}
	suffix, err := randomName()
	if err != nil {
		return err
	}
	tmp := root + "/bin/.incoming-" + suffix
	dest := root + "/bin/olym-" + c.Version
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, e := c.run(cleanup, []string{"rm", "-f", "--", tmp}, nil)
		if result == nil {
			result = e
		}
	}()
	if _, err = c.run(ctx, []string{"install", "-m", "0755", "/dev/stdin", tmp}, bytes.NewReader(data)); err != nil {
		return err
	}
	if err = c.verifyFiles(ctx, root+"/bin", map[string][]byte{path.Base(tmp): data}); err != nil {
		return err
	}
	// The version command runs only after the uploaded bytes were verified.
	out, err := c.run(ctx, []string{tmp, "version"}, nil)
	if err != nil || strings.TrimSpace(string(out)) != c.Version {
		return errors.New("deploy: activator version mismatch")
	}
	if _, err = c.run(ctx, []string{"mv", "-T", "--", tmp, dest}, nil); err != nil {
		return err
	}
	if _, err = c.run(ctx, []string{"ln", "-sfn", "--", path.Base(dest), root + "/bin/olym"}, nil); err != nil {
		return err
	}
	c.binary = dest
	c.checksum = fmt.Sprintf("%x", sha256.Sum256(data))
	return nil
}

// ToolSecrets matches the activator's stdin payload format.
type ToolSecrets struct {
	Secrets     map[string]string `json:"secrets,omitempty"`
	Environment map[string]string `json:"environment,omitempty"`
}

// SendSecrets writes only the declared tool secrets. Remote output is discarded.
func (c *Client) SendSecrets(ctx context.Context, m generation.Manifest, tools map[string]ToolSecrets) error {
	for tool, values := range tools {
		declared, ok := m.Tools[tool]
		if !ok {
			return errors.New("deploy: undeclared secret tool")
		}
		for name := range values.Secrets {
			found := false
			for _, s := range declared.Secrets {
				if s == name {
					found = true
				}
			}
			if !found {
				return errors.New("deploy: undeclared secret")
			}
		}
	}
	data, err := json.Marshal(struct {
		Version int                    `json:"version"`
		Tools   map[string]ToolSecrets `json:"tools"`
	}{1, tools})
	if err != nil {
		return errors.New("deploy: cannot encode secrets")
	}
	_, err = c.run(ctx, []string{"install", "-m", "0600", "/dev/stdin", (&activate.Activator{}).PayloadPath()}, bytes.NewReader(data))
	return err
}

// Activate starts a detached transient service and confirms over a fresh SSH
// connection. The returned code follows TRD 10.1, including lock contention.
func (c *Client) Activate(ctx context.Context, id string) (int, error) {
	if !idRE.MatchString(id) || c.binary == "" || c.Reconnect == nil {
		return 1, errors.New("deploy: activator upload and reconnect are required")
	}
	out, err := c.run(ctx, []string{"sha256sum", "--", c.binary}, nil)
	if err != nil || len(strings.Fields(string(out))) == 0 || strings.Fields(string(out))[0] != c.checksum {
		return 1, errors.New("deploy: activator checksum mismatch")
	}
	out, err = c.run(ctx, []string{c.binary, "version"}, nil)
	if err != nil || strings.TrimSpace(string(out)) != c.Version {
		return 1, errors.New("deploy: activator version mismatch")
	}
	// An earlier activation of this same id may still have an awaiting state.
	// Never confirm a nonce that existed before this service was launched.
	var previous activate.State
	if data, e := c.run(ctx, []string{"cat", root + "/state.json"}, nil); e == nil {
		_ = json.Unmarshal(data, &previous)
	}
	suffix, err := randomName()
	if err != nil {
		return 1, err
	}
	unit := "olympus-activate-" + suffix
	if _, err = c.run(ctx, []string{"systemd-run", "--quiet", "--unit", unit, "--property=Type=exec", "--property=RemainAfterExit=yes", "--property=StandardOutput=null", "--property=StandardError=null", c.binary, "activate", id}, nil); err != nil {
		return 1, err
	}
	confirmed := false
	interval := c.PollInterval
	if interval <= 0 {
		interval = 200 * time.Millisecond
	}
	for {
		// systemd is authoritative for completion, including failures before state.json.
		out, err = c.run(ctx, []string{"systemctl", "show", unit, "--property=ActiveState", "--property=SubState", "--property=ExecMainStatus", "--property=ExecMainCode"}, nil)
		if err != nil {
			return 1, err
		}
		props := map[string]string{}
		for _, line := range strings.Split(string(out), "\n") {
			k, v, ok := strings.Cut(line, "=")
			if ok {
				props[k] = v
			}
		}
		if props["SubState"] == "exited" || props["ActiveState"] == "failed" || props["ActiveState"] == "inactive" && props["ExecMainCode"] != "0" {
			// Keep the result until it has been read, then release the transient unit.
			_, _ = c.run(ctx, []string{"systemctl", "stop", unit}, nil)
			_, _ = c.run(ctx, []string{"systemctl", "reset-failed", unit}, nil)
			switch props["ExecMainStatus"] {
			case "0":
				if props["ExecMainCode"] == "1" {
					return 0, nil
				}
			case "3":
				return 3, errors.New("deploy: activation failed; server rolled back")
			case "4":
				return 4, errors.New("deploy: server lock is held")
			case "5":
				return 5, errors.New("deploy: activation failed and the server could not roll back; it may be down")
			}
			return 1, errors.New("deploy: activation failed; inspect server logs")
		}
		if !confirmed {
			stateBytes, e := c.run(ctx, []string{"cat", root + "/state.json"}, nil)
			var state activate.State
			if e == nil && json.Unmarshal(stateBytes, &state) == nil && state.Version == 1 && state.ID == id && state.Status == activate.StatusAwaiting && state.Nonce != "" && state.Nonce != previous.Nonce {
				confirmed = true // Attempt once; failure leaves the server to roll back.
				fresh, e := c.Reconnect()
				if e != nil {
					continue
				}
				args := []string{c.binary, "confirm", id, state.Nonce}
				if c.sudo {
					args = remote.Sudo(args)
				}
				_, _, e = fresh.Run(ctx, args, nil)
				fresh.Close()
				if e != nil {
					continue
				}
				confirmed = true
			}
		}
		select {
		case <-ctx.Done():
			return 1, ctx.Err()
		case <-time.After(interval):
		}
	}
}
