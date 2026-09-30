package core

import "strings"

// QuoteShellArg returns one POSIX-shell-safe argument. adb shell joins command
// arguments into a remote shell command without escaping them, so any
// user-controlled value embedded in that command must be quoted explicitly.
func QuoteShellArg(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
