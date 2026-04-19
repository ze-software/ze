package ifacevpp

import (
	"fmt"
	"testing"
	"time"

	"go.fd.io/govpp/api"
	"go.fd.io/govpp/binapi/interface_types"
	"go.fd.io/govpp/binapi/ip_neighbor"
	"go.fd.io/govpp/binapi/ip_types"

	"codeberg.org/thomas-mangin/ze/internal/component/iface"
)

// neighborChannel is a mock api.Channel wired specifically for
// IPNeighborDump multi-requests. It mirrors routeChannel's shape but
// keys replies on the Af byte of the dump request rather than the
// IPRouteV2Dump.IsIP6 boolean. Kept separate from routeChannel so
// neither test mock has to juggle two unrelated RPC protocols.
type neighborChannel struct {
	lastRequest api.Message
	allRequests []api.Message
	v4Details   []ip_neighbor.IPNeighborDetails
	v6Details   []ip_neighbor.IPNeighborDetails
	sendErr     error
	receiveErr  error
	dumpCalls   int
}

var _ api.Channel = (*neighborChannel)(nil)

func (c *neighborChannel) SendRequest(msg api.Message) api.RequestCtx {
	c.lastRequest = msg
	c.allRequests = append(c.allRequests, msg)
	return &neighborRequestCtx{ch: c}
}

func (c *neighborChannel) SendMultiRequest(msg api.Message) api.MultiRequestCtx {
	c.lastRequest = msg
	c.allRequests = append(c.allRequests, msg)
	details := c.v4Details
	if m, ok := msg.(*ip_neighbor.IPNeighborDump); ok {
		if m.Af == ip_types.ADDRESS_IP6 {
			details = c.v6Details
		}
	}
	c.dumpCalls++
	return &neighborMultiCtx{ch: c, details: details}
}

func (c *neighborChannel) SubscribeNotification(_ chan api.Message, _ api.Message) (api.SubscriptionCtx, error) {
	return nil, fmt.Errorf("SubscribeNotification not implemented")
}

func (c *neighborChannel) SetReplyTimeout(time.Duration)          {}
func (c *neighborChannel) CheckCompatiblity(...api.Message) error { return nil }
func (c *neighborChannel) Close()                                 {}

type neighborRequestCtx struct{ ch *neighborChannel }

func (r *neighborRequestCtx) ReceiveReply(_ api.Message) error {
	return r.ch.sendErr
}

type neighborMultiCtx struct {
	ch      *neighborChannel
	details []ip_neighbor.IPNeighborDetails
	pos     int
}

func (m *neighborMultiCtx) ReceiveReply(msg api.Message) (bool, error) {
	if m.ch.receiveErr != nil {
		return false, m.ch.receiveErr
	}
	if m.pos >= len(m.details) {
		return true, nil
	}
	d, ok := msg.(*ip_neighbor.IPNeighborDetails)
	if !ok {
		return false, fmt.Errorf("neighborMultiCtx: unexpected reply type %T", msg)
	}
	*d = m.details[m.pos]
	m.pos++
	return false, nil
}

// makeV4Neighbor builds an ip_neighbor.IPNeighborDetails entry for the
// given IPv4 address, MAC string ("" for zero MAC), SwIfIndex, and
// flags byte. The helper parses both values through the vendored
// ip_types / ethernet-types parsers so the test exercises the same
// encoding path as real VPP replies.
func makeV4Neighbor(t *testing.T, ip, mac string, swIfIndex uint32) ip_neighbor.IPNeighborDetails {
	t.Helper()
	addr, err := ip_types.ParseIP4Address(ip)
	if err != nil {
		t.Fatalf("ParseIP4Address(%q): %v", ip, err)
	}
	n := ip_neighbor.IPNeighbor{
		SwIfIndex: interface_types.InterfaceIndex(swIfIndex),
		IPAddress: ip_types.Address{
			Af: ip_types.ADDRESS_IP4,
			Un: ip_types.AddressUnionIP4(addr),
		},
	}
	if mac != "" {
		setMAC(t, &n, mac)
	}
	return ip_neighbor.IPNeighborDetails{Neighbor: n}
}

// makeV6Neighbor builds an IPv6 neighbor entry. Separate from the v4
// helper because IP4Address and IP6Address are different underlying
// arrays and do not share the same union accessor.
func makeV6Neighbor(t *testing.T, ip, mac string, swIfIndex uint32, flags ip_neighbor.IPNeighborFlags) ip_neighbor.IPNeighborDetails {
	t.Helper()
	addr, err := ip_types.ParseIP6Address(ip)
	if err != nil {
		t.Fatalf("ParseIP6Address(%q): %v", ip, err)
	}
	var un ip_types.AddressUnion
	copy(un.XXX_UnionData[:16], addr[:])
	n := ip_neighbor.IPNeighbor{
		SwIfIndex: interface_types.InterfaceIndex(swIfIndex),
		Flags:     flags,
		IPAddress: ip_types.Address{
			Af: ip_types.ADDRESS_IP6,
			Un: un,
		},
	}
	if mac != "" {
		setMAC(t, &n, mac)
	}
	return ip_neighbor.IPNeighborDetails{Neighbor: n}
}

// setMAC parses "aa:bb:cc:dd:ee:ff" into the fixed-size MacAddress
// array. Six colons-separated hex bytes are expected; otherwise the
// test fails fatally.
func setMAC(t *testing.T, n *ip_neighbor.IPNeighbor, mac string) {
	t.Helper()
	var b [6]byte
	if _, err := fmt.Sscanf(mac, "%02x:%02x:%02x:%02x:%02x:%02x",
		&b[0], &b[1], &b[2], &b[3], &b[4], &b[5]); err != nil {
		t.Fatalf("sscanf mac %q: %v", mac, err)
	}
	copy(n.MacAddress[:], b[:])
}

// --- ListNeighbors ---

// TestListNeighborsEmpty asserts a clean run against an empty neighbor
// table returns zero entries without error.
// VALIDATES: ListNeighbors handles the zero-entry case.
// PREVENTS: nil-slice / nil-vs-empty confusion at the caller.
func TestListNeighborsEmpty(t *testing.T) {
	ch := &neighborChannel{}
	b := &vppBackendImpl{ch: ch, names: newNameMap()}
	b.populate.Do(func() {})

	got, err := b.ListNeighbors(iface.NeighborFamilyAny)
	if err != nil {
		t.Fatalf("ListNeighbors: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("len: got %d, want 0", len(got))
	}
	if ch.dumpCalls != 2 {
		t.Errorf("dump calls: got %d, want 2 (v4 + v6)", ch.dumpCalls)
	}
}

// TestListNeighborsV4Only dumps IPv4 neighbors and skips v6.
// VALIDATES: NeighborFamilyIPv4 restricts the request to af=v4.
// PREVENTS: accidentally querying v6 when the caller wants v4 only.
func TestListNeighborsV4Only(t *testing.T) {
	ch := &neighborChannel{
		v4Details: []ip_neighbor.IPNeighborDetails{
			makeV4Neighbor(t, "192.0.2.10", "aa:bb:cc:dd:ee:01", 7),
		},
	}
	b := &vppBackendImpl{ch: ch, names: newNameMap()}
	b.names.Add("xe7", 7, "xe7")
	b.populate.Do(func() {})

	got, err := b.ListNeighbors(iface.NeighborFamilyIPv4)
	if err != nil {
		t.Fatalf("ListNeighbors: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len: got %d, want 1", len(got))
	}
	n := got[0]
	if n.Address != "192.0.2.10" {
		t.Errorf("Address: got %q, want 192.0.2.10", n.Address)
	}
	if n.Family != "ipv4" {
		t.Errorf("Family: got %q, want ipv4", n.Family)
	}
	if n.MAC != "aa:bb:cc:dd:ee:01" {
		t.Errorf("MAC: got %q, want aa:bb:cc:dd:ee:01", n.MAC)
	}
	if n.Device != "xe7" {
		t.Errorf("Device: got %q, want xe7", n.Device)
	}
	if n.State != "reachable" {
		t.Errorf("State: got %q, want reachable", n.State)
	}
	if ch.dumpCalls != 1 {
		t.Errorf("dump calls: got %d, want 1 (v4 only)", ch.dumpCalls)
	}
}

// TestListNeighborsV6Only dumps IPv6 neighbors and skips v4.
// VALIDATES: NeighborFamilyIPv6 restricts the request to af=v6.
// PREVENTS: accidentally querying v4 when the caller wants v6 only.
func TestListNeighborsV6Only(t *testing.T) {
	ch := &neighborChannel{
		v6Details: []ip_neighbor.IPNeighborDetails{
			makeV6Neighbor(t, "2001:db8::1", "aa:bb:cc:dd:ee:02", 0, ip_neighbor.IP_API_NEIGHBOR_FLAG_STATIC),
		},
	}
	b := &vppBackendImpl{ch: ch, names: newNameMap()}
	b.populate.Do(func() {})

	got, err := b.ListNeighbors(iface.NeighborFamilyIPv6)
	if err != nil {
		t.Fatalf("ListNeighbors: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len: got %d, want 1", len(got))
	}
	n := got[0]
	if n.Address != "2001:db8::1" {
		t.Errorf("Address: got %q, want 2001:db8::1", n.Address)
	}
	if n.Family != "ipv6" {
		t.Errorf("Family: got %q, want ipv6", n.Family)
	}
	if n.State != "permanent" {
		t.Errorf("State: got %q, want permanent (STATIC flag)", n.State)
	}
	if ch.dumpCalls != 1 {
		t.Errorf("dump calls: got %d, want 1 (v6 only)", ch.dumpCalls)
	}
}

// TestListNeighborsAnyConcatenates collects both families in v4-then-v6
// order. VALIDATES: NeighborFamilyAny issues two dumps and merges the
// results. PREVENTS: silently dropping one family or reversing order.
func TestListNeighborsAnyConcatenates(t *testing.T) {
	ch := &neighborChannel{
		v4Details: []ip_neighbor.IPNeighborDetails{
			makeV4Neighbor(t, "192.0.2.1", "aa:bb:cc:dd:ee:01", 1),
			makeV4Neighbor(t, "192.0.2.2", "aa:bb:cc:dd:ee:02", 1),
		},
		v6Details: []ip_neighbor.IPNeighborDetails{
			makeV6Neighbor(t, "2001:db8::1", "aa:bb:cc:dd:ee:03", 1, 0),
		},
	}
	b := &vppBackendImpl{ch: ch, names: newNameMap()}
	b.populate.Do(func() {})

	got, err := b.ListNeighbors(iface.NeighborFamilyAny)
	if err != nil {
		t.Fatalf("ListNeighbors: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("len: got %d, want 3 (2 v4 + 1 v6)", len(got))
	}
	wantFams := []string{"ipv4", "ipv4", "ipv6"}
	for i, w := range wantFams {
		if got[i].Family != w {
			t.Errorf("entry %d family: got %q, want %q (v4-before-v6 ordering)", i, got[i].Family, w)
		}
	}
	if ch.dumpCalls != 2 {
		t.Errorf("dump calls: got %d, want 2 (v4 + v6)", ch.dumpCalls)
	}
}

// TestListNeighborsUnknownSwIfIndex leaves Device empty when the VPP
// port has no ze name registered. Mirrors fib.go's policy of never
// exposing an opaque integer to the operator.
// VALIDATES: absent nameMap entry produces empty Device, not "99".
// PREVENTS: leaking raw VPP indexes in operator-visible output.
func TestListNeighborsUnknownSwIfIndex(t *testing.T) {
	ch := &neighborChannel{
		v4Details: []ip_neighbor.IPNeighborDetails{
			makeV4Neighbor(t, "192.0.2.3", "aa:bb:cc:dd:ee:04", 99),
		},
	}
	b := &vppBackendImpl{ch: ch, names: newNameMap()}
	b.populate.Do(func() {})

	got, err := b.ListNeighbors(iface.NeighborFamilyIPv4)
	if err != nil {
		t.Fatalf("ListNeighbors: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len: got %d, want 1", len(got))
	}
	if got[0].Device != "" {
		t.Errorf("Device: got %q, want empty (SwIfIndex unmapped)", got[0].Device)
	}
}

// TestListNeighborsMultiEntry asserts every entry in the dump reply is
// surfaced, preserving the order VPP sent them in.
// VALIDATES: ReceiveReply loop does not drop entries between first and
// last=true.
// PREVENTS: off-by-one in the multi-request drain loop.
func TestListNeighborsMultiEntry(t *testing.T) {
	ch := &neighborChannel{
		v4Details: []ip_neighbor.IPNeighborDetails{
			makeV4Neighbor(t, "192.0.2.1", "aa:bb:cc:dd:ee:01", 1),
			makeV4Neighbor(t, "192.0.2.2", "aa:bb:cc:dd:ee:02", 1),
			makeV4Neighbor(t, "192.0.2.3", "aa:bb:cc:dd:ee:03", 1),
			makeV4Neighbor(t, "192.0.2.4", "", 1),
		},
	}
	b := &vppBackendImpl{ch: ch, names: newNameMap()}
	b.populate.Do(func() {})

	got, err := b.ListNeighbors(iface.NeighborFamilyIPv4)
	if err != nil {
		t.Fatalf("ListNeighbors: %v", err)
	}
	if len(got) != 4 {
		t.Fatalf("len: got %d, want 4", len(got))
	}
	// Last entry has zero MAC -> MAC field must be empty.
	if got[3].MAC != "" {
		t.Errorf("entry 3 MAC: got %q, want empty (zero MAC = unresolved)", got[3].MAC)
	}
	// First three have real MACs.
	for i := range 3 {
		if got[i].MAC == "" {
			t.Errorf("entry %d MAC: empty, want non-empty", i)
		}
	}
}

// TestListNeighborsReceiveError propagates a channel failure so the
// operator sees the VPP-side problem instead of a silently empty list.
// VALIDATES: error path wraps the VPP error with context.
// PREVENTS: silent loss of VPP failures during dumps.
func TestListNeighborsReceiveError(t *testing.T) {
	ch := &neighborChannel{
		receiveErr: fmt.Errorf("VPP dead"),
		v4Details: []ip_neighbor.IPNeighborDetails{
			makeV4Neighbor(t, "192.0.2.1", "aa:bb:cc:dd:ee:01", 1),
		},
	}
	b := &vppBackendImpl{ch: ch, names: newNameMap()}
	b.populate.Do(func() {})

	_, err := b.ListNeighbors(iface.NeighborFamilyIPv4)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// TestListNeighborsInvalidFamily rejects an unknown family selector at
// the dispatch boundary -- matches the exact-or-reject policy.
// VALIDATES: unsupported family selector returns an error.
// PREVENTS: silently falling through to some default family.
func TestListNeighborsInvalidFamily(t *testing.T) {
	ch := &neighborChannel{}
	b := &vppBackendImpl{ch: ch, names: newNameMap()}
	b.populate.Do(func() {})

	if _, err := b.ListNeighbors(99); err == nil {
		t.Fatal("expected error for unsupported family, got nil")
	}
	if ch.dumpCalls != 0 {
		t.Errorf("dump calls: got %d, want 0 (family rejected before dispatch)", ch.dumpCalls)
	}
}

// --- neighborStateName ---

func TestNeighborStateNameStatic(t *testing.T) {
	if got := neighborStateName(ip_neighbor.IP_API_NEIGHBOR_FLAG_STATIC); got != "permanent" {
		t.Errorf("STATIC: got %q, want permanent", got)
	}
}

func TestNeighborStateNameNone(t *testing.T) {
	if got := neighborStateName(0); got != "reachable" {
		t.Errorf("0: got %q, want reachable (default)", got)
	}
}

func TestNeighborStateNameNoFibEntry(t *testing.T) {
	// NO_FIB_ENTRY alone (no STATIC) is still considered a resolved
	// cache entry, so the state remains "reachable".
	if got := neighborStateName(ip_neighbor.IP_API_NEIGHBOR_FLAG_NO_FIB_ENTRY); got != "reachable" {
		t.Errorf("NO_FIB_ENTRY: got %q, want reachable", got)
	}
}
