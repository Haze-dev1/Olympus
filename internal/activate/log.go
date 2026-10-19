package activate

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// logFile is one operation's log: <root>/log/<timestamp>-<command>.log (mode
// 0600, TRD 7.1), mirrored to the caller's writer when there is one.
type logFile struct {
	file *os.File
	out  io.Writer
}

// beginLog opens the log and returns the function that closes it.
func (a *Activator) beginLog(command string) (func(), error) {
	name := a.clock().Now().UTC().Format("20060102T150405Z") + "-" + command + ".log"
	f, err := os.OpenFile(filepath.Join(a.root(), "log", name), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, fmt.Errorf("activate: open log: %w", err)
	}
	a.mu.Lock()
	a.lg = &logFile{file: f, out: a.Log}
	a.mu.Unlock()
	return func() {
		a.mu.Lock()
		a.lg = nil
		a.mu.Unlock()
		f.Close()
	}, nil
}

// logf writes one line to the operation's log. It never formats secret values:
// callers pass tool names, generation ids and command output only (A1).
func (a *Activator) logf(format string, args ...any) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.lg == nil {
		return
	}
	line := fmt.Sprintf(format, args...)
	fmt.Fprintln(a.lg.file, line)
	if a.lg.out != nil {
		fmt.Fprintln(a.lg.out, line)
	}
}
