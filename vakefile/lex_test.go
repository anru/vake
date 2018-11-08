package vakefile

import (
	"io/ioutil"
	"testing"
)

type tokens []token

var fileTestCases = map[string]tokens{
	"lex-if": []token{
		token{val: "include_rules", typ: tokenKeywordIncludeRules},
		token{val: "ifeq", typ: tokenKeywordIfeq},
		token{val: "NODE_ENV", typ: tokenVariable},
		token{val: ",", typ: tokenComma},
		token{val: "development", typ: tokenIdentifier},
		token{val: "bundle_js", typ: tokenMacro},
		token{val: "=", typ: tokenAssign},
		token{val: "|>", typ: tokenPipe},
		token{val: "cat %f", typ: tokenString},
		token{val: "|>", typ: tokenPipe},
		token{val: "endif", typ: tokenKeywordEndif},
	},
	"rule-comment": []token{
		token{val: "bundles css", typ: tokenComment},
		token{val: ":", typ: tokenColon},
		token{val: "src/*.css", typ: tokenPathPattern},
		token{val: "|>", typ: tokenPipe},
		token{val: "b", typ: tokenMacro},
		token{val: "|>", typ: tokenPipe},
		token{val: "out.css", typ: tokenPathPattern},
	},
	"labels": []token{
		token{val: "js", typ: tokenLabel},
		token{val: "tup", typ: tokenIdentifier},
		token{val: ":", typ: tokenColon},
		token{val: "src/*.css", typ: tokenPathPattern},
		token{val: "|>", typ: tokenPipe},
		token{val: "b", typ: tokenMacro},
		token{val: "|>", typ: tokenPipe},
		token{val: "out.css", typ: tokenPathPattern},
		// token{val: "tup", typ: tokenLabel},
	},
}

var testCases = map[string]tokens{
	"!bundle_css = foreach": []token{
		token{val: "bundle_css", typ: tokenMacro},
		token{val: "=", typ: tokenAssign},
		token{val: "foreach", typ: tokenKeywordForeach},
	},
	"!bundle_css = foreach\ninclude_rules": []token{
		token{val: "bundle_css", typ: tokenMacro},
		token{val: "=", typ: tokenAssign},
		token{val: "foreach", typ: tokenKeywordForeach},
		token{val: "include_rules", typ: tokenKeywordIncludeRules},
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
		token{val: "echo ", typ: tokenString},
		token{val: "xv", typ: tokenVariable},
		token{val: " ", typ: tokenString},
		token{val: "\"|>\"", typ: tokenQuotedString},
		token{val: "|>", typ: tokenPipe},
	},
	"!foo = foreach src/*.css |> cat %f > %o |> dist/bundle.css": []token{
		token{val: "foo", typ: tokenMacro},
		token{val: "=", typ: tokenAssign},
		token{val: "foreach", typ: tokenKeywordForeach},
		token{val: "src/*.css", typ: tokenPathPattern},
		token{val: "|>", typ: tokenPipe},
		token{val: "cat %f > %o", typ: tokenString},
		token{val: "|>", typ: tokenPipe},
		token{val: "dist/bundle.css", typ: tokenPathPattern},
	},
	"!foo = foreach src/*.css |> cat %f > %o |>\n": []token{
		token{val: "foo", typ: tokenMacro},
		token{val: "=", typ: tokenAssign},
		token{val: "foreach", typ: tokenKeywordForeach},
		token{val: "src/*.css", typ: tokenPathPattern},
		token{val: "|>", typ: tokenPipe},
		token{val: "cat %f > %o", typ: tokenString},
		token{val: "|>", typ: tokenPipe},
	},
	": foreach src/*.css |> !bundle_css |> static/%b": []token{
		token{val: ":", typ: tokenColon},
		token{val: "foreach", typ: tokenKeywordForeach},
		token{val: "src/*.css", typ: tokenPathPattern},
		token{val: "|>", typ: tokenPipe},
		token{val: "bundle_css", typ: tokenMacro},
		token{val: "|>", typ: tokenPipe},
		token{val: "static/%b", typ: tokenPathPattern},
	},
}

func doTestLex(t *testing.T, filename, source string, tokens []token) {
	l := lex(filename, source)
	i := 0
	recvTokens := []token{}
	for token := range l.tokens {
		recvTokens = append(recvTokens, token)
		if i > len(tokens) {
			t.Errorf("[%s], there are more tokens when expected, extra token: %s", filename, token)
			t.Logf("recv. tokens: %v", recvTokens)
			break
		} else if i == len(tokens) {
			// last one should be EOF
			if token.typ != tokenEOF {
				t.Errorf("[%s], expected EOF token, got ('%s', %v)\nrecv. tokens: %v", filename, token.val, token.typ, recvTokens)
			}
		} else {
			if token.typ != tokens[i].typ || token.val != tokens[i].val {
				t.Errorf("[%s] error, expected: [%d]('%s', %v), got: ('%s', %v)\n\nrecv. tokens: %v", filename, i, tokens[i].val, tokens[i].typ, token.val, token.typ, recvTokens)
			}
		}
		i++
	}
	if i < len(tokens) {
		t.Errorf("[%s], there are less tokens when expected, needed token: %s", filename, tokens[i])
	}
}

func TestCases(t *testing.T) {
	for source, tokens := range testCases {
		doTestLex(t, source, source, tokens)
	}
}

func TestFileCases(t *testing.T) {
	for filename, tokens := range fileTestCases {
		content, err := ioutil.ReadFile("_lex-data/" + filename + ".ake")
		if err != nil {
			t.Fatal(err)
		}
		doTestLex(t, filename, string(content), tokens)
	}
}
