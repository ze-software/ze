//go:build integration && linux

// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- exact state selector sets.
// Related: rfc4301_state_scope_integration_linux_test.go -- real constructor and packet boundaries.
// RFC naming: untagged -- observed kernel ABI and discrimination records are still owed.

package engine

import (
	"bytes"
	"encoding/binary"
	"errors"
	"net"
	"testing"

	"github.com/vishvananda/netlink/nl"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
)

// The appended private ABI follows Linux 7.3's XFRMA_IPTFS_PKT_SIZE (41).
// Its declaration is include/uapi/linux/xfrm.h in the Ze runtime kernel patch:
// xfrm_user_state_scope is native u32 flags/count plus existing xfrm_selector[];
// xfrm_user_ze_caps is native u32 features/max_selectors/guard_refs/abi.
const (
	rfcScopeAttribute       = 42
	rfcScopeCapsAttribute   = 43
	rfcScopeGetDefault      = 0x28
	rfcScopeABIVersion      = 0x5a450001
	rfcScopeSelectorMaximum = 64
	rfcScopeFeatureExact    = 1 << 0
	rfcScopeFeatureQuote    = 1 << 1
	// Context, guard and final-ICMP bits may appear once their patches are present.
	rfcScopeKnownFeatures = (1 << 5) - 1
)

// rfcScopeAttributeValue deliberately reads raw netlink replies: the existing
// high-level state reader discards attributes it does not recognize.
func rfcScopeAttributeValue(t *testing.T, payload []byte, kind uint16) []byte {
	t.Helper()
	attrs, err := nl.ParseRouteAttr(payload)
	if err != nil {
		t.Fatalf("parse the raw XFRM attributes: %v", err)
	}
	var found []byte
	for _, attr := range attrs {
		if attr.Attr.Type&0x3fff != kind {
			continue
		}
		if found != nil {
			t.Fatalf("raw XFRM reply contains duplicate attribute %d", kind)
		}
		found = attr.Value
	}
	if found == nil {
		t.Fatalf("raw kernel reply has no attribute %d; an ignored request is not proof of scope enforcement", kind)
	}
	return found
}

func rfcScopeCapabilities(t *testing.T) [4]uint32 {
	t.Helper()
	req := nl.NewNetlinkRequest(rfcScopeGetDefault, unix.NLM_F_REQUEST)
	req.AddRawData([]byte{0, 0, 0})
	messages, err := req.Execute(unix.NETLINK_XFRM, rfcScopeGetDefault)
	if err != nil {
		t.Fatalf("read the actual kernel's XFRM capability receipt: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("GETDEFAULT returned %d messages, want one", len(messages))
	}
	if len(messages[0]) < 4 {
		t.Fatal("GETDEFAULT contains no aligned XFRMA_ZE_CAPS receipt; this kernel cannot attest exact state scopes")
	}
	value := rfcScopeAttributeValue(t, messages[0][4:], rfcScopeCapsAttribute)
	if len(value) != 16 {
		t.Fatalf("kernel capability receipt has %d bytes, want 16", len(value))
	}
	return [4]uint32{
		binary.NativeEndian.Uint32(value[0:4]), binary.NativeEndian.Uint32(value[4:8]),
		binary.NativeEndian.Uint32(value[8:12]), binary.NativeEndian.Uint32(value[12:16]),
	}
}

// An old kernel may silently ignore a new request attribute. Only a native
// GETDEFAULT receipt with the exact ABI identity can advertise this behavior.
//
//ze:guest-root
func TestRFC4301KernelAdvertisesExactStateScope(t *testing.T) {
	newEspPolLab(t)
	actual := rfcScopeCapabilities(t)
	if actual[3] != rfcScopeABIVersion {
		t.Fatalf("kernel scope ABI = %#x, want %#x", actual[3], rfcScopeABIVersion)
	}
	if actual[0]&rfcScopeFeatureExact == 0 {
		t.Fatal("kernel did not advertise immutable exact state scopes")
	}
	if actual[0]&rfcScopeFeatureQuote == 0 {
		t.Fatal("kernel did not advertise availability-aware quoted-header matching")
	}
	if actual[0]&^uint32(rfcScopeKnownFeatures) != 0 {
		t.Fatalf("kernel advertised unknown scope features %#x", actual[0])
	}
	if actual[1] != rfcScopeSelectorMaximum {
		t.Fatalf("kernel selector bound = %d, want %d", actual[1], rfcScopeSelectorMaximum)
	}
	if actual[2] != 0 {
		t.Fatalf("fresh namespace has %d fragment-guard references, want zero", actual[2])
	}
	if second := rfcScopeCapabilities(t); second != actual {
		t.Fatalf("read-only capability query changed its receipt: %x -> %x", actual, second)
	}
	// The capability attribute is a read-only receipt, never a mutable gather switch.
	req := nl.NewNetlinkRequest(rfcScopeGetDefault, unix.NLM_F_REQUEST)
	req.AddRawData([]byte{0, 0, 0, 0})
	req.AddData(nl.NewRtAttr(rfcScopeCapsAttribute, make([]byte, 16)))
	_, err := req.Execute(unix.NETLINK_XFRM, rfcScopeGetDefault)
	if !errors.Is(err, unix.EINVAL) {
		t.Fatalf("writing the read-only capability receipt returned %v, want EINVAL", err)
	}
}

func rfcScopeStateMessages(t *testing.T, spi uint32, destination net.IP, dump bool) [][]byte {
	t.Helper()
	flags := unix.NLM_F_REQUEST
	if dump {
		flags |= unix.NLM_F_DUMP
	}
	req := nl.NewNetlinkRequest(nl.XFRM_MSG_GETSA, flags)
	if !dump {
		id := &nl.XfrmUsersaId{Spi: nl.Swap32(spi), Family: unix.AF_INET, Proto: 50}
		id.Daddr.FromIP(destination)
		req.AddData(id)
	}
	messages, err := req.Execute(unix.NETLINK_XFRM, nl.XFRM_MSG_NEWSA)
	if err != nil {
		t.Fatalf("read raw SAD state SPI %#x (dump %t): %v", spi, dump, err)
	}
	return messages
}

func rfcScopeExpectedSelector(t *testing.T, pair tsPair, inbound bool) []byte {
	t.Helper()
	source, destination := pair.I, pair.R
	if inbound {
		source, destination = pair.R, pair.I
	}
	sel := nl.XfrmSelector{Family: unix.AF_INET, Proto: source.Proto}
	if source.Net.IP.To4() == nil {
		sel.Family = unix.AF_INET6
	}
	sel.Saddr.FromIP(source.Net.IP)
	sel.Daddr.FromIP(destination.Net.IP)
	sourceBits, _ := source.Net.Mask.Size()
	destinationBits, _ := destination.Net.Mask.Size()
	sel.PrefixlenS, sel.PrefixlenD = uint8(sourceBits), uint8(destinationBits)
	if source.Port.Form == ipsec.PortSingle {
		sel.Sport, sel.SportMask = nl.Swap16(source.Port.Port), 0xffff
	}
	if destination.Port.Form == ipsec.PortSingle {
		sel.Dport, sel.DportMask = nl.Swap16(destination.Port.Port), 0xffff
	}
	return sel.Serialize()
}

func rfcScopeWantState(t *testing.T, child *ChildSA, pairs []tsPair, inbound, dump bool) {
	t.Helper()
	spi, source, destination := child.OutboundSPI, child.LocalAddr, child.RemoteAddr
	if inbound {
		spi, source, destination = child.InboundSPI, child.RemoteAddr, child.LocalAddr
	}
	messages := rfcScopeStateMessages(t, spi, destination, dump)
	var state []byte
	for _, message := range messages {
		if len(message) < nl.SizeofXfrmUsersaInfo {
			t.Fatalf("raw SAD message has %d bytes, want at least %d", len(message), nl.SizeofXfrmUsersaInfo)
		}
		info := nl.DeserializeXfrmUsersaInfo(message)
		if info.Id.Spi != nl.Swap32(spi) {
			continue
		}
		if state != nil {
			t.Fatalf("raw SAD returned SPI %#x twice", spi)
		}
		if info.Family != unix.AF_INET {
			t.Fatalf("SPI %#x outer family = %d, want IPv4 despite mixed inner scopes", spi, info.Family)
		}
		if !info.Saddr.ToIP().Equal(source) {
			t.Fatalf("SPI %#x outer source = %s, want %s", spi, info.Saddr.ToIP(), source)
		}
		if !info.Id.Daddr.ToIP().Equal(destination) {
			t.Fatalf("SPI %#x outer destination = %s, want %s", spi, info.Id.Daddr.ToIP(), destination)
		}
		state = message
	}
	if state == nil {
		t.Fatalf("raw SAD did not return installed SPI %#x", spi)
	}
	value := rfcScopeAttributeValue(t, state[nl.SizeofXfrmUsersaInfo:], rfcScopeAttribute)
	if len(value) < 8 {
		t.Fatalf("SPI %#x scope has %d bytes, want flags and count", spi, len(value))
	}
	if flags := binary.NativeEndian.Uint32(value[:4]); flags != 0 {
		t.Fatalf("SPI %#x scope flags = %#x, want no unnegotiated noninitial-fragment permission", spi, flags)
	}
	count := binary.NativeEndian.Uint32(value[4:8])
	if count != uint32(len(pairs)) {
		t.Fatalf("SPI %#x retained %d selectors, want all %d negotiated pairs", spi, count, len(pairs))
	}
	if len(value) != 8+len(pairs)*nl.SizeofXfrmSelector {
		t.Fatalf("SPI %#x scope length = %d, want %d", spi, len(value), 8+len(pairs)*nl.SizeofXfrmSelector)
	}
	seen := make([]bool, len(pairs))
	for index := range pairs {
		actual := value[8+index*nl.SizeofXfrmSelector : 8+(index+1)*nl.SizeofXfrmSelector]
		found := false
		for pairIndex, pair := range pairs {
			if seen[pairIndex] {
				continue
			}
			if bytes.Equal(actual, rfcScopeExpectedSelector(t, pair, inbound)) {
				seen[pairIndex], found = true, true
				break
			}
		}
		if !found {
			t.Errorf("SPI %#x inbound %t dump %t selector %d is not an exact negotiated address/family/protocol/port-mask entry: %x", spi, inbound, dump, index, actual)
		}
	}
}

// RFC 4301 Section 4.4.2: "For each of the selectors defined in Section 4.4.1.1,
// the entry for an inbound SA in the SAD MUST be initially populated with the
// value or values negotiated at the time the SA was created."
//
//ze:guest-root
func TestRFC4301ChildStateScopeReadback(t *testing.T) {
	for _, mixed := range []bool{false, true} {
		name := "ipv4-pairs"
		if mixed {
			name = "mixed-inner-families"
		}
		t.Run(name, func(t *testing.T) {
			newEspPolLab(t)
			espPolSPD(t)
			pairs := rfcScopePairs(t, 5000, 5002)
			pairs[0].R.Port = ipsec.PortSelector{Form: ipsec.PortSingle, Port: sadSelSourcePort}
			if mixed {
				pairs[1].I.Net = mustCIDRNet(t, "2001:db8:122::/64")
				pairs[1].R.Net = mustCIDRNet(t, "2001:db8:121::/64")
			}
			children, _ := rfcScopeChildren(t, pairs)
			for _, dump := range []bool{false, true} {
				for _, inbound := range []bool{true, false} {
					rfcScopeWantState(t, children[0], pairs, inbound, dump)
				}
			}
		})
	}
}
