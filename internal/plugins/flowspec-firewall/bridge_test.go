package flowspecfirewall

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/firewall"
	"github.com/ze-software/ze/internal/core/slogutil"
)

func TestHandleUpdate(t *testing.T) {
	b := testBridge()

	event := daemonAddJSON("10.0.0.1", "rate-limit:0", `{
		"destination-ipv4": [["10.1.0.0/24"]],
		"protocol": [["=6"]],
		"destination-port": [["=80"]]
	}`)

	require.NoError(t, b.handleEvent(event))

	tables := b.rules.buildTable()
	require.NotNil(t, tables)
	require.Len(t, tables, 1)
	assert.Equal(t, "ze_flowspec", tables[0].Name)
	require.Len(t, tables[0].Chains, 1)
	assert.Equal(t, "flowspec-fwd", tables[0].Chains[0].Name)
	require.Len(t, tables[0].Chains[0].Terms, 1)

	term := tables[0].Chains[0].Terms[0]
	assert.NotEmpty(t, term.Name)
	require.Len(t, term.Actions, 1)
	assert.Equal(t, firewall.Drop{}, term.Actions[0])
}

func TestHandleUpdateWithdraw(t *testing.T) {
	b := testBridge()

	add := daemonAddJSON("10.0.0.1", "rate-limit:0", `{"destination-ipv4": [["10.1.0.0/24"]]}`)
	require.NoError(t, b.handleEvent(add))
	require.NotNil(t, b.rules.buildTable())

	withdraw := daemonUpdateJSON("10.0.0.1", nil,
		daemonOp{action: "del", nlri: []string{`{"destination-ipv4": [["10.1.0.0/24"]]}`}})
	require.NoError(t, b.handleEvent(withdraw))
	assert.Nil(t, b.rules.buildTable())
}

func TestHandlePeerDown(t *testing.T) {
	b := testBridge()

	event := daemonAddJSON("10.0.0.1", "rate-limit:0", `{"destination-ipv4": [["10.1.0.0/24"]]}`)
	require.NoError(t, b.handleEvent(event))
	require.NotNil(t, b.rules.buildTable())

	b.handlePeerDown("10.0.0.1")
	assert.Nil(t, b.rules.buildTable())
}

func TestHandleEventDispatch(t *testing.T) {
	b := testBridge()

	// State event: peer down
	err := b.handleEvent(daemonStateJSON("10.0.0.1", "down"))
	assert.NoError(t, err)

	// State event: peer up (no-op)
	err = b.handleEvent(daemonStateJSON("10.0.0.1", "up"))
	assert.NoError(t, err)

	// Malformed JSON
	err = b.handleEvent(`not json`)
	assert.NoError(t, err)

	// An event kind this bridge does not handle. It is dropped, and the
	// default branch in handleEvent is what records the drop rather than
	// letting it vanish the way every UPDATE used to.
	err = b.handleEvent(`{"type":"bgp","bgp":{"message":{"type":"keepalive"},` +
		`"peer":{"remote":{"address":"10.0.0.1","as":65001}}}}`)
	assert.NoError(t, err)
	assert.Nil(t, b.rules.buildTable())
}

// TestStateEventDropsRulesOnlyWhenThePeerIsNotUp pins both answers the state
// branch of handleEvent gives, which TestHandleEventDispatch only checks the
// error return of.
//
// VALIDATES: a down state removes the rules the peer's routes installed, an up
// state leaves them alone, and the removal reaches that peer's rules only.
// PREVENTS: an established session losing the rules it just installed, which is
// the failure that a state branch reading only "is this event a state change"
// would produce; and a down event clearing the whole table, which a second peer
// announcing its own rule is what detects.
func TestStateEventDropsRulesOnlyWhenThePeerIsNotUp(t *testing.T) {
	b := testBridge()

	require.NoError(t, b.handleEvent(
		daemonAddJSON("10.0.0.1", "rate-limit:0", `{"destination-ipv4": [["10.1.0.0/24"]]}`)))
	require.NoError(t, b.handleEvent(
		daemonAddJSON("10.0.0.2", "rate-limit:0", `{"destination-ipv4": [["10.2.0.0/24"]]}`)))
	require.Len(t, b.rules.buildTable()[0].Chains[0].Terms, 2)

	require.NoError(t, b.handleEvent(daemonStateJSON("10.0.0.1", "up")))
	require.Len(t, b.rules.buildTable()[0].Chains[0].Terms, 2,
		"a peer that is up keeps the rules it announced")

	// Removal is scoped to the departing peer. A state branch that dropped every
	// rule would satisfy the final assertion below on its own, so the surviving
	// second peer is what makes this test able to fail.
	require.NoError(t, b.handleEvent(daemonStateJSON("10.0.0.1", "down")))
	tables := b.rules.buildTable()
	require.NotNil(t, tables, "the peer that stayed up keeps its rules")
	assert.Len(t, tables[0].Chains[0].Terms, 1, "only the departing peer's rule is removed")

	require.NoError(t, b.handleEvent(daemonStateJSON("10.0.0.2", "down")))
	assert.Nil(t, b.rules.buildTable(), "a peer that went down keeps no rules")
}

func TestHandleUpdateLocalDest(t *testing.T) {
	b := testBridge()
	b.addrs.add(netip.MustParseAddr("10.1.0.5"))

	event := daemonAddJSON("10.0.0.1", "rate-limit:0", `{"destination-ipv4": [["10.1.0.0/24"]]}`)
	require.NoError(t, b.handleEvent(event))

	tables := b.rules.buildTable()
	require.NotNil(t, tables)
	require.Len(t, tables[0].Chains, 1)
	assert.Equal(t, "flowspec-in", tables[0].Chains[0].Name)
	assert.Equal(t, firewall.HookInput, tables[0].Chains[0].Hook)
}

func TestHandleUpdateNoAction(t *testing.T) {
	b := testBridge()

	event := daemonAddJSON("10.0.0.1", "target:65000:100", `{"destination-ipv4": [["10.1.0.0/24"]]}`)
	require.NoError(t, b.handleEvent(event))

	assert.Nil(t, b.rules.buildTable())
}

func TestHandleUpdateMultiplePeers(t *testing.T) {
	b := testBridge()

	event1 := daemonAddJSON("10.0.0.1", "rate-limit:0", `{"destination-ipv4": [["10.1.0.0/24"]]}`)
	event2 := daemonAddJSON("10.0.0.2", "rate-limit:0", `{"destination-ipv4": [["10.2.0.0/24"]]}`)
	require.NoError(t, b.handleEvent(event1))
	require.NoError(t, b.handleEvent(event2))

	tables := b.rules.buildTable()
	require.NotNil(t, tables)
	assert.Len(t, tables[0].Chains[0].Terms, 2)

	// Withdraw from peer1 only
	b.handlePeerDown("10.0.0.1")
	tables = b.rules.buildTable()
	require.NotNil(t, tables)
	assert.Len(t, tables[0].Chains[0].Terms, 1)
}

func TestHandleUpdateRateLimit(t *testing.T) {
	b := testBridge()

	event := daemonAddJSON("10.0.0.1", "rate-limit:8000", `{"destination-ipv4": [["10.1.0.0/24"]]}`)
	require.NoError(t, b.handleEvent(event))

	tables := b.rules.buildTable()
	require.NotNil(t, tables)
	term := tables[0].Chains[0].Terms[0]
	require.Len(t, term.Actions, 2)
	limit, ok := term.Actions[0].(firewall.Limit)
	require.True(t, ok)
	assert.Equal(t, uint64(8000), limit.Rate)
	assert.Equal(t, firewall.Accept{}, term.Actions[1])
}

func TestHandleUpdateDSCPMark(t *testing.T) {
	b := testBridge()

	event := daemonAddJSON("10.0.0.1", "mark:46", `{"destination-ipv4": [["10.1.0.0/24"]]}`)
	require.NoError(t, b.handleEvent(event))

	tables := b.rules.buildTable()
	require.NotNil(t, tables)
	term := tables[0].Chains[0].Terms[0]
	require.Len(t, term.Actions, 2)
	assert.Equal(t, firewall.SetDSCP{Value: 46}, term.Actions[0])
	assert.Equal(t, firewall.Accept{}, term.Actions[1])
}

func TestHandleUpdateMaxRules(t *testing.T) {
	b := &bridge{
		rules: newRuleMap(1),
		addrs: newLocalAddrs(),
		log:   slogutil.DiscardLogger(),
	}

	event1 := daemonAddJSON("10.0.0.1", "rate-limit:0", `{"destination-ipv4": [["10.1.0.0/24"]]}`)
	event2 := daemonAddJSON("10.0.0.1", "rate-limit:0", `{"destination-ipv4": [["10.2.0.0/24"]]}`)
	require.NoError(t, b.handleEvent(event1))
	require.NoError(t, b.handleEvent(event2))

	tables := b.rules.buildTable()
	require.NotNil(t, tables)
	assert.Len(t, tables[0].Chains[0].Terms, 1, "second rule should be rejected by max-rules cap")
}

func TestHandleUpdateUnsupportedComponent(t *testing.T) {
	b := testBridge()

	event := daemonAddJSON("10.0.0.1", "rate-limit:0",
		`{"destination-ipv4": [["10.1.0.0/24"]], "packet-length": [["=128"]]}`)
	require.NoError(t, b.handleEvent(event))

	assert.Nil(t, b.rules.buildTable(), "rule with unsupported component should not be installed")
}

func TestHandleUpdateEmptyPeer(t *testing.T) {
	b := testBridge()

	event := daemonAddJSON("", "rate-limit:0", `{"destination-ipv4": [["10.1.0.0/24"]]}`)
	err := b.handleEvent(event)
	assert.NoError(t, err)
	assert.Nil(t, b.rules.buildTable(), "events without peer address should be ignored")
}

// TestRFC8955NextHopIgnoredForFlowSpec verifies the next-hop carried with a received
// FlowSpec route has no effect on the firewall rules it lowers to.
//
// VALIDATES: handleUpdate (engine.go) reads only the peer address, the extended
// communities and the per-family NLRI, and translateFlowSpec (translate.go) builds its
// Terms from the NLRI components plus the parsed action -- no code path in this package
// reads a next-hop, so the field is ignored exactly as RFC 8955 Section 4 requires.
//
// PREVENTS: a next-hop leaking into the FlowSpec lowering path (rule identity, term name
// or match set), which would make an ignored field change forwarding behavior.
func TestRFC8955NextHopIgnoredForFlowSpec(t *testing.T) {
	// The fixture uses the real daemon envelope. The literals it replaces used a flat
	// envelope no writer in the tree produces, so this MUST was proven against bytes
	// the daemon never sends. The named form "rate-limit:0" is what the event
	// carries.
	nlri := `{"destination-ipv4": [["10.1.0.0/24"]],"protocol": [["=6"]],"destination-port": [["=80"]]}`
	withoutNextHop := daemonUpdateJSON("10.0.0.1", []string{"rate-limit:0"},
		daemonOp{action: "add", nlri: []string{nlri}})
	// Same UPDATE carrying a next-hop, which RFC 8955 Section 4 says MUST be ignored.
	withNextHop := daemonUpdateJSON("10.0.0.1", []string{"rate-limit:0"},
		daemonOp{action: "add", nextHop: "192.0.2.99", nlri: []string{nlri}})

	plain := testBridge()
	require.NoError(t, plain.handleEvent(withoutNextHop))
	plainTables := plain.rules.buildTable()
	require.Len(t, plainTables, 1)
	require.Len(t, plainTables[0].Chains, 1)
	require.Len(t, plainTables[0].Chains[0].Terms, 1)

	nh := testBridge()
	require.NoError(t, nh.handleEvent(withNextHop))
	nhTables := nh.rules.buildTable()

	// RFC 8955 Section 4: "the Network Address of Next-Hop field MUST be ignored."
	// RFC requirement: RFC8955-4-4 positive -- a FlowSpec UPDATE carrying a next-hop lowers to byte-identical firewall rules (§4)
	assert.Equal(t, plainTables, nhTables,
		"the next-hop must not change the firewall rules a FlowSpec route lowers to")
}
