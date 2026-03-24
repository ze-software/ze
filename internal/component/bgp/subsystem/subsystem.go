// Design: docs/architecture/subsystem-wiring.md — BGP subsystem adapter
// Design: plan/spec-arch-0-system-boundaries.md — Subsystem interface

package subsystem

import (
	"context"

	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

// Reactor is the interface that BGPSubsystem needs from the reactor.
// Defined here to avoid importing the reactor package (which would create a cycle).
type Reactor interface {
	StartWithContext(ctx context.Context) error
	Stop()
	Wait(ctx context.Context) error
}

// BGPSubsystem wraps a Reactor and implements ze.Subsystem.
// It is the adapter that connects the BGP reactor to the Engine supervisor
// and the Bus notification layer.
//
// MUST call NewBGPSubsystem() to create. Start() MUST be called before PublishNotification().
// Stop() MUST be called to release resources. Caller MUST call Wait after Stop
// on the underlying reactor.
type BGPSubsystem struct {
	reactor Reactor
	bus     ze.Bus
}

// NewBGPSubsystem creates a BGPSubsystem wrapping the given reactor.
func NewBGPSubsystem(reactor Reactor) *BGPSubsystem {
	return &BGPSubsystem{reactor: reactor}
}

// Name returns the subsystem identifier.
func (s *BGPSubsystem) Name() string {
	return "bgp"
}

// Start creates Bus topics, stores the Bus reference, and starts the reactor.
func (s *BGPSubsystem) Start(ctx context.Context, b ze.Bus, _ ze.ConfigProvider) error {
	s.bus = b

	topics := []string{
		"bgp/update",
		"bgp/state",
		"bgp/negotiated",
		"bgp/eor",
		"bgp/congestion",
	}
	for _, name := range topics {
		if _, err := b.CreateTopic(name); err != nil {
			return err
		}
	}

	return s.reactor.StartWithContext(ctx)
}

// Stop gracefully shuts down the reactor.
func (s *BGPSubsystem) Stop(ctx context.Context) error {
	s.reactor.Stop()
	return s.reactor.Wait(ctx)
}

// Reload applies configuration changes. Currently delegates to the reactor's
// existing reload mechanism (SIGHUP handler). Full ConfigProvider integration
// is deferred to a future spec.
func (s *BGPSubsystem) Reload(_ context.Context, _ ze.ConfigProvider) error {
	return nil
}

// PublishNotification publishes a lightweight notification to the Bus.
// This is fire-and-forget — the Bus delivers asynchronously to subscribers.
// The existing EventDispatcher data path is unaffected.
// PublishNotification publishes a lightweight notification to the Bus.
// Payload is nil — notifications carry information in metadata only.
// This is fire-and-forget — the Bus delivers asynchronously to subscribers.
// The existing EventDispatcher data path is unaffected.
func (s *BGPSubsystem) PublishNotification(topic string, metadata map[string]string) {
	if s.bus != nil {
		s.bus.Publish(topic, nil, metadata)
	}
}
