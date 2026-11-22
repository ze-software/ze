// Design: docs/architecture/route-selection.md -- next-hop admission before ranking
// Related: plugin_fixture_llgr_received.go -- the LLGR scenarios that seed their next hops through it
// Related: plugin_fixture_09.go -- lgInjectRoutes09, which seeds the looking glass next hop through it
// Related: sysrib_forwarding_capture_fixture.go -- sysribInstall, the route-install call it makes

package fixture

import (
	"context"
	"encoding/json"
	"fmt"
	"net/netip"
	"time"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// terminalCovers gives a scenario's unchanged BGP next hops independent
// terminal paths in the real system RIB: one static /32 (IPv4) or /128 (IPv6)
// per address, through a device and no gateway, so each address resolves at
// zero cost without recursion. A BGP route whose next hop does not resolve is
// excluded before best-path ranking, so a scenario that asserts a best path
// owes its next hop such a route. The device is route metadata, not a claim
// that a kernel interface exists or that a kernel route was installed: a
// scenario that uses this configures no FIB writer.
//
// The configuration MUST load the system RIB (`internal rib { use rib; }`):
// state reads each route back through its `show rib candidates` command.
type terminalCovers struct {
	label     string // names the scenario in every failure, such as "LLGR"
	device    string
	instance  uint32
	addresses []string
}

// terminalCoverSpec is one parsed host cover derived from a terminalCovers address.
type terminalCoverSpec struct {
	afi    uint16
	safi   uint8
	family family.Family
	prefix string
}

// terminalCoverRow is one prefix of the `show rib candidates prefix` answer.
type terminalCoverRow struct {
	Prefix string              `json:"prefix"`
	Family family.Family       `json:"family"`
	Paths  []terminalCoverPath `json:"paths"`
}

// terminalCoverPath is one candidate path of a terminalCoverRow.
type terminalCoverPath struct {
	Source    string          `json:"source"`
	Instance  uint32          `json:"instance"`
	Interface string          `json:"interface"`
	NextHop   json.RawMessage `json:"next-hop"`
	Metric    *uint32         `json:"metric"`
	Selected  bool            `json:"selected"`
	Hidden    *bool           `json:"hidden"`
}

// parseTerminalCover validates one address and derives its host prefix and family.
func parseTerminalCover(label, address string) (terminalCoverSpec, error) {
	addr, err := netip.ParseAddr(address)
	if err != nil {
		return terminalCoverSpec{}, fmt.Errorf("parse %s next-hop cover %q: %w", label, address, err)
	}
	if addr.Zone() != "" {
		return terminalCoverSpec{}, fmt.Errorf("%s next-hop cover %q must not carry a zone", label, address)
	}
	if addr.Is4In6() {
		return terminalCoverSpec{}, fmt.Errorf("%s next-hop cover %q must be plain IPv4 or IPv6", label, address)
	}
	if addr.Is4() {
		return terminalCoverSpec{
			afi:    1,
			safi:   1,
			family: family.IPv4Unicast,
			prefix: addr.String() + suffixHostV4,
		}, nil
	}
	return terminalCoverSpec{
		afi:    2,
		safi:   1,
		family: family.IPv6Unicast,
		prefix: addr.String() + suffixHostV6,
	}, nil
}

// specs parses every address into its typed IPv4 /32 or IPv6 /128 cover.
func (covers terminalCovers) specs() ([]terminalCoverSpec, error) {
	specs := make([]terminalCoverSpec, len(covers.addresses))
	for i, address := range covers.addresses {
		spec, err := parseTerminalCover(covers.label, address)
		if err != nil {
			return nil, err
		}
		specs[i] = spec
	}
	return specs, nil
}

// send installs the parsed covers through the engine route-install RPC.
func (covers terminalCovers) send(ctx context.Context, p *sdk.Plugin, specs []terminalCoverSpec) error {
	entries := make([]rpc.RouteInstallEntry, len(specs))
	for i, spec := range specs {
		entries[i] = rpc.RouteInstallEntry{Protocol: protocolStatic,
			Instance: covers.instance, AFI: spec.afi, SAFI: spec.safi, Prefix: spec.prefix,
			Interface: covers.device, RouteType: 1}
	}
	if err := sysribInstall(ctx, p, entries...); err != nil {
		return fmt.Errorf("install %s next-hop covers: %w", covers.label, err)
	}
	return nil
}

// seed sends the covers through the engine route-install RPC and waits until
// the engine resolves every address through them without cross-plugin dispatch,
// so a scenario can install covers in OnStarted before BGP peers dial.
func (covers terminalCovers) seed(ctx context.Context, p *sdk.Plugin) error {
	specs, err := covers.specs()
	if err != nil {
		return err
	}
	if err := covers.send(ctx, p, specs); err != nil {
		return err
	}
	last := fmt.Errorf("terminal %s covers have not resolved", covers.label)
	if !Poll(ctx, 20, 100*time.Millisecond, func() bool {
		last = covers.metricsState(ctx, p)
		return last == nil
	}) {
		return fmt.Errorf("%s next-hop covers were not resolved: %w", covers.label, last)
	}
	return nil
}

// install sends the covers through the route-install RPC, then waits until the
// system RIB selects each one and the engine resolves each address through it.
func (covers terminalCovers) install(ctx context.Context, p *sdk.Plugin) error {
	specs, err := covers.specs()
	if err != nil {
		return err
	}
	if err := covers.send(ctx, p, specs); err != nil {
		return err
	}
	last := fmt.Errorf("terminal %s covers have not been observed", covers.label)
	if !Poll(ctx, 20, 100*time.Millisecond, func() bool {
		last = covers.state(ctx, p, specs)
		return last == nil
	}) {
		return fmt.Errorf("%s next-hop covers were not ready: %w", covers.label, last)
	}
	return nil
}

// state answers nil once every cover is the one selected, visible path for its
// host prefix (/32 or /128), carrying exactly what install sent, and the engine
// resolves every address through it: resolved, not recursive, at zero cost.
func (covers terminalCovers) state(ctx context.Context, p *sdk.Plugin, specs []terminalCoverSpec) error {
	for _, spec := range specs {
		if err := covers.pathState(ctx, p, spec); err != nil {
			return err
		}
	}
	return covers.metricsState(ctx, p)
}

// metricsState verifies that the engine resolves every address non-recursively
// at zero cost.
func (covers terminalCovers) metricsState(ctx context.Context, p *sdk.Plugin) error {
	metrics, err := p.RouteMetrics(ctx, covers.addresses)
	if err != nil {
		return err
	}
	if len(metrics.Distances) != len(covers.addresses) {
		return fmt.Errorf("%s topology returned %d metrics, want %d", covers.label, len(metrics.Distances), len(covers.addresses))
	}
	for i, address := range covers.addresses {
		if metrics.Distances[i] != (rpc.RouteMetric{Resolved: true}) {
			return fmt.Errorf("%s next hop %s has metric %+v, want resolved, nonrecursive and zero cost", covers.label, address, metrics.Distances[i])
		}
	}
	return nil
}

// pathState reads one cover back from the system RIB.
func (covers terminalCovers) pathState(ctx context.Context, p *sdk.Plugin, spec terminalCoverSpec) error {
	var rows []terminalCoverRow
	status, err := Dispatch(ctx, p, cmdShowRIBCandidatesPrefix+spec.prefix, &rows)
	if err != nil {
		return err
	}
	if status != rpc.StatusDone {
		return fmt.Errorf("%s cover %s inspection answered %q instead of done", covers.label, spec.prefix, status)
	}
	if len(rows) != 1 {
		return fmt.Errorf("%s cover %s inspection returned %d prefixes, want one", covers.label, spec.prefix, len(rows))
	}
	row := &rows[0]
	if row.Prefix != spec.prefix {
		return fmt.Errorf("%s cover inspection returned prefix %q, want %s", covers.label, row.Prefix, spec.prefix)
	}
	if row.Family != spec.family {
		return fmt.Errorf("%s cover %s inspection returned family %s, want %s", covers.label, spec.prefix, row.Family, spec.family)
	}
	if len(row.Paths) != 1 {
		return fmt.Errorf("%s cover %s inspection returned %d paths, want one", covers.label, spec.prefix, len(row.Paths))
	}
	path := &row.Paths[0]
	if path.Source != protocolStatic {
		return fmt.Errorf("%s cover %s source is %q, want static", covers.label, spec.prefix, path.Source)
	}
	if path.Instance != covers.instance {
		return fmt.Errorf("%s cover %s instance is %d, want %d", covers.label, spec.prefix, path.Instance, covers.instance)
	}
	if path.Interface != covers.device {
		return fmt.Errorf("%s cover %s device is %q, want %s", covers.label, spec.prefix, path.Interface, covers.device)
	}
	if len(path.NextHop) != 0 {
		return fmt.Errorf("%s terminal cover %s unexpectedly names a gateway", covers.label, spec.prefix)
	}
	if path.Metric == nil {
		return fmt.Errorf("%s cover %s inspection omitted its metric", covers.label, spec.prefix)
	}
	if *path.Metric != 0 {
		return fmt.Errorf("%s cover %s metric is %d, want zero", covers.label, spec.prefix, *path.Metric)
	}
	if !path.Selected {
		return fmt.Errorf("%s cover %s is not selected", covers.label, spec.prefix)
	}
	if path.Hidden == nil {
		return fmt.Errorf("%s cover %s inspection omitted its hidden state", covers.label, spec.prefix)
	}
	if *path.Hidden {
		return fmt.Errorf("%s cover %s is hidden", covers.label, spec.prefix)
	}
	return nil
}
