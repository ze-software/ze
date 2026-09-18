package fixture

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func fibRIBEvent07(ctx context.Context, p *sdk.Plugin) error {
	if status := done07(ctx, p, "show bgp rib status").status; status != statusDone {
		return fmt.Errorf("rib plugin did not become ready: status=%s", status)
	}
	if result := command07(ctx, p, "request bgp rib inject 10.0.0.1 ipv4/unicast 10.10.0.0/24 origin igp"); result.status != statusDone {
		return fmt.Errorf("inject-route: status=%s error=%w", result.status, result.err)
	}
	result := command07(ctx, p, "show bgp rib best")
	if result.status != statusDone {
		return fmt.Errorf("show-best: status=%s error=%w", result.status, result.err)
	}
	found := false
	for _, row := range array07(object07(result.data)["best-path"]) {
		found = found || object07(row)["prefix"] == "10.10.0.0/24"
	}
	if !found {
		return fmt.Errorf("best-path-visible: 10.10.0.0/24 not in output: %s", text07(result.data))
	}
	return waitEOR07(ctx, p, 1)
}

func fibSRv6Kernel07(ctx context.Context, p *sdk.Plugin) error {
	commands := []string{
		"request fakefib emit add ipv6/unicast 2001:db8:100::/48 nexthop fc00::1 srv6-sid 2001:db8::1",
		"request fakefib emit withdraw ipv6/unicast 2001:db8:100::/48",
		"request fakefib emit add ipv4/unicast 10.0.0.0/24 nexthop 192.168.1.1",
	}
	for _, command := range commands {
		result := command07(ctx, p, command)
		if result.status != statusDone {
			return fmt.Errorf("%s: status=%s error=%w", command, result.status, result.err)
		}
		timer := time.NewTimer(time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	return waitEOR07(ctx, p, 1)
}

func fibSysRIB07(ctx context.Context, p *sdk.Plugin) error {
	if status := done07(ctx, p, "show bgp rib status").status; status != statusDone {
		return fmt.Errorf("rib plugin did not become ready: status=%s", status)
	}
	if result := command07(ctx, p, "request bgp rib inject 10.0.0.1 ipv4/unicast 10.20.0.0/24 origin igp"); result.status != statusDone {
		return fmt.Errorf("inject-route: status=%s error=%w", result.status, result.err)
	}
	result := until07(ctx, p, "show rib", 20, 250*time.Millisecond, func(result commandResult07) bool {
		for _, row := range array07(result.data) {
			if object07(row)["prefix"] == prefixTenTwenty {
				return result.status == statusDone
			}
		}
		return false
	})
	for _, row := range array07(result.data) {
		if object07(row)["prefix"] == prefixTenTwenty {
			return waitEOR07(ctx, p, 1)
		}
	}
	return fmt.Errorf("10.20.0.0/24 not in system RIB: %s", text07(result.data))
}

func fibTable07(ctx context.Context, p *sdk.Plugin) error {
	result := command07(ctx, p, "request fakefib emit add ipv4/unicast 10.0.0.0/24 nexthop 192.168.1.1 tableid 100")
	if result.status != statusDone {
		return fmt.Errorf("fakefib table-id emit: status=%s error=%w", result.status, result.err)
	}
	result = until07(ctx, p, "show metrics values", 40, 250*time.Millisecond, func(result commandResult07) bool {
		return result.status == statusDone && fibTableProcessed07(metricsText07(result.data))
	})
	if result.status != statusDone || !fibTableProcessed07(metricsText07(result.data)) {
		return fmt.Errorf("fib-kernel did not report table-id route processing: status=%s error=%w data=%s", result.status, result.err, text07(result.data))
	}
	fmt.Fprintln(os.Stderr, "OK: fib-kernel processed the table-id route")
	return waitEOR07(ctx, p, 1)
}

// fibTableProcessed07 reports whether fib-kernel accounted for the table-id
// route the scenario emitted. A backend that programs rich routes counts an
// install, and a backend that refuses them counts an add error, so the two
// counters are the same fact on two platforms: the sysrib best-change reached
// processEvent. The counters are read as values because a retry can take
// either above one.
func fibTableProcessed07(metrics string) bool {
	if installs, ok := counter07(metrics, "ze_fibkernel_route_installs_total"); ok && installs >= 1 {
		return true
	}
	failures, ok := counter07(metrics, `ze_fibkernel_errors_total{operation="add"}`)
	return ok && failures >= 1
}

const fibVPPConfig07 = `environment {
}

bgp {
	router-id 10.0.0.1;
	session { asn { local 65533; } }
}

vpp {
	enabled false;
}

fib {
%s	vpp {
		enabled true;
	}
}
`

func runZePatterns07(ctx context.Context, config string, env, required []string) error {
	cmd := exec.CommandContext(ctx, "ze", "-")
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdin = strings.NewReader(config)
	stderr, stderrWriter, err := os.Pipe()
	if err != nil {
		return err
	}
	cmd.Stderr = stderrWriter
	if err := cmd.Start(); err != nil {
		stderr.Close()       //nolint:errcheck // the start failure below is the error worth reporting
		stderrWriter.Close() //nolint:errcheck // the start failure below is the error worth reporting
		return err
	}
	stderrWriter.Close() //nolint:errcheck // child owns its duplicate
	defer stderr.Close() //nolint:errcheck // the fixture is exiting, so a close failure changes no assertion

	observed := make(chan string, len(required))
	scanErr := make(chan error, 1)
	go func() {
		// Each pattern is reported ONCE. A pattern can match many lines --
		// subsystem=fib.kernel matches most of them -- and the send used to
		// drop on a full channel, so duplicates of one pattern filled the
		// buffer and a DIFFERENT pattern's only line was discarded in silence.
		// The reader then hit its deadline and reported that pattern missing,
		// which is how a log line that did arrive was read as one that never
		// came. Reporting once bounds the sends by len(required), which is the
		// buffer, so the send never blocks and nothing is dropped.
		sent := make(map[string]bool, len(required))
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			fmt.Fprintln(os.Stderr, line)
			for _, pattern := range required {
				if sent[pattern] || !strings.Contains(line, pattern) {
					continue
				}
				sent[pattern] = true
				observed <- pattern
			}
		}
		scanErr <- scanner.Err()
	}()
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()

	seen := make(map[string]bool, len(required))
	deadline := time.NewTimer(10 * time.Second)
	defer deadline.Stop()
	for len(seen) < len(required) {
		select {
		case <-ctx.Done():
			_ = cmd.Process.Kill()
			<-exited
			return ctx.Err()
		case <-deadline.C:
			_ = cmd.Process.Signal(syscall.SIGTERM)
			<-exited
			return fmt.Errorf("missing required lifecycle patterns: %v", missing07(required, seen))
		case err := <-exited:
			return fmt.Errorf("ze exited before required lifecycle patterns %v: %w", missing07(required, seen), err)
		case pattern := <-observed:
			seen[pattern] = true
		}
	}

	_ = cmd.Process.Signal(syscall.SIGTERM)
	select {
	case <-exited:
	case <-time.After(3 * time.Second):
		_ = cmd.Process.Kill()
		<-exited
	}
	return <-scanErr
}

func missing07(required []string, seen map[string]bool) []string {
	var missing []string
	for _, pattern := range required {
		if !seen[pattern] {
			missing = append(missing, pattern)
		}
	}
	return missing
}

func fibVPPLoad07(ctx context.Context, _ []string) error {
	return runZePatterns07(ctx, fmt.Sprintf(fibVPPConfig07, ""), []string{"ze.log.fib.vpp=debug", "ze.log.vpp=info"}, []string{"VPP connector not available, using noop backend", "fib-vpp: running"})
}

func fibVPPCoexist07(ctx context.Context, _ []string) error {
	return runZePatterns07(ctx, fmt.Sprintf(fibVPPConfig07, "\tkernel {\n\t}\n"), []string{"ze.log.fib.vpp=debug", "ze.log.fib.kernel=debug", "ze.log.vpp=info"}, []string{"VPP connector not available, using noop backend", "fib-vpp: running", "subsystem=fib.kernel"})
}
