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
	"strings"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

const (
	scenarioLooseExpansion = "transit-loose-ero-expansion"

	captureFile = "/run/fr/rsvp.txt"

	addressIngress = "172.29.81.12"
	addressZe      = "172.29.81.3"
	addressEgress  = "172.29.81.14"
	loopbackEgress = "198.51.100.4"
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
			strings.Contains(message.text, hopFrom(addressZe))
	})
	if err != nil {
		return err
	}
	ero, err := explicitRoute(path.text)
	if err != nil {
		return err
	}
	if len(ero) != 2 {
		return fmt.Errorf("egress received ERO %q, want the inserted hop %s then loose %s", ero, addressEgress, loopbackEgress)
	}
	if !strings.Contains(ero[0], addressEgress) {
		return fmt.Errorf("egress received ERO %q: first hop is not the inserted %s", ero, addressEgress)
	}
	if !strings.Contains(ero[1], loopbackEgress) {
		return fmt.Errorf("egress received ERO %q: second hop is not %s", ero, loopbackEgress)
	}
	if !strings.Contains(strings.ToLower(ero[1]), "loose") {
		return fmt.Errorf("egress received ERO %q: %s lost its loose bit", ero, loopbackEgress)
	}
	if _, err := waitMessage(ctx, lab, peerIngress, "ingress receives Ze's RESV", timeout, func(message rsvpMessage) bool {
		return isMessage(message, addressZe, addressIngress, "Resv Message") && strings.Contains(message.text, "Label Object")
	}); err != nil {
		return err
	}
	return waitSwap(ctx, lab, timeout)
}

// waitSwap polls Ze's MPLS table until a label forwards toward the egress.
func waitSwap(ctx context.Context, lab interoplab.CheckerLab, timeout time.Duration) error {
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{Timeout: timeout, Interval: time.Second, Description: "Ze installs the transit swap"},
		func(ctx context.Context) (string, error) {
			return lab.Query(ctx, peerZe, []string{"ip", "-f", "mpls", "route", "show"}, nil)
		},
		func(table string) bool { return strings.Contains(table, "via inet "+addressEgress) })
	return err
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
	for _, peer := range []string{peerIngress, peerZe, peerEgress} {
		if capture, err := lab.Query(ctx, peer, []string{"sh", "-c", "tail -c 6000 " + captureFile + " 2>/dev/null; tail -n 40 /run/fr/console.txt 2>/dev/null"}, nil); err == nil {
			report.WriteString("\n--- " + peer + " capture ---\n" + capture)
		}
		if logs, err := lab.Logs(ctx, peer, 60); err == nil {
			report.WriteString("\n--- " + peer + " log ---\n" + logs.Text)
		}
	}
	return errors.New(report.String())
}
