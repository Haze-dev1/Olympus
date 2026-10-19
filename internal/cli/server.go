package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"

	"github.com/Haze-dev1/Olympus/internal/activate"
	"github.com/Haze-dev1/Olympus/internal/eval"
	"github.com/Haze-dev1/Olympus/internal/generation"
)

var generationID = regexp.MustCompile(`^sha256-[0-9a-f]{64}$`)

// storeManifest loads a generation's manifest from the server store, so health
// checks for activation and rollback come from the generation itself (ADR 0028).
func storeManifest(root string) activate.ManifestHealthSource {
	return func(id string) (generation.Manifest, error) {
		var m generation.Manifest
		if !generationID.MatchString(id) {
			return m, errors.New("invalid generation id")
		}
		b, err := os.ReadFile(filepath.Join(root, "store", "gen-"+id, "manifest.json"))
		if err != nil {
			return m, err
		}
		if len(b) > 4<<20 || json.Unmarshal(b, &m) != nil {
			return m, errors.New("invalid generation manifest")
		}
		return m, nil
	}
}

// quietCompose avoids copying untrusted container output into activator logs.
type quietCompose struct{ activate.Compose }

func (c quietCompose) Pull(ctx context.Context, p string) error {
	if c.Compose.Pull(ctx, p) != nil {
		return errors.New("compose pull failed")
	}
	return nil
}
func (c quietCompose) Up(ctx context.Context, p string) error {
	if c.Compose.Up(ctx, p) != nil {
		return errors.New("compose up failed")
	}
	return nil
}
func (c quietCompose) Down(ctx context.Context, p string) error {
	if c.Compose.Down(ctx, p) != nil {
		return errors.New("compose down failed")
	}
	return nil
}
func (c quietCompose) Exec(ctx context.Context, p, t string, a []string) error {
	if c.Compose.Exec(ctx, p, t, a) != nil {
		return errors.New("compose exec failed")
	}
	return nil
}

func server(args []string, out, stderr io.Writer) int {
	if args[0] == "version" {
		if len(args) != 1 {
			return 1
		}
		fmt.Fprintln(out, eval.Version)
		return 0
	}
	if args[0] == "activate" && len(args) != 2 || args[0] == "confirm" && len(args) != 3 {
		fmt.Fprintln(stderr, "usage: olym activate <id> | olym confirm <id> <nonce>")
		return 1
	}
	if os.Geteuid() != 0 {
		fmt.Fprintln(stderr, "server commands require root")
		return 1
	}
	runner := quietCompose{}
	a := activate.Activator{Runner: runner, Checker: activate.Checker{Runner: runner}, Health: storeManifest(activate.DefaultRoot), Log: out}
	var err error
	if args[0] == "activate" {
		err = a.Activate(context.Background(), args[1])
	} else {
		err = a.Confirm(args[1], args[2])
	}
	if err == nil {
		return 0
	}
	// Errors from payload decoding may include input bytes. Keep the CLI output
	// generic; the activator log contains only validated operational events.
	var locked *activate.LockedError
	if errors.As(err, &locked) {
		fmt.Fprintln(stderr, locked)
		return 4
	}
	var rollbackFailed *activate.RollbackFailedError
	if errors.As(err, &rollbackFailed) {
		fmt.Fprintln(stderr, "activation failed and the server could not roll back; it may be down")
		return 5
	}
	var rolledBack *activate.RolledBackError
	if errors.As(err, &rolledBack) {
		fmt.Fprintln(stderr, "activation failed; server rolled back")
		return 3
	}
	fmt.Fprintln(stderr, "server command failed; inspect the activation state and log")
	return 1
}
