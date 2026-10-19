// Package cli implements the two offline commands using the standard flag package.
package cli

import (
	"flag"
	"fmt"
	"io"
	"path/filepath"

	"github.com/Haze-dev1/Olympus/internal/eval"
	"github.com/Haze-dev1/Olympus/internal/stack"
)

func Run(args []string, out, stderr io.Writer) int {
	if len(args) == 0 || (args[0] != "check" && args[0] != "build") {
		fmt.Fprintln(stderr, "usage: olym {check|build} [-f stack.yaml] [-recipes recipes] [-o .olympus/build]")
		return 1
	}
	flags := flag.NewFlagSet(args[0], flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	file := flags.String("f", "stack.yaml", "stack file")
	recipes := flags.String("recipes", "recipes", "local recipe directory")
	output := flags.String("o", ".olympus/build", "build directory")
	if flags.Parse(args[1:]) != nil || flags.NArg() != 0 {
		fmt.Fprintln(stderr, "invalid flags; use -f, -recipes and -o")
		return 1
	}
	s, d := stack.Load(*file)
	stack.CheckEnv(d, *file)
	for _, e := range d.Errors {
		fmt.Fprintln(stderr, e.Error())
	}
	if len(d.Errors) > 0 {
		return 1
	}
	if args[0] == "check" {
		fmt.Fprintln(out, "Stack is valid.")
		return 0
	}
	inputs, errors := eval.Load(s, *recipes, filepath.Dir(*file))
	for _, e := range errors {
		fmt.Fprintln(stderr, e.Error())
	}
	if len(errors) > 0 {
		return 1
	}
	g, err := eval.Build(inputs)
	if err != nil {
		fmt.Fprintf(stderr, "%s:1:1: %s\n", *file, err)
		return 1
	}
	for _, warning := range g.Warnings {
		fmt.Fprintln(stderr, "warning:", warning)
	}
	if err = g.Write(*output); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintln(out, "Built", g.Manifest.ID)
	return 0
}
