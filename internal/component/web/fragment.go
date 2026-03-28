// Design: docs/architecture/web-components.md -- HTMX fragment handlers
// Related: handler_config.go -- Config view data types
// Related: handler_config_walk.go -- Schema and tree walking
// Related: render.go -- Template rendering

package web

import (
	"fmt"
	"net/http"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
)

// FieldMeta carries YANG metadata for a single field, rendered as data-* attributes
// so the browser JS can construct type-appropriate inputs without server round-trips.
type FieldMeta struct {
	Leaf        string // YANG leaf name
	Path        string // Parent YANG path for POST target
	Type        string // bool, string, enum, uint16, uint32, int, ip, prefix, duration
	Value       string // Current configured value
	Default     string // YANG default
	Description string // YANG description
	Options     string // Comma-separated enum values
	Min         string // Numeric min
	Max         string // Numeric max
	Pattern     string // Validation regex
}

// ErrorData holds the data for rendering an error item via the oob_error template.
type ErrorData struct {
	ID      string
	Path    string
	Message string
}

// WriteOOBError sends an error as an HTMX OOB swap appended to #error-list
// and opens the error panel. Renders via the oob_error template.
func WriteOOBError(w http.ResponseWriter, renderer *Renderer, path, message string, status int) {
	data := ErrorData{
		ID:      fmt.Sprintf("%d", len(message)+len(path)),
		Path:    path,
		Message: message,
	}
	html := renderer.RenderFragment("oob_error", data)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if _, writeErr := w.Write([]byte(html)); writeErr != nil {
		return // client disconnected
	}
}

// GetType returns the field type string for template dispatch.
func (f FieldMeta) GetType() string { return f.Type }

// SidebarEntry is one item in a sidebar section.
type SidebarEntry struct {
	Key      string
	URL      string
	HxPath   string // YANG path for hx-get
	Selected bool
}

// SidebarSection represents one level in the left sidebar hierarchy.
// For containers it shows just the name. For lists it shows entries + add.
type SidebarSection struct {
	Name        string         // Node name (e.g., "peer")
	Description string         // YANG description for tooltip
	URL         string         // Click navigates to this level
	HxPath      string         // YANG path for hx-get
	IsList      bool           // True for list nodes (show entries)
	Entries     []SidebarEntry // List entries at this level
	AddURL      string         // Base URL for add form (lists only)
	Selected    string         // Currently selected entry key
}

// ColumnItem is a single row in a Finder column.
type ColumnItem struct {
	Name        string // Display name
	URL         string // Navigation URL
	HxPath      string // YANG path for hx-get
	Selected    bool   // Currently selected (highlighted)
	HasChildren bool   // Shows > indicator (has sub-containers/lists)
	IsList      bool   // This item is a list node (shows entry count)
	Count       int    // Number of list entries (when IsList)
	AddURL      string // Base URL for add-entry overlay (lists only)
}

// FinderColumn is one column in the Finder-style navigation.
type FinderColumn struct {
	Items []ColumnItem
}

// FragmentData holds all data needed to render any fragment.
type FragmentData struct {
	// Path segments for the current view.
	Path []string
	// CurrentPath is the joined path for URLs.
	CurrentPath string
	// Children are navigation tiles (containers, lists).
	Children []ChildEntry
	// Fields are leaf nodes with YANG metadata.
	Fields []FieldMeta
	// Sidebar is the hierarchical navigation tree for the left panel (legacy).
	Sidebar []SidebarSection
	// Columns is the Finder-style column navigation (up to 3 visible).
	Columns []FinderColumn
	// ParentURL is the URL of the parent node (empty at root).
	ParentURL string
	// ParentHxPath is the YANG path for the parent (for hx-get).
	ParentHxPath string
	// Breadcrumbs for the breadcrumb fragment.
	Breadcrumbs []BreadcrumbSegment
	// HasSession is true when an authenticated session exists (for breadcrumb template).
	HasSession bool
	// Username is the authenticated user's name (for display in breadcrumb).
	Username string
	// Insecure is true when --insecure-web mode is active.
	Insecure bool
	// CommandForm holds admin command form data when viewing a leaf command.
	CommandForm *CommandFormData
	// CommandResult holds admin command execution result.
	CommandResult *CommandResultData
}

// HandleFragment returns an HTTP handler that serves HTMX fragments.
// A full page request renders the layout with all fragments embedded.
// An HTMX request (HX-Request header) returns only the requested fragment
// with out-of-band swaps for sidebar and breadcrumb.
func HandleFragment(renderer *Renderer, schema *config.Schema, tree *config.Tree, mgr *EditorManager, insecure bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := extractPath(r)
		if err := ValidatePathSegments(path); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		// Use the editor's working tree if the user has an active session,
		// so config changes (set values, created entries) are visible.
		viewTree := tree
		username := GetUsernameFromRequest(r)
		if mgr != nil && username != "" {
			if userTree := mgr.Tree(username); userTree != nil {
				viewTree = userTree
			}
		}
		data := buildFragmentData(schema, viewTree, path)
		data.Username = username
		data.Insecure = insecure

		// HTMX partial request: render OOB response via template.
		if r.Header.Get("HX-Request") == "true" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			html := renderer.RenderFragment("oob_response", data)
			if _, writeErr := w.Write([]byte(html)); writeErr != nil {
				return // client disconnected
			}
			return
		}

		// Full page: render all fragments via templates.
		content := renderer.RenderFragment("full_content", data)

		prompt := "/" + data.CurrentPath
		if prompt != "/" {
			prompt += "/"
		}
		prompt += ">"

		layoutData := LayoutData{
			Title:       "Ze: /" + data.CurrentPath,
			Content:     content,
			HasSession:  true,
			CLIPrompt:   prompt,
			Breadcrumbs: data.Breadcrumbs,
			Username:    data.Username,
			Insecure:    insecure,
		}

		if err := renderer.RenderLayout(w, layoutData); err != nil {
			http.Error(w, fmt.Sprintf("render: %v", err), http.StatusInternalServerError)
		}
	}
}

// extractPath parses the YANG path from the URL, handling both /show/X and ?path=X.
func extractPath(r *http.Request) []string {
	// Fragment endpoint: ?path=bgp/peer/1.2.3.4
	if r.URL.Query().Has("path") {
		p := r.URL.Query().Get("path")
		if p == "" {
			return nil
		}
		return splitPath(p)
	}

	// Show endpoint: /show/bgp/peer/1.2.3.4
	raw := strings.TrimPrefix(r.URL.Path, "/show/")
	raw = strings.TrimPrefix(raw, "/fragment/detail/")
	raw = strings.TrimSuffix(raw, "/")

	if raw == "" {
		return nil
	}

	return splitPath(raw)
}

func splitPath(s string) []string {
	s = strings.TrimPrefix(s, "/")
	s = strings.TrimSuffix(s, "/")
	if s == "" {
		return nil
	}
	return strings.Split(s, "/")
}

// buildFragmentData walks the schema/tree and assembles data for all fragments.
func buildFragmentData(schema *config.Schema, tree *config.Tree, path []string) *FragmentData {
	data := &FragmentData{
		Path:        path,
		CurrentPath: strings.Join(path, "/"),
		Breadcrumbs: buildBreadcrumbs(path),
		HasSession:  true, // Fragment handler is always behind auth middleware.
	}

	// Build sidebar for all levels including root.
	data.Sidebar = buildSidebarHierarchy(schema, tree, path)
	// Build Finder-style column navigation.
	data.Columns = buildFinderColumns(schema, tree, path)

	// Parent URL for back navigation in sidebar.
	if len(path) > 0 {
		parentPath := path[:len(path)-1]
		data.ParentHxPath = strings.Join(parentPath, "/")
		if len(parentPath) == 0 {
			data.ParentURL = "/show/"
		} else {
			data.ParentURL = "/show/" + strings.Join(parentPath, "/") + "/"
		}
	}

	// Root: no detail fields, just sidebar navigation.
	if len(path) == 0 {
		return data
	}

	schemaNode, err := walkSchema(schema, path)
	if err != nil || schemaNode == nil {
		return data
	}

	prefix := "/show/" + strings.Join(path, "/")

	switch n := schemaNode.(type) {
	case *config.ContainerNode:
		subtree := walkTree(tree, schema, path)
		populateFragmentFields(data, n, subtree, prefix)

	case *config.ListNode:
		// List at the leaf of the path: entries shown in sidebar (built above).
		subtree := walkTree(tree, schema, path)
		if subtree != nil {
			populateFragmentFields(data, n, subtree, prefix)
		}

	case *config.FlexNode:
		subtree := walkTree(tree, schema, path)
		populateFragmentFields(data, n, subtree, prefix)

	case *config.LeafNode:
		parentPath := strings.Join(path[:len(path)-1], "/")
		data.Fields = []FieldMeta{buildFieldMeta(path[len(path)-1], n, "", false, parentPath)}

	case *config.FreeformNode, *config.InlineListNode,
		*config.MultiLeafNode, *config.BracketLeafListNode, *config.ValueOrArrayNode:
		// Terminal nodes without further navigation.
	}

	return data
}

// populateFragmentFields fills Children and Fields from a container-like node.
func populateFragmentFields(data *FragmentData, provider childLister, subtree *config.Tree, prefix string) {
	for _, name := range provider.Children() {
		child := provider.Get(name)

		if leaf, ok := child.(*config.LeafNode); ok {
			var value string
			var configured bool
			if subtree != nil {
				value, configured = subtree.Get(name)
			}
			data.Fields = append(data.Fields, buildFieldMeta(name, leaf, value, configured, data.CurrentPath))
			continue
		}

		childURL := prefix + "/" + name + "/"
		data.Children = append(data.Children, ChildEntry{
			Name:   name,
			Kind:   nodeKindString(child.Kind()),
			URL:    childURL,
			HxPath: strings.TrimPrefix(strings.TrimSuffix(childURL, "/"), "/show/"),
		})
	}
}

// buildFieldMeta creates a FieldMeta from a LeafNode with full YANG metadata.
func buildFieldMeta(name string, leaf *config.LeafNode, value string, _ bool, parentPath string) FieldMeta {
	meta := FieldMeta{
		Leaf:        name,
		Path:        parentPath,
		Value:       value,
		Default:     leaf.Default,
		Description: leaf.Description,
	}

	meta.Type = valueTypeToFieldType(leaf.Type)

	switch leaf.Type {
	case config.TypeUint16:
		meta.Min = "0"
		meta.Max = "65535"
	case config.TypeUint32:
		meta.Min = "0"
		meta.Max = "4294967295"
	case config.TypeIPv4:
		meta.Pattern = `^(\d{1,3}\.){3}\d{1,3}$`
	case config.TypePrefix:
		meta.Pattern = `^[0-9a-fA-F.:]+/\d{1,3}$`
	case config.TypeString, config.TypeBool, config.TypeInt,
		config.TypeIPv6, config.TypeIP, config.TypeDuration:
		// No extra metadata needed.
	}

	if len(leaf.Enums) > 0 {
		meta.Type = "enum"
		meta.Options = strings.Join(leaf.Enums, ",")
	}

	return meta
}

// valueTypeToFieldType maps config.ValueType to the field type string for data-type.
func valueTypeToFieldType(vt config.ValueType) string {
	switch vt {
	case config.TypeBool:
		return "bool"
	case config.TypeUint16:
		return "uint16"
	case config.TypeUint32:
		return "uint32"
	case config.TypeInt:
		return "int"
	case config.TypeIPv4, config.TypeIPv6, config.TypeIP:
		return "ip"
	case config.TypePrefix:
		return "prefix"
	case config.TypeDuration:
		return "duration"
	case config.TypeString:
		return "string"
	}
	return "string"
}

// nodeDescription extracts the YANG description from a schema node, if available.
func nodeDescription(n config.Node) string {
	switch v := n.(type) {
	case *config.ContainerNode:
		return v.Description
	case *config.ListNode:
		return v.Description
	case *config.LeafNode:
		return v.Description
	}
	return ""
}

// buildSidebarHierarchy creates sidebar sections for the current node's children.
// Lists show their entries for selection. Containers show as clickable headings.
func buildSidebarHierarchy(schema *config.Schema, tree *config.Tree, path []string) []SidebarSection {
	var sections []SidebarSection

	// Determine what node we're looking at and its children.
	var provider childLister

	if len(path) == 0 {
		provider = schema
	} else {
		node, err := walkSchema(schema, path)
		if err != nil || node == nil {
			return nil
		}
		if cl, ok := node.(childLister); ok {
			provider = cl
		}
	}

	if provider == nil {
		return nil
	}

	// For each child, create a sidebar section.
	for _, name := range provider.Children() {
		child := provider.Get(name)
		childPath := append(append([]string{}, path...), name)
		url := "/show/" + strings.Join(childPath, "/") + "/"

		hxPath := strings.Join(childPath, "/")
		section := SidebarSection{
			Name:        name,
			Description: nodeDescription(child),
			URL:         url,
			HxPath:      hxPath,
		}

		if _, ok := child.(*config.ListNode); ok {
			section.IsList = true
			section.AddURL = url

			keys := collectListKeys(tree, schema, childPath)
			for _, k := range keys {
				entryPath := hxPath + "/" + k
				section.Entries = append(section.Entries, SidebarEntry{
					Key:    k,
					URL:    url + k + "/",
					HxPath: entryPath,
				})
			}
		}

		// Only include containers and lists in the sidebar (not leaves).
		if child.Kind() != config.NodeLeaf {
			sections = append(sections, section)
		}
	}

	return sections
}

// hasNonLeafChildren returns true if the node has at least one child that
// is a container, list, or other navigable node (not a leaf).
func hasNonLeafChildren(node config.Node) bool {
	cl, ok := node.(childLister)
	if !ok {
		return false
	}
	for _, name := range cl.Children() {
		child := cl.Get(name)
		if child != nil && child.Kind() != config.NodeLeaf {
			return true
		}
	}
	return false
}

// buildFinderColumns builds the Finder-style column navigation.
// For a path like ["bgp", "peer", "thomas"], it produces columns:
//
//	col 0: root children (env, system, plugin, bgp*, telemetry)
//	col 1: bgp children (community, filter, group, local, peer*)
//	col 2: peer entries (thomas*, alice, [+ new])
//
// Only the last 3 columns are kept visible.
func buildFinderColumns(schema *config.Schema, tree *config.Tree, path []string) []FinderColumn {
	var columns []FinderColumn

	// Build a column for each level from root to current.
	for depth := 0; depth <= len(path); depth++ {
		prefix := path[:depth]
		var selectedName string
		if depth < len(path) {
			selectedName = path[depth]
		}

		col := buildColumnAt(schema, tree, prefix, selectedName)
		if col != nil {
			columns = append(columns, *col)
		}
	}

	// Keep at most 3 columns visible.
	if len(columns) > 3 {
		columns = columns[len(columns)-3:]
	}

	return columns
}

// buildColumnAt builds one Finder column showing children of the node at prefix.
func buildColumnAt(schema *config.Schema, tree *config.Tree, prefix []string, selectedName string) *FinderColumn {
	var provider childLister

	if len(prefix) == 0 {
		provider = schema
	} else {
		node, err := walkSchema(schema, prefix)
		if err != nil || node == nil {
			return nil
		}
		// If this is a list node, show entries instead of schema children.
		if listNode, ok := node.(*config.ListNode); ok {
			return buildListColumn(tree, schema, prefix, listNode, selectedName)
		}
		cl, ok := node.(childLister)
		if !ok {
			return nil
		}
		provider = cl
	}

	col := &FinderColumn{}
	for _, name := range provider.Children() {
		child := provider.Get(name)
		if child == nil || child.Kind() == config.NodeLeaf {
			continue
		}

		childPath := append(append([]string{}, prefix...), name)
		url := "/show/" + strings.Join(childPath, "/") + "/"
		hxPath := strings.Join(childPath, "/")

		item := ColumnItem{
			Name:     name,
			URL:      url,
			HxPath:   hxPath,
			Selected: name == selectedName,
		}

		if _, ok := child.(*config.ListNode); ok {
			item.IsList = true
			item.HasChildren = true
			item.AddURL = url
			item.Count = len(collectListKeys(tree, schema, childPath))
		} else {
			item.HasChildren = hasNonLeafChildren(child)
		}

		col.Items = append(col.Items, item)
	}

	return col
}

// buildListColumn builds a column showing list entries (e.g., peer names).
func buildListColumn(tree *config.Tree, schema *config.Schema, prefix []string, _ *config.ListNode, selectedName string) *FinderColumn {
	col := &FinderColumn{}
	keys := collectListKeys(tree, schema, prefix)
	url := "/show/" + strings.Join(prefix, "/") + "/"

	for _, k := range keys {
		entryPath := strings.Join(prefix, "/") + "/" + k
		col.Items = append(col.Items, ColumnItem{
			Name:        k,
			URL:         url + k + "/",
			HxPath:      entryPath,
			Selected:    k == selectedName,
			HasChildren: true,
		})
	}

	// Add "+ new" entry.
	col.Items = append(col.Items, ColumnItem{
		Name:   "+ new",
		IsList: true,
		AddURL: url,
	})

	return col
}
