// Design: docs/architecture/api/process-protocol.md -- bounded event frames.
// Related: rib_event_sdk.go -- forked whole-family and replay event emission.
package rib

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/replay"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// TestSDKEventBusWholeFamilyFrames reads both the SDK emit request and the
// enveloped deliver-batch with real framing. Both replay payload shapes must
// deliver all routes exactly once, ordered, with their original replay identity.
func TestSDKEventBusWholeFamilyFrames(t *testing.T) {
	t.Run("best-change", func(t *testing.T) {
		batch := &ribevents.BestChangeBatch{Protocol: "bgp", Family: family.IPv4Unicast,
			ReplayID: replay.Broadcast, Changes: make([]ribevents.BestChangeEntry, 400)}
		attributes := bytes.Repeat([]byte{0xab}, 65535)
		for i := range batch.Changes {
			batch.Changes[i] = ribevents.BestChangeEntry{Action: routeaction.Add,
				Prefix: burstPrefix(i), NextHop: netip.MustParseAddr("192.0.2.1"),
				Metric: uint32(i), Attributes: attributes}
		}
		seen := 0
		frames := 0
		bus := burstSDKBus(t, func(input rpc.EmitEventInput) error {
			payload, err := burstDeliveredPayload(input)
			if err != nil {
				return err
			}
			var got ribevents.BestChangeBatch
			if err := json.Unmarshal(payload, &got); err != nil {
				return err
			}
			if got.Protocol != batch.Protocol || got.Family != batch.Family || got.ReplayID != batch.ReplayID {
				return errors.New("best-change chunk changed replay metadata")
			}
			for i := range got.Changes {
				entry := &got.Changes[i]
				if seen >= len(batch.Changes) || entry.Prefix != burstPrefix(seen) || entry.Metric != uint32(seen) || !bytes.Equal(entry.Attributes, attributes) {
					return fmt.Errorf("best-change route %d lost, duplicated or reordered", seen)
				}
				seen++
			}
			frames++
			return nil
		})
		if _, err := bus.Emit(ribevents.Namespace, ribevents.EventBestChange, batch); err != nil {
			t.Fatalf("whole-family best-change failed: %v", err)
		}
		if seen != len(batch.Changes) || frames < 2 {
			t.Fatalf("delivered %d/%d changes in %d frames", seen, len(batch.Changes), frames)
		}
	})
	t.Run("redistribution", func(t *testing.T) {
		batch := &redistevents.RouteChangeBatch{Protocol: 1, AFI: 1, SAFI: 1,
			ReplayID: 12345, OriginASN: 65000, Community: []uint32{6553700},
			Entries: make([]redistevents.RouteChangeEntry, 170000)}
		for i := range batch.Entries {
			batch.Entries[i] = redistevents.RouteChangeEntry{Action: redistevents.ActionAdd,
				Prefix: burstPrefix(i), NextHop: netip.MustParseAddr("192.0.2.1"), Metric: uint32(i)}
		}
		seen := 0
		frames := 0
		bus := burstSDKBus(t, func(input rpc.EmitEventInput) error {
			payload, err := burstDeliveredPayload(input)
			if err != nil {
				return err
			}
			var got redistevents.RouteChangeBatch
			if err := json.Unmarshal(payload, &got); err != nil {
				return err
			}
			if got.Protocol != batch.Protocol || got.AFI != batch.AFI || got.SAFI != batch.SAFI || got.ReplayID != batch.ReplayID || got.OriginASN != batch.OriginASN || len(got.Community) != 1 || got.Community[0] != batch.Community[0] {
				return errors.New("redistribution chunk changed replay metadata")
			}
			for _, entry := range got.Entries {
				if seen >= len(batch.Entries) || entry.Prefix != burstPrefix(seen) || entry.Metric != uint32(seen) {
					return fmt.Errorf("redistribution route %d lost, duplicated or reordered", seen)
				}
				seen++
			}
			frames++
			return nil
		})
		if _, err := bus.Emit("bgp", redistevents.EventType, batch); err != nil {
			t.Fatalf("whole-family redistribution failed: %v", err)
		}
		if seen != len(batch.Entries) || frames < 2 {
			t.Fatalf("delivered %d/%d routes in %d frames", seen, len(batch.Entries), frames)
		}
	})
}

// TestSDKEventBusChunkFailureStops proves a rejected chunk ends the bounded
// hand-off: acknowledged chunks are not retried and later chunks are not sent.
func TestSDKEventBusChunkFailureStops(t *testing.T) {
	batch := &ribevents.BestChangeBatch{Protocol: "bgp", Family: family.IPv4Unicast,
		Changes: make([]ribevents.BestChangeEntry, 800)}
	attributes := bytes.Repeat([]byte{0xab}, 65535)
	for i := range batch.Changes {
		batch.Changes[i] = ribevents.BestChangeEntry{Action: routeaction.Add,
			Prefix: burstPrefix(i), Attributes: attributes}
	}
	calls := 0
	bus := burstSDKBus(t, func(rpc.EmitEventInput) error {
		calls++
		if calls == 2 {
			return errors.New("refused second chunk")
		}
		if calls > 2 {
			return errors.New("sent after a rejected chunk")
		}
		return nil
	})
	if _, err := bus.Emit(ribevents.Namespace, ribevents.EventBestChange, batch); err == nil {
		t.Fatal("a rejected chunk reported success")
	}
	if calls != 2 {
		t.Fatalf("made %d exchanges, want one acknowledged then one rejected", calls)
	}
}

func burstPrefix(index int) netip.Prefix {
	return netip.PrefixFrom(netip.AddrFrom4([4]byte{10, byte(index >> 16), byte(index >> 8), byte(index)}), 32)
}

// burstDeliveredPayload traverses the consumer's enveloped deliver-batch frame,
// so a budget that measures only the emitter's request fails this same test.
func burstDeliveredPayload(input rpc.EmitEventInput) (json.RawMessage, error) {
	envelope, err := burstDeliveredEnvelope(input)
	return envelope.Payload, err
}

func burstDeliveredEnvelope(input rpc.EmitEventInput) (rpc.EventEnvelope, error) {
	envelope, err := json.Marshal(rpc.EventEnvelope{Namespace: input.Namespace,
		Event: input.EventType, Payload: json.RawMessage(input.Event), Metadata: nil})
	if err != nil {
		return rpc.EventEnvelope{}, err
	}
	quoted, err := json.Marshal(string(envelope))
	if err != nil {
		return rpc.EventEnvelope{}, err
	}
	var wire bytes.Buffer
	if err := rpc.WriteBatchFrame(&wire, ^uint64(0), [][]byte{quoted}); err != nil {
		return rpc.EventEnvelope{}, err
	}
	line, err := rpc.NewFrameReader(&wire).Read()
	if err != nil {
		return rpc.EventEnvelope{}, err
	}
	_, _, params, err := rpc.ParseLine(line)
	if err != nil {
		return rpc.EventEnvelope{}, err
	}
	events, err := rpc.ParseBatchEvents(params)
	if err != nil {
		return rpc.EventEnvelope{}, err
	}
	if len(events) != 1 {
		return rpc.EventEnvelope{}, errors.New("one emitted event became another batch shape")
	}
	var delivered string
	if err := json.Unmarshal(events[0], &delivered); err != nil {
		return rpc.EventEnvelope{}, err
	}
	return rpc.ParseEventEnvelope(delivered)
}

// burstSDKBus has one owned engine worker. Each accepted request is consumed
// before its acknowledgement, and cleanup closes and joins that worker.
func burstSDKBus(t *testing.T, consume func(rpc.EmitEventInput) error) *sdkEventBus {
	t.Helper()
	pluginEnd, engineEnd := net.Pipe()
	plugin := sdk.NewWithConn("rib-burst-test", pluginEnd)
	engine := rpc.NewMuxConn(rpc.NewConn(engineEnd, engineEnd))
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	var worker sync.WaitGroup
	worker.Add(1)
	go func() {
		defer worker.Done()
		for request := range engine.Requests() {
			var input rpc.EmitEventInput
			err := json.Unmarshal(request.Params, &input)
			if err == nil {
				err = consume(input)
			}
			if err != nil {
				if err := engine.SendError(ctx, request.ID, err.Error()); err != nil {
					return
				}
				continue
			}
			if err := engine.SendResult(ctx, request.ID, &rpc.EmitEventOutput{Delivered: 1}); err != nil {
				return
			}
		}
	}()
	t.Cleanup(func() { cancel(); _ = plugin.Close(); _ = engine.Close(); worker.Wait() })
	return &sdkEventBus{plugin: plugin}
}

// TestCanonicalEventFrameByteBounds checks the byte proof with actual request
// and subscriber readers, not source probes. Canonical escaping must survive
// at the widest ID; an indivisible near-boundary value uses its actual writer.
func TestCanonicalEventFrameByteBounds(t *testing.T) {
	for _, test := range []struct {
		name          string
		namespace     string
		wantNamespace string
		boundary      bool
	}{
		{"html-controls", "<>&\x00\n", "<>&\x00\n", false},
		{"invalid-utf8", "ns\xff", "ns\ufffd", false},
		{"unicode-separators", "ns\u2028\u2029", "ns\u2028\u2029", false},
		{"widest-id-boundary", "bgp-rib", "bgp-rib", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			input := rpc.EmitEventInput{Namespace: test.namespace, EventType: "\"\\\t",
				Direction: "<>&\r", PeerAddress: "\u2028\u2029"}
			budget := input.CanonicalPayloadBudget()
			value := "\"\\\b\f\n\r\t\x00<>&\u2028\u2029\xff []{},"
			if test.boundary {
				value = strings.Repeat("\\", (budget-2)/2)
			}
			canonical, err := json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			if len(canonical) > budget {
				t.Fatal("fixture canonical payload exceeds its byte budget")
			}
			route := ribevents.ValidationRoute{Peer: netip.MustParseAddr("fe80::1").WithZone(test.namespace),
				Family: family.IPv6Unicast, Prefix: netip.MustParsePrefix("2001:db8::/32"),
				PathID: ^uint32(0), NLRI: value}
			routeJSON, err := route.MarshalJSON()
			if err != nil {
				t.Fatal(err)
			}
			if len(routeJSON) > route.JSONSizeBound() {
				t.Fatal("validation byte bound read below actual JSON")
			}
			input.Event = string(canonical)
			params, err := json.Marshal(&input)
			if err != nil {
				t.Fatal(err)
			}
			line := rpc.AppendRequest(nil, ^uint64(0), rpc.MethodEmitEvent, params)
			var wire bytes.Buffer
			if err := rpc.NewFrameWriter(&wire).Write(line); err != nil {
				t.Fatal(err)
			}
			read, err := rpc.NewFrameReader(&wire).Read()
			if err != nil {
				t.Fatal(err)
			}
			id, method, data, err := rpc.ParseLine(read)
			if err != nil {
				t.Fatal(err)
			}
			if id != ^uint64(0) || method != rpc.MethodEmitEvent {
				t.Fatal("request framing changed its identity")
			}
			var got rpc.EmitEventInput
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatal(err)
			}
			if got.Namespace != test.wantNamespace || got.Event != input.Event || got.EventType != input.EventType || got.Direction != input.Direction || got.PeerAddress != input.PeerAddress {
				t.Fatal("request framing truncated or changed canonical payload/metadata")
			}
			envelope, err := burstDeliveredEnvelope(input)
			if err != nil {
				t.Fatal(err)
			}
			if envelope.Namespace != test.wantNamespace || envelope.Event != input.EventType || !bytes.Equal(envelope.Payload, canonical) || len(envelope.Metadata) != 0 {
				t.Fatal("enveloped subscriber framing changed payload or invented receipt metadata")
			}
		})
	}
	t.Run("indivisible-actual-id", func(t *testing.T) {
		batch := &ribevents.BestChangeBatch{Protocol: "bgp", Family: family.IPv4Unicast,
			Changes: []ribevents.BestChangeEntry{{Action: routeaction.Add, Prefix: burstPrefix(1), Interface: "x"}}}
		event, err := json.Marshal(batch)
		if err != nil {
			t.Fatal(err)
		}
		input := rpc.EmitEventInput{Namespace: ribevents.Namespace, EventType: ribevents.EventBestChange, Event: string(event)}
		params, err := json.Marshal(&input)
		if err != nil {
			t.Fatal(err)
		}
		actualPrefix := len(rpc.AppendRequest(nil, 1, rpc.MethodEmitEvent, json.RawMessage("0"))) - 1
		widestPrefix := len(rpc.AppendRequest(nil, ^uint64(0), rpc.MethodEmitEvent, json.RawMessage("0"))) - 1
		padding := rpc.MaxMessageSize - 8 - actualPrefix - len(params) + 1
		// Synthetic JSON, not a BGP attribute/OS-interface validity claim.
		batch.Changes[0].Interface = strings.Repeat("x", padding)
		event, err = json.Marshal(batch)
		if err != nil {
			t.Fatal(err)
		}
		input.Event = string(event)
		params, err = json.Marshal(&input)
		if err != nil {
			t.Fatal(err)
		}
		if actualPrefix+len(params) != rpc.MaxMessageSize-8 || widestPrefix+len(params) <= rpc.MaxMessageSize {
			t.Fatal("fixture does not distinguish actual-ID fit from widest-ID refusal")
		}
		calls := 0
		bus := burstSDKBus(t, func(got rpc.EmitEventInput) error {
			calls++
			if got.Event != input.Event {
				return errors.New("actual writer changed an indivisible payload")
			}
			return nil
		})
		if _, err := bus.Emit(input.Namespace, input.EventType, batch); err != nil {
			t.Fatal(err)
		}
		if calls != 1 {
			t.Fatalf("actual writer delivered %d indivisible events, want 1", calls)
		}
	})
	t.Run("singleton-overflow", func(t *testing.T) {
		batch := &ribevents.BestChangeBatch{Protocol: "bgp", Family: family.IPv4Unicast,
			Changes: []ribevents.BestChangeEntry{{Action: routeaction.Add, Prefix: burstPrefix(1),
				Attributes: bytes.Repeat([]byte{0x81}, 12*1024*1024)}}}
		calls := 0
		bus := burstSDKBus(t, func(rpc.EmitEventInput) error { calls++; return nil })
		if _, err := bus.Emit(ribevents.Namespace, ribevents.EventBestChange, batch); err == nil {
			t.Fatal("actual writer accepted a singleton above the frame ceiling")
		}
		if calls != 0 {
			t.Fatalf("oversized singleton leaked %d partial events", calls)
		}
	})
}
