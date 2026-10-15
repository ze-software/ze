// Design: docs/architecture/testing/interop.md -- BFD over link-local against FRR.
// Related: check_bfd_profile_edit.go -- frrBFDPeer, decodeFRRBFDPeer, the FRR BFD JSON this reads.
// Related: prepare.go -- linkLocalAt, the address each container owns.
// RFC: rfc/short/rfc5881.md Section 3
package bgp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/interoplab"
)

const (
	bfdLinkLocalScenario         = "bfd-link-local-frr"
	bfdLinkLocalTwoLinksScenario = "bfd-link-local-two-links-frr"
)

// zeBFDSession is the part of one `show bfd sessions` entry this checker
// reads (api.SessionState). The discriminators are pointers so a field ze did
// not print is told apart from a zero it did.
type zeBFDSession struct {
	Peer        string  `json:"peer"`
	Interface   string  `json:"interface"`
	State       string  `json:"state"`
	LocalDiscr  *uint64 `json:"local-discriminator"`
	RemoteDiscr *uint64 `json:"remote-discriminator"`
}

// RFC 5881 Section 3
//
// checkBFDLinkLocal proves a BFD-plugin-pinned single-hop session to an IPv6
// link-local peer reaches Up against FRR bfdd on eth0, and that each side
// learned the other's discriminator from the other's packets: FRR's remote-id
// is ze's local discriminator, and ze's remote discriminator is FRR's id. One
// direction alone satisfies neither clause, because each value exists only on
// the side that chose it until a packet carries it across.
//
// This is acceptance evidence. Both sides are Active, so the session also
// converges through Your Discriminator once FRR answers, and the scenario
// does not discriminate the zone reconciliation on receive; with one link the
// socket is bound to eth0, so it does not discriminate the send pinning either.
// bfd-link-local-two-links-frr (checkBFDLinkLocalTwoLinks) discriminates both.
func checkBFDLinkLocal(ctx context.Context, check *interoplab.CheckContext) error {
	if !check.Network.IPv6.IsValid() {
		return checkerFailure(ctx, check.Lab, bfdLinkLocalScenario, 1, errors.New("no IPv6 lab network was selected, so no link-local exists"))
	}
	return checkBFDLinkLocalOn(ctx, check, bfdLinkLocalScenario, containerInterface)
}

// RFC 5881 Section 3
//
// checkBFDLinkLocalTwoLinks proves that two sessions to ONE link-local
// address, one on each of two links, each reach Up on their own link with
// their own discriminators crossed. The links are the interfaces of ze's own
// sessions to FRR's link-local, so ze.conf declares them once and this reads
// them back; at least two are required.
//
// The scenario's files make it discriminating where bfd-link-local-frr is not.
// ze-prestart.sh drops every BFD Control packet ze sends with Your
// Discriminator zero, so FRR learns ze's discriminator only from a packet ze
// sends after it selected FRR's first packet by address and link: with the
// zone reconciliation broken no first packet is selected and neither side
// leaves Down. Two links put the sessions on an unbound socket, so each
// packet reaches its link only through the zone or the ifindex ze puts on the
// send; without either the kernel refuses the write and FRR hears nothing.
func checkBFDLinkLocalTwoLinks(ctx context.Context, check *interoplab.CheckContext) error {
	if !check.Network.IPv6.IsValid() {
		return checkerFailure(ctx, check.Lab, bfdLinkLocalTwoLinksScenario, 1, errors.New("no IPv6 lab network was selected, so no link-local exists"))
	}
	frrAddress := linkLocalAt(check.Network, frrHostNumber)
	sessions := zeCommand(zeShowBFDSessions)
	var links []string
	last, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: 30 * time.Second, Interval: time.Second, Description: "ze BFD sessions to FRR's link-local on two links",
	}, func(probeCtx context.Context) (string, error) {
		return check.Lab.Query(probeCtx, "ze", sessions, queryEnvironment("ze", sessions))
	}, func(output string) bool {
		found, findErr := zeLinkLocalLinks(output, frrAddress)
		if findErr != nil {
			return false
		}
		links = found
		return len(links) >= 2
	})
	if err != nil {
		return checkerFailure(ctx, check.Lab, bfdLinkLocalTwoLinksScenario, 2, withLastOutput(err, last))
	}
	for _, link := range links {
		if err := checkBFDLinkLocalOn(ctx, check, bfdLinkLocalTwoLinksScenario, link); err != nil {
			return err
		}
	}
	return nil
}

// checkBFDLinkLocalOn requires FRR's BFD peer for ze's link-local on link to
// be Up, ze's session to FRR's link-local on link to be Up, and the two to
// hold each other's discriminator. Its failures are numbered 2 to 5 under
// scenario.
func checkBFDLinkLocalOn(ctx context.Context, check *interoplab.CheckContext, scenario, link string) error {
	zeAddress := linkLocalAt(check.Network, zeHostNumber)
	frrAddress := linkLocalAt(check.Network, frrHostNumber)
	var text textbuf.Buffer
	frrPeer := []string{cmdVtysh, "-c", text.Str("show bfd peer ").Addr(zeAddress).Str(" interface ").Str(link).Str(" json").String()}

	last, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: 90 * time.Second, Interval: time.Second, Description: text.Reset().Str("FRR BFD peer up on ").Str(link).String(),
	}, func(probeCtx context.Context) (string, error) {
		return check.Lab.Query(probeCtx, peerFRR, frrPeer, nil)
	}, func(output string) bool {
		_, decodeErr := decodeFRRBFDPeer(output)
		return decodeErr == nil
	})
	if err != nil {
		return checkerFailure(ctx, check.Lab, scenario, 2, withLastOutput(err, last))
	}
	frr, err := decodeFRRBFDPeer(last)
	if err != nil {
		return checkerFailure(ctx, check.Lab, scenario, 3, err)
	}
	if frr.ID == nil {
		return checkerFailure(ctx, check.Lab, scenario, 3, errors.New("FRR BFD peer JSON has no id"))
	}
	if frr.RemoteID == nil {
		return checkerFailure(ctx, check.Lab, scenario, 3, errors.New("FRR BFD peer JSON has no remote-id"))
	}

	sessions := zeCommand(zeShowBFDSessions)
	var ze zeBFDSession
	last, _, err = interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: 30 * time.Second, Interval: time.Second, Description: text.Reset().Str("ze BFD session up on ").Str(link).String(),
	}, func(probeCtx context.Context) (string, error) {
		return check.Lab.Query(probeCtx, "ze", sessions, queryEnvironment("ze", sessions))
	}, func(output string) bool {
		session, findErr := zeLinkLocalSession(output, frrAddress, link)
		if findErr != nil {
			return false
		}
		ze = session
		return true
	})
	if err != nil {
		return checkerFailure(ctx, check.Lab, scenario, 4, withLastOutput(err, last))
	}
	if err := requireCrossedDiscriminators(*frr.ID, *frr.RemoteID, ze); err != nil {
		return checkerFailure(ctx, check.Lab, scenario, 5, err)
	}
	return nil
}

// zeLinkLocalLinks answers the sorted, distinct interfaces of ze's BFD
// sessions to peer in a `show bfd sessions` answer, whatever their state.
func zeLinkLocalLinks(output string, peer netip.Addr) ([]string, error) {
	var sessions []zeBFDSession
	if err := json.Unmarshal([]byte(output), &sessions); err != nil {
		return nil, fmt.Errorf("decode ze BFD sessions JSON: %w; output: %s", err, strings.TrimSpace(output))
	}
	var links []string
	for _, session := range sessions {
		address, err := netip.ParseAddr(session.Peer)
		if err != nil {
			return nil, fmt.Errorf("ze BFD session peer %q is not an address: %w", session.Peer, err)
		}
		if address.WithZone("") != peer {
			continue
		}
		if slices.Contains(links, session.Interface) {
			continue
		}
		links = append(links, session.Interface)
	}
	slices.Sort(links)
	return links, nil
}

// zeLinkLocalSession finds ze's Up session to peer on iface in a
// `show bfd sessions` answer, and names what it found instead when there is
// none.
func zeLinkLocalSession(output string, peer netip.Addr, iface string) (zeBFDSession, error) {
	var sessions []zeBFDSession
	if err := json.Unmarshal([]byte(output), &sessions); err != nil {
		return zeBFDSession{}, fmt.Errorf("decode ze BFD sessions JSON: %w; output: %s", err, strings.TrimSpace(output))
	}
	for _, session := range sessions {
		address, err := netip.ParseAddr(session.Peer)
		if err != nil {
			return zeBFDSession{}, fmt.Errorf("ze BFD session peer %q is not an address: %w", session.Peer, err)
		}
		if address.WithZone("") != peer {
			continue
		}
		if session.Interface != iface {
			continue
		}
		if session.State != "up" {
			return zeBFDSession{}, fmt.Errorf("ze BFD session to %s on %s is %q, want up", peer, iface, session.State)
		}
		return session, nil
	}
	return zeBFDSession{}, fmt.Errorf("ze holds no BFD session to %s on %s: %s", peer, iface, strings.TrimSpace(output))
}

// requireCrossedDiscriminators refuses unless each side holds the other's
// discriminator: FRR's remote-id is ze's local discriminator and ze's remote
// discriminator is FRR's id, neither of them zero.
func requireCrossedDiscriminators(frrID, frrRemoteID uint64, ze zeBFDSession) error {
	if ze.LocalDiscr == nil {
		return errors.New("ze BFD session JSON has no local-discriminator")
	}
	if ze.RemoteDiscr == nil {
		return errors.New("ze BFD session JSON has no remote-discriminator")
	}
	if frrID == 0 {
		return errors.New("FRR's BFD id is 0, which RFC 5880 Section 6.8.1 forbids for a session")
	}
	if *ze.LocalDiscr == 0 {
		return errors.New("ze's local discriminator is 0, which RFC 5880 Section 6.8.1 forbids for a session")
	}
	var problem textbuf.Buffer
	if frrRemoteID != *ze.LocalDiscr {
		return errors.New(problem.Str("FRR's remote-id is ").Uint(frrRemoteID).Str(" and ze's local discriminator is ").
			Uint(*ze.LocalDiscr).Str(", so FRR did not learn ze's discriminator from ze's packets").String())
	}
	if *ze.RemoteDiscr != frrID {
		return errors.New(problem.Str("ze's remote discriminator is ").Uint(*ze.RemoteDiscr).Str(" and FRR's id is ").
			Uint(frrID).Str(", so ze did not learn FRR's discriminator from FRR's packets").String())
	}
	return nil
}
