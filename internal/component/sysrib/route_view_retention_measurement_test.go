// Design: docs/architecture/rib/unified-locrib.md -- bounded external retention under a blocked transport.

package sysrib

import (
	"encoding/json"
	"net"
	"net/netip"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	sysribevents "github.com/ze-software/ze/internal/component/sysrib/events"
	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/internal/core/rib/routetype"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// One armed write holds a real MuxConn response. The test MUST release it or
// close the connection, and MUST join the SDK request before returning.
type routeViewMeasurementWriteGate struct {
	net.Conn
	mu      sync.Mutex
	entered chan struct{}
	release chan struct{}
	stopped chan struct{}
	stop    sync.Once
}

func (gate *routeViewMeasurementWriteGate) arm() (<-chan struct{}, chan<- struct{}) {
	gate.mu.Lock()
	defer gate.mu.Unlock()
	gate.entered, gate.release = make(chan struct{}), make(chan struct{})
	return gate.entered, gate.release
}

func (gate *routeViewMeasurementWriteGate) Write(value []byte) (int, error) {
	gate.mu.Lock()
	entered, release := gate.entered, gate.release
	gate.entered = nil
	gate.mu.Unlock()
	if entered != nil {
		close(entered)
		select {
		case <-release:
		case <-gate.stopped:
			return 0, net.ErrClosed
		}
	}
	return gate.Conn.Write(value)
}

func (gate *routeViewMeasurementWriteGate) Close() error {
	gate.stop.Do(func() { close(gate.stopped) })
	return gate.Conn.Close()
}

func routeViewMeasurementPrefix(index int) netip.Prefix {
	return netip.PrefixFrom(netip.AddrFrom4([4]byte{10, 201, byte(index >> 8), byte(index)}), 32)
}

func routeViewMeasurementTableSize(table projectionTable) int {
	count := 0
	for _, values := range table {
		count += values.Len()
	}
	return count
}

// The complete test table is 8209 selected prefixes, deliberately larger than
// two capture windows. This measures retained structures, not process RSS or
// throughput. The live TLS/kernel companion also stops the actual subprocess.
func TestRouteViewBlockedTransportRetention(t *testing.T) {
	const population = 2*rpc.RouteViewRecordsMax + 17
	previousMaintenance := routeViews.maintenance
	routeViews.maintenance = make(chan routeViewMaintenance, 1)
	t.Cleanup(func() { routeViews.maintenance = previousMaintenance })
	var gate *routeViewMeasurementWriteGate
	endpoint := newRouteViewMeasurementEndpoint(t, false, func(conn net.Conn) net.Conn {
		gate = &routeViewMeasurementWriteGate{Conn: conn, stopped: make(chan struct{})}
		return gate
	})
	rib := locrib.Default()
	if rib == nil {
		t.Fatal("retention measurement has no daemon Loc-RIB")
	}
	source := redistevents.RegisterProtocol("static")
	control := netip.MustParsePrefix("198.19.249.0/24")
	path := locrib.Path{Source: source, RouteType: routetype.Blackhole, Metric: 1}
	t.Cleanup(func() {
		if err := routeViews.cleanup(endpoint.owner); err != nil {
			t.Error(err)
		}
		rib.Remove(family.IPv4Unicast, control, source, 0)
		for index := range population {
			rib.Remove(family.IPv4Unicast, routeViewMeasurementPrefix(index), source, 0)
		}
	})
	rib.Insert(family.IPv4Unicast, control, path)
	var added, withdrawn atomic.Int32
	unsubscribe := sysribevents.BestChange.Subscribe(endpoint.host, func(batch *sysribevents.BestChangeBatch) {
		for _, entry := range batch.Changes {
			if entry.Prefix != control {
				continue
			}
			if entry.Action == routeaction.Add {
				added.Add(1)
			}
			if entry.Action == routeaction.Withdraw {
				withdrawn.Add(1)
			}
		}
	})
	t.Cleanup(unsubscribe)
	opened, err := endpoint.plugin.RouteViewOpen(endpoint.ctx, &rpc.RouteViewOpenInput{})
	if err != nil {
		t.Fatal(err)
	}
	projection, err := newSelectedProjection(*opened)
	if err != nil {
		t.Fatal(err)
	}
	page, err := endpoint.plugin.RouteViewRead(endpoint.ctx, &rpc.RouteViewReadInput{Session: opened.Session, Cursor: 0, Limit: rpc.RouteViewRecordsMax})
	if err != nil {
		t.Fatal(err)
	}
	if !page.Complete {
		t.Fatal("initial useful table was not complete")
	}
	if _, err := projection.Apply(page); err != nil {
		t.Fatal(err)
	}
	batch := sysribevents.BestChangeBatch{Family: family.IPv4Unicast, Changes: []sysribevents.BestChangeEntry{{Action: routeaction.Add, Prefix: control, Protocol: "static", RouteType: routetype.Blackhole, Metric: 1}}}
	wire, err := json.Marshal(batch)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := endpoint.plugin.RouteViewPublish(endpoint.ctx, &rpc.RouteViewPublishInput{Session: opened.Session, Cursor: page.Cursor, Operation: 1, Batch: wire}); err != nil {
		t.Fatal(err)
	}
	if added.Load() != 1 {
		t.Fatal("retention measurement never delivered its useful output control")
	}
	entered, release := gate.arm()
	type readResult struct {
		page *rpc.RouteViewReadOutput
		err  error
	}
	readDone := make(chan readResult, 1)
	// One lifecycle request spans the controlled transport hold. Its response
	// is joined below before any replacement session is opened.
	go func() {
		value, err := endpoint.plugin.RouteViewRead(endpoint.ctx, &rpc.RouteViewReadInput{Session: opened.Session, Cursor: page.Cursor, Limit: 257})
		readDone <- readResult{page: value, err: err}
	}()
	readJoined := false
	t.Cleanup(func() {
		if readJoined {
			return
		}
		_ = gate.Close()            //nolint:errcheck // Failure cleanup must unblock a response even if its peer already closed.
		_ = endpoint.plugin.Close() //nolint:errcheck // Failure cleanup must cancel the outstanding SDK call.
		select {
		case <-readDone:
		case <-time.After(10 * time.Second):
			t.Error("blocked SDK request did not join after connection cleanup")
		}
	})
	select {
	case <-entered:
	case <-endpoint.ctx.Done():
		t.Fatal(endpoint.ctx.Err())
	}
	for index := range population {
		rib.Insert(family.IPv4Unicast, routeViewMeasurementPrefix(index), path)
	}
	path.Metric = 9
	rib.Insert(family.IPv4Unicast, routeViewMeasurementPrefix(0), path)
	rib.Remove(family.IPv4Unicast, control, source, 0)
	// Production callbacks have returned although the response cannot write.
	select {
	case <-readDone:
		t.Fatal("blocked response escaped before release")
	default:
	}
	if err := routeViews.withSession(endpoint.ctx, endpoint.owner, opened.Session, func(session *routeViewSession) error {
		if cap(session.capture) != rpc.RouteViewRecordsMax {
			t.Fatalf("capture cap=%d", cap(session.capture))
		}
		if len(session.capture) > rpc.RouteViewRecordsMax {
			t.Fatalf("capture records=%d", len(session.capture))
		}
		if len(session.snapshot) != 0 {
			t.Fatal("completed initial snapshot retained selected paths")
		}
		if len(session.journal) > rpc.RouteViewRecordsMax {
			t.Fatal("blocked transport exceeded journal records")
		}
		if session.journalBytes > routeViewJournalBytesMax {
			t.Fatal("blocked transport exceeded journal bytes")
		}
		if session.output == nil {
			t.Fatal("useful publication never established its output ledger")
		}
		if len(session.output.ledger) > 1 {
			t.Fatal("output ledger retained undelivered table entries")
		}
		if ack := session.output.acknowledged; ack != nil {
			if len(ack.input.Batch.Changes) != 1 {
				t.Fatal("ACK retention grew beyond the last actual publication")
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	routeViewMaintainTest(t, &routeViews, time.Now())
	if withdrawn.Load() != 1 {
		t.Fatalf("overflow did not fence the proved output while transport was held: %d", withdrawn.Load())
	}
	close(release)
	select {
	case result := <-readDone:
		readJoined = true
		if result.err != nil {
			t.Fatal(result.err)
		}
		if _, err := projection.Apply(result.page); err != nil {
			t.Fatal(err)
		}
	case <-endpoint.ctx.Done():
		t.Fatal(endpoint.ctx.Err())
	}
	reset, err := endpoint.plugin.RouteViewRead(endpoint.ctx, &rpc.RouteViewReadInput{Session: opened.Session, Cursor: projection.Cursor(), Limit: 257})
	if err != nil {
		t.Fatal(err)
	}
	if reset.Phase != rpc.RouteViewPhaseReset {
		t.Fatalf("overflow became a successful empty page: %+v", reset)
	}
	if _, err := projection.Apply(reset); err != nil {
		t.Fatal(err)
	}
	if projection.current != nil {
		t.Fatal("reset retained the recipient's old current table")
	}
	if projection.staging != nil {
		t.Fatal("reset retained the recipient's old staging table")
	}
	if _, err := endpoint.plugin.RouteViewClose(endpoint.ctx, &rpc.RouteViewCloseInput{Session: opened.Session}); err != nil {
		t.Fatal(err)
	}
	opened, err = endpoint.plugin.RouteViewOpen(endpoint.ctx, &rpc.RouteViewOpenInput{})
	if err != nil {
		t.Fatal(err)
	}
	projection, err = newSelectedProjection(*opened)
	if err != nil {
		t.Fatal(err)
	}
	seen := 0
	for !projection.Synchronized() {
		page, err := endpoint.plugin.RouteViewRead(endpoint.ctx, &rpc.RouteViewReadInput{Session: opened.Session, Cursor: projection.Cursor(), Limit: 257})
		if err != nil {
			t.Fatal(err)
		}
		encoded, err := json.Marshal(page)
		if err != nil {
			t.Fatal(err)
		}
		if len(encoded) > rpc.RouteViewResultBudget() {
			t.Fatalf("in-flight reply bytes=%d", len(encoded))
		}
		if len(page.Entries) > 257 {
			t.Fatalf("in-flight reply rows=%d", len(page.Entries))
		}
		prior := seen
		seen += len(page.Entries)
		if _, err := projection.Apply(page); err != nil {
			t.Fatal(err)
		}
		if err := routeViews.withSession(endpoint.ctx, endpoint.owner, opened.Session, func(session *routeViewSession) error {
			if page.Complete {
				if session.snapshot != nil {
					t.Fatal("completed snapshot still retained its table")
				}
			} else {
				if len(session.snapshot) != population {
					t.Fatalf("snapshot records=%d want=%d", len(session.snapshot), population)
				}
				if cap(session.snapshot) != population {
					t.Fatalf("snapshot capacity=%d want=%d", cap(session.snapshot), population)
				}
				for index := prior; index < seen; index++ {
					if !reflect.DeepEqual(session.snapshot[index], rpc.RouteViewEntry{}) {
						t.Fatal("consumed snapshot cell retained its path")
					}
				}
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		if routeViewMeasurementTableSize(projection.staging)+routeViewMeasurementTableSize(projection.current) != seen {
			t.Fatal("projection duplicated or lost a paged selected value")
		}
	}
	if seen != population {
		t.Fatalf("reconstruction read only %d of %d selected entries", seen, population)
	}
	if routeViewMeasurementTableSize(projection.current) != population {
		t.Fatalf("reconstruction retained only %d selected entries", routeViewMeasurementTableSize(projection.current))
	}
	for index := range population {
		prefix := routeViewMeasurementPrefix(index)
		got, selected, exists := projection.LPM(family.IPv4Unicast, prefix.Addr())
		want := uint32(1)
		if index == 0 {
			want = 9
		}
		if !exists {
			t.Fatalf("current full table lost %s", prefix)
		}
		if selected != prefix {
			t.Fatalf("current full table used %s to cover %s", selected, prefix)
		}
		if got.Metric != want {
			t.Fatalf("current full table metric at %s=%d want=%d", prefix, got.Metric, want)
		}
		if got.RouteType != routetype.Blackhole {
			t.Fatalf("current full table route type at %s=%d", prefix, got.RouteType)
		}
	}
	if _, _, exists := projection.LPM(family.IPv4Unicast, control.Addr()); exists {
		t.Fatal("reconstruction resurrected the withdrawn control")
	}
	t.Logf("retained-state measurement: selected=%d capture-cap=%d journal-cap=%d journal-byte-cap=%d; complete current table reconstructed after blocked-write overflow", population, rpc.RouteViewRecordsMax, rpc.RouteViewRecordsMax, routeViewJournalBytesMax)
}

// Real wide, valid selected entries fill the journal byte budget before its
// record limit. No fabricated journalBytes value substitutes for owned data.
func TestRouteViewPopulatedJournalByteRetention(t *testing.T) {
	rib := locrib.NewRIB()
	source := redistevents.RegisterProtocol("static")
	authority := routeViewAuthority{maintenance: make(chan routeViewMaintenance, 1)}
	owner, opened := routeViewOpenTest(t, &authority, rib)
	initial := routeViewReadTest(t, &authority, owner, opened.Session, 0, 1)
	if !initial.Complete {
		t.Fatal("empty initial selection did not synchronize")
	}
	fences := 0
	if err := authority.withSession(t.Context(), owner, opened.Session, func(session *routeViewSession) error { return session.setFence(func() error { fences++; return nil }) }); err != nil {
		t.Fatal(err)
	}
	path := locrib.Path{Source: source, NextHop: netip.MustParseAddr("192.0.2.1"), Interface: "port1", OnLink: true, ECMP: make([]locrib.NextHop, 129)}
	for index := range path.ECMP {
		path.ECMP[index] = locrib.NextHop{Addr: netip.AddrFrom4([4]byte{192, 0, 2, byte(index + 2)}), Interface: strings.Repeat("p", 256), OnLink: true, Weight: 1}
	}
	retained, records := 0, 0
	for index := range rpc.RouteViewRecordsMax {
		prefix := routeViewMeasurementPrefix(index)
		rib.Insert(family.IPv4Unicast, prefix, path)
		entry, err := routeViewInspect(rib, overflowKey{fam: family.IPv4Unicast, prefix: prefix})
		if err != nil {
			t.Fatal(err)
		}
		if err := validateProjectionPath(&entry.Path); err != nil {
			t.Fatalf("wide selected control is invalid: %v", err)
		}
		if len(entry.Path.ECMP) != 129 {
			t.Fatalf("wide selected control lost its targets: %d", len(entry.Path.ECMP))
		}
		encoded, err := json.Marshal(entry)
		if err != nil {
			t.Fatal(err)
		}
		if len(encoded) >= rpc.RouteViewResultBudget() {
			t.Fatalf("wide selected control cannot cross one frame: %d", len(encoded))
		}
		bytes := routeViewEntryBytes(&entry)
		crosses := bytes > routeViewJournalBytesMax-retained
		err = authority.withSession(t.Context(), owner, opened.Session, func(session *routeViewSession) error {
			_, syncErr := session.synchronize(t.Context())
			if crosses {
				if syncErr == nil {
					t.Fatal("byte overflow was admitted")
				}
				if !strings.Contains(syncErr.Error(), "byte bound") {
					t.Fatalf("byte overflow was misclassified: %v", syncErr)
				}
				if session.journal != nil {
					t.Fatal("byte overflow retained journal records")
				}
				if session.journalBytes != 0 {
					t.Fatal("byte overflow retained accounted journal bytes")
				}
				if session.snapshot != nil {
					t.Fatal("byte overflow retained a snapshot")
				}
				if session.ready {
					t.Fatal("byte overflow retained forwarding readiness")
				}
				return nil
			}
			if syncErr != nil {
				return syncErr
			}
			sum := 0
			for _, record := range session.journal {
				sum += record.bytes
			}
			last := session.journal[len(session.journal)-1]
			if last.bytes != routeViewEntryBytes(&last.entry) {
				t.Fatal("new journal record bytes differ from its owned representation")
			}
			if !reflect.DeepEqual(last.entry, entry) {
				t.Fatal("new journal record differs from the selected input")
			}
			if len(session.journal) != records+1 {
				t.Fatal("journal record count differs from its inputs")
			}
			if sum != retained+bytes {
				t.Fatal("journal byte sum differs from accepted selected inputs")
			}
			if sum != session.journalBytes {
				t.Fatal("journal byte accounting differs from retained records")
			}
			if sum > routeViewJournalBytesMax {
				t.Fatal("journal exceeded its byte cap")
			}
			retained, records = sum, len(session.journal)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		if crosses {
			if records == 0 {
				t.Fatal("byte-bound proof admitted no useful journal control")
			}
			if records >= rpc.RouteViewRecordsMax {
				t.Fatal("record overflow preceded byte overflow")
			}
			if fences != 1 {
				t.Fatalf("byte overflow fenced %d times", fences)
			}
			page := routeViewReadTest(t, &authority, owner, opened.Session, initial.Cursor, 1)
			if page.Phase != rpc.RouteViewPhaseReset {
				t.Fatal("byte overflow did not require an explicit reset")
			}
			t.Logf("retained-byte measurement: accepted-records=%d accepted-encoded-bytes=%d next-record-bytes=%d cap=%d", records, retained, bytes, routeViewJournalBytesMax)
			return
		}
	}
	t.Fatal("wide selected rows reached the record cap without proving the byte cap")
}
