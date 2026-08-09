// Design: docs/architecture/web-workbench-pages.md -- generic system/service workbench dispatch
// Related: page_l2tp.go -- the ze_l2tp-gated BNG pages these were split from
// Related: workbench_pages.go -- the top-level workbench dispatcher

package web

import (
	"html/template"

	"github.com/ze-software/ze/internal/component/config"
)

// renderSystemPageContent dispatches system sub-pages. The path slice has the
// leading "system" segment already stripped. Returns (content, true) if a page
// handler matched, or ("", false) to fall through to generic YANG.
func renderSystemPageContent(renderer *Renderer, path []string, viewTree *config.Tree) (template.HTML, bool) {
	if len(path) == 0 || (len(path) == 1 && path[0] == "") {
		// /show/system/ defaults to identity.
		return HandleSystemIdentityPage(renderer, viewTree), true
	}

	switch path[0] {
	case "identity":
		return HandleSystemIdentityPage(renderer, viewTree), true
	case "resources":
		return HandleResourcesPage(), true
	case "hardware":
		return HandleHostHardwarePage(), true
	case "sysctl":
		return HandleSysctlProfilesPage(renderer, viewTree), true
	}

	return "", false
}

// renderServicePageContent dispatches a service page for a top-level path
// segment like "ssh", "web", etc. Returns (content, true) if handled.
func renderServicePageContent(renderer *Renderer, segment string, viewTree *config.Tree) (template.HTML, bool) {
	switch segment {
	case segSSH:
		return HandleSSHPage(renderer, viewTree), true
	case segWeb:
		return HandleWebServicePage(renderer, viewTree), true
	case segTelemetry:
		return HandleTelemetryPage(renderer, viewTree), true
	case segTACACS:
		return HandleTACACSPage(renderer, viewTree), true
	case segMCP:
		return HandleMCPPage(renderer, viewTree), true
	case segLG:
		return HandleLookingGlassPage(renderer, viewTree), true
	case segAPI:
		return HandleAPIPage(renderer, viewTree), true
	}
	return "", false
}
