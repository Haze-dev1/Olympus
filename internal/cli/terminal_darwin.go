package cli

import (
	"os"

	"golang.org/x/sys/unix"
)

func stdinTerminal() bool {
	_, err := unix.IoctlGetTermios(int(os.Stdin.Fd()), unix.TIOCGETA)
	return err == nil
}
