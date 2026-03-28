// Design: docs/architecture/web-interface.md -- Leaf input type and template helpers
// Overview: handler_config.go -- Config tree view handlers
// Related: handler_config_walk.go -- Schema and config tree walking

package web

import (
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
)

// buildBreadcrumbs creates breadcrumb navigation entries. The root segment
// is named "/" and links to /show/. Each path segment links to
// /show/<path-up-to-here>/.
func buildBreadcrumbs(path []string) []BreadcrumbSegment {
	crumbs := make([]BreadcrumbSegment, 0, 1+len(path))
	crumbs = append(crumbs, BreadcrumbSegment{Name: "ze", URL: "/show/", Active: len(path) == 0})

	for i, seg := range path {
		url := "/show/" + strings.Join(path[:i+1], "/") + "/"
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
	info.Value = value
	info.Default = leaf.Default
	info.IsConfigured = configured

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
	}

	return LeafField{InputType: "text"}
}

// nodeKindToTemplate maps a NodeKind to the template file name used for rendering.
func nodeKindToTemplate(kind config.NodeKind) string {
	switch kind {
	case config.NodeContainer:
		return "container.html"
	case config.NodeList:
		return "list.html"
	case config.NodeLeaf:
		return "leaf.html"
	case config.NodeFreeform:
		return "freeform.html"
	case config.NodeFlex:
		return "flex.html"
	case config.NodeInlineList:
		return "inline_list.html"
	}

	return "container.html"
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
