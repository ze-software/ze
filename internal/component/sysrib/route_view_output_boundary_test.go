// Design: docs/architecture/rib/unified-locrib.md -- authenticated output boundaries.
// Related: register_route_view_output.go -- JSON and native owner adapters.

package sysrib

import (
	"bytes"
	"context"
	"encoding/json"
	"net/netip"
	"reflect"
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin/process"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	sysribevents "github.com/ze-software/ze/internal/component/sysrib/events"
	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/replay"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// This fixture uses the real daemon event bus and the real generation authority.
// Transport dispatch itself is proved by the generic server and paired daemon tests.
func routeViewOutputHost(t *testing.T) (*pluginserver.Server, *process.Process, routeViewPublication, func() []sysribevents.BestChangeBatch) {
	t.Helper()
	redistevents.RegisterProtocol("static")
	host, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	owner, opened := routeViewOpenTest(t, &routeViews, locrib.NewRIB())
	page := routeViewReadTest(t, &routeViews, owner, opened.Session, 0, 1)
	var mu sync.Mutex
	var batches []sysribevents.BestChangeBatch
	unsubscribe := sysribevents.BestChange.Subscribe(host, func(batch *sysribevents.BestChangeBatch) {
		mu.Lock()
		batches = append(batches, routeViewCloneOutputBatch(*batch))
		mu.Unlock()
	})
	t.Cleanup(unsubscribe)
	input := routeViewPublication{Session: opened.Session, Cursor: page.Cursor, Operation: 1, Batch: routeViewOutputBatch()}
	return host, owner, input, func() []sysribevents.BestChangeBatch {
		mu.Lock()
		defer mu.Unlock()
		return append([]sysribevents.BestChangeBatch(nil), batches...)
	}
}

func routeViewOutputJSON(t *testing.T, input routeViewPublication) []byte {
	t.Helper()
	batch, err := json.Marshal(&input.Batch)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(rpc.RouteViewPublishInput{Session: input.Session, Cursor: input.Cursor, Operation: input.Operation, Batch: batch})
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// Both owner adapters deliver the same native metadata and retain the same ACK.
func TestRouteViewOutputAdaptersAgree(t *testing.T) {
	for _, first := range []string{"json", "native"} {
		t.Run(first, func(t *testing.T) {
			host, owner, input, received := routeViewOutputHost(t)
			input.Batch.ReplayID = replay.Broadcast
			wire := routeViewOutputJSON(t, input)
			var firstAck, retryAck any
			var err error
			if first == "json" {
				firstAck, err = routeViewPublishHandler(host, t.Context(), owner, wire)
				if err == nil {
					retryAck, err = routeViewPublishNative(host, t.Context(), owner, input)
				}
			} else {
				firstAck, err = routeViewPublishNative(host, t.Context(), owner, input)
				if err == nil {
					retryAck, err = routeViewPublishHandler(host, t.Context(), owner, wire)
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			wantAck := rpc.RouteViewPublishOutput{Session: input.Session, Cursor: input.Cursor, Operation: input.Operation}
			if !reflect.DeepEqual(firstAck, wantAck) || !reflect.DeepEqual(retryAck, wantAck) {
				t.Fatalf("adapters disagree on acknowledged values: first=%+v retry=%+v", firstAck, retryAck)
			}
			batches := received()
			if len(batches) != 1 || !reflect.DeepEqual(batches[0], input.Batch) {
				t.Fatalf("adapters changed metadata/replay or duplicated a retry: %+v", batches)
			}
		})
	}
}

// An acknowledged withdrawal retires only its prefix. A retained live control
// remains owned until close, and retrying the withdrawal emits nothing twice.
func TestRouteViewOutputAcknowledgedWithdrawalRetiresOwnership(t *testing.T) {
	host, owner, input, received := routeViewOutputHost(t)
	control := routeViewCloneOutputEntry(input.Batch.Changes[0])
	control.Prefix = netip.MustParsePrefix("203.0.113.0/24")
	input.Batch.Changes = append(input.Batch.Changes, control)
	if _, err := routeViewPublishNative(host, t.Context(), owner, input); err != nil {
		t.Fatal(err)
	}
	withdraw := input
	withdraw.Operation++
	withdraw.Batch.Changes = []sysribevents.BestChangeEntry{{Action: routeaction.Withdraw, Prefix: input.Batch.Changes[0].Prefix, TableID: 254}}
	ack, err := routeViewPublishHandler(host, t.Context(), owner, routeViewOutputJSON(t, withdraw))
	if err != nil {
		t.Fatal(err)
	}
	retried, err := routeViewPublishNative(host, t.Context(), owner, withdraw)
	if err != nil || !reflect.DeepEqual(ack, retried) {
		t.Fatalf("withdrawal retry lost its original acknowledgment: first=%+v retry=%+v err=%v", ack, retried, err)
	}
	batches := received()
	if len(batches) != 2 || !reflect.DeepEqual(batches[1], withdraw.Batch) {
		t.Fatalf("acknowledged withdrawal was changed, lost or repeated: %+v", batches)
	}
	if _, err := routeViews.close(owner, input.Session); err != nil {
		t.Fatal(err)
	}
	batches = received()
	if len(batches) != 3 || len(batches[2].Changes) != 1 {
		t.Fatalf("cleanup did not retain exactly the live control: %+v", batches)
	}
	if got := batches[2].Changes[0]; got.Action != routeaction.Withdraw || got.Prefix != control.Prefix || got.TableID != control.TableID {
		t.Fatalf("cleanup retained a retired prefix or changed its live control: %+v", got)
	}
}

// Invalid adapter values cannot consume the valid owner's first operation.
func TestRouteViewOutputAdapterRefusalsPreserveOwner(t *testing.T) {
	host, owner, input, received := routeViewOutputHost(t)
	var typedNil *routeViewPublication
	for name, value := range map[string]any{"nil": nil, "typed-nil": typedNil, "pointer": &input, "wrong-value": "not a publication"} {
		if _, err := routeViewPublishNative(host, t.Context(), owner, value); err == nil {
			t.Errorf("native adapter accepted %s", name)
		}
	}
	if _, err := routeViewPublishNative(nil, t.Context(), owner, input); err == nil {
		t.Error("native adapter accepted an unavailable host")
	}
	if _, err := routeViewPublishHandler(nil, t.Context(), owner, routeViewOutputJSON(t, input)); err == nil {
		t.Error("JSON adapter accepted an unavailable host")
	}
	if len(received()) != 0 {
		t.Fatal("an invalid adapter request emitted forwarding output")
	}
	if _, err := routeViewPublishNative(host, t.Context(), owner, input); err != nil {
		t.Fatalf("refused adapter input damaged the healthy owner: %v", err)
	}
	if len(received()) != 1 {
		t.Fatal("the healthy owner did not publish its control")
	}
}

// Malformed nested wire values must fail before any forwarding or ACK mutation.
func TestRouteViewOutputMalformedWireIsAtomic(t *testing.T) {
	host, owner, input, received := routeViewOutputHost(t)
	base, err := json.Marshal(&input.Batch)
	if err != nil {
		t.Fatal(err)
	}
	mutate := func(change func(map[string]json.RawMessage, map[string]json.RawMessage)) []byte {
		var batch map[string]json.RawMessage
		if err := json.Unmarshal(base, &batch); err != nil {
			t.Fatal(err)
		}
		var entries []map[string]json.RawMessage
		if err := json.Unmarshal(batch["changes"], &entries); err != nil {
			t.Fatal(err)
		}
		change(batch, entries[0])
		if _, hasChanges := batch["changes"]; hasChanges {
			if string(batch["changes"]) != "null" {
				batch["changes"], err = json.Marshal(entries)
				if err != nil {
					t.Fatal(err)
				}
			}
		}
		data, err := json.Marshal(batch)
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	cases := map[string][]byte{
		"null-batch":          []byte("null"),
		"missing-family":      mutate(func(batch, _ map[string]json.RawMessage) { delete(batch, "family") }),
		"null-family":         mutate(func(batch, _ map[string]json.RawMessage) { batch["family"] = json.RawMessage("null") }),
		"missing-changes":     mutate(func(batch, _ map[string]json.RawMessage) { delete(batch, "changes") }),
		"null-changes":        mutate(func(batch, _ map[string]json.RawMessage) { batch["changes"] = json.RawMessage("null") }),
		"null-replay":         mutate(func(batch, _ map[string]json.RawMessage) { batch["replay"] = json.RawMessage("null") }),
		"unknown-batch-field": mutate(func(batch, _ map[string]json.RawMessage) { batch["claimed-owner"] = json.RawMessage(`"someone-else"`) }),
		"unknown-entry-field": mutate(func(_, entry map[string]json.RawMessage) { entry["claimed-owner"] = json.RawMessage(`"someone-else"`) }),
		"null-label":          mutate(func(_, entry map[string]json.RawMessage) { entry["labels"] = json.RawMessage("[100,null]") }),
		"null-ecmp-member":    mutate(func(_, entry map[string]json.RawMessage) { entry["ecmp-paths"] = json.RawMessage("[null]") }),
		"unknown-ecmp-field": mutate(func(_, entry map[string]json.RawMessage) {
			entry["ecmp-paths"] = json.RawMessage(`[{"next-hop":"198.51.100.2","extra":1}]`)
		}),
		"null-backup-member": mutate(func(_, entry map[string]json.RawMessage) { entry["backup"] = json.RawMessage("[null]") }),
		"trailing-json":      append(append([]byte(nil), base...), []byte(" {}")...),
		"invalid-utf8":       bytes.Replace(base, []byte(`"port1"`), []byte{'"', 0xff, '"'}, 1),
	}
	for _, field := range []string{"action", "prefix", "protocol"} {
		cases["missing-"+field] = mutate(func(_, entry map[string]json.RawMessage) { delete(entry, field) })
	}
	for _, field := range []string{"action", "prefix", "protocol", "next-hop", "interface", "on-link", "weight", "labels", "route-type", "metric", "table-id", "srv6-sid", "ecmp-paths", "backup"} {
		cases["null-"+field] = mutate(func(_, entry map[string]json.RawMessage) { entry[field] = json.RawMessage("null") })
	}
	for name, batch := range cases {
		header := mustOutputJSON(t, map[string]any{"session": input.Session, "cursor": input.Cursor, "operation": input.Operation})
		wire := append(append([]byte(nil), header[:len(header)-1]...), []byte(`,"batch":`)...)
		wire = append(wire, batch...)
		wire = append(wire, '}')
		before := len(received())
		if _, err := routeViewPublishHandler(host, t.Context(), owner, wire); err == nil {
			t.Errorf("accepted malformed %s", name)
		}
		if len(received()) != before {
			t.Errorf("malformed %s emitted forwarding output", name)
		}
	}
	if _, err := routeViewPublishNative(host, t.Context(), owner, input); err != nil {
		t.Fatalf("malformed wire input consumed the healthy operation: %v", err)
	}
	if len(received()) != 1 {
		t.Fatal("healthy native control was not delivered exactly once")
	}
}

func mustOutputJSON(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// A complete ECMP group is a forwarding target even without a primary gateway.
func TestRouteViewOutputECMPOnlyTarget(t *testing.T) {
	redistevents.RegisterProtocol("static")
	var authority routeViewAuthority
	owner, opened := routeViewOpenTest(t, &authority, locrib.NewRIB())
	page := routeViewReadTest(t, &authority, owner, opened.Session, 0, 1)
	batch := routeViewOutputBatch()
	batch.Changes[0].NextHop = netip.Addr{}
	batch.Changes[0].Interface = ""
	batch.Changes[0].SRv6SID = netip.Addr{}
	bus := newTestEventBus()
	var seen bool
	sysribevents.BestChange.Subscribe(bus, func(got *sysribevents.BestChangeBatch) { seen = reflect.DeepEqual(*got, batch) })
	if _, err := authority.publish(context.Background(), owner, routeViewPublication{Session: opened.Session, Cursor: page.Cursor, Operation: 1, Batch: batch}, bus); err != nil {
		t.Fatalf("complete ECMP-only target was refused: %v", err)
	}
	if !seen {
		t.Fatal("ECMP-only target was not delivered with its members intact")
	}
}

// Native strings must preserve the same valid UTF-8 identity as JSON strings.
func TestRouteViewOutputNativeUTF8RefusalIsAtomic(t *testing.T) {
	for _, target := range []string{"interface", "zone"} {
		t.Run(target, func(t *testing.T) {
			host, owner, input, received := routeViewOutputHost(t)
			bad := input
			bad.Batch = routeViewCloneOutputBatch(input.Batch)
			if target == "interface" {
				bad.Batch.Changes[0].Interface = string([]byte{0xff})
			} else {
				bad.Batch.Changes[0].NextHop = netip.MustParseAddr("fe80::1").WithZone(string([]byte{0xff}))
			}
			if _, err := routeViewPublishNative(host, t.Context(), owner, bad); err == nil {
				t.Errorf("native adapter accepted invalid UTF-8 %s", target)
			}
			if len(received()) != 0 {
				t.Error("invalid UTF-8 acquired output ownership")
			}
			if _, err := routeViewPublishNative(host, t.Context(), owner, input); err != nil {
				t.Fatalf("invalid UTF-8 consumed the healthy operation: %v", err)
			}
		})
	}
}

// Decoding an excess-row frame must stop allocating rows at the declared bound.
func TestRouteViewOutputRowDecodeBounded(t *testing.T) {
	makeFrame := func(count int) []byte {
		rows := bytes.Repeat([]byte("{},"), count)
		rows = rows[:len(rows)-1]
		data := append([]byte(`{"family":`), mustOutputJSON(t, routeViewOutputBatch().Family)...)
		data = append(data, []byte(`,"changes":[`)...)
		data = append(data, rows...)
		return append(data, ']', '}')
	}
	measure := func(data []byte) float64 {
		return testing.AllocsPerRun(1, func() {
			if _, err := routeViewDecodeOutputBatch(data); err == nil {
				t.Fatal("excess-row frame was accepted")
			}
		})
	}
	small := measure(makeFrame(rpc.RouteViewRecordsMax + 1))
	large := measure(makeFrame(rpc.RouteViewRecordsMax * 16))
	if large > small+32 {
		t.Fatalf("excess rows increased decoder allocations beyond framing overhead: small=%g large=%g", small, large)
	}
	batch := routeViewOutputBatch()
	batch.Changes = make([]sysribevents.BestChangeEntry, rpc.RouteViewRecordsMax)
	for i := range batch.Changes {
		batch.Changes[i] = routeViewOutputBatch().Changes[0]
	}
	decoded, err := routeViewDecodeOutputBatch(mustOutputJSON(t, &batch))
	if err != nil || len(decoded.Changes) != rpc.RouteViewRecordsMax {
		t.Fatalf("valid exact-bound control failed: rows=%d error=%v", len(decoded.Changes), err)
	}
}

// Neither an unknown withdrawal nor an OS-owned install may acquire this lease.
func TestRouteViewOutputOwnershipRefusalsPreserveControl(t *testing.T) {
	t.Cleanup(redistevents.PreserveForTest())
	host, owner, input, received := routeViewOutputHost(t)
	unknown := input
	unknown.Batch = routeViewCloneOutputBatch(input.Batch)
	unknown.Batch.Changes[0] = sysribevents.BestChangeEntry{Action: routeaction.Withdraw, Prefix: netip.MustParsePrefix("203.0.113.0/24")}
	if _, err := routeViewPublishNative(host, t.Context(), owner, unknown); err != nil {
		t.Fatal(err)
	}
	if len(received()) != 0 {
		t.Fatal("an unknown withdrawal emitted a removal")
	}
	connectedID := redistevents.RegisterProtocol("connected")
	redistevents.RegisterOSInstalled(connectedID)
	osOwned := input
	osOwned.Operation = 2
	osOwned.Batch = routeViewCloneOutputBatch(input.Batch)
	osOwned.Batch.Changes[0].Protocol = "connected"
	if _, err := routeViewPublishNative(host, t.Context(), owner, osOwned); err == nil {
		t.Fatal("OS-owned protocol acquired forwarding ownership")
	}
	input.Operation = 2
	if _, err := routeViewPublishNative(host, t.Context(), owner, input); err != nil {
		t.Fatalf("ownership refusal damaged the admitted control: %v", err)
	}
	if len(received()) != 1 {
		t.Fatal("the admitted control was not emitted exactly once")
	}
	if _, err := routeViews.close(owner, input.Session); err != nil {
		t.Fatal(err)
	}
	batches := received()
	if len(batches) != 2 || len(batches[1].Changes) != 1 || batches[1].Changes[0].Prefix != input.Batch.Changes[0].Prefix {
		t.Fatalf("cleanup claimed an unknown or OS-owned prefix: %+v", batches)
	}
}
