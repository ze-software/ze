// Design: docs/architecture/static-routes.md -- VPP data-plane static backend selection

//go:build linux && ze_vpp

package static

import (
	"fmt"

	"go.fd.io/govpp/api"

	"github.com/ze-software/ze/internal/component/iface"
	vppcomp "github.com/ze-software/ze/internal/component/vpp"
	staticvpp "github.com/ze-software/ze/internal/plugins/static/vpp"
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
// An interface-only next-hop (no gateway address) is resolved to a VPP
// sw_if_index through the SAME shared iface.Resolve the netlink backend uses:
// the VPP iface backend publishes its sw_if_index through iface.Resolve's
// Binding.Ifindex (query.go detailsToInfo -> resolve.go bindingFromInfo), so no
// second resolver is needed (spec-fixit-static-interface-nexthops C-3, A-3a).
//
// Resolution is gated on the active iface backend being vpp (C-4/R-7):
// iface.Resolve reports whatever backend is loaded, and a netlink backend would
// return a KERNEL ifindex, which must never be programmed as a VPP sw_if_index.
// An unresolved or invalid index is rejected rather than emitting index 0
// (the wrong-path trap the original rejection guarded).
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
		if nh.Address.IsValid() {
			out.Paths = append(out.Paths, staticvpp.Path{
				NextHop: nh.Address,
				Weight:  capWeight(nh.Weight),
			})
			continue
		}

		idx, err := resolveVPPSwIfIndex(nh.Interface)
		if err != nil {
			return staticvpp.Route{}, err
		}
		out.Paths = append(out.Paths, staticvpp.Path{
			SwIfIndex: idx,
			Weight:    capWeight(nh.Weight),
		})
	}
	return out, nil
}

// resolveVPPSwIfIndex maps an interface-only next-hop's logical name to a VPP
// sw_if_index via the shared iface resolver, gated on the active iface backend
// being vpp so a kernel ifindex can never be programmed as a VPP index (R-7).
// It never returns a zero index without an error: index 0 is VPP's local0 and
// programming it for an unresolved name would silently install a wrong path.
func resolveVPPSwIfIndex(name string) (uint32, error) {
	if backend := iface.ActiveBackendName(); backend != "vpp" {
		if backend == "" {
			return 0, fmt.Errorf("static/vpp: interface-only next-hop %q needs the vpp iface backend, but no iface backend is loaded", name)
		}
		return 0, fmt.Errorf("static/vpp: interface-only next-hop %q needs the vpp iface backend, but the active iface backend is %q (a kernel ifindex must not be programmed as a VPP sw_if_index)", name, backend)
	}
	binding, err := iface.Resolve(name)
	if err != nil {
		return 0, fmt.Errorf("static/vpp: interface-only next-hop %q: %w", name, err)
	}
	if binding.Ifindex <= 0 {
		return 0, fmt.Errorf("static/vpp: interface-only next-hop %q resolved to invalid sw_if_index %d", name, binding.Ifindex)
	}
	return uint32(binding.Ifindex), nil
}

// capWeight narrows the parent's uint16 ECMP weight to VPP's uint8 (max 255).
// Zero is preserved; the static/vpp translator coerces 0 to 1.
func capWeight(w uint16) uint8 {
	if w > 255 {
		return 255
	}
	return uint8(w)
}
