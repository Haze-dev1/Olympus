package remote

import (
	"os/exec"
	"testing"
)

func TestQuoteRoundTripsThroughSh(t *testing.T) {
	inputs := []string{
		"",
		"plain",
		"two words",
		`single'quote`,
		`double"quote`,
		"$(echo pwned)",
		"`echo pwned`",
		"line1\nline2",
		"tab\there",
		"* ? [a-z]",
		`back\slash`,
		"$HOME",
		"; echo pwned",
		"&& echo pwned",
		"| cat",
		"> /tmp/x",
		`a'b"c$d` + "`e`",
		"-n",
		"%s",
	}
	for _, in := range inputs {
		out, err := exec.Command("/bin/sh", "-c", "printf %s "+quote(in)).Output()
		if err != nil {
			t.Fatalf("quote(%q): %v", in, err)
		}
		if string(out) != in {
			t.Errorf("quote(%q) round-tripped to %q", in, out)
		}
	}
}

func TestJoinArgsRoundTripsThroughSh(t *testing.T) {
	args := []string{"printf", "%s|%s", "a b", `c'd"e$f`}
	out, err := exec.Command("/bin/sh", "-c", joinArgs(args)).Output()
	if err != nil {
		t.Fatalf("joinArgs(%q): %v", args, err)
	}
	if want := `a b|c'd"e$f`; string(out) != want {
		t.Errorf("joinArgs round-tripped to %q, want %q", out, want)
	}
}
