// Design: docs/architecture/api/commands.md — the payload keys the peer handlers answer with
// Overview: peer.go — the handlers that write these keys

package peer

// The keys these handlers write into a response, and the same keys the column
// orders and address-field lists in peer.go, health.go and summary.go declare.
//
// Each one is named in two places at least: the declaration that publishes it,
// and the row builder that writes it. The renderer pairs the two by exact
// spelling (commandRegistry.lookup in
// internal/component/command/column_order.go), so a typo in either spelling is
// SILENT -- the column stops being ordered, or `| resolve` stops finding the
// address, and nothing reports it. Naming the key once removes that pairing
// from the reader's hands.
//
// The keys NOT named here are written in one or two places, where a reader
// sees every spelling at once and a name removes no pairing. Two vocabularies
// stay literal for a stronger reason than that, and MUST NOT be folded in even
// though they spell the same words: the `| summary` and `| peers` alias names
// in peer.go are what an OPERATOR types, and the path segments in
// prefix_update.go name YANG schema nodes.
const (
	// The peer address, under the two names the answers give it. `show bgp`
	// and `show bgp peer statistics` call it "address"; `show bgp health` and
	// `show bgp peer capabilities` call it "peer" (summary.go documents why
	// the two rows differ).
	fieldAddress = "address"
	fieldPeer    = "peer"

	// The rows themselves, under the key every multi-peer answer holds them in.
	fieldPeers = "peers"

	// What the session IS: who configured it, what it negotiated, how long it
	// has been up.
	fieldName                = "name"
	fieldGroup               = "group"
	fieldState               = "state"
	fieldUptime              = "uptime"
	fieldRemoteAS            = "remote-as"
	fieldLocalAS             = "local-as"
	fieldRouterID            = "router-id"
	fieldPeerType            = "peer-type"
	fieldNegotiationComplete = "negotiation-complete"

	// What the session has CARRIED, as cumulative counters.
	fieldUpdatesReceived    = "updates-received"
	fieldUpdatesSent        = "updates-sent"
	fieldKeepalivesReceived = "keepalives-received"
	fieldKeepalivesSent     = "keepalives-sent"
	fieldEORReceived        = "eor-received"
	fieldEORSent            = "eor-sent"
	fieldConnectionsDropped = "connections-dropped"

	// The same counters divided by uptime, which `show bgp peer statistics`
	// answers beside them.
	fieldRateUpdatesReceived    = "rate-updates-received"
	fieldRateUpdatesSent        = "rate-updates-sent"
	fieldRateKeepalivesReceived = "rate-keepalives-received"
	fieldRateKeepalivesSent     = "rate-keepalives-sent"

	// The operation a request handler ran, echoed back so the operator reads
	// what happened rather than inferring it from an empty answer.
	fieldAction = "action"
)
