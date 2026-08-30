//go:build linux

// Design: plan/spec-le-is-a-ze-binary.md -- step 10 guest-side evidence ports
// Replaces the former netns Python guest driver.
package qemu

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/functional"
	"github.com/ze-software/ze/internal/le/gaterun"
)

const (
	netnsCapabilities = "cap_net_admin,cap_net_raw,cap_net_bind_service+ep"
	netnsCapDir       = "/tmp/zebin"
	netnsStateDir     = "/tmp/zestate"
	netnsSuiteTimeout = 15 * time.Minute
)

var netnsSelections = map[string][]string{
	netnsFirewall: {
		"firewall-boot-apply", "firewall-reload", "firewall-coexistence", "firewall-cli-show",
		"firewall-match-in-set-addr", "firewall-dscp-ipv6-reject-validate", "firewall-setdscp-inet",
		"firewall-match-in-set-port", "firewall-byte-rate-limit", "firewall-snat-addr-range",
		"firewall-icmp-type", "firewall-iface-wildcard", "firewall-nat-exclude",
		"firewall-masquerade-ports", "firewall-masquerade-flags", "command-owner-firewall-root",
		"copp-bgp", "copp-trusted", "copp-withdraw", "flush-persist", "flush-crash", "ddos-local-withdraw",
	},
	netnsPolicy: {
		"policy-boot-apply", "policy-set-table", "policy-tcp-flags", "policy-tcp-mss",
		"policy-next-hop", "policy-reload",
	},
	netnsOSPF: {
		"ospf-instance-demux", "ospf-multiaf", "ospf-multiaf-reconcile", "ospf-multiaf-show",
		"ospf-multiaf-v4-route", "ospf-nbma", "ospf-ptmp", "ospf-show",
	},
	netnsOSPFv3: {"ospfv3-vlink", "ospfv3-nbma", "ospfv3-ptmp"},
	netnsPPPoE:  {"pppoe-basic", "pppoe-concurrent-l2tp", "pppoe-vlan"},
}

type netnsBinaries struct {
	Ze       string
	Stripped string
	Test     string
}

func qemuGuestArch() string {
	if named := os.Getenv("QEMU_GOARCH"); named != "" {
		return named
	}
	if runtime.GOARCH == ArchARM64 {
		return ArchARM64
	}
	return ArchAMD64
}

func netnsGuestBinaries() netnsBinaries {
	arch := qemuGuestArch()
	return netnsBinaries{
		Ze:       settingFromEnv("ZE_QEMU_BIN", filepath.Join("bin", "ze-linux-"+arch)),
		Stripped: settingFromEnv("ZE_QEMU_STRIPPED_BIN", filepath.Join("bin", "ze-stripped-linux-"+arch)),
		Test:     settingFromEnv("ZE_QEMU_TEST_BIN", filepath.Join("bin", "ze-test-linux-"+arch)),
	}
}

func validateNetnsSelection(suites []string) error {
	var tb textbuf.Buffer
	for _, suiteName := range suites {
		// PPPoE is release evidence rather than a functional gate, so it is not
		// in functional.Suites. Every other selected suite MUST resolve through
		// that exported registry rather than a duplicate table here.
		if suiteName != netnsPPPoE {
			if _, ok := functional.SuiteNamed(suiteName); !ok {
				return fmt.Errorf("functional suite %q is not declared", suiteName)
			}
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
	if err := os.Chown(netnsCapDir, 0, 1000); err != nil {
		return fmt.Errorf("make %s traversable by the credential-dropped test group: %w", netnsCapDir, err)
	}
	var tb textbuf.Buffer
	for _, pair := range []struct{ name, source string }{{"ze", binaries.Ze}, {"ze-stripped", binaries.Stripped}} {
		destination := filepath.Join(netnsCapDir, pair.name)
		copyLine := tb.Str("cp ").Str(pair.source).Byte(' ').Str(destination).
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

func inThrowawayGuest() bool {
	data, err := os.ReadFile("/proc/mounts")
	if err != nil {
		return false
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 3 && fields[1] == "/workspace" && fields[2] == "9p" {
			return true
		}
	}
	return false
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
	if _, err := os.Stat("/dev/ppp"); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return fmt.Errorf("inspect /dev/ppp: %w", err)
	}
	if !inThrowawayGuest() {
		return errors.New("refusing to chmod 0666 /dev/ppp: this is not the QEMU evidence VM, and the widened node would persist for every local user. Run: ./le qemu pppoe-test")
	}
	result, err = guestRun(ctx, "", []string{"sh", "-c", "chmod 0666 /dev/ppp"}, nil)
	if err != nil {
		return err
	}
	if result.Code != 0 {
		return fmt.Errorf("chmod /dev/ppp failed with exit %d", result.Code)
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
		noBuildKey:        "1",
		inVMKey:           "1",
		zeBinKey:          filepath.Join(netnsCapDir, "ze"),
		"ZE_STRIPPED_BIN": filepath.Join(netnsCapDir, "ze-stripped"),
		guestTestBinKey:   binaries.Test,
		"ZE_TEST_NETNS":   "1",
		"ZE_TEST_UID":     "1000",
		"ZE_TEST_GID":     "1000",
		"ze.config.dir":   netnsStateDir,
	})
	argv := append([]string{binaries.Test, suiteName, "-p", "1"}, ids...)
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
