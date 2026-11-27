// Design: docs/architecture/doctor-and-health-checks.md -- iface dropped-error reporting and doctor code coverage.
// Related: config_apply.go -- addDesiredAddresses; config_sysctl.go -- applySysctl, applySysctlProfiles; register.go -- emitInterfaceRollback, cleanupStaleIPv6DefaultRoutes.

package iface

import (
	"bytes"
	"errors"
	"log/slog"
	"os"
	"slices"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/diagnostic"
	ifaceevents "github.com/ze-software/ze/internal/core/iface/events"
	"github.com/ze-software/ze/internal/core/report"
	sysctlreg "github.com/ze-software/ze/internal/core/sysctl"
)

type failingIfaceBus struct {
	err error
}

func (b *failingIfaceBus) Emit(_, _ string, _ any) (int, error) {
	return 0, b.err
}

func (b *failingIfaceBus) Subscribe(_, _ string, _ func(any)) func() {
	return func() {}
}

type droppedErrBackend struct {
	Backend
	addAddrErr    error
	listRoutesErr error
}

func (b *droppedErrBackend) Close() error { return nil }

func (b *droppedErrBackend) AddAddress(_, _ string) error {
	return b.addAddrErr
}

func (b *droppedErrBackend) GetLink(name string) (*InterfaceInfo, error) {
	return &InterfaceInfo{Name: name, OsName: name, Index: 4}, nil
}

func (b *droppedErrBackend) ListRoutes(_, _ string) ([]RouteInfo, error) {
	return nil, b.listRoutesErr
}

func findReportErrorByCode(code string) *report.Issue {
	errs := report.Errors(0)
	for i := range errs {
		if errs[i].Code == code {
			return &errs[i]
		}
	}
	return nil
}

// TestAddDesiredAddressesReportsRealFailureAndSkipsBenign verifies that
// addDesiredAddresses warns with codeDoctorIfaceAddressAdd and raises on the
// report bus for non-benign backend errors, while skipping ErrBackendNotReady
// and already-exists errors cleanly.
//
// VALIDATES: AC-4 -- deferred address-add refusal warns and raises on report bus.
// PREVENTS: silently dropping real address-add errors at Debug during deferred
// reconcile, or falsely warning on EEXIST / ErrBackendNotReady.
func TestAddDesiredAddressesReportsRealFailureAndSkipsBenign(t *testing.T) {
	require.NotNil(t, diagnostic.Lookup(codeDoctorIfaceAddressAdd))
	require.True(t, slices.Contains(diagnostic.EventCodes(), codeDoctorIfaceAddressAdd))

	cfg := &ifaceConfig{
		Ethernet: []ifaceEntry{
			{
				Name:  "eth0",
				Units: []unitEntry{{Addresses: []string{"192.0.2.10/24"}}},
			},
		},
	}

	t.Run("real error warns and raises", func(t *testing.T) {
		report.ResetForTest()
		t.Cleanup(report.ResetForTest)

		be := &droppedErrBackend{addAddrErr: syscall.EPERM}
		out := logFrom(t, func() {
			addDesiredAddresses(cfg, be, map[string]string{"eth0": "eth0"})
		})

		require.Contains(t, out, "level=WARN")
		require.Contains(t, out, "code="+codeDoctorIfaceAddressAdd)
		require.Contains(t, out, "eth0")
		require.Contains(t, out, "192.0.2.10/24")
		require.Contains(t, out, "ze request reload")

		issue := findReportErrorByCode(codeDoctorIfaceAddressAdd)
		require.NotNil(t, issue)
		require.Contains(t, issue.Message, "192.0.2.10/24")
		require.Contains(t, issue.Message, syscall.EPERM.Error())
		require.NotEmpty(t, issue.Fix)
	})

	for _, tc := range []struct {
		name string
		err  error
	}{
		{"backend not ready", ErrBackendNotReady},
		{"syscall EEXIST", syscall.EEXIST},
		{"os ErrExist", os.ErrExist},
	} {
		t.Run("benign "+tc.name+" stays quiet", func(t *testing.T) {
			report.ResetForTest()
			t.Cleanup(report.ResetForTest)

			be := &droppedErrBackend{addAddrErr: tc.err}
			out := logFrom(t, func() {
				addDesiredAddresses(cfg, be, map[string]string{"eth0": "eth0"})
			})

			require.NotContains(t, out, "level=WARN")
			require.Nil(t, findReportErrorByCode(codeDoctorIfaceAddressAdd))
		})
	}
}

// TestApplySysctlAndProfilesReportEmitFailure verifies that EventBus emit
// failures in applySysctl and applySysctlProfiles log WARN with
// codeDoctorIfaceSysctlEmit and raise on the report bus.
//
// VALIDATES: AC-4 -- iface sysctl EventBus emit failures warn and raise.
// PREVENTS: silently dropping interface sysctl defaults and profile updates at Debug.
func TestApplySysctlAndProfilesReportEmitFailure(t *testing.T) {
	require.NotNil(t, diagnostic.Lookup(codeDoctorIfaceSysctlEmit))
	require.True(t, slices.Contains(diagnostic.EventCodes(), codeDoctorIfaceSysctlEmit))

	emitErr := errors.New("sysctl event bus unavailable")
	prevBus := GetEventBus()
	SetEventBus(&failingIfaceBus{err: emitErr})
	t.Cleanup(func() { SetEventBus(prevBus) })

	const profileName = "test-dropped-err-profile"
	sysctlreg.RegisterProfile(sysctlreg.ProfileDef{
		Name: profileName,
		Settings: []sysctlreg.ProfileSetting{
			{Key: "net.ipv4.conf.<iface>.rp_filter", Value: "1"},
		},
	})
	t.Cleanup(func() { sysctlreg.DeregisterProfile(profileName) })

	t.Run("applySysctl emits WARN and report issue", func(t *testing.T) {
		report.ResetForTest()
		t.Cleanup(report.ResetForTest)

		forwarding := true
		out := logFrom(t, func() {
			applySysctl("eth0", unitEntry{IPv4: &ipv4Settings{Forwarding: &forwarding}})
		})

		require.Contains(t, out, "level=WARN")
		require.Contains(t, out, "code="+codeDoctorIfaceSysctlEmit)
		require.Contains(t, out, "net.ipv4.conf.eth0.forwarding")
		require.Contains(t, out, "ze show sysctl")

		issue := findReportErrorByCode(codeDoctorIfaceSysctlEmit)
		require.NotNil(t, issue)
		require.Contains(t, issue.Message, "net.ipv4.conf.eth0.forwarding")
		require.Contains(t, issue.Fix, "ze show sysctl")
	})

	t.Run("applySysctlProfiles reports clear and setting emit failures", func(t *testing.T) {
		report.ResetForTest()
		t.Cleanup(report.ResetForTest)

		out := logFrom(t, func() {
			applySysctlProfiles("eth0", []string{profileName})
		})

		require.Contains(t, out, "level=WARN")
		require.Contains(t, out, "code="+codeDoctorIfaceSysctlEmit)
		require.Contains(t, out, "clear-profile-defaults")
		require.Contains(t, out, "net.ipv4.conf.eth0.rp_filter")

		var count int
		for _, issue := range report.Errors(0) {
			if issue.Code == codeDoctorIfaceSysctlEmit {
				count++
			}
		}
		require.Equal(t, 2, count, "expected report-bus events for both clear-profile-defaults and profile setting")
	})
}

// TestEmitInterfaceRollbackReportsEmitFailure verifies that a failed
// interface:rollback EventBus emit warns with codeDoctorIfaceRollbackEmit and
// raises on the report bus.
//
// VALIDATES: AC-4 -- rollback emit failure warns and raises on report bus.
// PREVENTS: silently dropping interface:rollback notifications at Debug.
func TestEmitInterfaceRollbackReportsEmitFailure(t *testing.T) {
	require.NotNil(t, diagnostic.Lookup(codeDoctorIfaceRollbackEmit))
	require.True(t, slices.Contains(diagnostic.EventCodes(), codeDoctorIfaceRollbackEmit))

	report.ResetForTest()
	t.Cleanup(report.ResetForTest)

	var out bytes.Buffer
	log := slog.New(slog.NewTextHandler(&out, &slog.HandlerOptions{Level: slog.LevelWarn}))
	emitErr := errors.New("rollback bus closed")

	emitInterfaceRollback(&failingIfaceBus{err: emitErr}, log)

	require.Contains(t, out.String(), "level=WARN")
	require.Contains(t, out.String(), "code="+codeDoctorIfaceRollbackEmit)
	require.Contains(t, out.String(), ifaceevents.EventRollback)
	require.Contains(t, out.String(), "ze request reload")

	issue := findReportErrorByCode(codeDoctorIfaceRollbackEmit)
	require.NotNil(t, issue)
	require.Contains(t, issue.Message, emitErr.Error())
	require.Contains(t, issue.Fix, "ze request reload")
}

// TestCleanupStaleIPv6DefaultRoutesReportsListFailure verifies that a
// ListRoutes failure during stale IPv6 default route cleanup logs WARN with
// codeDoctorIfaceStaleRouteCleanup and raises on the report bus.
//
// VALIDATES: AC-4 -- stale IPv6 default-route list failure warns and raises.
// PREVENTS: silently leaving pre-existing kernel ::/0 routes uninspected at Debug.
func TestCleanupStaleIPv6DefaultRoutesReportsListFailure(t *testing.T) {
	require.NotNil(t, diagnostic.Lookup(codeDoctorIfaceStaleRouteCleanup))
	require.True(t, slices.Contains(diagnostic.EventCodes(), codeDoctorIfaceStaleRouteCleanup))

	report.ResetForTest()
	t.Cleanup(report.ResetForTest)

	listErr := errors.New("netlink route dump refused")
	be := &droppedErrBackend{listRoutesErr: listErr}
	backendName := "test-" + t.Name()
	require.NoError(t, RegisterBackend(backendName, func() (Backend, error) { return be, nil }))
	require.NoError(t, LoadBackend(backendName))
	t.Cleanup(func() { _ = CloseBackend() })

	var out bytes.Buffer
	log := slog.New(slog.NewTextHandler(&out, &slog.HandlerOptions{Level: slog.LevelWarn}))

	cleanupStaleIPv6DefaultRoutes("eth0", log)

	require.Contains(t, out.String(), "level=WARN")
	require.Contains(t, out.String(), "code="+codeDoctorIfaceStaleRouteCleanup)
	require.Contains(t, out.String(), "eth0")
	require.Contains(t, out.String(), "ip -6 route show default dev eth0")

	issue := findReportErrorByCode(codeDoctorIfaceStaleRouteCleanup)
	require.NotNil(t, issue)
	require.Contains(t, issue.Message, "eth0")
	require.Contains(t, issue.Message, listErr.Error())
	require.Contains(t, issue.Fix, "ip -6 route show default dev eth0")
}
