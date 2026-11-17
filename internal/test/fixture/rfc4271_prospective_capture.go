// Design: docs/architecture/route-selection.md -- admission before publication
// Related: rfc4271_prospective_daemon_fixture.go -- installed seeds and replacements
package fixture

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// rfc4271ProspectiveCapture retains two fixed subject states and the first
// forbidden publication. A later withdrawal cannot erase a wrong intermediate
// election. Methods are safe for concurrent SDK delivery and scenario reads;
// construction is required and this type has no valid zero or nil value.
type rfc4271ProspectiveCapture struct {
	mu       sync.Mutex
	subjects map[string]*rfc4271ProspectivePublication
	err      error
}

// rfc4271ProspectivePublication is capture-owned state under the capture mutex.
// Its zero flags describe no observation; forbidden is set at construction.
type rfc4271ProspectivePublication struct {
	forbidden string
	seed      bool
	armed     bool
	withdrawn bool
}

func newRFC4271ProspectiveCapture() *rfc4271ProspectiveCapture {
	return &rfc4271ProspectiveCapture{subjects: map[string]*rfc4271ProspectivePublication{
		rfc4271ProspectiveDirect:   {forbidden: rfc4271ProspectiveDirectHop},
		rfc4271ProspectiveIndirect: {forbidden: rfc4271ProspectiveIndirectHop},
	}}
}

func (c *rfc4271ProspectiveCapture) setup(plugin *sdk.Plugin) error {
	plugin.SetStartupSubscriptionsIn(ribevents.Namespace, []string{ribevents.EventBestChange}, nil, "json")
	plugin.SetEnvelope(true)
	plugin.OnEvent(func(event string) error {
		envelope, err := rpc.ParseEventEnvelope(event)
		if err == nil {
			if envelope.Namespace != ribevents.Namespace {
				err = fmt.Errorf("unexpected prospective namespace %q", envelope.Namespace)
			}
		}
		if err == nil {
			if envelope.Event != ribevents.EventBestChange {
				err = fmt.Errorf("unexpected prospective event %q", envelope.Event)
			}
		}
		var batch ribevents.BestChangeBatch
		if err == nil {
			err = json.Unmarshal(envelope.Payload, &batch)
		}
		c.mu.Lock()
		defer c.mu.Unlock()
		if c.err != nil {
			return nil
		}
		if err != nil {
			c.err = err
			return nil
		}
		if batch.IsReplay() {
			return nil
		}
		for index := range batch.Changes {
			entry := &batch.Changes[index]
			state, watched := c.subjects[entry.Prefix.String()]
			if !watched {
				continue
			}
			if entry.Action == routeaction.Withdraw {
				if state.armed {
					state.withdrawn = true
				}
				continue
			}
			nextHop := entry.NextHop.String()
			if nextHop == rfc4271NextHopDirect {
				state.seed = true
			}
			if state.armed {
				if nextHop == state.forbidden {
					c.err = fmt.Errorf("BGP published forbidden prospective self-capture %s via %s before withdrawal", entry.Prefix, nextHop)
					return nil
				}
			}
		}
		return nil
	})
	return nil
}

func (c *rfc4271ProspectiveCapture) arm(ctx context.Context, prefix string) error {
	if Poll(ctx, importPollAttempts, importPollDelay, func() bool {
		c.mu.Lock()
		defer c.mu.Unlock()
		if c.err != nil {
			return false
		}
		state, found := c.subjects[prefix]
		if !found {
			return false
		}
		if !state.seed {
			return false
		}
		state.armed = true
		return true
	}) {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.err != nil {
		return fmt.Errorf("BGP seed publication for %s was not observed before replacement; delivery error: %w", prefix, c.err)
	}
	return fmt.Errorf("BGP seed publication for %s was not observed before replacement; delivery error: <nil>", prefix)
}

func (c *rfc4271ProspectiveCapture) withdrawn(ctx context.Context, prefix string) error {
	if Poll(ctx, importPollAttempts, importPollDelay, func() bool {
		c.mu.Lock()
		defer c.mu.Unlock()
		if c.err != nil {
			return false
		}
		state, found := c.subjects[prefix]
		if !found {
			return false
		}
		return state.withdrawn
	}) {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.err != nil {
		return fmt.Errorf("BGP withdrawal for %s did not follow its installed seed without a forbidden selection; delivery error: %w", prefix, c.err)
	}
	return fmt.Errorf("BGP withdrawal for %s did not follow its installed seed without a forbidden selection; delivery error: <nil>", prefix)
}
