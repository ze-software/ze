package dhcpserver

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"net"
	"net/netip"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/dnssource"
)

// dnsTestSubnet is a subnet with a 3600-second lease and the given option 6
// list and dns-server entries, served at 192.0.2.1 (its default router).
func dnsTestSubnet(t *testing.T, sources dnssource.List, configured ...netip.Addr) *dhcpHandler {
	t.Helper()
	h := newDHCPHandler(subnetConfig{
		Sources:       sources,
		Prefix:        netip.MustParsePrefix("192.0.2.0/24"),
		Ranges:        []addressRange{{Name: "pool", Start: netip.MustParseAddr("192.0.2.100"), Stop: netip.MustParseAddr("192.0.2.200")}},
		LeaseTimeSec:  3600,
		DefaultRouter: netip.MustParseAddr("192.0.2.1"),
		DNSServers:    configured,
	}, pxeConfig{})
	t.Cleanup(h.leases.stop)
	return h
}

// learnLease feeds follower one system-dns snapshot holding a WAN dhcpv4
// lease with addresses, valid for an hour. No address means no record.
func learnLease(t *testing.T, follower *dnssource.Follower, sequence uint64, addresses ...netip.Addr) {
	t.Helper()
	snapshot := dnssource.Snapshot{Sequence: sequence}
	if len(addresses) > 0 {
		snapshot.Records = []dnssource.Record{{
			Origin: "dhcpv4", Rank: 100, Interface: "wan0", Unit: "0",
			Addresses: addresses, Received: time.Now(), Expires: time.Now().Add(time.Hour),
		}}
	}
	body, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}
	fresh, err := follower.Accept(string(body))
	if err != nil {
		t.Fatalf("accept snapshot: %v", err)
	}
	if !fresh {
		t.Fatalf("snapshot %d not taken as fresh", sequence)
	}
}

// offerOption6 sends one DISCOVER through h and answers the option 6 servers
// and the lease time of the OFFER.
func offerOption6(t *testing.T, h *dhcpHandler, last byte) ([]netip.Addr, uint32) {
	t.Helper()
	offer := h.handle(buildDiscover(net.HardwareAddr{0x02, 0, 0, 0, 0x06, last}, 0x6000+uint32(last)))
	if offer == nil {
		t.Fatal("no DHCPOFFER")
	}
	return option6Of(t, offer), leaseOf(t, offer)
}

func option6Of(t *testing.T, reply []byte) []netip.Addr {
	t.Helper()
	raw := getResponseOption(reply, optDNS)
	if len(raw)%4 != 0 {
		t.Fatalf("option 6 length %d is not a multiple of 4", len(raw))
	}
	var servers []netip.Addr
	for i := 0; i < len(raw); i += 4 {
		servers = append(servers, netip.AddrFrom4([4]byte(raw[i:i+4])))
	}
	return servers
}

func leaseOf(t *testing.T, reply []byte) uint32 {
	t.Helper()
	raw := getResponseOption(reply, optLeaseTime)
	if len(raw) != 4 {
		t.Fatalf("option 51 length %d, want 4", len(raw))
	}
	return binary.BigEndian.Uint32(raw)
}

func writtenSubnetList(t *testing.T, mode dnssource.Mode, sources ...dnssource.Class) dnssource.List {
	t.Helper()
	list, err := dnssource.NewWrittenList(subnetDNSSourcePath, dnssource.FamilyIPv4, dnssource.AudienceHandedOn,
		&dnssource.Written{Sources: sources, Mode: mode}, subnetDNSRefusals...)
	if err != nil {
		t.Fatalf("list %v: %v", sources, err)
	}
	return list
}

var (
	wanDNS        = netip.MustParseAddr("192.0.2.53")
	configuredDNS = netip.MustParseAddr("198.51.100.1")
)

// TestOption6PassThroughDefault proves AC-14: a subnet with no dns-server and
// no dns-source hands out the WAN-learned server; once dns-server is set the
// reply carries it alone (D-5); once the lease is withdrawn no reply carries
// option 6. Method: DISCOVERs through one subnet, then through the same
// subnet rebuilt with dns-server over the same Follower.
func TestOption6PassThroughDefault(t *testing.T) {
	t.Parallel()

	h := dnsTestSubnet(t, subnetDNSDefault)
	learnLease(t, h.learned, 1, wanDNS)
	if got, _ := offerOption6(t, h, 1); !slices.Equal(got, []netip.Addr{wanDNS}) {
		t.Fatalf("option 6 with no dns-server = %v, want the learned %v", got, wanDNS)
	}

	set := dnsTestSubnet(t, subnetDNSDefault, configuredDNS)
	set.learned = h.learned
	if got, _ := offerOption6(t, set, 2); !slices.Equal(got, []netip.Addr{configuredDNS}) {
		t.Fatalf("option 6 with dns-server = %v, want %v alone", got, configuredDNS)
	}

	learnLease(t, h.learned, 2)
	if got, _ := offerOption6(t, h, 3); len(got) != 0 {
		t.Fatalf("option 6 after the lease was withdrawn = %v, want none", got)
	}
}

// TestLeaseShortWhileReceivedUnusable proves AC-25 (D-9): while the default
// list waits for a received entry, replies carry no option 6 and a 60-second
// lease, in the OFFER and in the ACK the lease table records; after a WAN
// lease arrives they carry 3600 and the learned server; a subnet with
// dns-server keeps 3600 throughout; a configured lease below 60 is kept.
func TestLeaseShortWhileReceivedUnusable(t *testing.T) {
	t.Parallel()

	h := dnsTestSubnet(t, subnetDNSDefault)
	offer, ack := exchange(t, h, net.HardwareAddr{0x02, 0, 0, 0, 0x25, 1}, 0x2501)
	if got := option6Of(t, offer); len(got) != 0 {
		t.Fatalf("option 6 with the WAN down = %v, want none", got)
	}
	if got := leaseOf(t, offer); got != shortLeaseSec {
		t.Fatalf("OFFER lease with the WAN down = %d, want %d", got, shortLeaseSec)
	}
	if got := leaseOf(t, ack); got != shortLeaseSec {
		t.Fatalf("ACK lease with the WAN down = %d, want %d", got, shortLeaseSec)
	}

	learnLease(t, h.learned, 1, wanDNS)
	got, lease := offerOption6(t, h, 2)
	if !slices.Equal(got, []netip.Addr{wanDNS}) {
		t.Fatalf("option 6 after the WAN lease = %v, want %v", got, wanDNS)
	}
	if lease != 3600 {
		t.Fatalf("lease after the WAN lease = %d, want 3600", lease)
	}

	set := dnsTestSubnet(t, subnetDNSDefault, configuredDNS)
	if _, lease := offerOption6(t, set, 3); lease != 3600 {
		t.Fatalf("lease of a subnet with dns-server and the WAN down = %d, want 3600", lease)
	}

	short := dnsTestSubnet(t, subnetDNSDefault)
	short.subnet.LeaseTimeSec = 45
	if _, lease := offerOption6(t, short, 4); lease != 45 {
		t.Fatalf("a configured 45-second lease with the WAN down = %d, want it kept", lease)
	}
}

// TestOption6FromDNSSource proves AC-7's option 6 half (D-6) and 23a AC-5:
// source self hands out Ze's address inside the subnet on the receiving
// interface; a relayed request, whose interface holds no address there,
// falls through to configured; an explicit source configured with no
// dns-server sends no option 6 and keeps the configured lease.
func TestOption6FromDNSSource(t *testing.T) {
	t.Parallel()

	self := dnsTestSubnet(t, writtenSubnetList(t, dnssource.ModeFirstUsable, dnssource.ClassSelf, dnssource.ClassConfigured), configuredDNS)
	if got, _ := offerOption6(t, self, 1); !slices.Equal(got, []netip.Addr{netip.MustParseAddr("192.0.2.1")}) {
		t.Fatalf("option 6 from self = %v, want Ze's 192.0.2.1", got)
	}

	discover := buildDiscover(net.HardwareAddr{0x02, 0, 0, 0, 0x07, 2}, 0x0702)
	copy(discover[24:28], []byte{192, 0, 2, 254}) // giaddr: a relay on the subnet
	relayed := segment{subnets: []*dhcpHandler{self}, device: []netip.Addr{netip.MustParseAddr("203.0.113.1")}, relayed: true}
	offer := relayed.serve(discover)
	if offer == nil {
		t.Fatal("no DHCPOFFER to the relayed DISCOVER")
	}
	if got := option6Of(t, offer); !slices.Equal(got, []netip.Addr{configuredDNS}) {
		t.Fatalf("option 6 of a relayed request = %v, want self to fall through to %v", got, configuredDNS)
	}

	configured := dnsTestSubnet(t, writtenSubnetList(t, dnssource.ModeUnspecified, dnssource.ClassConfigured))
	learnLease(t, configured.learned, 1, wanDNS)
	got, lease := offerOption6(t, configured, 3)
	if len(got) != 0 {
		t.Fatalf("option 6 of source configured with no dns-server = %v, want none", got)
	}
	if lease != 3600 {
		t.Fatalf("lease of source configured = %d, want 3600: the list never reaches received", lease)
	}
}

// TestOption6DropsHostOnlyAddresses proves AC-27 for option 6 (D-16): a
// loopback server, learned or configured, is never handed to a LAN client,
// and the list falls through to the next usable entry or class.
func TestOption6DropsHostOnlyAddresses(t *testing.T) {
	t.Parallel()

	h := dnsTestSubnet(t, subnetDNSDefault, netip.MustParseAddr("127.0.0.1"))
	learnLease(t, h.learned, 1, netip.MustParseAddr("127.0.0.53"), wanDNS)
	if got, _ := offerOption6(t, h, 1); !slices.Equal(got, []netip.Addr{wanDNS}) {
		t.Fatalf("option 6 = %v, want the loopback entries dropped and %v used", got, wanDNS)
	}
}

// TestOption6CutAtOptionMaximum proves the option 6 boundary: a merged list
// of 64 servers sends the first 63 in list order.
func TestOption6CutAtOptionMaximum(t *testing.T) {
	t.Parallel()

	var configured []netip.Addr
	for i := range 40 {
		configured = append(configured, netip.AddrFrom4([4]byte{198, 51, 100, byte(i + 1)}))
	}
	var learned []netip.Addr
	for i := range 24 {
		learned = append(learned, netip.AddrFrom4([4]byte{203, 0, 113, byte(i + 1)}))
	}
	h := dnsTestSubnet(t, writtenSubnetList(t, dnssource.ModeMerge, dnssource.ClassConfigured, dnssource.ClassReceived), configured...)
	learnLease(t, h.learned, 1, learned...)
	want := append(slices.Clone(configured), learned[:23]...)
	if got, _ := offerOption6(t, h, 1); !slices.Equal(got, want) {
		t.Fatalf("option 6 holds %d servers, want the first %d in list order", len(got), option6ServersMax)
	}
}

// TestSubnetDNSSourceRefused proves AC-11 and AC-16b for a DHCP subnet through
// the one parse the verifier and the boot run: radius is refused with the
// subnet's instance path (its shared-network and prefix keys filled in) and
// a fix command on that path, two sources without mode are refused, and the same list
// with mode, or one source without it, is accepted.
func TestSubnetDNSSourceRefused(t *testing.T) {
	t.Parallel()

	config := func(dnsSource string) string {
		return `{"service":{"dhcp-server":{"enabled":"true","shared-network":{"LAN":{"subnet":{"192.0.2.0/24":{` +
			`"range":{"start":"192.0.2.100","stop":"192.0.2.200"},"dns-source":` + dnsSource + `}}}}}}}`
	}
	var refused *dnssource.ClassRefusedError
	if _, err := parseConfig(config(`{"source":["radius"]}`)); !errors.As(err, &refused) {
		t.Fatalf("source radius: err = %v, want a ClassRefusedError", err)
	}
	const at = "service dhcp-server shared-network LAN subnet 192.0.2.0/24 dns-source"
	if refused.Path != at {
		t.Fatalf("source radius: refusal names %q, want the subnet's instance path %q", refused.Path, at)
	}
	if want := "delete " + at + " source radius"; !strings.HasPrefix(refused.Fix(), want) {
		t.Fatalf("source radius: fix %q, want it to start %q", refused.Fix(), want)
	}
	if _, err := parseConfig(config(`{"source":["self","received"]}`)); err == nil {
		t.Fatal("two sources without mode accepted")
	}
	for _, accepted := range []string{`{"source":["self","received"],"mode":"merge"}`, `{"source":["self"]}`} {
		if _, err := parseConfig(config(accepted)); err != nil {
			t.Fatalf("%s refused: %v", accepted, err)
		}
	}
}
