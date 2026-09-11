// Design: docs/architecture/testing/interop.md -- foreign receiver evidence.
package bgp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"strconv"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const (
	pathsLimitScenario = "bgp-paths-limit-frr"
	pathsLimitCommand  = "request interop paths-limit"
)

// The process is checker-driven, not a timed burst that can finish before the
// session exists. Each phase makes separate API writes, then sends a fresh
// sentinel prefix over the same session. Seeing that sentinel at FRR proves the
// preceding offered update has passed the sender's queue, even when it is denied.
func runPathsLimitProcess(name string) error {
	registration := sdk.Registration{Commands: []rpc.CommandDecl{{
		Name: pathsLimitCommand, Description: "Offer a PATHS-LIMIT interop phase and its ordering sentinel",
	}}}
	var runErr error
	code := sdk.RunOrDeclare(registration, func() int {
		plugin, err := sdk.NewFromEnv(name)
		if err != nil {
			runErr = err
			return 1
		}
		plugin.OnExecuteCommand(func(_ string, command string, args []string, _ string) (string, any, error) {
			if command != pathsLimitCommand || len(args) != 2 {
				return rpc.StatusError, nil, errors.New("paths-limit wants PHASE and NEXT-HOP")
			}
			phase, err := strconv.Atoi(args[0])
			if err != nil {
				return rpc.StatusError, nil, err
			}
			updates, err := pathsLimitPhaseUpdates(phase, args[1])
			if err != nil {
				return rpc.StatusError, nil, err
			}
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			for _, update := range updates {
				if _, _, err := plugin.UpdateRoute(ctx, "*", update); err != nil {
					return rpc.StatusError, nil, err
				}
			}
			return rpc.StatusDone, map[string]int{"phase": phase}, nil
		})
		runErr = plugin.Run(context.Background(), registration)
		if runErr != nil {
			return 1
		}
		return 0
	})
	if code != 0 && runErr == nil {
		return errors.New("paths-limit process declaration failed")
	}
	return runErr
}

func pathsLimitPhaseUpdates(phase int, nextHop string) ([]string, error) {
	address, err := netip.ParseAddr(nextHop)
	if err != nil || !address.Is4() {
		return nil, fmt.Errorf("invalid IPv4 next hop %q", nextHop)
	}
	id, metric, withdraw := 0, 0, false
	switch phase {
	case 1:
		id, metric = 1, 100
	case 2:
		id, metric = 2, 200
	case 3:
		id, metric = 3, 300 // denied: third distinct path while at two
	case 4:
		id, metric = 1, 101 // accepted: replacement at the cap
	case 5:
		id, withdraw = 2, true
	case 6:
		id, metric = 3, 300 // retry the denied identifier after withdrawal
	case 7:
		id, metric = 4, 400 // denied again: state must survive separate writes
	default:
		return nil, fmt.Errorf("unknown PATHS-LIMIT phase %d", phase)
	}
	announce := func(pathID, med int, prefix string) string {
		return fmt.Sprintf("update text path-information %d origin igp path 65001 med %d nhop %s nlri ipv4/unicast add %s", pathID, med, nextHop, prefix)
	}
	update := announce(id, metric, injectPrefixFirst)
	if withdraw {
		update = fmt.Sprintf("update text path-information %d nlri ipv4/unicast del %s", id, injectPrefixFirst)
	}
	return []string{update, announce(100+phase, phase, pathsLimitSentinel(phase))}, nil
}

func pathsLimitSentinel(phase int) string {
	return fmt.Sprintf("198.18.%d.0/24", phase)
}

// FRR 10.3.1 implements code 76 in bgpd/bgp_open.{h,c}; its documented
// `neighbor ... addpath-rx-paths-limit 2` sends 2, not a local inbound filter.
// bgpd/bgp_vty.c exports the negotiated OPEN evidence under pathsLimit. Reading
// both directions there avoids mistaking ADD-PATH alone for code 76 support.
// https://github.com/FRRouting/frr/blob/frr-10.3.1/bgpd/bgp_open.h
// https://github.com/FRRouting/frr/blob/frr-10.3.1/doc/user/bgp.rst
func checkPathsLimitFRR(ctx context.Context, check *interoplab.CheckContext) error {
	if !check.Network.IPv4.IsValid() {
		return errors.New("PATHS-LIMIT scenario has no selected IPv4 network")
	}
	zeAddress := networkHostAddress(check.Network, 2)
	frrAddress := networkHostAddress(check.Network, 3)
	session := operation{kind: opFRRSession, argument: zeAddress}
	if err := runOperation(ctx, check.Network, check.Lab, &session); err != nil {
		return err
	}
	neighbor, err := check.Lab.Query(ctx, peerFRR, []string{cmdVtysh, "-c", "show bgp neighbor " + zeAddress + " json"}, nil)
	if err != nil {
		return err
	}
	if err := requireFRRPathsLimit(neighbor, zeAddress); err != nil {
		return err
	}
	command := zeCommand("show bgp peer " + frrAddress + " capabilities")
	negotiated, err := check.Lab.Query(ctx, "ze", command, queryEnvironment("ze", command))
	if err != nil {
		return err
	}
	if err := requireZePathsLimit(negotiated, frrAddress); err != nil {
		return err
	}

	// Exact (Path Identifier, MED) sets, not a prefix-presence check or a best
	// path count. Replacement changes MED while preserving the identifier;
	// withdrawal removes only its identifier; retry then occupies the freed slot.
	states := []map[uint32]uint32{
		{1: 100},
		{1: 100, 2: 200},
		{1: 100, 2: 200},
		{1: 101, 2: 200},
		{1: 101},
		{1: 101, 3: 300},
		{1: 101, 3: 300},
	}
	for index, want := range states {
		phase := index + 1
		command = zeCommand(fmt.Sprintf("%s %d %s", pathsLimitCommand, phase, zeAddress))
		answer, err := check.Lab.Query(ctx, "ze", command, queryEnvironment("ze", command))
		if err != nil {
			return fmt.Errorf("offer phase %d: %w", phase, err)
		}
		var receipt struct {
			Phase int `json:"phase"`
		}
		if err := json.Unmarshal([]byte(answer), &receipt); err != nil || receipt.Phase != phase {
			return fmt.Errorf("phase %d has no successful process receipt: %s", phase, answer)
		}
		sentinel := operation{kind: opFRRRoute, argument: pathsLimitSentinel(phase), timeout: 30 * time.Second}
		if err := runOperation(ctx, check.Network, check.Lab, &sentinel); err != nil {
			return fmt.Errorf("phase %d ordering sentinel: %w", phase, err)
		}
		// Once the later prefix is present, do not poll away an excess path or
		// accept a transient earlier snapshot before all writes were processed.
		output, err := check.Lab.Query(ctx, peerFRR, []string{cmdVtysh, "-c", "show bgp ipv4 unicast " + injectPrefixFirst + " json"}, nil)
		if err != nil {
			return err
		}
		if err := requireFRRPathsLimitState(output, zeAddress, want); err != nil {
			return fmt.Errorf("phase %d: %w", phase, err)
		}
	}
	return runOperation(ctx, check.Network, check.Lab, &session)
}

func requireFRRPathsLimit(output, address string) error {
	var peers map[string]struct {
		State        string `json:"bgpState"`
		Capabilities struct {
			AddPath map[string]struct {
				Receive bool `json:"rxAdvertisedAndReceived"`
			} `json:"addPath"`
			PathsLimit map[string]struct {
				Negotiated bool   `json:"advertisedAndReceived"`
				Advertised uint16 `json:"advertisedPathsLimit"`
				Received   uint16 `json:"receivedPathsLimit"`
			} `json:"pathsLimit"`
		} `json:"neighborCapabilities"`
	}
	if err := json.Unmarshal([]byte(output), &peers); err != nil {
		return fmt.Errorf("decode FRR neighbor: %w", err)
	}
	peer := peers[address]
	limit := peer.Capabilities.PathsLimit["ipv4Unicast"]
	if peer.State != "Established" || !peer.Capabilities.AddPath["ipv4Unicast"].Receive ||
		!limit.Negotiated || limit.Advertised != 2 || limit.Received != 10 {
		return fmt.Errorf("FRR has no established ADD-PATH receive and bidirectional code76 evidence (advertised 2, received 10): %s", output)
	}
	return nil
}

func requireZePathsLimit(output, address string) error {
	var peers []struct {
		Peer       string `json:"peer"`
		Negotiated struct {
			PathsLimit struct {
				Send    map[string]uint16 `json:"send"`
				Receive map[string]uint16 `json:"receive"`
			} `json:"paths-limit"`
		} `json:"negotiated"`
	}
	if err := json.Unmarshal([]byte(output), &peers); err != nil {
		return fmt.Errorf("decode Ze negotiated capabilities: %w", err)
	}
	for _, peer := range peers {
		if peer.Peer == address && peer.Negotiated.PathsLimit.Send["ipv4/unicast"] == 2 && peer.Negotiated.PathsLimit.Receive["ipv4/unicast"] == 10 {
			return nil
		}
	}
	return fmt.Errorf("Ze did not receive FRR limit 2 separately from its advertised receive request 10: %s", output)
}

func requireFRRPathsLimitState(output, address string, want map[uint32]uint32) error {
	var document struct {
		Prefix string `json:"prefix"`
		Paths  []struct {
			ID     *uint32 `json:"addpathRxId"`
			Metric *uint32 `json:"metric"`
			Valid  bool    `json:"valid"`
			Peer   struct {
				ID string `json:"peerId"`
			} `json:"peer"`
		} `json:"paths"`
	}
	if err := json.Unmarshal([]byte(output), &document); err != nil {
		return fmt.Errorf("decode FRR paths: %w", err)
	}
	if document.Prefix != injectPrefixFirst || len(document.Paths) != len(want) {
		return fmt.Errorf("FRR holds %d paths for %q, want %v for %s", len(document.Paths), document.Prefix, want, injectPrefixFirst)
	}
	seen := make(map[uint32]bool, len(document.Paths))
	for _, path := range document.Paths {
		if path.ID == nil || path.Metric == nil || !path.Valid || path.Peer.ID != address {
			return fmt.Errorf("FRR path lacks valid received identifier, metric or source: %s", output)
		}
		metric, ok := want[*path.ID]
		if !ok || metric != *path.Metric || seen[*path.ID] {
			return fmt.Errorf("FRR path id=%d med=%d is extra, duplicated or stale; want %v", *path.ID, *path.Metric, want)
		}
		seen[*path.ID] = true
	}
	return nil
}
