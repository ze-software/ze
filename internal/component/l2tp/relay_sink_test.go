package l2tp

import (
	"log/slog"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/callsink"
)

// TestRelaySink_UnmatchedServiceTerminatesLocally -- AC-3.
//
// VALIDATES: a PPPoE service with no relay binding is not relayed (Relay
// returns accepted=false), so pppoe terminates PPP locally.
func TestRelaySink_UnmatchedServiceTerminatesLocally(t *testing.T) {
	s := &Subsystem{
		params: Parameters{
			Remotes: []Remote{{Name: "lns1", Address: netip.MustParseAddrPort("10.0.0.1:1701")}},
			Relays:  []RelayBinding{{Service: "wholesale", Remote: "lns1"}},
		},
		logger: slog.Default(),
	}
	accepted, err := (&relaySink{s: s}).Relay(callsink.Request{Service: "retail"})
	require.NoError(t, err)
	require.False(t, accepted)
}

// TestRelaySink_MatchedButNoListener -- AC-3 fallback.
//
// VALIDATES: a matched relay with no running L2TP listener returns an error
// so pppoe can fall back to local termination instead of stranding the
// subscriber.
func TestRelaySink_MatchedButNoListener(t *testing.T) {
	s := &Subsystem{
		params: Parameters{
			Remotes: []Remote{{Name: "lns1", Address: netip.MustParseAddrPort("10.0.0.1:1701")}},
			Relays:  []RelayBinding{{Service: "wholesale", Remote: "lns1"}},
		},
		logger: slog.Default(),
	}
	accepted, err := (&relaySink{s: s}).Relay(callsink.Request{Service: "wholesale"})
	require.ErrorIs(t, err, errRelayNoListener)
	require.False(t, accepted)
}

// TestRelaySink_MatchedDialsRemote -- AC-3 relay origination.
//
// VALIDATES: a subscriber on a relay-bound service triggers a dial toward the
// bound remote -- an SCCRQ leaves ze -- and the sink reports accepted so the
// PPPoE server skips local termination.
func TestRelaySink_MatchedDialsRemote(t *testing.T) {
	ln, r, _, stop := buildLogReactor(t)
	defer stop()
	client := newClient(t, ln)
	defer client.Close()

	remoteAddr := clientAddrPort(t, client)
	s := &Subsystem{
		params: Parameters{
			Remotes: []Remote{{Name: "lns1", Address: remoteAddr}},
			Relays:  []RelayBinding{{Service: "wholesale", Remote: "lns1"}},
		},
		reactors: []*L2TPReactor{r},
		logger:   slog.Default(),
	}

	accepted, err := (&relaySink{s: s}).Relay(callsink.Request{
		Service: "wholesale", SubscriberMAC: "aa:bb:cc:dd:ee:ff", ChannelFD: 7,
	})
	require.NoError(t, err)
	require.True(t, accepted, "matched relay hands the subscriber to L2TP")

	sccrq := readDatagram(t, client)
	require.Equal(t, MsgSCCRQ, msgTypeOf(t, sccrq), "ze dialed the relay remote")
}

// TestRelayTargetLocked_Resolution -- AC-3 binding resolution.
//
// VALIDATES: relayTargetLocked returns the bound remote for a configured
// service and reports no match for an unknown one.
func TestRelayTargetLocked_Resolution(t *testing.T) {
	s := &Subsystem{params: Parameters{
		Remotes: []Remote{{Name: "lns1", Address: netip.MustParseAddrPort("10.0.0.1:1701")}},
		Relays:  []RelayBinding{{Service: "wholesale", Remote: "lns1"}, {Service: "", Remote: "lns1"}},
	}}
	rem, ok := s.relayTargetLocked("wholesale")
	require.True(t, ok)
	require.Equal(t, "lns1", rem.Name)

	// Empty service (default) is a valid binding key.
	_, ok = s.relayTargetLocked("")
	require.True(t, ok)

	_, ok = s.relayTargetLocked("nope")
	require.False(t, ok)
}
