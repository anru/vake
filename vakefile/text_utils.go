package vakefile

import (
	"strings"
)

func isBreakWordRune(r rune) bool {
	switch {
	case r >= 'a' && r <= 'z':
		return false
	case r >= 'A' && r <= 'Z':
		return false
	case r == '_':
		return false
	}
	return true
}

func isWhiteSpace(r rune) bool {
	return strings.ContainsRune(" \t\r\n", r)
}
