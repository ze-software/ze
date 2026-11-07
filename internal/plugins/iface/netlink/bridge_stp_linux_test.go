//go:build linux

// Design: docs/features/interfaces.md -- bridge STP over rtnetlink.
// Related: bridge_linux.go -- BridgeSetSTP, bridgeSTPRequest.

package ifacenetlink

import (
	"errors"
	"syscall"
	"testing"

	"github.com/vishvananda/netlink/nl"
	"golang.org/x/sys/unix"
)

// bridgeLinkInfo decodes the attributes that follow a struct ifinfomsg in an
// RTM_NEWLINK message: it returns IFLA_INFO_KIND and the IFLA_INFO_DATA
// attributes inside IFLA_LINKINFO. The unit test reads the request Ze builds
// with it, and the integration test reads the kernel's answer with it, so both
// sides of the change are judged by one decoder.
func bridgeLinkInfo(attrs []byte) (string, []syscall.NetlinkRouteAttr, error) {
	top, err := nl.ParseRouteAttr(attrs)
	if err != nil {
		return "", nil, err
	}
	for _, attr := range top {
		if attr.Attr.Type != unix.IFLA_LINKINFO {
			continue
		}
		info, err := nl.ParseRouteAttr(attr.Value)
		if err != nil {
			return "", nil, err
		}
		var kind string
		var data []syscall.NetlinkRouteAttr
		for _, entry := range info {
			switch entry.Attr.Type {
			case nl.IFLA_INFO_KIND:
				kind = string(entry.Value)
			case nl.IFLA_INFO_DATA:
				data, err = nl.ParseRouteAttr(entry.Value)
				if err != nil {
					return "", nil, err
				}
			}
		}
		return kind, data, nil
	}
	return "", nil, errors.New("no IFLA_LINKINFO attribute")
}

// stpStateFrom returns the IFLA_BR_STP_STATE value among bridge data
// attributes, or an error when the attribute is absent or not a u32.
func stpStateFrom(data []syscall.NetlinkRouteAttr) (uint32, error) {
	for _, attr := range data {
		if attr.Attr.Type != unix.IFLA_BR_STP_STATE {
			continue
		}
		if len(attr.Value) != 4 {
			return 0, errors.New("IFLA_BR_STP_STATE is not a u32")
		}
		return nl.NativeEndian().Uint32(attr.Value), nil
	}
	return 0, errors.New("no IFLA_BR_STP_STATE attribute")
}

// TestBridgeSTPRequest checks the rtnetlink request BridgeSetSTP sends.
// VALIDATES: an RTM_NEWLINK change addressed by ifindex, acknowledged, never
// creating, whose IFLA_LINKINFO names kind "bridge" and carries exactly one
// IFLA_INFO_DATA attribute, IFLA_BR_STP_STATE as a u32 of 1 or 0.
// PREVENTS: STP going back through the read-only /sys of a container, a
// request that creates a missing bridge, and a kind the kernel's changelink
// lookup refuses. Method: serialize the request and decode it byte by byte.
func TestBridgeSTPRequest(t *testing.T) {
	for _, tc := range []struct {
		name    string
		enabled bool
		want    uint32
	}{
		{"on", true, 1},
		{"off", false, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			const index = 42
			wire := bridgeSTPRequest(index, tc.enabled).Serialize()
			if len(wire) < unix.SizeofNlMsghdr+unix.SizeofIfInfomsg {
				t.Fatalf("request is %d bytes, shorter than its headers", len(wire))
			}
			native := nl.NativeEndian()
			if got := native.Uint32(wire[0:4]); int(got) != len(wire) {
				t.Errorf("nlmsg_len %d, request is %d bytes", got, len(wire))
			}
			if got := native.Uint16(wire[4:6]); got != unix.RTM_NEWLINK {
				t.Errorf("nlmsg_type %d, want RTM_NEWLINK %d", got, unix.RTM_NEWLINK)
			}
			flags := native.Uint16(wire[6:8])
			if flags != unix.NLM_F_REQUEST|unix.NLM_F_ACK {
				t.Errorf("nlmsg_flags %#x, want NLM_F_REQUEST|NLM_F_ACK %#x (no create, no replace)",
					flags, unix.NLM_F_REQUEST|unix.NLM_F_ACK)
			}
			ifinfo := wire[unix.SizeofNlMsghdr : unix.SizeofNlMsghdr+unix.SizeofIfInfomsg]
			if ifinfo[0] != unix.AF_UNSPEC {
				t.Errorf("ifi_family %d, want AF_UNSPEC", ifinfo[0])
			}
			if got := native.Uint32(ifinfo[4:8]); got != index {
				t.Errorf("ifi_index %d, want %d", got, index)
			}
			kind, data, err := bridgeLinkInfo(wire[unix.SizeofNlMsghdr+unix.SizeofIfInfomsg:])
			if err != nil {
				t.Fatalf("decode IFLA_LINKINFO: %v", err)
			}
			if kind != linkTypeBridge {
				t.Errorf("IFLA_INFO_KIND %q, want %q", kind, linkTypeBridge)
			}
			if len(data) != 1 {
				t.Fatalf("IFLA_INFO_DATA carries %d attributes, want only IFLA_BR_STP_STATE: %v", len(data), data)
			}
			got, err := stpStateFrom(data)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Errorf("IFLA_BR_STP_STATE %d, want %d", got, tc.want)
			}
		})
	}
}
