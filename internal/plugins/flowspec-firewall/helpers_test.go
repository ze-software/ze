package flowspecfirewall

import (
	"strings"

	"github.com/ze-software/ze/internal/core/slogutil"
)

// The envelope helpers this package's tests build their events from.
//
// The bridge reads the ze-bgp envelope: the event kind in message.type, the
// peer beside it, and the path attributes and per-family NLRI under "update"
// (internal/component/bgp/format, appendFilterResultJSON,
// appendParsedUpdateJSONDirect and appendStateChangeJSON).
// TestDaemonFlowSpecUpdateReachesTheRuleTable (daemon_event_test.go) drives
// those writers and pins the shape below against them; the helpers exist so
// the behavior tests can vary one thing at a time -- the traffic action, the
// peer, the rule cap -- without a wire fixture for each.

func testBridge() *bridge {
	return newBridge(slogutil.DiscardLogger())
}

// daemonOp is one operation under a family key of the event's nlri object: an
// action, the next-hop RFC 8955 Section 4 says a FlowSpec receiver ignores,
// and the NLRI objects the FlowSpec writer emits.
type daemonOp struct {
	action  string
	nextHop string
	nlri    []string
}

// daemonUpdateJSON writes the ze-bgp UPDATE envelope for one peer, one set of
// extended communities, and any number of ipv4/flow operations.
func daemonUpdateJSON(peer string, extComms []string, ops ...daemonOp) string {
	var b strings.Builder
	b.WriteString(`{"type":"bgp","bgp":{"message":{"type":"update","id":1,"direction":"received"},`)
	b.WriteString(`"peer":{"local":{"address":"10.0.0.254","as":65001},"name":"peer1",`)
	b.WriteString(`"remote":{"address":"` + peer + `","as":65001}},"update":{"attr":{`)
	if len(extComms) > 0 {
		b.WriteString(`"extended-communities":["` + strings.Join(extComms, `","`) + `"]`)
	}
	b.WriteString(`},"nlri":{"ipv4/flow":[`)
	for i, op := range ops {
		if i > 0 {
			b.WriteString(`,`)
		}
		b.WriteString(`{`)
		if op.nextHop != "" {
			b.WriteString(`"next-hop":"` + op.nextHop + `",`)
		}
		b.WriteString(`"action":"` + op.action + `","nlri":[` + strings.Join(op.nlri, ",") + `]}`)
	}
	// Closes, in order: the family array, the nlri object, the update object,
	// the bgp object, and the outer envelope.
	b.WriteString(`]}}}}`)
	return b.String()
}

// daemonAddJSON is the common case: one peer, one traffic action, one added
// FlowSpec NLRI.
func daemonAddJSON(peer, extComm, nlri string) string {
	return daemonUpdateJSON(peer, []string{extComm}, daemonOp{action: "add", nlri: []string{nlri}})
}

// daemonStateJSON writes the ze-bgp state-change envelope.
func daemonStateJSON(peer, state string) string {
	return `{"type":"bgp","bgp":{"message":{"type":"state"},` +
		`"peer":{"local":{"address":"10.0.0.254","as":65001},"name":"peer1",` +
		`"remote":{"address":"` + peer + `","as":65001}},"state":"` + state + `"}}`
}
