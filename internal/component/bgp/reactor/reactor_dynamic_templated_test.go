// Design: docs/architecture/config/syntax.md — config variable substitution for dynamic peers
// Related: reactor_dynamic.go — resolveDynamicPeerSettings, writeDynamicTemplateLocked
// Related: ../filterapi/policy.go — Policy.CompileResolved

package reactor

import (
	"errors"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/message"
	routepolicy "github.com/ze-software/ze/internal/component/policy"
)

// compiledMarker stands for the library chain a TemplatedChain compiles; it
// names the first resolved ref, so a test reads which refs it was given.
type compiledMarker struct{ first string }

func (compiledMarker) Refused() uint64 { return 0 }

func (m compiledMarker) SameDecisions(other filterapi.LibraryChain) bool {
	return other == filterapi.LibraryChain(m)
}

// recordingTemplated is a TemplatedChain that compiles every resolved chain to
// a compiledMarker, or, when refuse is set, answers the fail-closed policy
// with an error, as bgp/config does for a resolved name naming no instance.
type recordingTemplated struct{ refuse bool }

func (r recordingTemplated) CompileResolved(resolved filterapi.Policy, _ filterapi.PeerFilterInfo) (filterapi.Policy, error) {
	if r.refuse {
		return resolved.WithLibrary(compiledMarker{first: "fail-closed"}), errors.New("no instance")
	}
	return resolved.WithLibrary(compiledMarker{first: resolved.Filters()[0].Name}), nil
}

func templatedGroup(t *testing.T, name string, refuse bool) *DynamicGroupConfig {
	t.Helper()
	dg := dynamicReloadGroup(t)
	dg.Settings.ImportPolicy = mustPolicy(t, []routepolicy.Ref{{Name: name}}, routepolicy.FinalAccept).
		WithTemplated(recordingTemplated{refuse: refuse})
	return dg
}

func openAndResolve(peer *Peer) {
	session := NewSession(peer.settings)
	session.mu.Lock()
	session.peerOpen = &message.Open{MyAS: dynamicPeerRemoteAS, BGPIdentifier: 0x0a0b0c0d}
	session.mu.Unlock()
	peer.mu.Lock()
	peer.session = session
	peer.mu.Unlock()
	peer.resolveDynamicPeerSettings(session)
}

// TestDynamicPeerCompilesItsResolvedChain proves that a dynamic peer's chain
// whose member names an instance through `$remote_as` is compiled from the
// RESOLVED refs, at establishment and again when a reload swaps the template
// of an established peer.
//
// VALIDATES: resolveDynamicPeerSettings and writeDynamicTemplateLocked publish
// Policy.CompileResolved's answer over the resolved refs.
// PREVENTS: a dynamic peer running the template's library, compiled from
// `ALLOW-$remote_as`, which names no instance (spec filter-wire-0, phase 4).
func TestDynamicPeerCompilesItsResolvedChain(t *testing.T) {
	addr := netip.MustParseAddr("185.1.69.46")
	r, peer := newDynamicReloadReactor(t, templatedGroup(t, "ALLOW-$remote_as", false), addr, nil)

	openAndResolve(peer)
	got := peer.ImportPolicy()
	assert.Equal(t, []routepolicy.Ref{{Name: "ALLOW-65010"}}, got.Filters())
	assert.Equal(t, compiledMarker{first: "ALLOW-65010"}, got.Library(), "the library is compiled from the resolved refs")
	assert.False(t, got.Templated(), "a resolved chain is never compiled again")

	reloadDynamicGroups(t, r, templatedGroup(t, "DENY-$remote_as", false))
	got = peer.ImportPolicy()
	assert.Equal(t, []routepolicy.Ref{{Name: "DENY-65010"}}, got.Filters())
	assert.Equal(t, compiledMarker{first: "DENY-65010"}, got.Library(), "the swap compiles the swapped template for the learned AS")
}

// TestDynamicPeerFailsClosedOnAnUnresolvableChain proves that a resolved name
// the TemplatedChain refuses installs the fail-closed policy it answers.
//
// VALIDATES: the refusal's policy, not the unresolved template, is published.
// PREVENTS: a dynamic peer accepting every route when `$remote_as` resolves to
// a name no instance has.
func TestDynamicPeerFailsClosedOnAnUnresolvableChain(t *testing.T) {
	addr := netip.MustParseAddr("185.1.69.47")
	_, peer := newDynamicReloadReactor(t, templatedGroup(t, "ALLOW-$remote_as", true), addr, nil)

	openAndResolve(peer)
	require.Equal(t, compiledMarker{first: "fail-closed"}, peer.ImportPolicy().Library())
}

// definitionTemplated is a TemplatedChain built from one config's definitions:
// definition stands for the list a member's match names, so two values differ
// only when a reload edited a definition, never a ref.
type definitionTemplated struct{ definition string }

func (d definitionTemplated) CompileResolved(resolved filterapi.Policy, _ filterapi.PeerFilterInfo) (filterapi.Policy, error) {
	return resolved.WithLibrary(compiledMarker{first: resolved.Filters()[0].Name + "@" + d.definition}), nil
}

func definitionGroup(t *testing.T, definition string) *DynamicGroupConfig {
	t.Helper()
	dg := dynamicReloadGroup(t)
	dg.Settings.ImportPolicy = mustPolicy(t, []routepolicy.Ref{{Name: "ALLOW-$remote_as"}}, routepolicy.FinalAccept).
		WithTemplated(definitionTemplated{definition: definition})
	return dg
}

// TestDynamicPeerRecompilesOnADefinitionOnlyReload proves that a reload which
// leaves an established dynamic peer's chain refs unchanged and edits only a
// definition the chain's match reaches recompiles the peer's running chain
// from the new template, in place.
//
// VALIDATES: planDynamicGroupsLocked reads the changed template as a change
// (peerSettingsEqual over the templated chain) and writeDynamicTemplateLocked
// compiles the swapped template for the learned AS.
// PREVENTS: the silent reload: the commit succeeds and the peer keeps the
// match compiled from the startup list.
func TestDynamicPeerRecompilesOnADefinitionOnlyReload(t *testing.T) {
	addr := netip.MustParseAddr("185.1.69.48")
	r, peer := newDynamicReloadReactor(t, definitionGroup(t, "startup"), addr, nil)

	openAndResolve(peer)
	require.Equal(t, compiledMarker{first: "ALLOW-65010@startup"}, peer.ImportPolicy().Library())
	session := peer.currentSession()

	reloadDynamicGroups(t, r, definitionGroup(t, "edited"))
	got := peer.ImportPolicy()
	assert.Equal(t, []routepolicy.Ref{{Name: "ALLOW-65010"}}, got.Filters())
	assert.Equal(t, compiledMarker{first: "ALLOW-65010@edited"}, got.Library(), "the running chain is compiled from the edited definition")
	assert.False(t, got.Templated(), "the running chain is resolved, never the template")
	assert.Same(t, session, peer.currentSession(), "the swap keeps the session")
}
