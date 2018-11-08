package vakefile

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/thoas/go-funk"
)

// Pos is current position in input
type Pos int
type tokenType int
type lexResult uint8

const hSpace = " \t"
const vSpace = "\r\n"
const anySpace = hSpace + vSpace

const (
	lexError lexResult = iota
	lexOk
	lexPass
	lexBreakStart
	lexBreakLabelBody
)

const (
	tokenError tokenType = iota
	tokenEOF
	tokenPipe
	tokenMacro
	tokenVariable
	tokenAtVariable
	tokenColon
	tokenComma
	// keywords
	tokenKeywordStart
	tokenKeywordForeach
	tokenKeywordIfeq
	tokenKeywordIfdef
	tokenKeywordIfndef
	tokenKeywordElse
	tokenKeywordEndif
	tokenKeywordIncludeRules
	tokenKeywordInclude
	tokenKeywordEnd
	//
	tokenAssign
	tokenPlusAssign
	tokenString
	tokenComment
	tokenShebang
	tokenPathPattern
	tokenQuotedString
	tokenLabel
	tokenIdentifier
)

var keywords = map[string]tokenType{
	"foreach":       tokenKeywordForeach,
	"ifeq":          tokenKeywordIfeq,
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
		return fmt.Sprintf("[Error]: '%s'", t.val)
	case len(t.val) > 10:
		return fmt.Sprintf("(%d, '%s...')", t.typ, t.val[:10])
	}
	return fmt.Sprintf("(%d, '%s')", t.typ, t.val)
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
	width    Pos  // width of last rune read from input
	nl       int  // 1 if read rune == '\n' otherwise 0
	lastRune rune //

	// processing state
	start     Pos // start position of this token
	pos       Pos // current position in the input
	line      int // 1+number of newlines seen
	wasBackup bool

	// result stream
	tokens           chan token // channel of scanned tokens
	isBuffering      int
	tokensBuffer     []token
	lastEmittedToken tokenType
}

func (l *lexer) readState() (Pos, int) {
	return l.pos, l.line
}

func (l *lexer) setReadState(pos Pos, line int) {
	l.wasBackup = false
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

func (l *lexer) backup() lexResult {
	if l.wasBackup {
		panic("twice backup")
	}
	l.pos -= l.width
	l.line -= l.nl
	l.wasBackup = true
	return lexPass
}

func (l *lexer) emitString(t tokenType, str string) lexResult {
	tok := token{t, l.start, str, l.line}
	if l.isBuffering != 0 {
		l.tokensBuffer = append(l.tokensBuffer, tok)
	} else {
		l.lastEmittedToken = t
		l.tokens <- tok
	}

	return lexOk
}

func (l *lexer) emit(t tokenType) lexResult {
	return l.emitString(t, l.input[l.start:l.pos])
}

func (l *lexer) emitTrimmed(t tokenType) lexResult {
	return l.emitString(t, strings.Trim(l.input[l.start:l.pos], " \t\r\n"))
}

func (l *lexer) bufferEmits(doBuffering bool) {
	if doBuffering {
		l.isBuffering++
	} else if l.isBuffering > 0 {
		l.isBuffering--
	}
}

func (l *lexer) flushBuffer() {
	for _, tok := range l.tokensBuffer {
		l.lastEmittedToken = tok.typ
		l.tokens <- tok
	}
	l.tokensBuffer = []token{}
}

// drop skips eaten runes
func (l *lexer) drop() {
	l.start = l.pos
}

func (l *lexer) eatIdentifier() string {
	start := l.pos
	for !isBreakIdentifierRune(l.next()) {
	}
	l.backup()
	return l.input[start:l.pos]
}

func (l *lexer) eatAnyOf(chars string) string {
	start := l.pos
	for strings.ContainsRune(chars, l.next()) {
	}
	l.backup()

	return l.input[start:l.pos]
}

func (l *lexer) _lexComments(drop bool) {
	l.eatAnyOf(hSpace)
	r := l.next()
	if r == '#' {
		l.drop()
		for r != '\n' && r != eof {
			r = l.next()
		}
		l.backup()
		if drop {
			l.drop()
		} else {
			l.emitTrimmed(tokenComment)
		}
		return
	}
	l.backup()
	l.drop()
}

func lexComments(l *lexer) lexResult {
	l._lexComments(false)
	return lexPass
}

func (l *lexer) dropComments() {
	l._lexComments(true)
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

func (l *lexer) eatOneOfTokenWord(keywords []string, m map[string]tokenType) (tokenType, bool) {
	start, line := l.readState()
	id := l.eatIdentifier()

	if funk.IndexOfString(keywords, id) == -1 {
		l.setReadState(start, line)
		return tokenError, false
	}

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

func (l *lexer) lexUntil(lexers, stopLexers []lexerFn, restToken tokenType) lexResult {
	var overallLexRes = lexPass
	startRestPos := l.pos
	var endResPos Pos = -1
	for {
		startAt := l.pos
		l.bufferEmits(true)
		lexRes := l.lexws(lexers, false)
		if lexRes != lexPass && overallLexRes != lexError {
			overallLexRes = lexRes
		}

		stopLexRes := l.lexws(stopLexers, false)

		l.bufferEmits(false)

		// if we read something
		if l.pos > startAt {
			if endResPos > startRestPos {
				l.emitString(restToken, l.input[startRestPos:endResPos])
				endResPos = -1
			}
		} else {
			if endResPos != l.pos {
				startRestPos = l.pos
			}
			r := l.next()
			if r == eof {
				if l.pos > startRestPos {
					l.emitString(restToken, l.input[startRestPos:l.pos])
				}
				l.flushBuffer()
				return overallLexRes
			}
			l.drop()
			endResPos = l.pos
		}
		l.flushBuffer()

		if stopLexRes == lexOk {
			return overallLexRes
		}
	}
}

func (l *lexer) lex(lexers []lexerFn) lexResult {
	return l.lexws(lexers, true)
}

// eat tokens by priority list while we can
func (l *lexer) lexws(lexers []lexerFn, eaths bool) lexResult {
	overallLexRes := lexPass
out:
	for {
		if eaths {
			l.eatAnyOf(hSpace)
			l.drop()
		}

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
			default:
				if lexRes > lexBreakStart {
					return lexRes
				}
			}
		}
		// ok, we are failed eat something more, stopping

		break
	}
	return overallLexRes
}

// run runs the state machine for the lexer.
func (l *lexer) run() {
	for state := stateInitial; state != nil; {
		state = state(l)
	}
	close(l.tokens)
}

func (l *lexer) errorf(message string, args ...interface{}) lexResult {
	l.tokens <- token{tokenError, l.pos, fmt.Sprintf(message, args...), l.line}
	return lexError
}

// ---- lexer functions

func lexColon(l *lexer) lexResult {
	r := l.next()
	if r != ':' {
		return l.backup()
	}
	return l.emit(tokenColon)
}

func lexMacro(l *lexer) lexResult {
	r := l.next()
	if r != '!' {
		return l.backup()
	}
	l.drop()

	if len(l.eatIdentifier()) == 0 {
		return l.errorf("Expected any valid identifier after ! symbol")
	}
	return l.emit(tokenMacro)
}

func lexIdentifier(l *lexer) lexResult {
	if len(l.eatIdentifier()) == 0 {
		return lexPass
	}
	return l.emit(tokenIdentifier)
}

func lexQuotedString(l *lexer) lexResult {
	start := l.pos
	openRune := l.next()
	if openRune == '"' {
		wasEscapeSymbol := false
		for {
			r := l.next()
			if r == eof || r == '\n' {
				return l.errorf("unterminated string literal %s", l.input[start:l.pos])
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

		return l.emit(tokenQuotedString)
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
		return l.emit(tokenPathPattern)
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
		return l.emit(tokenPipe)
	}

	l.setReadState(start, line)
	return lexPass
}

func lexWsPipe(l *lexer) lexResult {
	start, line := l.readState()
	l.eatAnyOf(hSpace)
	l.drop()
	lr := lexPipe(l)
	if lr == lexPass {
		l.setReadState(start, line)
	}
	return lr
}

func lexOneKeywordOf(kws []string) lexerFn {
	return func(l *lexer) lexResult {
		if keywordToken, ok := l.eatOneOfTokenWord(kws, keywords); ok {
			return l.emit(keywordToken)
		}
		return lexPass
	}
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
}

var commandStopLexers = []lexerFn{
	lexWsPipe,
}

func lexRuleDest(l *lexer) lexResult {
	return l.lex([]lexerFn{lexPathPattern})
}

func lexRuleCommand(l *lexer) lexResult {
	l.eatAnyOf(hSpace)
	l.drop()
	l.lexUntil(commandLexers, commandStopLexers, tokenString)

	if l.lastEmittedToken != tokenPipe {
		return l.errorf("Expected |>")
	}
	lexRuleDest(l)
	return lexOk
}

var lexForeach = lexOneKeywordOf([]string{"foreach"})

func lexRuleHead(l *lexer) lexResult {
	// foreach src/commin.css $(foo) |> cat %f | node_modules/.bin/csso -o %o |> dest.css
	// @TODO: optimize
	lexForeach(l)

	l.lex(inputPatternLexers)

	r := l.peek()
	if r == '\n' || r == eof {
		return lexOk
	}

	if lexPipe(l) != lexOk {
		return l.errorf("Rule definition: expected '|>', got '%v'", r)
	}

	lexRuleCommand(l)
	return lexOk
}

func lexMacroDef(l *lexer) lexResult {
	// !macro_name =
	if lexMacro(l) != lexOk {
		return lexPass
	}
	l.eatAnyOf(hSpace)
	l.drop()
	if l.next() != '=' {
		return l.errorf("expected '='")
	}
	l.emit(tokenAssign)

	l.eatAnyOf(hSpace)
	l.drop()

	lexRuleHead(l)
	return lexOk
}

func lexRuleDef(l *lexer) lexResult {
	if lexColon(l) != lexOk {
		return lexPass
	}
	l.eatAnyOf(hSpace)
	l.drop()

	lexRuleHead(l)
	return lexOk
}

func eatSpaces(l *lexer) lexResult {
	var start Pos
	for {
		start = l.pos
		l.eatAnyOf(vSpace)
		lexComments(l)
		if start == l.pos {
			break
		}
	}

	return lexPass
}

var ifExpLexers = []lexerFn{
	lexVariable,
	lexQuotedString,
	lexIdentifier,
}

func lexIfeq(l *lexer) lexResult {
	l.eatAnyOf(hSpace)
	l.drop()
	r := l.next()
	if r != '(' {
		return l.errorf("ifeq: expected '(', got '%v'", r)
	}
	if l.lex(ifExpLexers) == lexPass {
		return l.errorf("ifeq: empty left expression, put variable, quoted string or identifier")
	}
	l.eatAnyOf(hSpace)
	l.drop()

	r = l.next()
	if r != ',' {
		return l.errorf("ifeq: expected ',', got '%v'", r)
	}
	l.emit(tokenComma)
	if l.lex(ifExpLexers) == lexPass {
		return l.errorf("ifeq: empty right expression, put variable, quoted string or identifier")
	}

	l.eatAnyOf(hSpace)
	l.drop()
	r = l.next()
	if r != ')' {
		return l.errorf("ifeq: expected ')', got '%v'", r)
	}
	l.dropComments()

	return lexOk
}

// =,+=
// used after top level identifier
func lexOperators(l *lexer) lexResult {
	pos, line := l.readState()
	r := l.next()
	switch r {
	case '=':
		return l.emit(tokenAssign)
	case '+':
		if l.next() == '=' {
			return l.emit(tokenPlusAssign)
		}
	default:
		return lexPass
	}
	l.setReadState(pos, line)
	return lexPass
}

var labelDepsLexers = []lexerFn{
	lexIdentifier,
}

func lexLabelDeps(l *lexer) lexResult {
	l.lex(labelDepsLexers)
	r := l.next()
	if r != '\n' {
		return l.errorf("Label declaration: expected new line, got '%v'", r)
	}
	l.drop()
	return lexOk
}

func lexTopLevelIdentifier(l *lexer) lexResult {
	id := l.eatIdentifier()
	if len(id) == 0 {
		return lexPass
	}

	// check if read identifier is keyword
	if keywordToken, kwOk := keywords[id]; kwOk {
		l.emit(keywordToken)
		switch keywordToken {
		case tokenKeywordIfeq:
			return lexIfeq(l)
		}
		return lexOk
	}

	// check if read identifier is label
	r := l.next()
	if r == ':' {
		l.pos--
		l.emit(tokenLabel)
		l.pos++
		l.drop()
		if lexLabelDeps(l) == lexError {
			return lexError
		}
		return lexBreakLabelBody
	}
	l.backup()

	// otherwise probably our identifier is variable
	l.emit(tokenIdentifier)

	return lexOk
}

var topLevelLexers = []lexerFn{
	eatSpaces,
	lexMacroDef,
	lexRuleDef,
	lexTopLevelIdentifier,
}

func stateCodeBlock(l *lexer) stateFn {
	// TODO
	return stateInitial
}

func stateLabelBody(l *lexer) stateFn {
	r := l.next()
	if r == ':' {
		l.backup()
		// ok, it was label for rules
		return stateInitial
	}
	// for code blocks I want at least two spaces
	if r == ' ' {
		afterSpace := l.next()
		if afterSpace == ' ' {
			return stateCodeBlock
		}
		l.errorf("Expected at least two spaces for code blocks, got '%v' instead.", afterSpace)
		return nil
	}
	l.errorf("Unexpected symbol '%v' after label declaration. Expected code block or :-rule.", r)
	return nil
}

func stateInitial(l *lexer) stateFn {
	lexRes := l.lex(topLevelLexers)
	if lexRes == lexBreakLabelBody {
		return stateLabelBody
	}
	if l.pos == Pos(len(l.input)) {
		l.width = 0
		l.emit(tokenEOF)
	} else {
		l.errorf("Unparsed statements %s", textTrim(string(l.input[l.pos]), 25))
	}
	return nil
}
