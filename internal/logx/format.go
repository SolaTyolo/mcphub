package logx

import "strings"

func Truncate(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

func JoinArgs(args []string) string {
	return strings.Join(args, " ")
}
