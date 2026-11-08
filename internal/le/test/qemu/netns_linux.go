//go:build linux

// Design: docs/architecture/testing/qemu-integration.md -- guest-side evidence ports
// Replaces the former netns Python guest driver.
package testqemu

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/gaterun"
	testfunctional "github.com/ze-software/ze/internal/le/test/functional"
)

// netnsSuiteTimeout is the wall-clock cap one selected subset runs under. The
// paths, the capability set and the environment keys this launcher shares with
// the whole-suite run are netns.go.
const netnsSuiteTimeout = 15 * time.Minute

var netnsSelections = map[string][]string{
	netnsFirewall: {
		"firewall-boot-apply", "firewall-reload", "firewall-coexistence", "firewall-cli-show",
		"firewall-match-in-set-addr", "firewall-dscp-ipv6-reject-validate", "firewall-setdscp-inet",
		"firewall-match-in-set-port", "firewall-byte-rate-limit", "firewall-snat-addr-range",
		"firewall-icmp-type", "firewall-iface-wildcard", "firewall-nat-exclude",
		"firewall-masquerade-ports", "firewall-masquerade-flags", "command-owner-firewall-root",
		"copp-bgp", "copp-trusted", "copp-withdraw", "flush-persist", "flush-crash", "ddos-local-withdraw",
		// The GTSM pair declares option=netns-link: outside this launcher the
		// runner skips them, so this selection is the only place they run.
		"gtsm-related-icmp", "gtsm-listener-floor",
	},
	netnsPolicy: {
		"policy-boot-apply", "policy-set-table", "policy-tcp-flags", "policy-tcp-mss",
		"policy-next-hop", "policy-reload", "policy-interface-list",
		"policy-interface-list-counters",
	},
	netnsOSPF: {
		"ospf-instance-demux", "ospf-multiaf", "ospf-multiaf-reconcile", "ospf-multiaf-show",
		"ospf-multiaf-v4-route", "ospf-nbma", "ospf-ptmp", "ospf-show",
	},
	netnsOSPFv3: {"ospfv3-vlink", "ospfv3-nbma", "ospfv3-ptmp"},
	netnsPPPoE: {
		"pppoe-basic", "pppoe-concurrent-l2tp", "pppoe-service-name", "pppoe-vlan",
		"pppoe-per-mac-cap", "pppoe-padr-flood", "pppoe-reload-apply", "pppoe-reload-enable",
		"pppoe-client-lcp-renegotiation", "pppoe-realm-relay-nomatch",
	},
	netnsPlugin: {
		"rpki-aspa-valid", "rpki-aspa-invalid", "rpki-aspa-unknown", "rpki-aspa-disabled",
		"rpki-aspa-policy-logonly", "rpki-aspa-policy-reject", "rpki-aspa-policy-unknown-reject",
		"adj-rib-in-replay-rfc2545-next-hop",
		"redistribute-chain-static-ospf",
	},
}

// netnsBinaries names the two daemon binaries the launcher copies with
// capabilities. The harness, the running le itself, is copied beside them by
// prepareNetnsBinaries (netnsHarnessSource).
type netnsBinaries struct {
	Ze       string
	Stripped string
}

func netnsGuestBinaries() netnsBinaries {
	arch := GuestArch()
	return netnsBinaries{
		Ze:       settingFromEnv("ZE_QEMU_BIN", filepath.Join("bin", "ze-linux-"+arch)),
		Stripped: settingFromEnv("ZE_QEMU_STRIPPED_BIN", filepath.Join("bin", "ze-stripped-linux-"+arch)),
	}
}

func validateNetnsSelection(suites []string) error {
	var tb textbuf.Buffer
	for _, suiteName := range suites {
		if _, ok := testfunctional.SuiteNamed(suiteName); !ok {
			return fmt.Errorf("functional suite %q is not declared", suiteName)
		}
		ids, ok := netnsSelections[suiteName]
		if !ok {
			return fmt.Errorf("suite %q has no netns evidence selection", suiteName)
		}
		bad := make([]string, 0)
		for _, name := range ids {
			if _, err := strconv.Atoi(name); err == nil {
				bad = append(bad, tb.Quoted(name).
					Str(": numeric nick (a position, not an identity)").String())
				tb.Reset()
				continue
			}
			path := filepath.Join("test", suiteName, name+".ci")
			if info, err := os.Stat(path); err != nil || info.IsDir() {
				bad = append(bad, tb.Quoted(name).Str(": no ").Str(path).String())
				tb.Reset()
			}
		}
		if len(bad) != 0 {
			return fmt.Errorf("%s subset invalid; refusing to run a silently smaller gate: %s", suiteName, strings.Join(bad, "; "))
		}
		gaterun.Note(tb.Str("selection OK: ").Str(suiteName).Str(" names ").
			Int(int64(len(ids))).Str(" test(s)").String())
		tb.Reset()
	}
	return nil
}

func prepareNetnsBinaries(ctx context.Context, binaries netnsBinaries) error {
	if err := os.MkdirAll(netnsCapDir, 0o750); err != nil {
		return fmt.Errorf("create %s: %w", netnsCapDir, err)
	}
	if result, err := guestRun(ctx, "", []string{"sh", "-c", netnsCapMountScript()}, nil); err != nil || result.Code != 0 {
		if err != nil {
			return err
		}
		return fmt.Errorf("mount a tmpfs that honors file capabilities on %s failed with exit %d", netnsCapDir, result.Code)
	}
	if err := os.Chown(netnsCapDir, 0, 1000); err != nil {
		return fmt.Errorf("make %s traversable by the credential-dropped test group: %w", netnsCapDir, err)
	}
	harness, err := netnsHarnessSource()
	if err != nil {
		return err
	}
	var tb textbuf.Buffer
	copies := []struct{ name, source string }{{"ze", binaries.Ze}, {"ze-stripped", binaries.Stripped}, {leName, harness}}
	for _, pair := range copies {
		destination := filepath.Join(netnsCapDir, pair.name)
		// rm first: cp onto a binary a previous run still executes fails
		// with ETXTBSY, and an unlinked inode stays valid for that run.
		copyLine := tb.Str("rm -f ").Str(destination).Str(" && cp ").Str(pair.source).Byte(' ').Str(destination).
			Str(" && chmod 0755 ").Str(destination).String()
		tb.Reset()
		if result, err := guestRun(ctx, "", []string{"sh", "-c", copyLine}, nil); err != nil || result.Code != 0 {
			if err != nil {
				return err
			}
			return fmt.Errorf("copy %s failed with exit %d", pair.source, result.Code)
		}
		if result, err := guestRun(ctx, "", []string{"setcap", netnsCapabilities, destination}, nil); err != nil || result.Code != 0 {
			if err != nil {
				return err
			}
			return fmt.Errorf("setcap %s failed (no xattr support?)", destination)
		}
	}
	if _, err := guestRun(ctx, "", []string{"getcap", filepath.Join(netnsCapDir, "ze")}, nil); err != nil {
		return fmt.Errorf("inspect ze capabilities: %w", err)
	}
	return nil
}

func prepareNetnsState(ctx context.Context) error {
	var tb textbuf.Buffer
	line := tb.Str("rm -rf ").Str(netnsStateDir).Str(" && mkdir -p ").Str(netnsStateDir).
		Str(" && chown 1000:1000 ").Str(netnsStateDir).Str(" && chmod 0750 ").Str(netnsStateDir).String()
	result, err := guestRun(ctx, "", []string{"sh", "-c", line}, nil)
	if err != nil {
		return err
	}
	if result.Code != 0 {
		return fmt.Errorf("prepare %s failed with exit %d", netnsStateDir, result.Code)
	}
	result, err = guestRun(ctx, "", []string{"sh", "-c", netnsDevPPPScript()}, nil)
	if err != nil {
		return err
	}
	if result.Code != 0 {
		return fmt.Errorf("open /dev/ppp to the dropped user failed with exit %d: %s", result.Code, strings.TrimSpace(result.Stderr))
	}
	return nil
}

func hostNFTTables(ctx context.Context) (string, error) {
	result, err := guestRun(ctx, "", []string{"nft", "list", "tables"}, nil)
	if err != nil {
		return "", err
	}
	if result.Code != 0 {
		return "", fmt.Errorf("nft list tables failed with exit %d: %s", result.Code, strings.TrimSpace(result.Stderr))
	}
	return result.Stdout, nil
}

func runNetnsSuite(ctx context.Context, binaries netnsBinaries, suiteName string, ids []string) int {
	environ := withGuestEnv(os.Environ(), map[string]string{
		noBuildKey:     "1",
		inVMKey:        "1",
		zeBinKey:       filepath.Join(netnsCapDir, "ze"),
		strippedBinKey: filepath.Join(netnsCapDir, "ze-stripped"),
		netnsModeKey:   netnsModeValue,
		netnsUIDKey:    netnsUID,
		netnsGIDKey:    netnsUID,
		netnsConfigKey: netnsStateDir,
	})
	// runNetnsGuest validates registry membership before invoking a suite.
	suite, _ := testfunctional.SuiteNamed(suiteName)
	suiteArgs := suite.Args
	// prepareNetnsBinaries copied the harness here with the daemons' file
	// capabilities (netnsHarnessSource).
	harness := filepath.Join(netnsCapDir, leName)
	// The suite runs inside the slot the host's run holds (guestJobParent).
	parent, err := guestJobParent(netnsCapDir)
	if err != nil {
		gaterun.Note("name the host job inside " + netnsCapDir + ": " + err.Error())
		return 1
	}
	environ = append(environ, parent)
	argv := make([]string, 2, len(suiteArgs)+len(ids)+4)
	argv[0] = harness
	argv[1] = leTestWord
	for _, arg := range suiteArgs {
		if arg == allTests {
			break // Replace the registry's population and concurrency with this subset.
		}
		argv = append(argv, arg)
	}
	argv = append(argv, "-p", "1")
	argv = append(argv, ids...)
	var tb textbuf.Buffer
	gaterun.Note(tb.Str("+ ").Join(argv, " ").String())
	deadline, cancel := context.WithTimeout(ctx, netnsSuiteTimeout)
	defer cancel()
	// #nosec G204 -- argv selectors come from closed package tables and the executable is the established guest fixture binary.
	command := exec.CommandContext(deadline, argv[0], argv[1:]...)
	// The timeout branch below owns bounded process-group termination.
	command.Cancel = func() error { return nil }
	command.Env = environ
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := command.Start(); err != nil {
		if errors.Is(err, context.Canceled) {
			return 124
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return 124
		}
		tb.Reset()
		gaterun.Note(tb.Str("  cannot run ").Str(argv[0]).Str(": ").Err(err).String())
		return gaterun.CannotStart
	}
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	select {
	case err := <-done:
		if err == nil {
			return 0
		}
		return gaterun.ExitCode(err)
	case <-deadline.Done():
		_ = syscall.Kill(-command.Process.Pid, syscall.SIGTERM)
		select {
		case <-done:
		case <-time.After(processGrace):
			_ = syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
			select {
			case <-done:
			case <-time.After(processKillWait):
			}
		}
		return 124
	}
}

func runEveryNetnsSuite(ctx context.Context, binaries netnsBinaries, suites []string, run func(context.Context, netnsBinaries, string, []string) int) []GuestScenario {
	results := make([]GuestScenario, 0, len(suites))
	for _, suiteName := range suites {
		code := run(ctx, binaries, suiteName, netnsSelections[suiteName])
		scenario := GuestScenario{Name: suiteName, Verdict: VerdictPass}
		if code != 0 {
			scenario.Verdict = VerdictFail
			scenario.Failure = strconv.Itoa(code)
		}
		results = append(results, scenario)
	}
	return results
}

func runNetnsGuest(ctx context.Context, suites []string) (guestLabReport, error) {
	lab := labNetns
	if len(suites) == 1 && suites[0] == netnsPPPoE {
		lab = labPPPoENetns
	}
	report := guestLabReport{Lab: lab, Selected: append([]string(nil), suites...), Verdict: VerdictUnspecified}
	// Selection and file existence are checked before setcap, chmod, nft, or any
	// suite process. A bad closed name can therefore produce no partial effects.
	if err := validateNetnsSelection(suites); err != nil {
		return report, err
	}
	if err := requireGuestCommands("setcap", "getcap", "nft"); err != nil {
		return report, err
	}
	binaries := netnsGuestBinaries()
	if err := prepareNetnsBinaries(ctx, binaries); err != nil {
		return report, err
	}
	if err := prepareNetnsState(ctx); err != nil {
		return report, err
	}
	oldMask := syscall.Umask(0o077)
	defer syscall.Umask(oldMask)
	before, err := hostNFTTables(ctx)
	if err != nil {
		return report, err
	}
	report.Scenarios = runEveryNetnsSuite(ctx, binaries, suites, runNetnsSuite)
	after, err := hostNFTTables(ctx)
	if err != nil {
		return report, err
	}
	hostSafe := before == after
	report.HostSafe = &hostSafe
	report.Verdict = VerdictPass
	for _, scenario := range report.Scenarios {
		if scenario.Verdict != VerdictPass {
			report.Verdict = VerdictFail
		}
	}
	if !hostSafe {
		report.Verdict = VerdictFail
		var tb textbuf.Buffer
		report.Failure = tb.Str("host nft tables changed during the netns run\n--- before ---\n").
			Str(before).Str("\n--- after ---\n").Str(after).String()
		fmt.Fprintln(os.Stderr, "HOST-SAFETY FAIL: host nft tables changed during the netns run") //nolint:errcheck // evidence verdict
		fmt.Fprintf(os.Stderr, "--- before ---\n%s\n--- after ---\n%s\n", before, after)          //nolint:errcheck // evidence diagnostics
	}
	return report, nil
}
