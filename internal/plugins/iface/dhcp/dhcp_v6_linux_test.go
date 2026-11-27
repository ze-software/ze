// Design: docs/features/interfaces.md -- DHCPv6 shutdown address-removal reporting.
// Related: dhcp_v6_linux.go -- v6Worker.shutdown, the producer under test.
// Related: rfc9915_dhcp_v6_lease_linux_test.go -- v6Rig and loopBackend reused here.

//go:build linux

package ifacedhcp

import (
	"errors"
	"log/slog"
	"net"
	"net/netip"
	"slices"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
)

// TestDHCPv6ShutdownReportsFailedRemoval verifies that a non-ENOENT address
// removal failure during v6Worker.shutdown logs a WARN carrying interface,
// address, fix, and the registered doctor code while raising an error event on
// internal/core/report, and that ENOENT ("already gone") completes cleanly with
// neither a WARN nor a report-bus event.
//
// VALIDATES: AC-2, AC-4 -- non-ENOENT DHCPv6 shutdown removal failure is warned
// and raised on internal/core/report; ENOENT is treated as already gone.
// PREVENTS: silently leaving Ze-owned DHCPv6 addresses in the kernel at
// shutdown, or raising false warnings when an address was already removed.
func TestDHCPv6ShutdownReportsFailedRemoval(t *testing.T) {
	require.NotNil(t, diagnostic.Lookup(codeDoctorIfaceDHCPRemoveFailed),
		"doctor metadata for %s must be registered", codeDoctorIfaceDHCPRemoveFailed)
	require.True(t, slices.Contains(diagnostic.EventCodes(), codeDoctorIfaceDHCPRemoveFailed),
		"event code %s must be registered", codeDoctorIfaceDHCPRemoveFailed)

	t.Run("non-ENOENT failure warns and raises on report bus", func(t *testing.T) {
		report.ResetForTest()
		t.Cleanup(report.ResetForTest)

		r := newV6Rig(t, dHCPConfig{})
		r.srv.serve = leaseServer(lease10x16x30, true, false)
		r.startWorker()
		r.runUntil(seconds(5))

		r.backend.mu.Lock()
		r.backend.removeAddrErr = syscall.EPERM
		r.backend.mu.Unlock()

		require.True(t, r.stop(), "Stop must return promptly")

		var foundWarn *capturedLog
		r.logs.mu.Lock()
		for i := range r.logs.records {
			rec := &r.logs.records[i]
			if rec.level == slog.LevelWarn && rec.attrs["code"] == codeDoctorIfaceDHCPRemoveFailed {
				foundWarn = rec
				break
			}
		}
		r.logs.mu.Unlock()

		require.NotNil(t, foundWarn,
			"non-ENOENT address removal failure at shutdown must log WARN with code %s", codeDoctorIfaceDHCPRemoveFailed)
		require.Equal(t, r.client.ifaceName, foundWarn.attrs["iface"],
			"WARN line must name the interface")
		require.Equal(t, v6TestCIDR, foundWarn.attrs["addr"],
			"WARN line must name the leased address")
		require.NotEmpty(t, foundWarn.attrs["fix"],
			"WARN line must carry an operator fix")

		var foundIssue *report.Issue
		errs := report.Errors(0)
		for i := range errs {
			if errs[i].Code == codeDoctorIfaceDHCPRemoveFailed {
				foundIssue = &errs[i]
				break
			}
		}
		require.NotNil(t, foundIssue,
			"non-ENOENT address removal failure at shutdown must raise %s on internal/core/report", codeDoctorIfaceDHCPRemoveFailed)
		require.NotEmpty(t, foundIssue.Fix, "report issue must carry an operator fix")
		require.True(t, strings.Contains(foundIssue.Message, v6TestCIDR),
			"report issue message %q must name the leased address %s", foundIssue.Message, v6TestCIDR)
		require.True(t, strings.Contains(foundIssue.Message, syscall.EPERM.Error()),
			"report issue message %q must carry the removal error cause", foundIssue.Message)
	})

	t.Run("ENOENT already gone completes cleanly", func(t *testing.T) {
		report.ResetForTest()
		t.Cleanup(report.ResetForTest)

		r := newV6Rig(t, dHCPConfig{})
		r.srv.serve = leaseServer(lease10x16x30, true, false)
		r.startWorker()
		r.runUntil(seconds(5))

		r.backend.mu.Lock()
		r.backend.removeAddrErr = syscall.ENOENT
		r.backend.mu.Unlock()

		require.True(t, r.stop(), "Stop must return promptly")

		r.logs.mu.Lock()
		var warnCount int
		for _, rec := range r.logs.records {
			if rec.level == slog.LevelWarn && rec.attrs["code"] == codeDoctorIfaceDHCPRemoveFailed {
				warnCount++
			}
		}
		r.logs.mu.Unlock()
		require.Zero(t, warnCount, "ENOENT address removal must not log WARN")

		for _, issue := range report.Errors(0) {
			require.NotEqual(t, codeDoctorIfaceDHCPRemoveFailed, issue.Code,
				"ENOENT address removal must not raise on internal/core/report")
		}
	})
}

// TestDHCPv6DropRegisterAndReleaseFailuresAreReported verifies that:
//   - v6Worker.drop warns and raises codeDoctorIfaceDHCPRemoveFailed on non-ENOENT
//     removal errors and stays quiet on ENOENT,
//   - leased-address ownership registration failures warn and raise
//     codeDoctorIfaceDHCPLeaseRegister, and
//   - a Release socket-open failure at shutdown warns and raises
//     codeDoctorIfaceDHCPReleaseFailed.
//
// VALIDATES: AC-2, AC-4 -- DHCPv6 drop, lease-register, and Release failure reporting.
// PREVENTS: silently dropping removal, ownership-registration, or shutdown Release
// errors at Debug.
func TestDHCPv6DropRegisterAndReleaseFailuresAreReported(t *testing.T) {
	require.NotNil(t, diagnostic.Lookup(codeDoctorIfaceDHCPLeaseRegister))
	require.True(t, slices.Contains(diagnostic.EventCodes(), codeDoctorIfaceDHCPLeaseRegister))
	require.NotNil(t, diagnostic.Lookup(codeDoctorIfaceDHCPReleaseFailed))
	require.True(t, slices.Contains(diagnostic.EventCodes(), codeDoctorIfaceDHCPReleaseFailed))

	t.Run("v6Worker drop warns on EPERM and ignores ENOENT", func(t *testing.T) {
		report.ResetForTest()
		t.Cleanup(report.ResetForTest)

		r := newV6Rig(t, dHCPConfig{})
		b := v6Binding{
			kind:     v6KindAddress,
			prefix:   netip.MustParsePrefix(v6TestCIDR),
			received: r.clk.Now(),
			valid:    time.Hour,
		}
		w := &v6Worker{
			c:          r.client,
			owner:      "iface-dhcp-v6 " + r.client.ifaceName + " unit 0",
			device:     loopKernelDevice,
			lease:      &v6Lease{server: v6ServerA, bindings: []v6Binding{b}},
			dadChecked: make(map[netip.Prefix]bool),
		}

		r.backend.mu.Lock()
		r.backend.removeAddrErr = syscall.EPERM
		r.backend.mu.Unlock()
		w.drop(b)

		var foundWarn bool
		r.logs.mu.Lock()
		for _, rec := range r.logs.records {
			if rec.level == slog.LevelWarn && rec.attrs["code"] == codeDoctorIfaceDHCPRemoveFailed {
				foundWarn = true
			}
		}
		r.logs.mu.Unlock()
		require.True(t, foundWarn, "non-ENOENT v6Worker.drop failure must log WARN")

		var foundIssue bool
		for _, issue := range report.Errors(0) {
			if issue.Code == codeDoctorIfaceDHCPRemoveFailed {
				foundIssue = true
			}
		}
		require.True(t, foundIssue, "non-ENOENT v6Worker.drop failure must raise on report bus")

		report.ResetForTest()
		r.logs.mu.Lock()
		r.logs.records = nil
		r.logs.mu.Unlock()
		r.backend.mu.Lock()
		r.backend.removeAddrErr = syscall.ENOENT
		r.backend.mu.Unlock()
		w.drop(b)

		r.logs.mu.Lock()
		for _, rec := range r.logs.records {
			require.False(t, rec.level == slog.LevelWarn && rec.attrs["code"] == codeDoctorIfaceDHCPRemoveFailed)
		}
		r.logs.mu.Unlock()
		for _, issue := range report.Errors(0) {
			require.NotEqual(t, codeDoctorIfaceDHCPRemoveFailed, issue.Code)
		}
	})

	t.Run("leased-address registration conflict warns and raises", func(t *testing.T) {
		report.ResetForTest()
		t.Cleanup(report.ResetForTest)

		r := newV6Rig(t, dHCPConfig{})
		liveBinding := v6Binding{
			kind:     v6KindAddress,
			prefix:   netip.MustParsePrefix(v6TestCIDR),
			received: r.clk.Now(),
			valid:    time.Hour,
		}
		expiredBinding := v6Binding{
			kind:     v6KindAddress,
			prefix:   netip.MustParsePrefix("2001:db8::99/128"),
			received: r.clk.Now().Add(-2 * time.Second),
			valid:    time.Second,
		}
		w := &v6Worker{
			c:          r.client,
			owner:      "iface-dhcp-v6 " + r.client.ifaceName + " unit 0",
			device:     loopKernelDevice,
			lease:      &v6Lease{server: v6ServerA, bindings: []v6Binding{liveBinding, expiredBinding}},
			dadChecked: make(map[netip.Prefix]bool),
		}

		const conflictOwner = "test-dhcp-v6-conflict-owner"
		require.NoError(t, iface.RegisterLeasedAddresses(loopKernelDevice, conflictOwner, []string{v6TestCIDR}))
		t.Cleanup(func() { iface.UnregisterLeasedAddresses(conflictOwner) })

		// expireDue prunes expiredBinding and calls registerLeased with the
		// still-live v6TestCIDR, which fails with an ownership conflict against
		// conflictOwner.
		w.expireDue()

		var foundWarn bool
		r.logs.mu.Lock()
		for _, rec := range r.logs.records {
			if rec.level == slog.LevelWarn && rec.attrs["code"] == codeDoctorIfaceDHCPLeaseRegister {
				require.Equal(t, r.client.ifaceName, rec.attrs["iface"])
				require.NotEmpty(t, rec.attrs["fix"])
				foundWarn = true
			}
		}
		r.logs.mu.Unlock()
		require.True(t, foundWarn, "registerLeased failure must log WARN")

		var foundIssue bool
		for _, issue := range report.Errors(0) {
			if issue.Code == codeDoctorIfaceDHCPLeaseRegister {
				require.Contains(t, issue.Message, r.client.ifaceName)
				require.NotEmpty(t, issue.Fix)
				foundIssue = true
			}
		}
		require.True(t, foundIssue, "registerLeased failure must raise on report bus")
	})

	t.Run("shutdown Release socket open failure warns and raises", func(t *testing.T) {
		report.ResetForTest()
		t.Cleanup(report.ResetForTest)

		r := newV6Rig(t, dHCPConfig{})
		r.srv.serve = leaseServer(lease10x16x30, true, false)

		var dialFail atomic.Bool
		dialErr := errors.New("link socket unavailable at shutdown")
		origDial := r.client.v6Dial
		r.client.v6Dial = func(device string) (v6Link, net.HardwareAddr, error) {
			if dialFail.Load() {
				return nil, nil, dialErr
			}
			return origDial(device)
		}

		r.startWorker()
		r.runUntil(seconds(5))

		dialFail.Store(true)
		require.True(t, r.stop(), "Stop must return promptly when Release socket fails")

		var foundWarn bool
		r.logs.mu.Lock()
		for _, rec := range r.logs.records {
			if rec.level == slog.LevelWarn && rec.attrs["code"] == codeDoctorIfaceDHCPReleaseFailed {
				require.Equal(t, r.client.ifaceName, rec.attrs["iface"])
				require.NotEmpty(t, rec.attrs["fix"])
				foundWarn = true
			}
		}
		r.logs.mu.Unlock()
		require.True(t, foundWarn, "Release socket open failure at shutdown must log WARN")

		var foundIssue bool
		for _, issue := range report.Errors(0) {
			if issue.Code == codeDoctorIfaceDHCPReleaseFailed {
				require.Contains(t, issue.Message, dialErr.Error())
				require.NotEmpty(t, issue.Fix)
				foundIssue = true
			}
		}
		require.True(t, foundIssue, "Release socket open failure at shutdown must raise on report bus")
	})
}
