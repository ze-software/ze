// Design: docs/architecture/testing/interop.md -- best paths under a peer's PATHS-LIMIT, read at the peer.
// Related: check_paths_limit.go -- the capability readings this checker reuses.
// Related: register_paths_limit.go -- registry entry that dispatches this checker.
package bgp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

const (
	pathsLimitBestPathScenario = "bgp-paths-limit-best-path-frr"
	// pathsLimitBestPathPrefix is the prefix every static protocol of bird.conf
	// holds, one MED each.
	pathsLimitBestPathPrefix = "10.72.0.0/24"
	// pathsLimitBestPathBound is the PATHS-LIMIT frr.conf advertises.
	pathsLimitBestPathBound = 2
	// pathsLimitBestPathReturning is the MED of the path that is demoted by MED
	// 50 and comes back when MED 50 goes away.
	pathsLimitBestPathReturning = 200
)

// pathsLimitBestPathStep is one change the checker makes at BIRD, and the MEDs
// FRR must hold once Ze has relayed it.
type pathsLimitBestPathStep struct {
	protocol string
	enable   bool
	want     []uint32
}

// pathsLimitBestPathSteps offers the paths worst first, so arrival order and
// election order disagree from the third step on. Step 3 is AC-17-peer: the
// best path arrives last and displaces the worst. Step 4 is AC-18-peer: a better
// path demotes a held one. Step 5 brings the demoted path back.
var pathsLimitBestPathSteps = []pathsLimitBestPathStep{
	{"med_300", true, []uint32{300}},
	{"med_200", true, []uint32{200, 300}},
	{"med_100", true, []uint32{100, 200}},
	{"med_50", true, []uint32{50, 100}},
	{"med_50", false, []uint32{100, 200}},
}

// checkPathsLimitBestPath proves, at FRR, that Ze sends a peer that advertised
// PATHS-LIMIT 2 the two paths bgp-rib ranks best, withdraws a demoted path, and
// re-announces it under the identifier FRR held it with before. Every reading
// is FRR's own table, and a reading over the limit fails at once.
func checkPathsLimitBestPath(ctx context.Context, check *interoplab.CheckContext) error {
	fail := func(assertion int, cause error) error {
		return checkerFailure(ctx, check.Lab, pathsLimitBestPathScenario, assertion, cause)
	}
	if !check.Network.IPv4.IsValid() {
		return fail(1, errors.New("best-path scenario has no selected IPv4 network"))
	}
	zeAddress := networkHostAddress(check.Network, 2)
	frrAddress := networkHostAddress(check.Network, 3)

	// Assertions 1 and 2. Both sessions.
	sessions := []operation{
		{kind: opFRRSession, argument: zeAddress},
		{kind: opBIRDSession, argument: birdZeProtocol},
	}
	for index := range sessions {
		if err := runOperation(ctx, check.Network, check.Lab, &sessions[index]); err != nil {
			return fail(index+1, err)
		}
	}

	// Assertion 3. FRR advertised 2, and Ze holds it apart from its own 10.
	neighbor, err := check.Lab.Query(ctx, peerFRR, []string{cmdVtysh, "-c", "show bgp neighbor " + zeAddress + " json"}, nil)
	if err != nil {
		return fail(3, err)
	}
	if err := requireFRRPathsLimit(neighbor, zeAddress); err != nil {
		return fail(3, err)
	}
	command := zeCommand("show bgp peer " + frrAddress + " capabilities")
	negotiated, err := check.Lab.Query(ctx, "ze", command, queryEnvironment("ze", command))
	if err != nil {
		return fail(3, err)
	}
	if err := requireZePathsLimit(negotiated, frrAddress); err != nil {
		return fail(3, err)
	}

	// Assertions 4 to 8, one per step: FRR holds exactly the best two.
	var demotedID uint32
	for index, step := range pathsLimitBestPathSteps {
		assertion := 4 + index
		if err := pathsLimitBestPathToggle(ctx, check.Lab, step); err != nil {
			return fail(assertion, err)
		}
		held, err := waitPathsLimitBestPath(ctx, check.Lab, zeAddress, step.want)
		if err != nil {
			return fail(assertion, fmt.Errorf("after %s: %w", pathsLimitBestPathAction(step), err))
		}
		if index == 2 {
			demotedID = held[pathsLimitBestPathReturning]
		}
		if index == len(pathsLimitBestPathSteps)-1 {
			if err := requirePathsLimitSameID(held, demotedID); err != nil {
				return fail(assertion, err)
			}
		}
	}

	// Assertions 9 and 10. Both sessions survived every election.
	for index := range sessions {
		if err := runOperation(ctx, check.Network, check.Lab, &sessions[index]); err != nil {
			return fail(index+9, err)
		}
	}
	return nil
}

// pathsLimitBestPathAction names a step for a failure message.
func pathsLimitBestPathAction(step pathsLimitBestPathStep) string {
	if step.enable {
		return "enable " + step.protocol
	}
	return "disable " + step.protocol
}

// pathsLimitBestPathToggle enables or disables one static protocol at BIRD and
// requires birdc to confirm it.
func pathsLimitBestPathToggle(ctx context.Context, lab interoplab.CheckerLab, step pathsLimitBestPathStep) error {
	action := pathsLimitBestPathAction(step)
	output, err := lab.Query(ctx, peerBIRD, []string{cmdBirdc, action}, nil)
	if err != nil {
		return fmt.Errorf("birdc %s: %w", action, err)
	}
	state := "disabled"
	if step.enable {
		state = "enabled"
	}
	if !strings.Contains(output, step.protocol+": "+state) {
		return fmt.Errorf("birdc %s did not confirm %s: %s", action, state, strings.TrimSpace(output))
	}
	return nil
}

// pathsLimitBestPathReading is one reading of FRR's table: the path identifier
// of each MED, or the error that made it unusable.
type pathsLimitBestPathReading struct {
	held map[uint32]uint32
}

// waitPathsLimitBestPath polls FRR until it holds exactly the wanted MEDs. A
// reading with more paths than the limit ends the wait at once and fails: the
// peer must never hold more than it advertised, transiently included.
func waitPathsLimitBestPath(ctx context.Context, lab interoplab.CheckerLab, zeAddress string, want []uint32) (map[uint32]uint32, error) {
	reading, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: 60 * time.Second, Interval: time.Second, Description: "FRR holding the best paths",
	}, func(probe context.Context) (pathsLimitBestPathReading, error) {
		output, err := lab.Query(probe, peerFRR, []string{cmdVtysh, "-c", "show bgp ipv4 unicast " + pathsLimitBestPathPrefix + " json"}, nil)
		if err != nil {
			return pathsLimitBestPathReading{}, err
		}
		held, err := decodeFRRPathsByMED(output, zeAddress)
		return pathsLimitBestPathReading{held: held}, err
	}, func(reading pathsLimitBestPathReading) bool {
		if len(reading.held) > pathsLimitBestPathBound {
			return true
		}
		return pathsLimitBestPathHolds(reading.held, want)
	})
	if err != nil {
		return nil, err
	}
	if len(reading.held) > pathsLimitBestPathBound {
		return nil, fmt.Errorf("FRR holds %d paths %v, over the PATHS-LIMIT %d it advertised", len(reading.held), slices.Sorted(maps.Keys(reading.held)), pathsLimitBestPathBound)
	}
	return reading.held, nil
}

// pathsLimitBestPathHolds reports whether held carries exactly the wanted MEDs.
func pathsLimitBestPathHolds(held map[uint32]uint32, want []uint32) bool {
	if len(held) != len(want) {
		return false
	}
	for _, med := range want {
		if _, present := held[med]; !present {
			return false
		}
	}
	return true
}

// requirePathsLimitSameID requires the returning path to carry the identifier
// FRR held it with before it was demoted (RFC 7911 Section 5 treats a repeated
// prefix and identifier as a replacement, so a stable identifier keeps the path
// one path at the peer).
func requirePathsLimitSameID(held map[uint32]uint32, demotedID uint32) error {
	returned, present := held[pathsLimitBestPathReturning]
	if !present {
		return fmt.Errorf("FRR does not hold the MED %d path that was due back", pathsLimitBestPathReturning)
	}
	if returned != demotedID {
		return fmt.Errorf("the MED %d path came back as path id %d, but FRR held it as path id %d before its demotion", pathsLimitBestPathReturning, returned, demotedID)
	}
	return nil
}

// decodeFRRPathsByMED reads FRR's paths for the scenario prefix, all received
// from peer, as MED to received path identifier. FRR answers an absent prefix
// with an object that names no prefix, which is a table holding nothing. A path from another
// source, without an identifier or MED, invalid, or repeating a MED is refused:
// every path Ze relays here carries a distinct MED.
func decodeFRRPathsByMED(output, peer string) (map[uint32]uint32, error) {
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
		return nil, fmt.Errorf("decode FRR paths: %w", err)
	}
	held := make(map[uint32]uint32, len(document.Paths))
	if document.Prefix == "" {
		if len(document.Paths) != 0 {
			return nil, fmt.Errorf("FRR answer lists paths under no prefix: %s", output)
		}
		return held, nil
	}
	if document.Prefix != pathsLimitBestPathPrefix {
		return nil, fmt.Errorf("FRR answered for %q, want %s", document.Prefix, pathsLimitBestPathPrefix)
	}
	for _, path := range document.Paths {
		if path.ID == nil {
			return nil, fmt.Errorf("FRR path has no received path identifier: %s", output)
		}
		if path.Metric == nil {
			return nil, fmt.Errorf("FRR path id %d has no MED: %s", *path.ID, output)
		}
		if !path.Valid {
			return nil, fmt.Errorf("FRR path id %d is not valid: %s", *path.ID, output)
		}
		if path.Peer.ID != peer {
			return nil, fmt.Errorf("FRR path id %d came from %s, want %s", *path.ID, path.Peer.ID, peer)
		}
		if _, repeated := held[*path.Metric]; repeated {
			return nil, fmt.Errorf("FRR holds two paths with MED %d: %s", *path.Metric, output)
		}
		held[*path.Metric] = *path.ID
	}
	return held, nil
}
