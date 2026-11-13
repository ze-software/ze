// Design: docs/architecture/rib/unified-locrib.md -- bounded feed and Reset.
// Related: sysrib_forwarding_capture_fixture.go -- current live/replay evidence.

package fixture

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"strconv"
	"strings"
	"syscall"
	"time"

	sysribevents "github.com/ze-software/ze/internal/component/sysrib/events"
	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const (
	sysribSlowLink      = "zesrslow"
	sysribSlowConnected = "198.19.40.0/24"
	sysribSlowControl   = "198.19.42.0/24"
	sysribSlowForeign   = "198.19.43.0/24"
	sysribSlowWithheld  = "198.19.44.0/24"
	// More distinct mutations than both bounded record queues combined.
	// This also requires more than two maximum-sized snapshot pages.
	sysribSlowCount = 2*rpc.RouteViewRecordsMax + 17
)

func sysribSlowPrefix(index int) string {
	return netip.PrefixFrom(netip.AddrFrom4([4]byte{10, 253, byte(index >> 8), byte(index)}), 32).String()
}

func sysribSlowDriver(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("slow routing-view reader fixture takes no arguments")
	}
	stale, changed, tail := sysribSlowPrefix(0), sysribSlowPrefix(1), sysribSlowPrefix(sysribSlowCount-1)
	capture := newSysribFIBCapture(stale, changed, tail, sysribSlowControl)
	return observeConfigured(ctx, "sysrib-slow-test", &sdk.Registration{}, capture.setup, func(ctx context.Context, p *sdk.Plugin) (result error) {
		if err := sysribExecutionPlacement(ctx, p, "forked"); err != nil {
			return err
		}
		if err := importCreateLink(ctx, sysribSlowLink, "198.19.40.254/24"); err != nil {
			return err
		}
		if err := importCommandRun(ctx, "route", "add", "blackhole", sysribSlowForeign, "proto", "42"); err != nil {
			return err
		}
		// The paired carrier joins daemon shutdown before its dedicated
		// cleanup driver removes this device and the foreign route. Failed runs
		// retain their namespace owner's cleanup rather than deleting live FIB targets.
		if err := waitRIBWinner(ctx, p, sysribSlowConnected, "connected"); err != nil {
			return err
		}
		old := rpc.RouteInstallEntry{Protocol: protocolStatic, AFI: 1, SAFI: 1, Prefix: stale, RouteType: 6}
		control := old
		control.Prefix = sysribSlowControl
		withheld := rpc.RouteInstallEntry{Protocol: protocolBGP, AFI: 1, SAFI: 1, Prefix: sysribSlowWithheld, RouteType: 6, IsBGP: true, IsEBGP: true}
		if err := sysribInstall(ctx, p, old, control, withheld); err != nil {
			return err
		}
		if err := importKernel(ctx, stale, 1, "6 ", "proto 250 "); err != nil {
			return err
		}
		if err := importKernel(ctx, sysribSlowControl, 1, "6 "+sysribSlowControl+" ", "proto 250 "); err != nil {
			return err
		}
		if err := sysribHealthReady(ctx, p); err != nil {
			return err
		}
		before, err := sysribSlowScalar(ctx, p, "ze_rib_view_resync_total")
		if err != nil {
			return err
		}
		daemon, err := sysribProducerDaemon(ctx, p)
		if err != nil {
			return err
		}
		children, detail, err := sysribExecutionChildren(ctx, daemon)
		if err != nil {
			return err
		}
		if len(children) != 1 {
			return fmt.Errorf("slow-reader owner children %v; %s", children, detail)
		}
		pid := children[0]
		if err := syscall.Kill(pid, syscall.SIGSTOP); err != nil {
			return err
		}
		paused := true
		defer func() {
			if paused {
				result = errors.Join(result, syscall.Kill(pid, syscall.SIGCONT))
			}
		}()
		if err := sysribSlowStopped(ctx, pid); err != nil {
			return err
		}
		// Every input is accepted before the stopped consumer is resumed. This
		// is an observable progress barrier, without a throughput assumption.
		for start := 0; start < sysribSlowCount; start += 256 {
			entries := make([]rpc.RouteInstallEntry, 0, 256)
			for index := start; index < min(start+256, sysribSlowCount); index++ {
				entries = append(entries, rpc.RouteInstallEntry{Protocol: protocolStatic, AFI: 1, SAFI: 1, Prefix: sysribSlowPrefix(index), RouteType: 6})
			}
			if err := sysribInstall(ctx, p, entries...); err != nil {
				return fmt.Errorf("producer stalled behind stopped routing reader at prefix %d: %w", start, err)
			}
		}
		if err := sysribRemove(ctx, p, old); err != nil {
			return err
		}
		updated := rpc.RouteInstallEntry{Protocol: protocolStatic, AFI: 1, SAFI: 1, Prefix: changed, RouteType: 7}
		if err := sysribInstall(ctx, p, updated); err != nil {
			return err
		}
		if err := sysribSlowStopped(ctx, pid); err != nil {
			return err
		}
		// The daemon's fence must work even though no consumer instruction can
		// run. Keeping the old known-good control would conceal an unsafe lease.
		if err := importKernel(ctx, sysribSlowControl, 0); err != nil {
			return fmt.Errorf("bounded-feed reset did not fence outstanding output while its reader was stopped: %w", err)
		}
		if err := importKernel(ctx, stale, 0); err != nil {
			return err
		}
		if err := sysribSlowSurvivors(ctx); err != nil {
			return err
		}
		if err := importSay("all " + strconv.Itoa(sysribSlowCount) + " input routes were acknowledged while the actual routing child stayed stopped, and daemon reset fenced its old output"); err != nil {
			return err
		}
		if err := syscall.Kill(pid, syscall.SIGCONT); err != nil {
			return err
		}
		paused = false
		if err := sysribSlowCurrentKernel(ctx); err != nil {
			return err
		}
		if err := sysribHealthReady(ctx, p); err != nil {
			return err
		}
		var after float64
		var metricErr error
		if !Poll(ctx, 100, 100*time.Millisecond, func() bool {
			after, metricErr = sysribSlowScalar(ctx, p, "ze_rib_view_resync_total")
			if metricErr != nil {
				return false
			}
			return after > before
		}) {
			return fmt.Errorf("same-process routing recovery did not count reconstruction: before=%g after=%g: %w", before, after, metricErr)
		}
		current, detail, err := sysribExecutionChildren(ctx, daemon)
		if err != nil {
			return err
		}
		if len(current) != 1 {
			return fmt.Errorf("recovered children %v; %s", current, detail)
		}
		if current[0] != pid {
			return fmt.Errorf("slow-reader reset used replacement PID %d, want same %d", current[0], pid)
		}
		if err := sysribProducerAbsent(ctx, p, protocolStatic, stale); err != nil {
			return err
		}
		replayAt, err := capture.requestReplay(ctx, p)
		if err != nil {
			return err
		}
		for _, value := range []struct {
			prefix    string
			routeType sysribevents.RouteType
		}{{changed, 7}, {tail, 6}, {sysribSlowControl, 6}} {
			want := sysribevents.BestChangeEntry{Action: routeaction.Add, Prefix: netip.MustParsePrefix(value.prefix), Protocol: protocolStatic, RouteType: value.routeType}
			if err := capture.await(ctx, value.prefix, replayAt, want); err != nil {
				return err
			}
		}
		if err := importKernel(ctx, stale, 0); err != nil {
			return err
		}
		return importSay("the same slow routing child reconstructed the complete current multi-page table after explicit reset, retained the newer withdrawal/action and fresh replay, and recovered healthy doctor/telemetry")
	})
}

func sysribSlowStopped(ctx context.Context, pid int) error {
	var state string
	var last error
	if Poll(ctx, 100, 100*time.Millisecond, func() bool {
		state, last = daemonProcessState(ctx, pid)
		if last != nil {
			return false
		}
		return strings.HasPrefix(state, "T")
	}) {
		return nil
	}
	return fmt.Errorf("routing child PID %d is not stopped: state %q: %w", pid, state, last)
}

func sysribSlowSurvivors(ctx context.Context) error {
	if err := importKernel(ctx, sysribSlowConnected, 1, "dev "+sysribSlowLink+" ", "proto 2 "); err != nil {
		return err
	}
	if err := importKernel(ctx, sysribSlowForeign, 1, "6 "+sysribSlowForeign+" ", "proto 42 "); err != nil {
		return err
	}
	return importKernel(ctx, sysribSlowWithheld, 0)
}

func sysribSlowScalar(ctx context.Context, p *sdk.Plugin, name string) (float64, error) {
	data, err := requireDone02(ctx, p, "show metrics name "+name)
	if err != nil {
		return 0, err
	}
	var metrics struct {
		Series []struct {
			Line string `json:"line"`
		} `json:"series"`
	}
	if err := decode02(data, &metrics); err != nil {
		return 0, err
	}
	for _, series := range metrics.Series {
		fields := strings.Fields(series.Line)
		if len(fields) != 2 {
			continue
		}
		if fields[0] != name {
			continue
		}
		return strconv.ParseFloat(fields[1], 64)
	}
	return 0, fmt.Errorf("scalar %s is absent: %s", name, text02(data))
}

// sysribSlowCurrentKernel checks every offered prefix and its exact action in
// one kernel dump. Counting only the tail would accept a partial snapshot.
func sysribSlowCurrentKernel(ctx context.Context) error {
	var last string
	var lastErr error
	if !Poll(ctx, 200, 100*time.Millisecond, func() bool {
		data, err := netfilterCommandOutput(ctx, "ip", "-N", "-o", "route", "show", "table", "all", "proto", "250")
		if err != nil {
			lastErr = err
			return false
		}
		seen := make(map[string]uint8)
		for line := range strings.SplitSeq(data, "\n") {
			fields := strings.Fields(line)
			if len(fields) < 2 {
				continue
			}
			text := fields[1]
			if !strings.HasPrefix(text, "10.253.") {
				continue
			}
			prefix := text
			if !strings.Contains(text, "/") {
				address, err := netip.ParseAddr(text)
				if err != nil {
					lastErr = err
					return false
				}
				prefix = netip.PrefixFrom(address, address.BitLen()).String()
			}
			if _, duplicate := seen[prefix]; duplicate {
				last = "duplicate kernel entry for " + prefix
				return false
			}
			action, err := strconv.ParseUint(fields[0], 10, 8)
			if err != nil {
				lastErr = err
				return false
			}
			seen[prefix] = uint8(action)
		}
		if len(seen) != sysribSlowCount-1 {
			last = "current prefix count " + strconv.Itoa(len(seen)) + ", want " + strconv.Itoa(sysribSlowCount-1)
			return false
		}
		if _, found := seen[sysribSlowPrefix(0)]; found {
			last = "newer withdrawal was resurrected"
			return false
		}
		for index := 1; index < sysribSlowCount; index++ {
			want := uint8(6)
			if index == 1 {
				want = 7
			}
			prefix := sysribSlowPrefix(index)
			if seen[prefix] != want {
				last = "incorrect current action for " + prefix
				return false
			}
		}
		return true
	}) {
		return fmt.Errorf("multi-page snapshot did not reconstruct the exact kernel table: %s: %w", last, lastErr)
	}
	if err := importKernel(ctx, sysribSlowControl, 1, "6 "+sysribSlowControl+" ", "proto 250 "); err != nil {
		return err
	}
	return sysribSlowSurvivors(ctx)
}

func sysribSlowKernelEmpty(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("slow reader kernel-empty fixture takes no arguments")
	}
	if err := sysribSlowGeneratedAbsent(ctx); err != nil {
		return err
	}
	for _, prefix := range []string{sysribSlowConnected, sysribSlowControl, sysribSlowForeign, sysribSlowWithheld} {
		if err := importKernel(ctx, prefix, 0); err != nil {
			return err
		}
	}
	return importSay("slow-reader fixture prefixes are absent from every kernel table")
}

// sysribSlowGeneratedAbsent refuses any generated 10.253.0.0/16 host route in
// any kernel table, so a partial flush of the multi-page table cannot pass.
func sysribSlowGeneratedAbsent(ctx context.Context) error {
	data, err := netfilterCommandOutput(ctx, "ip", "-N", "-o", "route", "show", "table", "all")
	if err != nil {
		return err
	}
	for line := range strings.SplitSeq(data, "\n") {
		for field := range strings.FieldsSeq(line) {
			if strings.HasPrefix(field, "10.253.") {
				return fmt.Errorf("slow-reader fixture prefix remains in kernel: %s", line)
			}
		}
	}
	return nil
}

// This runs only after daemon/await-exit. Ze must have flushed its own
// prefixes while the actual device, its OS-owned route and the foreign proto
// 42 route still exist; deleting them first could erase a stale FIB entry and
// make that absence vacuous, and would hide a stop that flushed foreign state.
// The scenario stops and resumes the routing child, never the device.
func sysribSlowLinkCleanup(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("slow reader post-stop link cleanup takes no arguments")
	}
	if err := sysribSlowGeneratedAbsent(ctx); err != nil {
		return err
	}
	for _, prefix := range []string{sysribSlowControl, sysribSlowWithheld} {
		if err := importKernel(ctx, prefix, 0); err != nil {
			return err
		}
	}
	if err := importKernel(ctx, sysribSlowConnected, 1, "dev "+sysribSlowLink+" ", "proto 2 "); err != nil {
		return err
	}
	if err := importKernel(ctx, sysribSlowForeign, 1, "6 "+sysribSlowForeign+" ", "proto 42 "); err != nil {
		return err
	}
	if err := importCommandRun(ctx, "link", "show", "dev", sysribSlowLink); err != nil {
		return fmt.Errorf("joined daemon stop removed fixture device %s: %w", sysribSlowLink, err)
	}
	if err := importDeleteForeignBlackhole(ctx, sysribSlowForeign); err != nil {
		return err
	}
	if err := importDeleteLink(ctx, sysribSlowLink); err != nil {
		return err
	}
	return importSay("joined daemon flushed every Ze slow-reader route while the fixture device, its OS route and the foreign proto 42 route survived before owned cleanup")
}
