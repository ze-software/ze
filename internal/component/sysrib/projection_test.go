// Design: docs/architecture/rib/unified-locrib.md -- selected-route projection.
// Related: projection.go -- atomic snapshot and delta application.

package sysrib

import (
	"net/netip"
	"reflect"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func projectionFixture(t *testing.T) *selectedProjection {
	t.Helper()
	redistevents.RegisterProtocol("projection-test")
	p, err := newSelectedProjection(rpc.RouteViewOpenOutput{Session: "session", Epoch: "epoch", Snapshot: "snapshot"})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// Complete ECMP groups and all presence and repair metadata survive owned conversion.
func TestSelectedProjectionMetadata(t *testing.T) {
	p := projectionFixture(t)
	entry := projectionEntry("192.0.2.0/24")
	entry.Path.Instance = 9
	entry.Path.Distance = 120
	entry.Path.Weight = 3
	entry.Path.Labels = []uint32{100, 200}
	entry.Path.SRv6SID = netip.MustParseAddr("2001:db8::f")
	entry.Path.HasTag = true
	entry.Path.AIGPPresent = true
	entry.Path.IsBGP = true
	entry.Path.BackupNextHop = netip.MustParseAddr("198.51.100.2")
	entry.Path.BackupInterface = "backup0"
	entry.Path.BackupOnLink = true
	entry.Path.BackupRepairLabels = []uint32{300}
	for range 129 {
		entry.Path.ECMP = append(entry.Path.ECMP, rpc.RouteViewNextHop{Interface: "member0", OnLink: true, Weight: 5, Labels: []uint32{400}, SRv6SID: netip.MustParseAddr("2001:db8::e")})
	}
	if _, err := p.Apply(projectionPage(2, rpc.RouteViewPhaseSnapshot, true, entry)); err != nil {
		t.Fatal(err)
	}
	// Mutate the caller's slices after admission to expose a retained wire alias.
	entry.Path.Labels[0] = 999
	entry.Path.BackupRepairLabels[0] = 999
	entry.Path.ECMP[0].Labels[0] = 999
	entry.Path.ECMP[0].Interface = "changed"
	seen := p.Inspect(family.IPv4Unicast, entry.Prefix, func(value *selectedProjectionPath) {
		path := &value.path
		if path.Instance != 9 || path.AdminDistance != 120 || path.Weight != 3 || !path.HasTag || path.Tag != 0 || !path.AIGPPresent || path.AIGP != 0 || !path.IsBGP {
			t.Fatalf("metadata lost: %+v", path)
		}
		if !reflect.DeepEqual(path.Labels, []uint32{100, 200}) || !reflect.DeepEqual(path.BackupRepairLabels, []uint32{300}) {
			t.Fatal("label ownership lost")
		}
		if path.SRv6SID != netip.MustParseAddr("2001:db8::f") || path.BackupNextHop != netip.MustParseAddr("198.51.100.2") || path.BackupInterface != "backup0" || !path.BackupOnLink {
			t.Fatal("SID or repair metadata lost")
		}
		if len(path.ECMP) != 129 || len(value.members) != 129 {
			t.Fatalf("large ECMP group truncated: %d, %d", len(path.ECMP), len(value.members))
		}
		if path.ECMP[0].Interface != "member0" || path.ECMP[0].Weight != 5 || !path.ECMP[0].OnLink {
			t.Fatal("member metadata lost")
		}
		if !reflect.DeepEqual(value.members[0].labels, []uint32{400}) || value.members[0].sid != netip.MustParseAddr("2001:db8::e") {
			t.Fatal("member extension metadata lost")
		}
	})
	if !seen {
		t.Fatal("selected path unavailable for replay")
	}
}

// Invalid page identities and fields leave the complete selected table readable.
func TestSelectedProjectionRejectsMalformedPage(t *testing.T) {
	cases := []struct {
		name   string
		change func(*rpc.RouteViewReadOutput)
	}{
		{"foreign session", func(p *rpc.RouteViewReadOutput) { p.Session = "foreign" }},
		{"foreign epoch", func(p *rpc.RouteViewReadOutput) { p.Epoch = "foreign" }},
		{"stale cursor", func(p *rpc.RouteViewReadOutput) { p.Cursor = 1 }},
		{"future cursor", func(p *rpc.RouteViewReadOutput) { p.Cursor = 99 }},
		{"family mismatch", func(p *rpc.RouteViewReadOutput) { p.Entries[0].AFI = uint16(family.AFIIPv6) }},
		{"unknown safi", func(p *rpc.RouteViewReadOutput) { p.Entries[0].SAFI = 255 }},
		{"unmasked prefix", func(p *rpc.RouteViewReadOutput) { p.Entries[0].Prefix = netip.MustParsePrefix("192.0.2.1/24") }},
		{"unknown action", func(p *rpc.RouteViewReadOutput) { p.Entries[0].Action = 255 }},
		{"unknown route action", func(p *rpc.RouteViewReadOutput) { p.Entries[0].Path.RouteType = 255 }},
		{"bad sid", func(p *rpc.RouteViewReadOutput) { p.Entries[0].Path.SRv6SID = netip.MustParseAddr("192.0.2.1") }},
		{"empty member", func(p *rpc.RouteViewReadOutput) { p.Entries[0].Path.ECMP = []rpc.RouteViewNextHop{{}} }},
		{"oversized page", func(p *rpc.RouteViewReadOutput) {
			p.Entries[0].Path.Interface = strings.Repeat("x", rpc.MaxMessageSize)
		}},
		{"too many records", func(p *rpc.RouteViewReadOutput) {
			for len(p.Entries) <= rpc.RouteViewRecordsMax {
				p.Entries = append(p.Entries, projectionEntry("198.51.100.0/24"))
			}
			p.Cursor = uint64(len(p.Entries)) + 2
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := projectionFixture(t)
			if _, err := p.Apply(projectionPage(2, rpc.RouteViewPhaseSnapshot, true, projectionEntry("192.0.2.0/24"))); err != nil {
				t.Fatal(err)
			}
			page := projectionPage(3, rpc.RouteViewPhaseDelta, true, projectionEntry("198.51.100.0/24"))
			tc.change(page)
			if _, err := p.Apply(page); err == nil {
				t.Fatal("malformed page accepted")
			}
			if !p.Synchronized() {
				t.Fatal("rejected page destroyed current table")
			}
			if _, _, found := p.LPM(family.IPv4Unicast, netip.MustParseAddr("192.0.2.1")); !found {
				t.Fatal("current route lost")
			}
			if _, _, found := p.LPM(family.IPv4Unicast, netip.MustParseAddr("198.51.100.1")); found {
				t.Fatal("rejected page published route")
			}
		})
	}
}

// Reset makes old data unavailable; a fresh, completed snapshot restores it.
func TestSelectedProjectionReset(t *testing.T) {
	p := projectionFixture(t)
	if _, err := p.Apply(projectionPage(2, rpc.RouteViewPhaseSnapshot, true, projectionEntry("192.0.2.0/24"))); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Apply(projectionPage(2, rpc.RouteViewPhaseReset, false)); err != nil {
		t.Fatal(err)
	}
	if p.Synchronized() {
		t.Fatal("reset projection still synchronized")
	}
	if _, _, found := p.LPM(family.IPv4Unicast, netip.MustParseAddr("192.0.2.1")); found {
		t.Fatal("reset projection still readable")
	}
	if _, err := p.Apply(projectionPage(3, rpc.RouteViewPhaseDelta, true, projectionEntry("198.51.100.0/24"))); err == nil {
		t.Fatal("delta revived reset projection")
	}
}

func projectionPage(cursor uint64, phase rpc.RouteViewPhase, complete bool, entries ...rpc.RouteViewEntry) *rpc.RouteViewReadOutput {
	return &rpc.RouteViewReadOutput{Session: "session", Epoch: "epoch", Snapshot: "snapshot", Cursor: cursor, Phase: phase, Complete: complete, Entries: entries}
}

func projectionEntry(prefix string) rpc.RouteViewEntry {
	pfx := netip.MustParsePrefix(prefix)
	afi := uint16(family.AFIIPv6)
	if pfx.Addr().Is4() {
		afi = uint16(family.AFIIPv4)
	}
	return rpc.RouteViewEntry{AFI: afi, SAFI: uint8(family.SAFIUnicast), Prefix: pfx, Action: rpc.RouteViewActionAdd, Path: rpc.RouteViewPath{Protocol: "projection-test", Interface: "eth0", Metric: 7}}
}

// A projection is bound to an actual open reply, never fabricated from missing identity.
func TestSelectedProjectionRequiresOpenIdentity(t *testing.T) {
	for _, open := range []rpc.RouteViewOpenOutput{
		{},
		{Epoch: "epoch", Snapshot: "snapshot"},
		{Session: "session", Snapshot: "snapshot"},
		{Session: "session", Epoch: "epoch"},
	} {
		if _, err := newSelectedProjection(open); err == nil {
			t.Fatalf("missing identity accepted: %+v", open)
		}
	}
	var missing *selectedProjection
	if missing.Synchronized() {
		t.Fatal("nil projection synchronized")
	}
	if _, _, found := missing.LPM(family.IPv4Unicast, netip.MustParseAddr("192.0.2.1")); found {
		t.Fatal("nil projection proved reachability")
	}
}

// An ordered tombstone wins over a value seen earlier in the initial snapshot.
func TestSelectedProjectionSnapshotWithdrawalOrdering(t *testing.T) {
	p := projectionFixture(t)
	entry := projectionEntry("192.0.2.0/24")
	if _, err := p.Apply(projectionPage(1, rpc.RouteViewPhaseSnapshot, false, entry)); err != nil {
		t.Fatal(err)
	}
	withdraw := entry
	withdraw.Action = rpc.RouteViewActionWithdraw
	withdraw.Path = rpc.RouteViewPath{}
	if _, err := p.Apply(projectionPage(3, rpc.RouteViewPhaseSnapshot, true, withdraw)); err != nil {
		t.Fatal(err)
	}
	if !p.Synchronized() {
		t.Fatal("completed empty result unavailable")
	}
	if _, _, found := p.LPM(family.IPv4Unicast, netip.MustParseAddr("192.0.2.1")); found {
		t.Fatal("snapshot resurrected withdrawn route")
	}
	if _, err := p.Apply(projectionPage(1, rpc.RouteViewPhaseSnapshot, false, entry)); err == nil {
		t.Fatal("older snapshot overwrote completed result")
	}
}

// A snapshot becomes readable only after its completion marker, including an empty table.
func TestSelectedProjectionSnapshotAtomic(t *testing.T) {
	p := projectionFixture(t)
	entry := projectionEntry("192.0.2.0/24")
	if _, err := p.Apply(projectionPage(1, rpc.RouteViewPhaseSnapshot, false, entry)); err != nil {
		t.Fatal(err)
	}
	if p.Synchronized() {
		t.Fatal("partial snapshot is synchronized")
	}
	if _, _, found := p.LPM(family.IPv4Unicast, netip.MustParseAddr("192.0.2.1")); found {
		t.Fatal("partial snapshot is visible")
	}
	if _, err := p.Apply(projectionPage(2, rpc.RouteViewPhaseSnapshot, true)); err != nil {
		t.Fatal(err)
	}
	path, prefix, found := p.LPM(family.IPv4Unicast, netip.MustParseAddr("192.0.2.1"))
	if !found || prefix != entry.Prefix || path.Interface != "eth0" || path.Metric != 7 {
		t.Fatalf("completed snapshot: %v %v %v", path, prefix, found)
	}
	if !p.Synchronized() {
		t.Fatal("completed snapshot unavailable")
	}
	empty := projectionFixture(t)
	if _, err := empty.Apply(projectionPage(1, rpc.RouteViewPhaseSnapshot, true)); err != nil {
		t.Fatal(err)
	}
	if !empty.Synchronized() {
		t.Fatal("synchronized empty table reported unavailable")
	}
}

// A malformed later record must not apply an earlier withdrawal or advance the frontier.
func TestSelectedProjectionDeltaAtomic(t *testing.T) {
	p := projectionFixture(t)
	entry := projectionEntry("192.0.2.0/24")
	if _, err := p.Apply(projectionPage(2, rpc.RouteViewPhaseSnapshot, true, entry)); err != nil {
		t.Fatal(err)
	}
	withdraw := entry
	withdraw.Action = rpc.RouteViewActionWithdraw
	withdraw.Path = rpc.RouteViewPath{}
	bad := projectionEntry("2001:db8::/32")
	bad.Path.Protocol = "unregistered-projection-wire-name"
	if _, err := p.Apply(projectionPage(4, rpc.RouteViewPhaseDelta, true, withdraw, bad)); err == nil {
		t.Fatal("unknown protocol accepted")
	}
	if _, known := redistevents.ProtocolIDOf(bad.Path.Protocol); known {
		t.Fatal("wire protocol registered")
	}
	if _, _, found := p.LPM(family.IPv4Unicast, netip.MustParseAddr("192.0.2.1")); !found {
		t.Fatal("rejected page partially withdrew route")
	}
	if _, err := p.Apply(projectionPage(3, rpc.RouteViewPhaseDelta, true, withdraw)); err != nil {
		t.Fatal(err)
	}
	if _, _, found := p.LPM(family.IPv4Unicast, netip.MustParseAddr("192.0.2.1")); found {
		t.Fatal("valid withdrawal retained route")
	}
}
