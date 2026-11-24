// Design: docs/architecture/testing/interop.md -- raw VPNv6 family admission against FRR.
// Related: check_raw_vpnv6_admission_oracle.go -- independent full-frame and FRR JSON oracle.
// FRR: https://github.com/FRRouting/frr/blob/frr-10.7.1/bgpd/bgp_packet.c (bgp_update_receive, bgp_nlri_parse)
// FRR: https://github.com/FRRouting/frr/blob/frr-10.7.1/bgpd/bgp_attr.c (bgp_mp_reach_parse)
// FRR: https://github.com/FRRouting/frr/blob/frr-10.7.1/bgpd/bgp_mplsvpn.c (bgp_nlri_parse_vpn)
// FRR: https://github.com/FRRouting/frr/blob/frr-10.7.1/bgpd/bgp_route.c (bgp_show_table_rd, route_vty_out_detail)
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

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/interoplab"
)

// checkRawVPNv6FamilyAdmissionFRR proves AC-27 (RFC 4659 Section 3.4 / RFC 4760
// Section 8) against two independent FRR 10.7.1 peers behind transparent wire
// relays:
//   - peerFRR (via speaker at .10) negotiated ipv4/unicast and ipv6/mpls-vpn:
//     valid raw VPNv6 UPDATEs in both body and full-packet CLI forms reach the
//     wire verbatim and are decoded by FRR with exact RD, label, AS_PATH and
//     next hop, followed by an IPv4 fence route.
//   - peerFRRSink (via speaker2 at .11) negotiated ipv4/unicast only: the same
//     valid raw VPNv6 body and full-packet commands emit zero wire bytes, a
//     mixed raw UPDATE has its unnegotiated VPNv6 MP_REACH_NLRI stripped while
//     its IPv4 NLRI survives to FRR, and a standalone IPv4 control route reaches
//     FRR cleanly with the original session uninterrupted.
func checkRawVPNv6FamilyAdmissionFRR(ctx context.Context, check *interoplab.CheckContext) (resultErr error) {
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
		if resultErr != nil {
			if frrLog, err := check.Lab.Query(diagnostic, peerFRR, []string{cmdCat, "/tmp/frr.log"}, nil); err == nil {
				evidence["frr-log"] = frrLog
			}
			if zeCap, err := check.Lab.Query(diagnostic, peerSpeaker, []string{cmdCat, rawVPNv6NegotiatedCapture + "-ze.jsonl"}, nil); err == nil {
				evidence["speaker-ze-jsonl"] = zeCap
			}
			if frrCap, err := check.Lab.Query(diagnostic, peerSpeaker, []string{cmdCat, rawVPNv6NegotiatedCapture + "-frr.jsonl"}, nil); err == nil {
				evidence["speaker-frr-jsonl"] = frrCap
			}
		}
		resultErr = errors.Join(resultErr, versionErr, imageErr)
		verdict := verdictPassed
		if resultErr != nil {
			verdict = resultErr.Error()
		}
		payload := map[string]any{
			evidenceKeyScenario: check.Source.Name,
			evidenceKeyVerdict:  verdict,
			evidenceKeyEvidence: evidence,
		}
		data, marshalErr := json.MarshalIndent(payload, "", "  ")
		resultErr = errors.Join(resultErr, marshalErr)
		if marshalErr == nil {
			resultErr = errors.Join(resultErr, os.WriteFile(filepath.Join(directory, "proof.json"), data, 0o600))
		}
		if resultErr != nil {
			resultErr = fmt.Errorf("%w (retained proof: %s)", resultErr, directory)
		}
	}()
	if !check.Network.IPv4.IsValid() {
		return errors.New("raw VPNv6 admission scenario has no selected IPv4 network")
	}
	ze, err := netip.ParseAddr(networkHostAddress(check.Network, zeHostNumber))
	if err != nil {
		return err
	}
	frrAddr, err := netip.ParseAddr(networkHostAddress(check.Network, frrHostNumber))
	if err != nil {
		return err
	}
	sinkAddr, err := netip.ParseAddr(networkHostAddress(check.Network, extendedSinkHost))
	if err != nil {
		return err
	}
	relayNegotiated := networkHostAddress(check.Network, 10)
	relayV4Only := networkHostAddress(check.Network, 11)
	vpnNextHop := netip.MustParseAddr(rawVPNv6NextHop)

	if err := waitRawVPNv6SessionReady(ctx, check.Lab, evidence, "negotiated", peerFRR, peerSpeaker, rawVPNv6NegotiatedCapture, relayNegotiated, ze, frrAddr, true); err != nil {
		return err
	}
	if err := waitRawVPNv6SessionReady(ctx, check.Lab, evidence, "v4only", peerFRRSink, peerSpeaker2, rawVPNv6V4OnlyCapture, relayV4Only, ze, sinkAddr, false); err != nil {
		return err
	}

	vpnBodyOne := rawVPNv6UpdateBody(rawVPNv6BodyPrefixBytes, rawVPNv6BodyLabel)
	vpnBodyTwo := rawVPNv6UpdateBody(rawVPNv6FullPrefixBytes, rawVPNv6FullLabel)
	mixedBody := rawVPNv6MixedBody(rawVPNv6MixedPrefixBytes, rawVPNv6MixedLabel, 2, ze)
	negotiatedFence := rawVPNv6IPv4Body(1, ze)
	v4OnlyFence := rawVPNv6IPv4Body(255, ze)

	// Session 1 (negotiated ipv4/unicast + ipv6/mpls-vpn): body form, full-packet form, IPv4 fence.
	if err := sendRawVPNv6Command(ctx, check.Lab, evidence, "negotiated-body", relayNegotiated, false, vpnBodyOne); err != nil {
		return err
	}
	if err := sendRawVPNv6Command(ctx, check.Lab, evidence, "negotiated-full", relayNegotiated, true, vpnBodyTwo); err != nil {
		return err
	}
	if err := sendRawVPNv6Command(ctx, check.Lab, evidence, "negotiated-fence", relayNegotiated, false, negotiatedFence); err != nil {
		return err
	}

	// Session 2 (ipv4/unicast only): pure VPNv6 body + full-packet withheld, mixed stripped to IPv4, IPv4 fence.
	if err := sendRawVPNv6Command(ctx, check.Lab, evidence, "v4only-body", relayV4Only, false, vpnBodyOne); err != nil {
		return err
	}
	if err := sendRawVPNv6Command(ctx, check.Lab, evidence, "v4only-full", relayV4Only, true, vpnBodyTwo); err != nil {
		return err
	}
	if err := sendRawVPNv6Command(ctx, check.Lab, evidence, "v4only-mixed", relayV4Only, false, mixedBody); err != nil {
		return err
	}
	if err := sendRawVPNv6Command(ctx, check.Lab, evidence, "v4only-fence", relayV4Only, false, v4OnlyFence); err != nil {
		return err
	}

	// Wire proofs on both transparent relays (Ze->FRR exact sequence + FRR->Ze health).
	if err := waitRawVPNv6Wire(ctx, check.Lab, evidence, "negotiated", peerSpeaker, rawVPNv6NegotiatedCapture, ze, frrAddr, true, false); err != nil {
		return err
	}
	if err := waitRawVPNv6Wire(ctx, check.Lab, evidence, "negotiated", peerSpeaker, rawVPNv6NegotiatedCapture, ze, frrAddr, true, true); err != nil {
		return err
	}
	if err := waitRawVPNv6Wire(ctx, check.Lab, evidence, "v4only", peerSpeaker2, rawVPNv6V4OnlyCapture, ze, sinkAddr, false, false); err != nil {
		return err
	}
	if err := waitRawVPNv6Wire(ctx, check.Lab, evidence, "v4only", peerSpeaker2, rawVPNv6V4OnlyCapture, ze, sinkAddr, false, true); err != nil {
		return err
	}

	// Native FRR verification on Session 1 (negotiated): both VPNv6 routes + IPv4 fence.
	if err := waitRawVPNv6FRRRoute(ctx, check.Lab, evidence, peerFRR, rawVPNv6RD, rawVPNv6BodyPrefix, relayNegotiated, rawVPNv6BodyLabel, vpnNextHop); err != nil {
		return err
	}
	if err := waitRawVPNv6FRRRoute(ctx, check.Lab, evidence, peerFRR, rawVPNv6RD, rawVPNv6FullPrefix, relayNegotiated, rawVPNv6FullLabel, vpnNextHop); err != nil {
		return err
	}
	if err := waitRawVPNv6FRRTable(ctx, check.Lab, evidence, "negotiated-vpnv6-table", peerFRR, rawVPNv6RD, relayNegotiated, []string{rawVPNv6BodyPrefix, rawVPNv6FullPrefix}); err != nil {
		return err
	}
	if err := waitRawVPNv6IPv4Route(ctx, check.Lab, evidence, "negotiated-fence-route", peerFRR, rawVPNv6NegotiatedFence, relayNegotiated, ze); err != nil {
		return err
	}

	// Native FRR verification on Session 2 (ipv4/unicast only): mixed IPv4 + standalone IPv4 control, empty VPNv6 table.
	if err := waitRawVPNv6IPv4Route(ctx, check.Lab, evidence, "v4only-mixed-ipv4-route", peerFRRSink, rawVPNv6MixedIPv4Prefix, relayV4Only, ze); err != nil {
		return err
	}
	if err := waitRawVPNv6IPv4Route(ctx, check.Lab, evidence, "v4only-fence-route", peerFRRSink, rawVPNv6V4OnlyFence, relayV4Only, ze); err != nil {
		return err
	}
	if err := waitRawVPNv6FRRTable(ctx, check.Lab, evidence, "v4only-vpnv6-table", peerFRRSink, rawVPNv6RD, relayV4Only, nil); err != nil {
		return err
	}

	// Verify both original sessions remained Established with generation == 1 on both sides.
	if err := requireRawVPNv6SessionIntact(ctx, check.Lab, evidence, "negotiated", peerFRR, relayNegotiated); err != nil {
		return err
	}
	return requireRawVPNv6SessionIntact(ctx, check.Lab, evidence, "v4only", peerFRRSink, relayV4Only)
}

func waitRawVPNv6SessionReady(
	ctx context.Context,
	lab interoplab.CheckerLab,
	evidence map[string]string,
	label, frrPeer, relayPeer, captureBase, neighbor string,
	ze, native netip.Addr,
	negotiatedVPNv6 bool,
) error {
	var cmdBuf textbuf.Buffer
	neighborCmd := []string{cmdVtysh, "-c", cmdBuf.Str("show bgp neighbor ").Str(neighbor).Str(" json").String()}
	if err := waitJSONFields(ctx, lab, frrPeer, neighborCmd, 60*time.Second, map[string]string{frrSessionStateKey: stateEstablished}, nil); err != nil {
		return err
	}
	if err := waitZePeerState(ctx, lab, neighbor, 60*time.Second); err != nil {
		return err
	}
	before, err := lab.Query(ctx, frrPeer, neighborCmd, nil)
	evidence[label+"-neighbor-before"] = before
	if err != nil {
		return err
	}
	generation, err := parseFRRSessionGeneration(before, neighbor)
	if err != nil {
		return err
	}
	if generation != 1 {
		return fmt.Errorf("raw VPNv6 %s session did not retain its first generation (got %d)", label, generation)
	}
	if err := requireRawVPNv6NeighborCapabilities(before, neighbor, negotiatedVPNv6); err != nil {
		return err
	}
	var judgement error
	_, _, err = interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     30 * time.Second,
		Interval:    time.Second,
		Description: "raw VPNv6 " + label + " initial sync EORs",
	}, func(probe context.Context) (string, error) {
		return lab.Query(probe, relayPeer, []string{cmdCat, captureBase + "-ze.jsonl"}, nil)
	}, func(text string) bool {
		if !strings.HasSuffix(text, "\n") {
			return false
		}
		ready, err := judgeRawVPNv6InitialSync(text, ze, native, negotiatedVPNv6)
		judgement = err
		return ready || err != nil
	})
	return errors.Join(err, judgement)
}

func sendRawVPNv6Command(
	ctx context.Context,
	lab interoplab.CheckerLab,
	evidence map[string]string,
	label, neighbor string,
	fullPacket bool,
	body []byte,
) error {
	payload := body
	if fullPacket {
		payload = rawVPNv6FullPacket(body)
	}
	var line textbuf.Buffer
	line.Str("send bgp ").Str(neighbor).Str(" raw hex ").Str(hex.EncodeToString(payload))
	if !fullPacket {
		line.Str(" type update")
	}
	command := zeCommand(line.String())
	text, err := lab.Query(ctx, "ze", command, queryEnvironment("ze", command))
	evidence["command-"+label] = text
	if err != nil {
		return fmt.Errorf("raw VPNv6 %s command: %w", label, err)
	}
	return nil
}

func waitRawVPNv6Wire(
	ctx context.Context,
	lab interoplab.CheckerLab,
	evidence map[string]string,
	label, relayPeer, captureBase string,
	ze, native netip.Addr,
	negotiatedVPNv6, fromNative bool,
) error {
	direction := "ze"
	if fromNative {
		direction = peerFRR
	}
	var judgement error
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     30 * time.Second,
		Interval:    time.Second,
		Description: "complete raw VPNv6 " + label + " " + direction + " wire interval",
	}, func(probe context.Context) (string, error) {
		text, err := lab.Query(probe, relayPeer, []string{cmdCat, captureBase + "-" + direction + ".jsonl"}, nil)
		if err == nil {
			evidence["wire-"+label+"-"+direction] = text
		}
		return text, err
	}, func(text string) bool {
		if !strings.HasSuffix(text, "\n") {
			return false
		}
		complete, err := judgeRawVPNv6Wire(text, ze, native, negotiatedVPNv6, fromNative)
		judgement = err
		return complete || err != nil
	})
	return errors.Join(err, judgement)
}

func waitRawVPNv6FRRRoute(
	ctx context.Context,
	lab interoplab.CheckerLab,
	evidence map[string]string,
	frrPeer, rd, prefix, neighbor string,
	wantLabel uint32,
	wantNextHop netip.Addr,
) error {
	var cmdBuf textbuf.Buffer
	show := []string{cmdVtysh, "-c", cmdBuf.Str("show bgp ipv6 vpn rd ").Str(rd).Byte(' ').Str(prefix).Str(" json").String()}
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     30 * time.Second,
		Interval:    time.Second,
		Description: "FRR decodes negotiated raw VPNv6 route " + prefix,
	}, func(probe context.Context) (string, error) {
		text, err := lab.Query(probe, frrPeer, show, nil)
		if err == nil {
			evidence["frr-vpnv6-route-"+prefix] = text
		}
		return text, err
	}, func(text string) bool {
		return requireRawVPNv6FRRRoute(text, rd, prefix, neighbor, wantLabel, wantNextHop) == nil
	})
	return err
}

func waitRawVPNv6FRRTable(
	ctx context.Context,
	lab interoplab.CheckerLab,
	evidence map[string]string,
	evidenceKey, frrPeer, rd, neighbor string,
	wantPrefixes []string,
) error {
	show := []string{cmdVtysh, "-c", "show bgp ipv6 vpn rd all json"}
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     30 * time.Second,
		Interval:    time.Second,
		Description: "FRR VPNv6 RD table " + evidenceKey,
	}, func(probe context.Context) (string, error) {
		text, err := lab.Query(probe, frrPeer, show, nil)
		if err == nil {
			evidence[evidenceKey] = text
		}
		return text, err
	}, func(text string) bool {
		return requireRawVPNv6FRRTable(text, rd, neighbor, wantPrefixes) == nil
	})
	return err
}

func waitRawVPNv6IPv4Route(
	ctx context.Context,
	lab interoplab.CheckerLab,
	evidence map[string]string,
	evidenceKey, frrPeer, prefix, neighbor string,
	wantNextHop netip.Addr,
) error {
	var cmdBuf textbuf.Buffer
	show := []string{cmdVtysh, "-c", cmdBuf.Str("show bgp ipv4 unicast ").Str(prefix).Str(" json").String()}
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     30 * time.Second,
		Interval:    time.Second,
		Description: "FRR decodes raw IPv4 route " + prefix,
	}, func(probe context.Context) (string, error) {
		text, err := lab.Query(probe, frrPeer, show, nil)
		if err == nil {
			evidence[evidenceKey] = text
		}
		return text, err
	}, func(text string) bool {
		return requireRawVPNv6IPv4Route(text, prefix, neighbor, wantNextHop) == nil
	})
	return err
}

func requireRawVPNv6SessionIntact(
	ctx context.Context,
	lab interoplab.CheckerLab,
	evidence map[string]string,
	label, frrPeer, neighbor string,
) error {
	var cmdBuf textbuf.Buffer
	command := []string{cmdVtysh, "-c", cmdBuf.Str("show bgp neighbor ").Str(neighbor).Str(" json").String()}
	after, err := lab.Query(ctx, frrPeer, command, nil)
	evidence[label+"-neighbor-after"] = after
	if err != nil {
		return err
	}
	generation, err := parseFRRSessionGeneration(after, neighbor)
	if err != nil {
		return err
	}
	if generation != 1 {
		return fmt.Errorf("raw VPNv6 %s admission reset the native FRR session (generation %d)", label, generation)
	}
	return waitZePeerState(ctx, lab, neighbor, 30*time.Second)
}
