package rsvpte

import (
	"slices"
	"testing"
)

// captureSample is trimmed from the egress capture of a run of
// transit-loose-ero-expansion. The objects are verbatim; the PATH header line
// is reconstructed because that run's tail cut it off.
const captureSample = `07:08:20.636990 IP (tos 0x0, ttl 254, id 0, offset 0, flags [none], proto RSVP (46), length 204, options (RA))
    198.51.100.2 > 198.51.100.4: 
	RSVPv1 Path Message (1), Flags: [none], length: 180, ttl: 254, checksum: 0x0
	  RSVP Hop Object (3) Flags: [reject if unknown], Class-Type: IPv4 (1), length: 12
	    Previous/Next Interface: 172.29.81.3, Logical Interface Handle: 0x00000000
	  ERO Object (20) Flags: [reject if unknown], Class-Type: IPv4 (1), length: 20
	    Subobject Type: IPv4 prefix, length 8, Strict, 172.29.81.14/32, Flags: [none]
	    Subobject Type: IPv4 prefix, length 8, Loose, 198.51.100.4/32, Flags: [none]
	    0x0000:  0108 ac1d 510e 2000 8108 c633 6404 2000
	  Label Request Object (19) Flags: [reject if unknown], Class-Type: without label range (1), length: 8
07:08:20.638990 IP (tos 0x0, ttl 255, id 0, offset 0, flags [none], proto RSVP (46), length 132, options (RA))
    172.29.81.14 > 172.29.81.3: 
	RSVPv1 Resv Message (2), Flags: [none], length: 108, ttl: 255, checksum: 0xc4e4
	  Label Object (16) Flags: [reject if unknown], Class-Type: Label (1), length: 8
`

// VALIDATES: the capture splits into one message per packet with tcpdump's
// addresses, and the ERO reads back in order with each hop's strict or loose
// bit. PREVENTS: a checker that matches text across two packets.
func TestParseCaptureAndExplicitRoute(t *testing.T) {
	messages := parseCapture(captureSample)
	if len(messages) != 2 {
		t.Fatalf("got %d messages, want 2", len(messages))
	}
	if messages[0].source != "198.51.100.2" || messages[0].target != "198.51.100.4" {
		t.Fatalf("PATH addresses %q > %q", messages[0].source, messages[0].target)
	}
	if !isMessage(messages[1], addressEgress, addressZe, "Resv Message") {
		t.Fatalf("RESV not recognised: %+v", messages[1])
	}
	if isMessage(messages[0], addressEgress, addressZe, "Resv Message") {
		t.Fatal("PATH matched as the egress RESV")
	}
	hops, err := explicitRoute(messages[0].text)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"Subobject Type: IPv4 prefix, length 8, Strict, 172.29.81.14/32, Flags: [none]",
		"Subobject Type: IPv4 prefix, length 8, Loose, 198.51.100.4/32, Flags: [none]",
	}
	if !slices.Equal(hops, want) {
		t.Fatalf("ERO %q, want %q", hops, want)
	}
	if _, err := explicitRoute(messages[1].text); err == nil {
		t.Fatal("a RESV has no ERO, and explicitRoute must say so")
	}
}

// VALIDATES: every scenario directory has a checker and the catalog resolves
// through Discover. PREVENTS: a scenario directory the runner would refuse.
func TestScenarioCatalog(t *testing.T) {
	if got := ScenarioNames(); !slices.Equal(got, []string{scenarioLooseExpansion}) {
		t.Fatalf("scenario names %q", got)
	}
}
