// Design: docs/architecture/testing/interop.md -- paired IGP import-policy placement.
// Related: check_import_policy.go -- original checker assertions stay unchanged.

package bgp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/le/interoplab"
)

const (
	isisImportPolicyForkedScenario = "isis-import-policy-frr-forked"
	ospfImportPolicyForkedScenario = "ospf-import-policy-bird-forked"
	importPolicySysribOwner        = "sysrib-policy-owner"
	ospfImportCentralPrefix        = "10.88.3.0/24"
)

func checkISISImportPolicyInternal(ctx context.Context, check *interoplab.CheckContext) error {
	if err := checkImportPolicyPlacement(ctx, check, "internal"); err != nil {
		return err
	}
	if err := checkISISImportPeerIdentity(ctx, check); err != nil {
		return err
	}
	return checkISISImportPolicy(ctx, check)
}

func checkISISImportPolicyForked(ctx context.Context, check *interoplab.CheckContext) error {
	if err := checkImportPolicyPlacement(ctx, check, "forked"); err != nil {
		return err
	}
	if err := checkISISImportPeerIdentity(ctx, check); err != nil {
		return err
	}
	return checkISISImportPolicy(ctx, check)
}

func checkOSPFImportPolicyInternal(ctx context.Context, check *interoplab.CheckContext) error {
	if err := checkImportPolicyPlacement(ctx, check, "internal"); err != nil {
		return err
	}
	if err := checkOSPFImportPolicy(ctx, check); err != nil {
		return err
	}
	return checkOSPFImportCentral(ctx, check)
}

func checkOSPFImportPolicyForked(ctx context.Context, check *interoplab.CheckContext) error {
	if err := checkImportPolicyPlacement(ctx, check, "forked"); err != nil {
		return err
	}
	if err := checkOSPFImportPolicy(ctx, check); err != nil {
		return err
	}
	return checkOSPFImportCentral(ctx, check)
}

func checkImportPolicyPlacement(ctx context.Context, check *interoplab.CheckContext, placement string) error {
	query := func(command string) (string, error) {
		args := zeCommand(command)
		return check.Lab.Query(ctx, "ze", args, queryEnvironment("ze", args))
	}
	answer, err := query("plugin session ping")
	if err != nil {
		return err
	}
	var session struct {
		PID int `json:"pid"`
	}
	if err := json.Unmarshal([]byte(answer), &session); err != nil {
		return fmt.Errorf("decode daemon identity: %w", err)
	}
	if session.PID < 1 {
		return fmt.Errorf("daemon returned no process identity: %s", answer)
	}
	answer, err = query("system subsystem list")
	if err != nil {
		return err
	}
	var owners struct {
		Subsystems []struct {
			Name     string `json:"name"`
			Stage    string `json:"stage"`
			Running  bool   `json:"running"`
			Commands int    `json:"command-count"`
		} `json:"subsystems"`
	}
	if err := json.Unmarshal([]byte(answer), &owners); err != nil {
		return fmt.Errorf("decode configured system RIB owner: %w", err)
	}
	found := false
	for _, owner := range owners.Subsystems {
		if owner.Name != importPolicySysribOwner {
			continue
		}
		if !owner.Running {
			return errors.New("configured system RIB owner is not running")
		}
		if owner.Stage != "Running" {
			return errors.New("configured system RIB owner did not finish startup")
		}
		if owner.Commands < 1 {
			return errors.New("configured system RIB owner registered no commands")
		}
		found = true
	}
	if !found {
		return fmt.Errorf("configured system RIB owner is absent: %s", answer)
	}
	args := []string{"le", "test", "fixture", "plugin/sysrib-process-placement", strconv.Itoa(session.PID), "rib", placement}
	if _, err := check.Lab.Query(ctx, "ze", args, nil); err != nil {
		return fmt.Errorf("actual system RIB placement: %w", err)
	}
	return nil
}

func checkISISImportPeerIdentity(ctx context.Context, check *interoplab.CheckContext) error {
	for _, peer := range []string{peerFRR, peerFRRSink} {
		if _, err := check.Lab.PeerImage(ctx, peer); err != nil {
			return err
		}
		output, err := check.Lab.Query(ctx, peer, []string{cmdVtysh, "-c", frrShowVersion}, nil)
		if err != nil {
			return err
		}
		if !strings.Contains(output, "FRRouting 10.7.1") {
			return fmt.Errorf("IS-IS peer %s differs from inspected FRR 10.7.1: %s", peer, output)
		}
	}
	return nil
}

func checkOSPFImportCentral(ctx context.Context, check *interoplab.CheckContext) error {
	if err := waitRIBCandidates(ctx, check.Lab, netip.MustParsePrefix(ospfImportCentralPrefix), []ribCandidateWant{
		{source: sourceOSPF, hidden: true, hiddenBy: "central", chain: "rib import", distanceJudged: true, distance: 110},
	}); err != nil {
		return fmt.Errorf("central OSPF hidden candidate: %w", err)
	}
	operations := []operation{
		{kind: opRequireContains, peer: "ze", command: zeCommand("show ospf route"), contains: []string{ospfImportCentralPrefix, ospfImportUntaggedPrefix}},
		{kind: opRequireAbsent, peer: "ze", command: zeCommand("show rib"), absent: []string{ospfImportCentralPrefix}, proof: []string{ospfImportUntaggedPrefix}},
		{kind: opWaitContains, peer: "ze", command: []string{"ip", "-4", ipObjectRoute, ipActionShow}, contains: []string{ospfImportUntaggedPrefix}, timeout: importPolicyWait},
		{kind: opRequireAbsent, peer: "ze", command: []string{"ip", "-4", ipObjectRoute, ipActionShow}, absent: []string{ospfImportCentralPrefix}, proof: []string{ospfImportUntaggedPrefix}},
	}
	for index := range operations {
		if err := runOperation(ctx, check.Network, check.Lab, &operations[index]); err != nil {
			return fmt.Errorf("central OSPF assertion %d: %w", index+1, err)
		}
	}
	if _, err := check.Lab.PeerImage(ctx, peerBIRD); err != nil {
		return err
	}
	output, err := check.Lab.Query(ctx, peerBIRD, []string{cmdBirdc, "show status"}, nil)
	if err != nil {
		return err
	}
	if !strings.Contains(output, "BIRD 3.4.0") {
		return fmt.Errorf("OSPF source differs from inspected BIRD 3.4.0: %s", output)
	}
	return nil
}
