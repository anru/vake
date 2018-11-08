package vakefile

import (
	"fmt"
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

func textTrim(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return fmt.Sprintf("%s..", s[:maxLen])
}
