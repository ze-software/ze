// Design: docs/architecture/exabgp-bridge.md -- single event and command declaration tables for the ExaBGP translator.
// Related: bridge_event.go -- event kinds and ExaBGP API keys rendered onto script stdin.
// Related: bridge_command.go -- Ze CLI commands translated from script stdout.

package bridge

import (
	"slices"

	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/internal/core/events"
)

// Direction tag values carried by bidirectional BGP bus messages (`bgp:open`,
// `bgp:update`, `bgp:notification`, `bgp:keepalive`, `bgp:refresh`).
const (
	BusDirectionReceived = "received"
	BusDirectionSent     = "sent"
)

// BusMessageMapping pairs one Ze bus message name with its optional direction
// tag value for an ExaBGP neighbor `api` key (Q-E4, option (a)).
type BusMessageMapping struct {
	// Message is the canonical bus message name (e.g. "bgp:update").
	Message string
	// Direction is the direction tag value ("received" or "sent"), or empty
	// for directionless lifecycle events (state, connected, negotiated, fsm,
	// signal).
	Direction string
}

const (
	busMsgState        = bgpevents.Namespace + ":" + bgpevents.EventState
	busMsgConnected    = bgpevents.Namespace + ":" + bgpevents.EventConnected
	busMsgNegotiated   = bgpevents.Namespace + ":" + bgpevents.EventNegotiated
	busMsgFSM          = bgpevents.Namespace + ":" + bgpevents.EventFSM
	busMsgSignal       = bgpevents.Namespace + ":" + bgpevents.EventSignal
	busMsgOpen         = bgpevents.Namespace + ":" + bgpevents.EventOpen
	busMsgUpdate       = bgpevents.Namespace + ":" + bgpevents.EventUpdate
	busMsgNotification = bgpevents.Namespace + ":" + bgpevents.EventNotification
	busMsgKeepalive    = bgpevents.Namespace + ":" + bgpevents.EventKeepalive
	busMsgRefresh      = bgpevents.Namespace + ":" + bgpevents.EventRefresh
)

// declaredEventMessages is the single source of truth for every bus event
// message the ExaBGP translator renders onto script stdin.
var declaredEventMessages = []string{
	busMsgState,
	busMsgConnected,
	busMsgNegotiated,
	busMsgFSM,
	busMsgSignal,
	busMsgOpen,
	busMsgUpdate,
	busMsgNotification,
	busMsgKeepalive,
	busMsgRefresh,
}

// apiKeyBusMappings maps each supported ExaBGP neighbor `api` event key to the
// Ze bus message(s) and optional direction tag value that carry it (Q-E4).
var apiKeyBusMappings = map[string][]BusMessageMapping{
	apiKeyNeighborChanges:             {{Message: busMsgState}, {Message: busMsgConnected}},
	apiKeyNegotiated:                  {{Message: busMsgNegotiated}},
	apiKeyFSM:                         {{Message: busMsgFSM}},
	apiKeySignal:                      {{Message: busMsgSignal}},
	modeReceive + "-" + msgTypeOpen:   {{Message: busMsgOpen, Direction: BusDirectionReceived}},
	modeSend + "-" + msgTypeOpen:      {{Message: busMsgOpen, Direction: BusDirectionSent}},
	modeReceive + "-" + msgTypeUpdate: {{Message: busMsgUpdate, Direction: BusDirectionReceived}},
	modeSend + "-" + msgTypeUpdate:    {{Message: busMsgUpdate, Direction: BusDirectionSent}},
	modeReceive + "-" + msgTypeNotification: {
		{Message: busMsgNotification, Direction: BusDirectionReceived},
	},
	modeSend + "-" + msgTypeNotification: {
		{Message: busMsgNotification, Direction: BusDirectionSent},
	},
	modeReceive + "-" + msgTypeKeepalive: {
		{Message: busMsgKeepalive, Direction: BusDirectionReceived},
	},
	modeSend + "-" + msgTypeKeepalive: {
		{Message: busMsgKeepalive, Direction: BusDirectionSent},
	},
	modeReceive + "-" + msgTypeRefresh: {
		{Message: busMsgRefresh, Direction: BusDirectionReceived},
	},
	modeSend + "-" + msgTypeRefresh: {
		{Message: busMsgRefresh, Direction: BusDirectionSent},
	},
}

// formAPIKeys holds the six ExaBGP wire-form modifiers (`<dir>-<form>`). They
// control the translator's `--form` rendering rather than bus message
// subscriptions.
var formAPIKeys = map[string]bool{
	modeReceive + "-packets":     true,
	modeReceive + "-consolidate": true,
	modeReceive + "-parsed":      true,
	modeSend + "-packets":        true,
	modeSend + "-consolidate":    true,
	modeSend + "-parsed":         true,
}

// declaredCommandPrefixes is the single source of truth for every `cli:`
// command path prefix the ExaBGP translator emits and dispatches to Ze.
var declaredCommandPrefixes = []string{
	events.NamespaceCLI + ":send bgp",
	events.NamespaceCLI + ":request peer",
	events.NamespaceCLI + ":show bgp peer",
	events.NamespaceCLI + ":clear bgp rib in",
	events.NamespaceCLI + ":clear bgp rib advertised",
	events.NamespaceCLI + ":clear bgp rib out",
	events.NamespaceCLI + ":request bgp watchdog announce",
	events.NamespaceCLI + ":request bgp watchdog withdraw",
	events.NamespaceCLI + ":request bgp watchdog give",
	events.NamespaceCLI + ":create bgp peer",
	events.NamespaceCLI + ":delete bgp peer",
	events.NamespaceCLI + ":request shutdown",
	events.NamespaceCLI + ":help",
}

// DeclaredEventMessages returns a copy of every bus message name the ExaBGP
// translator renders onto script stdin.
func DeclaredEventMessages() []string {
	return slices.Clone(declaredEventMessages)
}

// MapAPIKeyToBusMessage maps an ExaBGP neighbor `api` key (such as
// "neighbor-changes" or "receive-update") to its Ze bus message(s) and
// optional direction tag value. Unsupported keys (including operational and
// form-modifier keys) return nil, false.
func MapAPIKeyToBusMessage(apiKey string) ([]BusMessageMapping, bool) {
	mapped, ok := apiKeyBusMappings[apiKey]
	if !ok {
		return nil, false
	}
	return slices.Clone(mapped), true
}

// IsFormAPIKey reports whether apiKey is one of the six ExaBGP rendering form
// modifiers (`receive-packets`, `receive-consolidate`, `receive-parsed`,
// `send-packets`, `send-consolidate`, `send-parsed`).
func IsFormAPIKey(apiKey string) bool {
	return formAPIKeys[apiKey]
}

// DeclaredCommandPrefixes returns a copy of every `cli:<prefix>` command path
// the ExaBGP translator can emit and dispatch to Ze.
func DeclaredCommandPrefixes() []string {
	return slices.Clone(declaredCommandPrefixes)
}
