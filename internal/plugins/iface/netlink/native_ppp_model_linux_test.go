// Design: docs/features/interfaces.md -- guarded PPP serializer and ACK models
// These are non-native wire models. Numeric fixtures grant no native authority.

//go:build linux

package ifacenetlink

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"net/netip"
	"testing"

	"github.com/vishvananda/netlink/nl"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/rtproto"
)

// TestNativePPPModelLinkRequests models exact original-index link request bytes;
// useful native authority and kernel effects are integration-test obligations.
func TestNativePPPModelLinkRequests(t *testing.T) {
	for _, entry := range []struct {
		name      string
		flags     uint32
		change    uint32
		attribute *nl.RtAttr
	}{
		{"mtu", 0, 0, nl.NewRtAttr(unix.IFLA_MTU, nl.Uint32Attr(1431))},
		{"up", unix.IFF_UP, unix.IFF_UP, nil},
		{"down", 0, unix.IFF_UP, nil},
		{"alias", 0, 0, nl.NewRtAttr(unix.IFLA_IFALIAS, nl.ZeroTerminated("ze:original"))},
	} {
		t.Run(entry.name, func(t *testing.T) {
			request := nativePPPLinkRequest(37, entry.flags, entry.change, entry.attribute)
			wire := request.Serialize()
			if request.Type != unix.RTM_SETLINK {
				t.Fatal("link mutation did not use RTM_SETLINK")
			}
			if request.Flags != unix.NLM_F_REQUEST|unix.NLM_F_ACK {
				t.Fatalf("link flags=%#x", request.Flags)
			}
			header := wire[unix.SizeofNlMsghdr:]
			if int32(nl.NativeEndian().Uint32(header[4:8])) != 37 {
				t.Fatal("link request lost captured original index")
			}
			if nl.NativeEndian().Uint32(header[8:12]) != entry.flags {
				t.Fatal("link request changed flags")
			}
			if nl.NativeEndian().Uint32(header[12:16]) != entry.change {
				t.Fatal("link request changed admin flag mask")
			}
			attributes := nativePPPModelAttributes(t, header[unix.SizeofIfInfomsg:])
			nativePPPModelOwnerLast(t, wire, nativePPPLinkOwnerAttribute)
			if entry.attribute != nil {
				if !bytes.Equal(attributes[uint16(entry.attribute.Type)], entry.attribute.Data) {
					t.Fatal("link mutation lost its actual requested value")
				}
			}
		})
	}
}

// TestNativePPPModelAddressRequests preserves the existing peer-mask encoding,
// full valid /64 IPv6 pair, exclusive address add and exact retained delete pair.
func TestNativePPPModelAddressRequests(t *testing.T) {
	for _, pair := range []struct {
		local  string
		peer   string
		family byte
	}{
		{"198.51.100.1/24", "198.51.100.2/24", unix.AF_INET},
		{"198.51.100.1/24", "198.51.100.2/32", unix.AF_INET},
		{"fe80::1/64", "fe80::2/64", unix.AF_INET6},
		{"2001:db8::1/128", "2001:db8::2/128", unix.AF_INET6},
	} {
		for _, kind := range []int{unix.RTM_NEWADDR, unix.RTM_DELADDR} {
			local := netip.MustParsePrefix(pair.local)
			peer := netip.MustParsePrefix(pair.peer)
			request := nativePPPAddressRequest(kind, 37, local, peer)
			wire := request.Serialize()
			header := wire[unix.SizeofNlMsghdr:]
			if header[0] != pair.family {
				t.Fatal("address request changed native family")
			}
			if int(header[1]) != peer.Bits() {
				t.Fatal("address request lost the retained peer mask")
			}
			if nl.NativeEndian().Uint32(header[4:8]) != 37 {
				t.Fatal("address request lost captured original index")
			}
			attributes := nativePPPModelAttributes(t, header[unix.SizeofIfAddrmsg:])
			if !bytes.Equal(attributes[unix.IFA_LOCAL], local.Addr().AsSlice()) {
				t.Fatal("address request lost actual local address")
			}
			if !bytes.Equal(attributes[unix.IFA_ADDRESS], peer.Addr().AsSlice()) {
				t.Fatal("address request lost actual retained peer address")
			}
			nativePPPModelOwnerLast(t, wire, nativePPPAddressOwnerAttribute)
			wantedFlags := uint16(unix.NLM_F_REQUEST | unix.NLM_F_ACK)
			if kind == unix.RTM_NEWADDR {
				wantedFlags |= unix.NLM_F_CREATE | unix.NLM_F_EXCL
			}
			if request.Flags != wantedFlags {
				t.Fatal("address request changed add/delete semantics")
			}
		}
	}
}

// TestNativePPPModelRouteReplaceRequests checks actual byte-level route identity
// and replacement flags; replacing exclusive creation would fail this model.
func TestNativePPPModelRouteReplaceRequests(t *testing.T) {
	for _, pair := range []struct{ destination, gateway string }{
		{"192.0.2.199/24", "198.51.100.2"},
		{"2001:db8:1234::19/48", "fe80::2"},
	} {
		for _, kind := range []int{unix.RTM_NEWROUTE, unix.RTM_DELROUTE} {
			destination := netip.MustParsePrefix(pair.destination)
			gateway := netip.MustParseAddr(pair.gateway)
			request := nativePPPRouteRequest(kind, 37, destination, gateway, 19, rtproto.Iface)
			wire := request.Serialize()
			header := wire[unix.SizeofNlMsghdr:]
			if int(header[1]) != destination.Bits() {
				t.Fatal("route request lost destination mask")
			}
			if header[4] != unix.RT_TABLE_MAIN {
				t.Fatal("route request changed main-table semantics")
			}
			if header[5] != byte(rtproto.Iface) {
				t.Fatal("route request lost ownership protocol")
			}
			attributes := nativePPPModelAttributes(t, header[unix.SizeofRtMsg:])
			if !bytes.Equal(attributes[unix.RTA_DST], destination.Masked().Addr().AsSlice()) {
				t.Fatal("route request changed masked destination")
			}
			if !bytes.Equal(attributes[unix.RTA_GATEWAY], gateway.AsSlice()) {
				t.Fatal("route request changed gateway")
			}
			if nl.NativeEndian().Uint32(attributes[unix.RTA_OIF]) != 37 {
				t.Fatal("route request lost captured original output index")
			}
			if nl.NativeEndian().Uint32(attributes[unix.RTA_PRIORITY]) != 19 {
				t.Fatal("route request lost retained priority")
			}
			nativePPPModelOwnerLast(t, wire, nativePPPRouteOwnerAttribute)
			wantedFlags := uint16(unix.NLM_F_REQUEST | unix.NLM_F_ACK)
			if kind == unix.RTM_NEWROUTE {
				wantedFlags |= unix.NLM_F_CREATE | unix.NLM_F_REPLACE
				if header[6] != unix.RT_SCOPE_UNIVERSE {
					t.Fatal("route replace changed scope")
				}
			}
			if request.Flags != wantedFlags {
				t.Fatalf("route flags=%#x, want %#x", request.Flags, wantedFlags)
			}
		}
	}
}

// TestNativePPPModelCompleteACK requires full original-request digests on
// NACKs and discriminates valid ACK/NACK from malformed or unrelated replies.
func TestNativePPPModelCompleteACK(t *testing.T) {
	request := nativePPPLinkRequest(37, 0, 0, nl.NewRtAttr(unix.IFLA_MTU, nl.Uint32Attr(1431)))
	request.Seq = 41
	request.Pid = 73
	original := request.Serialize()
	// Model-only fd number: these bytes never reach a native syscall and mint
	// no authority. Changing that original OWNER_FD must invalidate a NACK.
	nl.NativeEndian().PutUint32(original[len(original)-4:], 19)
	digest := sha256.Sum256(original)
	positive := nativePPPModelACK(original, 0, true)
	negative := nativePPPModelACK(original, -int32(unix.ESRCH), false)
	for _, control := range []struct {
		name string
		wire []byte
		want error
	}{
		{"positive", positive, nil},
		{"negative exact owner request", negative, unix.ESRCH},
	} {
		t.Run(control.name, func(t *testing.T) {
			nativeErr, parseErr := nativePPPParseACK(request.NlMsghdr, digest, control.wire)
			if parseErr != nil {
				t.Fatal(parseErr)
			}
			if !errors.Is(nativeErr, control.want) {
				t.Fatalf("native error=%v, want %v", nativeErr, control.want)
			}
		})
	}
	for _, malformed := range []struct {
		name string
		make func() []byte
	}{
		{"empty", func() []byte { return nil }},
		{"trailing byte", func() []byte { return append(bytes.Clone(positive), 1) }},
		{"duplicate ACK", func() []byte { return append(bytes.Clone(positive), positive...) }},
		{"NACK then malformed", func() []byte { return append(bytes.Clone(negative), 1) }},
		{"outer sequence", func() []byte { p := bytes.Clone(positive); p[8]++; return p }},
		{"outer port", func() []byte { p := bytes.Clone(positive); p[12]++; return p }},
		{"original type", func() []byte { p := bytes.Clone(positive); p[24]++; return p }},
		{"original flags", func() []byte { p := bytes.Clone(positive); p[26]++; return p }},
		{"original sequence", func() []byte { p := bytes.Clone(positive); p[28]++; return p }},
		{"original port", func() []byte { p := bytes.Clone(positive); p[32]++; return p }},
		{"owner-fd echo", func() []byte { p := bytes.Clone(negative); p[len(p)-4]++; return p }},
		{"original index echo", func() []byte { p := bytes.Clone(negative); p[40]++; return p }},
		{"capped NACK", func() []byte { return nativePPPModelACK(original, -int32(unix.ESRCH), true) }},
		{"positive errno", func() []byte { return nativePPPModelACK(original, 1, true) }},
		{"errno range", func() []byte { return nativePPPModelACK(original, -4096, false) }},
	} {
		t.Run(malformed.name, func(t *testing.T) {
			nativeErr, parseErr := nativePPPParseACK(request.NlMsghdr, digest, malformed.make())
			if parseErr == nil {
				t.Fatal("malformed ACK became a definitive native outcome")
			}
			if malformed.name == "NACK then malformed" {
				if !errors.Is(nativeErr, unix.ESRCH) {
					t.Fatal("later malformed data lost independently observed native errno")
				}
			}
		})
	}
}

func nativePPPModelAttributes(t *testing.T, wire []byte) map[uint16][]byte {
	t.Helper()
	attributes, err := nl.ParseRouteAttr(wire)
	if err != nil {
		t.Fatal(err)
	}
	result := make(map[uint16][]byte)
	for _, attribute := range attributes {
		if _, present := result[attribute.Attr.Type]; present {
			t.Fatal("duplicate attribute in model request")
		}
		result[attribute.Attr.Type] = attribute.Value
	}
	return result
}

func nativePPPModelOwnerLast(t *testing.T, wire []byte, kind uint16) {
	t.Helper()
	owner := wire[len(wire)-8:]
	if nl.NativeEndian().Uint16(owner[:2]) != 8 {
		t.Fatal("last owner attribute has another native width")
	}
	if nl.NativeEndian().Uint16(owner[2:4]) != kind {
		t.Fatal("original owner attribute is not last")
	}
	if nl.NativeEndian().Uint32(owner[4:]) != 0 {
		t.Fatal("serializer leaked a descriptor outside its actual borrow")
	}
}

func nativePPPModelACK(original []byte, code int32, capped bool) []byte {
	echo := original
	flags := uint16(0)
	if capped {
		echo = original[:unix.SizeofNlMsghdr]
		flags = unix.NLM_F_CAPPED
	}
	wire := make([]byte, unix.SizeofNlMsghdr+4+len(echo))
	nl.NativeEndian().PutUint32(wire[:4], uint32(len(wire)))
	nl.NativeEndian().PutUint16(wire[4:6], unix.NLMSG_ERROR)
	nl.NativeEndian().PutUint16(wire[6:8], flags)
	copy(wire[8:16], original[8:16])
	nl.NativeEndian().PutUint32(wire[16:20], uint32(code))
	copy(wire[20:], echo)
	return wire
}
