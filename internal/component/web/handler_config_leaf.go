// Design: docs/architecture/web-interface.md -- Leaf input type and template helpers
// Overview: handler_config.go -- Config tree view handlers
// Related: handler_config_walk.go -- Schema and config tree walking

package web

import (
	"html/template"

	"github.com/a-h/templ"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// buildBreadcrumbs creates breadcrumb navigation entries. Each path segment
// links to /show/<path-up-to-here>/. The ze logo serves as root navigation.
func buildBreadcrumbs(path []string) []BreadcrumbSegment {
	crumbs := make([]BreadcrumbSegment, 0, len(path))

	for i, seg := range path {
		url := showPathPrefix + textbuf.Join(path[:i+1], "/") + "/"
		crumbs = append(crumbs, BreadcrumbSegment{
			Name:   seg,
			URL:    url,
			Active: i == len(path)-1,
		})
	}

	return crumbs
}

// leafInputSelect is the LeafField.InputType of an enum leaf. The editor
// component switches on it and buildLeafField writes it, so both read one name.
const leafInputSelect = "select"

// The LeafField.InputType values leafInputType returns for a plain value. They
// are HTML input types, so the browser picks the keyboard and the validation
// from them.
const (
	leafInputText   = "text"
	leafInputNumber = "number"
)

// buildLeafField maps a LeafNode to an HTML input field description.
func buildLeafField(name string, leaf *config.LeafNode, value string, configured bool) LeafField {
	info := leafInputType(leaf.Type)
	info.Name = name
	// Never prefill the form input with a stored secret (secret.go). A new
	// bcrypt password is set through the plaintext-<name> sibling. A
	// resubmitted placeholder is filtered on the write path
	// (EditorManager.SetValue).
	info.Value = maskSecretLeaf(leaf, value)
	info.Default = leaf.Default
	info.IsConfigured = configured
	info.Description = leaf.Description
	if len(leaf.Enums) > 0 {
		info.InputType = leafInputSelect
		info.Options = leaf.Enums
	}

	return info
}

// leafInputType returns the HTML input attributes for a given ValueType.
func leafInputType(vt config.ValueType) LeafField {
	switch vt {
	case config.TypeString:
		return LeafField{InputType: leafInputText}

	case config.TypeBool:
		return LeafField{InputType: "checkbox"}

	case config.TypeUint16:
		return LeafField{InputType: leafInputNumber, Min: "0", Max: "65535"}

	case config.TypeUint32:
		return LeafField{InputType: leafInputNumber, Min: "0", Max: "4294967295"}

	case config.TypeIPv4:
		return LeafField{
			InputType:   leafInputText,
			Pattern:     `^(\d{1,3}\.){3}\d{1,3}$`,
			Placeholder: "e.g., 192.0.2.1",
		}

	case config.TypeIPv6:
		return LeafField{
			InputType:   leafInputText,
			Pattern:     `^[0-9a-fA-F:]+$`,
			Placeholder: "e.g., 2001:db8::1",
		}

	case config.TypeIP:
		return LeafField{InputType: leafInputText, Placeholder: "IPv4 or IPv6 address"}

	case config.TypePrefix:
		return LeafField{
			InputType:   leafInputText,
			Pattern:     `^[0-9a-fA-F.:]+/\d{1,3}$`,
			Placeholder: "e.g., 10.0.0.0/24",
		}

	case config.TypeDuration:
		return LeafField{InputType: leafInputText, Placeholder: "e.g., 5s, 100ms"}

	case config.TypeInt:
		return LeafField{InputType: leafInputNumber}
	case config.TypeEmpty:
		return LeafField{InputType: leafInputText}
	}

	return LeafField{InputType: leafInputText}
}

// configViewComponent resolves the component that renders one config node.
//
// EVERY node kind reaches markup. Two used to reach none. config.NodeLeaf
// named a template the renderer did not hold. config.NodeFlex named one whose
// markup read fields the caller never filled. Both rendered a blank panel. For
// a config editor that means the operator cannot read the configuration.
//
// Both are answered by configContainer, and neither needs markup of its own.
// buildConfigViewData (handler_config_walk.go) fills LeafFields for a leaf, and
// calls populateContainerView for a flex node, which fills Children and
// LeafFields. configContainer renders exactly those two.
func configViewComponent(kind config.NodeKind, v *ConfigViewData) templ.Component {
	switch kind {
	case config.NodeList:
		return configList(v)
	case config.NodeFreeform:
		return configFreeform(v)
	case config.NodeInlineList:
		return configInlineList(v)
	case config.NodeContainer, config.NodeLeaf, config.NodeFlex:
		return configContainer(v)
	}

	// A node kind added without a case above lands here. configContainer renders
	// the node's LeafFields and Children, which is the closest correct markup an
	// unknown kind can get, and the warning names the kind so the missing case is
	// findable. The caller used to test this return for nil to reach that warning,
	// which never fired: every branch of this function returns a component.
	serverLogger.Warn("config node kind has no view component",
		"kind", nodeKindString(kind), "path", v.CurrentPath)

	return configContainer(v)
}

// renderConfigContent renders the config view of one node.
//
// EVERY node kind reaches markup, so there is no empty-panel branch here.
// configViewComponent warns and falls back to configContainer for a kind it
// does not name.
func renderConfigContent(renderer *Renderer, v *ConfigViewData) template.HTML {
	return renderer.renderComponent("config view", configViewComponent(v.NodeKind, v))
}

// nodeKindString returns a human-readable kind string for display.
func nodeKindString(kind config.NodeKind) string {
	switch kind {
	case config.NodeContainer:
		return nodeKindNameContainer
	case config.NodeList:
		return nodeKindNameList
	case config.NodeLeaf:
		return nodeKindNameLeaf
	case config.NodeFreeform:
		return nodeKindNameFreeform
	case config.NodeFlex:
		return nodeKindNameFlex
	case config.NodeInlineList:
		return nodeKindNameInlineList
	}

	return nodeKindNameUnknown
}

// isBoolLeaf returns true if the named leaf at the given schema path has
// TypeBool. Returns false if the path or leaf cannot be resolved, or if
// the node is not a LeafNode.
func isBoolLeaf(schema *config.Schema, path []string, leaf string) bool {
	fullPath := make([]string, len(path)+1)
	copy(fullPath, path)
	fullPath[len(path)] = leaf
	node, err := walkSchema(schema, fullPath)
	if err != nil {
		return false
	}

	leafNode, ok := node.(*config.LeafNode)
	if !ok {
		return false
	}

	return leafNode.Type == config.TypeBool
}

// findLeafNode walks the schema to find a LeafNode at path + leaf.
func findLeafNode(schema *config.Schema, path []string, leaf string) *config.LeafNode {
	fullPath := make([]string, len(path)+1)
	copy(fullPath, path)
	fullPath[len(path)] = leaf

	node, err := walkSchema(schema, fullPath)
	if err != nil {
		return nil
	}

	leafNode, ok := node.(*config.LeafNode)
	if !ok {
		return nil
	}

	return leafNode
}

// buildFieldMetaFromLeaf creates a FieldMeta from a leaf name, YANG node, value, and parent path.
func buildFieldMetaFromLeaf(name string, leaf *config.LeafNode, value, parentPath string) FieldMeta {
	return buildFieldMeta(name, leaf, value, value != "", parentPath)
}
