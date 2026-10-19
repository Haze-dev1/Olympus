package remote

import (
	"strings"
	"testing"
)

func TestParseOSRelease(t *testing.T) {
	tests := []struct {
		name    string
		data    string
		want    OS
		wantErr bool
	}{
		{
			name: "ubuntu 22.04",
			data: "NAME=\"Ubuntu\"\nID=ubuntu\nVERSION_ID=\"22.04\"\n",
			want: OS{ID: "ubuntu", VersionID: "22.04"},
		},
		{
			name: "ubuntu 24.04 unquoted",
			data: "ID=ubuntu\nVERSION_ID=24.04\n",
			want: OS{ID: "ubuntu", VersionID: "24.04"},
		},
		{
			name: "debian 12",
			data: "PRETTY_NAME=\"Debian GNU/Linux 12 (bookworm)\"\nID=debian\nVERSION_ID=\"12\"\n",
			want: OS{ID: "debian", VersionID: "12"},
		},
		{
			name: "comments and blank lines",
			data: "# comment\n\nID=ubuntu\nVERSION_ID=\"22.04\"\n",
			want: OS{ID: "ubuntu", VersionID: "22.04"},
		},
		{name: "older ubuntu", data: "ID=ubuntu\nVERSION_ID=\"20.04\"\n", wantErr: true},
		{name: "debian 13", data: "ID=debian\nVERSION_ID=\"13\"\n", wantErr: true},
		{name: "fedora", data: "ID=fedora\nVERSION_ID=\"40\"\n", wantErr: true},
		{name: "missing version", data: "ID=ubuntu\n", wantErr: true},
		{name: "empty", data: "", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseOSRelease([]byte(tt.data))
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseOSRelease(%q) = %+v, want an error", tt.data, got)
				}
				if !strings.Contains(err.Error(), "Ubuntu 22.04") {
					t.Errorf("error %q does not name the supported systems", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseOSRelease(%q): %v", tt.data, err)
			}
			if got != tt.want {
				t.Errorf("ParseOSRelease(%q) = %+v, want %+v", tt.data, got, tt.want)
			}
		})
	}
}

func TestDockerInstallSteps(t *testing.T) {
	steps := DockerInstallSteps(OS{ID: "ubuntu", VersionID: "24.04"})
	if len(steps) == 0 {
		t.Fatal("DockerInstallSteps returned no steps")
	}
	var repo string
	installsCompose := false
	aptNonInteractive := 0
	for i, step := range steps {
		if len(step.Args) == 0 {
			t.Fatalf("step %d has no arguments", i)
		}
		if step.Args[0] == "apt-get" {
			t.Errorf("step %q runs apt-get without DEBIAN_FRONTEND", step.Args)
		}
		if len(step.Args) >= 3 && step.Args[0] == "env" && step.Args[1] == "DEBIAN_FRONTEND=noninteractive" && step.Args[2] == "apt-get" {
			aptNonInteractive++
		}
		if step.Args[0] == "tee" {
			repo = string(step.Stdin)
		}
		for _, arg := range step.Args {
			if arg == "docker-compose-plugin" {
				installsCompose = true
			}
		}
	}
	if aptNonInteractive != 4 {
		t.Errorf("apt steps with DEBIAN_FRONTEND=noninteractive = %d, want 4", aptNonInteractive)
	}
	if !strings.Contains(repo, "download.docker.com/linux/ubuntu") || !strings.Contains(repo, "noble") {
		t.Errorf("repository line = %q, want the ubuntu repository at codename noble", repo)
	}
	if !installsCompose {
		t.Error("no step installs docker-compose-plugin")
	}
}

func TestDockerInstallStepsUnknownOS(t *testing.T) {
	if steps := DockerInstallSteps(OS{ID: "centos", VersionID: "9"}); steps != nil {
		t.Errorf("DockerInstallSteps(centos) = %v, want nil", steps)
	}
}
