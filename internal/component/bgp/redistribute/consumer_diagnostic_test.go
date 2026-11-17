// Design: docs/guide/redistribution.md -- BGP delivery failure diagnostics.
package redistribute

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/config/redistribute"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

type consumerDiagnosticDispatcher struct {
	output      *rpc.UpdateRouteOutput
	cause       error
	reportCause error
	issues      []sdk.Issue
}

func (d *consumerDiagnosticDispatcher) UpdateRoute(context.Context, string, string) (uint32, uint32, error) {
	if d.output == nil {
		return 0, 0, d.cause
	}
	return d.output.Announced, d.output.Withdrawn, d.cause
}
func (d *consumerDiagnosticDispatcher) UpdateRouteResult(context.Context, string, string) (*rpc.UpdateRouteOutput, error) {
	return d.output, d.cause
}
func (d *consumerDiagnosticDispatcher) RaiseIssue(_ context.Context, issue sdk.Issue) error {
	d.issues = append(d.issues, issue)
	return d.reportCause
}

// This root uses only existing P1 interfaces: before the correction an actual
// operation failure produces no report despite a report-capable SDK updater.
func TestBGPConsumerFailureReachesOwningReportBus(t *testing.T) {
	for _, withdraw := range []bool{false, true} {
		t.Run(map[bool]string{false: "announce", true: "withdraw"}[withdraw], func(t *testing.T) {
			cause := errors.New("update send grant for redistribute is missing")
			dispatcher := &consumerDiagnosticDispatcher{cause: cause}
			consumer := NewBGPConsumer(dispatcher)
			if withdraw {
				consumer.WithdrawRoute(t.Context(), family.IPv4Unicast, "10.0.0.0/24")
			} else {
				consumer.InjectRoute(t.Context(), family.IPv4Unicast, redistribute.RouteEntry{Prefix: "10.0.0.0/24", Peer: "192.0.2.7"})
			}
			if len(dispatcher.issues) != 1 {
				t.Fatalf("failed operation raised %d reports, want one owning diagnostic", len(dispatcher.issues))
			}
			issue := dispatcher.issues[0]
			if issue.Code != "doctor-bgp-redistribution" || issue.Severity != rpc.ReportError {
				t.Fatalf("wrong failure owner: %+v", issue)
			}
			if !strings.Contains(issue.Message, cause.Error()) || !strings.Contains(issue.Message, "10.0.0.0/24") || !strings.Contains(issue.Message, "ipv4/unicast") {
				t.Fatalf("route or actual cause lost: %+v", issue)
			}
			if issue.Fix != "ze show bgp peers" {
				t.Fatalf("missing actionable peer inspection: %+v", issue)
			}
		})
	}
}

func TestBGPConsumerResultFailureUsesSameDiagnostic(t *testing.T) {
	dispatcher := &consumerDiagnosticDispatcher{output: &rpc.UpdateRouteOutput{Failure: "origin admission refused"}}
	NewBGPConsumer(dispatcher).InjectRoute(t.Context(), family.IPv4Unicast, redistribute.RouteEntry{Prefix: "10.0.0.0/24"})
	if len(dispatcher.issues) != 1 || !strings.Contains(dispatcher.issues[0].Message, "origin admission refused") {
		t.Fatalf("full-result cause did not reach report bus: %+v", dispatcher.issues)
	}
}

func TestBGPConsumerRetainedLoserDoesNotRaiseFailure(t *testing.T) {
	dispatcher := &consumerDiagnosticDispatcher{output: &rpc.UpdateRouteOutput{Admissions: []rpc.RouteAdmission{{Family: "ipv4/unicast", KeyNLRI: []byte{24, 10, 0, 0}, Destination: "192.0.2.7", PeerOwner: 17, Generation: 3, Outcome: rpc.RouteAdmissionRetained, Disposition: rpc.RouteAdmissionSuppressedSelection}}}}
	NewBGPConsumer(dispatcher).InjectRoute(t.Context(), family.IPv4Unicast, redistribute.RouteEntry{Prefix: "10.0.0.0/24"})
	if len(dispatcher.issues) != 0 {
		t.Fatalf("accepted retained loser was reported as failure: %+v", dispatcher.issues)
	}
	dispatcher.output = &rpc.UpdateRouteOutput{Withdrawn: 1}
	NewBGPConsumer(dispatcher).WithdrawRoute(t.Context(), family.IPv4Unicast, "10.0.0.0/24")
	if len(dispatcher.issues) != 0 {
		t.Fatalf("completed withdrawal was reported as failure: %+v", dispatcher.issues)
	}
}

func TestBGPConsumerDiagnosticIsRegistered(t *testing.T) {
	const code = "doctor-bgp-redistribution"
	if diagnostic.Lookup(code) == nil {
		t.Fatal("redistribution failure code has no operator explanation")
	}
	if !slices.Contains(diagnostic.EventCodes(), code) {
		t.Fatal("redistribution failures have no recent-events doctor owner")
	}
}

func TestBGPConsumerDiagnosticUsesActualSDKConnection(t *testing.T) {
	for _, native := range []bool{false, true} {
		t.Run(map[bool]string{false: "framed", true: "native"}[native], func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			client, server := net.Pipe()
			cause := errors.New("BGP engine refused redistribution update")
			reported := make(chan rpc.ReportIssueInput, 1)
			handle := func(method string, raw json.RawMessage) (json.RawMessage, error) {
				switch method {
				case rpc.MethodUpdateRoute:
					return nil, cause
				case rpc.MethodReportIssue:
					var issue rpc.ReportIssueInput
					if err := json.Unmarshal(raw, &issue); err != nil {
						return nil, err
					}
					reported <- issue
					return json.RawMessage(`{}`), nil
				default:
					return nil, fmt.Errorf("unexpected engine method %s", method)
				}
			}
			var engine *rpc.MuxConn
			stopped := make(chan struct{})
			connection := client
			if native {
				bridge := rpc.NewDirectBridge()
				bridge.SetDispatchRPC(func(_ context.Context, method string, raw json.RawMessage) (json.RawMessage, error) {
					return handle(method, raw)
				})
				bridge.SetReady()
				connection = rpc.NewBridgedConn(client, bridge)
				close(stopped)
			} else {
				engine = rpc.NewMuxConn(rpc.NewConn(server, server))
				go func() {
					defer close(stopped)
					for request := range engine.Requests() {
						output, err := handle(request.Method, request.Params)
						if err != nil {
							if engine.SendError(ctx, request.ID, err.Error()) != nil {
								return
							}
						} else if engine.SendResult(ctx, request.ID, output) != nil {
							return
						}
					}
				}()
			}
			updater := sdk.NewWithConn("redistribute", connection)
			t.Cleanup(func() {
				if err := updater.Close(); err != nil {
					t.Error(err)
				}
				if engine != nil {
					if err := engine.Close(); err != nil {
						t.Error(err)
					}
				}
				if err := server.Close(); err != nil {
					t.Error(err)
				}
				join, stop := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
				defer stop()
				select {
				case <-stopped:
				case <-join.Done():
					t.Error("diagnostic engine responder did not join")
				}
			})
			NewBGPConsumer(updater).WithdrawRoute(ctx, family.IPv4Unicast, "10.0.0.0/24")
			select {
			case issue := <-reported:
				if issue.Action != rpc.ReportRaise || issue.Severity != rpc.ReportError || issue.Code != "doctor-bgp-redistribution" || !strings.Contains(issue.Message, cause.Error()) || issue.Fix != "ze show bgp peers" {
					t.Fatalf("SDK lost original failure report: %+v", issue)
				}
			default:
				t.Fatal("actual updater connection never carried the failure report")
			}
		})
	}
}

func TestBGPConsumerReportRefusalKeepsOriginalFailureVisible(t *testing.T) {
	previous := logger
	t.Cleanup(func() { logger = previous })
	var buffer bytes.Buffer
	capture := slog.New(slog.NewTextHandler(&buffer, &slog.HandlerOptions{Level: slog.LevelWarn}))
	logger = func() *slog.Logger { return capture }
	cause := errors.New("original withdrawal refused")
	reportCause := errors.New("report connection unavailable")
	dispatcher := &consumerDiagnosticDispatcher{cause: cause, reportCause: reportCause}
	NewBGPConsumer(dispatcher).WithdrawRoute(t.Context(), family.IPv4Unicast, "10.0.0.0/24")
	output := buffer.String()
	if len(dispatcher.issues) != 1 || !strings.Contains(output, cause.Error()) || !strings.Contains(output, reportCause.Error()) || strings.Count(output, "code=doctor-bgp-redistribution") != 2 {
		t.Fatalf("route/report refusal was hidden or lost its doctor owner: %s", output)
	}
}
