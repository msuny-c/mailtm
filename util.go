package mailtm

import "strings"

func trimRightSlash(s string) string {
	return strings.TrimRight(s, "/")
}
