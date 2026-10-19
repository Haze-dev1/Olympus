package remote

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

func TestHostKeyKnownConnects(t *testing.T) {
	hostKey := newSigner(t)
	keyFile, pub := writeClientKey(t)
	addr := startTestServer(t, hostKey, pub)
	known := knownHostsFile(t, knownhosts.Line([]string{addr}, hostKey.PublicKey()))

	confirmCalled := false
	c, err := dialTest(t, Config{
		Host:       addr,
		User:       "tester",
		KeyFile:    keyFile,
		KnownHosts: known,
		Confirm:    func(string, string) bool { confirmCalled = true; return true },
	})
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer c.Close()
	if confirmCalled {
		t.Fatal("Confirm was called for a known host")
	}
}

func TestHostKeyUnknownPrompts(t *testing.T) {
	hostKey := newSigner(t)
	keyFile, pub := writeClientKey(t)
	addr := startTestServer(t, hostKey, pub)

	var gotHost, gotFingerprint string
	c, err := dialTest(t, Config{
		Host:       addr,
		User:       "tester",
		KeyFile:    keyFile,
		KnownHosts: knownHostsFile(t),
		Confirm: func(host, fingerprint string) bool {
			gotHost, gotFingerprint = host, fingerprint
			return true
		},
	})
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer c.Close()
	if gotHost != addr {
		t.Errorf("Confirm host = %q, want %q", gotHost, addr)
	}
	if want := ssh.FingerprintSHA256(hostKey.PublicKey()); gotFingerprint != want {
		t.Errorf("Confirm fingerprint = %q, want %q", gotFingerprint, want)
	}
}

func TestHostKeyUnknownRefused(t *testing.T) {
	hostKey := newSigner(t)
	keyFile, pub := writeClientKey(t)
	addr := startTestServer(t, hostKey, pub)

	_, err := dialTest(t, Config{
		Host:       addr,
		User:       "tester",
		KeyFile:    keyFile,
		KnownHosts: knownHostsFile(t),
	})
	if err == nil {
		t.Fatal("Dial succeeded for an unknown host with no confirm")
	}
	if !strings.Contains(err.Error(), "unknown host key") {
		t.Fatalf("error = %v, want it to mention an unknown host key", err)
	}
}

func TestHostKeyChangedRefused(t *testing.T) {
	hostKey := newSigner(t)
	otherKey := newSigner(t)
	keyFile, pub := writeClientKey(t)
	addr := startTestServer(t, hostKey, pub)
	known := knownHostsFile(t, knownhosts.Line([]string{addr}, otherKey.PublicKey()))

	confirmCalled := false
	_, err := dialTest(t, Config{
		Host:       addr,
		User:       "tester",
		KeyFile:    keyFile,
		KnownHosts: known,
		Confirm:    func(string, string) bool { confirmCalled = true; return true },
	})
	if err == nil {
		t.Fatal("Dial succeeded for a changed host key")
	}
	if !strings.Contains(err.Error(), "changed") {
		t.Fatalf("error = %v, want it to mention a changed host key", err)
	}
	if confirmCalled {
		t.Fatal("Confirm was called for a changed host key")
	}
}

func TestRunQuotesArgumentsAndSendsStdin(t *testing.T) {
	hostKey := newSigner(t)
	keyFile, pub := writeClientKey(t)
	addr := startTestServer(t, hostKey, pub)
	known := knownHostsFile(t, knownhosts.Line([]string{addr}, hostKey.PublicKey()))
	c, err := Dial(Config{Host: addr, User: "tester", KeyFile: keyFile, KnownHosts: known, Timeout: 5 * time.Second})
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer c.Close()
	ctx := context.Background()

	hostile := []string{
		"two words",
		`quote'and"double`,
		"$(id)",
		"`id`",
		"line1\nline2",
		"; echo pwned",
		"&& echo pwned",
		"* ? [a-z]",
	}
	for _, in := range hostile {
		out, _, err := c.Run(ctx, []string{"printf", "%s", in}, nil)
		if err != nil {
			t.Fatalf("Run printf %q: %v", in, err)
		}
		if string(out) != in {
			t.Errorf("Run printf %q round-tripped to %q", in, out)
		}
	}

	out, _, err := c.Run(ctx, []string{"printf", "%s|%s", "a b", `c'd$e`}, nil)
	if err != nil {
		t.Fatalf("Run printf multi-arg: %v", err)
	}
	if want := `a b|c'd$e`; string(out) != want {
		t.Errorf("multi-arg round-trip = %q, want %q", out, want)
	}

	out, _, err = c.Run(ctx, []string{"cat"}, strings.NewReader("hunter2 secret"))
	if err != nil {
		t.Fatalf("Run cat: %v", err)
	}
	if string(out) != "hunter2 secret" {
		t.Errorf("stdin round-trip = %q, want the secret", out)
	}

	_, stderr, err := c.Run(ctx, []string{"sh", "-c", "echo boom >&2; exit 7"}, nil)
	if err == nil {
		t.Fatal("Run of a failing command returned no error")
	}
	if !strings.Contains(string(stderr), "boom") {
		t.Errorf("stderr = %q, want it to contain the command's output", stderr)
	}
	if strings.Contains(err.Error(), "boom") {
		t.Errorf("error %q embeds stderr, which may leak a secret", err)
	}
}

// --- test helpers: an in-process SSH server, keys and known_hosts files ---

func dialTest(t *testing.T, cfg Config) (*Client, error) {
	t.Helper()
	cfg.Timeout = 5 * time.Second
	return Dial(cfg)
}

func newSigner(t *testing.T) ssh.Signer {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatalf("signer: %v", err)
	}
	return signer
}

func writeClientKey(t *testing.T) (string, ssh.PublicKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	block, err := ssh.MarshalPrivateKey(priv, "")
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	path := filepath.Join(t.TempDir(), "id_ed25519")
	if err := os.WriteFile(path, pem.EncodeToMemory(block), 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}
	sshPub, err := ssh.NewPublicKey(pub)
	if err != nil {
		t.Fatalf("public key: %v", err)
	}
	return path, sshPub
}

func knownHostsFile(t *testing.T, lines ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "known_hosts")
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o600); err != nil {
		t.Fatalf("write known_hosts: %v", err)
	}
	return path
}

func startTestServer(t *testing.T, hostKey ssh.Signer, clientKey ssh.PublicKey) string {
	t.Helper()
	cfg := &ssh.ServerConfig{
		PublicKeyCallback: func(_ ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			if bytes.Equal(key.Marshal(), clientKey.Marshal()) {
				return nil, nil
			}
			return nil, errors.New("remote_test: unknown client key")
		},
	}
	cfg.AddHostKey(hostKey)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go serveConn(conn, cfg)
		}
	}()
	return ln.Addr().String()
}

func serveConn(conn net.Conn, cfg *ssh.ServerConfig) {
	defer conn.Close()
	sc, chans, reqs, err := ssh.NewServerConn(conn, cfg)
	if err != nil {
		return
	}
	defer sc.Close()
	go ssh.DiscardRequests(reqs)
	for newChan := range chans {
		if newChan.ChannelType() != "session" {
			_ = newChan.Reject(ssh.UnknownChannelType, "only session channels")
			continue
		}
		ch, sreqs, err := newChan.Accept()
		if err != nil {
			continue
		}
		go serveSession(ch, sreqs)
	}
}

// serveSession runs the exec request's command through the local shell. The
// command string is already the caller's quoted argument list.
func serveSession(ch ssh.Channel, reqs <-chan *ssh.Request) {
	defer ch.Close()
	for req := range reqs {
		if req.Type != "exec" {
			_ = req.Reply(false, nil)
			continue
		}
		var payload struct{ Command string }
		if err := ssh.Unmarshal(req.Payload, &payload); err != nil {
			_ = req.Reply(false, nil)
			return
		}
		_ = req.Reply(true, nil)
		cmd := exec.Command("/bin/sh", "-c", payload.Command)
		cmd.Stdin = ch
		cmd.Stdout = ch
		cmd.Stderr = ch.Stderr()
		status := 0
		if err := cmd.Run(); err != nil {
			var exit *exec.ExitError
			if errors.As(err, &exit) {
				status = exit.ExitCode()
			} else {
				status = 1
			}
		}
		_, _ = ch.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{uint32(status)}))
		return
	}
}
