package deploy

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
)

// UseServer checks privilege without installing anything, for read and cleanup commands.
func (c *Client) UseServer(ctx context.Context) error {
	c.sudo = false
	b, err := c.run(ctx, []string{"id", "-u"}, nil)
	if err != nil {
		return err
	}
	c.sudo = string(b) != "0\n" && string(b) != "0"
	if c.sudo {
		_, err = c.run(ctx, []string{"true"}, nil)
	}
	return err
}

func (c *Client) GC(ctx context.Context, keep int) ([]byte, error) {
	if keep < 1 {
		return nil, errors.New("gc: keep must be positive")
	}
	return c.run(ctx, []string{c.binary, "gc", "--server", "-keep", strconv.Itoa(keep)}, nil)
}
func (c *Client) Destroy(ctx context.Context, data bool) error {
	args := []string{c.binary, "destroy", "--server"}
	if data {
		args = append(args, "--delete-data")
	}
	_, err := c.run(ctx, args, nil)
	return err
}

// SecretState reads only key material and fingerprints over verified SSH.
// Missing metadata is allowed only on a server with no current generation.
func (c *Client) SecretState(ctx context.Context, tools []string, haveCurrent bool) ([]byte, map[string]map[string]string, error) {
	key, err := c.run(ctx, []string{"head", "-c", "33", "--", root + "/key"}, nil)
	if err != nil && !haveCurrent {
		return nil, map[string]map[string]string{}, nil
	}
	if err != nil || len(key) != 32 {
		return nil, nil, errors.New("plan: missing or invalid server fingerprint key")
	}
	fingerprints := map[string]map[string]string{}
	for _, tool := range tools {
		b, e := c.run(ctx, []string{"head", "-c", "1048577", "--", root + "/secrets/" + tool + "/fingerprints.json"}, nil)
		if e != nil {
			continue
		}
		var fp map[string]string
		if len(b) > 1<<20 || json.Unmarshal(b, &fp) != nil {
			return nil, nil, errors.New("plan: invalid secret fingerprints")
		}
		fingerprints[tool] = fp
	}
	return key, fingerprints, nil
}

func (c *Client) Status(ctx context.Context) ([]byte, error) {
	return c.run(ctx, []string{c.binary, "status", "--server", "--json"}, nil)
}

// VerifyActivator permits read-only commands without uploading a binary. It
// applies the same local-byte checksum and version checks as deployment.
func (c *Client) VerifyActivator(ctx context.Context, local string) error {
	if local == "" {
		var err error
		local, err = os.Executable()
		if err != nil {
			return err
		}
	}
	b, err := os.ReadFile(local)
	if err != nil {
		return errors.New("deploy: cannot read activator")
	}
	c.binary = root + "/bin/olym-" + c.Version
	c.checksum = fmt.Sprintf("%x", sha256.Sum256(b))
	return c.verifyActivator(ctx)
}
