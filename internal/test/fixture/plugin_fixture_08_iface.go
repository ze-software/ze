package fixture

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func init() {
	registerPlugin08("plugin/iface-bridge-mac-match-apply", "iface-bridge-mac-match-apply", ifaceBridgeMACMatch08)
	registerPlugin08("plugin/iface-bridge-member-selector-apply", "iface-bridge-member-selector-apply", ifaceBridgeMember08)
	registerPlugin08("plugin/iface-ensure-rollback", "iface-ensure-rollback", ifaceEnsureRollback08)
	registerPlugin08("plugin/iface-learned-route-metric", "iface-learned-route-metric", ifaceLearnedRouteMetric08)
	registerPlugin08("plugin/iface-mac-match-address-apply", "iface-mac-match-address-apply", ifaceMACMatch08)
	registerPlugin08("plugin/iface-osname-alias-apply", "iface-osname-alias-apply", ifaceOSName08)
	registerPlugin08("plugin/iface-route-protocol-name", "iface-route-protocol-name", ifaceRouteProtocol08)
	registerPlugin08("plugin/iface-tunnel-kinds", "iface-tunnel-kinds-test", ifaceTunnelKinds08)
	Register("plugin/iface-tunnel-restart-boot", ifaceTunnelRestartBoot08)
	Register("plugin/iface-tunnel-restart-wait", ifaceTunnelRestartWait08)
	registerPlugin08("plugin/iface-tunnel-restart-check", "iface-tunnel-restart-check", ifaceTunnelRestartCheck08)
	registerPlugin08("plugin/iface-verbs", "iface-verbs", ifaceVerbs08)
	registerPlugin08("plugin/interface-errors-show", "interface-errors-show-test", interfaceErrors08)
}

func ipJSON08(ctx context.Context, args ...string) ([]map[string]any, error) {
	stdout, stderr, err := runCommand08(ctx, false, "ip", append([]string{"-j"}, args...)...)
	if err != nil {
		return nil, fmt.Errorf("ip -j %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr))
	}
	var out []map[string]any
	if strings.TrimSpace(stdout) == "" {
		return out, nil
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		return nil, err
	}
	return out, nil
}

func ipJSONOptional08(ctx context.Context, args ...string) []map[string]any {
	out, _ := ipJSON08(ctx, args...)
	return out
}

func addresses08(ctx context.Context, device string) []string {
	rows := ipJSONOptional08(ctx, "addr", "show", device)
	var found []string
	for _, row := range rows {
		infos, _ := row["addr_info"].([]any)
		for _, value := range infos {
			info, _ := value.(map[string]any)
			local, _ := info["local"].(string)
			if local != "" {
				found = append(found, fmt.Sprintf("%s/%.0f", local, number08(info["prefixlen"])))
			}
		}
	}
	return found
}

func containsString08(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func ifaceBridgeMACMatch08(ctx context.Context, p *sdk.Plugin) error {
	device, bridge, logical := "zebasel0", "zebabr0", "zebawan"
	mac, cidr := "02:00:00:00:be:33", "198.51.100.177/28"
	links := ipJSONOptional08(ctx, "link", "show", device)
	if len(links) == 0 {
		return fmt.Errorf("%s was not created: the apply never reached the selected device", device)
	}
	if links[0]["address"] != mac || links[0]["master"] != bridge {
		return fmt.Errorf("%s address/master=%v/%v, want %s/%s", device, links[0]["address"], links[0]["master"], mac, bridge)
	}
	bridges := ipJSONOptional08(ctx, "link", "show", bridge)
	if len(bridges) == 0 || bridges[0]["address"] != mac {
		return fmt.Errorf("%s was not created with MAC %s: %v", bridge, mac, bridges)
	}
	if found := addresses08(ctx, device); !containsString08(found, cidr) {
		return fmt.Errorf("%s carries %v, not %s", device, found, cidr)
	}
	if len(ipJSONOptional08(ctx, "link", "show", logical)) != 0 {
		return fmt.Errorf("a device named %s exists: the apply worked on the logical name", logical)
	}
	fmt.Fprintf(os.Stderr, "OK: mac/match selector applied %s to %s past the bridge wearing %s\n", cidr, device, mac)
	return nil
}

func ifaceBridgeMember08(ctx context.Context, p *sdk.Plugin) error {
	device, logical, bridge, cidr := "zebmsel0", "zebmwan", "zebmbr0", "198.51.100.145/28"
	if len(ipJSONOptional08(ctx, "link", "show", bridge)) == 0 {
		return fmt.Errorf("%s was not created", bridge)
	}
	links := ipJSONOptional08(ctx, "link", "show", device)
	if len(links) == 0 || links[0]["master"] != bridge {
		return fmt.Errorf("%s master=%v, want %s", device, func() any {
			if len(links) == 0 {
				return nil
			}
			return links[0]["master"]
		}(), bridge)
	}
	if len(ipJSONOptional08(ctx, "link", "show", logical)) != 0 {
		return fmt.Errorf("a device named %s exists", logical)
	}
	if found := addresses08(ctx, device); !containsString08(found, cidr) {
		return fmt.Errorf("%s carries %v, not %s", device, found, cidr)
	}
	fmt.Fprintf(os.Stderr, "OK: bridge %s holds %s, the device %s resolves to\n", bridge, device, logical)
	return nil
}

func status08(ctx context.Context, p *sdk.Plugin, command string) (string, string) {
	status, raw, err := command08(ctx, p, command)
	return status, string(raw) + fmt.Sprint(err)
}

func pollStatus08(ctx context.Context, p *sdk.Plugin, command, wanted string, attempts int) (string, string) {
	var status, message string
	Poll(ctx, attempts, 250*time.Millisecond, func() bool {
		status, message = status08(ctx, p, command)
		return status == wanted
	})
	return status, message
}

func ifaceEnsureRollback08(ctx context.Context, p *sdk.Plugin) error {
	present := func(name, why string) error {
		status, message := pollStatus08(ctx, p, "show interface name "+name+" detail", "done", 40)
		if status != "done" {
			return fmt.Errorf("%s: expected %s to exist: status=%s %s", why, name, status, message)
		}
		return nil
	}
	absent := func(name, why string) error {
		status, message := pollStatus08(ctx, p, "show interface name "+name+" detail", "error", 40)
		if status != "error" {
			return fmt.Errorf("%s: expected %s absent: status=%s %s", why, name, status, message)
		}
		return nil
	}
	done := func(command string) error { _, err := requireDone08(ctx, p, command); return err }
	errorOnce := func(command string) error {
		status, message := status08(ctx, p, command)
		if status != "error" {
			return fmt.Errorf("%q: expected status=error for VLAN 9999, got %s %s", command, status, message)
		}
		return nil
	}
	defer func() {
		for _, name := range []string{"zeens0.100", "zeens0", "zeens1", "zeens2"} {
			_, _, _ = command08(context.Background(), p, "delete interface name "+name)
		}
	}()
	if err := absent("zeens0", "precondition"); err != nil {
		return err
	}
	if err := done("create interface dummy name zeens0 unit 100"); err != nil {
		return err
	}
	if err := present("zeens0", "A: ensure-exists must auto-create the parent"); err != nil {
		return err
	}
	if err := present("zeens0.100", "A: the leaf unit must be created too"); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "OK A: zeens0 auto-created by the ensure chain and zeens0.100 created by the leaf")
	if err := absent("zeens1", "precondition"); err != nil {
		return err
	}
	if err := done("create interface dummy name zeens1"); err != nil {
		return err
	}
	if err := present("zeens1", "B: operator-created parent must exist first"); err != nil {
		return err
	}
	if err := errorOnce("create interface dummy name zeens1 unit 9999"); err != nil {
		return err
	}
	if err := present("zeens1", "B: rollback deleted a pre-existing parent"); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "OK B: zeens1 survived a failed compound create")
	if err := absent("zeens2", "precondition"); err != nil {
		return err
	}
	if err := errorOnce("create interface dummy name zeens2 unit 9999"); err != nil {
		return err
	}
	if err := absent("zeens2", "C: rollback did not delete the auto-created parent"); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "OK C: zeens2 rolled back after the leaf failed")
	return nil
}

func showRoutes08(ctx context.Context, p *sdk.Plugin, dest string) ([]map[string]any, error) {
	raw, err := requireDone08(ctx, p, "show route "+dest)
	if err != nil {
		return nil, err
	}
	obj, err := object08(raw)
	if err != nil {
		return nil, err
	}
	values, _ := obj["routes"].([]any)
	var rows []map[string]any
	for _, value := range values {
		row, _ := value.(map[string]any)
		if row["destination"] == dest {
			rows = append(rows, row)
		}
	}
	return rows, nil
}

func ifaceLearnedRouteMetric08(ctx context.Context, p *sdk.Plugin) error {
	stdout, _, err := runCommand08(ctx, true, "ze", "schema", "show", "ze-iface-conf")
	if err != nil {
		return err
	}
	leaf := regexp.MustCompile(`(?s)leaf route-priority \{(?:[^{}]|\{[^{}]*\})*\}`).FindString(stdout)
	match := regexp.MustCompile(`default (\d+);`).FindStringSubmatch(leaf)
	if len(match) != 2 {
		return fmt.Errorf("route-priority declares no default: %s", leaf)
	}
	metric, _ := strconv.Atoi(match[1])
	if metric == 0 {
		return fmt.Errorf("ze installs learned routes at metric 0, the static metric")
	}
	dev := "zermetric0"
	defer runCommand08(context.Background(), false, "ip", "link", "del", dev)
	commands := [][]string{{"link", "add", dev, "type", "dummy"}, {"address", "add", "198.51.100.193/28", "dev", dev}, {"link", "set", dev, "up"}, {"route", "replace", "203.0.113.192/28", "via", "198.51.100.194", "dev", dev, "proto", "static", "metric", "0"}, {"route", "replace", "203.0.113.192/28", "via", "198.51.100.195", "dev", dev, "proto", "253", "metric", strconv.Itoa(metric)}}
	for _, args := range commands {
		if _, _, err := runCommand08(ctx, true, "ip", args...); err != nil {
			return err
		}
	}
	rows, err := showRoutes08(ctx, p, "203.0.113.192/28")
	if err != nil {
		return err
	}
	var static, learned map[string]any
	for _, row := range rows {
		if number08(row["metric"]) == 0 {
			static = row
		}
		if number08(row["metric"]) == float64(metric) {
			learned = row
		}
	}
	if len(rows) != 2 || static == nil || static["nexthop"] != "198.51.100.194" || static["protocol"] != "static" {
		return fmt.Errorf("expected intact static route and two routes, got %v", rows)
	}
	if learned == nil || learned["nexthop"] != "198.51.100.195" || learned["protocol"] != "ze-iface" {
		return fmt.Errorf("wrong learned route: %v", learned)
	}
	chosen, _, err := runCommand08(ctx, true, "ip", "route", "get", "203.0.113.197")
	if err != nil || !strings.Contains(chosen, "via 198.51.100.194") {
		return fmt.Errorf("probe is not forwarded through static gateway: %s: %w", chosen, err)
	}
	fmt.Fprintf(os.Stderr, "OK: static route (metric 0) and learned route (metric %d) coexist, static forwards\n", metric)
	return nil
}

func ifaceMACMatch08(ctx context.Context, p *sdk.Plugin) error {
	device, logical, mac, cidr := "zemacsel0", "zemacwan", "02:00:00:00:be:11", "198.51.100.129/28"
	links := ipJSONOptional08(ctx, "link", "show", device)
	if len(links) == 0 || links[0]["address"] != mac {
		return fmt.Errorf("%s missing or carries wrong MAC: %v", device, links)
	}
	if found := addresses08(ctx, device); !containsString08(found, cidr) {
		return fmt.Errorf("%s carries %v, not %s", device, found, cidr)
	}
	if len(ipJSONOptional08(ctx, "link", "show", logical)) != 0 {
		return fmt.Errorf("logical device %s exists", logical)
	}
	fmt.Fprintf(os.Stderr, "OK: mac/match selector applied %s to %s (%s)\n", cidr, device, mac)
	return nil
}

func ifaceOSName08(ctx context.Context, p *sdk.Plugin) error {
	device, logical, cidr := "zeosnsel0", "zeosnwan", "198.51.100.113/28"
	links := ipJSONOptional08(ctx, "link", "show", device)
	if len(links) == 0 || number08(links[0]["mtu"]) != 1400 {
		return fmt.Errorf("%s absent or wrong MTU: %v", device, links)
	}
	flags, _ := links[0]["flags"].([]any)
	up := false
	for _, flag := range flags {
		if flag == "UP" {
			up = true
		}
	}
	if !up {
		return fmt.Errorf("%s is not admin-up: %v", device, flags)
	}
	if found := addresses08(ctx, device); !containsString08(found, cidr) {
		return fmt.Errorf("%s carries %v, not %s", device, found, cidr)
	}
	if len(ipJSONOptional08(ctx, "link", "show", logical)) != 0 {
		return fmt.Errorf("logical device %s exists", logical)
	}
	fmt.Fprintf(os.Stderr, "OK: os-name alias applied %s, mtu 1400 and admin-up to %s\n", cidr, device)
	return nil
}

func ifaceRouteProtocol08(ctx context.Context, p *sdk.Plugin) error {
	dev := "zertproto0"
	defer runCommand08(context.Background(), false, "ip", "link", "del", dev)
	commands := [][]string{{"link", "add", dev, "type", "dummy"}, {"address", "add", "198.51.100.161/28", "dev", dev}, {"link", "set", dev, "up"}, {"route", "add", "203.0.113.160/28", "via", "198.51.100.162", "dev", dev, "proto", "253", "metric", "7"}, {"route", "add", "203.0.113.176/28", "via", "198.51.100.162", "dev", dev, "proto", "boot", "metric", "7"}}
	for _, args := range commands {
		if _, _, err := runCommand08(ctx, true, "ip", args...); err != nil {
			return err
		}
	}
	rows, err := showRoutes08(ctx, p, "203.0.113.160/28")
	if err != nil || len(rows) == 0 {
		return fmt.Errorf("iface route missing: %v: %w", rows, err)
	}
	if rows[0]["protocol"] != "ze-iface" || rows[0]["device"] != dev || rows[0]["nexthop"] != "198.51.100.162" {
		return fmt.Errorf("wrong iface route: %v", rows[0])
	}
	rows, err = showRoutes08(ctx, p, "203.0.113.176/28")
	if err != nil || len(rows) == 0 || rows[0]["protocol"] != "boot" {
		return fmt.Errorf("boot control route wrong: %v: %w", rows, err)
	}
	fmt.Fprintln(os.Stderr, "OK: show route names proto 253 ze-iface and proto 3 boot")
	return nil
}

func ifaceTunnelKinds08(ctx context.Context, p *sdk.Plugin) error {
	raw, err := requireDone08(ctx, p, "show interface")
	if err != nil {
		return err
	}
	rows, err := list08(raw)
	if err != nil || len(rows) == 0 {
		return fmt.Errorf("interface-all: expected non-empty list: %w", err)
	}
	present := make(map[string]bool)
	for _, row := range rows {
		if name, ok := row["name"].(string); ok {
			present[name] = true
		}
	}
	wanted := map[string]string{"tkgre": "gre", "tkgretap": "gretap", "tkip6gre": "ip6gre", "tkip6gretap": "ip6gretap", "tkipip": "ipip", "tksit": "sit", "tkip6tnl": "ip6tnl", "tkipip6": "ipip6", "tkvxlan": "vxlan"}
	var missing []string
	for name, kind := range wanted {
		if !present[name] {
			missing = append(missing, name+" ("+kind+")")
		}
	}
	sort.Strings(missing)
	if len(missing) != 0 {
		return fmt.Errorf("kernel built no link for %s", strings.Join(missing, ", "))
	}
	fmt.Fprintf(os.Stderr, "OK: all %d tunnel kinds created\n", len(wanted))
	return nil
}

func sysfs08(path, label string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("%s: %s: %w", label, path, err)
	}
	return strings.TrimSpace(string(data)), nil
}

func ifaceTunnelRestartWait08(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("unexpected arguments: %v", args)
	}
	var index string
	if Poll(ctx, 121, 100*time.Millisecond, func() bool {
		value, ok := readTrimmed08("restart-ifindex")
		if ok && value != "" {
			index = value
			return true
		}
		return false
	}) {
		fmt.Printf("OK: tunnel restart pass 1 recorded ifindex %s\n", index)
		return nil
	}
	const message = "pass 1 recorded no ifindex in restart-ifindex: its observer never reached the sysfs read, so pass 2 has nothing to compare against"
	fmt.Fprintln(os.Stderr, message)
	entries, _ := os.ReadDir(".")
	for _, entry := range entries {
		info, err := entry.Info()
		if err == nil {
			fmt.Fprintf(os.Stderr, "%s\t%d\n", entry.Name(), info.Size())
		}
	}
	return fmt.Errorf("%s", message)
}

func ifaceTunnelRestartBoot08(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("unexpected arguments: %v", args)
	}
	p, err := newObserver("iface-tunnel-restart-boot")
	if err != nil {
		return err
	}
	defer p.Close()
	result := make(chan error, 1)
	p.OnAllPluginsReady(func() error {
		index, err := sysfs08("/sys/class/net/trs0/ifindex", "first run: kernel holds no trs0")
		if err == nil {
			err = os.WriteFile("restart-ifindex", []byte(index), 0644)
		}
		if err == nil {
			fmt.Fprintf(os.Stderr, "OK: tunnel restart pass 1 created trs0 ifindex %s\n", index)
		}
		result <- err
		return nil
	})
	runErr := p.Run(ctx, sdk.Registration{})
	select {
	case scenarioErr := <-result:
		if scenarioErr != nil {
			return scenarioErr
		}
		return runErr
	default:
		if ctx.Err() != nil {
			return nil
		}
		return runErr
	}
}

func ifaceTunnelRestartCheck08(ctx context.Context, p *sdk.Plugin) error {
	before, err := sysfs08("restart-ifindex", "second run: first run never recorded trs0")
	if err != nil || before == "" {
		return fmt.Errorf("first run ifindex unavailable: %w", err)
	}
	after, err := sysfs08("/sys/class/net/trs0/ifindex", "second run: trs0 did not survive restart")
	if err != nil {
		return err
	}
	if after != before {
		return fmt.Errorf("trs0 ifindex %s -> %s: apply replaced netdev", before, after)
	}
	raw, err := requireDone08(ctx, p, "show interface")
	if err != nil {
		return err
	}
	rows, err := list08(raw)
	if err != nil {
		return err
	}
	found := false
	for _, row := range rows {
		if row["name"] == "trs0" {
			found = true
		}
	}
	if !found {
		return fmt.Errorf("trs0 is in kernel and absent from show interface")
	}
	fmt.Fprintf(os.Stderr, "OK: tunnel restart pass 2 kept trs0 same ifindex %s\n", after)
	return nil
}

func ifaceVerbs08(ctx context.Context, p *sdk.Plugin) error {
	name := "zetest0"
	defer command08(context.Background(), p, "delete interface name "+name)
	for _, command := range []string{"create interface dummy name zetest0", "request interface zetest0 mtu 1400", "request interface zetest0 mac 02:de:ad:be:ef:01", "request interface zetest0 up", "create interface zetest0 address 10.99.0.1/24"} {
		if _, err := requireDone08(ctx, p, command); err != nil {
			return err
		}
	}
	var info map[string]any
	if !Poll(ctx, 40, 250*time.Millisecond, func() bool {
		raw, err := requireDone08(ctx, p, "show interface name zetest0 detail")
		if err != nil {
			return false
		}
		info, err = object08(raw)
		if err != nil {
			return false
		}
		addresses, _ := info["addresses"].([]any)
		return len(addresses) != 0
	}) {
		return fmt.Errorf("show interface detail has no addresses: %v", info)
	}
	if info["type"] != "dummy" || number08(info["mtu"]) != 1400 || strings.ToLower(fmt.Sprint(info["mac-address"])) != "02:de:ad:be:ef:01" || info["state"] != "up" {
		return fmt.Errorf("wrong interface detail: %v", info)
	}
	addresses, _ := info["addresses"].([]any)
	found := false
	for _, value := range addresses {
		row, _ := value.(map[string]any)
		if row["address"] == "10.99.0.1" && number08(row["prefix-length"]) == 24 {
			found = true
		}
	}
	if !found {
		return fmt.Errorf("addresses missing 10.99.0.1/24: %v", addresses)
	}
	fmt.Fprintln(os.Stderr, "OK: zetest0 dummy mtu=1400 mac=02:de:ad:be:ef:01 state=up addr=10.99.0.1/24 confirmed via show interface detail")
	for _, command := range []string{"delete interface name zetest0 address 10.99.0.1/24", "request interface zetest0 down", "delete interface name zetest0"} {
		if _, err := requireDone08(ctx, p, command); err != nil {
			return err
		}
	}
	status, _ := pollStatus08(ctx, p, "show interface name zetest0 detail", "error", 40)
	if status != "error" {
		return fmt.Errorf("delete: zetest0 still present")
	}
	fmt.Fprintln(os.Stderr, "OK: zetest0 removed; show interface detail reports error")
	return nil
}

func interfaceErrors08(ctx context.Context, p *sdk.Plugin) error {
	if err := waitPeerEOR08(ctx, p); err != nil {
		return err
	}
	raw, err := requireDone08(ctx, p, "show interface")
	if err != nil {
		return err
	}
	all, err := list08(raw)
	if err != nil || len(all) == 0 {
		return fmt.Errorf("interface-all empty: %w", err)
	}
	counters := []string{"rx-errors", "rx-dropped", "tx-errors", "tx-dropped"}
	expected := make(map[string][4]int)
	var clean []string
	withStats := 0
	for _, row := range all {
		stats, ok := row["stats"].(map[string]any)
		if !ok {
			continue
		}
		withStats++
		values := [4]int{}
		nonzero := false
		for i, key := range counters {
			values[i] = int(number08(stats[key]))
			nonzero = nonzero || values[i] != 0
		}
		name, _ := row["name"].(string)
		if nonzero {
			expected[name] = values
		} else {
			clean = append(clean, name)
		}
	}
	if withStats == 0 || len(clean) == 0 {
		return fmt.Errorf("interface-all cannot discriminate filter: withStats=%d clean=%v", withStats, clean)
	}
	raw, err = requireDone08(ctx, p, "show interface errors")
	if err != nil {
		return err
	}
	obj, err := object08(raw)
	if err != nil {
		return err
	}
	values, ok := obj["interfaces"].([]any)
	if !ok {
		return fmt.Errorf("interface-errors missing interfaces list: %v", obj)
	}
	got := make(map[string][4]int)
	for _, value := range values {
		row, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("malformed row %v", value)
		}
		name, ok := row["name"].(string)
		if !ok {
			return fmt.Errorf("row missing name: %v", row)
		}
		nums := [4]int{}
		for i, key := range counters {
			if _, ok := row[key]; !ok {
				return fmt.Errorf("row %s missing %s", name, key)
			}
			nums[i] = int(number08(row[key]))
		}
		got[name] = nums
	}
	if !maps.Equal(got, expected) {
		return fmt.Errorf("interface-errors reported %v, expected %v; clean=%v", got, expected, clean)
	}
	fmt.Fprintf(os.Stderr, "OK: show interface errors matches %d error link(s); %d clean link(s) excluded\n", len(expected), len(clean))
	return nil
}
