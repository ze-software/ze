// Design: docs/architecture/web-interface.md -- Config tree view handlers
// Detail: handler_config_walk.go -- Schema and config tree walking
// Detail: handler_config_leaf.go -- Leaf input type and template helpers
// Related: handler.go -- URL routing and content negotiation
// Related: render.go -- Template rendering
// Related: editor.go -- Per-user editor management
// Related: handler_admin.go -- Admin command handlers
// Related: sse.go -- SSE broker for live config change notifications
// Related: cli.go -- CLI bar command dispatch

package web

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"slices"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
)

const (
	htmxRequestTrue = "true"
	boolTrue        = "true"
	boolFalse       = "false"
)

// ConfigViewData holds all data needed for any config template.
type ConfigViewData struct {
	// Path is the current YANG path segments.
	Path []string
	// CurrentPath is the joined URL path for form actions (e.g., "bgp/peer/1.2.3.4").
	CurrentPath string
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
	// BasePath is the URL prefix for list key links (e.g., "/show/bgp/peer/").
	BasePath string
	// DetailPath is the URL path for the selected list entry's set forms.
	DetailPath string
	// LeafFields holds input field data for leaf nodes within a container or entry.
	LeafFields []LeafField
	// Entries holds freeform node entries.
	Entries []string
}

// ChildEntry represents a child node in a container view.
type ChildEntry struct {
	Name   string
	Kind   string // "container", "list", "leaf"
	URL    string
	HxPath string // YANG path for hx-get (without /show/ prefix)
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
	Modified     bool   // true when user has pending changes vs committed config
	OldValue     string // previous value before modification
}

// HandleConfigSet returns a POST handler for /config/set/<yang-path>/.
// It extracts the authenticated username from the request context, parses
// the form body for "leaf" (field name) and "value" (new value), and calls
// mgr.SetValue. For TypeBool leaves (checkboxes), the presence of the field
// is treated as "true" and absence as "false".
//
// On success, redirects one level up in the path hierarchy.
// On validation error from SetValue, returns an error notification.
// HTMX requests receive HX-Redirect instead of an HTTP redirect.
func HandleConfigSet(mgr *EditorManager, schema *config.Schema, renderer *Renderer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		username := GetUsernameFromRequest(r)
		if username == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		parsed, err := ParseURL(r)
		if err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		path := parsed.Path

		r.Body = http.MaxBytesReader(w, r.Body, 65536)

		if err := r.ParseForm(); err != nil {
			http.Error(w, fmt.Sprintf("parse form: %v", err), http.StatusBadRequest)
			return
		}

		leaf := r.FormValue("leaf")
		if leaf == "" {
			http.Error(w, "missing leaf field name", http.StatusBadRequest)
			return
		}

		if err := ValidatePathSegments([]string{leaf}); err != nil {
			http.Error(w, "invalid leaf name", http.StatusBadRequest)
			return
		}

		value := r.FormValue("value")

		// __default__ means "delete this leaf, revert to YANG default".
		if value == "__default__" {
			if err := mgr.DeleteValue(username, path, leaf); err != nil {
				errPath := strings.Join(append(path, leaf), "/")
				if renderer != nil {
					WriteOOBError(w, renderer, errPath, err.Error(), http.StatusBadRequest)
				} else {
					http.Error(w, fmt.Sprintf("delete value: %v", err), http.StatusBadRequest)
				}
				return
			}
		} else {
			// For boolean leaves, normalize to "true"/"false".
			// Toggle buttons send value=true/false explicitly.
			// HTML checkboxes send the field only when checked (legacy path).
			if isBoolLeaf(schema, path, leaf) {
				if value == boolTrue || value == "1" || value == "on" {
					value = boolTrue
				} else {
					value = boolFalse
				}
			}

			// Validate YANG type for the leaf.
			if leafNode := findLeafNode(schema, path, leaf); leafNode != nil {
				if valErr := config.ValidateValue(leafNode.Type, value); valErr != nil {
					errPath := strings.Join(append(path, leaf), "/")
					if renderer != nil {
						WriteOOBError(w, renderer, errPath, valErr.Error(), http.StatusBadRequest)
					} else {
						http.Error(w, valErr.Error(), http.StatusBadRequest)
					}
					return
				}
			}

			// Check unique constraints for inline table edits.
			if uniqueErr := validateUniqueOnSet(mgr.Tree(username), schema, path, leaf, value); uniqueErr != "" {
				errPath := strings.Join(append(path, leaf), "/")
				if renderer != nil {
					WriteOOBError(w, renderer, errPath, uniqueErr, http.StatusConflict)
				} else {
					http.Error(w, uniqueErr, http.StatusConflict)
				}
				return
			}

			if err := mgr.SetValue(username, path, leaf, value); err != nil {
				errPath := strings.Join(append(path, leaf), "/")
				if renderer != nil {
					WriteOOBError(w, renderer, errPath, err.Error(), http.StatusBadRequest)
				} else {
					http.Error(w, fmt.Sprintf("set value: %v", err), http.StatusBadRequest)
				}
				return
			}
		}

		// HTMX requests: return re-rendered field + OOB commit bar.
		if r.Header.Get("HX-Request") == htmxRequestTrue || r.Header.Get("X-Requested-With") == "fetch" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")

			// Re-render the field with updated value.
			if renderer != nil {
				effectiveValue := value
				if value == "__default__" {
					effectiveValue = ""
				}
				leafNode := findLeafNode(schema, path, leaf)
				if leafNode != nil {
					field := buildFieldMetaFromLeaf(leaf, leafNode, effectiveValue, strings.Join(path, "/"))
					fieldHTML := renderer.RenderField(field)
					if _, writeErr := w.Write([]byte(fieldHTML)); writeErr != nil {
						return
					}
				}
			}

			// OOB commit bar with change count.
			type saveOK struct{ ChangeCount int }
			count := mgr.ChangeCount(username)
			oob := renderer.RenderFragment("oob_save_ok", saveOK{ChangeCount: count})
			if _, writeErr := w.Write([]byte(oob)); writeErr != nil {
				return
			}
			return
		}

		redirectBackOneLevel(w, r, path)
	}
}

// HandleConfigAdd returns a POST handler for /config/add/<yang-path>/.
// It creates a list entry and sets any form field values.
// The entry key comes from the URL path (last segment) or the "name" form field.
// Form fields with "field:" prefix set values on the new entry (e.g., field:remote/ip=1.2.3.4).
// For HTMX requests, returns the updated list table fragment.
func HandleConfigAdd(mgr *EditorManager, schema *config.Schema, renderer *Renderer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		username := GetUsernameFromRequest(r)
		if username == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		parsed, err := ParseURL(r)
		if err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, 65536)
		if parseErr := r.ParseForm(); parseErr != nil {
			http.Error(w, "bad form data", http.StatusBadRequest)
			return
		}

		path := parsed.Path

		// Accept entry key from "name" form field (HTMX form) or URL path (JS).
		// Keys are always lowercase.
		if name := strings.ToLower(strings.TrimSpace(r.FormValue("name"))); name != "" {
			path = append(path, name)
		} else if len(path) > 0 {
			// Keyless list: auto-generate a sequential key.
			if listNode, ok := findListNode(schema, path); ok && listNode.KeyName == "" {
				existing := collectListKeys(mgr.Tree(username), schema, path)
				path = append(path, fmt.Sprintf("%d", len(existing)+1))
			}
		}

		if len(path) < 2 {
			http.Error(w, "path must include list name and entry key", http.StatusBadRequest)
			return
		}

		// Validate all fields BEFORE creating the entry.
		entryKey := path[len(path)-1]
		listPath := path[:len(path)-1]
		tree := mgr.Tree(username)
		listNode, walkErr := walkSchema(schema, listPath)
		if walkErr != nil {
			http.Error(w, "invalid list path", http.StatusBadRequest)
			return
		}

		// Check duplicate key.
		if tree != nil && slices.Contains(collectListKeys(tree, schema, listPath), entryKey) {
			returnAddError(w, r, renderer, schema, mgr, username, listPath, fmt.Sprintf("entry %q already exists", entryKey))
			return
		}

		// Enforce ze:required fields: reject if missing and no inherited value.
		if ln, ok := listNode.(*config.ListNode); ok && len(ln.Required) > 0 {
			parentTree := resolveParentDefaults(tree, listPath)
			for _, reqPath := range ln.Required {
				fieldStr := strings.Join(reqPath, "/")
				formVal := strings.TrimSpace(r.FormValue("field:" + fieldStr))
				inherited := resolveInheritedValue(parentTree, fieldStr)
				if formVal == "" && inherited == "" {
					returnAddError(w, r, renderer, schema, mgr, username, listPath,
						fmt.Sprintf("required field %q is missing", fieldStr))
					return
				}
			}
		}

		// Collect and validate field values before any mutation.
		type fieldSet struct {
			fieldPath, leaf, parentSuffix, value string
		}
		var fields []fieldSet
		for formKey, values := range r.PostForm {
			if !strings.HasPrefix(formKey, "field:") || len(values) == 0 {
				continue
			}
			fieldPath := strings.TrimPrefix(formKey, "field:")
			value := strings.TrimSpace(values[0])
			if value == "" {
				continue
			}

			// Validate value against YANG type.
			if ln, ok := listNode.(*config.ListNode); ok {
				fieldType := resolveLeafType(ln, fieldPath)
				if valErr := config.ValidateValue(fieldType, value); valErr != nil {
					returnAddError(w, r, renderer, schema, mgr, username, listPath,
						fmt.Sprintf("invalid %s: %v", fieldPath, valErr))
					return
				}
				// Check unique constraint.
				if conflict := checkUniqueConstraint(tree, schema, listPath, entryKey, fieldPath, value); conflict != "" {
					returnAddError(w, r, renderer, schema, mgr, username, listPath,
						fmt.Sprintf("duplicate %s %q (already used by %s)", fieldPath, value, conflict))
					return
				}
			}

			leaf, parentSuffix := splitFieldPath(fieldPath)
			fields = append(fields, fieldSet{fieldPath, leaf, parentSuffix, value})
		}

		// All validation passed. Create entry and set fields.
		if createErr := mgr.CreateEntry(username, path); createErr != nil {
			returnAddError(w, r, renderer, schema, mgr, username, listPath, fmt.Sprintf("create entry: %v", createErr))
			return
		}
		for _, f := range fields {
			setPath := make([]string, len(path))
			copy(setPath, path)
			if f.parentSuffix != "" {
				setPath = append(setPath, strings.Split(f.parentSuffix, "/")...)
			}
			if setErr := mgr.SetValue(username, setPath, f.leaf, f.value); setErr != nil {
				serverLogger.Warn("add-entry set field failed", "field", f.fieldPath, "error", setErr)
			}
		}

		// HTMX: return updated list table + commit bar + finder for the parent list path.
		if r.Header.Get("HX-Request") == htmxRequestTrue {
			// Keyless lists: redirect to the new entry so the page reloads with it.
			if ln, ok := listNode.(*config.ListNode); ok && ln.KeyName == "" {
				target := "/show/" + strings.Join(path, "/") + "/"
				w.Header().Set("HX-Redirect", target)
				w.WriteHeader(http.StatusOK)
				return
			}

			tree = mgr.Tree(username)
			data := buildFragmentData(schema, tree, listPath)
			tableHTML := renderer.RenderFragment("list_table", data)

			type saveOK struct{ ChangeCount int }
			count := mgr.ChangeCount(username)
			commitHTML := renderer.RenderFragment("oob_save_ok", saveOK{ChangeCount: count})

			// OOB finder update so the peer count refreshes.
			finderHTML := renderer.RenderFragment("finder_oob", data)

			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			if _, writeErr := w.Write([]byte(string(tableHTML) + string(commitHTML) + string(finderHTML))); writeErr != nil {
				return
			}
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}

// returnAddError returns the list table with an error notification for HTMX add requests,
// or a plain HTTP error for non-HTMX requests.
func returnAddError(w http.ResponseWriter, r *http.Request, renderer *Renderer, schema *config.Schema, mgr *EditorManager, username string, listPath []string, errMsg string) {
	if r.Header.Get("HX-Request") != htmxRequestTrue {
		http.Error(w, errMsg, http.StatusConflict)
		return
	}
	tree := mgr.Tree(username)
	data := buildFragmentData(schema, tree, listPath)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tableHTML := renderer.RenderFragment("list_table", data)
	notifHTML := renderer.RenderFragment("notification_error", struct{ Message string }{Message: errMsg})
	if _, writeErr := w.Write([]byte(string(tableHTML) + string(notifHTML))); writeErr != nil {
		return
	}
}

// HandleConfigAddForm returns a GET handler for /config/add-form/<yang-path>/.
// It renders an overlay form with inputs for the list key, required, suggest, and unique fields.
// Required and suggest fields are resolved from the config tree for inherited defaults.
func HandleConfigAddForm(mgr *EditorManager, schema *config.Schema, renderer *Renderer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		username := GetUsernameFromRequest(r)
		if username == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		parsed, err := ParseURL(r)
		if err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		listPath := parsed.Path
		if len(listPath) == 0 {
			http.Error(w, "path required", http.StatusBadRequest)
			return
		}

		schemaNode, walkErr := walkSchema(schema, listPath)
		if walkErr != nil {
			http.Error(w, "invalid path", http.StatusBadRequest)
			return
		}

		listNode, ok := schemaNode.(*config.ListNode)
		if !ok {
			http.Error(w, "not a list", http.StatusBadRequest)
			return
		}

		type formField struct {
			Path        string
			Placeholder string
			Category    string // "required", "suggest", or "unique"
			Inherited   string // pre-filled inherited value (empty if none)
		}

		listName := strings.ToUpper(listPath[len(listPath)-1][:1]) + listPath[len(listPath)-1][1:]
		keyless := listNode.KeyName == ""
		displayKey := listNode.DisplayKey

		data := struct {
			AddURL     string
			ListName   string
			KeyName    string
			Keyless    bool
			DisplayKey string
			Fields     []formField
		}{
			AddURL:     "/config/add/" + strings.Join(listPath, "/") + "/",
			ListName:   listName,
			KeyName:    listNode.KeyName,
			Keyless:    keyless,
			DisplayKey: displayKey,
		}

		// Resolve inherited defaults from parent context in the config tree.
		tree := mgr.Tree(username)
		parentTree := resolveParentDefaults(tree, listPath)

		// Track fields already added to avoid duplicates (required/suggest may overlap unique).
		seen := make(map[string]bool)

		for _, field := range collectRequiredFields(listNode) {
			seen[field] = true
			data.Fields = append(data.Fields, formField{
				Path:        field,
				Placeholder: resolveLeafDescription(listNode, field),
				Category:    "required",
				Inherited:   resolveInheritedValue(parentTree, field),
			})
		}

		for _, field := range collectSuggestFields(listNode) {
			if seen[field] {
				continue
			}
			seen[field] = true
			data.Fields = append(data.Fields, formField{
				Path:        field,
				Placeholder: resolveLeafDescription(listNode, field),
				Category:    "suggest",
				Inherited:   resolveInheritedValue(parentTree, field),
			})
		}

		for _, field := range collectUniqueFields(listNode) {
			if seen[field] {
				continue
			}
			data.Fields = append(data.Fields, formField{
				Path:        field,
				Placeholder: resolveLeafDescription(listNode, field),
				Category:    "unique",
			})
		}

		html := renderer.RenderFragment("add_form_overlay", data)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if _, writeErr := w.Write([]byte(html)); writeErr != nil {
			return
		}
	}
}

// resolveParentDefaults walks the config tree to find the parent context for a list path.
// For bgp/group/<name>/peer, it returns the group entry tree (inheritable defaults).
// For bgp/peer, it returns the bgp container tree (bgp-level defaults).
// Returns nil if no tree or path is too short.
func resolveParentDefaults(tree *config.Tree, listPath []string) *config.Tree {
	if tree == nil || len(listPath) < 2 {
		return nil
	}
	// Walk to the parent of the list. Segments alternate between container/list
	// names and list entry keys.
	parentPath := listPath[:len(listPath)-1]
	current := tree
	for i := 0; i < len(parentPath); i++ {
		seg := parentPath[i]
		// Try container first.
		if child := current.GetContainer(seg); child != nil {
			current = child
			continue
		}
		// Try as a list name with the next segment as entry key.
		if i+1 < len(parentPath) {
			entries := current.GetList(seg)
			if entry, ok := entries[parentPath[i+1]]; ok {
				current = entry
				i++ // Skip the key segment.
				continue
			}
		}
		return nil
	}
	return current
}

// resolveInheritedValue looks up a slash-separated field path in a config tree.
// Returns the leaf value if found, empty string otherwise.
func resolveInheritedValue(tree *config.Tree, field string) string {
	if tree == nil {
		return ""
	}
	parts := strings.Split(field, "/")
	current := tree
	for i, part := range parts {
		if i == len(parts)-1 {
			val, exists := current.Get(part)
			if exists {
				return val
			}
			return ""
		}
		child := current.GetContainer(part)
		if child == nil {
			return ""
		}
		current = child
	}
	return ""
}

// HandleConfigChanges returns a GET handler for /config/changes that returns
// the commit bar HTML reflecting current pending change count.
func HandleConfigChanges(mgr *EditorManager, renderer *Renderer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		username := GetUsernameFromRequest(r)
		if username == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		type saveOK struct{ ChangeCount int }
		count := mgr.ChangeCount(username)
		var html template.HTML
		if count > 0 {
			html = renderer.RenderFragment("oob_save_ok", saveOK{ChangeCount: count})
		} else {
			html = renderer.RenderFragment("commit_bar", nil)
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if _, writeErr := w.Write([]byte(html)); writeErr != nil {
			return
		}
	}
}

// HandleConfigDelete returns a POST handler for /config/delete/<yang-path>/.
// It extracts the authenticated username, parses the form body for "leaf",
// and calls mgr.DeleteValue to remove the configured value.
//
// On success, redirects one level up. HTMX support mirrors HandleConfigSet.
func HandleConfigDelete(mgr *EditorManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		username := GetUsernameFromRequest(r)
		if username == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		parsed, err := ParseURL(r)
		if err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		path := parsed.Path

		r.Body = http.MaxBytesReader(w, r.Body, 65536)

		if err := r.ParseForm(); err != nil {
			http.Error(w, fmt.Sprintf("parse form: %v", err), http.StatusBadRequest)
			return
		}

		leaf := r.FormValue("leaf")
		if leaf == "" {
			http.Error(w, "missing leaf field name", http.StatusBadRequest)
			return
		}

		if err := ValidatePathSegments([]string{leaf}); err != nil {
			http.Error(w, "invalid leaf name", http.StatusBadRequest)
			return
		}

		if err := mgr.DeleteValue(username, path, leaf); err != nil {
			http.Error(w, fmt.Sprintf("delete value: %v", err), http.StatusBadRequest)
			return
		}

		redirectBackOneLevel(w, r, path)
	}
}

// HandleConfigCommit returns a handler for /config/commit/.
// GET: shows the commit page with a diff of pending changes.
// POST: applies the user's pending changes via mgr.Commit.
//
// On successful commit, broadcasts a config-change SSE event to all
// connected web clients (if broker is non-nil) and redirects to
// /config/edit/ (config root).
// On conflict, re-renders the commit page with conflict errors.
// HTMX requests receive HX-Redirect instead of an HTTP redirect.
func HandleConfigCommit(mgr *EditorManager, renderer *Renderer, broker *EventBroker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		username := GetUsernameFromRequest(r)
		if username == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		if r.Method == http.MethodGet {
			handleCommitGet(w, mgr, renderer, username)
			return
		}

		if r.Method == http.MethodPost {
			handleCommitPost(w, r, mgr, renderer, username, broker)
			return
		}

		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleCommitGet renders the commit page showing a diff of pending changes.
func handleCommitGet(w http.ResponseWriter, mgr *EditorManager, renderer *Renderer, username string) {
	diff, err := mgr.Diff(username)
	if err != nil {
		http.Error(w, fmt.Sprintf("diff: %v", err), http.StatusInternalServerError)
		return
	}

	layoutData := LayoutData{
		Title: "Commit Changes",
	}

	if diff != "" {
		layoutData.NotificationHTML = template.HTML("<pre>" + template.HTMLEscapeString(diff) + "</pre>") //nolint:gosec // escaped
	}

	if err := renderer.RenderLayout(w, layoutData); err != nil {
		http.Error(w, fmt.Sprintf("render: %v", err), http.StatusInternalServerError)
	}
}

// handleCommitPost applies pending changes and redirects or re-renders on conflict.
// On successful commit (no conflicts), broadcasts a config-change SSE event.
func handleCommitPost(w http.ResponseWriter, r *http.Request, mgr *EditorManager, renderer *Renderer, username string, broker *EventBroker) {
	result, err := mgr.Commit(username)
	if err != nil {
		http.Error(w, fmt.Sprintf("commit: %v", err), http.StatusInternalServerError)
		return
	}

	if len(result.Conflicts) > 0 {
		var msg strings.Builder
		msg.WriteString("Commit conflicts:\n")

		for _, c := range result.Conflicts {
			fmt.Fprintf(&msg, "  %s: want %q, other (%s) has %q\n", c.Path, c.MyValue, c.OtherUser, c.OtherValue)
		}

		layoutData := LayoutData{
			Title:            "Commit Conflicts",
			NotificationHTML: template.HTML("<pre>" + template.HTMLEscapeString(msg.String()) + "</pre>"), //nolint:gosec // escaped
		}

		if err := renderer.RenderLayout(w, layoutData); err != nil {
			http.Error(w, fmt.Sprintf("render: %v", err), http.StatusInternalServerError)
		}

		return
	}

	// Broadcast config change notification to all connected SSE clients.
	// This runs only after CommitSession() returned successfully (AC-13).
	BroadcastConfigChange(broker, username, "committed")

	// Return closed diff modal + empty commit bar. No redirect -- the page
	// underneath the overlay stays unchanged.
	if r.Header.Get("HX-Request") == htmxRequestTrue {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		modal := renderer.RenderFragment("diff_modal", nil)
		type saveOK struct{ ChangeCount int }
		bar := renderer.RenderFragment("oob_save_ok", saveOK{ChangeCount: 0})
		if _, writeErr := w.Write([]byte(modal)); writeErr != nil {
			return
		}
		if _, writeErr := w.Write([]byte(bar)); writeErr != nil {
			return
		}
		return
	}

	htmxRedirect(w, r, "/")
}

// HandleConfigDiscard returns a POST handler for /config/discard/.
// It discards the user's pending changes and redirects to /config/edit/.
func HandleConfigDiscard(mgr *EditorManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		username := GetUsernameFromRequest(r)
		if username == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		if err := mgr.Discard(username); err != nil {
			http.Error(w, fmt.Sprintf("discard: %v", err), http.StatusInternalServerError)
			return
		}

		// Navigate back one level from where the user was.
		target := parentFromCurrentURL(r)
		htmxRedirect(w, r, target)
	}
}

// parentFromCurrentURL extracts the parent path from the HTMX HX-Current-URL
// header (or Referer). Used by handlers like discard that have no path in their
// own URL but need to navigate back one level from where the user was.
// Falls back to /config/edit/ if no usable URL is available.
func parentFromCurrentURL(r *http.Request) string {
	current := r.Header.Get("HX-Current-URL")
	if current == "" {
		current = r.Referer()
	}
	if current == "" {
		return configEditPath
	}

	// Strip scheme+host if present (HX-Current-URL is a full URL).
	if idx := strings.Index(current, "://"); idx >= 0 {
		if slash := strings.Index(current[idx+3:], "/"); slash >= 0 {
			current = current[idx+3+slash:]
		}
	}

	// Strip trailing slash, then remove the last segment.
	current = strings.TrimSuffix(current, "/")
	if last := strings.LastIndex(current, "/"); last > 0 {
		return current[:last+1]
	}

	return configEditPath
}

// redirectBackOneLevel computes the parent path by removing the last segment
// and redirects to /config/edit/<parent>/. For HTMX requests, it sets the
// HX-Redirect header instead of returning an HTTP redirect.
func redirectBackOneLevel(w http.ResponseWriter, r *http.Request, currentPath []string) {
	parentPath := configEditPath
	if len(currentPath) > 0 {
		parentPath = configEditPath + strings.Join(currentPath[:len(currentPath)-1], "/")
		if len(currentPath) > 1 {
			parentPath += "/"
		}
	}

	htmxRedirect(w, r, parentPath)
}

// htmxRedirect sends a redirect to the given target URL. For HTMX requests
// (identified by the HX-Request header), it sets the HX-Redirect response
// header so htmx performs a client-side redirect. For regular requests, it
// returns a standard HTTP 303 See Other redirect.
func htmxRedirect(w http.ResponseWriter, r *http.Request, target string) {
	if r.Header.Get("HX-Request") == htmxRequestTrue {
		w.Header().Set("HX-Redirect", target)
		w.WriteHeader(http.StatusOK)

		return
	}

	http.Redirect(w, r, target, http.StatusSeeOther)
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
			http.Error(w, "bad request", http.StatusBadRequest)
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
			http.Error(w, "not found", http.StatusNotFound)
			return
		}

		tmplName := nodeKindToTemplate(viewData.NodeKind)
		contentHTML := renderer.RenderConfigToHTML(tmplName, viewData)

		// HTMX partial: return content fragment without layout wrapper.
		if r.Header.Get("HX-Request") == htmxRequestTrue {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			if _, err := w.Write([]byte(contentHTML)); err != nil {
				return // client disconnected
			}
			return
		}

		// Full HTML: render config content inside layout.
		layoutData := LayoutData{
			Title:       "Ze: /" + strings.Join(path, "/"),
			Content:     contentHTML,
			Breadcrumbs: viewData.Breadcrumbs,
			HasSession:  true,
			CLIPrompt:   "/" + strings.Join(path, "/") + ">",
		}

		if err := renderer.RenderLayout(w, layoutData); err != nil {
			http.Error(w, fmt.Sprintf("render: %v", err), http.StatusInternalServerError)
		}
	}
}
