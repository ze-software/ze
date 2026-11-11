// Design: docs/architecture/api/process-protocol.md -- problems a plugin reports to the daemon.
// Related: report.go -- RaiseIssue and ClearIssue under test

package sdk

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestRaiseIssueSendsReportIssue proves RaiseIssue and ClearIssue each send
// one report-issue request carrying the fields the plugin gave and no source,
// and return the engine's answer.
//
// The method plays the engine on the other end of the connection, reads each
// request, checks its method and decoded body, and answers it.
func TestRaiseIssueSendsReportIssue(t *testing.T) {
	t.Parallel()

	p, engine := newTestPair(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	issue := Issue{
		Severity: rpc.ReportWarning,
		Code:     "doctor-internal-defect",
		Subject:  "peer 192.0.2.1",
		Message:  "the decoder met a state only a defect produces",
		Fix:      "show warnings",
	}
	result := make(chan error, 1)
	go func() { result <- p.RaiseIssue(ctx, issue) }()

	req := readEngineRequest(t, ctx, engine.mux)
	require.Equal(t, rpc.MethodReportIssue, req.Method)
	var got map[string]any
	require.NoError(t, json.Unmarshal(req.Params, &got))
	assert.Equal(t, map[string]any{
		"action":   "raise",
		"severity": "warning",
		"code":     "doctor-internal-defect",
		"subject":  "peer 192.0.2.1",
		"message":  "the decoder met a state only a defect produces",
		"fix":      "show warnings",
	}, got, "the wire names are kebab-case and carry no source")
	require.NoError(t, engine.mux.SendOK(ctx, req.ID))
	require.NoError(t, <-result)

	go func() { result <- p.ClearIssue(ctx, issue.Code, issue.Subject) }()
	req = readEngineRequest(t, ctx, engine.mux)
	require.Equal(t, rpc.MethodReportIssue, req.Method)
	got = nil
	require.NoError(t, json.Unmarshal(req.Params, &got))
	assert.Equal(t, map[string]any{"action": "clear", "code": "doctor-internal-defect", "subject": "peer 192.0.2.1"}, got)
	require.NoError(t, engine.mux.SendError(ctx, req.ID, "refused for the test"))
	err := <-result
	require.Error(t, err)
	assert.Contains(t, err.Error(), "refused for the test", "the engine's refusal reaches the caller")
}

// TestRaiseIssueRefusedBeforeSend proves a request the wire bounds refuse is
// refused by the SDK, naming the field and the bound, before it is sent.
//
// The engine side never answers, so a request that was sent would end on the
// context deadline instead of with a refusal naming the field.
func TestRaiseIssueRefusedBeforeSend(t *testing.T) {
	t.Parallel()

	p, _ := newTestPair(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := p.RaiseIssue(ctx, Issue{
		Severity: rpc.ReportWarning,
		Code:     "doctor-internal-defect",
		Subject:  "peer 192.0.2.1",
		Message:  "m",
		Fix:      strings.Repeat("f", rpc.ReportFixMax+1),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "fix")
	assert.Contains(t, err.Error(), "256")

	err = p.ClearIssue(ctx, "doctor-internal-defect", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "subject")
	require.NoError(t, ctx.Err(), "both refusals came before the deadline, so nothing waited on the engine")
}
