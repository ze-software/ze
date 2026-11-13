// Design: docs/architecture/rib/unified-locrib.md -- complete selected forwarding groups.
// Related: sysrib_forwarding_capture_fixture.go -- authenticated current/replay observations.

package fixture

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/component/config"
	sysribevents "github.com/ze-software/ze/internal/component/sysrib/events"
	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const (
	sysribNonwinnerTarget = "198.18.235.0/24"
	sysribNonwinnerStatic = "198.18.236.0/24"
	sysribNonwinnerOSPF   = "198.18.237.0/24"
	sysribNonwinnerLinkA  = "zesrnwa"
	sysribNonwinnerLinkB  = "zesrnwb"
	sysribNonwinnerLinkC  = "zesrnwc"
	// The connected subnets of the three fixture links: OS-owned routes that
	// stay foreign-owned controls Ze must never sweep.
	sysribNonwinnerConnectedA = "198.18.232.0/24"
	sysribNonwinnerConnectedB = "198.18.233.0/24"
	sysribNonwinnerConnectedC = "198.18.234.0/24"
	// sysribNonwinnerSecondary is static's secondary gateway; the reload
	// renames it to sysribNonwinnerSecondaryUpdated.
	sysribNonwinnerSecondary        = "198.18.233.1"
	sysribNonwinnerSecondaryUpdated = "198.18.233.2"
)

// sysribNonwinnerECMPDriver inserts the independent registered OSPF route
// before configuring static's explicit group. This exercises first-seen
// selection without presenting the SDK producer as a wire-protocol peer.
func sysribNonwinnerECMPDriver(ctx context.Context, args []string) error {
	if err := importPlacementArgs(args, 1); err != nil {
		return err
	}
	capture := newSysribFIBCapture(sysribNonwinnerTarget, sysribNonwinnerStatic, sysribNonwinnerOSPF,
		sysribNonwinnerConnectedA, sysribNonwinnerConnectedB, sysribNonwinnerConnectedC)
	return observeConfigured(ctx, "sysrib-nonwinner-ecmp-test", &sdk.Registration{}, capture.setup, func(ctx context.Context, p *sdk.Plugin) error {
		if err := sysribExecutionPlacement(ctx, p, args[0]); err != nil {
			return err
		}
		for _, link := range []struct{ name, address string }{
			{sysribNonwinnerLinkA, "198.18.232.254/24"},
			{sysribNonwinnerLinkB, "198.18.233.254/24"},
			{sysribNonwinnerLinkC, "198.18.234.254/24"},
		} {
			if err := importCreateLink(ctx, link.name, link.address); err != nil {
				return err
			}
		}
		// The paired carrier joins daemon shutdown before its dedicated
		// cleanup driver removes these devices. Failed runs retain their
		// namespace owner's cleanup rather than deleting live FIB targets.
		if err := sysribNonwinnerCovers(ctx, p); err != nil {
			return err
		}
		for _, prefix := range []string{sysribNonwinnerTarget, sysribNonwinnerOSPF} {
			if err := sysribInstall(ctx, p, rpc.RouteInstallEntry{Protocol: protocolOSPF, AFI: 1, SAFI: 1,
				Prefix: prefix, NextHop: "198.18.234.1", Interface: sysribNonwinnerLinkC,
				OnLink: true, Weight: 3, Metric: 10, RouteType: 1}); err != nil {
				return err
			}
		}
		// Both source controls and the chosen primary must be useful before
		// the judged static group exists. Later changes never reannounce OSPF.
		if err := sysribNonwinnerControls(ctx, p, true); err != nil {
			return err
		}
		if err := importCandidateState(ctx, p, sysribNonwinnerTarget, protocolOSPF, 20, nil, true, "", ""); err != nil {
			return err
		}
		if err := capture.await(ctx, sysribNonwinnerTarget, 0, sysribNonwinnerWant(false, false)); err != nil {
			return err
		}
		if err := importKernel(ctx, sysribNonwinnerTarget, 1, "via 198.18.234.1 ", "dev "+sysribNonwinnerLinkC+" ", "proto 250 "); err != nil {
			return err
		}
		tree, schema, err := importReadConfig()
		if err != nil {
			return err
		}
		static := tree.GetContainer("static")
		if static == nil {
			return errors.New("nonwinner fixture omitted static control")
		}
		table := static.GetList("table")["default"]
		if table == nil {
			return errors.New("nonwinner fixture omitted default static table")
		}
		// Parse the operator grammar, rather than constructing Path.ECMP.
		target, err := config.NewParser(schema).Parse(`static { table default { route 198.18.235.0/24 { distance 20; metric 10; next { hop 198.18.232.1 { weight 2; } hop 198.18.233.1 { weight 5; } } } } }`)
		if err != nil {
			return err
		}
		route := target.GetContainer("static").GetList("table")["default"].GetList("route")[sysribNonwinnerTarget]
		if route == nil {
			return errors.New("parsed nonwinner static target is absent")
		}
		table.AddListEntry("route", sysribNonwinnerTarget, route)
		if err := policyFinalReload(ctx, p, config.Serialize(tree, schema), "applied"); err != nil {
			return err
		}
		if err := sysribNonwinnerState(ctx, p, capture, false, false, 0); err != nil {
			return err
		}
		// Only static's SECONDARY gateway and share change. Its primary and
		// every OSPF input stay byte-for-byte unchanged.
		next := route.GetContainer("next")
		if next == nil {
			return errors.New("nonwinner static target omitted next")
		}
		secondary := next.GetList("hop")[sysribNonwinnerSecondary]
		if secondary == nil {
			return errors.New("nonwinner static target omitted secondary hop")
		}
		secondary.Set("weight", "7")
		if err := next.RenameListEntry("hop", sysribNonwinnerSecondary, sysribNonwinnerSecondaryUpdated); err != nil {
			return err
		}
		if err := policyFinalReload(ctx, p, config.Serialize(tree, schema), "applied"); err != nil {
			return err
		}
		if err := sysribNonwinnerState(ctx, p, capture, true, false, 0); err != nil {
			return err
		}
		rib := tree.GetContainer("rib")
		if rib == nil {
			return errors.New("nonwinner fixture omitted rib distances")
		}
		rib.SetSlice("fib-withhold", []string{protocolStatic})
		if err := policyFinalReload(ctx, p, config.Serialize(tree, schema), "applied"); err != nil {
			return err
		}
		if err := sysribNonwinnerState(ctx, p, capture, true, true, 0); err != nil {
			return err
		}
		rib.SetSlice("fib-withhold", nil)
		if err := policyFinalReload(ctx, p, config.Serialize(tree, schema), "applied"); err != nil {
			return err
		}
		if err := sysribNonwinnerState(ctx, p, capture, true, false, 0); err != nil {
			return err
		}
		capture.mu.Lock()
		replayAt := capture.replayCount + 1
		capture.mu.Unlock()
		if _, err := requireDone02(ctx, p, "request plugin restart fib-kernel"); err != nil {
			return err
		}
		if err := sysribNonwinnerState(ctx, p, capture, true, false, replayAt); err != nil {
			return err
		}
		return importSay("nonwinner configured static ECMP retains both weighted members through secondary-only reload, withholding, repermit and fresh FIB replay beside the unchanged OSPF primary and OS controls")
	})
}

func sysribNonwinnerWant(updated, permitted bool) sysribevents.BestChangeEntry {
	want := sysribevents.BestChangeEntry{Prefix: netip.MustParsePrefix(sysribNonwinnerTarget), Protocol: protocolOSPF,
		NextHop: netip.MustParseAddr("198.18.234.1"), Interface: sysribNonwinnerLinkC, OnLink: true,
		Weight: 3, Metric: 10, RouteType: 1}
	if permitted {
		secondary, weight := sysribNonwinnerSecondary, uint8(5)
		if updated {
			secondary, weight = sysribNonwinnerSecondaryUpdated, 7
		}
		want.ECMPPaths = []sysribevents.ECMPPath{
			{NextHop: netip.MustParseAddr("198.18.232.1"), Interface: sysribNonwinnerLinkA, Weight: 2},
			{NextHop: netip.MustParseAddr(secondary), Interface: sysribNonwinnerLinkB, Weight: weight},
		}
	}
	return want
}

func sysribNonwinnerState(ctx context.Context, p *sdk.Plugin, capture *sysribFIBCapture, updated, withheld bool, replayAt uint64) error {
	if err := importCandidateState(ctx, p, sysribNonwinnerTarget, protocolOSPF, 20, nil, true, "", ""); err != nil {
		return err
	}
	if err := importCandidateState(ctx, p, sysribNonwinnerTarget, protocolStatic, 20, nil, false, "", ""); err != nil {
		return err
	}
	want := sysribNonwinnerWant(updated, !withheld)
	if replayAt != 0 {
		want.Action = routeaction.Add
	}
	if err := capture.await(ctx, sysribNonwinnerTarget, replayAt, want); err != nil {
		return err
	}
	if withheld {
		if err := importKernel(ctx, sysribNonwinnerTarget, 1, "via 198.18.234.1 ", "dev "+sysribNonwinnerLinkC+" ", "proto 250 "); err != nil {
			return err
		}
	} else {
		secondary, weight := sysribNonwinnerSecondary, "5"
		if updated {
			secondary, weight = sysribNonwinnerSecondaryUpdated, "7"
		}
		if err := sysribKernelMultipath(ctx, sysribNonwinnerTarget,
			"nexthop via 198.18.234.1 dev "+sysribNonwinnerLinkC+" weight 3 ",
			"nexthop via 198.18.232.1 dev "+sysribNonwinnerLinkA+" weight 2 ",
			"nexthop via "+secondary+" dev "+sysribNonwinnerLinkB+" weight "+weight+" "); err != nil {
			return err
		}
	}
	if err := sysribNonwinnerKernelMembers(ctx, updated, withheld); err != nil {
		return err
	}
	return sysribNonwinnerControls(ctx, p, !withheld)
}

// The OS connected routes remain useful foreign-owned controls while static
// is withheld; they must never be swept by fib-kernel's subscriber restart.
func sysribNonwinnerCovers(ctx context.Context, p *sdk.Plugin) error {
	for _, cover := range []struct{ prefix, link string }{
		{sysribNonwinnerConnectedA, sysribNonwinnerLinkA},
		{sysribNonwinnerConnectedB, sysribNonwinnerLinkB},
		{sysribNonwinnerConnectedC, sysribNonwinnerLinkC},
	} {
		if err := waitRIBWinner(ctx, p, cover.prefix, "connected"); err != nil {
			return err
		}
		if err := importKernel(ctx, cover.prefix, 1, "dev "+cover.link+" ", "proto 2 "); err != nil {
			return err
		}
	}
	return nil
}

func sysribNonwinnerControls(ctx context.Context, p *sdk.Plugin, staticPermitted bool) error {
	if err := importCandidateState(ctx, p, sysribNonwinnerStatic, protocolStatic, 20, nil, true, "", ""); err != nil {
		return err
	}
	count := 0
	if staticPermitted {
		count = 1
	}
	if err := importKernel(ctx, sysribNonwinnerStatic, count, "6 "+sysribNonwinnerStatic+" ", "proto 250 "); err != nil {
		return err
	}
	if err := importCandidateState(ctx, p, sysribNonwinnerOSPF, protocolOSPF, 20, nil, true, "", ""); err != nil {
		return err
	}
	if err := importKernel(ctx, sysribNonwinnerOSPF, 1, "via 198.18.234.1 ", "dev "+sysribNonwinnerLinkC+" ", "proto 250 "); err != nil {
		return err
	}
	return sysribNonwinnerCovers(ctx, p)
}

func sysribNonwinnerKernelMembers(ctx context.Context, updated, withheld bool) error {
	var last string
	var readErr error
	if Poll(ctx, 100, 100*time.Millisecond, func() bool {
		last, readErr = netfilterCommandOutput(ctx, "ip", "-N", "-o", "route", "show", "table", "all", "exact", sysribNonwinnerTarget)
		if readErr != nil {
			return false
		}
		if withheld {
			if strings.Contains(last, "198.18.232.1") {
				return false
			}
			return !strings.Contains(last, "198.18.233.")
		}
		if strings.Count(last, "nexthop ") != 3 {
			return false
		}
		if updated {
			return !strings.Contains(last, sysribNonwinnerSecondary)
		}
		return !strings.Contains(last, sysribNonwinnerSecondaryUpdated)
	}) {
		return nil
	}
	if readErr != nil {
		return fmt.Errorf("nonwinner kernel membership updated=%t withheld=%t: %q; read error %w", updated, withheld, last, readErr)
	}
	return fmt.Errorf("nonwinner kernel membership updated=%t withheld=%t: %q; read error <nil>", updated, withheld, last)
}

func sysribNonwinnerECMPKernelEmpty(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("nonwinner ECMP kernel-empty fixture takes no arguments")
	}
	for _, prefix := range []string{sysribNonwinnerTarget, sysribNonwinnerStatic, sysribNonwinnerOSPF,
		sysribNonwinnerConnectedA, sysribNonwinnerConnectedB, sysribNonwinnerConnectedC} {
		if err := importKernel(ctx, prefix, 0); err != nil {
			return err
		}
	}
	return importSay("nonwinner ECMP fixture prefixes are absent from every kernel table")
}

// This runs only after daemon/await-exit. Ze must have flushed its own
// prefixes while the actual devices and OS-owned routes still exist; deleting
// a device first could erase a stale FIB entry and make that absence vacuous.
func sysribNonwinnerECMPLinksCleanup(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("nonwinner ECMP post-stop link cleanup takes no arguments")
	}
	for _, prefix := range []string{sysribNonwinnerTarget, sysribNonwinnerStatic, sysribNonwinnerOSPF} {
		if err := importKernel(ctx, prefix, 0); err != nil {
			return err
		}
	}
	links := []struct{ prefix, name string }{
		{sysribNonwinnerConnectedA, sysribNonwinnerLinkA},
		{sysribNonwinnerConnectedB, sysribNonwinnerLinkB},
		{sysribNonwinnerConnectedC, sysribNonwinnerLinkC},
	}
	for _, link := range links {
		if err := importKernel(ctx, link.prefix, 1, "dev "+link.name+" ", "proto 2 "); err != nil {
			return err
		}
		if err := importCommandRun(ctx, "link", "show", "dev", link.name); err != nil {
			return fmt.Errorf("joined daemon stop removed fixture device %s: %w", link.name, err)
		}
	}
	for _, link := range links {
		if err := importDeleteLink(ctx, link.name); err != nil {
			return err
		}
	}
	return importSay("joined daemon flushed Ze routes while all three fixture devices and OS routes survived before owned link cleanup")
}
