//go:build (!linux && !darwin) || (!amd64 && !arm64)

package generation

import (
	"fmt"
	"os"
)

func publishDirectory(_ *os.File, _, _ string, _ bool) error {
	return fmt.Errorf("atomic build publication requires Linux or macOS on amd64 or arm64")
}
