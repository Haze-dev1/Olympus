package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Haze-dev1/Olympus/internal/generation"
	"github.com/Haze-dev1/Olympus/internal/plan"
	"github.com/Haze-dev1/Olympus/internal/remote"
	"github.com/Haze-dev1/Olympus/internal/stack"
)

func selectKnownHosts(flag, env, home string) string {
	if flag != "" {
		return flag
	}
	if env != "" {
		return env
	}
	return filepath.Join(home, ".ssh", "known_hosts")
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

func connectionConfig(s *stack.Stack, known string, stderr io.Writer) (remote.Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintln(stderr, "cannot find home directory")
		return remote.Config{}, err
	}
	key := s.Server.SSHKey
	if strings.HasPrefix(key, "~/") {
		key = filepath.Join(home, key[2:])
	}
	port := s.Server.Port
	if port == 0 {
		port = 22
	}
	user := s.Server.User
	if user == "" {
		user = "root"
	}
	confirm := func(host, fingerprint string) bool {
		if !stdinTerminal() {
			return false
		}
		fmt.Fprintf(stderr, "Unknown host %s (%s). Trust this key? [y/N] ", host, fingerprint)
		answer, err := bufio.NewReader(os.Stdin).ReadString('\n')
		return err == nil && strings.EqualFold(strings.TrimSpace(answer), "y")
	}
	return remote.Config{Host: net.JoinHostPort(s.Server.Host, strconv.Itoa(port)), User: user, KeyFile: key, KnownHosts: selectKnownHosts(known, os.Getenv("OLYM_KNOWN_HOSTS"), home), Timeout: 15 * time.Second, Confirm: confirm}, nil
}
