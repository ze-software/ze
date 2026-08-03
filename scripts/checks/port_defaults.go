// Design: plan/learned/788-doctor-improvements.md -- listener default port table
//
// port_defaults pins the hand-maintained Go listener-default table
// (internal/component/config/listener_defaults.go, RegisterBuiltinListenerDefaults)
// to the `refine port { default N }` values in each service's YANG module. The
// Ze YANG compiler does not propagate refine defaults into the runtime schema,
// so the Go table restates them by hand; without this gate the two silently
// drift (someone edits the YANG default but not the Go table, or vice versa)
// and the daemon binds a port the schema documents differently.
//
// Scope: the central table in listener_defaults.go. Per-plugin listener
// registrations (e.g. as112) live in their own register.go and are outside this
// gate: their service names are schema-path-derived and a single module can
// carry several `refine port` blocks (as112 has two anycast listeners), which
// needs per-listener disambiguation this gate does not model.
//
// Usage:   go run scripts/checks/port_defaults.go [--json] [--selftest]
// Called by: make ze-port-defaults-check
//
//go:build ignore

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
)

// serviceYANG maps each central listener service (registered in
// listener_defaults.go) to the YANG module carrying its refine port default.
// The association (service <-> module) is structural; the port VALUE is read
// from the YANG at check time and compared to the Go table, so a drift in
// either source is caught.
var serviceYANG = map[string]string{
	"web":             "internal/component/web/yang/ze-web-conf.yang",
	"ssh":             "internal/component/ssh/yang/ze-ssh-conf.yang",
	"mcp":             "internal/component/mcp/yang/ze-mcp-conf.yang",
	"gnmi":            "internal/component/gnmi/yang/ze-gnmi-conf.yang",
	"looking-glass":   "internal/component/lg/yang/ze-lg-conf.yang",
	"api-server-rest": "internal/component/api/rest/yang/ze-rest-conf.yang",
	"api-server-grpc": "internal/component/api/grpc/yang/ze-grpc-conf.yang",
	"prometheus":      "internal/component/telemetry/exporter/yang/ze-telemetry-conf.yang",
}

const goTablePath = "internal/component/config/listener_defaults.go"

// Drift reason codes (stable, machine-readable).
const (
	reasonUnmapped   = "unmapped-service"  // service in Go table, no YANG module mapping
	reasonUnreadable = "unreadable-yang"   // mapped YANG module could not be read
	reasonNoDefault  = "no-refine-default" // no single `refine port { default N }` in the module
	reasonMismatch   = "port-mismatch"     // Go default != YANG default
	reasonStaleMap   = "stale-mapping"     // module mapped but service no longer in Go table
)

var (
	// RegisterListenerDefault("service", "ip", "port")
	goEntryRe = regexp.MustCompile(`RegisterListenerDefault\("([^"]+)",\s*"[^"]*",\s*"(\d+)"\)`)
	// refine port { default 3443; }
	refinePortRe = regexp.MustCompile(`refine\s+port\s*\{\s*default\s+(\d+)\s*;`)
)

type drift struct {
	Service  string `json:"service"`
	GoPort   int    `json:"go-port"`
	YANGPort int    `json:"yang-port"`
	File     string `json:"file,omitempty"`
	Reason   string `json:"reason"`
}

type result struct {
	Drifts  []drift `json:"drifts"`
	Checked int     `json:"services-checked"`
	Valid   bool    `json:"valid"`
}

func main() {
	jsonOut := false
	selftestMode := false
	for _, a := range os.Args[1:] {
		switch a {
		case "--json":
			jsonOut = true
		case "--selftest":
			selftestMode = true
		}
	}

	if selftestMode {
		if !selftest() {
			fmt.Fprintln(os.Stderr, "port-defaults: SELFTEST FAILED")
			os.Exit(2)
		}
		fmt.Fprintln(os.Stdout, "port-defaults: selftest OK")
		return
	}

	goTable, err := parseGoTable(goTablePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "port-defaults: read Go table: %v\n", err)
		os.Exit(2)
	}

	res := result{Drifts: compare(goTable, serviceYANG, readFileString)}
	res.Checked = len(goTable)
	res.Valid = len(res.Drifts) == 0

	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(res)
	} else {
		printResult(res)
	}
	if !res.Valid {
		os.Exit(1)
	}
}

// parseGoTable extracts the (service -> port) map from listener_defaults.go.
func parseGoTable(path string) (map[string]int, error) {
	content, err := readFileString(path)
	if err != nil {
		return nil, err
	}
	table := map[string]int{}
	for _, m := range goEntryRe.FindAllStringSubmatch(content, -1) {
		port, perr := strconv.Atoi(m[2])
		if perr != nil {
			return nil, fmt.Errorf("service %q: bad port %q: %w", m[1], m[2], perr)
		}
		table[m[1]] = port
	}
	return table, nil
}

// yangPortDefault extracts the single `refine port { default N }` value from a
// YANG module's text. Returns ok=false when zero or more than one is present
// (ambiguous), so the caller reports it rather than guessing.
func yangPortDefault(content string) (int, bool) {
	matches := refinePortRe.FindAllStringSubmatch(content, -1)
	if len(matches) != 1 {
		return 0, false
	}
	port, err := strconv.Atoi(matches[0][1])
	if err != nil {
		return 0, false
	}
	return port, true
}

// compare checks every Go-table service against its YANG refine port default.
// readFile is injected so the selftest can supply synthetic module content.
func compare(goTable map[string]int, yangMap map[string]string, readFile func(string) (string, error)) []drift {
	var drifts []drift

	services := make([]string, 0, len(goTable))
	for s := range goTable {
		services = append(services, s)
	}
	sort.Strings(services)

	for _, svc := range services {
		goPort := goTable[svc]
		yangFile, ok := yangMap[svc]
		if !ok {
			drifts = append(drifts, drift{Service: svc, GoPort: goPort, YANGPort: -1, Reason: reasonUnmapped})
			continue
		}
		content, err := readFile(yangFile)
		if err != nil {
			drifts = append(drifts, drift{Service: svc, GoPort: goPort, YANGPort: -1, File: yangFile, Reason: reasonUnreadable})
			continue
		}
		yangPort, found := yangPortDefault(content)
		if !found {
			drifts = append(drifts, drift{Service: svc, GoPort: goPort, YANGPort: -1, File: yangFile, Reason: reasonNoDefault})
			continue
		}
		if yangPort != goPort {
			drifts = append(drifts, drift{Service: svc, GoPort: goPort, YANGPort: yangPort, File: yangFile, Reason: reasonMismatch})
		}
	}

	// A stale serviceYANG entry (module mapped but the service was dropped from
	// the Go table) is also drift -- the map must track the table exactly.
	extras := make([]string, 0)
	for svc := range yangMap {
		if _, ok := goTable[svc]; !ok {
			extras = append(extras, svc)
		}
	}
	sort.Strings(extras)
	for _, svc := range extras {
		drifts = append(drifts, drift{Service: svc, GoPort: -1, YANGPort: -1, File: yangMap[svc], Reason: reasonStaleMap})
	}

	return drifts
}

func readFileString(path string) (string, error) {
	b, err := os.ReadFile(path) //nolint:gosec // fixed in-repo paths
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func printResult(r result) {
	fmt.Fprintf(os.Stdout, "# Listener Port-Default Gate\n\n")
	fmt.Fprintf(os.Stdout, "Services checked: %d\n\n", r.Checked)
	if len(r.Drifts) > 0 {
		fmt.Fprintf(os.Stdout, "## Drift (%d)\n\n", len(r.Drifts))
		for _, d := range r.Drifts {
			fmt.Fprintf(os.Stdout, "  [%s] service=%s go-port=%d yang-port=%d %s\n",
				d.Reason, d.Service, d.GoPort, d.YANGPort, d.File)
		}
		fmt.Fprintln(os.Stdout)
		fmt.Fprintln(os.Stdout, "port-defaults: FAILED")
		return
	}
	fmt.Fprintln(os.Stdout, "port-defaults: OK")
}

// selftest exercises compare() with synthetic inputs so a broken comparison is
// caught in CI independently of the live tree. Returns true when every case
// behaves as expected.
func selftest() bool {
	read := func(m map[string]string) func(string) (string, error) {
		return func(p string) (string, error) {
			c, ok := m[p]
			if !ok {
				return "", fmt.Errorf("no such file: %s", p)
			}
			return c, nil
		}
	}

	// Case 1: matching default -> no drift.
	d := compare(
		map[string]int{"x": 100},
		map[string]string{"x": "x.yang"},
		read(map[string]string{"x.yang": "uses zt:listener { refine port { default 100; } }"}),
	)
	if len(d) != 0 {
		fmt.Fprintf(os.Stderr, "selftest case1 (match): expected 0 drifts, got %d\n", len(d))
		return false
	}

	// Case 2: value drift -> one mismatch naming both values.
	d = compare(
		map[string]int{"x": 100},
		map[string]string{"x": "x.yang"},
		read(map[string]string{"x.yang": "refine port { default 200; }"}),
	)
	if len(d) != 1 || d[0].Reason != reasonMismatch || d[0].GoPort != 100 || d[0].YANGPort != 200 {
		fmt.Fprintf(os.Stderr, "selftest case2 (drift): unexpected result %+v\n", d)
		return false
	}

	// Case 3: unmapped service -> drift.
	d = compare(
		map[string]int{"y": 100},
		map[string]string{},
		read(map[string]string{}),
	)
	if len(d) != 1 || d[0].Reason != reasonUnmapped || d[0].Service != "y" {
		fmt.Fprintf(os.Stderr, "selftest case3 (unmapped): unexpected result %+v\n", d)
		return false
	}

	// Case 4: YANG without a refine port default -> drift.
	d = compare(
		map[string]int{"x": 100},
		map[string]string{"x": "x.yang"},
		read(map[string]string{"x.yang": "container x { leaf name { type string; } }"}),
	)
	if len(d) != 1 || d[0].Reason != reasonNoDefault {
		fmt.Fprintf(os.Stderr, "selftest case4 (no-default): unexpected result %+v\n", d)
		return false
	}

	// Case 5: stale serviceYANG mapping (service dropped from Go table) -> drift.
	d = compare(
		map[string]int{},
		map[string]string{"gone": "gone.yang"},
		read(map[string]string{"gone.yang": "refine port { default 1; }"}),
	)
	if len(d) != 1 || d[0].Reason != reasonStaleMap || d[0].Service != "gone" {
		fmt.Fprintf(os.Stderr, "selftest case5 (stale map): unexpected result %+v\n", d)
		return false
	}

	return true
}
