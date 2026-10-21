package activate

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/Haze-dev1/Olympus/internal/encode"
	"github.com/Haze-dev1/Olympus/internal/schema"
)

// Compose is the production Runner: docker compose scoped to the "olympus"
// project (A11), every command built as an argument list (A2). No secret value
// is ever passed on a command line (A1).
type Compose struct {
	Bin string    // docker by default
	Log io.Writer // optional sink for command output
}

// Pull pulls the generation's images.
func (c Compose) Pull(ctx context.Context, composeFile string) error {
	return c.run(ctx, c.args(composeFile, "pull", "--ignore-buildable"))
}

// Build prepares missing content-addressed images. Keeping an existing image
// preserves its identity across no-op activation and rollback, including offline.
func (c Compose) Build(ctx context.Context, file string) error {
	data, err := os.ReadFile(file)
	if err != nil {
		return errors.New("activate: cannot read Compose build configuration")
	}
	doc, err := encode.Unmarshal("yaml", data)
	if err != nil {
		return errors.New("activate: invalid Compose build configuration")
	}
	services, _ := doc["services"].(map[string]any)
	seen := map[string]bool{}
	for _, name := range schema.Keys(services) {
		service, _ := services[name].(map[string]any)
		if service["build"] == nil {
			continue
		}
		image, _ := service["image"].(string)
		if seen[image] {
			continue
		}
		seen[image] = true
		if exec.CommandContext(ctx, c.bin(), "image", "inspect", image).Run() == nil {
			continue
		}
		if err := c.run(ctx, c.args(file, "build", "--build-arg", "SOURCE_DATE_EPOCH=0", name)); err != nil {
			return err
		}
	}
	return nil
}

// Up starts the generation's services, removing containers no longer declared.
func (c Compose) Up(ctx context.Context, composeFile string) error {
	return c.run(ctx, c.args(composeFile, "up", "-d", "--no-build", "--pull", "never", "--remove-orphans"))
}

// Down stops the project's containers. Volumes are left alone.
func (c Compose) Down(ctx context.Context, composeFile string) error {
	return c.run(ctx, c.args(composeFile, "down"))
}

// Exec runs a command inside one tool's container (A8).
func (c Compose) Exec(ctx context.Context, composeFile, tool string, args []string) error {
	return c.run(ctx, c.args(composeFile, append([]string{"exec", "-T", tool}, args...)...))
}

func (c Compose) args(composeFile string, rest ...string) []string {
	return append([]string{c.bin(), "compose", "-p", Project, "-f", composeFile}, rest...)
}

func (c Compose) bin() string {
	if c.Bin == "" {
		return "docker"
	}
	return c.Bin
}

// run executes one command and keeps the last of its output to explain a
// failure. Secrets never reach here: arguments are verbs, service names and
// paths, and no value is passed on stdin.
func (c Compose) run(ctx context.Context, args []string) error {
	var tail tailBuffer
	out := c.Log
	if out == nil {
		out = io.Discard
	}
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	// One writer for both streams: os/exec copies them in separate goroutines.
	sink := &syncWriter{w: io.MultiWriter(out, &tail)}
	cmd.Stdout = sink
	cmd.Stderr = sink
	if err := cmd.Run(); err != nil {
		if line := tail.lastLine(); line != "" {
			return fmt.Errorf("activate: docker %s: %w: %s", args[1], err, line)
		}
		return fmt.Errorf("activate: docker %s: %w", args[1], err)
	}
	return nil
}

// tailBuffer keeps the last 4 KiB written, so a failed command can report its
// last line without the caller holding a shell.
type tailBuffer struct {
	mu  sync.Mutex
	buf []byte
}

func (t *tailBuffer) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.buf = append(t.buf, p...)
	if len(t.buf) > 4096 {
		t.buf = t.buf[len(t.buf)-4096:]
	}
	return len(p), nil
}

// lastLine is the last non-empty line, trimmed and shortened for one log line.
func (t *tailBuffer) lastLine() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	lines := splitLines(string(t.buf))
	if len(lines) == 0 {
		return ""
	}
	line := lines[len(lines)-1]
	if len(line) > 200 {
		line = line[:200]
	}
	return line
}

// syncWriter serialises the two copying goroutines os/exec starts.
type syncWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (s *syncWriter) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.w.Write(p)
}

// splitLines returns the non-empty trimmed lines of s.
func splitLines(s string) []string {
	var lines []string
	for _, line := range strings.Split(s, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

// Checker is the production HealthChecker: an HTTP GET from the host, so an
// image needs no curl, or a command inside the tool's container (TRD 8.5).
type Checker struct {
	Runner Runner
	Client *http.Client
}

// Check runs one health check once; the activator retries it.
func (c Checker) Check(ctx context.Context, composeFile, tool string, spec Health) error {
	switch {
	case spec.HTTP != nil:
		return c.checkHTTP(ctx, spec.HTTP)
	case len(spec.Command) > 0:
		if c.Runner == nil {
			return errors.New("activate: no Compose runner for the health command")
		}
		if err := c.Runner.Exec(ctx, composeFile, tool, spec.Command); err != nil {
			return fmt.Errorf("health command in %s: %w", tool, err)
		}
		return nil
	}
	return errors.New("activate: health check sets neither http nor command")
}

func (c Checker) checkHTTP(ctx context.Context, check *HTTPCheck) error {
	path := check.Path
	if path == "" {
		path = "/"
	}
	url := fmt.Sprintf("http://127.0.0.1:%d%s", check.Port, path)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("GET %s: %w", url, err)
	}
	resp, err := c.client().Do(req)
	if err != nil {
		return fmt.Errorf("GET %s: %w", url, err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	return nil
}

func (c Checker) client() *http.Client {
	if c.Client != nil {
		return c.Client
	}
	return defaultClient
}

// defaultClient bounds one attempt; the activator owns the overall timeout.
var defaultClient = &http.Client{Timeout: 5 * time.Second}

// Recreate selects only services whose secret fingerprints changed.
func (c Compose) Recreate(ctx context.Context, file string, tools []string) error {
	return c.run(ctx, c.args(file, append([]string{"up", "-d", "--no-deps", "--force-recreate"}, tools...)...))
}
