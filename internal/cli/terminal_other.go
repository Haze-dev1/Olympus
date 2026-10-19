//go:build !linux && !darwin

package cli

func stdinTerminal() bool { return false }
