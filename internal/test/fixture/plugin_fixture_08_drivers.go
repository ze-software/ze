package fixture

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func init() {
	Register("plugin/flowspec-metrics-registered", flowspecMetrics08)
	registerPlugin08("plugin/forked-route-install-kernel", "forked-route", forkedRouteKernel08)
	registerPlugin08("plugin/forked-route-install", "forked-route", forkedRoute08)
	Register("plugin/hub-external-plugin-acceptor", hubExternalAcceptor08)
}

func flowspecMetrics08(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: plugin/flowspec-metrics-registered PORT")
	}
	url := "http://127.0.0.1:" + args[0] + "/metrics"
	var body string
	client := http.Client{Timeout: 2 * time.Second}
	fetch := func() bool {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
		if err != nil {
			body = ""
			return false
		}
		response, err := client.Do(request)
		if err != nil {
			body = ""
			return false
		}
		defer response.Body.Close() //nolint:errcheck // the body is read
		if response.StatusCode != http.StatusOK {
			body = ""
			return false
		}
		data, err := io.ReadAll(response.Body)
		if err != nil {
			body = ""
			return false
		}
		body = string(data)
		return true
	}
	if !Poll(ctx, 400, 50*time.Millisecond, fetch) {
		return fmt.Errorf("the Prometheus endpoint on 127.0.0.1:%s never answered", args[0])
	}
	var absent []string
	for _, family := range []string{"go_goroutines", "process_start_time_seconds"} {
		if !strings.Contains(body, family) {
			absent = append(absent, family)
		}
	}
	if len(absent) != 0 {
		return fmt.Errorf("the serving registry is missing runtime families %v; it is not the populated one", absent)
	}
	fmt.Printf("CONTROL: serving registry populated, %d bytes\n", len(body))
	series := `ze_flowspec_rules_refused_total{reason="unknown-protocol"}`
	ok := Poll(ctx, 200, 100*time.Millisecond, func() bool { return fetch() && strings.Contains(body, series) })
	for line := range strings.SplitSeq(body, "\n") {
		if strings.HasPrefix(line, "ze_flowspec") {
			fmt.Println(line)
		}
	}
	if !ok {
		return fmt.Errorf("%s absent from a %d-byte scrape", series, len(body))
	}
	fmt.Println("flowspec refusal metric exposed")
	return nil
}

func runCommand08(ctx context.Context, check bool, name string, args ...string) (string, string, error) {
	cmd := exec.CommandContext(ctx, name, args...) //nolint:gosec // the fixture chooses the program and its arguments
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if check && err != nil {
		return stdout.String(), stderr.String(), fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), stderr.String(), err
}

func kernelHasRoute08(ctx context.Context, prefix string) bool {
	stdout, _, _ := runCommand08(ctx, false, "ip", "route", "show", prefix)
	return strings.Contains(stdout, prefix) && (strings.Contains(stdout, "proto ze") || strings.Contains(stdout, "proto 250"))
}

func routeEntry08(protocol string) rpc.RouteInstallEntry {
	return rpc.RouteInstallEntry{Protocol: protocol, AFI: 1, SAFI: 1, Prefix: "10.99.0.0/24", NextHop: addrTestNet1First, AdminDistance: 110, Metric: 10}
}

func routeRemove08() rpc.RouteRemoveEntry {
	return rpc.RouteRemoveEntry{Protocol: namespaceBGP, AFI: 1, SAFI: 1, Prefix: "10.99.0.0/24", Instance: 0}
}

func forkedRouteKernel08(ctx context.Context, p *sdk.Plugin) error {
	_, _, _ = runCommand08(ctx, false, "ip", "link", "del", "ze-fri0")
	if _, _, err := runCommand08(ctx, true, "ip", "link", "add", "ze-fri0", "type", "dummy"); err != nil {
		return err
	}
	defer runCommand08(context.Background(), false, "ip", "link", "del", "ze-fri0") //nolint:errcheck // best-effort cleanup
	if _, _, err := runCommand08(ctx, true, "ip", "addr", "add", "192.0.2.254/24", "dev", "ze-fri0"); err != nil {
		return err
	}
	if _, _, err := runCommand08(ctx, true, "ip", "link", "set", "ze-fri0", "up"); err != nil {
		return err
	}
	installed, err := p.RouteInstall(ctx, []rpc.RouteInstallEntry{routeEntry08("bgp")})
	if err != nil || installed != 1 {
		return fmt.Errorf("route-install: expected installed=1, got %d: %w", installed, err)
	}
	if !Poll(ctx, 40, 250*time.Millisecond, func() bool { return kernelHasRoute08(ctx, "10.99.0.0/24") }) {
		got, _, _ := runCommand08(ctx, false, "ip", "route", "show", "10.99.0.0/24")
		return fmt.Errorf("route-install: 10.99.0.0/24 not in kernel as RTPROT_ZE; ip route showed: %q", strings.TrimSpace(got))
	}
	fmt.Fprintln(os.Stderr, "OK: forked route-install produced an RTPROT_ZE kernel route")
	removed, err := p.RouteRemove(ctx, []rpc.RouteRemoveEntry{routeRemove08()})
	if err != nil || removed != 1 {
		return fmt.Errorf("route-remove: expected removed=1, got %d: %w", removed, err)
	}
	if !Poll(ctx, 40, 250*time.Millisecond, func() bool { return !kernelHasRoute08(ctx, "10.99.0.0/24") }) {
		return fmt.Errorf("route-remove: 10.99.0.0/24 still in kernel after withdrawal")
	}
	fmt.Fprintln(os.Stderr, "OK: route-remove swept the forked route from the kernel")
	return nil
}

func ribHas08(ctx context.Context, p *sdk.Plugin, prefix string) bool {
	status, raw, err := command08(ctx, p, "show rib")
	if err != nil || status != statusDone {
		return false
	}
	rows, err := list08(raw)
	if err != nil {
		return false
	}
	for _, row := range rows {
		if row["prefix"] == prefix {
			return true
		}
	}
	return false
}

func forkedRoute08(ctx context.Context, p *sdk.Plugin) error {
	installed, err := p.RouteInstall(ctx, []rpc.RouteInstallEntry{routeEntry08("bgp")})
	if err != nil || installed != 1 {
		return fmt.Errorf("route-install: expected installed=1, got %d: %w", installed, err)
	}
	if !Poll(ctx, 20, 250*time.Millisecond, func() bool { return ribHas08(ctx, p, "10.99.0.0/24") }) {
		return fmt.Errorf("route-install: 10.99.0.0/24 never reached sysrib (`show rib`)")
	}
	removed, err := p.RouteRemove(ctx, []rpc.RouteRemoveEntry{routeRemove08()})
	if err != nil || removed != 1 {
		return fmt.Errorf("route-remove: expected removed=1, got %d: %w", removed, err)
	}
	if !Poll(ctx, 20, 250*time.Millisecond, func() bool { return !ribHas08(ctx, p, "10.99.0.0/24") }) {
		return fmt.Errorf("route-remove: 10.99.0.0/24 still in sysrib after withdrawal")
	}
	bad := routeEntry08("")
	bad.Prefix = "10.98.0.0/24"
	if _, err := p.RouteInstall(ctx, []rpc.RouteInstallEntry{bad}); err == nil {
		return fmt.Errorf("route-install with an empty protocol name should have errored")
	}
	fmt.Fprintln(os.Stderr, "OK: forked route-install reached sysrib, route-remove withdrew it, bad input rejected")
	return nil
}

func hubExternalAcceptor08(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("unexpected arguments: %v", args)
	}
	p, err := newObserver("hub-acceptor-probe")
	if err != nil {
		return err
	}
	defer p.Close() //nolint:errcheck // fixture teardown
	p.OnStarted(func(context.Context) error {
		fmt.Fprintln(os.Stderr, "hub-external-plugin reached stage running")
		return nil
	})
	err = p.Run(ctx, sdk.Registration{})
	if ctx.Err() != nil {
		return nil
	}
	return err
}
