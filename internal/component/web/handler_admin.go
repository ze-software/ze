// Design: docs/architecture/web-interface.md -- Admin command handlers
// Related: handler.go -- URL routing
// Related: handler_config.go -- Config tree view handlers (navigation pattern)
// Related: render.go -- Template rendering

package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/command"
)

// CommandResultData holds the data for rendering a command result card.
// The command template shows the command name in the header and the output
// in the body. When Error is true, the card receives the command-error CSS
// class for visual distinction.
type CommandResultData struct {
	// CommandName is the human-readable command path (e.g., "peer 192.168.1.1 teardown").
	CommandName string
	// Output is the command's textual output.
	Output string
	// Error indicates whether the command execution failed.
	Error bool
}

// CommandFormData holds the data for rendering a command parameter form.
// Leaf commands (those with no sub-commands) render as a form with parameter
// fields and an "Execute" button that POSTs to ActionURL.
type CommandFormData struct {
	// CommandName is the human-readable command name.
	CommandName string
	// Description is the YANG description for this command, if available.
	Description string
	// ActionURL is the POST target (e.g., "/admin/peer/192.168.1.1/teardown").
	ActionURL string
	// Parameters lists the command's input parameters.
	Parameters []CommandParameter
}

// CommandParameter represents a single command parameter field.
type CommandParameter struct {
	// Name is the parameter's YANG name.
	Name string
	// Value is the pre-filled value, if any (e.g., from URL path segments).
	Value string
	// Placeholder is the hint text for the input field.
	Placeholder string
}

// CommandDispatcher executes an admin command and returns the output.
// The command string is the full command path (e.g., "peer 192.168.1.1 teardown").
// Username and remoteAddr carry the authenticated caller's identity so that
// authorization and accounting apply to web and MCP surfaces, not only SSH.
type CommandDispatcher func(command, username, remoteAddr string) (string, error)

// HandleAdminView returns an HTTP handler that serves the admin command tree
// using finder-style column navigation (same layout as config). Leaf commands
// render a form in the detail panel. The children map provides the static
// command tree structure.
func HandleAdminView(renderer *Renderer, children map[string][]string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		parsed, err := ParseURL(r)
		if err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		path := parsed.Path

		// JSON response: return the command tree structure.
		if parsed.Format == formatJSON {
			pathKey := strings.Join(path, "/")
			kids := children[pathKey]

			data := map[string]any{
				"path":     path,
				"children": kids,
			}

			w.Header().Set("Content-Type", "application/json")

			if err := json.NewEncoder(w).Encode(data); err != nil {
				http.Error(w, fmt.Sprintf("json encode: %v", err), http.StatusInternalServerError)
			}

			return
		}

		fragData := buildAdminFragmentData(path, children)

		// HTMX partial: return finder + detail via oob_response.
		if r.Header.Get("HX-Request") == htmxRequestTrue {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			html := renderer.RenderFragment("oob_response", fragData)
			if _, writeErr := w.Write([]byte(html)); writeErr != nil {
				return
			}
			return
		}

		// Full HTML: render inside layout.
		content := renderer.RenderFragment("full_content", fragData)
		layoutData := LayoutData{
			Title:       "Admin: /" + strings.Join(path, "/"),
			Content:     content,
			HasSession:  true,
			Breadcrumbs: fragData.Breadcrumbs,
		}

		if err := renderer.RenderLayout(w, layoutData); err != nil {
			http.Error(w, fmt.Sprintf("render: %v", err), http.StatusInternalServerError)
		}
	}
}

// HandleAdminExecute returns an HTTP handler that executes admin commands
// via POST. It reconstructs the command string from the URL path segments,
// dispatches the command through the provided dispatcher, and returns a
// result in the detail panel.
//
// Content negotiation: JSON requests receive the raw command output as
// a JSON object with "command", "output", and "error" fields.
// HTMX requests receive the result rendered as a detail panel fragment.
func HandleAdminExecute(renderer *Renderer, dispatch CommandDispatcher) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		parsed, err := ParseURL(r)
		if err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		path := parsed.Path
		commandStr := strings.Join(path, " ")

		if dispatch == nil {
			http.Error(w, "admin commands not available in standalone mode", http.StatusServiceUnavailable)
			return
		}

		username := GetUsernameFromRequest(r)
		output, execErr := dispatch(commandStr, username, r.RemoteAddr)

		result := CommandResultData{
			CommandName: commandStr,
			Output:      output,
			Error:       execErr != nil,
		}

		if execErr != nil && output == "" {
			result.Output = execErr.Error()
		}

		// JSON response: return raw command output.
		if parsed.Format == formatJSON {
			data := map[string]any{
				"command": commandStr,
				"output":  result.Output,
				"error":   result.Error,
			}

			w.Header().Set("Content-Type", "application/json")

			if err := json.NewEncoder(w).Encode(data); err != nil {
				http.Error(w, fmt.Sprintf("json encode: %v", err), http.StatusInternalServerError)
			}

			return
		}

		// HTMX: render result in the detail panel.
		fragData := &FragmentData{
			CommandResult: &result,
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		html := renderer.RenderFragment("detail", fragData)
		if _, writeErr := w.Write([]byte(html)); writeErr != nil {
			return
		}
	}
}

// buildAdminFragmentData builds FragmentData for the admin command tree,
// using finder-style columns for navigation and a command form in the detail
// panel for leaf commands.
func buildAdminFragmentData(path []string, children map[string][]string) *FragmentData {
	currentPath := strings.Join(path, "/")
	data := &FragmentData{
		Path:            path,
		CurrentPath:     currentPath,
		Breadcrumbs:     buildAdminBreadcrumbs(path),
		HasSession:      true,
		Columns:         buildAdminFinderColumns(path, children),
		CLIPrompt:       formatCLIPrompt(nil),
		CLIContextPath:  "",
		CLIPathSegments: nil,
		Services:        PortalServices(),
	}

	// Leaf command: show form in detail panel.
	pathKey := strings.Join(path, "/")
	kids := children[pathKey]
	if len(path) > 0 && len(kids) == 0 {
		data.CommandForm = &CommandFormData{
			CommandName: strings.Join(path, " "),
			ActionURL:   "/admin/" + strings.Join(path, "/"),
		}
	}

	return data
}

// buildAdminFinderColumns builds finder columns from the admin command tree.
// Each level of the tree gets a column showing available sub-commands.
func buildAdminFinderColumns(path []string, children map[string][]string) []FinderColumn {
	var columns []FinderColumn

	for depth := 0; depth <= len(path); depth++ {
		prefix := path[:depth]
		pathKey := strings.Join(prefix, "/")
		kids := children[pathKey]
		if len(kids) == 0 && depth < len(path) {
			break
		}

		var selectedName string
		if depth < len(path) {
			selectedName = path[depth]
		}

		col := FinderColumn{}
		for _, name := range kids {
			childPath := append(append([]string{}, prefix...), name)
			childKey := strings.Join(childPath, "/")
			url := "/admin/" + strings.Join(childPath, "/") + "/"

			col.UnnamedItems = append(col.UnnamedItems, ColumnItem{
				Name:        name,
				URL:         url,
				HxPath:      "admin/" + childKey,
				Selected:    name == selectedName,
				HasChildren: len(children[childKey]) > 0,
			})
		}
		if len(col.UnnamedItems) > 0 {
			columns = append(columns, col)
		}
	}

	// Keep at most 3 columns visible.
	if len(columns) > 3 {
		columns = columns[len(columns)-3:]
	}

	return columns
}

// buildAdminBreadcrumbs creates breadcrumb navigation entries for /admin/ paths.
// The root segment links to /admin/. Each path segment links to
// /admin/<path-up-to-here>/.
func buildAdminBreadcrumbs(path []string) []BreadcrumbSegment {
	crumbs := make([]BreadcrumbSegment, 0, 1+len(path))
	crumbs = append(crumbs, BreadcrumbSegment{Name: "admin", URL: "/admin/", Active: len(path) == 0})

	for i, seg := range path {
		url := "/admin/" + strings.Join(path[:i+1], "/") + "/"
		crumbs = append(crumbs, BreadcrumbSegment{
			Name:   seg,
			URL:    url,
			Active: i == len(path)-1,
		})
	}

	return crumbs
}

// BuildAdminCommandTree returns the static admin command tree derived from
// the ze-bgp-api YANG RPCs. The tree groups commands by category (peer,
// route, cache, system) for web UI navigation.
//
// Deprecated: Phase 6 of spec-web-2-operator-workbench replaces this map
// with [AdminTreeFromYANG], which derives the same structure from the
// merged YANG command tree so plugin-contributed commands appear without
// editing this file. Kept temporarily so existing call sites compile; new
// code MUST use AdminTreeFromYANG.
func BuildAdminCommandTree() map[string][]string {
	return map[string][]string{
		"":       {"peer", "route", "cache", "system"},
		"peer":   {"list", "show", "summary", "capabilities", "statistics", "add", "remove", "teardown", "clear-soft", "flush"},
		"route":  {"update", "borr", "eorr", "raw"},
		"cache":  {"list", "retain", "release", "expire", "forward"},
		"system": {"commit", "subscribe", "unsubscribe", "events"},
	}
}

// AdminTreeFromYANG converts a merged YANG operational command tree into
// the children-map format consumed by HandleAdminView. The returned map
// keys are slash-joined parent paths; values are the sorted child names at
// that depth. The empty key holds the top-level commands.
//
// Pass the result of yang.BuildCommandTree(loader). Plugin-contributed
// commands appear automatically because the loader registers every
// imported `-cmd` YANG module via init().
func AdminTreeFromYANG(tree *command.Node) map[string][]string {
	result := make(map[string][]string)
	walkAdminTree(tree, "", result)
	return result
}

// walkAdminTree recursively populates the children map. Keys are sorted at
// each level so the rendered finder columns are deterministic.
func walkAdminTree(node *command.Node, prefix string, result map[string][]string) {
	if node == nil || node.Children == nil {
		return
	}
	names := make([]string, 0, len(node.Children))
	for name := range node.Children {
		names = append(names, name)
	}
	sort.Strings(names)
	result[prefix] = names

	for _, name := range names {
		childPrefix := name
		if prefix != "" {
			childPrefix = prefix + "/" + name
		}
		walkAdminTree(node.Children[name], childPrefix, result)
	}
}
