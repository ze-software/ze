//go:build linux

package ifacedhcp

import (
	"net"
	"strings"
	"testing"
	"time"

	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/core/clock"
	ifaceevents "github.com/ze-software/ze/internal/core/iface/events"
	"github.com/ze-software/ze/internal/test/sim"
	"github.com/ze-software/ze/pkg/ze"
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

	// VALIDATES: newDHCPClient rejects invalid arguments before allocating.
	// PREVENTS: nil-pointer panics from missing eventBus, nonsensical configs
	// with both v4/v6 disabled, and invalid Linux interface names.

	bus := stubEventBus{}

	tests := []struct {
		name      string
		ifaceName string
		unit      string
		eventBus  ze.EventBus
		v4        bool
		v6        bool
		wantErr   string
	}{
		{
			name:      "nil eventBus",
			ifaceName: "eth0",
			unit:      "default",
			eventBus:  nil,
			v4:        true,
			v6:        false,
			wantErr:   "event bus is nil",
		},
		{
			name:      "both v4 and v6 false",
			ifaceName: "eth0",
			unit:      "default",
			eventBus:  bus,
			v4:        false,
			v6:        false,
			wantErr:   "at least one of v4 or v6",
		},
		{
			name:      "empty interface name",
			ifaceName: "",
			unit:      "default",
			eventBus:  bus,
			v4:        true,
			v6:        false,
			wantErr:   "length",
		},
		{
			name:      "interface name too long",
			ifaceName: strings.Repeat("x", 16),
			unit:      "default",
			eventBus:  bus,
			v4:        true,
			v6:        false,
			wantErr:   "length",
		},
		{
			name:      "interface name with slash",
			ifaceName: "eth/0",
			unit:      "default",
			eventBus:  bus,
			v4:        true,
			v6:        false,
			wantErr:   "forbidden character",
		},
		{
			name:      "interface name with path traversal",
			ifaceName: "eth..0",
			unit:      "default",
			eventBus:  bus,
			v4:        true,
			v6:        false,
			wantErr:   "path traversal",
		},
		{
			name:      "valid v4 only",
			ifaceName: "eth0",
			unit:      "default",
			eventBus:  bus,
			v4:        true,
			v6:        false,
			wantErr:   "",
		},
		{
			name:      "valid v6 only",
			ifaceName: "eth0",
			unit:      "uplink",
			eventBus:  bus,
			v4:        false,
			v6:        true,
			wantErr:   "",
		},
		{
			name:      "valid dual stack",
			ifaceName: "lo",
			unit:      "default",
			eventBus:  bus,
			v4:        true,
			v6:        true,
			wantErr:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			client, err := newDHCPClient(tt.ifaceName, tt.unit, tt.eventBus, tt.v4, tt.v6, dHCPConfig{}, clock.RealClock{})
			if tt.wantErr == "" {
				require.NoError(t, err)
				assert.NotNil(t, client)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				assert.Nil(t, client)
			}
		})
	}
}

func TestV4RequestModifiersHostname(t *testing.T) {
	t.Parallel()

	// VALIDATES: AC-3 - hostname option 12 added when configured.
	// PREVENTS: hostname silently dropped from DHCP packets.

	bus := stubEventBus{}
	client, err := newDHCPClient("eth0", "default", bus, true, false, dHCPConfig{
		Hostname: "ze-router",
		ClientID: "ze:01",
	}, clock.RealClock{})
	require.NoError(t, err)

	msg, err := dhcpv4.New(client.v4RequestModifiers()...)
	require.NoError(t, err)
	assert.Equal(t, "ze-router", msg.HostName(), "hostname option 12")
	assert.Equal(t, []byte("ze:01"), msg.Options.Get(dhcpv4.OptionClientIdentifier), "client-id option 61")
}

func TestV4RequestModifiersEmpty(t *testing.T) {
	t.Parallel()

	// VALIDATES: no hostname or client-id option when config has neither, and
	// the reserved flags bits are cleared while BROADCAST is kept.
	// PREVENTS: an empty option injected, and a reserved flags bit copied from
	// a server's OFFER or ACK into the message the client sends.

	bus := stubEventBus{}
	client, err := newDHCPClient("eth0", "default", bus, true, false, dHCPConfig{}, clock.RealClock{})
	require.NoError(t, err)

	msg, err := dhcpv4.New(dhcpv4.PrependModifiers(client.v4RequestModifiers(), func(d *dhcpv4.DHCPv4) { d.Flags = 0xffff })...)
	require.NoError(t, err)
	assert.False(t, msg.Options.Has(dhcpv4.OptionHostName), "no hostname option")
	assert.False(t, msg.Options.Has(dhcpv4.OptionClientIdentifier), "no client-id option")
	assert.Equal(t, uint16(0x8000), msg.Flags, "reserved flags bits cleared, BROADCAST kept")
}

func TestSleepOrStopWithClosedChannel(t *testing.T) {
	t.Parallel()

	// VALIDATES: sleepOrStop returns false immediately when the stop
	// channel is already closed (client was stopped before sleep).
	// PREVENTS: goroutine hangs when Stop() races with retry backoff.

	bus := stubEventBus{}
	client, err := newDHCPClient("eth0", "default", bus, true, false, dHCPConfig{}, clock.RealClock{})
	require.NoError(t, err)

	// Close the stop channel directly to simulate a stopped client.
	close(client.stop)

	// Should return false (stopped) almost immediately, not wait a full second.
	got := client.sleepOrStop(5 * 1e9) // 5 seconds -- would timeout the test if it blocks
	assert.False(t, got, "sleepOrStop should return false when stop channel is closed")
}

// TestSleepOrStopOnInjectedClock drives sleepOrStop from a fake clock.
//
// VALIDATES: A-7 -- the wait is an AfterFunc on the client's clock, so
// advancing the fake clock to the deadline wakes it (true), and Stop during a
// wait answers false.
// PREVENTS: a lease wait on the real clock, which no test can step.
func TestSleepOrStopOnInjectedClock(t *testing.T) {
	t.Parallel()
	clk := sim.NewFakeClock(time.Date(2026, 10, 18, 0, 0, 0, 0, time.UTC))
	client, err := newDHCPClient("eth0", "default", stubEventBus{}, true, false, dHCPConfig{}, clk)
	require.NoError(t, err)

	woke := make(chan bool, 1)
	go sleepInto(client, time.Minute, woke)
	require.True(t, clk.AwaitTimers(1, 5*time.Second), "sleepOrStop armed no timer on the clock")
	clk.Add(59 * time.Second)
	select {
	case <-woke:
		t.Fatal("sleepOrStop woke before its deadline")
	default: // still waiting, as it should
	}
	clk.Add(time.Second)
	assert.True(t, <-woke, "sleepOrStop did not wake at its deadline")

	go sleepInto(client, time.Minute, woke)
	require.True(t, clk.AwaitTimers(1, 5*time.Second), "sleepOrStop armed no timer on the clock")
	client.Stop()
	assert.False(t, <-woke, "sleepOrStop answered true after Stop")
}

// sleepInto runs sleepOrStop and delivers its answer.
func sleepInto(client *DHCPClient, d time.Duration, out chan<- bool) {
	out <- client.sleepOrStop(d)
}

// TestV4RenewalCarriesClientIdentity builds the extending DHCPREQUEST from an
// ACK with a client configured with a hostname and a client-id.
//
// VALIDATES: AC-20 -- every RENEW and REBIND carries the same option 61 and
// option 12 bytes as the acquisition modifiers put in the DORA REQUEST; and
// the message shape AC-10/AC-12 require: ciaddr the leased address, no option
// 54, no option 50, no reserved flags bit.
// PREVENTS: defect 9, the client identity dropped on renewal.
func TestV4RenewalCarriesClientIdentity(t *testing.T) {
	t.Parallel()
	client, err := newDHCPClient("eth0", "default", stubEventBus{}, true, false, dHCPConfig{
		Hostname: "ze-router",
		ClientID: "ze:01",
	}, clock.RealClock{})
	require.NoError(t, err)

	acquisition, err := dhcpv4.New(client.v4RequestModifiers()...)
	require.NoError(t, err)

	leased := net.IP{192, 0, 2, 50}
	ack, err := dhcpv4.New(
		dhcpv4.WithMessageType(dhcpv4.MessageTypeAck),
		dhcpv4.WithYourIP(leased),
		dhcpv4.WithOption(dhcpv4.OptServerIdentifier(net.IP{192, 0, 2, 1})),
		dhcpv4.WithOption(dhcpv4.OptIPAddressLeaseTime(time.Hour)),
	)
	require.NoError(t, err)
	ack.Flags = 0xffff

	renewal, err := client.v4ExtendRequest(ack)
	require.NoError(t, err)
	assert.Equal(t, acquisition.Options.Get(dhcpv4.OptionClientIdentifier), renewal.Options.Get(dhcpv4.OptionClientIdentifier), "option 61")
	assert.Equal(t, acquisition.Options.Get(dhcpv4.OptionHostName), renewal.Options.Get(dhcpv4.OptionHostName), "option 12")
	assert.NotEmpty(t, renewal.Options.Get(dhcpv4.OptionClientIdentifier), "option 61 present")
	assert.True(t, renewal.ClientIPAddr.Equal(leased), "ciaddr is the leased address")
	assert.False(t, renewal.Options.Has(dhcpv4.OptionServerIdentifier), "no option 54")
	assert.False(t, renewal.Options.Has(dhcpv4.OptionRequestedIPAddress), "no option 50")
	assert.Equal(t, uint16(0), renewal.Flags&^uint16(flagsBroadcast), "reserved flags bits")
}
