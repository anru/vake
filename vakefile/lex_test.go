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
	"!foo = foreach src/*.css |> cat %f > %o |> dist/bundle.css": []token{
		token{val: "foo", typ: tokenMacro},
		token{val: "=", typ: tokenAssign},
		token{val: "foreach", typ: tokenKeywordForeach},
		token{val: "src/*.css", typ: tokenString},
		token{val: "|>", typ: tokenPipe},
		token{val: "cat %f > %o", typ: tokenString},
		token{val: "|>", typ: tokenPipe},
		token{val: "dist/bundle.css", typ: tokenString},
	},
	"!foo = foreach src/*.css |> cat %f > %o\n": []token{
		token{val: "foo", typ: tokenMacro},
		token{val: "=", typ: tokenAssign},
		token{val: "foreach", typ: tokenKeywordForeach},
		token{val: "src/*.css", typ: tokenString},
		token{val: "|>", typ: tokenPipe},
		token{val: "cat %f > %o", typ: tokenString},
	},
}

func TestCases(t *testing.T) {
	for source, tokens := range testCases {
		l := lex("testCase", source)
		i := 0
		for token := range l.tokens {
			if i >= len(tokens) {
				t.Fatalf("there are more tokens when expected, extra token: %s", token)
			}
			if token.typ != tokens[i].typ || token.val != tokens[i].val {
				t.Errorf("%s error, expected: ('%s', %v), got: ('%s', %v)", source, tokens[i].val, tokens[i].typ, token.val, token.typ)
			}
			i++
		}
		if i < len(tokens) {
			t.Errorf("there are less tokens when expected, needed token: %s", tokens[i])
		}
	}
}
