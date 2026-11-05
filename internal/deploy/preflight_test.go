package deploy

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

const (
	cmdOS     = "cat /etc/os-release"
	cmdUname  = "uname -m"
	cmdMemory = "cat /proc/meminfo"
	cmdDisk   = "df -kP /var/lib"
	cmdPorts  = "ss -ltnp"
	cmdDocker = "docker ps --filter label=com.docker.compose.project=olympus --format {{.Ports}}"
)

// healthyResponses is a supported server with enough memory and disk and a
// single unrelated listener on SSH.
func healthyResponses() map[string]string {
	return map[string]string{
		cmdOS:     "ID=ubuntu\nVERSION_ID=\"24.04\"\n",
		cmdUname:  "x86_64\n",
		cmdMemory: "MemTotal:        8388608 kB\nMemFree:         4194304 kB\n",
		cmdDisk:   "Filesystem 1024-blocks Used Available Capacity Mounted on\n/dev/vda1 10000000 4000000 6000000 40% /var\n",
		cmdPorts:  "State Recv-Q Send-Q Local Address:Port Peer Address:Port Process\nLISTEN 0 4096 127.0.0.1:22 0.0.0.0:* users:((\"sshd\",pid=1,fd=3))\n",
		cmdDocker: "",
	}
}

func preflightConn(t *testing.T, responses map[string]string, failing map[string]bool) *fakeConn {
	t.Helper()
	f := &fakeConn{}
	f.run = func(args []string, _ io.Reader) ([]byte, []byte, error) {
		command := strings.Join(args, " ")
		if failing[command] {
			return nil, nil, errors.New("failed")
		}
		out, ok := responses[command]
		if !ok {
			t.Errorf("unexpected remote command %q", command)
			return nil, nil, errors.New("unexpected")
		}
		return []byte(out), nil, nil
	}
	return f
}

func baseRequest() PreflightRequest {
	return PreflightRequest{
		Images:    map[string]string{"codex": "ghcr.io/acme/codex@sha256:" + strings.Repeat("a", 64)},
		Platforms: map[string][]string{"codex": {"linux/amd64"}},
		MinRAM:    map[string]uint64{"codex": 4_000_000_000},
	}
}

func TestPreflightPassesHealthyServer(t *testing.T) {
	f := preflightConn(t, healthyResponses(), nil)
	report, err := (&Client{Conn: f}).Preflight(context.Background(), baseRequest())
	if err != nil {
		t.Fatal(err)
	}
	if report.Arch != "amd64" || report.RAMBytes != 8<<30 || report.DiskBytes != 6_000_000*1024 {
		t.Fatalf("unexpected report %+v", report)
	}
	if len(report.Warnings) != 0 {
		t.Fatalf("unexpected warnings %v", report.Warnings)
	}
	want := []string{cmdOS, cmdUname, cmdMemory, cmdDisk}
	if len(f.calls) != len(want) {
		t.Fatalf("calls = %v, want %v", f.calls, want)
	}
	for i, args := range f.calls {
		if strings.Join(args, " ") != want[i] {
			t.Fatalf("call %d = %v, want %s", i, args, want[i])
		}
	}
}

func TestPreflightRejectsUnsupportedOS(t *testing.T) {
	responses := healthyResponses()
	responses[cmdOS] = "ID=fedora\nVERSION_ID=40\n"
	f := preflightConn(t, responses, nil)
	_, err := (&Client{Conn: f}).Preflight(context.Background(), baseRequest())
	if err == nil || !strings.Contains(err.Error(), "unsupported OS") || !strings.Contains(err.Error(), "Debian 12") {
		t.Fatalf("got %v", err)
	}
	if len(f.calls) != 1 {
		t.Fatalf("kept probing an unsupported system: %v", f.calls)
	}
}

func TestPreflightRejectsUnsupportedArchitecture(t *testing.T) {
	responses := healthyResponses()
	responses[cmdUname] = "riscv64\n"
	_, err := (&Client{Conn: preflightConn(t, responses, nil)}).Preflight(context.Background(), baseRequest())
	if err == nil || !strings.Contains(err.Error(), "unsupported server architecture") || !strings.Contains(err.Error(), "riscv64") {
		t.Fatalf("got %v", err)
	}
}

func TestPreflightChecksLockedPlatforms(t *testing.T) {
	responses := healthyResponses()
	responses[cmdUname] = "aarch64\n"

	req := baseRequest()
	req.Platforms["codex"] = []string{"linux/amd64"}
	_, err := (&Client{Conn: preflightConn(t, responses, nil)}).Preflight(context.Background(), req)
	if err == nil || !strings.Contains(err.Error(), "linux/arm64") || !strings.Contains(err.Error(), req.Images["codex"]) {
		t.Fatalf("got %v", err)
	}

	req.Platforms["codex"] = []string{"linux/amd64", "linux/arm64"}
	report, err := (&Client{Conn: preflightConn(t, responses, nil)}).Preflight(context.Background(), req)
	if err != nil || report.Arch != "arm64" {
		t.Fatalf("got %v, %+v", err, report)
	}
}

func TestPreflightWarnsWithoutPlatformRecord(t *testing.T) {
	req := baseRequest()
	req.Platforms["codex"] = nil
	report, err := (&Client{Conn: preflightConn(t, healthyResponses(), nil)}).Preflight(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Warnings) != 1 || !strings.Contains(report.Warnings[0], "olym update") {
		t.Fatalf("warnings = %v", report.Warnings)
	}
}

func TestPreflightWarnsForLowMemory(t *testing.T) {
	responses := healthyResponses()
	responses[cmdMemory] = "MemTotal:        2097152 kB\n"
	report, err := (&Client{Conn: preflightConn(t, responses, nil)}).Preflight(context.Background(), baseRequest())
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Warnings) != 1 || !strings.Contains(report.Warnings[0], "codex") || !strings.Contains(report.Warnings[0], "RAM") {
		t.Fatalf("warnings = %v", report.Warnings)
	}
}

func TestPreflightDiskThresholds(t *testing.T) {
	responses := healthyResponses()
	responses[cmdDisk] = "Filesystem 1024-blocks Used Available Capacity Mounted on\n/dev/vda1 10000000 9600000 400000 96% /var\n"
	_, err := (&Client{Conn: preflightConn(t, responses, nil)}).Preflight(context.Background(), baseRequest())
	if err == nil || !strings.Contains(err.Error(), "free on /var/lib") {
		t.Fatalf("got %v", err)
	}

	responses[cmdDisk] = "Filesystem 1024-blocks Used Available Capacity Mounted on\n/dev/vda1 10000000 7000000 3000000 70% /var\n"
	report, err := (&Client{Conn: preflightConn(t, responses, nil)}).Preflight(context.Background(), baseRequest())
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Warnings) != 1 || !strings.Contains(report.Warnings[0], "/var/lib") {
		t.Fatalf("warnings = %v", report.Warnings)
	}
}

func TestPreflightPortConflicts(t *testing.T) {
	responses := healthyResponses()
	responses[cmdPorts] = "State Recv-Q Send-Q Local Address:Port Peer Address:Port Process\n" +
		"LISTEN 0 4096 127.0.0.1:4096 0.0.0.0:* users:((\"nginx\",pid=42,fd=6))\n"
	req := baseRequest()
	req.Ports = []Port{{Tool: "codex", Number: 4096}}

	_, err := (&Client{Conn: preflightConn(t, responses, nil)}).Preflight(context.Background(), req)
	if err == nil || !strings.Contains(err.Error(), "port 4096") || !strings.Contains(err.Error(), "nginx") || !strings.Contains(err.Error(), "codex") {
		t.Fatalf("got %v", err)
	}

	responses[cmdDocker] = "127.0.0.1:4096->4096/tcp, [::1]:4096->4096/tcp\n"
	report, err := (&Client{Conn: preflightConn(t, responses, nil)}).Preflight(context.Background(), req)
	if err != nil || len(report.Warnings) != 0 {
		t.Fatalf("current generation conflicted with itself: %v %+v", err, report)
	}

	// A different project's container is not ours: its port is not in the
	// olympus label query, so the conflict stands.
	responses[cmdDocker] = "127.0.0.1:5000->5000/tcp\n"
	if _, err = (&Client{Conn: preflightConn(t, responses, nil)}).Preflight(context.Background(), req); err == nil {
		t.Fatal("a foreign container port was ignored")
	}
}

func TestPreflightSanitizesRemoteText(t *testing.T) {
	responses := healthyResponses()
	responses[cmdPorts] = "State Recv-Q Send-Q Local Address:Port Peer Address:Port Process\n" +
		"LISTEN 0 4096 127.0.0.1:4096 0.0.0.0:* users:((\"\x1b[31mbad\",pid=42,fd=6))\n"
	req := baseRequest()
	req.Ports = []Port{{Tool: "codex", Number: 4096}}
	_, err := (&Client{Conn: preflightConn(t, responses, nil)}).Preflight(context.Background(), req)
	if err == nil {
		t.Fatal("expected a port conflict")
	}
	if strings.ContainsAny(err.Error(), "\x1b\r") || strings.Contains(err.Error(), "\t") {
		t.Fatalf("remote text reached the message unsanitized: %q", err.Error())
	}
}

func TestPreflightPortBindAddress(t *testing.T) {
	header := "State Recv-Q Send-Q Local Address:Port Peer Address:Port Process\n"
	responses := healthyResponses()
	req := baseRequest()
	req.Ports = []Port{{Tool: "codex", Number: 4096}}

	// A listener on another specific address does not block loopback.
	responses[cmdPorts] = header + "LISTEN 0 4096 10.0.0.5:4096 0.0.0.0:* users:((\"api\",pid=9,fd=3))\n"
	if _, err := (&Client{Conn: preflightConn(t, responses, nil)}).Preflight(context.Background(), req); err != nil {
		t.Fatalf("non-loopback listener blocked loopback: %v", err)
	}

	// A public bind uses every address, so that listener does conflict.
	req.Ports[0].Public = true
	if _, err := (&Client{Conn: preflightConn(t, responses, nil)}).Preflight(context.Background(), req); err == nil {
		t.Fatal("public port ignored a listener on another address")
	}

	// A wildcard listener always conflicts with loopback.
	req.Ports[0].Public = false
	responses[cmdPorts] = header + "LISTEN 0 4096 [::]:4096 0.0.0.0:* users:((\"api\",pid=9,fd=3))\n"
	if _, err := (&Client{Conn: preflightConn(t, responses, nil)}).Preflight(context.Background(), req); err == nil {
		t.Fatal("wildcard listener did not conflict with loopback")
	}
}

func TestPreflightPublicPortConflict(t *testing.T) {
	responses := healthyResponses()
	responses[cmdPorts] = "State Recv-Q Send-Q Local Address:Port Peer Address:Port Process\n" +
		"LISTEN 0 4096 0.0.0.0:8080 0.0.0.0:* users:((\"caddy\",pid=7,fd=4))\n"
	req := baseRequest()
	req.Ports = []Port{{Tool: "codex", Number: 8080, Public: true}}
	if _, err := (&Client{Conn: preflightConn(t, responses, nil)}).Preflight(context.Background(), req); err == nil || !strings.Contains(err.Error(), "caddy") {
		t.Fatalf("got %v", err)
	}
}

func TestPreflightSkipsUnreadableMeasurements(t *testing.T) {
	for _, command := range []string{cmdMemory, cmdDisk, cmdPorts} {
		t.Run(command, func(t *testing.T) {
			failing := map[string]bool{command: true}
			req := baseRequest()
			req.Ports = []Port{{Tool: "codex", Number: 4096}}
			report, err := (&Client{Conn: preflightConn(t, healthyResponses(), failing)}).Preflight(context.Background(), req)
			if err != nil {
				t.Fatal(err)
			}
			if len(report.Warnings) != 1 {
				t.Fatalf("warnings = %v", report.Warnings)
			}
		})
	}
}
