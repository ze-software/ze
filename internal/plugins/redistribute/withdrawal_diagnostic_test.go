// Design: docs/architecture/doctor-and-health-checks.md -- redistribution problems use the original SDK reporter.
package redistribute

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"net/netip"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/ze-software/ze/internal/component/plugin/issuequeue"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	ribpolicy "github.com/ze-software/ze/internal/core/rib/policy"
	"github.com/ze-software/ze/internal/test/pluginengine"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

type withdrawalReporter struct {
	ctx   context.Context
	queue *issuequeue.Queue
}

// startWithdrawalReporter MUST return the exact SDK lifetime it starts. The
// test engine MUST close and join that SDK before the test's inputs disappear;
// the SDK entry MUST stop its queue before closing its connection.
func startWithdrawalReporter(t *testing.T, transport pluginengine.Transport) (*pluginengine.Engine, withdrawalReporter) {
	t.Helper()
	ready := make(chan withdrawalReporter, 1)
	engine := pluginengine.Start(t, transport, func(conn net.Conn) int {
		p := sdk.NewWithConn(Name, conn)
		defer func() { _ = p.Close() }()
		queue := issuequeue.Start(p)
		defer queue.Stop()
		p.OnStarted(func(ctx context.Context) error {
			ready <- withdrawalReporter{ctx: withRedistributionIssues(ctx, queue), queue: queue}
			return nil
		})
		if err := p.Run(t.Context(), &sdk.Registration{}); err != nil {
			return 1
		}
		return 0
	})
	engine.SetRPCHandler(func(_ context.Context, method string, _ json.RawMessage) (any, error) {
		if method == rpc.MethodReportIssue {
			return nil, nil
		}
		return nil, pluginengine.ErrUnhandledRPC
	})
	return engine, <-ready
}

// Unknown source removal reaches no consumer, and its actual report-issue
// request stays on the original SDK lifetime even while another is live.
// A valid registered source still withdraws normally and adds no report.
// MUTATION: log an unknown source without queueing its daemon report.
func TestUnknownWithdrawalReportsOriginalRuntime(t *testing.T) {
	for _, transport := range pluginengine.Transports() {
		t.Run(transport.String(), func(t *testing.T) {
			original, reporter := startWithdrawalReporter(t, transport)
			replacement, other := startWithdrawalReporter(t, transport)
			consumer := &stubConsumer{name: "diagnostic-consumer"}
			entry := redistevents.RouteChangeEntry{Action: redistevents.ActionRemove, Prefix: netip.MustParsePrefix("192.0.2.0/24")}
			dispatchEntryToConsumer(reporter.ctx, consumer, family.IPv4Unicast,
				"unregistered-withdrawal-source", "", 0, nil, &entry, nil, ribpolicy.Sets{}, 0)
			reporter.queue.Flush()
			other.queue.Flush()
			require.Empty(t, consumer.snapshotWithdrawn(), "unknown source cannot remove the consumer route")
			require.Empty(t, consumer.snapshotInjected())
			var reports []rpc.ReportIssueInput
			for _, call := range original.RPCs() {
				if call.Method != rpc.MethodReportIssue {
					continue
				}
				var in rpc.ReportIssueInput
				require.NoError(t, json.Unmarshal(call.Params, &in))
				reports = append(reports, in)
			}
			require.Len(t, reports, 1, "the real SDK must publish the refusal on its original connection")
			require.Equal(t, rpc.ReportRaise, reports[0].Action)
			require.Equal(t, rpc.ReportError, reports[0].Severity)
			require.Equal(t, unknownWithdrawalCode, reports[0].Code)
			require.Equal(t, unknownWithdrawalFix, reports[0].Fix)
			require.Contains(t, reports[0].Message, "unregistered-withdrawal-source")
			require.Contains(t, reports[0].Message, "192.0.2.0/24")
			require.True(t, slices.Contains(diagnostic.EventCodes(), reports[0].Code))
			require.NotNil(t, diagnostic.Lookup(reports[0].Code))
			for _, call := range replacement.RPCs() {
				require.NotEqual(t, rpc.MethodReportIssue, call.Method, "another live runtime cannot borrow the original report")
			}
			redistevents.RegisterProtocol("fakeredist")
			dispatchEntryToConsumer(reporter.ctx, consumer, family.IPv4Unicast,
				"fakeredist", "", 0, nil, &entry, nil, ribpolicy.Sets{}, 0)
			reporter.queue.Flush()
			require.Len(t, consumer.snapshotWithdrawn(), 1)
			require.Equal(t, "192.0.2.0/24", consumer.snapshotWithdrawn()[0].prefix)
			var after int
			for _, call := range original.RPCs() {
				if call.Method == rpc.MethodReportIssue {
					after++
				}
			}
			require.Equal(t, 1, after, "valid withdrawal adds no refusal event")
		})
	}
}

// A callback retaining the original reporting context after its queue stopped
// cannot publish through another live SDK. Its refusal remains visible, and
// its unknown-source withdrawal reaches no consumer.
func TestUnknownWithdrawalAfterReporterStopRefusesOriginalQueue(t *testing.T) {
	for _, transport := range pluginengine.Transports() {
		t.Run(transport.String(), func(t *testing.T) {
			original, reporter := startWithdrawalReporter(t, transport)
			replacement, other := startWithdrawalReporter(t, transport)
			var logs bytes.Buffer
			previous := logger()
			setLogger(slog.New(slog.NewTextHandler(&logs, nil)))
			t.Cleanup(func() { setLogger(previous) })
			consumer := &stubConsumer{name: "late-diagnostic-consumer"}
			entry := redistevents.RouteChangeEntry{Action: redistevents.ActionRemove, Prefix: netip.MustParsePrefix("198.51.100.0/24")}
			lateCallback := func() {
				dispatchEntryToConsumer(reporter.ctx, consumer, family.IPv4Unicast,
					"late-unregistered-withdrawal-source", "", 0, nil, &entry, nil, ribpolicy.Sets{}, 0)
			}
			reporter.queue.Stop()
			lateCallback()
			other.queue.Flush()
			require.Empty(t, consumer.snapshotWithdrawn())
			require.Empty(t, consumer.snapshotInjected())
			require.Contains(t, logs.String(), "code="+unknownWithdrawalCode)
			require.Contains(t, logs.String(), "198.51.100.0/24")
			require.Contains(t, logs.String(), "report-error=")
			require.Contains(t, logs.String(), issuequeue.ErrStopped.Error())
			for _, engine := range []*pluginengine.Engine{original, replacement} {
				for _, call := range engine.RPCs() {
					require.NotEqual(t, rpc.MethodReportIssue, call.Method,
						"a stopped original queue refuses its late offer instead of borrowing another live reporter")
				}
			}
		})
	}
}
