// Design: docs/architecture/route-selection.md -- next-hop admission before ranking
// Related: plugin_fixture_llgr_received.go -- the LLGR scenarios that seed their next hops through it
// Related: plugin_fixture_09.go -- lgInjectRoutes09, which seeds the looking glass next hop through it
// Related: sysrib_forwarding_capture_fixture.go -- sysribInstall, the route-install call it makes

package fixture

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// terminalCovers gives a scenario's unchanged BGP next hops independent
// terminal paths in the real system RIB: one static /32 per address, through a
// device and no gateway, so each address resolves at zero cost without
// recursion. A BGP route whose next hop does not resolve is excluded before
// best-path ranking, so a scenario that asserts a best path owes its next hop
// such a route. The device is route metadata, not a claim that a kernel
// interface exists or that a kernel route was installed: a scenario that uses
// this configures no FIB writer.
//
// The configuration MUST load the system RIB (`internal rib { use rib; }`):
// state reads each route back through its `show rib candidates` command.
type terminalCovers struct {
	label     string // names the scenario in every failure, such as "LLGR"
	device    string
	instance  uint32
	addresses []string
}

// install sends the covers through the route-install RPC, then waits until the
// system RIB selects each one and the engine resolves each address through it.
func (covers terminalCovers) install(ctx context.Context, p *sdk.Plugin) error {
	entries := make([]rpc.RouteInstallEntry, len(covers.addresses))
	for i, address := range covers.addresses {
		entries[i] = rpc.RouteInstallEntry{Protocol: protocolStatic,
			Instance: covers.instance, AFI: 1, SAFI: 1, Prefix: address + suffixHostV4,
			Interface: covers.device, RouteType: 1}
	}
	if err := sysribInstall(ctx, p, entries...); err != nil {
		return fmt.Errorf("install %s next-hop covers: %w", covers.label, err)
	}
	last := fmt.Errorf("terminal %s covers have not been observed", covers.label)
	if !Poll(ctx, 20, 100*time.Millisecond, func() bool {
		last = covers.state(ctx, p)
		return last == nil
	}) {
		return fmt.Errorf("%s next-hop covers were not ready: %w", covers.label, last)
	}
	return nil
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

// state answers nil once every cover is the one selected, visible path for its
// /32, carrying exactly what install sent, and the engine resolves every
// address through it: resolved, not recursive, at zero cost.
func (covers terminalCovers) state(ctx context.Context, p *sdk.Plugin) error {
	for _, address := range covers.addresses {
		if err := covers.pathState(ctx, p, address+suffixHostV4); err != nil {
			return err
		}
	}
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
func (covers terminalCovers) pathState(ctx context.Context, p *sdk.Plugin, prefix string) error {
	var rows []terminalCoverRow
	status, err := Dispatch(ctx, p, cmdShowRIBCandidatesPrefix+prefix, &rows)
	if err != nil {
		return err
	}
	if status != rpc.StatusDone {
		return fmt.Errorf("%s cover %s inspection answered %q instead of done", covers.label, prefix, status)
	}
	if len(rows) != 1 {
		return fmt.Errorf("%s cover %s inspection returned %d prefixes, want one", covers.label, prefix, len(rows))
	}
	row := &rows[0]
	if row.Prefix != prefix {
		return fmt.Errorf("%s cover inspection returned prefix %q, want %s", covers.label, row.Prefix, prefix)
	}
	if row.Family != family.IPv4Unicast {
		return fmt.Errorf("%s cover %s inspection returned family %s", covers.label, prefix, row.Family)
	}
	if len(row.Paths) != 1 {
		return fmt.Errorf("%s cover %s inspection returned %d paths, want one", covers.label, prefix, len(row.Paths))
	}
	path := &row.Paths[0]
	if path.Source != protocolStatic {
		return fmt.Errorf("%s cover %s source is %q, want static", covers.label, prefix, path.Source)
	}
	if path.Instance != covers.instance {
		return fmt.Errorf("%s cover %s instance is %d, want %d", covers.label, prefix, path.Instance, covers.instance)
	}
	if path.Interface != covers.device {
		return fmt.Errorf("%s cover %s device is %q, want %s", covers.label, prefix, path.Interface, covers.device)
	}
	if len(path.NextHop) != 0 {
		return fmt.Errorf("%s terminal cover %s unexpectedly names a gateway", covers.label, prefix)
	}
	if path.Metric == nil {
		return fmt.Errorf("%s cover %s inspection omitted its metric", covers.label, prefix)
	}
	if *path.Metric != 0 {
		return fmt.Errorf("%s cover %s metric is %d, want zero", covers.label, prefix, *path.Metric)
	}
	if !path.Selected {
		return fmt.Errorf("%s cover %s is not selected", covers.label, prefix)
	}
	if path.Hidden == nil {
		return fmt.Errorf("%s cover %s inspection omitted its hidden state", covers.label, prefix)
	}
	if *path.Hidden {
		return fmt.Errorf("%s cover %s is hidden", covers.label, prefix)
	}
	return nil
}
