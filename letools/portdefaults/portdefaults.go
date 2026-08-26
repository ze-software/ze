// Design: docs/architecture/doctor-and-health-checks.md -- listener default port table
//
// Package portdefaults pins the hand-maintained Go listener-default table
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
//     anycast listeners), which needs per-listener disambiguation not modeled
//     here.
//   - RegisterListenerDefaultIPs, the dual-stack kind. It names SEVERAL
//     addresses against a single `refine ip`, so there is no one-to-one value to
//     compare, and its only caller today is geodns, which the paragraph above
//     already excludes for a reason of its own.
//     In the central table it is reported as unknown-registration rather than
//     read, so adopting it centrally is a decision someone has to make and not a
//     way to fall out of coverage.
//   - A service whose refine and daemon deliberately DISAGREE. This gate pins
//     agreement, so it has nothing to say about a divergence that is known and
//     recorded, and it carries no allowlist to say it with: absence from
//     serviceYANG is how such a service is expressed, and the comment on the map
//     says why. See the mcp entry that is not there.

package portdefaults

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/ze-software/ze/letools/leaction"
	"github.com/ze-software/ze/letools/lepath"
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
	// refine port { default 3443; } is the shape this reads.
	refinePortRe = regexp.MustCompile(`refine\s+port\s*\{\s*default\s+(\d+)\s*;`)
)

// knownRegistrations are the call spellings goEntryRe can read. A call in the
// central table that is not one of these is reported as unknown-registration.
var knownRegistrations = map[string]bool{
	"RegisterListenerDefault(":      true,
	"RegisterListenerEntryDefault(": true,
}

// readFile is the shape compare() takes its YANG text through, so the selftest
// can supply synthetic module content in place of a tree.
type readFile func(string) (string, error)

// Check reads tree and answers every drift between the Go listener table and
// the YANG refine defaults.
//
// The error is about the READ rather than about the tree: the central table
// itself could not be read or holds a port that is not a number. A YANG module
// that cannot be read is a DRIFT rather than an error, because a mapped module
// that is gone is exactly the disagreement this gate is about.
func Check(tree string) (Result, error) {
	table, err := parseGoTable(filepath.Join(tree, goTablePath))
	if err != nil {
		return Result{}, fmt.Errorf("read Go table: %w", err)
	}

	read := func(rel string) (string, error) { return readFileString(filepath.Join(tree, rel)) }
	result := Result{Drifts: compare(table, serviceYANG, read)}

	content, err := read(goTablePath)
	if err != nil {
		return Result{}, fmt.Errorf("read Go table: %w", err)
	}
	for _, call := range unknownRegistrations(content) {
		result.Drifts = append(result.Drifts, Drift{
			Service: call, GoPort: -1, YANGPort: -1, File: goTablePath, Reason: ReasonUnknownReg,
		})
	}

	result.Checked = len(table)
	result.Valid = len(result.Drifts) == 0
	return result, nil
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
	for line := range strings.SplitSeq(content, "\n") {
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
	for _, match := range goEntryRe.FindAllStringSubmatch(content, -1) {
		port, convErr := strconv.Atoi(match[2])
		if convErr != nil {
			return nil, fmt.Errorf("service %q: bad port %q: %w", match[1], match[2], convErr)
		}
		table[match[1]] = port
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
// read is injected so the selftest can supply synthetic module content.
func compare(goTable map[string]int, yangMap map[string]string, read readFile) []Drift {
	var drifts []Drift

	services := make([]string, 0, len(goTable))
	for service := range goTable {
		services = append(services, service)
	}
	sort.Strings(services)

	for _, service := range services {
		goPort := goTable[service]
		module, ok := yangMap[service]
		if !ok {
			drifts = append(drifts, Drift{Service: service, GoPort: goPort, YANGPort: -1, Reason: ReasonUnmapped})
			continue
		}
		content, err := read(module)
		if err != nil {
			drifts = append(drifts, Drift{Service: service, GoPort: goPort, YANGPort: -1, File: module, Reason: ReasonUnreadable})
			continue
		}
		yangPort, found := yangPortDefault(content)
		if !found {
			drifts = append(drifts, Drift{Service: service, GoPort: goPort, YANGPort: -1, File: module, Reason: ReasonNoDefault})
			continue
		}
		if yangPort != goPort {
			drifts = append(drifts, Drift{Service: service, GoPort: goPort, YANGPort: yangPort, File: module, Reason: ReasonMismatch})
		}
	}

	// A stale serviceYANG entry (module mapped but the service was dropped from
	// the Go table) is also drift -- the map must track the table exactly.
	extras := make([]string, 0)
	for service := range yangMap {
		if _, ok := goTable[service]; !ok {
			extras = append(extras, service)
		}
	}
	sort.Strings(extras)
	for _, service := range extras {
		drifts = append(drifts, Drift{Service: service, GoPort: -1, YANGPort: -1, File: yangMap[service], Reason: ReasonStaleMap})
	}

	return drifts
}

func readFileString(path string) (string, error) {
	body, err := os.ReadFile(path) //nolint:gosec // fixed in-repo paths under the tree lepath answers
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// runCheck is the `check` action: read the checkout and answer what drifted.
func runCheck() (any, int) {
	tree, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}

	result, err := Check(tree)
	if err != nil {
		// 2 rather than 1: a read that did not complete is a different fact
		// from a table that drifted.
		leaction.ReportError(err)
		return nil, 2
	}
	if !result.Valid {
		return result, 1
	}
	return result, 0
}
