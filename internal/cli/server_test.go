package cli

import (
	"bytes"
	"github.com/Haze-dev1/Olympus/internal/eval"
	"testing"
)

func TestServerDispatch(t *testing.T) {
	for _, args := range [][]string{{"activate"}, {"confirm", "id"}, {"version", "extra"}} {
		var out, err bytes.Buffer
		if Run(args, &out, &err) != 1 {
			t.Fatal(args)
		}
	}
	var out, err bytes.Buffer
	if Run([]string{"version"}, &out, &err) != 0 || out.String() != eval.Version+"\n" {
		t.Fatalf("version: %s %s", &out, &err)
	}
}
