// Design: docs/architecture/web-interface.md -- LG view models
// Overview: server.go -- LG server and route registration
// Related: handler_ui.go -- the handlers that build these structs
// Related: peers.templ -- the peer table that renders peerRow and peersColumns

package lg

import "strconv"

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
	// Page names the body the layout wraps, so the head loads that page's
	// assets rather than every page's. page_assets.go carries the sets and
	// internal/le/web/assets/webassets.go derives them. A zero Page is a page nobody
	// named, and it gets every asset: an extra file costs bytes, while a
	// missing one gives a page that renders correctly and does nothing.
	Page pageID
}

// peerRow is one row of the BGP peer table. Every counter is a string,
// because the engine omits a count it cannot produce. An omitted count renders
// empty rather than as a zero Ze never sent (handler_api.go,
// routeCountsAvailable).
//
// StateChanged, ConnectionsDropped and LastError are the session history the
// engine answers under `state-changed`, `connections-dropped` and `last-error`
// (bgp/plugins/cmd/peer/summary.go, handleBgpSummary). The looking glass shows
// them as given and computes nothing from them.
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
	// StateChanged is the time of the last state change, in the engine's
	// RFC 3339 text. The engine answers "" for a peer that never changed
	// state, and the cell is then empty.
	StateChanged string
	// ConnectionsDropped is how often the session dropped. Its zero value is
	// "the engine sent no count", which renders an empty cell.
	ConnectionsDropped connectionsDropped
	// LastError is the text of the last error, "" for a peer with none. The
	// table has no column for it: the peer header shows it (peerInfoRow).
	LastError string
	// Group is the name of the peer's group, "" for a peer in none. The table
	// has no column for it: it is the tooltip of the description cell, and the
	// peer header shows it.
	Group string
}

// connectionsDropped is the number of times a peer's session dropped, or the fact
// that the engine sent none. The two are apart because a zero is a true count
// (a session that never dropped) and an operator must never read it for a row
// whose count the engine did not answer.
//
// Only connectionsDroppedFrom builds a known one. The zero value is the unknown
// count.
type connectionsDropped struct {
	count uint64
	known bool
}

// Text is the count as the page shows it: grouped digits for a known count,
// and "" for one the engine did not send.
func (c connectionsDropped) Text() string {
	if !c.known {
		return ""
	}

	return formatNum(strconv.FormatUint(c.count, 10))
}

// peersColumns is the header of the peer table, one entry per cell of a
// peersTableBody row and in the same order. peersStreamError spans its length,
// so the reason row always covers the whole table.
var peersColumns = [...]string{
	"Peer", "Remote AS", "State", "Uptime", "Last Change", "Drops",
	"Received", "Accepted", "Sent", "Updates In", "Updates Out", "Description",
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

	// NextHopName is the name the operator gave the range NextHop is in
	// (nexthop_ranges.go), "" for a next hop in no named range. Nothing else
	// fills it: Ze derives no name.
	NextHopName string

	// Age is how long ago bgp-rib last stored a change to the route, as the
	// row's `age` member says it. A row with no reading (a sent route, a best
	// path row) holds the zero value, which renders an empty cell.
	Age routeAge
}

// routeAge is the whole seconds since a route last changed, or the fact that
// the engine sent no reading. The two are apart because zero is a true age (a
// route stored this second) and a reader must never see it for a row whose
// age the engine did not answer. The looking glass computes nothing here: it
// carries the seconds bgp-rib wrote in the row.
//
// Only routeAgeFrom builds a known one. The zero value is the unknown age.
type routeAge struct {
	seconds uint64
	known   bool
}

// Text is the age as the page shows it, in the form of the peer table's
// uptime (formatElapsed), and "" for an age the engine did not send.
func (a routeAge) Text() string {
	if !a.known {
		return ""
	}

	return formatElapsed(a.seconds)
}

// Seconds is the exact age in decimal seconds, for the cell's data-seconds
// attribute, and "" for an age the engine did not send.
func (a routeAge) Seconds() string {
	if !a.known {
		return ""
	}

	return strconv.FormatUint(a.seconds, 10)
}

// histogramRow is one line of the prefix-length histogram a large peer gets
// instead of its route list.
type histogramRow struct {
	Family string
	Length string
	Count  string
}

// peerInfoRow is the peer header above a peer's routes. It carries the same
// session history as the peer's table row, and it is where the last error is
// shown: the text is too long for a column.
type peerInfoRow struct {
	State              string
	RemoteAS           string
	RemoteASName       string
	Description        string
	StateChanged       string
	ConnectionsDropped connectionsDropped
	LastError          string

	// Group is the name of the peer's group, "" for a peer in none. The
	// description above is the peer's own label and never the group.
	Group string
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
	Peer *peerInfoRow
	// RoutesReceived counts the routes the peer sent Ze (bgp-rib's `received`
	// scope) and RoutesSent the routes Ze sent the peer (its `sent` scope).
	// A count the engine could not answer is the empty string, rendered as an
	// empty cell, never as a zero.
	RoutesReceived string
	RoutesSent     string
	Histogram      []histogramRow
	Routes         []routeRow
	Error          string
}

// routeDetailView is the expanded attribute panel for one route.
type routeDetailView struct {
	Prefix string
	// Route is nil when no route matched the prefix and peer.
	Route *routeRow
}
