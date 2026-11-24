// Design: docs/architecture/fib/fib-vpp-route-ownership.md -- VPP FIB ECMP functional fixture
// Related: register_vpp_ecmp.go -- registers vpp/vpp-fib-ecmp and vpp/vpp-fib-ecmp-observer
// Related: misc_fixture_vpp.go -- shared VPP stub, ze launcher and stub log helpers

package fixture

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const (
	vppECMPObserverPlugin = "vpp-ecmp-observer"
	vppECMPConnectedNet   = "10.0.0.0/24"
	vppECMPOKLine         = "OK: fib-vpp programmed 2-path ECMP and updated to 1-path for 10.20.0.0/24"
	vppECMPPollDelay      = 100 * time.Millisecond
)

// vppECMPEvidence records the stub log line numbers of the 2-path ECMP
// install, the subsequent 1-path update after member withdrawal, and whether
// any withdraw (is_add=false) for the prefix appeared between them.
type vppECMPEvidence struct {
	twoPathLine   int
	onePathLine   int
	deleteBetween bool
}

// vppECMPConfig renders the daemon configuration for the VPP ECMP stub test.
// The dummy interface provides the real connected cover 10.0.0.0/24 for both
// BGP next hops (10.0.0.11 and 10.0.0.12), while the observer plugin injects
// and withdraws routes through bgp-rib and inspects the VPP stub JSONL log.
func vppECMPConfig(socket, logPath string) string {
	var tb textbuf.Buffer
	tb.Str("environment {\n}\n").
		Str("bus {\n").
		Str("    endpoint ").Str(vppECMPObserverPlugin).Str(" {\n").
		Str("        external {\n").
		Str("            run \"le test fixture vpp/vpp-fib-ecmp-observer ").Str(logPath).Str("\";\n").
		Str("        }\n").
		Str("        encoding json;\n").
		Str("    }\n").
		Str("    endpoint bgp-rib {\n").
		Str("        internal bgp-rib;\n").
		Str("    }\n").
		Str("    endpoint rib {\n").
		Str("        internal rib;\n").
		Str("    }\n").
		Str("}\n").
		Str("bgp {\n").
		Str("    router-id 10.0.0.1;\n").
		Str("    session { asn { local 65000; } }\n").
		Str("    multipath {\n").
		Str("        maximum-paths 4;\n").
		Str("        relax-as-path true;\n").
		Str("    }\n").
		Str("}\n").
		Str("vpp {\n").
		Str("    enabled true;\n").
		Str("    external true;\n").
		Str("    api-socket ").Str(socket).Str(";\n").
		Str("    lcp { enabled false; }\n").
		Str("}\n").
		Str("connected {}\n").
		Str("interface {\n").
		Str("    backend vpp;\n").
		Str("    dummy vpp-ecmp0 {\n").
		Str("        unit 0 {\n").
		Str("            ipv4 {\n").
		Str("                address [ 10.0.0.2/24 ];\n").
		Str("            }\n").
		Str("        }\n").
		Str("    }\n").
		Str("}\n").
		Str("fib {\n").
		Str("    vpp {\n").
		Str("        enabled true;\n").
		Str("    }\n").
		Str("}\n")
	return tb.String()
}

func vppECMPDriver(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("vpp-fib-ecmp fixture takes no arguments")
	}
	dir, socket, logPath, err := vppWorkPaths("vpp-ecmp-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir) //nolint:errcheck // fixture cleanup

	stub, err := startVPPStub(ctx, socket, logPath, 30)
	if err != nil {
		return err
	}
	defer stopFixtureProcess(stub, 2*time.Second)

	if !waitForFile(ctx, socket, 100, 50*time.Millisecond) {
		return errors.New("stub socket did not appear")
	}

	ze, err := startVPPZe(ctx, vppECMPConfig(socket, logPath), map[string]string{
		envConfigDir: dir,
		envLogVPP:    logLevelInfo,
		envLogFIBVPP: logLevelDebug,
		envLogBGP:    logLevelInfo,
	})
	if err != nil {
		return err
	}

	if err := waitFixtureProcess(ctx, ze, 25*time.Second); err != nil {
		stopFixtureProcess(ze, 3*time.Second)
		dumpVPPStubLog(logPath)
		return fmt.Errorf("ze did not exit cleanly within 25s: %w", err)
	}
	if !strings.Contains(ze.output.String(), vppECMPOKLine) {
		dumpVPPStubLog(logPath)
		return errors.New("vpp-fib-ecmp observer did not confirm 2-path ECMP install and 1-path update")
	}
	return nil
}

func vppECMPObserverDriver(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return errors.New("vpp-fib-ecmp-observer requires the VPP stub log path")
	}
	logPath := args[0]
	return Observe(ctx, vppECMPObserverPlugin, &sdk.Registration{}, func(ctx context.Context, p *sdk.Plugin) error {
		return vppECMPScenario(ctx, p, logPath)
	})
}

func vppECMPScenario(ctx context.Context, p *sdk.Plugin, logPath string) error {
	if err := waitVPPECMPRuntimeReady(ctx, p); err != nil {
		return err
	}

	if err := injectVPPECMPPath(ctx, p, ecmpNextHop1); err != nil {
		return err
	}
	if err := injectVPPECMPPath(ctx, p, ecmpNextHop2); err != nil {
		return err
	}

	twoPathLine, err := waitVPPECMPTwoPaths(ctx, logPath)
	if err != nil {
		return err
	}

	var withdrawCmd textbuf.Buffer
	withdrawCmd.Str("request bgp rib withdraw ").Str(ecmpNextHop1).Str(" ipv4/unicast ").Str(prefixTenTwenty)
	if err := fixture06DispatchDone(ctx, p, withdrawCmd.String()); err != nil {
		return err
	}

	evidence, err := waitVPPECMPOnePathAfter(ctx, logPath, twoPathLine)
	if err != nil {
		return err
	}
	if evidence.deleteBetween {
		return fmt.Errorf("AC-14 violation: stub logged is_add=false withdraw for %s between 2-path install (line %d) and 1-path update (line %d)",
			prefixTenTwenty, evidence.twoPathLine, evidence.onePathLine)
	}

	var ok textbuf.Buffer
	ok.Str(vppECMPOKLine).Byte('\n')
	return ok.StdErr()
}

func waitVPPECMPRuntimeReady(ctx context.Context, p *sdk.Plugin) error {
	if !Poll(ctx, 60, vppECMPPollDelay, func() bool {
		return fixture06DispatchDone(ctx, p, "show bgp rib status") == nil
	}) {
		return errors.New("bgp-rib plugin did not become ready")
	}
	if !Poll(ctx, 60, vppECMPPollDelay, func() bool {
		return fixture06DispatchDone(ctx, p, "show fib vpp") == nil
	}) {
		return errors.New("fib-vpp writer did not become ready")
	}
	if !Poll(ctx, 60, vppECMPPollDelay, func() bool {
		rows, err := fixture06DispatchArray(ctx, p, fixture06RIBCommand)
		return err == nil && fixture06RIBEntry(rows, vppECMPConnectedNet) != nil
	}) {
		return fmt.Errorf("connected cover %s did not appear in sysrib", vppECMPConnectedNet)
	}
	return nil
}

func injectVPPECMPPath(ctx context.Context, p *sdk.Plugin, nextHop string) error {
	var cmd textbuf.Buffer
	cmd.Str("request bgp rib inject ").Str(nextHop).
		Str(" ipv4/unicast ").Str(prefixTenTwenty).
		Str(" origin igp localpref 100 aspath 65001 nexthop ").Str(nextHop)
	return fixture06DispatchDone(ctx, p, cmd.String())
}

func waitVPPECMPTwoPaths(ctx context.Context, logPath string) (int, error) {
	var evidence vppECMPEvidence
	var scanErr error
	found := Poll(ctx, 120, vppECMPPollDelay, func() bool {
		evidence, scanErr = readVPPECMPEvidence(logPath, 0)
		if scanErr != nil {
			return true
		}
		return evidence.twoPathLine > 0
	})
	if scanErr != nil {
		return 0, scanErr
	}
	if !found {
		return 0, fmt.Errorf("stub did not log 2-path ECMP ip_route_add_del_v2 for %s via [%s %s]",
			prefixTenTwenty, ecmpNextHop1, ecmpNextHop2)
	}
	return evidence.twoPathLine, nil
}

func waitVPPECMPOnePathAfter(ctx context.Context, logPath string, twoPathLine int) (vppECMPEvidence, error) {
	var evidence vppECMPEvidence
	var scanErr error
	found := Poll(ctx, 120, vppECMPPollDelay, func() bool {
		evidence, scanErr = readVPPECMPEvidence(logPath, twoPathLine)
		if scanErr != nil {
			return true
		}
		return evidence.onePathLine > twoPathLine
	})
	if scanErr != nil {
		return vppECMPEvidence{}, scanErr
	}
	if !found {
		return vppECMPEvidence{}, fmt.Errorf("stub did not log 1-path ip_route_add_del_v2 update for %s via [%s] after line %d",
			prefixTenTwenty, ecmpNextHop2, twoPathLine)
	}
	return evidence, nil
}

// readVPPECMPEvidence scans the VPP stub JSONL log for ip_route_add_del_v2
// entries matching prefixTenTwenty. When afterTwoPath is 0 it locates the
// first 2-path install with both ECMP next hops; when afterTwoPath > 0 it
// locates the first 1-path surviving-member update after that line and notes
// any intervening is_add=false withdrawal.
func readVPPECMPEvidence(logPath string, afterTwoPath int) (vppECMPEvidence, error) {
	wantTwo := []string{ecmpNextHop1, ecmpNextHop2}
	wantOne := []string{ecmpNextHop2}
	evidence := vppECMPEvidence{twoPathLine: afterTwoPath}
	line := 0
	_, err := vppLogMatches(logPath, func(entry map[string]any) bool {
		line++
		if entry["msg"] != "ip_route_add_del_v2" {
			return false
		}
		fields, ok := entry["fields"].(map[string]any)
		if !ok {
			return false
		}
		if fields["prefix"] != prefixTenTwenty {
			return false
		}
		isAdd, _ := fields["is_add"].(bool)
		if afterTwoPath == 0 {
			if isAdd && fields["n_paths"] == float64(2) {
				if nextHops, ok := vppRoutePathNextHops(fields); ok && slices.Equal(nextHops, wantTwo) {
					evidence.twoPathLine = line
					return true
				}
			}
			return false
		}
		if line <= afterTwoPath {
			return false
		}
		if !isAdd {
			evidence.deleteBetween = true
			return false
		}
		if fields["n_paths"] == float64(1) {
			if nextHops, ok := vppRoutePathNextHops(fields); ok && slices.Equal(nextHops, wantOne) {
				evidence.onePathLine = line
				return true
			}
		}
		return false
	})
	return evidence, err
}

func vppRoutePathNextHops(fields map[string]any) ([]string, bool) {
	rawPaths, ok := fields["paths"].([]any)
	if !ok {
		return nil, false
	}
	nextHops := make([]string, 0, len(rawPaths))
	for _, raw := range rawPaths {
		path, ok := raw.(map[string]any)
		if !ok {
			return nil, false
		}
		nh, ok := path["next_hop"].(string)
		if !ok {
			return nil, false
		}
		nextHops = append(nextHops, nh)
	}
	slices.Sort(nextHops)
	return nextHops, true
}

func dumpVPPStubLog(logPath string) {
	data, err := os.ReadFile(logPath) //nolint:gosec // the path is the fixture's own scratch file
	if err != nil {
		return
	}
	var dump textbuf.Buffer
	dump.Str("--- stub log ---\n").Str(string(data)).Str("--- end ---\n")
	_ = dump.StdErr() //nolint:errcheck // diagnostic output for an error already being returned
}
