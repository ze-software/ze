// Design: docs/architecture/api/commands.md -- show policy test, its text-before and text-after
// Related: filter_format.go -- appendSingleAttr, the producer under test
// Related: policy_dryrun_display.go -- displayUpdate and displayAttrOrder
// Related: filter_chain.go -- policyAttr* names, the vocabulary asserted here

package reactor

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
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

// attrsFixtureSubject is the display every shown attribute renders into, in
// the order displayAttrOrder fixes. Each pair is written as the AppendText of
// its own type produces it (internal/core/bgp/attribute/text_append.go), so a
// value-shape change is a red here rather than a silent change to what show
// policy test prints.
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

// attrsFixtureWire builds one UPDATE carrying every attribute displayAttrOrder
// shows, on a session that negotiated four-octet AS numbers. It is the UPDATE
// the dry run is handed, not a hand-written display: the defect this file
// exists to catch survived for the life of the function precisely because
// every test read a string a human typed.
func attrsFixtureWire(t *testing.T) *wireu.WireUpdate {
	t.Helper()

	aggregator := make([]byte, 8)
	binary.BigEndian.PutUint32(aggregator, 65000)
	copy(aggregator[4:], []byte{4, 4, 4, 4})

	aigp := []byte{0x01, 0x00, 0x0B, 0, 0, 0, 0, 0, 0, 0, 42}

	largeCommunity := make([]byte, 12)
	binary.BigEndian.PutUint32(largeCommunity[0:], 65000)
	binary.BigEndian.PutUint32(largeCommunity[4:], 1)
	binary.BigEndian.PutUint32(largeCommunity[8:], 2)

	return as4FilterUpdate(t, true,
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

// TestFilterSubjectNamesEveryAdvertisedAttribute displays an UPDATE carrying
// all thirteen attributes displayAttrOrder shows.
//
// VALIDATES: AC-1 -- every attribute the display shows appears whenever the
// UPDATE carries that attribute, with the value shape its own AppendText
// produces and in the order displayAttrOrder fixes.
// PREVENTS: an attribute the display silently omits. appendSingleAttr type-
// switches on the boxed attribute, so an arm naming a type no parser builds can
// never match and drops the attribute in silence. The loop over
// displayAttrOrder is what makes a fourteenth entry visible here rather than
// silently absent.
func TestFilterSubjectNamesEveryAdvertisedAttribute(t *testing.T) {
	display, err := displayUpdate(attrsFixtureWire(t))
	require.NoError(t, err)

	for i, code := range displayAttrOrder {
		assert.NotEmpty(t, display.attrs[i],
			"the display must show every attribute displayAttrOrder names, missing code %d", code)
	}
	assert.Equal(t, attrsFixtureSubject, display.text())
}

// TestFilterSubjectNamesClusterList displays an UPDATE whose CLUSTER_LIST holds
// two identifiers that are not in ascending order.
//
// VALIDATES: AC-12 -- the display names cluster-list followed by dotted
// decimal identifiers in WIRE order, with no brackets, which is the shape
// RFC 4456 Section 8 puts on the wire.
// PREVENTS: a route reflector operator reasoning about the clusters a route
// already traversed being shown nothing at all, and a renderer that sorts or
// brackets the list changing which cluster the operator believes came first.
func TestFilterSubjectNamesClusterList(t *testing.T) {
	subject := as4FilterDisplay(t, as4FilterUpdate(t, true,
		as4FilterNextHop(),
		as4FilterAttr(attribute.FlagOptional, attribute.AttrClusterList,
			[]byte{9, 9, 9, 9, 1, 1, 1, 1}),
	))

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
// from live import, originated export and forwarded export. The dry-run runs
// no plugin filter outside a compiled chain (traceUncompiledChain). The plugin may modify LOCAL_PREF but decides from MED. Matching,
// nonmatching and absent MED must therefore produce different route outcomes.
// Typed-set and raw rewrites exercise both export materialization paths.
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

	for _, mode := range []string{"sets", "raw"} {
		policy := compiledPolicy(t, libraryTestMixed, routepolicy.FinalAccept, frefs("subject-policy:"+mode)...)
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
				destination.session.egressRouteFilter = func(body []byte) (bool, []byte, []byte) {
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
				f.r.orderedEgressSteps = buildOrderedEgressSteps()
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
		}
	}
}

// TestFilterSubjectDryRunWithdrawalFallback keeps a library chain member's MED
// decision visible in the trace even when final reject replaces a mixed UPDATE
// with its explicit withdrawal, and proves a raw rewrite does not get past
// that final.
//
// Method: a final-reject policy over the external member probe:f, answered by
// the externalTransport stand-in, which reads MED but may set only
// LOCAL_PREF. It answers a change only when it was sent med 100: a
// local-preference 231 set in the sets mode, the received body rewritten to
// LOCAL_PREF 231 in the raw mode, which its trace entry shows as a modify
// whose text-after is the rewritten body; otherwise it rejects. The pasted UPDATE
// withdraws 198.51.100.0/24 and announces 203.0.113.0/24 with med 100, run on
// import and on export.
func TestFilterSubjectDryRunWithdrawalFallback(t *testing.T) {
	withdrawal := []byte{24, 198, 51, 100}
	announcement, err := message.UnpackUpdate(filterSubjectBody(100))
	require.NoError(t, err)
	body := makeUpdateBody(withdrawal, announcement.PathAttributes, announcement.NLRI)
	for _, mode := range []string{"sets", "raw"} {
		for _, direction := range []string{directionImport, directionExport} {
			t.Run(mode+"/"+direction, func(t *testing.T) {
				transport := filterSubjectMEDTransport(t, mode)
				r := New(&Config{})
				r.filterTransportSeam = transport
				settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65001, 0x01010101)
				policy := externalFinalPolicy(t, routepolicy.FinalReject)
				settings.ImportPolicy = policy
				settings.ExportPolicy = policy
				r.peers[settings.PeerKey()] = NewPeer(settings)
				result, err := (&reactorAPIAdapter{r: r}).PolicyDryRun(
					settings.Address.String(), direction, "", body, true)
				require.NoError(t, err)
				require.Len(t, transport.sent, 1, "probe:f is asked once")
				require.Len(t, result.Trace, 1)
				if mode == "sets" {
					require.Equal(t, dryRunActionModify, result.Trace[0].Action,
						"the filter must see MED despite setting only LOCAL_PREF")
					require.Equal(t, "local-preference 231", result.Trace[0].Delta)
				}
				if mode == "raw" {
					require.Equal(t, dryRunActionModify, result.Trace[0].Action,
						"a member that rewrote the body modified the route")
					require.Contains(t, result.Trace[0].TextAfter, "local-preference 231",
						"the member's entry shows the body it rewrote")
				}
				require.Contains(t, result.TextBefore, "med 100")
				require.Equal(t, dryRunActionModify, result.Action)
				require.Equal(t, "nlri del 198.51.100.0/24", result.TextAfter,
					"final reject retains only the original explicit withdrawal")
			})
		}
	}
}

// filterSubjectMEDTransport answers probe:f for
// TestFilterSubjectDryRunWithdrawalFallback: it reads MED and, only when sent
// med 100, answers mode's LOCAL_PREF 231 change; any other route is rejected.
func filterSubjectMEDTransport(t *testing.T, mode string) *externalTransport {
	t.Helper()
	transport := &externalTransport{reads: []string{"med"}, sets: []string{"local-preference"}}
	if mode == "raw" {
		transport.reads = []string{rpc.FilterFieldRaw, "med"}
		transport.sets = nil
	}
	transport.answer = func(in rpc.FilterUpdateInput) rpc.FilterUpdateOutput {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(in.Fields, &fields); err != nil {
			t.Errorf("the filter's fields do not decode: %v (%s)", err, in.Fields)
			return rpc.FilterUpdateOutput{Action: rpc.FilterReject}
		}
		if string(fields["med"]) != "100" {
			return rpc.FilterUpdateOutput{Action: rpc.FilterReject}
		}
		if mode != "raw" {
			return rpc.FilterUpdateOutput{Action: rpc.FilterModify, Sets: []rpc.FilterSet{
				{Field: "local-preference", Action: rpc.FilterSetActionSet, Value: json.RawMessage("231")}}}
		}
		raw := bytes.Clone(in.Raw)
		sections, err := wire.ParseUpdateSections(raw)
		if err != nil {
			t.Errorf("the raw UPDATE does not parse: %v", err)
			return rpc.FilterUpdateOutput{Action: rpc.FilterReject}
		}
		_, _, localPref, found := attribute.AttrFind(sections.Attrs(raw), attribute.AttrLocalPref)
		if !found || len(localPref) != 4 {
			t.Error("the raw UPDATE carries no 4-octet LOCAL_PREF")
			return rpc.FilterUpdateOutput{Action: rpc.FilterReject}
		}
		binary.BigEndian.PutUint32(localPref, 231)
		return rpc.FilterUpdateOutput{Action: rpc.FilterModify, Raw: raw}
	}
	return transport
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
			p.OnFilter(func(route *sdk.FilterRoute) (*sdk.FilterUpdateOutput, error) {
				input, err := route.Input()
				if err != nil {
					return nil, err
				}
				var fields map[string]json.RawMessage
				if err := json.Unmarshal(input.Fields, &fields); err != nil {
					return nil, err
				}
				metric, present := fields[policyAttrMED]
				if !present {
					return &sdk.FilterUpdateOutput{Action: sdk.FilterReject}, nil
				}
				if string(metric) != "100" {
					return &sdk.FilterUpdateOutput{Action: sdk.FilterReject}, nil
				}
				if input.Filter == "sets" {
					return &sdk.FilterUpdateOutput{Action: sdk.FilterModify, Sets: []rpc.FilterSet{
						{Field: policyAttrLocalPreference, Action: rpc.FilterSetActionSet, Value: json.RawMessage("231")}}}, nil
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
				{Name: "sets", Direction: sdk.FilterBoth, Reads: []string{policyAttrMED}, Sets: []string{policyAttrLocalPreference}, OnError: sdk.OnErrorReject},
				{Name: "raw", Direction: sdk.FilterBoth, Reads: []string{sdk.FilterFieldRaw, policyAttrMED}, OnError: sdk.OnErrorReject},
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
