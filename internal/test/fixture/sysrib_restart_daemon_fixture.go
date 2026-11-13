// Design: docs/architecture/doctor-and-health-checks.md -- failed owner recovery.
// Related: sysrib_forwarding_capture_fixture.go -- old output withdrawal and fresh output.

package fixture

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"syscall"
	"time"

	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const (
	sysribRestartLink     = "zesrstop"
	sysribRestartForward  = "198.19.11.0/24"
	sysribRestartDiscard  = "198.19.12.0/24"
	sysribRestartForeign  = "198.19.13.0/24"
	sysribRestartWithheld = "198.19.14.0/24"
)

func sysribRestartDriver(ctx context.Context, args []string) error {
	if err := importPlacementArgs(args, 1); err != nil {
		return err
	}
	capture := newSysribFIBCapture(sysribRestartForward, sysribRestartDiscard, sysribRestartWithheld)
	return observeConfigured(ctx, "sysrib-restart-test", &sdk.Registration{}, capture.setup, func(ctx context.Context, p *sdk.Plugin) error {
		if err := sysribExecutionPlacement(ctx, p, args[0]); err != nil {
			return err
		}
		if err := importCreateLink(ctx, sysribRestartLink, "198.19.10.254/24"); err != nil {
			return err
		}
		if _, err := importCommandOutput(ctx, "ip", "route", "add", "blackhole", sysribRestartForeign, "proto", "42"); err != nil {
			return err
		}
		// The paired carrier joins daemon shutdown before its dedicated
		// cleanup driver removes this device and the foreign route. Failed runs
		// retain their namespace owner's cleanup rather than deleting live FIB targets.
		if err := waitRIBWinner(ctx, p, "198.19.10.0/24", "connected"); err != nil {
			return err
		}
		if err := sysribInstall(ctx, p,
			rpc.RouteInstallEntry{Protocol: "static", AFI: 1, SAFI: 1, Prefix: sysribRestartForward, NextHop: "198.19.10.1", Interface: sysribRestartLink, OnLink: true, RouteType: 1},
			rpc.RouteInstallEntry{Protocol: "static", AFI: 1, SAFI: 1, Prefix: sysribRestartDiscard, RouteType: 6},
			rpc.RouteInstallEntry{Protocol: "bgp", AFI: 1, SAFI: 1, Prefix: sysribRestartWithheld, NextHop: "198.19.10.1", Interface: sysribRestartLink, OnLink: true, IsBGP: true, IsEBGP: true, RouteType: 1},
		); err != nil {
			return err
		}
		if err := sysribRestartHealthy(ctx, p); err != nil {
			return err
		}
		capture.mu.Lock()
		beforeForward := capture.withdrawals[sysribRestartForward]
		beforeDiscard := capture.withdrawals[sysribRestartDiscard]
		capture.mu.Unlock()
		oldChild := 0
		if args[0] == "forked" {
			var session struct {
				PID int `json:"pid"`
			}
			data, err := requireDone02(ctx, p, "plugin session ping")
			if err != nil {
				return err
			}
			if err := decode02(data, &session); err != nil {
				return err
			}
			children, detail, err := sysribExecutionChildren(ctx, session.PID)
			if err != nil {
				return err
			}
			if len(children) != 1 {
				return fmt.Errorf("before kill: owner children %v: %s", children, detail)
			}
			oldChild = children[0]
			if err := syscall.Kill(oldChild, syscall.SIGKILL); err != nil {
				return err
			}
			if !Poll(ctx, 100, 100*time.Millisecond, func() bool {
				state, err := daemonProcessState(ctx, oldChild)
				if err != nil {
					return false
				}
				if state == "" {
					return true
				}
				return strings.HasPrefix(state, "Z")
			}) {
				return fmt.Errorf("killed owner PID %d did not exit", oldChild)
			}
			if err := importKernel(ctx, sysribRestartForward, 0); err != nil {
				return err
			}
			if err := importKernel(ctx, sysribRestartDiscard, 0); err != nil {
				return err
			}
			if err := sysribRestartSurvivors(ctx); err != nil {
				return err
			}
			if err := sysribRestartDoctorFailed(ctx, p); err != nil {
				return err
			}
		}
		// The sole producer stays alive and sends nothing else. Both manual
		// internal replacement and external crash recovery must scan current authority.
		if _, err := requireDone02(ctx, p, "request plugin restart "+sysribExecutionOwner); err != nil {
			return err
		}
		if !Poll(ctx, 100, 100*time.Millisecond, func() bool {
			capture.mu.Lock()
			defer capture.mu.Unlock()
			if capture.err != nil {
				return false
			}
			if capture.withdrawals[sysribRestartForward] <= beforeForward {
				return false
			}
			return capture.withdrawals[sysribRestartDiscard] > beforeDiscard
		}) {
			return fmt.Errorf("owner replacement never withdrew both old-generation outputs")
		}
		if err := sysribExecutionPlacement(ctx, p, args[0]); err != nil {
			return err
		}
		if oldChild != 0 {
			var session struct {
				PID int `json:"pid"`
			}
			data, err := requireDone02(ctx, p, "plugin session ping")
			if err != nil {
				return err
			}
			if err := decode02(data, &session); err != nil {
				return err
			}
			children, detail, err := sysribExecutionChildren(ctx, session.PID)
			if err != nil {
				return err
			}
			if len(children) != 1 {
				return fmt.Errorf("recovery has children %v; %s", children, detail)
			}
			if children[0] == oldChild {
				return fmt.Errorf("recovery retained old PID %d", oldChild)
			}
		}
		if err := sysribRestartHealthy(ctx, p); err != nil {
			return err
		}
		return importSay("owner replacement withdrew old output, preserved OS/foreign/withheld survivors and reinstalled both current controls without producer input, with healthy doctor recovery")
	})
}

func sysribRestartHealthy(ctx context.Context, p *sdk.Plugin) error {
	if err := waitRIBWinner(ctx, p, sysribRestartForward, "static"); err != nil {
		return err
	}
	if err := waitRIBWinner(ctx, p, sysribRestartDiscard, "static"); err != nil {
		return err
	}
	if err := waitRIBWinner(ctx, p, sysribRestartWithheld, "bgp"); err != nil {
		return err
	}
	if err := importKernel(ctx, sysribRestartForward, 1, "via 198.19.10.1 ", "dev "+sysribRestartLink+" ", "proto 250 "); err != nil {
		return err
	}
	if err := importKernel(ctx, sysribRestartDiscard, 1, "6 "+sysribRestartDiscard+" ", "proto 250 "); err != nil {
		return err
	}
	if err := sysribRestartSurvivors(ctx); err != nil {
		return err
	}
	return sysribHealthReady(ctx, p)
}

func sysribRestartSurvivors(ctx context.Context) error {
	if err := importKernel(ctx, "198.19.10.0/24", 1, "dev "+sysribRestartLink+" ", "proto 2 "); err != nil {
		return err
	}
	if err := importKernel(ctx, sysribRestartForeign, 1, "6 "+sysribRestartForeign+" ", "proto 42 "); err != nil {
		return err
	}
	return importKernel(ctx, sysribRestartWithheld, 0)
}

func sysribRestartDoctorFailed(ctx context.Context, p *sdk.Plugin) error {
	var last string
	if Poll(ctx, 60, 100*time.Millisecond, func() bool {
		data, err := requireDone02(ctx, p, "show doctor")
		if err != nil {
			last = err.Error()
			return false
		}
		last = text02(data)
		var report struct {
			Diagnostics []struct {
				Code    string `json:"code"`
				Message string `json:"message"`
				Fix     string `json:"fix"`
			} `json:"diagnostics"`
		}
		if decode02(data, &report) != nil {
			return false
		}
		for _, finding := range report.Diagnostics {
			if finding.Code != "doctor-plugin-unreachable" {
				continue
			}
			if !strings.Contains(finding.Message, sysribExecutionOwner) {
				continue
			}
			if finding.Fix != "ze request plugin restart "+sysribExecutionOwner {
				continue
			}
			return true
		}
		return false
	}) {
		return nil
	}
	return fmt.Errorf("failed routing owner doctor omitted exact recovery command: %s", last)
}

func sysribRestartKernelEmpty(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("restart kernel-empty fixture takes no arguments")
	}
	for _, prefix := range []string{sysribRestartForward, sysribRestartDiscard, sysribRestartForeign, sysribRestartWithheld, "198.19.10.0/24"} {
		if err := importKernel(ctx, prefix, 0); err != nil {
			return err
		}
	}
	return importSay("restart fixture prefixes are absent from every kernel table")
}

// This runs only after daemon/await-exit. Ze must have flushed its own
// prefixes while the actual device, its OS-owned route and the foreign proto
// 42 route still exist; deleting them first could erase a stale FIB entry and
// make that absence vacuous, and would hide a stop that flushed foreign state.
// The forked placement kills the routing owner on purpose, but a replacement
// reinstalls both controls before the scenario returns, so the same end state
// holds for both placements.
func sysribRestartLinkCleanup(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("restart post-stop link cleanup takes no arguments")
	}
	for _, prefix := range []string{sysribRestartForward, sysribRestartDiscard, sysribRestartWithheld} {
		if err := importKernel(ctx, prefix, 0); err != nil {
			return err
		}
	}
	if err := importKernel(ctx, "198.19.10.0/24", 1, "dev "+sysribRestartLink+" ", "proto 2 "); err != nil {
		return err
	}
	if err := importKernel(ctx, sysribRestartForeign, 1, "6 "+sysribRestartForeign+" ", "proto 42 "); err != nil {
		return err
	}
	if _, err := importCommandOutput(ctx, "ip", "link", "show", "dev", sysribRestartLink); err != nil {
		return fmt.Errorf("joined daemon stop removed fixture device %s: %w", sysribRestartLink, err)
	}
	if err := importDeleteForeignBlackhole(ctx, sysribRestartForeign); err != nil {
		return err
	}
	if err := importDeleteLink(ctx, sysribRestartLink); err != nil {
		return err
	}
	return importSay("joined daemon flushed Ze restart routes while the fixture device, its OS route and the foreign proto 42 route survived before owned cleanup")
}
