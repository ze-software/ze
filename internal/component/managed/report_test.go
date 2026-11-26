// VALIDATES: managed client periodic fleet reporting -- health projection matches
// the daemon /health HTTP schema (including omitting inapplicable checks), the
// per-session worker sends inventory and health reports immediately after
// config-fetch (even while holding local edits) and repeats on each producer's
// interval, unknown-method errors disable only that producer until reconnect,
// missing replies time out without blocking ping or config-changed, report
// failures raise and clear CodeDoctorManagedReportFailed on the report bus, and
// session stop joins the worker cleanly.
// PREVENTS: fleet reporting blocking config sync or heartbeat, old hubs breaking
// new clients, report failures going silent, or per-session worker goroutines
// leaking on disconnect.

package managed

import (
	"context"
	"encoding/base64"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/health"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/pkg/fleet"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

const testReportTimestamp = "2026-11-26T12:00:00Z"

func sampleInventoryBytes(t *testing.T, swVersion string) []byte {
	t.Helper()
	rep := fleet.InventoryReport{
		CollectedAt:     testReportTimestamp,
		SoftwareVersion: swVersion,
		CPU:             fleet.InventoryCPU{Available: true, LogicalCPUs: 8, Model: "AMD EPYC"},
		NIC:             fleet.InventoryNIC{Available: true, Count: 2, Drivers: []string{"igb", "mlx5_core"}},
		Memory: fleet.InventoryMemory{
			Available:  true,
			TotalBytes: 32 << 30,
			ECC:        &fleet.InventoryECC{CorrectableErrors: 1, UncorrectableErrors: 0},
		},
		Storage: fleet.InventoryStorage{Available: true, DeviceCount: 1, TotalBytes: 256 << 30},
		DMI:     fleet.InventoryDMI{Available: true, Model: "PowerEdge R6515", Serial: "SYS-01"},
	}
	b, err := fleet.EncodeInventoryReport(rep)
	require.NoError(t, err)
	return b
}

func sampleHealthBytes(t *testing.T, status fleet.HealthStatus, reason string) []byte {
	t.Helper()
	rep := fleet.HealthReport{
		Status: status,
		Components: []fleet.HealthComponent{
			{
				Name:      "bgp",
				Status:    status,
				Reason:    reason,
				CheckedAt: testReportTimestamp,
			},
		},
		CheckedAt: testReportTimestamp,
	}
	b, err := fleet.EncodeHealthReport(rep)
	require.NoError(t, err)
	return b
}

func findReportWarning(device, kind string) (report.Issue, bool) {
	subject := fitReportSubject(device, kind)
	for _, w := range report.Warnings() {
		if w.Source == doctorComponent && w.Code == CodeDoctorManagedReportFailed && w.Subject == subject {
			return w, true
		}
	}
	return report.Issue{}, false
}

// TestHealthReportJSONMatchesHealthHandler proves that ProjectFleetHealth and
// fleet.HealthReport preserve the exact JSON structure and semantics of
// internal/core/health.Registry.Handler(), including omitting inapplicable
// checks and emitting an empty JSON array when no component applies.
//
// VALIDATES: AC-3, AC-16 (health report wire format parity with /health and
// omission of inapplicable checks) and init() registration of the health
// producer.
// PREVENTS: fleet health schema drifting from /health or inapplicable checks
// leaking into reported components.
func TestHealthReportJSONMatchesHealthHandler(t *testing.T) {
	t.Run("health producer registered at init", func(t *testing.T) {
		var found bool
		for _, p := range fleet.ReportProducers() {
			if p.Method == fleet.VerbHealthReport {
				found = true
				assert.Equal(t, fleetHealthProducerName, p.Name)
				assert.Equal(t, FleetHealthInterval, p.Interval)
				assert.NotNil(t, p.Collect)
			}
		}
		require.True(t, found, "health producer must be registered via init()")
	})

	t.Run("healthy degraded and inapplicable checks match HTTP handler JSON", func(t *testing.T) {
		reg := &health.Registry{}
		reg.Register("bgp", func() (health.Status, string) {
			return health.StatusDegraded, "2 of 4 peers down"
		})
		reg.Register("config", func() (health.Status, string) {
			return health.StatusHealthy, ""
		})
		reg.RegisterWhen(
			"ospf",
			func() bool { return false },
			func() (health.Status, string) { return health.StatusDown, "must not appear" },
		)

		rec := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/health", http.NoBody)
		reg.Handler().ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)

		httpRep, err := fleet.DecodeHealthReport(rec.Body.Bytes())
		require.NoError(t, err)

		projected, err := ProjectFleetHealth(reg.Check())
		require.NoError(t, err)
		wire, err := fleet.EncodeHealthReport(projected)
		require.NoError(t, err)
		decoded, err := fleet.DecodeHealthReport(wire)
		require.NoError(t, err)

		assert.Equal(t, fleet.HealthStatusDegraded, httpRep.Status)
		assert.Equal(t, httpRep.Status, decoded.Status)
		require.Len(t, httpRep.Components, 2)
		require.Len(t, decoded.Components, 2)
		for i := range httpRep.Components {
			assert.Equal(t, httpRep.Components[i].Name, decoded.Components[i].Name)
			assert.Equal(t, httpRep.Components[i].Status, decoded.Components[i].Status)
			assert.Equal(t, httpRep.Components[i].Reason, decoded.Components[i].Reason)
			assert.NotEqual(t, "ospf", decoded.Components[i].Name)
		}
		assert.Equal(t, "bgp", decoded.Components[0].Name)
		assert.Equal(t, fleet.HealthStatusDegraded, decoded.Components[0].Status)
		assert.Equal(t, "2 of 4 peers down", decoded.Components[0].Reason)
		assert.Equal(t, "config", decoded.Components[1].Name)
		assert.Equal(t, fleet.HealthStatusHealthy, decoded.Components[1].Status)
	})

	t.Run("empty registry emits empty components array and down reason sanitized", func(t *testing.T) {
		emptyReg := &health.Registry{}
		rec := httptest.NewRecorder()
		emptyReg.Handler().ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/health", http.NoBody))
		httpEmpty, err := fleet.DecodeHealthReport(rec.Body.Bytes())
		require.NoError(t, err)

		projEmpty, err := ProjectFleetHealth(emptyReg.Check())
		require.NoError(t, err)
		wireEmpty, err := fleet.EncodeHealthReport(projEmpty)
		require.NoError(t, err)
		assert.Contains(t, string(wireEmpty), `"components":[]`)
		assert.Equal(t, fleet.HealthStatusHealthy, httpEmpty.Status)
		assert.Empty(t, httpEmpty.Components)

		downReg := &health.Registry{}
		downReg.Register("rib", func() (health.Status, string) {
			return health.StatusDown, "line1\nline2\t" + strings.Repeat("x", fleet.ReportTextMax+40)
		})
		projDown, err := ProjectFleetHealth(downReg.Check())
		require.NoError(t, err)
		assert.Equal(t, fleet.HealthStatusDown, projDown.Status)
		require.Len(t, projDown.Components, 1)
		assert.LessOrEqual(t, len(projDown.Components[0].Reason), fleet.ReportTextMax)
		assert.NotContains(t, projDown.Components[0].Reason, "\n")
		_, err = fleet.EncodeHealthReport(projDown)
		require.NoError(t, err)
	})
}

// TestClientSendsInventoryReport proves that a connected managed client sends
// inventory-report immediately after its initial config-fetch completes,
// repeats it on the producer's interval, and still reports when the client is
// holding local configuration edits.
//
// VALIDATES: AC-1 (initial and periodic inventory-report after config-fetch)
// and holding-client reporting.
// PREVENTS: reporting starting before config-fetch, failing to repeat, or
// being suppressed while a device holds local edits.
func TestClientSendsInventoryReport(t *testing.T) {
	t.Run("sends after config-fetch and repeats on interval", func(t *testing.T) {
		addr, admitted := fetchHub(t)
		cfg, _, _ := fetchClient(t, addr)
		payload := sampleInventoryBytes(t, "26.11.01")
		cfg.reportProducersFn = func() []fleet.ReportProducer {
			return []fleet.ReportProducer{{
				Name:     "inventory",
				Method:   fleet.VerbInventoryReport,
				Interval: 40 * time.Millisecond,
				Collect:  func(context.Context) ([]byte, error) { return payload, nil },
			}}
		}
		_ = runClient(t, cfg)

		hub := nextSession(t, admitted)
		// First request MUST be config-fetch before any report is sent.
		answerFetchCurrent(t, hub)

		first := hubRequest(t, hub)
		require.Equal(t, fleet.VerbInventoryReport, first.Method)
		inv1, err := fleet.DecodeInventoryReport(first.Params)
		require.NoError(t, err)
		assert.Equal(t, "26.11.01", inv1.SoftwareVersion)
		assert.True(t, inv1.CPU.Available)
		assert.Equal(t, 8, inv1.CPU.LogicalCPUs)
		require.NoError(t, hub.SendOK(t.Context(), first.ID))

		second := hubRequest(t, hub)
		require.Equal(t, fleet.VerbInventoryReport, second.Method)
		_, err = fleet.DecodeInventoryReport(second.Params)
		require.NoError(t, err)
		require.NoError(t, hub.SendOK(t.Context(), second.ID))
	})

	t.Run("holding client still sends inventory report", func(t *testing.T) {
		addr, admitted := fetchHub(t)
		store := holdDevice(t, holdDeviceConfig, holdEarlierConfig)
		cfg, _, applied := holdClient(addr, NewBaselineAccess(store))
		payload := sampleInventoryBytes(t, "26.11.02")
		cfg.reportProducersFn = func() []fleet.ReportProducer {
			return []fleet.ReportProducer{{
				Name:     "inventory",
				Method:   fleet.VerbInventoryReport,
				Interval: time.Hour,
				Collect:  func(context.Context) ([]byte, error) { return payload, nil },
			}}
		}
		_ = runClient(t, cfg)

		hub := nextSession(t, admitted)
		// Offer a different config so the client holds, uploads held config, and acks holding.
		fetchReq := hubRequest(t, hub)
		require.Equal(t, fleet.VerbConfigFetch, fetchReq.Method)
		require.NoError(t, hub.SendResult(t.Context(), fetchReq.ID, fleet.ConfigFetchResponse{
			Version: fleet.VersionHash([]byte(holdHubConfig)),
			Config:  base64.StdEncoding.EncodeToString([]byte(holdHubConfig)),
		}))

		var gotInventory bool
		for range 4 {
			req := hubRequest(t, hub)
			switch req.Method {
			case fleet.VerbConfigHeld, fleet.VerbConfigAck:
				require.NoError(t, hub.SendOK(t.Context(), req.ID))
			case fleet.VerbInventoryReport:
				inv, err := fleet.DecodeInventoryReport(req.Params)
				require.NoError(t, err)
				assert.Equal(t, "26.11.02", inv.SoftwareVersion)
				require.NoError(t, hub.SendOK(t.Context(), req.ID))
				gotInventory = true
			default:
				t.Fatalf("unexpected request method %q while holding", req.Method)
			}
			if gotInventory {
				break
			}
		}
		require.True(t, gotInventory, "holding client must still send inventory-report")
		assert.Empty(t, applied, "holding client must not apply hub config")
	})
}

// TestClientSendsHealthReport proves that a connected managed client sends
// health-report immediately after config-fetch and periodically on its
// configured interval using ProjectFleetHealth over a health.Registry.
//
// VALIDATES: AC-3 (initial and periodic health-report with component statuses).
// PREVENTS: health reports failing to fire on connect or failing to repeat on
// the scheduled interval.
func TestClientSendsHealthReport(t *testing.T) {
	reg := &health.Registry{}
	reg.Register("bgp", func() (health.Status, string) {
		return health.StatusDegraded, "1 peer idle"
	})

	addr, admitted := fetchHub(t)
	cfg, _, _ := fetchClient(t, addr)
	cfg.reportProducersFn = func() []fleet.ReportProducer {
		return []fleet.ReportProducer{{
			Name:     "health",
			Method:   fleet.VerbHealthReport,
			Interval: 40 * time.Millisecond,
			Collect: func(context.Context) ([]byte, error) {
				rep, err := ProjectFleetHealth(reg.Check())
				if err != nil {
					return nil, err
				}
				return fleet.EncodeHealthReport(rep)
			},
		}}
	}
	_ = runClient(t, cfg)

	hub := nextSession(t, admitted)
	answerFetchCurrent(t, hub)

	for i := range 2 {
		req := hubRequest(t, hub)
		require.Equal(t, fleet.VerbHealthReport, req.Method, "report %d", i+1)
		got, err := fleet.DecodeHealthReport(req.Params)
		require.NoError(t, err)
		assert.Equal(t, fleet.HealthStatusDegraded, got.Status)
		require.Len(t, got.Components, 1)
		assert.Equal(t, "bgp", got.Components[0].Name)
		assert.Equal(t, "1 peer idle", got.Components[0].Reason)
		require.NoError(t, hub.SendOK(t.Context(), req.ID))
	}
}

// TestReportUnknownMethodAndTimeout exercises backward compatibility, timeout
// isolation, report-bus diagnostics, and clean worker shutdown.
//
// VALIDATES: AC-11 (unknown-method suppresses only that report until reconnect;
// missing reply times out without blocking ping or config-changed), report-bus
// warning raise/clear under CodeDoctorManagedReportFailed, and worker join on
// session end.
// PREVENTS: an old hub breaking health reporting or config sync, a stalled
// report blocking liveness/pushes, silent report failures, or leaked goroutines.
func TestReportUnknownMethodAndTimeout(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)

	t.Run("unknown method disables only that report until reconnect", func(t *testing.T) {
		report.ResetForTest()
		addr, admitted := fetchHub(t)
		cfg, _, _ := fetchClient(t, addr)
		invPayload := sampleInventoryBytes(t, "26.11.01")
		healthPayload := sampleHealthBytes(t, fleet.HealthStatusHealthy, "")
		cfg.reportProducersFn = func() []fleet.ReportProducer {
			return []fleet.ReportProducer{
				{
					Name:     "inventory",
					Method:   fleet.VerbInventoryReport,
					Interval: 20 * time.Millisecond,
					Collect:  func(context.Context) ([]byte, error) { return invPayload, nil },
				},
				{
					Name:     "health",
					Method:   fleet.VerbHealthReport,
					Interval: 35 * time.Millisecond,
					Collect:  func(context.Context) ([]byte, error) { return healthPayload, nil },
				},
			}
		}
		_ = runClient(t, cfg)

		// Session 1: old hub rejects inventory-report with unknown method, accepts health-report.
		first := nextSession(t, admitted)
		answerFetchCurrent(t, first)

		invCount1 := 0
		healthCount1 := 0
		for healthCount1 < 3 {
			req := hubRequest(t, first)
			switch req.Method {
			case fleet.VerbInventoryReport:
				invCount1++
				require.NoError(t, first.SendError(t.Context(), req.ID, unknownMethodPrefix+fleet.VerbInventoryReport))
			case fleet.VerbHealthReport:
				healthCount1++
				require.NoError(t, first.SendOK(t.Context(), req.ID))
			default:
				t.Fatalf("unexpected request on session 1: %q", req.Method)
			}
		}
		assert.Equal(t, 1, invCount1, "inventory-report must be suppressed for the rest of session 1 after unknown method")
		_, warnPresent := findReportWarning(cfg.Name, "inventory")
		assert.False(t, warnPresent, "unknown method on an old hub must not raise a report failure warning")

		// Drop session 1 and verify reconnect retries inventory-report.
		first.drop()
		second := nextSession(t, admitted)
		answerFetchCurrent(t, second)

		gotInv2 := false
		gotHealth2 := false
		for !gotInv2 || !gotHealth2 {
			req := hubRequest(t, second)
			switch req.Method {
			case fleet.VerbInventoryReport:
				gotInv2 = true
				require.NoError(t, second.SendOK(t.Context(), req.ID))
			case fleet.VerbHealthReport:
				gotHealth2 = true
				require.NoError(t, second.SendOK(t.Context(), req.ID))
			default:
				t.Fatalf("unexpected request on session 2: %q", req.Method)
			}
		}
		assert.True(t, gotInv2, "reconnect must retry previously suppressed inventory-report")
		assert.True(t, gotHealth2, "reconnect must continue sending health-report")
	})

	t.Run("missing reply times out without blocking ping or config-changed and raises then clears warning", func(t *testing.T) {
		report.ResetForTest()
		addr, admitted := fetchHub(t)
		cfg, _, _ := fetchClient(t, addr)
		cfg.reportTimeout = 250 * time.Millisecond
		invPayload := sampleInventoryBytes(t, "26.11.01")
		cfg.reportProducersFn = func() []fleet.ReportProducer {
			return []fleet.ReportProducer{{
				Name:     "inventory",
				Method:   fleet.VerbInventoryReport,
				Interval: 50 * time.Millisecond,
				Collect:  func(context.Context) ([]byte, error) { return invPayload, nil },
			}}
		}
		returned := runFetchConnection(t, &cfg)

		hub := nextSession(t, admitted)
		answerFetchCurrent(t, hub)

		// Receive the first inventory-report and intentionally do not reply (hang).
		hung := hubRequest(t, hub)
		require.Equal(t, fleet.VerbInventoryReport, hung.Method)

		// While the report RPC waits for its reply, ping and config-changed MUST still work.
		pingCtx, pingCancel := context.WithTimeout(t.Context(), holdWait)
		defer pingCancel()
		_, err := hub.CallRPC(pingCtx, fleet.VerbPing, struct{}{})
		require.NoError(t, err, "ping must succeed while a report RPC has no reply")

		require.NoError(t, pushChanged(t, hub), "config-changed must succeed while a report RPC has no reply")
		fetchReq := hubRequest(t, hub)
		require.Equal(t, fleet.VerbConfigFetch, fetchReq.Method)
		require.NoError(t, hub.SendResult(t.Context(), fetchReq.ID, fleet.ConfigFetchResponse{Status: fetchStatusCurrent}))

		// Wait for the report call timeout to raise CodeDoctorManagedReportFailed.
		require.Eventually(t, func() bool {
			w, ok := findReportWarning(cfg.Name, "inventory")
			if !ok {
				return false
			}
			return strings.Contains(w.Message, cfg.Name) &&
				strings.Contains(w.Message, "inventory") &&
				strings.Contains(w.Fix, "ze show doctor")
		}, holdWait, 10*time.Millisecond, "timed-out report must raise CodeDoctorManagedReportFailed on report bus")

		requireStillConnected(t, returned)

		// Next scheduled report succeeds and clears the warning.
		retryReq := hubRequest(t, hub)
		require.Equal(t, fleet.VerbInventoryReport, retryReq.Method)
		require.NoError(t, hub.SendOK(t.Context(), retryReq.ID))

		require.Eventually(t, func() bool {
			_, ok := findReportWarning(cfg.Name, "inventory")
			return !ok
		}, holdWait, 10*time.Millisecond, "next accepted report must clear CodeDoctorManagedReportFailed")
	})

	t.Run("collection error and hub refusal raise warning and next success clears", func(t *testing.T) {
		report.ResetForTest()
		clientEnd, hubEnd := net.Pipe()
		t.Cleanup(func() {
			_ = clientEnd.Close()
			_ = hubEnd.Close()
		})
		mc := rpc.NewMuxConn(rpc.NewConn(clientEnd, clientEnd))
		t.Cleanup(func() { _ = mc.Close() })
		hubMux := rpc.NewMuxConn(rpc.NewConn(hubEnd, hubEnd))
		t.Cleanup(func() { _ = hubMux.Close() })

		var attempt atomic.Int32
		healthPayload := sampleHealthBytes(t, fleet.HealthStatusHealthy, "")
		cfg := &ClientConfig{
			Name:          "edge-warn",
			reportTimeout: time.Second,
			reportProducersFn: func() []fleet.ReportProducer {
				return []fleet.ReportProducer{{
					Name:     "health",
					Method:   fleet.VerbHealthReport,
					Interval: 25 * time.Millisecond,
					Collect: func(context.Context) ([]byte, error) {
						if attempt.Add(1) == 1 {
							return nil, errors.New("probe failed")
						}
						return healthPayload, nil
					},
				}}
			},
		}

		stop := startSessionReporter(t.Context(), mc, cfg)
		defer stop()

		// Attempt 1 failed in Collect -> warning raised.
		require.Eventually(t, func() bool {
			w, ok := findReportWarning(cfg.Name, "health")
			return ok && strings.Contains(w.Message, "probe failed")
		}, holdWait, 5*time.Millisecond)

		// Attempt 2 reaches hub; hub refuses with a non-unknown-method error -> warning updated.
		req2 := nextRequest(t, hubMux.Requests())
		require.Equal(t, fleet.VerbHealthReport, req2.Method)
		require.NoError(t, hubMux.SendError(t.Context(), req2.ID, "rate limit exceeded"))
		require.Eventually(t, func() bool {
			w, ok := findReportWarning(cfg.Name, "health")
			return ok && strings.Contains(w.Message, "rate limit exceeded")
		}, holdWait, 5*time.Millisecond)

		// Attempt 3 reaches hub; hub accepts -> warning cleared.
		req3 := nextRequest(t, hubMux.Requests())
		require.Equal(t, fleet.VerbHealthReport, req3.Method)
		require.NoError(t, hubMux.SendOK(t.Context(), req3.ID))
		require.Eventually(t, func() bool {
			_, ok := findReportWarning(cfg.Name, "health")
			return !ok
		}, holdWait, 5*time.Millisecond)
	})

	t.Run("stop cancels in-flight call and joins worker cleanly", func(t *testing.T) {
		clientEnd, hubEnd := net.Pipe()
		t.Cleanup(func() {
			_ = clientEnd.Close()
			_ = hubEnd.Close()
		})
		mc := rpc.NewMuxConn(rpc.NewConn(clientEnd, clientEnd))
		t.Cleanup(func() { _ = mc.Close() })
		hubMux := rpc.NewMuxConn(rpc.NewConn(hubEnd, hubEnd))
		t.Cleanup(func() { _ = hubMux.Close() })

		invPayload := sampleInventoryBytes(t, "26.11.01")
		cfg := &ClientConfig{
			Name:          "edge-stop",
			reportTimeout: 30 * time.Second,
			reportProducersFn: func() []fleet.ReportProducer {
				return []fleet.ReportProducer{{
					Name:     "inventory",
					Method:   fleet.VerbInventoryReport,
					Interval: time.Hour,
					Collect:  func(context.Context) ([]byte, error) { return invPayload, nil },
				}}
			},
		}

		stop := startSessionReporter(t.Context(), mc, cfg)
		req := nextRequest(t, hubMux.Requests())
		require.Equal(t, fleet.VerbInventoryReport, req.Method)

		stopped := make(chan struct{})
		go func() {
			stop()
			close(stopped)
		}()
		select {
		case <-stopped:
		case <-time.After(time.Second):
			t.Fatal("stop() did not cancel in-flight report RPC and join worker promptly")
		}
	})
}
