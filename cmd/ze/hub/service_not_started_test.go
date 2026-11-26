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
	// VALIDATES: the runtime phase holds the check that declares
	// doctor-service-not-started, so `show doctor` runs it and the log lines
	// under that code name a code a registered check declares.
	// PREVENTS: a code that is logged and that no doctor check reports.
	for _, check := range diagnostic.DoctorChecksForPhase(diagnostic.DoctorPhaseRuntime) {
		if check.Name != "hub-services-not-started" {
			continue
		}
		require.Equal(t, []string{codeDoctorServiceNotStarted}, check.Codes)
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
