// Related: peer_settings_negotiation.go — the negotiation-equivalence procedure under test
// Related: peer_settings_apply.go — the swap-or-restart decision it feeds
package reactor

import (
	"reflect"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/capability"
)

// capIPv4 and friends are the capability values the tests build OPENs from. Each
// is a fresh value per call: capabilitiesEqual compares by wire encoding, so a
// shared pointer would hide a fixture that failed to differ.
func capIPv4() capability.Capability {
	return &capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast}
}

func capIPv6() capability.Capability {
	return &capability.Multiprotocol{AFI: capability.AFIIPv6, SAFI: capability.SAFIUnicast}
}

func capAddPathIPv4Both() capability.Capability {
	return &capability.AddPath{Families: []capability.AddPathFamily{{
		AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast, Mode: capability.AddPathBoth,
	}}}
}

// negotiationSettings is a peer whose ONLY interesting field is its configured
// capability list, so two of them differ in Capabilities and in nothing else.
func negotiationSettings(caps ...capability.Capability) *PeerSettings {
	ps := NewPeerSettings(mustParseAddr("10.0.0.2"), 65001, 65002, 0x01020304)
	ps.Connection = ConnectionPassive
	ps.Capabilities = caps
	return ps
}

// establishNegotiatedSession attaches to p a session that has exchanged OPENs and
// negotiated, which is the state the reload decision is taken against.
//
// The local OPEN comes from buildOpen, the same producer sendOpen uses, and the
// peer OPEN carries peerCaps in real optional-parameter form. Nothing is
// hand-assembled: a fixture that encoded capabilities differently from the wire
// would let the procedure pass over an OPEN ze never sends.
func establishNegotiatedSession(t *testing.T, p *Peer, peerCaps ...capability.Capability) *Session {
	t.Helper()

	s := NewSession(p.settings)
	s.setConfigCapabilityGetter(p.configuredCapabilities)
	s.localOpen = s.buildOpen(p.settings, p.settings.Capabilities)
	peerParams, peerExtended := buildOptionalParams(peerCaps)
	s.peerOpen = &message.Open{
		Version:        4,
		MyAS:           uint16(p.settings.PeerAS), //nolint:gosec // test ASN is 65002
		HoldTime:       uint16(p.settings.ReceiveHoldTime.Seconds()),
		BGPIdentifier:  0x05060708,
		ASN4:           p.settings.PeerAS,
		OptionalParams: peerParams,
		ExtendedParams: peerExtended,
	}

	localCaps, err := capability.ParseFromOptionalParams(s.localOpen.OptionalParams, s.localOpen.ExtendedParams)
	require.NoError(t, err)
	s.negotiateWith(localCaps, peerCaps)
	require.NotNil(t, s.negotiated, "the fixture must reach a negotiated session")

	p.mu.Lock()
	p.session = s
	p.mu.Unlock()
	return s
}

// TestNegotiationOutcomeDecidesSwapOrRestart drives the owner's ruling of
// 2026-08-07 over the four cases it names.
//
// The ruling, verbatim: "if capabilities are removed from the peer which were not
// used or if added when the other peer does not use them, ie: if the resulting
// negotiation would be similar and lead to the same encoding and same families, we
// can accept the change and keep the BGP session up, otherwise have to re-start
// and re-negotiate".
//
// VALIDATES: the decision is taken by RUNNING the negotiation against what the
// peer advertised, not by classifying fields. Every row changes Capabilities and
// nothing else, so the only thing that can move the verdict is the negotiated
// outcome.
// PREVENTS: both failures the ruling sits between. Restarting for a capability the
// peer never used bounces a session for no wire change; swapping a capability the
// peer DOES use leaves the session encoding one thing while the config says
// another, with nothing reporting it.
//
// DISCRIMINATION: the swap rows and the restart rows are driven by the SAME peer
// capability set within each pair, so no row can pass for a reason outside the
// procedure. Inverting negotiationOutcomeUnchanged to a constant fails half the
// table whichever constant is chosen: true fails every restart row, false fails
// every swap row.
func TestNegotiationOutcomeDecidesSwapOrRestart(t *testing.T) {
	tests := []struct {
		name     string
		current  []capability.Capability
		next     []capability.Capability
		peerCaps []capability.Capability
		want     string // "" means swap
	}{
		{
			name:     "capability removed that the peer never used",
			current:  []capability.Capability{capIPv4(), &capability.ExtendedMessage{}},
			next:     []capability.Capability{capIPv4()},
			peerCaps: []capability.Capability{capIPv4()},
			want:     "",
		},
		{
			name:     "capability added that the peer does not support",
			current:  []capability.Capability{capIPv4()},
			next:     []capability.Capability{capIPv4(), &capability.ExtendedMessage{}},
			peerCaps: []capability.Capability{capIPv4()},
			want:     "",
		},
		{
			name:     "family added that the peer does not support",
			current:  []capability.Capability{capIPv4()},
			next:     []capability.Capability{capIPv4(), capIPv6()},
			peerCaps: []capability.Capability{capIPv4()},
			want:     "",
		},
		{
			name:     "family removed that the peer does use",
			current:  []capability.Capability{capIPv4(), capIPv6()},
			next:     []capability.Capability{capIPv4()},
			peerCaps: []capability.Capability{capIPv4(), capIPv6()},
			want:     "Capabilities",
		},
		{
			name:     "add-path added and the peer offers it, which changes the NLRI encoding",
			current:  []capability.Capability{capIPv4()},
			next:     []capability.Capability{capIPv4(), capAddPathIPv4Both()},
			peerCaps: []capability.Capability{capIPv4(), capAddPathIPv4Both()},
			want:     "Capabilities",
		},
		{
			name:     "extended message added and the peer offers it, which changes the max message size",
			current:  []capability.Capability{capIPv4()},
			next:     []capability.Capability{capIPv4(), &capability.ExtendedMessage{}},
			peerCaps: []capability.Capability{capIPv4(), &capability.ExtendedMessage{}},
			want:     "Capabilities",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			current := negotiationSettings(tc.current...)
			next := negotiationSettings(tc.next...)
			require.False(t, peerSettingsEqual(current, next), "the fixture must be a real change")

			r, peer := newSwapTestReactor(t, current, next)
			session := establishNegotiatedSession(t, peer, tc.peerCaps...)
			_ = r

			_, reason := peerSettingsSwapPlan(current, next, session)
			assert.Equal(t, tc.want, reason)
		})
	}
}

// TestReloadCapabilityChangeKeepsTheSessionWhenNegotiationIsUnchanged verifies the
// swap verdict end to end: the reload delivers the new capability set to the
// running peer AND leaves the session alone.
//
// VALIDATES: the ruling's "we can accept the change and keep the BGP session up".
// Both halves are asserted, because either alone is a known failure mode: keeping
// the session without delivering the set is the silent-discard defect this spec
// exists to fix, and delivering it by bouncing is the regression the swap path
// exists to remove.
//
// DISCRIMINATION: the assertions are peer OBJECT IDENTITY, the generation counter,
// and the capability set read back off the running peer. "A peer exists at this
// key" passes against both failures; identity plus the delivered set passes
// against neither.
func TestReloadCapabilityChangeKeepsTheSessionWhenNegotiationIsUnchanged(t *testing.T) {
	current := negotiationSettings(capIPv4(), &capability.ExtendedMessage{})
	next := negotiationSettings(capIPv4())

	r, peer := newSwapTestReactor(t, current, next)
	establishNegotiatedSession(t, peer, capIPv4())
	peer.state.Store(int32(PeerStateEstablished))
	generationBefore := r.peerGeneration.Load()

	adapter := &reactorAPIAdapter{r: r}
	require.NoError(t, adapter.Reload())

	r.mu.RLock()
	after := r.peers[current.PeerKey()]
	r.mu.RUnlock()

	require.NotNil(t, after, "the peer must still be configured after reload")
	assert.True(t, after == peer, "a capability the peer never used must not replace the peer object")
	assert.Equal(t, generationBefore, r.peerGeneration.Load(),
		"peerGeneration must not advance: a restart re-adds the peer and bumps it")
	assert.Equal(t, PeerStateEstablished, after.State(),
		"the established session must survive a capability change the negotiation ignores")
	assert.True(t, capabilitiesEqual(next.Capabilities, after.configuredCapabilities()),
		"the new capability set must reach the running peer, so the NEXT OPEN carries it")
}

// TestReloadCapabilityChangeRestartsWhenNegotiationChanges verifies the restart
// verdict end to end.
//
// VALIDATES: the ruling's "otherwise have to re-start and re-negotiate". Removing
// a family the peer DOES use changes the negotiated family set, so the running
// session cannot be left encoding for a family the config no longer asks for.
// PREVENTS: the swap path over-reaching into changes the wire can see.
func TestReloadCapabilityChangeRestartsWhenNegotiationChanges(t *testing.T) {
	current := negotiationSettings(capIPv4(), capIPv6())
	next := negotiationSettings(capIPv4())

	r, peer := newSwapTestReactor(t, current, next)
	establishNegotiatedSession(t, peer, capIPv4(), capIPv6())
	generationBefore := r.peerGeneration.Load()

	adapter := &reactorAPIAdapter{r: r}
	require.NoError(t, adapter.Reload())

	r.mu.RLock()
	after := r.peers[current.PeerKey()]
	r.mu.RUnlock()

	require.NotNil(t, after, "the peer must be re-added after the restart")
	assert.False(t, after == peer, "dropping a negotiated family must rebuild the peer")
	assert.Greater(t, r.peerGeneration.Load(), generationBefore,
		"a restart re-adds the peer, which advances peerGeneration")
}

// TestNegotiationProbeFailsClosed verifies that every state in which the procedure
// cannot PROVE the outcome identical answers restart.
//
// VALIDATES: the safety direction (ai/rules/evidence.md). A wrong restart costs one
// reconverge and announces itself; a wrong swap leaves a session running settings
// nobody checked and says nothing.
// PREVENTS: the probe reading "no evidence of a change" as "no change". Each row
// removes one piece of the evidence the decision rests on, so a probe that
// defaulted to true would pass every one of them.
func TestNegotiationProbeFailsClosed(t *testing.T) {
	current := negotiationSettings(capIPv4(), &capability.ExtendedMessage{})
	next := negotiationSettings(capIPv4())

	t.Run("no session at all", func(t *testing.T) {
		_, reason := peerSettingsSwapPlan(current, next, nil)
		assert.Equal(t, "Capabilities", reason, "a peer with no session has no negotiation to preserve")
	})

	t.Run("session that never exchanged OPENs", func(t *testing.T) {
		s := NewSession(current)
		_, reason := peerSettingsSwapPlan(current, next, s)
		assert.Equal(t, "Capabilities", reason, "a session with no OPEN pair proves nothing")
	})

	t.Run("session with our OPEN but not the peer's", func(t *testing.T) {
		_, peer := newSwapTestReactor(t, current, next)
		s := NewSession(peer.settings)
		s.setConfigCapabilityGetter(peer.configuredCapabilities)
		s.localOpen = s.buildOpen(peer.settings, peer.settings.Capabilities)
		_, reason := peerSettingsSwapPlan(current, next, s)
		assert.Equal(t, "Capabilities", reason, "half an OPEN exchange proves nothing")
	})

	t.Run("router id change alongside an ignorable capability change", func(t *testing.T) {
		_, peer := newSwapTestReactor(t, current, next)
		session := establishNegotiatedSession(t, peer, capIPv4())

		rotated := negotiationSettings(capIPv4())
		rotated.RouterID = 0x0A0B0C0D
		_, reason := peerSettingsSwapPlan(current, rotated, session)
		// Exact, not Contains. This is the scenario peerSettingsSwapPlan's own comment
		// gives as the reason the walk continues past Capabilities, and it is the only
		// unit fixture in which two fields force the restart together. A Contains
		// assertion passes when one of the two names is missing, which is exactly the
		// operator-visible failure the walk exists to prevent: one name fixed, the
		// session flaps again on the other.
		assert.Equal(t, "Capabilities,RouterID", reason,
			"both reasons must be named: the capability edit AND the BGP Identifier the peer sees unmediated")
	})
}

// TestOpenHeaderEqualCoversEveryOpenField verifies that openHeaderEqual
// discriminates on EVERY field of message.Open except OptionalParams.
//
// VALIDATES: the derived shape of openHeaderEqual. The field list is enumerated by
// reflection from message.Open itself, so a field added to that struct joins this
// test on the next build with no edit here.
// PREVENTS: the fail-OPEN direction. A field openHeaderEqual does not read is a
// difference the peer can see and the probe cannot, so the swap would be taken and
// the session kept under a negotiation the peer never agreed to.
//
// DISCRIMINATION: dropping any one comparison from openHeaderEqual fails the row
// for that field. The test cannot pass vacuously: it requires a real difference per
// field, and requires it to be reported.
func TestOpenHeaderEqualCoversEveryOpenField(t *testing.T) {
	base := &message.Open{
		Version:        4,
		MyAS:           65001,
		HoldTime:       90,
		BGPIdentifier:  0x01020304,
		ASN4:           65001,
		OptionalParams: []byte{0x02, 0x00},
	}
	require.True(t, openHeaderEqual(base, base), "a header must equal itself")

	// One mutator per message.Open field, keyed by field name. The reflection walk
	// below fails when a field has no entry, so a new field cannot be added to
	// message.Open without either a mutator here or a deliberate exclusion.
	mutate := map[string]func(o *message.Open){
		"Version":       func(o *message.Open) { o.Version = 5 },
		"MyAS":          func(o *message.Open) { o.MyAS = 65099 },
		"HoldTime":      func(o *message.Open) { o.HoldTime = 180 },
		"BGPIdentifier": func(o *message.Open) { o.BGPIdentifier = 0x0A0B0C0D },
		"ASN4":          func(o *message.Open) { o.ASN4 = 4200000000 },
		// A peer that switches between RFC 4271 and RFC 9072 parameter framing has
		// changed how its optional parameters must be READ, which is a property of
		// the header rather than of any capability inside it.
		"ExtendedParams": func(o *message.Open) { o.ExtendedParams = true },
	}

	for field := range reflect.TypeFor[message.Open]().Fields() {
		name := field.Name
		if name == "OptionalParams" {
			// Excluded on purpose: the capabilities are exactly what the ruling
			// allows to differ. Asserted in the other direction below.
			continue
		}
		t.Run(name, func(t *testing.T) {
			apply, ok := mutate[name]
			require.True(t, ok,
				"message.Open gained field %s: give it a mutator here so openHeaderEqual is proved to read it", name)
			changed := *base
			apply(&changed)
			require.NotEqual(t, *base, changed, "the mutator must produce a real difference")
			assert.False(t, openHeaderEqual(base, &changed),
				"a difference in %s reaches the peer unmediated by any negotiation", name)
		})
	}

	t.Run("OptionalParams is excluded", func(t *testing.T) {
		differentCaps := *base
		differentCaps.OptionalParams = []byte{0x02, 0x06, 0x01, 0x04, 0x00, 0x01, 0x00, 0x01}
		assert.True(t, openHeaderEqual(base, &differentCaps),
			"the capability parameters are what the negotiation comparison judges, not the header")
	})

	t.Run("a missing side is not a comparison", func(t *testing.T) {
		assert.False(t, openHeaderEqual(nil, base))
		assert.False(t, openHeaderEqual(base, nil))
	})
}

// TestReloadDecisionReadsPeerSettingsUnderLock runs the reload reconcile against a
// peer whose settings are being written on another goroutine.
//
// VALIDATES: reconcilePeersJournaled (reactor_api.go) reads the running peer's
// settings through Peer.settingsSnapshot (peer.go), which takes p.mu. That read is
// a WHOLE-STRUCT read -- peerSettingsEqual, then the c := *current copy and the
// Capabilities read inside peerSettingsSwapPlan (peer_settings_apply.go) -- so the
// per-field accessors do not serve it and the live pointer must not be used.
// PREVENTS: the weaker claim that serializing reconciles makes the read safe. The
// struct has writers on two goroutines, not one, so no serialization of the reload
// goroutine against itself can cover it.
//
// The concurrent writer here is applyHotSwappableSettings, which takes p.mu for its
// whole write. It is used rather than resolveDynamicPeerSettings, the production
// establishment-goroutine writer, because that function carries an unlocked read of
// its own and would report ITS race rather than this one
// (plan/deferrals/fixit-dynamic-peer-settings-unlocked-read.md).
//
// DISCRIMINATION: this test asserts nothing by itself. It is a RACE DETECTOR
// test, and it is evidence only under `./le test-unit bgp`. Reverting
// settingsSnapshot to Settings in reconcilePeersJournaled makes it report the write
// below against the read in the reconcile.
func TestReloadDecisionReadsPeerSettingsUnderLock(t *testing.T) {
	current := negotiationSettings(capIPv4())
	next := negotiationSettings(capIPv4())
	next.ImportFilters = []filterapi.FilterRef{{Name: "reloaded"}}

	r, peer := newSwapTestReactor(t, current, next)
	establishNegotiatedSession(t, peer, capIPv4())
	peer.state.Store(int32(PeerStateEstablished))

	other := negotiationSettings(capIPv4())
	other.ImportFilters = []filterapi.FilterRef{{Name: "concurrent"}}
	other.PrefixUpdated = map[string]string{"ipv4/unicast": "2026-08-08"}

	adapter := &reactorAPIAdapter{r: r}

	var wg sync.WaitGroup
	wg.Go(func() {
		for range 200 {
			peer.applyHotSwappableSettings(other, hotSwappableSettings)
		}
	})

	for range 200 {
		require.NoError(t, adapter.Reload())
	}
	wg.Wait()
}

// TestNegotiatedOutcomeEqualIgnoresMismatchesOnly verifies the two exclusions in
// the outcome comparison, in the direction each of them matters.
//
// VALIDATES: excluding RFC 5492 Section 3 Mismatches is what makes the ruling's
// first example reachable at all. Removing a capability the peer never had removes
// its mismatch entry, so a comparison including Mismatches would report every such
// change as a difference and the procedure would never swap.
// PREVENTS: widening the exclusion. The second row keeps a real encoding change
// visible, so an implementation that excluded more than Mismatches fails it.
func TestNegotiatedOutcomeEqualIgnoresMismatchesOnly(t *testing.T) {
	peerCaps := []capability.Capability{capIPv4()}

	withExtMsg := capability.Negotiate(
		[]capability.Capability{capIPv4(), &capability.ExtendedMessage{}}, peerCaps, 65001, 65002)
	withoutExtMsg := capability.Negotiate(
		[]capability.Capability{capIPv4()}, peerCaps, 65001, 65002)

	require.NotEmpty(t, withExtMsg.Mismatches, "the fixture must produce a mismatch to ignore")
	require.Empty(t, withoutExtMsg.Mismatches)
	assert.True(t, negotiatedOutcomeEqual(withExtMsg, withoutExtMsg),
		"a mismatch entry is reporting, not negotiated state")

	supported := capability.Negotiate(
		[]capability.Capability{capIPv4(), &capability.ExtendedMessage{}},
		[]capability.Capability{capIPv4(), &capability.ExtendedMessage{}}, 65001, 65002)
	assert.False(t, negotiatedOutcomeEqual(withoutExtMsg, supported),
		"a capability the peer DOES offer changes the encoding and must stay visible")

	assert.False(t, negotiatedOutcomeEqual(nil, withoutExtMsg), "a missing side is not a comparison")
	assert.False(t, negotiatedOutcomeEqual(withoutExtMsg, nil), "a missing side is not a comparison")
}
