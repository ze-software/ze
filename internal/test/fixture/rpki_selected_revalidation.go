// Design: docs/architecture/plugin/rib-storage-design.md -- validation changes rerun selection.
// Related: plugin_fixture_15_rpki.go -- deferred RTR synchronization and retained-byte checks.
// Related: docs/architecture/rib/unified-locrib.md -- authoritative candidates and selected views.
package fixture

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const (
	rpkiSelectedSubject = "10.0.1.0/24"
	rpkiSelectedControl = "10.0.2.0/24"
	rpkiSelectedNextHop = "10.0.0.1"
	rpkiSelectedRelease = "rpki-selected-sync.release"
	rpkiSelectedLink    = "zerpkisel"
	rpkiSelectedAttrHex = "attr-hex"
	rpkiSelectedNHopHex = "nhop-hex"
	rpkiSelectedNLRIHex = "nlri-hex"
)

func init() {
	Register("plugin/rpki-revalidate-selected-route", plugin15Observe("rpki-selected-revalidation-test", &sdk.Registration{}, rpkiSelectedRevalidation), StopsDaemon)
}

// rpkiSelectedRevalidation releases the first covering VRP only after both
// received routes are retained and selected through the real daemon. It then
// requires the Invalid route to leave BGP best and the authoritative Loc-RIB,
// without deleting its received bytes or the independently Valid control.
// This addition-only probe does not prove all of RFC 6811 Section 4.
func rpkiSelectedRevalidation(ctx context.Context, plugin *sdk.Plugin) error {
	if err := waitRIBWinner(ctx, plugin, "10.0.0.0/24", "connected"); err != nil {
		return fmt.Errorf("next-hop prerequisite: %w", err)
	}
	if _, err := waitCandidates(ctx, plugin, "10.0.0.0/24", func(row candidateRow) bool {
		path, found := candidatePathOf(row, "connected")
		return found && path.Selected && path.Interface == rpkiSelectedLink
	}); err != nil {
		return fmt.Errorf("connected namespace device prerequisite: %w", err)
	}
	before, err := rpkiSelectedRetained(ctx, plugin, "before synchronization", 2, false, 2)
	if err != nil {
		return err
	}
	if err := rpkiSelectedWait(ctx, plugin, true, "before synchronization"); err != nil {
		return err
	}
	if count := plugin15RPKICount(ctx, plugin); count != 0 {
		return fmt.Errorf("pre-sync VRP count is %v, want zero before releasing the mapping", count)
	}
	if err := os.WriteFile(rpkiSelectedRelease, nil, 0o600); err != nil {
		return fmt.Errorf("release the selected-route RTR synchronization: %w", err)
	}
	if !Poll(ctx, 40, 100*time.Millisecond, func() bool { return plugin15RPKICount(ctx, plugin) == 1 }) {
		return errors.New("the deferred RTR cache never published its one covering VRP")
	}
	after, err := rpkiSelectedRetained(ctx, plugin, "after synchronization", 3, true, 1)
	if err != nil {
		return err
	}
	for i, prefix := range []string{rpkiSelectedSubject, rpkiSelectedControl} {
		if after[i] != before[i] {
			return fmt.Errorf("revalidation changed received bytes for %s: before=%+v after=%+v", prefix, before[i], after[i])
		}
	}
	if err := rpkiSelectedWait(ctx, plugin, false, "after synchronization"); err != nil {
		return err
	}
	if err := plugin15WaitEOR(ctx, plugin, "peer1"); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "OK: VRP addition removes the retained Invalid route from BGP best and Loc-RIB while the Valid control stays selected")
	return nil
}

type rpkiSelectedWire struct {
	attributes string
	nextHop    string
	nlri       string
}

// rpkiSelectedRetained requires both exact verdicts and nonempty immutable
// wire fields. A missing route or a malformed command answer cannot pass.
func rpkiSelectedRetained(ctx context.Context, plugin *sdk.Plugin, phase string, subjectState int, subjectIneligible bool, controlState int) ([2]rpkiSelectedWire, error) {
	var snapshots [2]rpkiSelectedWire
	var last string
	if Poll(ctx, 80, 100*time.Millisecond, func() bool {
		raw, err := requireDone02(ctx, plugin, "show bgp adj-rib-in")
		if err != nil {
			last = err.Error()
			return false
		}
		data, err := map02(raw)
		if err != nil {
			last = err.Error()
			return false
		}
		last = text02(raw)
		if !plugin14RouteValidation(data, rpkiSelectedSubject, subjectState, subjectIneligible) {
			return false
		}
		if !plugin14RouteValidation(data, rpkiSelectedControl, controlState, false) {
			return false
		}
		for i, prefix := range []string{rpkiSelectedSubject, rpkiSelectedControl} {
			snapshots[i], err = rpkiSelectedRouteWire(plugin14FindRoute(data, prefix))
			if err != nil {
				last = err.Error()
				return false
			}
			if snapshots[i].nextHop != "0a000001" {
				last = "received next-hop bytes do not name " + rpkiSelectedNextHop
				return false
			}
			wantNLRI := "180a0001"
			if prefix == rpkiSelectedControl {
				wantNLRI = "180a0002"
			}
			if snapshots[i].nlri != wantNLRI {
				last = "received NLRI bytes do not name " + prefix
				return false
			}
		}
		return true
	}) {
		return snapshots, nil
	}
	return snapshots, fmt.Errorf("%s: retained subject must have state=%d ineligible=%t and control state=%d ineligible=false: %s", phase, subjectState, subjectIneligible, controlState, last)
}

func rpkiSelectedRouteWire(route map[string]any) (rpkiSelectedWire, error) {
	var wire rpkiSelectedWire
	for key, target := range map[string]*string{rpkiSelectedAttrHex: &wire.attributes, rpkiSelectedNHopHex: &wire.nextHop, rpkiSelectedNLRIHex: &wire.nlri} {
		value, ok := route[key].(string)
		if !ok {
			return wire, fmt.Errorf("retained route has no string %s", key)
		}
		if value == "" {
			return wire, fmt.Errorf("retained route has empty %s", key)
		}
		*target = value
	}
	return wire, nil
}

// rpkiSelectedWait reads three independent selection surfaces. The positive
// control must remain selected in each, so a disconnected or empty RIB cannot
// satisfy the subject's negative phase.
func rpkiSelectedWait(ctx context.Context, plugin *sdk.Plugin, subjectSelected bool, phase string) error {
	last := errors.New("no complete selection snapshot read")
	if Poll(ctx, 80, 100*time.Millisecond, func() bool {
		if last = rpkiSelectedBest(ctx, plugin, subjectSelected); last != nil {
			return false
		}
		if last = rpkiSelectedCandidates(ctx, plugin, subjectSelected); last != nil {
			return false
		}
		last = rpkiSelectedView(ctx, plugin, subjectSelected)
		return last == nil
	}) {
		return nil
	}
	return fmt.Errorf("%s: selected-route revalidation did not reach subject-selected=%t with its control still selected: %w", phase, subjectSelected, last)
}

func rpkiSelectedBest(ctx context.Context, plugin *sdk.Plugin, subjectSelected bool) error {
	raw, err := requireDone02(ctx, plugin, "show bgp rib best")
	if err != nil {
		return err
	}
	var answer struct {
		Paths []struct {
			Prefix string `json:"prefix"`
			Peer   string `json:"best-peer"`
		} `json:"best-path"`
	}
	if err := decode02(raw, &answer); err != nil {
		return err
	}
	if answer.Paths == nil {
		return fmt.Errorf("show bgp rib best has no path list: %s", text02(raw))
	}
	subject, control := false, false
	for _, path := range answer.Paths {
		if path.Prefix == rpkiSelectedSubject {
			subject = true
			if path.Peer != addrLoopback {
				return fmt.Errorf("subject best peer is %q, want its actual ingress peer", path.Peer)
			}
		}
		if path.Prefix == rpkiSelectedControl {
			control = path.Peer == addrLoopback
		}
	}
	if subject != subjectSelected {
		return fmt.Errorf("BGP best subject presence=%t, want %t: %s", subject, subjectSelected, text02(raw))
	}
	if !control {
		return fmt.Errorf("BGP best lost its accepted control: %s", text02(raw))
	}
	return nil
}

func rpkiSelectedCandidates(ctx context.Context, plugin *sdk.Plugin, subjectSelected bool) error {
	raw, err := requireDone02(ctx, plugin, "show rib candidates")
	if err != nil {
		return err
	}
	var rows []candidateRow
	if err := decode02(raw, &rows); err != nil {
		return err
	}
	if rows == nil {
		return fmt.Errorf("show rib candidates has no candidate list: %s", text02(raw))
	}
	// A successful authority read and the selected control distinguish absence
	// from a failed lookup. Ineligible input may remain an unselected candidate.
	subject, control := false, false
	for _, row := range rows {
		path, found := candidatePathOf(row, importSourceBGP)
		if !found {
			continue
		}
		if row.Prefix != rpkiSelectedSubject {
			if row.Prefix != rpkiSelectedControl {
				continue
			}
		}
		if path.Selected {
			if path.Hidden {
				return fmt.Errorf("selected %s is hidden in Loc-RIB candidates", row.Prefix)
			}
			if path.NextHop != rpkiSelectedNextHop {
				return fmt.Errorf("selected %s next hop is %q, want %s", row.Prefix, path.NextHop, rpkiSelectedNextHop)
			}
		}
		if row.Prefix == rpkiSelectedSubject {
			subject = path.Selected
		}
		if row.Prefix == rpkiSelectedControl {
			control = path.Selected
		}
	}
	if subject != subjectSelected {
		return fmt.Errorf("authoritative Loc-RIB subject-selected=%t, want %t: %s", subject, subjectSelected, text02(raw))
	}
	if !control {
		return fmt.Errorf("authoritative Loc-RIB lost its selected control: %s", text02(raw))
	}
	return nil
}

func rpkiSelectedView(ctx context.Context, plugin *sdk.Plugin, subjectSelected bool) error {
	raw, err := requireDone02(ctx, plugin, "show rib")
	if err != nil {
		return err
	}
	var rows []struct {
		Prefix   string `json:"prefix"`
		Protocol string `json:"protocol"`
		NextHop  string `json:"next-hop"`
	}
	if err := decode02(raw, &rows); err != nil {
		return err
	}
	if rows == nil {
		return fmt.Errorf("show rib has no selected-route list: %s", text02(raw))
	}
	subject, control := false, false
	for _, row := range rows {
		if row.Prefix != rpkiSelectedSubject {
			if row.Prefix != rpkiSelectedControl {
				continue
			}
		}
		if row.Protocol != importSourceBGP {
			return fmt.Errorf("selected %s protocol is %q, want bgp", row.Prefix, row.Protocol)
		}
		if row.NextHop != rpkiSelectedNextHop {
			return fmt.Errorf("selected %s next hop is %q, want %s", row.Prefix, row.NextHop, rpkiSelectedNextHop)
		}
		if row.Prefix == rpkiSelectedSubject {
			subject = true
		}
		if row.Prefix == rpkiSelectedControl {
			control = true
		}
	}
	if subject != subjectSelected {
		return fmt.Errorf("system selected view subject presence=%t, want %t: %s", subject, subjectSelected, text02(raw))
	}
	if !control {
		return fmt.Errorf("system selected view lost its accepted control: %s", text02(raw))
	}
	return nil
}
