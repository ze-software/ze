// Package config implements schema-driven configuration parsing.
//
// It supports multiple input formats (ExaBGP, set-style, KDL) that all
// validate against the same schema and produce the same Config output.
package config

import (
	"fmt"
	"net/netip"
	"strconv"
	"strings"
)

// ValueType represents the type of a leaf value.
type ValueType int

const (
	TypeString ValueType = iota
	TypeBool
	TypeUint16
	TypeUint32
	TypeIPv4
	TypeIPv6
	TypeIP     // IPv4 or IPv6
	TypePrefix // CIDR prefix
)

func (t ValueType) String() string {
	switch t {
	case TypeString:
		return "string"
	case TypeBool:
		return "bool"
	case TypeUint16:
		return "uint16"
	case TypeUint32:
		return "uint32"
	case TypeIPv4:
		return "ipv4"
	case TypeIPv6:
		return "ipv6"
	case TypeIP:
		return "ip"
	case TypePrefix:
		return "prefix"
	default:
		return "unknown"
	}
}

// NodeKind represents the kind of schema node.
type NodeKind int

const (
	NodeLeaf NodeKind = iota
	NodeContainer
	NodeList
	NodeFreeform   // accepts "word word;" entries as key->true
	NodeFlex       // can be flag (;), value (word;), or block ({})
	NodeInlineList // list that supports inline: "route PREFIX attr val;"
)

// Node is the interface for all schema nodes.
type Node interface {
	Kind() NodeKind
}

// LeafNode represents a terminal value.
type LeafNode struct {
	Type    ValueType
	Default string
}

func (n *LeafNode) Kind() NodeKind { return NodeLeaf }

// ContainerNode represents a group of child nodes.
type ContainerNode struct {
	children map[string]Node
	order    []string // preserve definition order
}

func (n *ContainerNode) Kind() NodeKind { return NodeContainer }

// Has returns true if the container has a child with the given name.
func (n *ContainerNode) Has(name string) bool {
	_, ok := n.children[name]
	return ok
}

// Get returns a child node by name.
func (n *ContainerNode) Get(name string) Node {
	return n.children[name]
}

// Children returns all child names in definition order.
func (n *ContainerNode) Children() []string {
	return n.order
}

// ListNode represents a keyed collection of containers.
type ListNode struct {
	KeyType  ValueType
	children map[string]Node
	order    []string
}

func (n *ListNode) Kind() NodeKind { return NodeList }

// Has returns true if the list entry has a child with the given name.
func (n *ListNode) Has(name string) bool {
	_, ok := n.children[name]
	return ok
}

// Get returns a child node by name.
func (n *ListNode) Get(name string) Node {
	return n.children[name]
}

// Children returns all child names in definition order.
func (n *ListNode) Children() []string {
	return n.order
}

// Schema holds the configuration schema definition.
type Schema struct {
	root *ContainerNode
}

// NewSchema creates a new empty schema.
func NewSchema() *Schema {
	return &Schema{
		root: &ContainerNode{
			children: make(map[string]Node),
		},
	}
}

// Define adds a top-level node to the schema.
func (s *Schema) Define(name string, node Node) {
	s.root.children[name] = node
	s.root.order = append(s.root.order, name)
}

// Has returns true if the schema has a top-level node with the given name.
func (s *Schema) Has(name string) bool {
	return s.root.Has(name)
}

// Get returns a top-level node by name.
func (s *Schema) Get(name string) Node {
	return s.root.Get(name)
}

// Lookup finds a node by dot-separated path.
func (s *Schema) Lookup(path string) (Node, error) {
	parts := strings.Split(path, ".")
	var current Node = s.root

	for _, part := range parts {
		switch n := current.(type) {
		case *ContainerNode:
			child := n.Get(part)
			if child == nil {
				return nil, fmt.Errorf("unknown path element: %s", part)
			}
			current = child
		case *ListNode:
			child := n.Get(part)
			if child == nil {
				return nil, fmt.Errorf("unknown path element: %s", part)
			}
			current = child
		case *LeafNode:
			return nil, fmt.Errorf("cannot traverse into leaf at: %s", part)
		default:
			return nil, fmt.Errorf("unknown node type at: %s", part)
		}
	}

	return current, nil
}

// Field is a helper for defining container/list children.
type FieldDef struct {
	Name string
	Node Node
}

// Field creates a field definition.
func Field(name string, node Node) FieldDef {
	return FieldDef{Name: name, Node: node}
}

// Leaf creates a leaf node with the given type.
func Leaf(typ ValueType) *LeafNode {
	return &LeafNode{Type: typ}
}

// LeafWithDefault creates a leaf node with a default value.
func LeafWithDefault(typ ValueType, def string) *LeafNode {
	return &LeafNode{Type: typ, Default: def}
}

// Container creates a container node with the given children.
func Container(fields ...FieldDef) *ContainerNode {
	c := &ContainerNode{
		children: make(map[string]Node),
	}
	for _, f := range fields {
		c.children[f.Name] = f.Node
		c.order = append(c.order, f.Name)
	}
	return c
}

// List creates a list node with the given key type and children.
func List(keyType ValueType, fields ...FieldDef) *ListNode {
	l := &ListNode{
		KeyType:  keyType,
		children: make(map[string]Node),
	}
	for _, f := range fields {
		l.children[f.Name] = f.Node
		l.order = append(l.order, f.Name)
	}
	return l
}

// FreeformNode accepts "word word ...;" entries, storing as "word word" -> true.
// Used for family { ipv4 unicast; ipv6 unicast; }.
type FreeformNode struct{}

func (n *FreeformNode) Kind() NodeKind { return NodeFreeform }

// Freeform creates a freeform node.
func Freeform() *FreeformNode {
	return &FreeformNode{}
}

// MultiLeafNode accepts multiple words until semicolon: "word word word;".
type MultiLeafNode struct {
	Type ValueType
}

func (n *MultiLeafNode) Kind() NodeKind { return NodeLeaf }

// MultiLeaf creates a multi-word leaf node.
func MultiLeaf(typ ValueType) *MultiLeafNode {
	return &MultiLeafNode{Type: typ}
}

// ArrayLeafNode accepts [ item item ... ] syntax.
type ArrayLeafNode struct {
	Type ValueType
}

func (n *ArrayLeafNode) Kind() NodeKind { return NodeLeaf }

// ArrayLeaf creates an array leaf node.
func ArrayLeaf(typ ValueType) *ArrayLeafNode {
	return &ArrayLeafNode{Type: typ}
}

// ValueOrArrayNode accepts either "value;" or "[ item item ... ];" syntax.
// Stores result as space-separated string in both cases.
type ValueOrArrayNode struct {
	Type ValueType
}

func (n *ValueOrArrayNode) Kind() NodeKind { return NodeLeaf }

// ValueOrArray creates a node that accepts either a single value or bracketed list.
func ValueOrArray(typ ValueType) *ValueOrArrayNode {
	return &ValueOrArrayNode{Type: typ}
}

// FlexNode can be a flag (;), value (word;), or block ({}).
// Used for capabilities like graceful-restart which support all three forms.
type FlexNode struct {
	children map[string]Node
	order    []string
}

func (n *FlexNode) Kind() NodeKind { return NodeFlex }

// Has returns true if the flex node has a child with the given name.
func (n *FlexNode) Has(name string) bool {
	_, ok := n.children[name]
	return ok
}

// Get returns a child node by name.
func (n *FlexNode) Get(name string) Node {
	return n.children[name]
}

// Children returns all child names in definition order.
func (n *FlexNode) Children() []string {
	return n.order
}

// Flex creates a flex node with optional children for block mode.
func Flex(fields ...FieldDef) *FlexNode {
	f := &FlexNode{
		children: make(map[string]Node),
	}
	for _, field := range fields {
		f.children[field.Name] = field.Node
		f.order = append(f.order, field.Name)
	}
	return f
}

// InlineListNode is a list that supports inline attribute syntax.
// e.g., "route 10.0.0.0/8 next-hop 1.1.1.1;" or "route 10.0.0.0/8 { next-hop 1.1.1.1; }".
type InlineListNode struct {
	KeyType  ValueType
	children map[string]Node
	order    []string
}

func (n *InlineListNode) Kind() NodeKind { return NodeInlineList }

// Has returns true if the list entry has a child with the given name.
func (n *InlineListNode) Has(name string) bool {
	_, ok := n.children[name]
	return ok
}

// Get returns a child node by name.
func (n *InlineListNode) Get(name string) Node {
	return n.children[name]
}

// Children returns all child names in definition order.
func (n *InlineListNode) Children() []string {
	return n.order
}

// InlineList creates an inline list node.
func InlineList(keyType ValueType, fields ...FieldDef) *InlineListNode {
	l := &InlineListNode{
		KeyType:  keyType,
		children: make(map[string]Node),
	}
	for _, f := range fields {
		l.children[f.Name] = f.Node
		l.order = append(l.order, f.Name)
	}
	return l
}

// ValidateValue validates a string value against a type.
func ValidateValue(typ ValueType, value string) error {
	switch typ {
	case TypeString:
		return nil

	case TypeBool:
		if value != configTrue && value != "false" && value != "enable" && value != "disable" { //nolint:goconst // String literals for validation
			return fmt.Errorf("invalid bool: %q (expected true/false/enable/disable)", value)
		}
		return nil

	case TypeUint16:
		n, err := strconv.ParseUint(value, 10, 16)
		if err != nil {
			return fmt.Errorf("invalid uint16: %q", value)
		}
		if n > 65535 {
			return fmt.Errorf("uint16 overflow: %s", value)
		}
		return nil

	case TypeUint32:
		_, err := strconv.ParseUint(value, 10, 32)
		if err != nil {
			return fmt.Errorf("invalid uint32: %q", value)
		}
		return nil

	case TypeIPv4:
		addr, err := netip.ParseAddr(value)
		if err != nil || !addr.Is4() {
			return fmt.Errorf("invalid IPv4: %q", value)
		}
		return nil

	case TypeIPv6:
		addr, err := netip.ParseAddr(value)
		if err != nil || !addr.Is6() {
			return fmt.Errorf("invalid IPv6: %q", value)
		}
		return nil

	case TypeIP:
		_, err := netip.ParseAddr(value)
		if err != nil {
			return fmt.Errorf("invalid IP: %q", value)
		}
		return nil

	case TypePrefix:
		// Try as prefix first
		_, err := netip.ParsePrefix(value)
		if err != nil {
			// Try as plain IP (host route /32 or /128)
			_, err2 := netip.ParseAddr(value)
			if err2 != nil {
				return fmt.Errorf("invalid prefix: %q", value)
			}
		}
		return nil

	default:
		return fmt.Errorf("unknown type: %v", typ)
	}
}

// NormalizeBool converts enable/disable to true/false.
func NormalizeBool(value string) string {
	switch value {
	case "enable":
		return configTrue
	case "disable":
		return "false"
	default:
		return value
	}
}
