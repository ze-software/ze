// Design: docs/architecture/web-workbench-pages.md -- Workbench page dispatch
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

	"github.com/ze-software/ze/internal/component/config"
)

const (
	bgpGroupSegment = "group"
	bgpPeerSegment  = "peer"
)

// renderPageContent checks if the given path corresponds to a purpose-built
// page (Interfaces, IP, Tools, Logs, Dashboard sub-pages) and renders its
// content. Returns the rendered HTML and true if the path was handled, or
// empty HTML and false if the path should fall through to the generic YANG
// detail view.
//
// Every page below reads its own values out of the tree by path. The leaf the
// schema marks secret is unidentifiable by the time a value reaches a field.
// The tree they read is masked instead (secret.go), and display() is the only
// way a page below receives one. No branch passes viewTree, which is the raw
// working tree, and TestWorkbenchPagesReceiveOnlyTheMaskedTree reads this
// function's source to keep that true.
//
// The mask deep-copies the configuration, so display() computes it at most once
// and only for a branch that reads config. A tools, logs, firewall or VPN page
// reads none, and neither does the fall-through to the generic YANG view, which
// masks per leaf.
func renderPageContent(renderer *Renderer, r *http.Request, path []string, viewTree *config.Tree, schema *config.Schema, dispatch CommandDispatcher, broker *EventBroker, powerUsers []string) (template.HTML, bool) {
	if len(path) == 0 {
		return "", false
	}

	var masked *config.Tree
	display := func() *config.Tree {
		if masked == nil {
			masked = config.MaskSecrets(viewTree, schema)
		}
		return masked
	}

	switch path[0] {
	case segIface:
		return handleInterfacesPage(renderer, r, path[1:], display()), true
	case "ip":
		if len(path) < 2 {
			return "", false
		}
		switch path[1] {
		case segAddresses:
			return handleAddressesPage(renderer, r), true
		case "routes":
			return HandleRoutesPage(renderer, r), true
		case segDNS:
			return handleDNSPage(renderer, display()), true
		}
	case segBGP:
		return renderBGPPageContent(renderer, r, path[1:], display(), schema, dispatch)
	case segFirewall:
		return renderFirewallPageContent(renderer, r, path[1:])
	case segSystem:
		return renderSystemPageContent(renderer, path[1:], display())
	case segUsers:
		return handleUsersPage(renderer, display(), powerUsers), true
	case segL2TP:
		return renderL2TPPageContent(renderer, path[1:], display())
	case segSSH, segWeb, segTelemetry, segTACACS, segMCP, segLG, segAPI:
		return renderServicePageContent(renderer, path[0], display())
	case "vpn":
		return renderVPNPageContent(renderer, r, path[1:])
	case segTools:
		return renderToolPageContent(renderer, r, path[1:], dispatch)
	case segLogs:
		return renderLogPageContent(renderer, r, path[1:], dispatch, broker)
	case segHealth:
		return handleDashboardHealthPage(renderer, display(), r, dispatch), true
	case segEvents:
		return handleDashboardEventsPage(renderer, r, dispatch), true
	}

	return "", false
}

// renderBGPPageContent dispatches BGP sub-pages. The path slice has the
// leading "bgp" segment already stripped. Returns (content, true) if a
// page handler matched, or ("", false) to fall through to generic YANG.
func renderBGPPageContent(renderer *Renderer, r *http.Request, path []string, viewTree *config.Tree, schema *config.Schema, dispatch CommandDispatcher) (template.HTML, bool) {
	if len(path) == 0 {
		// /show/bgp/ itself falls through to generic YANG detail.
		return "", false
	}

	switch path[0] {
	case bgpPeerSegment:
		// /show/bgp/peer/ shows the peers table; /show/bgp/peer/<name>/ falls
		// through to the generic YANG detail view (the peer's editable config),
		// mirroring how bgp/group/<name> is handled below. Without this, the
		// peer row "Edit" link re-rendered the whole table instead of the peer.
		if len(path) == 1 || (len(path) == 2 && path[1] == "") {
			filterGroup := r.URL.Query().Get(bgpGroupSegment)
			return handleBGPPeersPage(renderer, r, viewTree, filterGroup, dispatch), true
		}
		return "", false
	case bgpGroupSegment:
		// /show/bgp/group/ shows the groups table.
		if len(path) == 1 || (len(path) == 2 && path[1] == "") {
			return handleBGPGroupsPage(renderer, viewTree), true
		}
		// /show/bgp/group/<name>/peer/ shows the peer table scoped to the group.
		if len(path) >= 3 && path[2] == bgpPeerSegment {
			return handleBGPPeersPage(renderer, r, viewTree, path[1], dispatch), true
		}
		return "", false
	case segSummary:
		return handleBGPSummaryPage(renderer, r, viewTree, dispatch), true
	case segFamily:
		return handleBGPFamiliesPage(renderer, viewTree), true
	case segPolicy:
		// /show/bgp/policy/ shows the filters table; deeper paths fall through.
		if len(path) == 1 || (len(path) == 2 && path[1] == "") {
			return handleBGPPolicyPage(renderer, viewTree, schema), true
		}
		return "", false
	}

	return "", false
}
