// Design: docs/architecture/rib/unified-locrib.md -- acknowledged forwarding output.
// Related: route_view_output.go -- authenticated publication and scoped cleanup.

package sysrib

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/netip"
	"reflect"
	"testing"

	sysribevents "github.com/ze-software/ze/internal/component/sysrib/events"
	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/replay"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/internal/core/rib/routetype"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func routeViewOutputBatch() sysribevents.BestChangeBatch {
	return sysribevents.BestChangeBatch{Family: family.IPv4Unicast, Changes: []sysribevents.BestChangeEntry{{
		Action: routeaction.Add, Prefix: netip.MustParsePrefix("192.0.2.0/24"), Protocol: "static",
		NextHop: netip.MustParseAddr("198.51.100.1"), Interface: "port1", OnLink: true, Weight: 7,
		Labels: []uint32{100, 200}, RouteType: routetype.Unicast, Metric: 37,
		SRv6SID:   netip.MustParseAddr("2001:db8::1"),
		ECMPPaths: []sysribevents.ECMPPath{{NextHop: netip.MustParseAddr("198.51.100.2"), Interface: "port2", OnLink: true, Weight: 9, Labels: []uint32{300}}},
		Backup:    []sysribevents.ECMPPath{{NextHop: netip.MustParseAddr("198.51.100.3"), Interface: "port3", OnLink: true, Labels: []uint32{400}}},
	}}}
}

// An acknowledgment means one accepted typed-bus publication. Retrying the
// same operation never emits again, and caller-owned slices cannot change it.
func TestRouteViewOutputGenerationCleanup(t *testing.T) {
	redistevents.RegisterProtocol("static")
	var authority routeViewAuthority
	owner, opened := routeViewOpenTest(t, &authority, locrib.NewRIB())
	page := routeViewReadTest(t, &authority, owner, opened.Session, 0, 1)
	bus := newTestEventBus()
	var delivered []*sysribevents.BestChangeBatch
	sysribevents.BestChange.Subscribe(bus, func(batch *sysribevents.BestChangeBatch) { delivered = append(delivered, batch) })
	input := routeViewPublication{Session: opened.Session, Cursor: page.Cursor, Operation: 1, Batch: routeViewOutputBatch()}
	input.Batch.ReplayID = replay.Broadcast
	want := routeViewOutputBatch()
	want.ReplayID = replay.Broadcast
	ack, err := authority.publish(context.Background(), owner, input, bus)
	if err != nil {
		t.Fatal(err)
	}
	if ack != (rpc.RouteViewPublishOutput{Session: opened.Session, Cursor: page.Cursor, Operation: 1}) {
		t.Fatalf("wrong acknowledgment: %+v", ack)
	}
	if len(delivered) != 1 || !reflect.DeepEqual(*delivered[0], want) {
		t.Fatalf("typed output lost metadata or replay: %+v", delivered)
	}
	retried, err := authority.publish(context.Background(), owner, input, bus)
	if err != nil || retried != ack || len(delivered) != 1 {
		t.Fatalf("retry duplicated output or changed ack: %+v %v count=%d", retried, err, len(delivered))
	}
	input.Batch.Changes[0].Labels[0] = 999
	input.Batch.Changes[0].ECMPPaths[0].Labels[0] = 999
	input.Batch.Changes[0].Backup[0].Labels[0] = 999
	if !reflect.DeepEqual(*delivered[0], want) {
		t.Fatal("caller mutation changed emitted forwarding values")
	}
	if _, err := authority.publish(context.Background(), owner, input, bus); err == nil {
		t.Fatal("operation identity accepted changed values")
	}
	if _, err := authority.close(owner, opened.Session); err != nil {
		t.Fatal(err)
	}
	if len(delivered) != 2 || len(delivered[1].Changes) != 1 || delivered[1].Changes[0].Action != routeaction.Withdraw || delivered[1].Changes[0].Prefix != want.Changes[0].Prefix {
		t.Fatalf("close failed scoped withdrawal: %+v", delivered)
	}
	replacement, fresh := routeViewOpenTest(t, &authority, locrib.NewRIB())
	freshPage := routeViewReadTest(t, &authority, replacement, fresh.Session, 0, 1)
	control := routeViewPublication{Session: fresh.Session, Cursor: freshPage.Cursor, Operation: 1, Batch: routeViewOutputBatch()}
	if _, err := authority.publish(context.Background(), replacement, control, bus); err != nil {
		t.Fatal(err)
	}
	before := len(delivered)
	if err := authority.cleanup(owner); err != nil {
		t.Fatal(err)
	}
	if _, err := authority.publish(context.Background(), owner, control, bus); err == nil {
		t.Fatal("old generation published replacement values")
	}
	if len(delivered) != before {
		t.Fatal("old cleanup withdrew replacement forwarding")
	}
	if _, err := authority.publish(context.Background(), replacement, control, bus); err != nil {
		t.Fatalf("old generation damaged healthy replacement: %v", err)
	}
}

type routeViewOutputFaultBus struct {
	*testEventBus
	failPublish  bool
	failWithdraw bool
}

func (bus *routeViewOutputFaultBus) Emit(namespace, eventType string, payload any) (int, error) {
	count, err := bus.testEventBus.Emit(namespace, eventType, payload)
	if err != nil {
		return count, err
	}
	batch, ok := payload.(*sysribevents.BestChangeBatch)
	if !ok {
		return count, errors.New("fault bus received a noncanonical payload")
	}
	withdraw := len(batch.Changes) > 0 && batch.Changes[0].Action == routeaction.Withdraw
	if withdraw {
		if bus.failWithdraw {
			return count, errors.New("withdrawal delivery failed after delivery")
		}
		return count, nil
	}
	if bus.failPublish {
		return count, errors.New("publication delivery failed after delivery")
	}
	return count, nil
}

// A bus can deliver before returning an error. Every attempted prefix must
// therefore enter cleanup, and failed cleanup prevents replacement admission.
func TestRouteViewOutputFailureTracksAttemptedBatch(t *testing.T) {
	redistevents.RegisterProtocol("static")
	var authority routeViewAuthority
	owner, opened := routeViewOpenTest(t, &authority, locrib.NewRIB())
	page := routeViewReadTest(t, &authority, owner, opened.Session, 0, 1)
	bus := &routeViewOutputFaultBus{testEventBus: newTestEventBus(), failPublish: true, failWithdraw: true}
	input := routeViewPublication{Session: opened.Session, Cursor: page.Cursor, Operation: 1, Batch: routeViewOutputBatch()}
	second := input.Batch.Changes[0]
	second.Prefix = netip.MustParsePrefix("203.0.113.0/24")
	input.Batch.Changes = append(input.Batch.Changes, second)
	if _, err := authority.publish(context.Background(), owner, input, bus); err == nil {
		t.Fatal("failed emission was acknowledged")
	}
	seen := make(map[netip.Prefix]bool)
	for _, event := range bus.events {
		batch := event.Payload.(*sysribevents.BestChangeBatch)
		for _, change := range batch.Changes {
			if change.Action == routeaction.Withdraw {
				seen[change.Prefix] = true
			}
		}
	}
	if len(seen) != 2 {
		t.Fatalf("partially failed emission abandoned attempted output: %v", seen)
	}
	if _, err := authority.publish(context.Background(), owner, input, bus); err == nil {
		t.Fatal("failed fence became healthy on retry")
	}
	if _, err := authority.close(owner, opened.Session); err == nil {
		t.Fatal("failed withdrawal close was acknowledged")
	}
	replacement := routeViewOwner()
	if _, err := authority.open(context.Background(), replacement, locrib.NewRIB(), rpc.RouteViewOpenInput{}); err == nil {
		t.Fatal("replacement abandoned failed predecessor cleanup")
	}
	bus.failWithdraw = false
	bus.failPublish = false
	fresh, err := authority.open(context.Background(), replacement, locrib.NewRIB(), rpc.RouteViewOpenInput{})
	if err != nil {
		t.Fatalf("successful cleanup did not restore admission: %v", err)
	}
	t.Cleanup(func() {
		if err := authority.cleanup(replacement); err != nil {
			t.Error(err)
		}
	})
	freshPage := routeViewReadTest(t, &authority, replacement, fresh.Session, 0, 1)
	input.Session, input.Cursor = fresh.Session, freshPage.Cursor
	if _, err := authority.publish(context.Background(), replacement, input, bus); err != nil {
		t.Fatalf("healthy replacement control failed: %v", err)
	}
}

// Refusal cannot consume an operation or emit a valid prefix from the same
// batch. Each malformed attempt is followed by its original permitted control.
func TestRouteViewOutputRefusalsAreAtomic(t *testing.T) {
	redistevents.RegisterProtocol("static")
	for _, kind := range []string{"future-cursor", "foreign-token", "zero-operation", "invalid-prefix", "unknown-protocol", "invalid-action", "missing-target", "bad-label", "oversized-ecmp", "invalid-family", "foreign-table"} {
		t.Run(kind, func(t *testing.T) {
			var authority routeViewAuthority
			owner, opened := routeViewOpenTest(t, &authority, locrib.NewRIB())
			page := routeViewReadTest(t, &authority, owner, opened.Session, 0, 1)
			bus := newTestEventBus()
			input := routeViewPublication{Session: opened.Session, Cursor: page.Cursor, Operation: 1, Batch: routeViewOutputBatch()}
			bad := routeViewPublication{Session: opened.Session, Cursor: page.Cursor, Operation: 1, Batch: routeViewOutputBatch()}
			trailing := bad.Batch.Changes[0]
			trailing.Prefix = netip.MustParsePrefix("203.0.113.0/24")
			switch kind {
			case "future-cursor":
				bad.Cursor++
			case "foreign-token":
				bad.Session = "foreign"
			case "zero-operation":
				bad.Operation = 0
			case "invalid-prefix":
				trailing.Prefix = netip.MustParsePrefix("203.0.113.1/24")
			case "unknown-protocol":
				trailing.Protocol = "unregistered-output-protocol"
			case "invalid-action":
				trailing.Action = routeaction.Del
			case "missing-target":
				trailing.NextHop = netip.Addr{}
				trailing.Interface = ""
				trailing.SRv6SID = netip.Addr{}
				trailing.ECMPPaths = nil
				trailing.Backup = nil
			case "bad-label":
				trailing.Labels = []uint32{1 << 20}
			case "oversized-ecmp":
				trailing.ECMPPaths = make([]sysribevents.ECMPPath, sysribevents.MaxECMPPaths+1)
			case "invalid-family":
				bad.Batch.Family = family.Family{}
			case "foreign-table":
				trailing.TableID = 7000
			}
			bad.Batch.Changes = append(bad.Batch.Changes, trailing)
			if _, err := authority.publish(context.Background(), owner, bad, bus); err == nil {
				t.Fatal("invalid publication succeeded")
			}
			if len(bus.events) != 0 {
				t.Fatal("invalid batch emitted its permitted leading row")
			}
			if _, err := authority.publish(context.Background(), owner, input, bus); err != nil {
				t.Fatalf("refusal consumed healthy operation: %v", err)
			}
			if len(bus.events) != 1 {
				t.Fatalf("control emitted %d events", len(bus.events))
			}
		})
	}
}

func TestRouteViewOutputMaximumOperationRetry(t *testing.T) {
	redistevents.RegisterProtocol("static")
	var authority routeViewAuthority
	owner, opened := routeViewOpenTest(t, &authority, locrib.NewRIB())
	page := routeViewReadTest(t, &authority, owner, opened.Session, 0, 1)
	bus := newTestEventBus()
	input := routeViewPublication{Session: opened.Session, Cursor: page.Cursor, Operation: math.MaxUint64, Batch: routeViewOutputBatch()}
	ack, err := authority.publish(context.Background(), owner, input, bus)
	if err != nil {
		t.Fatal(err)
	}
	input.Operation = 1
	if _, err := authority.publish(context.Background(), owner, input, bus); err == nil {
		t.Fatal("exhausted operation wrapped")
	}
	input.Operation = math.MaxUint64
	retry, err := authority.publish(context.Background(), owner, input, bus)
	if err != nil || retry != ack || len(bus.events) != 1 {
		t.Fatalf("maximum retry lost original ack: %+v %v count=%d", retry, err, len(bus.events))
	}
}

// Native sizing must match canonical JSON without serializing route values on
// the native path. Escapes, UTF-8, zones and omitted fields all affect the frame.
func TestRouteViewOutputNativeFrameSize(t *testing.T) {
	for _, text := range []string{"port1", "a\"b\\c\n", "é\u2028<&>", string([]byte{0xff}), ""} {
		input := routeViewPublication{Session: "s", Cursor: math.MaxUint64, Operation: math.MaxUint64, Batch: routeViewOutputBatch()}
		input.Batch.Changes[0].Interface = text
		input.Batch.Changes[0].NextHop = netip.MustParseAddr("fe80::1%port1")
		batch, err := json.Marshal(&input.Batch)
		if err != nil {
			t.Fatal(err)
		}
		encoded, err := json.Marshal(rpc.RouteViewPublishInput{Session: input.Session, Cursor: input.Cursor, Operation: input.Operation, Batch: batch})
		if err != nil {
			t.Fatal(err)
		}
		if got := routeViewPublicationBytes(input); got != len(encoded) {
			t.Fatalf("native size=%d JSON=%d for %q", got, len(encoded), text)
		}
	}
}
