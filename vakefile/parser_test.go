package vakefile

import (
	"io/ioutil"
	"reflect"
	"testing"
)

type nodes []Node

var parserTestCases = map[string]nodes{
	"simplest-rule": nodes{
		&RuleNode{
			Inputs: []string{
				"src/*.js",
			},
			Command: "cat %f > %o",
			Output:  "app.js",
		},
	},
}

func doParserTest(t *testing.T, filename string, env *ParserEnv) {
	content, err := ioutil.ReadFile("_test-files/" + filename + ".ake")
	if err != nil {
		t.Fatal(err)
	}
	nodes, present := parserTestCases[filename]
	if !present {
		t.Fatalf("There is no expectations for %s", filename)
	}
	source := string(content)

	p := Parse(filename, source, env)
	i := 0
	recvNodes := []Node{}
MainLoop:
	for {
		select {
		case node, ok := <-p.nodes:
			if !ok {
				break MainLoop
			}
			recvNodes = append(recvNodes, node)
			if i >= len(nodes) {
				t.Errorf("[%s], there are more nodes when expected, extra node: %s", filename, node)
				// t.Logf("recv. nodes: %v", recvNodes)
				break
			} else {
				if !reflect.DeepEqual(node, nodes[i]) {
					t.Errorf("[%s] error at %d, expected: %v), got: %v", filename, i, nodes[i], node)
				}
			}
			i++
		case err := <-p.errc:
			t.Errorf("%v", err)
			break MainLoop
		}
	}
	if i < len(nodes) {
		t.Errorf("[%s], there are less nodes when expected, needed node: %v", filename, nodes[i])
	}
}

func TestParser(t *testing.T) {
	env := ParserEnv{}
	for filename := range parserTestCases {
		doParserTest(t, filename, &env)
	}
}
