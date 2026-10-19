package remote

import (
	"context"
	"fmt"
	"strings"
)

// OS is a supported server operating system, as read from /etc/os-release.
type OS struct {
	ID        string // "ubuntu" or "debian"
	VersionID string // "22.04", "24.04" or "12"
}

// codenames maps each supported OS to the apt repository codename used to
// install Docker. The map also is the allow-list: an OS not in it is rejected.
var codenames = map[OS]string{
	{ID: "ubuntu", VersionID: "22.04"}: "jammy",
	{ID: "ubuntu", VersionID: "24.04"}: "noble",
	{ID: "debian", VersionID: "12"}:    "bookworm",
}

const supportedList = "Ubuntu 22.04, Ubuntu 24.04 and Debian 12"

// ParseOSRelease parses the contents of /etc/os-release. It is pure: it reads
// no files. Only the supported systems are accepted; anything else is an error
// naming the supported list.
func ParseOSRelease(data []byte) (OS, error) {
	fields := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		fields[strings.TrimSpace(key)] = unquoteOSRelease(strings.TrimSpace(value))
	}
	os := OS{ID: strings.ToLower(fields["ID"]), VersionID: fields["VERSION_ID"]}
	if _, ok := codenames[os]; !ok {
		name := strings.TrimSpace(os.ID + " " + os.VersionID)
		if name == "" {
			name = "unknown"
		}
		return OS{}, fmt.Errorf("remote: unsupported OS %q: Olympus supports %s", name, supportedList)
	}
	return os, nil
}

// DetectOS reads /etc/os-release on the server and validates it with
// ParseOSRelease.
func (c *Client) DetectOS(ctx context.Context) (OS, error) {
	out, _, err := c.Run(ctx, []string{"cat", "/etc/os-release"}, nil)
	if err != nil {
		return OS{}, err
	}
	return ParseOSRelease(out)
}

// unquoteOSRelease removes the optional double quotes os-release uses and the
// backslash escapes it permits inside them.
func unquoteOSRelease(v string) string {
	if len(v) < 2 || v[0] != '"' || v[len(v)-1] != '"' {
		return v
	}
	var b strings.Builder
	for i := 1; i < len(v)-1; i++ {
		if v[i] == '\\' && i+1 < len(v)-1 {
			i++
		}
		b.WriteByte(v[i])
	}
	return b.String()
}

// Step is one server-preparation command: an argument list and optional
// standard input. It never holds a raw shell string (A2).
type Step struct {
	Args  []string
	Stdin []byte
}

// DockerInstallSteps returns the commands that add Docker's apt repository and
// install Docker Engine with the Compose v2 plugin for the given OS. It runs
// nothing; the caller decides when to execute the steps, wrapping them with
// Sudo as needed. o must come from ParseOSRelease; an unsupported OS returns
// nil.
func DockerInstallSteps(o OS) []Step {
	codename, ok := codenames[o]
	if !ok {
		return nil
	}
	base := "https://download.docker.com/linux/" + o.ID
	repo := "deb [signed-by=/etc/apt/keyrings/docker.asc] " + base + " " + codename + " stable\n"
	return []Step{
		{Args: []string{"apt-get", "update"}},
		{Args: []string{"apt-get", "install", "-y", "ca-certificates", "curl"}},
		{Args: []string{"install", "-m", "0755", "-d", "/etc/apt/keyrings"}},
		{Args: []string{"curl", "-fsSL", base + "/gpg", "-o", "/etc/apt/keyrings/docker.asc"}},
		{Args: []string{"chmod", "a+r", "/etc/apt/keyrings/docker.asc"}},
		// tee carries the repository line on stdin: there is no argument-list
		// command that writes a file, and a shell redirect would be a raw
		// string (A2).
		{Args: []string{"tee", "/etc/apt/sources.list.d/docker.list"}, Stdin: []byte(repo)},
		{Args: []string{"apt-get", "update"}},
		{Args: []string{
			"apt-get", "install", "-y", "docker-ce", "docker-ce-cli", "containerd.io",
			"docker-buildx-plugin", "docker-compose-plugin",
		}},
	}
}
