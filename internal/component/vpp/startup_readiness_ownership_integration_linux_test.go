//go:build integration && linux

// Design: docs/research/vpp-deployment-reference.md -- startup readiness with actual physical custody
// Related: startup_readiness_test.go -- portable setup-refusal receipt question
package vpp

import (
	"context"
	"errors"
	"testing"

	"go.fd.io/govpp/api"

	vppevents "github.com/ze-software/ze/internal/core/vpp/events"
)

type startupReadinessConnection struct {
	recordingConnection
	channel api.Channel
	peer    *processWitness
}

func (c *startupReadinessConnection) processPeer() *processWitness { return c.peer }

func (c *startupReadinessConnection) NewAPIChannel() (api.Channel, error) {
	return c.channel, nil
}

type startupReadinessChannel struct{ startupProbeChannel }

func (startupReadinessChannel) Close() {}

// Readiness must follow the actual API compatibility answer, with counters
// already published. Refusal and cancellation withdraw counters and emit no
// connected event; the selected compatible plugin retains both publications.
func TestVPPStartupRuntimeGatesReadinessAndStats(t *testing.T) {
	for _, outcome := range []string{"compatible", "incompatible", "canceled"} {
		t.Run(outcome, func(t *testing.T) {
			manager := runtimeStatsTestManager(t)
			defer func() {
				if lease, ok := GetActiveStatsProvider().(*runtimeStats); ok {
					if lease.owner == manager.connector {
						lease.Close()
					}
				}
			}()
			feature := startupRuntimeTestFeature(t,
				&startupProbeMessage{name: "probe_request", crc: "1234abcd"})
			manager.settings.startup = []startupFeature{feature}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			checks := 0
			channel := startupReadinessChannel{startupProbeChannel{check: func(...api.Message) error {
				checks++
				if GetActiveStatsProvider() == nil {
					t.Error("plugin readiness preceded stats publication")
				}
				switch outcome {
				case "compatible":
					return nil
				case "incompatible":
					return &api.CompatibilityError{IncompatibleMessages: []string{"probe_request_1234abcd"}}
				case "canceled":
					cancel()
					return nil
				}
				return errors.New("unrecognized test outcome")
			}}}
			manager.connector.conn = &startupReadinessConnection{channel: channel, peer: physicalFixturePeer(t, manager.connector)}
			events := 0
			setVPPEventBus(runtimeStatsTestBus{emit: func(string, string, any) {
				events++
				if checks != 1 {
					t.Error("readiness preceded plugin compatibility query")
				}
			}})
			client := &runtimeStatsTestClient{peer: newRuntimeStatsTestPeer(t)}
			activated, err := manager.activateRuntimeStats(ctx, func(string) (runtimeStatsClient, error) { return client, nil })
			if checks != 1 {
				t.Fatalf("plugin compatibility queries=%d", checks)
			}
			if outcome == "compatible" {
				if err != nil {
					t.Fatal(err)
				}
				if activated == nil {
					t.Fatal("compatible activation returned no statistics lease")
				}
				if events != 1 {
					t.Fatalf("compatible readiness events=%d", events)
				}
				lease, ok := GetActiveStatsProvider().(*runtimeStats)
				if !ok {
					t.Fatal("compatible activation lost statistics")
				}
				lease.Close()
				return
			}
			if activated != nil {
				t.Fatal("refused activation returned a statistics lease")
			}
			if outcome == "incompatible" {
				if _, ok := errors.AsType[*api.CompatibilityError](err); !ok {
					t.Fatalf("incompatible activation error=%v", err)
				}
			}
			if outcome == "canceled" {
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("canceled activation error=%v", err)
				}
			}
			if events != 0 {
				t.Fatalf("refused readiness events=%d", events)
			}
			if GetActiveStatsProvider() != nil {
				t.Fatal("refused activation retained statistics publication")
			}
			if client.disconnects != 1 {
				t.Fatalf("refused statistics disconnects=%d", client.disconnects)
			}
		})
	}
}

// API loss during the initial statistics read cannot be treated as an
// optional counters failure while publishing a dead API as connected.
func TestVPPStartupRuntimeRejectsAPILossDuringStats(t *testing.T) {
	manager := runtimeStatsTestManager(t)
	setVPPEventBus(runtimeStatsTestBus{emit: func(string, string, any) {
		t.Error("retired API generation published readiness")
	}})
	client := &runtimeStatsTestClient{peer: newRuntimeStatsTestPeer(t), read: func(*api.InterfaceStats) error {
		manager.connector.mu.Lock()
		manager.connector.generation++
		manager.connector.connected = false
		manager.connector.mu.Unlock()
		return nil
	}}
	lease, err := manager.activateRuntimeStats(t.Context(), func(string) (runtimeStatsClient, error) { return client, nil })
	if !errors.Is(err, errVppNotConnected) {
		t.Fatalf("retired API activation error=%v", err)
	}
	if lease != nil {
		t.Fatal("retired API activation returned statistics")
	}
	if client.disconnects != 1 {
		t.Fatalf("retired statistics disconnects=%d", client.disconnects)
	}
	if GetActiveStatsProvider() != nil {
		t.Fatal("retired API retained a statistics publication")
	}
}

// The startup receipt follows the published ready event exactly once. Each
// actual ready lifetime retires once, including intentional replacement; an
// initial candidate which never became ready emits no loss event.
func TestVPPStartupReceiptAndReadinessLifetimes(t *testing.T) {
	manager := runtimeStatsTestManager(t)
	receipt := make(chan error, 1)
	manager.startup = receipt
	var events []string
	setVPPEventBus(runtimeStatsTestBus{emit: func(_, event string, _ any) {
		events = append(events, event)
		if event == vppevents.EventConnected {
			select {
			case <-receipt:
				t.Error("startup receipt preceded the connected event")
			default:
			}
		}
	}})
	manager.retireReadiness()
	if len(events) != 0 {
		t.Fatal("initial candidate manufactured a disconnected event")
	}
	client := &runtimeStatsTestClient{peer: newRuntimeStatsTestPeer(t)}
	lease, err := manager.activateRuntimeStats(t.Context(), func(string) (runtimeStatsClient, error) { return client, nil })
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(lease.Close)
	select {
	case failure := <-receipt:
		if failure != nil {
			t.Fatal(failure)
		}
	default:
		t.Fatal("ready manager did not answer startup")
	}
	manager.notifyStartup(errors.New("later failure"))
	select {
	case <-receipt:
		t.Fatal("startup receipt was sent twice")
	default:
	}
	manager.retireReadiness()
	manager.retireReadiness()
	lease.Close()
	replacement, err := manager.activateRuntimeStats(t.Context(), func(string) (runtimeStatsClient, error) {
		return &runtimeStatsTestClient{peer: newRuntimeStatsTestPeer(t)}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(replacement.Close)
	manager.retireReadiness()
	want := []string{vppevents.EventConnected, vppevents.EventDisconnected,
		vppevents.EventReconnected, vppevents.EventDisconnected}
	if len(events) != len(want) {
		t.Fatalf("lifecycle events=%v", events)
	}
	for index, event := range want {
		if events[index] != event {
			t.Fatalf("lifecycle events=%v", events)
		}
	}
}
