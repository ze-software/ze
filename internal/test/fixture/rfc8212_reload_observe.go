// Design: docs/functional-tests.md -- real wire and session observations for RFC 8212 drafts.
// Related: rfc8212_reload.go -- parsed-config reload scenarios, without product lifecycle substitutes.
package fixture

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// rfc8212ReloadWire is safe for concurrent event delivery and scenario reads.
// The constructor supplies its maps. It retains observations, not eligible routes.
type rfc8212ReloadWire struct {
	mu      sync.Mutex
	routes  map[string]string
	adds    map[string]int
	downs   int
	failure error
}

func newRFC8212ReloadWire() *rfc8212ReloadWire {
	return &rfc8212ReloadWire{routes: make(map[string]string), adds: make(map[string]int)}
}

func (s *rfc8212ReloadWire) receive(event string) error {
	var e struct {
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
			State string `json:"state"`
			Raw   struct {
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
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := json.Unmarshal([]byte(event), &e); err != nil {
		s.failure = err
		return nil
	}
	if e.BGP.State == "down" {
		s.downs++
	}
	if e.BGP.Peer.Remote.Address != "127.0.0.2" {
		return nil
	}
	if e.BGP.Message.Type != "update" {
		return nil
	}
	if e.BGP.Message.Direction != "received" {
		return nil
	}
	for family, operations := range e.BGP.Update.NLRI {
		if family != "ipv4/unicast" {
			s.failure = fmt.Errorf("unexpected echoed family %q", family)
			return nil
		}
		for _, operation := range operations {
			for _, prefix := range operation.NLRI {
				switch operation.Action {
				case "add":
					if len(s.adds) == 256 {
						s.failure = errors.New("RFC8212 wire observation limit exceeded")
						return nil
					}
					s.routes[prefix] = e.BGP.Raw.Attributes
					s.adds[prefix]++
				case "del":
					delete(s.routes, prefix)
				default:
					s.failure = fmt.Errorf("unexpected echoed action %q", operation.Action)
				}
			}
		}
	}
	return nil
}

func (s *rfc8212ReloadWire) wait(ctx context.Context, prefixes ...string) error {
	if Poll(ctx, 80, liveStagesPollDelay, func() bool {
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.failure != nil {
			return true
		}
		if len(s.routes) != len(prefixes) {
			return false
		}
		for _, prefix := range prefixes {
			if _, ok := s.routes[prefix]; !ok {
				return false
			}
		}
		return true
	}) {
		s.mu.Lock()
		defer s.mu.Unlock()
		return s.failure
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return fmt.Errorf("wire routes=%v, want exactly %v", s.routes, prefixes)
}

func (s *rfc8212ReloadWire) path(ctx context.Context, prefix string, alternate bool) error {
	var lastErr error
	if Poll(ctx, 80, liveStagesPollDelay, func() bool {
		s.mu.Lock()
		encoded, exists := s.routes[prefix]
		s.mu.Unlock()
		if !exists {
			lastErr = fmt.Errorf("wire prefix %s absent", prefix)
			return false
		}
		attrs, err := hex.DecodeString(encoded)
		if err != nil {
			lastErr = err
			return false
		}
		attributeCase := "10.72.1.0/24"
		if alternate {
			attributeCase = policyFinalAlternatePrefix
		}
		lastErr = policyFinalWireAttributes(attributeCase, attrs)
		return lastErr == nil
	}) {
		return nil
	}
	return fmt.Errorf("wire path %s alternate=%t: %w", prefix, alternate, lastErr)
}

func rfc8212ReloadSessions(ctx context.Context, p *sdk.Plugin, before map[string]map[string]any, refreshDelta int64) error {
	if err := policyFinalSessions(ctx, p, 3); err != nil {
		return err
	}
	after, err := peerRows03(ctx, p, "*")
	if err != nil {
		return err
	}
	for address, prior := range before {
		current := after[address]
		for _, field := range []string{"local-ip", "local-port", "remote-port", "connections-established", "connections-dropped"} {
			if _, ok := prior[field]; !ok {
				return fmt.Errorf("%s omitted session identity %s", address, field)
			}
			if !reflect.DeepEqual(prior[field], current[field]) {
				return fmt.Errorf("%s changed %s: %v -> %v", address, field, prior[field], current[field])
			}
		}
	}
	oldRefresh, err := rfc8212ReloadCounter(before, "127.0.0.1", "sent", "route-refresh")
	if err != nil {
		return err
	}
	newRefresh, err := rfc8212ReloadCounter(after, "127.0.0.1", "sent", "route-refresh")
	if err != nil {
		return err
	}
	if newRefresh-oldRefresh != refreshDelta {
		return fmt.Errorf("source refresh requests %d -> %d, want delta %d", oldRefresh, newRefresh, refreshDelta)
	}
	return nil
}

func rfc8212ReloadInputDelta(ctx context.Context, p *sdk.Plugin, before map[string]map[string]any, delta int64) error {
	prior, err := rfc8212ReloadCounter(before, "127.0.0.1", "received", "updates")
	if err != nil {
		return err
	}
	return rfc8212ReloadWaitInput(ctx, p, prior, delta)
}

// rfc8212ReloadCounter reads the JSON counter produced by handleBgpPeerDetail.
// Zero is a valid count, never a substitute for a missing peer, field or number.
func rfc8212ReloadCounter(rows map[string]map[string]any, peer, direction, kind string) (int64, error) {
	row, ok := rows[peer]
	if !ok {
		return 0, fmt.Errorf("counter peer %s absent from detail", peer)
	}
	messages, ok := row["messages"].(map[string]any)
	if !ok {
		return 0, fmt.Errorf("%s omitted messages object: %v", peer, row)
	}
	counters, ok := messages[direction].(map[string]any)
	if !ok {
		return 0, fmt.Errorf("%s omitted messages.%s object: %v", peer, direction, messages)
	}
	raw, ok := counters[kind]
	if !ok {
		return 0, fmt.Errorf("%s omitted messages.%s.%s counter", peer, direction, kind)
	}
	// peerRows03 decodes JSON into interface values, so numbers are float64.
	// The producer's counter is uint32: reject fractional and out-of-range data.
	number, ok := raw.(float64)
	if !ok {
		return 0, fmt.Errorf("%s messages.%s.%s is %T (%v), want uint32", peer, direction, kind, raw, raw)
	}
	if number < 0 {
		return 0, fmt.Errorf("%s messages.%s.%s is negative: %v", peer, direction, kind, number)
	}
	if number > float64(^uint32(0)) {
		return 0, fmt.Errorf("%s messages.%s.%s exceeds uint32: %v", peer, direction, kind, number)
	}
	count := uint32(number)
	if number != float64(count) {
		return 0, fmt.Errorf("%s messages.%s.%s is not an integer: %v", peer, direction, kind, number)
	}
	return int64(count), nil
}

func rfc8212ReloadInput(ctx context.Context, p *sdk.Plugin) (int64, error) {
	rows, err := peerRows03(ctx, p, "127.0.0.1")
	if err != nil {
		return 0, err
	}
	return rfc8212ReloadCounter(rows, "127.0.0.1", "received", "updates")
}

func rfc8212ReloadWaitInput(ctx context.Context, p *sdk.Plugin, prior, delta int64) error {
	want := prior + delta
	var after int64
	var queryErr error
	reached := Poll(ctx, 80, 250*time.Millisecond, func() bool {
		after, queryErr = rfc8212ReloadInput(ctx, p)
		if queryErr != nil {
			return true
		}
		return after >= want
	})
	if queryErr != nil {
		return fmt.Errorf("source UPDATE count after %d: %w", prior, queryErr)
	}
	if !reached {
		return fmt.Errorf("source UPDATE count %d -> %d, never reached exact delta %d", prior, after, delta)
	}
	if after != want {
		return fmt.Errorf("source UPDATE count %d -> %d, want exact delta %d", prior, after, delta)
	}
	return nil
}

func rfc8212ReloadBest(ctx context.Context, p *sdk.Plugin, prefix, owner string) error {
	var last any
	var queryErr error
	if Poll(ctx, 60, liveStagesPollDelay, func() bool {
		var answer struct {
			Paths []struct {
				Prefix string `json:"prefix"`
				Peer   string `json:"best-peer"`
			} `json:"best-path"`
		}
		var status string
		status, queryErr = Dispatch(ctx, p, "show bgp rib best prefix "+prefix, &answer)
		last = answer
		if queryErr != nil {
			return false
		}
		if status != statusDone {
			return false
		}
		if owner == "" {
			return len(answer.Paths) == 0
		}
		if len(answer.Paths) != 1 {
			return false
		}
		return answer.Paths[0].Prefix == prefix && answer.Paths[0].Peer == owner
	}) {
		return nil
	}
	return fmt.Errorf("eligible %s owner want %q: last=%v error=%v", prefix, owner, last, queryErr)
}
