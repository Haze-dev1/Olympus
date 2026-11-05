package deploy

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/Haze-dev1/Olympus/internal/activate"
	"github.com/Haze-dev1/Olympus/internal/remote"
	"github.com/Haze-dev1/Olympus/internal/schema"
)

const (
	// diskPath is the filesystem that will hold images, store entries and
	// volumes. It exists on every supported server, unlike /var/lib/olympus on
	// a first apply.
	diskPath = "/var/lib"
	// minDiskFree refuses a first image pull that cannot fit. warnDiskFree asks
	// the user to confirm a tight server; the edge-case table only requires a
	// confirmation for low memory.
	minDiskFree  = 1 << 30
	warnDiskFree = 5 << 30
)

// architectures maps the machine name reported by uname to the OCI
// architecture used in image platforms.
var architectures = map[string]string{
	"x86_64":  "amd64",
	"amd64":   "amd64",
	"aarch64": "arm64",
	"arm64":   "arm64",
}

// Port is one host port a tool wants to bind. A public port binds every
// address, so any listener on that number conflicts; a private port binds
// loopback only, so a listener on another specific address does not.
type Port struct {
	Tool   string
	Number int
	Public bool
}

// listener is one listening socket. address is the local address as ss
// printed it, including the port and IPv6 brackets.
type listener struct {
	address string
	who     string
}

// PreflightRequest is everything the client knows before touching the server.
// Values come from the stack file, recipes and stack.lock, never from the
// server.
type PreflightRequest struct {
	Images    map[string]string
	Platforms map[string][]string
	Ports     []Port
	MinRAM    map[string]uint64
}

// PreflightReport holds the measured server values and the warnings the plan
// prints. A nil error with warnings means the user must confirm before apply.
type PreflightReport struct {
	Arch      string
	RAMBytes  uint64
	DiskBytes uint64
	Warnings  []string
}

// PreflightError lists every failed check, so one run reports all of them.
type PreflightError struct{ Problems []string }

func (e *PreflightError) Error() string {
	return "preflight failed:\n  - " + strings.Join(e.Problems, "\n  - ")
}

// Preflight runs read-only checks over SSH before plan output and before any
// server mutation. Every command is an argument list; remote output is parsed
// as data and never included in an error (A1, A2).
func (c *Client) Preflight(ctx context.Context, req PreflightRequest) (PreflightReport, error) {
	var report PreflightReport
	var problems, warnings []string

	out, err := c.run(ctx, []string{"cat", "/etc/os-release"}, nil)
	if err != nil {
		return report, err
	}
	if _, err = remote.ParseOSRelease(out); err != nil {
		// Nothing else can be trusted on a system Olympus does not support.
		return report, &PreflightError{Problems: []string{err.Error()}}
	}

	arch := ""
	out, err = c.run(ctx, []string{"uname", "-m"}, nil)
	if err != nil {
		return report, err
	}
	machine := strings.TrimSpace(string(out))
	arch, supported := architectures[machine]
	report.Arch = arch
	if !supported {
		problems = append(problems, fmt.Sprintf("unsupported server architecture %q; Olympus supports amd64 and arm64", machine))
	}

	if supported {
		for _, name := range schema.Keys(req.Images) {
			platforms := req.Platforms[name]
			if len(platforms) == 0 {
				warnings = append(warnings, fmt.Sprintf("%s: stack.lock does not record the image platforms; run olym update to check linux/%s support", name, arch))
				continue
			}
			if !contains(platforms, "linux/"+arch) {
				problems = append(problems, fmt.Sprintf("%s: image %s has no linux/%s build (locked platforms: %s)", name, req.Images[name], arch, strings.Join(platforms, ", ")))
			}
		}
	}

	out, err = c.run(ctx, []string{"cat", "/proc/meminfo"}, nil)
	if err != nil {
		warnings = append(warnings, "cannot read server memory; skipped the minimum-memory check")
	} else if total, ok := parseMemTotal(out); ok {
		report.RAMBytes = total
		for _, name := range schema.Keys(req.MinRAM) {
			if need := req.MinRAM[name]; need > 0 && total < need {
				warnings = append(warnings, fmt.Sprintf("%s: recipe requires at least %s of RAM; the server has %s", name, humanBytes(need), humanBytes(total)))
			}
		}
	} else {
		warnings = append(warnings, "cannot parse server memory; skipped the minimum-memory check")
	}

	out, err = c.run(ctx, []string{"df", "-kP", diskPath}, nil)
	if err != nil {
		warnings = append(warnings, "cannot read free disk space; skipped the disk check")
	} else if free, ok := parseDF(out); ok {
		report.DiskBytes = free
		switch {
		case free < minDiskFree:
			problems = append(problems, fmt.Sprintf("only %s free on %s; images cannot be pulled until space is freed", humanBytes(free), diskPath))
		case free < warnDiskFree:
			warnings = append(warnings, fmt.Sprintf("%s free on %s; images and volumes may not fit", humanBytes(free), diskPath))
		}
	} else {
		warnings = append(warnings, "cannot parse free disk space; skipped the disk check")
	}

	if len(req.Ports) > 0 {
		listeners, err := c.listeners(ctx)
		if err != nil {
			warnings = append(warnings, "cannot list listening ports; skipped the port check")
		} else {
			owned := c.ownedPorts(ctx)
			for _, p := range req.Ports {
				if p.Number <= 0 || owned[p.Number] {
					continue
				}
				for _, l := range listeners[p.Number] {
					if p.Public || conflictsWithLoopback(l.address) {
						problems = append(problems, fmt.Sprintf("%s: port %d is already in use (%s)", p.Tool, p.Number, l.who))
						break
					}
				}
			}
		}
	}

	if len(problems) > 0 {
		return report, &PreflightError{Problems: problems}
	}
	report.Warnings = warnings
	return report, nil
}

// listeners groups listening TCP sockets by port.
func (c *Client) listeners(ctx context.Context) (map[int][]listener, error) {
	out, err := c.run(ctx, []string{"ss", "-ltnp"}, nil)
	if err != nil {
		return nil, err
	}
	ports := map[int][]listener{}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 || fields[0] != "LISTEN" {
			continue
		}
		n, ok := portFromAddress(fields[3])
		if !ok || n <= 0 {
			continue
		}
		who := "listening on " + printable(fields[3])
		if m := processRE.FindStringSubmatch(line); m != nil {
			who = fmt.Sprintf("process %s listening on %s", printable(m[1]), printable(fields[3]))
		}
		ports[n] = append(ports[n], listener{address: fields[3], who: who})
	}
	return ports, nil
}

// conflictsWithLoopback reports whether a socket on address would block a bind
// to 127.0.0.1. A specific non-loopback address does not.
func conflictsWithLoopback(address string) bool {
	host := address
	if i := strings.LastIndexByte(host, ':'); i >= 0 {
		host = host[:i]
	}
	host = strings.Trim(host, "[]")
	switch {
	case host == "", host == "*", host == "0.0.0.0", host == "::":
		return true
	case host == "localhost", host == "::1", strings.HasPrefix(host, "127."):
		return true
	}
	return false
}

// ownedPorts lists host ports published by the olympus Compose project, so an
// existing generation never conflicts with itself. A missing Docker daemon
// means no olympus containers can hold a port.
func (c *Client) ownedPorts(ctx context.Context) map[int]bool {
	out, err := c.run(ctx, []string{"docker", "ps", "--filter", "label=com.docker.compose.project=" + activate.Project, "--format", "{{.Ports}}"}, nil)
	if err != nil {
		return nil
	}
	owned := map[int]bool{}
	for _, line := range strings.Split(string(out), "\n") {
		for _, mapping := range strings.Split(line, ",") {
			host, _, ok := strings.Cut(strings.TrimSpace(mapping), "->")
			if !ok {
				continue
			}
			if n, ok := portFromAddress(host); ok {
				owned[n] = true
			}
		}
	}
	return owned
}

func portFromAddress(address string) (int, bool) {
	i := strings.LastIndexByte(address, ':')
	if i < 0 || i == len(address)-1 {
		return 0, false
	}
	n, err := strconv.Atoi(address[i+1:])
	return n, err == nil
}

func parseMemTotal(meminfo []byte) (uint64, bool) {
	for _, line := range strings.Split(string(meminfo), "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok || strings.TrimSpace(key) != "MemTotal" {
			continue
		}
		fields := strings.Fields(value)
		if len(fields) == 0 || (len(fields) > 1 && fields[1] != "kB") {
			return 0, false
		}
		kb, err := strconv.ParseUint(fields[0], 10, 64)
		if err != nil {
			return 0, false
		}
		return kb * 1024, true
	}
	return 0, false
}

// parseDF reads POSIX df -kP output: header plus filesystem, 1024-blocks,
// used, available, capacity and mount point.
func parseDF(out []byte) (uint64, bool) {
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) < 2 {
		return 0, false
	}
	fields := strings.Fields(lines[len(lines)-1])
	if len(fields) < 4 {
		return 0, false
	}
	available, err := strconv.ParseUint(fields[3], 10, 64)
	if err != nil {
		return 0, false
	}
	return available * 1024, true
}

// humanBytes formats bytes in decimal units, matching recipe sizes such as
// "4GB".
func humanBytes(n uint64) string {
	const unit = 1000
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	value := float64(n)
	for _, suffix := range []string{"kB", "MB", "GB", "TB"} {
		value /= unit
		if value < unit || suffix == "TB" {
			return fmt.Sprintf("%.1f %s", value, suffix)
		}
	}
	return ""
}

// printable keeps remote text from carrying terminal control sequences into a
// plan message.
func printable(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= 0x20 && r < 0x7f {
			return r
		}
		return '?'
	}, s)
}

func contains(values []string, want string) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}

var processRE = regexp.MustCompile(`\("([^"]+)"`)
