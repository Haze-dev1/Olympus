package remote

import "strings"

// quote returns s as a single POSIX shell word. Single quotes protect
// everything inside them; an embedded single quote closes the quotes, escapes
// the quote and reopens them.
func quote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// joinArgs quotes every argument and joins them with spaces. This is the only
// place the remote command string is built (A2).
func joinArgs(args []string) string {
	quoted := make([]string, len(args))
	for i, arg := range args {
		quoted[i] = quote(arg)
	}
	return strings.Join(quoted, " ")
}
