// VALIDATES: RFC 7999 Section 3.1 on the SEND side. Ze puts the BLACKHOLE
// community on a peer's wire only when that session recorded the agreement.
// PREVENTS: `announce blackhole` reaching a peer that agreed to nothing, which
// asks a network to drop traffic on a signal it never consented to read.

package announce

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/selector"
)

var (
	agreedAddr   = netip.MustParseAddr("192.0.2.1")
	unagreedAddr = netip.MustParseAddr("192.0.2.2")
)

// agreementReactor is a BGPReactor and a ReactorLifecycle at once: the handler
// dispatches through the first and reads the running config through the second.
type agreementReactor struct {
	bgptypes.BGPReactor
	plugin.ReactorLifecycle

	peers []plugin.PeerInfo
	tree  map[string]any

	sels    []*selector.Selector
	batches []bgptypes.NLRIBatch
}

func (r *agreementReactor) AnnounceNLRIBatch(sel *selector.Selector, batch bgptypes.NLRIBatch, _ plugin.Sender) error {
	r.sels = append(r.sels, sel)
	r.batches = append(r.batches, batch)
	return nil
}

func (r *agreementReactor) Peers() []plugin.PeerInfo      { return r.peers }
func (r *agreementReactor) GetConfigTree() map[string]any { return r.tree }

// agreementCtx builds a command context over two configured peers: 192.0.2.1
// names the communities given, 192.0.2.2 names none.
func agreementCtx(t *testing.T, communities ...string) (*pluginserver.CommandContext, *agreementReactor) {
	t.Helper()

	peerCfg := map[string]any{
		"connection": map[string]any{"remote": map[string]any{"ip": agreedAddr.String()}},
	}
	if len(communities) > 0 {
		peerCfg["blackhole"] = map[string]any{"communities": communities}
	}

	rctr := &agreementReactor{
		peers: []plugin.PeerInfo{
			{Address: agreedAddr, Name: "agreed"},
			{Address: unagreedAddr, Name: "unagreed"},
		},
		tree: map[string]any{"bgp": map[string]any{"peer": map[string]any{
			"agreed": peerCfg,
			"unagreed": map[string]any{
				"connection": map[string]any{"remote": map[string]any{"ip": unagreedAddr.String()}},
			},
		}}},
	}
	server, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, rctr)
	require.NoError(t, err)
	return &pluginserver.CommandContext{Server: server}, rctr
}

func newReg() *Registry {
	return NewRegistry(func(*selector.Selector, bgptypes.NLRIBatch, plugin.Sender) error { return nil })
}

// carriesBlackhole reports whether a dispatched batch tags the route with
// 65535:666, read back off the built attribute bytes.
func carriesBlackhole(t *testing.T, batch bgptypes.NLRIBatch) bool {
	t.Helper()
	require.NotNil(t, batch.Attrs)
	wire := batch.Attrs.Build()
	for i := 0; i+4 <= len(wire); i++ {
		if wire[i] == 0xFF && wire[i+1] == 0xFF && wire[i+2] == 0x02 && wire[i+3] == 0x9A {
			return true
		}
	}
	return false
}

// RFC requirement: RFC7999-3.1-2 positive -- the two networks agreed on use of
// the BLACKHOLE community for this session, recorded as the community named on
// the peer, so Ze advertises the tagged route to it.
func TestAnnounceBlackholeReachesAnAgreedPeer(t *testing.T) {
	ctx, rctr := agreementCtx(t, "blackhole")

	_, err := handleAnnounceBlackhole(ctx, rctr, newReg(), []string{"198.51.100.1/32"})
	require.NoError(t, err)

	require.Len(t, rctr.batches, 1, "the agreed peer must receive exactly one batch")
	assert.True(t, carriesBlackhole(t, rctr.batches[0]), "the advertised route does not carry 65535:666")
	assert.True(t, rctr.sels[0].Matches(agreedAddr), "the agreed peer was not selected")
}

// RFC requirement: RFC7999-3.1-2 negative -- the session with 192.0.2.2 recorded
// no agreement, so the BLACKHOLE community MUST NOT be advertised to it. The
// peer that did agree still gets it, which is what makes this a gate rather than
// a switch that turns the verb off.
func TestAnnounceBlackholeIsWithheldFromAPeerThatDidNotAgree(t *testing.T) {
	ctx, rctr := agreementCtx(t, "blackhole")

	_, err := handleAnnounceBlackhole(ctx, rctr, newReg(), []string{"198.51.100.1/32"})
	require.NoError(t, err)

	require.Len(t, rctr.sels, 1)
	assert.False(t, rctr.sels[0].Matches(unagreedAddr),
		"the un-agreed peer is inside the announcement's selector: 65535:666 reaches its wire")
}

// No selected peer agreed, so nothing is advertised and the operator is told.
// Announcing the prefix untagged instead would attract traffic to an address
// that is under attack, which is the opposite of what was asked for.
func TestAnnounceBlackholeRefusedWhenNoPeerAgreed(t *testing.T) {
	ctx, rctr := agreementCtx(t)

	resp, err := handleAnnounceBlackhole(ctx, rctr, newReg(), []string{"198.51.100.1/32"})

	require.Error(t, err)
	assert.Empty(t, rctr.batches, "a route was advertised to peers that agreed to nothing")
	require.NotNil(t, resp)
	assert.Contains(t, resp.Error, "unagreed", "the refusal does not name the peers that have not agreed")
}

// A session that agreed to its own RTBH community has not agreed to BLACKHOLE.
// RFC 7999 Section 3.1 binds the well-known value, so the gate answers per value.
func TestAnnounceBlackholeRefusedForAnOwnCommunityAgreement(t *testing.T) {
	ctx, rctr := agreementCtx(t, "65001:666")

	_, err := handleAnnounceBlackhole(ctx, rctr, newReg(), []string{"198.51.100.1/32"})

	require.Error(t, err)
	assert.Empty(t, rctr.batches, "BLACKHOLE went to a session that agreed to 65001:666 instead")
}

// The sibling origination path. `announce unicast ... community blackhole` puts
// the same value on the same wire, so it meets the same gate.
func TestAnnounceUnicastWithBlackholeCommunityMeetsTheSameGate(t *testing.T) {
	ctx, rctr := agreementCtx(t)

	_, err := handleAnnounceUnicast(ctx, rctr, newReg(), []string{"198.51.100.1/32", "community", "65535:666"})

	require.Error(t, err)
	assert.Empty(t, rctr.batches, "the unicast verb advertised BLACKHOLE to a peer that never agreed")
}

// The gate is scoped to the one community RFC 7999 governs. Every other
// community an operator attaches is untouched.
func TestAnnounceUnicastWithAnotherCommunityIsNotGated(t *testing.T) {
	ctx, rctr := agreementCtx(t)

	_, err := handleAnnounceUnicast(ctx, rctr, newReg(), []string{"198.51.100.1/32", "community", "65001:1"})

	require.NoError(t, err)
	require.Len(t, rctr.batches, 1, "an ordinary community announcement was refused")
	assert.True(t, rctr.sels[0].Matches(unagreedAddr), "an ordinary announcement was narrowed to the agreed peers")
}

// A community value is not a spelling: the shorthand keyword and the numeric
// form are one agreement and one gate.
func TestAnnounceBlackholeAgreementAcceptsEitherSpelling(t *testing.T) {
	for _, spelling := range []string{"blackhole", "65535:666"} {
		t.Run(spelling, func(t *testing.T) {
			ctx, rctr := agreementCtx(t, spelling)
			_, err := handleAnnounceBlackhole(ctx, rctr, newReg(), []string{"198.51.100.1/32"})
			require.NoError(t, err)
			assert.Len(t, rctr.batches, 1)
		})
	}
}

// An explicit peer selector is narrowed by the same test. Naming the un-agreed
// peer directly does not buy the agreement RFC 7999 asks for.
func TestAnnounceBlackholeRefusesAnExplicitlyNamedUnagreedPeer(t *testing.T) {
	ctx, rctr := agreementCtx(t, "blackhole")
	ctx.Peer = unagreedAddr.String()

	_, err := handleAnnounceBlackhole(ctx, rctr, newReg(), []string{"198.51.100.1/32"})

	require.Error(t, err)
	assert.Empty(t, rctr.batches)
}

// A malformed agreement refuses the command rather than resolving to an empty
// one. Reading "this peer agreed to nothing" out of a value nobody could parse
// is the same silence the parse refuses everywhere else.
func TestAnnounceBlackholeRefusesAnUnreadableAgreement(t *testing.T) {
	ctx, rctr := agreementCtx(t, "not-a-community")

	_, err := handleAnnounceBlackhole(ctx, rctr, newReg(), []string{"198.51.100.1/32"})

	require.Error(t, err)
	assert.Empty(t, rctr.batches)
}

// The withdrawal must reach exactly the peers the announcement reached, so the
// narrowed selector is what the tag registry stores.
func TestAnnounceBlackholeTracksTheNarrowedSelector(t *testing.T) {
	ctx, rctr := agreementCtx(t, "blackhole")
	reg := newReg()

	_, err := handleAnnounceBlackhole(ctx, rctr, reg, []string{"198.51.100.1/32", "tag", "k", "v"})
	require.NoError(t, err)

	entries := reg.List(listFilter{})
	require.Len(t, entries, 1)
	assert.False(t, entries[0].Selector.Matches(unagreedAddr),
		"a withdraw would fan out past the peers the announcement reached")
	assert.True(t, entries[0].Selector.Matches(agreedAddr))
}

// The gate reads the community as a VALUE, so an agreement stated at the group
// level reaches the peer under it.
func TestAnnounceBlackholeReadsAGroupLevelAgreement(t *testing.T) {
	rctr := &agreementReactor{
		peers: []plugin.PeerInfo{{Address: agreedAddr, Name: "agreed"}},
		tree: map[string]any{"bgp": map[string]any{"group": map[string]any{"g1": map[string]any{
			"blackhole": map[string]any{"communities": []string{"blackhole"}},
			"peer": map[string]any{"agreed": map[string]any{
				"connection": map[string]any{"remote": map[string]any{"ip": agreedAddr.String()}},
			}},
		}}}},
	}
	server, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, rctr)
	require.NoError(t, err)
	ctx := &pluginserver.CommandContext{Server: server}

	_, err = handleAnnounceBlackhole(ctx, rctr, newReg(), []string{"198.51.100.1/32"})
	require.NoError(t, err)
	assert.Len(t, rctr.batches, 1, "a group-level agreement did not reach the peer under it")
}

// A session configured with prefixes alone has agreed to the well-known value,
// so it is a legitimate destination for `announce blackhole`. The send side and
// the receive side read one answer, so the default cannot mean two things.
func TestAnnounceBlackholeReachesAPeerConfiguredWithPrefixesAlone(t *testing.T) {
	rctr := &agreementReactor{
		peers: []plugin.PeerInfo{{Address: agreedAddr, Name: "agreed"}},
		tree: map[string]any{"bgp": map[string]any{"peer": map[string]any{"agreed": map[string]any{
			"connection": map[string]any{"remote": map[string]any{"ip": agreedAddr.String()}},
			"blackhole":  map[string]any{"prefixes": []string{"10.0.0.0/24"}},
		}}}},
	}
	server, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, rctr)
	require.NoError(t, err)
	ctx := &pluginserver.CommandContext{Server: server}

	_, err = handleAnnounceBlackhole(ctx, rctr, newReg(), []string{"198.51.100.1/32"})
	require.NoError(t, err)
	assert.Len(t, rctr.batches, 1, "a session configured with prefixes alone was refused the community it defaults to")
}

// AC-7 on the send side. A session the reactor created from a dynamic group
// carries the agreement its group stated: the operator writes one `blackhole`
// block on the listen-range group, and the member's own address appears nowhere
// in the document for the gate to key on.
//
// RFC requirement: RFC7999-3.1-2 positive -- the two networks agreed on use of
// the BLACKHOLE community for this session, recorded on the group the session
// was built from, so Ze advertises the tagged route to it.
func TestAnnounceBlackholeReachesAMemberOfADynamicGroup(t *testing.T) {
	member := netip.MustParseAddr("192.0.2.10")
	rctr := &agreementReactor{
		peers: []plugin.PeerInfo{{Address: member, Name: "dyn-192.0.2.10", GroupName: "ix"}},
		tree: map[string]any{"bgp": map[string]any{"group": map[string]any{"ix": map[string]any{
			"connection": map[string]any{"remote": map[string]any{"ip": "dynamic", "range": []string{"192.0.2.0/24"}}},
			"blackhole":  map[string]any{"communities": []string{"blackhole"}},
		}}}},
	}
	server, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, rctr)
	require.NoError(t, err)
	ctx := &pluginserver.CommandContext{Server: server}

	_, err = handleAnnounceBlackhole(ctx, rctr, newReg(), []string{"198.51.100.1/32"})

	require.NoError(t, err, "the group's agreement did not reach the member built from it")
	require.Len(t, rctr.batches, 1)
	assert.True(t, carriesBlackhole(t, rctr.batches[0]), "the advertised route does not carry 65535:666")
	assert.True(t, rctr.sels[0].Matches(member), "the member was not selected")
}

// The pair that makes the case above discriminate. The same group states the
// same agreement, and this session belongs to no group, so nothing answers for
// it.
//
// RFC requirement: RFC7999-3.1-2 negative -- a session that joined no group
// agreed to nothing, so the BLACKHOLE community MUST NOT be advertised to it.
func TestAnnounceBlackholeIsWithheldFromASessionOutsideTheGroup(t *testing.T) {
	stranger := netip.MustParseAddr("198.51.100.9")
	rctr := &agreementReactor{
		peers: []plugin.PeerInfo{{Address: stranger, Name: "stranger"}},
		tree: map[string]any{"bgp": map[string]any{"group": map[string]any{"ix": map[string]any{
			"connection": map[string]any{"remote": map[string]any{"ip": "dynamic", "range": []string{"192.0.2.0/24"}}},
			"blackhole":  map[string]any{"communities": []string{"blackhole"}},
		}}}},
	}
	server, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, rctr)
	require.NoError(t, err)
	ctx := &pluginserver.CommandContext{Server: server}

	_, err = handleAnnounceBlackhole(ctx, rctr, newReg(), []string{"198.51.100.1/32"})

	require.Error(t, err)
	assert.Empty(t, rctr.batches, "a session outside the group was advertised on the group's agreement")
}

// Sanity on the fixture: the well-known value the gate tests for is the one
// RFC 7999 registers, so a fixture drift cannot make every case pass.
func TestBlackholeCommunityValue(t *testing.T) {
	assert.Equal(t, uint32(0xFFFF029A), uint32(attribute.CommunityBlackhole))
}
