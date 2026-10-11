// Design: docs/architecture/testing/interop.md -- RSVP-TE interop against freeRouter.
// Related: rsvpte.go -- the topology these checks observe.
//
// Every assertion reads what a peer saw on its own wire: each container runs
// tcpdump on eth0 and the checker reads its decoded text. A message Ze logs
// as sent counts for nothing until a freeRouter peer has received it.
package rsvpte

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

const (
	scenarioLooseExpansion  = "transit-loose-ero-expansion"
	scenarioResvErrRelayed  = "ingress-resv-error-relayed"
	scenarioStrictForwarded = "transit-strict-hop-forwarded"
	scenarioStrictRefused   = "transit-strict-hop-outside-refused"
	scenarioResvTearRelayed = "transit-resv-tear-relayed"
	scenarioIncreaseInPlace = "transit-resv-increase-refused-in-place"
	scenarioFFUnknownSender = "transit-ff-resv-unknown-sender"
	scenarioBackupPathToMP  = "plr-backup-path-to-egress-merge-point"
	scenarioImplicitNullPop = "egress-implicit-null-penultimate-pop"
	scenarioResvExpires     = "transit-resv-state-expires"

	// labelImplicitNull is how tcpdump prints the LABEL object value 3, the
	// Implicit NULL label (RFC 3032 Section 2.1), which the patched egress of
	// egress-implicit-null-penultimate-pop distributes.
	labelImplicitNull = "3"

	// raisedRate is how tcpdump prints the FLOWSPEC rate the patched egress
	// asks once in transit-resv-increase-refused-in-place: 125000000 bytes/s
	// (egress-env.txt), 1 Gbit/s, ten times what the Ze transit's eth0 reserves.
	raisedRate = "1000 Mbps"

	// freeRtrRefresh is the pinned freeRouter's PATH refresh period
	// (ipFwd.untriggeredRecomputation, 120 s, not configurable). A freeRouter
	// egress originates a RESV only when a PATH arrives, so its second RESV
	// follows the ingress's first refresh by up to this long.
	freeRtrRefresh = 120 * time.Second

	// freeRtrResvLifetime is how long the Ze transit of
	// transit-resv-state-expires keeps a reservation nobody refreshes: the
	// freeRouter egress's RESV carries TIME_VALUES of freeRtrRefresh, and Ze
	// times the RSB out after that period times its refresh-multiplier, 3 in
	// that scenario's transit.conf (RFC 2205 Section 3.7).
	freeRtrResvLifetime = 3 * freeRtrRefresh

	captureFile = "/run/fr/rsvp.txt"
	// catCommand reads a peer's capture file.
	catCommand = "cat"

	// A Ze node answers on its container address, a freeRouter node on its
	// own address beside it (rsvpte.go, labRole).
	addressZeIngress      = "172.29.81.2"
	addressZeTransit      = "172.29.81.3"
	addressZeEgress       = "172.29.81.4"
	addressFreeRtrIngress = "172.29.81.12"
	addressFreeRtrEgress  = "172.29.81.14"
	addressFreeRtrRelay   = "172.29.81.15"
	loopbackEgress        = "198.51.100.4"

	// addressProtectedMP is the freeRouter egress on VLAN 100, the far end of
	// the PLR's protected link prot0 and the merge point of its bypass, in
	// plr-backup-path-to-egress-merge-point.
	addressProtectedMP = "10.0.14.14"
	// protectedTunnel is how tcpdump prints that scenario's protected session.
	protectedTunnel = "Tunnel ID: 0x0001"
	// ospfNeighborFull is the state `show ospf neighbor` prints for a Full
	// adjacency (internal/plugins/ospf/neighbor).
	ospfNeighborFull = "full"
)

type scenarioCheck func(context.Context, interoplab.CheckerLab, time.Duration) error

// checker adapts a scenario body to the protocol-neutral Checker signature and
// appends every peer's capture and log tail to a failure.
func checker(check scenarioCheck, timeout time.Duration) interoplab.Checker {
	return func(ctx context.Context, state *interoplab.CheckContext) error {
		if state == nil || state.Lab == nil {
			return errors.New("RSVP-TE checker has no lab")
		}
		err := check(ctx, state.Lab, timeout)
		if err == nil {
			return nil
		}
		return diagnosticError(ctx, state.Lab, err)
	}
}

// rsvpMessage is one decoded packet from a tcpdump -vvv capture: the line
// naming its addresses and every indented line under it.
type rsvpMessage struct {
	source string
	target string
	text   string
}

// parseCapture splits tcpdump text into packets. A packet starts at an
// unindented line. Its "source > target:" line is the second line of a plain
// IP packet, and the third of an MPLS-labeled one, whose label stack line and
// IP header line come first: the first indented line that opens with an address
// is the one.
func parseCapture(capture string) []rsvpMessage {
	var messages []rsvpMessage
	var current []string
	flush := func() {
		addresses := ""
		for _, line := range current[min(1, len(current)):] {
			trimmed := strings.TrimSpace(line)
			if trimmed != "" && trimmed[0] >= '0' && trimmed[0] <= '9' && strings.Contains(trimmed, " > ") {
				addresses = trimmed
				break
			}
		}
		source, rest, found := strings.Cut(addresses, " > ")
		if !found {
			current = nil
			return
		}
		target, _, _ := strings.Cut(rest, ":")
		messages = append(messages, rsvpMessage{source: source, target: target, text: strings.Join(current, "\n")})
		current = nil
	}
	for line := range strings.SplitSeq(capture, "\n") {
		if line != "" && line[0] != ' ' && line[0] != '\t' {
			flush()
		}
		if line != "" {
			current = append(current, line)
		}
	}
	flush()
	return messages
}

// waitMessage polls one peer's capture until a message from source to target
// satisfies match, and answers that message.
func waitMessage(ctx context.Context, lab interoplab.CheckerLab, peer, description string, timeout time.Duration, match func(rsvpMessage) bool) (rsvpMessage, error) {
	found, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{Timeout: timeout, Interval: time.Second, Description: description},
		func(ctx context.Context) (rsvpMessage, error) {
			capture, err := lab.Query(ctx, peer, []string{catCommand, captureFile}, nil)
			if err != nil {
				return rsvpMessage{}, err
			}
			for _, message := range parseCapture(capture) {
				if match(message) {
					return message, nil
				}
			}
			return rsvpMessage{}, nil
		},
		func(message rsvpMessage) bool { return message.text != "" })
	return found, err
}

// hopFrom is how tcpdump prints an RSVP_HOP object naming address.
func hopFrom(address string) string {
	return "Previous/Next Interface: " + address + ","
}

func isMessage(message rsvpMessage, source, target, kind string) bool {
	if !strings.HasPrefix(message.source, source) {
		return false
	}
	if !strings.HasPrefix(message.target, target) {
		return false
	}
	return strings.Contains(message.text, kind)
}

// checkLooseExpansion proves RFC 3209 Section 4.3.4.1 steps 5 and 6 at a Ze
// transit against freeRouter on both sides. The ingress names Ze and then the
// egress loopback as loose hops. Ze's native route to that loopback runs
// through the egress's segment address, which no subobject names, so the PATH
// the egress receives MUST carry that address as an inserted hop ahead of the
// loose loopback, and the LSP MUST come up: the ingress receives Ze's RESV with
// a label and Ze holds a swap toward the egress.
func checkLooseExpansion(ctx context.Context, lab interoplab.CheckerLab, timeout time.Duration) error {
	// A PATH carries the sender's address as its IP source (RFC 2205 Section
	// 3.1.3), so Ze's relay is the PATH whose RSVP_HOP names Ze.
	path, err := waitMessage(ctx, lab, peerEgress, "egress receives Ze's PATH", timeout, func(message rsvpMessage) bool {
		return strings.HasPrefix(message.target, loopbackEgress) && strings.Contains(message.text, "Path Message") &&
			strings.Contains(message.text, hopFrom(addressZeTransit))
	})
	if err != nil {
		return err
	}
	ero, err := explicitRoute(path.text)
	if err != nil {
		return err
	}
	if len(ero) != 2 {
		return fmt.Errorf("egress received ERO %q, want the inserted hop %s then loose %s", ero, addressFreeRtrEgress, loopbackEgress)
	}
	if !strings.Contains(ero[0], addressFreeRtrEgress) {
		return fmt.Errorf("egress received ERO %q: first hop is not the inserted %s", ero, addressFreeRtrEgress)
	}
	if !strings.Contains(ero[1], loopbackEgress) {
		return fmt.Errorf("egress received ERO %q: second hop is not %s", ero, loopbackEgress)
	}
	if !strings.Contains(strings.ToLower(ero[1]), "loose") {
		return fmt.Errorf("egress received ERO %q: %s lost its loose bit", ero, loopbackEgress)
	}
	if _, err := waitMessage(ctx, lab, peerIngress, "ingress receives Ze's RESV", timeout, func(message rsvpMessage) bool {
		return isMessage(message, addressZeTransit, addressFreeRtrIngress, "Resv Message") && strings.Contains(message.text, "Label Object")
	}); err != nil {
		return err
	}
	return waitSwap(ctx, lab, timeout)
}

// waitSwap polls the Ze transit's MPLS table until a label forwards toward the
// freeRouter egress, the only downstream every swap-checking scenario names.
func waitSwap(ctx context.Context, lab interoplab.CheckerLab, timeout time.Duration) error {
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{Timeout: timeout, Interval: time.Second, Description: "Ze installs the transit swap"},
		func(ctx context.Context) (string, error) {
			return lab.Query(ctx, peerTransit, []string{"ip", "-f", "mpls", "route", "show"}, nil)
		},
		func(table string) bool { return strings.Contains(table, "via inet "+addressFreeRtrEgress) })
	return err
}

// errorFrom is how tcpdump prints the ERROR_SPEC node address.
func errorFrom(address string) string {
	return "Error Node Address: " + address + ","
}

// checkResvErrRelayed proves RFC 2205 Sections 2.5 and 3.1.8 across an
// independent relay: Ze ingress, freeRouter transit, Ze egress. The ingress
// interface reserves less than the tunnel asks, so the ingress refuses the
// RESV freeRouter relays and originates a ResvErr naming itself, admission
// control failure. freeRouter relays it downstream, and the Ze egress MUST
// receive it from freeRouter with Ze's error node and code intact.
func checkResvErrRelayed(ctx context.Context, lab interoplab.CheckerLab, timeout time.Duration) error {
	if _, err := waitMessage(ctx, lab, peerRelay, "freeRouter receives the Ze ingress's ResvErr", timeout, func(message rsvpMessage) bool {
		return isMessage(message, addressZeIngress, addressFreeRtrRelay, "ResvErr Message") && strings.Contains(message.text, errorFrom(addressZeIngress))
	}); err != nil {
		return err
	}
	relayed, err := waitMessage(ctx, lab, peerEgress, "Ze egress receives the ResvErr freeRouter relays", timeout, func(message rsvpMessage) bool {
		return strings.HasPrefix(message.target, addressZeEgress) && strings.Contains(message.text, "ResvErr Message") &&
			strings.Contains(message.text, hopFrom(addressFreeRtrRelay))
	})
	if err != nil {
		return err
	}
	if !strings.Contains(relayed.text, errorFrom(addressZeIngress)) {
		return fmt.Errorf("relayed ResvErr lost the error node %s:\n%s", addressZeIngress, relayed.text)
	}
	// RFC 2205 Appendix B: Error Code 1 is Admission Control failure, and the
	// ingress refuses the reservation for want of bandwidth, value 2.
	code, value, err := errorSpec(relayed.text)
	if err != nil {
		return err
	}
	if code != "1" {
		return fmt.Errorf("relayed ResvErr carries Error Code %s, want 1 Admission Control failure:\n%s", code, relayed.text)
	}
	if value != "2" {
		return fmt.Errorf("relayed ResvErr carries Error Value %s, want 2 requested bandwidth unavailable:\n%s", value, relayed.text)
	}
	return nil
}

// errorSpec answers the Error Code and Error Value of the ERROR_SPEC object in
// one decoded message. tcpdump names neither for every code (it prints
// Admission Control failure as "unknown (1)"), so the check compares the two
// numbers it prints in parentheses: the first is the code, the last the value.
func errorSpec(text string) (code, value string, err error) {
	for line := range strings.SplitSeq(text, "\n") {
		_, rest, found := strings.Cut(line, "Error Code: ")
		if !found {
			continue
		}
		var numbers []string
		for {
			_, after, open := strings.Cut(rest, "(")
			if !open {
				break
			}
			number, tail, closed := strings.Cut(after, ")")
			if !closed {
				break
			}
			numbers = append(numbers, number)
			rest = tail
		}
		if len(numbers) < 2 {
			return "", "", fmt.Errorf("ERROR_SPEC line %q names no code and value", strings.TrimSpace(line))
		}
		return numbers[0], numbers[len(numbers)-1], nil
	}
	return "", "", errors.New("message carries no ERROR_SPEC object")
}

// checkStrictForwarded proves RFC 3209 Section 4.3.4.1 at a Ze transit fed by
// an independent relay: Ze ingress, freeRouter, Ze transit, freeRouter egress.
// The ingress names every hop strict. The PATH the freeRouter egress receives
// from Ze MUST carry the ERO shortened to the egress alone, still strict, and
// the LSP MUST come up: the Ze ingress receives a labeled RESV through
// freeRouter and the Ze transit holds a swap toward the egress.
func checkStrictForwarded(ctx context.Context, lab interoplab.CheckerLab, timeout time.Duration) error {
	path, err := waitMessage(ctx, lab, peerEgress, "freeRouter egress receives Ze's PATH", timeout, func(message rsvpMessage) bool {
		return strings.Contains(message.text, "Path Message") && strings.Contains(message.text, hopFrom(addressZeTransit))
	})
	if err != nil {
		return err
	}
	ero, err := explicitRoute(path.text)
	if err != nil {
		return err
	}
	if len(ero) != 1 || !strings.Contains(ero[0], addressFreeRtrEgress+"/32") || !strings.Contains(ero[0], "Strict") {
		return fmt.Errorf("egress received ERO %q, want only strict %s", ero, addressFreeRtrEgress)
	}
	if _, err := waitMessage(ctx, lab, peerIngress, "Ze ingress receives the RESV freeRouter relays", timeout, func(message rsvpMessage) bool {
		return isMessage(message, addressFreeRtrRelay, addressZeIngress, "Resv Message") && strings.Contains(message.text, "Label Object")
	}); err != nil {
		return err
	}
	return waitSwap(ctx, lab, timeout)
}

// checkStrictRefused proves RFC 3209 Section 4.3.3.1 across an independent
// relay: Ze ingress, freeRouter, Ze transit. The ERO's last hop is strict and
// the transit's native route to it runs through a node outside both abstract
// nodes, so the transit MUST NOT forward the PATH and MUST send PathErr
// "Routing Problem / Bad strict node" (24/2). freeRouter relays it upstream,
// and the Ze ingress MUST receive it from freeRouter with the transit as the
// error node.
func checkStrictRefused(ctx context.Context, lab interoplab.CheckerLab, timeout time.Duration) error {
	refusal, err := waitMessage(ctx, lab, peerIngress, "Ze ingress receives the PathErr freeRouter relays", timeout, func(message rsvpMessage) bool {
		return isMessage(message, addressFreeRtrRelay, addressZeIngress, "PathErr Message")
	})
	if err != nil {
		return err
	}
	if !strings.Contains(refusal.text, errorFrom(addressZeTransit)) {
		return fmt.Errorf("relayed PathErr does not name the Ze transit %s:\n%s", addressZeTransit, refusal.text)
	}
	// RFC 3209 Section 4.3.3.1: Routing Problem (24), Bad strict node (2).
	code, value, err := errorSpec(refusal.text)
	if err != nil {
		return err
	}
	if code != "24" {
		return fmt.Errorf("relayed PathErr carries Error Code %s, want 24 Routing Problem:\n%s", code, refusal.text)
	}
	if value != "2" {
		return fmt.Errorf("relayed PathErr carries Error Value %s, want 2 Bad strict node:\n%s", value, refusal.text)
	}
	capture, err := lab.Query(ctx, peerTransit, []string{catCommand, captureFile}, nil)
	if err != nil {
		return err
	}
	for _, message := range parseCapture(capture) {
		if strings.Contains(message.text, "Path Message") && strings.Contains(message.text, hopFrom(addressZeTransit)) {
			return fmt.Errorf("the Ze transit forwarded the refused PATH:\n%s", message.text)
		}
	}
	return nil
}

// checkResvTearRelayed proves RFC 2205 Section 3.1.6 across an independent
// relay: Ze ingress, freeRouter, Ze transit, Ze egress. Once the LSP is up the
// egress is frozen, so the transit's reservation times out while its path
// state is refreshed. The transit MUST send a ResvTear upstream, freeRouter
// relays it, and the Ze ingress MUST receive it from freeRouter.
func checkResvTearRelayed(ctx context.Context, lab interoplab.CheckerLab, timeout time.Duration) error {
	if _, err := waitMessage(ctx, lab, peerIngress, "Ze ingress receives the RESV freeRouter relays", timeout, func(message rsvpMessage) bool {
		return isMessage(message, addressFreeRtrRelay, addressZeIngress, "Resv Message") && strings.Contains(message.text, "Label Object")
	}); err != nil {
		return err
	}
	if _, err := lab.Query(ctx, peerEgress, []string{"sh", "-c", "kill -STOP $(pidof ze) && echo frozen"}, nil); err != nil {
		return fmt.Errorf("freeze the Ze egress: %w", err)
	}
	if _, err := waitMessage(ctx, lab, peerRelay, "freeRouter receives the Ze transit's ResvTear", timeout, func(message rsvpMessage) bool {
		return isMessage(message, addressZeTransit, addressFreeRtrRelay, "ResvTear Message")
	}); err != nil {
		return err
	}
	_, err := waitMessage(ctx, lab, peerIngress, "Ze ingress receives the ResvTear freeRouter relays", timeout, func(message rsvpMessage) bool {
		return isMessage(message, addressFreeRtrRelay, addressZeIngress, "ResvTear Message")
	})
	return err
}

// checkIncreaseInPlace proves RFC 2205 Section 3.1.8 for an in-place increase
// at a Ze transit: freeRouter ingress, Ze transit, patched freeRouter egress.
// The ingress signals 10 Mbit/s. Once the LSP is up the egress's second RESV,
// answering the ingress's first PATH refresh, raises its reservation to
// 1 Gbit/s, same session, sender and LSP-ID, past the 100 Mbit/s Ze's eth0
// reserves. Ze MUST send the egress a ResvErr naming itself, Admission Control
// failure (1) / requested bandwidth unavailable (2), with the InPlace flag on,
// MUST NOT relay the raised request upstream, and MUST keep the reservation it
// holds: its swap stays installed.
func checkIncreaseInPlace(ctx context.Context, lab interoplab.CheckerLab, timeout time.Duration) error {
	if _, err := waitMessage(ctx, lab, peerIngress, "ingress receives Ze's RESV", timeout, func(message rsvpMessage) bool {
		return isMessage(message, addressZeTransit, addressFreeRtrIngress, "Resv Message") && strings.Contains(message.text, "Label Object")
	}); err != nil {
		return err
	}
	if err := waitSwap(ctx, lab, timeout); err != nil {
		return err
	}
	// The positive control: the stimulus crossed the wire. Without it a
	// missing ResvErr would read the same as a knob that never fired.
	if _, err := waitMessage(ctx, lab, peerTransit, "Ze transit receives the raised RESV", timeout+freeRtrRefresh, func(message rsvpMessage) bool {
		return isMessage(message, addressFreeRtrEgress, addressZeTransit, "Resv Message") && strings.Contains(message.text, raisedRate)
	}); err != nil {
		return err
	}
	refusal, err := waitMessage(ctx, lab, peerEgress, "egress receives the Ze transit's ResvErr", timeout, func(message rsvpMessage) bool {
		return isMessage(message, addressZeTransit, addressFreeRtrEgress, "ResvErr Message") && strings.Contains(message.text, errorFrom(addressZeTransit))
	})
	if err != nil {
		return err
	}
	code, value, err := errorSpec(refusal.text)
	if err != nil {
		return err
	}
	if code != "1" {
		return fmt.Errorf("ResvErr carries Error Code %s, want 1 Admission Control failure:\n%s", code, refusal.text)
	}
	if value != "2" {
		return fmt.Errorf("ResvErr carries Error Value %s, want 2 requested bandwidth unavailable:\n%s", value, refusal.text)
	}
	// RFC 2205 Appendix A.5: InPlace is flag 0x01 of the ERROR_SPEC.
	if !strings.Contains(refusal.text, errorFrom(addressZeTransit)+" Flags: [0x01]") {
		return fmt.Errorf("ResvErr for a failed increase does not carry InPlace (0x01):\n%s", refusal.text)
	}
	if err := waitSwap(ctx, lab, timeout); err != nil {
		return fmt.Errorf("the reservation in place lost its swap after the refused increase: %w", err)
	}
	capture, err := lab.Query(ctx, peerIngress, []string{catCommand, captureFile}, nil)
	if err != nil {
		return err
	}
	for _, message := range parseCapture(capture) {
		if isMessage(message, addressZeTransit, addressFreeRtrIngress, "Resv Message") && strings.Contains(message.text, raisedRate) {
			return fmt.Errorf("the Ze transit relayed the refused increase upstream:\n%s", message.text)
		}
	}
	return nil
}

// checkFFUnknownSender proves RFC 2205 Section 3.1.8 for a fixed-filter RESV
// at a Ze transit: freeRouter ingress, Ze transit, patched freeRouter egress.
// Every RESV the egress sends is fixed-filter and carries a second flow
// descriptor naming LSP-ID+1, a sender with no path state. Ze MUST send the
// egress a ResvErr for that descriptor alone (one FILTER_SPEC, the unknown
// LSP-ID, No sender information (4)), MUST NOT name the known sender in any
// ResvErr, and MUST bring the known sender's LSP up: a labeled RESV naming
// only that sender reaches the ingress and the swap is installed.
func checkFFUnknownSender(ctx context.Context, lab interoplab.CheckerLab, timeout time.Duration) error {
	path, err := waitMessage(ctx, lab, peerEgress, "egress receives Ze's PATH", timeout, func(message rsvpMessage) bool {
		return strings.Contains(message.text, "Path Message") && strings.Contains(message.text, hopFrom(addressZeTransit))
	})
	if err != nil {
		return err
	}
	senders := lspIDs(path.text)
	if len(senders) != 1 {
		return fmt.Errorf("PATH names LSP-IDs %q, want one sender:\n%s", senders, path.text)
	}
	known := senders[0]
	// The positive control: the stimulus crossed the wire.
	stimulus, err := waitMessage(ctx, lab, peerTransit, "Ze transit receives the two-sender FF RESV", timeout, func(message rsvpMessage) bool {
		return isMessage(message, addressFreeRtrEgress, addressZeTransit, "Resv Message") && strings.Contains(message.text, "Fixed Filter") &&
			len(lspIDs(message.text)) == 2
	})
	if err != nil {
		return err
	}
	unknown := ""
	for _, id := range lspIDs(stimulus.text) {
		if id != known {
			unknown = id
		}
	}
	if unknown == "" {
		return fmt.Errorf("FF RESV names no sender beside the known LSP-ID %s:\n%s", known, stimulus.text)
	}
	refusal, err := waitMessage(ctx, lab, peerEgress, "egress receives the Ze transit's ResvErr", timeout, func(message rsvpMessage) bool {
		return isMessage(message, addressZeTransit, addressFreeRtrEgress, "ResvErr Message") && strings.Contains(message.text, errorFrom(addressZeTransit))
	})
	if err != nil {
		return err
	}
	if named := lspIDs(refusal.text); !slices.Equal(named, []string{unknown}) {
		return fmt.Errorf("ResvErr names LSP-IDs %q, want only the unknown sender %s:\n%s", named, unknown, refusal.text)
	}
	code, _, err := errorSpec(refusal.text)
	if err != nil {
		return err
	}
	// RFC 2205 Appendix B: Error Code 4, "No sender information for this
	// Resv message"; the session has path state, only this sender has none.
	if code != "4" {
		return fmt.Errorf("ResvErr carries Error Code %s, want 4 No sender information:\n%s", code, refusal.text)
	}
	relayed, err := waitMessage(ctx, lab, peerIngress, "ingress receives Ze's RESV", timeout, func(message rsvpMessage) bool {
		return isMessage(message, addressZeTransit, addressFreeRtrIngress, "Resv Message") && strings.Contains(message.text, "Label Object")
	})
	if err != nil {
		return err
	}
	if named := lspIDs(relayed.text); !slices.Equal(named, []string{known}) {
		return fmt.Errorf("RESV Ze relays names LSP-IDs %q, want only the known sender %s:\n%s", named, known, relayed.text)
	}
	if err := waitSwap(ctx, lab, timeout); err != nil {
		return err
	}
	capture, err := lab.Query(ctx, peerEgress, []string{catCommand, captureFile}, nil)
	if err != nil {
		return err
	}
	for _, message := range parseCapture(capture) {
		if isMessage(message, addressZeTransit, addressFreeRtrEgress, "ResvErr Message") && slices.Contains(lspIDs(message.text), known) {
			return fmt.Errorf("a ResvErr names the known sender %s:\n%s", known, message.text)
		}
	}
	return nil
}

// checkImplicitNullPop proves RFC 3032 Section 2.1 at a Ze transit that is the
// penultimate hop: freeRouter ingress, Ze transit, patched freeRouter egress.
// Every RESV the egress sends distributes Implicit NULL (label 3). Ze MUST
// relay a RESV carrying a label of its own to the ingress, and MUST pop that
// label toward the egress rather than swap it for 3: its MPLS table holds the
// label via the egress with no outgoing label.
func checkImplicitNullPop(ctx context.Context, lab interoplab.CheckerLab, timeout time.Duration) error {
	// The positive control: the stimulus crossed the wire. Without it a pop
	// would read the same as an egress that distributed an ordinary label.
	if _, err := waitMessage(ctx, lab, peerTransit, "Ze transit receives the egress's Implicit NULL RESV", timeout, func(message rsvpMessage) bool {
		return isMessage(message, addressFreeRtrEgress, addressZeTransit, "Resv Message") &&
			slices.Equal(resvLabels(message.text), []string{labelImplicitNull})
	}); err != nil {
		return err
	}
	relayed, err := waitMessage(ctx, lab, peerIngress, "ingress receives Ze's labeled RESV", timeout, func(message rsvpMessage) bool {
		return isMessage(message, addressZeTransit, addressFreeRtrIngress, "Resv Message") && len(resvLabels(message.text)) == 1
	})
	if err != nil {
		return err
	}
	inLabel := resvLabels(relayed.text)[0]
	if inLabel == labelImplicitNull {
		return fmt.Errorf("the Ze transit relayed Implicit NULL upstream instead of a label it popped:\n%s", relayed.text)
	}
	last := ""
	_, _, err = interoplab.Wait(ctx, interoplab.WaitOptions{Timeout: timeout, Interval: time.Second, Description: "Ze installs a pop of label " + inLabel + " toward the egress"},
		func(ctx context.Context) (bool, error) {
			table, err := lab.Query(ctx, peerTransit, []string{"ip", "-f", "mpls", "route", "show"}, nil)
			if err != nil {
				return false, err
			}
			last = table
			return isPop(table, inLabel), nil
		},
		func(popped bool) bool { return popped })
	if err != nil {
		return fmt.Errorf("%w; last MPLS table at the Ze transit:\n%s", err, last)
	}
	return nil
}

// isPop reports whether an `ip -f mpls route show` table forwards inLabel
// toward the freeRouter egress with no outgoing label: iproute2 prints a swap
// as "<in> as to <out> via inet <hop>" and a pop as "<in> via inet <hop>".
func isPop(table, inLabel string) bool {
	for line := range strings.SplitSeq(table, "\n") {
		label, rest, found := strings.Cut(strings.TrimSpace(line), " ")
		if !found {
			continue
		}
		if label != inLabel {
			continue
		}
		if strings.Contains(rest, "as to") {
			return false
		}
		return strings.Contains(rest, "via inet "+addressFreeRtrEgress)
	}
	return false
}

// resvLabels answers the values of a message's LABEL object, in order.
// tcpdump prints each value indented under the object header as "Label: N";
// the header itself reads "Class-Type: Label (1)", so it never matches.
func resvLabels(text string) []string {
	lines := strings.Split(text, "\n")
	for index, line := range lines {
		if !strings.Contains(line, "Label Object") {
			continue
		}
		var labels []string
		for _, sub := range lines[index+1:] {
			if strings.Contains(sub, "Object (") {
				break
			}
			if _, value, found := strings.Cut(sub, "Label: "); found {
				labels = append(labels, strings.TrimSpace(value))
			}
		}
		return labels
	}
	return nil
}

// checkResvStateExpires proves RFC 2205 Sections 2.3 and 3.1.6 at a Ze transit
// between two freeRouters: freeRouter ingress, Ze transit, freeRouter egress.
// Once the LSP is up the egress's freeRouter is frozen, so no RESV refreshes
// the reservation Ze holds while the ingress keeps its path state refreshed.
// Ze MUST time the reservation out: it MUST send the ingress a ResvTear, and
// MUST withdraw the swap the reservation installed.
func checkResvStateExpires(ctx context.Context, lab interoplab.CheckerLab, timeout time.Duration) error {
	if _, err := waitMessage(ctx, lab, peerIngress, "ingress receives Ze's labeled RESV", timeout, func(message rsvpMessage) bool {
		return isMessage(message, addressZeTransit, addressFreeRtrIngress, "Resv Message") && strings.Contains(message.text, "Label Object")
	}); err != nil {
		return err
	}
	if err := waitSwap(ctx, lab, timeout); err != nil {
		return err
	}
	// Before the freeze no ResvTear may exist, or the one found later could
	// have come from something other than the expiry.
	early, err := lab.Query(ctx, peerIngress, []string{catCommand, captureFile}, nil)
	if err != nil {
		return err
	}
	for _, message := range parseCapture(early) {
		if isMessage(message, addressZeTransit, addressFreeRtrIngress, "ResvTear Message") {
			return fmt.Errorf("the Ze transit sent a ResvTear while the egress still refreshed its RESV:\n%s", message.text)
		}
	}
	if _, err := lab.Query(ctx, peerEgress, []string{"sh", "-c", "kill -STOP $(pidof java) && echo frozen"}, nil); err != nil {
		return fmt.Errorf("freeze the freeRouter egress: %w", err)
	}
	if _, err := waitMessage(ctx, lab, peerIngress, "freeRouter ingress receives the Ze transit's ResvTear", timeout+freeRtrResvLifetime, func(message rsvpMessage) bool {
		return isMessage(message, addressZeTransit, addressFreeRtrIngress, "ResvTear Message")
	}); err != nil {
		return err
	}
	// The withdrawn table can be empty, and Query refuses empty output, so the
	// probe prints a marker after the table: the marker is the positive
	// evidence that the read ran, which the absent swap alone cannot be.
	const tableEnd = "end-of-mpls-table"
	last := ""
	_, _, err = interoplab.Wait(ctx, interoplab.WaitOptions{Timeout: timeout, Interval: time.Second, Description: "Ze withdraws the expired reservation's swap"},
		func(ctx context.Context) (bool, error) {
			table, err := lab.Query(ctx, peerTransit, []string{"sh", "-c", "ip -f mpls route show && echo " + tableEnd}, nil)
			if err != nil {
				return false, err
			}
			last = table
			if !strings.Contains(table, tableEnd) {
				return false, nil
			}
			return !strings.Contains(table, "via inet "+addressFreeRtrEgress), nil
		},
		func(withdrawn bool) bool { return withdrawn })
	if err != nil {
		return fmt.Errorf("%w; last MPLS table at the Ze transit:\n%s", err, last)
	}
	return nil
}

// lspIDs answers every LSP-ID tcpdump prints in one message, in order: one per
// SENDER_TEMPLATE or FILTER_SPEC of an LSP tunnel.
func lspIDs(text string) []string {
	var ids []string
	for line := range strings.SplitSeq(text, "\n") {
		_, rest, found := strings.Cut(line, "LSP-ID: ")
		if !found {
			continue
		}
		id, _, _ := strings.Cut(rest, ",")
		ids = append(ids, strings.TrimSpace(id))
	}
	return ids
}

// senderIs reports whether a PATH's SENDER_TEMPLATE or a RESV's FILTER_SPEC
// names address. tcpdump prints the LSP_TUNNEL_IPv4 SENDER_TEMPLATE as
// "IPv4 Tunnel Sender Address: X, LSP-ID" and the FILTER_SPEC as
// "Source Address: X, LSP-ID".
func senderIs(text, address string) bool {
	return strings.Contains(text, "Tunnel Sender Address: "+address+",") ||
		strings.Contains(text, "Source Address: "+address+",")
}

// ospfNeighbor is the part of one `show ospf neighbor` entry the PLR check reads.
type ospfNeighbor struct {
	RouterID string `json:"router-id"`
	State    string `json:"state"`
}

// routerLSAWithLinkOctets is the length of a router-LSA carrying the egress's
// stub network and one more link (RFC 2328 Section A.4.2: a 20-octet header,
// 4 octets of flags and link count, then 12 octets for each link without TOS).
const routerLSAWithLinkOctets = 48

// waitMergePointAttributed waits until the Ze PLR's database holds a router-LSA
// from the freeRouter egress with its point-to-point link, whose Link Data is
// the eth0 address 172.29.81.14 (RFC 2328 Section A.4.2). A Full adjacency is
// not that proof: the egress re-originates its router-LSA after the adjacency
// forms, and until it arrives the egress advertises its stub network alone, so
// nothing ties 172.29.81.14 to 10.0.14.14 and the PLR rightly refuses the
// merge point's RESV from that address. Cutting the link before then measures
// the lab's timing, not Ze.
func waitMergePointAttributed(ctx context.Context, lab interoplab.CheckerLab, timeout time.Duration) error {
	command := []string{"ze", "cli", "-c", "show ospf database router", "--user", "interop", "--format", "json"}
	environment := []interoplab.EnvironmentVariable{{Name: "ZE_SSH_PASSWORD", Value: "testpass"}}
	last := ""
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{Timeout: timeout, Interval: time.Second, Description: "Ze PLR holds the freeRouter egress's router-LSA with its point-to-point link"},
		func(ctx context.Context) (bool, error) {
			answer, err := lab.Query(ctx, peerTransit, command, environment)
			if err != nil {
				return false, err
			}
			last = answer
			var database any
			if err := json.Unmarshal([]byte(answer), &database); err != nil {
				return false, fmt.Errorf("decode `show ospf database router` reply %q: %w", answer, err)
			}
			return egressRouterLSALinked(database), nil
		},
		func(linked bool) bool { return linked })
	if err != nil {
		return fmt.Errorf("%w; last `show ospf database router` answer: %s", err, last)
	}
	return nil
}

// egressRouterLSALinked walks the decoded `show ospf database router` reply for
// the egress's router-LSA and reports whether it carries a link beside its stub.
// The walk descends only Ze's own CLI reply, a structure a few levels deep.
func egressRouterLSALinked(node any) bool {
	switch value := node.(type) {
	case []any:
		return slices.ContainsFunc(value, egressRouterLSALinked)
	case map[string]any:
		if value["advertising-router"] == addressProtectedMP {
			if length, ok := value["length"].(float64); ok && length >= routerLSAWithLinkOctets {
				return true
			}
		}
		for _, item := range value {
			if egressRouterLSALinked(item) {
				return true
			}
		}
	}
	return false
}

// waitOSPFFull waits until the Ze PLR holds a Full OSPF adjacency with the
// freeRouter egress, the first condition for the database to attribute the
// egress's eth0 address to the merge point (waitMergePointAttributed).
func waitOSPFFull(ctx context.Context, lab interoplab.CheckerLab, timeout time.Duration) error {
	command := []string{"ze", "cli", "-c", "show ospf neighbor", "--user", "interop", "--format", "json"}
	environment := []interoplab.EnvironmentVariable{{Name: "ZE_SSH_PASSWORD", Value: "testpass"}}
	last := ""
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{Timeout: timeout, Interval: 2 * time.Second, Description: "Ze PLR holds a Full OSPF adjacency with the freeRouter egress " + addressProtectedMP},
		// Wait retries a probe error: the CLI answers only once the SSH server
		// and the OSPF plugin run.
		func(ctx context.Context) (bool, error) {
			answer, err := lab.Query(ctx, peerTransit, command, environment)
			if err != nil {
				return false, err
			}
			last = answer
			var neighbors []ospfNeighbor
			if err := json.Unmarshal([]byte(answer), &neighbors); err != nil {
				return false, fmt.Errorf("decode `show ospf neighbor` reply %q: %w", answer, err)
			}
			for _, neighbor := range neighbors {
				if neighbor.RouterID == addressProtectedMP && neighbor.State == ospfNeighborFull {
					return true, nil
				}
			}
			return false, nil
		},
		func(full bool) bool { return full })
	if err != nil {
		return fmt.Errorf("%w; last `show ospf neighbor` answer: %s", err, last)
	}
	return nil
}

// isBackupPath reports whether message is the PLR's backup PATH for the
// protected tunnel: the PLR is both its sender (RFC 4090 Section 6.1) and its
// RSVP_HOP, because the PATH crosses the bypass LSP labeled and the relay
// never processes it. The bypass's own PATH reaches the egress with the relay
// as its RSVP_HOP and under a bypass tunnel ID, so it cannot match.
func isBackupPath(message rsvpMessage) bool {
	return strings.Contains(message.text, "Path Message") &&
		strings.Contains(message.text, hopFrom(addressZeTransit)) &&
		senderIs(message.text, addressZeTransit) &&
		strings.Contains(message.text, protectedTunnel)
}

// checkBackupPathToMP proves RFC 4090 Sections 6.4 and 6.1 at a Ze PLR
// against a freeRouter merge point: Ze ingress, Ze PLR, freeRouter relay on
// the bypass, freeRouter egress. The ingress asks facility protection for a
// tunnel whose protected hop is the PLR's prot0 (VLAN 100) to the egress, and
// the PLR holds a configured bypass through the relay that merges at the same
// egress. Before the failure the egress MUST NOT see a backup PATH. Once prot0
// goes down the PLR MUST send the egress, through the bypass, a PATH whose
// sender and RSVP_HOP are the PLR and whose ERO is the merge point alone, and
// freeRouter MUST answer it: the PLR captures labeled RESVs from the egress
// naming the PLR's sender, refreshed, not a one-off.
func checkBackupPathToMP(ctx context.Context, lab interoplab.CheckerLab, timeout time.Duration) error {
	if _, err := waitMessage(ctx, lab, peerIngress, "Ze ingress receives the PLR's labeled RESV", timeout, func(message rsvpMessage) bool {
		return isMessage(message, addressZeTransit, addressZeIngress, "Resv Message") && strings.Contains(message.text, "Label Object")
	}); err != nil {
		return err
	}
	if _, err := waitMessage(ctx, lab, peerTransit, "Ze PLR receives the bypass RESV freeRouter relays", timeout, func(message rsvpMessage) bool {
		return isMessage(message, addressFreeRtrRelay, addressZeTransit, "Resv Message") && strings.Contains(message.text, "Label Object")
	}); err != nil {
		return err
	}
	if err := waitOSPFFull(ctx, lab, timeout); err != nil {
		return err
	}
	if err := waitMergePointAttributed(ctx, lab, timeout); err != nil {
		return err
	}
	early, err := lab.Query(ctx, peerEgress, []string{catCommand, captureFile}, nil)
	if err != nil {
		return err
	}
	for _, message := range parseCapture(early) {
		if isBackupPath(message) {
			return fmt.Errorf("freeRouter egress holds a backup PATH before the protected link failed:\n%s", message.text)
		}
	}
	beforeCut, err := lab.Query(ctx, peerTransit, []string{catCommand, captureFile}, nil)
	if err != nil {
		return err
	}
	cutIndex := len(parseCapture(beforeCut))
	if _, err := lab.Query(ctx, peerTransit, []string{"sh", "-c", "ip link set prot0 down && echo down"}, nil); err != nil {
		return fmt.Errorf("take the protected link down: %w", err)
	}
	cutAt := time.Now()
	backup, err := waitMessage(ctx, lab, peerEgress, "freeRouter egress receives the PLR's backup PATH", timeout, isBackupPath)
	if err != nil {
		return err
	}
	ero, err := explicitRoute(backup.text)
	if err != nil {
		return err
	}
	if len(ero) != 1 {
		return fmt.Errorf("backup PATH carries ERO %q, want the merge point %s alone", ero, addressProtectedMP)
	}
	if !strings.Contains(ero[0], addressProtectedMP+"/32") {
		return fmt.Errorf("backup PATH carries ERO %q, want the merge point %s alone", ero, addressProtectedMP)
	}
	answers := func(message rsvpMessage) bool {
		if !strings.HasPrefix(message.target, addressZeTransit) {
			return false
		}
		if !strings.Contains(message.text, "Resv Message") {
			return false
		}
		if !strings.HasPrefix(message.source, addressFreeRtrEgress) && !strings.HasPrefix(message.source, addressProtectedMP) {
			return false
		}
		return strings.Contains(message.text, "Label Object") && senderIs(message.text, addressZeTransit) && strings.Contains(message.text, protectedTunnel)
	}
	_, _, err = interoplab.Wait(ctx, interoplab.WaitOptions{Timeout: timeout, Interval: time.Second, Description: "Ze PLR receives two labeled RESVs from the merge point for its backup sender"},
		func(ctx context.Context) (int, error) { return countCaptured(ctx, lab, peerTransit, answers) },
		func(count int) bool { return count >= 2 })
	if err != nil {
		return protectedSessionTrace(ctx, lab, err)
	}
	// RFC 4090 Section 6.4.3: "The MP sends Resv, ResvTear, and PathErr
	// messages by sending them directly to the address in the RSVP_HOP
	// object". freeRouter does, from its eth0 address; the PLR MUST take that
	// RESV as the merge point's and so MUST NOT refuse it with a ResvErr.
	capture, err := lab.Query(ctx, peerTransit, []string{catCommand, captureFile}, nil)
	if err != nil {
		return err
	}
	for _, message := range parseCapture(capture) {
		if strings.HasPrefix(message.source, addressZeTransit) && strings.Contains(message.text, "ResvErr Message") && strings.Contains(message.text, protectedTunnel) {
			return protectedSessionTrace(ctx, lab, fmt.Errorf("the Ze PLR refused the merge point's RESV for its backup sender:\n%s", message.text))
		}
	}
	// RFC 4090 Section 6.4.3: the PLR has "to start sending the control
	// traffic for the protected LSP onto the bypass tunnel". The ingress keeps
	// refreshing the PATH whose strict hop lies across the failed link, so the
	// PLR MUST relay that refresh rather than refuse it with a PathErr.
	var refusal *rsvpMessage
	_, _, err = interoplab.Wait(ctx, interoplab.WaitOptions{Timeout: timeout, Interval: time.Second, Description: "Ze PLR handles the ingress's PATH refresh of the repaired LSP"},
		func(ctx context.Context) (bool, error) {
			capture, err := lab.Query(ctx, peerTransit, []string{catCommand, captureFile}, nil)
			if err != nil {
				return false, err
			}
			messages := parseCapture(capture)
			var handled bool
			handled, refusal = upstreamRefreshHandled(messages[min(cutIndex, len(messages)):])
			return handled, nil
		},
		func(handled bool) bool { return handled })
	if err != nil {
		return protectedSessionTrace(ctx, lab, err)
	}
	if refusal != nil {
		return protectedSessionTrace(ctx, lab, fmt.Errorf("the Ze PLR refused the ingress's PATH refresh of the repaired LSP:\n%s", refusal.text))
	}
	// No refusal is an absence, and a PLR that dropped the refresh in silence
	// leaves one too: its own timer keeps sending the backup PATH until the
	// protected PSB expires. So the repair MUST still run after that state's
	// lifetime has passed: the egress receives a further backup PATH, and the
	// PLR a further labeled RESV from the merge point.
	if err := repairOutlivesProtectedState(ctx, lab, cutAt, answers, timeout); err != nil {
		return protectedSessionTrace(ctx, lab, err)
	}
	return nil
}

// protectedStateLifetime bounds how long the PLR keeps the protected LSP's PSB
// with no ingress refresh. RFC 2205 Section 3.7: "To avoid premature loss of
// state, L must satisfy L >= (K + 0.5)*1.5*R, where K is a small integer." and
// "Currently K = 3 is suggested as the default." The scenario's ingress
// refreshes every 2 seconds (R, ingress.conf refresh-period), so L is 10.5
// seconds; Ze's own expiry, refresh-period times refresh-multiplier 3 in
// transit.conf, is 6 seconds and falls inside it.
const protectedStateLifetime = time.Duration((3 + 0.5) * 1.5 * 2 * float64(time.Second))

// repairOutlivesProtectedState waits until protectedStateLifetime has passed
// since the protected link went down, then requires a backup PATH at the
// egress and a labeled RESV from the merge point at the PLR that the captures
// did not hold at that moment. A PLR whose protected PSB expired stops both.
func repairOutlivesProtectedState(ctx context.Context, lab interoplab.CheckerLab, cutAt time.Time, answers func(rsvpMessage) bool, timeout time.Duration) error {
	wait := time.NewTimer(time.Until(cutAt.Add(protectedStateLifetime)))
	defer wait.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-wait.C:
	}
	paths, err := countCaptured(ctx, lab, peerEgress, isBackupPath)
	if err != nil {
		return err
	}
	resvs, err := countCaptured(ctx, lab, peerTransit, answers)
	if err != nil {
		return err
	}
	for _, step := range []struct {
		peer, description string
		match             func(rsvpMessage) bool
		seen              int
	}{
		{peerEgress, "freeRouter egress still receives the PLR's backup PATH after the protected PSB's lifetime", isBackupPath, paths},
		{peerTransit, "Ze PLR still receives the merge point's labeled RESV after the protected PSB's lifetime", answers, resvs},
	} {
		_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{Timeout: timeout, Interval: time.Second, Description: step.description},
			func(ctx context.Context) (int, error) { return countCaptured(ctx, lab, step.peer, step.match) },
			func(count int) bool { return count > step.seen })
		if err != nil {
			return err
		}
	}
	return nil
}

// countCaptured answers how many messages in peer's capture match.
func countCaptured(ctx context.Context, lab interoplab.CheckerLab, peer string, match func(rsvpMessage) bool) (int, error) {
	capture, err := lab.Query(ctx, peer, []string{catCommand, captureFile}, nil)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, message := range parseCapture(capture) {
		if match(message) {
			count++
		}
	}
	return count, nil
}

// upstreamRefreshesBeforeVerdict is how many of the ingress's PATH refreshes
// of the protected LSP the PLR capture must hold after the cut before the
// absence of a refusal counts. The scenario refreshes every 2 seconds and the
// PLR answers a PATH in milliseconds, so a PathErr for the first refresh is
// captured long before the last; the count does not rest on how the PLR's
// timer-driven sends interleave with its packet handling.
const upstreamRefreshesBeforeVerdict = 3

// protectedLSP is the sender template tcpdump prints for the protected LSP the
// ingress signals first, and refreshes after the PLR repairs it.
const protectedLSP = "Tunnel Sender Address: " + addressZeIngress + ", LSP-ID: 0x0001"

// upstreamRefreshHandled reads the PLR's capture from the moment the protected
// link went down. It reports the refresh handled once the PLR refused a PATH of
// the protected LSP with a Routing Problem PathErr, returning that refusal, or
// once the capture holds upstreamRefreshesBeforeVerdict PATH refreshes of that
// LSP from the ingress with no such refusal. Two other PathErrs are correct and
// do not count: the Notify "Tunnel locally repaired" the PLR owes the ingress
// for the protected LSP (RFC 4090 Section 6.5.1), and a Routing Problem for a
// new LSP-ID the ingress signals over the same ERO, which holds no repair and
// whose strict hop is no longer adjacent.
func upstreamRefreshHandled(messages []rsvpMessage) (bool, *rsvpMessage) {
	refreshes := 0
	for index := range messages {
		message := &messages[index]
		if !strings.Contains(message.text, protectedTunnel) {
			continue
		}
		if !strings.Contains(message.text, protectedLSP+"\n") {
			continue
		}
		if isMessage(*message, addressZeTransit, "", "PathErr Message") && strings.Contains(message.text, "Routing Problem (24)") {
			return true, message
		}
		if isMessage(*message, addressZeIngress, "", "Path Message") {
			refreshes++
		}
	}
	return refreshes >= upstreamRefreshesBeforeVerdict, nil
}

// protectedSessionTrace appends the last messages of the protected session each
// of the PLR and the egress captured, which the capture tails diagnosticError
// prints are usually too short to reach once the bypass refreshes.
func protectedSessionTrace(ctx context.Context, lab interoplab.CheckerLab, cause error) error {
	var report strings.Builder
	report.WriteString(cause.Error())
	for _, peer := range []string{peerTransit, peerEgress} {
		capture, err := lab.Query(ctx, peer, []string{catCommand, captureFile}, nil)
		if err != nil {
			report.WriteString("\n--- " + peer + " capture unreadable: " + err.Error())
			continue
		}
		var session []rsvpMessage
		for _, message := range parseCapture(capture) {
			if strings.Contains(message.text, protectedTunnel) {
				session = append(session, message)
			}
		}
		report.WriteString("\n--- " + peer + " protected-session messages, last 6 ---")
		for _, message := range session[max(0, len(session)-6):] {
			report.WriteString("\n" + message.text)
		}
	}
	return errors.New(report.String())
}

// explicitRoute answers the subobject lines of a PATH's EXPLICIT_ROUTE object,
// in order. tcpdump prints each subobject indented under the object header.
func explicitRoute(text string) ([]string, error) {
	lines := strings.Split(text, "\n")
	for index, line := range lines {
		if !strings.Contains(line, "ERO Object") {
			continue
		}
		var hops []string
		for _, sub := range lines[index+1:] {
			if strings.Contains(sub, "Object (") {
				break
			}
			if strings.Contains(sub, "Subobject Type") {
				hops = append(hops, strings.TrimSpace(sub))
			}
		}
		return hops, nil
	}
	return nil, errors.New("PATH carries no EXPLICIT_ROUTE object")
}

// diagnosticError appends each peer's capture and log tail to a failure, so
// a red run says what crossed the wire without a second run.
func diagnosticError(ctx context.Context, lab interoplab.CheckerLab, cause error) error {
	var report strings.Builder
	report.WriteString(cause.Error())
	for _, role := range labRoles {
		peer := role.name
		if capture, err := lab.Query(ctx, peer, []string{"sh", "-c", "tail -c 6000 " + captureFile + " 2>/dev/null; tail -n 40 /run/fr/console.txt 2>/dev/null"}, nil); err == nil {
			report.WriteString("\n--- " + peer + " capture ---\n" + capture)
		}
		if logs, err := lab.Logs(ctx, peer, 60); err == nil {
			report.WriteString("\n--- " + peer + " log ---\n" + logs.Text)
		}
	}
	return errors.New(report.String())
}
