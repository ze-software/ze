// Design: docs/architecture/web-interface.md -- workbench shell handler
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
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// workbenchConfig holds optional dependencies for the workbench handler.
// Dependencies that are nil degrade gracefully (tool forms render but
// command dispatch is unavailable).
type workbenchConfig struct {
	dispatch   CommandDispatcher
	broker     *EventBroker
	powerUsers []string
	authorizer aaa.Authorizer
}

// WorkbenchOption configures optional workbench handler dependencies.
type WorkbenchOption func(*workbenchConfig)

// WithDispatch sets the CommandDispatcher for tool and log page handlers.
func WithDispatch(d CommandDispatcher) WorkbenchOption {
	return func(c *workbenchConfig) { c.dispatch = d }
}

// WithAuthorizer sets the aaa.Authorizer used to hide edit controls from
// read-only users (AC-1). Nil (the default) keeps everything editable, matching
// the route-gate fail-open semantics (CanEdit / R-1).
func WithAuthorizer(a aaa.Authorizer) WorkbenchOption {
	return func(c *workbenchConfig) { c.authorizer = a }
}

// WithBroker sets the EventBroker for Live Log SSE streaming.
func WithBroker(b *EventBroker) WorkbenchOption {
	return func(c *workbenchConfig) { c.broker = b }
}

// WithPowerUsers sets the zefs power user names for the Users page.
func WithPowerUsers(names []string) WorkbenchOption {
	return func(c *workbenchConfig) { c.powerUsers = names }
}

// HandleWorkbench returns an HTTP handler that serves /show/* and the root
// page in workbench mode. HTMX partial requests fall back to the existing
// fragment OOB response so HTMX-driven navigation continues to work; only
// the full-page render is replaced by the workbench shell.
func HandleWorkbench(renderer *Renderer, schema *config.Schema, tree *config.Tree, mgr *EditorManager, insecure bool, opts ...WorkbenchOption) http.HandlerFunc {
	var cfg workbenchConfig
	for _, o := range opts {
		o(&cfg)
	}

	return func(w http.ResponseWriter, r *http.Request) {
		path := extractPath(r)
		if err := ValidatePathSegments(path); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		viewTree := tree
		username := GetUsernameFromRequest(r)
		// Hide edit controls from read-only users (AC-1). Enforcement is at the
		// route/mutation layer; this is the matching UI gate so we never show a
		// button that would 403 on click. Fail-open when no authorizer (R-1).
		readOnly := !canEdit(r, cfg.authorizer)
		if mgr != nil && username != "" {
			if userTree := mgr.Tree(username); userTree != nil {
				viewTree = userTree
			}
		}

		routerIdentity := resolveRouterIdentity(viewTree)
		fleetPeers := CollectFleetPeers(viewTree, routerIdentity)
		changeCount := 0
		if mgr != nil && username != "" {
			changeCount = mgr.ChangeCount(username)
		}

		// Purpose-built pages handle their own data sourcing and do not
		// walk the YANG schema. Detect them before the generic schema walk.
		if pageContent, handled := renderPageContent(renderer, r, path, viewTree, schema, cfg.dispatch, cfg.broker, cfg.powerUsers); handled {
			if r.Header.Get("HX-Request") == htmxRequestTrue {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				if _, writeErr := w.Write([]byte(pageContent)); writeErr != nil {
					return
				}
				return
			}

			data := buildFragmentData(schema, viewTree, nil)
			data.Username = username
			data.Insecure = insecure
			data.Services = PortalServices()
			data.ActiveUI = uiModeTokenWorkbench
			data.ReadOnly = readOnly
			pathBar := renderer.renderComponent("path_bar_inner", pathBarInner(data))

			wb := workbenchData{
				LayoutData: LayoutData{
					Title:          func() string { var tb textbuf.Buffer; return tb.Str("Ze: /").Join(path, "/").String() }(),
					Content:        pageContent,
					HasSession:     true,
					CLIPrompt:      data.CLIPrompt,
					CLIContextPath: data.CLIContextPath,
					CLIPathBar:     pathBar,
					Breadcrumbs:    data.Breadcrumbs,
					Username:       data.Username,
					Insecure:       insecure,
					ActiveUI:       uiModeTokenWorkbench,
					RouterIdentity: routerIdentity,
					FleetPeers:     fleetPeers,
					ChangeCount:    changeCount,
					ReadOnly:       readOnly,
				},
				Sections: workbenchSections(path),
			}

			if renderErr := renderer.RenderWorkbench(w, wb); renderErr != nil {
				http.Error(w, fmt.Sprintf("render: %v", renderErr), http.StatusInternalServerError)
			}
			return
		}

		if len(path) > 0 {
			var tb textbuf.Buffer
			schemaNode, walkErr := walkSchema(schema, path)
			if walkErr != nil || schemaNode == nil {
				target := tb.Str("/show/?error=").Str(url.QueryEscape(tb.Reset().Str("invalid path: ").Join(path, "/").String())).String()
				http.Redirect(w, r, target, http.StatusFound)
				return
			}
			if isListEntryPath(schema, path) && walkTree(viewTree, schema, path) == nil {
				entryKey := path[len(path)-1]
				target := tb.Reset().Str("/show/?error=").Str(url.QueryEscape(tb.Reset().Str("entry ").Str(strconv.Quote(entryKey)).Str(" does not exist").String())).String()
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
		data.ActiveUI = uiModeTokenWorkbench
		data.ReadOnly = readOnly

		// Workbench-only enrichment: surface row tool buttons and pending-change
		// markers. The Finder fragment handler skips this so its output is
		// unchanged.
		var pendingPaths []string
		if mgr != nil && username != "" {
			pendingPaths = mgr.pendingChangePaths(username)
		}
		enrichWorkbenchTable(data, schema, viewTree, path, pendingPaths)

		if r.Header.Get("HX-Request") == htmxRequestTrue {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			html := renderer.renderComponent("oob_response", oobResponse(data))
			if _, writeErr := w.Write([]byte(html)); writeErr != nil {
				return
			}
			return
		}

		// Full page: at the root path render the dashboard overview; for
		// all other paths render the existing detail fragment.
		var content template.HTML
		if len(path) == 0 {
			dashData := buildDashboardData(viewTree, schema)
			content = renderer.renderComponent("workbench_dashboard", workbenchDashboard(dashData))
		} else {
			content = renderer.renderComponent("detail", detail(data))
		}
		pathBar := renderer.renderComponent("path_bar_inner", pathBarInner(data))

		var tb2 textbuf.Buffer
		wb := workbenchData{
			LayoutData: LayoutData{
				Title:          tb2.Str("Ze: /").Str(data.CurrentPath).String(),
				Content:        content,
				HasSession:     true,
				CLIPrompt:      data.CLIPrompt,
				CLIContextPath: data.CLIContextPath,
				CLIPathBar:     pathBar,
				Breadcrumbs:    data.Breadcrumbs,
				Username:       data.Username,
				Insecure:       insecure,
				ActiveUI:       uiModeTokenWorkbench,
				RouterIdentity: routerIdentity,
				FleetPeers:     fleetPeers,
				ChangeCount:    changeCount,
				ReadOnly:       readOnly,
			},
			Sections: workbenchSections(path),
		}

		if err := renderer.RenderWorkbench(w, wb); err != nil {
			http.Error(w, fmt.Sprintf("render: %v", err), http.StatusInternalServerError)
		}
	}
}
