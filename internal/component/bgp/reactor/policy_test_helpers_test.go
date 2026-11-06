// Design: docs/architecture/core-design.md — policy filter chain tests
// Related: filter_answer_transport_test.go -- answerTransport, answerPolicy
//
// Policy and UPDATE-body helpers shared by the reactor's tests. They moved
// here from the text chain's test files (filter_chain, filter_failed_flag,
// filter_delta, filter_nlri_delta), which are deleted with the text format.
package reactor

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/policychain"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	routepolicy "github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// mustPolicy constructs an explicit test policy and reports invalid fixture input.
func mustPolicy(t testing.TB, refs []routepolicy.Ref, final routepolicy.Final) filterapi.Policy {
	t.Helper()
	policy, err := filterapi.NewPolicy(refs, final)
	require.NoError(t, err)
	return policy
}

// compiledPolicy is the policy bgp/config builds for refs that name plugin
// filters: each "<plugin>:<filter>" ref becomes an external member of a
// library chain compiled at libraryTestMixed under an accepting final
// (bgpconfig compileLibraryChain). mustPolicy alone builds a policy no chain
// runs, which the reactor fails closed on, so a test whose filters must run
// uses this.
func compiledPolicy(t testing.TB, refs ...routepolicy.Ref) filterapi.Policy {
	t.Helper()
	const final = routepolicy.FinalAccept
	reg, err := routepolicy.NewRegistry()
	require.NoError(t, err)
	members := make([]policychain.Member, len(refs))
	for i, ref := range refs {
		plugin, filter, ok := strings.Cut(ref.Name, ":")
		require.True(t, ok, "ref %q names no plugin", ref.Name)
		members[i] = policychain.Member{Name: filter, Plugin: plugin}
	}
	chain, err := policychain.Compile(libraryTestMixed, reg, members, final, policychain.Bases{LocalPreference: 100})
	require.NoError(t, err)
	return mustPolicy(t, refs, final).WithLibrary(chain)
}

// acceptingPolicies gives each of settings an explicit import and export policy
// with no filter and an accepting final, and returns settings. A live fixture's
// peers are EBGP, and RFC 8212 Section 3 rejects at an absent policy: "Routes
// SHALL NOT be added to an Adj-RIB-Out associated with an EBGP peer if no
// explicit Export Policy has been applied." So a test about forwarding,
// election or recovery states the accepting policy it relies on.
func acceptingPolicies(t testing.TB, settings ...*PeerSettings) []*PeerSettings {
	t.Helper()
	for _, s := range settings {
		s.ImportPolicy = mustPolicy(t, nil, routepolicy.FinalAccept)
		s.ExportPolicy = mustPolicy(t, nil, routepolicy.FinalAccept)
	}
	return settings
}

// acceptingLiveSettings is lowLiveSettings for an EBGP session whose test
// relies on its UPDATEs being accepted and forwarded: it carries the explicit
// accepting import and export policies RFC 8212 Section 3 requires.
func acceptingLiveSettings(t testing.TB, address string, local, remote uint32) *PeerSettings {
	t.Helper()
	return acceptingPolicies(t, lowLiveSettings(address, local, remote))[0]
}

// bindRefs marks every ref of p named like one of owned as bound to its
// registered stage: the ownership config resolution (bgpconfig
// canonicalizeFilterRefs) grants through loopdetect.BindLoopDetection, which
// these tests call to obtain owned.
func bindRefs(p filterapi.Policy, owned ...routepolicy.Ref) filterapi.Policy {
	refs := p.Filters()
	bound := make([]bool, len(refs))
	for i, ref := range refs {
		for _, o := range owned {
			if ref.Name == o.Name {
				bound[i] = true
			}
		}
	}
	return p.WithBoundFilters(refs, bound)
}

// frefs builds a policy.Ref chain from canonical strings, treating an
// "inactive:" prefix as the structural deactivation flag. It lets these tests
// keep expressing chains in the compact string form they were written against
// while exercising the []policy.Ref API.
func frefs(names ...string) []routepolicy.Ref {
	if len(names) == 0 {
		return nil
	}
	out := make([]routepolicy.Ref, len(names))
	for i, n := range names {
		if rest, ok := strings.CutPrefix(n, "inactive:"); ok {
			out[i] = routepolicy.Ref{Name: rest, Inactive: true}
		} else {
			out[i] = routepolicy.Ref{Name: n}
		}
	}
	return out
}

// fakeFilterTransport answers the three calls the policy filter's IPC body
// makes. It is the filterTransport seam, which exists because the production
// implementation is a concrete *pluginserver.Server whose filter answer arrives
// off a live plugin socket.
type fakeFilterTransport struct {
	onError rpc.OnErrorPolicy
	out     *rpc.FilterUpdateOutput
	err     error
}

// FilterReadvertise answers that the filter does not opt into the stale
// readvertise rail.
func (f *fakeFilterTransport) FilterReadvertise(_, _ string) (readvertise, ok bool) {
	return false, true
}

// FilterJudgesFinal answers that the filter does not run again over a rewritten body.
func (f *fakeFilterTransport) FilterJudgesFinal(_, _ string) (judgesFinal, ok bool) {
	return false, true
}

// exportFilterForBody runs body through the chain and the normalization of the
// session's export gate, for a test that holds no session: production reaches them
// through Peer.egressGate, which also stamps the write.
func (r *Reactor) exportFilterForBody(peer *Peer, body []byte) (suppress bool, override, follow []byte) {
	facts, pending := peer.exportGateFacts()
	return r.exportFilterFromFacts(peer, facts, pending, body)
}

// FilterDeclaration answers every filter as declared, reading the prefix and
// setting nothing, under onError: a library chain member with no declaration
// or no reads fails every route before its answer is read.
func (f *fakeFilterTransport) FilterDeclaration(_, _ string) ([]string, []string, rpc.OnErrorPolicy, bool) {
	return []string{"prefix"}, nil, f.onError, true
}

func (f *fakeFilterTransport) FilterReads(_, _ string) []string { return nil }

func (f *fakeFilterTransport) FilterOnError(_, _ string) rpc.OnErrorPolicy { return f.onError }

func (f *fakeFilterTransport) CallFilterUpdate(_ context.Context, _ string, _ *rpc.FilterRoute) (*rpc.FilterUpdateOutput, error) {
	return f.out, f.err
}

// liveFilterServer is the production filter transport: a real plugin server with
// no plugin processes, so CallFilterUpdate cannot find the named plugin and
// FilterOnError answers OnErrorReject for it. That is the daemon's own answer
// for a filter whose plugin died, not a stand-in for one.
func liveFilterServer(t *testing.T) *pluginserver.Server {
	t.Helper()
	srv, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, &reactorAPIAdapter{r: &Reactor{}})
	require.NoError(t, err)
	return srv
}

func nlriHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(strings.ReplaceAll(s, " ", ""))
	require.NoError(t, err)
	return b
}

func payloadASPathValue(t testing.TB, payload []byte) []byte {
	t.Helper()
	require.GreaterOrEqual(t, len(payload), 4)
	wdLen := int(binary.BigEndian.Uint16(payload[0:2]))
	attrLenOff := 2 + wdLen
	require.GreaterOrEqual(t, len(payload), attrLenOff+2)
	attrLen := int(binary.BigEndian.Uint16(payload[attrLenOff : attrLenOff+2]))
	off := attrLenOff + 2
	end := off + attrLen
	require.GreaterOrEqual(t, len(payload), end)
	for off < end {
		require.GreaterOrEqual(t, end, off+3)
		code := payload[off+1]
		hdrLen := 3
		valueLen := int(payload[off+2])
		if payload[off]&0x10 != 0 {
			require.GreaterOrEqual(t, end, off+4)
			hdrLen = 4
			valueLen = int(binary.BigEndian.Uint16(payload[off+2 : off+4]))
		}
		valueStart := off + hdrLen
		valueEnd := valueStart + valueLen
		require.GreaterOrEqual(t, end, valueEnd)
		if code == byte(attribute.AttrASPath) {
			return payload[valueStart:valueEnd]
		}
		off = valueEnd
	}
	t.Fatalf("AS_PATH not found")
	return nil
}

// prependFixture builds an AttributesWire and the matching UPDATE payload for
// an as-path-prepend test: ORIGIN, then AS_PATH holding asPathValue, then an
// AS4_PATH holding as4PathValue when one is given.
//
// The payload advertises 10.0.0.0/24, because buildModifiedPayload refuses to
// CREATE a path attribute on a body that advertises nothing (advertiseGate,
// RFC 4271 Sections 4.3 and 6.3), and creating AS4_PATH is what half these
// cases are about.
func prependFixture(asPathValue, as4PathValue []byte) (attrs *attribute.AttributesWire, payload []byte) {
	packed := makeAttr(0x40, byte(attribute.AttrOrigin), []byte{0x00})
	packed = append(packed, makeAttr(0x40, byte(attribute.AttrASPath), asPathValue)...)
	if as4PathValue != nil {
		packed = append(packed, makeAttr(0xC0, byte(attribute.AttrAS4Path), as4PathValue)...)
	}
	return attribute.NewAttributesWire(packed, 0), buildModTestPayload(packed, modTestNLRI)
}

// attrValueFromPayload returns the VALUE bytes of one attribute in a rebuilt
// UPDATE payload, so an assertion reads what a peer would parse rather than a
// decoded struct.
func attrValueFromPayload(t *testing.T, payload []byte, code attribute.AttributeCode) ([]byte, bool) {
	t.Helper()

	require.GreaterOrEqual(t, len(payload), 4, "payload holds a withdrawn length and an attribute length")
	wdLen := int(binary.BigEndian.Uint16(payload[0:2]))
	require.GreaterOrEqual(t, len(payload), 2+wdLen+2, "payload holds an attribute length")
	attrLen := int(binary.BigEndian.Uint16(payload[2+wdLen : 4+wdLen]))
	start := 4 + wdLen
	require.GreaterOrEqual(t, len(payload), start+attrLen, "payload holds its whole attribute section")

	section := payload[start : start+attrLen]
	for off := 0; off+3 <= len(section); {
		flags, attrCode := section[off], section[off+1]
		valOff, valLen := off+3, int(section[off+2])
		if flags&0x10 != 0 {
			require.GreaterOrEqual(t, len(section), off+4, "extended-length header fits")
			valOff, valLen = off+4, int(binary.BigEndian.Uint16(section[off+2:off+4]))
		}
		require.GreaterOrEqual(t, len(section), valOff+valLen, "attribute value fits its section")
		if attrCode == byte(code) {
			return section[valOff : valOff+valLen], true
		}
		off = valOff + valLen
	}
	return nil, false
}

// flatASNs returns every AS number of a decoded path, in path order. The policy
// prepend joins a leading AS_SEQUENCE but opens its own segment in front of an
// AS_SET or a full sequence, so this is the path a receiving peer computes
// whatever the segment layout, and Segments[0] alone can be only its leading
// half.
func flatASNs(path *attribute.ASPath) []uint32 {
	var out []uint32
	for _, seg := range path.Segments {
		out = append(out, seg.ASNs...)
	}
	return out
}
