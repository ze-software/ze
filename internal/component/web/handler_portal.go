// Design: docs/architecture/web-interface.md -- Portal frame for embedded services
// Related: render.go -- Template rendering and LayoutData
// Related: handler.go -- URL routing

package web

import (
	"fmt"
	"html/template"
	"net/http"
	"sync"
)

// PortalService describes an external service that can be embedded in the
// portal iframe. Available is checked at render time so the breadcrumb
// menu only shows reachable services.
type PortalService struct {
	Key   string
	Title string
	Path  string
	Icon  string
}

var (
	portalMu       sync.RWMutex
	portalServices []PortalService
)

// RegisterPortalService adds a service to the portal menu. Call during
// startup before the HTTP server begins serving.
func RegisterPortalService(svc PortalService) {
	portalMu.Lock()
	portalServices = append(portalServices, svc)
	portalMu.Unlock()
}

// PortalServices returns the currently registered portal services.
func PortalServices() []PortalService {
	portalMu.RLock()
	out := make([]PortalService, len(portalServices))
	copy(out, portalServices)
	portalMu.RUnlock()
	return out
}

// portalTarget looks up a registered service by key.
func portalTarget(key string) (PortalService, bool) {
	portalMu.RLock()
	defer portalMu.RUnlock()
	for _, svc := range portalServices {
		if svc.Key == key {
			return svc, true
		}
	}
	return PortalService{}, false
}

// HandlePortal returns an HTTP handler that renders the ze layout with an
// iframe embedding the requested service. The URL pattern is /portal/{key}
// where key matches a registered PortalService. This avoids open redirects
// by only allowing pre-registered targets.
//
// The portal page renders inside the workbench or finder shell based on the
// user's ze-ui cookie (falling back to defaultMode) so the topbar remains
// visible for navigation back.
func HandlePortal(renderer *Renderer, defaultMode UIMode) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := extractPortalKey(r)
		if key == "" {
			http.Redirect(w, r, "/show/", http.StatusFound)
			return
		}

		svc, ok := portalTarget(key)
		if !ok {
			http.Error(w, "unknown portal service", http.StatusNotFound)
			return
		}

		username := GetUsernameFromRequest(r)

		//nolint:gosec // svc.Path comes from pre-registered whitelist, not user input
		content := template.HTML(
			`<iframe src="` + template.HTMLEscapeString(svc.Path) +
				`" class="portal-frame" title="` + template.HTMLEscapeString(svc.Title) +
				`"></iframe>`,
		)

		breadcrumbs := []BreadcrumbSegment{
			{Name: "portal", URL: "/show/"},
			{Name: svc.Title, URL: "/portal/" + svc.Key, Active: true},
		}

		mode := ReadUIModeFromRequest(r, defaultMode)
		layoutData := LayoutData{
			Title:       "Ze: " + svc.Title,
			Content:     content,
			HasSession:  true,
			Breadcrumbs: breadcrumbs,
			Username:    username,
			ActiveUI:    mode.String(),
		}

		if mode == UIModeWorkbench {
			wb := WorkbenchData{
				LayoutData: layoutData,
				Sections:   WorkbenchSections(nil),
			}
			if err := renderer.RenderWorkbench(w, wb); err != nil {
				http.Error(w, fmt.Sprintf("render: %v", err), http.StatusInternalServerError)
			}
			return
		}

		if err := renderer.RenderLayout(w, layoutData); err != nil {
			http.Error(w, fmt.Sprintf("render: %v", err), http.StatusInternalServerError)
		}
	}
}

// extractPortalKey extracts the service key from /portal/{key}.
func extractPortalKey(r *http.Request) string {
	path := r.URL.Path
	const prefix = "/portal/"
	if len(path) <= len(prefix) {
		return ""
	}
	key := path[len(prefix):]
	if key != "" && key[len(key)-1] == '/' {
		key = key[:len(key)-1]
	}
	return key
}
