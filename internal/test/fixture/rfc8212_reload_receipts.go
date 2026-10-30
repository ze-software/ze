// Design: docs/architecture/api/process-protocol.md -- an event callback application is not queue receipt.
// Related: rfc8212_reload_fault.go -- an external process retains real received route observations.
package fixture

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

// record is safe for concurrent calls. It persists only after applying the
// delivered event to the fixture's retained source state. It implements no
// source-policy lifecycle: production owner declaration and callbacks are absent.
func (state *rfc8212ReloadFault) record(event string) error {
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
			Update struct {
				NLRI map[string][]struct {
					Action string   `json:"action"`
					NLRI   []string `json:"nlri"`
				} `json:"nlri"`
			} `json:"update"`
		} `json:"bgp"`
	}
	if err := json.Unmarshal([]byte(event), &envelope); err != nil {
		return err
	}
	bgp := &envelope.BGP
	if bgp.Peer.Remote.Address != addrLoopback {
		return nil
	}
	if bgp.Message.Type != "update" {
		return nil
	}
	if bgp.Message.Direction != "received" {
		return nil
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	for _, operations := range bgp.Update.NLRI {
		for _, operation := range operations {
			for _, prefix := range operation.NLRI {
				if len(state.received) == 256 {
					return errors.New("source observation limit exceeded")
				}
				switch operation.Action {
				case "add":
					state.routes[prefix] = event
					state.received[prefix]++
				case "del":
					delete(state.routes, prefix)
				default:
					return fmt.Errorf("unexpected source event action %q", operation.Action)
				}
			}
		}
	}
	data, err := json.Marshal(state.received)
	if err != nil {
		return err
	}
	return os.WriteFile("owner.received", data, 0o600)
}

func rfc8212ReloadAppliedOnce(ctx context.Context, prefix string) error {
	var counts map[string]int
	var lastErr error
	if !Poll(ctx, 80, liveStagesPollDelay, func() bool {
		data, err := os.ReadFile("owner.received")
		if err != nil {
			lastErr = err
			return false
		}
		if err := json.Unmarshal(data, &counts); err != nil {
			lastErr = err
			return false
		}
		return counts[prefix] != 0
	}) {
		if lastErr == nil {
			return fmt.Errorf("external consumer never applied buffered %s: counts=%v", prefix, counts)
		}
		return fmt.Errorf("external consumer never applied buffered %s: counts=%v: %w", prefix, counts, lastErr)
	}
	if counts[prefix] != 1 {
		return fmt.Errorf("external consumer applied buffered %s %d times, want exactly once", prefix, counts[prefix])
	}
	return nil
}
