package ifacevpp

import (
	"fmt"
	"testing"
	"time"

	"go.fd.io/govpp/api"
	interfaces "go.fd.io/govpp/binapi/interface"
	"go.fd.io/govpp/binapi/interface_types"
)

// dumpChannel is a mock api.Channel specialised for SwInterfaceDump
// multi-requests. Unit reuses the fibvpp testChannel pattern but returns
// SwInterfaceDetails instead of IPRouteAddDelReply.
type dumpChannel struct {
	lastRequest api.Message
	details     []interfaces.SwInterfaceDetails
	macReply    interfaces.SwInterfaceSetMacAddressReply
	sendErr     error
	receiveErr  error
	closed      bool
}

var _ api.Channel = (*dumpChannel)(nil)

func (c *dumpChannel) SendRequest(msg api.Message) api.RequestCtx {
	c.lastRequest = msg
	return &dumpRequestCtx{ch: c}
}

func (c *dumpChannel) SendMultiRequest(msg api.Message) api.MultiRequestCtx {
	c.lastRequest = msg
	return &dumpMultiCtx{ch: c, pos: 0}
}

func (c *dumpChannel) SubscribeNotification(_ chan api.Message, _ api.Message) (api.SubscriptionCtx, error) {
	return nil, fmt.Errorf("SubscribeNotification not implemented in dumpChannel")
}

func (c *dumpChannel) SetReplyTimeout(time.Duration)          {}
func (c *dumpChannel) CheckCompatiblity(...api.Message) error { return nil }
func (c *dumpChannel) Close()                                 { c.closed = true }

type dumpRequestCtx struct{ ch *dumpChannel }

func (r *dumpRequestCtx) ReceiveReply(msg api.Message) error {
	if r.ch.sendErr != nil {
		return r.ch.sendErr
	}
	if reply, ok := msg.(*interfaces.SwInterfaceSetMacAddressReply); ok {
		*reply = r.ch.macReply
	}
	return nil
}

type dumpMultiCtx struct {
	ch  *dumpChannel
	pos int
}

func (m *dumpMultiCtx) ReceiveReply(msg api.Message) (bool, error) {
	if m.ch.receiveErr != nil {
		return false, m.ch.receiveErr
	}
	if m.pos >= len(m.ch.details) {
		return true, nil
	}
	d, ok := msg.(*interfaces.SwInterfaceDetails)
	if !ok {
		return false, fmt.Errorf("dumpMultiCtx: unexpected reply type %T", msg)
	}
	*d = m.ch.details[m.pos]
	m.pos++
	return false, nil
}

// asciiName converts a string to a 64-byte VPP-style fixed field.
func asciiName(s string) string {
	b := make([]byte, 64)
	copy(b, s)
	return string(b)
}

// --- trimCString ---

func TestTrimCStringNULTerminated(t *testing.T) {
	// VALIDATES: AC-10 -- VPP fixed-length strings parsed correctly
	// PREVENTS: returning strings with embedded NULs to consumers
	got := trimCString("TenGigabitEthernet3/0/0\x00\x00\x00")
	if got != "TenGigabitEthernet3/0/0" {
		t.Errorf("trimCString: got %q, want %q", got, "TenGigabitEthernet3/0/0")
	}
}

func TestTrimCStringNoNUL(t *testing.T) {
	// VALIDATES: strings without NUL are returned verbatim
	got := trimCString("loop0")
	if got != "loop0" {
		t.Errorf("trimCString: got %q, want loop0", got)
	}
}

// --- detailsToInfo ---

func TestDetailsToInfoAdminUp(t *testing.T) {
	// VALIDATES: AC-10 -- admin state "up" derived from ADMIN_UP flag
	d := &interfaces.SwInterfaceDetails{
		SwIfIndex:     1,
		InterfaceName: asciiName("xe0"),
		Flags:         interface_types.IF_STATUS_API_FLAG_ADMIN_UP,
		Mtu:           []uint32{9000, 0, 0, 0},
	}
	info := detailsToInfo(d)
	if info.Name != "xe0" {
		t.Errorf("Name: got %q, want xe0", info.Name)
	}
	if info.State != "up" {
		t.Errorf("State: got %q, want up", info.State)
	}
	if info.MTU != 9000 {
		t.Errorf("MTU: got %d, want 9000", info.MTU)
	}
	if info.Index != 1 {
		t.Errorf("Index: got %d, want 1", info.Index)
	}
}

func TestDetailsToInfoAdminDown(t *testing.T) {
	// VALIDATES: AC-10 -- absence of ADMIN_UP flag reports state "down"
	d := &interfaces.SwInterfaceDetails{
		SwIfIndex:     2,
		InterfaceName: asciiName("loop0"),
		Flags:         0,
	}
	info := detailsToInfo(d)
	if info.State != "down" {
		t.Errorf("State: got %q, want down", info.State)
	}
}

// --- ListInterfaces ---

func TestListInterfacesConvertsEveryDetails(t *testing.T) {
	// VALIDATES: AC-10 -- SwInterfaceDump results converted to InterfaceInfo
	ch := &dumpChannel{
		details: []interfaces.SwInterfaceDetails{
			{SwIfIndex: 0, InterfaceName: asciiName("local0")},
			{SwIfIndex: 1, InterfaceName: asciiName("loop0"),
				Flags: interface_types.IF_STATUS_API_FLAG_ADMIN_UP},
		},
	}
	b := &vppBackendImpl{ch: ch, names: newNameMap()}

	got, err := b.ListInterfaces()
	if err != nil {
		t.Fatalf("ListInterfaces: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len: got %d, want 2", len(got))
	}
	if got[0].Name != "local0" {
		t.Errorf("got[0].Name = %q, want local0", got[0].Name)
	}
	if got[1].State != "up" {
		t.Errorf("got[1].State = %q, want up", got[1].State)
	}
}

func TestListInterfacesRequestType(t *testing.T) {
	// VALIDATES: AC-10 -- SwInterfaceDump is the RPC invoked
	ch := &dumpChannel{}
	b := &vppBackendImpl{ch: ch, names: newNameMap()}
	_, err := b.ListInterfaces()
	if err != nil {
		t.Fatalf("ListInterfaces: %v", err)
	}
	if _, ok := ch.lastRequest.(*interfaces.SwInterfaceDump); !ok {
		t.Errorf("lastRequest type: got %T, want *interfaces.SwInterfaceDump", ch.lastRequest)
	}
}

func TestListInterfacesReceiveError(t *testing.T) {
	// VALIDATES: reply error propagates as ifacevpp error
	ch := &dumpChannel{receiveErr: fmt.Errorf("VPP dead")}
	b := &vppBackendImpl{ch: ch, names: newNameMap()}
	if _, err := b.ListInterfaces(); err == nil {
		t.Fatal("expected error when ReceiveReply fails")
	}
}

// --- GetInterface ---

func TestGetInterfaceExactMatch(t *testing.T) {
	// VALIDATES: NameFilter is substring -- exact match required
	ch := &dumpChannel{
		details: []interfaces.SwInterfaceDetails{
			{SwIfIndex: 10, InterfaceName: asciiName("xe0")},
			{SwIfIndex: 11, InterfaceName: asciiName("xe0.100")},
		},
	}
	b := &vppBackendImpl{ch: ch, names: newNameMap()}

	info, err := b.GetInterface("xe0")
	if err != nil {
		t.Fatalf("GetInterface: %v", err)
	}
	if info.Index != 10 {
		t.Errorf("Index: got %d, want 10 (not sub-if 11)", info.Index)
	}
}

func TestGetInterfaceNotFound(t *testing.T) {
	// VALIDATES: missing interface returns error with name
	ch := &dumpChannel{
		details: []interfaces.SwInterfaceDetails{
			{SwIfIndex: 10, InterfaceName: asciiName("xe1")},
		},
	}
	b := &vppBackendImpl{ch: ch, names: newNameMap()}
	if _, err := b.GetInterface("xe0"); err == nil {
		t.Fatal("expected error for missing interface")
	}
}

// --- Get/SetMACAddress ---

func TestGetMACAddressFromDetails(t *testing.T) {
	// VALIDATES: L2Address bytes formatted as EUI-48 colon form
	ch := &dumpChannel{
		details: []interfaces.SwInterfaceDetails{
			{
				SwIfIndex:     5,
				InterfaceName: asciiName("xe0"),
				L2Address:     [6]byte{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff},
			},
		},
	}
	b := &vppBackendImpl{ch: ch, names: newNameMap()}

	mac, err := b.GetMACAddress("xe0")
	if err != nil {
		t.Fatalf("GetMACAddress: %v", err)
	}
	if mac != "aa:bb:cc:dd:ee:ff" {
		t.Errorf("mac: got %q, want aa:bb:cc:dd:ee:ff", mac)
	}
}

func TestSetMACAddressSendsRequest(t *testing.T) {
	// VALIDATES: SwInterfaceSetMacAddress invoked with parsed MAC bytes
	ch := &dumpChannel{}
	b := &vppBackendImpl{ch: ch, names: newNameMap()}
	b.names.Add("xe0", 3, "xe0")

	if err := b.SetMACAddress("xe0", "aa:bb:cc:dd:ee:ff"); err != nil {
		t.Fatalf("SetMACAddress: %v", err)
	}
	req, ok := ch.lastRequest.(*interfaces.SwInterfaceSetMacAddress)
	if !ok {
		t.Fatalf("lastRequest type: got %T, want *SwInterfaceSetMacAddress", ch.lastRequest)
	}
	if req.SwIfIndex != 3 {
		t.Errorf("SwIfIndex: got %d, want 3", req.SwIfIndex)
	}
	want := [6]byte{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff}
	if req.MacAddress != want {
		t.Errorf("MacAddress: got %v, want %v", req.MacAddress, want)
	}
}

func TestSetMACAddressInvalidString(t *testing.T) {
	// VALIDATES: malformed MAC rejected before SwInterfaceSetMacAddress call
	ch := &dumpChannel{}
	b := &vppBackendImpl{ch: ch, names: newNameMap()}
	b.names.Add("xe0", 1, "xe0")

	if err := b.SetMACAddress("xe0", "not-a-mac"); err == nil {
		t.Fatal("expected error for invalid MAC")
	}
	// The lazy populate dump already ran; what matters is that no
	// SwInterfaceSetMacAddress request was issued.
	if _, ok := ch.lastRequest.(*interfaces.SwInterfaceSetMacAddress); ok {
		t.Error("SwInterfaceSetMacAddress should not be sent for invalid MAC")
	}
}

func TestSetMACAddressUnknownInterface(t *testing.T) {
	// VALIDATES: unknown interface rejected before parsing MAC
	ch := &dumpChannel{}
	b := &vppBackendImpl{ch: ch, names: newNameMap()}

	if err := b.SetMACAddress("xe99", "aa:bb:cc:dd:ee:ff"); err == nil {
		t.Fatal("expected error for unknown interface")
	}
}

func TestSetMACAddressRetvalError(t *testing.T) {
	// VALIDATES: non-zero retval produces error
	ch := &dumpChannel{macReply: interfaces.SwInterfaceSetMacAddressReply{Retval: -1}}
	b := &vppBackendImpl{ch: ch, names: newNameMap()}
	b.names.Add("xe0", 1, "xe0")

	if err := b.SetMACAddress("xe0", "aa:bb:cc:dd:ee:ff"); err == nil {
		t.Fatal("expected error for retval=-1")
	}
}

// --- populateNameMap (AC-13 NameMappingPopulate) ---

func TestPopulateNameMap(t *testing.T) {
	// VALIDATES: AC-13 -- map populated from SwInterfaceDump at startup
	ch := &dumpChannel{
		details: []interfaces.SwInterfaceDetails{
			{SwIfIndex: 0, InterfaceName: asciiName("local0")},
			{SwIfIndex: 1, InterfaceName: asciiName("TenGigabitEthernet3/0/0")},
			{SwIfIndex: 2, InterfaceName: asciiName("loop0")},
		},
	}
	b := &vppBackendImpl{ch: ch, names: newNameMap()}

	if err := b.populateNameMap(); err != nil {
		t.Fatalf("populateNameMap: %v", err)
	}
	if b.names.Len() != 3 {
		t.Errorf("map size: got %d, want 3", b.names.Len())
	}
	idx, ok := b.names.LookupIndex("TenGigabitEthernet3/0/0")
	if !ok || idx != 1 {
		t.Errorf("LookupIndex(Ten3/0/0): got %d,%v want 1,true", idx, ok)
	}
}

func TestPopulateNameMapEmptyNameSkipped(t *testing.T) {
	// VALIDATES: interfaces with blank name (pure NUL) are skipped
	ch := &dumpChannel{
		details: []interfaces.SwInterfaceDetails{
			{SwIfIndex: 0, InterfaceName: asciiName("")},
			{SwIfIndex: 1, InterfaceName: asciiName("xe0")},
		},
	}
	b := &vppBackendImpl{ch: ch, names: newNameMap()}

	if err := b.populateNameMap(); err != nil {
		t.Fatalf("populateNameMap: %v", err)
	}
	if b.names.Len() != 1 {
		t.Errorf("map size: got %d, want 1 (blank skipped)", b.names.Len())
	}
}
