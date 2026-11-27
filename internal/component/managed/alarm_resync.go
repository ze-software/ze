// Design: docs/architecture/fleet-config.md -- config-resync-failed hub alarm kind registration and evaluation
// Related: register.go -- init calls registerResyncFailedAlarm beside registerAlarmCommands
// Related: alarm_resync_test.go -- verifies AC-1 open/clear/separation and AC-2 hiding behind device-offline

package managed

import (
	"cmp"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	// AlarmKindConfigResyncFailed is the registered hub alarm kind opened when
	// both diff resync and full configuration replay fail on a non-holding
	// declared managed device with a non-verify error.
	AlarmKindConfigResyncFailed = "config-resync-failed"

	resyncAlarmDefaultSummary = "Configuration resync failed"
	resyncAlarmDefaultDiff    = "diff resync did not converge"
	resyncAlarmDefaultFull    = "full replay did not converge"
	resyncAlarmFixCommand     = "ze show doctor"
	resyncAlarmHashNone       = "none"
	resyncAlarmReasonMax      = 256
	resyncAlarmTruncatedTag   = "..."
)

func registerResyncFailedAlarm() {
	pluginserver.MustRegisterAlarmKind(pluginserver.AlarmKindSpec{
		Name:     AlarmKindConfigResyncFailed,
		Severity: pluginserver.AlarmSeverityWarning,
		Scope:    pluginserver.AlarmScopeDevice,
		Causes:   []string{pluginserver.AlarmKindDeviceOffline},
		Settles:  false,
		Summary:  resyncAlarmDefaultSummary,
		Evaluate: evaluateResyncFailedAlarm,
	})
}

// evaluateResyncFailedAlarm derives config-resync-failed observations from the
// hub's read-only per-device snapshot. Only declared, non-holding devices with
// an active non-verify ResyncFailure emit an observation.
func evaluateResyncFailedAlarm(ctx pluginserver.AlarmEvalContext) []pluginserver.AlarmObservation {
	order := make([]int, 0, len(ctx.Devices))
	for i := range ctx.Devices {
		dev := &ctx.Devices[i]
		if !dev.Declared {
			continue
		}
		if dev.ConfigDiverged {
			continue
		}
		if dev.ResyncFailure == nil {
			continue
		}
		if dev.Name == "" {
			continue
		}
		order = append(order, i)
	}
	if len(order) == 0 {
		return nil
	}
	slices.SortFunc(order, func(a, b int) int {
		return cmp.Compare(ctx.Devices[a].Name, ctx.Devices[b].Name)
	})

	out := make([]pluginserver.AlarmObservation, 0, len(order))
	var tb textbuf.Buffer
	for _, idx := range order {
		dev := &ctx.Devices[idx]
		failure := dev.ResyncFailure
		failedAt := failure.FailedAt.UTC()
		if failedAt.IsZero() {
			failedAt = ctx.Now.UTC()
		}
		ts := failedAt.Format(time.RFC3339)

		diffReason := boundResyncAlarmText(failure.DiffReason, resyncAlarmDefaultDiff)
		fullReason := boundResyncAlarmText(failure.FullReason, resyncAlarmDefaultFull)
		clientChain := boundResyncAlarmText(failure.ClientChain, resyncAlarmHashNone)
		expectedChain := boundResyncAlarmText(failure.ExpectedChain, resyncAlarmHashNone)

		summary := tb.Reset().
			Str("Device ").Str(dev.Name).
			Str(" configuration resync failed at ").Str(ts).
			Str(" (diff: ").Str(diffReason).
			Str("; full: ").Str(fullReason).
			Str("; code ").Str(pluginserver.CodeDoctorManagedChainDrift).
			Str("); run ").Str(resyncAlarmFixCommand).
			String()

		detail := tb.Reset().
			Str("diff=").Str(diffReason).
			Str("; full=").Str(fullReason).
			Str("; client-chain=").Str(clientChain).
			Str("; expected-chain=").Str(expectedChain).
			Str("; code=").Str(pluginserver.CodeDoctorManagedChainDrift).
			Str("; fix=").Str(resyncAlarmFixCommand).
			String()

		out = append(out, pluginserver.AlarmObservation{
			Device:     dev.Name,
			Subject:    "",
			Summary:    summary,
			Detail:     detail,
			ObservedAt: failedAt,
		})
	}
	return out
}

// boundResyncAlarmText collapses control characters to single spaces, applies
// fallback when empty, and truncates on a UTF-8 rune boundary to at most
// resyncAlarmReasonMax bytes so alarm summaries and details stay bounded and
// never cut off trailing codes or fix commands.
func boundResyncAlarmText(raw, fallback string) string {
	var tb textbuf.Buffer
	prevSpace := false
	for _, r := range raw {
		if unicode.IsControl(r) {
			if tb.Len() > 0 && !prevSpace {
				tb.Byte(' ')
				prevSpace = true
			}
			continue
		}
		if unicode.IsSpace(r) {
			if tb.Len() > 0 && !prevSpace {
				tb.Byte(' ')
				prevSpace = true
			}
			continue
		}
		tb.Str(string(r))
		prevSpace = false
	}
	cleaned := strings.TrimSpace(tb.String())
	if cleaned == "" {
		return fallback
	}
	if len(cleaned) <= resyncAlarmReasonMax {
		return cleaned
	}
	limit := resyncAlarmReasonMax - len(resyncAlarmTruncatedTag)
	for limit > 0 && !utf8.RuneStart(cleaned[limit]) {
		limit--
	}
	return cleaned[:limit] + resyncAlarmTruncatedTag
}
