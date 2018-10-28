package vakefile

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

type Pos int
type tokenType int

const (
	tokenError tokenType = iota
	tokenEOF
	tokenPipe
	tokenMacro
	// keywords
	tokenKeyword
	tokenKeywordForeach
	tokenKeywordIfdef
	tokenKeywordIfndef
	tokenKeywordElse
	tokenKeywordEndif
	tokenKeywordIncludeRules
	tokenKeywordInclude
	tokenKeywordEnd
	//
	tokenAssign
	tokenString
)

var keywords = map[string]tokenType{
	"foreach":       tokenKeywordForeach,
	"ifdef":         tokenKeywordIfdef,
	"ifndef":        tokenKeywordIfndef,
	"else":          tokenKeywordElse,
	"endif":         tokenKeywordEndif,
	"include_rules": tokenKeywordIncludeRules,
	"include":       tokenKeywordInclude,
}

const eof = -1

type token struct {
	typ  tokenType
	pos  Pos
	val  string
	line int
}

func (t token) String() string {
	switch {
	case t.typ == tokenEOF:
		return "!EOF"
	case t.typ == tokenError:
		return fmt.Sprintf("[Error]: %s", t.val)
	case len(t.val) > 10:
		return fmt.Sprintf("%s...", t.val[:10])
	}
	return fmt.Sprintf("%s", t.val)
}

// stateFn represents the state of the scanner as a function that returns the next state.
type stateFn func(*lexer) stateFn

// lexer holds the state of the scanner.
type lexer struct {
	// input state
	name  string // the name of the input; used only for error reports
	input string // the string being scanned

	// processing state
	pos   Pos // current position in the input
	start Pos // start position of this item
	width Pos // width of last rune read from input
	line  int // 1+number of newlines seen

	// result stream
	tokens chan token // channel of scanned items
}

// next returns the next rune in the input.
func (l *lexer) next() rune {
	if int(l.pos) >= len(l.input) {
		l.width = 0
		return eof
	}
	r, w := utf8.DecodeRuneInString(l.input[l.pos:])
	l.width = Pos(w)
	l.pos += l.width
	if r == '\n' {
		l.line++
	}
	return r
}

// peek returns but does not consume the next rune in the input.
func (l *lexer) peek() rune {
	r := l.next()
	l.backup()
	return r
}

// backup steps back one rune. Can only be called once per call of next.
func (l *lexer) backup() {
	l.pos -= l.width
	// Correct newline count.
	if l.width == 1 && l.input[l.pos] == '\n' {
		l.line--
	}
}

// emit passes an item back to the client.
func (l *lexer) emit(t tokenType) {
	l.tokens <- token{t, l.start, l.input[l.start:l.pos], l.line}

	l.drop()
}

func (l *lexer) emitTrimmed(t tokenType) {
	trimmed := strings.Trim(l.input[l.start:l.pos], " \t\r\n")
	l.tokens <- token{t, l.start, trimmed, l.line}

	l.drop()
}

// drop skips eaten runes
func (l *lexer) drop() {
	// l.line += strings.Count(l.input[l.start:l.pos], "\n")
	l.start = l.pos
}

// rollback reset pos to start position
func (l *lexer) rollback() {
	l.pos = l.start
}

func (l *lexer) eatWord() string {
	start := l.pos
	for !isBreakWordRune(l.next()) {
	}
	l.backup()
	return l.input[start:l.pos]
}

func (l *lexer) eatUntil(stopMasks []string) bool {
	start := l.pos
	for r := l.next(); true; r = l.next() {
		if r == eof {
			return true
		}
		for _, stopMask := range stopMasks {
			if strings.HasSuffix(l.input[start:l.pos], stopMask) {
				// rollback for length of stopMask
				l.pos -= Pos(len(stopMask))
				return true
			}
		}
	}
	return false
}

func (l *lexer) eatWhiteSpace() {
	for isWhiteSpace(l.next()) {
	}
	l.backup()
}

// i want to eat string which exactly equals to passed one
// it rollbacks if eating was unsuccessful
func (l *lexer) eat(s string) bool {
	start := l.pos
	r := l.next()
	for i := 0; i < len(s); {
		stringRune, size := utf8.DecodeRuneInString(s[i:])
		i += size
		if r != stringRune {
			l.pos = start
			return false
		}
		r = l.next()
	}
	l.backup()
	return true
}

func (l *lexer) eatTokenWord(m map[string]tokenType) (tokenType, bool) {
	start := l.pos
	word := l.eatWord()
	tok, ok := m[word]
	if !ok {
		l.pos = start
		return tokenError, false
	}
	return tok, true
}

func (l *lexer) nextToken() token {
	return <-l.tokens
}

func (l *lexer) drain() []token {
	var tokens []token
	for token := range l.tokens {
		tokens = append(tokens, token)
	}
	return tokens
}

// lex creates a new scanner for the input string.
func lex(name, input string) *lexer {
	l := &lexer{
		name:   name,
		input:  input,
		tokens: make(chan token),
		line:   1,
	}
	go l.run()
	return l
}

// run runs the state machine for the lexer.
func (l *lexer) run() {
	for state := lexInitial; state != nil; {
		state = state(l)
	}
	close(l.tokens)
}

func (l *lexer) errorf(message string, args ...interface{}) stateFn {
	l.tokens <- token{tokenError, l.pos, fmt.Sprintf(message, args...), l.line}
	return nil
}

// ---- state functions

func lexRule(l *lexer) stateFn {
	// foreach src/commin.css |> cat %f | node_modules/.bin/csso -o %o |> dest.css
	// tokens read priority:
	// newline: '\n'
	// eof: EOF ;
	// pipe : '|>'
	// keywords : A | B | C | D
	// string : a-z/%

	l.eatWhiteSpace()
	l.drop()
	r := l.peek()
	if r == '\n' || r == eof {
		return lexInitial
	}

	if l.eat("|>") {
		l.emit(tokenPipe)
	} else if keywordToken, ok := l.eatTokenWord(keywords); ok {
		l.emit(keywordToken)
	} else if l.eatUntil([]string{"|>", "\n"}) {
		l.emitTrimmed(tokenString)
	}

	return lexRule
}

func lexMacroDef(l *lexer) stateFn {
	// we already parsed ! symbol
	l.drop()
	l.eatWord()
	l.emit(tokenMacro)
	l.eatWhiteSpace()
	l.drop()
	if l.next() != '=' {
		return l.errorf("expected '='")
	}
	l.emit(tokenAssign)

	return lexRule
}

func lexInitial(l *lexer) stateFn {
	r := l.next()
	if r == '!' {
		// macro definition
		return lexMacroDef
	}

	return nil
}
