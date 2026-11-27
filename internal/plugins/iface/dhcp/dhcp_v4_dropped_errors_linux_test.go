// Design: docs/features/interfaces.md -- DHCPv4 removal and EventBus publish failure reporting.
// Related: dhcp_v4_linux.go -- removeV4Route, removeV4Address; dhcp_linux.go -- publishDHCP.

//go:build linux

package ifacedhcp

import (
	"errors"
	"log/slog"
	"slices"
	"strings"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
)

type failingDHCPBus struct {
	err error
}

func (b *failingDHCPBus) Emit(_, _ string, _ any) (int, error) {
	return 0, b.err
}

func (b *failingDHCPBus) Subscribe(_, _ string, _ func(any)) func() {
	return func() {}
}

// TestDHCPv4RemovalReportsRealErrorAndIgnoresAbsent verifies that removeV4Route
// and removeV4Address warn with codeDoctorIfaceDHCPRemoveFailed and raise on
// internal/core/report for real removal errors, while treating already-gone
// errors (ENOENT, ESRCH, EADDRNOTAVAIL) as silent success.
//
// VALIDATES: AC-2, AC-4 -- DHCPv4 route/address removal failures are warned and
// raised on report bus; absent kernel objects complete cleanly.
// PREVENTS: silently leaving Ze-owned DHCPv4 addresses or routes in the kernel,
// or raising false warnings when already removed.
func TestDHCPv4RemovalReportsRealErrorAndIgnoresAbsent(t *testing.T) {
	require.NotNil(t, diagnostic.Lookup(codeDoctorIfaceDHCPRemoveFailed))
	require.True(t, slices.Contains(diagnostic.EventCodes(), codeDoctorIfaceDHCPRemoveFailed))

	t.Run("removeV4Route real error warns and raises", func(t *testing.T) {
		report.ResetForTest()
		t.Cleanup(report.ResetForTest)

		r := newV4Rig(t)
		r.backend.mu.Lock()
		r.backend.removeRouteErr = syscall.EPERM
		r.backend.mu.Unlock()

		route := v4Route{gateway: "192.0.2.254", installed: true}
		r.client.removeV4Route(&route)
		require.False(t, route.installed)

		var foundWarn bool
		r.logs.mu.Lock()
		for _, rec := range r.logs.records {
			if rec.level == slog.LevelWarn && rec.attrs["code"] == codeDoctorIfaceDHCPRemoveFailed {
				require.Equal(t, r.client.ifaceName, rec.attrs["iface"])
				require.Equal(t, "192.0.2.254", rec.attrs["gw"])
				require.NotEmpty(t, rec.attrs["fix"])
				foundWarn = true
			}
		}
		r.logs.mu.Unlock()
		require.True(t, foundWarn, "non-ENOENT removeV4Route failure must log WARN")

		var foundIssue bool
		for _, issue := range report.Errors(0) {
			if issue.Code == codeDoctorIfaceDHCPRemoveFailed {
				require.Contains(t, issue.Message, "192.0.2.254")
				require.Contains(t, issue.Message, syscall.EPERM.Error())
				require.NotEmpty(t, issue.Fix)
				foundIssue = true
			}
		}
		require.True(t, foundIssue, "non-ENOENT removeV4Route failure must raise on report bus")
	})

	for _, absentErr := range []error{syscall.ENOENT, syscall.ESRCH} {
		t.Run("removeV4Route "+absentErr.Error()+" is silent", func(t *testing.T) {
			report.ResetForTest()
			t.Cleanup(report.ResetForTest)

			r := newV4Rig(t)
			r.backend.mu.Lock()
			r.backend.removeRouteErr = absentErr
			r.backend.mu.Unlock()

			route := v4Route{gateway: "192.0.2.254", installed: true}
			r.client.removeV4Route(&route)

			r.logs.mu.Lock()
			for _, rec := range r.logs.records {
				require.False(t, rec.level == slog.LevelWarn && rec.attrs["code"] == codeDoctorIfaceDHCPRemoveFailed)
			}
			r.logs.mu.Unlock()
			for _, issue := range report.Errors(0) {
				require.NotEqual(t, codeDoctorIfaceDHCPRemoveFailed, issue.Code)
			}
		})
	}

	t.Run("removeV4Address real error warns and raises", func(t *testing.T) {
		report.ResetForTest()
		t.Cleanup(report.ResetForTest)

		r := newV4Rig(t)
		r.backend.mu.Lock()
		r.backend.removeAddrErr = syscall.EPERM
		r.backend.mu.Unlock()

		r.client.removeV4Address("192.0.2.50/24")

		var foundWarn bool
		r.logs.mu.Lock()
		for _, rec := range r.logs.records {
			if rec.level == slog.LevelWarn && rec.attrs["code"] == codeDoctorIfaceDHCPRemoveFailed {
				require.Equal(t, r.client.ifaceName, rec.attrs["iface"])
				require.Equal(t, "192.0.2.50/24", rec.attrs["cidr"])
				require.NotEmpty(t, rec.attrs["fix"])
				foundWarn = true
			}
		}
		r.logs.mu.Unlock()
		require.True(t, foundWarn, "non-ENOENT removeV4Address failure must log WARN")

		var foundIssue bool
		for _, issue := range report.Errors(0) {
			if issue.Code == codeDoctorIfaceDHCPRemoveFailed {
				require.Contains(t, issue.Message, "192.0.2.50/24")
				require.Contains(t, issue.Message, syscall.EPERM.Error())
				foundIssue = true
			}
		}
		require.True(t, foundIssue, "non-ENOENT removeV4Address failure must raise on report bus")
	})

	for _, absentErr := range []error{syscall.ENOENT, syscall.EADDRNOTAVAIL} {
		t.Run("removeV4Address "+absentErr.Error()+" is silent", func(t *testing.T) {
			report.ResetForTest()
			t.Cleanup(report.ResetForTest)

			r := newV4Rig(t)
			r.backend.mu.Lock()
			r.backend.removeAddrErr = absentErr
			r.backend.mu.Unlock()

			r.client.removeV4Address("192.0.2.50/24")

			r.logs.mu.Lock()
			for _, rec := range r.logs.records {
				require.False(t, rec.level == slog.LevelWarn && rec.attrs["code"] == codeDoctorIfaceDHCPRemoveFailed)
			}
			r.logs.mu.Unlock()
			for _, issue := range report.Errors(0) {
				require.NotEqual(t, codeDoctorIfaceDHCPRemoveFailed, issue.Code)
			}
		})
	}
}

// TestDHCPPublishReportsEmitFailureRateLimited verifies that publishDHCP logs
// WARN with codeDoctorIfaceDHCPEventEmit and raises on internal/core/report
// when EventBus emit fails, and rate-limits consecutive duplicates within the
// reporting window.
//
// VALIDATES: AC-4 -- DHCP lease event emit failure warns and raises once per window.
// PREVENTS: silently dropping lease acquisition/expiry events at Debug or
// flooding logs on repeated bus failures.
func TestDHCPPublishReportsEmitFailureRateLimited(t *testing.T) {
	require.NotNil(t, diagnostic.Lookup(codeDoctorIfaceDHCPEventEmit))
	require.True(t, slices.Contains(diagnostic.EventCodes(), codeDoctorIfaceDHCPEventEmit))

	report.ResetForTest()
	t.Cleanup(report.ResetForTest)

	r := newV4Rig(t)
	emitErr := errors.New("dhcp event bus unavailable")
	r.client.eventBus = &failingDHCPBus{err: emitErr}

	payload := iface.DHCPPayload{Name: r.client.ifaceName, Address: "192.0.2.50/24"}
	r.client.publishDHCP(iface.TopicDHCPLeaseAcquired, payload)
	// Immediate second failure for the same interface and event is suppressed.
	r.client.publishDHCP(iface.TopicDHCPLeaseAcquired, payload)

	var warnCount int
	r.logs.mu.Lock()
	for _, rec := range r.logs.records {
		if rec.level == slog.LevelWarn && rec.attrs["code"] == codeDoctorIfaceDHCPEventEmit {
			require.Equal(t, r.client.ifaceName, rec.attrs["iface"])
			require.NotEmpty(t, rec.attrs["fix"])
			warnCount++
		}
	}
	r.logs.mu.Unlock()
	require.Equal(t, 1, warnCount, "repeated publishDHCP failures must be rate-limited to one WARN")

	var issueCount int
	for _, issue := range report.Errors(0) {
		if issue.Code == codeDoctorIfaceDHCPEventEmit {
			require.True(t, strings.Contains(issue.Message, emitErr.Error()))
			require.NotEmpty(t, issue.Fix)
			issueCount++
		}
	}
	require.Equal(t, 1, issueCount, "repeated publishDHCP failures must raise once per window")
}
