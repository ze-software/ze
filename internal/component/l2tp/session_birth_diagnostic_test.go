// Design: docs/architecture/doctor-and-health-checks.md -- coded refusals remain visible to runtime doctor
// Related: session_birth.go -- invalid logical references are refused before state mutation

package l2tp

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
	"github.com/ze-software/ze/internal/component/l2tp/subscriber"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/internal/core/subscriberid"
)

// TestL2TPMissingBirthRefusalsCarryDoctorFinding drives each actual invalid-Birth
// guard and requires its coded warning and retained original operation context.
// MUTATION: omit the code attribute or retained raise in reportSessionBirthRefusal.
func TestL2TPMissingBirthRefusalsCarryDoctorFinding(t *testing.T) {
	// The binary entry point registers builtin metadata explicitly, not through
	// package init. Use that same composition step in this focused unit.
	diagnostic.RegisterBuiltinCodes()
	const tunnelID uint16 = 64123
	const sessionID uint16 = 64124
	cases := []struct {
		operation string
		refuse    func(*testing.T, *slog.Logger)
	}{
		{"route assignment", func(_ *testing.T, logger *slog.Logger) {
			observer := newSubscriberRouteObserver(logger, nil)
			observer.OnSessionIPUp(subscriberid.L2TPBirth{}, tunnelID, sessionID, "original",
				netip.MustParseAddr("192.0.2.23"), nil)
		}},
		{"route withdrawal", func(_ *testing.T, logger *slog.Logger) {
			observer := newSubscriberRouteObserver(logger, nil)
			observer.OnSessionDown(subscriberid.L2TPBirth{}, tunnelID, sessionID)
		}},
		{"subscriber session-up", func(t *testing.T, logger *slog.Logger) {
			bridge := sessionBirthDiagnosticBridge(t, logger)
			bridge.onSessionUp(&l2tpevents.SessionUpPayload{
				SubscriberID: "original-subscriber", TunnelID: tunnelID, SessionID: sessionID,
			})
		}},
		{"subscriber session-down", func(t *testing.T, logger *slog.Logger) {
			bridge := sessionBirthDiagnosticBridge(t, logger)
			bridge.onSessionDown(&l2tpevents.SessionDownPayload{
				SubscriberID: "original-subscriber", TunnelID: tunnelID, SessionID: sessionID,
			})
		}},
		{"subscriber address assignment", func(t *testing.T, logger *slog.Logger) {
			bridge := sessionBirthDiagnosticBridge(t, logger)
			bridge.onSessionIPAssigned(&l2tpevents.SessionIPAssignedPayload{
				SubscriberID: "original-subscriber", TunnelID: tunnelID, SessionID: sessionID,
				PeerAddr: "192.0.2.23",
			})
		}},
	}
	for _, item := range cases {
		t.Run(item.operation, func(t *testing.T) {
			preserveSessionBirthDiagnostic(t, sessionBirthRefusalReportSubject, tunnelID, sessionID)
			var logs bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: slog.LevelWarn}))
			item.refuse(t, logger)
			var warning map[string]any
			require.NoError(t, json.Unmarshal(bytes.TrimSpace(logs.Bytes()), &warning))
			require.Equal(t, report.CodeInternalDefect, warning["code"], "the refusal log must name a declared doctor code")
			require.NotNil(t, diagnostic.Lookup(report.CodeInternalDefect))
			require.Equal(t, "ze doctor", warning["fix"])
			require.Equal(t, item.operation, warning["operation"])
			finding, present := sessionBirthDiagnostic(sessionBirthRefusalReportSubject)
			require.True(t, present, "runtime doctor must retain the refused operation")
			require.Equal(t, item.operation, finding.Detail["operation"])
			require.Equal(t, tunnelID, finding.Detail["tunnel-id"])
			require.Equal(t, sessionID, finding.Detail["session-id"])
			require.Equal(t, "ze doctor", finding.Fix)
		})
	}
}

// TestL2TPRouteSourceRefusalCarriesDoctorCode observes the actual allocator's
// exhaustion cause at its existing reporting boundary, without changing the
// process allocator or another route lifetime's token.
// MUTATION: omit the code attribute from reportSourceRefusal's warning.
func TestL2TPRouteSourceRefusalCarriesDoctorCode(t *testing.T) {
	const tunnelID uint16 = 64125
	const sessionID uint16 = 64126
	preserveSessionBirthDiagnostic(t, "subscriber route source", tunnelID, sessionID)
	sequence := routeSourceSequence{last: ^uint32(0)}
	_, err := sequence.next()
	require.ErrorIs(t, err, errRouteSourceExhausted)
	birth, err := subscriberid.ParseL2TPBirth("l2tp-00000000000000000000000000000023")
	require.NoError(t, err)
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: slog.LevelWarn}))
	observer := newSubscriberRouteObserver(logger, nil)
	observer.reportSourceRefusal(subscriber.Session{ID: birth.String(), L2TPBirth: birth, AccessType: subscriber.AccessL2TP,
		TunnelID: tunnelID, SessionID: sessionID}, errRouteSourceExhausted)
	var warning map[string]any
	require.NoError(t, json.Unmarshal(bytes.TrimSpace(logs.Bytes()), &warning))
	require.Equal(t, report.CodeInternalDefect, warning["code"], "the allocator refusal log must name its doctor code")
	require.Equal(t, errRouteSourceExhausted.Fix(), warning["fix"])
	finding, present := sessionBirthDiagnostic("subscriber route source")
	require.True(t, present)
	require.Equal(t, birth.String(), finding.Detail["subscriber-id"])
	require.Equal(t, tunnelID, finding.Detail["tunnel-id"])
	require.Equal(t, sessionID, finding.Detail["session-id"])
	require.Equal(t, errRouteSourceExhausted.Error(), finding.Detail["cause"])
}

func sessionBirthDiagnosticBridge(t *testing.T, logger *slog.Logger) *subscriberBridge {
	t.Helper()
	bridge := newSubscriberBridge(subscriber.NewRegistry(), newTestBus(), logger)
	t.Cleanup(bridge.stop)
	return bridge
}

// preserveSessionBirthDiagnostic leaves unrelated warning subjects intact. It
// restores a previous finding only while this test still owns the latest
// coordinates; a later owner is never cleared or overwritten by cleanup.
func preserveSessionBirthDiagnostic(t *testing.T, subject string, tunnelID, sessionID uint16) {
	t.Helper()
	previous, existed := sessionBirthDiagnostic(subject)
	t.Cleanup(func() {
		current, present := sessionBirthDiagnostic(subject)
		if !present {
			return
		}
		if current.Detail["tunnel-id"] != tunnelID {
			return
		}
		if current.Detail["session-id"] != sessionID {
			return
		}
		if existed {
			report.RaiseWarningFix(previous.Source, previous.Code, previous.Subject,
				previous.Message, previous.Fix, previous.Detail)
			return
		}
		report.ClearWarning(SubsystemName, report.CodeInternalDefect, subject)
	})
}

func sessionBirthDiagnostic(subject string) (report.Issue, bool) {
	// The report store bounds the warning snapshot by its configured capacity.
	for _, finding := range report.Warnings() {
		if finding.Source != SubsystemName {
			continue
		}
		if finding.Code != report.CodeInternalDefect {
			continue
		}
		if finding.Subject != subject {
			continue
		}
		return finding, true
	}
	return report.Issue{}, false
}
