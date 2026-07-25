package flowspecfirewall

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/firewall"
	"github.com/ze-software/ze/internal/core/slogutil"
)

func testBridge() *bridge {
	return newBridge(slogutil.DiscardLogger())
}

func TestHandleUpdate(t *testing.T) {
	b := testBridge()

	event := `{
		"type": "update",
		"peer": {"address": "10.0.0.1"},
		"extended-communities": ["rate-limit:0"],
		"ipv4/flow": [{
			"action": "add",
			"nlri": {
				"destination": [["10.1.0.0/24"]],
				"protocol": [["=6"]],
				"destination-port": [["=80"]]
			}
		}]
	}`

	b.handleUpdate(event, "10.0.0.1")

	tables := b.rules.buildTable()
	require.NotNil(t, tables)
	require.Len(t, tables, 1)
	assert.Equal(t, "flowspec", tables[0].Name)
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

	add := `{
		"type": "update",
		"peer": {"address": "10.0.0.1"},
		"extended-communities": ["rate-limit:0"],
		"ipv4/flow": [{
			"action": "add",
			"nlri": {"destination": [["10.1.0.0/24"]]}
		}]
	}`
	b.handleUpdate(add, "10.0.0.1")
	require.NotNil(t, b.rules.buildTable())

	withdraw := `{
		"type": "update",
		"peer": {"address": "10.0.0.1"},
		"ipv4/flow": [{
			"action": "del",
			"nlri": {"destination": [["10.1.0.0/24"]]}
		}]
	}`
	b.handleUpdate(withdraw, "10.0.0.1")
	assert.Nil(t, b.rules.buildTable())
}

func TestHandlePeerDown(t *testing.T) {
	b := testBridge()

	event := `{
		"type": "update",
		"peer": {"address": "10.0.0.1"},
		"extended-communities": ["rate-limit:0"],
		"ipv4/flow": [{
			"action": "add",
			"nlri": {"destination": [["10.1.0.0/24"]]}
		}]
	}`
	b.handleUpdate(event, "10.0.0.1")
	require.NotNil(t, b.rules.buildTable())

	b.handlePeerDown("10.0.0.1")
	assert.Nil(t, b.rules.buildTable())
}

func TestHandleEventDispatch(t *testing.T) {
	b := testBridge()

	// State event: peer down
	stateDown := `{"type":"state","peer":{"address":"10.0.0.1"},"state":"down"}`
	err := b.handleEvent(stateDown)
	assert.NoError(t, err)

	// State event: peer up (no-op)
	stateUp := `{"type":"state","peer":{"address":"10.0.0.1"},"state":"up"}`
	err = b.handleEvent(stateUp)
	assert.NoError(t, err)

	// Malformed JSON
	err = b.handleEvent(`not json`)
	assert.NoError(t, err)
}

func TestHandleUpdateLocalDest(t *testing.T) {
	b := testBridge()
	b.addrs.add(netip.MustParseAddr("10.1.0.5"))

	event := `{
		"type": "update",
		"peer": {"address": "10.0.0.1"},
		"extended-communities": ["rate-limit:0"],
		"ipv4/flow": [{
			"action": "add",
			"nlri": {"destination": [["10.1.0.0/24"]]}
		}]
	}`
	b.handleUpdate(event, "10.0.0.1")

	tables := b.rules.buildTable()
	require.NotNil(t, tables)
	require.Len(t, tables[0].Chains, 1)
	assert.Equal(t, "flowspec-in", tables[0].Chains[0].Name)
	assert.Equal(t, firewall.HookInput, tables[0].Chains[0].Hook)
}

func TestHandleUpdateNoAction(t *testing.T) {
	b := testBridge()

	event := `{
		"type": "update",
		"peer": {"address": "10.0.0.1"},
		"extended-communities": ["target:65000:100"],
		"ipv4/flow": [{
			"action": "add",
			"nlri": {"destination": [["10.1.0.0/24"]]}
		}]
	}`
	b.handleUpdate(event, "10.0.0.1")

	assert.Nil(t, b.rules.buildTable())
}

func TestHandleUpdateMultiplePeers(t *testing.T) {
	b := testBridge()

	event1 := `{
		"type": "update",
		"peer": {"address": "10.0.0.1"},
		"extended-communities": ["rate-limit:0"],
		"ipv4/flow": [{
			"action": "add",
			"nlri": {"destination": [["10.1.0.0/24"]]}
		}]
	}`
	event2 := `{
		"type": "update",
		"peer": {"address": "10.0.0.2"},
		"extended-communities": ["rate-limit:0"],
		"ipv4/flow": [{
			"action": "add",
			"nlri": {"destination": [["10.2.0.0/24"]]}
		}]
	}`
	b.handleUpdate(event1, "10.0.0.1")
	b.handleUpdate(event2, "10.0.0.2")

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

	event := `{
		"type": "update",
		"peer": {"address": "10.0.0.1"},
		"extended-communities": ["rate-limit:8000"],
		"ipv4/flow": [{
			"action": "add",
			"nlri": {"destination": [["10.1.0.0/24"]]}
		}]
	}`
	b.handleUpdate(event, "10.0.0.1")

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

	event := `{
		"type": "update",
		"peer": {"address": "10.0.0.1"},
		"extended-communities": ["mark:46"],
		"ipv4/flow": [{
			"action": "add",
			"nlri": {"destination": [["10.1.0.0/24"]]}
		}]
	}`
	b.handleUpdate(event, "10.0.0.1")

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

	event1 := `{
		"type": "update",
		"peer": {"address": "10.0.0.1"},
		"extended-communities": ["rate-limit:0"],
		"ipv4/flow": [{"action": "add", "nlri": {"destination": [["10.1.0.0/24"]]}}]
	}`
	event2 := `{
		"type": "update",
		"peer": {"address": "10.0.0.1"},
		"extended-communities": ["rate-limit:0"],
		"ipv4/flow": [{"action": "add", "nlri": {"destination": [["10.2.0.0/24"]]}}]
	}`
	b.handleUpdate(event1, "10.0.0.1")
	b.handleUpdate(event2, "10.0.0.1")

	tables := b.rules.buildTable()
	require.NotNil(t, tables)
	assert.Len(t, tables[0].Chains[0].Terms, 1, "second rule should be rejected by max-rules cap")
}

func TestHandleUpdateUnsupportedComponent(t *testing.T) {
	b := testBridge()

	event := `{
		"type": "update",
		"peer": {"address": "10.0.0.1"},
		"extended-communities": ["rate-limit:0"],
		"ipv4/flow": [{
			"action": "add",
			"nlri": {"destination": [["10.1.0.0/24"]], "packet-length": [["=128"]]}
		}]
	}`
	b.handleUpdate(event, "10.0.0.1")

	assert.Nil(t, b.rules.buildTable(), "rule with unsupported component should not be installed")
}

func TestHandleUpdateEmptyPeer(t *testing.T) {
	b := testBridge()

	event := `{
		"type": "update",
		"extended-communities": ["rate-limit:0"],
		"ipv4/flow": [{"action": "add", "nlri": {"destination": [["10.1.0.0/24"]]}}]
	}`
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
	withoutNextHop := `{
		"type": "update",
		"peer": {"address": "10.0.0.1"},
		"extended-communities": ["rate-limit:0"],
		"ipv4/flow": [{
			"action": "add",
			"nlri": {
				"destination": [["10.1.0.0/24"]],
				"protocol": [["=6"]],
				"destination-port": [["=80"]]
			}
		}]
	}`
	// Same UPDATE carrying a next-hop, which RFC 8955 Section 4 says MUST be ignored.
	withNextHop := `{
		"type": "update",
		"peer": {"address": "10.0.0.1"},
		"extended-communities": ["rate-limit:0"],
		"ipv4/flow": [{
			"action": "add",
			"next-hop": "192.0.2.99",
			"nlri": {
				"destination": [["10.1.0.0/24"]],
				"protocol": [["=6"]],
				"destination-port": [["=80"]]
			}
		}]
	}`

	plain := testBridge()
	plain.handleUpdate(withoutNextHop, "10.0.0.1")
	plainTables := plain.rules.buildTable()
	require.Len(t, plainTables, 1)
	require.Len(t, plainTables[0].Chains, 1)
	require.Len(t, plainTables[0].Chains[0].Terms, 1)

	nh := testBridge()
	nh.handleUpdate(withNextHop, "10.0.0.1")
	nhTables := nh.rules.buildTable()

	// RFC 8955 Section 4: "the Network Address of Next-Hop field MUST be ignored."
	// RFC requirement: RFC8955-4-4 positive -- a FlowSpec UPDATE carrying a next-hop lowers to byte-identical firewall rules (§4)
	assert.Equal(t, plainTables, nhTables,
		"the next-hop must not change the firewall rules a FlowSpec route lowers to")
}
