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

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/textbuf"
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
	// Description is the one-line summary of this command, from its YANG
	// description statement. It is the lede above the form.
	Description string
	// Help is the long explanation of this command, from its ze:help
	// extension. It is the body under the lede, and it keeps the newlines its
	// author wrote. Empty means the command declares no explanation.
	Help string
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

// CommandDispatcher executes an admin command and returns the typed response.
// It is an alias for the unified plugin.CommandDispatcher every surface shares;
// the web handlers render the JSON string at their edge via
// CommandDispatcher.JSON, threading the authenticated caller's identity so that
// authorization and accounting apply to web and MCP surfaces, not only SSH.
type CommandDispatcher = plugin.CommandDispatcher

// HandleAdminView returns an HTTP handler that serves the admin command tree
// using finder-style column navigation (same layout as config). Leaf commands
// render a form in the detail panel.
//
// tree is the merged YANG operational command tree. The handler reads its
// shape for the navigation columns and its two help texts for the command
// form, so the page shows the same summary and explanation every other surface
// shows. A nil tree serves an empty console rather than panicking.
func HandleAdminView(renderer *Renderer, tree *command.Node) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		parsed, err := ParseURL(r)
		if err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		path := parsed.Path

		// JSON response: return the command tree structure.
		if parsed.Format == formatJSON {
			kids := adminChildNames(adminNodeAt(tree, path))

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

		fragData := buildAdminFragmentData(path, tree)

		// HTMX partial: return finder + detail via oob_response.
		if r.Header.Get("HX-Request") == htmxRequestTrue {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			html := renderer.renderComponent("oob_response", oobResponse(fragData))
			if _, writeErr := w.Write([]byte(html)); writeErr != nil {
				return
			}
			return
		}

		// Full HTML: render inside layout.
		content := renderer.renderComponent("full_content", fullContent(fragData))
		var tb textbuf.Buffer
		layoutData := LayoutData{
			Title:       tb.Str("Admin: /").Join(path, "/").String(),
			Content:     content,
			HasSession:  true,
			Breadcrumbs: fragData.Breadcrumbs,
			ActiveUI:    uiModeTokenFinder,
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
		commandStr := textbuf.Join(path, " ")

		if dispatch == nil {
			http.Error(w, "admin commands not available in standalone mode", http.StatusServiceUnavailable)
			return
		}

		username := GetUsernameFromRequest(r)
		rendered, execErr := dispatch.JSON(r.Context(), plugin.CallerIdentity{Username: username, RemoteAddr: r.RemoteAddr}, commandStr)
		defer rendered.TransportComplete()
		output := rendered.Output

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
				"command":    commandStr,
				"output":     result.Output,
				jsonKeyError: result.Error,
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
		html := renderer.renderComponent("detail", detail(fragData))
		if _, writeErr := w.Write([]byte(html)); writeErr != nil {
			return
		}
	}
}

// buildAdminFragmentData builds FragmentData for the admin command tree,
// using finder-style columns for navigation and a command form in the detail
// panel for leaf commands.
func buildAdminFragmentData(path []string, tree *command.Node) *FragmentData {
	currentPath := textbuf.Join(path, "/")
	data := &FragmentData{
		Path:            path,
		CurrentPath:     currentPath,
		Breadcrumbs:     buildAdminBreadcrumbs(path),
		HasSession:      true,
		Columns:         buildAdminFinderColumns(path, tree),
		CLIPrompt:       formatCLIPrompt(nil),
		CLIContextPath:  "",
		CLIPathSegments: nil,
		Services:        PortalServices(),
		ActiveUI:        uiModeTokenFinder,
	}

	// Leaf command: show form in detail panel.
	node := adminNodeAt(tree, path)
	if len(path) > 0 && !adminHasChildren(node) {
		var tb textbuf.Buffer
		form := &CommandFormData{
			CommandName: textbuf.Join(path, " "),
			ActionURL:   tb.Str("/admin/").Join(path, "/").String(),
		}
		// A path the tree does not hold still renders a form, which is what the
		// children map did before it. It shows no help, because it has none to
		// read. An absent command MUST NOT borrow its parent's text.
		if node != nil {
			form.Description = node.Description
			form.Help = node.Help
		}
		data.CommandForm = form
	}

	return data
}

// adminNodeAt walks the command tree to the node the path names. It answers nil
// for the path no node holds, so a caller reading help text off the result must
// test for it: an empty summary and an absent command must not read the same.
//
// The walk is bounded by the path length, which the URL parser caps.
func adminNodeAt(tree *command.Node, path []string) *command.Node {
	node := tree
	for _, name := range path {
		if node == nil || node.Children == nil {
			return nil
		}
		node = node.Children[name]
	}
	return node
}

// adminHasChildren answers whether the node holds a child command. The finder
// asks it for each rendered item, so it counts rather than building a list.
func adminHasChildren(node *command.Node) bool {
	return node != nil && len(node.Children) > 0
}

// adminChildNames answers the node's child command names, sorted, so the
// rendered finder columns are deterministic. A nil node has none.
func adminChildNames(node *command.Node) []string {
	if node == nil || len(node.Children) == 0 {
		return nil
	}
	names := make([]string, 0, len(node.Children))
	for name := range node.Children {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// buildAdminFinderColumns builds finder columns from the admin command tree.
// Each level of the tree gets a column showing available sub-commands.
func buildAdminFinderColumns(path []string, tree *command.Node) []FinderColumn {
	var columns []FinderColumn

	for depth := 0; depth <= len(path); depth++ {
		prefix := path[:depth]
		parent := adminNodeAt(tree, prefix)
		kids := adminChildNames(parent)
		if len(kids) == 0 && depth < len(path) {
			break
		}

		var selectedName string
		if depth < len(path) {
			selectedName = path[depth]
		}

		col := FinderColumn{}
		var tb textbuf.Buffer
		for _, name := range kids {
			childPath := append(append([]string{}, prefix...), name)
			childKey := textbuf.Join(childPath, "/")
			url := tb.Reset().Str("/admin/").Join(childPath, "/").Byte('/').String()

			col.UnnamedItems = append(col.UnnamedItems, ColumnItem{
				Name:        name,
				URL:         url,
				HxPath:      tb.Reset().Str("admin/").Str(childKey).String(),
				Selected:    name == selectedName,
				HasChildren: adminHasChildren(adminNodeAt(tree, childPath)),
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
	crumbs = append(crumbs, BreadcrumbSegment{Name: prefixAdmin, URL: "/admin/", Active: len(path) == 0})

	var tb textbuf.Buffer
	for i, seg := range path {
		url := tb.Reset().Str("/admin/").Join(path[:i+1], "/").Byte('/').String()
		crumbs = append(crumbs, BreadcrumbSegment{
			Name:   seg,
			URL:    url,
			Active: i == len(path)-1,
		})
	}

	return crumbs
}
