package activate

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"time"
)

// LockInfo is lock.json: who holds the lock, for the next caller's error.
type LockInfo struct {
	PID     int       `json:"pid"`
	User    string    `json:"user"`
	Command string    `json:"command"`
	Started time.Time `json:"started"`
}

// LockedError reports that another run holds the server lock (exit code 4).
type LockedError struct{ Info LockInfo }

func (e *LockedError) Error() string {
	if e.Info.PID == 0 {
		return "activate: the server lock is held by another run"
	}
	return fmt.Sprintf("activate: the server lock is held by %s (pid %d) running %s since %s",
		e.Info.User, e.Info.PID, e.Info.Command, e.Info.Started.UTC().Format(time.RFC3339))
}

// lock is one held flock.
type lock struct {
	file *os.File
	info string
}

// lock takes the exclusive flock of TRD 8.2 and records the holder. A second
// caller fails immediately, naming the holder. The kernel releases the flock
// when the process exits, so a crash never leaves a stale lock.
func (a *Activator) lock(command string) (*lock, error) {
	f, err := os.OpenFile(filepath.Join(a.root(), "lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("activate: open lock: %w", err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return nil, &LockedError{Info: readLockInfo(a.lockInfoPath())}
	}
	info := LockInfo{PID: os.Getpid(), User: userName(), Command: command, Started: a.clock().Now().UTC()}
	if err := writeJSON(a.lockInfoPath(), info, 0o600); err != nil {
		f.Close()
		return nil, err
	}
	return &lock{file: f, info: a.lockInfoPath()}, nil
}

// Close releases the flock and removes the holder description.
func (l *lock) Close() {
	os.Remove(l.info)
	l.file.Close()
}

func (a *Activator) lockInfoPath() string { return filepath.Join(a.root(), "lock.json") }

// readLockInfo returns the holder, or the zero value when it cannot be read.
func readLockInfo(path string) LockInfo {
	var info LockInfo
	if err := readJSON(path, &info); err != nil {
		return LockInfo{}
	}
	return info
}

// userName names the holder for lock.json.
func userName() string {
	if u := os.Getenv("USER"); u != "" {
		return u
	}
	return strconv.Itoa(os.Getuid())
}
