package vakefile

import (
	"strings"
	"unicode/utf8"
)

func isBreakIdentifierRune(r rune) bool {
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

const allowedPatternChars = "/*%.:-"

func isBreakPatternRune(r rune) bool {
	switch {
	case !isBreakIdentifierRune(r):
		return false
	case strings.ContainsRune(allowedPatternChars, r):
		return false
	}
	return true
}

const validFlags = "foObBedg%"

func isValidFlag(r rune) bool {
	if r > utf8.RuneSelf {
		return false
	}
	return strings.ContainsRune(validFlags, r)
}
