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

		htmxRedirect(w, r, "/config/edit/")
	}
}

// redirectBackOneLevel computes the parent path by removing the last segment
// and redirects to /config/edit/<parent>/. For HTMX requests, it sets the
// HX-Redirect header instead of returning an HTTP redirect.
func redirectBackOneLevel(w http.ResponseWriter, r *http.Request, currentPath []string) {
	parentPath := "/config/edit/"
	if len(currentPath) > 0 {
		parentPath = "/config/edit/" + strings.Join(currentPath[:len(currentPath)-1], "/")
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
