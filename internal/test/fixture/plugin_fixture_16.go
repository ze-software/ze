package fixture

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

type plugin16Response struct {
	status string
	data   any
	err    error
}

type plugin16Scenario = ObserverScenario

type plugin16Observer struct {
	fixture string
	plugin  string
	run     plugin16Scenario
}

func plugin16AfterEOR(run plugin16Scenario) plugin16Scenario {
	return func(ctx context.Context, p *sdk.Plugin) error {
		if !plugin16WaitEOR(ctx, p, "show bgp peer * detail", 100) {
			return fmt.Errorf("ze never sent its initial-sync End-of-RIB")
		}
		return run(ctx, p)
	}
}

func init() {
	observers := []plugin16Observer{
		{"plugin/system-cpu-show", "system-cpu-show-test", plugin16AfterEOR(plugin16SystemCPU)},
		{"plugin/system-date-show", "system-date-show-test", plugin16AfterEOR(plugin16SystemDate)},
		{"plugin/system-fd-show", "system-fd-show-test", plugin16AfterEOR(plugin16SystemFD)},
		{"plugin/system-goroutines-show", "system-goroutines-show-test", plugin16AfterEOR(plugin16SystemGoroutines)},
		{"plugin/system-kernel-log-show", "system-kernel-log-show-test", plugin16SystemKernelLog},
		{"plugin/system-memory-map-show", "system-memory-map-show-test", plugin16AfterEOR(plugin16SystemMemory)},
		{"plugin/system-ntp-show", "system-ntp-show-test", plugin16AfterEOR(plugin16SystemNTP)},
		{"plugin/system-platform-show", "system-platform-show-test", plugin16AfterEOR(plugin16SystemPlatform)},
		{"plugin/system-profile-show", "system-profile-show-test", plugin16AfterEOR(plugin16SystemProfile)},
		{"plugin/system-sockets-show", "system-sockets-show-test", plugin16AfterEOR(plugin16SystemSockets)},
		{"plugin/system-update-backend-show", "system-update-backend-show", plugin16AfterEOR(plugin16SystemUpdate)},
		{"plugin/tcp-check-show", "tcp-check-show-test", plugin16AfterEOR(plugin16TCPCheck)},
		{"plugin/traceroute-show", "traceroute-show-test", plugin16AfterEOR(plugin16Traceroute)},
		{"plugin/traffic-feature-show", "traffic-feature-show-test", plugin16AfterEOR(plugin16TrafficFeature)},
		{"plugin/traffic-monitor", "traffic-monitor-done", plugin16TrafficMonitor},
		{"plugin/uptime-show", "uptime-show-test", plugin16AfterEOR(plugin16Uptime)},
		{"plugin/warnings-filter-show", "warnings-filter-show-test", plugin16WarningsFilter},
		{"plugin/warnings-show", "warnings-show-test", plugin16AfterEOR(plugin16Warnings)},
		{"plugin/test-pipe-first-last", "pipe-first-last-test", plugin16PipeFirstLast},
		{"plugin/wellknown-no-advertise-egress", "wellknown-noadv", plugin16WellknownNoAdvertise},
		{"plugin/wellknown-no-export-egress", "wellknown-noexport", plugin16WellknownNoExport},
		{"plugin/wellknown-no-export-withdraw-egress", "wellknown-withdraw", plugin16WellknownWithdraw},
		{"plugin/teardown-cmd", "teardown-test", plugin16TeardownCmd},
		{"plugin/teardown-msg", "teardown-msg-test", plugin16TeardownMsg},
		{"plugin/watchdog-med-override", "service-watchdog", plugin16AfterEOR(plugin16WatchdogMED)},
		{"plugin/wire-edit-api-origin-order", "wire-edit-api-origin-order", plugin16WireEditOriginOrder},
	}
	for _, observer := range observers {
		o := observer
		Register(o.fixture, func(ctx context.Context, _ []string) error {
			return Observe(ctx, o.plugin, sdk.Registration{}, o.run)
		})
	}
}

func plugin16Dispatch(ctx context.Context, p *sdk.Plugin, command string) plugin16Response {
	var raw json.RawMessage
	status, err := Dispatch(ctx, p, command, &raw)
	response := plugin16Response{status: status, err: err}
	if len(raw) == 0 {
		return response
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if decodeErr := decoder.Decode(&response.data); decodeErr != nil {
		response.err = fmt.Errorf("decode %q: %w", command, decodeErr)
		return response
	}
	if text, ok := response.data.(string); ok && text != "" {
		decoder = json.NewDecoder(strings.NewReader(text))
		decoder.UseNumber()
		var nested any
		if decoder.Decode(&nested) == nil {
			response.data = nested
		}
	}
	return response
}

func (r plugin16Response) text() string {
	if r.err != nil {
		return r.err.Error()
	}
	if text, ok := r.data.(string); ok {
		return text
	}
	encoded, _ := json.Marshal(r.data)
	return string(encoded)
}

func plugin16DoneMap(ctx context.Context, p *sdk.Plugin, command, label string) (map[string]any, error) {
	r := plugin16Dispatch(ctx, p, command)
	if r.err != nil || r.status != statusDone {
		return nil, fmt.Errorf("%s: status=%s data=%s", label, r.status, r.text())
	}
	data, ok := r.data.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s: expected object, got %T: %v", label, r.data, r.data)
	}
	return data, nil
}

func plugin16Number(value any) (int64, bool) {
	n, ok := value.(json.Number)
	if !ok {
		return 0, false
	}
	v, err := n.Int64()
	return v, err == nil
}

func plugin16RequireNumber(data map[string]any, key string, minimum int64, label string) (int64, error) {
	value, exists := data[key]
	if !exists {
		return 0, fmt.Errorf("%s: missing %q", label, key)
	}
	n, ok := plugin16Number(value)
	if !ok {
		return 0, fmt.Errorf("%s: %q=%v is not an integer", label, key, value)
	}
	if n < minimum {
		return 0, fmt.Errorf("%s: %q=%d is below minimum %d", label, key, n, minimum)
	}
	return n, nil
}

func plugin16SystemCPU(ctx context.Context, p *sdk.Plugin) error {
	data, err := plugin16DoneMap(ctx, p, "show system cpu", "system-cpu")
	if err != nil {
		return err
	}
	numCPU, err := plugin16RequireNumber(data, "num-cpu", 1, "system-cpu")
	if err != nil {
		return err
	}
	if _, err = plugin16RequireNumber(data, "num-goroutines", 1, "system-cpu"); err != nil {
		return err
	}
	if _, err = plugin16RequireNumber(data, "max-procs", 1, "system-cpu"); err != nil {
		return err
	}
	goVersion, ok := data["go-version"].(string)
	if !ok || !strings.HasPrefix(goVersion, "go") {
		return fmt.Errorf("system-cpu: go-version=%v is not a Go version", data["go-version"])
	}
	if failure, exists := data["hardware-error"]; exists {
		return fmt.Errorf("system-cpu: hardware detection failed: %v", failure)
	}
	hardware, ok := data["hardware"].(map[string]any)
	if !ok {
		return fmt.Errorf("system-cpu: hardware is %T, not an object", data["hardware"])
	}
	if _, exists := hardware["vendor"]; !exists {
		return fmt.Errorf("system-cpu: hardware missing vendor")
	}
	logical, err := plugin16RequireNumber(hardware, "logical-cpus", 1, "system-cpu hardware")
	if err != nil {
		return err
	}
	if logical < numCPU {
		return fmt.Errorf("system-cpu: hardware logical-cpus=%d is below runtime num-cpu=%d", logical, numCPU)
	}
	fmt.Fprintf(os.Stderr, "OK: show system cpu num-cpu=%d logical-cpus=%d go-version=%s\n", numCPU, logical, goVersion)
	return nil
}

func plugin16SystemDate(ctx context.Context, p *sdk.Plugin) error {
	before := time.Now()
	data, err := plugin16DoneMap(ctx, p, "show system date", "system-date")
	after := time.Now()
	if err != nil {
		return err
	}
	unix, err := plugin16RequireNumber(data, "unix", -1<<62, "system-date")
	if err != nil {
		return err
	}
	unixNano, err := plugin16RequireNumber(data, "unix-nano", -1<<62, "system-date")
	if err != nil {
		return err
	}
	offset, err := plugin16RequireNumber(data, "utc-offset-secs", -1<<62, "system-date")
	if err != nil {
		return err
	}
	if unixNano/1_000_000_000 != unix {
		return fmt.Errorf("system-date: unix-nano=%d truncates to %d, but unix=%d", unixNano, unixNano/1_000_000_000, unix)
	}
	zone, ok := data["timezone"].(string)
	if !ok || zone == "" {
		return fmt.Errorf("system-date: timezone=%v is not a zone name", data["timezone"])
	}
	rawTime, ok := data["time"].(string)
	if !ok || rawTime == "" {
		return fmt.Errorf("system-date: time=%v is not an RFC3339 string", data["time"])
	}
	parsed, err := time.Parse(time.RFC3339Nano, rawTime)
	if err != nil {
		return fmt.Errorf("system-date: time=%q does not parse as RFC3339: %w", rawTime, err)
	}
	_, parsedOffset := parsed.Zone()
	if int64(parsedOffset) != offset {
		return fmt.Errorf("system-date: time offset %d disagrees with utc-offset-secs=%d", parsedOffset, offset)
	}
	if delta := parsed.Unix() - unix; delta < -1 || delta > 1 {
		return fmt.Errorf("system-date: time=%s is %d epoch, but unix=%d", rawTime, parsed.Unix(), unix)
	}
	window := 300 * time.Second
	instant := time.Unix(unix, 0)
	if instant.Before(before.Add(-window)) || instant.After(after.Add(window)) {
		return fmt.Errorf("system-date: unix=%d is outside observer window", unix)
	}
	fmt.Fprintf(os.Stderr, "OK: show system date unix=%d timezone=%s offset=%d\n", unix, zone, offset)
	return nil
}

func plugin16SystemFD(ctx context.Context, p *sdk.Plugin) error {
	r := plugin16Dispatch(ctx, p, "show system file-descriptors summary")
	if strings.Contains(r.text(), "not available on this platform") {
		fmt.Fprintln(os.Stderr, "OK: file-descriptors not available on this platform (expected on non-Linux)")
		return nil
	}
	if r.err != nil || r.status != statusDone {
		return fmt.Errorf("fd: status=%s data=%s", r.status, r.text())
	}
	data, ok := r.data.(map[string]any)
	if !ok {
		return fmt.Errorf("fd: expected object, got %T", r.data)
	}
	if _, ok := data["total"]; !ok {
		return fmt.Errorf("fd: missing total in %v", data)
	}
	byType, ok := data["by-type"].(map[string]any)
	if !ok {
		return fmt.Errorf("fd: missing by-type in %v", data)
	}
	keys := make([]string, 0, len(byType))
	for key := range byType {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	fmt.Fprintf(os.Stderr, "OK: file-descriptors total=%v types=%v\n", data["total"], keys)
	return nil
}

func plugin16SystemGoroutines(ctx context.Context, p *sdk.Plugin) error {
	data, err := plugin16DoneMap(ctx, p, "show system goroutines summary", "goroutines")
	if err != nil {
		return err
	}
	total, err := plugin16RequireNumber(data, "total", 1, "goroutines")
	if err != nil {
		return err
	}
	states, ok := data["by-state"].(map[string]any)
	if !ok {
		return fmt.Errorf("goroutines: missing by-state in %v", data)
	}
	fmt.Fprintf(os.Stderr, "OK: goroutines summary total=%d states=%d\n", total, len(states))
	return nil
}

func plugin16WaitEOR(ctx context.Context, p *sdk.Plugin, command string, attempts int) bool {
	return Poll(ctx, attempts, 100*time.Millisecond, func() bool {
		r := plugin16Dispatch(ctx, p, command)
		return r.status == statusDone && plugin16HasEOR(r.data)
	})
}

func plugin16HasEOR(value any) bool {
	switch typed := value.(type) {
	case map[string]any:
		if count, ok := plugin16Number(typed["eor-sent"]); ok && count >= 1 {
			return true
		}
		for _, child := range typed {
			if plugin16HasEOR(child) {
				return true
			}
		}
	case []any:
		if slices.ContainsFunc(typed, plugin16HasEOR) {
			return true
		}
	}
	return false
}

func plugin16SystemKernelLog(ctx context.Context, p *sdk.Plugin) error {
	if !plugin16WaitEOR(ctx, p, "show bgp peer peer1 detail", 60) {
		return fmt.Errorf("peer1 never sent its initial-sync End-of-RIB")
	}
	r := plugin16Dispatch(ctx, p, "show system kernel-log count 5")
	if strings.Contains(r.text(), "not available on this platform") {
		fmt.Fprintln(os.Stderr, "OK: kernel-log not available on this platform (expected on non-Linux)")
		return nil
	}
	if strings.Contains(r.text(), "kernel log unavailable") {
		fmt.Fprintf(os.Stderr, "OK: kernel-log refused by host policy: %s\n", r.text())
		return nil
	}
	if r.err != nil || r.status != statusDone {
		return fmt.Errorf("kernel-log: status=%s data=%s", r.status, r.text())
	}
	data, ok := r.data.(map[string]any)
	if !ok {
		return fmt.Errorf("kernel-log: expected object, got %T", r.data)
	}
	if _, ok := data["entries"]; !ok {
		return fmt.Errorf("kernel-log: missing entries in %v", data)
	}
	if _, ok := data["count"]; !ok {
		return fmt.Errorf("kernel-log: missing count in %v", data)
	}
	fmt.Fprintf(os.Stderr, "OK: kernel-log returned count=%v\n", data["count"])
	return nil
}

func plugin16SystemMemory(ctx context.Context, p *sdk.Plugin) error {
	r := plugin16Dispatch(ctx, p, "show system memory")
	if strings.Contains(r.text(), "not available on this platform") {
		fmt.Fprintln(os.Stderr, "OK: memory-map not available on this platform (expected on non-Linux)")
		return nil
	}
	if r.err != nil || r.status != statusDone {
		return fmt.Errorf("memory-map: status=%s data=%s", r.status, r.text())
	}
	data, ok := r.data.(map[string]any)
	if !ok {
		return fmt.Errorf("memory-map: expected object, got %T", r.data)
	}
	fmt.Fprintf(os.Stderr, "OK: memory-map returned %d keys\n", len(data))
	return nil
}

func plugin16SystemNTP(ctx context.Context, p *sdk.Plugin) error {
	data, err := plugin16DoneMap(ctx, p, "show system ntp", "ntp")
	if err != nil {
		return err
	}
	if _, ok := data["enabled"]; !ok {
		return fmt.Errorf("ntp: missing enabled in %v", data)
	}
	peers, err := plugin16DoneMap(ctx, p, "show system ntp peers", "ntp peers")
	if err != nil {
		return err
	}
	if _, ok := peers["peers"]; !ok {
		return fmt.Errorf("ntp peers: missing peers in %v", peers)
	}
	if _, ok := peers["count"]; !ok {
		return fmt.Errorf("ntp peers: missing count in %v", peers)
	}
	fmt.Fprintf(os.Stderr, "OK: ntp enabled=%v peers-count=%v\n", data["enabled"], peers["count"])
	return nil
}

func plugin16SystemPlatform(ctx context.Context, p *sdk.Plugin) error {
	data, err := plugin16DoneMap(ctx, p, "show system platform", "platform")
	if err != nil {
		return err
	}
	platformType, ok := data["type"].(string)
	valid := map[string]bool{"gokrazy": true, "systemd": true, fieldContainer: true, "plain-linux": true, "darwin": true, valueUnknown: true}
	if !ok || !valid[platformType] {
		return fmt.Errorf("platform: type=%v is not valid", data["type"])
	}
	for _, key := range []string{"read-only-root", "reboot-allowed", "fd-limit-raisable"} {
		if _, ok := data[key]; !ok {
			return fmt.Errorf("platform: missing %q in %v", key, data)
		}
	}
	fmt.Fprintf(os.Stderr, "OK: platform type=%s reboot=%v\n", platformType, data["reboot-allowed"])
	return nil
}

func plugin16SystemProfile(ctx context.Context, p *sdk.Plugin) error {
	data, err := plugin16DoneMap(ctx, p, "show system profile heap", "profile")
	if err != nil {
		return err
	}
	if data["type"] != "heap" || data["format"] != "pprof-base64" {
		return fmt.Errorf("profile: type=%v format=%v", data["type"], data["format"])
	}
	payload, ok := data["data"].(string)
	if !ok {
		return fmt.Errorf("profile: missing data field")
	}
	fmt.Fprintf(os.Stderr, "OK: profile heap returned %d bytes base64\n", len(payload))
	return nil
}

func plugin16SystemSockets(ctx context.Context, p *sdk.Plugin) error {
	r := plugin16Dispatch(ctx, p, "show system sockets tcp")
	if strings.Contains(r.text(), "not available on this platform") {
		fmt.Fprintln(os.Stderr, "OK: sockets not available on this platform (expected on non-Linux)")
		return nil
	}
	if r.err != nil || r.status != statusDone {
		return fmt.Errorf("sockets: status=%s data=%s", r.status, r.text())
	}
	data, ok := r.data.(map[string]any)
	if !ok {
		return fmt.Errorf("sockets: expected object, got %T", r.data)
	}
	if _, ok := data["sockets"]; !ok {
		return fmt.Errorf("sockets: missing sockets in %v", data)
	}
	if _, ok := data["count"]; !ok {
		return fmt.Errorf("sockets: missing count in %v", data)
	}
	fmt.Fprintf(os.Stderr, "OK: sockets tcp returned count=%v\n", data["count"])
	return nil
}

func plugin16SystemUpdate(ctx context.Context, p *sdk.Plugin) error {
	data, err := plugin16DoneMap(ctx, p, "show system update", "system update")
	if err != nil {
		return err
	}
	if data["backend"] != "ze-self-update" {
		return fmt.Errorf("backend=%v data=%v", data["backend"], data)
	}
	return nil
}

func plugin16TCPCheck(ctx context.Context, p *sdk.Plugin) error {
	r := plugin16Dispatch(ctx, p, "show tcp-check host 127.0.0.1 port 1 timeout 2s")
	if r.status == "" && r.err == nil {
		return fmt.Errorf("tcp-check: dispatch returned no status")
	}
	if strings.Contains(r.text(), "unknown command") {
		return fmt.Errorf("tcp-check: command does not resolve: %s", r.text())
	}
	fmt.Fprintf(os.Stderr, "OK: tcp-check resolves (status=%s)\n", r.status)
	return nil
}

func plugin16Traceroute(ctx context.Context, p *sdk.Plugin) error {
	r := plugin16Dispatch(ctx, p, "show traceroute 127.0.0.1 max-hops 1 timeout 2s probes 1")
	if r.status == "" && r.err == nil {
		return fmt.Errorf("traceroute: dispatch returned no status")
	}
	if strings.Contains(r.text(), "unknown command") {
		return fmt.Errorf("traceroute: command does not resolve: %s", r.text())
	}
	fmt.Fprintf(os.Stderr, "OK: traceroute resolves (status=%s)\n", r.status)
	return nil
}

func plugin16TrafficFeature(ctx context.Context, p *sdk.Plugin) error {
	data, err := plugin16DoneMap(ctx, p, "show traffic feature", "traffic-feature")
	if err != nil {
		return err
	}
	if _, ok := data["degraded"]; !ok {
		return fmt.Errorf("traffic-feature: missing degraded in %v", data)
	}
	list, ok := data["top-source-ips"].([]any)
	if !ok {
		return fmt.Errorf("traffic-feature: top-source-ips not a list: %v", data["top-source-ips"])
	}
	fmt.Fprintf(os.Stderr, "OK: traffic-feature degraded=%v sources=%d\n", data["degraded"], len(list))
	return nil
}

func plugin16TrafficMonitor(ctx context.Context, p *sdk.Plugin) error {
	data, err := plugin16DoneMap(ctx, p, "show traffic stat", "show traffic stat")
	if err != nil {
		return err
	}
	if _, ok := data["degraded"]; !ok {
		return fmt.Errorf("show traffic stat: missing degraded key")
	}
	if _, ok := data["interfaces"].([]any); !ok {
		return fmt.Errorf("show traffic stat: interfaces is %T, not list", data["interfaces"])
	}
	return nil
}

func plugin16Uptime(ctx context.Context, p *sdk.Plugin) error {
	data, err := plugin16DoneMap(ctx, p, "show uptime", "uptime")
	if err != nil {
		return err
	}
	start, ok := data["start-time"].(string)
	if !ok || !strings.Contains(start, "T") {
		return fmt.Errorf("uptime: expected RFC3339 start-time, got %v", data["start-time"])
	}
	uptime, ok := data["uptime"].(string)
	if !ok || uptime == "" {
		return fmt.Errorf("uptime: expected non-empty uptime string, got %v", data["uptime"])
	}
	fmt.Fprintf(os.Stderr, "OK: show uptime start-time=%s uptime=%s\n", start, uptime)
	return nil
}

func plugin16Warnings(ctx context.Context, p *sdk.Plugin) error {
	data, err := plugin16DoneMap(ctx, p, "show warnings", "warnings")
	if err != nil {
		return err
	}
	count, ok := plugin16Number(data["count"])
	if !ok || count < 1 {
		return fmt.Errorf("expected at least 1 warning, got %v", data["count"])
	}
	warnings, _ := data["warnings"].([]any)
	found := false
	for _, value := range warnings {
		warning, _ := value.(map[string]any)
		if warning["source"] == namespaceBGP && warning["code"] == "prefix-stale" {
			found = true
			message, _ := warning["message"].(string)
			if !strings.Contains(message, "2024-01-01") {
				return fmt.Errorf("stale warning missing date: %v", warning)
			}
		}
	}
	if !found {
		return fmt.Errorf("no bgp/prefix-stale warning in %v", warnings)
	}
	fmt.Fprintf(os.Stderr, "OK: show warnings returned %d warning(s)\n", count)
	return nil
}

func plugin16WarningsFilter(ctx context.Context, p *sdk.Plugin) error {
	all, err := plugin16DoneMap(ctx, p, "show warnings", "warnings")
	if err != nil {
		return err
	}
	total, ok := plugin16Number(all["count"])
	if !ok || total < 1 {
		return fmt.Errorf("expected at least 1 warning, got %v", all["count"])
	}
	bgp, err := plugin16DoneMap(ctx, p, "show warnings source bgp", "warnings bgp")
	if err != nil {
		return err
	}
	bgpCount, ok := plugin16Number(bgp["count"])
	if !ok || bgpCount < 1 {
		return fmt.Errorf("expected at least 1 bgp warning, got %v", bgp["count"])
	}
	warnings, ok := bgp["warnings"].([]any)
	if !ok {
		return fmt.Errorf("warnings bgp: warnings is %T, not list", bgp["warnings"])
	}
	for _, value := range warnings {
		warning, _ := value.(map[string]any)
		if warning["source"] != namespaceBGP {
			return fmt.Errorf("non-bgp warning in filtered result: %v", warning)
		}
	}
	none, err := plugin16DoneMap(ctx, p, "show warnings source nonexistent", "warnings nonexistent")
	if err != nil {
		return err
	}
	noneCount, ok := plugin16Number(none["count"])
	if !ok || noneCount != 0 {
		return fmt.Errorf("expected 0 for nonexistent source, got %v", none["count"])
	}
	fmt.Fprintf(os.Stderr, "OK: filter works (total=%d bgp=%d nonexistent=0)\n", total, bgpCount)
	return nil
}

func plugin16PipeFirstLast(ctx context.Context, p *sdk.Plugin) error {
	var settled map[string]any
	if !Poll(ctx, 40, 100*time.Millisecond, func() bool {
		r := plugin16Dispatch(ctx, p, "show bgp rib received count")
		settled, _ = r.data.(map[string]any)
		count, ok := plugin16Number(settled["count"])
		return r.status == statusDone && ok && count == 5
	}) {
		return fmt.Errorf("adj-rib-in never reached 5 routes, last=%v", settled)
	}
	first, err := plugin16DoneMap(ctx, p, "show bgp rib received first 3", "first 3")
	if err != nil {
		return err
	}
	values, ok := first["routes"].([]any)
	if !ok {
		return fmt.Errorf("first 3 missing routes: %v", first)
	}
	prefixes := make([]string, 0, 3)
	for _, value := range values {
		row, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("first 3 carries a non-route row: %v", value)
		}
		if row["direction"] != directionReceived {
			continue
		}
		if _, ok := row["peer"].(string); !ok {
			return fmt.Errorf("first 3 route carries no peer: %v", row)
		}
		prefix, _ := row["prefix"].(string)
		prefixes = append(prefixes, prefix)
	}
	want := []string{prefixTenOne, "10.2.0.0/24", "10.3.0.0/24"}
	if !reflect.DeepEqual(prefixes, want) {
		return fmt.Errorf("first 3 returned %v, expected %v", prefixes, want)
	}
	fmt.Fprintln(os.Stderr, "OK: first 3 returned correct count")
	for _, check := range []struct {
		command string
		want    int64
		label   string
	}{{"show bgp rib received first 3 count", 3, "first 3 count"}, {"show bgp rib received last 2 count", 2, "last 2 count"}} {
		data, err := plugin16DoneMap(ctx, p, check.command, check.label)
		if err != nil {
			return err
		}
		count, ok := plugin16Number(data["count"])
		if !ok || count != check.want {
			return fmt.Errorf("%s returned %v, expected %d", check.label, data["count"], check.want)
		}
		fmt.Fprintf(os.Stderr, "OK: %s = %d\n", check.label, count)
	}
	fmt.Fprintln(os.Stderr, "OK: all pipe first/last tests passed")
	if !plugin16WaitEOR(ctx, p, "show bgp", 40) {
		return fmt.Errorf("ze never sent EOR to the peer")
	}
	fmt.Fprintln(os.Stderr, "OK: ze sent its EOR")
	return nil
}

func plugin16PeerRealUpdates(ctx context.Context, p *sdk.Plugin, peer string) (int64, bool) {
	r := plugin16Dispatch(ctx, p, "show bgp peer "+peer+" detail")
	if r.status != statusDone {
		return 0, false
	}
	return plugin16FindCounterDifference(r.data, "updates-sent", "eor-sent")
}

func plugin16FindCounterDifference(value any, left, right string) (int64, bool) {
	switch typed := value.(type) {
	case map[string]any:
		l, lok := plugin16Number(typed[left])
		r, rok := plugin16Number(typed[right])
		if lok && rok {
			return l - r, true
		}
		for _, child := range typed {
			if value, ok := plugin16FindCounterDifference(child, left, right); ok {
				return value, true
			}
		}
	case []any:
		for _, child := range typed {
			if value, ok := plugin16FindCounterDifference(child, left, right); ok {
				return value, true
			}
		}
	}
	return 0, false
}

func plugin16WellknownCounts(ctx context.Context, p *sdk.Plugin, internalWant, externalWant int64, internalMessage, externalMessage string) error {
	q := plugin16Dispatch(ctx, p, "request quiesce")
	if q.err != nil || q.status != statusDone {
		return fmt.Errorf("quiesce: status=%s data=%s", q.status, q.text())
	}
	for _, check := range []struct {
		peer string
		want int64
		msg  string
	}{{addrLoopbackSecond, internalWant, internalMessage}, {addrLoopbackThird, externalWant, externalMessage}} {
		var count int64
		if !Poll(ctx, 40, 100*time.Millisecond, func() bool {
			var ok bool
			count, ok = plugin16PeerRealUpdates(ctx, p, check.peer)
			return ok && count >= check.want
		}) {
			return fmt.Errorf("%s", check.msg)
		}
		if count != check.want {
			return fmt.Errorf("peer %s was sent %d real updates, want %d", check.peer, count, check.want)
		}
	}
	return nil
}

func plugin16WellknownNoAdvertise(ctx context.Context, p *sdk.Plugin) error {
	if err := plugin16WellknownCounts(ctx, p, 2, 1, "internal peer was not sent the subconfed route and the fence", "external peer was not sent the fence route"); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "OK: internal peer received exactly the subconfed route and the fence")
	fmt.Fprintln(os.Stderr, "OK: external peer received the fence and nothing else")
	return nil
}

func plugin16WellknownNoExport(ctx context.Context, p *sdk.Plugin) error {
	if err := plugin16WellknownCounts(ctx, p, 2, 1, "internal peer was not sent both routes", "external peer was not sent the fence route"); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "OK: internal peer received both routes")
	fmt.Fprintln(os.Stderr, "OK: external peer received the fence and nothing else")
	return nil
}

func plugin16WellknownWithdraw(ctx context.Context, p *sdk.Plugin) error {
	if err := plugin16WellknownCounts(ctx, p, 3, 2, "internal peer was not sent both halves and the fence", "external peer was not sent the withdrawal and the fence"); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "OK: internal peer received both halves and the fence")
	fmt.Fprintln(os.Stderr, "OK: external peer received the withdrawal and the fence, nothing else")
	return nil
}

func plugin16UpdateAndQuiesce(ctx context.Context, p *sdk.Plugin, selector, command string) error {
	if _, _, err := p.UpdateRoute(ctx, selector, command); err != nil {
		return err
	}
	q := plugin16Dispatch(ctx, p, "request quiesce")
	if q.err != nil || q.status != statusDone {
		return fmt.Errorf("quiesce after %q: status=%s data=%s", command, q.status, q.text())
	}
	return nil
}

func plugin16TeardownCmd(ctx context.Context, p *sdk.Plugin) error {
	if err := plugin16UpdateAndQuiesce(ctx, p, "*", "update text nhop 1.1.1.1 nlri ipv4/unicast add 1.1.0.0/16"); err != nil {
		return err
	}
	r := plugin16Dispatch(ctx, p, "request peer 127.0.0.1 teardown 4")
	if r.err != nil || r.status != statusDone {
		return fmt.Errorf("teardown: status=%s data=%s", r.status, r.text())
	}
	return nil
}

func plugin16TeardownMsg(ctx context.Context, p *sdk.Plugin) error {
	if err := plugin16UpdateAndQuiesce(ctx, p, "*", "update text nhop 1.1.1.1 nlri ipv4/unicast add 1.1.0.0/16"); err != nil {
		return err
	}
	r := plugin16Dispatch(ctx, p, "request peer 127.0.0.1 teardown 2 test")
	if r.err != nil || r.status != statusDone {
		return fmt.Errorf("teardown message: status=%s data=%s", r.status, r.text())
	}
	return nil
}

func plugin16WatchdogMED(ctx context.Context, p *sdk.Plugin) error {
	if q := plugin16Dispatch(ctx, p, "request quiesce"); q.err != nil || q.status != statusDone {
		return fmt.Errorf("startup quiesce: status=%s data=%s", q.status, q.text())
	}
	for _, command := range []string{
		"request bgp watchdog announce dnsr med 500",
		"request bgp watchdog announce dnsr med 1000",
		cmdWatchdogWithdrawDNSR,
		"request bgp watchdog announce dnsr med 200",
	} {
		r := plugin16Dispatch(ctx, p, command)
		if r.err != nil || r.status != statusDone {
			return fmt.Errorf("%s: status=%s data=%s", command, r.status, r.text())
		}
		q := plugin16Dispatch(ctx, p, "request quiesce")
		if q.err != nil || q.status != statusDone {
			return fmt.Errorf("quiesce after %s: status=%s data=%s", command, q.status, q.text())
		}
	}
	return nil
}

func plugin16WireEditOriginOrder(ctx context.Context, p *sdk.Plugin) error {
	return plugin16UpdateAndQuiesce(ctx, p, "*", "update text origin igp community [65000:100] large-community [65000:1:2] nhop 10.0.0.9 nlri ipv4/unicast add 10.0.0.7/32")
}
