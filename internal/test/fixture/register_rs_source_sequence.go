// Design: docs/architecture/update-building.md -- a route server elects the
// route a client without ADD-PATH holds.
// Related: register_vpn_withdrawal.go -- the fixed-address gate this one generalizes.
package fixture

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"strconv"
	"strings"
	"time"

	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

func init() {
	Register("plugin/rs-source-sequence", rsSourceSequence)
}

// The step keywords of an rs-source-sequence argument list.
const (
	rsStepUp   = "up"
	rsStepSend = "send"
	rsStepSent = "sent"
	rsStepHeld = "held"
	rsStepGone = "gone"
)

// rsSourceSequenceStep is one step of the sequence: a keyword and its operand.
type rsSourceSequenceStep struct {
	keyword string
	operand string
	address netip.Addr
	prefix  netip.Prefix
	count   int
}

// rsSourceSequence drives several ze-peer sources in an order the test names,
// each action released only once the state the next one depends on exists.
// The first argument is the client's address; the rest are steps, run in
// order:
//
//	up <address>             wait until bgp-rs reports the peer up
//	send <address> <prefix>  send the peer one route; its script acts on it
//	sent <n>                 wait until the client was sent n UPDATEs (End-of-RIB excluded)
//	held <address>           wait until bgp-rib holds a route received from the peer
//	gone <address>           wait until bgp-rib holds no route received from the peer
//
// A source's script expects the route of each `send` before its next action,
// so a source acts only when the fixture releases it, and the client's script
// asserts the exact UPDATEs that sequence produces. The fixture returns once
// the last step holds, which requests the daemon's shutdown.
func rsSourceSequence(ctx context.Context, args []string) error {
	if len(args) < 2 {
		return errors.New("rs-source-sequence requires the client address, then steps: up <address>, send <address> <prefix>, sent <n>, held <address>, gone <address>")
	}
	client, err := netip.ParseAddr(args[0])
	if err != nil {
		return fmt.Errorf("rs-source-sequence client %q is not an IP address: %w", args[0], err)
	}
	steps, err := rsSourceSequenceParse(args[1:])
	if err != nil {
		return err
	}
	return runPlugin13(ctx, "rs-source-sequence", nil, func(ctx context.Context, plugin *sdk.Plugin) error {
		for index, step := range steps {
			if err := rsSourceSequenceRun(ctx, plugin, client, step); err != nil {
				return fmt.Errorf("rs-source-sequence step %d (%s %s): %w", index+1, step.keyword, step.operand, err)
			}
		}
		return nil
	}, true)
}

// rsSourceSequenceParse reads every step before the daemon is driven, so a
// malformed sequence fails before any source acts.
func rsSourceSequenceParse(args []string) ([]rsSourceSequenceStep, error) {
	var steps []rsSourceSequenceStep
	for index := 0; index < len(args); {
		if index+1 >= len(args) {
			return nil, fmt.Errorf("rs-source-sequence step %q has no operand", args[index])
		}
		step := rsSourceSequenceStep{keyword: args[index], operand: args[index+1]}
		index += 2
		switch step.keyword {
		case rsStepSent:
			count, err := strconv.Atoi(step.operand)
			if err != nil {
				return nil, fmt.Errorf("rs-source-sequence sent %q is not a number: %w", step.operand, err)
			}
			step.count = count
		case rsStepUp, rsStepHeld, rsStepGone, rsStepSend:
			address, err := netip.ParseAddr(step.operand)
			if err != nil {
				return nil, fmt.Errorf("rs-source-sequence %s %q is not an IP address: %w", step.keyword, step.operand, err)
			}
			step.address = address
			if step.keyword == rsStepSend {
				if index >= len(args) {
					return nil, fmt.Errorf("rs-source-sequence send %s has no prefix", step.operand)
				}
				prefix, err := netip.ParsePrefix(args[index])
				if err != nil {
					return nil, fmt.Errorf("rs-source-sequence send %s prefix %q is not a prefix: %w", step.operand, args[index], err)
				}
				step.prefix = prefix
				index++
			}
		default:
			return nil, fmt.Errorf("rs-source-sequence step %q is unknown: use up, send, sent, held or gone", step.keyword)
		}
		steps = append(steps, step)
	}
	return steps, nil
}

// rsSourceSequenceRun performs one step and returns an error naming the state
// that never arrived.
func rsSourceSequenceRun(ctx context.Context, plugin *sdk.Plugin, client netip.Addr, step rsSourceSequenceStep) error {
	switch step.keyword {
	case rsStepUp:
		if !rsSourceSequenceWaitUp(ctx, plugin, step.address.String()) {
			return errors.New("bgp-rs never reported the peer up")
		}
		return nil
	case rsStepSend:
		command := "update text origin igp nhop 1.1.1.1 nlri add " + step.prefix.String()
		if _, _, err := plugin.UpdateRoute(ctx, step.address.String(), command); err != nil {
			return fmt.Errorf("the route to the peer was not sent: %w", err)
		}
		return nil
	case rsStepSent:
		if !rsSourceSequenceWaitSent(ctx, plugin, client.String(), step.count) {
			return fmt.Errorf("the client %s was not sent %d UPDATEs", client, step.count)
		}
		return nil
	case rsStepHeld:
		if !rsSourceSequenceWaitReceived(ctx, plugin, step.address.String(), true) {
			return errors.New("bgp-rib never held a route received from the peer")
		}
		return nil
	case rsStepGone:
		if !rsSourceSequenceWaitReceived(ctx, plugin, step.address.String(), false) {
			return errors.New("bgp-rib still holds a route received from the peer")
		}
		return nil
	default:
		return fmt.Errorf("step %q is unknown", step.keyword)
	}
}

// rsSourceSequenceWaitUp polls bgp-rs's peer list until the address is up.
func rsSourceSequenceWaitUp(ctx context.Context, plugin *sdk.Plugin, address string) bool {
	return Poll(ctx, 100, 250*time.Millisecond, func() bool {
		result := command13(ctx, plugin, commandShowRSPeers)
		if !done13(result) {
			return false
		}
		rows, ok := result.object()["peers"].([]any)
		if !ok {
			return false
		}
		for _, value := range rows {
			row, ok := value.(map[string]any)
			if !ok {
				continue
			}
			if row["address"] == address {
				up, _ := row["up"].(bool)
				return up
			}
		}
		return false
	})
}

// rsSourceSequenceWaitSent polls the client's counters until it was sent count
// UPDATEs other than End-of-RIB.
func rsSourceSequenceWaitSent(ctx context.Context, plugin *sdk.Plugin, client string, count int) bool {
	return Poll(ctx, 100, 250*time.Millisecond, func() bool {
		result := command13(ctx, plugin, "show bgp peer "+client+" detail")
		return peerCounter13(result, "updates-sent")-peerCounter13(result, "eor-sent") >= count
	})
}

// rsSourceSequenceWaitReceived polls bgp-rib's Adj-RIB-In until a route row
// received from address exists (want true) or none does (want false). Every
// row names its peer, so the quoted address appears only while such a row
// exists.
func rsSourceSequenceWaitReceived(ctx context.Context, plugin *sdk.Plugin, address string, want bool) bool {
	quoted := strconv.Quote(address)
	return Poll(ctx, 100, 250*time.Millisecond, func() bool {
		result := command13(ctx, plugin, "show bgp rib received")
		if !done13(result) {
			return false
		}
		return strings.Contains(result.text(), quoted) == want
	})
}
