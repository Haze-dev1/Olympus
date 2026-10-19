// Package activate is the server-side activator. It brings one generation up
// under a lock, waits for the operator to confirm the server is still
// reachable, and re-activates the previous generation otherwise. Every Compose
// command is scoped to the "olympus" project (A11) and built from an argument
// list (A2); secret values only ever travel in files (A1).
package activate

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Haze-dev1/Olympus/internal/generation"
	"github.com/Haze-dev1/Olympus/internal/schema"
	"github.com/Haze-dev1/Olympus/internal/stack"
)

const (
	// Project is the only Docker Compose project the activator touches (A11).
	Project = "olympus"
	// DefaultRoot is where the activator keeps its state on the server.
	DefaultRoot = "/var/lib/olympus"
	// DefaultConfirmTimeout is how long the activator waits for Confirm.
	DefaultConfirmTimeout = 60 * time.Second
	// DefaultPollInterval is how often the activator looks for a confirmation.
	DefaultPollInterval = 100 * time.Millisecond
	// DefaultRetryInterval is how often TRD 8.5 polls a health check.
	DefaultRetryInterval = 2 * time.Second
	// DefaultHealthTimeout is the per-tool health timeout from TRD 8.5.
	DefaultHealthTimeout = 120 * time.Second
	// DefaultKeep is how many generations `gc` keeps by default.
	DefaultKeep = 5
)

// Status values in state.json.
const (
	StatusAwaiting   = "awaiting-confirm"
	StatusConfirmed  = "confirmed"
	StatusRolledBack = "rolled-back"
	// StatusRollbackFailed means the new generation failed and the server
	// could not be returned to the previous one.
	StatusRollbackFailed = "rollback_failed"
)

// Clock is the activator's view of time, so tests never wait.
type Clock interface {
	Now() time.Time
	After(d time.Duration) <-chan time.Time
}

// Runner runs Docker Compose for the "olympus" project. Implementations build
// argument lists, never shell strings (A2).
type Runner interface {
	Pull(ctx context.Context, composeFile string) error
	Up(ctx context.Context, composeFile string) error
	Down(ctx context.Context, composeFile string) error
	Exec(ctx context.Context, composeFile, tool string, args []string) error
}

// HealthChecker runs one tool's readiness check.
type HealthChecker interface {
	Check(ctx context.Context, composeFile, tool string, spec Health) error
}

// HealthSource returns the health checks for one generation, keyed by tool
// name. A tool with no entry is not checked.
type HealthSource interface {
	Health(id string) (map[string]Health, error)
}

// Health is one tool's readiness check: an HTTP GET from the host, so images
// need no curl, or a command inside the tool's container (TRD 8.5). Exactly
// one of HTTP and Command is set.
type Health struct {
	HTTP    *HTTPCheck
	Command []string
	// Timeout is how long to keep retrying. Zero means DefaultHealthTimeout.
	Timeout time.Duration
}

// HTTPCheck is a request to 127.0.0.1:<Port><Path> that must answer 2xx.
type HTTPCheck struct {
	Port int
	Path string
}

// validate rejects a check that cannot be run.
func (h Health) validate() error {
	switch {
	case h.HTTP != nil && len(h.Command) > 0:
		return errors.New("health check sets both http and command")
	case h.HTTP != nil:
		if h.HTTP.Port < 1 || h.HTTP.Port > 65535 {
			return fmt.Errorf("health check port %d is out of range", h.HTTP.Port)
		}
		if h.HTTP.Path != "" && !strings.HasPrefix(h.HTTP.Path, "/") {
			return fmt.Errorf("health check path %q does not start with /", h.HTTP.Path)
		}
	case len(h.Command) > 0:
		for _, arg := range h.Command {
			if arg == "" {
				return errors.New("health check command has an empty argument")
			}
		}
	default:
		return errors.New("health check sets neither http nor command")
	}
	if h.Timeout < 0 {
		return fmt.Errorf("health check timeout %s is negative", h.Timeout)
	}
	return nil
}

// State is state.json: the activation the server is waiting on.
type State struct {
	Version  int       `json:"version"`
	ID       string    `json:"id"`
	Status   string    `json:"status"`
	Nonce    string    `json:"nonce,omitempty"`
	Deadline time.Time `json:"deadline,omitempty"`
}

// pending is the confirmation file Confirm writes and Activate waits for.
type pending struct {
	Version int    `json:"version"`
	ID      string `json:"id"`
	Nonce   string `json:"nonce"`
}

// ErrNoPrevious reports that there is nothing to roll back to (exit code 1).
var ErrNoPrevious = errors.New("activate: no previous generation")

// RolledBackError reports that activation failed after the server was touched
// and is back on the previous generation (exit code 3).
type RolledBackError struct {
	Cause error
}

func (e *RolledBackError) Error() string {
	return "activate: the server rolled back: " + e.Cause.Error()
}

func (e *RolledBackError) Unwrap() error { return e.Cause }

// RollbackFailedError reports that a generation failed and the server could
// not be returned to the previous one: it may be down or only partly up. The
// generation that failed is in state.json with status rollback_failed.
type RollbackFailedError struct {
	Activation error // why the new generation failed
	Rollback   error // why returning to the previous one failed
}

func (e *RollbackFailedError) Error() string {
	return fmt.Sprintf("activate: the server could not roll back, %v; rollback failed: %v", e.Activation, e.Rollback)
}

// Unwrap exposes both causes.
func (e *RollbackFailedError) Unwrap() []error { return []error{e.Activation, e.Rollback} }

// Activator performs one activation, rollback or collection at a time under
// Root. Zero values fall back to the defaults; Runner and Health are required.
type Activator struct {
	Root           string
	Runner         Runner
	Checker        HealthChecker
	Health         HealthSource
	Clock          Clock
	Log            io.Writer // optional second log sink, for example stdout
	ConfirmTimeout time.Duration
	PollInterval   time.Duration
	RetryInterval  time.Duration

	mu sync.Mutex
	lg *logFile
}

// target is one generation resolved from the store.
type target struct {
	id       string
	dir      string
	manifest generation.Manifest
}

func (t target) composeFile() string { return filepath.Join(t.dir, "compose.yaml") }

var idPattern = regexp.MustCompile(`^sha256-[0-9a-f]{64}$`)

// Activate brings the generation up and, if it stays healthy, waits for
// Confirm before pointing current at it. Any failure after Compose ran
// re-activates the previous generation and returns a RolledBackError.
func (a *Activator) Activate(ctx context.Context, id string) error {
	if err := a.ensureRoot(); err != nil {
		return err
	}
	if a.Runner == nil {
		return errors.New("activate: no Compose runner")
	}
	lock, err := a.lock("activate " + id)
	if err != nil {
		return err
	}
	defer lock.Close()
	endLog, err := a.beginLog("activate")
	if err != nil {
		return err
	}
	defer endLog()

	t, err := a.load(id)
	if err != nil {
		return err
	}
	a.logf("activating generation %s", t.id)
	// A1/A5: secrets arrive in a root-only file and are gone before Compose runs.
	bk, err := a.installSecrets(t)
	if err != nil {
		return err
	}
	if err := a.start(ctx, t); err != nil {
		return a.revertOrDown(ctx, t, err, bk)
	}
	// A confirmation left behind by an earlier activation is never reused.
	if err := a.clearConfirm(); err != nil {
		return a.revertOrDown(ctx, t, err, bk)
	}
	nonce, err := newNonce()
	if err != nil {
		return err
	}
	state := State{Version: 1, ID: t.id, Status: StatusAwaiting, Nonce: nonce, Deadline: a.clock().Now().Add(a.confirmTimeout())}
	if err := a.writeState(state); err != nil {
		return a.revertOrDown(ctx, t, err, bk)
	}
	a.logf("generation %s is up; waiting for confirmation until %s", t.id, state.Deadline.UTC().Format(time.RFC3339))
	if err := a.waitConfirm(ctx, state); err != nil {
		return a.revertOrDown(ctx, t, err, bk)
	}
	n, err := a.publish(t.id)
	if err != nil {
		return a.revertOrDown(ctx, t, err, bk)
	}
	a.clearConfirm()
	a.discardSecrets(bk)
	a.finish(t.id, StatusConfirmed)
	a.logf("generation %s is live as generations/%d", t.id, n)
	return nil
}

// Confirm records that the operator reached the server over a new connection
// while this activation was waiting. It is a no-op error otherwise.
func (a *Activator) Confirm(id, nonce string) error {
	state, err := a.readState()
	if errors.Is(err, os.ErrNotExist) {
		return errors.New("activate: no activation is waiting for confirmation; the server has already rolled back")
	}
	if err != nil {
		return err
	}
	if state.Status != StatusAwaiting {
		return fmt.Errorf("activate: no activation is waiting for confirmation; %s is %s", state.ID, state.Status)
	}
	if state.ID != id || nonce == "" || state.Nonce != nonce {
		return fmt.Errorf("activate: confirmation does not match the waiting activation %s; the server has already rolled back or this confirmation is stale", state.ID)
	}
	if err := writeJSON(a.pendingPath(), pending{Version: 1, ID: id, Nonce: nonce}, 0o600); err != nil {
		return err
	}
	a.logf("generation %s was confirmed", id)
	return nil
}

// Rollback re-activates the generation before current and points current at
// it, with the same health checks and the same confirm-or-revert protection.
func (a *Activator) Rollback(ctx context.Context) error {
	if err := a.ensureRoot(); err != nil {
		return err
	}
	if a.Runner == nil {
		return errors.New("activate: no Compose runner")
	}
	lock, err := a.lock("rollback")
	if err != nil {
		return err
	}
	defer lock.Close()
	endLog, err := a.beginLog("rollback")
	if err != nil {
		return err
	}
	defer endLog()

	cur, n, ok, err := a.current()
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("%w: no generation is current", ErrNoPrevious)
	}
	prev, pn, ok, err := a.previous(n)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("%w: %s is the first generation", ErrNoPrevious, cur.id)
	}
	a.logf("rolling back from %s to %s", cur.id, prev.id)
	if err := a.start(ctx, prev); err != nil {
		return a.revertOrDown(ctx, prev, err, nil)
	}
	// A confirmation left behind by an earlier activation is never reused.
	if err := a.clearConfirm(); err != nil {
		return a.revertOrDown(ctx, prev, err, nil)
	}
	nonce, err := newNonce()
	if err != nil {
		return err
	}
	state := State{Version: 1, ID: prev.id, Status: StatusAwaiting, Nonce: nonce, Deadline: a.clock().Now().Add(a.confirmTimeout())}
	if err := a.writeState(state); err != nil {
		return a.revertOrDown(ctx, prev, err, nil)
	}
	a.logf("generation %s is up; waiting for confirmation until %s", prev.id, state.Deadline.UTC().Format(time.RFC3339))
	if err := a.waitConfirm(ctx, state); err != nil {
		return a.revertOrDown(ctx, prev, err, nil)
	}
	if err := a.pointCurrent(pn); err != nil {
		return a.revertOrDown(ctx, prev, err, nil)
	}
	a.clearConfirm()
	a.finish(prev.id, StatusConfirmed)
	a.logf("generation %s is live as generations/%d", prev.id, pn)
	return nil
}

// PayloadPath is where the CLI writes the secrets payload (TRD 8.4). The
// activator deletes it as soon as it has read it.
func (a *Activator) PayloadPath() string { return filepath.Join(a.root(), "secrets.incoming") }

// start pulls, brings the generation up and health-checks it.
func (a *Activator) start(ctx context.Context, t target) error {
	file := t.composeFile()
	if err := a.Runner.Pull(ctx, file); err != nil {
		return fmt.Errorf("pull images for %s: %w", t.id, err)
	}
	if err := a.Runner.Up(ctx, file); err != nil {
		return fmt.Errorf("start %s: %w", t.id, err)
	}
	return a.healthy(ctx, t)
}

// revertDown takes the project down when no generation was live before.
func (a *Activator) revertDown(ctx context.Context, failed target, cause error) error {
	a.logf("generation %s failed: %v", failed.id, cause)
	a.logf("no previous generation: taking the %s project down", Project)
	if err := a.Runner.Down(ctx, failed.composeFile()); err != nil {
		a.logf("taking the project down failed: %v", err)
		a.finish(failed.id, StatusRollbackFailed)
		return &RollbackFailedError{Activation: cause, Rollback: fmt.Errorf("taking the project down: %w", err)}
	}
	a.finish(failed.id, StatusRolledBack)
	return &RolledBackError{Cause: cause}
}

// revertOrDown restores the previous secrets, re-activates the generation
// current points at, or takes the project down when the failed generation was
// the first one. It reports RolledBackError only when that worked.
func (a *Activator) revertOrDown(ctx context.Context, failed target, cause error, bk *secretsBackup) error {
	a.restoreSecrets(bk)
	prev, _, ok, err := a.current()
	if err != nil {
		a.logf("reading current failed: %v", err)
		return a.revertDown(ctx, failed, cause)
	}
	if !ok {
		return a.revertDown(ctx, failed, cause)
	}
	a.logf("generation %s failed: %v; re-activating %s", failed.id, cause, prev.id)
	if err := a.start(ctx, prev); err != nil {
		a.logf("re-activating %s failed: %v", prev.id, err)
		a.finish(failed.id, StatusRollbackFailed)
		return &RollbackFailedError{Activation: cause, Rollback: err}
	}
	a.finish(failed.id, StatusRolledBack)
	return &RolledBackError{Cause: cause}
}

// healthy runs every declared health check in parallel, each with its own
// timeout, and reports all failures.
func (a *Activator) healthy(ctx context.Context, t target) error {
	if a.Health == nil {
		return errors.New("activate: no health source")
	}
	specs, err := a.Health.Health(t.id)
	if err != nil {
		return fmt.Errorf("health checks for %s: %w", t.id, err)
	}
	type result struct {
		tool string
		err  error
	}
	var pending int
	results := make(chan result, len(specs))
	for _, tool := range schema.Keys(t.manifest.Tools) {
		spec, ok := specs[tool]
		if !ok {
			continue
		}
		if err := spec.validate(); err != nil {
			return fmt.Errorf("tool %q: %w", tool, err)
		}
		if a.Checker == nil {
			return errors.New("activate: no health checker")
		}
		pending++
		go func(tool string, spec Health) {
			results <- result{tool, a.waitHealthy(ctx, t, tool, spec)}
		}(tool, spec)
	}
	var failed []result
	for i := 0; i < pending; i++ {
		if r := <-results; r.err != nil {
			failed = append(failed, r)
		}
	}
	if len(failed) == 0 {
		return nil
	}
	sort.Slice(failed, func(i, j int) bool { return failed[i].tool < failed[j].tool })
	names := make([]string, len(failed))
	for i, r := range failed {
		names[i] = r.tool
	}
	return fmt.Errorf("%d of %d health checks failed (%s): %w", len(failed), pending, strings.Join(names, ", "), failed[0].err)
}

// waitHealthy polls one tool every RetryInterval until it passes or its
// timeout expires.
func (a *Activator) waitHealthy(ctx context.Context, t target, tool string, spec Health) error {
	timeout := spec.Timeout
	if timeout == 0 {
		timeout = DefaultHealthTimeout
	}
	deadline := a.clock().Now().Add(timeout)
	for {
		err := a.Checker.Check(ctx, t.composeFile(), tool, spec)
		if err == nil {
			a.logf("tool %s is healthy", tool)
			return nil
		}
		if ctx.Err() != nil {
			return err
		}
		if !a.clock().Now().Before(deadline) {
			return fmt.Errorf("tool %s is not healthy after %s: %w", tool, timeout, err)
		}
		a.logf("tool %s is not healthy yet: %v", tool, err)
		select {
		case <-ctx.Done():
			return err
		case <-a.clock().After(a.retryInterval()):
		}
	}
}

// waitConfirm waits for a confirmation matching state, or gives up at its
// deadline.
func (a *Activator) waitConfirm(ctx context.Context, state State) error {
	for {
		p, err := a.readPending()
		if err == nil && p.ID == state.ID && p.Nonce == state.Nonce {
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if !a.clock().Now().Before(state.Deadline) {
			return fmt.Errorf("no confirmation for %s before the deadline at %s", state.ID, state.Deadline.UTC().Format(time.RFC3339))
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-a.clock().After(a.pollInterval()):
		}
	}
}

// load resolves a generation id inside the store. It is the trust boundary for
// values that arrived in a file: nothing may escape the store directory.
func (a *Activator) load(id string) (target, error) {
	if !idPattern.MatchString(id) {
		return target{}, fmt.Errorf("activate: invalid generation id %q", id)
	}
	dir := filepath.Join(a.root(), "store", "gen-"+id)
	data, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return target{}, fmt.Errorf("activate: read manifest of %s: %w", id, err)
	}
	var manifest generation.Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return target{}, fmt.Errorf("activate: parse manifest of %s: %w", id, err)
	}
	if manifest.Version != 1 {
		return target{}, fmt.Errorf("activate: manifest of %s has version %d", id, manifest.Version)
	}
	if manifest.ID != id {
		return target{}, fmt.Errorf("activate: manifest of %s calls itself %s", id, manifest.ID)
	}
	if _, err := os.Stat(filepath.Join(dir, "compose.yaml")); err != nil {
		return target{}, fmt.Errorf("activate: generation %s has no compose file: %w", id, err)
	}
	for _, tool := range schema.Keys(manifest.Tools) {
		if !stack.ValidName(tool) {
			return target{}, fmt.Errorf("activate: generation %s names the tool %q", id, tool)
		}
		for _, name := range manifest.Tools[tool].Secrets {
			if !safeName(name) {
				return target{}, fmt.Errorf("activate: tool %s declares the secret name %q", tool, name)
			}
		}
	}
	return target{id: id, dir: dir, manifest: manifest}, nil
}

// safeName reports whether a secret name can name a file inside its tool's
// secrets directory.
func safeName(name string) bool {
	return name != "" && name != "." && name != ".." && len(name) <= 128 &&
		!strings.ContainsAny(name, "/\\\x00")
}

// current resolves what the current link points at, and its number.
func (a *Activator) current() (target, int, bool, error) {
	dst, err := os.Readlink(a.currentLink())
	if errors.Is(err, os.ErrNotExist) {
		return target{}, 0, false, nil
	}
	if err != nil {
		return target{}, 0, false, fmt.Errorf("activate: read current: %w", err)
	}
	n, err := strconv.Atoi(filepath.Base(dst))
	if err != nil {
		return target{}, 0, false, fmt.Errorf("activate: current points at %q", dst)
	}
	t, ok, err := a.number(n)
	return t, n, ok, err
}

// previous resolves the newest generation numbered below n, and its number.
func (a *Activator) previous(n int) (target, int, bool, error) {
	numbers, err := a.generations()
	if err != nil {
		return target{}, 0, false, err
	}
	for _, candidate := range numbers { // newest first
		if candidate >= n {
			continue
		}
		t, ok, err := a.number(candidate)
		if err != nil {
			return target{}, 0, false, err
		}
		if !ok {
			return target{}, 0, false, fmt.Errorf("activate: generations/%d is missing its store entry", candidate)
		}
		return t, candidate, true, nil
	}
	return target{}, 0, false, nil
}

// number resolves generations/<n> to the generation it links to.
func (a *Activator) number(n int) (target, bool, error) {
	dst, err := filepath.EvalSymlinks(filepath.Join(a.generationsDir(), strconv.Itoa(n)))
	if errors.Is(err, os.ErrNotExist) {
		return target{}, false, nil
	}
	if err != nil {
		return target{}, false, fmt.Errorf("activate: resolve generations/%d: %w", n, err)
	}
	id := strings.TrimPrefix(filepath.Base(dst), "gen-")
	t, err := a.load(id)
	if err != nil {
		return target{}, false, err
	}
	return t, true, nil
}

// generations lists the numbers in the generations directory, newest first.
func (a *Activator) generations() ([]int, error) {
	entries, err := os.ReadDir(a.generationsDir())
	if err != nil {
		return nil, fmt.Errorf("activate: read generations: %w", err)
	}
	var numbers []int
	for _, e := range entries {
		if n, err := strconv.Atoi(e.Name()); err == nil {
			numbers = append(numbers, n)
		}
	}
	sort.Sort(sort.Reverse(sort.IntSlice(numbers)))
	return numbers, nil
}

// publish creates generations/<n> for the generation and points current at it.
func (a *Activator) publish(id string) (int, error) {
	numbers, err := a.generations()
	if err != nil {
		return 0, err
	}
	n := 1
	if len(numbers) > 0 {
		n = numbers[0] + 1
	}
	if err := os.Symlink(filepath.Join("..", "store", "gen-"+id), filepath.Join(a.generationsDir(), strconv.Itoa(n))); err != nil {
		return 0, fmt.Errorf("activate: link generations/%d: %w", n, err)
	}
	if err := a.pointCurrent(n); err != nil {
		return 0, err
	}
	return n, nil
}

// pointCurrent atomically points the current link at generations/<n>.
func (a *Activator) pointCurrent(n int) error {
	tmp := filepath.Join(a.root(), ".current.tmp")
	os.Remove(tmp)
	if err := os.Symlink(filepath.Join("generations", strconv.Itoa(n)), tmp); err != nil {
		return fmt.Errorf("activate: create current link: %w", err)
	}
	if err := os.Rename(tmp, a.currentLink()); err != nil {
		return fmt.Errorf("activate: replace current: %w", err)
	}
	return nil
}

// secretsBackup is the secrets an activation replaced. They live under
// secrets/<tool>.previous until the generation is confirmed.
type secretsBackup struct {
	tools []string
}

// installSecrets writes the secrets the generation's tools declare. The
// payload is deleted before anything is written, so no secret outlives the
// activation that needed it (A1, A5). The current values are copied aside
// first, so a revert can re-activate the previous generation with its own
// secrets rather than the new ones.
func (a *Activator) installSecrets(t target) (*secretsBackup, error) {
	path := a.PayloadPath()
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("activate: read secrets payload: %w", err)
	}
	if err := os.Remove(path); err != nil {
		return nil, fmt.Errorf("activate: delete secrets payload: %w", err)
	}
	var p payload
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&p); err != nil {
		return nil, fmt.Errorf("activate: parse secrets payload: %w", err)
	}
	if p.Version != 1 {
		return nil, fmt.Errorf("activate: secrets payload has version %d", p.Version)
	}
	// Validate the whole payload before replacing a single file.
	files := make(map[string]map[string][]byte, len(p.Tools))
	for _, tool := range schema.Keys(p.Tools) {
		declaredTool, ok := t.manifest.Tools[tool]
		if !ok {
			return nil, fmt.Errorf("activate: secrets payload names the unknown tool %q", tool)
		}
		declared := make(map[string]bool, len(declaredTool.Secrets))
		for _, name := range declaredTool.Secrets {
			declared[name] = true
		}
		ts := p.Tools[tool]
		content := make(map[string][]byte, len(ts.Secrets)+1)
		for _, name := range schema.Keys(ts.Secrets) {
			if !declared[name] || !safeName(name) {
				return nil, fmt.Errorf("activate: tool %q does not declare the secret %q", tool, name)
			}
			content[name] = []byte(ts.Secrets[name])
		}
		if len(ts.Environment) > 0 {
			env, err := environmentFile(ts.Environment)
			if err != nil {
				return nil, fmt.Errorf("activate: tool %s: %w", tool, err)
			}
			// The offline Compose file points env_file at this path.
			content["environment"] = env
		}
		if len(content) > 0 {
			files[tool] = content
		}
	}
	bk := &secretsBackup{}
	for _, tool := range schema.Keys(files) {
		if err := a.backupSecrets(tool); err != nil {
			return nil, err
		}
		bk.tools = append(bk.tools, tool)
		dir := a.secretsDir(tool)
		if err := writeSecretDir(dir); err != nil {
			return nil, err
		}
		for _, name := range schema.Keys(files[tool]) {
			if err := writeFile(filepath.Join(dir, name), files[tool][name], 0o600); err != nil {
				return nil, fmt.Errorf("activate: write secret %s/%s: %w", tool, name, err)
			}
		}
	}
	a.logf("installed secrets for %s", t.id)
	return bk, nil
}

// backupSecrets copies one tool's current secret files aside. An empty copy
// means the tool had no secrets before this activation.
func (a *Activator) backupSecrets(tool string) error {
	prev := a.previousSecretsDir(tool)
	if err := os.RemoveAll(prev); err != nil {
		return fmt.Errorf("activate: clear the previous secrets of %s: %w", tool, err)
	}
	if err := os.MkdirAll(prev, 0o700); err != nil {
		return fmt.Errorf("activate: keep the previous secrets of %s: %w", tool, err)
	}
	dir := a.secretsDir(tool)
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("activate: read the secrets of %s: %w", tool, err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !safeName(entry.Name()) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return fmt.Errorf("activate: keep the secret %s/%s: %w", tool, entry.Name(), err)
		}
		if err := writeFile(filepath.Join(prev, entry.Name()), data, 0o600); err != nil {
			return fmt.Errorf("activate: keep the secret %s/%s: %w", tool, entry.Name(), err)
		}
	}
	return nil
}

// restoreSecrets puts the values an activation replaced back in place before
// the previous generation is re-activated. It never fails the revert: a
// restore problem is logged, because bringing the services back matters more.
func (a *Activator) restoreSecrets(bk *secretsBackup) {
	if bk == nil {
		return
	}
	for _, tool := range bk.tools {
		if err := a.restoreToolSecrets(tool); err != nil {
			a.logf("restoring the secrets of %s failed: %v", tool, err)
		}
	}
}

func (a *Activator) restoreToolSecrets(tool string) error {
	prev := a.previousSecretsDir(tool)
	entries, err := os.ReadDir(prev)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	dir := a.secretsDir(tool)
	kept := make(map[string]bool, len(entries))
	for _, entry := range entries {
		kept[entry.Name()] = true
		data, err := os.ReadFile(filepath.Join(prev, entry.Name()))
		if err != nil {
			return err
		}
		if err := writeSecretDir(dir); err != nil {
			return err
		}
		if err := writeFile(filepath.Join(dir, entry.Name()), data, 0o600); err != nil {
			return err
		}
	}
	// Drop the files this activation added; the tool did not have them before.
	if current, err := os.ReadDir(dir); err == nil {
		for _, entry := range current {
			if entry.IsDir() || kept[entry.Name()] {
				continue
			}
			if err := os.Remove(filepath.Join(dir, entry.Name())); err != nil {
				return err
			}
		}
	}
	if len(entries) == 0 {
		if err := os.RemoveAll(dir); err != nil {
			return err
		}
	}
	return os.RemoveAll(prev)
}

// discardSecrets drops the copies once the new values are live.
func (a *Activator) discardSecrets(bk *secretsBackup) {
	if bk == nil {
		return
	}
	for _, tool := range bk.tools {
		if err := os.RemoveAll(a.previousSecretsDir(tool)); err != nil {
			a.logf("dropping the previous secrets of %s failed: %v", tool, err)
		}
	}
}

// environmentFile renders VAR=value lines, sorted so the file is stable. A
// value that spans lines would forge another entry, so it is rejected.
func environmentFile(env map[string]string) ([]byte, error) {
	var b bytes.Buffer
	for _, name := range schema.Keys(env) {
		if !envNamePattern.MatchString(name) {
			return nil, fmt.Errorf("invalid environment name %q", name)
		}
		if strings.ContainsAny(env[name], "\r\n\x00") {
			return nil, fmt.Errorf("the value of %s spans lines", name)
		}
		fmt.Fprintf(&b, "%s=%s\n", name, env[name])
	}
	return b.Bytes(), nil
}

// envNamePattern is the portable form of an environment variable name.
var envNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// payload is the file the CLI writes into PayloadPath: only the secrets each
// tool's recipe declares, never a value in an argument list (A1).
type payload struct {
	Version int                   `json:"version"`
	Tools   map[string]toolSecret `json:"tools"`
}

type toolSecret struct {
	Secrets     map[string]string `json:"secrets,omitempty"`
	Environment map[string]string `json:"environment,omitempty"`
}

// finish records the end of an activation in state.json.
func (a *Activator) finish(id, status string) {
	if err := a.writeState(State{Version: 1, ID: id, Status: status}); err != nil {
		a.logf("recording status %s failed: %v", status, err)
	}
}

func (a *Activator) writeState(state State) error {
	if err := writeJSON(a.statePath(), state, 0o600); err != nil {
		return fmt.Errorf("activate: write state: %w", err)
	}
	return nil
}

func (a *Activator) readState() (State, error) {
	var state State
	if err := readJSON(a.statePath(), &state); err != nil {
		return State{}, err
	}
	if state.Version != 1 {
		return State{}, fmt.Errorf("activate: state.json has version %d", state.Version)
	}
	return state, nil
}

// clearConfirm discards a confirmation from an earlier activation. It runs
// before state.json announces a new nonce, so a confirmation can only match
// the activation that is actually waiting.
func (a *Activator) clearConfirm() error {
	if err := os.Remove(a.pendingPath()); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("activate: discard stale confirmation: %w", err)
	}
	return nil
}

func (a *Activator) readPending() (pending, error) {
	var p pending
	if err := readJSON(a.pendingPath(), &p); err != nil {
		return pending{}, err
	}
	if p.Version != 1 {
		return pending{}, fmt.Errorf("activate: confirmation has version %d", p.Version)
	}
	return p, nil
}

// ensureRoot creates the layout of TRD 7.1 under the root, mode 0700.
func (a *Activator) ensureRoot() error {
	root := a.root()
	info, err := os.Lstat(root)
	switch {
	case errors.Is(err, os.ErrNotExist):
		if err := os.MkdirAll(root, 0o700); err != nil {
			return fmt.Errorf("activate: create root: %w", err)
		}
	case err != nil:
		return fmt.Errorf("activate: root: %w", err)
	case info.Mode()&os.ModeSymlink != 0:
		return fmt.Errorf("activate: root %s is a symlink", root)
	case !info.IsDir():
		return fmt.Errorf("activate: root %s is not a directory", root)
	}
	for _, name := range []string{"store", "generations", "secrets", "log"} {
		if err := os.MkdirAll(filepath.Join(root, name), 0o700); err != nil {
			return fmt.Errorf("activate: %w", err)
		}
	}
	return nil
}

func (a *Activator) root() string {
	if a.Root == "" {
		return DefaultRoot
	}
	return a.Root
}

func (a *Activator) clock() Clock {
	if a.Clock == nil {
		return systemClock{}
	}
	return a.Clock
}

func (a *Activator) confirmTimeout() time.Duration {
	if a.ConfirmTimeout <= 0 {
		return DefaultConfirmTimeout
	}
	return a.ConfirmTimeout
}

func (a *Activator) pollInterval() time.Duration {
	if a.PollInterval <= 0 {
		return DefaultPollInterval
	}
	return a.PollInterval
}

func (a *Activator) retryInterval() time.Duration {
	if a.RetryInterval <= 0 {
		return DefaultRetryInterval
	}
	return a.RetryInterval
}

func (a *Activator) generationsDir() string { return filepath.Join(a.root(), "generations") }
func (a *Activator) currentLink() string    { return filepath.Join(a.root(), "current") }
func (a *Activator) secretsDir(tool string) string {
	return filepath.Join(a.root(), "secrets", tool)
}

// previousSecretsDir holds one tool's secrets as they were before the current
// activation, until that activation is confirmed or reverted.
func (a *Activator) previousSecretsDir(tool string) string {
	return filepath.Join(a.root(), "secrets", tool+".previous")
}
func (a *Activator) statePath() string { return filepath.Join(a.root(), "state.json") }
func (a *Activator) pendingPath() string {
	return filepath.Join(a.root(), "confirm")
}

// newNonce is the random value that ties a confirmation to one activation.
func newNonce() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("activate: random nonce: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

// systemClock is the real clock.
type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

func (systemClock) After(d time.Duration) <-chan time.Time { return time.After(d) }

// writeSecretDir creates one tool's secrets directory.
func writeSecretDir(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("activate: create %s: %w", dir, err)
	}
	return nil
}

// writeFile atomically replaces path with data at mode.
func writeFile(path string, data []byte, mode os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+"-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return err
	}
	return nil
}

// writeJSON atomically replaces path with the canonical encoding: two-space
// indent, sorted keys and one trailing newline.
func writeJSON(path string, value any, mode os.FileMode) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return writeFile(path, append(data, '\n'), mode)
}

// readJSON parses a file, rejecting unknown fields and trailing data.
func readJSON(path string, value any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(value); err != nil {
		return fmt.Errorf("activate: parse %s: %w", path, err)
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return fmt.Errorf("activate: parse %s: trailing data", path)
	}
	return nil
}
