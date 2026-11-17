// Design: docs/architecture/doctor-and-health-checks.md -- event code ownership.
// Related: rib_selected_diagnostic.go -- selected output raises through the real server.
package rib

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// TestSelectedOutputReportRegistrationBothTransports reaches the production
// report-issue validator after a real five-stage handshake. A doctor declaration
// or catalog entry alone must not masquerade as an event-code registration.
func TestSelectedOutputReportRegistrationBothTransports(t *testing.T) {
	for _, native := range []bool{true, false} {
		name := "framed-json"
		if native {
			name = "native"
		}
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			client, owner := selectedReportClient(t, ctx, native)
			r := &RIBManager{plugin: client}
			cause := errors.New("destination 192.0.2.1 Session 7: " + strings.Repeat("unavailable ", rpc.ReportMessageMax))
			require.NoError(t, r.reportSelectedDestinations(ctx, cause))
			warnings := report.Warnings()
			require.Len(t, warnings, 1)
			require.Equal(t, owner, warnings[0].Source)
			require.Equal(t, diagnostic.CodeDoctorBGPSelectedOutput, warnings[0].Code)
			require.Equal(t, "selected destinations", warnings[0].Subject)
			require.Contains(t, warnings[0].Message, "destination 192.0.2.1 Session 7")
			require.LessOrEqual(t, len(warnings[0].Message), rpc.ReportMessageMax)
			require.Equal(t, "ze show bgp peers", warnings[0].Fix)
			require.Contains(t, diagnostic.EventCodes(), diagnostic.CodeDoctorBGPSelectedOutput)
			require.NotNil(t, diagnostic.Lookup(diagnostic.CodeDoctorBGPSelectedOutput))

			// Neither an invented code nor an unrelated config-only catalog code
			// may use the selected-output registration to bypass admission.
			for _, code := range []string{"doctor-selected-output-unregistered-test", "doctor-bgp-md5"} {
				require.NotContains(t, diagnostic.EventCodes(), code)
				err := client.RaiseIssue(ctx, sdk.Issue{Severity: rpc.ReportWarning,
					Code: code, Subject: "unrelated", Message: "must be refused"})
				require.ErrorContains(t, err, "not a registered event code")
				require.ErrorContains(t, err, code)
			}
			require.Len(t, report.Warnings(), 1, "a refused code reached the report bus")
			require.NoError(t, r.reportSelectedDestinations(ctx, nil))
			require.Empty(t, report.Warnings(), "successful reconciliation must clear the owned warning")
		})
	}
}

// selectedReportClient runs the SDK under a configured owner different from its
// implementation name. Hiding the optional bridge selects the subprocess's JSON
// rail while retaining the same real server handshake and runtime dispatcher.
func selectedReportClient(t *testing.T, ctx context.Context, native bool) (*sdk.Plugin, string) {
	t.Helper()
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	diagnostic.RegisterBuiltinCodes()
	snapshot := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snapshot) })
	registry.Reset()
	const owner = "selected-report-west"
	started := make(chan *sdk.Plugin, 1)
	require.NoError(t, registry.Register(registry.Registration{
		Name: owner,
		RunEngine: func(conn net.Conn) int {
			if !native {
				conn = struct{ net.Conn }{conn}
			}
			p := sdk.NewWithConn("bgp-rib", conn)
			p.OnStarted(func(context.Context) error {
				started <- p
				return nil
			})
			if err := p.Run(ctx, &sdk.Registration{
				DoctorChecks: []sdk.DoctorCheckDecl{advertiseDoctorDeclaration()},
			}); err != nil && ctx.Err() == nil {
				return 1
			}
			return 0
		},
		CLIHandler: func([]string) int { return 0 },
	}))
	s, err := pluginserver.NewServer(&pluginserver.ServerConfig{
		Plugins:         []plugin.PluginConfig{{Name: owner, Internal: true, Encoder: plugin.EncodingJSON}},
		ConfigNeedHolds: config.ConfigNeedHolds(map[string]any{}),
	}, nil)
	require.NoError(t, err)
	spawner := &selectedReportSpawner{ctx: ctx}
	s.SetProcessSpawner(spawner)
	t.Cleanup(func() {
		s.Stop()
		join, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		require.NoError(t, s.Wait(join))
		if manager := s.ProcessManager(); manager != nil {
			require.NoError(t, manager.Wait(join))
		}
	})
	require.NoError(t, s.StartWithContext(ctx))
	require.NoError(t, s.WaitForStartupComplete(ctx))
	select {
	case client := <-started:
		require.Equal(t, owner, client.InstanceName())
		require.Equal(t, native, client.IsInternal(), "the SDK must use the requested transport")
		return client, owner
	case <-ctx.Done():
		t.Fatal("the reporting plugin did not finish its real startup handshake")
		return nil, ""
	}
}

type selectedReportSpawner struct {
	ctx     context.Context
	manager *process.ProcessManager
}

func (s *selectedReportSpawner) SpawnMore(configs []plugin.PluginConfig) error {
	if s.manager == nil {
		s.manager = process.NewProcessManager(configs)
		return s.manager.StartWithContext(s.ctx)
	}
	return s.manager.StartMore(configs)
}

func (s *selectedReportSpawner) GetProcessManager() any { return s.manager }
