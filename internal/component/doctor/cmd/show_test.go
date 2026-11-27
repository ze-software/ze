package cmd

import (
	"context"
	"encoding/json"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/commandtest"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/doctor"
	"github.com/ze-software/ze/internal/component/host"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func TestPlatformMatchesEmpty(t *testing.T) {
	t.Parallel()
	assert.True(t, platformMatches(nil, nil), "nil platforms = match any")
	assert.True(t, platformMatches([]string{}, nil), "empty platforms = match any")
}

func TestPlatformMatchesAny(t *testing.T) {
	t.Parallel()
	assert.True(t, platformMatches([]string{"any"}, nil))
}

func TestPlatformMatchesSpecific(t *testing.T) {
	t.Parallel()
	info := &host.PlatformInfo{Type: host.PlatformDarwin}
	assert.True(t, platformMatches([]string{"darwin"}, info))
	assert.False(t, platformMatches([]string{"gokrazy"}, info))
}

func TestPlatformMatchesNilInfo(t *testing.T) {
	t.Parallel()
	assert.False(t, platformMatches([]string{"darwin"}, nil))
}

func TestNormalizeSeverity(t *testing.T) {
	t.Parallel()
	assert.Equal(t, diagnostic.SeverityError, normalizeSeverity("error"))
	assert.Equal(t, diagnostic.SeverityWarning, normalizeSeverity("warning"))
	assert.Equal(t, diagnostic.SeverityInfo, normalizeSeverity("info"), "a plugin's info finding stays info")
	assert.Equal(t, diagnostic.SeverityWarning, normalizeSeverity("unknown"))
	assert.Equal(t, diagnostic.SeverityWarning, normalizeSeverity(""))
}

// TestDoctorOneProducer proves AC-14 for doctor: `ze doctor` (the local-data
// handler registered at doctor), and the daemon's doctor and show doctor (one
// RPC handler, HandleDoctor) answer through one producer.
//
// Method: run the registered local handler on the local route and HandleDoctor
// on the daemon route with the same words, and compare the encoded payloads.
// The kernel-capabilities mode is used because its probe answers the same on
// two calls, where the readiness checks report live disk and descriptor
// figures. A refused word must be refused alike on both routes, and the daemon
// error must be the local refusal's first line minus its "error: " prefix.
func TestDoctorOneProducer(t *testing.T) {
	handler, rest := command.LookupLocalData([]string{"doctor"})
	require.NotNil(t, handler, "doctor is not registered as a local-data command")
	require.Empty(t, rest)

	local := command.AnswerLocal(handler, commandtest.Args("kernel-capabilities"))
	require.Equal(t, 0, local.Code, "local diagnostic: %s", local.Diagnostic)
	want, err := json.Marshal(local.Payload)
	require.NoError(t, err)

	resp, err := HandleDoctor(&pluginserver.CommandContext{}, commandtest.Args("kernel-capabilities"))
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, resp.Status)
	got, ok := resp.Data.(plugin.RawJSON)
	require.True(t, ok, "daemon data is %T, want the encoded producer answer", resp.Data)
	assert.JSONEq(t, string(want), string(got), "daemon and local answers differ")
	assert.Contains(t, string(got), `"capabilities"`)

	refusedLocal := command.AnswerLocal(handler, commandtest.Args("bogus"))
	require.Equal(t, 1, refusedLocal.Code)
	firstLine, _, _ := strings.Cut(refusedLocal.Diagnostic, "\n")
	_, err = HandleDoctor(&pluginserver.CommandContext{}, commandtest.Args("bogus"))
	require.Error(t, err)
	reason := strings.TrimPrefix(firstLine, "error: ")
	assert.True(t, strings.HasPrefix(err.Error(), reason), "daemon refusal %q does not open with the local reason %q", err.Error(), reason)
}

// TestDoctorDaemonRouteCarriesPluginChecks proves the daemon form adds the
// plugin checks the producer is handed, and the local route adds none.
//
// Method: call the producer with a runtime source holding one error, then
// with none, over a config file that does not exist (both report the missing
// config the same way), and compare what each answers.
func TestDoctorDaemonRouteCarriesPluginChecks(t *testing.T) {
	runtime := func(*config.Tree) []diagnostic.Diagnostic {
		return []diagnostic.Diagnostic{{Code: "fixture-plugin-check", Severity: diagnostic.SeverityError, Message: "plugin fixture is unhappy"}}
	}
	withChecks := doctor.Answer(commandtest.Args(), runtime)
	encoded, err := json.Marshal(withChecks.Payload)
	require.NoError(t, err)
	assert.Contains(t, string(encoded), `"fixture-plugin-check"`)
	assert.Contains(t, string(encoded), `"ready":false`)

	without := doctor.Answer(commandtest.Args(), nil)
	encoded, err = json.Marshal(without.Payload)
	require.NoError(t, err)
	assert.NotContains(t, string(encoded), `"fixture-plugin-check"`)
}

// fakeDoctorCheckSource is a running-plugin set for collectDoctorChecksFrom:
// one plugin declaring one check, answering a fixed diagnostic.
type fakeDoctorCheckSource struct {
	answer rpc.DoctorCheckOutput
}

func (fakeDoctorCheckSource) DoctorCheckPlugins() map[string][]plugin.DoctorCheckRegistration {
	return map[string][]plugin.DoctorCheckRegistration{
		"fixture-plugin": {{Name: "fixture-check", Phase: rpc.DoctorPhaseRuntime, Platforms: []string{"any"}}},
	}
}

func (f fakeDoctorCheckSource) RunDoctorChecks(requests []pluginserver.DoctorCheckPluginRequest, _, _ time.Duration) []pluginserver.DoctorCheckPluginResult {
	results := make([]pluginserver.DoctorCheckPluginResult, len(requests))
	for i, req := range requests {
		outcomes := make([]pluginserver.DoctorCheckOutcome, len(req.Checks))
		for j, check := range req.Checks {
			answer := f.answer
			outcomes[j] = pluginserver.DoctorCheckOutcome{Check: check, Output: &answer}
		}
		results[i] = pluginserver.DoctorCheckPluginResult{Plugin: req.Plugin, Outcomes: outcomes}
	}
	return results
}

// TestCollectPluginDoctorChecksCarriesFix asks a running plugin for its
// doctor check and reads what the daemon route builds from the answer.
//
// VALIDATES: AC-3 on the daemon route, the fix a plugin's doctor-check answer
// carries reaches the doctor diagnostic unchanged.
// PREVENTS: the callback bridge rebuilding a diagnostic from code, severity
// and message alone and dropping the fix command.
func TestCollectPluginDoctorChecksCarriesFix(t *testing.T) {
	source := fakeDoctorCheckSource{answer: rpc.DoctorCheckOutput{Diagnostics: []rpc.DoctorCheckDiagnostic{{
		Code: "doctor-fixture-degraded", Severity: "error", Message: "fixture state is degraded", Fix: "show fixture state",
	}}}}
	diags := collectDoctorChecksFrom(source, "").diagnostics
	require.Len(t, diags, 1)
	assert.Equal(t, "doctor-fixture-degraded", diags[0].Code)
	assert.Equal(t, diagnostic.SeverityError, diags[0].Severity)
	assert.Equal(t, "fixture state is degraded", diags[0].Message)
	assert.Equal(t, "show fixture state", diags[0].Fix)
}

// TestDoctorRuntimePhaseRunsOnDaemonRoute registers a runtime-phase check and
// runs show doctor's handler.
//
// VALIDATES: AC-1, the daemon route runs a registered runtime check and its
// diagnostic reaches the answer with code, severity, message and fix.
// PREVENTS: a runtime check that no route runs.
func TestDoctorRuntimePhaseRunsOnDaemonRoute(t *testing.T) {
	const name = "test-daemon-runtime"
	check := diagnostic.DoctorCheck{
		Name: name, Phase: diagnostic.DoctorPhaseRuntime, Component: "test",
		Dependencies: []string{"x"}, Platforms: []string{diagnostic.DoctorPlatformAny},
		Codes: []string{"doctor-test-daemon-runtime"},
		Check: func(diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
			return []diagnostic.Diagnostic{{
				Code: "doctor-test-daemon-runtime", Severity: diagnostic.SeverityWarning,
				Message: "daemon runtime state is degraded", Fix: "show test runtime",
			}}
		},
	}
	require.NoError(t, diagnostic.RegisterDoctorCheck(check))
	t.Cleanup(func() { diagnostic.UnregisterDoctorCheckForTest(name) })

	resp, err := HandleDoctor(&pluginserver.CommandContext{}, commandtest.Args())
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, resp.Status)
	got, ok := resp.Data.(plugin.RawJSON)
	require.True(t, ok, "daemon data is %T, want the encoded producer answer", resp.Data)

	var result diagnostic.DoctorResult
	require.NoError(t, json.Unmarshal([]byte(got), &result))
	var found *diagnostic.Diagnostic
	for i := range result.Diagnostics {
		if result.Diagnostics[i].Code == "doctor-test-daemon-runtime" {
			found = &result.Diagnostics[i]
		}
	}
	require.NotNil(t, found, "show doctor did not run the runtime check: %s", got)
	assert.Equal(t, diagnostic.SeverityWarning, found.Severity)
	assert.Equal(t, "daemon runtime state is degraded", found.Message)
	assert.Equal(t, "show test runtime", found.Fix)
}

// countingDoctorCheckSource is a running-plugin set holding one plugin that
// declared one runtime check at Stage 1; it counts the callbacks it answers.
type countingDoctorCheckSource struct {
	calls *int
}

func (countingDoctorCheckSource) DoctorCheckPlugins() map[string][]plugin.DoctorCheckRegistration {
	return map[string][]plugin.DoctorCheckRegistration{
		"fixture-runtime-plugin": {{Name: "fixture-runtime-check", Phase: rpc.DoctorPhaseRuntime, Platforms: []string{"any"}}},
	}
}

func (c countingDoctorCheckSource) RunDoctorChecks(requests []pluginserver.DoctorCheckPluginRequest, _, _ time.Duration) []pluginserver.DoctorCheckPluginResult {
	results := make([]pluginserver.DoctorCheckPluginResult, len(requests))
	for i, req := range requests {
		outcomes := make([]pluginserver.DoctorCheckOutcome, len(req.Checks))
		for j, check := range req.Checks {
			*c.calls++
			outcomes[j] = pluginserver.DoctorCheckOutcome{
				Check: check,
				Output: &rpc.DoctorCheckOutput{Diagnostics: []rpc.DoctorCheckDiagnostic{{
					Code: "doctor-fixture-runtime", Severity: "warning", Message: "fixture live state is degraded",
				}}},
			}
		}
		results[i] = pluginserver.DoctorCheckPluginResult{Plugin: req.Plugin, Outcomes: outcomes}
	}
	return results
}

// TestDoctorPluginRuntimeCheckRunsOnce runs one show doctor answer for a
// plugin whose runtime check is declared at Stage 1, after trying to register
// the same check as a registry function too.
//
// VALIDATES: a plugin runtime check has one route, the doctor callback: the
// plugin registry refuses the registry form, and one daemon answer calls the
// check once and lists its finding once.
// PREVENTS: RunRuntimeChecks running the plugin registry's runtime phase in
// the daemon's process beside the callback, which ran the check twice (and,
// for a forked plugin, over the daemon's empty copy of its state).
func TestDoctorPluginRuntimeCheckRunsOnce(t *testing.T) {
	snap := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snap) })

	calls := 0
	err := registry.Register(registry.Registration{
		Name:        "fixture-runtime-plugin",
		Description: "runtime check counted",
		RunEngine:   func(net.Conn) int { return 0 },
		CLIHandler:  func([]string) int { return 0 },
		DoctorChecks: []registry.DoctorCheckDef{{
			Name:         "fixture-runtime-check",
			Phase:        rpc.DoctorPhaseRuntime,
			Dependencies: []string{"live-state"},
			Platforms:    []string{"any"},
			Codes:        []string{"doctor-fixture-runtime"},
			Check: func(registry.DoctorCheckContext) []rpc.DoctorCheckDiagnostic {
				calls++
				return []rpc.DoctorCheckDiagnostic{{
					Code: "doctor-fixture-runtime", Severity: "warning", Message: "fixture live state is degraded",
				}}
			},
		}},
	})
	assert.ErrorIs(t, err, registry.ErrInvalidDoctorCheck, "the plugin registry accepted a runtime check as a registry function")

	resp, err := answerDaemonDoctor(countingDoctorCheckSource{calls: &calls}, "", commandtest.Args())
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, resp.Status)
	got, ok := resp.Data.(plugin.RawJSON)
	require.True(t, ok, "daemon data is %T, want the encoded producer answer", resp.Data)

	var result diagnostic.DoctorResult
	require.NoError(t, json.Unmarshal([]byte(got), &result))
	listed := 0
	for i := range result.Diagnostics {
		if result.Diagnostics[i].Code == "doctor-fixture-runtime" {
			listed++
		}
	}
	assert.Equal(t, 1, calls, "one show doctor answer ran the plugin runtime check %d times", calls)
	assert.Equal(t, 1, listed, "one show doctor answer listed the plugin runtime finding %d times: %s", listed, got)
}

type doctorTestPlugin struct {
	name   string
	checks []rpc.DoctorCheckDecl
	handle sdk.DoctorCheckHandler
}

type doctorTestSpawner struct {
	admissionMu sync.Mutex
	sealed      bool
	ctx         context.Context
	manager     *process.ProcessManager
}

func (s *doctorTestSpawner) SpawnMore(configs []plugin.PluginConfig) error {
	s.admissionMu.Lock()
	if s.sealed {
		s.admissionMu.Unlock()
		return process.ErrDaemonStopping
	}
	first := s.manager == nil
	if first {
		s.manager = process.NewProcessManager(configs)
	}
	pm := s.manager
	s.admissionMu.Unlock()
	if first {
		return pm.StartWithContext(s.ctx)
	}
	return pm.StartMore(configs)
}

func (s *doctorTestSpawner) GetProcessManager() any {
	s.admissionMu.Lock()
	defer s.admissionMu.Unlock()
	return s.manager
}

func (s *doctorTestSpawner) SealProcessAdmission() func() {
	s.admissionMu.Lock()
	defer s.admissionMu.Unlock()
	s.sealed = true
	return func() {}
}

func startDoctorTestServer(t *testing.T, plugins []doctorTestPlugin) *pluginserver.Server {
	t.Helper()
	snap := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snap) })

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	configs := make([]plugin.PluginConfig, 0, len(plugins))
	for _, spec := range plugins {
		require.NoError(t, registry.Register(registry.Registration{
			Name:        spec.name,
			Description: "doctor concurrency test plugin",
			RunEngine: func(conn net.Conn) int {
				p := sdk.NewWithConn(spec.name, conn)
				p.OnDoctorCheck(spec.handle)
				if err := p.Run(ctx, &sdk.Registration{
					DoctorChecks: spec.checks,
				}); err != nil && ctx.Err() == nil {
					return 1
				}
				return 0
			},
			CLIHandler: func([]string) int { return 0 },
		}))
		configs = append(configs, plugin.PluginConfig{
			Name:     spec.name,
			Internal: true,
			Encoder:  plugin.EncodingJSON,
		})
	}

	srv, err := pluginserver.NewServer(&pluginserver.ServerConfig{
		Plugins:         configs,
		ConfigNeedHolds: config.ConfigNeedHolds(map[string]any{}),
	}, nil)
	require.NoError(t, err)
	spawner := &doctorTestSpawner{ctx: ctx}
	srv.SetProcessSpawner(spawner)
	t.Cleanup(func() {
		srv.Stop()
		join, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		require.NoError(t, srv.Wait(join))
		if manager := srv.ProcessManager(); manager != nil {
			require.NoError(t, manager.Wait(join))
		}
	})
	require.NoError(t, srv.StartWithContext(ctx))

	waitCtx, stopWait := context.WithTimeout(ctx, 10*time.Second)
	defer stopWait()
	require.NoError(t, srv.WaitForStartupComplete(waitCtx))
	return srv
}

// TestCollectDoctorChecksConcurrent proves AC-1 and AC-2 end-to-end across
// collectDoctorChecksWithTimeouts and (*pluginserver.Server).RunDoctorChecks.
//
// VALIDATES: concurrent execution across plugins, isolation of a hung plugin
// (and exclusion from ownedCodes), serial execution with fresh per-check bounds
// within one plugin, deterministic (Order, plugin, check) sorting, concurrent
// show doctor queuing with pickup-based check clocks and discard of expired
// requests, and caller-skip before worker submission.
// PREVENTS: linear latency growth across plugins, random equal-Order output,
// sibling timeout expiry within one plugin, and self-deadlock on caller dispatch.
func TestCollectDoctorChecksConcurrent(t *testing.T) {
	t.Run("concurrent-across-plugins", func(t *testing.T) {
		const (
			checkTimeout = 400 * time.Millisecond
			queueWait    = 400 * time.Millisecond
			delay        = 320 * time.Millisecond // 0.8 of checkTimeout
		)
		srv := startDoctorTestServer(t, []doctorTestPlugin{
			{
				name: "slow-a",
				checks: []rpc.DoctorCheckDecl{{
					Name: "check-a", Phase: rpc.DoctorPhaseRuntime, Platforms: []string{"any"},
					Codes: []string{"doctor-slow-a"},
				}},
				handle: func(string) ([]rpc.DoctorCheckDiagnostic, error) {
					time.Sleep(delay)
					return []rpc.DoctorCheckDiagnostic{{
						Code: "doctor-slow-a", Severity: "warning", Message: "slow-a warning",
					}}, nil
				},
			},
			{
				name: "slow-b",
				checks: []rpc.DoctorCheckDecl{{
					Name: "check-b", Phase: rpc.DoctorPhaseRuntime, Platforms: []string{"any"},
					Codes: []string{"doctor-slow-b"},
				}},
				handle: func(string) ([]rpc.DoctorCheckDiagnostic, error) {
					time.Sleep(delay)
					return []rpc.DoctorCheckDiagnostic{{
						Code: "doctor-slow-b", Severity: "warning", Message: "slow-b warning",
					}}, nil
				},
			},
		})

		start := time.Now()
		answer := collectDoctorChecksWithTimeouts(srv, "", checkTimeout, queueWait)
		elapsed := time.Since(start)

		assert.Less(t, elapsed, 560*time.Millisecond, "two 320ms plugins took %v; expected concurrent completion under one bound plus margin", elapsed)
		require.Len(t, answer.diagnostics, 2)
		assert.Equal(t, "doctor-slow-a", answer.diagnostics[0].Code)
		assert.Equal(t, "doctor-slow-b", answer.diagnostics[1].Code)
	})

	t.Run("hung-plugin-does-not-delay-fast-plugin", func(t *testing.T) {
		const (
			checkTimeout = 200 * time.Millisecond
			queueWait    = 200 * time.Millisecond
		)
		releaseHung := make(chan struct{})
		defer close(releaseHung)

		var fastDoneAt atomic.Int64
		srv := startDoctorTestServer(t, []doctorTestPlugin{
			{
				name: "hung-plugin",
				checks: []rpc.DoctorCheckDecl{{
					Name: "hung-check", Phase: rpc.DoctorPhaseRuntime, Platforms: []string{"any"},
					Codes: []string{"doctor-hung-owned"},
				}},
				handle: func(string) ([]rpc.DoctorCheckDiagnostic, error) {
					<-releaseHung
					return nil, nil
				},
			},
			{
				name: "fast-plugin",
				checks: []rpc.DoctorCheckDecl{{
					Name: "fast-check", Phase: rpc.DoctorPhaseRuntime, Platforms: []string{"any"},
					Codes: []string{"doctor-fast-owned"},
				}},
				handle: func(string) ([]rpc.DoctorCheckDiagnostic, error) {
					fastDoneAt.Store(time.Now().UnixNano())
					return []rpc.DoctorCheckDiagnostic{{
						Code: "doctor-fast-owned", Severity: "warning", Message: "fast plugin finding",
					}}, nil
				},
			},
		})

		start := time.Now()
		answer := collectDoctorChecksWithTimeouts(srv, "", checkTimeout, queueWait)

		require.NotZero(t, fastDoneAt.Load(), "fast-plugin never ran")
		fastElapsed := time.Unix(0, fastDoneAt.Load()).Sub(start)
		assert.Less(t, fastElapsed, 150*time.Millisecond, "hung plugin delayed fast plugin (%v)", fastElapsed)
		assert.True(t, answer.codes["doctor-fast-owned"], "answering plugin must record its owned codes")
		assert.False(t, answer.codes["doctor-hung-owned"], "timed-out plugin must not record owned codes")

		require.Len(t, answer.diagnostics, 2)
		assert.Equal(t, "doctor-fast-owned", answer.diagnostics[0].Code)
		assert.Equal(t, "doctor-plugin-unreachable", answer.diagnostics[1].Code)
		assert.Equal(t, diagnostic.SeverityWarning, answer.diagnostics[1].Severity)
		assert.Equal(t, "ze request plugin restart hung-plugin", answer.diagnostics[1].Fix)
	})

	t.Run("serial-within-one-plugin-fresh-bound", func(t *testing.T) {
		const (
			checkTimeout = 250 * time.Millisecond
			queueWait    = 250 * time.Millisecond
			delay        = 150 * time.Millisecond
		)
		var (
			inFlight   atomic.Int32
			overlapped atomic.Bool
			mu         sync.Mutex
			order      []string
		)
		srv := startDoctorTestServer(t, []doctorTestPlugin{
			{
				name: "pair-plugin",
				checks: []rpc.DoctorCheckDecl{
					{Name: "check-b", Phase: rpc.DoctorPhaseRuntime, Platforms: []string{"any"}, Order: 20, Codes: []string{"doctor-pair-check-b"}},
					{Name: "check-a", Phase: rpc.DoctorPhaseRuntime, Platforms: []string{"any"}, Order: 10, Codes: []string{"doctor-pair-check-a"}},
				},
				handle: func(name string) ([]rpc.DoctorCheckDiagnostic, error) {
					if inFlight.Add(1) > 1 {
						overlapped.Store(true)
					}
					defer inFlight.Add(-1)
					mu.Lock()
					order = append(order, name)
					mu.Unlock()
					time.Sleep(delay)
					return []rpc.DoctorCheckDiagnostic{{
						Code: "doctor-pair-" + name, Severity: "info", Message: name,
					}}, nil
				},
			},
		})

		answer := collectDoctorChecksWithTimeouts(srv, "", checkTimeout, queueWait)
		assert.False(t, overlapped.Load(), "checks of the same plugin overlapped")
		assert.Equal(t, []string{"check-a", "check-b"}, order)
		require.Len(t, answer.diagnostics, 2)
		assert.Equal(t, "doctor-pair-check-a", answer.diagnostics[0].Code)
		assert.Equal(t, "doctor-pair-check-b", answer.diagnostics[1].Code)
	})

	t.Run("deterministic-order-equal-order-out-of-order-completion", func(t *testing.T) {
		srv := startDoctorTestServer(t, []doctorTestPlugin{
			{
				name: "plugin-c",
				checks: []rpc.DoctorCheckDecl{
					{Name: "check-late", Phase: rpc.DoctorPhaseRuntime, Platforms: []string{"any"}, Order: 10, Codes: []string{"doctor-c-late"}},
					{Name: "check-early", Phase: rpc.DoctorPhaseRuntime, Platforms: []string{"any"}, Order: 5, Codes: []string{"doctor-c-early"}},
				},
				handle: func(name string) ([]rpc.DoctorCheckDiagnostic, error) {
					if name == "check-early" {
						return []rpc.DoctorCheckDiagnostic{{Code: "doctor-c-early", Severity: "info", Message: "c-early"}}, nil
					}
					return []rpc.DoctorCheckDiagnostic{{Code: "doctor-c-late", Severity: "info", Message: "c-late"}}, nil
				},
			},
			{
				name: "plugin-b",
				checks: []rpc.DoctorCheckDecl{
					{Name: "check-z", Phase: rpc.DoctorPhaseRuntime, Platforms: []string{"any"}, Order: 10, Codes: []string{"doctor-b-z"}},
					{Name: "check-a", Phase: rpc.DoctorPhaseRuntime, Platforms: []string{"any"}, Order: 10, Codes: []string{"doctor-b-a"}},
				},
				handle: func(name string) ([]rpc.DoctorCheckDiagnostic, error) {
					time.Sleep(20 * time.Millisecond)
					if name == "check-a" {
						return []rpc.DoctorCheckDiagnostic{{Code: "doctor-b-a", Severity: "info", Message: "b-a"}}, nil
					}
					return []rpc.DoctorCheckDiagnostic{{Code: "doctor-b-z", Severity: "info", Message: "b-z"}}, nil
				},
			},
			{
				name: "plugin-a",
				checks: []rpc.DoctorCheckDecl{
					{Name: "check-1", Phase: rpc.DoctorPhaseRuntime, Platforms: []string{"any"}, Order: 10, Codes: []string{"doctor-a-1"}},
				},
				handle: func(string) ([]rpc.DoctorCheckDiagnostic, error) {
					time.Sleep(60 * time.Millisecond)
					return []rpc.DoctorCheckDiagnostic{{Code: "doctor-a-1", Severity: "info", Message: "a-1"}}, nil
				},
			},
		})

		want := []string{"doctor-c-early", "doctor-a-1", "doctor-b-a", "doctor-b-z", "doctor-c-late"}
		for range 5 {
			answer := collectDoctorChecksWithTimeouts(srv, "", 500*time.Millisecond, 500*time.Millisecond)
			got := make([]string, len(answer.diagnostics))
			for i, d := range answer.diagnostics {
				got[i] = d.Code
			}
			require.Equal(t, want, got)
		}
	})

	t.Run("concurrent-callers-queue-pickup-clock-and-discard-expired", func(t *testing.T) {
		var calls atomic.Int32
		call1Entered := make(chan struct{})
		release1 := make(chan struct{})
		var releaseOnce sync.Once
		release := func() { releaseOnce.Do(func() { close(release1) }) }
		defer release()

		srv := startDoctorTestServer(t, []doctorTestPlugin{
			{
				name: "queued-plugin",
				checks: []rpc.DoctorCheckDecl{{
					Name: "queued-check", Phase: rpc.DoctorPhaseRuntime, Platforms: []string{"any"},
					Codes: []string{"doctor-queued-ok"},
				}},
				handle: func(string) ([]rpc.DoctorCheckDiagnostic, error) {
					n := calls.Add(1)
					if n == 1 {
						close(call1Entered)
						<-release1
					} else {
						time.Sleep(150 * time.Millisecond)
					}
					return []rpc.DoctorCheckDiagnostic{{
						Code: "doctor-queued-ok", Severity: "info", Message: "queued ok",
					}}, nil
				},
			},
		})

		ans1Ch := make(chan pluginCheckAnswer, 1)
		go func() {
			ans1Ch <- collectDoctorChecksWithTimeouts(srv, "", 2*time.Second, 2*time.Second)
		}()

		select {
		case <-call1Entered:
		case <-time.After(2 * time.Second):
			t.Fatal("first doctor check never reached the plugin handler")
		}

		ans2Ch := make(chan pluginCheckAnswer, 1)
		go func() {
			ans2Ch <- collectDoctorChecksWithTimeouts(srv, "", 250*time.Millisecond, 2*time.Second)
		}()

		ans3Ch := make(chan pluginCheckAnswer, 1)
		go func() {
			ans3Ch <- collectDoctorChecksWithTimeouts(srv, "", 250*time.Millisecond, 80*time.Millisecond)
		}()

		var ans3 pluginCheckAnswer
		select {
		case ans3 = <-ans3Ch:
		case <-time.After(2 * time.Second):
			t.Fatal("third doctor check did not expire at queueWait")
		}
		require.Len(t, ans3.diagnostics, 1)
		assert.Equal(t, "doctor-plugin-unreachable", ans3.diagnostics[0].Code)
		assert.Contains(t, ans3.diagnostics[0].Message, "queued-plugin")
		assert.Equal(t, "ze request plugin restart queued-plugin", ans3.diagnostics[0].Fix)

		// Hold past Call 2's checkTimeout while Call 2 is still queued so its
		// success proves the per-check clock started at pickup, not enqueue.
		time.Sleep(180 * time.Millisecond)
		release()

		ans1 := <-ans1Ch
		ans2 := <-ans2Ch
		require.Len(t, ans1.diagnostics, 1)
		assert.Equal(t, "doctor-queued-ok", ans1.diagnostics[0].Code)
		require.Len(t, ans2.diagnostics, 1)
		assert.Equal(t, "doctor-queued-ok", ans2.diagnostics[0].Code)
		assert.Equal(t, int32(2), calls.Load(), "expired third request must be discarded unrun")
	})

	t.Run("caller-skip-omits-calling-plugin", func(t *testing.T) {
		releaseCaller := make(chan struct{})
		defer close(releaseCaller)
		var callerInvoked atomic.Int32

		srv := startDoctorTestServer(t, []doctorTestPlugin{
			{
				name: "caller-plugin",
				checks: []rpc.DoctorCheckDecl{{
					Name: "caller-check", Phase: rpc.DoctorPhaseRuntime, Platforms: []string{"any"},
					Codes: []string{"doctor-caller-skip"},
				}},
				handle: func(string) ([]rpc.DoctorCheckDiagnostic, error) {
					callerInvoked.Add(1)
					<-releaseCaller
					return nil, nil
				},
			},
			{
				name: "other-plugin",
				checks: []rpc.DoctorCheckDecl{{
					Name: "other-check", Phase: rpc.DoctorPhaseRuntime, Platforms: []string{"any"},
					Codes: []string{"doctor-other-ok"},
				}},
				handle: func(string) ([]rpc.DoctorCheckDiagnostic, error) {
					return []rpc.DoctorCheckDiagnostic{{
						Code: "doctor-other-ok", Severity: "info", Message: "other plugin ok",
					}}, nil
				},
			},
		})

		start := time.Now()
		answer := collectDoctorChecksWithTimeouts(srv, "caller-plugin", 250*time.Millisecond, 250*time.Millisecond)
		elapsed := time.Since(start)

		assert.Less(t, elapsed, 200*time.Millisecond, "caller-skip waited on the calling plugin (%v)", elapsed)
		assert.Zero(t, callerInvoked.Load(), "calling plugin check must not be invoked")
		require.Len(t, answer.diagnostics, 1)
		assert.Equal(t, "doctor-other-ok", answer.diagnostics[0].Code)
	})
}
