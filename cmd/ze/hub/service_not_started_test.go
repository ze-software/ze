// Design: docs/architecture/core-design.md -- a service whose configuration cannot be served is not started, and never stops ze

package hub

import (
	"bytes"
	"errors"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"

	zeconfig "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/env"
)

// errBindRefused stands for the error a listener answers when it cannot bind.
var errBindRefused = errors.New("listen tcp 192.0.2.1:8081: bind: cannot assign requested address")

func TestReportServiceNotStartedLogsPrintsAndFeedsDoctor(t *testing.T) {
	// VALIDATES: a service the start leaves down is reported three ways from
	// one call: an error line on the log under doctor-service-not-started with
	// the service and the fix, the same problem on the terminal with the words
	// that say ze itself runs, and one error finding of the runtime doctor
	// check with the fix on its own field and the cause as what was found.
	// PREVENTS: a service left down with no log line under a doctor code, or
	// with no `ze doctor` finding, and a finding with no fix.
	resetServicesNotStarted()
	t.Cleanup(resetServicesNotStarted)
	require.Empty(t, checkServicesNotStarted(diagnostic.DoctorCheckContext{}), "a start that left nothing down reports nothing")

	var logged, console bytes.Buffer
	log := slog.New(slog.NewTextHandler(&logged, nil))
	_, parseErr := zeconfig.ParseCompoundListen("not-a-listen-address")
	require.Error(t, parseErr)

	reportServiceNotStarted(log, &console,
		notStarted(serviceNameWeb, listenUnreadable(serviceNameWeb, "ze.web.listen", "not-a-listen-address", "0.0.0.0:3443", parseErr)))

	for _, want := range []string{
		"level=ERROR",
		"code=" + codeDoctorServiceNotStarted,
		`service="web server"`,
		"the web server is not started; ze and its other services keep running",
		"ze.web.listen",
		"not-a-listen-address",
	} {
		require.Contains(t, logged.String(), want)
	}
	for _, want := range []string{
		"error: the web server is not started; ze and its other services keep running: ",
		"ze.web.listen holds \"not-a-listen-address\"",
		"Set ze.web.listen to endpoints such as 0.0.0.0:3443, or unset it, then restart ze.",
	} {
		require.Contains(t, console.String(), want)
	}

	diags := checkServicesNotStarted(diagnostic.DoctorCheckContext{})
	require.Len(t, diags, 1)
	require.Equal(t, codeDoctorServiceNotStarted, diags[0].Code)
	require.Equal(t, diagnostic.SeverityError, diags[0].Severity)
	require.Contains(t, diags[0].Message, "The web server cannot read the addresses it binds")
	require.Contains(t, diags[0].Message, "the web server is not started")
	require.Equal(t, "Set ze.web.listen to endpoints such as 0.0.0.0:3443, or unset it, then restart ze.", diags[0].Fix)
	require.Equal(t, parseErr.Error(), diags[0].Actual, "the finding keeps the parser's answer as its evidence")

	t.Run("each service left down is its own finding, in the order decided", func(t *testing.T) {
		reportServiceNotStarted(log, &console,
			notStarted(serviceNameREST, buildFailed(serviceNameREST, "environment api-server rest", errBindRefused)))
		diags := checkServicesNotStarted(diagnostic.DoctorCheckContext{})
		require.Len(t, diags, 2)
		require.Contains(t, diags[1].Message, "The REST API could not be started.")
		require.Equal(t, errBindRefused.Error(), diags[1].Actual)
		require.NotEmpty(t, diags[1].Fix)
	})

	t.Run("a new start reports only its own decisions", func(t *testing.T) {
		resetServicesNotStarted()
		require.Empty(t, checkServicesNotStarted(diagnostic.DoctorCheckContext{}))
	})
}

func TestServicesNotStartedCheckIsRegisteredWithItsCode(t *testing.T) {
	// VALIDATES: the runtime phase holds the check that declares both
	// doctor-service-not-started and doctor-dataplane-mismatch, so
	// `show doctor` runs it and the log lines under those codes name codes a
	// registered check declares.
	// PREVENTS: a code that is logged and that no doctor check reports.
	for _, check := range diagnostic.DoctorChecksForPhase(diagnostic.DoctorPhaseRuntime) {
		if check.Name != "hub-services-not-started" {
			continue
		}
		require.Equal(t, []string{codeDoctorServiceNotStarted, zeconfig.CodeDoctorDataPlaneMismatch}, check.Codes)
		return
	}
	t.Fatal("the runtime phase holds no hub-services-not-started check")
}

func TestPlanAPIListenersLeavesOnlyTheUnreadableTransportDown(t *testing.T) {
	// VALIDATES: at start, a REST listen variable that does not parse leaves
	// REST off with no address and named with its fix, while gRPC resolves
	// from the same config and ok still says a transport binds; the strict
	// form a reload uses refuses the same input as a whole.
	// PREVENTS: one transport's variable stopping ze or taking the other
	// transport down, REST bound on the block's addresses in place of the
	// variable that outranks them, and a reload that accepts such a config.
	clearAPIEnv(t)
	require.NoError(t, env.Set("ze.api-server.rest.enabled", "1"))
	require.NoError(t, env.Set("ze.api-server.grpc.enabled", "1"))
	require.NoError(t, env.Set("ze.api-server.rest.listen", "no-port-here"))

	cfg, ok, down := planAPIListeners(dormantAPIBlock(t, true))

	require.True(t, ok, "gRPC still binds")
	require.False(t, cfg.RESTOn)
	require.Empty(t, cfg.REST, "REST is not bound on the address the block names")
	require.True(t, cfg.GRPCOn)
	require.Len(t, cfg.GRPC, 1)
	require.Equal(t, "127.0.0.1:50052", cfg.GRPC[0].Listen())
	require.Equal(t, "api-s3cret", cfg.Token, "the settings the block names still apply to the transport that serves")
	require.Equal(t, []string{"127.0.0.1:50052"}, apiGuardAddrs(cfg), "the guard sees only the transport that binds")

	require.Len(t, down, 1)
	require.Equal(t, serviceNameREST, down[0].service)
	for _, want := range []string{"ze.api-server.rest.listen", "no-port-here", "0.0.0.0:8081", "restart ze"} {
		require.Contains(t, down[0].problem.Error(), want)
	}

	_, strictOK, strictErr := resolveAPIListeners(dormantAPIBlock(t, true))
	require.Error(t, strictErr, "a reload refuses the config as a whole")
	require.False(t, strictOK)

	t.Run("both variables unreadable leaves both down and nothing binds", func(t *testing.T) {
		require.NoError(t, env.Set("ze.api-server.grpc.listen", "also-no-port"))
		cfg, ok, down := planAPIListeners(dormantAPIBlock(t, true))
		require.False(t, ok)
		require.Empty(t, apiGuardAddrs(cfg))
		require.Len(t, down, 2)
		require.Equal(t, serviceNameGRPC, down[1].service)
	})
}

func TestLGRangesUnusedKeepsTheListsOwnFix(t *testing.T) {
	// VALIDATES: a next-hop range list that cannot be used is reported as the
	// looking glass serving without the names, never as the looking glass
	// down, and the report carries the fix the list's own refusal names.
	// PREVENTS: an invalid name list taking the looking glass down, and a
	// report whose fix is less exact than the refusal it wraps.
	tree := zeconfig.NewTree()
	lg := tree.GetOrCreateContainer("environment").GetOrCreateContainer("looking-glass")
	entry := zeconfig.NewTree()
	entry.Set("name", "transit")
	lg.AddListEntry("next-hop-range", "not-a-prefix", entry)
	_, refusal := zeconfig.ExtractLGNextHopRanges(tree)
	require.Error(t, refusal)

	down := lgRangesUnused(refusal)

	require.Equal(t, "the looking glass serves without next-hop range names; ze and its other services keep running", down.effect)
	require.ErrorIs(t, down.problem, refusal)
	require.Contains(t, down.problem.Fix(), "next-hop-range not-a-prefix")
	require.Contains(t, down.problem.Fix(), "then restart ze.")
}

func TestConfigBlockDisplay(t *testing.T) {
	// VALIDATES: configBlockDisplay converts internal slash/bracket config tree
	// paths into the space-separated block syntax operators write in commands
	// and configuration files, with no leading, trailing, or doubled spaces.
	// PREVENTS: raw tree paths like "bus/hub/server[main]" leaking into
	// operator-facing error messages and doctor findings.
	cases := []struct {
		path string
		want string
	}{
		{path: "environment/web", want: "environment web"},
		{path: "/environment/web", want: "environment web"},
		{path: "bus/hub/server[main]", want: "bus hub server main"},
		{path: "bus/hub/server[main]/tls", want: "bus hub server main tls"},
		{path: "bgp/rpki/cache-server[rtr1]", want: "bgp rpki cache-server rtr1"},
		{path: "environment/looking-glass/next-hop-range", want: "environment looking-glass next-hop-range"},
		{path: "l2tp", want: "l2tp"},
		{path: "", want: ""},
	}
	for _, tc := range cases {
		require.Equal(t, tc.want, configBlockDisplay(tc.path), "path %q", tc.path)
	}
}

func TestReportExcludedBlockLogsPrintsAndFeedsDoctor(t *testing.T) {
	// VALIDATES: an ExcludedBlock returned by LoadConfigForStart is reported
	// through reportServiceNotStarted with operator-readable block syntax, the
	// original validation cause, doctor-service-not-started on the log, console
	// stderr text stating ze keeps running, and a runtime doctor finding whose
	// fix names `ze config validate <path>`.
	// PREVENTS: a service block dropped at start staying silent in logs or
	// missing from `ze doctor`.
	resetServicesNotStarted()
	t.Cleanup(resetServicesNotStarted)

	excl := zeconfig.ExcludedBlock{
		Path:    "environment/web",
		Service: serviceNameWeb,
		Cause:   errors.New(`environment/web/certificate: certificate "no-such-cert" is not defined`),
	}
	const cfgPath = "/etc/ze/ze.conf"

	var logged, console bytes.Buffer
	log := slog.New(slog.NewTextHandler(&logged, nil))
	reportServiceNotStarted(log, &console,
		notStarted(excl.Service, blockUnservable(excl.Service, configBlockDisplay(excl.Path), cfgPath, excl.Cause)))

	for _, want := range []string{
		"level=ERROR",
		"code=" + codeDoctorServiceNotStarted,
		`service="web server"`,
		"the web server is not started; ze and its other services keep running",
		"environment web",
		"no-such-cert",
	} {
		require.Contains(t, logged.String(), want)
	}
	for _, want := range []string{
		"error: the web server is not started; ze and its other services keep running: ",
		"`environment web` cannot be served as written",
		"Correct `environment web` as `ze config validate " + cfgPath + "` reports, then restart ze.",
		"no-such-cert",
	} {
		require.Contains(t, console.String(), want)
	}

	diags := checkServicesNotStarted(diagnostic.DoctorCheckContext{})
	require.Len(t, diags, 1)
	require.Equal(t, codeDoctorServiceNotStarted, diags[0].Code)
	require.Equal(t, diagnostic.SeverityError, diags[0].Severity)
	require.Contains(t, diags[0].Message, "`environment web` cannot be served as written")
	require.Contains(t, diags[0].Message, "the web server is not started")
	require.Equal(t, "Correct `environment web` as `ze config validate "+cfgPath+"` reports, then restart ze.", diags[0].Fix)
	require.Equal(t, excl.Cause.Error(), diags[0].Actual)
}

func TestDataPlaneExcludedLogsPrintsAndFeedsDoctor(t *testing.T) {
	// VALIDATES: an ExcludedDataPlane record returned by LoadConfigForStart is
	// reported under doctor-dataplane-mismatch on the log, on the terminal
	// with the effect stating the feature is left off while ze keeps running,
	// and as a runtime doctor error finding with the exact fix.
	// PREVENTS: a data-plane-mismatched feature left off at start staying
	// silent in logs, logged under the wrong doctor code, or missing from
	// `ze doctor` / `show doctor`.
	resetServicesNotStarted()
	t.Cleanup(resetServicesNotStarted)

	excl := zeconfig.ExcludedDataPlane{
		Problem: `firewall uses firewall backend "kernel", and interface backend is "vpp"`,
		Why:     `interface backend "vpp" forwards the packets, so firewall on "kernel" would act on a different data plane`,
		Fix:     "set firewall backend vpp",
	}

	var logged, console bytes.Buffer
	log := slog.New(slog.NewTextHandler(&logged, nil))
	reportServiceNotStarted(log, &console, dataPlaneExcluded(excl))

	for _, want := range []string{
		"level=ERROR",
		"code=" + zeconfig.CodeDoctorDataPlaneMismatch,
		`service="data-plane feature"`,
		"that feature is left off; ze and its other services keep running",
		"set firewall backend vpp",
	} {
		require.Contains(t, logged.String(), want)
	}
	for _, want := range []string{
		"error: that feature is left off; ze and its other services keep running: ",
		excl.Problem,
		excl.Why,
		excl.Fix,
	} {
		require.Contains(t, console.String(), want)
	}

	diags := checkServicesNotStarted(diagnostic.DoctorCheckContext{})
	require.Len(t, diags, 1)
	require.Equal(t, zeconfig.CodeDoctorDataPlaneMismatch, diags[0].Code)
	require.Equal(t, diagnostic.SeverityError, diags[0].Severity)
	require.Contains(t, diags[0].Message, excl.Problem)
	require.Contains(t, diags[0].Message, excl.Why)
	require.Contains(t, diags[0].Message, "that feature is left off")
	require.Equal(t, excl.Fix, diags[0].Fix)
	require.Empty(t, diags[0].Actual)
}
