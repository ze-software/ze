// Design: docs/architecture/route-selection.md -- retained input and selected output
// Related: rfc4271_prospective_daemon_fixture.go -- prospective admission topology
package fixture

import (
	"context"
	"errors"
	"fmt"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// rfc4271ProspectiveReceivedPath describes one expected public JSON input row.
// Its zero value is a comparison value only, never a valid expected wire path.
type rfc4271ProspectiveReceivedPath struct {
	Peer       string
	NextHop    string
	Preference uint32
}

func rfc4271ProspectiveInitialReceived() map[string]rfc4271ProspectiveReceivedPath {
	expected := make(map[string]rfc4271ProspectiveReceivedPath)
	for _, prefix := range []string{rfc4271ProspectiveDirect, rfc4271ProspectiveIndirect, rfc4271ProspectiveProtected} {
		expected[prefix] = rfc4271ProspectiveReceivedPath{Peer: rfc4271NextHopPeerA, NextHop: rfc4271NextHopDirect, Preference: 200}
	}
	expected[rfc4271NextHopControl] = rfc4271ProspectiveReceivedPath{Peer: rfc4271NextHopPeerB, NextHop: rfc4271NextHopDirect, Preference: 100}
	return expected
}

func rfc4271ProspectiveReceived(ctx context.Context, plugin *sdk.Plugin, expected map[string]rfc4271ProspectiveReceivedPath) error {
	var last string
	if Poll(ctx, importPollAttempts, importPollDelay, func() bool {
		data, err := requireDone02(ctx, plugin, "show bgp rib | received")
		if err != nil {
			last = err.Error()
			return false
		}
		last = text02(data)
		var answer struct {
			Routes []struct {
				Peer       string `json:"peer"`
				Prefix     string `json:"prefix"`
				NextHop    string `json:"next-hop"`
				Preference struct {
					Value uint32 `json:"value"`
				} `json:"local-preference"`
				Origin struct {
					Value string `json:"value"`
				} `json:"origin"`
				ASPath struct {
					Value []uint32 `json:"value"`
				} `json:"as-path"`
			} `json:"routes"`
		}
		if err := decode02(data, &answer); err != nil {
			last = err.Error()
			return false
		}
		if len(answer.Routes) != len(expected) {
			return false
		}
		seen := make(map[string]bool, len(expected))
		for _, row := range answer.Routes {
			want, found := expected[row.Prefix]
			if !found {
				return false
			}
			if seen[row.Prefix] {
				return false
			}
			seen[row.Prefix] = true
			if row.Peer != want.Peer {
				return false
			}
			if row.NextHop != want.NextHop {
				return false
			}
			if row.Preference.Value != want.Preference {
				return false
			}
			if row.Origin.Value != "igp" {
				return false
			}
			if len(row.ASPath.Value) != 1 {
				return false
			}
			if row.ASPath.Value[0] != 65002 {
				return false
			}
		}
		return true
	}) {
		return nil
	}
	return fmt.Errorf("received wire paths differ from exact prefix/peer/NEXT_HOP/LOCAL_PREF/ORIGIN/AS_PATH expectations %+v: %s", expected, last)
}

func rfc4271ProspectiveResolved(ctx context.Context, plugin *sdk.Plugin, address string) error {
	metrics, err := plugin.RouteMetrics(ctx, []string{address})
	if err != nil {
		return err
	}
	if !metrics.Distances[0].Resolved {
		return fmt.Errorf("ordinary pre-replacement next-hop resolution for %s is false: %+v", address, metrics.Distances[0])
	}
	return nil
}

func rfc4271ProspectiveAbsent(ctx context.Context, plugin *sdk.Plugin, prefix string) error {
	if err := rfc4271NextHopBest(ctx, plugin, prefix, ""); err != nil {
		return err
	}
	var last string
	if !Poll(ctx, importPollAttempts, importPollDelay, func() bool {
		data, err := requireDone02(ctx, plugin, "show rib candidates prefix "+prefix)
		if err != nil {
			last = err.Error()
			return false
		}
		last = text02(data)
		var rows []candidateRow
		if err := decode02(data, &rows); err != nil {
			last = err.Error()
			return false
		}
		return len(rows) == 0
	}) {
		return fmt.Errorf("prospective subject %s remained in Loc-RIB candidates: %s", prefix, last)
	}
	return importKernel(ctx, prefix, 0)
}

func rfc4271ProspectiveSurvivors(ctx context.Context, plugin *sdk.Plugin, protected bool) error {
	if err := rfc4271NextHopSurvivor(ctx, plugin); err != nil {
		return err
	}
	for _, prefix := range []string{rfc4271ProspectiveCover, rfc4271ProspectiveHost} {
		if err := waitRIBWinner(ctx, plugin, prefix, protocolOSPF); err != nil {
			return err
		}
		if err := importKernel(ctx, prefix, 1, "via "+rfc4271ProspectiveGateway+" ", "dev "+rfc4271NextHopLink+" ", "proto 250 "); err != nil {
			return err
		}
	}
	if !protected {
		return nil
	}
	if err := waitRIBWinner(ctx, plugin, rfc4271ProspectiveRecursive, protocolOSPF); err != nil {
		return err
	}
	if err := importKernel(ctx, rfc4271ProspectiveRecursive, 1, "via "+rfc4271ProspectiveGateway+" ", "dev "+rfc4271NextHopLink+" ", "proto 250 "); err != nil {
		return err
	}
	return rfc4271NextHopWinner(ctx, plugin, rfc4271ProspectiveProtected, rfc4271NextHopPeerA, rfc4271ProspectiveProtectedHop, rfc4271ProspectiveGateway)
}

// Every topology prefix is listed here, including the recursive interior and
// protective /32. Old cleanup readers intentionally retain their original scope.
func rfc4271ProspectivePrefixes() []string {
	return []string{rfc4271ProspectiveDirect, rfc4271ProspectiveIndirect, rfc4271ProspectiveProtected,
		rfc4271NextHopControl, rfc4271ProspectiveCover, rfc4271ProspectiveHost, rfc4271ProspectiveRecursive}
}

func rfc4271ProspectiveKernelEmpty(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("prospective kernel-empty fixture takes no arguments")
	}
	for _, prefix := range append(rfc4271ProspectivePrefixes(), rfc4271NextHopConnected) {
		if err := importKernel(ctx, prefix, 0); err != nil {
			return err
		}
	}
	return importSay("every prospective fixture prefix is absent from every kernel table")
}

func rfc4271ProspectiveLinkCleanup(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("prospective post-stop link cleanup takes no arguments")
	}
	for _, prefix := range rfc4271ProspectivePrefixes() {
		if err := importKernel(ctx, prefix, 0); err != nil {
			return err
		}
	}
	if err := importKernel(ctx, rfc4271NextHopConnected, 1, "dev "+rfc4271NextHopLink+" ", "proto 2 "); err != nil {
		return err
	}
	if err := importCommandRun(ctx, "link", "show", "dev", rfc4271NextHopLink); err != nil {
		return err
	}
	if err := importDeleteLink(ctx, rfc4271NextHopLink); err != nil {
		return err
	}
	return importSay("joined daemon removed every prospective Ze route before owned link cleanup")
}
