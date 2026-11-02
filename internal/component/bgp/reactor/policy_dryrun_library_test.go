package reactor

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/policychain"
	"github.com/ze-software/ze/internal/component/bgp/policyview"
	"github.com/ze-software/ze/internal/component/plugin"
	routepolicy "github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestPolicyDryRunRunsLibraryChain proves the in-process half of AC-16
// (D-11): show policy test on a library chain runs the chain the live import
// step runs, display only, instead of sending a filter-update for a member no
// plugin serves. The decision and the resulting text are the live path's, the
// trace names each member with the sets it asked, and the chain counts no
// failure.
//
// Method: a member that sets local-preference 200 must report modify with
// that set and the rewritten text; a member that rejects must report reject.
// The text path answered both as a failed RPC to a plugin named "member".
func TestPolicyDryRunRunsLibraryChain(t *testing.T) {
	tests := []struct {
		name, filterType, action string
		entry                    plugin.PolicyTraceEntry
		textAfter                string
	}{
		{name: "sets", filterType: libraryTypeLP200, action: dryRunActionModify,
			entry:     plugin.PolicyTraceEntry{Filter: "member", Canonical: "member", Action: dryRunActionModify, Delta: "local-preference 200"},
			textAfter: "local-preference 200"},
		{name: "rejects", filterType: libraryTypeReject, action: dryRunActionReject,
			entry: plugin.PolicyTraceEntry{Filter: "member", Canonical: "member", Action: dryRunActionReject}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := New(&Config{})
			settings := NewPeerSettings(mustParseAddr("192.0.2.1"), 65000, 65001, 0x01010101)
			settings.ImportPolicy = libraryPolicy(t, libraryTestImport, tc.filterType, routepolicy.FinalAccept)
			r.peers[settings.PeerKey()] = NewPeer(settings)
			result, err := (&reactorAPIAdapter{r: r}).PolicyDryRun(
				"192.0.2.1", directionImport, "", libraryTestBody(100), true)
			require.NoError(t, err)
			require.Equal(t, tc.action, result.Action)
			if tc.textAfter != "" {
				// A passing member's entry carries the UPDATE as rewritten
				// up to it: here the whole chain's result.
				tc.entry.TextAfter = result.TextAfter
			}
			require.Equal(t, []plugin.PolicyTraceEntry{tc.entry}, result.Trace)
			if tc.textAfter != "" {
				require.True(t, strings.Contains(result.TextAfter, tc.textAfter), "text-after %q lacks %q", result.TextAfter, tc.textAfter)
			}
			chain, ok := settings.ImportPolicy.Library().(*policychain.Chain)
			require.True(t, ok)
			require.Zero(t, chain.Refused(), "a dry-run refuses no route")
		})
	}
}

// twoMemberLibraryPolicy is an import policy whose library chain runs "first"
// (filterType first) then "second" (filterType second), final accept.
func twoMemberLibraryPolicy(t *testing.T, first, second string) filterapi.Policy {
	t.Helper()
	reg, err := routepolicy.NewRegistry(routepolicy.Instance{Name: "first", Type: first}, routepolicy.Instance{Name: "second", Type: second})
	require.NoError(t, err)
	chain, err := policychain.Compile(libraryTestImport, reg, []policychain.Member{{Name: "first"}, {Name: "second"}},
		routepolicy.FinalAccept, policychain.Bases{LocalPreference: 100})
	require.NoError(t, err)
	refs := []routepolicy.Ref{{Name: "first"}, {Name: "second"}}
	return mustPolicy(t, refs, routepolicy.FinalAccept).WithLibrary(chain)
}

// TestPolicyDryRunLibraryFilterOverride proves show policy test's single
// filter override on a library chain runs that one member through the chain,
// display only, rather than a filter-update to a plugin named after it.
//
// Method: the chain's first member rejects and its second sets
// local-preference 200. The override names the second, so the run must
// modify with only the second's entry: running the whole chain rejects, and
// the text path answers a failed RPC. An unknown name is refused.
func TestPolicyDryRunLibraryFilterOverride(t *testing.T) {
	r := New(&Config{})
	settings := NewPeerSettings(mustParseAddr("192.0.2.1"), 65000, 65001, 0x01010101)
	settings.ImportPolicy = twoMemberLibraryPolicy(t, libraryTypeReject, libraryTypeLP200)
	r.peers[settings.PeerKey()] = NewPeer(settings)
	adapter := &reactorAPIAdapter{r: r}

	result, err := adapter.PolicyDryRun("192.0.2.1", directionImport, "second", libraryTestBody(100), true)
	require.NoError(t, err)
	require.Equal(t, dryRunActionModify, result.Action)
	require.Len(t, result.Trace, 1)
	entry := result.Trace[0]
	require.Equal(t, "second", entry.Canonical)
	require.Equal(t, dryRunActionModify, entry.Action)
	require.Equal(t, "local-preference 200", entry.Delta)
	require.Contains(t, result.TextAfter, "local-preference 200")
	require.Equal(t, []string{"first", "second"}, result.Policy.Filters,
		"the report names the policy the live session runs")

	_, err = adapter.PolicyDryRun("192.0.2.1", directionImport, "absent", libraryTestBody(100), true)
	require.ErrorIs(t, err, errFilterNotFound)

	chain, ok := settings.ImportPolicy.Library().(*policychain.Chain)
	require.True(t, ok)
	require.Zero(t, chain.Refused(), "a dry-run refuses no route")
}

// TestPolicyDryRunLibraryEntryTextAfter proves each library member's trace
// entry carries the UPDATE as rewritten up to that member, as a text filter's
// entry carries the text after its delta: a passing member's rewrite, and no
// text for the member that rejected, as the text path's rejecting entry.
//
// Method: the first member sets local-preference 200, the second rejects.
// The first entry must render LOCAL_PREF 200 and not the received 100; an
// entry left empty, or rendered from the received UPDATE, fails.
func TestPolicyDryRunLibraryEntryTextAfter(t *testing.T) {
	r := New(&Config{})
	settings := NewPeerSettings(mustParseAddr("192.0.2.1"), 65000, 65001, 0x01010101)
	settings.ImportPolicy = twoMemberLibraryPolicy(t, libraryTypeLP200, libraryTypeReject)
	r.peers[settings.PeerKey()] = NewPeer(settings)

	result, err := (&reactorAPIAdapter{r: r}).PolicyDryRun("192.0.2.1", directionImport, "", libraryTestBody(100), true)
	require.NoError(t, err)
	require.Equal(t, dryRunActionReject, result.Action)
	require.Len(t, result.Trace, 2)
	first := result.Trace[0].TextAfter
	require.Contains(t, first, "local-preference 200")
	require.NotContains(t, first, "local-preference 100")
	require.Equal(t, strings.Replace(result.TextBefore, "local-preference 100", "local-preference 200", 1), first,
		"the first entry is the received UPDATE with only LOCAL_PREF rewritten")
	require.Empty(t, result.Trace[1].TextAfter, "a rejecting member's entry carries no text, as the text path's")
}

// TestPolicyDryRunLibraryReportsWireChanges proves that show policy test over
// a library chain explains the route the live export sees and reports the wire
// operations the live run performs: an UPDATE pasted as received over a
// two-octet session (AS_PATH [64500 23456], AS4_PATH [4200000000]) is first
// collapsed to the four-octet form, as the ingest does (RFC 6793 Section
// 4.2.3), so an external filter reading the AS path is served, and its as-path
// remove "private" intent rewrites the merged AS_PATH, which wire-changes then
// names (RFC 6996 Section 4).
//
// VALIDATES: the dry-run collapses a two-octet body before the chains run, and
// its wire-changes come from the operations the library rewrite built.
// PREVENTS: every library member reading the AS path failing on a two-octet
// paste, and an intent answered as sets leaving wire-changes empty.
func TestPolicyDryRunLibraryReportsWireChanges(t *testing.T) {
	// AS_PATH AS_SEQUENCE [64500 23456] at two octets, AS4_PATH
	// AS_SEQUENCE [4200000000], LOCAL_PREF 100, NLRI 10.0.0.0/24.
	body := nlriHex(t, "0000 0024 40010100 400206 0202 fbf4 5ba0 40030401010101"+
		" c01106 0201 fa56ea00 40050400000064 180a0000")
	transport := &externalTransport{reads: []string{"as-path"}, sets: []string{"as-path"},
		reply: rpc.FilterUpdateOutput{Action: rpc.FilterModify,
			Sets: []rpc.FilterSet{intentSet("as-path", rpc.FilterSetActionRemove, `"private"`)}}}
	r := New(&Config{})
	r.filterTransportSeam = transport
	settings := NewPeerSettings(mustParseAddr("192.0.2.1"), 65000, 65001, 0x01010101)
	settings.ExportPolicy = rfc6996ExportPolicy(t)
	peer := NewPeer(settings)
	peer.state.Store(int32(PeerStateEstablished))
	r.peers[settings.PeerKey()] = peer

	result, err := (&reactorAPIAdapter{r: r}).PolicyDryRun("192.0.2.1", directionExport, "", body, false)
	if err != nil {
		t.Fatalf("dry-run: %v", err)
	}
	if result.Action != dryRunActionModify {
		t.Fatalf("action = %q, want %q (trace %+v)", result.Action, dryRunActionModify, result.Trace)
	}
	if !strings.Contains(result.TextBefore, "as-path [64500 4200000000]") {
		t.Fatalf("text-before = %q, want the merged as-path [64500 4200000000]", result.TextBefore)
	}
	if !strings.Contains(result.TextAfter, "as-path 64500 next-hop") {
		t.Fatalf("text-after = %q, want as-path 64500", result.TextAfter)
	}
	if !slices.Equal(result.WireChanges, []string{"AS_PATH set"}) {
		t.Fatalf("wire-changes = %v, want [AS_PATH set]", result.WireChanges)
	}
}

// libraryTypeSessionFacts is a member that passes only when the session it
// runs under answers the facts the live step gives it: peer-name is the
// session peer's name, source-peer-name is absent (export: the pasted UPDATE
// came from no peer), and on import a metadata set succeeds.
const libraryTypeSessionFacts = "reactor-library-test-session-facts"

// dryRunPeerName is the name the session-facts member requires.
const dryRunPeerName = "dry-peer"

func init() {
	routepolicy.MustRegisterFilterType(libraryTypeSessionFacts, nil, nil)
	policychain.MustRegisterEvaluator(libraryTypeSessionFacts, rpc.OnErrorReject, func(string, map[string]any, routepolicy.InstanceResolver) (policychain.Evaluator, error) {
		return evaluateDryRunSessionFacts, nil
	})
}

// evaluateDryRunSessionFacts is libraryTypeSessionFacts' evaluator.
func evaluateDryRunSessionFacts(v *policyview.View, _ *policychain.Sets) (policychain.Decision, error) {
	name, err := readSessionFact(v, policyview.FieldPeerName)
	if err != nil {
		return policychain.DecisionReject, err
	}
	if !name.Present() || name.Text() != dryRunPeerName {
		return policychain.DecisionReject, nil
	}
	source, err := readSessionFact(v, policyview.FieldSourcePeerName)
	if err != nil {
		return policychain.DecisionReject, err
	}
	if source.Present() {
		return policychain.DecisionReject, nil
	}
	if v.Session().SenderKnown {
		rebuilt, ok := policyview.MetadataNamed(filterapi.MetaKeyRebuilt)
		if !ok {
			return policychain.DecisionReject, errors.New("rebuilt metadata not registered")
		}
		if err := v.SetMetadata(rebuilt, true); err != nil {
			return policychain.DecisionReject, err
		}
	}
	return policychain.DecisionPass, nil
}

// readSessionFact reads field through its registered reader.
func readSessionFact(v *policyview.View, field routepolicy.Field) (policyview.Value, error) {
	reader, err := policyview.ReaderFor(field)
	if err != nil {
		return policyview.Value{}, err
	}
	return v.Read(reader)
}

// TestPolicyDryRunAnswersTheSessionFacts proves show policy test runs a
// library chain under the session the live step builds: the peer's session
// facts (policyview.ImportSession on import, ExportSession toward the
// destination on export, with no source peer for a pasted UPDATE) and, on
// import, a writable metadata map.
//
// Method: the member passes only when peer-name answers the peer's name,
// source-peer-name answers absent, and on import a metadata set succeeds;
// otherwise it rejects (or fails, which on-error reject also answers reject).
// The dry-run passed a session holding only the EBGP flag and the sender AS,
// so peer-name answered absent and every direction rejected.
func TestPolicyDryRunAnswersTheSessionFacts(t *testing.T) {
	for _, direction := range []string{directionImport, directionExport} {
		t.Run(direction, func(t *testing.T) {
			r := New(&Config{})
			settings := NewPeerSettings(mustParseAddr("192.0.2.1"), 65000, 65001, 0x01010101)
			settings.Name = dryRunPeerName
			if direction == directionImport {
				settings.ImportPolicy = libraryPolicy(t, libraryTestImport, libraryTypeSessionFacts, routepolicy.FinalAccept)
			} else {
				settings.ExportPolicy = libraryPolicy(t, libraryTestExport, libraryTypeSessionFacts, routepolicy.FinalAccept)
			}
			r.peers[settings.PeerKey()] = NewPeer(settings)
			result, err := (&reactorAPIAdapter{r: r}).PolicyDryRun("192.0.2.1", direction, "", libraryTestBody(100), true)
			require.NoError(t, err)
			require.Equal(t, dryRunActionAccept, result.Action, "trace %+v", result.Trace)
		})
	}
}
