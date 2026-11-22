package activate

import (
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Haze-dev1/Olympus/internal/generation"
	"golang.org/x/sys/unix"
)

func readConfig(filename string, limit int, digest string) ([]byte, error) {
	root, err := unix.Open("/", unix.O_PATH|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	defer unix.Close(root)

	fd, err := unix.Openat2(root, strings.TrimPrefix(filename, "/"), &unix.OpenHow{
		Flags:   unix.O_PATH | unix.O_CLOEXEC,
		Resolve: unix.RESOLVE_BENEATH | unix.RESOLVE_NO_SYMLINKS,
	})
	if err != nil {
		return nil, err
	}
	defer unix.Close(fd)

	// Inspect the descriptor before opening it for reading so special files
	// cannot block or have side effects, even if the named path is replaced.
	var stat unix.Stat_t
	if err := unix.Fstat(fd, &stat); err != nil {
		return nil, err
	}
	if stat.Mode&unix.S_IFMT != unix.S_IFREG || stat.Size > int64(limit) {
		return nil, errReadConfig
	}
	file, err := os.Open(filepath.Join("/proc/self/fd", strconv.Itoa(fd)))
	if err != nil {
		return nil, err
	}
	defer file.Close()

	body, err := io.ReadAll(io.LimitReader(file, int64(limit)+1))
	if err != nil {
		return nil, err
	}
	if len(body) > limit || generation.Hash(body) != digest {
		return nil, errReadConfig
	}
	return body, nil
}
