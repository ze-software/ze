// Design: docs/architecture/web-components.md -- graphical collection editors
// Related: handler_graphical_values.go -- authorized saves through the shared editor

package web

import (
	"slices"
	"strconv"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// GraphicalMember preserves a member's position and activation marker.
type GraphicalMember struct {
	Value    string
	Inactive bool
}

// GraphicalEntry is one freeform entry. A flag has value true.
type GraphicalEntry struct {
	Name  string
	Value string
}

// GraphicalValues is the form for one schema-defined collection or freeform node.
type GraphicalValues struct {
	Name             string
	Path             string
	Members          []GraphicalMember
	Options          []string
	Freeform         bool
	Entries          []GraphicalEntry
	MemberActivation bool // Multi-word leaves have only whole-setting activation.
}

// GraphicalInlineList carries entry links and the keyed creation target.
type GraphicalInlineList struct {
	Name    string
	Path    string
	Entries []ChildEntry
}

func buildGraphicalValues(path []string, name string, node config.Node, tree *config.Tree) GraphicalValues {
	v := GraphicalValues{Name: name, Path: encodeConfigPath(path)}
	switch n := node.(type) {
	case *config.ValueOrArrayNode:
		v.Options = n.ValidValues
		v.MemberActivation = true
	case *config.BracketLeafListNode:
		v.Options = n.Enums
		v.MemberActivation = true
	case *config.MultiLeafNode:
		v.Options = n.Enums
	case *config.FreeformNode:
		v.Freeform = true
	}
	if tree == nil {
		return v
	}
	if v.Freeform {
		if entries := tree.GetContainer(name); entries != nil {
			for _, key := range entries.Values() {
				value, _ := entries.Get(key)
				v.Entries = append(v.Entries, GraphicalEntry{Name: key, Value: value})
			}
		}
		return v
	}
	for _, state := range tree.GetMultiValuesState(name) {
		v.Members = append(v.Members, GraphicalMember{Value: state.Value, Inactive: state.Inactive})
	}
	if len(v.Members) == 0 {
		if value, present := tree.Get(name); present {
			for _, member := range config.SplitLeafListMembers(value) {
				v.Members = append(v.Members, GraphicalMember{Value: member})
			}
		}
	}
	return v
}

func populateInlineList(data *FragmentData, schema *config.Schema, tree *config.Tree, node *config.InlineListNode) {
	parentPath := data.Path[:len(data.Path)-1]
	if parent, err := walkSchema(schema, parentPath); err == nil {
		if _, isInline := parent.(*config.InlineListNode); isInline {
			populateFragmentFields(data, node, walkTree(tree, schema, data.Path), showPathPrefix+data.CurrentPath)
			return
		}
	}
	v := &GraphicalInlineList{Name: data.Path[len(data.Path)-1], Path: data.CurrentPath}
	if parent := walkTree(tree, schema, parentPath); parent != nil {
		for _, key := range parent.ListKeys(v.Name) {
			where := slices.Concat(data.Path, []string{key})
			hxPath := encodeConfigPath(where)
			v.Entries = append(v.Entries, ChildEntry{Name: key, URL: showPathPrefix + hxPath + "/", HxPath: hxPath})
		}
	}
	data.InlineList = v
}

func graphicalOperation(name string, index int) string {
	var b textbuf.Buffer
	return b.Str(name).Byte(':').Int(int64(index)).String()
}

func graphicalInactive(inactive bool) string { return strconv.FormatBool(inactive) }

func graphicalEntryValue(value string) string {
	if value == boolTrue {
		return ""
	}
	return value
}
