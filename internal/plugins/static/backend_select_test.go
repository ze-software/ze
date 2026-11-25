// VALIDATES: AC-7 (G) -- static's named-table route backend follows the
// configured primary data plane resolved from the candidate configuration,
// never VPP connector presence; under vpp a failed channel open returns an
// opererr.Error naming doctor-static-vpp-channel and never falls back to the
// kernel; main-table routes stay on the Loc-RIB path on both planes.
// PREVENTS: named-table static routes silently landing in the Linux kernel
// under interface backend vpp, or landing in VPP under interface backend kernel.

package static

import (
	"errors"
	"net/netip"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/internal/core/routingtable"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

const (
	testRoutingTableLNS  = `{"routing-table":{"table":{"lns":{"id":"100"}}}}`
	testStaticNamedOnly  = `{"static":{"table":{"lns":{"route":{"10.10.0.0/24":{"next":{"hop":{"192.0.2.1":{}}}}}}}}}`
	testStaticMainOnly   = `{"static":{"table":{"default":{"route":{"10.20.0.0/24":{"next":{"hop":{"192.0.2.2":{}}}}}}}}}`
	testStaticBothTables = `{"static":{"table":{` +
		`"default":{"route":{"10.20.0.0/24":{"next":{"hop":{"192.0.2.2":{}}}}}},` +
		`"lns":{"route":{"10.10.0.0/24":{"next":{"hop":{"192.0.2.1":{}}}}}}}}}`
	testIfaceKernel = `{"interface":{"backend":"kernel"}}`
	testIfaceVPP    = `{"interface":{"backend":"vpp"}}`
)

// recordingPlaneBackend captures named-table route writes and closes for one
// data plane so tests can assert which plane received each route.
type recordingPlaneBackend struct {
	plane   string
	applied []staticRoute
	removed []staticRoute
	closed  bool
}

func (b *recordingPlaneBackend) applyRoute(r staticRoute) error {
	b.applied = append(b.applied, r)
	return nil
}

func (b *recordingPlaneBackend) removeRoute(r staticRoute) error {
	b.removed = append(b.removed, r)
	return nil
}

func (b *recordingPlaneBackend) listRoutes() ([]installedStaticRoute, error) {
	return nil, nil
}

func (b *recordingPlaneBackend) close() error {
	b.closed = true
	return nil
}

func TestStaticBackendFollowsConfiguredDataPlane(t *testing.T) {
	t.Run("kernel explicit and default select kernel backend", func(t *testing.T) {
		for _, tc := range []struct {
			name     string
			sections []sdk.ConfigSection
		}{
			{
				name: "explicit kernel",
				sections: []sdk.ConfigSection{
					{Root: pluginName, Data: testStaticNamedOnly},
					{Root: routingtable.ConfigRoot, Data: testRoutingTableLNS},
					{Root: iface.ConfigRoot, Data: testIfaceKernel},
				},
			},
			{
				name: "omitted interface uses schema default kernel",
				sections: []sdk.ConfigSection{
					{Root: pluginName, Data: testStaticNamedOnly},
					{Root: routingtable.ConfigRoot, Data: testRoutingTableLNS},
				},
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				routes, plane, found, err := verifyCandidateStaticWithDataPlane(tc.sections)
				if err != nil {
					t.Fatalf("verifyCandidateStaticWithDataPlane: %v", err)
				}
				if !found {
					t.Fatal("expected static section found")
				}
				if plane != dataPlaneKernel {
					t.Fatalf("resolved data plane = %q, want %q", plane, dataPlaneKernel)
				}

				var opened []string
				kernelBE := &recordingPlaneBackend{plane: dataPlaneKernel}
				vppBE := &recordingPlaneBackend{plane: dataPlaneVPP}
				rm := newRouteManager(nil)
				rm.setBackendOpener(func(p string) (routeBackend, error) {
					opened = append(opened, p)
					switch p {
					case dataPlaneKernel:
						return kernelBE, nil
					case dataPlaneVPP:
						return vppBE, nil
					default:
						return nil, unrecognizedDataPlaneError(p)
					}
				})

				if err := rm.applyRoutesForDataPlane(routes, plane); err != nil {
					t.Fatalf("applyRoutesForDataPlane: %v", err)
				}
				if !slices.Equal(opened, []string{dataPlaneKernel}) {
					t.Fatalf("opened planes = %v, want [%s]", opened, dataPlaneKernel)
				}
				if len(kernelBE.applied) != 1 || kernelBE.applied[0].Table != 100 {
					t.Fatalf("kernel applied = %+v, want 1 route in table 100", kernelBE.applied)
				}
				if len(vppBE.applied) != 0 {
					t.Fatalf("vpp applied = %+v, want 0", vppBE.applied)
				}
			})
		}
	})

	t.Run("vpp selects vpp backend and failed channel refuses kernel fallback", func(t *testing.T) {
		sections := []sdk.ConfigSection{
			{Root: pluginName, Data: testStaticBothTables},
			{Root: routingtable.ConfigRoot, Data: testRoutingTableLNS},
			{Root: iface.ConfigRoot, Data: testIfaceVPP},
		}
		routes, plane, found, err := verifyCandidateStaticWithDataPlane(sections)
		if err != nil {
			t.Fatalf("verifyCandidateStaticWithDataPlane: %v", err)
		}
		if !found {
			t.Fatal("expected static section found")
		}
		if plane != dataPlaneVPP {
			t.Fatalf("resolved data plane = %q, want %q", plane, dataPlaneVPP)
		}

		loc := locrib.NewRIB()
		var opened []string
		kernelBE := &recordingPlaneBackend{plane: dataPlaneKernel}
		vppBE := &recordingPlaneBackend{plane: dataPlaneVPP}
		channelAvailable := false
		channelCause := errors.New("vpp: not connected")

		rm := newRouteManager(nil)
		rm.setLocRIB(loc, nil)
		rm.setBackendOpener(func(p string) (routeBackend, error) {
			opened = append(opened, p)
			switch p {
			case dataPlaneKernel:
				return kernelBE, nil
			case dataPlaneVPP:
				if !channelAvailable {
					return nil, staticVPPChannelError(channelCause)
				}
				return vppBE, nil
			default:
				return nil, unrecognizedDataPlaneError(p)
			}
		})

		if err := rm.applyRoutesForDataPlane(routes, plane); err != nil {
			t.Fatalf("applyRoutesForDataPlane: %v", err)
		}
		if !slices.Equal(opened, []string{dataPlaneVPP}) {
			t.Fatalf("opened planes = %v, want [%s]", opened, dataPlaneVPP)
		}
		if len(kernelBE.applied) != 0 {
			t.Fatalf("kernel backend received %d named-table routes under vpp, want 0", len(kernelBE.applied))
		}

		// Named-table route MUST be skipped with doctor-static-vpp-channel.
		skipped := rm.skippedRoutes()
		if len(skipped) != 1 {
			t.Fatalf("skipped routes = %d, want 1", len(skipped))
		}
		if skipped[0].route.Table != 100 {
			t.Errorf("skipped table = %d, want 100", skipped[0].route.Table)
		}
		if !strings.Contains(skipped[0].reason, doctorCodeVPPChannel) {
			t.Errorf("skip reason = %q, want it to name %q", skipped[0].reason, doctorCodeVPPChannel)
		}

		// Main-table route MUST still land in Loc-RIB unaffected by the VPP channel failure.
		mainPfx := netip.MustParsePrefix("10.20.0.0/24")
		if _, ok := loc.Best(familyOf(mainPfx), mainPfx); !ok {
			t.Fatal("main-table route missing from Loc-RIB while VPP channel failed")
		}

		// Once the VPP channel becomes available, re-applying installs the
		// named-table route into VPP and clears the skip.
		channelAvailable = true
		if err := rm.applyRoutesForDataPlane(routes, plane); err != nil {
			t.Fatalf("retry applyRoutesForDataPlane: %v", err)
		}
		if len(rm.skippedRoutes()) != 0 {
			t.Fatalf("skipped routes after VPP channel recovery = %+v, want none", rm.skippedRoutes())
		}
		if len(vppBE.applied) != 1 || vppBE.applied[0].Table != 100 {
			t.Fatalf("vpp applied = %+v, want 1 route in table 100", vppBE.applied)
		}
		if len(kernelBE.applied) != 0 {
			t.Fatalf("kernel applied = %+v, want 0", kernelBE.applied)
		}
	})

	t.Run("main-table only routes do not open named-table backend on either plane", func(t *testing.T) {
		for _, tc := range []struct {
			plane     string
			ifaceData string
		}{
			{plane: dataPlaneKernel, ifaceData: testIfaceKernel},
			{plane: dataPlaneVPP, ifaceData: testIfaceVPP},
		} {
			t.Run(tc.plane, func(t *testing.T) {
				sections := []sdk.ConfigSection{
					{Root: pluginName, Data: testStaticMainOnly},
					{Root: iface.ConfigRoot, Data: tc.ifaceData},
				}
				routes, plane, found, err := verifyCandidateStaticWithDataPlane(sections)
				if err != nil {
					t.Fatalf("verifyCandidateStaticWithDataPlane: %v", err)
				}
				if !found || plane != tc.plane {
					t.Fatalf("found=%v plane=%q, want true %q", found, plane, tc.plane)
				}

				loc := locrib.NewRIB()
				openCalls := 0
				rm := newRouteManager(nil)
				rm.setLocRIB(loc, nil)
				rm.setBackendOpener(func(string) (routeBackend, error) {
					openCalls++
					return nil, errors.New("unexpected backend open for main-table route")
				})

				if err := rm.applyRoutesForDataPlane(routes, plane); err != nil {
					t.Fatalf("applyRoutesForDataPlane: %v", err)
				}
				if openCalls != 0 {
					t.Fatalf("openBackend called %d times for main-table-only routes, want 0", openCalls)
				}
				mainPfx := netip.MustParsePrefix("10.20.0.0/24")
				if _, ok := loc.Best(familyOf(mainPfx), mainPfx); !ok {
					t.Fatalf("main-table route missing from Loc-RIB under %s", tc.plane)
				}
			})
		}
	})

	t.Run("reload switching data plane withdraws old backend and rollback restores it", func(t *testing.T) {
		kernelBE := &recordingPlaneBackend{plane: dataPlaneKernel}
		vppBE := &recordingPlaneBackend{plane: dataPlaneVPP}
		loc := locrib.NewRIB()
		rm := newRouteManager(nil)
		rm.setLocRIB(loc, nil)
		rm.setBackendOpener(func(p string) (routeBackend, error) {
			switch p {
			case dataPlaneKernel:
				return kernelBE, nil
			case dataPlaneVPP:
				return vppBE, nil
			default:
				return nil, unrecognizedDataPlaneError(p)
			}
		})

		var mu sync.Mutex
		var currentRoutes []staticRoute
		var currentPlane string
		var pending pendingSection
		tx := staticTx{
			pending:      &pending,
			rm:           rm,
			mu:           &mu,
			current:      &currentRoutes,
			currentPlane: &currentPlane,
		}

		// Commit 1: kernel plane with main + named-table route.
		routes1, plane1, _, err := verifyCandidateStaticWithDataPlane([]sdk.ConfigSection{
			{Root: pluginName, Data: testStaticBothTables},
			{Root: routingtable.ConfigRoot, Data: testRoutingTableLNS},
			{Root: iface.ConfigRoot, Data: testIfaceKernel},
		})
		if err != nil {
			t.Fatalf("verify kernel: %v", err)
		}
		pending.setWithDataPlane(routes1, plane1)
		if err := tx.apply(); err != nil {
			t.Fatalf("apply kernel: %v", err)
		}
		if len(kernelBE.applied) != 1 || len(vppBE.applied) != 0 {
			t.Fatalf("after kernel apply: kernel=%d vpp=%d, want 1, 0", len(kernelBE.applied), len(vppBE.applied))
		}

		// Commit 2: switch to vpp plane, then roll back.
		routes2, plane2, _, err := verifyCandidateStaticWithDataPlane([]sdk.ConfigSection{
			{Root: pluginName, Data: testStaticBothTables},
			{Root: routingtable.ConfigRoot, Data: testRoutingTableLNS},
			{Root: iface.ConfigRoot, Data: testIfaceVPP},
		})
		if err != nil {
			t.Fatalf("verify vpp: %v", err)
		}
		pending.setWithDataPlane(routes2, plane2)
		if err := tx.apply(); err != nil {
			t.Fatalf("apply vpp: %v", err)
		}
		if len(kernelBE.removed) != 1 || !kernelBE.closed {
			t.Fatalf("switch to vpp: kernel removed=%d closed=%v, want 1, true", len(kernelBE.removed), kernelBE.closed)
		}
		if len(vppBE.applied) != 1 {
			t.Fatalf("switch to vpp: vpp applied=%d, want 1", len(vppBE.applied))
		}

		if err := tx.rollback(); err != nil {
			t.Fatalf("rollback to kernel: %v", err)
		}
		if len(vppBE.removed) != 1 || !vppBE.closed {
			t.Fatalf("rollback: vpp removed=%d closed=%v, want 1, true", len(vppBE.removed), vppBE.closed)
		}
		if len(kernelBE.applied) != 2 {
			t.Fatalf("rollback: kernel applied=%d, want 2 (initial + restored)", len(kernelBE.applied))
		}
		if currentPlane != dataPlaneKernel {
			t.Fatalf("currentPlane after rollback = %q, want %q", currentPlane, dataPlaneKernel)
		}
		mainPfx := netip.MustParsePrefix("10.20.0.0/24")
		if _, ok := loc.Best(familyOf(mainPfx), mainPfx); !ok {
			t.Fatal("main-table route missing from Loc-RIB after plane switch and rollback")
		}
	})

	t.Run("real constructor refuses vpp without connector and fails closed on unknown plane", func(t *testing.T) {
		be, err := newStaticBackendForDataPlane(dataPlaneVPP)
		if err == nil {
			_ = be.close()
			t.Fatal("newStaticBackendForDataPlane(vpp) succeeded without active VPP connector, want error")
		}
		if be != nil {
			_ = be.close()
			t.Fatalf("newStaticBackendForDataPlane(vpp) returned non-nil backend %T on error", be)
		}
		opErr, ok := errors.AsType[*opererr.Error](err)
		if !ok {
			t.Fatalf("error type = %T, want *opererr.Error", err)
		}
		if !strings.Contains(opErr.Error(), doctorCodeVPPChannel) {
			t.Errorf("error %q does not name doctor code %q", opErr.Error(), doctorCodeVPPChannel)
		}

		for _, bad := range []string{"", "unknown-plane"} {
			got, badErr := newStaticBackendForDataPlane(bad)
			if badErr == nil {
				if got != nil {
					_ = got.close()
				}
				t.Fatalf("newStaticBackendForDataPlane(%q) = nil error, want refusal", bad)
			}
			if _, isOper := errors.AsType[*opererr.Error](badErr); !isOper {
				t.Errorf("newStaticBackendForDataPlane(%q) error type = %T, want *opererr.Error", bad, badErr)
			}
		}
	})

	t.Run("doctor code is registered as diagnostic and event code", func(t *testing.T) {
		meta := diagnostic.Lookup(doctorCodeVPPChannel)
		if meta == nil {
			t.Fatalf("diagnostic.Lookup(%q) = nil, want registered", doctorCodeVPPChannel)
		}
		if meta.Title == "" || meta.Description == "" || len(meta.Examples) == 0 {
			t.Fatalf("diagnostic metadata incomplete: %+v", meta)
		}
		if !slices.Contains(diagnostic.EventCodes(), doctorCodeVPPChannel) {
			t.Fatalf("diagnostic.EventCodes() does not contain %q", doctorCodeVPPChannel)
		}
	})
}
