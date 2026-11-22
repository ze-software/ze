// Design: docs/architecture/testing/interop.md -- per-family LLGR capability interop proof.
// RFC: rfc/short/rfc9494.md -- Section 5 per-AFI/SAFI affirmative LLGR configuration.
// Related: check_llgr.go -- shared FRR session identity and LLGR text parser helpers.
package bgp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

const (
	llgrPerFamilyIPv4Prefix = "10.94.0.0/24"
	llgrPerFamilyIPv6Prefix = "2001:db8:94::/48"
)

// checkLLGRPerFamilyFRR proves RFC 9494 Section 5 against FRR 10.7.1: a dual-stack
// peer carrying both ipv4/unicast and ipv6/unicast with long-lived configured only
// for ipv4/unicast advertises code 64 for both families and code 71 for ipv4/unicast
// alone.
//
// FRR 10.7.1 parses code 71 per-AFI/SAFI in bgp_capability_llgr (bgpd/bgp_open.c),
// reports code 64 received families in `show bgp neighbor <addr> json` under
// `neighborCapabilities.addressFamiliesByPeer`, and prints code 71 received families
// in `show bgp neighbor <addr>` text under `Long-lived Graceful Restart:` ->
// `Address families by peer:` (bgp_show_peer, bgpd/bgp_vty.c). Both IPv4 and IPv6
// routes are verified at FRR first, so IPv6's absence from code 71 cannot come from
// an unnegotiated IPv6 family.
func checkLLGRPerFamilyFRR(ctx context.Context, check *interoplab.CheckContext) error {
	if !check.Network.IPv4.IsValid() {
		return errors.New("per-family LLGR scenario has no selected IPv4 network")
	}
	if !check.Network.IPv6.IsValid() {
		return errors.New("per-family LLGR scenario has no selected IPv6 network")
	}
	if err := waitFRRRoute(ctx, check.Lab, llgrPerFamilyIPv4Prefix, frrIPv4Unicast, 60*time.Second, true); err != nil {
		return fmt.Errorf("IPv4 unicast route control: %w", err)
	}
	if err := waitFRRRoute(ctx, check.Lab, llgrPerFamilyIPv6Prefix, frrFamilyIPv6Unicast, 60*time.Second, true); err != nil {
		return fmt.Errorf("IPv6 unicast route control: %w", err)
	}
	address := networkHostAddress(check.Network, 2)
	neighborJSON, err := check.Lab.Query(ctx, peerFRR, []string{cmdVtysh, "-c", "show bgp neighbor " + address + " json"}, nil)
	if err != nil {
		return err
	}
	identity, err := llgrParseFRRSession(neighborJSON, address)
	if err != nil {
		return err
	}
	if err := llgrPerFamilyFRRGRJSON(neighborJSON, address); err != nil {
		return interoplab.WithSearchedOutput(err, neighborJSON)
	}
	text, err := check.Lab.Query(ctx, peerFRR, []string{cmdVtysh, "-c", "show bgp neighbor " + address}, nil)
	if err != nil {
		return err
	}
	if err := llgrPerFamilyFRRText(text); err != nil {
		return interoplab.WithSearchedOutput(err, text)
	}
	return llgrRequireSession(ctx, check, identity)
}

// llgrPerFamilyFRRGRJSON checks FRR 10.7.1's `show bgp neighbor <addr> json`
// output to confirm code 64 ("gracefulRestart") was bilaterally negotiated and
// carried both ipv4Unicast and ipv6Unicast from Ze.
func llgrPerFamilyFRRGRJSON(output, address string) error {
	var document map[string]struct {
		Capabilities struct {
			GR       string                     `json:"gracefulRestart"`
			Families map[string]json.RawMessage `json:"addressFamiliesByPeer"`
		} `json:"neighborCapabilities"`
	}
	if err := json.Unmarshal([]byte(output), &document); err != nil {
		return err
	}
	peer, ok := document[address]
	if !ok {
		return errors.New("FRR neighbor JSON omits the selected Ze address")
	}
	if peer.Capabilities.GR != "advertisedAndReceived" {
		return fmt.Errorf("FRR gracefulRestart capability is %q, want advertisedAndReceived", peer.Capabilities.GR)
	}
	if _, hasV4 := peer.Capabilities.Families["ipv4Unicast"]; !hasV4 {
		return errors.New("FRR GR addressFamiliesByPeer has no received ipv4Unicast tuple")
	}
	if _, hasV6 := peer.Capabilities.Families["ipv6Unicast"]; !hasV6 {
		return errors.New("FRR GR addressFamiliesByPeer has no received ipv6Unicast tuple")
	}
	return nil
}

// llgrPerFamilyFRRText reads FRR 10.7.1's `show bgp neighbor` LLGR text section
// (the same section llgrFRRFamilies reads) and requires IPv4 Unicast present
// and IPv6 Unicast absent.
func llgrPerFamilyFRRText(output string) error {
	inLLGR, inFamilies := false, false
	ipv4, ipv6 := false, false
	for line := range strings.SplitSeq(output, "\n") {
		if strings.HasPrefix(line, "    Long-lived Graceful Restart:") {
			if inLLGR {
				return errors.New("FRR neighbor output repeats its LLGR section")
			}
			if strings.TrimSpace(line) != "Long-lived Graceful Restart: advertised and received" {
				return errors.New("FRR text output does not confirm bilateral LLGR")
			}
			inLLGR = true
			continue
		}
		if !inLLGR {
			continue
		}
		if strings.TrimSpace(line) != "" && !strings.HasPrefix(line, "     ") {
			break
		}
		if line == "      Address families by peer:" {
			inFamilies = true
			continue
		}
		if !inFamilies {
			continue
		}
		if line == "           IPv4 Unicast" {
			ipv4 = true
		}
		if line == "           IPv6 Unicast" {
			ipv6 = true
		}
	}
	if !inLLGR {
		return errors.New("FRR neighbor output lacks Long-lived Graceful Restart section")
	}
	if !ipv4 {
		return errors.New("FRR LLGR text section has no received IPv4 Unicast tuple")
	}
	if ipv6 {
		return errors.New("FRR LLGR text section unexpectedly received IPv6 Unicast (code 71 must carry configured families alone)")
	}
	return nil
}
