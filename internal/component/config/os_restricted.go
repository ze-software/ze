// Design: docs/architecture/config/yang-config-design.md -- ze:os prunes a node another OS carries
// Related: yang_schema.go -- yangToNode, where the node is pruned
// Related: parser.go -- hierarchical parse, unknown-field refusal
// Related: parser_list.go -- list entry parse, unknown-field refusal
// Related: setparser.go -- set-format parse, unknown-field refusal

package config

import (
	"fmt"
	"runtime"
	"sync"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// NodeOfAnotherOSError refuses a config node the schema carries only on
// another operating system (YANG ze:os). The build prunes the node on this
// OS, because GOOS is fixed for the binary, so without this error the parser
// answered "unknown field", as if the operator had misspelled it. The verdict
// is about the host running the command, not about the config: the same file
// loads on the OS the node names. `ze config validate` reports it under the
// host-verdict code config-node-other-os.
type NodeOfAnotherOSError struct {
	Line   int    // the line holding the node, 0 when unknown
	Parent string // the schema node holding it, as the parser names it
	Name   string // the node's name
	OS     string // the GOOS the node exists on (ze:os)
	HostOS string // the GOOS of this binary
}

// Error names the node, the OS that carries it, and the fix.
func (e *NodeOfAnotherOSError) Error() string {
	var b textbuf.Buffer
	if e.Line > 0 {
		b.Str("line ").Int(int64(e.Line)).Str(": ")
	}
	b.Str(e.Name)
	if e.Parent != "" {
		b.Str(" in ").Str(e.Parent)
	}
	b.Str(" exists only on ").Str(e.OS).
		Str(", and this ze runs on ").Str(e.HostOS).
		Str(": the config is not wrong, this host cannot carry it; validate it with ze config validate on a ").
		Str(e.OS).Str(" host")
	if e.Line > 0 {
		b.Str(" (line ").Int(int64(e.Line)).Byte(')')
	}
	return b.String()
}

// schemaOS is the GOOS the schema is built for. It is runtime.GOOS except in
// a test that builds the schema as another OS would (SetSchemaOSForTest).
var schemaOS = runtime.GOOS

// SetSchemaOSForTest builds every later schema as the named GOOS would, and
// answers the function that restores this binary's own. The caller MUST call
// the restore function when its test ends. Only for use in tests.
func SetSchemaOSForTest(goos string) (restore func()) {
	previous := schemaOS
	schemaOS = goos
	yangSchemaCache.reset()
	return func() {
		schemaOS = previous
		yangSchemaCache.reset()
	}
}

// otherOSNode is one node a schema build pruned for its ze:os.
type otherOSNode struct {
	path  string // the schema path of the pruned node, as yangToNode received it
	child otherOSChild
}

// otherOSChild is what a parent records about one child the build pruned for
// its ze:os: the GOOS that carries it, and the node as this build converts it.
// The parsers read only os, to refuse the name. LookupDeclared reads node, to
// answer what the schema declares about it. A child pruned inside that node
// for its own ze:os is not in node.
type otherOSChild struct {
	os   string
	node Node
}

var (
	otherOSNodesMu sync.Mutex
	otherOSNodes   []otherOSNode
)

// resetOtherOSNodes starts a schema build's record of pruned nodes.
func resetOtherOSNodes() {
	otherOSNodesMu.Lock()
	otherOSNodes = nil
	otherOSNodesMu.Unlock()
}

// recordOtherOSNode records a node yangToNode pruned because its ze:os names
// another OS, with the node it converted.
func recordOtherOSNode(path, goos string, node Node) {
	otherOSNodesMu.Lock()
	otherOSNodes = append(otherOSNodes, otherOSNode{path: path, child: otherOSChild{os: goos, node: node}})
	otherOSNodesMu.Unlock()
}

// markOtherOSNodes records each pruned node on the schema node that would have
// held it, so the parsers refuse its name with NodeOfAnotherOSError instead of
// "unknown field". A parent the build does not carry has nothing to refuse.
func markOtherOSNodes(schema *Schema) {
	otherOSNodesMu.Lock()
	pruned := otherOSNodes
	otherOSNodes = nil
	otherOSNodesMu.Unlock()
	for _, node := range pruned {
		segments := SplitPath(node.path)
		name := segments[len(segments)-1]
		if len(segments) == 1 {
			schema.root.otherOS = withOtherOS(schema.root.otherOS, name, node.child)
			continue
		}
		switch parent := schema.LookupTokenPath(segments[:len(segments)-1]).(type) {
		case *ContainerNode:
			parent.otherOS = withOtherOS(parent.otherOS, name, node.child)
		case *ListNode:
			parent.otherOS = withOtherOS(parent.otherOS, name, node.child)
		}
	}
}

// withOtherOS answers marks with child recorded under name.
func withOtherOS(marks map[string]otherOSChild, name string, child otherOSChild) map[string]otherOSChild {
	if marks == nil {
		marks = make(map[string]otherOSChild)
	}
	marks[name] = child
	return marks
}

// nodeOfAnotherOS answers the refusal for name under parent when the schema
// pruned it for its ze:os, and nil when parent never had such a child.
func nodeOfAnotherOS(parent Node, parentName, name string, line int) error {
	var marks map[string]otherOSChild
	switch p := parent.(type) {
	case *ContainerNode:
		marks = p.otherOS
	case *ListNode:
		marks = p.otherOS
	}
	child, pruned := marks[name]
	if !pruned {
		return nil
	}
	goos := child.os
	return &NodeOfAnotherOSError{Line: line, Parent: parentName, Name: name, OS: goos, HostOS: schemaOS}
}

// LookupDeclared finds a node by path as Lookup does, and also through a node
// this build pruned for its ze:os. It answers what the schema declares, such
// as a container's defaults, to code that holds a config of a node another OS
// carries and needs its declared values whatever host it runs on. The parsers
// never use it: they still refuse a node of another OS (nodeOfAnotherOS). A
// node pruned inside a pruned node for its own ze:os is not reachable, and is
// refused as unknown.
func (s *Schema) LookupDeclared(path string) (Node, error) {
	var current Node = s.root
	for _, part := range SplitPath(path) {
		child, err := declaredChild(current, part)
		if err != nil {
			return nil, err
		}
		current = child
	}
	return current, nil
}

// declaredChild answers the child of parent called name: the built child, or
// the node the build pruned for its ze:os.
func declaredChild(parent Node, name string) (Node, error) {
	var child Node
	var marks map[string]otherOSChild
	switch p := parent.(type) {
	case *ContainerNode:
		child, marks = p.Get(name), p.otherOS
	case *ListNode:
		child, marks = p.Get(name), p.otherOS
	case *LeafNode:
		return nil, fmt.Errorf("cannot traverse into leaf at: %s", name)
	default:
		return nil, fmt.Errorf("unknown node type at: %s", name)
	}
	if child != nil {
		return child, nil
	}
	pruned, ok := marks[name]
	if !ok {
		return nil, fmt.Errorf("unknown path element: %s", name)
	}
	if pruned.node == nil {
		return nil, fmt.Errorf("unknown path element: %s (it exists only on %s, and this build could not convert it)", name, pruned.os)
	}
	return pruned.node, nil
}
