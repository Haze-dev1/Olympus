package recipe

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// sizeRE accepts an integer followed by an optional unit. Units are matched
// after upper-casing: B, KB, MB, GB, TB (decimal) and KiB, MiB, GiB, TiB
// (binary), with or without the trailing B.
var sizeRE = regexp.MustCompile(`^([0-9]+)([A-Z]*)$`)

var sizeUnits = map[string]uint64{
	"":    1,
	"B":   1,
	"K":   1000,
	"KB":  1000,
	"KI":  1 << 10,
	"KIB": 1 << 10,
	"M":   1000 * 1000,
	"MB":  1000 * 1000,
	"MI":  1 << 20,
	"MIB": 1 << 20,
	"G":   1000 * 1000 * 1000,
	"GB":  1000 * 1000 * 1000,
	"GI":  1 << 30,
	"GIB": 1 << 30,
	"T":   1000 * 1000 * 1000 * 1000,
	"TB":  1000 * 1000 * 1000 * 1000,
	"TI":  1 << 40,
	"TIB": 1 << 40,
}

// ParseSize reads a recipe size such as "4GB" or "512MiB" into bytes. It is
// deliberately strict: a value the preflight cannot compare is a recipe error,
// not a silently skipped check.
func ParseSize(s string) (uint64, error) {
	compact := strings.ReplaceAll(strings.ToUpper(strings.TrimSpace(s)), " ", "")
	m := sizeRE.FindStringSubmatch(compact)
	if m == nil {
		return 0, fmt.Errorf("invalid size %q", s)
	}
	unit, ok := sizeUnits[m[2]]
	if !ok {
		return 0, fmt.Errorf("invalid size unit in %q", s)
	}
	n, err := strconv.ParseUint(m[1], 10, 64)
	if err != nil || (unit > 1 && n > math.MaxUint64/unit) {
		return 0, fmt.Errorf("size %q is too large", s)
	}
	return n * unit, nil
}
