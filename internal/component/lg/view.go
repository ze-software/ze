// Design: docs/architecture/web-interface.md -- LG view models
// Overview: server.go -- LG server and route registration
// Related: handler_ui.go -- the handlers that build these structs

package lg

// The looking glass passed map[string]any to html/template until 2026-08-14.
// A map key the markup misspells renders empty and reports no error, so a
// renamed field produced a blank panel and nothing in the log. Every struct
// below replaces one of those maps. A field the markup misspells is now a
// build failure.

// layoutView is the chrome every full page shares.
type layoutView struct {
	// Title is the browser title, before " - Ze Looking Glass".
	Title string
	// ActiveTab marks which tab in the header carries tab-active.
	ActiveTab string
}

// peerRow is one row of the BGP peer table. Every counter is a string,
// because the engine omits a count it cannot produce. An omitted count renders
// empty rather than as a zero Ze never sent (handler_api.go,
// routeCountsAvailable).
type peerRow struct {
	Address         string
	RemoteAS        string
	RemoteASName    string
	State           string
	Uptime          string
	RoutesReceived  string
	RoutesAccepted  string
	RoutesSent      string
	UpdatesReceived string
	UpdatesSent     string
	Description     string
}

// bmpPeerRow is one row of the BMP monitored-peer table.
type bmpPeerRow struct {
	Router     string
	BGPID      string
	PeerAS     string
	PeerASName string
	State      string
	IPv6       bool
}

// routeRow is one route as the browser shows it. The BGP RIB reaches the
// looking glass as decoded JSON, so this is where that JSON becomes typed.
type routeRow struct {
	Prefix              string
	NextHop             string
	ASPath              []string
	Origin              string
	LocalPreference     string
	MED                 string
	PeerAddress         string
	Best                bool
	Communities         []string
	LargeCommunities    []string
	ExtendedCommunities []string
}

// prefixSummaryRow is one line of the prefix-length summary a large peer gets
// instead of its route list.
type prefixSummaryRow struct {
	Family string
	Length string
	Count  string
}

// peerInfoRow is the peer header above a peer's routes.
type peerInfoRow struct {
	State        string
	RemoteAS     string
	RemoteASName string
	Description  string
}

// peersView is the peer dashboard.
type peersView struct {
	layoutView
	Peers    []peerRow
	BMPPeers []bmpPeerRow
	Error    string
}

// The help page has no view struct. It reads nothing, so helpPage takes no
// parameter and handleUIHelp builds only the layoutView the chrome needs.

// searchView is the route search page and every fragment it swaps in.
//
// ONE struct serves all three call sites. That is a decision, not an accident.
// handleUISearchForm sends four empty filters. handleUISearch adds Routes and
// Count. renderSearchError adds Error.
//
// The markup reads the union of the three. Three structs would encode a
// distinction no template makes. A pointer field would spell "absent" where
// the markup asks only "empty". A call site with no routes leaves Routes nil,
// which is what missingkey=zero gave it before.
type searchView struct {
	layoutView
	Prefix    string
	ASPath    string
	Community string
	Family    string
	Routes    []routeRow
	Count     int
	Error     string
}

// peerRoutesView is one peer's route table, or its prefix-length summary when
// the table is too large to show.
type peerRoutesView struct {
	layoutView
	Address string
	// Peer is nil when the peer is gone. handleUIPeerRoutes answers 404 in that
	// case, so only the template capture reaches the nil branch today.
	Peer          *peerInfoRow
	PrefixSummary []prefixSummaryRow
	Routes        []routeRow
	Error         string
}

// routeDetailView is the expanded attribute panel for one route.
type routeDetailView struct {
	Prefix string
	// Route is nil when no route matched the prefix and peer.
	Route *routeRow
}
