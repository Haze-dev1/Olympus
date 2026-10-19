package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/Haze-dev1/Olympus/internal/activate"
	"github.com/Haze-dev1/Olympus/internal/deploy"
	"github.com/Haze-dev1/Olympus/internal/eval"
	"github.com/Haze-dev1/Olympus/internal/generation"
	"github.com/Haze-dev1/Olympus/internal/plan"
	"github.com/Haze-dev1/Olympus/internal/schema"
	"github.com/Haze-dev1/Olympus/internal/stack"
)

type savedPlan struct {
	Version      int                          `json:"version"`
	Generation   string                       `json:"generation"`
	Current      string                       `json:"current"`
	Server       string                       `json:"server"`
	Fingerprints map[string]map[string]string `json:"fingerprints"`
}

func ask(prompt string, stderr io.Writer) bool {
	fmt.Fprint(stderr, prompt+" [y/N] ")
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	return err == nil && strings.EqualFold(strings.TrimSpace(line), "y")
}

func manageGeneration(command string, in eval.Inputs, g *generation.Generation, file, known, activator, saved, output string, yes, detailed bool, out, stderr io.Writer) int {
	secrets, err := toolSecrets(in, file)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	cfg, err := connectionConfig(in.Stack, known, stderr)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	c, err := deploy.Dial(cfg, eval.Version)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	defer c.Conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	fail := func(err error) int { fmt.Fprintln(stderr, err); return 1 }
	if err = c.UseServer(ctx); err != nil {
		return fail(err)
	}
	current, actual, err := plan.ReadCurrent(ctx, c.Conn, cfg.User == "root")
	if err != nil {
		return fail(err)
	}
	names := schema.Keys(secrets)
	key, previous, err := c.SecretState(ctx, names, current.ID != "")
	if err != nil {
		return fail(err)
	}
	fingerprints := map[string]map[string]string{}
	changed := map[string]bool{}
	for name, ts := range secrets {
		if len(key) == 0 {
			// A first-server plan must not persist a publicly keyed secret hash.
			// The initial apply reads current values; there is no old secret to compare.
			fingerprints[name] = nil
			changed[name] = len(ts.Secrets) > 0 || len(ts.Environment) > 0
			continue
		}
		fp := activate.Fingerprints(key, ts.Secrets, ts.Environment)
		fingerprints[name] = fp
		changed[name] = !reflect.DeepEqual(fp, previous[name]) && (len(fp) > 0 || len(previous[name]) > 0)
	}
	snapshot := savedPlan{Version: 1, Generation: g.Manifest.ID, Current: current.ID, Server: cfg.User + "@" + cfg.Host, Fingerprints: fingerprints}
	if saved != "" {
		b, e := os.ReadFile(saved)
		if e != nil {
			return fail(errors.New("cannot read saved plan"))
		}
		var expected savedPlan
		if json.Unmarshal(b, &expected) != nil || !reflect.DeepEqual(expected, snapshot) {
			return fail(errors.New("saved plan is stale: generation, server current or secrets changed; run olym plan"))
		}
	}
	diff := plan.Compare(g.Manifest, current, actual)
	diff.SecretChanges(changed)
	fmt.Fprintf(out, "Plan: stack %q → %s\n", file, snapshot.Server)
	diff.Print(out)
	if command == "plan" {
		if output != "" {
			b, e := json.MarshalIndent(snapshot, "", "  ")
			if e != nil {
				return fail(e)
			}
			if e = os.WriteFile(output, append(b, '\n'), 0600); e != nil {
				return fail(errors.New("cannot write saved plan"))
			}
			fmt.Fprintln(out, "Saved plan to", output)
		}
		if detailed && len(diff.Changes) > 0 {
			return 2
		}
		return 0
	}
	if len(diff.Changes) == 0 {
		fmt.Fprintln(out, "No changes.")
		return 0
	}
	if !yes && !ask("Apply these changes?", stderr) {
		fmt.Fprintln(out, "Cancelled.")
		return 1
	}
	for _, t := range g.Manifest.Tools {
		if !strings.Contains(t.Image, "@sha256:") {
			return fail(errors.New("apply requires digest-pinned images; run olym check or olym update"))
		}
	}
	if err = c.PrepareServer(ctx); err != nil {
		return fail(err)
	}
	if err = c.UploadGeneration(ctx, g); err != nil {
		return fail(err)
	}
	if err = c.UploadActivator(ctx, activator); err != nil {
		return fail(err)
	}
	if err = c.SendSecrets(ctx, g.Manifest, secrets); err != nil {
		return fail(err)
	}
	c.ExpectedCurrent = &snapshot.Current
	c.ConfirmTimeout = in.Stack.Server.ConfirmTimeout
	code, err := c.Activate(ctx, g.Manifest.ID)
	if err != nil {
		fmt.Fprintln(stderr, err)
	} else {
		fmt.Fprintln(out, "Applied", g.Manifest.ID)
	}
	return code
}

func serverCommand(command string, s *stack.Stack, known, activator string, keep int, data, jsonOutput bool, out, stderr io.Writer) int {
	if command == "destroy" && !ask("Destroy the Olympus project?", stderr) {
		fmt.Fprintln(out, "Cancelled.")
		return 1
	}
	cfg, err := connectionConfig(s, known, stderr)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	c, err := deploy.Dial(cfg, eval.Version)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	defer c.Conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	fail := func(e error) int { fmt.Fprintln(stderr, e); return deploy.ExitCode(e) }
	if err = c.UseServer(ctx); err != nil {
		return fail(err)
	}
	if command == "status" {
		current, _, e := plan.ReadCurrent(ctx, c.Conn, cfg.User == "root")
		if e != nil {
			return fail(e)
		}
		if current.ID == "" {
			if jsonOutput {
				fmt.Fprintln(out, "[]")
			} else {
				fmt.Fprintln(out, "No tools deployed.")
			}
			return 0
		}
		if e = c.VerifyActivator(ctx, activator); e != nil {
			return fail(e)
		}
		b, e := c.Status(ctx)
		if e != nil {
			return fail(e)
		}
		var rows []activate.ToolStatus
		if json.Unmarshal(b, &rows) != nil {
			return fail(errors.New("invalid server status"))
		}
		if jsonOutput {
			json.NewEncoder(out).Encode(rows)
		} else {
			for _, r := range rows {
				fmt.Fprintf(out, "%s\t%s\t%s\t%s\tgeneration %d\n", r.Name, r.State, r.Health, r.Image, r.Generation)
			}
		}
		return 0
	}
	if err = c.UploadActivator(ctx, activator); err != nil {
		return fail(err)
	}
	c.ConfirmTimeout = s.Server.ConfirmTimeout
	switch command {
	case "rollback":
		code, e := c.Rollback(ctx)
		if e != nil {
			fmt.Fprintln(stderr, e)
		} else {
			fmt.Fprintln(out, "Rolled back.")
		}
		return code
	case "gc":
		if keep == 0 {
			keep = s.Server.Keep
		}
		if keep == 0 {
			keep = activate.DefaultKeep
		}
		b, e := c.GC(ctx, keep)
		if e != nil {
			return fail(e)
		}
		out.Write(b)
	case "destroy":
		if err = c.Destroy(ctx, data); err != nil {
			return fail(err)
		}
		if data {
			fmt.Fprintln(out, "Destroyed Olympus, including project volumes and server data.")
		} else {
			fmt.Fprintln(out, "Destroyed Olympus containers. Project volumes and /var/lib/olympus were kept.")
		}
	}
	return 0
}
