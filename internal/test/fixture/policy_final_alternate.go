// Design: docs/functional-tests.md -- destination-specific alternate selection across final-only reloads.
// Related: policy_final_reload.go -- the same parsed configuration and three original sockets.
package fixture

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// policyFinalAlternatePrefix is the route whose permitted, longer AS_PATH is the
// destination's alternate behind a denied, preferred path.
const policyFinalAlternatePrefix = "10.72.3.0/24"

func policyFinalAlternate(ctx context.Context, plugin *sdk.Plugin, state *policyFinalWireState, accept, reject string) error {
	// The permitted, longer AS_PATH arrives first and owns the destination's
	// entry. The shorter preferred route arrives last but its ASN is denied.
	// Source peers have no RS send attachment: only the destination receives
	// forwards, and trigger routes reach the sources through the real API.
	if err := liveStagesRoute(ctx, plugin, "127.0.0.3 update text nhop 192.0.2.1 nlri add 192.168.3.0/24"); err != nil {
		return err
	}
	if err := state.wait(ctx, "permitted alternate installed", "10.72.1.0/24", "10.72.2.0/24", policyFinalAlternatePrefix); err != nil {
		return err
	}
	state.mu.Lock()
	state.protectAlternate = true
	state.mu.Unlock()
	if err := liveStagesRoute(ctx, plugin, "127.0.0.1 update text nhop 192.0.2.1 nlri add 192.168.3.0/24"); err != nil {
		return err
	}
	if err := state.wait(ctx, "preferred denied fence preserved alternate", "10.72.1.0/24", "10.72.2.0/24", policyFinalAlternatePrefix, "10.72.253.0/24"); err != nil {
		peers, queryErr := peerRows03(ctx, plugin, "*")
		return fmt.Errorf("%w; peer counters at failed fence=%v (query error=%w)", err, peers, queryErr)
	}
	if err := policyFinalPreferred(ctx, plugin); err != nil {
		return err
	}
	before, err := peerRows03(ctx, plugin, "*")
	if err != nil {
		return err
	}
	state.mu.Lock()
	state.protectAlternate = false
	state.mu.Unlock()
	if err := policyFinalReload(ctx, plugin, reject, "applied"); err != nil {
		return err
	}
	if err := state.wait(ctx, "final reject withdrew every candidate"); err != nil {
		return err
	}
	if err := policyFinalReload(ctx, plugin, accept, "applied"); err != nil {
		return err
	}
	if err := state.wait(ctx, "restored permitted alternate behind rejected preferred", "10.72.1.0/24", "10.72.2.0/24", policyFinalAlternatePrefix, "10.72.253.0/24"); err != nil {
		return err
	}
	after, err := peerRows03(ctx, plugin, "*")
	if err != nil {
		return err
	}
	for _, peer := range []string{addrLoopback, addrLoopbackThird} {
		if number03(before[peer]["updates-received"]) != number03(after[peer]["updates-received"]) {
			return fmt.Errorf("source %s sent an UPDATE during alternate reconciliation: before=%v after=%v", peer, before[peer], after[peer])
		}
	}
	return policyFinalPreferred(ctx, plugin)
}

func policyFinalPreferred(ctx context.Context, plugin *sdk.Plugin) error {
	var best struct {
		Paths []struct {
			Prefix string `json:"prefix"`
			Peer   string `json:"best-peer"`
		} `json:"best-path"`
	}
	var lastErr error
	if !Poll(ctx, 60, liveStagesPollDelay, func() bool {
		var status string
		best.Paths = nil
		status, lastErr = Dispatch(ctx, plugin, "show bgp rib best prefix "+policyFinalAlternatePrefix, &best)
		if lastErr != nil {
			return false
		}
		if status != statusDone {
			lastErr = fmt.Errorf("best-path query status=%s", status)
			return false
		}
		if len(best.Paths) != 1 {
			return false
		}
		return best.Paths[0].Prefix == policyFinalAlternatePrefix && best.Paths[0].Peer == addrLoopback
	}) {
		return fmt.Errorf("shorter denied path was not the preferred candidate: paths=%v error=%w", best.Paths, lastErr)
	}
	return nil
}

// policyFinalWireState reads only UPDATEs echoed by the destination over its
// original TCP connection. Sent events and the selecting RIB are not wire proof.
// A rejected retained candidate may emit a withdrawal for an already absent
// key; that packet does not prevent a later permitted candidate restoring it.
type policyFinalWireState struct {
	mu               sync.Mutex
	routes           map[string]bool
	failure          error
	protectAlternate bool
}

func (state *policyFinalWireState) receive(event string) error {
	var envelope struct {
		BGP struct {
			Message struct {
				Type      string `json:"type"`
				Direction string `json:"direction"`
			} `json:"message"`
			Peer struct {
				Remote struct {
					Address string `json:"address"`
				} `json:"remote"`
			} `json:"peer"`
			Raw struct {
				Attributes string `json:"attributes"`
			} `json:"raw"`
			Update struct {
				NLRI map[string][]struct {
					Action string   `json:"action"`
					NLRI   []string `json:"nlri"`
				} `json:"nlri"`
			} `json:"update"`
		} `json:"bgp"`
	}
	if err := json.Unmarshal([]byte(event), &envelope); err != nil {
		state.mu.Lock()
		state.failure = fmt.Errorf("decode echoed wire event: %w", err)
		state.mu.Unlock()
		return nil
	}
	bgp := &envelope.BGP
	if bgp.Message.Type != "update" || bgp.Message.Direction != "received" || bgp.Peer.Remote.Address != "127.0.0.2" {
		return nil
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.failure != nil {
		return nil
	}
	attrs, err := hex.DecodeString(bgp.Raw.Attributes)
	if err != nil {
		state.failure = fmt.Errorf("decode echoed attributes: %w", err)
		return nil
	}
	for fam, operations := range bgp.Update.NLRI {
		if fam != "ipv4/unicast" {
			state.failure = fmt.Errorf("unexpected echoed family %s", fam)
			return nil
		}
		for _, operation := range operations {
			for _, prefix := range operation.NLRI {
				switch prefix {
				case "10.72.1.0/24", "10.72.2.0/24", policyFinalAlternatePrefix, "10.72.253.0/24":
				default:
					state.failure = fmt.Errorf("unexpected echoed prefix %s", prefix)
					return nil
				}
				switch operation.Action {
				case "del":
					if state.protectAlternate && prefix == policyFinalAlternatePrefix {
						state.failure = fmt.Errorf("preferred rejection withdrew installed alternate %s", prefix)
						return nil
					}
					delete(state.routes, prefix)
				case "add":
					if err := policyFinalWireAttributes(prefix, attrs); err != nil {
						state.failure = err
						return nil
					}
					state.routes[prefix] = true
				default:
					state.failure = fmt.Errorf("unexpected echoed action %q", operation.Action)
					return nil
				}
			}
		}
	}
	return nil
}

// Every announcement, not just the final snapshot, must carry the original
// permitted path. This permanently fails on a transient leak of ASN 65010.
func policyFinalWireAttributes(prefix string, attrs []byte) error {
	want := map[attribute.AttributeCode]string{
		attribute.AttrOrigin:    "00",
		attribute.AttrASPath:    "02010000fde9",
		attribute.AttrNextHop:   "c0000201",
		attribute.AttrMED:       "00000009",
		attribute.AttrCommunity: "fde90063",
	}
	if prefix == policyFinalAlternatePrefix {
		want[attribute.AttrASPath] = "02030000fdeb0000fdeb0000fdeb"
		want[attribute.AttrNextHop] = "c0000203"
		want[attribute.AttrCommunity] = "fdeb0021"
	}
	iter := attribute.NewAttrIterator(attrs)
	for code, _, value, ok := iter.Next(); ok; code, _, value, ok = iter.Next() {
		expected, checked := want[code]
		if !checked {
			continue
		}
		if hex.EncodeToString(value) != expected {
			return fmt.Errorf("echoed %s attribute %d = %x, want %s", prefix, code, value, expected)
		}
		delete(want, code)
	}
	if iter.Remaining() != 0 || len(want) != 0 {
		return fmt.Errorf("echoed %s has malformed or missing original attributes: missing=%v remaining=%d", prefix, want, iter.Remaining())
	}
	return nil
}

func (state *policyFinalWireState) wait(ctx context.Context, phase string, prefixes ...string) error {
	matched := Poll(ctx, 100, liveStagesPollDelay, func() bool {
		state.mu.Lock()
		defer state.mu.Unlock()
		if state.failure != nil {
			return true
		}
		if len(state.routes) != len(prefixes) {
			return false
		}
		for _, prefix := range prefixes {
			if !state.routes[prefix] {
				return false
			}
		}
		return true
	})
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.failure != nil {
		return fmt.Errorf("%s: %w", phase, state.failure)
	}
	if !matched {
		return fmt.Errorf("%s: echoed wire routes=%v, want %v", phase, state.routes, prefixes)
	}
	return nil
}
