package vakefile

type NodeType int

type Node interface {
	Type() NodeType
}

const (
	NodeError NodeType = iota
	// ex: [:src/*.js |> !bundle_js |> app/bundle.js]
	NodeRule
	// ex: [!bundle_js = |> cat %f | node_modules/.bin/terser -c > %o |>]
	NodeMacro
	NodeVariable
	NodeCodeBlock
)

type RuleNode struct {
	Foreach bool
	Inputs  []string
	Command string
	Output  string
}

func (n *RuleNode) Type() NodeType {
	return NodeRule
}

type MacroNode struct {
	Name string
}

func (n *MacroNode) Type() NodeType {
	return NodeMacro
}
