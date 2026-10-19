package cli

import (
	"golang.org/x/sys/unix"
	"os"
)

func stdinTerminal() bool {
	_, err := unix.IoctlGetTermios(int(os.Stdin.Fd()), unix.TCGETS)
	return err == nil
}
