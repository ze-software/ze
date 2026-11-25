// VALIDATES: AC-7 (G) under ze_vpp -- with a live VPP channel opener present,
// data plane "kernel" still selects the Linux kernel backend without touching
// VPP; data plane "vpp" selects vppStaticBackend and programs named-table
// routes into GoVPP; a failing VPP channel opener returns an *opererr.Error
// naming doctor-static-vpp-channel and never falls back to the kernel backend.
// PREVENTS: VPP connector presence overriding configured interface backend, or
// a VPP channel failure silently writing named-table routes to the Linux kernel.

//go:build linux && ze_vpp

package static

import (
	"errors"
	"net/netip"
	"strings"
	"testing"

	"go.fd.io/govpp/api"
	"go.fd.io/govpp/binapi/ip"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/internal/core/routingtable"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

func TestStaticVPPBackendFollowsConfiguredDataPlaneWithLiveConnector(t *testing.T) {
	prevOpener := vppChannelOpener
	t.Cleanup(func() { vppChannelOpener = prevOpener })

	t.Run("kernel ignores live VPP connector", func(t *testing.T) {
		vppOpenCalls := 0
		vppChannelOpener = func() (api.Channel, error) {
			vppOpenCalls++
			return &fakeChannel{}, nil
		}

		be, err := newStaticBackendForDataPlane(dataPlaneKernel)
		if err != nil {
			t.Fatalf("newStaticBackendForDataPlane(kernel): %v", err)
		}
		t.Cleanup(func() { _ = be.close() })
		if vppOpenCalls != 0 {
			t.Fatalf("vppChannelOpener called %d times under kernel, want 0", vppOpenCalls)
		}
		if _, ok := be.(*netlinkStaticBackend); !ok {
			t.Fatalf("backend type = %T, want *netlinkStaticBackend", be)
		}
	})

	t.Run("vpp opens GoVPP channel and programs named-table route into VPP", func(t *testing.T) {
		ch := &fakeChannel{}
		vppChannelOpener = func() (api.Channel, error) {
			return ch, nil
		}

		routes, plane, found, err := verifyCandidateStaticWithDataPlane([]sdk.ConfigSection{
			{Root: pluginName, Data: testStaticNamedOnly},
			{Root: routingtable.ConfigRoot, Data: testRoutingTableLNS},
			{Root: iface.ConfigRoot, Data: testIfaceVPP},
		})
		if err != nil {
			t.Fatalf("verifyCandidateStaticWithDataPlane: %v", err)
		}
		if !found || plane != dataPlaneVPP {
			t.Fatalf("found=%v plane=%q, want true %q", found, plane, dataPlaneVPP)
		}

		rm := newRouteManager(nil)
		rm.setBackendOpener(newStaticBackendForDataPlane)
		t.Cleanup(func() { _ = rm.closeBackend() })

		if err := rm.applyRoutesForDataPlane(routes, plane); err != nil {
			t.Fatalf("applyRoutesForDataPlane: %v", err)
		}
		if len(rm.skippedRoutes()) != 0 {
			t.Fatalf("skipped routes = %+v, want none", rm.skippedRoutes())
		}
		req, ok := ch.lastRequest.(*ip.IPRouteAddDel)
		if !ok {
			t.Fatalf("VPP channel lastRequest = %T, want *ip.IPRouteAddDel", ch.lastRequest)
		}
		if !req.IsAdd || req.Route.TableID != 100 {
			t.Fatalf("VPP request = IsAdd:%v TableID:%d, want true, 100", req.IsAdd, req.Route.TableID)
		}
	})

	t.Run("vpp channel failure returns doctor code error and never falls back to kernel", func(t *testing.T) {
		channelErr := errors.New("govpp channel open refused")
		vppChannelOpener = func() (api.Channel, error) {
			return nil, channelErr
		}

		be, err := newStaticBackendForDataPlane(dataPlaneVPP)
		if err == nil {
			_ = be.close()
			t.Fatal("newStaticBackendForDataPlane(vpp) succeeded despite failing channel opener")
		}
		if be != nil {
			_ = be.close()
			t.Fatalf("backend = %T, want nil (must never fall back to kernel)", be)
		}
		opErr, ok := errors.AsType[*opererr.Error](err)
		if !ok {
			t.Fatalf("error type = %T, want *opererr.Error", err)
		}
		if !strings.Contains(opErr.Error(), doctorCodeVPPChannel) {
			t.Errorf("error %q does not name %q", opErr.Error(), doctorCodeVPPChannel)
		}
		if !errors.Is(err, channelErr) {
			t.Errorf("error %v does not wrap channel failure %v", err, channelErr)
		}

		// End-to-end through routeManager: main-table route still installs in
		// Loc-RIB, named-table route is skipped with doctor-static-vpp-channel,
		// and no kernel backend is ever constructed.
		routes, plane, _, err := verifyCandidateStaticWithDataPlane([]sdk.ConfigSection{
			{Root: pluginName, Data: testStaticBothTables},
			{Root: routingtable.ConfigRoot, Data: testRoutingTableLNS},
			{Root: iface.ConfigRoot, Data: testIfaceVPP},
		})
		if err != nil {
			t.Fatalf("verifyCandidateStaticWithDataPlane: %v", err)
		}
		loc := locrib.NewRIB()
		rm := newRouteManager(nil)
		rm.setLocRIB(loc, nil)
		rm.setBackendOpener(newStaticBackendForDataPlane)
		t.Cleanup(func() { _ = rm.closeBackend() })

		if err := rm.applyRoutesForDataPlane(routes, plane); err != nil {
			t.Fatalf("applyRoutesForDataPlane: %v", err)
		}
		mainPfx := netip.MustParsePrefix("10.20.0.0/24")
		if _, ok := loc.Best(familyOf(mainPfx), mainPfx); !ok {
			t.Fatal("main-table route missing from Loc-RIB under VPP channel failure")
		}
		skipped := rm.skippedRoutes()
		if len(skipped) != 1 || skipped[0].route.Table != 100 || !strings.Contains(skipped[0].reason, doctorCodeVPPChannel) {
			t.Fatalf("skipped = %+v, want 1 table-100 route citing %q", skipped, doctorCodeVPPChannel)
		}
		rm.mu.Lock()
		gotBE := rm.backend
		rm.mu.Unlock()
		if gotBE != nil {
			t.Fatalf("rm.backend = %T, want nil (no kernel fallback)", gotBE)
		}
	})
}
