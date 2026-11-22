package activate

import (
	"errors"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Haze-dev1/Olympus/internal/recipe"
)

// MaxConfigBytes is the maximum size of a body used in a semantic diff.
const MaxConfigBytes = 4 << 20

var (
	configDigestPattern = regexp.MustCompile(`^sha256-[0-9a-f]{64}$`)
	errReadConfig       = errors.New("cannot read store config")
)

// ReadConfig returns a confined, digest-verified regular file from the store.
func (a *Activator) ReadConfig(hash, relative string, limit int, digest string) ([]byte, error) {
	if !configDigestPattern.MatchString(hash) || !configDigestPattern.MatchString(digest) ||
		!recipe.SafeRelative(relative) || filepath.Clean(relative) != relative ||
		strings.ContainsAny(relative, "\x00\r\n\x1b") || limit < 1 || limit > MaxConfigBytes {
		return nil, errReadConfig
	}
	body, err := readConfig(filepath.Join(a.root(), "store", hash, "files", relative), limit, digest)
	if err != nil {
		return nil, errReadConfig
	}
	return body, nil
}
