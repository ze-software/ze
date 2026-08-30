package fixture

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func summaryFlat04(ctx context.Context, p *sdk.Plugin) error {
	_, value, err := pollCommand04(ctx, p, 40, "show bgp", func(status string, value any) bool {
		data, _ := value.(map[string]any)
		peer := findPeer04(data)
		return status == statusDone && peer != nil && peer["state"] == stateEstablished
	})
	if err != nil {
		return err
	}
	data, _ := value.(map[string]any)
	peer := findPeer04(data)
	if peer == nil || peer["state"] != stateEstablished {
		return fmt.Errorf("the peer never reached established: %v", data)
	}
	if _, ok := data["summary"]; ok {
		return fmt.Errorf("the summary envelope is back")
	}
	for _, key := range []string{fieldRouterID, fieldLocalAS, columnUptime, fieldPeersConfigured, fieldPeersEstablished} {
		if _, ok := data[key]; !ok {
			return fmt.Errorf("%s is not a top-level key", key)
		}
	}
	if _, ok := data["peers"].([]any); !ok {
		return fmt.Errorf("peers is %T, want a list beside the aggregates", data["peers"])
	}
	for _, key := range []string{fieldFamily, "peers-in-family"} {
		if _, ok := data[key]; ok {
			return fmt.Errorf("%s appeared without a family filter", key)
		}
	}
	filtered, err := requireDone04(ctx, p, "show bgp ipv4/unicast")
	if err != nil {
		return err
	}
	if _, ok := filtered["summary"]; ok {
		return fmt.Errorf("the summary envelope is back under a filter")
	}
	if filtered["family"] != familyIPv4Unicast || number04(filtered["peers-in-family"]) != 1 {
		return fmt.Errorf("filtered family/count = %v/%v, want ipv4/unicast/1", filtered["family"], filtered["peers-in-family"])
	}
	for _, key := range []string{fieldRouterID, fieldLocalAS, columnUptime, fieldPeersConfigured, fieldPeersEstablished} {
		if _, ok := filtered[key]; !ok {
			return fmt.Errorf("%s left the filtered record", key)
		}
	}
	fmt.Fprintf(os.Stderr, "OK: flat summary payload verified\n")
	return nil
}

func summaryCounts04(ctx context.Context, p *sdk.Plugin) error {
	_, value, err := pollCommand04(ctx, p, 100, "show bgp", func(status string, value any) bool {
		data, _ := value.(map[string]any)
		peer := findPeer04(data)
		return status == statusDone && peer != nil && number04(peer["routes-received"]) >= 1 && number04(peer["eor-sent"]) >= 1
	})
	if err != nil {
		return err
	}
	data, _ := value.(map[string]any)
	peer := findPeer04(data)
	if peer == nil {
		return errors.New("127.0.0.1 not in summary peers")
	}
	recv, recvOK := peer["routes-received"]
	accepted, acceptedOK := peer["routes-accepted"]
	_, sentOK := peer["routes-sent"]
	if !recvOK || !acceptedOK || !sentOK {
		return fmt.Errorf("summary peer row missing route-count keys: %v", peer)
	}
	if number04(recv) < 1 || number04(accepted) != number04(recv) {
		return fmt.Errorf("invalid received/accepted counts: %v/%v", recv, accepted)
	}
	if _, ok := peer["routes-filtered"]; ok {
		return fmt.Errorf("routes-filtered present; Ze does not track filtered routes")
	}
	if _, err := requireDone04(ctx, p, "request bgp rib inject 127.0.0.1 ipv6/unicast 2001:db8::/32 origin igp nexthop 2001:db8::1"); err != nil {
		return err
	}
	_, _, err = pollCommand04(ctx, p, 100, "show bgp", func(status string, value any) bool {
		data, _ := value.(map[string]any)
		return status == statusDone && number04(findPeer04(data)["routes-received"]) >= 2
	})
	if err != nil {
		return err
	}
	both, err := requireDone04(ctx, p, "show bgp")
	if err != nil {
		return err
	}
	v4, err := requireDone04(ctx, p, "show bgp ipv4/unicast")
	if err != nil {
		return err
	}
	if number04(findPeer04(both)["routes-received"]) != 2 {
		return fmt.Errorf("all-family routes-received is not 2: %v", both)
	}
	if number04(findPeer04(v4)["routes-received"]) != 1 {
		return fmt.Errorf("ipv4/unicast routes-received is not 1: %v", v4)
	}
	fmt.Fprintln(os.Stderr, "OK: summary route counts merged and family scoped")
	return nil
}

func rsObserver04(expected int, prefix string, requireReplay bool) Driver {
	return func(ctx context.Context, args []string) error {
		if len(args) != 0 {
			return fmt.Errorf("unexpected arguments: %v", args)
		}
		p, err := newObserver("fixture-rs-observer-04")
		if err != nil {
			return err
		}
		defer p.Close() //nolint:errcheck // fixture teardown
		events := make(chan string, expected+16)
		p.OnEvent(func(event string) error {
			var decoded map[string]any
			if json.Unmarshal([]byte(event), &decoded) != nil {
				return nil //nolint:nilerr // a malformed event is skipped, and failing the handler would end the session
			}
			bgp, _ := decoded["bgp"].(map[string]any)
			message, _ := bgp["message"].(map[string]any)
			if message["direction"] != directionSent {
				return nil
			}
			if !requireReplay && prefix != "" && !strings.Contains(event, prefix) {
				return nil
			}
			select {
			case events <- event:
			default:
			}
			return nil
		})
		p.SetStartupSubscriptions([]string{eventUpdate}, nil, "parsed")
		result := make(chan error, 1)
		p.OnAllPluginsReady(func() error {
			go func() {
				idleTimeout := testBudgetDuration04(30*time.Second, 0.60)
				idleDeadline := time.Now().Add(idleTimeout)
				hardDeadline := time.Now().Add(idleTimeout * 8)
				needEOR := expected
				eorPeers := make(map[string]struct{}, needEOR)
				forwardSeen := prefix == ""
				if !requireReplay {
					forwardSeen = false
					needEOR = 0
				}
				for (len(eorPeers) < needEOR || !forwardSeen) && ctx.Err() == nil {
					deadline := idleDeadline
					if hardDeadline.Before(deadline) {
						deadline = hardDeadline
					}
					remaining := time.Until(deadline)
					if remaining <= 0 {
						break
					}
					timer := time.NewTimer(remaining)
					select {
					case event := <-events:
						if !timer.Stop() {
							select {
							case <-timer.C:
							default:
							}
						}
						if !requireReplay {
							forwardSeen = true
							idleDeadline = time.Now().Add(idleTimeout)
							continue
						}
						var decoded map[string]any
						if json.Unmarshal([]byte(event), &decoded) != nil {
							continue
						}
						bgp, _ := decoded["bgp"].(map[string]any)
						update, _ := bgp["update"].(map[string]any)
						nlri, present := update["nlri"]
						emptyNLRI := !present || nlri == nil
						switch value := nlri.(type) {
						case []any:
							emptyNLRI = len(value) == 0
						case string:
							emptyNLRI = value == ""
						case map[string]any:
							emptyNLRI = len(value) == 0
						}
						if emptyNLRI {
							peer, _ := bgp["peer"].(map[string]any)
							remote, _ := peer["remote"].(map[string]any)
							address, _ := remote["address"].(string)
							if address == "" {
								address, _ = peer["address"].(string)
							}
							if address != "" {
								before := len(eorPeers)
								eorPeers[address] = struct{}{}
								if len(eorPeers) != before {
									idleDeadline = time.Now().Add(idleTimeout)
								}
							}
						} else if !forwardSeen {
							encoded, _ := json.Marshal(update)
							if strings.Contains(string(encoded), prefix) {
								forwardSeen = true
								idleDeadline = time.Now().Add(idleTimeout)
							}
						}
					case <-timer.C:
					case <-ctx.Done():
						if !timer.Stop() {
							select {
							case <-timer.C:
							default:
							}
						}
					}
				}
				var scenarioErr error
				if requireReplay && (len(eorPeers) < needEOR || !forwardSeen) {
					scenarioErr = fmt.Errorf("route server did not replay (EOR peers=%d/%d, forward=%t)", len(eorPeers), needEOR, forwardSeen)
				}
				shutdownCtx, cancel := context.WithTimeout(context.Background(), testBudgetDuration04(15*time.Second, 0.25))
				defer cancel()
				_, _, _ = p.DispatchCommand(shutdownCtx, "request shutdown")
				result <- scenarioErr
			}()
			return nil
		})
		runErr := p.Run(ctx, sdk.Registration{})
		select {
		case scenarioErr := <-result:
			return errors.Join(scenarioErr, runErr)
		default:
			return runErr
		}
	}
}
