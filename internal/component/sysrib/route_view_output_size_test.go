// Design: docs/plugin-development/protocol.md -- complete routing frames.

package sysrib

import (
	"encoding/json"
	"math"
	"net/netip"
	"strings"
	"testing"

	sysribevents "github.com/ze-software/ze/internal/component/sysrib/events"
	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/replay"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func TestRouteViewOutputSizeBranches(t *testing.T) {
	base := routeViewOutputBatch()
	wide := routeViewCloneOutputBatch(base)
	wide.Changes[0].Weight = math.MaxUint8
	wide.Changes[0].Metric = math.MaxUint32
	wide.Changes[0].TableID = math.MaxUint32
	wide.Changes[0].Labels = []uint32{0, 9, 10, 999, math.MaxUint32}
	wide.Changes[0].NextHop = netip.MustParseAddr("fe80::1").WithZone("é\"\\\n<&>\u2028\u2029")
	wide.Changes[0].ECMPPaths = append(wide.Changes[0].ECMPPaths, sysribevents.ECMPPath{})
	withdraw := sysribevents.BestChangeBatch{Family: family.IPv6Unicast, Changes: []sysribevents.BestChangeEntry{{Action: routeaction.Withdraw, Prefix: netip.MustParsePrefix("2001:db8::/32")}}}
	for _, test := range []struct {
		name  string
		batch sysribevents.BestChangeBatch
	}{
		{name: "nil", batch: sysribevents.BestChangeBatch{Family: family.IPv4Unicast}},
		{name: "empty", batch: sysribevents.BestChangeBatch{Family: family.IPv4Unicast, Changes: []sysribevents.BestChangeEntry{}}},
		{name: "withdraw", batch: withdraw},
		{name: "metadata", batch: base},
		{name: "numeric-zone-and-empty-hop", batch: wide},
	} {
		t.Run(test.name, func(t *testing.T) {
			for _, token := range []uint64{0, replay.Broadcast} {
				input := routeViewPublication{Session: "session\"\\\n<&>é\u2028", Cursor: math.MaxUint64, Operation: 10, Batch: test.batch}
				input.Batch.ReplayID = token
				if got, want := routeViewPublicationBytes(input), len(routeViewOutputJSON(t, input)); got != want {
					t.Fatalf("size=%d canonical JSON=%d replay=%d", got, want, token)
				}
			}
		})
	}
}

func TestRouteViewOutputNativeSizeNoAlloc(t *testing.T) {
	input := routeViewPublication{Session: "native-owner", Cursor: math.MaxUint64, Operation: math.MaxUint64, Batch: routeViewOutputBatch()}
	var size int
	if allocs := testing.AllocsPerRun(100, func() { size = routeViewPublicationBytes(input) }); allocs != 0 {
		t.Fatalf("native frame sizing allocated %g times", allocs)
	}
	if size != len(routeViewOutputJSON(t, input)) {
		t.Fatal("allocation control did not measure the complete canonical request")
	}
}

// Use HTML escapes to fill the wire frame while keeping every interface well
// inside the declared character limit. Shared test input is immutable.
func routeViewOutputFrame(t *testing.T, input routeViewPublication) routeViewPublication {
	t.Helper()
	escaped := func(width int) string { return strings.Repeat("<", width/6) + strings.Repeat("x", width%6) }
	hops := make([]sysribevents.ECMPPath, sysribevents.MaxECMPPaths)
	for i := range hops {
		hops[i] = sysribevents.ECMPPath{NextHop: netip.MustParseAddr("198.51.100.2"), Interface: escaped(512)}
	}
	build := func(count int) routeViewPublication {
		value := input
		value.Batch = sysribevents.BestChangeBatch{Family: family.IPv4Unicast, Changes: make([]sysribevents.BestChangeEntry, (count+len(hops)-1)/len(hops))}
		for i := range value.Batch.Changes {
			end := min(len(hops), count-i*len(hops))
			value.Batch.Changes[i] = sysribevents.BestChangeEntry{Action: routeaction.Add, Prefix: netip.MustParsePrefix("192.0.2.0/24"), Protocol: "static", NextHop: netip.MustParseAddr("198.51.100.1"), ECMPPaths: hops[:end]}
		}
		return value
	}
	budget := rpc.RouteViewRequestBudget(rpc.MethodRouteViewPublish)
	low, high := 1, rpc.RouteViewRecordsMax*len(hops)
	for low < high {
		mid := low + (high-low+1)/2
		if routeViewPublicationBytes(build(mid)) <= budget {
			low = mid
		} else {
			high = mid - 1
		}
	}
	value := build(low)
	remaining := budget - routeViewPublicationBytes(value)
	tail := &value.Batch.Changes[len(value.Batch.Changes)-1]
	tail.ECMPPaths = append([]sysribevents.ECMPPath(nil), tail.ECMPPaths...)
	tail.ECMPPaths[len(tail.ECMPPaths)-1].Interface = escaped(512 + remaining)
	if err := routeViewValidateOutput(&value.Batch); err != nil {
		t.Fatalf("frame-edge control violated the semantic contract: %v", err)
	}
	if got, encoded := routeViewPublicationBytes(value), len(routeViewOutputJSON(t, value)); got != budget || encoded != budget {
		t.Fatalf("frame-edge control size=%d canonical=%d budget=%d", got, encoded, budget)
	}
	return value
}

func TestRouteViewOutputCompleteFrameBoundary(t *testing.T) {
	host, owner, input, received := routeViewOutputHost(t)
	input = routeViewOutputFrame(t, input)
	ack, err := routeViewPublishNative(host, t.Context(), owner, input)
	if err != nil {
		t.Fatalf("last valid native frame was refused: %v", err)
	}
	oversized := input
	oversized.Operation++
	oversized.Batch = routeViewCloneOutputBatch(input.Batch)
	tail := &oversized.Batch.Changes[len(oversized.Batch.Changes)-1]
	tail.ECMPPaths[len(tail.ECMPPaths)-1].Interface += "x"
	budget := rpc.RouteViewRequestBudget(rpc.MethodRouteViewPublish)
	if got := len(routeViewOutputJSON(t, oversized)); got != budget+1 {
		t.Fatalf("first invalid control size=%d, want %d", got, budget+1)
	}
	if _, err := routeViewPublishNative(host, t.Context(), owner, oversized); err == nil {
		t.Error("native adapter accepted the first oversized complete frame")
	}
	if _, err := routeViewPublishHandler(host, t.Context(), owner, routeViewOutputJSON(t, oversized)); err == nil {
		t.Error("JSON adapter accepted the first oversized complete frame")
	}
	retried, err := routeViewPublishHandler(host, t.Context(), owner, routeViewOutputJSON(t, input))
	if err != nil || retried != ack || len(received()) != 1 {
		t.Fatalf("frame refusal changed the original ACK or output: ack=%+v err=%v emissions=%d", retried, err, len(received()))
	}
}

func FuzzRouteViewOutputSize(f *testing.F) {
	for _, seed := range [][]byte{nil, []byte("port1"), []byte("\"\\\n\x00<&>é\u2028\u2029"), {0xff, 0xc0, 0x80}} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 256 {
			t.Skip()
		}
		input := routeViewPublication{Session: string(data), Cursor: uint64(len(data)), Operation: math.MaxUint64, Batch: routeViewOutputBatch()}
		input.Batch.Changes[0].Interface = string(data)
		input.Batch.Changes[0].NextHop = netip.MustParseAddr("fe80::1").WithZone(string(data))
		input.Batch.Changes[0].ECMPPaths[0].Interface = string(data)
		if got, want := routeViewPublicationBytes(input), len(routeViewOutputJSON(t, input)); got != want {
			t.Fatalf("size=%d canonical JSON=%d bytes=%x", got, want, data)
		}
	})
}

func FuzzRouteViewOutputBatchDecoder(f *testing.F) {
	for _, seed := range []string{`{}`, `null`, `{"family":"ipv4 unicast","changes":[]}`, `{"family":"ipv4 unicast","changes":[null]}`, `{"family":"ipv4 unicast","changes":[{"action":"withdraw","prefix":"192.0.2.0/24","protocol":""}]}`} {
		f.Add([]byte(seed))
	}
	for _, seed := range []string{`{"family":"ipv4/unicast","changes":[]}`, `{"family":"ipv4/unicast","changes":[null]}`, `{"family":"ipv4/unicast","changes":[{"action":"withdraw","prefix":"192.0.2.0/24","protocol":""}]}`} {
		f.Add([]byte(seed))
	}
	metadata, err := json.Marshal(routeViewOutputBatch())
	if err != nil {
		f.Fatal(err)
	}
	f.Add(metadata)
	f.Add([]byte{0xff})
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 16*1024 {
			t.Skip()
		}
		batch, err := routeViewDecodeOutputBatch(json.RawMessage(data))
		if err == nil && (batch.Changes == nil || len(batch.Changes) > rpc.RouteViewRecordsMax) {
			t.Fatalf("decoder accepted an absent or unbounded change vector: %d", len(batch.Changes))
		}
	})
}
