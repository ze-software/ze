// Design: docs/architecture/web-interface.md -- Config tree view handlers
// Related: handler.go -- URL routing and content negotiation
// Related: render.go -- Template rendering

package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
)

// schemaGetter is the interface shared by Schema, ContainerNode, ListNode,
// FlexNode, and InlineListNode for walking the schema tree.
type schemaGetter interface {
	Get(name string) config.Node
}

// childLister extends schemaGetter with ordered child enumeration.
type childLister interface {
	schemaGetter
	Children() []string
}

// ConfigViewData holds all data needed for any config template.
type ConfigViewData struct {
	// Path is the current YANG path segments.
	Path []string
	// Breadcrumbs is the navigation trail from root to current node.
	Breadcrumbs []BreadcrumbSegment
	// NodeKind is the schema node kind at this path.
	NodeKind config.NodeKind
	// Children lists sub-entries for containers (non-leaf children as links).
	Children []ChildEntry
	// Keys lists key strings for list nodes.
	Keys []string
	// SelectedKey is the currently selected list key, if any.
	SelectedKey string
	// SelectedDetail holds the detail view for a selected list entry.
	SelectedDetail *ConfigViewData
	// LeafFields holds input field data for leaf nodes within a container or entry.
	LeafFields []LeafField
	// Entries holds freeform node entries.
	Entries []string
}

// ChildEntry represents a child node in a container view.
type ChildEntry struct {
	Name string
	Kind string // "container", "list", "leaf"
	URL  string
}

// LeafField holds the data for rendering a leaf input field.
type LeafField struct {
	Name         string
	Value        string // configured value, or ""
	Default      string // YANG default, or ""
	InputType    string // "text", "checkbox", "number", "select"
	Placeholder  string
	Description  string // from YANG, if available
	Pattern      string // for text inputs
	Min          string // for number inputs
	Max          string // for number inputs
	Options      []string
	IsConfigured bool
	ReadOnly     bool
}

// walkConfigPath walks both schema and config tree in parallel to the given
// path. Returns the schema node at the end of the path, the config subtree
// at that position (nil if not configured), and any error from schema lookup.
func walkConfigPath(schema *config.Schema, tree *config.Tree, path []string) (config.Node, *config.Tree, error) {
	schemaNode, err := walkSchema(schema, path)
	if err != nil {
		return nil, nil, err
	}

	subtree := walkTree(tree, schema, path)

	return schemaNode, subtree, nil
}

// errEmptyPath is returned when walkSchema is called with an empty path.
var errEmptyPath = fmt.Errorf("empty schema path")

// walkSchema walks the schema tree following path segments and returns the
// node at the path end. List keys consume 2 path segments (name + key value),
// except when the next segment is a valid child of the list (anonymous entry).
// Returns errEmptyPath when path has no segments.
func walkSchema(schema *config.Schema, path []string) (config.Node, error) {
	if len(path) == 0 {
		return nil, errEmptyPath
	}

	var current schemaGetter = schema

	i := 0
	var lastNode config.Node

	for i < len(path) {
		name := path[i]
		node := current.Get(name)
		if node == nil {
			return nil, fmt.Errorf("unknown schema path element: %q", name)
		}

		switch n := node.(type) {
		case *config.ContainerNode:
			current = n
			lastNode = n
			i++

		case *config.ListNode:
			// Anonymous vs keyed: anonymous if no next element or next
			// element is a valid child of the list.
			if i+1 >= len(path) || n.Get(path[i+1]) != nil {
				i++
			} else {
				i += 2
			}
			current = n
			lastNode = n

		case *config.FlexNode:
			current = n
			lastNode = n
			i++

		case *config.InlineListNode:
			if i+1 >= len(path) || n.Get(path[i+1]) != nil {
				i++
			} else {
				i += 2
			}
			current = n
			lastNode = n

		case *config.FreeformNode:
			lastNode = n
			i++

		case *config.LeafNode:
			lastNode = n
			i++

		case *config.MultiLeafNode, *config.BracketLeafListNode, *config.ValueOrArrayNode:
			// Terminal leaf-like nodes that cannot be walked into further.
			lastNode = n
			i++
		}
	}

	return lastNode, nil
}

// walkTree walks the config tree following the same pattern as walkSchema,
// returning the subtree at the given path or nil if the path does not exist
// in the configured data.
func walkTree(tree *config.Tree, schema *config.Schema, path []string) *config.Tree {
	if tree == nil || len(path) == 0 {
		return tree
	}

	current := tree
	var currentSchema schemaGetter = schema

	i := 0
	for i < len(path) {
		name := path[i]
		schemaNode := currentSchema.Get(name)
		if schemaNode == nil {
			return nil
		}

		switch n := schemaNode.(type) {
		case *config.ContainerNode:
			child := current.GetContainer(name)
			if child == nil {
				return nil
			}
			current = child
			currentSchema = n
			i++

		case *config.ListNode:
			var key string
			var step int
			if i+1 >= len(path) || n.Get(path[i+1]) != nil {
				key = config.KeyDefault
				step = 1
			} else {
				key = path[i+1]
				step = 2
			}
			entries := current.GetList(name)
			if entries == nil {
				return nil
			}
			entry := entries[key]
			if entry == nil {
				return nil
			}
			current = entry
			currentSchema = n
			i += step

		case *config.FlexNode:
			child := current.GetContainer(name)
			if child == nil {
				return nil
			}
			current = child
			currentSchema = n
			i++

		case *config.InlineListNode:
			var key string
			var step int
			if i+1 >= len(path) || n.Get(path[i+1]) != nil {
				key = config.KeyDefault
				step = 1
			} else {
				key = path[i+1]
				step = 2
			}
			entries := current.GetList(name)
			if entries == nil {
				return nil
			}
			entry := entries[key]
			if entry == nil {
				return nil
			}
			current = entry
			currentSchema = n
			i += step

		case *config.LeafNode, *config.FreeformNode,
			*config.MultiLeafNode, *config.BracketLeafListNode, *config.ValueOrArrayNode:
			// Terminal nodes: tree walk cannot descend further.
			return current
		}
	}

	return current
}

// buildConfigViewData walks the schema and tree to the given path, determines
// the node kind, and assembles template data for the config view.
// For containers and list entries, leaves are placed in LeafFields and
// non-leaf children are placed in Children.
func buildConfigViewData(schema *config.Schema, tree *config.Tree, path []string) (*ConfigViewData, error) {
	data := &ConfigViewData{
		Path:        path,
		Breadcrumbs: buildBreadcrumbs(path),
	}

	// Root view: list top-level schema children as containers.
	if len(path) == 0 {
		data.NodeKind = config.NodeContainer
		for _, name := range schema.Children() {
			node := schema.Get(name)
			data.Children = append(data.Children, ChildEntry{
				Name: name,
				Kind: nodeKindString(node.Kind()),
				URL:  "/show/" + name + "/",
			})
		}
		return data, nil
	}

	schemaNode, err := walkSchema(schema, path)
	if err != nil {
		return nil, err
	}
	if schemaNode == nil {
		return nil, fmt.Errorf("schema node not found at path: %s", strings.Join(path, "/"))
	}

	data.NodeKind = schemaNode.Kind()
	prefix := "/show/" + strings.Join(path, "/")

	switch n := schemaNode.(type) {
	case *config.ContainerNode:
		subtree := walkTree(tree, schema, path)
		populateContainerView(data, n, subtree, prefix)

	case *config.ListNode:
		// When the path ends at the list itself (no key selected),
		// show the list of keys.
		data.Keys = collectListKeys(tree, schema, path)

		// When we have a list node and the path included a key (walkTree
		// descended into an entry), also populate leaf fields.
		subtree := walkTree(tree, schema, path)
		if subtree != nil {
			populateContainerView(data, n, subtree, prefix)
		}

	case *config.FlexNode:
		subtree := walkTree(tree, schema, path)
		populateContainerView(data, n, subtree, prefix)

	case *config.InlineListNode:
		data.Keys = collectListKeys(tree, schema, path)

	case *config.LeafNode:
		subtree := walkTree(tree, schema, path[:len(path)-1])
		leafName := path[len(path)-1]
		var value string
		var configured bool
		if subtree != nil {
			value, configured = subtree.Get(leafName)
		}
		data.LeafFields = []LeafField{buildLeafField(leafName, n, value, configured)}

	case *config.FreeformNode:
		subtree := walkTree(tree, schema, path)
		if subtree != nil {
			data.Entries = subtree.Values()
		}

	case *config.MultiLeafNode, *config.BracketLeafListNode, *config.ValueOrArrayNode:
		// Leaf-like terminal nodes: no further navigation.

		// No default: all config.Node implementations are handled above.
	}

	return data, nil
}

// populateContainerView fills LeafFields and Children from a container-like
// schema node. Leaves go into LeafFields (with values from the config tree),
// non-leaves go into Children as navigation links.
func populateContainerView(data *ConfigViewData, provider childLister, subtree *config.Tree, prefix string) {
	for _, name := range provider.Children() {
		child := provider.Get(name)

		if leaf, ok := child.(*config.LeafNode); ok {
			var value string
			var configured bool
			if subtree != nil {
				value, configured = subtree.Get(name)
			}
			data.LeafFields = append(data.LeafFields, buildLeafField(name, leaf, value, configured))
			continue
		}

		data.Children = append(data.Children, ChildEntry{
			Name: name,
			Kind: nodeKindString(child.Kind()),
			URL:  prefix + "/" + name + "/",
		})
	}
}

// collectListKeys reads the configured list keys from the tree for the list
// at the end of path. Returns key strings in insertion order.
func collectListKeys(tree *config.Tree, schema *config.Schema, path []string) []string {
	var parentTree *config.Tree
	listName := path[len(path)-1]

	if len(path) == 1 {
		parentTree = tree
	} else {
		parentTree = walkTree(tree, schema, path[:len(path)-1])
	}

	if parentTree == nil {
		return nil
	}

	ordered := parentTree.GetListOrdered(listName)
	keys := make([]string, 0, len(ordered))
	for _, kv := range ordered {
		keys = append(keys, kv.Key)
	}

	return keys
}

// buildBreadcrumbs creates breadcrumb navigation entries. The root segment
// is named "/" and links to /show/. Each path segment links to
// /show/<path-up-to-here>/.
func buildBreadcrumbs(path []string) []BreadcrumbSegment {
	crumbs := make([]BreadcrumbSegment, 0, 1+len(path))
	crumbs = append(crumbs, BreadcrumbSegment{Name: "/", URL: "/show/", Active: len(path) == 0})

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

// HandleConfigView returns an HTTP handler that serves the config tree view.
// It parses the URL path (stripping the /show/ prefix), walks both schema and
// tree, and renders the appropriate template. JSON responses return the subtree
// as a map. HTMX partial requests (HX-Request header) return the content
// fragment without the layout wrapper.
func HandleConfigView(renderer *Renderer, schema *config.Schema, tree *config.Tree) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		parsed, err := ParseURL(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		path := parsed.Path

		// JSON response: return tree data as JSON map.
		if parsed.Format == formatJSON {
			subtree := walkTree(tree, schema, path)
			var data map[string]any
			if subtree != nil {
				data = subtree.ToMap()
			}
			if data == nil {
				data = make(map[string]any)
			}

			w.Header().Set("Content-Type", "application/json")

			if err := json.NewEncoder(w).Encode(data); err != nil {
				http.Error(w, fmt.Sprintf("json encode: %v", err), http.StatusInternalServerError)
			}

			return
		}

		// HTML response: build view data and render template.
		viewData, err := buildConfigViewData(schema, tree, path)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		_ = nodeKindToTemplate(viewData.NodeKind)

		// HTMX partial: return content fragment without layout wrapper.
		if r.Header.Get("HX-Request") == "true" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			// Partial content for HTMX requests. Full template rendering
			// will be wired when config templates are created.
			if _, err := fmt.Fprintf(w, "<div>%s</div>", strings.Join(path, " / ")); err != nil {
				http.Error(w, fmt.Sprintf("write partial: %v", err), http.StatusInternalServerError)
			}
			return
		}

		// Full HTML: render inside layout.
		layoutData := LayoutData{
			Title:      "Config: /" + strings.Join(path, "/"),
			Breadcrumb: viewData.Breadcrumbs,
		}

		if err := renderer.RenderLayout(w, layoutData); err != nil {
			http.Error(w, fmt.Sprintf("render: %v", err), http.StatusInternalServerError)
		}
	}
}
