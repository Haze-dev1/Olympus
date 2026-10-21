// Package remote is Olympus's SSH layer. It opens one connection to a server,
// verifies the server's host key, runs commands built from argument lists and
// reports the server's operating system.
//
// There is deliberately no API that takes a raw command string. Every remote
// command is an []string, and each element is shell-quoted before it reaches
// the server's shell, so no value can become a second command or a redirect
// (security rule A2).
package remote

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
)

// ConfirmFunc decides whether to trust a host key that is not in known_hosts.
// host is the dialled address and fingerprint is an SSH SHA256 fingerprint such
// as "SHA256:AbC…". Returning false aborts the connection.
//
// A changed key never reaches ConfirmFunc: it is always an error (A6).
type ConfirmFunc func(host, fingerprint string) bool

// Config describes one SSH connection.
type Config struct {
	// HostKeyAlgorithms optionally restricts negotiation to the pinned key type.
	// Verification against KnownHosts still applies to every negotiated key.
	HostKeyAlgorithms []string
	Host              string        // "host" or "host:port"; the port defaults to 22
	User              string        // SSH user
	KeyFile           string        // private key path; empty means use the SSH agent
	KnownHosts        string        // known_hosts path; empty means ~/.ssh/known_hosts
	Timeout           time.Duration // dial timeout; zero means no timeout
	Confirm           ConfirmFunc   // called only for an unknown host key
}

// Client is an authenticated SSH connection to one server.
type Client struct {
	conn  *ssh.Client
	agent net.Conn // set when the SSH agent was used; closed with the client
}

// Dial connects to the server in cfg. It authenticates with cfg.KeyFile or, if
// that is empty, the SSH agent. The server's host key is verified against
// known_hosts: an unchanged key connects, an unknown key is offered to
// cfg.Confirm, and a changed key is always refused (A6). Verification is never
// disabled.
func Dial(cfg Config) (*Client, error) {
	if cfg.Host == "" || cfg.User == "" {
		return nil, errors.New("remote: host and user are required")
	}
	auth, agentConn, err := authMethods(cfg)
	if err != nil {
		return nil, err
	}
	callback, err := hostKeyCallback(cfg.KnownHosts, cfg.Confirm)
	if err != nil {
		closeIf(agentConn)
		return nil, err
	}
	addr := cfg.Host
	if _, _, err := net.SplitHostPort(addr); err != nil {
		addr = net.JoinHostPort(addr, "22")
	}
	if len(cfg.HostKeyAlgorithms) == 0 {
		cfg.HostKeyAlgorithms = knownAlgorithms(addr, cfg.KnownHosts)
	}
	conn, err := ssh.Dial("tcp", addr, &ssh.ClientConfig{
		HostKeyAlgorithms: cfg.HostKeyAlgorithms,
		User:              cfg.User,
		Auth:              auth,
		HostKeyCallback:   callback,
		Timeout:           cfg.Timeout,
	})
	if err != nil {
		closeIf(agentConn)
		return nil, fmt.Errorf("remote: connect to %s@%s: %w", cfg.User, addr, err)
	}
	return &Client{conn: conn, agent: agentConn}, nil
}

// Close ends the connection.
func (c *Client) Close() error {
	err := c.conn.Close()
	if c.agent != nil {
		if aerr := c.agent.Close(); aerr != nil && err == nil {
			err = aerr
		}
	}
	return err
}

// Run executes args on the server. args[0] is the program and the rest are its
// arguments; the whole list is shell-quoted before it is sent (A2). stdin, when
// non-nil, is sent to the command's standard input: that is how secrets travel,
// never as arguments (A1). stdout and stderr are returned even on error, and the
// error never embeds stderr, which may not be safe to print.
func (c *Client) Run(ctx context.Context, args []string, stdin io.Reader) (stdout, stderr []byte, err error) {
	if len(args) == 0 {
		return nil, nil, errors.New("remote: command has no arguments")
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	session, err := c.conn.NewSession()
	if err != nil {
		return nil, nil, fmt.Errorf("remote: open session: %w", err)
	}
	defer session.Close()

	var out, errBuf bytes.Buffer
	session.Stdout = &out
	session.Stderr = &errBuf
	session.Stdin = stdin

	if err := session.Start(joinArgs(args)); err != nil {
		return nil, nil, fmt.Errorf("remote: start %s: %w", args[0], err)
	}
	done := make(chan error, 1)
	go func() { done <- session.Wait() }()
	select {
	case err = <-done:
	case <-ctx.Done():
		_ = session.Close()
		err = ctx.Err()
	}
	if err != nil {
		return out.Bytes(), errBuf.Bytes(), fmt.Errorf("remote: run %s: %w", args[0], err)
	}
	return out.Bytes(), errBuf.Bytes(), nil
}

// Sudo returns args wrapped with passwordless sudo ("sudo -n"). Use it only for
// a non-root user whose passwordless sudo was confirmed: "sudo -n" fails instead
// of waiting for a password on a connection with no terminal.
func Sudo(args []string) []string {
	return append([]string{"sudo", "-n"}, args...)
}

// IsRoot reports whether the connected user is uid 0 on the server.
func (c *Client) IsRoot(ctx context.Context) (bool, error) {
	out, _, err := c.Run(ctx, []string{"id", "-u"}, nil)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(string(out)) == "0", nil
}

// HasPasswordlessSudo reports whether running a command under "sudo -n" works.
// A missing sudo or a password prompt counts as false, not an error.
func (c *Client) HasPasswordlessSudo(ctx context.Context) (bool, error) {
	_, _, err := c.Run(ctx, Sudo([]string{"true"}), nil)
	if err == nil {
		return true, nil
	}
	var exit *ssh.ExitError
	if errors.As(err, &exit) {
		return false, nil
	}
	return false, err
}

// authMethods builds the client authentication methods from cfg. A key file
// takes precedence; otherwise the SSH agent is used and its connection is
// returned so the caller can close it.
func authMethods(cfg Config) ([]ssh.AuthMethod, net.Conn, error) {
	if cfg.KeyFile != "" {
		key, err := os.ReadFile(cfg.KeyFile)
		if err != nil {
			return nil, nil, fmt.Errorf("remote: read key %s: %w", cfg.KeyFile, err)
		}
		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			return nil, nil, fmt.Errorf("remote: parse key %s: %w", cfg.KeyFile, err)
		}
		return []ssh.AuthMethod{ssh.PublicKeys(signer)}, nil, nil
	}
	sock := os.Getenv("SSH_AUTH_SOCK")
	if sock == "" {
		return nil, nil, errors.New("remote: no SSH credentials: set KeyFile or start an SSH agent")
	}
	conn, err := net.Dial("unix", sock)
	if err != nil {
		return nil, nil, fmt.Errorf("remote: connect to SSH agent: %w", err)
	}
	return []ssh.AuthMethod{ssh.PublicKeysCallback(agent.NewClient(conn).Signers)}, conn, nil
}

// hostKeyCallback returns the ssh host key callback that enforces A6: known
// keys pass, unknown keys are offered to confirm, changed keys are always
// refused.
func hostKeyCallback(path string, confirm ConfirmFunc) (ssh.HostKeyCallback, error) {
	path, err := knownHostsPath(path)
	if err != nil {
		return nil, err
	}
	check, err := knownhosts.New(path)
	if err != nil {
		return nil, fmt.Errorf("remote: load known_hosts %s: %w", path, err)
	}
	return func(host string, remote net.Addr, key ssh.PublicKey) error {
		err := check(host, remote, key)
		if err == nil {
			return nil
		}
		var keyErr *knownhosts.KeyError
		if !errors.As(err, &keyErr) {
			return err
		}
		fingerprint := ssh.FingerprintSHA256(key)
		if len(keyErr.Want) > 0 {
			return fmt.Errorf("remote: host key for %s changed (now %s): refusing to connect", host, fingerprint)
		}
		if confirm != nil && confirm(host, fingerprint) {
			if err := appendKnownHost(path, host, key); err != nil {
				return fmt.Errorf("remote: save host key for %s: %w", host, err)
			}
			return nil
		}
		return fmt.Errorf("remote: unknown host key for %s (%s): refusing to connect", host, fingerprint)
	}, nil
}

// knownHostsPath resolves path, defaulting to ~/.ssh/known_hosts, and creates an
// empty file when none exists so that every host is treated as unknown and goes
// through ConfirmFunc rather than failing to load.
func knownHostsPath(path string) (string, error) {
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("remote: find home directory: %w", err)
		}
		path = filepath.Join(home, ".ssh", "known_hosts")
	}
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return "", fmt.Errorf("remote: create known_hosts directory %s: %w", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, nil, 0o600); err != nil {
			return "", fmt.Errorf("remote: create known_hosts %s: %w", path, err)
		}
	} else if err != nil {
		return "", fmt.Errorf("remote: stat known_hosts %s: %w", path, err)
	}
	return path, nil
}

// appendKnownHost adds key for host to the known_hosts file after the user
// confirmed it, so later connections verify silently and a different key is
// reported as changed instead of unknown (A6). It keeps the file at 0600 and
// never adds the same line twice.
func appendKnownHost(path, host string, key ssh.PublicKey) error {
	line := knownhosts.Line([]string{knownhosts.Normalize(host)}, key)
	existing, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	for _, l := range strings.Split(string(existing), "\n") {
		if strings.TrimSpace(l) == line {
			return nil
		}
	}
	prefix := ""
	if len(existing) > 0 && existing[len(existing)-1] != '\n' {
		prefix = "\n"
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	if _, err := f.WriteString(prefix + line + "\n"); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Chmod(path, 0o600)
}

func closeIf(c net.Conn) {
	if c != nil {
		c.Close()
	}
}

// Prefer algorithms for keys already trusted for this host. Otherwise a server
// offering multiple keys can negotiate an unknown RSA key despite a pinned
// Ed25519 key. Verification still runs normally, including changed-key refusal.
func knownAlgorithms(addr, file string) []string {
	file, err := knownHostsPath(file)
	if err != nil {
		return nil
	}
	check, err := knownhosts.New(file)
	if err != nil {
		return nil
	}
	b, err := os.ReadFile(file)
	if err != nil {
		return nil
	}
	remote, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		return nil
	}
	algorithms := []string{}
	for len(b) > 0 {
		marker, _, key, _, rest, e := ssh.ParseKnownHosts(b)
		if e != nil {
			break
		}
		b = rest
		if marker != "" || check(addr, remote, key) != nil {
			continue
		}
		if key.Type() == ssh.KeyAlgoRSA {
			algorithms = append(algorithms, ssh.KeyAlgoRSASHA512, ssh.KeyAlgoRSASHA256)
		} else {
			algorithms = append(algorithms, key.Type())
		}
	}
	return algorithms
}
