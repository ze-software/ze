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
		url := "/show/" + textbuf.Join(path[:i+1], "/") + "/"
		crumbs = append(crumbs, BreadcrumbSegment{
			Name:   seg,
			URL:    url,
			Active: i == len(path)-1,
		})
	}

	return crumbs
}

// buildLeafField maps a LeafNode to an HTML input field description.
func buildLeafField(name string, leaf *config.LeafNode, value string, configured bool) LeafField {
	info := leafInputType(leaf.Type)
	info.Name = name
	// Never prefill the form input with a stored ze:bcrypt hash: mask it. A new
	// password is set via the plaintext-<name> sibling; a resubmitted placeholder
	// is filtered on the write path (EditorManager.SetValue).
	if leaf.Bcrypt && value != "" {
		value = config.SecretDataPlaceholder
	}
	info.Value = value
	info.Default = leaf.Default
	info.IsConfigured = configured
	info.Description = leaf.Description
	if len(leaf.Enums) > 0 {
		info.InputType = "select"
		info.Options = leaf.Enums
	}

	return info
}

// leafInputType returns the HTML input attributes for a given ValueType.
func leafInputType(vt config.ValueType) LeafField {
	switch vt {
	case config.TypeString:
		return LeafField{InputType: "text"}

	case config.TypeBool:
		return LeafField{InputType: "checkbox"}

	case config.TypeUint16:
		return LeafField{InputType: "number", Min: "0", Max: "65535"}

	case config.TypeUint32:
		return LeafField{InputType: "number", Min: "0", Max: "4294967295"}

	case config.TypeIPv4:
		return LeafField{
			InputType:   "text",
			Pattern:     `^(\d{1,3}\.){3}\d{1,3}$`,
			Placeholder: "e.g., 192.0.2.1",
		}

	case config.TypeIPv6:
		return LeafField{
			InputType:   "text",
			Pattern:     `^[0-9a-fA-F:]+$`,
			Placeholder: "e.g., 2001:db8::1",
		}

	case config.TypeIP:
		return LeafField{InputType: "text", Placeholder: "IPv4 or IPv6 address"}

	case config.TypePrefix:
		return LeafField{
			InputType:   "text",
			Pattern:     `^[0-9a-fA-F.:]+/\d{1,3}$`,
			Placeholder: "e.g., 10.0.0.0/24",
		}

	case config.TypeDuration:
		return LeafField{InputType: "text", Placeholder: "e.g., 5s, 100ms"}

	case config.TypeInt:
		return LeafField{InputType: "number"}
	case config.TypeEmpty:
		return LeafField{InputType: "text"}
	}

	return LeafField{InputType: "text"}
}

// configViewComponent resolves the component that renders one config node.
//
// This replaced a lookup by template name. The renderer held a map keyed on
// that name. A name it did not hold rendered nothing. Two of the six names this
// function used to answer reach no markup, and the config editor answers both
// with a blank panel:
//
//   - config.NodeLeaf named leaf.html, which is in neither the embedded tree
//     nor the parsed set.
//   - config.NodeFlex named flex.html, whose markup reads Name, Value and
//     LeafField. The caller passes a ConfigViewData, which carries none.
//
// Both are recorded in plan/journal/silent-fall-through.md and neither is fixed
// here: this phase ports markup and changes no rendered byte. They answer nil,
// which renderConfigContent reports at debug rather than swallowing.
func configViewComponent(kind config.NodeKind, v *ConfigViewData) templ.Component {
	switch kind {
	case config.NodeList:
		return configList(v)
	case config.NodeFreeform:
		return configFreeform(v)
	case config.NodeInlineList:
		return configInlineList(v)
	case config.NodeLeaf, config.NodeFlex:
		return nil
	case config.NodeContainer:
		return configContainer(v)
	}

	return configContainer(v)
}

// renderConfigContent renders the config view of one node. A node kind with no
// component of its own yields empty markup, which is what the operator saw
// before the port. It says so in the log.
//
// AT DEBUG, because a leaf is a routine view. Two of the six node kinds reach
// no markup. A warning here is one line for each such view, on a path an
// operator walks all day. The defect belongs to
// plan/journal/silent-fall-through.md, and a log level cannot fix it.
func renderConfigContent(renderer *Renderer, v *ConfigViewData) template.HTML {
	component := configViewComponent(v.NodeKind, v)
	if component == nil {
		serverLogger.Debug("config node kind has no view component",
			"kind", nodeKindString(v.NodeKind), "path", v.CurrentPath)

		return ""
	}

	return renderer.renderComponent("config view", component)
}

// nodeKindString returns a human-readable kind string for display.
func nodeKindString(kind config.NodeKind) string {
	switch kind {
	case config.NodeContainer:
		return "container"
	case config.NodeList:
		return "list"
	case config.NodeLeaf:
		return "leaf"
	case config.NodeFreeform:
		return "freeform"
	case config.NodeFlex:
		return "flex"
	case config.NodeInlineList:
		return "inline-list"
	}

	return "unknown"
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
