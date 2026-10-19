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
	f.String("activator", "", "binary")
	args := interspersedArgs(f, []string{"-f", "stack.yaml", "plan.json", "--yes", "-activator", "binary"})
	if err := f.Parse(args); err != nil || !reflect.DeepEqual(f.Args(), []string{"plan.json"}) || f.Lookup("yes").Value.String() != "true" {
		t.Fatalf("%v %v", args, err)
	}
}
