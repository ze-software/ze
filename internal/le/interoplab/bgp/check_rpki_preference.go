// Design: docs/architecture/testing/interop.md -- RTR cache preference, failover and fail-back observed at BIRD.
// Related: prepare.go -- the standby StayRTR peer, served from vrps-standby.json at host 14.
// Related: check_rpki_reload.go -- the single-cache StayRTR checker this one follows.
package bgp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

const (
	rpkiPreferenceScenario = "rtr-stayrtr-preference"
	// rpkiPreferencePrefix is Valid under the preferred cache's VRPs and
	// Invalid under the standby's, so with `action invalid reject` it reaches
	// BIRD only while Ze validates against the preferred cache.
	rpkiPreferencePrefix = "9.59.0.0/24"
	// rpkiPreferenceControl is Valid under both caches and stays at BIRD.
	rpkiPreferenceControl = "10.59.0.0/16"
	// The lab hosts of the two caches: preference 10 and preference 20 in ze.conf.
	rpkiPreferredHost uint8 = 12
	rpkiStandbyHost   uint8 = 14
	// rpkiPreferencePath is the AS_PATH BIRD holds for a route FRR originated
	// and Ze relayed.
	rpkiPreferencePath = "65000 65001"
)

type rpkiPreferenceVRP struct {
	Prefix    string      `json:"prefix"`
	MaxLength int         `json:"max-length"`
	ASN       json.Number `json:"asn"`
}

// The VRP sets of vrps.json and vrps-standby.json. Ze's store holds exactly one
// of them at a time: the set of the cache whose data is in use, never a union.
var (
	rpkiPreferredVRPs = []rpkiPreferenceVRP{
		{"9.59.0.0/24", 24, "65001"},
		{"10.59.0.0/16", 16, "65001"},
		{"192.0.2.0/24", 24, "65010"},
	}
	rpkiStandbyVRPs = []rpkiPreferenceVRP{
		{"9.59.0.0/16", 24, "4200000002"},
		{"10.59.0.0/16", 16, "65001"},
	}
)

type rpkiPreferenceCache struct {
	Address string `json:"address"`
	Synced  bool   `json:"synced"`
}

// rpkiPreferenceObservation is one reading of the lab: which caches Ze says
// hold its data, the VRPs Ze validates against, and what BIRD received.
type rpkiPreferenceObservation struct {
	Caches []rpkiPreferenceCache
	VRPs   []rpkiPreferenceVRP
	Export string
}

// rpkiPreferencePhase is what one step of the scenario requires of an observation.
type rpkiPreferencePhase struct {
	name     string
	live     string
	idle     string
	vrps     []rpkiPreferenceVRP
	exported bool
}

// checkRPKICachePreference proves RFC 8210 Section 10's preference order end to
// end. Ze starts on the preferred cache and exports the prefix only its VRPs
// make Valid. The preferred StayRTR is then stopped: Ze must load the standby's
// set, whose VRPs make that prefix Invalid, and withdraw it from BIRD. The
// preferred StayRTR is started again: Ze must load it back within its Retry
// Interval and BIRD must receive the prefix again. At each step exactly one
// cache reports synced data, and it is the one whose set Ze holds.
func checkRPKICachePreference(ctx context.Context, check *interoplab.CheckContext) error {
	initial := []operation{
		{kind: opFRRSession, argument: zeLabAddress},
		{kind: opBIRDSession, argument: birdZeProtocol},
		{kind: opFRRRoute, argument: rpkiPreferencePrefix},
		{kind: opFRRRoute, argument: rpkiPreferenceControl},
	}
	for index := range initial {
		if err := runOperation(ctx, check.Network, check.Lab, &initial[index]); err != nil {
			return checkerFailure(ctx, check.Lab, rpkiPreferenceScenario, index+1, err)
		}
	}
	preferred := networkHostAddress(check.Network, rpkiPreferredHost)
	standby := networkHostAddress(check.Network, rpkiStandbyHost)
	steps := []struct {
		change *operation
		phase  rpkiPreferencePhase
	}{
		{nil, rpkiPreferencePhase{"preferred cache in use", preferred, standby, rpkiPreferredVRPs, true}},
		{&operation{kind: opStop, peer: peerStayRTR, timeout: 5 * time.Second}, rpkiPreferencePhase{"failover to the standby cache", standby, preferred, rpkiStandbyVRPs, false}},
		{&operation{kind: opStart, peer: peerStayRTR}, rpkiPreferencePhase{"fail-back to the preferred cache", preferred, standby, rpkiPreferredVRPs, true}},
	}
	for index, step := range steps {
		assertion := len(initial) + index + 1
		if step.change != nil {
			if err := runOperation(ctx, check.Network, check.Lab, step.change); err != nil {
				return checkerFailure(ctx, check.Lab, rpkiPreferenceScenario, assertion, fmt.Errorf("%s: %w", step.phase.name, err))
			}
		}
		if err := waitRPKIPreferencePhase(ctx, check.Lab, step.phase); err != nil {
			return checkerFailure(ctx, check.Lab, rpkiPreferenceScenario, assertion, err)
		}
	}
	return nil
}

// waitRPKIPreferencePhase polls the lab until an observation meets phase. The
// bound covers one Refresh Interval to notice a dead cache, a failed connect,
// and one Retry Interval before the preferred cache is asked again (10 seconds
// each, test/interop/Dockerfile.stayrtr), with room for a loaded host.
func waitRPKIPreferencePhase(ctx context.Context, lab interoplab.CheckerLab, phase rpkiPreferencePhase) error {
	var lastErr error
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: 90 * time.Second, Interval: time.Second, Description: "RTR cache preference: " + phase.name,
	}, func(probeCtx context.Context) (bool, error) {
		observed, err := readRPKIPreferenceObservation(probeCtx, lab)
		if err == nil {
			err = rpkiPreferencePhaseError(observed, phase)
		}
		lastErr = err
		return err == nil, err
	}, func(met bool) bool { return met })
	if err != nil {
		return fmt.Errorf("%s: %w", phase.name, errors.Join(err, lastErr))
	}
	return nil
}

func readRPKIPreferenceObservation(ctx context.Context, lab interoplab.CheckerLab) (rpkiPreferenceObservation, error) {
	var observed rpkiPreferenceObservation
	var status struct {
		Caches []rpkiPreferenceCache `json:"cache-servers"`
	}
	if err := queryRPKIReloadJSON(ctx, lab, "show bgp rpki status", &status); err != nil {
		return observed, err
	}
	var roa struct {
		Entries []rpkiPreferenceVRP `json:"entries"`
	}
	if err := queryRPKIReloadJSON(ctx, lab, "show bgp rpki roa", &roa); err != nil {
		return observed, err
	}
	export, err := lab.Query(ctx, peerBIRD, []string{cmdBirdc, "show route protocol " + birdZeProtocol + " all"}, nil)
	if err != nil {
		return observed, err
	}
	observed.Caches, observed.VRPs, observed.Export = status.Caches, roa.Entries, export
	return observed, nil
}

// rpkiPreferencePhaseError answers why observed does not meet phase, or nil.
func rpkiPreferencePhaseError(observed rpkiPreferenceObservation, phase rpkiPreferencePhase) error {
	if err := requireRPKIPreferenceLiveCache(observed.Caches, phase); err != nil {
		return err
	}
	if err := requireRPKIPreferenceVRPs(observed.VRPs, phase.vrps); err != nil {
		return err
	}
	blocks := birdPrefixBlocks(observed.Export)
	if err := requireBIRDASPath(blocks[rpkiPreferenceControl], rpkiPreferenceControl, rpkiPreferencePath); err != nil {
		return err
	}
	if phase.exported {
		return requireBIRDASPath(blocks[rpkiPreferencePrefix], rpkiPreferencePrefix, rpkiPreferencePath)
	}
	if block, present := blocks[rpkiPreferencePrefix]; present {
		return fmt.Errorf("BIRD still holds %s, which the standby cache's VRPs make Invalid: %s", rpkiPreferencePrefix, block)
	}
	return nil
}

// requireRPKIPreferenceLiveCache requires both caches listed and exactly one,
// phase.live, reporting synced data.
func requireRPKIPreferenceLiveCache(caches []rpkiPreferenceCache, phase rpkiPreferencePhase) error {
	listed := make(map[string]bool, len(caches))
	var synced []string
	for _, cache := range caches {
		listed[cache.Address] = true
		if cache.Synced {
			synced = append(synced, cache.Address)
		}
	}
	for _, address := range []string{phase.live, phase.idle} {
		if !listed[address] {
			return fmt.Errorf("show bgp rpki status lists no cache at %s: %+v", address, caches)
		}
	}
	if len(synced) != 1 {
		return fmt.Errorf("show bgp rpki status names %d caches with data in use %v, want only %s", len(synced), synced, phase.live)
	}
	if synced[0] != phase.live {
		return fmt.Errorf("show bgp rpki status names %s as the cache in use, want %s", synced[0], phase.live)
	}
	return nil
}

// requireRPKIPreferenceVRPs requires the store to hold exactly want: a union of
// both caches' sets, or a leftover entry, fails.
func requireRPKIPreferenceVRPs(held, want []rpkiPreferenceVRP) error {
	if len(held) != len(want) {
		return fmt.Errorf("ze holds %d VRPs %+v, want the %d of the cache in use %+v", len(held), held, len(want), want)
	}
	missing := make(map[rpkiPreferenceVRP]bool, len(want))
	for _, entry := range want {
		missing[entry] = true
	}
	for _, entry := range held {
		if !missing[entry] {
			return fmt.Errorf("ze holds unexpected or duplicate VRP %+v, want the set %+v", entry, want)
		}
		delete(missing, entry)
	}
	return nil
}

// birdPrefixBlocks splits `birdc show route ... all` output into one block per
// prefix, so an attribute check reads only the lines of the route it names.
func birdPrefixBlocks(output string) map[string]string {
	blocks := make(map[string]string)
	current := ""
	for line := range strings.SplitSeq(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if _, err := netip.ParsePrefix(fields[0]); err == nil {
			current = fields[0]
		}
		if current != "" {
			blocks[current] += line + "\n"
		}
	}
	return blocks
}
