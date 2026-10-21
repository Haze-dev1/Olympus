// Package cli implements stack validation, catalog updates and local builds.
package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Haze-dev1/Olympus/internal/catalog"
	"github.com/Haze-dev1/Olympus/internal/eval"
	"github.com/Haze-dev1/Olympus/internal/lock"
	"github.com/Haze-dev1/Olympus/internal/schema"
	"github.com/Haze-dev1/Olympus/internal/stack"
)

func Run(args []string, out, stderr io.Writer) int {
	return run(args, out, stderr, catalog.Fetcher{Token: os.Getenv("GITHUB_TOKEN")}, lock.Resolver{})
}

func run(args []string, out, stderr io.Writer, fetch catalog.Fetcher, resolver lock.Resolver) int {
	if len(args) > 1 && (args[0] == "rollback" || args[0] == "gc" || args[0] == "destroy" || args[0] == "status") && args[1] == "--server" {
		return server(args, out, stderr)
	}
	if len(args) > 0 && (args[0] == "activate" || args[0] == "confirm" || args[0] == "version") {
		return server(args, out, stderr)
	}
	if len(args) == 0 || (args[0] != "check" && args[0] != "build" && args[0] != "update" && args[0] != "plan" && args[0] != "apply" && args[0] != "status" && args[0] != "rollback" && args[0] != "gc" && args[0] != "destroy") {
		fmt.Fprintln(stderr, "usage: olym {check|build|update|plan|apply|status|rollback|gc|destroy} [-f stack.yaml] [-recipes recipes] [-o .olympus/build]")
		return 1
	}
	flags := flag.NewFlagSet(args[0], flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	file := flags.String("f", "stack.yaml", "stack file")
	recipes := flags.String("recipes", "", "local recipe directory")
	output := flags.String("o", ".olympus/build", "build directory")
	known := flags.String("known-hosts", "", "SSH known_hosts file")
	activator := flags.String("activator", "", "Linux activator binary")
	planOut := flags.String("out", "", "saved plan output")
	yes := flags.Bool("yes", false, "skip apply prompt")
	keep := flags.Int("keep", 0, "generations to keep")
	deleteData := flags.Bool("delete-data", false, "delete project volumes and data")
	jsonOutput := flags.Bool("json", false, "JSON status")
	detailed := flags.Bool("detailed-exitcode", false, "exit 2 when plan finds changes")
	parseArgs := interspersedArgs(flags, args[1:])
	if flags.Parse(parseArgs) != nil || (flags.NArg() != 0 && (args[0] != "apply" || flags.NArg() != 1)) || *keep < 0 {
		fmt.Fprintln(stderr, "invalid flags; use -f, -recipes, -o, -known-hosts and --detailed-exitcode")
		return 1
	}
	s, d := stack.Load(*file)
	if args[0] == "check" || args[0] == "build" || args[0] == "apply" || args[0] == "plan" {
		stack.CheckEnv(d, *file)
	}
	for _, e := range d.Errors {
		fmt.Fprintln(stderr, e.Error())
	}
	if len(d.Errors) > 0 {
		return 1
	}
	if args[0] == "status" || args[0] == "rollback" || args[0] == "gc" || args[0] == "destroy" {
		return serverCommand(args[0], s, *known, *activator, *keep, *deleteData, *jsonOutput, out, stderr)
	}
	inputs, err := prepare(context.Background(), s, *file, *recipes, args[0], out, fetch, resolver)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if args[0] == "update" {
		fmt.Fprintln(out, "Updated stack.lock.")
		return 0
	}
	if args[0] == "check" {
		if _, err := eval.Build(inputs); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		fmt.Fprintln(out, "Stack is valid.")
		return 0
	}

	g, err := eval.Build(inputs)
	if err != nil {
		fmt.Fprintf(stderr, "%s:1:1: %s\n", *file, err)
		return 1
	}
	for _, warning := range g.Warnings {
		fmt.Fprintln(stderr, "warning:", warning)
	}
	if args[0] == "plan" || args[0] == "apply" {
		return manageGeneration(args[0], inputs, g, *file, *known, *activator, flags.Arg(0), *planOut, *yes, *detailed, out, stderr)
	}
	if err = g.Write(*output); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintln(out, "Built", g.Manifest.ID)
	return 0
}

func prepare(ctx context.Context, s *stack.Stack, file, local, command string, out io.Writer, fetch catalog.Fetcher, resolver lock.Resolver) (eval.Inputs, error) {
	var empty eval.Inputs
	lockPath := filepath.Join(filepath.Dir(file), "stack.lock")
	var pinned *lock.File
	refresh := command == "update"
	if !refresh {
		var err error
		pinned, err = lock.Read(lockPath)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return empty, err
		}
		if errors.Is(err, os.ErrNotExist) && local == "" {
			if command != "check" {
				return empty, fmt.Errorf("stack.lock is missing; run olym check or olym update")
			}
			refresh = true
		}
	}
	if pinned != nil {
		for _, name := range schema.Keys(s.Tools()) {
			if _, ok := pinned.Tools[name]; !ok {
				return empty, fmt.Errorf("stack.lock lacks tool %s; run olym update", name)
			}
			if pinned.Tools[name].ImageTag != s.Tools()[name].ImageTag {
				return empty, fmt.Errorf("%s: image_tag differs from stack.lock; run olym update", name)
			}
		}
	}
	source := s.Inputs.Recipes
	if source == "" {
		source = "github:Haze-dev1/Olympus/recipes"
	}
	root, revision := local, ""
	var input lock.Input
	if local != "" {
		fmt.Fprintln(out, "Local -recipes override: skipping the lock's catalog pin.")
		if refresh {
			return empty, fmt.Errorf("olym update requires a GitHub catalog; omit -recipes")
		}
	} else {
		src, err := catalog.ParseSource(source)
		if err != nil {
			return empty, err
		}
		if refresh {
			revision, err = fetch.Latest(ctx, src)
			if err != nil {
				return empty, err
			}
			var hash string
			root, hash, err = fetch.Fetch(ctx, src, revision, "")
			input = lock.Input{URL: source, Rev: revision, Hash: hash}
		} else {
			var ok bool
			input, ok = pinned.Inputs["recipes"]
			if !ok || input.URL != source {
				return empty, fmt.Errorf("stack.lock catalog input differs from stack; run olym update")
			}
			revision = input.Rev
			root, _, err = fetch.Fetch(ctx, src, revision, input.Hash)
		}
		if err != nil {
			return empty, err
		}
	}

	inputs, diagnostics := eval.Load(s, root, filepath.Dir(file))
	if len(diagnostics) > 0 {
		var messages string
		for _, d := range diagnostics {
			messages += d.Error() + "\n"
		}
		return empty, errors.New(messages)
	}
	inputs.Revision = revision
	if refresh {
		pinned = &lock.File{Version: 1, Inputs: map[string]lock.Input{"recipes": input}, Tools: map[string]lock.Tool{}}
		for _, name := range schema.Keys(s.Tools()) {
			tool := s.Tools()[name]
			image, err := eval.Image(inputs.Recipes[tool.Recipe].Image, tool.ImageTag)
			if err != nil {
				return empty, fmt.Errorf("%s: %w", name, err)
			}
			image, err = resolver.Resolve(ctx, image)
			if err != nil {
				return empty, err
			}
			pinned.Tools[name] = lock.Tool{Image: image, ImageTag: tool.ImageTag}
		}
		if command == "check" {
			inputs.Images = map[string]string{}
			for name, tool := range pinned.Tools {
				inputs.Images[name] = tool.Image
			}
			if _, err := eval.Build(inputs); err != nil {
				return empty, err
			}
		}
		if err := lock.Write(lockPath, pinned); err != nil {
			return empty, err
		}
	}
	if pinned != nil {
		inputs.Images = map[string]string{}
		for name, tool := range pinned.Tools {
			inputs.Images[name] = tool.Image
		}
	}
	return inputs, nil
}

// flag.FlagSet stops at the first positional argument. Preserve flag/value
// pairs while allowing the documented `apply plan.json --yes` syntax.
func interspersedArgs(flags *flag.FlagSet, args []string) []string {
	options, positionals := []string{}, []string{}
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			positionals = append(positionals, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(a, "-") || a == "-" {
			positionals = append(positionals, a)
			continue
		}
		options = append(options, a)
		name, _, hasValue := strings.Cut(strings.TrimLeft(a, "-"), "=")
		f := flags.Lookup(name)
		if f == nil || hasValue {
			continue
		}
		b, ok := f.Value.(interface{ IsBoolFlag() bool })
		if ok && b.IsBoolFlag() {
			continue
		}
		if i+1 < len(args) {
			i++
			options = append(options, args[i])
		}
	}
	return append(options, positionals...)
}
