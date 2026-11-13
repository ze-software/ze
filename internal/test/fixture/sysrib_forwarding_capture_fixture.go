// Design: docs/architecture/rib/unified-locrib.md -- selected output and current replay.
// Related: sysrib_recursive_daemon_fixture.go -- real daemon/route-install caller.

package fixture

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	sysribevents "github.com/ze-software/ze/internal/component/sysrib/events"
	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/replay"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// sysribFIBCapture retains one owned current live/replay entry per watched
// prefix, never the full stream. Its SDK callback and scenario run concurrently.
type sysribFIBCapture struct {
	mu          sync.Mutex
	watched     map[string]bool
	live        map[string]sysribevents.BestChangeEntry
	replayed    map[string]sysribevents.BestChangeEntry
	withdrawals map[string]uint64
	replayedAt  map[string]uint64
	replayCount uint64
	err         error
}

func newSysribFIBCapture(prefixes ...string) *sysribFIBCapture {
	c := &sysribFIBCapture{watched: make(map[string]bool), live: make(map[string]sysribevents.BestChangeEntry), replayed: make(map[string]sysribevents.BestChangeEntry), replayedAt: make(map[string]uint64), withdrawals: make(map[string]uint64)}
	for _, prefix := range prefixes {
		c.watched[prefix] = true
	}
	return c
}

func (c *sysribFIBCapture) setup(p *sdk.Plugin) error {
	p.SetStartupSubscriptionsIn(sysribevents.Namespace, []string{sysribevents.EventBestChange}, nil, "json")
	p.SetEnvelope(true)
	p.OnEvent(func(event string) error {
		envelope, err := rpc.ParseEventEnvelope(event)
		var batch sysribevents.BestChangeBatch
		if err == nil {
			if envelope.Namespace != sysribevents.Namespace {
				err = fmt.Errorf("unexpected FIB namespace %s", envelope.Namespace)
			}
		}
		if err == nil {
			if envelope.Event != sysribevents.EventBestChange {
				err = fmt.Errorf("unexpected FIB event %s", envelope.Event)
			}
		}
		if err == nil {
			err = json.Unmarshal(envelope.Payload, &batch)
		}
		c.mu.Lock()
		defer c.mu.Unlock()
		if err != nil {
			c.err = err
			return nil
		}
		if batch.IsReplay() {
			c.replayCount++
		}
		for index := range batch.Changes {
			entry := batch.Changes[index]
			prefix := entry.Prefix.String()
			if !c.watched[prefix] {
				continue
			}
			if entry.Action == routeaction.Withdraw {
				c.withdrawals[prefix]++
			}
			if batch.IsReplay() {
				c.replayed[prefix] = entry
				c.replayedAt[prefix] = c.replayCount
			} else {
				c.live[prefix] = entry
			}
		}
		return nil
	})
	return nil
}

func (c *sysribFIBCapture) await(ctx context.Context, prefix string, replayAfter uint64, want sysribevents.BestChangeEntry) error {
	var last sysribevents.BestChangeEntry
	var lastErr error
	if Poll(ctx, 100, 100*time.Millisecond, func() bool {
		c.mu.Lock()
		defer c.mu.Unlock()
		lastErr = c.err
		if lastErr != nil {
			return false
		}
		value, present := c.live[prefix]
		if replayAfter != 0 {
			if c.replayedAt[prefix] < replayAfter {
				return false
			}
			value, present = c.replayed[prefix]
		}
		last = value
		if !present {
			return false
		}
		if want.Action == routeaction.Unspecified {
			if value.Action != routeaction.Add {
				if value.Action != routeaction.Update {
					return false
				}
			}
			value.Action = routeaction.Unspecified
		}
		return reflect.DeepEqual(value, want)
	}) {
		return nil
	}
	if lastErr != nil {
		return fmt.Errorf("FIB %s replay-after=%d: observed %+v, want %+v; delivery error: %w", prefix, replayAfter, last, want, lastErr)
	}
	return fmt.Errorf("FIB %s replay-after=%d: observed %+v, want %+v; delivery error: <nil>", prefix, replayAfter, last, want)
}

func (c *sysribFIBCapture) requestReplay(ctx context.Context, p *sdk.Plugin) (uint64, error) {
	c.mu.Lock()
	next := c.replayCount + 1
	c.mu.Unlock()
	data, err := json.Marshal(replay.Request{ReplayID: replay.Broadcast})
	if err != nil {
		return 0, err
	}
	count, err := p.EmitEvent(ctx, sysribevents.Namespace, sysribevents.EventReplayRequest, "", "", string(data))
	if err != nil {
		return 0, err
	}
	if count < 1 {
		return 0, fmt.Errorf("system replay request reached no consumer")
	}
	return next, nil
}

func sysribInstall(ctx context.Context, p *sdk.Plugin, entries ...rpc.RouteInstallEntry) error {
	count, err := p.RouteInstall(ctx, entries)
	if err != nil {
		return err
	}
	if int(count) != len(entries) {
		return fmt.Errorf("route-install accepted %d of %d routes", count, len(entries))
	}
	return nil
}

func sysribRemove(ctx context.Context, p *sdk.Plugin, entries ...rpc.RouteInstallEntry) error {
	routes := make([]rpc.RouteRemoveEntry, len(entries))
	for i := range entries {
		entry := &entries[i]
		routes[i] = rpc.RouteRemoveEntry{Protocol: entry.Protocol, AFI: entry.AFI, SAFI: entry.SAFI, Prefix: entry.Prefix, Instance: entry.Instance}
	}
	count, err := p.RouteRemove(ctx, routes)
	if err != nil {
		return err
	}
	if int(count) != len(routes) {
		return fmt.Errorf("route-remove accepted %d of %d routes", count, len(routes))
	}
	return nil
}

// sysribKernel6 uses the same exact-prefix/all-table inspection as importKernel,
// selecting IPv6 explicitly so the host's default route family cannot hide it.
func sysribKernel6(ctx context.Context, prefix string, want int, words ...string) error {
	var last string
	var lastErr error
	if Poll(ctx, 100, 100*time.Millisecond, func() bool {
		last, lastErr = netfilterCommandOutput(ctx, "ip", "-N", "-6", "route", "show", "table", "all", "exact", prefix)
		if lastErr != nil {
			return false
		}
		var entries []string
		for line := range strings.SplitSeq(last, "\n") {
			if strings.TrimSpace(line) != "" {
				entries = append(entries, strings.Join(strings.Fields(line), " ")+" ")
			}
		}
		if len(entries) != want {
			return false
		}
		for _, entry := range entries {
			for _, word := range words {
				if !strings.Contains(entry, word) {
					return false
				}
			}
		}
		return true
	}) {
		return nil
	}
	if lastErr != nil {
		return fmt.Errorf("IPv6 kernel %s: want %d entries containing %q; observed %q; read error %w", prefix, want, words, last, lastErr)
	}
	return fmt.Errorf("IPv6 kernel %s: want %d entries containing %q; observed %q; read error <nil>", prefix, want, words, last)
}
