// Design: docs/architecture/rib/unified-locrib.md -- complete forwarding targets and replay.
// Related: sysrib_forwarding_capture_fixture.go -- owned live/replay observations.

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
	sysribECMPLinkA    = "zesrma"
	sysribECMPLinkB    = "zesrmb"
	sysribECMPPlain    = "198.18.249.0/24"
	sysribECMPDevice   = "198.18.250.0/24"
	sysribECMPLabels   = "198.18.251.0/24"
	sysribECMPSID      = "198.18.252.0/24"
	sysribECMPBackup   = "198.18.253.0/24"
	sysribECMPWithheld = "198.18.254.0/24"
)

func sysribECMPDriver(ctx context.Context, args []string) error {
	if err := importPlacementArgs(args, 1); err != nil {
		return err
	}
	capture := newSysribFIBCapture(sysribECMPPlain, sysribECMPDevice, sysribECMPLabels, sysribECMPSID, sysribECMPBackup, sysribECMPWithheld, "198.18.247.0/24", "198.18.248.0/24")
	return observeConfigured(ctx, "sysrib-ecmp-test", &sdk.Registration{}, capture.setup, func(ctx context.Context, p *sdk.Plugin) (result error) {
		if err := sysribExecutionPlacement(ctx, p, args[0]); err != nil {
			return err
		}
		if err := importCreateLink(ctx, sysribECMPLinkA, "198.18.247.254/24", "2001:db8:247::fe/64"); err != nil {
			return err
		}
		if err := importCreateLink(ctx, sysribECMPLinkB, "198.18.248.254/24", "2001:db8:248::fe/64"); err != nil {
			return err
		}
		// The paired carrier joins daemon shutdown before its dedicated
		// cleanup driver removes these devices. Failed runs retain their
		// namespace owner's cleanup rather than deleting live FIB targets.
		for _, prefix := range []string{"198.18.247.0/24", "198.18.248.0/24", "2001:db8:247::/64", "2001:db8:248::/64"} {
			if err := waitRIBWinner(ctx, p, prefix, "connected"); err != nil {
				return err
			}
		}
		coverA := rpc.RouteInstallEntry{Protocol: "ospf", AFI: 1, SAFI: 1, Prefix: "10.249.1.0/24", NextHop: "198.18.247.1", Interface: sysribECMPLinkA, OnLink: true, Metric: 3, RouteType: 1}
		coverB := rpc.RouteInstallEntry{Protocol: "ospf", AFI: 1, SAFI: 1, Prefix: "10.249.2.0/24", NextHop: "198.18.248.1", Interface: sysribECMPLinkB, OnLink: true, Metric: 5, RouteType: 1}
		sidCover := rpc.RouteInstallEntry{Protocol: "ospf", AFI: 2, SAFI: 1, Prefix: "2001:db8:249::/64", NextHop: "2001:db8:248::1", Interface: sysribECMPLinkB, OnLink: true, Metric: 5, RouteType: 1}
		if err := sysribInstall(ctx, p, coverA, coverB, sidCover); err != nil {
			return err
		}
		for _, prefix := range []string{sysribECMPPlain, sysribECMPLabels, sysribECMPSID} {
			a, b := sysribECMPPath(prefix, true), sysribECMPPath(prefix, false)
			if err := sysribInstall(ctx, p, a, b); err != nil {
				return err
			}
		}
		device := rpc.RouteInstallEntry{Protocol: "static", AFI: 1, SAFI: 1, Prefix: sysribECMPDevice, Interface: sysribECMPLinkA, Weight: 3, Metric: 7, RouteType: 1,
			ECMP: []rpc.RouteNextHop{{Interface: sysribECMPLinkB, Weight: 7}}}
		backup := sysribECMPPath(sysribECMPBackup, true)
		backup.BackupNextHop = "198.18.248.1"
		backup.BackupInterface = sysribECMPLinkB
		backup.BackupOnLink = true
		backup.BackupRepairLabels = []uint32{17001, 17002}
		withheld := rpc.RouteInstallEntry{Protocol: "bgp", AFI: 1, SAFI: 1, Prefix: sysribECMPWithheld, NextHop: "198.18.247.1", Interface: sysribECMPLinkA, OnLink: true, IsBGP: true, IsEBGP: true, Metric: 7, RouteType: 1}
		if err := sysribInstall(ctx, p, device, backup, withheld); err != nil {
			return err
		}
		if err := sysribECMPState(ctx, p, capture, false, 0); err != nil {
			return err
		}
		if err := waitRIBWinner(ctx, p, sysribECMPWithheld, "bgp"); err != nil {
			return err
		}
		if err := importKernel(ctx, sysribECMPWithheld, 0); err != nil {
			return err
		}
		// Only a cover changes. No selected path, label, SID, device member or
		// repair is announced again during loss, promotion and recovery.
		if err := sysribRemove(ctx, p, coverA); err != nil {
			return err
		}
		if err := sysribECMPState(ctx, p, capture, true, 0); err != nil {
			return err
		}
		if err := sysribRemove(ctx, p, sidCover); err != nil {
			return err
		}
		if err := capture.await(ctx, sysribECMPSID, 0, sysribevents.BestChangeEntry{Action: routeaction.Withdraw, Prefix: netip.MustParsePrefix(sysribECMPSID)}); err != nil {
			return err
		}
		if err := importKernel(ctx, sysribECMPSID, 0); err != nil {
			return err
		}
		if err := sysribInstall(ctx, p, sidCover); err != nil {
			return err
		}
		if err := sysribECMPState(ctx, p, capture, true, 0); err != nil {
			return err
		}
		if err := sysribInstall(ctx, p, coverA); err != nil {
			return err
		}
		if err := sysribECMPState(ctx, p, capture, false, 0); err != nil {
			return err
		}
		// Reload only permission, leaving every producer and selection alive.
		tree, schema, err := importReadConfig()
		if err != nil {
			return err
		}
		rib := tree.GetContainer("rib")
		if rib == nil {
			return errors.New("ECMP fixture omitted rib")
		}
		rib.SetSlice("fib-withhold", nil)
		if err := policyFinalReload(ctx, p, config.Serialize(tree, schema), "applied"); err != nil {
			return err
		}
		if err := importKernel(ctx, sysribECMPWithheld, 1, "via 198.18.247.1 ", "proto 250 "); err != nil {
			return err
		}
		rib.SetSlice("fib-withhold", []string{"bgp"})
		if err := policyFinalReload(ctx, p, config.Serialize(tree, schema), "applied"); err != nil {
			return err
		}
		if err := waitRIBWinner(ctx, p, sysribECMPWithheld, "bgp"); err != nil {
			return err
		}
		if err := importKernel(ctx, sysribECMPWithheld, 0); err != nil {
			return err
		}
		// A real FIB restart creates a fresh subscriber after all transitions.
		// Its replay must contain current targets, never withheld or OS entries.
		capture.mu.Lock()
		replayAt := capture.replayCount + 1
		capture.mu.Unlock()
		if _, err := requireDone02(ctx, p, "request plugin restart fib-kernel"); err != nil {
			return err
		}
		if err := sysribECMPState(ctx, p, capture, false, replayAt); err != nil {
			return err
		}
		capture.mu.Lock()
		for _, prefix := range []string{sysribECMPWithheld, "198.18.247.0/24", "198.18.248.0/24"} {
			if capture.replayedAt[prefix] >= replayAt {
				result = fmt.Errorf("fresh FIB replay programmed withheld/OS-owned prefix %s", prefix)
				break
			}
		}
		capture.mu.Unlock()
		if result != nil {
			return result
		}
		if err := importKernel(ctx, sysribECMPWithheld, 0); err != nil {
			return err
		}
		return importSay("weighted and device-only ECMP, promoted labels and SID, SID-only loss, repair metadata and permitted FIB replay survive cover recovery without producer reinjection")
	})
}

func sysribECMPPath(prefix string, first bool) rpc.RouteInstallEntry {
	p := rpc.RouteInstallEntry{Protocol: "static", AFI: 1, SAFI: 1, Prefix: prefix, Instance: 101, NextHop: "10.249.1.1", Weight: 2, Metric: 7, RouteType: 1}
	if !first {
		p.Instance = 102
		p.NextHop = "10.249.2.1"
		p.Weight = 5
	}
	if prefix == sysribECMPLabels {
		p.Labels = []uint32{16001}
		if !first {
			p.Labels = []uint32{16002}
		}
	}
	if prefix == sysribECMPSID {
		p.SRv6SID = "2001:db8:247::a"
		if !first {
			p.SRv6SID = "2001:db8:249::b"
		}
	}
	return p
}

func sysribECMPWant(prefix string, promoted bool) sysribevents.BestChangeEntry {
	entry := sysribevents.BestChangeEntry{Prefix: netip.MustParsePrefix(prefix), Protocol: "static", NextHop: netip.MustParseAddr("198.18.247.1"), Interface: sysribECMPLinkA, OnLink: true, Weight: 2, Metric: 7, RouteType: 1}
	member := sysribevents.ECMPPath{NextHop: netip.MustParseAddr("198.18.248.1"), Interface: sysribECMPLinkB, OnLink: true, Weight: 5}
	if prefix == sysribECMPLabels {
		entry.Labels = []uint32{16001}
		member.Labels = []uint32{16002}
	}
	if prefix == sysribECMPSID {
		entry.SRv6SID = netip.MustParseAddr("2001:db8:247::a")
	}
	if promoted {
		entry.NextHop, entry.Interface, entry.Weight = member.NextHop, member.Interface, member.Weight
		entry.Labels = member.Labels
		if prefix == sysribECMPSID {
			entry.SRv6SID = netip.MustParseAddr("2001:db8:249::b")
		}
	} else {
		entry.ECMPPaths = []sysribevents.ECMPPath{member}
	}
	return entry
}

func sysribECMPState(ctx context.Context, p *sdk.Plugin, capture *sysribFIBCapture, promoted bool, replayAt uint64) error {
	for _, prefix := range []string{sysribECMPPlain, sysribECMPLabels, sysribECMPSID} {
		want := sysribECMPWant(prefix, promoted)
		if replayAt != 0 {
			want.Action = routeaction.Add
		}
		if err := capture.await(ctx, prefix, replayAt, want); err != nil {
			return err
		}
	}
	device := sysribevents.BestChangeEntry{Prefix: netip.MustParsePrefix(sysribECMPDevice), Protocol: "static", Interface: sysribECMPLinkA, Weight: 3, Metric: 7, RouteType: 1, ECMPPaths: []sysribevents.ECMPPath{{Interface: sysribECMPLinkB, Weight: 7}}}
	if replayAt != 0 {
		device.Action = routeaction.Add
	}
	if err := capture.await(ctx, sysribECMPDevice, replayAt, device); err != nil {
		return err
	}
	if !promoted {
		backup := sysribECMPWant(sysribECMPBackup, false)
		backup.ECMPPaths = nil
		backup.Backup = []sysribevents.ECMPPath{{NextHop: netip.MustParseAddr("198.18.248.1"), Interface: sysribECMPLinkB, OnLink: true, Weight: 1, Labels: []uint32{17001, 17002}}}
		if replayAt != 0 {
			backup.Action = routeaction.Add
		}
		if err := capture.await(ctx, sysribECMPBackup, replayAt, backup); err != nil {
			return err
		}
	}
	if promoted {
		if err := importKernel(ctx, sysribECMPPlain, 1, "via 198.18.248.1 ", "dev "+sysribECMPLinkB+" ", "proto 250 "); err != nil {
			return err
		}
	} else if err := sysribKernelMultipath(ctx, sysribECMPPlain, "via 198.18.247.1 ", "dev "+sysribECMPLinkA+" ", "weight 2 ", "via 198.18.248.1 ", "dev "+sysribECMPLinkB+" ", "weight 5 "); err != nil {
		return err
	}
	if promoted {
		if err := importKernel(ctx, sysribECMPLabels, 1, "via 198.18.248.1 ", "proto 250 ", "16002"); err != nil {
			return err
		}
		if err := importKernel(ctx, sysribECMPSID, 1, "via 198.18.248.1 ", "proto 250 ", "2001:db8:249::b"); err != nil {
			return err
		}
	} else {
		if err := sysribKernelMultipath(ctx, sysribECMPLabels, "via 198.18.247.1 ", "via 198.18.248.1 ", "16001", "16002"); err != nil {
			return err
		}
		if err := sysribKernelMultipath(ctx, sysribECMPSID, "via 198.18.247.1 ", "via 198.18.248.1 ", "2001:db8:247::a"); err != nil {
			return err
		}
		// The kernel has no standby member, so a route with a fast-reroute
		// backup is installed with its primary path alone and the backup next
		// hop and its repair labels never reach it.
		if err := importKernel(ctx, sysribECMPBackup, 1, "via 198.18.247.1 ", "dev "+sysribECMPLinkA+" ", "proto 250 "); err != nil {
			return err
		}
		if err := sysribKernelLacks(ctx, sysribECMPBackup, "via 198.18.248.1 ", "17001"); err != nil {
			return err
		}
	}
	if err := sysribKernelMultipath(ctx, sysribECMPDevice, "dev "+sysribECMPLinkA+" ", "weight 3 ", "dev "+sysribECMPLinkB+" ", "weight 7 "); err != nil {
		return err
	}
	for _, value := range []struct{ prefix, link string }{{"198.18.247.0/24", sysribECMPLinkA}, {"198.18.248.0/24", sysribECMPLinkB}} {
		if err := importKernel(ctx, value.prefix, 1, "dev "+value.link+" ", "proto 2 "); err != nil {
			return err
		}
	}
	return nil
}

// sysribKernelLacks reads the kernel routes for prefix once and fails when the
// record holds any forbidden token. -o keeps every nexthop on the route's one
// line, so a multipath member cannot hide on a continuation line. A failed read
// is an error, never a pass.
func sysribKernelLacks(ctx context.Context, prefix string, forbidden ...string) error {
	out, err := netfilterCommandOutput(ctx, "ip", "-N", "-o", "route", "show", "table", "all", "exact", prefix)
	if err != nil {
		return fmt.Errorf("kernel %s: read routes: %w", prefix, err)
	}
	line := strings.Join(strings.Fields(out), " ") + " "
	for _, word := range forbidden {
		if strings.Contains(line, word) {
			return fmt.Errorf("kernel %s: route holds %q, which must not reach the kernel; observed %q", prefix, word, out)
		}
	}
	return nil
}

// -o keeps all nexthops in the route's one record; ordinary ip output uses
// continuation lines that must not be counted as extra kernel routes.
func sysribKernelMultipath(ctx context.Context, prefix string, words ...string) error {
	var last string
	var lastErr error
	if Poll(ctx, 100, 100*time.Millisecond, func() bool {
		last, lastErr = netfilterCommandOutput(ctx, "ip", "-N", "-o", "route", "show", "table", "all", "exact", prefix)
		if lastErr != nil {
			return false
		}
		lines := strings.Split(strings.TrimSpace(last), "\n")
		if len(lines) != 1 {
			return false
		}
		line := strings.Join(strings.Fields(lines[0]), " ") + " "
		if !strings.Contains(line, "proto 250 ") {
			return false
		}
		for _, word := range words {
			if !strings.Contains(line, word) {
				return false
			}
		}
		return true
	}) {
		return nil
	}
	return fmt.Errorf("multipath kernel %s: expected one proto250 route with %q, observed %q; error %v", prefix, words, last, lastErr)
}

func sysribECMPKernelEmpty(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("ECMP kernel-empty fixture takes no arguments")
	}
	for _, prefix := range []string{sysribECMPPlain, sysribECMPDevice, sysribECMPLabels, sysribECMPSID, sysribECMPBackup, sysribECMPWithheld, "198.18.247.0/24", "198.18.248.0/24", "10.249.1.0/24", "10.249.2.0/24"} {
		if err := importKernel(ctx, prefix, 0); err != nil {
			return err
		}
	}
	for _, prefix := range []string{"2001:db8:247::/64", "2001:db8:248::/64", "2001:db8:249::/64"} {
		if err := sysribKernel6(ctx, prefix, 0); err != nil {
			return err
		}
	}
	return importSay("ECMP fixture prefixes are absent from every kernel table")
}

// This runs only after daemon/await-exit. Ze must have flushed its own
// prefixes while the actual devices and OS-owned routes still exist; deleting
// a device first could erase a stale FIB entry and make that absence vacuous.
func sysribECMPLinksCleanup(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("ECMP post-stop link cleanup takes no arguments")
	}
	for _, prefix := range []string{sysribECMPPlain, sysribECMPDevice, sysribECMPLabels, sysribECMPSID, sysribECMPBackup, sysribECMPWithheld, "10.249.1.0/24", "10.249.2.0/24"} {
		if err := importKernel(ctx, prefix, 0); err != nil {
			return err
		}
	}
	if err := sysribKernel6(ctx, "2001:db8:249::/64", 0); err != nil {
		return err
	}
	links := []struct{ prefix4, prefix6, name string }{
		{"198.18.247.0/24", "2001:db8:247::/64", sysribECMPLinkA},
		{"198.18.248.0/24", "2001:db8:248::/64", sysribECMPLinkB},
	}
	for _, link := range links {
		if err := importKernel(ctx, link.prefix4, 1, "dev "+link.name+" ", "proto 2 "); err != nil {
			return err
		}
		if err := sysribKernel6(ctx, link.prefix6, 1, "dev "+link.name+" ", "proto 2 "); err != nil {
			return err
		}
		if _, err := importCommandOutput(ctx, "ip", "link", "show", "dev", link.name); err != nil {
			return fmt.Errorf("joined daemon stop removed fixture device %s: %w", link.name, err)
		}
	}
	for _, link := range links {
		if err := importDeleteLink(ctx, link.name); err != nil {
			return err
		}
	}
	return importSay("joined daemon flushed Ze ECMP routes while both fixture devices and their IPv4 and IPv6 OS routes survived before owned link cleanup")
}
