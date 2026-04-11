// Design: docs/architecture/plugin/rib-storage-design.md -- tests for the
// cmd-9 best-path reason terminal driven through bestPipeline.

package rib

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/rib/storage"
	"codeberg.org/thomas-mangin/ze/internal/core/family"
)

// testWireLocalPref200 carries a LOCAL_PREF = 200 value.
var testWireLocalPref200 = []byte{0x40, 0x05, 0x04, 0x00, 0x00, 0x00, 0xC8}

// TestBestPipelineReason_LocalPrefWinner drives the full bestPipeline path
// for `bgp rib show best reason` with two candidates for the same prefix
// on different peers. The higher-LOCAL_PREF peer must win and the JSON
// must name the deciding step.
//
// VALIDATES: cmd-9 reason terminal wiring -- parser accepts "reason",
// bestPipeline invokes bestReasonTerminal, and the output JSON has the
// expected "best-path-reason" shape with the deciding step named.
// PREVENTS: Regression where the "reason" keyword is rejected as unknown
// or silently falls back to the default JSON terminal.
func TestBestPipelineReason_LocalPrefWinner(t *testing.T) {
	r := newTestRIBManager(t)

	fam := family.IPv4Unicast
	nlriBytes := []byte{24, 10, 0, 0} // 10.0.0.0/24

	// Peer A: LOCAL_PREF=100 (loser)
	attrA := concatBytes(testWireOriginIGP, testWireASPath65001, testWireNextHop, testWireLocalPref100)
	peerA := storage.NewPeerRIB("192.0.2.1")
	peerA.Insert(fam, attrA, nlriBytes)
	r.ribInPool["192.0.2.1"] = peerA

	// Peer B: LOCAL_PREF=200 (winner)
	attrB := concatBytes(testWireOriginIGP, testWireASPath65001, testWireNextHop, testWireLocalPref200)
	peerB := storage.NewPeerRIB("192.0.2.2")
	peerB.Insert(fam, attrB, nlriBytes)
	r.ribInPool["192.0.2.2"] = peerB

	result := r.bestPipeline("*", []string{"reason"})

	var parsed map[string]any
	require.NoError(t, json.Unmarshal([]byte(result), &parsed))
	entries, ok := parsed["best-path-reason"].([]any)
	require.True(t, ok, "expected best-path-reason array, got %v", parsed)
	require.Len(t, entries, 1)

	entry, ok := entries[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "ipv4/unicast", entry["family"])
	assert.Equal(t, "10.0.0.0/24", entry["prefix"])
	assert.Equal(t, "192.0.2.2", entry["winner-peer"],
		"higher LOCAL_PREF peer must win")

	// The candidates slice lists every peer in the reduction order.
	cands, ok := entry["candidates"].([]any)
	require.True(t, ok)
	assert.Len(t, cands, 2)

	// Exactly one step, and it must be local-preference.
	steps, ok := entry["steps"].([]any)
	require.True(t, ok)
	require.Len(t, steps, 1)
	step, ok := steps[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "local-preference", step["step"])
	assert.Equal(t, "192.0.2.2", step["winner"])
	reason, ok := step["reason"].(string)
	require.True(t, ok, "reason field must be a string")
	assert.Contains(t, reason, "200")
	assert.Contains(t, reason, "100")
}

// TestBestPipelineReason_SingleCandidate verifies a prefix with only one
// candidate still emits a valid explanation entry with an empty step list.
//
// VALIDATES: cmd-9 reason terminal for degenerate single-peer prefixes.
// PREVENTS: Empty JSON or missing entries when no comparisons were needed.
func TestBestPipelineReason_SingleCandidate(t *testing.T) {
	r := newTestRIBManager(t)

	fam := family.IPv4Unicast
	nlriBytes := []byte{24, 10, 0, 0}
	attrA := concatBytes(testWireOriginIGP, testWireASPath65001, testWireNextHop, testWireLocalPref100)
	peerA := storage.NewPeerRIB("192.0.2.1")
	peerA.Insert(fam, attrA, nlriBytes)
	r.ribInPool["192.0.2.1"] = peerA

	result := r.bestPipeline("*", []string{"reason"})

	var parsed map[string]any
	require.NoError(t, json.Unmarshal([]byte(result), &parsed))
	entries, ok := parsed["best-path-reason"].([]any)
	require.True(t, ok)
	require.Len(t, entries, 1)

	entry, ok := entries[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "192.0.2.1", entry["winner-peer"])
	steps, ok := entry["steps"].([]any)
	require.True(t, ok)
	assert.Empty(t, steps, "no comparisons needed for a single candidate")
}

// TestBestPipelineReason_WithPrefixFilter verifies that filter stages
// between the bestSource and the reason terminal still apply -- only
// matching prefixes get an explanation.
//
// VALIDATES: cmd-9 reason terminal composes with existing filters.
// PREVENTS: Filter stages being silently ignored in the reason path.
func TestBestPipelineReason_WithPrefixFilter(t *testing.T) {
	r := newTestRIBManager(t)
	fam := family.IPv4Unicast

	// Insert two prefixes; we'll filter for only one.
	nlri1 := []byte{24, 10, 0, 0} // 10.0.0.0/24
	nlri2 := []byte{24, 10, 0, 1} // 10.0.1.0/24
	attr := concatBytes(testWireOriginIGP, testWireASPath65001, testWireNextHop, testWireLocalPref100)

	peer := storage.NewPeerRIB("192.0.2.1")
	peer.Insert(fam, attr, nlri1)
	peer.Insert(fam, attr, nlri2)
	r.ribInPool["192.0.2.1"] = peer

	result := r.bestPipeline("*", []string{"cidr", "10.0.0.0/24", "reason"})

	var parsed map[string]any
	require.NoError(t, json.Unmarshal([]byte(result), &parsed))
	entries, ok := parsed["best-path-reason"].([]any)
	require.True(t, ok)
	require.Len(t, entries, 1, "cidr filter should leave exactly one prefix")

	entry, ok := entries[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "10.0.0.0/24", entry["prefix"])
}

// TestBestPipelineReason_UnknownKeyword verifies that the parser rejects
// a typo like "reasons" instead of silently falling through.
//
// VALIDATES: cmd-9 reason terminal CLI hygiene -- typos surface as errors.
// PREVENTS: Misspelled "reasons" being treated as the default json terminal.
func TestBestPipelineReason_UnknownKeyword(t *testing.T) {
	r := newTestRIBManager(t)
	result := r.bestPipeline("*", []string{"reasons"})

	var parsed map[string]any
	require.NoError(t, json.Unmarshal([]byte(result), &parsed))
	errStr, ok := parsed["error"].(string)
	require.True(t, ok, "expected error key for unknown keyword")
	assert.Contains(t, errStr, "unknown keyword")
	assert.Contains(t, errStr, "reasons")
}

// TestBestPipeline_MultipathPeersInOutput verifies that the best-path JSON
// terminal includes MultipathPeers in its output when two peers advertise
// the same prefix with byte-identical AS_PATH and bgp/multipath/maximum-paths
// is > 1.
//
// VALIDATES: cmd-3 phase 3 wiring -- SelectMultipath is invoked by the
// bestSource and the sibling peer lands in the JSON "multipath-peers" field.
// PREVENTS: The ECMP set being silently dropped on the CLI query path even
// when the algorithm is correct.
func TestBestPipeline_MultipathPeersInOutput(t *testing.T) {
	r := newTestRIBManager(t)
	r.maximumPaths.Store(4)

	// Same prefix advertised by two peers with identical attribute bytes.
	// Because the attribute pool deduplicates, both peers' AS_PATH handles
	// will compare equal and the strict-content multipath path fires.
	fam := family.IPv4Unicast
	attrBytes := concatBytes(testWireOriginIGP, testWireASPath65001, testWireNextHop, testWireLocalPref100)
	nlriBytes := []byte{24, 10, 0, 0} // 10.0.0.0/24

	peerA := storage.NewPeerRIB("192.0.2.1")
	peerA.Insert(fam, attrBytes, nlriBytes)
	r.ribInPool["192.0.2.1"] = peerA

	peerB := storage.NewPeerRIB("192.0.2.2")
	peerB.Insert(fam, attrBytes, nlriBytes)
	r.ribInPool["192.0.2.2"] = peerB

	result := r.bestPipeline("*", nil)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal([]byte(result), &parsed))
	entries, ok := parsed["best-path"].([]any)
	require.True(t, ok, "expected best-path array")
	require.Len(t, entries, 1)

	entry, ok := entries[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "192.0.2.1", entry["best-peer"], "lower peer address is primary")
	mp, ok := entry["multipath-peers"].([]any)
	require.True(t, ok, "multipath-peers must be present when maximumPaths>1 and siblings exist")
	require.Len(t, mp, 1)
	assert.Equal(t, "192.0.2.2", mp[0])
}

// TestBestPipeline_MultipathDisabledDefaults verifies that with the default
// maximumPaths=1 the JSON output does not include a multipath-peers field
// even when several candidates would tie.
//
// VALIDATES: cmd-3 phase 3 default behavior -- zero ECMP when multipath is
// off. Guards against accidental ECMP activation on existing deployments.
// PREVENTS: multipath-peers field leaking into the output when the config
// has no bgp/multipath block.
func TestBestPipeline_MultipathDisabledDefaults(t *testing.T) {
	r := newTestRIBManager(t)
	// maximumPaths defaults to 1 via the RIBManager constructor, but set
	// explicitly here to document the assumption.
	r.maximumPaths.Store(1)

	fam := family.IPv4Unicast
	attrBytes := concatBytes(testWireOriginIGP, testWireASPath65001, testWireNextHop, testWireLocalPref100)
	nlriBytes := []byte{24, 10, 0, 0}

	peerA := storage.NewPeerRIB("192.0.2.1")
	peerA.Insert(fam, attrBytes, nlriBytes)
	r.ribInPool["192.0.2.1"] = peerA
	peerB := storage.NewPeerRIB("192.0.2.2")
	peerB.Insert(fam, attrBytes, nlriBytes)
	r.ribInPool["192.0.2.2"] = peerB

	result := r.bestPipeline("*", nil)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal([]byte(result), &parsed))
	entries, _ := parsed["best-path"].([]any)
	require.Len(t, entries, 1)
	entry, _ := entries[0].(map[string]any)
	_, has := entry["multipath-peers"]
	assert.False(t, has, "multipath-peers must be absent when maximumPaths=1")
}
