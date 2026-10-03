// sFlow v5 counter polling schedule, driven through the exporter's snapshot
// path (notifySnapshot) with a real Sender to a loopback UDP collector.
//
// VALIDATES: the polling interval is a maximum. With the iface rate tracker's
// one-second snapshots arriving slightly late, no gap between two counter
// datagrams exceeds the interval, each datagram carries every interface of its
// snapshot, and a snapshot that finds the exporter busy still polls.
// PREVENTS: a poll taken only once elapsed >= interval, which lands one tick
// late, and a busy exporter silently dropping the due poll.

package flowexport

import (
	"context"
	"encoding/binary"
	"net"
	"testing"
	"time"
)

// ifIndexListEncoder sends one datagram per poll that lists the IfIndex of
// every interface in the snapshot, four octets each, so the collector side can
// see which counters a poll carried.
type ifIndexListEncoder struct{}

func (ifIndexListEncoder) Encode(snap CounterSnapshot, sender *Sender) (int, error) {
	buf := make([]byte, 4*len(snap.Interfaces))
	for i := range snap.Interfaces {
		binary.BigEndian.PutUint32(buf[4*i:], snap.Interfaces[i].IfIndex)
	}
	return len(snap.Interfaces), sender.Send(buf)
}

func (ifIndexListEncoder) EncodeTemplate(_ *Sender) error { return nil }

// newPollingExporter builds an exporter with one sflow collector at the given
// polling interval, wired to ifIndexListEncoder, and returns the collector socket.
func newPollingExporter(t *testing.T, interval int) (*exporter, net.PacketConn) {
	t.Helper()
	var lc net.ListenConfig
	pc, err := lc.ListenPacket(context.Background(), "udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = pc.Close() })
	addr, ok := pc.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatal("unexpected address type")
	}
	exp, err := newExporter(&Config{Collectors: []CollectorConfig{{
		Name: "c1", Address: "127.0.0.1", Port: addr.Port, Protocol: "sflow",
		PollingInterval: interval, TemplateRefresh: 600, MaxDatagramSize: DatagramSizeDefault,
	}}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(exp.stop)
	exp.setEncoder("c1", ifIndexListEncoder{})
	return exp, pc
}

// readIfIndexes reads one datagram from pc, or returns nil when none arrives
// within a short deadline.
func readIfIndexes(t *testing.T, pc net.PacketConn) []uint32 {
	t.Helper()
	if err := pc.SetReadDeadline(time.Now().Add(50 * time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 1500)
	n, _, err := pc.ReadFrom(buf)
	if err != nil {
		return nil
	}
	got := make([]uint32, 0, n/4)
	for off := 0; off+4 <= n; off += 4 {
		got = append(got, binary.BigEndian.Uint32(buf[off:]))
	}
	return got
}

var pollingInterfaces = []InterfaceCounters{{IfIndex: 1}, {IfIndex: 2}, {IfIndex: 3}}

// RFC requirement: SFLOW-V5-x-14 positive -- with a 5 s polling interval and
// snapshots every 1.01 s for 30 s, a counter datagram is sent at least once in
// every 5 s: no gap between two polls exceeds the interval.
// RFC requirement: SFLOW-V5-x-14 negative -- the same late-tick schedule under
// a poll taken only once elapsed >= interval leaves a 5.05 s gap; the test
// requires every gap to stay at or under 5 s, so that schedule is refused.
func TestSFlowV5CounterGapNeverExceedsInterval(t *testing.T) {
	const interval = 5 * time.Second
	exp, pc := newPollingExporter(t, 5)

	t0 := time.Now()
	var lastPoll time.Time
	polls := 0
	for step := range 30 {
		now := t0.Add(time.Duration(step) * 1010 * time.Millisecond)
		exp.notifySnapshot(CounterSnapshot{Time: now, Interfaces: pollingInterfaces})
		if readIfIndexes(t, pc) == nil {
			continue
		}
		if !lastPoll.IsZero() && now.Sub(lastPoll) > interval {
			t.Errorf("poll at %v follows the previous one by %v, over the %v maximum", now.Sub(t0), now.Sub(lastPoll), interval)
		}
		lastPoll = now
		polls++
	}
	if polls < 6 {
		t.Fatalf("polls = %d over 30 s at a 5 s maximum, want at least 6", polls)
	}
}

// RFC requirement: SFLOW-V5-x-26 positive -- when the maximum interval is due,
// the datagram sent carries the counters of every interface in the snapshot
// (IfIndex 1, 2 and 3), at every due poll over 30 s of late 1.01 s ticks.
// RFC requirement: SFLOW-V5-x-26 negative -- a snapshot that arrives while the
// exporter is busy (its mutex held) at the due time is not dropped: the
// datagram with the outstanding counters is sent once the exporter is free.
func TestSFlowV5DueCountersAreSent(t *testing.T) {
	exp, pc := newPollingExporter(t, 5)

	t0 := time.Now()
	for step := range 30 {
		now := t0.Add(time.Duration(step) * 1010 * time.Millisecond)
		exp.notifySnapshot(CounterSnapshot{Time: now, Interfaces: pollingInterfaces})
		got := readIfIndexes(t, pc)
		if got == nil {
			continue
		}
		if len(got) != 3 || got[0] != 1 || got[1] != 2 || got[2] != 3 {
			t.Fatalf("step %d: poll carried interfaces %v, want [1 2 3]", step, got)
		}
	}

	// The next snapshot is due (well past the interval). Hold the exporter
	// while it arrives, as a flow export or a status read would.
	due := t0.Add(60 * time.Second)
	exp.mu.Lock()
	done := make(chan struct{})
	go func() {
		exp.notifySnapshot(CounterSnapshot{Time: due, Interfaces: pollingInterfaces})
		close(done)
	}()
	time.Sleep(20 * time.Millisecond)
	exp.mu.Unlock()
	<-done
	got := readIfIndexes(t, pc)
	if len(got) != 3 {
		t.Fatalf("due snapshot met a busy exporter: poll carried %v, want the outstanding counters [1 2 3]", got)
	}
}
