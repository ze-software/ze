// Design: docs/architecture/testing/interop.md -- RSVP-TE interop against freeRouter.
// Related: rsvpte.go -- the topology these checks observe.
//
// Every assertion reads what a peer saw on its own wire: each container runs
// tcpdump on eth0 and the checker reads its decoded text. A message Ze logs
// as sent counts for nothing until a freeRouter peer has received it.
package rsvpte

import (
	"context"
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

	// raisedRate is how tcpdump prints the FLOWSPEC rate the patched egress
	// asks once in transit-resv-increase-refused-in-place: 125000000 bytes/s
	// (egress-env.txt), 1 Gbit/s, ten times what the Ze transit's eth0 reserves.
	raisedRate = "1000 Mbps"

	// freeRtrRefresh is the pinned freeRouter's PATH refresh period
	// (ipFwd.untriggeredRecomputation, 120 s, not configurable). A freeRouter
	// egress originates a RESV only when a PATH arrives, so its second RESV
	// follows the ingress's first refresh by up to this long.
	freeRtrRefresh = 120 * time.Second

	captureFile = "/run/fr/rsvp.txt"

	// A Ze node answers on its container address, a freeRouter node on its
	// own address beside it (rsvpte.go, labRole).
	addressZeIngress      = "172.29.81.2"
	addressZeTransit      = "172.29.81.3"
	addressZeEgress       = "172.29.81.4"
	addressFreeRtrIngress = "172.29.81.12"
	addressFreeRtrEgress  = "172.29.81.14"
	addressFreeRtrRelay   = "172.29.81.15"
	loopbackEgress        = "198.51.100.4"
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
// unindented line; its second line carries "source > target:".
func parseCapture(capture string) []rsvpMessage {
	var messages []rsvpMessage
	var current []string
	flush := func() {
		if len(current) < 2 {
			current = nil
			return
		}
		addresses := strings.TrimSpace(current[1])
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
			capture, err := lab.Query(ctx, peer, []string{"cat", captureFile}, nil)
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
	return waitSwap(ctx, lab, addressFreeRtrEgress, timeout)
}

// waitSwap polls the Ze transit's MPLS table until a label forwards toward next.
func waitSwap(ctx context.Context, lab interoplab.CheckerLab, next string, timeout time.Duration) error {
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{Timeout: timeout, Interval: time.Second, Description: "Ze installs the transit swap"},
		func(ctx context.Context) (string, error) {
			return lab.Query(ctx, peerTransit, []string{"ip", "-f", "mpls", "route", "show"}, nil)
		},
		func(table string) bool { return strings.Contains(table, "via inet "+next) })
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
// the LSP MUST come up: the Ze ingress receives a labelled RESV through
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
	return waitSwap(ctx, lab, addressFreeRtrEgress, timeout)
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
	capture, err := lab.Query(ctx, peerTransit, []string{"cat", captureFile}, nil)
	if err != nil {
		return err
	}
	for _, message := range parseCapture(capture) {
		if strings.Contains(message.text, "Path Message") && strings.Contains(message.text, hopFrom(addressZeTransit)) {
			return fmt.Errorf("Ze transit forwarded the refused PATH:\n%s", message.text)
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
	if err := waitSwap(ctx, lab, addressFreeRtrEgress, timeout); err != nil {
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
	if err := waitSwap(ctx, lab, addressFreeRtrEgress, timeout); err != nil {
		return fmt.Errorf("the reservation in place lost its swap after the refused increase: %w", err)
	}
	capture, err := lab.Query(ctx, peerIngress, []string{"cat", captureFile}, nil)
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
	if err := waitSwap(ctx, lab, addressFreeRtrEgress, timeout); err != nil {
		return err
	}
	capture, err := lab.Query(ctx, peerEgress, []string{"cat", captureFile}, nil)
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
