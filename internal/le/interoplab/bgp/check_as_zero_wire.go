// Design: docs/architecture/testing/interop.md -- AS-zero raw commands against FRR.
// Related: check_as_zero_wire_oracle.go -- independent full-frame judgement.
// FRR: https://github.com/FRRouting/frr/blob/frr-10.7.1/bgpd/bgp_attr.c
package bgp

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// checkASZeroRawFRR exercises the actual raw command after both sides establish.
// FRR may itself withdraw AS_PATH zero or discard an aggregator, and an ASN4
// receiver would ignore compatibility attributes from a NEW speaker. This
// fixture configures Ze's existing two-octet mode, so FRR instead reconstructs
// AS4_PATH 70000 from an OLD wire AS_PATH carrying AS_TRANS. These are scoped
// raw-diagnostic twins, not a claim about NEW-speaker capability advertisement.
// Consequently native absence alone is insufficient: exact captured withdrawals
// and a later valid route delimit the output of all eight commands.
func checkASZeroRawFRR(ctx context.Context, check *interoplab.CheckContext) (resultErr error) {
	directory, err := retainedProofDirectory(check)
	if err != nil {
		return err
	}
	evidence := make(map[string]string)
	defer func() {
		diagnostic, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancel()
		version, versionErr := check.Lab.Query(diagnostic, peerFRR, []string{cmdVtysh, "-c", frrShowVersion}, nil)
		evidence["frr-version"] = version
		image, imageErr := check.Lab.PeerImage(diagnostic, peerFRR)
		evidence["frr-image"] = image
		resultErr = errors.Join(resultErr, versionErr, imageErr)
		verdict := verdictPassed
		if resultErr != nil {
			verdict = resultErr.Error()
		}
		data, marshalErr := json.MarshalIndent(map[string]any{"scenario": check.Source.Name, "verdict": verdict, "evidence": evidence}, "", "  ")
		resultErr = errors.Join(resultErr, marshalErr)
		if marshalErr == nil {
			resultErr = errors.Join(resultErr, os.WriteFile(filepath.Join(directory, "proof.json"), data, 0o600))
		}
		if resultErr != nil {
			resultErr = fmt.Errorf("%w (retained proof: %s)", resultErr, directory)
		}
	}()
	if !check.Network.IPv4.IsValid() {
		return errors.New("AS-zero scenario has no selected IPv4 network")
	}
	ze, err := netip.ParseAddr(networkHostAddress(check.Network, 2))
	if err != nil {
		return err
	}
	native, err := netip.ParseAddr(networkHostAddress(check.Network, 3))
	if err != nil {
		return err
	}
	neighbor := networkHostAddress(check.Network, 10)
	command := []string{cmdVtysh, "-c", "show bgp neighbor " + neighbor + " json"}
	if err := waitJSONFields(ctx, check.Lab, peerFRR, command, 60*time.Second, map[string]string{frrSessionStateKey: stateEstablished}, nil); err != nil {
		return err
	}
	if err := waitZePeerState(ctx, check.Lab, neighbor, 60*time.Second); err != nil {
		return err
	}
	before, err := check.Lab.Query(ctx, peerFRR, command, nil)
	evidence["frr-neighbor-before"] = before
	if err != nil {
		return err
	}
	generation, err := parseFRRSessionGeneration(before, neighbor)
	if err != nil {
		return err
	}
	if generation != 1 {
		return errors.New("AS-zero fixture did not retain its first native session")
	}
	inputs := asZeroWireInputs(ze)
	for _, input := range inputs {
		if err := asZeroRawCommand(ctx, check.Lab, evidence, fmt.Sprintf("clean-%d", input.code), neighbor, input.clean); err != nil {
			return err
		}
		if err := waitASZeroNativePath(ctx, check.Lab, evidence, input.prefix, neighbor, input.path, input.aggregator); err != nil {
			return err
		}
	}
	for _, input := range inputs {
		if err := asZeroRawCommand(ctx, check.Lab, evidence, fmt.Sprintf("zero-%d", input.code), neighbor, input.zero); err != nil {
			return err
		}
	}
	if err := asZeroRawCommand(ctx, check.Lab, evidence, "fence", neighbor, asZeroWireBody(0, 255, false, ze)); err != nil {
		return err
	}
	if err := waitASZeroWire(ctx, check.Lab, evidence, ze, native, false); err != nil {
		return err
	}
	if err := waitASZeroWire(ctx, check.Lab, evidence, ze, native, true); err != nil {
		return err
	}
	if err := waitASZeroNativePath(ctx, check.Lab, evidence, asZeroFencePrefix, neighbor, "65001 65020", 0); err != nil {
		return err
	}
	_, _, err = interoplab.Wait(ctx, interoplab.WaitOptions{Timeout: 30 * time.Second, Interval: time.Second, Description: "FRR retains the final AS-zero control and withdraws all four subjects"}, func(probe context.Context) (string, error) {
		text, err := check.Lab.Query(probe, peerFRR, []string{cmdVtysh, "-c", "show bgp ipv4 unicast json"}, nil)
		if err == nil {
			evidence["frr-final-table"] = text
		}
		return text, err
	}, func(text string) bool {
		present, measured := jsonHasRoute(text, asZeroFencePrefix)
		if !measured || !present {
			return false
		}
		for _, input := range inputs {
			present, measured := jsonHasRoute(text, input.prefix)
			if !measured || present {
				return false
			}
		}
		return true
	})
	if err != nil {
		return err
	}
	after, err := check.Lab.Query(ctx, peerFRR, command, nil)
	evidence["frr-neighbor-after"] = after
	if err != nil {
		return err
	}
	afterGeneration, err := parseFRRSessionGeneration(after, neighbor)
	if err != nil {
		return err
	}
	if afterGeneration != generation {
		return errors.New("AS-zero refusal reset the native session")
	}
	return waitZePeerState(ctx, check.Lab, neighbor, 30*time.Second)
}

func asZeroRawCommand(ctx context.Context, lab interoplab.CheckerLab, evidence map[string]string, label, neighbor string, body []byte) error {
	command := zeCommand("send bgp " + neighbor + " raw hex " + hex.EncodeToString(body) + " type update")
	text, err := lab.Query(ctx, "ze", command, queryEnvironment("ze", command))
	evidence["command-"+label] = text
	if err != nil {
		return fmt.Errorf("AS-zero %s raw command: %w", label, err)
	}
	return nil
}

func waitASZeroNativePath(ctx context.Context, lab interoplab.CheckerLab, evidence map[string]string, prefix, neighbor, asPath string, aggregator uint32) error {
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{Timeout: 30 * time.Second, Interval: time.Second, Description: "FRR decodes the valid AS-zero twin " + prefix}, func(probe context.Context) (string, error) {
		text, err := queryFRRIPv4Route(probe, lab, prefix)
		if err == nil {
			evidence["frr-route-"+prefix] = text
		}
		return text, err
	}, func(text string) bool { return requireASZeroFRRPath(text, prefix, neighbor, asPath, aggregator) == nil })
	return err
}

func waitASZeroWire(ctx context.Context, lab interoplab.CheckerLab, evidence map[string]string, ze, native netip.Addr, fromNative bool) error {
	direction := "ze"
	if fromNative {
		direction = "frr"
	}
	var judgement error
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{Timeout: 30 * time.Second, Interval: time.Second, Description: "complete AS-zero " + direction + " wire interval"}, func(probe context.Context) (string, error) {
		text, err := lab.Query(probe, peerSpeaker, []string{cmdCat, asZeroWireCapture + "-" + direction + ".jsonl"}, nil)
		if err == nil {
			evidence["wire-"+direction] = text
		}
		return text, err
	}, func(text string) bool {
		// cat may observe the append before its last JSON row is complete.
		if !strings.HasSuffix(text, "\n") {
			return false
		}
		complete, err := judgeASZeroWire(text, ze, native, fromNative)
		judgement = err
		return complete || err != nil
	})
	return errors.Join(err, judgement)
}
