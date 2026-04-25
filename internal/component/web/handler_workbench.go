// Design: docs/architecture/web-interface.md -- V2 workbench shell handler
// Related: fragment.go -- shared fragment data builder reused by the workbench
// Related: render.go -- WorkbenchData and RenderWorkbench
// Related: workbench_sections.go -- left navigation taxonomy
// Related: ui_mode.go -- runtime selector that picks between Finder and workbench
//
// Spec: plan/spec-web-2-operator-workbench.md (Phase 1).
//
// The workbench handler reuses the same fragment data path the Finder handler
// uses; only the page chrome differs. The workspace area renders the existing
// `detail` fragment so list tables, fields, and command results appear inside
// the workbench shell exactly as they do today inside Finder.

package web

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
)

// HandleWorkbench returns an HTTP handler that serves /show/* and the root
// page in workbench mode. HTMX partial requests fall back to the existing
// fragment OOB response so HTMX-driven navigation continues to work; only
// the full-page render is replaced by the workbench shell.
func HandleWorkbench(renderer *Renderer, schema *config.Schema, tree *config.Tree, mgr *EditorManager, insecure bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := extractPath(r)
		if err := ValidatePathSegments(path); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		viewTree := tree
		username := GetUsernameFromRequest(r)
		if mgr != nil && username != "" {
			if userTree := mgr.Tree(username); userTree != nil {
				viewTree = userTree
			}
		}

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
		renderer.ResolveDecorations(data.Fields)
		data.Username = username
		data.Insecure = insecure
		data.Services = PortalServices()
		data.Monitor = strings.HasPrefix(r.URL.Path, "/monitor/")

		// V2-only enrichment: surface row tool buttons and pending-change
		// markers. The Finder fragment handler skips this so its output is
		// unchanged.
		var pendingPaths []string
		if mgr != nil && username != "" {
			pendingPaths = mgr.PendingChangePaths(username)
		}
		enrichWorkbenchTable(data, schema, viewTree, path, pendingPaths)

		if r.Header.Get("HX-Request") == "true" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			html := renderer.RenderFragment("oob_response", data)
			if _, writeErr := w.Write([]byte(html)); writeErr != nil {
				return
			}
			return
		}

		// Full page: render the workspace from the existing detail fragment so
		// list tables, fields, and command results appear inside the workbench.
		content := renderer.RenderFragment("detail", data)
		pathBar := renderer.RenderFragment("path_bar_inner", data)

		wb := WorkbenchData{
			LayoutData: LayoutData{
				Title:          "Ze: /" + data.CurrentPath,
				Content:        content,
				HasSession:     true,
				CLIPrompt:      data.CLIPrompt,
				CLIContextPath: data.CLIContextPath,
				CLIPathBar:     pathBar,
				Breadcrumbs:    data.Breadcrumbs,
				Username:       data.Username,
				Insecure:       insecure,
			},
			Sections: WorkbenchSections(path),
		}

		if err := renderer.RenderWorkbench(w, wb); err != nil {
			http.Error(w, fmt.Sprintf("render: %v", err), http.StatusInternalServerError)
		}
	}
}
