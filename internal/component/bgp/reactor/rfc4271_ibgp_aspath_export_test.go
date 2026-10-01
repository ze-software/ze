// Design: docs/architecture/wire/attributes.md -- AS_PATH toward an internal peer
// Related: filter_ordered.go -- runEgressPolicyChainASN4, the export chain every egress rail runs

package reactor

import (
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// exportFilterTo runs the export chain with an operator filter whose answer
// is the given text delta, toward a destination of the given AS with local AS
// 65000, and answers the AS_PATH that leaves (the body's own path when the
// chain produced no override).
func exportFilterTo(t *testing.T, delta string, body []byte, destPeerAS uint32) []uint32 {
	t.Helper()
	ctxID, err := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))
	require.NoError(t, err)

	r := &Reactor{
		api: &pluginserver.Server{},
		policyFilterSeam: func(_, _, _, _ string, _ uint32, _ string) PolicyResponse {
			return PolicyResponse{Action: PolicyModify, Delta: delta}
		},
		attrModHandlers: attrModHandlersWithDefaults(),
	}
	res := r.runEgressPolicyChainASN4([]filterapi.FilterRef{{Name: "aspath-edit"}},
		"10.0.0.30", destPeerAS, 65000, destPeerAS == 65000, wireu.NewWireUpdate(body, ctxID), true)
	require.True(t, res.accept, "the export chain accepts the route")

	payload := body
	if res.wireOverride != nil {
		payload = res.wireOverride.Payload()
	}
	value, found := attrValueFromPayload(t, payload, attribute.AttrASPath)
	require.True(t, found, "the route leaves with an AS_PATH")
	path, err := attribute.ParseASPath(value, true)
	require.NoError(t, err)
	return flatASNs(path)
}

// privateASPathBody answers an UPDATE body whose four-octet AS_PATH is one
// AS_SEQUENCE [64512 64496]: a private AS (RFC 6996) ahead of a public one.
func privateASPathBody() []byte {
	_, payload := prependFixture([]byte{byte(attribute.ASSequence), 2, 0, 0, 0xFC, 0x00, 0, 0, 0xFB, 0xF0}, nil)
	return payload
}

// TestRFC4271ExportPrependNeverModifiesAnInternalPeersASPath drives the two
// operator inputs that ask Ze to modify the AS_PATH of a route it advertises:
// an export filter answering "as-path-prepend 2", and one answering
// "remove-private strip".
//
// VALIDATES: toward an internal peer (peer AS = local AS) the route leaves with
// its AS_PATH unchanged, [64496] and [64512 64496]; the same filters toward an
// external peer do modify it, [65000 65000 64496] and [64496], so each filter
// really fired.
// PREVENTS: an export policy breaking the IBGP AS_PATH invariant, which every
// internal peer relies on for loop detection and the path-length step.
//
// RFC requirement: RFC4271-5.1.2-2 negative -- an export as-path-prepend or
// remove-private toward an internal peer is not applied:
// runEgressPolicyChainASN4 leaves the AS_PATH unchanged, while the
// external-peer controls are prepended and stripped.
func TestRFC4271ExportPrependNeverModifiesAnInternalPeersASPath(t *testing.T) {
	assert.Equal(t, []uint32{64496}, exportFilterTo(t, "as-path-prepend 2", asPathBodyAtWidth(true), 65000),
		"internal peer: the prepend leaves the AS_PATH unmodified")
	assert.Equal(t, []uint32{65000, 65000, 64496}, exportFilterTo(t, "as-path-prepend 2", asPathBodyAtWidth(true), 65001),
		"external peer: the same filter prepends the local AS twice")
	assert.Equal(t, []uint32{64512, 64496}, exportFilterTo(t, "remove-private strip", privateASPathBody(), 65000),
		"internal peer: remove-private leaves the AS_PATH unmodified")
	assert.Equal(t, []uint32{64496}, exportFilterTo(t, "remove-private strip", privateASPathBody(), 65001),
		"external peer: the same filter strips the private AS")
}

// migrationSettings answers the settings of a session toward peer AS 64999
// with local AS 65000. With migration set, 64999 is also configured as this
// router's second AS (RFC 7705 Section 4.2), which makes the session internal;
// without it, the same peer AS is an external session.
func migrationSettings(addr string, migration bool) *PeerSettings {
	settings := NewPeerSettings(mustParseAddr(addr), 65000, 64999, 0x01010101)
	if migration {
		settings.MigrationAS = 64999
	}
	return settings
}

// exportThroughSession runs an operator export filter whose answer is delta
// through exportFilterForBody, over the forwarding facts the peer builds from
// its own settings, and answers the AS_PATH that leaves.
func exportThroughSession(t *testing.T, settings *PeerSettings, delta string, body []byte) []uint32 {
	t.Helper()
	ctxID, err := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))
	require.NoError(t, err)

	settings.ExportFilters = []filterapi.FilterRef{{Name: "aspath-edit"}}
	peer := NewPeer(settings)
	facts := peer.buildForwardFacts()
	facts.sendCtxID = ctxID
	facts.sendASN4 = true
	peer.fwdFacts.Store(facts)

	r := &Reactor{
		api: &pluginserver.Server{},
		policyFilterSeam: func(_, _, _, _ string, _ uint32, _ string) PolicyResponse {
			return PolicyResponse{Action: PolicyModify, Delta: delta}
		},
		attrModHandlers: attrModHandlersWithDefaults(),
	}
	suppress, override := r.exportFilterForBody(peer, body)
	require.False(t, suppress, "the export chain accepts the route")

	payload := body
	if override != nil {
		payload = override
	}
	value, found := attrValueFromPayload(t, payload, attribute.AttrASPath)
	require.True(t, found, "the route leaves with an AS_PATH")
	path, err := attribute.ParseASPath(value, true)
	require.NoError(t, err)
	return flatASNs(path)
}

// TestRFC4271ExportASPathEditsFollowTheMigrationAwareInternalVerdict drives
// the export chain through exportFilterForBody for a session whose peer AS is
// not the local AS, but is the second AS of a migrating router, so RFC 7705
// Section 4.2 makes it internal: "the BGP speaker MUST treat UPDATEs sent and
// received to this peer as if this was a natively configured iBGP session".
// The verdict comes from the peer's own forwarding facts, built from its
// settings, so the test reaches the guard the way a live session does.
//
// VALIDATES: toward the migration-internal peer an export as-path-prepend and
// remove-private leave the AS_PATH unchanged, [64496] and [64512 64496]; the
// same peer AS without the migration AS is external and gets both edits,
// [65000 65000 64496] and [64496].
// PREVENTS: an equality test (peer AS = local AS) standing in for the session's
// internal verdict, which modified the AS_PATH sent to a migration iBGP peer.
//
// RFC requirement: RFC4271-5.1.2-2 negative -- an export as-path-prepend or
// remove-private toward a peer that is internal by RFC 7705 Section 4.2 (peer
// AS = the configured migration AS) is not applied by exportFilterForBody,
// while the same peer AS with no migration AS is prepended and stripped.
func TestRFC4271ExportASPathEditsFollowTheMigrationAwareInternalVerdict(t *testing.T) {
	assert.Equal(t, []uint32{64496},
		exportThroughSession(t, migrationSettings("10.0.0.31", true), "as-path-prepend 2", asPathBodyAtWidth(true)),
		"migration-internal peer: the prepend leaves the AS_PATH unmodified")
	assert.Equal(t, []uint32{65000, 65000, 64496},
		exportThroughSession(t, migrationSettings("10.0.0.32", false), "as-path-prepend 2", asPathBodyAtWidth(true)),
		"external peer with the same AS: the filter prepends the local AS twice")
	assert.Equal(t, []uint32{64512, 64496},
		exportThroughSession(t, migrationSettings("10.0.0.33", true), "remove-private strip", privateASPathBody()),
		"migration-internal peer: remove-private leaves the AS_PATH unmodified")
	assert.Equal(t, []uint32{64496},
		exportThroughSession(t, migrationSettings("10.0.0.34", false), "remove-private strip", privateASPathBody()),
		"external peer with the same AS: the filter strips the private AS")
}

// dryRunWireChanges runs `policy test peer` through PolicyDryRun for a peer
// built from settings, with one filter whose answer is "as-path-prepend 2",
// and answers the wire changes the operator is shown.
func dryRunWireChanges(t *testing.T, settings *PeerSettings, direction string) []string {
	t.Helper()
	r := New(&Config{})
	r.api = &pluginserver.Server{}
	r.policyFilterSeam = func(_, _, _, _ string, _ uint32, _ string) PolicyResponse {
		return PolicyResponse{Action: PolicyModify, Delta: "as-path-prepend 2"}
	}
	settings.ImportFilters = []filterapi.FilterRef{{Name: "aspath-edit"}}
	settings.ExportFilters = []filterapi.FilterRef{{Name: "aspath-edit"}}
	peer := NewPeer(settings)
	peer.state.Store(int32(PeerStateEstablished))
	r.peers[settings.PeerKey()] = peer

	result, err := (&reactorAPIAdapter{r: r}).PolicyDryRun(
		settings.Address.String(), direction, "", dryRunUpdateBody(t), true)
	require.NoError(t, err)
	return result.WireChanges
}

// TestRFC4271PolicyDryRunAgreesNoASPathEditTowardAnInternalPeer asks the
// dry-run the question the runtime answers in
// TestRFC4271ExportASPathEditsFollowTheMigrationAwareInternalVerdict: an
// export as-path-prepend toward an internal peer is not applied, so the
// operator must not be told it is.
//
// VALIDATES: an export dry-run toward an ordinary internal peer and toward a
// migration-internal peer reports no "AS_PATH prepend"; toward the external
// peer of the same AS it does; an import dry-run from the internal peer still
// does, because Section 5.1.2 binds advertising and the import chain applies
// the prepend.
// PREVENTS: `policy test peer` promising an AS_PATH edit the runtime refuses.
//
// RFC requirement: RFC4271-5.1.2-2 negative -- PolicyDryRun reports no export
// AS_PATH prepend toward an internal peer (peer AS = local AS, or the RFC 7705
// migration AS), and reports it toward the external control.
func TestRFC4271PolicyDryRunAgreesNoASPathEditTowardAnInternalPeer(t *testing.T) {
	ordinary := NewPeerSettings(mustParseAddr("192.0.2.41"), 65000, 65000, 0x01010101)
	assert.NotContains(t, dryRunWireChanges(t, ordinary, directionExport), "AS_PATH prepend",
		"ordinary internal peer: no export prepend is reported")
	assert.NotContains(t, dryRunWireChanges(t, migrationSettings("192.0.2.42", true), directionExport), "AS_PATH prepend",
		"migration-internal peer: no export prepend is reported")
	assert.Contains(t, dryRunWireChanges(t, migrationSettings("192.0.2.43", false), directionExport), "AS_PATH prepend",
		"external peer with the same AS: the export prepend is reported")
	internalImport := NewPeerSettings(mustParseAddr("192.0.2.44"), 65000, 65000, 0x01010101)
	assert.Contains(t, dryRunWireChanges(t, internalImport, directionImport), "AS_PATH prepend",
		"import from an internal peer: the import chain applies the prepend")
}
