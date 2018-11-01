package vakefile

import (
	"testing"
)

type tokens []token

var testCases = map[string]tokens{
	"!bundle_css = foreach": []token{
		token{val: "bundle_css", typ: tokenMacro},
		token{val: "=", typ: tokenAssign},
		token{val: "foreach", typ: tokenKeywordForeach},
	},
	"!foo = foreach\n!boo = |> !foo |>": []token{
		token{val: "foo", typ: tokenMacro},
		token{val: "=", typ: tokenAssign},
		token{val: "foreach", typ: tokenKeywordForeach},
		token{val: "boo", typ: tokenMacro},
		token{val: "=", typ: tokenAssign},
		token{val: "|>", typ: tokenPipe},
		token{val: "foo", typ: tokenMacro},
		token{val: "|>", typ: tokenPipe},
	},
	"!foo = a.js |> echo $(xv) \"|>\" |>": []token{
		token{val: "foo", typ: tokenMacro},
		token{val: "=", typ: tokenAssign},
		token{val: "a.js", typ: tokenPathPattern},
		token{val: "|>", typ: tokenPipe},
		token{val: "echo", typ: tokenString},
		token{val: "xv", typ: tokenVariable},
		token{val: "|>", typ: tokenQuotedString},
		token{val: "|>", typ: tokenPipe},
	},
	"!foo = foreach src/*.css |> cat %f > %o |> dist/bundle.css": []token{
		token{val: "foo", typ: tokenMacro},
		token{val: "=", typ: tokenAssign},
		token{val: "foreach", typ: tokenKeywordForeach},
		token{val: "src/*.css", typ: tokenPathPattern},
		token{val: "|>", typ: tokenPipe},
		token{val: "cat", typ: tokenString},
		token{val: "f", typ: tokenPercentFlag},
		token{val: ">", typ: tokenString},
		token{val: "o", typ: tokenPercentFlag},
		token{val: "|>", typ: tokenPipe},
		token{val: "dist/bundle.css", typ: tokenPathPattern},
	},
	"!foo = foreach src/*.css |> cat %f > %o |>\n": []token{
		token{val: "foo", typ: tokenMacro},
		token{val: "=", typ: tokenAssign},
		token{val: "foreach", typ: tokenKeywordForeach},
		token{val: "src/*.css", typ: tokenPathPattern},
		token{val: "|>", typ: tokenPipe},
		token{val: "cat", typ: tokenString},
		token{val: "f", typ: tokenPercentFlag},
		token{val: ">", typ: tokenString},
		token{val: "o", typ: tokenPercentFlag},
		token{val: "|>", typ: tokenPipe},
	},
}

func TestCases(t *testing.T) {
	for source, tokens := range testCases {
		l := lex("testCase", source)
		i := 0
		for token := range l.tokens {
			if i >= len(tokens) {
				t.Fatalf("[%s], there are more tokens when expected, extra token: %s", source, token)
			}
			if token.typ != tokens[i].typ || token.val != tokens[i].val {
				t.Errorf("[%s] error, expected: ('%s', %v), got: ('%s', %v)", source, tokens[i].val, tokens[i].typ, token.val, token.typ)
			}
			i++
		}
		if i < len(tokens) {
			t.Errorf("[%s], there are less tokens when expected, needed token: %s", source, tokens[i])
		}
	}
}
