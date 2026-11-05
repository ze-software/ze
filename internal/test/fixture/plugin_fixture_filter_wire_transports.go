// Design: docs/architecture/core-design.md -- Ingress Filter Pipeline, cached canonical bytes.
// Related: register_filter_wire_transports.go -- fixture registration.

package fixture

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const filterWireTransportsSourcePeer = "source"

// filterWireTransports forwards the actual post-import cache entries, not a
// reconstructed route. The receiving peer independently checks every byte.
// Its arguments name the scenario, transport, accepted UPDATE count and the
// exact typed-slot and encoded-route counts expected after the input fence.
func filterWireTransports(ctx context.Context, args []string) error {
	if len(args) != 5 {
		return errors.New("filter-wire-transports takes <scenario> <transport> <forwarded> <slot-routes> <batch-routes>")
	}
	counts := [3]int{}
	for i := range counts {
		value, err := strconv.Atoi(args[i+2])
		if err != nil {
			return err
		}
		counts[i] = value
	}
	var forwarded atomic.Int32
	registration := sdk.Registration{CacheConsumer: true}
	if args[0] == "dryrun" {
		registration.Filters = []sdk.FilterDecl{{Name: "check", Direction: sdk.FilterImport,
			Reads: []string{"local-preference"}, Sets: []string{"med"}, OnError: sdk.OnErrorReject}}
	}
	setup := func(p *sdk.Plugin) error {
		p.OnFilter(filterWireTransportsCheckLP)
		p.SetStartupSubscriptions([]string{"update"}, nil, "parsed")
		p.OnEvent(func(raw string) error {
			var event struct {
				BGP struct {
					Message struct {
						ID        uint64 `json:"id"`
						Direction string `json:"direction"`
					} `json:"message"`
					Peer struct {
						Name string `json:"name"`
					} `json:"peer"`
					Update map[string]json.RawMessage `json:"update"`
				} `json:"bgp"`
			}
			if err := json.Unmarshal([]byte(raw), &event); err != nil {
				return err
			}
			if event.BGP.Message.Direction != "received" {
				return nil
			}
			id := []uint64{event.BGP.Message.ID}
			if _, eor := event.BGP.Update["eor"]; eor {
				return p.ReleaseCached(ctx, id)
			}
			if event.BGP.Peer.Name != filterWireTransportsSourcePeer {
				return p.ReleaseCached(ctx, id)
			}
			if err := p.ForwardCached(ctx, id, []string{"127.0.0.2"}); err != nil {
				return err
			}
			forwarded.Add(1)
			return nil
		})
		return nil
	}
	scenario := func(ctx context.Context, p *sdk.Plugin) error {
		for _, peer := range []string{filterWireTransportsSourcePeer, "destination"} {
			if !waitPeerEORSent(ctx, p, peer, WaitAttempts(80, 100*time.Millisecond, 70)) {
				return fmt.Errorf("%s never sent its initial EOR", peer)
			}
		}
		if args[0] == "irr" {
			// The first EOR can precede initialSyncEOROwed clearing. Settle the
			// whole sync before relying on a later one-shot EOR to release MP.
			if err := quiesce(ctx, p); err != nil {
				return fmt.Errorf("settle IRR peer initial sync: %w", err)
			}
			if status, _, err := p.DispatchCommand(ctx, "update bgp irr asn 65001"); err != nil || status != rpc.StatusDone {
				return fmt.Errorf("resolve IRR before input: status=%s: %w", status, err)
			}
		}
		if args[0] == "dryrun" {
			if err := filterWireTransportsDryRun(ctx, p); err != nil {
				return err
			}
		}
		before, err := filterWireTransportsCounts(ctx, p)
		if err != nil {
			return err
		}
		sentBefore, readable := p12PeerCounter(ctx, p, "127.0.0.2", "updates-sent")
		if !readable {
			return errors.New("receiver's updates-sent baseline is unavailable")
		}
		ackBefore, readable := p12PeerCounter(ctx, p, "destination", "eor-received")
		if !readable {
			return errors.New("receiver's acknowledgment baseline is unavailable")
		}
		acksExpected := ackBefore + 1
		if err := os.WriteFile("input-ready-"+args[1], nil, 0o600); err != nil {
			return err
		}
		if args[0] == "irr" {
			acksExpected++
			// Queuing cached forwards does not prove the receiver consumed them.
			// Release MP input only after every IPv4 golden has matched.
			if !p12WaitPeerCounter(ctx, p, "destination", "eor-received", ackBefore+1) {
				return errors.New("receiver never acknowledged its exact IPv4 UPDATE expectations")
			}
			status, _, err := p.DispatchCommand(ctx, "send bgp source update text nlri eor")
			if err != nil {
				return fmt.Errorf("release IRR MP input: %w", err)
			}
			if status != rpc.StatusDone {
				return fmt.Errorf("release IRR MP input: status=%s", status)
			}
		}
		if !Poll(ctx, 100, 100*time.Millisecond, func() bool {
			peer, err := peerDetail(ctx, p, filterWireTransportsSourcePeer)
			if err != nil {
				return false
			}
			return number07(peer["eor-received"]) >= 1 && int(forwarded.Load()) == counts[0]
		}) {
			return fmt.Errorf("input fence not reached: forwarded %d, want %d", forwarded.Load(), counts[0])
		}
		if !p12WaitPeerCounter(ctx, p, "127.0.0.2", "updates-sent", sentBefore+counts[0]) {
			return errors.New("cached UPDATEs never reached the receiver's socket")
		}
		// The receiver sends each marker only after matching its exact UPDATEs.
		// Socket writes alone do not prove the peer consumed its assertions.
		if !p12WaitPeerCounter(ctx, p, "destination", "eor-received", acksExpected) {
			return errors.New("receiver never acknowledged its exact UPDATE expectations")
		}
		after, err := filterWireTransportsCounts(ctx, p)
		if err != nil {
			return err
		}
		if after[0]-before[0] != float64(counts[1]) {
			return fmt.Errorf("typed slot judgments=%v, want %d", after[0]-before[0], counts[1])
		}
		if after[1]-before[1] != float64(counts[2]) {
			return fmt.Errorf("encoded batch routes=%v, want %d", after[1]-before[1], counts[2])
		}
		filterSubjectSaid("OK: cached receiver bytes and exact transport counts ", []byte(strings.Join(args, " ")))
		return nil
	}
	return observeConfigured(ctx, "wire-observer", &registration, setup, scenario)
}

func filterWireTransportsCounts(ctx context.Context, p *sdk.Plugin) ([2]float64, error) {
	var counts [2]float64
	status, data, err := dispatchValue(ctx, p, "show metrics values")
	if err != nil || status != rpc.StatusDone {
		return counts, fmt.Errorf("read transport metrics status=%s: %w", status, err)
	}
	for i, selector := range []string{"ze_plugin_filter_slot_routes_total", "ze_plugin_filter_batch_routes_sum"} {
		value, present := counter07(metricsText07(data), selector)
		if !present {
			return counts, fmt.Errorf("transport metric %s is absent", selector)
		}
		counts[i] = value
	}
	return counts, nil
}

func filterWireTransportsCheckLP(route *sdk.FilterRoute) (*sdk.FilterUpdateOutput, error) {
	fields, err := route.Fields()
	if err != nil {
		return nil, err
	}
	var values struct {
		LocalPreference uint32 `json:"local-preference"`
	}
	if err := json.Unmarshal(fields, &values); err != nil {
		return nil, err
	}
	if values.LocalPreference != 200 {
		return nil, fmt.Errorf("mixed member read local-preference %d, want 200", values.LocalPreference)
	}
	return &sdk.FilterUpdateOutput{Action: sdk.FilterModify, Sets: []rpc.FilterSet{{
		Field: "med", Action: rpc.FilterSetActionSet, Value: json.RawMessage("50"),
	}}}, nil
}

func filterWireTransportsDryRun(ctx context.Context, p *sdk.Plugin) error {
	status, data, err := dispatchMap11(ctx, p, "show policy test peer source import update "+filterWireTransportsInput)
	if err != nil || status != rpc.StatusDone {
		return fmt.Errorf("mixed dryrun status=%s: %w", status, err)
	}
	if data["action"] != rpc.FilterModify.String() {
		return fmt.Errorf("mixed dryrun action=%v, want modify", data["action"])
	}
	policy, ok := data["policy"].(map[string]any)
	if !ok {
		return fmt.Errorf("dryrun has no policy: %v", data)
	}
	if policy["final"] != rpc.FilterAccept.String() {
		return fmt.Errorf("dryrun final=%v, want accept", policy["final"])
	}
	trace, ok := data["trace"].([]any)
	if !ok {
		return fmt.Errorf("dryrun has no trace: %v", data)
	}
	if len(trace) != 3 {
		return fmt.Errorf("mixed dryrun trace length=%d, want 3: %v", len(trace), trace)
	}
	for i, want := range []string{"SETLP", "check", "NoV6"} {
		entry, ok := trace[i].(map[string]any)
		if !ok {
			return fmt.Errorf("trace entry %d is not an object", i)
		}
		if entry["filter"] != want {
			return fmt.Errorf("trace entry %d filter=%v, want %s", i, entry["filter"], want)
		}
		wantAction := rpc.FilterModify.String()
		if i == 2 {
			wantAction = rpc.FilterAccept.String()
			if entry["text-after"] != data["text-after"] {
				return fmt.Errorf("final trace and resulting UPDATE differ: %v", data)
			}
		}
		if entry["action"] != wantAction {
			return fmt.Errorf("trace entry %d action=%v, want %s", i, entry["action"], wantAction)
		}
		if entry["error"] != nil && entry["error"] != "" {
			return fmt.Errorf("trace entry %d failed: %v", i, entry)
		}
	}
	encoded, err := json.Marshal(data)
	if err != nil {
		return err
	}
	for _, value := range []string{"local-preference 200", "med 50"} {
		if !strings.Contains(string(encoded), value) {
			return fmt.Errorf("dryrun trace lacks %s: %s", value, encoded)
		}
	}
	return nil
}

const filterWireTransportsInput = "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF002F02000000144001010040020602010000FDE940030401010101180A0000"
