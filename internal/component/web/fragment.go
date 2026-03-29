// Design: docs/architecture/web-components.md -- HTMX fragment handlers
// Related: handler_config.go -- Config view data types
// Related: handler_config_walk.go -- Schema and tree walking
// Related: render.go -- Template rendering

package web

import (
	"fmt"
	"net/http"
	"net/url"
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

// ListTableColumn describes one column in a list table view.
type ListTableColumn struct {
	Name string // Display name (leaf name or dotted path like "remote/ip")
	Key  bool   // True for the list key column (clickable, navigates)
}

// ListTableCell is one editable cell in a list table row.
type ListTableCell struct {
	Value       string // Current value
	Leaf        string // Leaf name for the set command (e.g., "ip")
	Path        string // Full YANG path for hx-post (e.g., "bgp/peer/london/remote")
	Placeholder string // YANG description or type hint for empty cells
}

// ListTableRow is one entry row in a list table.
type ListTableRow struct {
	KeyValue string          // The list key value (e.g., peer name)
	Cells    []ListTableCell // Editable cells (same order as non-key columns)
	URL      string          // Navigation URL for this entry
	HxPath   string          // YANG path for hx-get
}

// ListTableView holds data for rendering a multi-key list as a table.
type ListTableView struct {
	Name    string            // List name (e.g., "peer")
	Columns []ListTableColumn // Column definitions
	Rows    []ListTableRow    // Entry rows
	AddURL  string            // Base URL for adding entries (/config/add/bgp/peer/)
	FormURL string            // URL for the HTMX add-form overlay (/config/add-form/bgp/peer/)
	SetURL  string            // Base URL for config/set (inline edits)
}

// FinderColumn is one column in the Finder-style navigation.
type FinderColumn struct {
	NamedItems   []ColumnItem // Named containers (lists with keys) - shown first
	UnnamedItems []ColumnItem // Unnamed containers (no list key) - shown after separator
}

// ContextEntry is one named node in the context heading (e.g., ListName="peer", Key="thomas").
type ContextEntry struct {
	ListName string
	Key      string
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
	// Monitor is true when the view is auto-refreshing (/monitor/ URL).
	Monitor bool
	// ContextHeading shows the current named node context (e.g., "Peer thomas").
	// Only populated when inside a list entry. Each entry is "ListName KeyValue".
	ContextHeading []ContextEntry
	// ListTable holds data for rendering a list with unique constraints as a table.
	// Populated when the current path ends at a list node with unique fields.
	ListTable *ListTableView
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
		// Redirect to root if path is invalid or refers to a non-existent entry.
		if len(path) > 0 {
			schemaNode, walkErr := walkSchema(schema, path)
			if walkErr != nil || schemaNode == nil {
				target := "/show/?error=" + url.QueryEscape(fmt.Sprintf("invalid path: %s", strings.Join(path, "/")))
				http.Redirect(w, r, target, http.StatusFound)
				return
			}
			if isListEntryPath(schema, path) && walkTree(viewTree, schema, path) == nil {
				entryKey := path[len(path)-1]
				target := "/show/?error=" + url.QueryEscape(fmt.Sprintf("entry %q does not exist", entryKey))
				http.Redirect(w, r, target, http.StatusFound)
				return
			}
		}

		data := buildFragmentData(schema, viewTree, path)
		data.Username = username
		data.Insecure = insecure
		data.Monitor = strings.HasPrefix(r.URL.Path, "/monitor/")

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

	// Show/monitor endpoint: /show/bgp/peer or /monitor/bgp/peer.
	raw := strings.TrimPrefix(r.URL.Path, "/show/")
	raw = strings.TrimPrefix(raw, "/monitor/")
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
	// Build context heading from named nodes (list entries) in the path.
	data.ContextHeading = buildContextHeading(schema, path)

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
		// Distinguish list view (bgp/peer) from entry view (bgp/peer/thomas).
		// If the last path segment is NOT a schema child of the list, it's an entry key.
		atEntry := isListEntryPath(schema, path)

		if atEntry {
			// Entry view: show the entry's fields (same as container).
			subtree := walkTree(tree, schema, path)
			if subtree != nil {
				populateFragmentFields(data, n, subtree, prefix)
			}
		} else {
			// List view: render table for lists with unique constraints.
			uniqueFields := collectUniqueFields(n)
			if len(uniqueFields) > 0 {
				keys := collectListKeys(tree, schema, path)
				baseURL := "/show/" + strings.Join(path, "/") + "/"
				data.ListTable = buildListTable(tree, schema, path, n, keys, uniqueFields, baseURL)
			}
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

	// Booleans without an explicit YANG default are implicitly false.
	// Show the "default-no" color hint so the toggle isn't colorless.
	if leaf.Type == config.TypeBool && meta.Default == "" {
		meta.Default = "false"
	}

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
		// If this is a list node at the list level (not an entry), handle specially.
		if listNode, ok := node.(*config.ListNode); ok {
			if !isListEntryPath(schema, prefix) {
				// List view: lists with unique constraints show table in detail panel.
				if len(collectUniqueFields(listNode)) > 0 {
					return nil
				}
				return buildListColumn(tree, schema, prefix, listNode, selectedName)
			}
			// Entry view: fall through to show schema children.
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

		// Named lists (with KeyName) go in NamedItems, everything else in UnnamedItems.
		if listNode, ok := child.(*config.ListNode); ok {
			item.IsList = true
			item.HasChildren = true
			item.AddURL = url
			item.Count = len(collectListKeys(tree, schema, childPath))
			if listNode.KeyName != "" {
				col.NamedItems = append(col.NamedItems, item)
			} else {
				col.UnnamedItems = append(col.UnnamedItems, item)
			}
		} else {
			item.HasChildren = hasNonLeafChildren(child)
			col.UnnamedItems = append(col.UnnamedItems, item)
		}
	}

	return col
}

// buildListColumn builds a column showing list entries (e.g., peer names).
// Lists with unique constraints render their table in the detail panel (not here).
func buildListColumn(tree *config.Tree, schema *config.Schema, prefix []string, listNode *config.ListNode, selectedName string) *FinderColumn {
	col := &FinderColumn{}
	keys := collectListKeys(tree, schema, prefix)
	url := "/show/" + strings.Join(prefix, "/") + "/"

	// Show entries in the column.
	keyless := listNode.KeyName == ""
	for _, k := range keys {
		entryPath := strings.Join(prefix, "/") + "/" + k
		displayName := k
		hasName := false
		if keyless {
			// For keyless lists, show a summary from the entry content
			// instead of the meaningless numeric key.
			entryTree := walkTree(tree, schema, append(append([]string{}, prefix...), k))
			if summary := keylessEntrySummary(entryTree, listNode); summary != "" {
				displayName = summary
				hasName = true
			} else {
				displayName = "#" + k
			}
		}
		item := ColumnItem{
			Name:        displayName,
			URL:         url + k + "/",
			HxPath:      entryPath,
			Selected:    k == selectedName,
			HasChildren: true,
		}
		// Named keyless entries before numbered ones.
		if keyless && !hasName {
			col.UnnamedItems = append(col.UnnamedItems, item)
		} else {
			col.NamedItems = append(col.NamedItems, item)
		}
	}

	// Add "+ new" entry at the top.
	col.NamedItems = append([]ColumnItem{{
		Name:   "+ new",
		IsList: true,
		AddURL: url,
	}}, col.NamedItems...)

	return col
}

// collectUniqueFields returns all distinct leaf paths from a list's unique constraints.
func collectUniqueFields(listNode *config.ListNode) []string {
	seen := make(map[string]bool)
	var fields []string
	for _, constraint := range listNode.Unique {
		for _, field := range constraint {
			if !seen[field] {
				seen[field] = true
				fields = append(fields, field)
			}
		}
	}
	return fields
}

// buildListTable builds a ListTableView for a list with unique constraints.
func buildListTable(tree *config.Tree, schema *config.Schema, prefix []string, listNode *config.ListNode, keys, uniqueFields []string, baseURL string) *ListTableView {
	listName := prefix[len(prefix)-1]
	table := &ListTableView{
		Name:    listName,
		AddURL:  "/config/add/" + strings.Join(prefix, "/") + "/",
		FormURL: "/config/add-form/" + strings.Join(prefix, "/") + "/",
		SetURL:  "/config/set/" + strings.Join(prefix, "/") + "/",
	}

	// Key column first, then unique field columns.
	table.Columns = append(table.Columns, ListTableColumn{Name: listNode.KeyName, Key: true})
	for _, f := range uniqueFields {
		table.Columns = append(table.Columns, ListTableColumn{Name: f})
	}

	// Build rows from configured entries.
	for _, key := range keys {
		row := ListTableRow{
			KeyValue: key,
			URL:      baseURL + key + "/",
			HxPath:   strings.Join(prefix, "/") + "/" + key,
		}

		// Resolve each unique field's value from the entry's subtree.
		entryPath := append(append([]string{}, prefix...), key)
		entryTree := walkTree(tree, schema, entryPath)
		for _, field := range uniqueFields {
			// Split "remote/ip" into parent path "remote" and leaf "ip".
			leaf, parentSuffix := splitFieldPath(field)
			cellPath := strings.Join(entryPath, "/")
			if parentSuffix != "" {
				cellPath += "/" + parentSuffix
			}
			row.Cells = append(row.Cells, ListTableCell{
				Value:       resolveNestedValue(entryTree, field),
				Leaf:        leaf,
				Path:        cellPath,
				Placeholder: resolveLeafDescription(listNode, field),
			})
		}

		table.Rows = append(table.Rows, row)
	}

	return table
}

// splitFieldPath splits a slash-separated field path into the leaf name and the parent path suffix.
// "remote/ip" returns ("ip", "remote"). "ip" returns ("ip", "").
func splitFieldPath(field string) (leaf, parentSuffix string) {
	idx := strings.LastIndex(field, "/")
	if idx < 0 {
		return field, ""
	}
	return field[idx+1:], field[:idx]
}

// resolveLeafDescription walks the YANG list schema to find the description of a unique field leaf.
// field is a slash-separated path like "remote/ip".
func resolveLeafDescription(listNode *config.ListNode, field string) string {
	parts := strings.Split(field, "/")
	var current schemaGetter = listNode
	for i, part := range parts {
		child := current.Get(part)
		if child == nil {
			return ""
		}
		if i == len(parts)-1 {
			return nodeDescription(child)
		}
		if next, ok := child.(schemaGetter); ok {
			current = next
		} else {
			return ""
		}
	}
	return ""
}

// resolveLeafType walks the YANG list schema to find the ValueType of a unique field leaf.
func resolveLeafType(listNode *config.ListNode, field string) config.ValueType {
	parts := strings.Split(field, "/")
	var current schemaGetter = listNode
	for i, part := range parts {
		child := current.Get(part)
		if child == nil {
			return config.TypeString
		}
		if i == len(parts)-1 {
			if leaf, ok := child.(*config.LeafNode); ok {
				return leaf.Type
			}
			return config.TypeString
		}
		if next, ok := child.(schemaGetter); ok {
			current = next
		} else {
			return config.TypeString
		}
	}
	return config.TypeString
}

// buildContextHeading extracts named nodes (list entries) from the path.
// For path ["bgp", "peer", "thomas", "family", "ipv4/unicast"],
// returns [{"peer", "thomas"}, {"family", "ipv4/unicast"}].
func buildContextHeading(schema *config.Schema, path []string) []ContextEntry {
	var entries []ContextEntry
	var current schemaGetter = schema
	for i := 0; i < len(path); {
		node := current.Get(path[i])
		if node == nil {
			break
		}
		if listNode, ok := node.(*config.ListNode); ok {
			if i+1 < len(path) && listNode.Get(path[i+1]) == nil {
				entries = append(entries, ContextEntry{ListName: path[i], Key: path[i+1]})
				current = listNode
				i += 2
				continue
			}
			current = listNode
			i++
			continue
		}
		if sg, ok := node.(schemaGetter); ok {
			current = sg
		}
		i++
	}
	return entries
}

// keylessEntrySummary builds a human-readable label for a keyless list entry
// by inspecting its content. Prefers the ze:display-key leaf, then looks for
// child list keys (e.g., nlri families in an update block), then falls back
// to the first non-empty leaf value.
func keylessEntrySummary(tree *config.Tree, schema *config.ListNode) string {
	if tree == nil {
		return ""
	}
	// Prefer ze:display-key leaf if set.
	if dk := schema.DisplayKey; dk != "" {
		if v, ok := tree.Get(dk); ok && v != "" {
			return v
		}
	}
	// Look for list children and collect their keys as the summary.
	for _, name := range schema.Children() {
		child := schema.Get(name)
		if _, ok := child.(*config.ListNode); ok {
			entries := tree.GetListOrdered(name)
			if len(entries) > 0 {
				var keys []string
				for _, e := range entries {
					keys = append(keys, e.Key)
				}
				return strings.Join(keys, ", ")
			}
		}
	}
	// Fallback: first non-empty leaf value (skip display-key, already checked).
	for _, name := range schema.Children() {
		if name == schema.DisplayKey {
			continue
		}
		if _, ok := schema.Get(name).(*config.LeafNode); ok {
			if v, ok := tree.Get(name); ok && v != "" {
				return v
			}
		}
	}
	return ""
}

// resolveNestedValue resolves a slash-separated path (e.g., "remote/ip") to a leaf value in a tree.
func resolveNestedValue(tree *config.Tree, path string) string {
	if tree == nil {
		return ""
	}
	parts := strings.Split(path, "/")
	current := tree
	for i, part := range parts {
		if i == len(parts)-1 {
			val, _ := current.Get(part)
			return val
		}
		current = current.GetContainer(part)
		if current == nil {
			return ""
		}
	}
	return ""
}
