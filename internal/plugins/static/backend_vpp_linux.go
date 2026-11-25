// Design: docs/architecture/static-routes.md -- VPP data-plane static backend selection

//go:build linux && ze_vpp

package static

import (
	"errors"
	"fmt"

	"go.fd.io/govpp/api"

	"github.com/ze-software/ze/internal/component/iface"
	vppcomp "github.com/ze-software/ze/internal/component/vpp"
	staticvpp "github.com/ze-software/ze/internal/plugins/static/vpp"
)

// vppStaticBackend programs named-table static routes into the VPP data plane
// through the static/vpp GoVPP backend. It is selected by
// newStaticBackendForDataPlane when the configured data plane is "vpp".
//
// The per-route table id is honored by constructing a lightweight
// staticvpp.Backend per operation over the shared channel (NewBackend only
// stores the channel + table id; the channel is closed once in close()).
type vppStaticBackend struct {
	ch api.Channel
}

// vppChannelOpener opens a GoVPP API channel for named-table static routes.
// Replaced only in tests to inject a live or failing VPP channel without
// opening real VPP sockets.
var vppChannelOpener = openActiveVPPChannel

var errNoActiveVPPConnector = errors.New("no active VPP connector")

func openActiveVPPChannel() (api.Channel, error) {
	connector := vppcomp.GetActiveConnector()
	if connector == nil {
		return nil, errNoActiveVPPConnector
	}
	ch, err := connector.NewChannel()
	if err != nil {
		return nil, err
	}
	return ch, nil
}

// newVPPStaticBackend returns a VPP-backed routeBackend when a GoVPP API
// channel can be opened. When no VPP connector is active or opening the
// channel fails, it returns an *opererr.Error naming doctorCodeVPPChannel and
// NEVER falls back to the Linux kernel backend.
func newVPPStaticBackend() (routeBackend, error) {
	ch, err := vppChannelOpener()
	if err != nil {
		return nil, staticVPPChannelError(err)
	}
	logger().Info("static: VPP data plane active, programming named-table routes into VPP")
	return &vppStaticBackend{ch: ch}, nil
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
// sw_if_index through the SAME shared iface.Resolve the kernel backend uses:
// the VPP iface backend publishes its sw_if_index through iface.Resolve's
// Binding.Ifindex (query.go detailsToInfo -> resolve.go bindingFromInfo), so no
// second resolver is needed (spec-fixit-static-interface-nexthops C-3, A-3a).
//
// Resolution is gated on the active iface backend being vpp (C-4/R-7):
// iface.Resolve reports whatever backend is loaded, and a kernel backend would
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
		panic("BUG: static: invalid route action")
	}

	for i := range r.NextHops {
		nh := r.NextHops[i]
		if nh.Address.IsValid() {
			// A next hop named by address alone leaves by whichever interface
			// VPP resolves it to, so the path names none (NoInterface, never 0).
			out.Paths = append(out.Paths, staticvpp.Path{
				NextHop:   nh.Address,
				Weight:    capWeight(nh.Weight),
				SwIfIndex: staticvpp.NoInterface,
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
// sw_if_index. The gate on the active iface backend, and the refusal to answer
// zero, live in iface.ResolveVPPIndex, which the VPP FIB plugin calls for the
// same job on the main-table path (R-7).
func resolveVPPSwIfIndex(name string) (uint32, error) {
	idx, err := iface.ResolveVPPIndex(name)
	if err != nil {
		return 0, fmt.Errorf("static/vpp: interface-only next-hop: %w", err)
	}
	return idx, nil
}

// capWeight narrows the parent's uint16 ECMP weight to VPP's uint8 (max 255).
// Zero is preserved; the static/vpp translator coerces 0 to 1.
func capWeight(w uint16) uint8 {
	if w > 255 {
		return 255
	}
	return uint8(w)
}
