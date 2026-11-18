// Design: docs/features/interfaces.md -- original PPP guarded request serialization
// Related: native_ppp_linux.go -- actual positive owner admission and file borrow

//go:build linux

package ifacenetlink

import (
	"context"
	"encoding/binary"
	"errors"
	"net/netip"
	"strings"

	"github.com/vishvananda/netlink/nl"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/rtproto"
)

const (
	nativePPPLinkOwnerAttribute    = 70
	nativePPPAddressOwnerAttribute = 13
	nativePPPRouteOwnerAttribute   = 33
)

func (w *nativePPPWriter) SetMTU(ctx context.Context, mtu int) error {
	if err := validateMTU(mtu); err != nil {
		return err
	}
	return w.mutate(ctx, nativePPPSetlink, func(index int) (*nl.NetlinkRequest, error) {
		return nativePPPLinkRequest(index, 0, 0, nl.NewRtAttr(unix.IFLA_MTU, nl.Uint32Attr(uint32(mtu)))), nil
	})
}

func (w *nativePPPWriter) SetAdminUp(ctx context.Context) error {
	return w.mutate(ctx, nativePPPSetlink, func(index int) (*nl.NetlinkRequest, error) {
		return nativePPPLinkRequest(index, unix.IFF_UP, unix.IFF_UP, nil), nil
	})
}

func (w *nativePPPWriter) SetAdminDown(ctx context.Context) error {
	return w.mutate(ctx, nativePPPSetlink, func(index int) (*nl.NetlinkRequest, error) {
		return nativePPPLinkRequest(index, 0, unix.IFF_UP, nil), nil
	})
}

func (w *nativePPPWriter) SetAlias(ctx context.Context, alias string) error {
	if strings.IndexByte(alias, 0) >= 0 {
		return errors.New("PPP original alias contains a NUL byte")
	}
	if len(alias) >= 256 {
		return errors.New("PPP original alias exceeds the native alias limit")
	}
	return w.mutate(ctx, nativePPPAlias, func(index int) (*nl.NetlinkRequest, error) {
		return nativePPPLinkRequest(index, 0, 0, nl.NewRtAttr(unix.IFLA_IFALIAS, nl.ZeroTerminated(alias))), nil
	})
}

func nativePPPLinkRequest(index int, flags, change uint32, attribute *nl.RtAttr) *nl.NetlinkRequest {
	request := nl.NewNetlinkRequest(unix.RTM_SETLINK, unix.NLM_F_ACK)
	header := nl.NewIfInfomsg(unix.AF_UNSPEC)
	header.Index = int32(index)
	header.Flags = flags
	header.Change = change
	request.AddData(header)
	if attribute != nil {
		request.AddData(attribute)
	}
	request.AddData(nl.NewRtAttr(nativePPPLinkOwnerAttribute, nl.Uint32Attr(0)))
	return request
}

func (w *nativePPPWriter) AddAddressP2P(ctx context.Context, local, peer netip.Prefix) error {
	feature, err := nativePPPAddressFeature(local, peer)
	if err != nil {
		return err
	}
	return w.mutate(ctx, feature, func(index int) (*nl.NetlinkRequest, error) {
		return nativePPPAddressRequest(unix.RTM_NEWADDR, index, local, peer), nil
	})
}

// RemoveAddressP2P requires the kernel's exact tuple and IPv4 isolation feature.
// That writer validates local/peer/mask under its native writer lock and keeps
// same-subnet secondaries without changing a name-based sysctl. A family bit
// alone never authorizes this operation on an older carried kernel.
func (w *nativePPPWriter) RemoveAddressP2P(ctx context.Context, local, peer netip.Prefix) error {
	feature, err := nativePPPAddressFeature(local, peer)
	if err != nil {
		return err
	}
	return w.mutate(ctx, feature|nativePPPExactAddressPair, func(index int) (*nl.NetlinkRequest, error) {
		return nativePPPAddressRequest(unix.RTM_DELADDR, index, local, peer), nil
	})
}

func nativePPPAddressFeature(local, peer netip.Prefix) (uint32, error) {
	if !local.IsValid() {
		return 0, errors.New("PPP original local address prefix is invalid")
	}
	if !peer.IsValid() {
		return 0, errors.New("PPP original peer address prefix is invalid")
	}
	if local.Addr().Is4In6() {
		return 0, errors.New("PPP original local address has an IPv4-mapped mask")
	}
	if peer.Addr().Is4In6() {
		return 0, errors.New("PPP original peer address has an IPv4-mapped mask")
	}
	if local.Addr().Is4() != peer.Addr().Is4() {
		return 0, errors.New("PPP original address pair has different families")
	}
	if local.Addr().Is4() {
		return nativePPPAddr4, nil
	}
	return nativePPPAddr6, nil
}

func nativePPPAddressRequest(kind, index int, local, peer netip.Prefix) *nl.NetlinkRequest {
	flags := unix.NLM_F_ACK
	if kind == unix.RTM_NEWADDR {
		flags |= unix.NLM_F_CREATE | unix.NLM_F_EXCL
	}
	request := nl.NewNetlinkRequest(kind, flags)
	family := unix.AF_INET6
	if local.Addr().Is4() {
		family = unix.AF_INET
	}
	header := nl.NewIfAddrmsg(family)
	header.Index = uint32(index)
	// Existing netlink AddrAdd(Peer != nil) uses the peer's mask in ifaddrmsg.
	header.Prefixlen = uint8(peer.Bits())
	request.AddData(header)
	request.AddData(nl.NewRtAttr(unix.IFA_LOCAL, local.Addr().AsSlice()))
	request.AddData(nl.NewRtAttr(unix.IFA_ADDRESS, peer.Addr().AsSlice()))
	if family == unix.AF_INET {
		if peer.Bits() < 31 {
			// Preserve the existing AddrAdd producer's computed broadcast,
			// even for a P2P pair with a non-host peer prefix.
			address := local.Addr().As4()
			var broadcast [4]byte
			hosts := ^uint32(0) >> uint(peer.Bits())
			binary.BigEndian.PutUint32(broadcast[:], binary.BigEndian.Uint32(address[:])|hosts)
			request.AddData(nl.NewRtAttr(unix.IFA_BROADCAST, broadcast[:]))
		}
	}
	request.AddData(nl.NewRtAttr(nativePPPAddressOwnerAttribute, nl.Uint32Attr(0)))
	return request
}

func (w *nativePPPWriter) AddRoute(ctx context.Context, destination netip.Prefix, gateway netip.Addr, metric int, protocol rtproto.Proto) error {
	feature, err := nativePPPRouteFeature(destination, gateway, metric, protocol)
	if err != nil {
		return err
	}
	return w.mutate(ctx, feature, func(index int) (*nl.NetlinkRequest, error) {
		return nativePPPRouteRequest(unix.RTM_NEWROUTE, index, destination, gateway, metric, protocol), nil
	})
}

func (w *nativePPPWriter) RemoveRoute(ctx context.Context, destination netip.Prefix, gateway netip.Addr, metric int, protocol rtproto.Proto) error {
	feature, err := nativePPPRouteFeature(destination, gateway, metric, protocol)
	if err != nil {
		return err
	}
	return w.mutate(ctx, feature, func(index int) (*nl.NetlinkRequest, error) {
		return nativePPPRouteRequest(unix.RTM_DELROUTE, index, destination, gateway, metric, protocol), nil
	})
}

func nativePPPRouteFeature(destination netip.Prefix, gateway netip.Addr, metric int, protocol rtproto.Proto) (uint32, error) {
	if !destination.IsValid() {
		return 0, errors.New("PPP original route destination is invalid")
	}
	if destination.Addr().Is4In6() {
		return 0, errors.New("PPP original route destination has an IPv4-mapped mask")
	}
	if !gateway.IsValid() {
		return 0, errors.New("PPP original route gateway is invalid")
	}
	if gateway.Zone() != "" {
		return 0, errors.New("PPP original route gateway has an unsupported zone")
	}
	gateway = gateway.Unmap()
	if gateway.IsUnspecified() {
		return 0, errors.New("PPP original route gateway is unspecified")
	}
	if destination.Addr().Is4() != gateway.Is4() {
		return 0, errors.New("PPP original route destination and gateway have different families")
	}
	if metric < 0 {
		return 0, errors.New("PPP original route metric is negative")
	}
	if uint64(metric) > uint64(^uint32(0)) {
		return 0, errors.New("PPP original route metric exceeds native width")
	}
	if protocol <= rtproto.Any {
		return 0, errors.New("PPP original route protocol names no producer")
	}
	if uint64(protocol) > uint64(^uint8(0)) {
		return 0, errors.New("PPP original route protocol exceeds native width")
	}
	if destination.Addr().Is4() {
		return nativePPPRoute4, nil
	}
	return nativePPPRoute6, nil
}

func nativePPPRouteRequest(kind, index int, destination netip.Prefix, gateway netip.Addr, metric int, protocol rtproto.Proto) *nl.NetlinkRequest {
	flags := unix.NLM_F_ACK
	header := nl.NewRtDelMsg()
	if kind == unix.RTM_NEWROUTE {
		// This is RouteReplace, not RouteAdd: a matching owned route may be
		// replaced, and the kernel validates every affected old/new owner leg.
		flags |= unix.NLM_F_CREATE | unix.NLM_F_REPLACE
		header = nl.NewRtMsg()
	}
	header.Family = unix.AF_INET6
	if destination.Addr().Is4() {
		header.Family = unix.AF_INET
	}
	header.Protocol = uint8(protocol)
	header.Dst_len = uint8(destination.Bits())
	request := nl.NewNetlinkRequest(kind, flags)
	request.AddData(header)
	request.AddData(nl.NewRtAttr(unix.RTA_DST, destination.Masked().Addr().AsSlice()))
	request.AddData(nl.NewRtAttr(unix.RTA_GATEWAY, gateway.Unmap().AsSlice()))
	request.AddData(nl.NewRtAttr(unix.RTA_OIF, nl.Uint32Attr(uint32(index))))
	if metric > 0 {
		request.AddData(nl.NewRtAttr(unix.RTA_PRIORITY, nl.Uint32Attr(uint32(metric))))
	}
	request.AddData(nl.NewRtAttr(nativePPPRouteOwnerAttribute, nl.Uint32Attr(0)))
	return request
}
