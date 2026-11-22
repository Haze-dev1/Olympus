package eval

import (
	"fmt"

	"github.com/Haze-dev1/Olympus/internal/recipe"
)

const skillLedger = ".olympus-managed-files"

// pruneSkills removes only files recorded by a previous init. It deliberately
// leaves directories and unrecorded files alone, including agent-created skills.
func pruneSkills(dir string) (string, error) {
	if !recipe.ShellSafe(dir) {
		return "", fmt.Errorf("skill directory is not shell-safe")
	}
	// The volume is writable by the tool. Validate the old ledger and every
	// ancestor before deleting anything; a symlink must never redirect pruning.
	return fmt.Sprintf(`set -eu
root='%s'
ledger="$root/%s"
check_path() {
 current=''
 rest="$1"
 while [ -n "$rest" ]; do
  part=${rest%%%%/*}
  if [ "$part" = "$rest" ]; then rest=''; else rest=${rest#*/}; fi
  [ -n "$part" ] || continue
  case "$part" in .|..) exit 1;; esac
  current="$current/$part"
  [ ! -L "$current" ] || exit 1
 done
}
check_path "$ledger"
if [ -f "$ledger" ]; then
 while IFS= read -r rel || [ -n "$rel" ]; do
  case "$rel" in ''|/*|*[!A-Za-z0-9._@+/-]*|'%s') exit 1;; esac
  check_path "$root/$rel"
 done < "$ledger"
 while IFS= read -r rel || [ -n "$rel" ]; do
  rm -f -- "$root/$rel"
 done < "$ledger"
fi
`, dir, skillLedger, skillLedger), nil
}
