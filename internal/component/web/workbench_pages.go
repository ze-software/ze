// Design: plan/spec-web-4-interfaces.md -- Workbench page dispatch
// Related: handler_workbench.go -- Workbench handler that calls renderPageContent
// Related: page_interfaces.go -- Interface table and detail pages
// Related: page_traffic.go -- Traffic monitoring page
// Related: page_ip_addresses.go -- IP Addresses page
// Related: page_ip_routes.go -- IP Routes page
// Related: page_ip_dns.go -- DNS configuration form page

package web

import (
	"html/template"
	"net/http"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
)

// renderPageContent checks if the given path corresponds to a purpose-built
// page (Interfaces, IP, Tools, Logs, Dashboard sub-pages) and renders its
// content. Returns the rendered HTML and true if the path was handled, or
// empty HTML and false if the path should fall through to the generic YANG
// detail view.
func renderPageContent(renderer *Renderer, r *http.Request, path []string, viewTree *config.Tree, dispatch CommandDispatcher, broker *EventBroker) (template.HTML, bool) {
	if len(path) == 0 {
		return "", false
	}

	switch path[0] {
	case "iface":
		return HandleInterfacesPage(renderer, r, path[1:]), true
	case "ip":
		if len(path) < 2 {
			return "", false
		}
		switch path[1] {
		case "addresses":
			return HandleAddressesPage(renderer, r), true
		case "routes":
			return HandleRoutesPage(renderer, r), true
		case "dns":
			return HandleDNSPage(renderer, viewTree), true
		}
	case "bgp":
		return renderBGPPageContent(renderer, r, path[1:], viewTree)
	case segFirewall:
		return renderFirewallPageContent(renderer, r, path[1:])
	case segSystem:
		return renderSystemPageContent(renderer, path[1:], viewTree)
	case "users":
		return HandleUsersPage(renderer, viewTree), true
	case segL2TP:
		return renderL2TPPageContent(renderer, path[1:], viewTree)
	case segSSH, segTelemetry, segTACACS, segMCP, segLG, segAPI:
		return renderServicePageContent(renderer, path[0], viewTree)
	case segTools:
		return renderToolPageContent(renderer, r, path[1:], dispatch)
	case segLogs:
		return renderLogPageContent(renderer, r, path[1:], dispatch, broker)
	case segHealth:
		return HandleDashboardHealthPage(renderer, viewTree, r, dispatch), true
	case segEvents:
		return HandleDashboardEventsPage(renderer, r, dispatch), true
	}

	return "", false
}

// renderBGPPageContent dispatches BGP sub-pages. The path slice has the
// leading "bgp" segment already stripped. Returns (content, true) if a
// page handler matched, or ("", false) to fall through to generic YANG.
func renderBGPPageContent(renderer *Renderer, r *http.Request, path []string, viewTree *config.Tree) (template.HTML, bool) {
	if len(path) == 0 {
		// /show/bgp/ itself falls through to generic YANG detail.
		return "", false
	}

	switch path[0] {
	case "peer":
		filterGroup := r.URL.Query().Get("group")
		return HandleBGPPeersPage(renderer, viewTree, filterGroup), true
	case "group":
		// /show/bgp/group/ shows the groups table; deeper paths fall through.
		if len(path) == 1 || (len(path) == 2 && path[1] == "") {
			return HandleBGPGroupsPage(renderer, viewTree), true
		}
		return "", false
	case "summary":
		return HandleBGPSummaryPage(renderer, viewTree), true
	case "family":
		return HandleBGPFamiliesPage(renderer, viewTree), true
	case "policy":
		// /show/bgp/policy/ shows the filters table; deeper paths fall through.
		if len(path) == 1 || (len(path) == 2 && path[1] == "") {
			return HandleBGPPolicyPage(renderer, viewTree), true
		}
		return "", false
	}

	return "", false
}
