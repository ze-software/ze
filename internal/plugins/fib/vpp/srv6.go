// Design: plan/learned/1113-fib-depth-4-srv6.md -- VPP SRv6 SR steer programming
// Related: mpls.go -- MPLS backend pattern (model for SRv6)
// Related: fibvpp.go -- processEvent dispatches to SRv6 when SRv6SID present

package fibvpp

import (
	"fmt"
	"net/netip"

	"codeberg.org/thomas-mangin/ze/internal/core/bgp/routeaction"

	"go.fd.io/govpp/api"
	"go.fd.io/govpp/binapi/ip_types"
	"go.fd.io/govpp/binapi/sr"
	"go.fd.io/govpp/binapi/sr_types"
)

// srv6Backend extends fibVPP with SRv6 SR steering operations.
type srv6Backend interface {
	addSRv6Steer(prefix netip.Prefix, sid netip.Addr, tableID uint32) error
	delSRv6Steer(prefix netip.Prefix, tableID uint32) error
}

// processSRv6Change handles a single best-change entry with SRv6 SID.
// Caller must hold f.mu.
func (f *fibVPP) processSRv6Change(c *incomingChange) {
	if f.srv6Backend == nil {
		logger().Warn("fib-vpp: SRv6 change but no SRv6 backend configured", "prefix", c.Prefix)
		return
	}
	pfxStr := c.Prefix.String()
	switch c.Action.Verb() { //nolint:exhaustive // Unspecified is a no-op for SRv6
	case routeaction.VerbInstall, routeaction.VerbReplace:
		if err := f.srv6Backend.addSRv6Steer(c.Prefix, c.SRv6SID, c.TableID); err != nil {
			logger().Error("fib-vpp: SRv6 steer failed", "prefix", c.Prefix, "sid", c.SRv6SID, "error", err)
			return
		}
		f.srv6Installed[pfxStr] = true
		if m := fibVPPMetricsPtr.Load(); m != nil {
			m.routeInstalls.Inc()
		}
	case routeaction.VerbRemove:
		if err := f.srv6Backend.delSRv6Steer(c.Prefix, c.TableID); err != nil {
			logger().Error("fib-vpp: SRv6 steer del failed", "prefix", c.Prefix, "error", err)
			return
		}
		delete(f.srv6Installed, pfxStr)
		if m := fibVPPMetricsPtr.Load(); m != nil {
			m.routeRemovals.Inc()
		}
	}
}

// govppSRv6Backend implements srv6Backend using GoVPP SR binary API.
type govppSRv6Backend struct {
	ch      api.Channel
	tableID uint32
}

func newGovppSRv6Backend(ch api.Channel, tableID uint32) *govppSRv6Backend {
	return &govppSRv6Backend{ch: ch, tableID: tableID}
}

// addSRv6Steer creates an SR steering policy for traffic matching prefix
// to be encapsulated toward the given SRv6 SID (used as BSID).
func (b *govppSRv6Backend) addSRv6Steer(prefix netip.Prefix, sid netip.Addr, tableID uint32) error {
	tbl := b.tableID
	if tableID != 0 {
		tbl = tableID
	}
	req := &sr.SrSteeringAddDel{
		IsDel:       false,
		BsidAddr:    toIP6Address(sid),
		TableID:     tbl,
		Prefix:      toVPPPrefix(prefix),
		TrafficType: steerTypeForPrefix(prefix),
	}
	reply := &sr.SrSteeringAddDelReply{}
	if err := b.ch.SendRequest(req).ReceiveReply(reply); err != nil {
		return fmt.Errorf("sr SrSteeringAddDel: %w", err)
	}
	if reply.Retval != 0 {
		return fmt.Errorf("sr SrSteeringAddDel retval=%d", reply.Retval)
	}
	return nil
}

// delSRv6Steer removes an SR steering policy for the given prefix.
func (b *govppSRv6Backend) delSRv6Steer(prefix netip.Prefix, tableID uint32) error {
	tbl := b.tableID
	if tableID != 0 {
		tbl = tableID
	}
	req := &sr.SrSteeringAddDel{
		IsDel:       true,
		TableID:     tbl,
		Prefix:      toVPPPrefix(prefix),
		TrafficType: steerTypeForPrefix(prefix),
	}
	reply := &sr.SrSteeringAddDelReply{}
	if err := b.ch.SendRequest(req).ReceiveReply(reply); err != nil {
		return fmt.Errorf("sr del SrSteeringAddDel: %w", err)
	}
	if reply.Retval != 0 {
		return fmt.Errorf("sr del SrSteeringAddDel retval=%d", reply.Retval)
	}
	return nil
}

func steerTypeForPrefix(prefix netip.Prefix) sr_types.SrSteer {
	if prefix.Addr().Is6() {
		return sr_types.SR_STEER_API_IPV6
	}
	return sr_types.SR_STEER_API_IPV4
}

func toIP6Address(addr netip.Addr) ip_types.IP6Address {
	a16 := addr.As16()
	var ip6 ip_types.IP6Address
	copy(ip6[:], a16[:])
	return ip6
}
