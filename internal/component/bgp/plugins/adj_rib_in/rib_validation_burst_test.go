// Design: docs/architecture/api/process-protocol.md -- byte-bounded validation events.
// Related: rib_eligibility.go -- emission and outstanding notification ownership.
package adj_rib_in

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// TestValidationEventBurstCrossesRealRPC sends individually valid route keys
// whose combined JSON exceeds one frame. The real RPC reader and production
// event decoder must receive every key once, in original order.
func TestValidationEventBurstCrossesRealRPC(t *testing.T) {
	changes := make([]ribevents.ValidationRoute, 170000)
	for i := range changes {
		changes[i] = ribevents.ValidationRoute{
			Peer: netip.MustParseAddr("2001:db8::1"), Family: family.IPv6Unicast,
			Prefix: netip.MustParsePrefix("2001:db8:1234:5678::/64"), PathID: uint32(i),
		}
	}
	encoded, err := json.Marshal(changes)
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) <= rpc.MaxMessageSize {
		t.Fatal("fixture does not cross the frame ceiling")
	}
	pluginEnd, engineEnd := net.Pipe()
	plugin := sdk.NewWithConn("validation-burst-test", pluginEnd)
	engine := rpc.NewMuxConn(rpc.NewConn(engineEnd, engineEnd))
	t.Cleanup(func() { _ = plugin.Close(); _ = engine.Close() })
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	result := make(chan error, 1)
	go consumeValidationBurst(ctx, engine, changes, result)
	r := newTestManager(t)
	r.plugin = plugin
	if err := r.emitValidationEvent(changes); err != nil {
		t.Fatalf("valid validation changes were refused instead of chunked: %v", err)
	}
	select {
	case err := <-result:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
}

func consumeValidationBurst(ctx context.Context, engine *rpc.MuxConn, want []ribevents.ValidationRoute, result chan<- error) {
	seen := 0
	frames := 0
	for seen < len(want) {
		select {
		case request, ok := <-engine.Requests():
			if !ok {
				result <- errors.New("event consumer closed before all changes arrived")
				return
			}
			var input rpc.EmitEventInput
			if err := json.Unmarshal(request.Params, &input); err != nil {
				result <- err
				return
			}
			envelope, err := json.Marshal(rpc.EventEnvelope{Namespace: input.Namespace,
				Event: input.EventType, Payload: json.RawMessage(input.Event)})
			if err != nil {
				result <- err
				return
			}
			batch, matched, err := ribevents.DecodeValidationChange(string(envelope))
			if err != nil {
				result <- fmt.Errorf("invalid validation event: matched=%v routes=%d: %w", matched, len(batch), err)
				return
			}
			if !matched {
				result <- fmt.Errorf("invalid validation event: matched=%v routes=%d", matched, len(batch))
				return
			}
			if len(batch) == 0 {
				result <- fmt.Errorf("invalid validation event: matched=%v routes=%d", matched, len(batch))
				return
			}
			for _, route := range batch {
				if seen >= len(want) || route != want[seen] {
					result <- fmt.Errorf("validation change %d duplicated or reordered", seen)
					return
				}
				seen++
			}
			frames++
			if err := engine.SendResult(ctx, request.ID, &rpc.EmitEventOutput{Delivered: 1}); err != nil {
				result <- err
				return
			}
		case <-ctx.Done():
			result <- ctx.Err()
			return
		}
	}
	if frames < 2 {
		result <- errors.New("the oversized event did not cross multiple frames")
		return
	}
	result <- nil
}

// TestDisableValidationRefusesIncompleteNotification proves a failed recovery
// hand-off cannot be reported as a successful command or erase its owed keys.
func TestDisableValidationRefusesIncompleteNotification(t *testing.T) {
	r := newTestManager(t)
	r.validationEnabled = true
	r.validationGate = true
	failure := errors.New("engine refused validation notification")
	r.emitValidation = func([]ribevents.ValidationRoute) error { return failure }
	peer := netip.MustParseAddr("192.0.2.1")
	key := compactRouteKey{Fam: family.IPv4Unicast, Prefix: netip.MustParsePrefix("203.0.113.0/24")}
	r.pending[pendingKey(peer, key)] = &pendingRoute{peerAddr: peer, family: key.Fam,
		routeKey: key, route: &RawRoute{Family: key.Fam, Ineligible: true}}
	status, _, err := r.handleCommand("request bgp adj-rib-in disable-validation", nil, "")
	if !errors.Is(err, failure) || status != statusError {
		t.Fatalf("incomplete command status=%q error=%v, want error %v", status, err, failure)
	}
	r.mu.RLock()
	owed := len(r.validationChanges)
	r.mu.RUnlock()
	if owed != 1 {
		t.Fatalf("notification failure retained %d owed keys, want 1", owed)
	}
}

// TestDisableValidationRetriesOnlyUnacknowledgedChunks rejects the second real
// RPC chunk, then retries the already-disabled command. Every acknowledged key
// must stay delivered once; the retained suffix must keep its order and complete.
func TestDisableValidationRetriesOnlyUnacknowledgedChunks(t *testing.T) {
	const routes = 170000
	r := newTestManager(t)
	r.validationEnabled, r.validationGate = true, true
	peer := netip.MustParseAddr("2001:db8::1")
	prefix := netip.MustParsePrefix("2001:db8:1234:5678::/64")
	for index := range routes {
		key := compactRouteKey{Fam: family.IPv6Unicast, Prefix: prefix, PathID: uint32(index)}
		r.pending[pendingKey(peer, key)] = &pendingRoute{peerAddr: peer, family: key.Fam,
			routeKey: key, route: &RawRoute{Family: key.Fam, PathID: key.PathID, HasPathID: true, Ineligible: true}}
	}
	pluginEnd, engineEnd := net.Pipe()
	plugin := sdk.NewWithConn("validation-retry-test", pluginEnd)
	engine := rpc.NewMuxConn(rpc.NewConn(engineEnd, engineEnd))
	r.plugin = plugin
	r.emitValidation = r.emitValidationEvent
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()
	order := make(chan []ribevents.ValidationRoute, 1)
	result := make(chan error, 1)
	done := make(chan struct{})
	t.Cleanup(func() { cancel(); _ = plugin.Close(); _ = engine.Close(); <-done })
	go func() {
		defer close(done)
		seen := make([]bool, routes)
		accepted := 0
		calls := 0
		var suffix []ribevents.ValidationRoute
		suffixIndex := 0
		for accepted < routes {
			select {
			case request, ok := <-engine.Requests():
				if !ok {
					result <- errors.New("retry consumer closed")
					return
				}
				calls++
				var input rpc.EmitEventInput
				if err := json.Unmarshal(request.Params, &input); err != nil {
					result <- err
					return
				}
				var changes []ribevents.ValidationRoute
				if err := json.Unmarshal([]byte(input.Event), &changes); err != nil {
					result <- err
					return
				}
				if calls == 2 {
					if err := engine.SendError(ctx, request.ID, "refused second validation chunk"); err != nil {
						result <- err
						return
					}
					select {
					case suffix = <-order:
					case <-ctx.Done():
						result <- ctx.Err()
						return
					}
					if len(suffix) < len(changes) {
						result <- errors.New("retained suffix lost part of the rejected wire chunk")
						return
					}
					for i := range changes {
						if suffix[i] != changes[i] {
							result <- errors.New("retained suffix reordered the rejected wire chunk")
							return
						}
					}
					continue
				}
				for _, change := range changes {
					index := int(change.PathID)
					if index >= routes || seen[index] || change.Peer != peer || change.Prefix != prefix || change.Family != family.IPv6Unicast {
						result <- fmt.Errorf("retry duplicated or changed a delivered key: %+v", change)
						return
					}
					if calls > 2 {
						if suffixIndex >= len(suffix) || change != suffix[suffixIndex] {
							result <- errors.New("retry reordered the retained unacknowledged suffix")
							return
						}
						suffixIndex++
					}
					seen[index] = true
					accepted++
				}
				if err := engine.SendResult(ctx, request.ID, &rpc.EmitEventOutput{Delivered: 1}); err != nil {
					result <- err
					return
				}
			case <-ctx.Done():
				result <- ctx.Err()
				return
			}
		}
		if calls < 3 || suffixIndex != len(suffix) {
			result <- errors.New("retry did not complete its retained suffix")
			return
		}
		result <- nil
	}()
	status, _, err := r.handleCommand("request bgp adj-rib-in disable-validation", nil, "")
	if status != statusError || err == nil || !strings.Contains(err.Error(), "refused second validation chunk") {
		t.Fatalf("partial disable status=%q error=%v, want the second-chunk refusal", status, err)
	}
	r.mu.RLock()
	remaining := append([]ribevents.ValidationRoute(nil), r.validationChanges...)
	disabled := !r.validationEnabled
	r.mu.RUnlock()
	if !disabled || len(remaining) == 0 || len(remaining) >= routes {
		t.Fatalf("partial hand-off disabled=%v retained=%d, want only an unacknowledged suffix", disabled, len(remaining))
	}
	order <- remaining
	status, _, err = r.handleCommand("request bgp adj-rib-in disable-validation", nil, "")
	if err != nil || status != statusDone {
		t.Fatalf("already-disabled retry status=%q error=%v", status, err)
	}
	select {
	case err := <-result:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	r.mu.RLock()
	owed := len(r.validationChanges)
	installed := r.ribIn[peer].Len()
	r.mu.RUnlock()
	if owed != 0 || installed != routes {
		t.Fatalf("completed retry owes=%d installed=%d/%d", owed, installed, routes)
	}
}
