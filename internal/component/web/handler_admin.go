// Design: docs/architecture/web-interface.md -- Admin command handlers
// Related: handler.go -- URL routing
// Related: handler_config.go -- Config tree view handlers (navigation pattern)
// Related: render.go -- Template rendering

package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
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

// AdminViewData holds the data for rendering the admin command tree navigation.
// Containers show navigable links to sub-commands. Leaf commands show a form.
type AdminViewData struct {
	// Path is the current YANG path segments under /admin/.
	Path []string
	// Breadcrumbs is the navigation trail for /admin/ paths.
	Breadcrumbs []BreadcrumbSegment
	// Children lists sub-commands as navigable links (for container nodes).
	Children []ChildEntry
	// IsLeaf is true when the resolved node is a leaf command (no sub-commands).
	IsLeaf bool
	// Form holds the command form data when IsLeaf is true.
	Form *CommandFormData
}

// CommandDispatcher executes an admin command and returns the output.
// The command string is the full command path (e.g., "peer 192.168.1.1 teardown").
// Returns the output text and any error from execution.
type CommandDispatcher func(command string) (string, error)

// HandleAdminView returns an HTTP handler that serves the admin command tree
// navigation view. It builds breadcrumbs for /admin/ paths and lists
// sub-commands as navigable links. When children is nil (leaf command),
// it renders a command form instead.
//
// The children map provides the static command tree structure. Each key is
// a path prefix, and the value is the list of child command names at that
// level. An empty or missing entry means the path is a leaf command.
func HandleAdminView(renderer *Renderer, children map[string][]string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		parsed, err := ParseURL(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
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

		viewData := buildAdminViewData(path, children)

		// HTMX partial: return content fragment without layout wrapper.
		if r.Header.Get("HX-Request") == htmxRequestTrue {
			if viewData.IsLeaf && viewData.Form != nil {
				if err := renderer.RenderConfigTemplate(w, "command_form.html", viewData.Form); err != nil {
					http.Error(w, fmt.Sprintf("render: %v", err), http.StatusInternalServerError)
				}

				return
			}

			if err := renderer.RenderConfigTemplate(w, "container.html", containerFromAdmin(viewData)); err != nil {
				http.Error(w, fmt.Sprintf("render: %v", err), http.StatusInternalServerError)
			}

			return
		}

		// Full HTML: render inside layout with breadcrumb.
		layoutData := LayoutData{
			Title:      "Admin: /" + strings.Join(path, "/"),
			Breadcrumb: viewData.Breadcrumbs,
		}

		if err := renderer.RenderLayout(w, layoutData); err != nil {
			http.Error(w, fmt.Sprintf("render: %v", err), http.StatusInternalServerError)
		}
	}
}

// HandleAdminExecute returns an HTTP handler that executes admin commands
// via POST. It reconstructs the command string from the URL path segments,
// dispatches the command through the provided dispatcher, and returns a
// titled result card. On error, the result card has the error styling.
//
// Content negotiation: JSON requests receive the raw command output as
// a JSON object with "command", "output", and "error" fields.
func HandleAdminExecute(renderer *Renderer, dispatch CommandDispatcher) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		parsed, err := ParseURL(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		path := parsed.Path
		commandStr := strings.Join(path, " ")

		output, execErr := dispatch(commandStr)

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

		// HTML response: render result card template.
		if err := renderer.RenderConfigTemplate(w, "command.html", result); err != nil {
			http.Error(w, fmt.Sprintf("render: %v", err), http.StatusInternalServerError)
		}
	}
}

// buildAdminViewData assembles the view data for an admin command tree path.
// It builds breadcrumbs for /admin/ paths and determines whether the path
// points to a container (has children) or a leaf command (no children).
func buildAdminViewData(path []string, children map[string][]string) *AdminViewData {
	data := &AdminViewData{
		Path:        path,
		Breadcrumbs: buildAdminBreadcrumbs(path),
	}

	pathKey := strings.Join(path, "/")
	kids, hasChildren := children[pathKey]

	if !hasChildren || len(kids) == 0 {
		// Leaf command: render a form.
		data.IsLeaf = true
		data.Form = &CommandFormData{
			CommandName: strings.Join(path, " "),
			ActionURL:   "/admin/" + strings.Join(path, "/"),
		}

		return data
	}

	// Container: list children as navigable links.
	prefix := "/admin/"
	if len(path) > 0 {
		prefix = "/admin/" + strings.Join(path, "/") + "/"
	}

	for _, name := range kids {
		data.Children = append(data.Children, ChildEntry{
			Name: name,
			Kind: "container",
			URL:  prefix + name + "/",
		})
	}

	return data
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

// containerFromAdmin adapts AdminViewData to a minimal struct that the
// container.html template can render. This reuses the existing container
// template for command tree navigation.
func containerFromAdmin(data *AdminViewData) struct {
	ContainerChildren []ChildEntry
	ListChildren      []ChildEntry
	LeafFields        []LeafField
} {
	return struct {
		ContainerChildren []ChildEntry
		ListChildren      []ChildEntry
		LeafFields        []LeafField
	}{
		ContainerChildren: data.Children,
	}
}
