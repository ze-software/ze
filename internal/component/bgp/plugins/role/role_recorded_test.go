// Related: role.go -- setFilterState, recordNoRemoteRole, remoteRoleRecorded
// Related: otc.go -- OTCEgressFilter's export-set branch, the one reader that
// needs "recorded as no role" apart from "never recorded"
//
// Spec: plan/spec-fixit-stored-route-relay-hardening.md (R6-1, AC-8).
//
// Deliberately NOT in otc_test.go: that file carries `RFC requirement:` tags and
// is the proof behind a public RFC 9234 compliance claim. Nothing here changes
// an RFC 9234 decision, so nothing here belongs beside those.

package role

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
)

// TestReconfigureKeepsLearnedRolesForStillConfiguredPeers is the route
// black-holing half.
//
// A learned remote role is a property of the SESSION: it is what the peer put in
// its OPEN. setFilterState used to wipe the whole map on every OnConfigure, and
// an ESTABLISHED peer sends no second OPEN, so nothing rewrote the entry. Every
// live peer's role therefore vanished on config reload, getFilterConfig read the
// absent key as "", and OTCEgressFilter's export-set branch classified the peer
// roleUnknown -- so a source configured `role { export customer }` silently
// stopped advertising to its customers until each session bounced.
//
// VALIDATES: a reconfigure retains the learned role of every peer the new config
// still names, and drops it for peers it does not.
// PREVENTS: the reload black-hole above, and the opposite error of keeping a
// role for a peer that has been removed from the config.
func TestReconfigureKeepsLearnedRolesForStillConfiguredPeers(t *testing.T) {
	clearFilterState(t)

	configs := map[string]*peerRoleConfig{
		"10.0.0.1": {role: roleProvider},
		"10.0.0.2": {role: roleProvider},
	}
	setFilterState(configs, nil)
	setFilterRemoteRole("10.0.0.1", roleCustomer)
	setFilterRemoteRole("10.0.0.2", rolePeer)

	// Reload: 10.0.0.1 survives, 10.0.0.2 is removed, 10.0.0.3 is new.
	setFilterState(map[string]*peerRoleConfig{
		"10.0.0.1": {role: roleProvider},
		"10.0.0.3": {role: roleProvider},
	}, nil)

	_, learned := getFilterConfig("10.0.0.1")
	assert.Equal(t, roleCustomer, learned,
		"a peer whose session survived the reload has not withdrawn the role it advertised")
	assert.True(t, remoteRoleRecorded("10.0.0.1"),
		"the surviving peer's role is still recorded, so the export set can still be evaluated")

	assert.False(t, remoteRoleRecorded("10.0.0.2"),
		"a peer the new config no longer names keeps no learned role: it can never be read again")

	assert.False(t, remoteRoleRecorded("10.0.0.3"),
		"a newly configured peer has advertised nothing yet; its role is genuinely unrecorded")
}

// TestOpenDeclaringNoRoleIsRecordedNotDeleted is the ambiguity half.
//
// applyValidateOpen used to DELETE the map entry when an OPEN carried no usable
// Role capability. Deletion made "this peer's OPEN declared no role" -- an
// answer, and the one `export { unknown }` names -- share a representation with
// "no OPEN was ever recorded for this peer", which is a MISS reachable whenever
// broadcastValidateOpen skips the plugin.
//
// VALIDATES: an OPEN with no role RECORDS the absence; the RFC 9234 gates still
// see "" and still fall back to the configured complement.
// PREVENTS: the two states sharing one representation, which is what let an
// unrecorded peer borrow the answer meant for a peer that declared none.
func TestOpenDeclaringNoRoleIsRecordedNotDeleted(t *testing.T) {
	clearFilterState(t)

	// A peer name that is NOT the one the sibling tests use, so the name -> IP
	// resolution in recordNoRemoteRole is exercised rather than assumed.
	configs := map[string]*peerRoleConfig{"10.0.0.1": {role: roleProvider}}
	nameToIP := map[string]string{"edge1": "10.0.0.1"}
	setFilterState(configs, nameToIP)

	require.False(t, remoteRoleRecorded("10.0.0.1"),
		"before any OPEN, nothing is recorded")

	applyValidateOpen(configs, nameToIP, openWithRole("edge1", 3)) // Customer
	_, learned := getFilterConfig("10.0.0.1")
	require.Equal(t, roleCustomer, learned)
	require.True(t, remoteRoleRecorded("10.0.0.1"))

	// The peer reconnects advertising no Role capability at all.
	applyValidateOpen(configs, nameToIP, openWithoutRole("edge1"))

	_, learned = getFilterConfig("10.0.0.1")
	assert.Empty(t, learned,
		"the stale role must not survive: the peer stopped claiming it")
	assert.True(t, remoteRoleRecorded("10.0.0.1"),
		"but the OPEN WAS seen, and 'declared no role' is an answer -- it must not read as 'never recorded'")
	assert.Equal(t, roleCustomer, resolvePeerRole(learned, configs["10.0.0.1"]),
		"the RFC 9234 Section 5 gates still take the configured complement, unchanged")
}

// TestExportSetSuppressionDistinguishesUnrecordedFromPolicy is the operator
// signal, driven from the filter entry point rather than the helpers.
//
// Both cases suppress, both are policy decisions (Thomas's 2026-08-03 ruling
// makes `unknown` cover the unrecorded state too), and that is unchanged. What
// must differ is the REASON, because the two call for opposite operator actions:
// "your export set excludes this peer" points at the config, and "we never
// learned what this peer is" points at validate-open.
//
// VALIDATES: an unrecorded destination role counts dropRoleUnrecorded, and a
// recorded one that genuinely misses the export set counts dropExportSet.
// PREVENTS: a validate-open failure being reported as an export-set decision,
// which sends an operator to check a policy that never ran.
func TestExportSetSuppressionDistinguishesUnrecordedFromPolicy(t *testing.T) {
	clearFilterState(t)
	rec := installRecordingMetrics(t)

	// Source exports to customers only, so any other destination role misses.
	setFilterState(map[string]*peerRoleConfig{
		"10.0.0.1": {
			role:           roleProvider,
			export:         []string{"customer"},
			resolvedExport: resolveExport(roleProvider, []string{"customer"}),
		},
		"10.0.0.20": {role: roleCustomer},
		"10.0.0.21": {role: roleCustomer},
	}, nil)
	// 10.0.0.20's OPEN was seen and declared a role that is not in the set.
	setFilterRemoteRole("10.0.0.20", roleProvider)
	// 10.0.0.21's OPEN was never recorded at all.

	src := filterapi.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.1")}
	noOTC := buildTestPayload(buildTestAttrs(0), nil)

	recorded := filterapi.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.20")}
	require.False(t, OTCEgressFilter(src, recorded, noOTC, nil, nil),
		"a recorded role outside the export set is suppressed")
	assert.Equal(t, int64(1), rec.value(metricRouteSuppressions, reasonLabelExportSet),
		"a role we DID learn, absent from the set, is an export-set decision")
	assert.Equal(t, int64(0), rec.value(metricRouteSuppressions, reasonLabelRoleUnrecorded),
		"a decided suppression must not be reported as a miss")

	unrecorded := filterapi.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.21")}
	require.False(t, OTCEgressFilter(src, unrecorded, noOTC, nil, nil),
		"an unrecorded role still suppresses: a guard that cannot decide denies")
	assert.Equal(t, int64(1), rec.value(metricRouteSuppressions, reasonLabelRoleUnrecorded),
		"the miss is counted under its OWN reason, not folded into export-set")
	assert.Equal(t, int64(1), rec.value(metricRouteSuppressions, reasonLabelExportSet),
		"and it must not also increment the export-set counter")
}

// TestExportSetUnrecordedStillMatchesExplicitUnknown tests a DECISION, not a
// status quo. Thomas ruled it on 2026-08-03, answering R6-1's Q-1 in
// plan/spec-fixit-stored-route-relay-hardening.md: "KEEP MATCHING. Pin it as
// intended."
//
// The question was whether a destination whose role was NEVER RECORDED -- a
// validate-open RPC timeout, a plugin conn not yet up, a plugin respawn --
// should still match an explicit `export { unknown }`. `export { unknown }` is
// a documented operator token meaning "also send to peers with no role
// configured" (config.go). The two readings were: `unknown` names "the peer
// announced none", so a peer we never validated is something else and should
// not match; against that, `unknown` is the operator's own word for "this
// peer's role is not known to us", which is exactly the unrecorded state.
//
// The owner took the second reading. Operator intent is honored literally, no
// working config changes, and the accepted cost is stated: during an RPC or
// plugin failure ze advertises to a peer whose role is genuinely unknown.
//
// The consequence worth knowing when reading this file: roleUnknown is
// therefore a TOTAL answer over the destination-role state, so the export-set
// membership test always evaluates a defined input and its suppression is a
// policy DECISION -- which is what the reactor's forward rail counts it as, and
// what the stored-route relay reports as a handled route. dropRoleUnrecorded
// (the sibling test above) survives the ruling as an operator signal about
// WHICH flavor of unknown, not as a claim that the guard could not decide.
//
// VALIDATES: an explicit `unknown` in the export set admits a destination whose
// role was never recorded, as ruled.
// PREVENTS: the ruling being reversed by a later edit that reads the unrecorded
// state as an unanswered question -- which would turn an accepted route into a
// dropped one for every operator who wrote `export { default, unknown }`. Do not
// re-open this without a fresh owner ruling.
func TestExportSetUnrecordedStillMatchesExplicitUnknown(t *testing.T) {
	clearFilterState(t)
	rec := installRecordingMetrics(t)

	setFilterState(map[string]*peerRoleConfig{
		"10.0.0.1": {
			role:           roleProvider,
			export:         []string{"customer", "unknown"},
			resolvedExport: resolveExport(roleProvider, []string{"customer", "unknown"}),
		},
		"10.0.0.22": {role: roleCustomer},
	}, nil)

	src := filterapi.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.1")}
	dest := filterapi.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.22")}
	noOTC := buildTestPayload(buildTestAttrs(0), nil)

	require.False(t, remoteRoleRecorded("10.0.0.22"), "precondition: nothing recorded")
	assert.True(t, OTCEgressFilter(src, dest, noOTC, nil, nil),
		"an explicit `unknown` in the export set admits this peer; Thomas ruled this on 2026-08-03 (R6-1 / Q-1), so do not change it without a fresh owner ruling")
	assert.Equal(t, int64(0), rec.value(metricRouteSuppressions, reasonLabelRoleUnrecorded),
		"an accepted route records no drop at all")
}
