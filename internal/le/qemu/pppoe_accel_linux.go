//go:build linux

// Design: plan/spec-le-is-a-ze-binary.md -- step 10 guest-side evidence ports
// Replaces the former effective PPPoE Python guest driver.
package qemu

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	pppoePeerAddress  = "10.11.0.1"
	pppoeLocalAddress = "10.11.0.2"
	pppoeUsername     = "alice"
	pppoePassword     = "s3cr3t" //nolint:gosec // fixed interop fixture credential from the producer
	pppoeService      = "internet"
)

const (
	pppoeAccelStartupWait = 2 * time.Second
	pppoeZeSessionTimeout = 75 * time.Second
	pppoeACSessionTimeout = 30 * time.Second
	pppoeAddressTimeout   = 20 * time.Second
	pppoeTeardownTimeout  = 20 * time.Second
)

func pppoeAccelConfig(work, acVeth string) []byte {
	var tb textbuf.Buffer
	return tb.Str("[modules]\n").
		Str("log_file\n").
		Str("pppoe\n").
		Str("auth_chap_md5\n").
		Str("chap-secrets\n").
		Str("ippool\n\n").
		Str("[core]\nthread-count=1\n\n").
		Str("[log]\nlog-file=").Str(filepath.Join(work, "accel.log")).Str("\nlevel=4\n\n").
		Str("[pppoe]\ninterface=").Str(acVeth).
		Str("\nac-name=ze-accel-ac\nservice-name=").Str(pppoeService).Str("\nverbose=1\n\n").
		Str("[ppp]\nverbose=1\nmtu=1492\nmru=1492\nipv4=require\nipv6=deny\n").
		Str("lcp-echo-interval=30\nlcp-echo-failure=3\nmppe=deny\n\n").
		Str("[ip-pool]\ngw-ip-address=").Str(pppoePeerAddress).Byte('\n').
		Str(pppoeLocalAddress).Str("-10\n\n").
		Str("[chap-secrets]\nchap-secrets=").Str(filepath.Join(work, "chap-secrets")).Byte('\n').
		Bytes()
}

func pppoeZeConfig(zeVeth string) []byte {
	var tb textbuf.Buffer
	return tb.Str("interface {\n").
		Str("    pppoe-client pppoe0 {\n").
		Str("        source-interface ").Str(zeVeth).Str(";\n").
		Str("        service-name ").Str(pppoeService).Str(";\n").
		Str("        authentication {\n").
		Str("            username ").Str(pppoeUsername).Str(";\n").
		Str("            password ").Str(pppoePassword).Str(";\n").
		Str("        }\n").
		Str("    }\n").
		Str("}\n").
		Bytes()
}

func rejectPPPoEProbeSkip() error {
	for _, key := range []string{"ZE_PPPOE_SKIP_KERNEL_PROBE", "ze.pppoe.skip-kernel-probe"} {
		if _, present := os.LookupEnv(key); present {
			return fmt.Errorf("refusing to run with %s set; full proof must not skip the kernel probe", key)
		}
	}
	return nil
}

func probePPPoEKernel(ctx context.Context) error {
	if !hasGuestNetAdmin() {
		return errors.New("full PPPoE evidence requires root or CAP_NET_ADMIN")
	}
	info, err := os.Stat("/dev/ppp")
	if err != nil || info.Mode()&os.ModeCharDevice == 0 {
		return errors.New("missing /dev/ppp character device (CONFIG_PPP)")
	}
	if modprobe, err := execLookPath("modprobe"); err == nil && os.Geteuid() == 0 {
		for _, module := range []string{"ppp_generic", "pppox", "pppoe"} {
			if _, loadErr := guestRun(ctx, "", []string{modprobe, module}, nil); loadErr != nil {
				fmt.Fprintf(os.Stderr, "load optional PPPoE module %s: %v\n", module, loadErr) //nolint:errcheck // probe diagnostics
			}
		}
	}
	if _, moduleErr := os.Stat("/sys/module/pppoe"); moduleErr != nil {
		if _, procErr := os.Stat("/proc/net/pppoe"); procErr != nil {
			return errors.New("missing PPPoE kernel support: expected /sys/module/pppoe or /proc/net/pppoe (CONFIG_PPPOE)")
		}
	}
	return nil
}

func setupPPPoENamespaces(ctx context.Context, zeNS, acNS, zeVeth, acVeth string) error {
	if err := os.MkdirAll("/run/netns", 0o750); err != nil {
		return fmt.Errorf("create /run/netns: %w", err)
	}
	var tb textbuf.Buffer
	operation := func(prefix, name, suffix string) string {
		value := tb.Str(prefix).Str(name).Str(suffix).String()
		tb.Reset()
		return value
	}
	steps := []struct {
		ns   string
		argv []string
		what string
	}{
		{argv: []string{"ip", "netns", "add", zeNS}, what: operation("create netns ", zeNS, "")},
		{argv: []string{"ip", "netns", "add", acNS}, what: operation("create netns ", acNS, "")},
		{argv: []string{"ip", "link", "add", zeVeth, "type", "veth", "peer", "name", acVeth}, what: "create PPPoE access-link veth pair"},
		{argv: []string{"ip", "link", "set", zeVeth, "netns", zeNS}, what: "move Ze veth"},
		{argv: []string{"ip", "link", "set", acVeth, "netns", acNS}, what: "move AC veth"},
		{ns: zeNS, argv: []string{"ip", "link", "set", "lo", "up"}, what: operation("bring up ", zeNS, " loopback")},
		{ns: zeNS, argv: []string{"ip", "link", "set", zeVeth, "up"}, what: operation("bring up ", zeVeth, "")},
		{ns: acNS, argv: []string{"ip", "link", "set", "lo", "up"}, what: operation("bring up ", acNS, " loopback")},
		{ns: acNS, argv: []string{"ip", "link", "set", acVeth, "up"}, what: operation("bring up ", acVeth, "")},
	}
	for _, step := range steps {
		if err := guestRequired(ctx, step.ns, step.argv, step.what); err != nil {
			return err
		}
	}
	return nil
}

func pppLinks(ctx context.Context, namespace string) (map[string]bool, error) {
	result, err := guestRun(ctx, namespace, []string{"ip", "-o", "link", "show", "type", "ppp"}, nil)
	if err != nil {
		return nil, err
	}
	if result.Code != 0 {
		return nil, fmt.Errorf("list PPP links in %s failed with exit %d", namespace, result.Code)
	}
	links := make(map[string]bool)
	for line := range strings.SplitSeq(result.Stdout, "\n") {
		_, rest, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		name := strings.TrimSpace(rest)
		if at := strings.IndexAny(name, ":@"); at >= 0 {
			name = name[:at]
		}
		if name != "" {
			links[name] = true
		}
	}
	return links, nil
}

func newLinks(current, initial map[string]bool) []string {
	added := make([]string, 0, len(current))
	for name := range current {
		if !initial[name] {
			added = append(added, name)
		}
	}
	sort.Strings(added)
	return added
}

func waitNewPPP(ctx context.Context, namespace, role string, initial map[string]bool, process *guestProcess, timeout time.Duration) (string, error) {
	var found string
	err := waitGuest(ctx, timeout, 500*time.Millisecond, func() (bool, error) {
		current, err := pppLinks(ctx, namespace)
		if err != nil {
			return false, err
		}
		added := newLinks(current, initial)
		if len(added) > 1 {
			return false, fmt.Errorf("more than one new PPP interface in %s namespace: %s", role, strings.Join(added, ", "))
		}
		if len(added) == 1 {
			found = added[0]
			return true, nil
		}
		if process.exited() {
			return false, fmt.Errorf("%s process exited before a pppN interface appeared", role)
		}
		return false, nil
	})
	if err != nil {
		return "", fmt.Errorf("no pppN interface appeared in %s namespace within %s: %w", role, timeout, err)
	}
	return found, nil
}

func waitPPPAddress(ctx context.Context, namespace, iface, local, peer string, timeout time.Duration) error {
	last := ""
	err := waitGuest(ctx, timeout, 500*time.Millisecond, func() (bool, error) {
		result, err := guestRun(ctx, namespace, []string{"ip", "-o", "addr", "show", "dev", iface}, nil)
		if err != nil {
			return false, err
		}
		last = result.Stdout
		return strings.Contains(last, local) && strings.Contains(last, peer), nil
	})
	if err != nil {
		return fmt.Errorf("%s never got local %s peer %s within %s:\n%s", iface, local, peer, timeout, last)
	}
	return nil
}

func waitPPPLinksCleared(ctx context.Context, namespace string, initial map[string]bool, timeout time.Duration) error {
	err := waitGuest(ctx, timeout, 200*time.Millisecond, func() (bool, error) {
		current, err := pppLinks(ctx, namespace)
		if err != nil {
			return false, err
		}
		if len(current) != len(initial) {
			return false, nil
		}
		for name := range current {
			if !initial[name] {
				return false, nil
			}
		}
		return true, nil
	})
	if err != nil {
		return fmt.Errorf("PPP links in %s did not return to their initial set: %w", namespace, err)
	}
	return nil
}

func runPPPoEAccelGuest(ctx context.Context, root string) (report guestLabReport, runErr error) {
	report = guestLabReport{Lab: "pppoe-accel", Selected: []string{"pppoe-accel"}, Verdict: VerdictUnspecified}
	if err := rejectPPPoEProbeSkip(); err != nil {
		return report, err
	}
	if err := requireGuestCommands("ip", "ping", "accel-pppd"); err != nil {
		return report, err
	}
	if err := probePPPoEKernel(ctx); err != nil {
		return report, err
	}
	ze, err := buildGuestZe(ctx, root, "ze-pppoe-accel", guestEvidenceZeKey, zeBinKey)
	if err != nil {
		return report, err
	}
	parent := filepath.Join(root, "tmp", "evidence")
	if err := os.MkdirAll(parent, 0o750); err != nil {
		return report, fmt.Errorf("create evidence directory: %w", err)
	}
	work, err := os.MkdirTemp(parent, "effective-pppoe-accel-")
	if err != nil {
		return report, fmt.Errorf("create PPPoE evidence directory: %w", err)
	}
	var tb textbuf.Buffer
	suffix := pidSuffix("")
	zeNS := tb.Str("ze-pppoe-ze-").Str(suffix).String()
	tb.Reset()
	acNS := tb.Str("ze-pppoe-ac-").Str(suffix).String()
	tb.Reset()
	zeVeth := tb.Str("zpoez").Str(suffix).String()
	tb.Reset()
	acVeth := tb.Str("zpoea").Str(suffix).String()
	cleanupNamespaces(context.Background(), []string{zeNS, acNS}, []string{zeVeth, acVeth}, nil)

	chapPath := filepath.Join(work, "chap-secrets")
	tb.Reset()
	chap := tb.Str(pppoeUsername).Str("\t*\t").Str(pppoePassword).Str("\t*\n").Bytes()
	if err := os.WriteFile(chapPath, chap, 0o600); err != nil {
		return report, fmt.Errorf("write chap-secrets: %w", err)
	}
	accelConfigPath := filepath.Join(work, "accel-ppp.conf")
	if err := os.WriteFile(accelConfigPath, pppoeAccelConfig(work, acVeth), 0o600); err != nil {
		return report, fmt.Errorf("write accel-ppp config: %w", err)
	}
	zeConfigPath := filepath.Join(work, "ze.conf")
	if err := os.WriteFile(zeConfigPath, pppoeZeConfig(zeVeth), 0o600); err != nil {
		return report, fmt.Errorf("write ze config: %w", err)
	}

	var zeProcess, accelProcess *guestProcess
	success := false
	defer func() {
		if zeProcess != nil {
			zeProcess.stop()
			report.Cleanup = append(report.Cleanup, "process:ze")
		}
		if accelProcess != nil {
			accelProcess.stop()
			report.Cleanup = append(report.Cleanup, "process:accel-pppd")
		}
		cleanupNamespaces(context.Background(), []string{zeNS, acNS}, []string{zeVeth, acVeth}, &report.Cleanup)
		if success {
			_ = os.RemoveAll(work)
		} else {
			report.Artifacts = []string{work, accelConfigPath, zeConfigPath, filepath.Join(work, "accel.log")}
		}
	}()

	failure := func(err error) (guestLabReport, error) {
		report.Verdict = VerdictFail
		report.Failure = err.Error()
		fmt.Fprintf(os.Stderr, "FAIL: %s\n\n--- diagnostics ---\n", err) //nolint:errcheck // evidence diagnostics
		for _, namespace := range []string{zeNS, acNS} {
			links, _ := guestRun(context.Background(), namespace, []string{"ip", "-o", "link", "show", "type", "ppp"}, nil)
			text := strings.TrimSpace(links.Stdout)
			if text == "" {
				text = "(none)"
			}
			fmt.Fprintf(os.Stderr, "%s ppp links: %s\n", namespace, text) //nolint:errcheck // evidence diagnostics
		}
		// #nosec G304 -- work is a fixture root created above with os.MkdirTemp.
		if data, readErr := os.ReadFile(filepath.Join(work, "accel.log")); readErr == nil {
			lines := strings.SplitAfter(string(data), "\n")
			if len(lines) != 0 && lines[len(lines)-1] == "" {
				lines = lines[:len(lines)-1]
			}
			if len(lines) > 40 {
				lines = lines[len(lines)-40:]
			}
			fmt.Fprint(os.Stderr, "\naccel.log tail:\n", strings.Join(lines, "")) //nolint:errcheck // evidence diagnostics
		}
		return report, nil
	}

	if err := setupPPPoENamespaces(ctx, zeNS, acNS, zeVeth, acVeth); err != nil {
		return failure(err)
	}
	initialZe, err := pppLinks(ctx, zeNS)
	if err != nil {
		return failure(err)
	}
	initialAC, err := pppLinks(ctx, acNS)
	if err != nil {
		return failure(err)
	}
	accelProcess, _, err = startGuestProcess(ctx, acNS,
		[]string{"accel-pppd", "-c", accelConfigPath, "-p", filepath.Join(work, "accel.pid")}, os.Environ(), "accel> ")
	if err != nil {
		return failure(err)
	}
	select {
	case <-ctx.Done():
		return report, ctx.Err()
	case <-time.After(pppoeAccelStartupWait):
	}
	if accelProcess.exited() {
		return failure(errors.New("accel-pppd exited during startup"))
	}
	environ := withoutGuestEnv(os.Environ(), "ZE_PPPOE_SKIP_KERNEL_PROBE", "ze.pppoe.skip-kernel-probe")
	environ = withGuestEnv(environ, map[string]string{
		"ze.log.interface":  "debug",
		guestStorageBlobKey: "false",
		guestConfigDirKey:   filepath.Join(work, "ze"),
	})
	zeProcess, _, err = startGuestProcess(ctx, zeNS, []string{ze, "start", zeConfigPath}, environ, "ze> ")
	if err != nil {
		return failure(err)
	}
	zeIface, err := waitNewPPP(ctx, zeNS, "Ze", initialZe, zeProcess, pppoeZeSessionTimeout)
	if err != nil {
		return failure(err)
	}
	acIface, err := waitNewPPP(ctx, acNS, "accel-ppp", initialAC, accelProcess, pppoeACSessionTimeout)
	if err != nil {
		return failure(err)
	}
	if err := waitPPPAddress(ctx, zeNS, zeIface, pppoeLocalAddress, pppoePeerAddress, pppoeAddressTimeout); err != nil {
		return failure(err)
	}
	if err := waitPPPAddress(ctx, acNS, acIface, pppoePeerAddress, pppoeLocalAddress, pppoeAddressTimeout); err != nil {
		return failure(err)
	}
	ping, err := guestRun(ctx, zeNS, []string{"ping", "-c", "2", "-W", "3", pppoePeerAddress}, nil)
	if err != nil {
		return failure(err)
	}
	if ping.Code != 0 {
		return failure(fmt.Errorf("dataplane ping to AC gateway %s through %s failed", pppoePeerAddress, zeIface))
	}
	zeProcess.stop()
	zeProcess = nil
	report.Cleanup = append(report.Cleanup, "process:ze")
	if err := waitPPPLinksCleared(ctx, zeNS, initialZe, pppoeTeardownTimeout); err != nil {
		return failure(err)
	}
	if err := waitPPPLinksCleared(ctx, acNS, initialAC, pppoeTeardownTimeout); err != nil {
		return failure(err)
	}
	tb.Reset()
	detail := tb.Str("OK: ze pppoe-client completed discovery, CHAP, IPCP against accel-ppp; Ze ").
		Str(zeIface).Str(" (").Str(pppoeLocalAddress).Str(") and accel ").Str(acIface).
		Str(" (").Str(pppoePeerAddress).Str(") up, dataplane ping ok, clean teardown").String()
	report.Scenarios = []GuestScenario{{Name: "pppoe-accel", Verdict: VerdictPass, Details: []string{detail}}}
	report.Verdict = VerdictPass
	success = true
	return report, nil
}
