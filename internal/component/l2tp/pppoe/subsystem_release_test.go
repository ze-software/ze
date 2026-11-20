// Design: docs/architecture/l2tp/bng-5-pppoe.md -- PPPoE consumer of PPP events
//
// Goal: prove PPPoE's consumer of the PPP event stream carries a family
// release to the subscriber registry and the subscriber namespace, and that a
// PPP event it has no case for is reported rather than dropped. Method: drive
// Subsystem.handlePPPEvent, the entry point the driver's event consumer
// calls, on a session the real PPP driver brought up.

package pppoe

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/component/l2tp/subscriber"
	subevents "github.com/ze-software/ze/internal/component/l2tp/subscriber/events"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
)

// TestPPPoEFamilyReleaseWithdrawsOneFamily: an IPv4 release clears the
// address and both DNS servers from the registry entry and publishes the
// subscriber release with the family, while the IPv6 interface identifier
// stays; an IPv6 release then clears the identifier. Fails before the
// release case: handlePPPEvent had no case for the event.
func TestPPPoEFamilyReleaseWithdrawsOneFamily(t *testing.T) {
	const ifIndex = 11
	const sid = uint16(51)
	bus := newRecordBus()
	sub := newTestSubsystem(t, bus, ifIndex)
	srv := sub.servers[ifIndex]
	session := &Session{SID: sid, IfName: "eth0", Opened: fixtureOpened(t, "ppp51"), State: StateSession}
	require.NoError(t, srv.sessions.Add(session))
	id := pppoeSessionID(ifIndex, sid)
	t.Cleanup(func() { subscriber.DefaultRegistry.Remove(id) })

	var released []*subevents.SessionIPReleasedPayload
	subevents.SessionIPReleased.Subscribe(bus, func(p *subevents.SessionIPReleasedPayload) { released = append(released, p) })

	owner := fixtureCurrentStart(t, srv, srv.sessions.Lookup(sid), &fixtureCurrentStartConfig{method: ppp.AuthMethodNone, dual: false})
	owner.authenticate(t, &fixtureCurrentAuthConfig{username: ""})
	up := owner.up(t)
	sub.handlePPPEvent(up)

	entry, ok := subscriber.DefaultRegistry.Get(id)
	require.True(t, ok, "session-up registers the session")
	identifier := [8]byte{0x02, 0, 0, 0, 0, 0, 0, 0x51}
	entry.IPv4Addr = netip.MustParseAddr("192.0.2.51")
	entry.DNSPrimary = netip.MustParseAddr("192.0.2.53")
	entry.DNSSecondary = netip.MustParseAddr("192.0.2.54")
	entry.IPv6InterfaceID = identifier
	subscriber.DefaultRegistry.Add(&entry)

	sub.handlePPPEvent(ppp.EventSessionIPReleased{Control: up.Control, SessionID: sid, Family: ppp.AddressFamilyIPv4})
	require.Len(t, released, 1)
	require.Equal(t, l2tpevents.AddressFamilyIPv4, released[0].Family)
	require.False(t, released[0].Session.IPv4Addr.IsValid())
	require.Equal(t, id, released[0].Session.ID)
	entry, ok = subscriber.DefaultRegistry.Get(id)
	require.True(t, ok, "a family release keeps the session")
	require.False(t, entry.IPv4Addr.IsValid(), "IPv4 release clears the address")
	require.False(t, entry.DNSPrimary.IsValid(), "IPv4 release clears dns-primary")
	require.False(t, entry.DNSSecondary.IsValid(), "IPv4 release clears dns-secondary")
	require.Equal(t, identifier, entry.IPv6InterfaceID, "IPv4 release keeps the IPv6 family")

	sub.handlePPPEvent(ppp.EventSessionIPReleased{Control: up.Control, SessionID: sid, Family: ppp.AddressFamilyIPv6})
	require.Len(t, released, 2)
	require.Equal(t, l2tpevents.AddressFamilyIPv6, released[1].Family)
	entry, ok = subscriber.DefaultRegistry.Get(id)
	require.True(t, ok)
	require.Equal(t, [8]byte{}, entry.IPv6InterfaceID, "IPv6 release clears the interface identifier")
	require.Equal(t, subscriber.StateActive, entry.State)

	sub.handlePPPEvent(owner.down(t))
}

// unknownPPPEvent stands for a ppp.Event type PPPoE's switch has no case for.
// Embedding the interface promotes its unexported marker method.
type unknownPPPEvent struct{ ppp.Event }

func pppEventFinding() (report.Issue, bool) {
	for _, finding := range report.Warnings() {
		if finding.Source != SubsystemName {
			continue
		}
		if finding.Code != diagnostic.CodeDoctorInternalDefect {
			continue
		}
		if finding.Subject != pppEventReportSubject {
			continue
		}
		return finding, true
	}
	return report.Issue{}, false
}

// TestPPPoEUnknownEventIsLogged (AC-13): EventEchoRTT, which PPPoE ignores on
// purpose, logs nothing and raises nothing; a type with no case logs one Warn
// with the internal-defect doctor code, its fix and the Go type, and raises
// the matching runtime doctor finding. Fails before the default arm: the
// event was dropped with no log line.
func TestPPPoEUnknownEventIsLogged(t *testing.T) {
	// The binary entry point registers the built-in codes; a test binary
	// must, or diagnostic.Lookup answers nil for CodeDoctorInternalDefect.
	diagnostic.RegisterBuiltinCodes()
	sub := newTestSubsystem(t, newRecordBus(), 12)
	var logs bytes.Buffer
	sub.logger = slog.New(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))

	before, beforePresent := pppEventFinding()
	sub.handlePPPEvent(ppp.EventEchoRTT{SessionID: 1})
	require.Empty(t, logs.String(), "an echo round-trip time is not a defect")
	after, afterPresent := pppEventFinding()
	require.Equal(t, beforePresent, afterPresent, "an echo round-trip time raises no finding")
	require.Equal(t, before, after)

	sub.handlePPPEvent(unknownPPPEvent{})
	var warning map[string]any
	require.NoError(t, json.Unmarshal(bytes.TrimSpace(logs.Bytes()), &warning))
	require.Equal(t, "WARN", warning["level"])
	require.Equal(t, diagnostic.CodeDoctorInternalDefect, warning["code"])
	require.NotNil(t, diagnostic.Lookup(diagnostic.CodeDoctorInternalDefect))
	require.Equal(t, "ze show doctor", warning["fix"])
	require.Equal(t, "pppoe.unknownPPPEvent", warning["type"])
	finding, present := pppEventFinding()
	require.True(t, present, "runtime doctor holds the unhandled-event finding")
	require.Equal(t, "ze show doctor", finding.Fix)
	require.Equal(t, "pppoe.unknownPPPEvent", finding.Detail["type"])
}
