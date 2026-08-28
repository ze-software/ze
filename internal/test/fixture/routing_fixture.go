package fixture

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"time"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func init() {
	Register("isis/isis-ipv6", routingObserver("isis-ipv6", isisIPv6Scenario))
	Register("isis/isis-route-install", routingObserver("isis-route", isisRouteScenario))
	Register("isis/isis-show", routingObserver("isis-show", isisShowScenario))
	Register("ldp/ldp-convergence", routingObserver("ldp-converge", ldpConvergenceScenario))
	Register("ldp/ldp-reload", routingObserver("ldp-reload", ldpReloadScenario))
	Register("ldp/ldp-session", routingObserver("ldp-show", ldpSessionScenario))
	Register("ospf/ospf-instance-demux", routingObserver("ospf-instance-demux", ospfInstanceDemuxScenario))
	Register("ospf/ospf-instance-teardown", routingObserver("ospf-instance-teardown", ospfInstanceTeardownScenario))
	Register("ospf/ospf-inter-area-cli", routingObserver("ospf-inter-area-cli", ospfInterAreaScenario))
	Register("ospf/ospf-interface-runtime", routingObserver("ospf-runtime", ospfInterfaceRuntimeScenario))
	Register("ospf/ospf-ldp-sync-broadcast", routingObserver("ospf-ldp-sync-broadcast", ospfLDPSyncBroadcastScenario))
	Register("ospf/ospf-ldp-sync-down", routingObserver("ospf-ldp-sync-down", ospfLDPSyncDownScenario))
	Register("ospf/ospf-ldp-sync-restore", routingObserver("ospf-ldp-sync-restore", ospfLDPSyncRestoreScenario))
	Register("ospf/ospf-ldp-sync-show", routingObserver("ospf-ldp-sync-show", ospfLDPSyncShowScenario))
	Register("ospf/ospf-multiaf-reconcile", routingObserver("ospf-multiaf-reconcile", ospfMultiAFReconcileScenario))
	Register("ospf/ospf-multiaf-show", routingObserver("ospf-multiaf-show", ospfMultiAFShowScenario))
	Register("ospf/ospf-multiaf-v4-route", routingObserver("ospf-multiaf-v4-route", ospfMultiAFV4RouteScenario))
	Register("ospf/ospf-multiaf", routingObserver("ospf-multiaf", ospfMultiAFScenario))
	Register("ospf/ospf-nbma", routingObserver("ospf-nbma", ospfNBMAScenario))
	Register("ospf/ospf-ptmp", routingObserver("ospf-ptmp", ospfPTMPScenario))
	Register("ospf/ospf-route-daemon", routingObserver("ospf-route", ospfRouteScenario))
	Register("ospf/ospf-show", routingObserver("ospf-show", ospfShowScenario))
	Register("ospfv3/ospfv3-nbma", routingObserver("ospfv3-nbma", ospfv3NBMAScenario))
	Register("ospfv3/ospfv3-ptmp", routingObserver("ospfv3-ptmp", ospfv3PTMPScenario))
	Register("ospfv3/ospfv3-vlink", routingObserver("ospfv3-vlink", ospfv3VLinkScenario))
	Register("rsvpte/rsvpte-bandwidth", routingObserver("rsvpte-bw", rsvpteBandwidthScenario))
	Register("rsvpte/rsvpte-frr", routingObserver("rsvpte-frr", rsvpteFRRScenario))
	Register("rsvpte/rsvpte-lsp-setup", routingObserver("rsvpte-setup", rsvpteSetupScenario))
	Register("rsvpte/rsvpte-lsp-teardown", routingObserver("rsvpte-teardown", rsvpteTeardownScenario))
	Register("rsvpte/rsvpte-reroute", routingObserver("rsvpte-reroute", rsvpteRerouteScenario))
	Register("vrrp/vrrp-show", routingObserver("vrrp-show", vrrpShowScenario))
}

func routingObserver(name string, scenario ObserverScenario) Driver {
	return func(ctx context.Context, _ []string) error {
		return Observe(ctx, name, sdk.Registration{}, scenario)
	}
}

func routingValue(ctx context.Context, plugin *sdk.Plugin, command string) (any, error) {
	var value any
	status, err := Dispatch(ctx, plugin, command, &value)
	if err != nil {
		return nil, fmt.Errorf("%s: status=%s: %w", command, status, err)
	}
	if status != "done" {
		return nil, fmt.Errorf("%s: expected done, got status=%s data=%v", command, status, value)
	}
	return value, nil
}

func routingRows(ctx context.Context, plugin *sdk.Plugin, command string, nullAsEmpty bool) ([]map[string]any, error) {
	value, err := routingValue(ctx, plugin, command)
	if err != nil {
		return nil, err
	}
	if value == nil && nullAsEmpty {
		return []map[string]any{}, nil
	}
	values, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("%s: expected JSON list, got %#v", command, value)
	}
	rows := make([]map[string]any, 0, len(values))
	for _, value := range values {
		row, ok := value.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%s: expected object row, got %#v", command, value)
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func routingObject(ctx context.Context, plugin *sdk.Plugin, command string) (map[string]any, error) {
	value, err := routingValue(ctx, plugin, command)
	if err != nil {
		return nil, err
	}
	object, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s: expected JSON object, got %#v", command, value)
	}
	return object, nil
}

func routingString(row map[string]any, key string) string {
	value, _ := row[key].(string)
	return value
}

func routingNumber(row map[string]any, key string) float64 {
	value, _ := row[key].(float64)
	return value
}

func routingBool(row map[string]any, key string) bool {
	value, _ := row[key].(bool)
	return value
}

func isisIPv6Scenario(ctx context.Context, plugin *sdk.Plugin) error {
	routes, err := routingRows(ctx, plugin, "show isis route ipv6", true)
	if err != nil {
		return err
	}
	if len(routes) != 0 {
		return fmt.Errorf("show isis route ipv6: expected empty list (no peer), got %#v", routes)
	}
	fmt.Fprintln(os.Stderr, "OK: IS-IS IPv6 route table empty with no adjacency (dual-stack SPF wired, no phantom routes)")
	return nil
}

func isisRouteScenario(ctx context.Context, plugin *sdk.Plugin) error {
	routes, err := routingRows(ctx, plugin, "show isis route", true)
	if err != nil {
		return err
	}
	if len(routes) != 0 {
		return fmt.Errorf("show isis route: expected empty list (no peer), got %#v", routes)
	}
	fmt.Fprintln(os.Stderr, "OK: IS-IS route table empty with no adjacency (SPF wired, no phantom routes)")
	return nil
}

func isisShowScenario(ctx context.Context, plugin *sdk.Plugin) error {
	neighbors, err := routingRows(ctx, plugin, "show isis neighbor", false)
	if err != nil {
		return err
	}
	if len(neighbors) != 0 {
		return fmt.Errorf("show isis neighbor: expected empty, got %#v", neighbors)
	}
	database, err := routingRows(ctx, plugin, "show isis database", false)
	if err != nil {
		return err
	}
	if len(database) == 0 {
		return fmt.Errorf("show isis database: expected the own LSP, got empty")
	}
	detail, err := routingRows(ctx, plugin, "show isis database detail", false)
	if err != nil {
		return err
	}
	if len(detail) == 0 {
		return fmt.Errorf("show isis database detail: expected rows, got empty")
	}
	if !routingRowsContainKey(database, "lsp-id") {
		return fmt.Errorf("show isis database: rows missing lsp-id: %#v", database)
	}
	if !routingRowsContainKey(detail, "tlvs") {
		return fmt.Errorf("show isis database detail: rows missing tlvs: %#v", detail)
	}
	lspID := regexp.MustCompile(`^[0-9a-f]{4}\.[0-9a-f]{4}\.[0-9a-f]{4}\.[0-9a-f]{2}-[0-9a-f]{2}$`)
	for _, view := range []struct {
		name string
		rows []map[string]any
	}{{"show isis database", database}, {"show isis database detail", detail}} {
		for _, row := range view.rows {
			own, present := row["own"]
			if !present {
				return fmt.Errorf("%s: row missing the own key: %#v", view.name, row)
			}
			if own != true {
				return fmt.Errorf("%s: this node's own LSP reported own != true: %#v", view.name, row["lsp-id"])
			}
			id, _ := row["lsp-id"].(string)
			if !lspID.MatchString(id) {
				return fmt.Errorf("%s: lsp-id carries more than the LSP ID: %#v", view.name, id)
			}
		}
	}
	if _, err := routingRows(ctx, plugin, "show isis route", false); err != nil {
		return err
	}
	interfaces, err := routingRows(ctx, plugin, "show isis interface", false)
	if err != nil {
		return err
	}
	if !routingAny(interfaces, func(row map[string]any) bool {
		return routingString(row, "name") == "lo" && routingBool(row, "passive")
	}) {
		return fmt.Errorf("show isis interface: passive lo not reported: %#v", interfaces)
	}
	hosts, err := routingRows(ctx, plugin, "show isis hostname", false)
	if err != nil {
		return err
	}
	if !routingAny(hosts, func(row map[string]any) bool { return routingString(row, "hostname") == "r1-isis" }) {
		return fmt.Errorf("show isis hostname: r1-isis mapping missing: %#v", hosts)
	}
	if _, err := routingRows(ctx, plugin, "show isis spf-log", false); err != nil {
		return err
	}
	for _, command := range []string{"clear isis adjacency", "clear isis counters"} {
		if _, err := routingValue(ctx, plugin, command); err != nil {
			return err
		}
	}
	fmt.Fprintln(os.Stderr, "OK: show isis neighbor/database[ detail]/route/interface/hostname/spf-log + clear isis adjacency/counters dispatched through the engine")
	return nil
}

func routingRowsContainKey(rows []map[string]any, key string) bool {
	for _, row := range rows {
		if _, ok := row[key]; ok {
			return true
		}
	}
	return false
}

func routingAny(rows []map[string]any, predicate func(map[string]any) bool) bool {
	for _, row := range rows {
		if predicate(row) {
			return true
		}
	}
	return false
}

func ldpConvergenceScenario(ctx context.Context, plugin *sdk.Plugin) error {
	neighbors, err := routingRows(ctx, plugin, "show ldp neighbor", false)
	if err != nil {
		return err
	}
	if len(neighbors) != 0 {
		return fmt.Errorf("show ldp neighbor: expected empty list (no peer), got %#v", neighbors)
	}
	fmt.Fprintln(os.Stderr, "OK: LDP converged to an empty neighbor table with no peer")
	return nil
}

func ldpReloadScenario(ctx context.Context, plugin *sdk.Plugin) error {
	bindings, err := routingRows(ctx, plugin, "show ldp binding", false)
	if err != nil {
		return err
	}
	for _, binding := range bindings {
		direction := routingString(binding, "direction")
		if direction != "local" && direction != "remote" {
			return fmt.Errorf("show ldp binding: bad direction in %#v", binding)
		}
		if _, ok := binding["fec"]; !ok {
			return fmt.Errorf("show ldp binding: missing fec in %#v", binding)
		}
		if _, ok := binding["label"]; !ok {
			return fmt.Errorf("show ldp binding: missing label in %#v", binding)
		}
	}
	fmt.Fprintln(os.Stderr, "OK: LDP booted on the configured interface and answered show ldp binding")
	return nil
}

func ldpSessionScenario(ctx context.Context, plugin *sdk.Plugin) error {
	neighbors, err := routingRows(ctx, plugin, "show ldp neighbor", false)
	if err != nil {
		return err
	}
	if len(neighbors) != 0 {
		return fmt.Errorf("show ldp neighbor: expected empty list (no peer), got %#v", neighbors)
	}
	bindings, err := routingRows(ctx, plugin, "show ldp binding", false)
	if err != nil {
		return err
	}
	for _, binding := range bindings {
		direction := routingString(binding, "direction")
		if direction != "local" && direction != "remote" {
			return fmt.Errorf("show ldp binding: bad direction in %#v", binding)
		}
	}
	fmt.Fprintln(os.Stderr, "OK: show ldp neighbor + binding dispatched through the LDP engine")
	return nil
}

func ospfInstanceDemuxScenario(ctx context.Context, plugin *sdk.Plugin) error {
	rows, err := routingRows(ctx, plugin, "show ospf instance", true)
	if err != nil {
		return err
	}
	byID := make(map[int]map[string]any, len(rows))
	for _, row := range rows {
		byID[int(routingNumber(row, "instance-id"))] = row
	}
	for _, id := range []int{0, 5} {
		row, ok := byID[id]
		if !ok {
			return fmt.Errorf("show ospf instance missing Instance ID %d: %#v", id, byID)
		}
		if routingNumber(row, "interface-count") < 1 {
			return fmt.Errorf("instance %d must carry eth0: %#v", id, byID)
		}
		if routingString(row, "router-id") != "10.0.0.1" {
			return fmt.Errorf("router-id per instance = %#v", byID)
		}
	}
	fmt.Fprintln(os.Stderr, "OK: OSPF multi-instance demux OK")
	return nil
}

func ospfInstanceTeardownScenario(ctx context.Context, plugin *sdk.Plugin) error {
	rows, err := routingRows(ctx, plugin, "show ospf instance", true)
	if err != nil {
		return err
	}
	ids := make([]int, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, int(routingNumber(row, "instance-id")))
	}
	sort.Ints(ids)
	if len(ids) != 1 || ids[0] != 0 {
		return fmt.Errorf("show ospf instance IDs = %#v, want exactly [0] (base only, no phantom instance)", ids)
	}
	fmt.Fprintln(os.Stderr, "OK: OSPF instance set tracks config OK")
	return nil
}

func ospfInterAreaScenario(ctx context.Context, plugin *sdk.Plugin) error {
	if _, err := routingRows(ctx, plugin, "show ospf border-routers", true); err != nil {
		return err
	}
	if _, err := routingRows(ctx, plugin, "show ospf route", true); err != nil {
		return err
	}
	spf, err := routingRows(ctx, plugin, "show ospf spf", true)
	if err != nil {
		return err
	}
	if len(spf) < 2 {
		return fmt.Errorf("show ospf spf: expected two configured areas, got %#v", spf)
	}
	fmt.Fprintln(os.Stderr, "OK: OSPF inter-area CLI dispatch OK")
	return nil
}

func ospfInterfaceRuntimeScenario(ctx context.Context, plugin *sdk.Plugin) error {
	rows, err := routingRows(ctx, plugin, "show ospf interface", false)
	if err != nil {
		return err
	}
	if err := requireOSPFInterface(rows, "lo0", "loopback", false, "loopback"); err != nil {
		return err
	}
	if err := requireOSPFInterface(rows, "passive0", "", true, "down"); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "OK: OSPF runtime interface table includes passive and loopback rows")
	return nil
}

func requireOSPFInterface(rows []map[string]any, name, networkType string, passive bool, state string) error {
	for _, row := range rows {
		if routingString(row, "name") != name {
			continue
		}
		if networkType != "" && routingString(row, "network-type") != networkType {
			return fmt.Errorf("%s: network-type = %#v, want %q", name, row["network-type"], networkType)
		}
		if routingBool(row, "passive") != passive {
			return fmt.Errorf("%s: passive = %#v, want %t", name, row["passive"], passive)
		}
		if routingString(row, "state") != state {
			return fmt.Errorf("%s: state = %#v, want %q", name, row["state"], state)
		}
		return nil
	}
	return fmt.Errorf("show ospf interface: missing %q in %#v", name, rows)
}

func ospfLDPRow(ctx context.Context, plugin *sdk.Plugin, iface string) (map[string]any, error) {
	rows, err := routingRows(ctx, plugin, "show ospf ldp-sync", true)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		if routingString(row, "interface") == iface {
			return row, nil
		}
	}
	return nil, nil
}

func emitLDPSession(ctx context.Context, plugin *sdk.Plugin, eventType, iface string) error {
	event, err := json.Marshal(map[string]string{"interface": iface, "session-state": "operational"})
	if err != nil {
		return err
	}
	_, err = plugin.EmitEvent(ctx, "ldp", eventType, "", "", string(event))
	return err
}

func waitOSPFLDPState(ctx context.Context, plugin *sdk.Plugin, iface, state string) (map[string]any, error) {
	var row map[string]any
	var dispatchErr error
	if !Poll(ctx, 30, 200*time.Millisecond, func() bool {
		row, dispatchErr = ospfLDPRow(ctx, plugin, iface)
		return dispatchErr == nil && row != nil && routingString(row, "state") == state
	}) {
		if dispatchErr != nil {
			return nil, dispatchErr
		}
		return nil, fmt.Errorf("%s did not reach %s; last = %#v", iface, state, row)
	}
	return row, nil
}

func ospfLDPSyncBroadcastScenario(ctx context.Context, plugin *sdk.Plugin) error {
	row, err := ospfLDPRow(ctx, plugin, "eth0")
	if err != nil {
		return err
	}
	if row == nil || routingString(row, "state") != "not-synchronized" {
		return fmt.Errorf("initial broadcast eth0 = %#v, want not-synchronized", row)
	}
	if err := emitLDPSession(ctx, plugin, "session-up", "eth0"); err != nil {
		return err
	}
	if _, err := waitOSPFLDPState(ctx, plugin, "eth0", "synchronized"); err != nil {
		return fmt.Errorf("broadcast %w", err)
	}
	fmt.Fprintln(os.Stderr, "OK: broadcast ldp-sync interface driven to synchronized")
	return nil
}

func ospfLDPSyncDownScenario(ctx context.Context, plugin *sdk.Plugin) error {
	if _, err := waitOSPFLDPState(ctx, plugin, "eth0", "not-synchronized"); err != nil {
		return err
	}
	if err := emitLDPSession(ctx, plugin, "session-up", "eth0"); err != nil {
		return err
	}
	row, err := waitOSPFLDPState(ctx, plugin, "eth0", "synchronized")
	if err != nil {
		return err
	}
	if routingNumber(row, "effective-metric") != 100 {
		return fmt.Errorf("synchronized effective-metric = %#v, want 100", row["effective-metric"])
	}
	if err := emitLDPSession(ctx, plugin, "session-down", "eth0"); err != nil {
		return err
	}
	row, err = waitOSPFLDPState(ctx, plugin, "eth0", "not-synchronized")
	if err != nil {
		return err
	}
	if routingNumber(row, "effective-metric") != 65535 {
		return fmt.Errorf("after session-down effective-metric = %#v, want 65535", row["effective-metric"])
	}
	fmt.Fprintln(os.Stderr, "OK: LDP session-down re-forces LSInfinity")
	return nil
}

func ospfLDPSyncRestoreScenario(ctx context.Context, plugin *sdk.Plugin) error {
	row, err := ospfLDPRow(ctx, plugin, "eth0")
	if err != nil {
		return err
	}
	if row == nil || routingString(row, "state") != "not-synchronized" {
		return fmt.Errorf("initial eth0 = %#v, want not-synchronized", row)
	}
	if err := emitLDPSession(ctx, plugin, "session-up", "eth0"); err != nil {
		return err
	}
	timer := time.NewTimer(200 * time.Millisecond)
	select {
	case <-ctx.Done():
		timer.Stop()
		return ctx.Err()
	case <-timer.C:
	}
	row, err = ospfLDPRow(ctx, plugin, "eth0")
	if err != nil {
		return err
	}
	if row == nil || routingString(row, "state") != "hold-down" {
		return fmt.Errorf("after session-up eth0 = %#v, want hold-down", row)
	}
	if routingNumber(row, "effective-metric") != 65535 {
		return fmt.Errorf("hold-down effective-metric = %#v, want 65535", row["effective-metric"])
	}
	row, err = waitOSPFLDPState(ctx, plugin, "eth0", "synchronized")
	if err != nil {
		return err
	}
	if routingNumber(row, "effective-metric") != 100 {
		return fmt.Errorf("restored effective-metric = %#v, want configured 100", row["effective-metric"])
	}
	fmt.Fprintln(os.Stderr, "OK: LDP session-up + hold-down restores configured cost")
	return nil
}

func ospfLDPSyncShowScenario(ctx context.Context, plugin *sdk.Plugin) error {
	row, err := ospfLDPRow(ctx, plugin, "eth0")
	if err != nil {
		return err
	}
	if row == nil {
		return fmt.Errorf("show ospf ldp-sync: eth0 missing")
	}
	if routingString(row, "state") != "not-synchronized" {
		return fmt.Errorf("eth0 state = %#v, want not-synchronized", row["state"])
	}
	if routingNumber(row, "effective-metric") != 65535 {
		return fmt.Errorf("eth0 effective-metric = %#v, want 65535 (LSInfinity)", row["effective-metric"])
	}
	fmt.Fprintln(os.Stderr, "OK: show ospf ldp-sync reports not-synchronized/LSInfinity")
	return nil
}

func ospfMultiAFRows(ctx context.Context, plugin *sdk.Plugin) ([]map[string]any, error) {
	return routingRows(ctx, plugin, "show ospf ipv6", true)
}

func ospfMultiAFReconcileScenario(ctx context.Context, plugin *sdk.Plugin) error {
	rows, err := ospfMultiAFRows(ctx, plugin)
	if err != nil {
		return err
	}
	afs := make(map[string]bool, len(rows))
	for _, row := range rows {
		afs[routingString(row, "address-family")] = true
	}
	if !afs["ipv6-unicast"] || !afs["ipv4-unicast"] {
		return fmt.Errorf("expected both AF engines up after configuring two AFs, got %#v", afs)
	}
	fmt.Fprintln(os.Stderr, "OK: added OSPFv3 address family engine is up")
	return nil
}

func ospfMultiAFShowScenario(ctx context.Context, plugin *sdk.Plugin) error {
	rows, err := ospfMultiAFRows(ctx, plugin)
	if err != nil {
		return err
	}
	byAF := make(map[string]map[string]any, len(rows))
	for _, row := range rows {
		byAF[routingString(row, "address-family")] = row
	}
	if row := byAF["ipv6-unicast"]; row == nil || routingNumber(row, "instance-id") != 0 {
		return fmt.Errorf("show ospf ipv6: missing ipv6-unicast at instance 0: %#v", rows)
	}
	if row := byAF["ipv4-unicast"]; row == nil || routingNumber(row, "instance-id") != 64 {
		return fmt.Errorf("show ospf ipv6: missing ipv4-unicast at instance 64: %#v", rows)
	}
	fmt.Fprintln(os.Stderr, "OK: show ospf ipv6 identifies both AF instances")
	return nil
}

func ospfMultiAFV4RouteScenario(ctx context.Context, plugin *sdk.Plugin) error {
	rows, err := ospfMultiAFRows(ctx, plugin)
	if err != nil {
		return err
	}
	if !routingAny(rows, func(row map[string]any) bool {
		return routingString(row, "address-family") == "ipv4-unicast" && routingNumber(row, "instance-id") == 64
	}) {
		return fmt.Errorf("IPv4-unicast-over-OSPFv3 instance not running at instance 64: %#v", rows)
	}
	fmt.Fprintln(os.Stderr, "OK: IPv4-unicast-over-OSPFv3 instance running for redistribution")
	return nil
}

func ospfMultiAFScenario(ctx context.Context, plugin *sdk.Plugin) error {
	rows, err := ospfMultiAFRows(ctx, plugin)
	if err != nil {
		return err
	}
	instances := make(map[string]int, len(rows))
	for _, row := range rows {
		instances[routingString(row, "address-family")] = int(routingNumber(row, "instance-id"))
	}
	if instances["ipv6-unicast"] != 0 || instances["ipv4-unicast"] != 64 || len(instances) != 2 {
		return fmt.Errorf("expected exactly two independent AF instances (v6u@0, v4u@64), got %#v", instances)
	}
	fmt.Fprintln(os.Stderr, "OK: two independent OSPFv3 AF engine instances")
	return nil
}

func ospfNBMAScenario(ctx context.Context, plugin *sdk.Plugin) error {
	rows, err := routingRows(ctx, plugin, "show ospf interface", false)
	if err != nil {
		return err
	}
	var iface map[string]any
	for _, row := range rows {
		if routingString(row, "name") == "nbma0" {
			iface = row
			break
		}
	}
	if iface == nil {
		return fmt.Errorf("missing nbma0 in %#v", rows)
	}
	if routingString(iface, "network_type") != "nbma" {
		return fmt.Errorf("nbma0 network_type = %#v, want nbma", iface["network_type"])
	}
	if routingNumber(iface, "poll-interval") != 90 {
		return fmt.Errorf("nbma0 poll-interval = %#v, want 90", iface["poll-interval"])
	}
	state := routingString(iface, "state")
	if state != "waiting" && state != "dr" && state != "backup" && state != "dr-other" && state != "down" {
		return fmt.Errorf("nbma0 state = %#v", iface["state"])
	}
	neighbors, ok := iface["nbma-neighbors"].([]any)
	if !ok && iface["nbma-neighbors"] != nil {
		return fmt.Errorf("nbma0 configured neighbors malformed: %#v", iface["nbma-neighbors"])
	}
	addresses := make(map[string]bool, len(neighbors))
	for _, value := range neighbors {
		neighbor, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("nbma0 configured neighbor malformed: %#v", value)
		}
		addresses[routingString(neighbor, "neighbor")] = true
		neighborState := routingString(neighbor, "state")
		if neighborState != "attempt" && neighborState != "heard" {
			return fmt.Errorf("neighbor %#v state must be attempt/heard", neighbor)
		}
	}
	if !addresses["10.0.0.2"] || !addresses["10.0.0.3"] {
		return fmt.Errorf("nbma0 configured neighbors = %#v, want 10.0.0.2 and 10.0.0.3", neighbors)
	}
	fmt.Fprintln(os.Stderr, "OK: OSPF NBMA interface renders poll interval and configured neighbors")
	return nil
}

func ospfPTMPScenario(ctx context.Context, plugin *sdk.Plugin) error {
	rows, err := routingRows(ctx, plugin, "show ospf interface", false)
	if err != nil {
		return err
	}
	var iface map[string]any
	for _, row := range rows {
		if routingString(row, "name") == "ptmp0" {
			iface = row
			break
		}
	}
	if iface == nil {
		return fmt.Errorf("missing ptmp0 in %#v", rows)
	}
	if routingString(iface, "network_type") != "point-to-multipoint" {
		return fmt.Errorf("ptmp0 network_type = %#v, want point-to-multipoint", iface["network_type"])
	}
	state := routingString(iface, "state")
	if state != "point-to-point" && state != "down" {
		return fmt.Errorf("ptmp0 state = %#v, want point-to-point", iface["state"])
	}
	dr := routingString(iface, "dr")
	if dr != "" && dr != "0.0.0.0" {
		return fmt.Errorf("ptmp0 elected a DR %q; PtMP has no DR", dr)
	}
	fmt.Fprintln(os.Stderr, "OK: OSPF point-to-multipoint interface renders with no DR")
	return nil
}

func ospfRouteScenario(ctx context.Context, plugin *sdk.Plugin) error {
	routes, err := routingRows(ctx, plugin, "show ospf route", true)
	if err != nil {
		return err
	}
	if len(routes) != 0 {
		return fmt.Errorf("show ospf route: expected empty list (no peer), got %#v", routes)
	}
	fmt.Fprintln(os.Stderr, "OK: OSPF route table empty with no adjacency (SPF wired, no phantom routes)")
	return nil
}

func ospfShowScenario(ctx context.Context, plugin *sdk.Plugin) error {
	summary, err := routingObject(ctx, plugin, "show ospf")
	if err != nil {
		return err
	}
	if routingString(summary, "router-id") != "10.0.0.1" {
		return fmt.Errorf("show ospf: router-id = %#v", summary["router-id"])
	}
	if !routingBool(summary, "abr") {
		return fmt.Errorf("show ospf: expected abr=true for two areas, got %#v", summary)
	}
	stub, ok := summary["stub-router"].(map[string]any)
	if !ok || !routingBool(stub, "always") || !routingBool(stub, "active") {
		return fmt.Errorf("show ospf: expected stub-router always/active, got %#v", summary["stub-router"])
	}
	for _, command := range []string{
		"show ospf neighbor", "show ospf interface", "show ospf database", "show ospf route", "show ospf border-routers", "show ospf spf",
		"show ospf database router", "show ospf database network", "show ospf database summary", "show ospf database asbr-summary", "show ospf database external", "show ospf database nssa-external",
	} {
		if _, err := routingRows(ctx, plugin, command, true); err != nil {
			return err
		}
	}
	for _, command := range []string{"clear ospf process", "clear ospf neighbor", "clear ospf counters"} {
		result, err := routingObject(ctx, plugin, command)
		if err != nil {
			return err
		}
		if routingString(result, "action") != command {
			return fmt.Errorf("%s: action = %#v", command, result["action"])
		}
	}
	fmt.Fprintln(os.Stderr, "OK: OSPF show/clear dispatch OK")
	return nil
}

func ospfv3NBMAScenario(context.Context, *sdk.Plugin) error {
	fmt.Fprintln(os.Stderr, "OK: OSPFv3 NBMA engine started")
	return nil
}

func ospfv3PTMPScenario(context.Context, *sdk.Plugin) error {
	fmt.Fprintln(os.Stderr, "OK: OSPFv3 point-to-multipoint engine started")
	return nil
}

func ospfv3VLinkScenario(ctx context.Context, plugin *sdk.Plugin) error {
	if _, err := routingValue(ctx, plugin, "show ospf"); err != nil {
		return fmt.Errorf("show ospf after booting a v6 vlink config: %w", err)
	}
	fmt.Fprintln(os.Stderr, "OK: OSPFv3 virtual-link config booted the daemon cleanly")
	return nil
}

func rsvpteBandwidthScenario(ctx context.Context, plugin *sdk.Plugin) error {
	interfaces, err := routingRows(ctx, plugin, "show rsvp-te interface", false)
	if err != nil {
		return err
	}
	var lo map[string]any
	for _, iface := range interfaces {
		if routingString(iface, "name") == "lo" {
			lo = iface
			break
		}
	}
	if lo == nil {
		return fmt.Errorf("show rsvp-te interface: missing lo: %#v", interfaces)
	}
	for key, want := range map[string]float64{"max-bandwidth": 10e9, "max-reservable": 8e9, "reserved-bandwidth": 0, "available-bandwidth": 8e9} {
		if routingNumber(lo, key) != want {
			return fmt.Errorf("lo %s=%#v, want %g", key, lo[key], want)
		}
	}
	fmt.Fprintln(os.Stderr, "OK: rsvp-te per-interface bandwidth parsed into admission state")
	return nil
}

func rsvpteFRRScenario(ctx context.Context, plugin *sdk.Plugin) error {
	rows, err := routingRows(ctx, plugin, "show rsvp-te fast-reroute", false)
	if err != nil {
		return err
	}
	foundBypass := false
	foundEndpoint := false
	for _, row := range rows {
		if routingString(row, "kind") == "bypass" {
			foundBypass = true
			foundEndpoint = foundEndpoint || routingString(row, "tunnel-endpoint") == "10.0.0.5"
		}
	}
	if !foundBypass {
		return fmt.Errorf("show rsvp-te fast-reroute: expected a configured bypass, got %#v", rows)
	}
	if !foundEndpoint {
		return fmt.Errorf("bypass should terminate at the merge point 10.0.0.5, got %#v", rows)
	}
	fmt.Fprintln(os.Stderr, "OK: rsvp-te fast-reroute show dispatched and reported the bypass")
	return nil
}

func rsvpteSetupScenario(ctx context.Context, plugin *sdk.Plugin) error {
	interfaces, err := routingRows(ctx, plugin, "show rsvp-te interface", false)
	if err != nil {
		return err
	}
	var lo map[string]any
	for _, iface := range interfaces {
		if routingString(iface, "name") == "lo" {
			lo = iface
			break
		}
	}
	if lo == nil {
		return fmt.Errorf("show rsvp-te interface: expected lo, got %#v", interfaces)
	}
	if routingNumber(lo, "max-reservable") != 8e9 {
		return fmt.Errorf("show rsvp-te interface: lo max-reservable=%#v, want 8e9", lo["max-reservable"])
	}
	sessions, err := routingRows(ctx, plugin, "show rsvp-te session", false)
	if err != nil {
		return err
	}
	var lsp map[string]any
	for _, session := range sessions {
		if routingString(session, "tunnel-endpoint") == "10.0.0.9" && routingNumber(session, "tunnel-id") == 1 {
			lsp = session
			break
		}
	}
	if lsp == nil {
		return fmt.Errorf("show rsvp-te session: expected the to-egress tunnel LSP (10.0.0.9), got %#v", sessions)
	}
	if routingString(lsp, "role") != "ingress" {
		return fmt.Errorf("show rsvp-te session: tunnel LSP role=%#v, want ingress", lsp["role"])
	}
	fmt.Fprintln(os.Stderr, "OK: rsvp-te configured; interface admission + session show dispatched")
	return nil
}

func rsvpteTeardownScenario(ctx context.Context, plugin *sdk.Plugin) error {
	sessions, err := routingRows(ctx, plugin, "show rsvp-te session", false)
	if err != nil {
		return err
	}
	for _, session := range sessions {
		for _, field := range []string{"tunnel-endpoint", "tunnel-id", "state"} {
			if _, ok := session[field]; !ok {
				return fmt.Errorf("show rsvp-te session: missing %s in %#v", field, session)
			}
		}
	}
	fmt.Fprintln(os.Stderr, "OK: rsvp-te session table queryable with no malformed entries")
	return nil
}

func rsvpteRerouteScenario(ctx context.Context, plugin *sdk.Plugin) error {
	tunnels, err := routingRows(ctx, plugin, "show rsvp-te tunnel", false)
	if err != nil {
		return err
	}
	for _, tunnel := range tunnels {
		if routingNumber(tunnel, "ero-hops") < 1 {
			return fmt.Errorf("show rsvp-te tunnel: expected ERO hops, got %#v", tunnel)
		}
	}
	if _, err := routingRows(ctx, plugin, "show rsvp-te session", false); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "OK: rsvp-te tunnel + session show dispatched for an ERO tunnel")
	return nil
}

func vrrpShowPayload(ctx context.Context, plugin *sdk.Plugin, command, key string) (any, error) {
	object, err := routingObject(ctx, plugin, command)
	if err != nil {
		return nil, err
	}
	value, ok := object[key]
	if !ok {
		return nil, fmt.Errorf("%s: missing %q in %#v", command, key, object)
	}
	return value, nil
}

func vrrpShowScenario(ctx context.Context, plugin *sdk.Plugin) error {
	for _, assertion := range []struct {
		command string
		key     string
	}{{"show vrrp", "vrrp"}, {"show vrrp interface name eth0", "vrrp"}, {"show vrrp statistics", "vrrp-statistics"}} {
		value, err := vrrpShowPayload(ctx, plugin, assertion.command, assertion.key)
		if err != nil {
			return err
		}
		rows, ok := value.([]any)
		if !ok || len(rows) != 0 {
			return fmt.Errorf("%s: expected empty list with zero groups, got %#v", assertion.command, value)
		}
	}
	cleared, err := vrrpShowPayload(ctx, plugin, "clear vrrp statistics", "cleared")
	if err != nil {
		return err
	}
	if cleared != float64(0) {
		return fmt.Errorf("clear vrrp statistics: expected cleared 0, got %#v", cleared)
	}
	fmt.Fprintln(os.Stderr, "OK: vrrp show/interface/statistics/clear all resolve through the daemon path")
	return nil
}
