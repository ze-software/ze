// Design: plan/learned/650-static-routes.md -- VPP data-plane static backend selection

//go:build linux

package static

import (
	"fmt"

	"go.fd.io/govpp/api"

	vppcomp "codeberg.org/thomas-mangin/ze/internal/component/vpp"
	staticvpp "codeberg.org/thomas-mangin/ze/internal/plugins/static/vpp"
)

// vppStaticBackend programs static routes into the VPP data plane through the
// static/vpp GoVPP backend. It is selected by newStaticBackend when the VPP
// component is active (a connector is available), so that in a VPP data-plane
// deployment static routes land in VPP's FIB rather than the kernel.
//
// The per-route table id is honored by constructing a lightweight
// staticvpp.Backend per operation over the shared channel (NewBackend only
// stores the channel + table id; the channel is closed once in close()).
type vppStaticBackend struct {
	ch api.Channel
}

// newVPPStaticBackend returns a VPP-backed routeBackend when a VPP connector is
// active and a channel can be opened, or nil to signal "fall back to kernel".
func newVPPStaticBackend() routeBackend {
	connector := vppcomp.GetActiveConnector()
	if connector == nil {
		return nil
	}
	ch, err := connector.NewChannel()
	if err != nil {
		logger().Warn("static: VPP connector present but channel open failed, using kernel backend", "error", err)
		return nil
	}
	logger().Info("static: VPP data plane active, programming routes into VPP")
	return &vppStaticBackend{ch: ch}
}

func (v *vppStaticBackend) applyRoute(r staticRoute) error {
	route, err := toVPPRoute(r)
	if err != nil {
		return err
	}
	return staticvpp.NewBackend(v.ch, r.Table).ApplyRoute(route)
}

func (v *vppStaticBackend) removeRoute(r staticRoute) error {
	return staticvpp.NewBackend(v.ch, r.Table).RemoveRoute(r.Prefix)
}

func (v *vppStaticBackend) listRoutes() ([]installedStaticRoute, error) {
	// VPP FIB enumeration is not wired; the RIB/config is authoritative for the
	// installed set, so listing is a no-op here (kernel backend owns reconcile).
	return nil, nil
}

func (v *vppStaticBackend) close() error {
	if v.ch != nil {
		v.ch.Close()
	}
	return nil
}

// toVPPRoute translates a parent staticRoute into the static/vpp Route type.
// Interface-only next-hops are rejected: mapping a logical interface name to a
// VPP sw_if_index is not yet wired, so failing loudly beats programming a wrong
// (index-0) path silently.
func toVPPRoute(r staticRoute) (staticvpp.Route, error) {
	out := staticvpp.Route{
		Prefix: r.Prefix,
		Metric: r.Metric,
	}
	switch r.Action {
	case actionForward:
		out.Action = staticvpp.ActionForward
	case actionBlackhole:
		out.Action = staticvpp.ActionBlackhole
		return out, nil
	case actionReject:
		out.Action = staticvpp.ActionReject
		return out, nil
	default:
		return staticvpp.Route{}, fmt.Errorf("static/vpp: unknown action %d", r.Action)
	}

	for i := range r.NextHops {
		nh := r.NextHops[i]
		if !nh.Address.IsValid() {
			return staticvpp.Route{}, fmt.Errorf("static/vpp: interface-only next-hop %q needs a VPP sw_if_index (not yet supported)", nh.Interface)
		}
		out.Paths = append(out.Paths, staticvpp.Path{
			NextHop: nh.Address,
			Weight:  capWeight(nh.Weight),
		})
	}
	return out, nil
}

// capWeight narrows the parent's uint16 ECMP weight to VPP's uint8 (max 255).
// Zero is preserved; the static/vpp translator coerces 0 to 1.
func capWeight(w uint16) uint8 {
	if w > 255 {
		return 255
	}
	return uint8(w)
}
