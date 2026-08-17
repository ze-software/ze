// Design: docs/architecture/web-interface.md -- Config tree view handlers
// Detail: handler_config_walk.go -- Schema and config tree walking
// Detail: handler_config_leaf.go -- Leaf input type and template helpers
// Detail: handler_config_form.go -- Leaf set/delete and Workbench form save handlers
// Detail: handler_config_entry.go -- List entry add/rename handlers and add-form overlay
// Detail: handler_config_commit.go -- Commit, discard, and pending-change handlers
// Related: fragment.go -- HTMX fragment helpers (form field collection)
// Related: handler.go -- URL routing and content negotiation
// Related: render.go -- Template rendering
// Related: editor.go -- Per-user editor management
// Related: handler_admin.go -- Admin command handlers
// Related: cli.go -- CLI bar command dispatch

package web

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/component/api"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	htmxRequestTrue = "true"
	boolTrue        = "true"
	boolFalse       = "false"
)

const (
	webCommandConfigSet      = api.ConfigAuthSet
	webCommandConfigAdd      = "config add"
	webCommandConfigDelete   = api.ConfigAuthDelete
	webCommandConfigRename   = "config rename"
	webCommandConfigCommit   = api.ConfigAuthCommit
	webCommandConfigDiscard  = api.ConfigAuthDiscard
	webCommandConfigRollback = "config rollback"
	webCommandConfigSave     = "config save"
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

func authorizeWebConfigMutation(w http.ResponseWriter, r *http.Request, authorizer aaa.Authorizer, username, command string) bool {
	authorizer = authorizerForRequest(r, authorizer)
	if authorizer == nil {
		return true
	}
	if authorizer.Authorize(username, r.RemoteAddr, command, false) {
		return true
	}
	http.Error(w, "forbidden", http.StatusForbidden)
	return false
}

// redirectBackOneLevel computes the parent path by removing the last segment
// and redirects to /config/edit/<parent>/. For HTMX requests, it sets the
// HX-Redirect header instead of returning an HTTP redirect.
func redirectBackOneLevel(w http.ResponseWriter, r *http.Request, currentPath []string) {
	parentPath := configEditPath
	if len(currentPath) > 0 {
		var tb textbuf.Buffer
		tb.Str(configEditPath).Join(currentPath[:len(currentPath)-1], "/")
		if len(currentPath) > 1 {
			tb.Byte('/')
		}
		parentPath = tb.String()
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

// handleConfigViewForTest returns the legacy show-tree handler used by focused
// compatibility tests.
func handleConfigViewForTest(renderer *Renderer, schema *config.Schema, tree *config.Tree) http.HandlerFunc {
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

		contentHTML := renderConfigContent(renderer, viewData)

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
			Title:       func() string { var tb textbuf.Buffer; return tb.Str("Ze: /").Join(path, "/").String() }(),
			Content:     contentHTML,
			Breadcrumbs: viewData.Breadcrumbs,
			HasSession:  true,
			CLIPrompt:   func() string { var tb textbuf.Buffer; return tb.Byte('/').Join(path, "/").Byte('>').String() }(),
			ActiveUI:    uiModeTokenFinder,
		}

		if err := renderer.RenderLayout(w, layoutData); err != nil {
			http.Error(w, fmt.Sprintf("render: %v", err), http.StatusInternalServerError)
		}
	}
}
