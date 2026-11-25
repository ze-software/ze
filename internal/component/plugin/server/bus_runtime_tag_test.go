// Design: docs/architecture/api/architecture.md -- runtime non-direction event tag filtering on emit-event and BroadcastEvent/EmitEngineEvent paths.
// Related: bus_runtime_tag.go -- runtime tag extraction and inbound tag filtering.
// Related: bus_op_gate.go -- filterReceiversByCompiledSender runtime enforcement.
// Related: dispatch.go -- deliverEventCaptured delivery pipeline.

package server

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

const (
	rtEventRouteChange   = "route-change"
	rtTagSource          = "source"
	rtTagScope           = "scope"
	rtValProto           = "proto"
	rtValStatic          = "static"
	rtValGlobal          = "global"
	rtSenderName         = "rt-tag-sender"
	rtFilteredName       = "rt-tag-filtered"
	rtMultiFilteredName  = "rt-tag-multi"
	rtUnfilteredName     = "rt-tag-unfiltered"
	rtDetailField        = "detail"
	rtDetailFromProto    = "from-proto"
	rtDetailFromStatic   = "from-static"
	rtDetailNestedTags   = "from-nested-tags"
	rtDetailMissingTag   = "missing-source"
	rtDetailUnknownValue = "unknown-source"
)

// TestEmitEventRuntimeTagFilter verifies that live emit-event RPC and
// deliverEvent calls enforce compiled non-direction tag filters on receivers
// while unfiltered receivers keep receiving every matching and non-matching
// event, and that unknown payload tag values never grow the TagInterner.
//
// VALIDATES: AC-44, S41 (runtime non-direction event tag matching on emit-event).
// PREVENTS: filterReceiversByCompiledSender ignoring non-direction tag filters,
// dropping events on unfiltered receivers, or interning untrusted payload strings.
func TestEmitEventRuntimeTagFilter(t *testing.T) {
	registerTestBusMessages(t)
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)

	s := &Server{
		subscriptions:     newSubscriptionManager(),
		engineSubscribers: newEngineEventSubscribers(),
		dispatcher:        NewDispatcher(),
	}
	s.ctx, s.cancel = context.WithCancel(t.Context())
	t.Cleanup(s.cancel)

	nsID := events.LookupNamespaceID(testNSProto)
	etID := events.LookupEventTypeID(rtEventRouteChange)

	filteredProc, filteredBatches := newDeliveryProbe(t, rtFilteredName)
	t.Cleanup(func() { s.subscriptions.clearProcess(filteredProc) })
	s.subscriptions.Add(filteredProc, &Subscription{
		Namespace: nsID,
		EventType: etID,
		Direction: events.DirBoth,
	})

	multiProc, multiBatches := newDeliveryProbe(t, rtMultiFilteredName)
	t.Cleanup(func() { s.subscriptions.clearProcess(multiProc) })
	s.subscriptions.Add(multiProc, &Subscription{
		Namespace: nsID,
		EventType: etID,
		Direction: events.DirBoth,
	})

	unfilteredProc, unfilteredBatches := newDeliveryProbe(t, rtUnfilteredName)
	t.Cleanup(func() { s.subscriptions.clearProcess(unfilteredProc) })
	s.subscriptions.Add(unfilteredProc, &Subscription{
		Namespace: nsID,
		EventType: etID,
		Direction: events.DirBoth,
	})

	sender := process.NewProcess(plugin.PluginConfig{Name: rtSenderName})
	sender.SetRegistration(&plugin.PluginRegistration{
		Sends: []rpc.MessageSendDecl{{Message: testMsgRouteChange}},
	})

	err := s.UpdateDeliveryGraphWithEndpoints("", nil, []BusParticipantInput{
		{
			Path:         plugin.EndpointPath(rtSenderName),
			EndpointName: rtSenderName,
			Sends:        []rpc.MessageSendDecl{{Message: testMsgRouteChange}},
		},
		{
			Path:         plugin.EndpointPath(rtFilteredName),
			EndpointName: rtFilteredName,
			Receives:     []rpc.MessageReceiveDecl{{Message: testMsgRouteChange}},
			OwnBlock: plugin.BusBlock{
				Receive: plugin.BusDirectionBlock{
					Messages: map[string]*plugin.BusMessageRule{
						testMsgRouteChange: {
							Tags: map[string][]string{
								rtTagSource: {rtValProto},
							},
						},
					},
				},
			},
		},
		{
			Path:         plugin.EndpointPath(rtMultiFilteredName),
			EndpointName: rtMultiFilteredName,
			Receives:     []rpc.MessageReceiveDecl{{Message: testMsgRouteChange}},
			OwnBlock: plugin.BusBlock{
				Receive: plugin.BusDirectionBlock{
					Messages: map[string]*plugin.BusMessageRule{
						testMsgRouteChange: {
							Tags: map[string][]string{
								rtTagSource: {rtValProto},
								rtTagScope:  {rtValGlobal},
							},
						},
					},
				},
			},
		},
		{
			Path:         plugin.EndpointPath(rtUnfilteredName),
			EndpointName: rtUnfilteredName,
			Receives:     []rpc.MessageReceiveDecl{{Message: testMsgRouteChange}},
		},
	})
	require.NoError(t, err)

	compiled := s.DeliveryGraph().Compiled()
	require.NotNil(t, compiled)
	require.Nil(t, compiled.Filter(plugin.EndpointPath(rtUnfilteredName)).ReceiveRules[testMsgRouteChange].InboundTags)

	emitRPC := func(eventJSON string) int {
		t.Helper()
		raw, mErr := json.Marshal(rpc.EmitEventInput{
			Namespace: testNSProto,
			EventType: rtEventRouteChange,
			Event:     eventJSON,
		})
		require.NoError(t, mErr)
		out, eErr := s.emitEvent(sender, raw)
		require.NoError(t, eErr)
		return out.Delivered
	}

	// 1. Non-matching source=static: only unfiltered receives it; filtered and
	// multi-filtered drop it.
	assert.Equal(t, 1, emitRPC(`{"source":"static","detail":"from-static"}`))
	gotUnfiltered := waitBatch(t, unfilteredBatches)
	require.Len(t, gotUnfiltered, 1)
	assert.Contains(t, gotUnfiltered[0], rtDetailFromStatic)

	// 2. Matching source=proto without scope: filtered and unfiltered receive
	// it; multi-filtered (requiring source=proto AND scope=global) drops it.
	assert.Equal(t, 2, emitRPC(`{"source":"proto","detail":"from-proto"}`))
	gotFiltered := waitBatch(t, filteredBatches)
	require.Len(t, gotFiltered, 1)
	assert.Contains(t, gotFiltered[0], rtDetailFromProto)
	assert.NotContains(t, gotFiltered[0], rtDetailFromStatic)

	gotUnfiltered = waitBatch(t, unfilteredBatches)
	require.Len(t, gotUnfiltered, 1)
	assert.Contains(t, gotUnfiltered[0], rtDetailFromProto)

	// 3. Nested "tags" object carrying both source=proto and scope=global: all
	// three receivers match.
	assert.Equal(t, 3, emitRPC(`{"tags":{"source":"proto","scope":"global"},"detail":"from-nested-tags"}`))
	gotFiltered = waitBatch(t, filteredBatches)
	require.Len(t, gotFiltered, 1)
	assert.Contains(t, gotFiltered[0], rtDetailNestedTags)

	gotMulti := waitBatch(t, multiBatches)
	require.Len(t, gotMulti, 1)
	assert.Contains(t, gotMulti[0], rtDetailNestedTags)

	gotUnfiltered = waitBatch(t, unfilteredBatches)
	require.Len(t, gotUnfiltered, 1)
	assert.Contains(t, gotUnfiltered[0], rtDetailNestedTags)

	// 4. Missing tag field drops for filtered receivers and delivers to unfiltered.
	assert.Equal(t, 1, emitRPC(`{"detail":"missing-source"}`))
	gotUnfiltered = waitBatch(t, unfilteredBatches)
	require.Len(t, gotUnfiltered, 1)
	assert.Contains(t, gotUnfiltered[0], rtDetailMissingTag)

	// 5. Untrusted unknown tag value drops for filtered receivers, delivers to
	// unfiltered, and MUST NOT intern the unknown value into TagInterner.
	const untrustedVal = "untrusted-never-interned-source"
	assert.Equal(t, 1, emitRPC(`{"source":"`+untrustedVal+`","detail":"unknown-source"}`))
	gotUnfiltered = waitBatch(t, unfilteredBatches)
	require.Len(t, gotUnfiltered, 1)
	assert.Contains(t, gotUnfiltered[0], rtDetailUnknownValue)
	_, interned := compiled.Interner().LookupValue(untrustedVal)
	assert.False(t, interned, "runtime tag extraction must not intern unknown untrusted values")

	// Confirm filtered probes still have zero buffered non-matching deliveries.
	assert.Empty(t, filteredBatches)
	assert.Empty(t, multiBatches)
}

type rtRouteChangeStructPayload struct {
	Source string `json:"source"`
	Detail string `json:"detail"`
}

// TestEmitEngineEventRuntimeTagFilter verifies that engine-emitted events
// (EmitEngineEvent / BroadcastEvent with nil emitter) enforce compiled
// non-direction InboundTags filters across JSON string, map, and struct
// payloads while delivering all events to unfiltered receivers.
//
// VALIDATES: AC-44, S41 (engine-emitted event tag filtering).
// PREVENTS: EmitEngineEvent bypassing non-direction receive tag filters.
func TestEmitEngineEventRuntimeTagFilter(t *testing.T) {
	registerTestBusMessages(t)
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)

	s := &Server{
		subscriptions:     newSubscriptionManager(),
		engineSubscribers: newEngineEventSubscribers(),
		dispatcher:        NewDispatcher(),
	}
	s.ctx, s.cancel = context.WithCancel(t.Context())
	t.Cleanup(s.cancel)

	nsID := events.LookupNamespaceID(testNSProto)
	etID := events.LookupEventTypeID(rtEventRouteChange)

	filteredProc, filteredBatches := newDeliveryProbe(t, rtFilteredName)
	t.Cleanup(func() { s.subscriptions.clearProcess(filteredProc) })
	s.subscriptions.Add(filteredProc, &Subscription{
		Namespace: nsID,
		EventType: etID,
		Direction: events.DirBoth,
	})

	unfilteredProc, unfilteredBatches := newDeliveryProbe(t, rtUnfilteredName)
	t.Cleanup(func() { s.subscriptions.clearProcess(unfilteredProc) })
	s.subscriptions.Add(unfilteredProc, &Subscription{
		Namespace: nsID,
		EventType: etID,
		Direction: events.DirBoth,
	})

	err := s.UpdateDeliveryGraphWithEndpoints("", nil, []BusParticipantInput{
		{
			Path:         plugin.EndpointPath(rtFilteredName),
			EndpointName: rtFilteredName,
			Receives:     []rpc.MessageReceiveDecl{{Message: testMsgRouteChange}},
			OwnBlock: plugin.BusBlock{
				Receive: plugin.BusDirectionBlock{
					Messages: map[string]*plugin.BusMessageRule{
						testMsgRouteChange: {
							Tags: map[string][]string{
								rtTagSource: {rtValProto},
							},
						},
					},
				},
			},
		},
		{
			Path:         plugin.EndpointPath(rtUnfilteredName),
			EndpointName: rtUnfilteredName,
			Receives:     []rpc.MessageReceiveDecl{{Message: testMsgRouteChange}},
		},
	})
	require.NoError(t, err)

	// Map payload with source=static -> only unfiltered receives it.
	delivered, err := s.EmitEngineEvent(testNSProto, rtEventRouteChange, map[string]any{
		rtTagSource:   rtValStatic,
		rtDetailField: rtDetailFromStatic,
	})
	require.NoError(t, err)
	assert.Equal(t, 1, delivered)
	gotUnfiltered := waitBatch(t, unfilteredBatches)
	require.Len(t, gotUnfiltered, 1)
	assert.Contains(t, gotUnfiltered[0], rtDetailFromStatic)

	// Struct payload with source=proto -> both filtered and unfiltered receive it.
	delivered, err = s.EmitEngineEvent(testNSProto, rtEventRouteChange, rtRouteChangeStructPayload{
		Source: rtValProto,
		Detail: rtDetailFromProto,
	})
	require.NoError(t, err)
	assert.Equal(t, 2, delivered)

	gotFiltered := waitBatch(t, filteredBatches)
	require.Len(t, gotFiltered, 1)
	assert.Contains(t, gotFiltered[0], rtDetailFromProto)

	gotUnfiltered = waitBatch(t, unfilteredBatches)
	require.Len(t, gotUnfiltered, 1)
	assert.Contains(t, gotUnfiltered[0], rtDetailFromProto)
	assert.Empty(t, filteredBatches)
}

// TestRuntimeTagFilterZeroOverheadWhenUnfiltered verifies that when no receiver
// carries a non-direction tag filter on an edge, both
// filterReceiversByCompiledSender and filterReceiversByInboundTags allocate
// nothing and skip tag extraction.
//
// VALIDATES: AC-44 zero-overhead guarantee for unfiltered receivers.
// PREVENTS: Per-event JSON parsing or heap allocation on unfiltered edges.
func TestRuntimeTagFilterZeroOverheadWhenUnfiltered(t *testing.T) {
	registerTestBusMessages(t)
	nsID := events.LookupNamespaceID(testNSProto)
	etID := events.LookupEventTypeID(rtEventRouteChange)

	bus, err := CompileBus(NewTagInterner(), []BusParticipantInput{
		{
			Path:         plugin.EndpointPath(rtSenderName),
			EndpointName: rtSenderName,
			Sends:        []rpc.MessageSendDecl{{Message: testMsgRouteChange}},
		},
		{
			Path:         plugin.EndpointPath(rtUnfilteredName),
			EndpointName: rtUnfilteredName,
			Receives:     []rpc.MessageReceiveDecl{{Message: testMsgRouteChange}},
		},
	})
	require.NoError(t, err)
	require.Empty(t, bus.deliveryFilterTags(plugin.EndpointPath(rtSenderName), nsID, etID, events.DirUnspecified))
	require.Empty(t, bus.inboundTagFilterFor(nsID, etID))

	proc := process.NewProcess(plugin.PluginConfig{Name: rtUnfilteredName})
	senderPath := plugin.EndpointPath(rtSenderName)
	payload := `{"source":"proto","detail":"zero-alloc"}`
	candidates := []*process.Process{proc}

	senderAllocs := testing.AllocsPerRun(500, func() {
		candidates[0] = proc
		kept := filterReceiversByCompiledSender(bus, senderPath, nsID, etID, events.DirUnspecified, payload, candidates)
		if len(kept) != 1 {
			t.Fatalf("kept = %d, want 1", len(kept))
		}
	})
	assert.Zero(t, senderAllocs, "unfiltered sender edge must perform zero allocations")

	inboundAllocs := testing.AllocsPerRun(500, func() {
		candidates[0] = proc
		kept := filterReceiversByInboundTags(bus, nsID, etID, payload, candidates)
		if len(kept) != 1 {
			t.Fatalf("kept = %d, want 1", len(kept))
		}
	})
	assert.Zero(t, inboundAllocs, "unfiltered inbound check must perform zero allocations")
}
