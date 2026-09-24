// Design: docs/architecture/wire/nlri-bgpls.md -- native IS-IS snapshot behavior.
package isis

import (
	"bytes"
	"encoding/json"
	"net/netip"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/ze-software/ze/internal/core/linkstateevents"
	"github.com/ze-software/ze/internal/plugins/isis/lsdb"
	"github.com/ze-software/ze/internal/plugins/isis/packet"
	"github.com/ze-software/ze/internal/plugins/isis/transport"
	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// TestISISBGPLSDatabaseReplacement drives the real LSDB and publication hook,
// including replay after replacement, purge, deletion, and engine shutdown.
func TestISISBGPLSDatabaseReplacement(t *testing.T) {
	bus := &bgplsTestBus{handlers: make(map[[2]string]map[int]func(any))}
	eng := newEngine(transport.New(&fakeBackend{}))
	eng.setEventSink(newEventSink(bus))
	cfg, err := parseISISConfig(sec(`{"isis":{"net":"49.0001.0000.0000.0001.00"}}`))
	if err != nil {
		t.Fatal(err)
	}
	eng.setConfig(cfg)
	defer eng.shutdown()
	latest := make(map[linkstateevents.Protocol]linkstateevents.Snapshot)
	var generations []uint64
	unsub := isisLinkState.Subscribe(bus, func(snapshot *linkstateevents.Snapshot) {
		// Subscribers cannot retain any borrowed slice, even between two levels.
		encoded, err := json.Marshal(snapshot)
		if err != nil {
			t.Fatal(err)
		}
		var detached linkstateevents.Snapshot
		if err := json.Unmarshal(encoded, &detached); err != nil {
			t.Fatal(err)
		}
		latest[snapshot.Domain.Protocol] = detached
		if snapshot.Domain.Protocol == linkstateevents.ISISLevel1 {
			generations = append(generations, snapshot.Generation)
		}
	})
	defer unsub()
	eng.startBGPLS()
	if len(latest) != 2 || len(latest[linkstateevents.ISISLevel1].Nodes) != 0 {
		t.Fatalf("initial domains = %+v", latest)
	}
	id := types.LSPID{0, 0, 0, 0, 0, 2, 0, 0}
	bgplsStoreLSP(t, eng, lsdb.Level1, id, 1, 1200, []packet.TLV{
		{Type: packet.TLVDynamicHostname, Value: []byte("remote")},
		{Type: packet.TLVExtendedIPReach, Value: []byte{0, 0, 0, 10, 24, 192, 0, 2}},
	})
	eng.publishLSPChange("l1", id.String(), 1, "add")
	first := latest[linkstateevents.ISISLevel1]
	if len(first.Nodes) != 1 || len(first.Prefixes) != 1 || first.Prefixes[0].Prefix != netip.MustParsePrefix("192.0.2.0/24") {
		t.Fatalf("initial native contents = %+v", first)
	}
	bgplsStoreLSP(t, eng, lsdb.Level1, id, 2, 1200, []packet.TLV{
		{Type: packet.TLVExtendedIPReach, Value: []byte{0, 0, 0, 20, 24, 198, 51, 100}},
	})
	eng.publishLSPChange("l1", id.String(), 2, "refresh")
	if _, err := linkstateevents.Request.Emit(bus); err != nil {
		t.Fatal(err)
	}
	current := latest[linkstateevents.ISISLevel1]
	if len(current.Prefixes) != 1 || current.Prefixes[0].Prefix != netip.MustParsePrefix("198.51.100.0/24") {
		t.Fatalf("replacement/replay retained old reachability: %+v", current.Prefixes)
	}
	if first.Prefixes[0].Prefix != netip.MustParsePrefix("192.0.2.0/24") {
		t.Fatal("replacement changed a detached consumer snapshot")
	}
	bgplsStoreLSP(t, eng, lsdb.Level1, id, 3, 0, nil)
	eng.publishLSPChange("l1", id.String(), 3, "purge")
	if len(latest[linkstateevents.ISISLevel1].Nodes) != 0 || len(latest[linkstateevents.ISISLevel1].Prefixes) != 0 {
		t.Fatal("purged LSP was exported")
	}
	if !eng.lsdb.Delete(lsdb.Level1, id) {
		t.Fatal("purge entry was not retained for deletion")
	}
	eng.publishLSPChange("l1", id.String(), 3, "purge")
	bgplsStoreLSP(t, eng, lsdb.Level2, id, 1, 1200, nil)
	eng.publishLSPChange("l2", id.String(), 1, "add")
	if len(latest[linkstateevents.ISISLevel2].Nodes) != 1 {
		t.Fatal("level 2 database was not exported")
	}
	eng.stopBGPLS()
	for _, snapshot := range latest {
		if len(snapshot.Nodes)+len(snapshot.Links)+len(snapshot.Prefixes) != 0 {
			t.Fatalf("stop did not withdraw domain: %+v", snapshot)
		}
	}
	count := len(generations)
	if _, err := linkstateevents.Request.Emit(bus); err != nil {
		t.Fatal(err)
	}
	eng.publishLSPChange("l2", id.String(), 1, "refresh")
	if len(generations) != count {
		t.Fatal("stopped source replayed its database")
	}
	for i := 1; i < len(generations); i++ {
		if generations[i] <= generations[i-1] {
			t.Fatalf("generation order = %v", generations)
		}
	}
}

// TestISISBGPLSMultiTopologyAndOpaque keeps the topology attached to each
// adjacency/prefix, translates metric/TE fields, and checks native provenance.
func TestISISBGPLSMultiTopologyAndOpaque(t *testing.T) {
	eng := newEngine(transport.New(&fakeBackend{}))
	defer eng.shutdown()
	id := types.LSPID{0, 0, 0, 0, 0, 2, 0, 0}
	// MT 2: neighbor is pseudonode 0000.0000.0003.07, metric 17.
	link := []byte{0, 2, 0, 0, 0, 0, 0, 3, 7, 0, 0, 17, 10, 3, 4, 0, 0, 0, 9, 250, 2, 0xaa, 0xbb}
	bgplsStoreLSP(t, eng, lsdb.Level1, id, 1, 1200, []packet.TLV{
		{Type: 222, Value: link},
		{Type: 235, Value: []byte{0, 2, 0, 0, 0, 10, 0xd8, 192, 0, 2, 4, 249, 2, 0xcc, 0xdd}},
		{Type: 250, Value: []byte{0x11, 0x22}},
	})
	var builder bgplsBuilder
	builder.build(eng.lsdb.RawSnapshot(lsdb.Level1))
	snapshot := &builder.snapshot
	if len(snapshot.Nodes) != 1 || len(snapshot.Links) != 1 || len(snapshot.Prefixes) != 1 {
		t.Fatalf("mapped objects = nodes %d links %d prefixes %d", len(snapshot.Nodes), len(snapshot.Links), len(snapshot.Prefixes))
	}
	gotLink := &snapshot.Links[0]
	if !bytes.Equal(gotLink.Local.RouterID, id[:6]) || !bytes.Equal(gotLink.Remote.RouterID, link[2:9]) {
		t.Fatalf("router/pseudonode identities = %x -> %x", gotLink.Local.RouterID, gotLink.Remote.RouterID)
	}
	if len(gotLink.Topologies) != 1 || gotLink.Topologies[0] != 2 || snapshot.Prefixes[0].Topology != 2 {
		t.Fatalf("MT context lost: link %v prefix %d", gotLink.Topologies, snapshot.Prefixes[0].Topology)
	}
	bgplsWantAttribute(t, gotLink.Attributes, 1095, []byte{0, 0, 17})
	bgplsWantAttribute(t, gotLink.Attributes, 1088, []byte{0, 0, 0, 9})
	bgplsWantAttribute(t, snapshot.Prefixes[0].Attributes, 1152, []byte{0x80})
	for _, item := range []struct {
		got  []linkstateevents.Opaque
		want []byte
	}{
		{snapshot.Nodes[0].Opaque, []byte{250, 2, 0x11, 0x22}},
		{gotLink.Opaque, []byte{250, 2, 0xaa, 0xbb}},
		{snapshot.Prefixes[0].Opaque, []byte{249, 2, 0xcc, 0xdd}},
	} {
		if len(item.got) != 1 || item.got[0].Source != linkstateevents.ISIS || !bytes.Equal(item.got[0].Value, item.want) {
			t.Fatalf("native opaque = %+v, want %x", item.got, item.want)
		}
	}
	// A replacement moving the same prefix into MT 3 must not retain MT 2.
	bgplsStoreLSP(t, eng, lsdb.Level1, id, 2, 1200, []packet.TLV{
		{Type: 235, Value: []byte{0, 3, 0, 0, 0, 5, 24, 192, 0, 2}},
	})
	builder.build(eng.lsdb.RawSnapshot(lsdb.Level1))
	if len(builder.snapshot.Links) != 0 || len(builder.snapshot.Prefixes) != 1 || builder.snapshot.Prefixes[0].Topology != 3 {
		t.Fatalf("old topology retained: %+v", builder.snapshot)
	}
}

func bgplsStoreLSP(t *testing.T, eng *engine, level lsdb.Level, id types.LSPID, sequence uint32, lifetime uint16, tlvs []packet.TLV) {
	t.Helper()
	lsp := packet.LSP{PDUType: packet.PDUTypeL1LSP, LSPID: id, SequenceNumber: types.SequenceNumber(sequence), RemainingLifetime: types.RemainingLifetime(lifetime), TLVs: tlvs, TypeBlock: packet.LSPFlagISTypeL1}
	if level == lsdb.Level2 {
		lsp.PDUType = packet.PDUTypeL2LSP
	}
	raw := make([]byte, lsp.EncodedLen())
	lsp.WriteTo(raw, 0)
	eng.lsdb.Receive(level, &lsp, raw, false)
}

func bgplsWantAttribute(t *testing.T, attrs []linkstateevents.TLV, typ uint16, want []byte) {
	t.Helper()
	for _, attr := range attrs {
		if attr.Type == typ && bytes.Equal(attr.Value, want) {
			return
		}
	}
	t.Fatalf("attribute %d=%x missing from %+v", typ, want, attrs)
}

// TestISISBGPLSSegmentRouting maps SRGB/SRLB and SRv6 advertisements carried by
// real LSP bytes, including the different reserved-field layouts in BGP-LS.
func TestISISBGPLSSegmentRouting(t *testing.T) {
	eng := newEngine(transport.New(&fakeBackend{}))
	defer eng.shutdown()
	id := types.LSPID{0, 0, 0, 0, 0, 2, 0, 0}
	capabilities := []byte{
		192, 0, 2, 2, 0,
		2, 9, 0xc0, 0, 0, 100, 1, 3, 0, 0x3e, 0x80,
		22, 9, 0, 0, 0, 50, 1, 3, 0, 0x5d, 0xc0,
		19, 2, 0, 1, 23, 2, 41, 4, 24, 1, 128,
		25, 2, 0x40, 0,
	}
	end := []byte{0, 0, 1}
	endAddress := netip.MustParseAddr("2001:db8:1::1").As16()
	end = append(end, endAddress[:]...)
	end = append(end, 6, 1, 4, 32, 32, 16, 48)
	locator := []byte{0, 2, 0, 0, 0, 10, 0, 0, 64, 0x20, 1, 0x0d, 0xb8, 0, 1, 0, 0, byte(len(end) + 2), 5, byte(len(end))}
	locator = append(locator, end...)
	adjacency := []byte{0x20, 0, 1, 0, 5}
	adjAddress := netip.MustParseAddr("2001:db8:1::2").As16()
	adjacency = append(adjacency, adjAddress[:]...)
	adjacency = append(adjacency, 6, 1, 4, 32, 32, 16, 48)
	link := []byte{0, 2, 0, 0, 0, 0, 0, 3, 0, 0, 0, 10, byte(len(adjacency) + 2), 43, byte(len(adjacency))}
	link = append(link, adjacency...)
	bgplsStoreLSP(t, eng, lsdb.Level1, id, 1, 1200, []packet.TLV{
		{Type: 242, Value: capabilities},
		{Type: 27, Value: locator},
		{Type: 222, Value: link},
	})
	var builder bgplsBuilder
	builder.build(eng.lsdb.RawSnapshot(lsdb.Level1))
	snapshot := &builder.snapshot
	if len(snapshot.Nodes) != 1 || len(snapshot.Prefixes) != 1 || len(snapshot.SIDs) != 1 || len(snapshot.Links) != 1 {
		t.Fatalf("SR objects = %+v", snapshot)
	}
	bgplsWantAttribute(t, snapshot.Nodes[0].Attributes, 1034, []byte{0xc0, 0, 0, 0, 100, 4, 0x89, 0, 3, 0, 0x3e, 0x80})
	bgplsWantAttribute(t, snapshot.Nodes[0].Attributes, 1036, []byte{0, 0, 0, 0, 50, 4, 0x89, 0, 3, 0, 0x5d, 0xc0})
	bgplsWantAttribute(t, snapshot.Nodes[0].Attributes, 1035, []byte{0, 1})
	bgplsWantAttribute(t, snapshot.Nodes[0].Attributes, 266, []byte{41, 4})
	bgplsWantAttribute(t, snapshot.Nodes[0].Attributes, 1037, []byte{128})
	bgplsWantAttribute(t, snapshot.Nodes[0].Attributes, 1038, []byte{0x40, 0, 0, 0})
	bgplsWantAttribute(t, snapshot.Prefixes[0].Attributes, 1162, []byte{0, 0, 0, 0, 0, 0, 0, 10})
	if snapshot.SIDs[0].SID != netip.AddrFrom16(endAddress) || snapshot.SIDs[0].Topology != 2 {
		t.Fatalf("End SID identity = %+v", snapshot.SIDs[0])
	}
	bgplsWantAttribute(t, snapshot.SIDs[0].Attributes, 1250, []byte{0, 1, 0, 0})
	bgplsWantAttribute(t, snapshot.SIDs[0].Attributes, 1252, []byte{32, 32, 16, 48})
	wantAdjacency := append([]byte{0, 5, 0x20, 0, 1, 0}, adjAddress[:]...)
	wantAdjacency = append(wantAdjacency, 4, 0xe4, 0, 4, 32, 32, 16, 48)
	bgplsWantAttribute(t, snapshot.Links[0].Attributes, 1106, wantAdjacency)

	// A conflicting algorithm in another fragment invalidates the complete
	// locator advertisement, its End SIDs, and adjacency SIDs using it.
	fragment := id
	fragment[7] = 1
	conflict := bytes.Clone(locator)
	conflict[7] = 1
	bgplsStoreLSP(t, eng, lsdb.Level1, fragment, 1, 1200, []packet.TLV{{Type: 27, Value: conflict}})
	builder.build(eng.lsdb.RawSnapshot(lsdb.Level1))
	if len(snapshot.Prefixes) != 0 || len(snapshot.SIDs) != 0 {
		t.Fatalf("conflicting locator algorithms were exported: %+v", snapshot)
	}
	for _, attr := range snapshot.Links[0].Attributes {
		if attr.Type == 1106 {
			t.Fatal("End.X SID survived its locator conflict")
		}
	}
	bgplsStoreLSP(t, eng, lsdb.Level1, fragment, 2, 0, nil)
	wrongTopology := bytes.Clone(link)
	wrongTopology[1] = 3
	bgplsStoreLSP(t, eng, lsdb.Level1, id, 2, 1200, []packet.TLV{
		{Type: 27, Value: locator},
		{Type: 222, Value: wrongTopology},
	})
	builder.build(eng.lsdb.RawSnapshot(lsdb.Level1))
	if len(snapshot.SIDs) != 1 {
		t.Fatal("unrelated topology suppressed the valid locator End SID")
	}
	for _, attr := range snapshot.Links[0].Attributes {
		if attr.Type == 1106 {
			t.Fatal("End.X SID matched a locator in another topology")
		}
	}
}

// TestISISBGPLSFragmentsAndLegacy verifies a node's fragments merge rather than
// overwrite each other, and preserves narrow external/down prefix semantics.
func TestISISBGPLSFragmentsAndLegacy(t *testing.T) {
	eng := newEngine(transport.New(&fakeBackend{}))
	defer eng.shutdown()
	id := types.LSPID{0, 0, 0, 0, 0, 2, 0, 0}
	link := []byte{0, 0, 0, 0, 0, 3, 0, 0, 0, 9, 7, 31, 5, 0x30, 1, 0, 0x3e, 0x81}
	bgplsStoreLSP(t, eng, lsdb.Level1, id, 1, 1200, []packet.TLV{
		{Type: 229, Value: []byte{0, 0, 0x80, 2}},
		{Type: 22, Value: link},
		{Type: packet.TLVIPExternalReachability, Value: []byte{0xc7, 0x80, 0x80, 0x80, 192, 0, 2, 0, 255, 255, 255, 0}},
	})
	id[7] = 1
	link2 := bytes.Clone(link)
	link2[len(link2)-1] = 0x82
	bgplsStoreLSP(t, eng, lsdb.Level1, id, 1, 1200, []packet.TLV{
		{Type: 22, Value: link2},
		{Type: 137, Value: []byte("fragment-host")},
	})
	var builder bgplsBuilder
	builder.build(eng.lsdb.RawSnapshot(lsdb.Level1))
	snapshot := &builder.snapshot
	if len(snapshot.Nodes) != 1 || len(snapshot.Links) != 1 || len(snapshot.Prefixes) != 1 {
		t.Fatalf("fragment merge = %+v", snapshot)
	}
	bgplsWantAttribute(t, snapshot.Nodes[0].Attributes, 263, []byte{0, 0, 0x80, 2})
	bgplsWantAttribute(t, snapshot.Nodes[0].Attributes, 1026, []byte("fragment-host"))
	bgplsWantAttribute(t, snapshot.Links[0].Attributes, 1099, []byte{0x30, 1, 0, 0, 0, 0x3e, 0x81})
	bgplsWantAttribute(t, snapshot.Links[0].Attributes, 1099, []byte{0x30, 1, 0, 0, 0, 0x3e, 0x82})
	bgplsWantAttribute(t, snapshot.Prefixes[0].Attributes, 1155, []byte{0, 0, 0, 7})
	bgplsWantAttribute(t, snapshot.Prefixes[0].Attributes, 1152, []byte{0x80})
	bgplsWantAttribute(t, snapshot.Prefixes[0].Attributes, 1170, []byte{0x80})
	if len(snapshot.Prefixes[0].Opaque) != 1 {
		t.Fatal("legacy external metric type was lost")
	}
}

// TestISISBGPLSLinkIdentities checks unnumbered IDs survive when native address
// sub-TLVs contain only forbidden link-local addresses, with auxiliary IDs
// obtained from the actual endpoint nodes rather than invented from addresses.
func TestISISBGPLSLinkIdentities(t *testing.T) {
	eng := newEngine(transport.New(&fakeBackend{}))
	defer eng.shutdown()
	id := types.LSPID{0, 0, 0, 0, 0, 2, 0, 0}
	subs := []byte{4, 8, 0, 0, 0, 42, 0, 0, 0, 43, 12, 16}
	local := netip.MustParseAddr("fe80::2").As16()
	remote := netip.MustParseAddr("fe80::3").As16()
	subs = append(subs, local[:]...)
	subs = append(subs, 13, 16)
	subs = append(subs, remote[:]...)
	link := append([]byte{0, 0, 0, 0, 0, 3, 0, 0, 0, 10, byte(len(subs))}, subs...)
	bgplsStoreLSP(t, eng, lsdb.Level1, id, 1, 1200, []packet.TLV{
		{Type: 134, Value: []byte{192, 0, 2, 2}}, {Type: 22, Value: link},
	})
	id[5] = 3
	bgplsStoreLSP(t, eng, lsdb.Level1, id, 1, 1200, []packet.TLV{{Type: 134, Value: []byte{192, 0, 2, 3}}})
	var builder bgplsBuilder
	builder.build(eng.lsdb.RawSnapshot(lsdb.Level1))
	if len(builder.snapshot.Links) != 1 {
		t.Fatalf("native adjacency count = %d", len(builder.snapshot.Links))
	}
	got := &builder.snapshot.Links[0]
	if !got.HasLinkIDs || got.LocalID != 42 || got.RemoteID != 43 || len(got.LocalAddresses)+len(got.RemoteAddresses) != 0 {
		t.Fatalf("unnumbered identity = %+v", got)
	}
	bgplsWantAttribute(t, got.Attributes, 1028, []byte{192, 0, 2, 2})
	bgplsWantAttribute(t, got.Attributes, 1030, []byte{192, 0, 2, 3})
	subs = append(subs, 6, 4, 192, 0, 2, 4, 6, 4, 192, 0, 2, 5, 8, 4, 192, 0, 2, 6)
	link = append(link[:10:10], byte(len(subs)))
	link = append(link, subs...)
	id[5] = 2
	bgplsStoreLSP(t, eng, lsdb.Level1, id, 2, 1200, []packet.TLV{{Type: 22, Value: link}})
	builder.build(eng.lsdb.RawSnapshot(lsdb.Level1))
	got = &builder.snapshot.Links[0]
	if len(got.LocalAddresses) != 2 || got.LocalAddresses[0] != netip.MustParseAddr("192.0.2.4") || got.LocalAddresses[1] != netip.MustParseAddr("192.0.2.5") || len(got.RemoteAddresses) != 1 || got.RemoteAddresses[0] != netip.MustParseAddr("192.0.2.6") {
		t.Fatalf("numbered identity = %+v", got)
	}
	bgplsWantAttribute(t, got.Attributes, 258, []byte{0, 0, 0, 42, 0, 0, 0, 43})
}

// TestISISBGPLSShutdownDuringPublication holds an actual synchronous consumer
// in Emit while shutdown starts. Withdrawal must be last, never stale replay.
func TestISISBGPLSShutdownDuringPublication(t *testing.T) {
	bus := &bgplsTestBus{handlers: make(map[[2]string]map[int]func(any))}
	eng := newEngine(transport.New(&fakeBackend{}))
	eng.setEventSink(newEventSink(bus))
	cfg, err := parseISISConfig(sec(`{"isis":{"net":"49.0001.0000.0000.0001.00"}}`))
	if err != nil {
		t.Fatal(err)
	}
	eng.setConfig(cfg)
	defer eng.shutdown()
	bgplsStoreLSP(t, eng, lsdb.Level1, types.LSPID{0, 0, 0, 0, 0, 2, 0, 0}, 1, 1200, nil)
	entered, release := make(chan struct{}), make(chan struct{})
	var block atomic.Bool
	var lastGeneration uint64
	var lastNodes int
	ordered := true
	unsub := isisLinkState.Subscribe(bus, func(snapshot *linkstateevents.Snapshot) {
		if snapshot.Domain.Protocol != linkstateevents.ISISLevel1 {
			return
		}
		if block.CompareAndSwap(true, false) {
			close(entered)
			<-release
		}
		ordered = ordered && snapshot.Generation > lastGeneration
		lastGeneration, lastNodes = snapshot.Generation, len(snapshot.Nodes)
	})
	defer unsub()
	eng.startBGPLS()
	block.Store(true)
	published := make(chan struct{})
	go func() {
		eng.publishBGPLS()
		close(published)
	}()
	<-entered
	stopping, stopped := make(chan struct{}), make(chan struct{})
	go func() {
		close(stopping)
		eng.stopBGPLS()
		close(stopped)
	}()
	<-stopping
	close(release)
	<-published
	<-stopped
	if !ordered || lastNodes != 0 {
		t.Fatalf("publication/withdrawal order = %v, final nodes = %d", ordered, lastNodes)
	}
	generation := lastGeneration
	if _, err := linkstateevents.Request.Emit(bus); err != nil {
		t.Fatal(err)
	}
	if lastGeneration != generation {
		t.Fatal("stopped source answered replay")
	}
}

// TestISISBGPLSPseudonodeTopologies attaches the router-advertised MT set to a
// shared pseudonode's reverse edge instead of inventing topology zero.
func TestISISBGPLSPseudonodeTopologies(t *testing.T) {
	eng := newEngine(transport.New(&fakeBackend{}))
	defer eng.shutdown()
	router := types.LSPID{0, 0, 0, 0, 0, 2, 0, 0}
	toPN := []byte{0, 2, 0, 0, 0, 0, 0, 3, 1, 0, 0, 10, 0}
	second := bytes.Clone(toPN)
	second[1] = 3
	bgplsStoreLSP(t, eng, lsdb.Level1, router, 1, 1200, []packet.TLV{{Type: 222, Value: toPN}, {Type: 222, Value: second}})
	pseudonode := types.LSPID{0, 0, 0, 0, 0, 3, 1, 0}
	bgplsStoreLSP(t, eng, lsdb.Level1, pseudonode, 1, 1200, []packet.TLV{{Type: 22, Value: []byte{0, 0, 0, 0, 0, 2, 0, 0, 0, 0, 0}}})
	var builder bgplsBuilder
	builder.build(eng.lsdb.RawSnapshot(lsdb.Level1))
	found := false
	for _, link := range builder.snapshot.Links {
		if len(link.Local.RouterID) != 7 {
			continue
		}
		found = true
		if len(link.Remote.RouterID) != 6 || len(link.Topologies) != 2 || link.Topologies[0] != 2 || link.Topologies[1] != 3 {
			t.Fatalf("pseudonode topology identity = %+v", link)
		}
	}
	if !found {
		t.Fatal("pseudonode reverse edge was lost")
	}
}

// TestISISBGPLSNativeSPFReachability keeps remote router and pseudonode LSPs
// alive while the local edge disappears. Native SPF completion, not an update
// to either remote LSP, must suppress and then restore those originators.
// RFC requirement: RFC9552-5.9-1 positive -- completed native SPF republishes restored source eligibility for unchanged router and pseudonode LSDB records.
// RFC requirement: RFC9552-5.9-1 negative -- pending or unknown native SPF decisions cannot undo previously published source withdrawal eligibility.
func TestISISBGPLSNativeSPFReachability(t *testing.T) {
	bus := &bgplsTestBus{handlers: make(map[[2]string]map[int]func(any))}
	eng := newEngine(transport.New(&fakeBackend{}))
	eng.setEventSink(newEventSink(bus))
	cfg, err := parseISISConfig(sec(`{"isis":{"net":"49.0001.0000.0000.0001.00"}}`))
	if err != nil {
		t.Fatal(err)
	}
	cfg.Level = LevelL1
	eng.setConfig(cfg)
	defer eng.shutdown()
	root := types.LSPID{0, 0, 0, 0, 0, 1, 0, 0}
	remote := types.LSPID{0, 0, 0, 0, 0, 2, 0, 0}
	pseudonode := types.LSPID{0, 0, 0, 0, 0, 2, 1, 0}
	edge := func(id types.LSPID, metric byte) []byte {
		value := append([]byte(nil), id[:7]...)
		return append(value, 0, 0, metric, 0)
	}
	rootTLVs := []packet.TLV{{Type: 22, Value: edge(remote, 10)}}
	bgplsStoreLSP(t, eng, lsdb.Level1, root, 1, 1200, rootTLVs)
	bgplsStoreLSP(t, eng, lsdb.Level1, remote, 1, 1200, []packet.TLV{
		{Type: 22, Value: append(edge(root, 10), edge(pseudonode, 1)...)},
		{Type: 135, Value: []byte{0, 0, 0, 10, 24, 192, 0, 2}},
	})
	bgplsStoreLSP(t, eng, lsdb.Level1, pseudonode, 1, 1200, []packet.TLV{{Type: 22, Value: edge(remote, 0)}})
	raw := eng.lsdb.RawSnapshot(lsdb.Level1)
	staleRouter, stalePN := bytes.Clone(raw[1]), bytes.Clone(raw[2])
	var latest linkstateevents.Snapshot
	publications := 0
	unsubscribe := isisLinkState.Subscribe(bus, func(snapshot *linkstateevents.Snapshot) {
		if snapshot.Domain.Protocol != linkstateevents.ISISLevel1 {
			return
		}
		if publications != 0 && snapshot.Generation <= latest.Generation {
			t.Fatalf("SPF completion regressed generation: %d <= %d", snapshot.Generation, latest.Generation)
		}
		encoded, err := json.Marshal(snapshot)
		if err != nil {
			t.Fatal(err)
		}
		latest = linkstateevents.Snapshot{}
		if err := json.Unmarshal(encoded, &latest); err != nil {
			t.Fatal(err)
		}
		publications++
	})
	defer unsubscribe()
	eng.startBGPLS()
	eng.spf.Run()
	if len(latest.Unreachable) != 0 || len(latest.Nodes) != 3 {
		t.Fatalf("connected native graph = %+v", latest)
	}

	// Only the local LSP changes. The completed SPF callback must itself publish
	// the new policy state, even though no usable route delta is produced.
	bgplsStoreLSP(t, eng, lsdb.Level1, root, 2, 1200, nil)
	before := publications
	eng.spf.Run()
	if publications <= before {
		t.Fatal("SPF completion did not publish changed reachability")
	}
	if len(latest.Unreachable) != 2 {
		t.Fatalf("disconnected originators = %+v", latest.Unreachable)
	}
	for _, id := range latest.Unreachable {
		if !bytes.Equal(id.RouterID, remote[:6]) && !bytes.Equal(id.RouterID, pseudonode[:7]) {
			t.Fatalf("incorrect unreachable source identity: %x", id.RouterID)
		}
	}
	if len(latest.Nodes) != 3 || len(latest.Prefixes) != 1 || latest.Prefixes[0].Prefix != netip.MustParsePrefix("192.0.2.0/24") || len(latest.Links) != 3 {
		t.Fatalf("unreachable policy erased complete LSDB objects: %+v", latest)
	}

	// A newly received originator absent from the last native graph is unknown,
	// not yet a demonstrated unreachable vertex. The next native run decides.
	newcomer := types.LSPID{0, 0, 0, 0, 0, 3, 0, 0}
	bgplsStoreLSP(t, eng, lsdb.Level1, newcomer, 1, 1200, nil)
	if _, err := linkstateevents.Request.Emit(bus); err != nil {
		t.Fatal(err)
	}
	if len(latest.Unreachable) != 2 || len(latest.Nodes) != 4 {
		t.Fatalf("replay invented uncomputed reachability: %+v", latest)
	}
	eng.spf.Run()
	if len(latest.Unreachable) != 3 {
		t.Fatalf("new native graph did not classify disconnected originator: %+v", latest.Unreachable)
	}

	// A live nonzero fragment remains in the complete database when fragment
	// zero is purged, but the native graph deliberately excludes that origin.
	// Even a matching completed graph must not treat absence as recovery.
	fragment := newcomer
	fragment[7] = 1
	bgplsStoreLSP(t, eng, lsdb.Level1, fragment, 1, 1200, []packet.TLV{{Type: 137, Value: []byte("retained-fragment")}})
	bgplsStoreLSP(t, eng, lsdb.Level1, newcomer, 2, 0, nil)
	eng.spf.Run()
	if len(latest.Nodes) != 4 || len(latest.Unreachable) != 3 {
		t.Fatalf("completed graph omission resurrected an unknown origin: %+v", latest)
	}
	bgplsStoreLSP(t, eng, lsdb.Level1, newcomer, 3, 1200, nil)
	eng.spf.Run()

	// A configuration change invalidates the native cache. Its absence is not
	// evidence that previously unreachable origins have recovered.
	changed, err := parseISISConfig(sec(`{"isis":{"net":"49.0001.0000.0000.0002.00"}}`))
	if err != nil {
		t.Fatal(err)
	}
	changed.Level = LevelL1
	eng.setConfig(changed)
	if _, err := linkstateevents.Request.Emit(bus); err != nil {
		t.Fatal(err)
	}
	if len(latest.Unreachable) != 3 {
		t.Fatalf("unready new-root SPF resurrected withdrawn origins: %+v", latest.Unreachable)
	}
	// Actual SPF from the new root reaches the unchanged router/pseudonode
	// topology, but the disconnected newcomer remains unreachable.
	eng.spf.Run()
	if len(latest.Unreachable) != 1 || !bytes.Equal(latest.Unreachable[0].RouterID, newcomer[:6]) {
		t.Fatalf("completed new-root SPF did not restore reachable origins: %+v", latest.Unreachable)
	}

	// RFC 9552 section 5.9: "it MUST re-advertise those link-state objects
	// after that node becomes reachable again in the IGP domain."
	rootTLVs[0].Value = append(edge(remote, 10), edge(newcomer, 10)...)
	bgplsStoreLSP(t, eng, lsdb.Level1, root, 3, 1200, rootTLVs)
	before = publications
	eng.spf.Run()
	if publications <= before || len(latest.Unreachable) != 0 || len(latest.Nodes) != 4 || len(latest.Prefixes) != 1 || len(latest.Links) != 5 {
		t.Fatalf("native reconnection did not restore eligibility: %+v", latest)
	}
	raw = eng.lsdb.RawSnapshot(lsdb.Level1)
	if !bytes.Equal(raw[1], staleRouter) || !bytes.Equal(raw[2], stalePN) {
		t.Fatal("reachability recovery required changes to remote LSPs")
	}
}

// bgplsTestBus only supplies synchronous transport; all topology state and
// decoding exercised above belongs to the production LSDB and adapter.
type bgplsTestBus struct {
	mu       sync.Mutex
	next     int
	handlers map[[2]string]map[int]func(any)
}

func (b *bgplsTestBus) Subscribe(namespace, event string, handler func(any)) func() {
	b.mu.Lock()
	key := [2]string{namespace, event}
	if b.handlers[key] == nil {
		b.handlers[key] = make(map[int]func(any))
	}
	b.next++
	id := b.next
	b.handlers[key][id] = handler
	b.mu.Unlock()
	return func() {
		b.mu.Lock()
		delete(b.handlers[key], id)
		b.mu.Unlock()
	}
}

func (b *bgplsTestBus) Emit(namespace, event string, payload any) (int, error) {
	b.mu.Lock()
	handlers := make([]func(any), 0, len(b.handlers[[2]string{namespace, event}]))
	for _, handler := range b.handlers[[2]string{namespace, event}] {
		handlers = append(handlers, handler)
	}
	b.mu.Unlock()
	for _, handler := range handlers {
		handler(payload)
	}
	return 0, nil
}
