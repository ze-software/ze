package ifacevpp

import (
	"fmt"
	"time"

	"go.fd.io/govpp/api"
	"go.fd.io/govpp/binapi/gre"
	"go.fd.io/govpp/binapi/interface_types"
	"go.fd.io/govpp/binapi/ipip"
	"go.fd.io/govpp/binapi/lcp"
	"go.fd.io/govpp/binapi/span"
	"go.fd.io/govpp/binapi/vxlan"
	"go.fd.io/govpp/binapi/wireguard"
)

// progChannel is a programmable api.Channel fake shared by the tunnel,
// VXLAN, SPAN, WireGuard, and LCP tests. It records every request and fills
// each add/del reply's Retval (and SwIfIndex / PeerIndex where the reply
// carries one) from the configured fields, so a test can assert both the
// request the backend built and the backend's handling of the VPP return
// value.
type progChannel struct {
	requests  []api.Message
	retval    int32
	swIfIndex interface_types.InterfaceIndex
	peerIndex uint32
	sendErr   error

	// Dump responses delivered by SendMultiRequest, used by GetWireguardDevice
	// round-trip tests. Each slice is streamed one entry per ReceiveReply,
	// then a final (last=true) terminates the dump.
	wgIfaceDetails []wireguard.WireguardInterfaceDetails
	wgPeerDetails  []wireguard.WireguardPeersDetails
}

var _ api.Channel = (*progChannel)(nil)

func (c *progChannel) SendRequest(msg api.Message) api.RequestCtx {
	c.requests = append(c.requests, msg)
	return &progReqCtx{ch: c}
}

func (c *progChannel) SendMultiRequest(msg api.Message) api.MultiRequestCtx {
	c.requests = append(c.requests, msg)
	switch msg.(type) {
	case *wireguard.WireguardInterfaceDump:
		ctx := &progMultiCtx{}
		for i := range c.wgIfaceDetails {
			d := c.wgIfaceDetails[i]
			ctx.details = append(ctx.details, &d)
		}
		return ctx
	case *wireguard.WireguardPeersDump:
		ctx := &progMultiCtx{}
		for i := range c.wgPeerDetails {
			d := c.wgPeerDetails[i]
			ctx.details = append(ctx.details, &d)
		}
		return ctx
	default:
		return &progMultiCtx{}
	}
}

func (c *progChannel) SubscribeNotification(_ chan api.Message, _ api.Message) (api.SubscriptionCtx, error) {
	return nil, fmt.Errorf("SubscribeNotification not implemented")
}

func (c *progChannel) SetReplyTimeout(time.Duration)          {}
func (c *progChannel) CheckCompatiblity(...api.Message) error { return nil }
func (c *progChannel) Close()                                 {}

type progReqCtx struct{ ch *progChannel }

func (r *progReqCtx) ReceiveReply(msg api.Message) error {
	if r.ch.sendErr != nil {
		return r.ch.sendErr
	}
	switch reply := msg.(type) {
	case *gre.GreTunnelAddDelReply:
		reply.Retval = r.ch.retval
		reply.SwIfIndex = r.ch.swIfIndex
	case *ipip.IpipAddTunnelReply:
		reply.Retval = r.ch.retval
		reply.SwIfIndex = r.ch.swIfIndex
	case *ipip.IpipDelTunnelReply:
		reply.Retval = r.ch.retval
	case *vxlan.VxlanAddDelTunnelV3Reply:
		reply.Retval = r.ch.retval
		reply.SwIfIndex = r.ch.swIfIndex
	case *span.SwInterfaceSpanEnableDisableReply:
		reply.Retval = r.ch.retval
	case *wireguard.WireguardInterfaceCreateReply:
		reply.Retval = r.ch.retval
		reply.SwIfIndex = r.ch.swIfIndex
	case *wireguard.WireguardInterfaceDeleteReply:
		reply.Retval = r.ch.retval
	case *wireguard.WireguardPeerAddReply:
		reply.Retval = r.ch.retval
		reply.PeerIndex = r.ch.peerIndex
	case *wireguard.WireguardPeerRemoveReply:
		reply.Retval = r.ch.retval
	case *lcp.LcpItfPairAddDelReply:
		reply.Retval = r.ch.retval
	}
	return nil
}

// progMultiCtx streams the recorded dump details one per ReceiveReply. When the
// details are exhausted it returns last=true, matching GoVPP multi-request
// semantics. An empty details slice terminates immediately (an empty dump).
type progMultiCtx struct {
	details []api.Message
	idx     int
}

func (m *progMultiCtx) ReceiveReply(msg api.Message) (bool, error) {
	if m.idx >= len(m.details) {
		return true, nil
	}
	switch dst := msg.(type) {
	case *wireguard.WireguardInterfaceDetails:
		if src, ok := m.details[m.idx].(*wireguard.WireguardInterfaceDetails); ok {
			*dst = *src
		}
	case *wireguard.WireguardPeersDetails:
		if src, ok := m.details[m.idx].(*wireguard.WireguardPeersDetails); ok {
			*dst = *src
		}
	}
	m.idx++
	return false, nil
}
