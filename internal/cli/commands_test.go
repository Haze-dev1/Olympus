package cli

import (
	"bytes"
	"flag"
	"reflect"
	"strings"
	"testing"

	"github.com/Haze-dev1/Olympus/internal/deploy"
	"github.com/Haze-dev1/Olympus/internal/eval"
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

func TestWarnDevActivator(t *testing.T) {
	origVersion := eval.Version
	origAmd64 := deploy.ReleaseChecksumLinuxAmd64
	origArm64 := deploy.ReleaseChecksumLinuxArm64
	defer func() {
		eval.Version = origVersion
		deploy.ReleaseChecksumLinuxAmd64 = origAmd64
		deploy.ReleaseChecksumLinuxArm64 = origArm64
	}()

	tests := []struct {
		name               string
		version            string
		amd64Checksum      string
		arm64Checksum      string
		unsafeDevActivator bool
		wantWarning        bool
	}{
		{
			name:               "dev build without checksums and without flag warns",
			version:            eval.DevelopmentVersion,
			amd64Checksum:      "",
			arm64Checksum:      "",
			unsafeDevActivator: false,
			wantWarning:        true,
		},
		{
			name:               "dev build without checksums with flag suppresses warning",
			version:            eval.DevelopmentVersion,
			amd64Checksum:      "",
			arm64Checksum:      "",
			unsafeDevActivator: true,
			wantWarning:        false,
		},
		{
			name:               "dev build with embedded checksums suppresses warning",
			version:            eval.DevelopmentVersion,
			amd64Checksum:      "hash-amd64",
			arm64Checksum:      "hash-arm64",
			unsafeDevActivator: false,
			wantWarning:        false,
		},
		{
			name:               "untagged release script build suppresses warning",
			version:            "0.1.0-dev+6378475",
			amd64Checksum:      "hash-amd64",
			arm64Checksum:      "hash-arm64",
			unsafeDevActivator: false,
			wantWarning:        false,
		},
		{
			name:               "tagged release build suppresses warning",
			version:            "1.0.0",
			amd64Checksum:      "hash-amd64",
			arm64Checksum:      "hash-arm64",
			unsafeDevActivator: false,
			wantWarning:        false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			eval.Version = tc.version
			deploy.ReleaseChecksumLinuxAmd64 = tc.amd64Checksum
			deploy.ReleaseChecksumLinuxArm64 = tc.arm64Checksum

			var buf bytes.Buffer
			warnDevActivator(&buf, tc.unsafeDevActivator)
			hasWarning := strings.Contains(buf.String(), "Warning: development build; unsigned activators require -unsafe-dev-activator")
			if hasWarning != tc.wantWarning {
				t.Fatalf("warnDevActivator() warning=%v, want %v (output: %q)", hasWarning, tc.wantWarning, buf.String())
			}
		})
	}
}
