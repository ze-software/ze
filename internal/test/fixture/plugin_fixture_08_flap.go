package fixture

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const (
	flapDevice08      = "zeflapv0"
	flapPeer08        = "zeflapv1"
	flapGateway08     = "fe80::f1a9"
	flapBaseMetric08  = 254
	flapDownMetric08  = 1278
	flapTransitions08 = 101
	flapRounds08      = 6
)

func init() {
	Register("plugin/iface-link-flap-during-commit", func(ctx context.Context, args []string) error {
		if len(args) != 1 {
			return fmt.Errorf("usage: plugin/iface-link-flap-during-commit METRICS_PORT")
		}
		return Observe(ctx, "iface-link-flap-during-commit", sdk.Registration{}, func(ctx context.Context, p *sdk.Plugin) error {
			return ifaceLinkFlap08(ctx, p, args[0])
		})
	})
}

func readTrimmed08(path string) (string, bool) {
	data, err := os.ReadFile(path) //nolint:gosec // the path is the fixture's own scratch file
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(string(data)), true
}

func flapCarrier08() (string, bool) {
	return readTrimmed08("/sys/class/net/" + flapDevice08 + "/carrier")
}

var metricRE08 = regexp.MustCompile(`\bmetric (\d+)`)

func flapDefaultMetric08(ctx context.Context) (int, bool) {
	stdout, _, _ := runCommand08(ctx, false, "ip", "-6", "route", "show", "default", "dev", flapDevice08)
	match := metricRE08.FindStringSubmatch(stdout)
	if len(match) != 2 {
		return 0, false
	}
	value, err := strconv.Atoi(match[1])
	return value, err == nil
}

func netlinkDrops08() int {
	data, err := os.ReadFile("/proc/net/netlink")
	if err != nil {
		return -1
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) == 0 {
		return -1
	}
	header := strings.Fields(lines[0])
	column := -1
	for i, name := range header {
		if name == "Drops" {
			column = i
		}
	}
	if column < 0 {
		return -1
	}
	total := 0
	for _, line := range lines[1:] {
		fields := strings.Fields(line)
		if len(fields) <= column {
			continue
		}
		value, err := strconv.Atoi(fields[column])
		if err == nil {
			total += value
		}
	}
	return total
}

func scrape08(ctx context.Context, port string) (string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://127.0.0.1:"+port+"/metrics", http.NoBody)
	if err != nil {
		return "", err
	}
	client := http.Client{Timeout: 5 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close() //nolint:errcheck // the body is read
	data, err := io.ReadAll(response.Body)
	return string(data), err
}

func namedCounter08(body, metric, name string) float64 {
	pattern := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(metric) + `\{[^}]*name="` + regexp.QuoteMeta(name) + `"[^}]*\} ([0-9.e+\-]+)$`)
	match := pattern.FindStringSubmatch(body)
	if len(match) != 2 {
		return 0
	}
	value, _ := strconv.ParseFloat(match[1], 64)
	return value
}

func flapCounter08(ctx context.Context, port, metric string) (float64, error) {
	body, err := scrape08(ctx, port)
	if err != nil {
		return 0, err
	}
	return namedCounter08(body, metric, flapDevice08), nil
}

func flapCommit08(pid int) error {
	data, err := os.ReadFile("ze-bgp.conf")
	if err != nil {
		return err
	}
	text := strings.ReplaceAll(string(data), "hostname flap-a", "hostname flap-Z")
	text = strings.ReplaceAll(text, "hostname flap-b", "hostname flap-a")
	text = strings.ReplaceAll(text, "hostname flap-Z", "hostname flap-b")
	if err := os.WriteFile("ze-bgp.conf", []byte(text), 0o600); err != nil {
		return err
	}
	return syscall.Kill(pid, syscall.SIGHUP)
}

func ifaceLinkFlap08(ctx context.Context, _ *sdk.Plugin, port string) error {
	defer runCommand08(context.Background(), false, "ip", "-6", "neigh", "del", flapGateway08, "dev", flapDevice08) //nolint:errcheck // best-effort cleanup
	defer runCommand08(context.Background(), false, "ip", "link", "set", flapPeer08, "up")                          //nolint:errcheck // best-effort cleanup
	if !Poll(ctx, 1000, 20*time.Millisecond, func() bool {
		_, a := os.Stat("/sys/class/net/" + flapDevice08)
		_, b := os.Stat("/sys/class/net/" + flapPeer08)
		return a == nil && b == nil
	}) {
		return fmt.Errorf("timed out waiting for %s and %s to exist", flapDevice08, flapPeer08)
	}
	for _, args := range [][]string{{ipObjectLink, ipWordSet, flapDevice08, "up"}, {ipObjectLink, ipWordSet, flapPeer08, "up"}} {
		if _, _, err := runCommand08(ctx, true, "ip", args...); err != nil {
			return err
		}
	}
	if !Poll(ctx, 1000, 20*time.Millisecond, func() bool { carrier, ok := flapCarrier08(); return ok && carrier == "1" }) {
		return fmt.Errorf("timed out waiting for %s carrier to come up", flapDevice08)
	}
	if _, _, err := runCommand08(ctx, true, "ip", "-6", "neigh", "replace", flapGateway08, "lladdr", "02:00:00:00:f1:a9", "dev", flapDevice08, "router", "nud", "permanent"); err != nil {
		return err
	}
	if !Poll(ctx, 1000, 20*time.Millisecond, func() bool { metric, ok := flapDefaultMetric08(ctx); return ok && metric == flapBaseMetric08 }) {
		return fmt.Errorf("timed out waiting for default route at metric %d", flapBaseMetric08)
	}
	var batch strings.Builder
	for i := range flapTransitions08 {
		state := "up"
		if i%2 == 0 {
			state = linkStateDown
		}
		fmt.Fprintf(&batch, "link set %s %s\n", flapPeer08, state)
	}
	if err := os.WriteFile("flap.batch", []byte(batch.String()), 0o600); err != nil {
		return err
	}
	pidText, ok := readTrimmed08("daemon.pid")
	if !ok {
		return fmt.Errorf("daemon.pid unavailable")
	}
	pid, err := strconv.Atoi(pidText)
	if err != nil {
		return err
	}
	dropsBefore := netlinkDrops08()
	resyncsBefore, err := flapCounter08(ctx, port, "ze_iface_carrier_resyncs_total")
	if err != nil {
		return err
	}
	stalled := 0
	pressures := make([]float64, 0, flapRounds08)
	for round := range flapRounds08 {
		pressureBefore, err := flapCounter08(ctx, port, "ze_iface_link_events_coalesced_total")
		if err != nil {
			return err
		}
		if err := flapCommit08(pid); err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
		if _, _, err := runCommand08(ctx, true, "ip", "-force", "-batch", "flap.batch"); err != nil {
			return err
		}
		if metric, ok := flapDefaultMetric08(ctx); ok && metric == flapBaseMetric08 {
			stalled++
		}
		if !Poll(ctx, 750, 20*time.Millisecond, func() bool { metric, ok := flapDefaultMetric08(ctx); return ok && metric == flapDownMetric08 }) {
			return fmt.Errorf("round %d: default route did not reach metric %d", round, flapDownMetric08)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(1600 * time.Millisecond):
		}
		pressureNow, err := flapCounter08(ctx, port, "ze_iface_link_events_coalesced_total")
		if err != nil {
			return err
		}
		pressure := pressureNow - pressureBefore
		pressures = append(pressures, pressure)
		if pressure <= 0 {
			return fmt.Errorf("round %d: coalesced counter did not move; burst never outran worker", round)
		}
		carrier, _ := flapCarrier08()
		metric, _ := flapDefaultMetric08(ctx)
		if carrier != "0" || metric != flapDownMetric08 {
			return fmt.Errorf("round %d: carrier=%q metric=%d, want carrier 0 metric %d", round, carrier, metric, flapDownMetric08)
		}
		if _, _, err := runCommand08(ctx, true, "ip", "link", "set", flapPeer08, "up"); err != nil {
			return err
		}
		if !Poll(ctx, 1000, 20*time.Millisecond, func() bool { value, ok := flapCarrier08(); return ok && value == "1" }) {
			return fmt.Errorf("round %d: carrier did not return up", round)
		}
		if !Poll(ctx, 1000, 20*time.Millisecond, func() bool { value, ok := flapDefaultMetric08(ctx); return ok && value == flapBaseMetric08 }) {
			return fmt.Errorf("round %d: route did not return to metric %d", round, flapBaseMetric08)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(1600 * time.Millisecond):
		}
	}
	if dropped := netlinkDrops08() - dropsBefore; dropped != 0 {
		return fmt.Errorf("kernel dropped %d netlink notifications during this run", dropped)
	}
	resyncsNow, err := flapCounter08(ctx, port, "ze_iface_carrier_resyncs_total")
	if err != nil {
		return err
	}
	if resyncs := resyncsNow - resyncsBefore; resyncs != 0 {
		return fmt.Errorf("carrier resync counter rose by %g, want 0: event path lost a transition", resyncs)
	}
	if stalled < flapRounds08/2 {
		return fmt.Errorf("only %d of %d rounds found worker blocked; commit no longer stalls it", stalled, flapRounds08)
	}
	fmt.Fprintf(os.Stderr, "OK: %d rounds of %d transitions during commits, stalled=%d, pressure=%v, resyncs=0\n", flapRounds08, flapTransitions08, stalled, pressures)
	return nil
}
