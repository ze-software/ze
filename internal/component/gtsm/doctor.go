// Design: docs/architecture/doctor-and-health-checks.md -- the readiness check this component owns
// Overview: register.go -- the init() that installs the registration below
// Related: route_linux.go, route_other.go -- hopLimitRouteInstalled and peerRouteResolvable, per platform
//
// A GTSM peer's related ICMP messages are protected by two pieces of kernel
// state this package installs (gtsm.go): the host route carrying hop limit
// 255, and the ze_gtsm table. Both are installed at config apply and both can
// fail there, most often because the peer's interface is not up yet, and the
// daemon then says so on one log line. This check is what asks the kernel
// afterwards, so an operator running `show doctor` sees which peer is short
// of which half.
//
// It answers two different questions, because it runs in two different
// processes and the same question cannot be answered in both:
//
//   - Inside the daemon (`show doctor`, the support bundle), SetPeers has run,
//     so the set this daemon asked the kernel for is known, and the check reads
//     whether the kernel holds it. This is the installed-state question.
//   - In `ze doctor`, an operator's own process, nothing has been published,
//     and the kernel state is legitimately absent before the daemon starts.
//     The check derives the GTSM peer set from the configuration instead, and
//     asks the one thing that can be known before a start: whether the kernel
//     resolves a route to each peer, which is what the route install needs.
//     Reading installed state here would warn on every box before every
//     start, and a warning an operator learns to ignore protects nobody.

package gtsm

import (
	"errors"
	"net/netip"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/infra"
	"github.com/ze-software/ze/internal/component/firewall"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// doctorKernelStateCode names a GTSM peer whose kernel state is missing, or
// could not be read, or cannot be installed.
const doctorKernelStateCode = "doctor-gtsm-kernel-state"

// ConfigPeersFunc derives the GTSM peer set from a resolved bgp{} tree, the
// map infra.ResolveBGPTree answers with. The BGP reactor owns that derivation,
// because it is the reactor's own peer parse that decides which peer carries
// a `ttl` block and which port its session uses, and a second reading of the
// configuration here would be a second declaration of the same fact.
type ConfigPeersFunc func(bgpTree map[string]any) ([]Peer, error)

// configPeers is the seam the reactor fills at init. It stays nil in a binary
// with no BGP engine, and a bgp{} block cannot exist in that binary either,
// because its schema is gated by the same tag.
var configPeers ConfigPeersFunc

// SetConfigPeers installs the reactor's derivation. Called once, from the
// reactor package's init.
func SetConfigPeers(fn ConfigPeersFunc) { configPeers = fn }

// readKernelTables reads the ze-owned tables the kernel holds, through the
// firewall backend the daemon loaded. It is a var so the unit test can stand
// in a kernel; nothing else assigns it.
var readKernelTables = func() ([]firewall.Table, error) {
	backend := firewall.GetBackend()
	if backend == nil {
		return nil, errNoFirewallBackend
	}
	return backend.ListTables()
}

var errNoFirewallBackend = errors.New("no firewall backend is loaded, so the ze_gtsm table was never applied")

// kernelStateDoctorCheck is the registration register.go installs.
//
// Order 2220 puts it after the BGP cache and collector probes (2200, 2210)
// and before the writable-destination check (2270), with the other checks
// that read what a running configuration reaches.
func kernelStateDoctorCheck() diagnostic.DoctorCheck {
	return diagnostic.DoctorCheck{
		Name:         "gtsm-kernel-state",
		Phase:        diagnostic.DoctorPhasePostConfig,
		Order:        2220,
		Component:    "gtsm",
		Dependencies: []string{"config-tree", "netlink"},
		Platforms:    []string{diagnostic.DoctorPlatformAny},
		Codes:        []string{doctorKernelStateCode},
		Check:        checkKernelState,
	}
}

// checkKernelState is the check function. The file header says which of the
// two questions it answers and why.
//
// A nil tree is the missing-config phase, which this check does not run in,
// and a context carrying anything else is a runner defect the runner's own
// type assertion reports (doctorTree, internal/component/doctor/registry.go).
func checkKernelState(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	tree, ok := ctx.Tree.(*config.Tree)
	if !ok || tree == nil {
		return nil
	}
	if tree.GetContainer("bgp") == nil {
		return nil
	}

	peers, inDaemon := publishedPeers()
	if inDaemon {
		return installedStateDiagnostics(peers)
	}

	peers, err := peersFromConfigTree(tree)
	if err != nil {
		return []diagnostic.Diagnostic{{
			Code:     doctorKernelStateCode,
			Severity: diagnostic.SeverityWarning,
			Message:  "gtsm: the GTSM peer set could not be derived from the configuration: " + err.Error(),
		}}
	}
	return resolvableRouteDiagnostics(peers)
}

// peersFromConfigTree resolves the bgp{} section the way the engine does and
// hands the map to the reactor's derivation.
//
// The tree is CLONED before it is pruned: PruneInactive modifies in place, and
// the doctor runner shares one tree across every later check.
func peersFromConfigTree(tree *config.Tree) ([]Peer, error) {
	if configPeers == nil {
		return nil, errNoPeerDerivation
	}
	schema, err := config.YANGSchema()
	if err != nil {
		return nil, err
	}
	clone := tree.Clone()
	config.PruneInactive(clone, schema)
	bgpTree, err := infra.ResolveBGPTree(clone)
	if err != nil {
		return nil, err
	}
	return configPeers(bgpTree)
}

var errNoPeerDerivation = errors.New("no BGP engine in this binary derives the GTSM peer set")

// installedStateDiagnostics is the in-daemon answer: one diagnostic for each
// published peer whose kernel state is short of what SetPeers asked for.
//
// The table is read once for the whole set, and only when a peer owes terms,
// so a deployment whose GTSM peers are all IPv6 never touches the firewall
// backend here, exactly as SetPeers never loads one for it. A table read that
// fails is reported once, and no peer is then also reported as missing the
// table, because nothing was learned about it.
func installedStateDiagnostics(peers []Peer) []diagnostic.Diagnostic {
	var diags []diagnostic.Diagnostic

	tableKnown := false
	tablePresent := false
	if filterTables(peers) != nil {
		tables, err := readKernelTables()
		if err != nil {
			diags = append(diags, diagnostic.Diagnostic{
				Code:     doctorKernelStateCode,
				Severity: diagnostic.SeverityWarning,
				Message:  "gtsm: the kernel firewall tables could not be read: " + err.Error(),
			})
		} else {
			tableKnown = true
			tablePresent = kernelHoldsFilterTable(tables)
		}
	}

	for _, p := range peers {
		var missing []string
		if p.HopLimit != 0 {
			if reason, absent := hopLimitRouteMissing(p); absent {
				missing = append(missing, reason)
			}
		}
		if tableKnown && !tablePresent && len(peerTerms(p)) > 0 {
			missing = append(missing, "the "+filterTableName+" table")
		}
		if len(missing) == 0 {
			continue
		}
		diags = append(diags, missingStateDiagnostic(p.Addr, missing))
	}
	return diags
}

// kernelHoldsFilterTable reports whether the kernel's ze-owned tables include
// this package's table, in the family it is published in.
func kernelHoldsFilterTable(tables []firewall.Table) bool {
	for i := range tables {
		if tables[i].Name == filterTableName && tables[i].Family == firewall.FamilyInet {
			return true
		}
	}
	return false
}

// hopLimitRouteMissing answers what to say about a peer whose route is not
// what SetPeers asked for: the route is absent or carries another hop limit,
// or the kernel could not be asked. A route that is there answers nothing.
func hopLimitRouteMissing(p Peer) (string, bool) {
	installed, err := hopLimitRouteInstalled(p)
	if err != nil {
		return "the host route could not be read: " + err.Error(), true
	}
	if installed {
		return "", false
	}
	return "the host route carrying hop limit " + textbuf.StringUint8(p.HopLimit), true
}

// missingStateDiagnostic names the peer and each half of its state that the
// kernel does not hold.
func missingStateDiagnostic(addr netip.Addr, missing []string) diagnostic.Diagnostic {
	var tb textbuf.Buffer
	tb.Str("gtsm: peer ").Addr(addr).Str(": the kernel does not hold ").Join(missing, " or ").
		Str(", so its related ICMP messages are not protected until the next peer reconcile installs it")
	return diagnostic.Diagnostic{
		Code:     doctorKernelStateCode,
		Severity: diagnostic.SeverityWarning,
		Message:  tb.String(),
		Path:     textbuf.StringAddr(addr),
	}
}

// resolvableRouteDiagnostics is the offline answer: one diagnostic for each
// configured peer the kernel resolves no route to, because that is the
// install that will fail when the daemon applies this configuration.
func resolvableRouteDiagnostics(peers []Peer) []diagnostic.Diagnostic {
	var diags []diagnostic.Diagnostic
	for _, p := range peers {
		if p.HopLimit == 0 {
			continue
		}
		err := peerRouteResolvable(p.Addr)
		if err == nil {
			continue
		}
		var tb textbuf.Buffer
		tb.Str("gtsm: peer ").Addr(p.Addr).Str(": the kernel resolves no route to it (").Err(err).
			Str("), so the host route carrying hop limit ").Uint(uint64(p.HopLimit)).
			Str(" cannot be installed when the daemon applies this configuration")
		diags = append(diags, diagnostic.Diagnostic{
			Code:     doctorKernelStateCode,
			Severity: diagnostic.SeverityWarning,
			Message:  tb.String(),
			Path:     textbuf.StringAddr(p.Addr),
		})
	}
	return diags
}
