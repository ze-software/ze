// Design: docs/features/interfaces.md -- Netlink interface monitor
// Related: nduseropt_linux.go -- decodeNDUserOpt and the nd-user-option emit

//go:build linux

package ifacenetlink

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"net/netip"
	"testing"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/iface"
	ifaceevents "github.com/ze-software/ze/internal/core/iface/events"
)

// ndUserOptIfindex is the interface index every test message names.
const ndUserOptIfindex = 7

// ndUserOptBody builds an RTM_NEWNDUSEROPT body on ndUserOptIfindex the way
// ndisc_ra_useropt does: the native-endian header, the option, padding to 4,
// then NDUSEROPT_SRCADDR.
func ndUserOptBody(icmpType uint8, option []byte, router netip.Addr) []byte {
	body := make([]byte, ndUserOptHeaderLen, 64)
	body[0] = unix.AF_INET6
	binary.NativeEndian.PutUint16(body[2:4], uint16(len(option)))
	binary.NativeEndian.PutUint32(body[4:8], ndUserOptIfindex)
	body[8] = icmpType
	body = append(body, option...)
	for len(body)%4 != 0 {
		body = append(body, 0)
	}
	attr := make([]byte, 4, 20)
	binary.NativeEndian.PutUint16(attr[0:2], 20)
	binary.NativeEndian.PutUint16(attr[2:4], ndUserOptSourceAddr)
	r := router.As16()
	attr = append(attr, r[:]...)
	return append(body, attr...)
}

// rdnssWire is one RDNSS option: Length 3, Lifetime 600, 2001:db8::53.
var rdnssWire = []byte{
	25, 3, 0, 0, 0, 0, 0x02, 0x58,
	0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x53,
}

// TestDecodeNDUserOpt checks the header, option and source-address
// decoding of one message as the kernel lays it out.
func TestDecodeNDUserOpt(t *testing.T) {
	router := netip.MustParseAddr("fe80::1")
	msg, err := decodeNDUserOpt(ndUserOptBody(icmpv6RouterAdvertisement, rdnssWire, router))
	if err != nil {
		t.Fatal(err)
	}
	if msg.ifindex != ndUserOptIfindex {
		t.Fatalf("ifindex %d, want %d", msg.ifindex, ndUserOptIfindex)
	}
	if msg.icmpType != icmpv6RouterAdvertisement {
		t.Fatalf("icmp type %d", msg.icmpType)
	}
	if !bytes.Equal(msg.options, rdnssWire) {
		t.Fatalf("options % x", msg.options)
	}
	if msg.router != router {
		t.Fatalf("router %v, want %v", msg.router, router)
	}
}

// TestDecodeNDUserOptRefusesShortMessages checks that a body shorter than
// its header or its own options length is an error, never a read past it.
func TestDecodeNDUserOptRefusesShortMessages(t *testing.T) {
	full := ndUserOptBody(icmpv6RouterAdvertisement, rdnssWire, netip.MustParseAddr("fe80::1"))
	for _, n := range []int{0, ndUserOptHeaderLen - 1, ndUserOptHeaderLen + len(rdnssWire) - 1} {
		if _, err := decodeNDUserOpt(full[:n]); err == nil {
			t.Fatalf("%d octets decoded", n)
		}
	}
	wrongFamily := bytes.Clone(full)
	wrongFamily[0] = unix.AF_INET
	if _, err := decodeNDUserOpt(wrongFamily); err == nil {
		t.Fatal("AF_INET message decoded")
	}
}

// TestHandleNDUserOptEmitsOption checks that an RA message becomes one
// nd-user-option event naming the device, the router and the option bytes,
// and that a message from another ICMPv6 type emits nothing.
func TestHandleNDUserOptEmitsOption(t *testing.T) {
	m, bus := newTestMonitor()
	m.linkNames.Store(ndUserOptIfindex, "wan0")
	m.handleNDUserOpt(ndUserOptBody(133, rdnssWire, netip.MustParseAddr("fe80::1")))
	if events := bus.snapshot(); len(events) != 0 {
		t.Fatalf("non-RA message emitted %v", events)
	}
	m.handleNDUserOpt(ndUserOptBody(icmpv6RouterAdvertisement, rdnssWire, netip.MustParseAddr("fe80::1")))
	events := bus.snapshot()
	if len(events) != 1 {
		t.Fatalf("events %v, want one", events)
	}
	if events[0].EventType != ifaceevents.EventNDUserOption {
		t.Fatalf("event type %q", events[0].EventType)
	}
	data, ok := events[0].Payload.(string)
	if !ok {
		t.Fatalf("payload %T", events[0].Payload)
	}
	var p iface.NDUserOptionPayload
	if err := json.Unmarshal([]byte(data), &p); err != nil {
		t.Fatal(err)
	}
	if p.Name != "wan0" || p.Router != "fe80::1" || !bytes.Equal(p.Option, rdnssWire) {
		t.Fatalf("payload %+v", p)
	}
}
