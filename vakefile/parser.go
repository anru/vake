package vakefile

import (
	"fmt"
	"runtime"
)

type parseResult int

const (
	parseError = iota
	parseOk
	parsePass
)

const maxBufSize = 3

type ParserEnv struct {
	macros map[string]RuleNode
	vars   map[string]string
}

func (e *ParserEnv) hasMacro(name string) bool {
	_, ok := e.macros[name]
	return ok
}

type Parser struct {
	// input name and lexer
	name  string
	lexer *lexer

	// parsing environment
	env *ParserEnv

	// processing state
	ring        [maxBufSize]*token
	ringNext    int // next cell of the ring
	ringCurrent int // offset of bufHead
	ringLock    int // value we want to protect and keep ability to return to

	// output stream
	nodes chan Node
	errc  chan error
}

type parserStateFn func(*Parser) parserStateFn

func incmod(value, max int) int {
	if value++; value >= max {
		return 0
	}
	return value
}

func decmod(value, max int) int {
	if value--; value < 0 {
		return max - 1
	}
	return value
}

func (p *Parser) next() *token {
	if incmod(p.ringCurrent, maxBufSize) == p.ringNext {
		if p.ringNext == p.ringLock {
			panic("Parser buffer is full, probably you should increase buffer size or optimize parsing functions")
		}
		tok := p.lexer.nextToken()
		p.ring[p.ringNext] = &tok
		p.ringCurrent = p.ringNext
		p.ringNext = incmod(p.ringNext, maxBufSize)
	} else {
		p.ringCurrent = incmod(p.ringCurrent, maxBufSize)
	}

	return p.ring[p.ringCurrent]
}

func (p *Parser) back() parseResult {
	if p.ringCurrent == p.ringNext {
		panic("You can't back more")
	}
	p.ringCurrent = decmod(p.ringCurrent, maxBufSize)
	return parsePass
}

func (p *Parser) peek() *token {
	t := p.next()
	p.back()
	return t
}

func (p *Parser) keep() {
	p.ringLock = p.ringCurrent
}

func (p *Parser) restore() parseResult {
	p.ringCurrent = p.ringLock
	p.ringLock = -1
	return parsePass
}

func (p *Parser) recover() {
	if e := recover(); e != nil {
		if _, ok := e.(runtime.Error); ok {
			panic(e)
		}
		if p != nil {
			p.lexer.drain()
			p.errc <- e.(error)
		}
	}
}

func (p *Parser) run() {
	defer p.recover()
	for state := parseStateInitial; state != nil; {
		state = state(p)
	}
	close(p.nodes)
}

func (p *Parser) errorf(format string, a ...interface{}) {
	panic(fmt.Errorf(format, a...))
}

func (p *Parser) readTokensWhile(f func(*token) bool) []string {
	out := []string{}

	for t := p.next(); f(t); t = p.next() {
		out = append(out, t.val)
	}
	p.back()

	return out
}

func (p *Parser) expect(typ tokenType) *token {
	t := p.next()
	if t.typ != typ {
		p.errorf("expected token type %v got %v", typ, t)
	}

	return t
}

// :src/*.js |> !bundle_js |> app/bundle.js
// :foreach src/*.js |> !bundle_js |> app/%b
func parseRule(p *Parser) parseResult {
	t := p.next()
	if t.typ != tokenColon {
		return p.back()
	}
	// ok, create node now
	n := RuleNode{}
	t = p.peek()

	if t.typ == tokenKeywordForeach {
		n.Foreach = true
	}

	n.Inputs = p.readTokensWhile(func(t *token) bool {
		return t.typ == tokenPathPattern
	})

	if len(n.Inputs) == 0 {
		p.errorf("empty input for rule")
	}

	p.expect(tokenPipe)

	// now we expect (tokenMacro | [tokenVariable, tokenQuotedString, tokenString]+)

	var atLeastOneTokenForCommandEaten bool = false
CommandLoop:
	for {
		t = p.next()

		switch t.typ {
		case tokenMacro:
			macroRule, hasMacro := p.env.macros[t.val]
			if !hasMacro {
				p.errorf("Macro %s is not defined", t.val)
			}

			n.Command = macroRule.Command
			n.Output = macroRule.Output
			break CommandLoop
		case tokenVariable:
			variableValue, hasVariable := p.env.vars[t.val]
			if !hasVariable {
				p.errorf("Variable %s is not defined", t.val)
			}
			n.Command += variableValue
		case tokenString:
			n.Command += t.val
		case tokenQuotedString:
			n.Command += t.val
		case tokenPipe:
			if !atLeastOneTokenForCommandEaten {
				p.errorf("Expected non-empty command body")
			}
			break CommandLoop
		case tokenError:
			p.errorf("Lexer error: %v, L%d:%d", t.val, t.line, t.pos)
		default:
			p.errorf("Unexpected token %s", t.val)
		}
		atLeastOneTokenForCommandEaten = true
	}

	t = p.expect(tokenPathPattern)
	if len(n.Output) != 0 {
		n.Output += " "
	}
	n.Output += t.val

	p.nodes <- &n
	return parsePass
}

func parseStateInitial(p *Parser) parserStateFn {
	parseRule(p)
	return nil
}

func Parse(name, input string, env *ParserEnv) *Parser {
	p := &Parser{
		env:      env,
		name:     name,
		nodes:    make(chan Node),
		errc:     make(chan error, 1),
		ringLock: -1,
		ringNext: 1,
	}

	p.lexer = lex(name, input)
	go p.run()

	return p
}
