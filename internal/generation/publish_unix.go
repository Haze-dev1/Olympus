//go:build (linux || darwin) && (amd64 || arm64)

package generation

import (
	"os"
	"runtime"
	"syscall"
	"unsafe"
)

// Linux renameat2(RENAME_EXCHANGE/NOREPLACE) and Darwin
// renameatx_np(RENAME_SWAP/EXCL) publish without a missing-directory window.
// The syscall numbers come from Linux's asm/unistd headers and XNU's
// bsd/kern/syscalls.master. Both exchange flags are 2; exclusive flags differ.
func publishDirectory(parent *os.File, stage, destination string, replace bool) error {
	trap, flags := uintptr(316), uintptr(1) // Linux amd64: renameat2, RENAME_NOREPLACE.
	if runtime.GOARCH == "arm64" {
		trap = 276
	}
	if runtime.GOOS == "darwin" {
		trap, flags = 488, 4
	} // renameatx_np, RENAME_EXCL.
	if replace {
		flags = 2
	}
	from, err := syscall.BytePtrFromString(stage)
	if err != nil {
		return err
	}
	to, err := syscall.BytePtrFromString(destination)
	if err != nil {
		return err
	}
	_, _, errno := syscall.Syscall6(trap, parent.Fd(), uintptr(unsafe.Pointer(from)), parent.Fd(), uintptr(unsafe.Pointer(to)), flags, 0)
	runtime.KeepAlive(from)
	runtime.KeepAlive(to)
	runtime.KeepAlive(parent)
	if errno != 0 {
		return errno
	}
	return nil
}
