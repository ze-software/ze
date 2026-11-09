package l2tppool

import (
	"net/netip"
	"slices"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/dnssource"
)

// TestPoolPublishesSubscriberSelection proves the pool plugin publishes each
// running pool's subscriber list for show system dns and ze doctor: once
// when it joins the bus, again when a fresh snapshot changes the answer, and
// never for an unchanged one. RADIUS is per session, so the published
// selection carries no radius server. Method: a pool with no dns-primary on
// an in-process bus, the typed publication handle, and two snapshots.
func TestPoolPublishesSubscriberSelection(t *testing.T) {
	bus := newReleaseBus()
	var published []*dnssource.Publication
	dnssource.PublicationEvent.Subscribe(bus, func(p *dnssource.Publication) { published = append(published, p) })

	p := &poolPlugin{pool: newIPv4Pool(testGW, netip.MustParseAddr("10.0.0.1"),
		netip.MustParseAddr("10.0.0.10"), netip.Addr{}, netip.Addr{})}
	p.setEventBus(bus)
	if len(published) != 1 {
		t.Fatalf("joining the bus published %d sets, want 1", len(published))
	}
	if published[0].Consumer != poolConsumer || len(published[0].Selections) != 1 {
		t.Fatalf("published %+v, want one selection from %q", published[0], poolConsumer)
	}
	first := published[0].Selections[0]
	if first.Path != poolDNSSourcePath || first.Instance != "" || !first.Default || len(first.Servers) != 0 {
		t.Fatalf("before any lease the default pool published %+v, want no server at %q", first, poolDNSSourcePath)
	}

	learned := netip.MustParseAddr("192.0.2.53")
	snapshot := &dnssource.Snapshot{Sequence: 1, Records: []dnssource.Record{{
		Origin: "dhcpv4", Rank: 100, Interface: "wan0", Unit: "0",
		Addresses: []netip.Addr{learned}, Received: time.Now(), Expires: time.Now().Add(time.Hour),
	}}}
	if _, err := dnssource.SnapshotEvent.Emit(bus, snapshot); err != nil {
		t.Fatalf("emit snapshot: %v", err)
	}
	if len(published) != 2 {
		t.Fatalf("a fresh snapshot published %d sets in total, want 2", len(published))
	}
	got := published[1].Selections[0]
	if !slices.Equal(got.Servers, []netip.Addr{learned}) || slices.Contains(got.Used, dnssource.ClassRadius) {
		t.Fatalf("after the lease the pool published %+v, want the learned %s and no radius", got, learned)
	}

	p.publishPools(bus)
	if len(published) != 2 {
		t.Fatalf("an unchanged selection was published again: %d sets", len(published))
	}
}

// TestPoolWithdrawPublishesEmptySelection proves withdraw, called directly,
// sends one withdrawn publication with no selection once the pool plugin has
// published, that nothing is published after it, and that a second
// setEventBus publishes the pool's selection again (published[2]). Method: a
// pool on an in-process bus, call withdraw, then a fresh snapshot and a
// config publish that would each publish a changed set, then a second
// setEventBus. It does not drive the registered OnBye handler; the
// daemon-level proof that a removed pool leaves show system dns is
// test/plugin/show-system-dns-withdraw-l2tp-pool.ci.
//
// VALIDATES: spec-home-cpe-23b, the withdrawal body, the silence after it,
// and the publication a new run sends.
// PREVENTS: a withdrawn pool plugin publishing its selections again.
func TestPoolWithdrawPublishesEmptySelection(t *testing.T) {
	bus := newReleaseBus()
	var published []*dnssource.Publication
	dnssource.PublicationEvent.Subscribe(bus, func(p *dnssource.Publication) { published = append(published, p) })

	p := &poolPlugin{pool: newIPv4Pool(testGW, netip.MustParseAddr("10.0.0.1"),
		netip.MustParseAddr("10.0.0.10"), netip.Addr{}, netip.Addr{})}
	p.setEventBus(bus)
	if len(published) != 1 {
		t.Fatalf("joining the bus published %d sets, want 1", len(published))
	}

	p.withdraw()
	if len(published) != 2 {
		t.Fatalf("withdraw sent %d publications in total, want 2", len(published))
	}
	withdrawal := published[1]
	if withdrawal.Consumer != poolConsumer || !withdrawal.Withdrawn || len(withdrawal.Selections) != 0 {
		t.Fatalf("withdrawal = %+v, want a withdrawn %q with no selection", withdrawal, poolConsumer)
	}

	snapshot := &dnssource.Snapshot{Sequence: 1, Records: []dnssource.Record{{
		Origin: "dhcpv4", Rank: 100, Interface: "wan0", Unit: "0",
		Addresses: []netip.Addr{netip.MustParseAddr("192.0.2.53")}, Received: time.Now(), Expires: time.Now().Add(time.Hour),
	}}}
	if _, err := dnssource.SnapshotEvent.Emit(bus, snapshot); err != nil {
		t.Fatalf("emit snapshot: %v", err)
	}
	p.publishPools(bus)
	if len(published) != 2 {
		t.Fatalf("a withdrawn pool plugin published again: %d publications, want 2: %+v", len(published), published[2:])
	}

	p.setEventBus(bus)
	if len(published) != 3 {
		t.Fatalf("a new run published %d publications in total, want 3", len(published))
	}
	if published[2].Withdrawn || len(published[2].Selections) != 1 {
		t.Fatalf("a new run published %+v, want the pool's selection again", published[2])
	}
}
