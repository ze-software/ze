// Design: docs/architecture/core-design.md -- a service whose configuration cannot be served is not started, and never stops ze

package hub

import (
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	zeconfig "github.com/ze-software/ze/internal/component/config"
	zepki "github.com/ze-software/ze/internal/component/pki"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/env"
)

// errNameUnresolved stands for the error the pki store answers for a name it
// does not hold.
var errNameUnresolved = errors.New("pki: certificate typo-cert is not defined")

func TestPlanLGEndpointsLeavesUnservableEntriesDown(t *testing.T) {
	// VALIDATES: the one rule that decides each looking-glass entry keeps the
	// entries it can serve when one entry cannot be served: a `tls true` entry
	// with no certificate to present is unservable and its `tls false` sibling
	// keeps its binding, and a named certificate that cannot be presented makes
	// every entry that asks for TLS unservable while `tls false` still serves.
	// PREVENTS: one entry's missing certificate taking the whole looking glass
	// down, and a named certificate replaced by the self-signed pair or by
	// plain HTTP.
	const (
		public = "0.0.0.0:80"
		secure = "0.0.0.0:8443"
		strict = "0.0.0.0:9443"
	)
	off := lgListener{addr: public, tls: zeconfig.LGServerTLSOff}
	inherited := lgListener{addr: secure, tls: zeconfig.LGServerTLSInherited}
	required := lgListener{addr: strict, tls: zeconfig.LGServerTLSRequired}

	t.Run("tls true with nothing to present stays down, its sibling serves", func(t *testing.T) {
		plan := planLGEndpoints([]lgListener{off, required}, "", nil, false)
		require.Equal(t, []lgBinding{{addr: public, tls: false}}, plan.bindings)
		require.Len(t, plan.unservable, 1)
		require.Equal(t, strict, plan.unservable[0].addr)
		require.Empty(t, plan.downgraded)
		for _, want := range []string{strict, "tls true", "ze init", "tls false"} {
			require.Contains(t, plan.unservable[0].problem.Error(), want, "the entry is reported with what, why and the fix")
		}
		require.NotContains(t, plan.unservable[0].problem.Error(), public, "the report names the entry that is down, not its sibling")
	})

	t.Run("a named certificate that cannot be presented takes down every TLS entry", func(t *testing.T) {
		// hasStore is set: the self-signed pair is available, and is never used
		// in place of the named certificate.
		plan := planLGEndpoints([]lgListener{off, inherited, required}, "typo-cert", errNameUnresolved, true)
		require.Equal(t, []lgBinding{{addr: public, tls: false}}, plan.bindings)
		require.False(t, plan.servesTLS())
		require.Empty(t, plan.downgraded, "an entry that left tls unset is not served on HTTP in place of a named certificate")
		require.Len(t, plan.unservable, 2)
		require.Equal(t, secure, plan.unservable[0].addr)
		require.Equal(t, strict, plan.unservable[1].addr)
		for _, down := range plan.unservable {
			require.ErrorIs(t, down.problem, errNameUnresolved, "the report keeps the store's answer as its evidence")
			for _, want := range []string{down.addr, "typo-cert", "edit pki certificate typo-cert", "environment looking-glass certificate"} {
				require.Contains(t, down.problem.Error(), want)
			}
		}
	})

	t.Run("a named certificate that resolves serves every TLS entry", func(t *testing.T) {
		plan := planLGEndpoints([]lgListener{off, inherited, required}, "lan", nil, false)
		require.Empty(t, plan.unservable)
		require.NoError(t, plan.refusal())
		require.Equal(t, map[string]bool{public: false, secure: true, strict: true}, plan.tlsByAddr())
	})

	t.Run("a reload is refused over the entry a start leaves down, with the same words", func(t *testing.T) {
		plan := planLGEndpoints([]lgListener{off, required}, "", nil, false)
		strictPlan, refusal := planLGTransports([]lgListener{off, required}, "", false)
		require.Error(t, refusal)
		require.Empty(t, strictPlan.bindings)
		require.Equal(t, plan.unservable[0].problem.Error(), refusal.Error())
		require.Equal(t, plan.refusal().Error(), refusal.Error())
	})
}

func TestReportLGServerDown(t *testing.T) {
	// VALIDATES: a looking-glass entry a start leaves down is reported on the
	// log at error level under doctor-lg-tls-unservable, with its address, the
	// fix and the statement that ze keeps running, and the same on the terminal.
	// PREVENTS: an entry that is silently absent, and a log line no doctor code
	// explains.
	var logged, console strings.Builder
	log := slog.New(slog.NewTextHandler(&logged, nil))
	const addr = "127.0.0.1:8443"

	reportLGServerDown(log, &console, lgUnservable{addr: addr, problem: lgTLSNeedsCertificate(addr)})

	for _, want := range []string{"level=ERROR", "code=" + codeDoctorLGTLSUnservable, addr, "ze init", "tls false", "is not started", "keep running"} {
		require.Contains(t, logged.String(), want)
	}
	for _, want := range []string{"error: ", addr, "is not started", "keep running", "fix: ", "ze init"} {
		require.Contains(t, console.String(), want)
	}
}

func TestReportLGNotStarted(t *testing.T) {
	// VALIDATES: a looking glass whose ze.looking-glass.listen does not parse
	// is reported under doctor-lg-listen-unreadable with the value, the fix and
	// the statement that ze keeps running, and that the config it leaves
	// describes no listener, so no looking glass is built.
	// PREVENTS: a daemon that exits over the address list of one service.
	require.NoError(t, env.Set("ze.looking-glass.listen", "not-an-endpoint"))
	t.Cleanup(func() { _ = env.Set("ze.looking-glass.listen", "") })

	cfg, err := lgListenerConfigFrom(nil)
	require.Error(t, err)
	require.Empty(t, cfg.listeners, "an unreadable address list describes no listener to build")

	var logged, console strings.Builder
	reportLGNotStarted(slog.New(slog.NewTextHandler(&logged, nil)), &console, err)
	for _, want := range []string{"level=ERROR", "code=" + codeDoctorLGListenUnreadable, "not-an-endpoint", "is not started", "keep running"} {
		require.Contains(t, logged.String(), want)
	}
	for _, want := range []string{"error: ", "not-an-endpoint", "is not started", "keep running", "ze.looking-glass.listen"} {
		require.Contains(t, console.String(), want)
	}
}

func TestWebCertificateProblem(t *testing.T) {
	// VALIDATES: the start gate of the web server: a name the pki store holds
	// is no problem, and a name it does not hold is an operator error that
	// names the certificate, the leaf and the fix; and the report of a web
	// server that is not started carries doctor-tls-reference, the fix and the
	// statement that ze keeps running.
	// PREVENTS: a daemon that exits over the web certificate name, which leaves
	// the operator nothing to reach it and correct the name with.
	loadTwoCertStore(t)

	problem, unpresentable := webCertificateProblem("web-cert")
	require.False(t, unpresentable, "a name the store holds is presented")
	require.Nil(t, problem)

	problem, unpresentable = webCertificateProblem("typo-cert")
	require.True(t, unpresentable)
	for _, want := range []string{"web server", "typo-cert", "environment web certificate", "edit pki certificate typo-cert", "restart ze"} {
		require.Contains(t, problem.Error(), want)
	}
	require.Error(t, problem.Unwrap(), "the store's answer stays as the evidence")

	var logged, console strings.Builder
	reportWebNotStarted(slog.New(slog.NewTextHandler(&logged, nil)), &console, problem)
	for _, want := range []string{"level=ERROR", "code=" + diagnostic.CodeDoctorTLSReference, "typo-cert", "edit pki certificate typo-cert", "is not started", "keep running"} {
		require.Contains(t, logged.String(), want)
	}
	for _, want := range []string{"error: ", "the web server is not started", "keep running", "typo-cert", "fix: "} {
		require.Contains(t, console.String(), want)
	}
}

func TestCheckLGTransportWithoutRunningLookingGlass(t *testing.T) {
	// VALIDATES: the reload gate refuses a `tls true` entry with no certificate
	// to present also when no looking glass runs, which is the state a start
	// leaves when it could serve no entry: the gate reads the store the daemon
	// was started with. A daemon with a store accepts the same config.
	// PREVENTS: a reload of the config a start could not serve being accepted
	// in silence because the looking glass it would refuse for is down.
	cfg := lgListenerConfig{listeners: []lgListener{{addr: fakeListenAddr, tls: zeconfig.LGServerTLSRequired}}}

	storeless := newListenerMigrator()
	storeless.setDaemonStore(false)
	err := storeless.checkLGTransport(cfg)
	require.Error(t, err, "no looking glass runs, and the reload is still refused")
	for _, want := range []string{fakeListenAddr, "tls true", "ze init"} {
		require.Contains(t, err.Error(), want)
	}

	stored := newListenerMigrator()
	stored.setDaemonStore(true)
	require.NoError(t, stored.checkLGTransport(cfg), "a store keeps the self-signed pair, so the entry is servable")

	require.NoError(t, storeless.checkLGTransport(lgListenerConfig{
		listeners: []lgListener{{addr: fakeListenAddr, tls: zeconfig.LGServerTLSOff}},
	}), "a config every entry of which is servable is accepted")
}

func TestRestoreLGTransportKeepsUnservableEntriesDown(t *testing.T) {
	// VALIDATES: a rollback on a daemon that started with one looking-glass
	// entry down returns the entries that serve to their protocol and leaves
	// the down entry down, where a reload that asks for the same config is
	// refused and changes nothing.
	// PREVENTS: every rolled-back reload failing its restore on a daemon that
	// started with an unservable entry.
	t.Cleanup(func() { _ = zepki.Load(nil) })
	require.NoError(t, zepki.Load(nil))
	const public = "127.0.0.1:3080"
	cfg := lgListenerConfig{listeners: []lgListener{
		{addr: public, tls: zeconfig.LGServerTLSOff},
		{addr: fakeListenAddr, tls: zeconfig.LGServerTLSRequired},
	}}

	t.Run("the restore applies the served entries", func(t *testing.T) {
		fake := &fakeTLSUpdatable{listenerTLS: map[string]bool{public: false}}
		lm := newListenerMigrator()
		lm.setLGTransport(fake)

		require.NoError(t, lm.restoreLGTransport(cfg))
		require.Zero(t, fake.calls, "no served entry presents a certificate")
		require.Equal(t, map[string]bool{public: false}, fake.requestedTLS)
	})

	t.Run("a reload of the same config is refused", func(t *testing.T) {
		fake := &fakeTLSUpdatable{listenerTLS: map[string]bool{public: false}}
		lm := newListenerMigrator()
		lm.setLGTransport(fake)

		require.Error(t, lm.checkLGTransport(cfg))
		require.Error(t, lm.updateLGTransport(cfg))
		require.Nil(t, fake.requestedTLS, "a refused reload changes no listener")
	})

	t.Run("a named certificate that does not resolve leaves the TLS entries down", func(t *testing.T) {
		fake := &fakeTLSUpdatable{listenerTLS: map[string]bool{public: false}}
		lm := newListenerMigrator()
		lm.setLGTransport(fake)

		named := lgListenerConfig{listeners: cfg.listeners, certName: "typo-cert"}
		require.NoError(t, lm.restoreLGTransport(named))
		require.Zero(t, fake.calls, "a name that does not resolve installs nothing")
		require.Equal(t, map[string]bool{public: false}, fake.requestedTLS)
	})
}

func TestLGListenerDiagnosticsReportEveryDownEntry(t *testing.T) {
	// VALIDATES: `ze doctor` names each looking-glass entry a start would
	// leave down, with its own address and the fix, and stays silent on the
	// `tls false` entry that serves.
	// PREVENTS: doctor reporting the first unservable entry only, so that the
	// operator fixes one and finds the next at the following start.
	servers := map[string]any{
		"public": map[string]any{"ip": "127.0.0.1", "port": "3080", "tls": "false"},
		"first":  map[string]any{"ip": "127.0.0.1", "port": "3443", "tls": "true"},
		"second": map[string]any{"ip": "127.0.0.1", "port": "3444", "tls": "true"},
	}
	diags := lgListenerDiagnostics(lgEntriesTree(map[string]any{"enabled": "true"}, servers), false)

	require.Len(t, diags, 2)
	reported := make([]string, 0, len(diags))
	for _, diag := range diags {
		require.Equal(t, codeDoctorLGTLSUnservable, diag.Code)
		require.Equal(t, diagnostic.SeverityError, diag.Severity)
		require.Contains(t, diag.Fix, "ze init")
		require.NotContains(t, diag.Message, "127.0.0.1:3080")
		reported = append(reported, diag.Message)
	}
	joined := strings.Join(reported, "\n")
	require.Contains(t, joined, "127.0.0.1:3443")
	require.Contains(t, joined, "127.0.0.1:3444")
}
