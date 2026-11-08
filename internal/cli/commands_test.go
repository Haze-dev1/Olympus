package cli

import (
	"flag"
	"reflect"
	"testing"
)

func TestApplySavedPlanFlags(t *testing.T) {
	f := flag.NewFlagSet("apply", flag.ContinueOnError)
	f.String("f", "", "file")
	f.Bool("yes", false, "yes")
	f.Bool("unsafe-dev-activator", false, "allow an unsigned development activator")
	f.String("activator", "", "binary")
	args := interspersedArgs(f, []string{"-f", "stack.yaml", "plan.json", "--yes", "-unsafe-dev-activator", "-activator", "binary"})
	if err := f.Parse(args); err != nil || !reflect.DeepEqual(f.Args(), []string{"plan.json"}) || f.Lookup("yes").Value.String() != "true" || f.Lookup("unsafe-dev-activator").Value.String() != "true" {
		t.Fatalf("%v %v", args, err)
	}
}
