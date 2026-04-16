//go:build linux

package ifacedhcp

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/iface"
	ifaceevents "codeberg.org/thomas-mangin/ze/internal/component/iface/events"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

// stubEventBus satisfies ze.EventBus for constructor validation tests.
// No methods are called in the error-path tests; only non-nil is needed.
type stubEventBus struct{}

func (stubEventBus) Emit(string, string, any) (int, error)                    { return 0, nil }
func (stubEventBus) Subscribe(string, string, func(any)) (unsubscribe func()) { return func() {} }

func TestDhcpTopicToEventType(t *testing.T) {
	t.Parallel()

	// VALIDATES: dhcpTopicToEventType maps every known DHCP bus topic to
	// the corresponding stream event type.
	// PREVENTS: stale or missing topic-to-event mappings after constant renames.

	tests := []struct {
		name      string
		topic     string
		wantEvent string
		wantOK    bool
	}{
		{
			name:      "acquired",
			topic:     iface.TopicDHCPLeaseAcquired,
			wantEvent: ifaceevents.EventDHCPAcquired,
			wantOK:    true,
		},
		{
			name:      "renewed",
			topic:     iface.TopicDHCPLeaseRenewed,
			wantEvent: ifaceevents.EventDHCPRenewed,
			wantOK:    true,
		},
		{
			name:      "expired",
			topic:     iface.TopicDHCPLeaseExpired,
			wantEvent: ifaceevents.EventDHCPExpired,
			wantOK:    true,
		},
		{
			name:      "unknown topic returns false",
			topic:     "dhcp.lease.unknown",
			wantEvent: "",
			wantOK:    false,
		},
		{
			name:      "empty topic returns false",
			topic:     "",
			wantEvent: "",
			wantOK:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := dhcpTopicToEventType(tt.topic)
			assert.Equal(t, tt.wantOK, ok, "ok mismatch for topic %q", tt.topic)
			assert.Equal(t, tt.wantEvent, got, "event type mismatch for topic %q", tt.topic)
		})
	}
}

func TestNewDHCPClientValidation(t *testing.T) {
	t.Parallel()

	// VALIDATES: NewDHCPClient rejects invalid arguments before allocating.
	// PREVENTS: nil-pointer panics from missing eventBus, nonsensical configs
	// with both v4/v6 disabled, and invalid Linux interface names.

	bus := stubEventBus{}

	tests := []struct {
		name      string
		ifaceName string
		unit      int
		eventBus  ze.EventBus
		v4        bool
		v6        bool
		wantErr   string
	}{
		{
			name:      "nil eventBus",
			ifaceName: "eth0",
			unit:      0,
			eventBus:  nil,
			v4:        true,
			v6:        false,
			wantErr:   "event bus is nil",
		},
		{
			name:      "both v4 and v6 false",
			ifaceName: "eth0",
			unit:      0,
			eventBus:  bus,
			v4:        false,
			v6:        false,
			wantErr:   "at least one of v4 or v6",
		},
		{
			name:      "empty interface name",
			ifaceName: "",
			unit:      0,
			eventBus:  bus,
			v4:        true,
			v6:        false,
			wantErr:   "length",
		},
		{
			name:      "interface name too long",
			ifaceName: strings.Repeat("x", 16),
			unit:      0,
			eventBus:  bus,
			v4:        true,
			v6:        false,
			wantErr:   "length",
		},
		{
			name:      "interface name with slash",
			ifaceName: "eth/0",
			unit:      0,
			eventBus:  bus,
			v4:        true,
			v6:        false,
			wantErr:   "forbidden character",
		},
		{
			name:      "interface name with path traversal",
			ifaceName: "eth..0",
			unit:      0,
			eventBus:  bus,
			v4:        true,
			v6:        false,
			wantErr:   "path traversal",
		},
		{
			name:      "valid v4 only",
			ifaceName: "eth0",
			unit:      0,
			eventBus:  bus,
			v4:        true,
			v6:        false,
			wantErr:   "",
		},
		{
			name:      "valid v6 only",
			ifaceName: "eth0",
			unit:      100,
			eventBus:  bus,
			v4:        false,
			v6:        true,
			wantErr:   "",
		},
		{
			name:      "valid dual stack",
			ifaceName: "lo",
			unit:      0,
			eventBus:  bus,
			v4:        true,
			v6:        true,
			wantErr:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			client, err := NewDHCPClient(tt.ifaceName, tt.unit, tt.eventBus, tt.v4, tt.v6, DHCPConfig{})
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				assert.Nil(t, client)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, client)
			}
		})
	}
}

func TestV4RequestModifiersHostname(t *testing.T) {
	t.Parallel()

	// VALIDATES: AC-3 - hostname option 12 added when configured.
	// PREVENTS: hostname silently dropped from DHCP packets.

	bus := stubEventBus{}
	client, err := NewDHCPClient("eth0", 0, bus, true, false, DHCPConfig{
		Hostname: "ze-router",
		ClientID: "ze:01",
	})
	require.NoError(t, err)

	mods := client.v4RequestModifiers()
	assert.Len(t, mods, 2, "should have hostname and client-id modifiers")
}

func TestV4RequestModifiersEmpty(t *testing.T) {
	t.Parallel()

	// VALIDATES: no modifiers when config has no hostname/client-id.
	// PREVENTS: nil/empty modifier accidentally injected.

	bus := stubEventBus{}
	client, err := NewDHCPClient("eth0", 0, bus, true, false, DHCPConfig{})
	require.NoError(t, err)

	mods := client.v4RequestModifiers()
	assert.Empty(t, mods, "should have no modifiers with empty config")
}

func TestSleepOrStopWithClosedChannel(t *testing.T) {
	t.Parallel()

	// VALIDATES: sleepOrStop returns false immediately when the stop
	// channel is already closed (client was stopped before sleep).
	// PREVENTS: goroutine hangs when Stop() races with retry backoff.

	bus := stubEventBus{}
	client, err := NewDHCPClient("eth0", 0, bus, true, false, DHCPConfig{})
	require.NoError(t, err)

	// Close the stop channel directly to simulate a stopped client.
	close(client.stop)

	// Should return false (stopped) almost immediately, not wait a full second.
	got := client.sleepOrStop(5 * 1e9) // 5 seconds -- would timeout the test if it blocks
	assert.False(t, got, "sleepOrStop should return false when stop channel is closed")
}
