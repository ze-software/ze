// Design: docs/architecture/rib/unified-locrib.md -- scoped output retirement.

package sysrib

import (
	"net/netip"
	"reflect"
	"testing"

	sysribevents "github.com/ze-software/ze/internal/component/sysrib/events"
	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/internal/core/rib/routetype"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func TestRouteViewOutputCleanupKeepsFamiliesSeparate(t *testing.T) {
	bus := newTestEventBus()
	output := routeViewOutput{bus: bus, ledger: make(map[prefixKey]sysribevents.BestChangeEntry)}
	for fam, prefix := range map[family.Family]netip.Prefix{
		family.IPv4Unicast: netip.MustParsePrefix("192.0.2.0/24"),
		family.IPv6Unicast: netip.MustParsePrefix("2001:db8::/32"),
	} {
		output.ledger[prefixKey{family: fam, prefix: prefix}] = sysribevents.BestChangeEntry{Prefix: prefix, TableID: 254}
	}
	seen := make(map[family.Family]netip.Prefix)
	sysribevents.BestChange.Subscribe(bus, func(batch *sysribevents.BestChangeBatch) {
		if len(batch.Changes) != 1 {
			t.Fatalf("cleanup combined family rows: %+v", batch)
		}
		entry := batch.Changes[0]
		key := prefixKey{family: batch.Family, prefix: entry.Prefix}
		if _, owned := output.ledger[key]; !owned {
			t.Fatalf("cleanup changed an owned family/prefix key: %+v", batch)
		}
		if entry.Action != routeaction.Withdraw || entry.TableID != 254 {
			t.Fatalf("cleanup changed owned withdrawal metadata: %+v", entry)
		}
		seen[batch.Family] = entry.Prefix
	})
	if err := output.withdraw(routeViewOwner()); err != nil {
		t.Fatal(err)
	}
	if len(seen) != 2 || output.ledger != nil {
		t.Fatalf("cleanup abandoned an owned family: seen=%v ledger=%v", seen, output.ledger)
	}
}

// Successful chunks must leave the ledger even when another chunk fails.
// Retained event values must survive both the remaining chunks and the retry.
func TestRouteViewOutputCleanupRetriesOnlyFailedChunks(t *testing.T) {
	bus := &routeViewOutputFaultBus{testEventBus: newTestEventBus()}
	output := routeViewOutput{bus: bus, ledger: make(map[prefixKey]sysribevents.BestChangeEntry), acknowledged: &routeViewOutputAck{}}
	const count = 2*rpc.RouteViewRecordsMax + 17
	for i := range count {
		prefix := netip.PrefixFrom(netip.AddrFrom4([4]byte{10, 0, byte(i >> 8), byte(i)}), 32)
		output.ledger[prefixKey{family: family.IPv4Unicast, prefix: prefix}] = sysribevents.BestChangeEntry{Prefix: prefix, TableID: 254}
	}
	var attempts []*sysribevents.BestChangeBatch
	sysribevents.BestChange.Subscribe(bus, func(batch *sysribevents.BestChangeBatch) {
		attempts = append(attempts, batch)
		bus.failWithdraw = len(attempts) == 1
	})
	owner := routeViewOwner()
	if err := output.withdraw(owner); err == nil {
		t.Fatal("partially failed cleanup was acknowledged")
	}
	if len(attempts) != 3 {
		t.Fatalf("cleanup stopped before attempting every chunk: attempts=%d", len(attempts))
	}
	if output.acknowledged != nil {
		t.Fatal("failed cleanup retained its fenced publication")
	}
	seen := make(map[netip.Prefix]bool)
	retained := make([]sysribevents.BestChangeBatch, len(attempts))
	for i, batch := range attempts {
		if batch.Family != family.IPv4Unicast || len(batch.Changes) == 0 || len(batch.Changes) > rpc.RouteViewRecordsMax || cap(batch.Changes) > rpc.RouteViewRecordsMax {
			t.Fatalf("cleanup chunk did not own bounded family storage: family=%v len=%d cap=%d", batch.Family, len(batch.Changes), cap(batch.Changes))
		}
		retained[i] = routeViewCloneOutputBatch(*batch)
		for _, entry := range batch.Changes {
			if entry.Action != routeaction.Withdraw || entry.TableID != 254 || seen[entry.Prefix] {
				t.Fatalf("first cleanup changed or repeated an owned withdrawal: %+v", entry)
			}
			seen[entry.Prefix] = true
		}
	}
	if len(seen) != count || len(output.ledger) != rpc.RouteViewRecordsMax {
		t.Fatalf("partial cleanup lost attempted or retained ownership: attempted=%d ledger=%d", len(seen), len(output.ledger))
	}
	failed := make(map[netip.Prefix]bool, len(attempts[0].Changes))
	for _, entry := range attempts[0].Changes {
		if _, owned := output.ledger[prefixKey{family: family.IPv4Unicast, prefix: entry.Prefix}]; !owned {
			t.Fatalf("the failed chunk lost ownership of %v", entry.Prefix)
		}
		failed[entry.Prefix] = true
	}
	if err := output.withdraw(owner); err != nil {
		t.Fatalf("failed chunk did not recover on retry: %v", err)
	}
	if len(attempts) != 4 || len(attempts[3].Changes) != rpc.RouteViewRecordsMax {
		t.Fatalf("retry did not attempt exactly the failed chunk: attempts=%d", len(attempts))
	}
	for _, entry := range attempts[3].Changes {
		if !failed[entry.Prefix] {
			t.Fatalf("retry withdrew a previously successful prefix: %v", entry.Prefix)
		}
		delete(failed, entry.Prefix)
	}
	for i := range retained {
		if !reflect.DeepEqual(*attempts[i], retained[i]) {
			t.Fatalf("later cleanup mutated retained chunk %d", i)
		}
	}
	if output.ledger != nil {
		t.Fatal("successful retry retained the ledger backing storage")
	}
}

// Retained event values must survive later cleanup chunks, while the retired
// lease releases the bulk publication and the ledger's backing storage.
func TestRouteViewOutputCleanupReleasesOwnedStorage(t *testing.T) {
	redistevents.RegisterProtocol("static")
	var authority routeViewAuthority
	owner, opened := routeViewOpenTest(t, &authority, locrib.NewRIB())
	page := routeViewReadTest(t, &authority, owner, opened.Session, 0, 1)
	bus := newTestEventBus()
	var withdrawn []*sysribevents.BestChangeBatch
	sysribevents.BestChange.Subscribe(bus, func(batch *sysribevents.BestChangeBatch) {
		if len(batch.Changes) > 0 && batch.Changes[0].Action == routeaction.Withdraw {
			withdrawn = append(withdrawn, batch)
		}
	})
	const count = 2*rpc.RouteViewRecordsMax + 17
	for start, operation := 0, uint64(1); start < count; operation++ {
		end := min(start+rpc.RouteViewRecordsMax, count)
		batch := sysribevents.BestChangeBatch{Family: family.IPv4Unicast, Changes: make([]sysribevents.BestChangeEntry, end-start)}
		for i := start; i < end; i++ {
			addr := netip.AddrFrom4([4]byte{10, byte(i >> 16), byte(i >> 8), byte(i)})
			batch.Changes[i-start] = sysribevents.BestChangeEntry{Action: routeaction.Add, Prefix: netip.PrefixFrom(addr, 32), Protocol: "static", RouteType: routetype.Blackhole, TableID: 254}
		}
		input := routeViewPublication{Session: opened.Session, Cursor: page.Cursor, Operation: operation, Batch: batch}
		if _, err := authority.publish(t.Context(), owner, input, bus); err != nil {
			t.Fatal(err)
		}
		start = end
	}
	var output *routeViewOutput
	if err := authority.withSession(t.Context(), owner, opened.Session, func(session *routeViewSession) error {
		output = session.output
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if output == nil || output.acknowledged == nil || len(output.ledger) != count {
		t.Fatal("the live lease did not retain its acknowledged output")
	}
	if _, err := authority.close(owner, opened.Session); err != nil {
		t.Fatal(err)
	}
	seen := make(map[netip.Prefix]bool)
	for _, batch := range withdrawn {
		if batch.Family != family.IPv4Unicast || len(batch.Changes) == 0 || len(batch.Changes) > rpc.RouteViewRecordsMax {
			t.Fatalf("cleanup emitted an invalid bounded family chunk: family=%v count=%d", batch.Family, len(batch.Changes))
		}
		for _, entry := range batch.Changes {
			if entry.Action != routeaction.Withdraw || entry.TableID != 254 || seen[entry.Prefix] {
				t.Fatalf("retained cleanup values were changed or repeated: %+v", entry)
			}
			seen[entry.Prefix] = true
		}
	}
	if len(seen) != count {
		t.Fatalf("cleanup withdrew %d distinct owned prefixes, want %d", len(seen), count)
	}
	if output.acknowledged != nil {
		t.Error("retired output retained its acknowledged publication bulk")
	}
	if output.ledger != nil {
		t.Error("retired output retained its empty ledger backing storage")
	}
}

// A retained operation remains retryable after a later completed view read.
// New output must still use that newly consumed cursor.
func TestRouteViewOutputRetryAfterCursorAdvance(t *testing.T) {
	protocol := redistevents.RegisterProtocol("static")
	rib := locrib.NewRIB()
	var authority routeViewAuthority
	owner, opened := routeViewOpenTest(t, &authority, rib)
	page := routeViewReadTest(t, &authority, owner, opened.Session, 0, 1)
	bus := newTestEventBus()
	input := routeViewPublication{Session: opened.Session, Cursor: page.Cursor, Operation: 1, Batch: routeViewOutputBatch()}
	ack, err := authority.publish(t.Context(), owner, input, bus)
	if err != nil {
		t.Fatal(err)
	}
	rib.Insert(family.IPv4Unicast, netip.MustParsePrefix("203.0.113.0/24"), locrib.Path{Source: protocol, RouteType: routetype.Blackhole})
	next := routeViewReadTest(t, &authority, owner, opened.Session, page.Cursor, rpc.RouteViewRecordsMax)
	if !next.Complete || next.Cursor <= page.Cursor {
		t.Fatalf("the control did not advance the completed reader: %+v", next)
	}
	retried, err := authority.publish(t.Context(), owner, input, bus)
	if err != nil || retried != ack || len(bus.events) != 1 {
		t.Fatalf("cursor advance lost the retained original ACK or duplicated output: ack=%+v err=%v emissions=%d", retried, err, len(bus.events))
	}
	input.Operation++
	if _, err := authority.publish(t.Context(), owner, input, bus); err == nil {
		t.Fatal("new output accepted the obsolete cursor")
	}
	input.Cursor = next.Cursor
	if _, err := authority.publish(t.Context(), owner, input, bus); err != nil {
		t.Fatalf("the fresh-cursor control failed: %v", err)
	}
}
