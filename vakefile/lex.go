package vakefile

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

type Pos int
type tokenType int
type lexResult uint8

const hSpace = " \t"
const vSpace = "\r\n"

const (
	lexError lexResult = iota
	lexOk
	lexPass
)

const (
	tokenError tokenType = iota
	tokenEOF
	tokenPipe
	tokenMacro
	tokenVariable
	tokenAtVariable
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
	tokenPathPattern
	tokenQuotedString
	tokenPercentFlag
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

// lexerFn represens algo which can parse defined lexem
type lexerFn func(*lexer) lexResult

// stateFn represents the state of the scanner as a function that returns the next state.
type stateFn func(*lexer) stateFn

// lexer holds the state of the scanner.
type lexer struct {
	// input state
	name  string // the name of the input; used only for error reports
	input string // the string being scanned

	// read state
	start     Pos  // start position of this item
	width     Pos  // width of last rune read from input
	nl        int  // 1 if read rune == '\n' otherwise 0
	lastRune  rune //
	wasBackup bool

	// processing state
	pos           Pos       // current position in the input
	line          int       // 1+number of newlines seen
	stators       []stateFn // stack of state fns, lets call it stator
	currentStator stateFn

	// result stream
	tokens chan token // channel of scanned items
}

func (l *lexer) readState() (Pos, int) {
	return l.pos, l.line
}

func (l *lexer) setReadState(pos Pos, line int) {
	l.pos = pos
	l.line = line
}

// next returns the next rune in the input.
func (l *lexer) next() rune {
	if l.wasBackup {
		l.wasBackup = false
		l.pos += l.width
		l.line += l.nl
		return l.lastRune
	}
	if int(l.pos) >= len(l.input) {
		l.width = 0
		l.lastRune = eof
		l.nl = 0
		return eof
	}
	r, w := utf8.DecodeRuneInString(l.input[l.pos:])
	l.lastRune = r
	l.width = Pos(w)
	l.pos += l.width
	l.nl = 0
	if r == '\n' {
		l.nl = 1
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
func (l *lexer) backup() lexResult {
	if l.wasBackup {
		panic("twice backup")
	}
	l.pos -= l.width
	l.line -= l.nl
	l.wasBackup = true
	return lexPass
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

func (l *lexer) eatIdentifier() string {
	start := l.pos
	for !isBreakIdentifierRune(l.next()) {
	}
	l.backup()
	return l.input[start:l.pos]
}

func (l *lexer) eatUntil(stopMasks []string) bool {
	start := l.pos
	for r := l.next(); true; r = l.next() {
		if r == eof {
			return l.pos > start
		}
		for _, stopMask := range stopMasks {
			if strings.HasSuffix(l.input[start:l.pos], stopMask) {
				// rollback for length of stopMask
				endPos := l.pos
				l.pos -= Pos(len(stopMask))
				l.line -= strings.Count(l.input[l.pos:endPos], "\n")
				return l.pos > start
			}
		}
	}
	return false
}

func (l *lexer) eatAnyOf(chars string) {
	for strings.ContainsRune(chars, l.next()) {
	}
	l.backup()
}

func (l *lexer) eatVarPrefix() rune {
	start, line := l.readState()
	r := l.next()
	if (r == '$' || r == '@') && l.next() == '(' {
		return r
	}
	l.setReadState(start, line)
	return 0
}

// i want to eat string which exactly equals to passed one
// it rollbacks if eating was unsuccessful
func (l *lexer) eat(s string) bool {
	start, line := l.readState()
	r := l.next()
	for i := 0; i < len(s); {
		stringRune, size := utf8.DecodeRuneInString(s[i:])
		i += size
		if r != stringRune {
			l.setReadState(start, line)
			return false
		}
		r = l.next()
	}
	l.backup()
	return true
}

func (l *lexer) eatTokenWord(m map[string]tokenType) (tokenType, bool) {
	start, line := l.readState()
	id := l.eatIdentifier()
	tok, ok := m[id]
	if !ok {
		l.setReadState(start, line)
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

func (l *lexer) pushCurrent(f stateFn) stateFn {
	l.stators = append(l.stators, l.currentStator)
	return f
}

// pop top stator form stators stack
func (l *lexer) popState() stateFn {
	if len(l.stators) > 0 {
		lastStator := l.stators[len(l.stators)-1]
		l.stators = l.stators[:len(l.stators)-1]
		return lastStator
	}
	return l.currentStator
}

// eat tokens by priority list while we can
func (l *lexer) lex(lexers []lexerFn) lexResult {
	overallLexRes := lexPass
out:
	for {
		l.eatAnyOf(hSpace)
		l.drop()
		if l.peek() == eof {
			return overallLexRes
		}

		for _, lexer := range lexers {
			lexRes := lexer(l)
			switch lexRes {
			case lexOk:
				overallLexRes = lexOk
				continue out
			case lexError:
				return lexError
			}
		}
		// ok, we are failed eat something more, stopping

		break
	}
	return overallLexRes
}

// run runs the state machine for the lexer.
func (l *lexer) run() {
	for state := lexInitial; state != nil; {
		l.currentStator = state
		state = state(l)
	}
	close(l.tokens)
}

func (l *lexer) errorf(message string, args ...interface{}) lexResult {
	l.tokens <- token{tokenError, l.pos, fmt.Sprintf(message, args...), l.line}
	return lexError
}

// ---- lexer functions

func lexMacro(l *lexer) lexResult {
	r := l.next()
	if r != '!' {
		return l.backup()
	}
	l.drop()

	if len(l.eatIdentifier()) == 0 {
		return l.errorf("Expected any valid identifier after ! symbol")
	}
	l.emit(tokenMacro)
	return lexOk
}

func lexQuotedString(l *lexer) lexResult {
	openRune := l.next()
	if openRune == '"' {
		l.drop()
		wasEscapeSymbol := false
		for {
			r := l.next()
			if r == eof || r == '\n' {
				return l.errorf("unterminated string literal %s", l.input[l.start:l.pos])
			}
			if wasEscapeSymbol {
				wasEscapeSymbol = false
				continue
			}
			if r == '\\' {
				wasEscapeSymbol = true
			}
			if r == openRune {
				// we done here
				break
			}
		}

		l.pos--
		l.emit(tokenQuotedString)
		l.pos++
		l.drop()

		return lexOk
	}
	return l.backup()
}

func lexVariable(l *lexer) lexResult {
	var r rune
	if r = l.eatVarPrefix(); r == 0 {
		return lexPass
	}
	var varToken tokenType
	switch r {
	case '@':
		varToken = tokenAtVariable
	case '$':
		varToken = tokenVariable
	default:
		return l.errorf("Unknown variable predicate %v", r)
	}

	l.drop()
	if len(l.eatIdentifier()) == 0 {
		return l.errorf("Expected identifier for variable declaration")
	}

	if nextRune := l.next(); nextRune != ')' {
		return l.errorf("Invalid variable declaration, expected ')', got %v", nextRune)
	}
	l.pos--
	l.emit(varToken)
	l.pos++
	l.drop()

	return lexOk
}

func lexPathPattern(l *lexer) lexResult {
	start := l.pos
	for r := l.next(); !isBreakPatternRune(r); r = l.next() {
	}
	l.backup()
	if l.pos > start {
		l.emit(tokenPathPattern)
		return lexOk
	}
	return lexPass
}

func lexPipe(l *lexer) lexResult {
	start, line := l.readState()
	r := l.next()

	if r != '|' {
		return l.backup()
	}
	if l.next() == '>' {
		l.emit(tokenPipe)
		return lexOk
	} else {
		l.setReadState(start, line)
		return lexPass
	}
}

func lexKeyword(l *lexer) lexResult {
	if keywordToken, ok := l.eatTokenWord(keywords); ok {
		l.emit(keywordToken)
		return lexOk
	}
	return lexPass
}

func lexStringUntil(stopStrings []string) lexerFn {
	return func(l *lexer) lexResult {
		if l.eatUntil(stopStrings) {
			l.emitTrimmed(tokenString)
			return lexOk
		}
		return lexPass
	}
}

func lexPercentFlag(l *lexer) lexResult {
	r := l.next()
	if r != '%' {
		return l.backup()
	}
	l.drop()
	r = l.next()
	if isValidFlag(r) {
		l.emit(tokenPercentFlag)
		return lexOk
	}
	return l.errorf("invalid flag %v", r)
}

// example: "foo/bar" $(foo) src/commin/*.css
var inputPatternLexers = []lexerFn{
	lexQuotedString,
	lexVariable,
	lexPathPattern,
}

// "asd" $(abc) %f <any until given list>
var commandLexers = []lexerFn{
	lexQuotedString,
	lexVariable,
	lexMacro,
	lexPercentFlag,
	lexStringUntil([]string{"|>", "\n", " ", "\t"}),
}

// ---- state (parsing) funnctions ----

func lexRuleDest(l *lexer) stateFn {
	l.lex([]lexerFn{lexPathPattern})
	return lexInitial
}

func lexRuleCommand(l *lexer) stateFn {
	l.lex(commandLexers)

	if lexPipe(l) != lexOk {
		l.errorf("Expected |>")
		return nil
	}
	return lexRuleDest
}

func lexRule(l *lexer) stateFn {
	// foreach src/commin.css $(foo) |> cat %f | node_modules/.bin/csso -o %o |> dest.css
	lexKeyword(l)
	l.lex(inputPatternLexers)

	r := l.peek()
	if r == '\n' || r == eof {
		return lexInitial
	}

	if lexPipe(l) != lexOk {
		l.errorf("Rule definition: expected |>, got %v", r)
		return nil
	}

	return lexRuleCommand
}

func lexMacroDef(l *lexer) stateFn {
	// !macro_name =
	lexMacro(l)
	l.eatAnyOf(hSpace)
	l.drop()
	if l.next() != '=' {
		l.errorf("expected '='")
		return nil
	}
	l.emit(tokenAssign)

	l.eatAnyOf(hSpace)
	l.drop()

	return lexRule
}

func lexInitial(l *lexer) stateFn {
	l.eatAnyOf(hSpace + vSpace)
	l.drop()
	r := l.peek()
	if r == '!' {
		// macro definition
		return lexMacroDef
	}

	return nil
}
