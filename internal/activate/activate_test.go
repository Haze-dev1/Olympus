package activate

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Haze-dev1/Olympus/internal/generation"
)

// fakeClock advances only when the test asks, so no test waits on real time.
type fakeClock struct {
	mu      sync.Mutex
	now     time.Time
	waiters []*fakeTimer
}

type fakeTimer struct {
	at time.Time
	ch chan time.Time
}

func newFakeClock() *fakeClock {
	return &fakeClock{now: time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)}
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) After(d time.Duration) <-chan time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	t := &fakeTimer{at: c.now.Add(d), ch: make(chan time.Time, 1)}
	c.waiters = append(c.waiters, t)
	return t.ch
}

// advance moves time forward and wakes every waiter that is now due.
func (c *fakeClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
	kept := c.waiters[:0]
	for _, t := range c.waiters {
		if !t.at.After(c.now) {
			t.ch <- c.now
			continue
		}
		kept = append(kept, t)
	}
	c.waiters = kept
}

// fakeRunner records Compose calls and fails the verbs a test names.
type fakeRunner struct {
	mu    sync.Mutex
	log   []string
	fail  map[string]error
	execs []string
}

func (r *fakeRunner) record(call string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.log = append(r.log, call)
	return r.fail[strings.Fields(call)[0]]
}

func (r *fakeRunner) Pull(_ context.Context, file string) error { return r.record("pull " + file) }
func (r *fakeRunner) Up(_ context.Context, file string) error   { return r.record("up " + file) }
func (r *fakeRunner) Down(_ context.Context, file string) error { return r.record("down " + file) }

func (r *fakeRunner) Exec(_ context.Context, file, tool string, args []string) error {
	r.mu.Lock()
	r.execs = append(r.execs, tool+" "+strings.Join(args, " "))
	r.mu.Unlock()
	return r.record("exec " + file)
}

func (r *fakeRunner) calls() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.log...)
}

func (r *fakeRunner) count(verb string) int {
	n := 0
	for _, call := range r.calls() {
		if strings.HasPrefix(call, verb+" ") {
			n++
		}
	}
	return n
}

// fakeChecker fails a tool while its compose file contains contains. A negative
// count fails forever; a positive one fails that many times.
type fakeChecker struct {
	mu       sync.Mutex
	contains string
	fail     map[string]int
	calls    []string
}

func (c *fakeChecker) Check(_ context.Context, file, tool string, _ Health) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls = append(c.calls, tool)
	if c.contains != "" && !strings.Contains(file, c.contains) {
		return nil
	}
	n := c.fail[tool]
	if n == 0 {
		return nil
	}
	if n > 0 {
		c.fail[tool] = n - 1
	}
	return fmt.Errorf("%s is not healthy", tool)
}

func (c *fakeChecker) count(tool string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := 0
	for _, name := range c.calls {
		if name == tool {
			n++
		}
	}
	return n
}

// fakeHealth answers HealthSource from a test map.
type fakeHealth map[string]map[string]Health

func (h fakeHealth) Health(id string) (map[string]Health, error) { return h[id], nil }

// safeBuffer is a writer the log goroutines and the test can share.
type safeBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *safeBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *safeBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

// fixture is one activator over a temporary root with offline fakes.
type fixture struct {
	*Activator
	clock   *fakeClock
	runner  *fakeRunner
	checker *fakeChecker
	health  fakeHealth
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	clock := newFakeClock()
	f := &fixture{
		clock:   clock,
		runner:  &fakeRunner{fail: map[string]error{}},
		checker: &fakeChecker{fail: map[string]int{}},
		health:  fakeHealth{},
	}
	f.Activator = &Activator{
		Root:    t.TempDir(),
		Clock:   clock,
		Log:     io.Discard,
		Runner:  f.runner,
		Checker: f.checker,
		Health:  f.health,
	}
	return f
}

// addGeneration writes one store entry as the CLI would have uploaded it.
func (f *fixture) addGeneration(t *testing.T, seed string, tools map[string]generation.Tool, compose string) string {
	t.Helper()
	id := generation.Hash([]byte(seed))
	dir := filepath.Join(f.Root, "store", "gen-"+id)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	manifest := generation.Manifest{
		Version: 1,
		ID:      id,
		Inputs:  map[string]string{"recipes": generation.Hash([]byte("recipes-" + seed))},
		Tools:   tools,
		Olym:    "0.1.0-dev",
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "compose.yaml"), []byte(compose), 0o600); err != nil {
		t.Fatal(err)
	}
	return id
}

// tool is one manifest tool entry.
func tool(secrets ...string) generation.Tool {
	return generation.Tool{
		Hash:    generation.Hash([]byte("tool")),
		Image:   "busybox@sha256:" + strings.Repeat("0", 64),
		Files:   map[string]string{},
		Secrets: secrets,
	}
}

// wait returns the operation's result, advancing the clock so polling loops
// make progress.
func (f *fixture) wait(t *testing.T, result <-chan error) error {
	t.Helper()
	for i := 0; i < 20000; i++ {
		select {
		case err := <-result:
			return err
		default:
		}
		f.clock.advance(time.Second)
		time.Sleep(100 * time.Microsecond)
	}
	t.Fatal("operation did not finish")
	return nil
}

// waitAwaiting returns state.json once the activation is waiting for a
// confirmation.
func (f *fixture) waitAwaiting(t *testing.T) State {
	t.Helper()
	for i := 0; i < 20000; i++ {
		if state, err := f.readState(); err == nil && state.Status == StatusAwaiting {
			return state
		}
		f.clock.advance(100 * time.Millisecond)
		time.Sleep(100 * time.Microsecond)
	}
	t.Fatal("activation never reached awaiting-confirm")
	return State{}
}

// activate runs one activation that is confirmed as soon as it is waiting.
func (f *fixture) activate(t *testing.T, id string) {
	t.Helper()
	result := make(chan error, 1)
	go func() { result <- f.Activate(context.Background(), id) }()
	state := f.waitAwaiting(t)
	if err := f.Confirm(id, state.Nonce); err != nil {
		t.Fatalf("confirm %s: %v", id, err)
	}
	if err := f.wait(t, result); err != nil {
		t.Fatalf("activate %s: %v", id, err)
	}
}

func (f *fixture) mustState(t *testing.T) State {
	t.Helper()
	state, err := f.readState()
	if err != nil {
		t.Fatal(err)
	}
	return state
}

func readlink(t *testing.T, path string) string {
	t.Helper()
	dst, err := os.Readlink(path)
	if err != nil {
		t.Fatal(err)
	}
	return dst
}

func exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

func TestActivateConfirmedBecomesCurrent(t *testing.T) {
	f := newFixture(t)
	id := f.addGeneration(t, "gen1", map[string]generation.Tool{"observatory": tool()}, "services: {}\n")
	f.health[id] = map[string]Health{"observatory": {HTTP: &HTTPCheck{Port: 8080, Path: "/healthz"}}}

	f.activate(t, id)

	if got := readlink(t, filepath.Join(f.Root, "current")); got != "generations/1" {
		t.Errorf("current points at %q, want generations/1", got)
	}
	if got, want := readlink(t, filepath.Join(f.Root, "generations", "1")), filepath.Join("..", "store", "gen-"+id); got != want {
		t.Errorf("generations/1 points at %q, want %q", got, want)
	}
	state := f.mustState(t)
	if state.Status != StatusConfirmed || state.ID != id {
		t.Errorf("state is %+v, want confirmed %s", state, id)
	}
	if state.Nonce != "" {
		t.Error("the nonce outlived the confirmation")
	}
	if f.runner.count("pull") != 1 || f.runner.count("up") != 1 {
		t.Errorf("compose calls are %v", f.runner.calls())
	}
	if exists(filepath.Join(f.Root, "confirm")) {
		t.Error("the confirmation file outlived the activation")
	}
}

func TestActivateDeadlineRollsBack(t *testing.T) {
	f := newFixture(t)
	id := f.addGeneration(t, "gen1", map[string]generation.Tool{"observatory": tool()}, "services: {}\n")

	result := make(chan error, 1)
	go func() { result <- f.Activate(context.Background(), id) }()
	state := f.waitAwaiting(t)
	if state.Nonce == "" {
		t.Fatal("no nonce was written")
	}

	err := f.wait(t, result)
	var rolled *RolledBackError
	if !errors.As(err, &rolled) {
		t.Fatalf("want a RolledBackError, got %v", err)
	}
	if !strings.Contains(err.Error(), "confirmation") {
		t.Errorf("error does not mention the missing confirmation: %v", err)
	}
	if exists(filepath.Join(f.Root, "current")) {
		t.Error("the first generation became current after a timeout")
	}
	if f.runner.count("down") != 1 {
		t.Errorf("the project was not taken down: %v", f.runner.calls())
	}
	if got := f.mustState(t).Status; got != StatusRolledBack {
		t.Errorf("state is %q, want %q", got, StatusRolledBack)
	}
}

func TestActivateHealthFailureRollsBack(t *testing.T) {
	f := newFixture(t)
	first := f.addGeneration(t, "gen1", map[string]generation.Tool{"observatory": tool()}, "services: {}\n")
	f.activate(t, first)

	second := f.addGeneration(t, "gen2", map[string]generation.Tool{"observatory": tool()}, "services: {}\n")
	f.checker.contains = "gen-" + second
	f.checker.fail["observatory"] = -1
	f.health[second] = map[string]Health{"observatory": {HTTP: &HTTPCheck{Port: 8080, Path: "/healthz"}}}

	err := f.wait(t, func() <-chan error {
		result := make(chan error, 1)
		go func() { result <- f.Activate(context.Background(), second) }()
		return result
	}())
	var rolled *RolledBackError
	if !errors.As(err, &rolled) {
		t.Fatalf("want a RolledBackError, got %v", err)
	}
	if !strings.Contains(err.Error(), "observatory") {
		t.Errorf("error does not name the failed tool: %v", err)
	}
	if got := readlink(t, filepath.Join(f.Root, "current")); got != "generations/1" {
		t.Errorf("current is %q, want generations/1", got)
	}
	if f.runner.count("up") != 3 { // first activation, the new generation, the re-activation
		t.Errorf("compose calls are %v", f.runner.calls())
	}
	if got := f.mustState(t).Status; got != StatusRolledBack {
		t.Errorf("state is %q, want %q", got, StatusRolledBack)
	}
}

func TestHealthCheckRetriesUntilItPasses(t *testing.T) {
	f := newFixture(t)
	id := f.addGeneration(t, "gen1", map[string]generation.Tool{"observatory": tool()}, "services: {}\n")
	f.checker.fail["observatory"] = 2
	f.health[id] = map[string]Health{"observatory": {HTTP: &HTTPCheck{Port: 8080, Path: "/"}}}

	f.activate(t, id)

	if n := f.checker.count("observatory"); n != 3 {
		t.Errorf("checked %d times, want 3", n)
	}
}

func TestFirstGenerationFailureLeavesNoCurrent(t *testing.T) {
	f := newFixture(t)
	id := f.addGeneration(t, "gen1", map[string]generation.Tool{"observatory": tool()}, "services: {}\n")
	f.checker.fail["observatory"] = -1
	f.health[id] = map[string]Health{"observatory": {Command: []string{"true"}}}

	err := f.wait(t, func() <-chan error {
		result := make(chan error, 1)
		go func() { result <- f.Activate(context.Background(), id) }()
		return result
	}())
	var rolled *RolledBackError
	if !errors.As(err, &rolled) {
		t.Fatalf("want a RolledBackError, got %v", err)
	}
	if f.runner.count("down") != 1 {
		t.Errorf("the project was not taken down: %v", f.runner.calls())
	}
	if exists(filepath.Join(f.Root, "current")) {
		t.Error("a current generation was left behind")
	}
	if exists(filepath.Join(f.Root, "generations", "1")) {
		t.Error("a generation link was left behind")
	}
	if got := f.mustState(t).Status; got != StatusRolledBack {
		t.Errorf("state is %q, want %q", got, StatusRolledBack)
	}
}

func TestSecondActivationFailsWithLockHolder(t *testing.T) {
	f := newFixture(t)
	first := f.addGeneration(t, "gen1", map[string]generation.Tool{"observatory": tool()}, "services: {}\n")
	second := f.addGeneration(t, "gen2", map[string]generation.Tool{"observatory": tool()}, "services: {}\n")

	result := make(chan error, 1)
	go func() { result <- f.Activate(context.Background(), first) }()
	state := f.waitAwaiting(t)

	err := f.Activate(context.Background(), second)
	var locked *LockedError
	if !errors.As(err, &locked) {
		t.Fatalf("want a LockedError, got %v", err)
	}
	if locked.Info.PID != os.Getpid() {
		t.Errorf("holder pid is %d, want %d", locked.Info.PID, os.Getpid())
	}
	if !strings.Contains(locked.Info.Command, first) {
		t.Errorf("holder command is %q, want the running activation", locked.Info.Command)
	}
	if !strings.Contains(err.Error(), "held by") || !strings.Contains(err.Error(), first) {
		t.Errorf("error does not name the holder: %v", err)
	}

	if err := f.Confirm(first, state.Nonce); err != nil {
		t.Fatal(err)
	}
	if err := f.wait(t, result); err != nil {
		t.Fatal(err)
	}
	if got := readlink(t, filepath.Join(f.Root, "current")); got != "generations/1" {
		t.Errorf("current is %q", got)
	}
}

func TestConfirmRejectsStaleOrWrongNonce(t *testing.T) {
	f := newFixture(t)
	id := f.addGeneration(t, "gen1", map[string]generation.Tool{"observatory": tool()}, "services: {}\n")

	if err := f.Confirm(id, "nothing-is-waiting"); err == nil || !strings.Contains(err.Error(), "already rolled back") {
		t.Errorf("confirm with no activation: %v", err)
	}

	result := make(chan error, 1)
	go func() { result <- f.Activate(context.Background(), id) }()
	state := f.waitAwaiting(t)

	if err := f.Confirm(id, "not-the-nonce"); err == nil || !strings.Contains(err.Error(), "stale") {
		t.Errorf("confirm with a wrong nonce: %v", err)
	}
	if err := f.Confirm(generation.Hash([]byte("other")), state.Nonce); err == nil {
		t.Error("confirm for the wrong generation was accepted")
	}
	if exists(filepath.Join(f.Root, "confirm")) {
		t.Error("a rejected confirmation was written")
	}

	if err := f.Confirm(id, state.Nonce); err != nil {
		t.Fatal(err)
	}
	if err := f.wait(t, result); err != nil {
		t.Fatal(err)
	}
	if err := f.Confirm(id, state.Nonce); err == nil {
		t.Error("a spent confirmation was accepted again")
	}
}

func TestRollbackWithoutPreviousGeneration(t *testing.T) {
	f := newFixture(t)
	id := f.addGeneration(t, "gen1", map[string]generation.Tool{"observatory": tool()}, "services: {}\n")
	f.activate(t, id)

	err := f.Rollback(context.Background())
	if !errors.Is(err, ErrNoPrevious) {
		t.Fatalf("want ErrNoPrevious, got %v", err)
	}
	if got := readlink(t, filepath.Join(f.Root, "current")); got != "generations/1" {
		t.Errorf("current moved to %q", got)
	}
}

func TestRollbackConfirmedSwitchesCurrent(t *testing.T) {
	f := newFixture(t)
	first := f.addGeneration(t, "gen1", map[string]generation.Tool{"observatory": tool()}, "services: {}\n")
	second := f.addGeneration(t, "gen2", map[string]generation.Tool{"observatory": tool()}, "services: {}\n")
	f.activate(t, first)
	f.activate(t, second)
	if got := readlink(t, filepath.Join(f.Root, "current")); got != "generations/2" {
		t.Fatalf("current is %q, want generations/2", got)
	}
	f.health[first] = map[string]Health{"observatory": {Command: []string{"true"}}}

	result := make(chan error, 1)
	go func() { result <- f.Rollback(context.Background()) }()
	state := f.waitAwaiting(t)
	if state.ID != first {
		t.Fatalf("rollback is waiting for %s, want %s", state.ID, first)
	}
	if err := f.Confirm(first, state.Nonce); err != nil {
		t.Fatal(err)
	}
	if err := f.wait(t, result); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if got := readlink(t, filepath.Join(f.Root, "current")); got != "generations/1" {
		t.Errorf("current is %q, want generations/1", got)
	}
	if got := f.mustState(t).Status; got != StatusConfirmed {
		t.Errorf("state is %q", got)
	}
}

func TestGCKeepsReferencedEntries(t *testing.T) {
	f := newFixture(t)
	first := f.addGeneration(t, "gen1", map[string]generation.Tool{"observatory": tool()},
		"services:\n  a:\n    volumes:\n      - "+f.Root+"/store/legacy/files:/opt/legacy:ro\n")
	second := f.addGeneration(t, "gen2", map[string]generation.Tool{"observatory": tool()}, "services: {}\n")
	third := f.addGeneration(t, "gen3", map[string]generation.Tool{"observatory": tool()}, "services: {}\n")
	f.activate(t, first)
	f.activate(t, second)
	f.activate(t, third)

	// An entry nothing references, and an upload in progress.
	for _, dir := range []string{
		filepath.Join(f.Root, "store", "orphan"),
		filepath.Join(f.Root, "store", "legacy", "files"),
		filepath.Join(f.Root, "store", ".incoming", "abc"),
	} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "data"), []byte("0123456789"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	result, err := f.GC(context.Background(), 2)
	if err != nil {
		t.Fatal(err)
	}

	want := []string{"gen-" + first, "legacy", "orphan"}
	sort.Strings(want)
	if strings.Join(result.Entries, ",") != strings.Join(want, ",") {
		t.Errorf("deleted %v, want %v", result.Entries, want)
	}
	if len(result.Generations) != 1 || result.Generations[0] != 1 {
		t.Errorf("deleted generations %v, want [1]", result.Generations)
	}
	if result.Freed <= 0 {
		t.Errorf("reported %d bytes freed", result.Freed)
	}

	for _, entry := range []string{"gen-" + second, "gen-" + third, ".incoming"} {
		if !exists(filepath.Join(f.Root, "store", entry)) {
			t.Errorf("store/%s was deleted", entry)
		}
	}
	for _, entry := range []string{"gen-" + first, "legacy", "orphan"} {
		if exists(filepath.Join(f.Root, "store", entry)) {
			t.Errorf("store/%s survived", entry)
		}
	}
	if exists(filepath.Join(f.Root, "generations", "1")) {
		t.Error("generations/1 survived")
	}
	if !exists(filepath.Join(f.Root, "generations", "2")) || !exists(filepath.Join(f.Root, "generations", "3")) {
		t.Error("a kept generation link was deleted")
	}
	if got := readlink(t, filepath.Join(f.Root, "current")); got != "generations/3" {
		t.Errorf("current is %q", got)
	}
}

func writePayload(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestSecretsAreWrittenAndPayloadDeleted(t *testing.T) {
	f := newFixture(t)
	id := f.addGeneration(t, "gen1", map[string]generation.Tool{"observatory": tool("model_key")}, "services: {}\n")
	writePayload(t, f.PayloadPath(), `{"version":1,"tools":{"observatory":{"secrets":{"model_key":"value-123"},"environment":{"MODEL_KEY":"value-123","OTHER":"x"}}}}`)

	f.activate(t, id)

	if exists(f.PayloadPath()) {
		t.Error("the payload survived the activation")
	}
	secret := filepath.Join(f.Root, "secrets", "observatory", "model_key")
	data, err := os.ReadFile(secret)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "value-123" {
		t.Errorf("secret is %q", data)
	}
	env, err := os.ReadFile(filepath.Join(f.Root, "secrets", "observatory", "environment"))
	if err != nil {
		t.Fatal(err)
	}
	if string(env) != "MODEL_KEY=value-123\nOTHER=x\n" {
		t.Errorf("environment is %q", env)
	}
	for _, path := range []string{secret, filepath.Join(f.Root, "secrets", "observatory", "environment")} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if perm := info.Mode().Perm(); perm != 0o600 {
			t.Errorf("%s is mode %o, want 600", path, perm)
		}
	}
}

func TestPayloadRejectsUndeclaredSecret(t *testing.T) {
	f := newFixture(t)
	id := f.addGeneration(t, "gen1", map[string]generation.Tool{"observatory": tool("model_key")}, "services: {}\n")
	writePayload(t, f.PayloadPath(), `{"version":1,"tools":{"observatory":{"secrets":{"telegram_token":"x"}}}}`)

	err := f.Activate(context.Background(), id)
	if err == nil || !strings.Contains(err.Error(), "does not declare") {
		t.Fatalf("want a declared-secret error, got %v", err)
	}
	if calls := f.runner.calls(); len(calls) != 0 {
		t.Errorf("compose ran before the payload was rejected: %v", calls)
	}
	if exists(f.PayloadPath()) {
		t.Error("the rejected payload survived")
	}
	if exists(filepath.Join(f.Root, "current")) {
		t.Error("a current generation was left behind")
	}
}

func TestNoSecretValueInLogsOrState(t *testing.T) {
	f := newFixture(t)
	const value = "sentinel-5f1c2b7a"
	id := f.addGeneration(t, "gen1", map[string]generation.Tool{"observatory": tool("model_key")}, "services: {}\n")
	writePayload(t, f.PayloadPath(), `{"version":1,"tools":{"observatory":{"secrets":{"model_key":"`+value+`"},"environment":{"MODEL_KEY":"`+value+`"}}}}`)

	f.activate(t, id)

	secrets := filepath.Join(f.Root, "secrets") + string(filepath.Separator)
	err := filepath.WalkDir(f.Root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || entry.Type()&fs.ModeSymlink != 0 || strings.HasPrefix(path, secrets) {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if bytes.Contains(data, []byte(value)) {
			t.Errorf("%s contains the secret value", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestComposeCommandsAreScopedAndListed(t *testing.T) {
	var out safeBuffer
	c := Compose{Bin: "/bin/echo", Log: &out}
	ctx := context.Background()
	if err := c.Pull(ctx, "/srv/compose.yaml"); err != nil {
		t.Fatal(err)
	}
	if err := c.Up(ctx, "/srv/compose.yaml"); err != nil {
		t.Fatal(err)
	}
	if err := c.Down(ctx, "/srv/compose.yaml"); err != nil {
		t.Fatal(err)
	}
	if err := c.Exec(ctx, "/srv/compose.yaml", "observatory", []string{"curl", "-fsS", "http://127.0.0.1:8080/healthz"}); err != nil {
		t.Fatal(err)
	}

	got := out.String()
	for _, want := range []string{
		"compose -p olympus -f /srv/compose.yaml pull",
		"compose -p olympus -f /srv/compose.yaml up -d --remove-orphans",
		"compose -p olympus -f /srv/compose.yaml down",
		"compose -p olympus -f /srv/compose.yaml exec -T observatory curl -fsS http://127.0.0.1:8080/healthz",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in %q", want, got)
		}
	}
	if strings.Contains(got, "sh -c") {
		t.Errorf("a shell was used: %q", got)
	}
}

func TestInvalidGenerationID(t *testing.T) {
	f := newFixture(t)
	for _, id := range []string{"", "gen1", "../../etc", "sha256-xyz"} {
		if err := f.Activate(context.Background(), id); err == nil || !strings.Contains(err.Error(), "invalid generation id") {
			t.Errorf("Activate(%q) = %v", id, err)
		}
	}
}

func TestManifestRejectsAnUnsafeSecretName(t *testing.T) {
	f := newFixture(t)
	id := f.addGeneration(t, "gen1", map[string]generation.Tool{"observatory": tool("../escape")}, "services: {}\n")

	err := f.Activate(context.Background(), id)
	if err == nil || !strings.Contains(err.Error(), "declares the secret name") {
		t.Fatalf("Activate = %v", err)
	}
	if exists(filepath.Join(f.Root, "current")) {
		t.Error("current appeared")
	}
}

func TestEnvironmentRejectsAnUnsafeName(t *testing.T) {
	f := newFixture(t)
	id := f.addGeneration(t, "gen1", map[string]generation.Tool{"observatory": tool("model_key")}, "services: {}\n")
	writePayload(t, f.PayloadPath(), `{"version":1,"tools":{"observatory":{"environment":{"BAD-NAME":"x"}}}}`)

	err := f.Activate(context.Background(), id)
	if err == nil || !strings.Contains(err.Error(), "tool observatory") {
		t.Fatalf("Activate = %v", err)
	}
	if got := f.runner.count("up"); got != 0 {
		t.Errorf("started the generation %d times", got)
	}
}

func TestEnvironmentRejectsANewline(t *testing.T) {
	f := newFixture(t)
	id := f.addGeneration(t, "gen1", map[string]generation.Tool{"observatory": tool("model_key")}, "services: {}\n")
	writePayload(t, f.PayloadPath(), `{"version":1,"tools":{"observatory":{"environment":{"OK":"line1\nline2"}}}}`)

	err := f.Activate(context.Background(), id)
	if err == nil || !strings.Contains(err.Error(), "tool observatory") {
		t.Fatalf("Activate = %v", err)
	}
	if exists(filepath.Join(f.Root, "secrets", "observatory", "environment")) {
		t.Error("the environment file was written")
	}
}

func TestGCNeverDeletesWhenAKeptGenerationIsUnreadable(t *testing.T) {
	f := newFixture(t)
	var ids []string
	for _, seed := range []string{"gen1", "gen2", "gen3"} {
		id := f.addGeneration(t, seed, map[string]generation.Tool{"observatory": tool()}, "services: {}\n")
		f.activate(t, id)
		ids = append(ids, id)
	}
	if err := os.Remove(filepath.Join(f.Root, "store", "gen-"+ids[1], "compose.yaml")); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(f.Root, "store", "orphan"), 0o700); err != nil {
		t.Fatal(err)
	}

	_, err := f.GC(context.Background(), 2)
	if err == nil || !strings.Contains(err.Error(), "refusing to collect") {
		t.Fatalf("GC = %v, want a refusal", err)
	}
	for _, entry := range []string{"gen-" + ids[0], "gen-" + ids[1], "gen-" + ids[2], "orphan"} {
		if !exists(filepath.Join(f.Root, "store", entry)) {
			t.Errorf("store/%s was deleted", entry)
		}
	}
}

func TestRevertReportsWhenRollbackItselfFails(t *testing.T) {
	f := newFixture(t)
	first := f.addGeneration(t, "gen1", map[string]generation.Tool{"observatory": tool()}, "services: {}\n")
	f.health[first] = map[string]Health{"observatory": {HTTP: &HTTPCheck{Port: 8080, Path: "/"}}}
	f.activate(t, first)

	second := f.addGeneration(t, "gen2", map[string]generation.Tool{"observatory": tool()}, "services: {}\n")
	f.health[second] = map[string]Health{"observatory": {HTTP: &HTTPCheck{Port: 8080, Path: "/"}}}
	// Every tool is unhealthy from here on, so the re-activation fails too.
	f.checker.fail["observatory"] = -1

	err := f.wait(t, func() <-chan error {
		result := make(chan error, 1)
		go func() { result <- f.Activate(context.Background(), second) }()
		return result
	}())
	var failed *RollbackFailedError
	if !errors.As(err, &failed) {
		t.Fatalf("want a RollbackFailedError, got %v", err)
	}
	var rolled *RolledBackError
	if errors.As(err, &rolled) {
		t.Errorf("a failed rollback was reported as a clean rollback: %v", err)
	}
	if failed.Activation == nil || failed.Rollback == nil {
		t.Errorf("both causes must be kept: %v", err)
	}
	if got := f.mustState(t).Status; got != StatusRollbackFailed {
		t.Errorf("state is %q, want %q", got, StatusRollbackFailed)
	}
}

func TestFailedActivationRestoresThePreviousSecrets(t *testing.T) {
	f := newFixture(t)
	tools := map[string]generation.Tool{"observatory": tool("model_key")}
	first := f.addGeneration(t, "gen1", tools, "services: {}\n")
	writePayload(t, f.PayloadPath(), `{"version":1,"tools":{"observatory":{"secrets":{"model_key":"old"},"environment":{"MODEL_KEY":"old"}}}}`)
	f.activate(t, first)

	second := f.addGeneration(t, "gen2", tools, "services: {}\n")
	f.checker.contains = "gen-" + second
	f.checker.fail["observatory"] = -1
	f.health[second] = map[string]Health{"observatory": {HTTP: &HTTPCheck{Port: 8080, Path: "/"}}}
	writePayload(t, f.PayloadPath(), `{"version":1,"tools":{"observatory":{"secrets":{"model_key":"new"},"environment":{"MODEL_KEY":"new"}}}}`)

	err := f.wait(t, func() <-chan error {
		result := make(chan error, 1)
		go func() { result <- f.Activate(context.Background(), second) }()
		return result
	}())
	var rolled *RolledBackError
	if !errors.As(err, &rolled) {
		t.Fatalf("want a RolledBackError, got %v", err)
	}
	secret, err := os.ReadFile(filepath.Join(f.Root, "secrets", "observatory", "model_key"))
	if err != nil {
		t.Fatal(err)
	}
	if string(secret) != "old" {
		t.Errorf("secret is %q after the revert, want the previous value", secret)
	}
	env, err := os.ReadFile(filepath.Join(f.Root, "secrets", "observatory", "environment"))
	if err != nil {
		t.Fatal(err)
	}
	if string(env) != "MODEL_KEY=old\n" {
		t.Errorf("environment is %q after the revert", env)
	}
	if exists(filepath.Join(f.Root, "secrets", "observatory.previous")) {
		t.Error("the copy survived the revert")
	}
}

func TestConfirmedActivationKeepsOnlyTheNewSecrets(t *testing.T) {
	f := newFixture(t)
	tools := map[string]generation.Tool{"observatory": tool("model_key")}
	first := f.addGeneration(t, "gen1", tools, "services: {}\n")
	writePayload(t, f.PayloadPath(), `{"version":1,"tools":{"observatory":{"secrets":{"model_key":"old"}}}}`)
	f.activate(t, first)

	second := f.addGeneration(t, "gen2", tools, "services: {}\n")
	writePayload(t, f.PayloadPath(), `{"version":1,"tools":{"observatory":{"secrets":{"model_key":"new"}}}}`)
	f.activate(t, second)

	secret, err := os.ReadFile(filepath.Join(f.Root, "secrets", "observatory", "model_key"))
	if err != nil {
		t.Fatal(err)
	}
	if string(secret) != "new" {
		t.Errorf("secret is %q, want the new value", secret)
	}
	if exists(filepath.Join(f.Root, "secrets", "observatory.previous")) {
		t.Error("a copy outlived the confirmation")
	}
}

func TestRollbackRecordsTheGenerationItTriedToActivate(t *testing.T) {
	f := newFixture(t)
	first := f.addGeneration(t, "gen1", map[string]generation.Tool{"observatory": tool()}, "services: {}\n")
	f.activate(t, first)
	second := f.addGeneration(t, "gen2", map[string]generation.Tool{"observatory": tool()}, "services: {}\n")
	f.activate(t, second)

	err := f.wait(t, func() <-chan error {
		result := make(chan error, 1)
		go func() { result <- f.Rollback(context.Background()) }()
		return result
	}())
	var rolled *RolledBackError
	if !errors.As(err, &rolled) {
		t.Fatalf("want a RolledBackError, got %v", err)
	}
	if got := f.mustState(t).ID; got != first {
		t.Errorf("state records %s, want the generation rollback tried to activate", got)
	}
	if got := f.mustState(t).Status; got != StatusRolledBack {
		t.Errorf("state is %q", got)
	}
	if got := readlink(t, filepath.Join(f.Root, "current")); got != "generations/2" {
		t.Errorf("current is %q, want generations/2", got)
	}
}

func TestGCKeepsManifestToolEntries(t *testing.T) {
	f := newFixture(t)
	entry := tool()
	entry.Files["/config"] = generation.Hash([]byte("config"))
	id := f.addGeneration(t, "kept", map[string]generation.Tool{"demo": entry}, "services: {}\n")
	f.activate(t, id)
	dir := filepath.Join(f.Root, "store", entry.Hash)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := f.GC(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	if !exists(dir) {
		t.Fatal("deleted referenced tool entry")
	}
}

func TestInstallSecretsRestoresEarlierToolsOnIOFailure(t *testing.T) {
	f := newFixture(t)
	id := f.addGeneration(t, "io-failure", map[string]generation.Tool{"alpha": tool("credential"), "beta": tool("credential")}, "services: {}\n")
	if err := f.ensureRoot(); err != nil {
		t.Fatal(err)
	}
	dir := f.secretsDir("alpha")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "credential"), []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	// Fingerprint validation succeeds, then backing up beta hits a real I/O
	// error after alpha has already been replaced. A dangling source works
	// even when tests run as root; chmod-only failures would not.
	if err := os.MkdirAll(f.secretsDir("beta"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("missing-credential", filepath.Join(f.secretsDir("beta"), "credential")); err != nil {
		t.Fatal(err)
	}
	writePayload(t, f.PayloadPath(), `{"version":1,"tools":{"alpha":{"secrets":{"credential":"new"}},"beta":{"secrets":{"credential":"new"}}}}`)
	if err := f.Activate(context.Background(), id); err == nil || !strings.Contains(err.Error(), "keep the secret beta/credential") {
		t.Fatalf("did not reach the second backup I/O failure: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "credential"))
	if err != nil || string(b) != "old" {
		t.Fatal("earlier tool secrets were not restored")
	}
	if f.runner.count("up") != 0 {
		t.Fatal("started Compose after secret install failed")
	}
	if exists(filepath.Join(dir, "fingerprints.json")) {
		t.Fatal("new fingerprint survived failed installation")
	}
}

func TestFingerprintsBindServerKeyNameAndValue(t *testing.T) {
	base := Fingerprints([]byte("server one"), map[string]string{"a": "sentinel"}, nil)
	for _, fp := range []map[string]string{Fingerprints([]byte("server two"), map[string]string{"a": "sentinel"}, nil), Fingerprints([]byte("server one"), map[string]string{"b": "sentinel"}, nil), Fingerprints([]byte("server one"), map[string]string{"a": "changed"}, nil)} {
		if fmt.Sprint(fp) == fmt.Sprint(base) {
			t.Fatal("fingerprint did not change")
		}
	}
}
