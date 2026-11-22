package cli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/Haze-dev1/Olympus/internal/generation"
	"github.com/Haze-dev1/Olympus/internal/plan"
)

func TestKnownHostsOrder(t *testing.T) {
	for _, tc := range []struct{ flag, env, want string }{
		{"explicit", "environment", "explicit"}, {"", "environment", "environment"}, {"", "", "/home/example/.ssh/known_hosts"},
	} {
		if got := selectKnownHosts(tc.flag, tc.env, "/home/example"); got != tc.want {
			t.Fatal(got)
		}
	}
}

type planReader struct {
	manifest []byte
	id       string
	fail     bool
}

func (r planReader) Run(_ context.Context, args []string, _ io.Reader) ([]byte, []byte, error) {
	if r.fail {
		return nil, nil, fmt.Errorf("sensitive")
	}
	if args[0] == "head" {
		return r.manifest, nil, nil
	}
	if args[1] == "--" {
		return []byte("generations/1\n"), nil, nil
	}
	return []byte("/var/lib/olympus/store/gen-" + r.id + "\n"), nil, nil
}

func readPlan(ctx context.Context, r plan.Reader, root bool, want generation.Manifest, file, server string, detailed bool, out, stderr io.Writer) int {
	current, actual, err := plan.ReadCurrent(ctx, r, root)
	if err != nil {
		fmt.Fprintln(stderr, "plan:", err)
		return 1
	}
	id := current.ID
	if id == "" {
		id = "empty"
	}
	fmt.Fprintf(out, "Plan: stack %q → %s (generation %s → new)\n\n", file, server, id)
	diff := plan.Compare(want, current, actual)
	diff.Print(out)
	if detailed && len(diff.Changes) > 0 {
		return 2
	}
	return 0
}
func TestPlanFakeReader(t *testing.T) {
	g := generation.Generation{Manifest: generation.Manifest{Version: 1, Olym: "test", Tools: map[string]generation.Tool{}}, Files: map[string][]byte{}}
	if err := g.Finish(); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		changed, detailed, fail bool
		code                    int
	}{{false, true, false, 0}, {true, true, false, 2}, {true, false, false, 0}, {false, true, true, 1}} {
		want := g.Manifest
		want.Tools = map[string]generation.Tool{}
		if tc.changed {
			want.Tools["demo"] = generation.Tool{}
		}
		var out, stderr bytes.Buffer
		code := readPlan(context.Background(), planReader{g.Files["manifest.json"], g.Manifest.ID, tc.fail}, true, want, "stack.yaml", "server", tc.detailed, &out, &stderr)
		if code != tc.code || strings.Contains(stderr.String(), "sensitive") {
			t.Fatalf("code %d, %s", code, &stderr)
		}
		if tc.changed && !tc.fail && !strings.Contains(out.String(), "new tool") {
			t.Fatal(out.String())
		}
	}
}
