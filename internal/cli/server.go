package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"time"

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
	flags := flag.NewFlagSet(args[0], flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	_ = flags.Bool("server", false, "internal server command")
	request := flags.String("request", "", "activation request")
	payload := flags.String("payload", "", "secrets payload basename")
	timeout := flags.Duration("confirm-timeout", activate.DefaultConfirmTimeout, "confirmation timeout")
	expected := flags.String("expected-current", "", "expected current id")
	keep := flags.Int("keep", activate.DefaultKeep, "generations to keep")
	data := flags.Bool("delete-data", false, "delete data")
	_ = flags.Bool("json", false, "JSON output")
	if flags.Parse(args[1:]) != nil || *timeout <= 0 || *timeout > 24*time.Hour {
		return 1
	}
	if *payload != "" && !regexp.MustCompile(`^secrets-[0-9a-f]{32}\.incoming$`).MatchString(*payload) {
		return 1
	}
	if *request != "" && !regexp.MustCompile(`^[0-9a-f]{32}$`).MatchString(*request) {
		return 1
	}
	if os.Geteuid() != 0 {
		fmt.Fprintln(stderr, "server commands require root")
		return 1
	}
	runner := quietCompose{}
	a := activate.Activator{Runner: runner, Checker: activate.Checker{Runner: runner}, Health: storeManifest(activate.DefaultRoot), Log: out, Request: *request, Payload: *payload, ConfirmTimeout: *timeout}
	flags.Visit(func(f *flag.Flag) {
		if f.Name == "expected-current" {
			a.ExpectedCurrent = expected
		}
	})
	var err error
	switch args[0] {
	case "activate":
		if flags.NArg() != 1 {
			return 1
		}
		err = a.Activate(context.Background(), flags.Arg(0))
	case "confirm":
		if flags.NArg() != 2 {
			return 1
		}
		err = a.Confirm(flags.Arg(0), flags.Arg(1))
	case "rollback":
		if flags.NArg() != 0 {
			return 1
		}
		err = a.Rollback(context.Background())
	case "gc":
		if flags.NArg() != 0 {
			return 1
		}
		var result activate.GCResult
		result, err = a.GC(context.Background(), *keep)
		if err == nil {
			err = json.NewEncoder(out).Encode(result)
		}
	case "destroy":
		if flags.NArg() != 0 {
			return 1
		}
		err = a.Destroy(context.Background(), *data)
	case "status":
		if flags.NArg() != 0 {
			return 1
		}
		var result []activate.ToolStatus
		result, err = a.Status(context.Background())
		if err == nil {
			err = json.NewEncoder(out).Encode(result)
		}
	default:
		return 1
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

func (c quietCompose) Recreate(ctx context.Context, p string, tools []string) error {
	if c.Compose.Recreate(ctx, p, tools) != nil {
		return errors.New("compose recreate failed")
	}
	return nil
}
