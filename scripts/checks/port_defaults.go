// Design: docs/architecture/doctor-and-health-checks.md -- listener default port table
//
// port_defaults pins the hand-maintained Go listener-default table
// (internal/component/config/listener_defaults.go, RegisterBuiltinListenerDefaults)
// to the `refine port { default N }` values in each service's YANG module. The
// Ze YANG compiler does not propagate refine defaults into the runtime schema,
// so the Go table restates them by hand; without this gate the two silently
// drift (someone edits the YANG default but not the Go table, or vice versa)
// and the daemon binds a port the schema documents differently.
//
// Scope: the central table in listener_defaults.go, in BOTH of the spellings it
// uses -- RegisterListenerDefault and RegisterListenerEntryDefault. The two
// differ only in whether an empty server list also listens, and both restate a
// `refine port` by hand, which is the thing that drifts. Only the PORT is
// compared; the ip literal beside it is not, for any service.
//
// A registration spelling this gate cannot read is refused rather than skipped
// (unknownRegistrations, reason unknown-registration). That branch exists
// because the silent version already happened: RegisterListenerEntryDefault was
// added to the central table and the gate went on checking the other services,
// green, with l2tp unread.
//
// Three things are outside this gate, and each is refused or absent on purpose
// rather than unnoticed:
//
//   - Per-plugin registrations (as112, geodns) live in their own register.go,
//     which this gate never reads. Their service names are schema-path-derived
//     and a single module can carry several `refine port` blocks (as112 has two
//     anycast listeners), which needs per-listener disambiguation not modelled
//     here.
//   - RegisterListenerDefaultIPs, the dual-stack kind. It names SEVERAL
//     addresses against a single `refine ip`, so there is no one-to-one value to
//     compare, and its only caller today is geodns, already out of scope above.
//     In the central table it is reported as unknown-registration rather than
//     read, so adopting it centrally is a decision someone has to make and not a
//     way to fall out of coverage.
//   - A service whose refine and daemon deliberately DISAGREE. This gate pins
//     agreement, so it has nothing to say about a divergence that is known and
//     recorded, and it carries no allowlist to say it with: absence from
//     serviceYANG is how such a service is expressed, and the comment on the map
//     says why. See the mcp entry that is not there.
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
	"strings"
)

// serviceYANG maps each central listener service (registered in
// listener_defaults.go) to the YANG module carrying its refine port default.
// The association (service <-> module) is structural; the port VALUE is read
// from the YANG at check time and compared to the Go table, so a drift in
// either source is caught.
//
// mcp is DELIBERATELY absent, and re-adding it turns this gate red. Its module
// declares `refine port { default 8080; }` and the daemon never applies it:
// extractMCPBlock (internal/component/config/loader_extract.go) passes an empty
// default port, so an mcp server entry that omits the port starts no listener at
// all. It is therefore not in RegisterBuiltinListenerDefaults either, and a
// mapping here would report that as stale-mapping drift. The divergence is real,
// open, and owned: plan/deferrals/mcp-port-default-divergence.md. Whoever closes
// it puts BOTH the Go registration and this line back in the same change.
var serviceYANG = map[string]string{
	"web":  "internal/component/web/yang/ze-web-conf.yang",
	"ssh":  "internal/component/ssh/yang/ze-ssh-conf.yang",
	"gnmi": "internal/component/gnmi/yang/ze-gnmi-conf.yang",
	// Registered with RegisterListenerEntryDefault rather than
	// RegisterListenerDefault, because an empty l2tp server list starts no
	// listener at all. The distinction does not matter here: the line still
	// restates `refine port { default 1701; }` by hand, which is what drifts.
	"l2tp":            "internal/component/l2tp/yang/ze-l2tp-conf.yang",
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
	reasonUnknownReg = "unknown-registration"
)

var (
	// RegisterListenerDefault("service", "ip", "port") and its entry-only
	// sibling RegisterListenerEntryDefault, which take the same three literals
	// and differ only in whether an EMPTY server list also listens. That
	// distinction is invisible to this gate: both restate a `refine port` by
	// hand, which is the only thing being pinned.
	//
	// The `\(` immediately after Default is load-bearing: it stops this matching
	// RegisterListenerDefaultIPs, whose second argument is a slice literal and
	// which unknownRegistrationRe refuses in this file instead.
	goEntryRe = regexp.MustCompile(`RegisterListener(?:Entry)?Default\("([^"]+)",\s*"[^"]*",\s*"(\d+)"\)`)
	// Any RegisterListener*( call, so a spelling this gate cannot read fails
	// loudly instead of being skipped. Round 5 added RegisterListenerEntryDefault
	// to the central table and the gate went on reporting the other seven, green
	// and one service short.
	unknownRegistrationRe = regexp.MustCompile(`\bRegisterListener[A-Za-z]*\(`)
	// refine port { default 3443; }
	refinePortRe = regexp.MustCompile(`refine\s+port\s*\{\s*default\s+(\d+)\s*;`)
)

// knownRegistrations are the call spellings goEntryRe can read. A call in the
// central table that is not one of these is reported as unknown-registration.
var knownRegistrations = map[string]bool{
	"RegisterListenerDefault(":      true,
	"RegisterListenerEntryDefault(": true,
}

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
	if content, rerr := readFileString(goTablePath); rerr == nil {
		for _, call := range unknownRegistrations(content) {
			res.Drifts = append(res.Drifts, drift{
				Service: call, GoPort: -1, YANGPort: -1, File: goTablePath, Reason: reasonUnknownReg,
			})
		}
	}
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

// unknownRegistrations returns the RegisterListener* spellings used in the
// central table that goEntryRe cannot read, so a registration kind added later
// fails this gate instead of quietly leaving its service unchecked.
//
// Comment lines are skipped: the file names these functions in prose to explain
// which one to use, and a mention is not a registration.
func unknownRegistrations(content string) []string {
	seen := map[string]bool{}
	var out []string
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "//") {
			continue
		}
		for _, call := range unknownRegistrationRe.FindAllString(line, -1) {
			if knownRegistrations[call] || seen[call] {
				continue
			}
			seen[call] = true
			out = append(out, call)
		}
	}
	sort.Strings(out)
	return out
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

	// Case 6: the reader sees EVERY registration kind the central table uses.
	// One case per spelling, because the gate reported seven services and stayed
	// green while RegisterListenerEntryDefault sat unread: a kind the regex
	// misses costs no drift, it costs a whole service.
	goSrc := `func RegisterBuiltinListenerDefaults() {
	RegisterListenerDefault("plain", "0.0.0.0", "111")
	RegisterListenerEntryDefault("entryonly", "0.0.0.0", "222")
	// see RegisterListenerDefaultIPs( for the dual-stack kind
}`
	table := map[string]int{}
	for _, m := range goEntryRe.FindAllStringSubmatch(goSrc, -1) {
		p, _ := strconv.Atoi(m[2])
		table[m[1]] = p
	}
	if len(table) != 2 || table["plain"] != 111 || table["entryonly"] != 222 {
		fmt.Fprintf(os.Stderr, "selftest case6 (both kinds read): got %+v\n", table)
		return false
	}

	// Case 7: the known kinds pass the unknown-spelling guard, and a PROSE
	// mention of an unreadable kind does not trip it -- the central table names
	// these functions in comments to say which one to use.
	if u := unknownRegistrations(goSrc); len(u) != 0 {
		fmt.Fprintf(os.Stderr, "selftest case7 (known kinds and comments): got %+v\n", u)
		return false
	}

	// Case 8: a spelling the reader cannot parse is REFUSED, not skipped.
	// RegisterListenerDefaultIPs is the live example: its second argument is a
	// slice, so there is no single ip literal to compare, and goEntryRe must not
	// half-match it either.
	dualSrc := `	RegisterListenerDefaultIPs("dual", []string{"127.0.0.1", "::1"}, "53")`
	unknown := unknownRegistrations(dualSrc)
	if len(unknown) != 1 || unknown[0] != "RegisterListenerDefaultIPs(" {
		fmt.Fprintf(os.Stderr, "selftest case8 (unknown kind refused): got %+v\n", unknown)
		return false
	}
	if len(goEntryRe.FindAllStringSubmatch(dualSrc, -1)) != 0 {
		fmt.Fprintln(os.Stderr, "selftest case8 (unknown kind refused): goEntryRe matched the dual-stack kind")
		return false
	}

	return true
}
