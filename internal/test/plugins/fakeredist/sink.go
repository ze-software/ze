// Design: docs/guide/redistribution.md -- the consumer half of redistribution
// Related: fakeredist.go -- the producer half, and the command dispatch
// Related: internal/component/config/redistribute/consumer.go -- RedistConsumer

// A synthetic consumer a .ci scenario registers when it chooses. Registering it
// AFTER a producer emitted is the shape the late-join replay exists for.
//
// Nothing else in the test tree can produce that order. A real consumer
// registers in its own plugin's startup, and nothing orders the plugin tiers.

package fakeredist

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sync"

	configredist "github.com/ze-software/ze/internal/component/config/redistribute"
	"github.com/ze-software/ze/internal/core/family"
)

var (
	errUsageFakeredistConsume  = errors.New("usage: fakeredist consume <destination-name>")
	errUsageFakeredistConsumed = errors.New("usage: fakeredist consumed <destination-name>")
)

// sink is a RedistConsumer that keeps what it was given. Safe for concurrent
// use: the orchestrator dispatches on the emitting goroutine and a .ci scenario
// reads back on the command goroutine.
type sink struct {
	name string

	mu       sync.Mutex
	prefixes map[string]struct{}
}

// sinks holds every synthetic consumer this process registered, keyed by the
// name it registered under, so `fakeredist consumed <name>` can read one back.
// The consumer registry itself hands back a RedistConsumer and a caller cannot
// ask it for the routes.
var (
	sinkMu sync.Mutex
	sinks  = map[string]*sink{}
)

func (s *sink) Name() string { return s.name }

func (s *sink) InjectRoute(_ context.Context, _ family.Family, entry configredist.RouteEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prefixes[entry.Prefix] = struct{}{}
}

func (s *sink) WithdrawRoute(_ context.Context, _ family.Family, prefix string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.prefixes, prefix)
}

// held answers the sorted prefix list, so a scenario's assertion is on a stable
// order rather than on a map walk.
func (s *sink) held() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, 0, len(s.prefixes))
	for prefix := range s.prefixes {
		out = append(out, prefix)
	}
	slices.Sort(out)
	return out
}

// runConsume handles `fakeredist consume <destination-name>`. Re-registering an
// existing name replaces the consumer and CLEARS what it held. A scenario can
// therefore register one destination twice, and tell the second registration's
// replay from the first one's routes.
func runConsume(args []string) (json.RawMessage, error) {
	if len(args) != 1 || args[0] == "" {
		return nil, errUsageFakeredistConsume
	}
	name := args[0]
	if name == ProtocolName {
		return nil, fmt.Errorf("consumer name %q is this plugin's own source name; a protocol is never redistributed into itself", name)
	}

	s := &sink{name: name, prefixes: map[string]struct{}{}}
	sinkMu.Lock()
	sinks[name] = s
	sinkMu.Unlock()

	// ReregisterConsumer rather than RegisterConsumer. A scenario that registers
	// one name twice is asking about the second registration, so a conflict
	// error would report its own subject as a mistake.
	replaced := configredist.ReregisterConsumer(s)
	logger().Debug("fakeredist: synthetic consumer registered", "name", name, "replaced", replaced)

	held := s.held()
	return marshalSink(name, held)
}

// runConsumed handles `fakeredist consumed <destination-name>`.
func runConsumed(args []string) (json.RawMessage, error) {
	if len(args) != 1 || args[0] == "" {
		return nil, errUsageFakeredistConsumed
	}
	name := args[0]
	sinkMu.Lock()
	s := sinks[name]
	sinkMu.Unlock()
	if s == nil {
		return nil, fmt.Errorf("no synthetic consumer registered as %q", name)
	}
	return marshalSink(name, s.held())
}

// marshalSink renders one consumer's state as the answer both commands return.
func marshalSink(name string, held []string) (json.RawMessage, error) {
	payload, err := json.Marshal(map[string]any{
		"consumer": name,
		"prefixes": held,
		"count":    len(held),
	})
	if err != nil {
		return nil, fmt.Errorf("marshal consumer state: %w", err)
	}
	return json.RawMessage(payload), nil
}
