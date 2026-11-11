package testdeployment

import (
	"encoding/binary"
	"slices"
	"strings"
	"testing"
)

// vppMPLSFIBPop is `show mpls fib table 0` in the shape mpls_fib_show writes
// it: the table header, a reserved label VPP prints by name, then label 16
// as an egress pop. Each entry line is format_fib_entry, each path line
// format_fib_path.
const vppMPLSFIBPop = `MPLS-VRF:0, fib_index:0 locks:[interface:1, CLI:1, API:1, ]
ip4-explicit-null:neos/21 fib:0 index:30 locks:2
  special refs:1 entry-flags:exclusive, src-flags:added,contributing,active,
    path-list:[43] locks:2 flags:exclusive, uPRF-list:31 len:0 itfs:[]
      path:[45] pl-index:43 mpls weight=1 pref=0 exclusive:  oper-flags:resolved, cfg-flags:exclusive,

 forwarding:   mpls-neos-chain
  [@0]: dpo-load-balance: [proto:mpls index:33 buckets:1 uRPF:31 to:[0:0]]
16:eos/21 fib:0 index:40 locks:2
  API refs:1 src-flags:added,contributing,active,
    path-list:[50] locks:2 flags:shared, uPRF-list:45 len:0 itfs:[]
      path:[52] pl-index:50 ip4 weight=1 pref=0 deag:  oper-flags:resolved,
       fib-index:0
 forwarding:   mpls-eos-chain
  [@0]: dpo-load-balance: [proto:mpls index:42 buckets:1 uRPF:45 to:[0:0]]
16:neos/21 fib:0 index:41 locks:2
  API refs:1 src-flags:added,contributing,active,
    path-list:[51] locks:2 flags:shared, uPRF-list:46 len:0 itfs:[]
      path:[53] pl-index:51 mpls weight=1 pref=0 deag:  oper-flags:resolved,
 forwarding:   mpls-neos-chain
  [@0]: dpo-load-balance: [proto:mpls index:43 buckets:1 uRPF:46 to:[0:0]]
`

// fixtureIndex answers where needle starts in vppMPLSFIBPop, failing the
// test when the fixture no longer holds it.
func fixtureIndex(t *testing.T, needle string) int {
	t.Helper()
	at := strings.Index(vppMPLSFIBPop, needle)
	if at < 0 {
		t.Fatalf("the show mpls fib fixture lost %q", needle)
	}
	return at
}

// TestSinglePoppedLabelReadsTheEgressPop validates the parser that decides
// the mpls-label-pop "installed" check, on VPP's own output shape.
//
// VALIDATES: an allocated label is a pop only when its eos entry has an ip4
// deag path and its neos entry an mpls deag path, and exactly one label is.
// PREVENTS: a reserved label, a half-programmed label (one stack position),
// a swap path, or a second label passing as LDP's one egress pop.
func TestSinglePoppedLabelReadsTheEgressPop(t *testing.T) {
	label, ok := singlePoppedLabel(vppMPLSFIBPop)
	if !ok {
		t.Fatalf("singlePoppedLabel found no pop in:\n%s", vppMPLSFIBPop)
	}
	if label != 16 {
		t.Fatalf("singlePoppedLabel = %d, want 16", label)
	}

	onlyEOS, _, found := strings.Cut(vppMPLSFIBPop, "16:neos/21")
	if !found {
		t.Fatal("fixture lost its neos entry")
	}
	swapped := strings.Replace(vppMPLSFIBPop, "pl-index:50 ip4 weight=1 pref=0 deag:",
		"pl-index:50 ip4 weight=1 pref=0 attached-nexthop:", 1)
	eosAt := fixtureIndex(t, "16:eos")
	second := vppMPLSFIBPop + strings.ReplaceAll(vppMPLSFIBPop[eosAt:], "16:", "17:")
	for name, text := range map[string]string{
		"only eos":      onlyEOS,
		"swap path":     swapped,
		"two labels":    second,
		"empty table":   "MPLS-VRF:0, fib_index:0 locks:[API:1, ]\n",
		"reserved only": vppMPLSFIBPop[:eosAt],
	} {
		t.Run(name, func(t *testing.T) {
			if got, ok := singlePoppedLabel(text); ok {
				t.Fatalf("singlePoppedLabel = %d, true; want no single pop", got)
			}
		})
	}
}

// TestLabelHasEntrySeesEitherStackPosition validates the removal check.
//
// VALIDATES: a label counts as present while either its eos or its neos
// entry remains, and absent only when both are gone.
// PREVENTS: a reload that removes one stack position passing as removal.
func TestLabelHasEntrySeesEitherStackPosition(t *testing.T) {
	if !labelHasEntry(vppMPLSFIBPop, 16) {
		t.Fatal("label 16 not seen in the full table")
	}
	neosOnly := vppMPLSFIBPop[:fixtureIndex(t, "16:eos")] + vppMPLSFIBPop[fixtureIndex(t, "16:neos"):]
	if !labelHasEntry(neosOnly, 16) {
		t.Fatal("label 16 not seen with only its neos entry left")
	}
	if labelHasEntry(vppMPLSFIBPop, 17) {
		t.Fatal("label 17 seen in a table that does not hold it")
	}
}

// TestTraceVerdictsFollowTheNodeOrder validates the packet-trace verdicts.
//
// VALIDATES: a popped frame needs an IPv4 lookup after mpls-lookup; a control
// frame needs mpls-drop after mpls-lookup and no IPv4 lookup.
// PREVENTS: a frame MPLS input refused (mpls-not-enabled) or one dropped for
// another reason passing either check.
func TestTraceVerdictsFollowTheNodeOrder(t *testing.T) {
	popped := `Packet 1

00:00:12:345678: pg-input
  stream mpls-pop, 60 bytes, sw_if_index 2
00:00:12:345690: ethernet-input
  MPLS: 02:fe:00:00:30:02 -> 02:fe:00:00:30:01
00:00:12:345700: mpls-input
  MPLS: next mpls-lookup[1]  label 16 ttl 64 exp 0
00:00:12:345710: mpls-lookup
  MPLS: next [6], lookup fib index 0, LB index 40 hash 0 label 16 eos 1
00:00:12:345720: ip4-mpls-label-disposition-pipe
00:00:12:345730: lookup-ip4-dst
00:00:12:345740: ip4-drop
`
	nodes := traceNodes(popped)
	want := []string{"pg-input", "ethernet-input", "mpls-input", "mpls-lookup",
		"ip4-mpls-label-disposition-pipe", "lookup-ip4-dst", "ip4-drop"}
	if !slices.Equal(nodes, want) {
		t.Fatalf("traceNodes = %v, want %v", nodes, want)
	}
	if !tracePopped(nodes) {
		t.Fatal("tracePopped refused a frame that reached lookup-ip4-dst after mpls-lookup")
	}
	if traceDropped(nodes) {
		t.Fatal("traceDropped accepted a frame that reached an IPv4 lookup")
	}

	dropped := []string{"pg-input", "ethernet-input", "mpls-input", "mpls-lookup", "mpls-drop", "error-drop"}
	if !traceDropped(dropped) {
		t.Fatal("traceDropped refused a miss dropped at mpls-drop")
	}
	if tracePopped(dropped) {
		t.Fatal("tracePopped accepted a frame dropped at mpls-drop")
	}

	refused := []string{"pg-input", "ethernet-input", "mpls-input", "mpls-not-enabled", "error-drop"}
	if tracePopped(refused) {
		t.Fatal("tracePopped accepted a frame MPLS input refused")
	}
	if traceDropped(refused) {
		t.Fatal("traceDropped accepted a frame that never reached mpls-lookup")
	}
	if tracePopped([]string{"lookup-ip4-dst", "mpls-lookup"}) {
		t.Fatal("tracePopped accepted an IPv4 lookup before mpls-lookup")
	}
}

// TestMPLSPopFrameCarriesOneBottomLabel validates the injected frame.
//
// VALIDATES: the frame is addressed to the packet-generator interface, is
// typed MPLS unicast, carries the label with S set and TTL 64, and holds an
// IPv4 header with a valid checksum to the LSR-ID.
// PREVENTS: a frame ethernet-input drops for its address or type, or one
// whose label VPP looks up as a neos entry.
func TestMPLSPopFrameCarriesOneBottomLabel(t *testing.T) {
	var frame [60]byte
	if err := mplsPopFrame(frame[:], 16); err != nil {
		t.Fatal(err)
	}
	if got := frame[0:6]; !slices.Equal(got, []byte{0x02, 0xfe, 0x00, 0x00, 0x30, 0x01}) {
		t.Fatalf("destination MAC = % x", got)
	}
	if got := binary.BigEndian.Uint16(frame[12:14]); got != 0x8847 {
		t.Fatalf("EtherType = %#04x, want 0x8847", got)
	}
	entry := binary.BigEndian.Uint32(frame[14:18])
	if entry>>12 != 16 {
		t.Fatalf("label = %d, want 16", entry>>12)
	}
	if entry>>8&1 != 1 {
		t.Fatal("S bit clear on the only label stack entry")
	}
	if entry&0xff != 64 {
		t.Fatalf("label TTL = %d, want 64", entry&0xff)
	}
	if ipv4HeaderChecksum(frame[18:38]) != 0 {
		t.Fatal("IPv4 header checksum does not verify")
	}
	if got := frame[34:38]; !slices.Equal(got, []byte{10, 0, 0, 1}) {
		t.Fatalf("IPv4 destination = %v, want the LSR-ID", got)
	}
}

// TestMPLSInputAndGeneratorReaders validates the two small VPP readers.
//
// VALIDATES: `show mpls interface` counts only the interface whose own next
// line says enabled; `show packet-generator` answers the named stream's count.
// PREVENTS: another interface's enabled line, or another stream's count,
// answering for ours.
func TestMPLSInputAndGeneratorReaders(t *testing.T) {
	if !vppMPLSInputEnabled("loop0\n\n  MPLS disabled\npg0\n\n  MPLS enabled\n") {
		t.Fatal("pg0 enabled not read")
	}
	if vppMPLSInputEnabled("pg0\n\n  MPLS disabled\nloop0\n\n  MPLS enabled\n") {
		t.Fatal("loop0's enabled line answered for pg0")
	}
	streams := "Name                 Enabled        Count     Parameters\nother                  No             7\nmpls-pop               No             1      limit 1\n"
	if got := pgGenerated(streams, "mpls-pop"); got != 1 {
		t.Fatalf("pgGenerated = %d, want 1", got)
	}
	if got := pgGenerated(streams, "absent"); got != -1 {
		t.Fatalf("pgGenerated(absent) = %d, want -1", got)
	}
}

// TestMPLSLabelPopConfigLeavesTheWriterToTheEngine validates the scenario's
// two configurations.
//
// VALIDATES: neither configuration names a fib block, both keep the VPP
// interface with MPLS enabled, and only the first holds ldp.
// PREVENTS: a fib block choosing fib-vpp and hiding a NeedsDataPlane
// regression, or the reload dropping the interface instead of only LDP.
func TestMPLSLabelPopConfigLeavesTheWriterToTheEngine(t *testing.T) {
	with, without := vppMPLSLabelPopConfig(true), vppMPLSLabelPopConfig(false)
	for name, config := range map[string]string{"with ldp": with, "without ldp": without} {
		if strings.Contains(config, "fib {") {
			t.Fatalf("%s names a fib block:\n%s", name, config)
		}
		if !strings.Contains(config, "ethernet pg0 {\n        unit default { mpls { enable true; } }") {
			t.Fatalf("%s lost the MPLS-enabled VPP interface:\n%s", name, config)
		}
	}
	if !strings.Contains(with, "ldp {\n    lsr-id 10.0.0.1;") {
		t.Fatalf("the first configuration holds no ldp block:\n%s", with)
	}
	if strings.Contains(without, "ldp {") {
		t.Fatalf("the reload configuration still holds ldp:\n%s", without)
	}
}
