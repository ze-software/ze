// Design: docs/architecture/api/process-protocol.md -- the filter text protocol
// Related: filter_format.go -- appendSingleAttr, appendAllAttrs, the producers under test
// Related: filter_chain.go -- policyAttr* names, the vocabulary asserted here

package reactor

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"log/slog"
	"net"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	routepolicy "github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/bgp/wire"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// attrsFixtureSubject is the subject every advertised attribute renders into,
// in the order appendAllAttrs fixes. Each pair is written as the AppendText of
// its own type produces it (internal/core/bgp/attribute/text_append.go), so a
// value-shape change is a red here rather than a silent contract change for
// every text-mode filter.
const attrsFixtureSubject = "origin incomplete " +
	"as-path [65001 65002] " +
	"next-hop 10.0.0.1 " +
	"med 100 " +
	"local-preference 150 " +
	"atomic-aggregate " +
	"aggregator 65000:4.4.4.4 " +
	"community 65000:100 " +
	"originator-id 3.3.3.3 " +
	"cluster-list 1.1.1.1 2.2.2.2 " +
	"extended-community 0002fde800000064 " +
	"aigp 42 " +
	"large-community 65000:1:2"

// attrsFixtureWire builds one AttributesWire carrying every attribute
// attrNameToCode advertises, on a session that negotiated four-octet AS
// numbers. It is the UPDATE a filter chain is handed, not a hand-written
// subject: the defect this file exists to catch survived for the life of the
// function precisely because every test read a string a human typed.
func attrsFixtureWire(t *testing.T) *attribute.AttributesWire {
	t.Helper()

	aggregator := make([]byte, 8)
	binary.BigEndian.PutUint32(aggregator, 65000)
	copy(aggregator[4:], []byte{4, 4, 4, 4})

	aigp := []byte{0x01, 0x00, 0x0B, 0, 0, 0, 0, 0, 0, 0, 42}

	largeCommunity := make([]byte, 12)
	binary.BigEndian.PutUint32(largeCommunity[0:], 65000)
	binary.BigEndian.PutUint32(largeCommunity[4:], 1)
	binary.BigEndian.PutUint32(largeCommunity[8:], 2)

	return as4FilterWire(t, true,
		as4FilterAttr(attribute.FlagTransitive, attribute.AttrOrigin, []byte{2}),
		as4FilterAttr(attribute.FlagTransitive, attribute.AttrASPath, as4FilterPath4(65001, 65002)),
		as4FilterAttr(attribute.FlagTransitive, attribute.AttrNextHop, []byte{10, 0, 0, 1}),
		as4FilterAttr(attribute.FlagOptional, attribute.AttrMED, []byte{0, 0, 0, 100}),
		as4FilterAttr(attribute.FlagTransitive, attribute.AttrLocalPref, []byte{0, 0, 0, 150}),
		as4FilterAttr(attribute.FlagTransitive, attribute.AttrAtomicAggregate, nil),
		as4FilterAttr(attribute.FlagOptional|attribute.FlagTransitive, attribute.AttrAggregator, aggregator),
		as4FilterAttr(attribute.FlagOptional|attribute.FlagTransitive, attribute.AttrCommunity,
			[]byte{0xFD, 0xE8, 0x00, 0x64}),
		as4FilterAttr(attribute.FlagOptional, attribute.AttrOriginatorID, []byte{3, 3, 3, 3}),
		as4FilterAttr(attribute.FlagOptional, attribute.AttrClusterList,
			[]byte{1, 1, 1, 1, 2, 2, 2, 2}),
		as4FilterAttr(attribute.FlagOptional|attribute.FlagTransitive, attribute.AttrExtCommunity,
			[]byte{0x00, 0x02, 0xFD, 0xE8, 0x00, 0x00, 0x00, 0x64}),
		as4FilterAttr(attribute.FlagOptional, attribute.AttrAIGP, aigp),
		as4FilterAttr(attribute.FlagOptional|attribute.FlagTransitive, attribute.AttrLargeCommunity,
			largeCommunity),
	)
}

// TestFilterSubjectNamesEveryAdvertisedAttribute renders an UPDATE carrying all
// thirteen attributes attrNameToCode advertises, through the arm every runtime
// caller reaches (an empty declared list).
//
// VALIDATES: AC-1 -- an attribute name Ze advertises appears in the subject
// whenever the UPDATE carries that attribute, with the value shape its own
// AppendText produces and in the order appendAllAttrs fixes.
// PREVENTS: an advertised name that reaches no filter. appendSingleAttr type-
// switches on the boxed attribute, so an arm naming a type no parser builds can
// never match and drops the attribute in silence. The loop over attrNameToCode
// is what makes a fourteenth name visible here rather than silently absent.
func TestFilterSubjectNamesEveryAdvertisedAttribute(t *testing.T) {
	subject := string(AppendAttrsForFilter(nil, attrsFixtureWire(t), nil))

	for name := range attrNameToCode {
		assert.Contains(t, subject, name+" ",
			"the subject must name every attribute attrNameToCode advertises")
	}
	assert.Equal(t, attrsFixtureSubject, subject)
}

// TestFilterSubjectDeclaredArmNamesTheSameFive renders the same UPDATE through
// the declared-attribute arm of AppendAttrsForFilter, naming the five the
// pointer arms dropped.
//
// VALIDATES: AC-2 -- both arms share one renderer, so a filter that declares
// its attributes reads the same values as a filter that takes them all.
// PREVENTS: a fix applied to appendAllAttrs alone. The two arms differ only in
// which codes they walk; the rendering is one function and must stay one.
func TestFilterSubjectDeclaredArmNamesTheSameFive(t *testing.T) {
	declared := []string{
		policyAttrOrigin,
		policyAttrMED,
		policyAttrLocalPreference,
		policyAttrAtomicAggregate,
		policyAttrClusterList,
	}

	subject := string(AppendAttrsForFilter(nil, attrsFixtureWire(t), declared))

	assert.Equal(t, "origin incomplete med 100 local-preference 150 "+
		"atomic-aggregate cluster-list 1.1.1.1 2.2.2.2", subject)
	for _, absent := range []string{policyAttrASPath, policyAttrNextHop, policyAttrCommunity} {
		assert.NotContains(t, subject, absent,
			"the declared arm must render the declared names and nothing else")
	}
}

// TestFilterSubjectNamesClusterList renders an UPDATE whose CLUSTER_LIST holds
// two identifiers that are not in ascending order.
//
// VALIDATES: AC-12 -- the subject names cluster-list followed by dotted decimal
// identifiers in WIRE order, with no brackets, which is the shape RFC 4456
// Section 8 puts on the wire and the shape parseFilterAttrs reads back.
// PREVENTS: a route reflector policy reasoning about the clusters a route
// already traversed being handed nothing at all, and a renderer that sorts or
// brackets the list changing which cluster the filter believes came first.
func TestFilterSubjectNamesClusterList(t *testing.T) {
	attrs := as4FilterWire(t, true,
		as4FilterNextHop(),
		as4FilterAttr(attribute.FlagOptional, attribute.AttrClusterList,
			[]byte{9, 9, 9, 9, 1, 1, 1, 1}),
	)

	subject := string(AppendAttrsForFilter(nil, attrs, nil))

	assert.Equal(t, "next-hop 10.0.0.1 cluster-list 9.9.9.9 1.1.1.1", subject)
}

// filterUnnamedAttr is an attribute whose concrete type no arm of
// appendSingleAttr names. Nothing in knownAttrParsers builds one today, which
// is the point: the switch is over an interface, so no compile-time
// exhaustiveness check can exist and only a default arm can report the miss.
type filterUnnamedAttr struct{}

func (filterUnnamedAttr) Code() attribute.AttributeCode   { return attribute.AttrOrigin }
func (filterUnnamedAttr) Flags() attribute.AttributeFlags { return attribute.FlagTransitive }
func (filterUnnamedAttr) Len() int                        { return 0 }
func (filterUnnamedAttr) WriteTo(_ []byte, _ int) int     { return 0 }
func (filterUnnamedAttr) WriteToWithContext(_ []byte, _ int, _, _ *bgpctx.EncodingContext) int {
	return 0
}

// TestAppendSingleAttrWarnsOnUnnamedType hands appendSingleAttr an attribute no
// switch arm names.
//
// VALIDATES: AC-3 -- nothing is appended, the first flag is untouched so the
// separator discipline survives, and one warning names the type.
// PREVENTS: the silence that let five arms name a type no parser builds for the
// whole life of the function. The chain still runs to a verdict on the subject
// it has, which is the degrade-and-speak shape asPathForFilter already uses in
// this file; what must not survive is reaching that verdict with nothing said.
func TestAppendSingleAttrWarnsOnUnnamedType(t *testing.T) {
	var sink bytes.Buffer
	prev := fwdLogger
	fwdLogger = func() *slog.Logger {
		return slog.New(slog.NewTextHandler(&sink, &slog.HandlerOptions{Level: slog.LevelDebug}))
	}
	t.Cleanup(func() { fwdLogger = prev })

	buf, first := appendSingleAttr([]byte("origin igp"), filterUnnamedAttr{}, false)

	assert.Equal(t, "origin igp", string(buf), "an unnamed type must not leave a dangling separator")
	assert.False(t, first, "the first flag reports what was written, and nothing was")
	assert.Contains(t, sink.String(), filterUnnamedAttrPhrase)
	assert.Contains(t, sink.String(), "filterUnnamedAttr", "the line must name the type it dropped")

	// The control: a named type says nothing at all, so the line above is a
	// signal rather than one more token on every UPDATE.
	sink.Reset()
	buf, first = appendSingleAttr(nil, attribute.MED(100), true)
	assert.Equal(t, "med 100", string(buf))
	assert.False(t, first)
	assert.Empty(t, sink.String())
}

// TestFilterSubjectReadsUnmodifiableAttributes drives native plugin dispatch
// from live import, originated export, forwarded export and both dry-run
// directions. The plugin may modify LOCAL_PREF but decides from MED. Matching,
// nonmatching and absent MED must therefore produce different route outcomes.
// Text and raw rewrites exercise both export materialization paths.
func TestFilterSubjectReadsUnmodifiableAttributes(t *testing.T) {
	source, _ := tePropagationPeer(t, "192.0.2.1", 65001, family.IPv4Unicast)
	r := &Reactor{
		peers: map[netip.AddrPort]*Peer{
			source.Settings().PeerKey(): source,
		},
		recentUpdates:       newRecentUpdateCache(16),
		attrModHandlers:     attrModHandlersWithDefaults(),
		orderedIngressSteps: []orderedIngressStep{{name: policyChainStepName, policyChain: true}},
	}
	t.Cleanup(r.recentUpdates.Stop)
	filterSubjectPlugin(t, r)

	for _, mode := range []string{"text", "raw"} {
		policy := mustPolicy(t, frefs("subject-policy:"+mode), routepolicy.FinalAccept)
		source.settings.ImportPolicy = policy
		source.settings.ExportPolicy = policy
		for _, metric := range []struct {
			name  string
			value int
		}{
			{name: "matching", value: 100},
			{name: "nonmatching", value: 101},
			{name: "absent", value: -1},
		} {
			body := filterSubjectBody(metric.value)
			accepted := metric.value == 100
			t.Run(mode+"/"+metric.name+"/import", func(t *testing.T) {
				var delivered [][]byte
				r.setMessageReceiver(&testDeliveryReceiver{consumerCount: 1,
					onReceived: func(_ plugin.PeerInfo, msg bgptypes.RawMessage) {
						delivered = append(delivered, bytes.Clone(msg.RawBytes))
					},
				})
				update := wireu.NewWireUpdate(body, source.recvCtxID)
				update.SetSourceID(source.SourceID())
				r.notifyMessageReceiver(source.Settings().Address, msgtype.TypeUPDATE, body, update,
					0, rpc.DirectionReceived, BufHandle{ID: noPoolBufID, Buf: body}, nil, "", 0)
				if !accepted {
					require.Empty(t, delivered, "a refused announcement must not reach the consumer")
					return
				}
				require.Len(t, delivered, 1)
				filterSubjectAssertAccepted(t, delivered[0])
			})
			t.Run(mode+"/"+metric.name+"/originated-export", func(t *testing.T) {
				// Each case starts before its route has been advertised. Sharing
				// a Session would let final-wire dedup suppress the raw case's
				// identical output after the text case already sent it.
				destination, socket := tePropagationPeer(t, "192.0.2.2", 65001, family.IPv4Unicast)
				destination.settings.ExportPolicy = policy
				destination.refreshForwardFacts()
				destination.session.egressRouteFilter = func(body []byte) (bool, []byte) {
					return r.exportFilterForBody(destination, body)
				}
				update, err := message.UnpackUpdate(body)
				require.NoError(t, err)
				require.NoError(t, destination.session.SendUpdate(update))
				written := socket.written()
				if !accepted {
					require.Empty(t, written, "a refused announcement must not reach the socket")
					return
				}
				frames := bgpFrames(t, written)
				require.Len(t, frames, 1)
				filterSubjectAssertAccepted(t, frames[0][message.HeaderLen:])
			})
			t.Run(mode+"/"+metric.name+"/forwarded-export", func(t *testing.T) {
				f := newAIGPReplayFixture(t, nil)
				f.r.api = r.api
				f.r.orderedEgressSteps = []orderedEgressStep{{name: policyChainStepName, policyChain: true}}
				f.destination.settings.ExportPolicy = policy
				f.destination.settings.PeerAS = f.destination.settings.LocalAS
				f.destination.settings.NextHopMode = NextHopUnchanged
				f.destination.refreshForwardFacts()
				// A policy reject withdraws only a route the destination was
				// advertised (RFC 4271 Section 3.1).
				f.advertiseFirst(t, body)
				id := f.receive(t, body)
				f.forward(t, id)
				f.drain(t)
				bodies := aigpSocketBodies(t, f.conn)
				require.Len(t, bodies, 1, "the managed writer must emit the policy's route outcome")
				if accepted {
					filterSubjectAssertAccepted(t, bodies[0])
					return
				}
				update, err := message.UnpackUpdate(bodies[0])
				require.NoError(t, err)
				require.Empty(t, update.NLRI, "the rejected announcement must not escape")
				require.Equal(t, []byte{24, 203, 0, 113}, update.WithdrawnRoutes)
				require.Empty(t, update.PathAttributes, "a policy withdrawal must not retain announcement attributes")
			})
			for _, direction := range []string{directionImport, directionExport} {
				t.Run(mode+"/"+metric.name+"/dry-run-"+direction, func(t *testing.T) {
					result, err := (&reactorAPIAdapter{r: r}).PolicyDryRun(
						source.Settings().Address.String(), direction, "", body, true)
					require.NoError(t, err)
					require.Len(t, result.Trace, 1)
					if !accepted {
						require.Equal(t, dryRunActionReject, result.Action)
						require.Equal(t, dryRunActionReject, result.Trace[0].Action)
						require.Empty(t, result.TextAfter)
						return
					}
					require.Equal(t, dryRunActionModify, result.Action)
					require.Equal(t, dryRunActionModify, result.Trace[0].Action)
					require.Contains(t, result.TextBefore, "med 100")
					if mode == "text" {
						require.Contains(t, result.TextAfter, "local-preference 231")
						require.Equal(t, []string{policyAttrLocalPreference}, result.ChangedAttrs)
					}
				})
			}
		}
	}
}

// TestFilterSubjectDryRunWithdrawalFallback keeps the real native filter's MED
// decision visible in the trace even when final reject replaces a mixed UPDATE
// with its explicit withdrawal. A raw rewrite must not bypass that final.
func TestFilterSubjectDryRunWithdrawalFallback(t *testing.T) {
	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65001, 0x01010101)
	peer := NewPeer(settings)
	r := &Reactor{peers: map[netip.AddrPort]*Peer{settings.PeerKey(): peer}}
	filterSubjectPlugin(t, r)
	withdrawal := []byte{24, 198, 51, 100}
	for _, mode := range []string{"text", "raw"} {
		policy := mustPolicy(t, frefs("subject-policy:"+mode), routepolicy.FinalReject)
		peer.settings.ImportPolicy = policy
		peer.settings.ExportPolicy = policy
		for _, direction := range []string{directionImport, directionExport} {
			t.Run(mode+"/"+direction, func(t *testing.T) {
				announcement, err := message.UnpackUpdate(filterSubjectBody(100))
				require.NoError(t, err)
				body := makeUpdateBody(withdrawal, announcement.PathAttributes, announcement.NLRI)
				result, err := (&reactorAPIAdapter{r: r}).PolicyDryRun(
					settings.Address.String(), direction, "", body, true)
				require.NoError(t, err)
				require.Len(t, result.Trace, 1)
				require.Equal(t, dryRunActionModify, result.Trace[0].Action,
					"the filter must see MED despite declaring only LOCAL_PREF")
				require.Contains(t, result.TextBefore, "med 100")
				require.Equal(t, dryRunActionModify, result.Action)
				require.Equal(t, "nlri del 198.51.100.0/24", result.TextAfter,
					"final reject retains only the original explicit withdrawal")
			})
		}
	}
}

// filterSubjectPlugin starts a native SDK filter, including its real declaration
// and direct dispatch. It changes LOCAL_PREF only after matching undeclared MED;
// it never echoes a precomputed response or substitutes the policy executor.
func filterSubjectPlugin(t *testing.T, r *Reactor) {
	t.Helper()
	const name = "subject-policy"
	snapshot := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snapshot) })
	require.NoError(t, registry.Register(registry.Registration{
		Name: name, Description: "Attribute subject decision fixture",
		RunEngine: func(conn net.Conn) int {
			p := sdk.NewWithConn(name, conn)
			defer p.Close() //nolint:errcheck // The server owns shutdown.
			p.OnFilterUpdate(func(input *sdk.FilterUpdateInput) (*sdk.FilterUpdateOutput, error) {
				metric, present := parseFilterAttrs(input.Update).get(faMED)
				if !present {
					return &sdk.FilterUpdateOutput{Action: sdk.FilterReject}, nil
				}
				if metric != "100" {
					return &sdk.FilterUpdateOutput{Action: sdk.FilterReject}, nil
				}
				if input.Filter == "text" {
					return &sdk.FilterUpdateOutput{Action: sdk.FilterModify, Update: "local-preference 231"}, nil
				}
				body := bytes.Clone(input.Raw)
				sections, err := wire.ParseUpdateSections(body)
				if err != nil {
					return nil, err
				}
				_, _, localPref, found := attribute.AttrFind(sections.Attrs(body), attribute.AttrLocalPref)
				if !found {
					return nil, errors.New("subject-policy: raw UPDATE has no LOCAL_PREF")
				}
				if len(localPref) != 4 {
					return nil, errors.New("subject-policy: raw UPDATE has malformed LOCAL_PREF")
				}
				binary.BigEndian.PutUint32(localPref, 231)
				return &sdk.FilterUpdateOutput{Action: sdk.FilterModify, Raw: body}, nil
			})
			if err := p.Run(context.Background(), &sdk.Registration{Filters: []sdk.FilterDecl{
				{Name: "text", Direction: sdk.FilterBoth, Attributes: []string{policyAttrLocalPreference}, OnError: sdk.OnErrorReject},
				{Name: "raw", Direction: sdk.FilterBoth, Attributes: []string{policyAttrLocalPreference}, Raw: true, OnError: sdk.OnErrorReject},
			}}); err != nil {
				return 1
			}
			return 0
		},
		CLIHandler: func([]string) int { return 0 },
	}))
	policyFinalPluginServer(t, r, name)
}

// filterSubjectBody encodes a real route with an optional MED, never filter text.
func filterSubjectBody(metric int) []byte {
	attrs := makeAttr(0x40, byte(attribute.AttrOrigin), []byte{0})
	attrs = append(attrs, makeAttr(0x40, byte(attribute.AttrASPath), as4FilterPath4(64496))...)
	attrs = append(attrs, makeAttr(0x40, byte(attribute.AttrNextHop), []byte{192, 0, 2, 1})...)
	attrs = append(attrs, makeAttr(0x40, byte(attribute.AttrLocalPref), []byte{0, 0, 0, 150})...)
	if metric >= 0 {
		var value [4]byte
		binary.BigEndian.PutUint32(value[:], uint32(metric)) //nolint:gosec // Bounded fixture metrics.
		attrs = append(attrs, makeAttr(0x80, byte(attribute.AttrMED), value[:])...)
	}
	return makeUpdateBody(nil, attrs, []byte{24, 203, 0, 113})
}

// filterSubjectAssertAccepted judges the consumer's wire route, including the
// declared modification and the untouched attribute that decided admission.
func filterSubjectAssertAccepted(t *testing.T, body []byte) {
	t.Helper()
	update, err := message.UnpackUpdate(body)
	require.NoError(t, err)
	require.Equal(t, []byte{24, 203, 0, 113}, update.NLRI)
	require.Empty(t, update.WithdrawnRoutes)
	_, _, localPref, present := attribute.AttrFind(update.PathAttributes, attribute.AttrLocalPref)
	require.True(t, present)
	require.Equal(t, []byte{0, 0, 0, 231}, localPref)
	_, _, metric, present := attribute.AttrFind(update.PathAttributes, attribute.AttrMED)
	require.True(t, present)
	require.Equal(t, []byte{0, 0, 0, 100}, metric)
}
