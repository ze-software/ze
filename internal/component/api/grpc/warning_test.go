package grpc

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ze-software/ze/internal/component/api"
	"github.com/ze-software/ze/internal/component/plugin"
)

// TestExecResultToProtoCarriesWarnings converts a response that warns about
// its answer into the gRPC message a client receives.
//
// VALIDATES: CommandResponse.warnings holds every line `ze <command>` prints
// on stderr, beside the status and the data.
// PREVENTS: the gRPC surface dropping a warning the other surfaces show.
func TestExecResultToProtoCarriesWarnings(t *testing.T) {
	resp := execResultToProto(&api.ExecResult{
		Status:   plugin.StatusWarning,
		Data:     plugin.Map{"answer": "kept"},
		Warnings: []string{"warning: first line", "warning: second line"},
	})
	assert.Equal(t, plugin.StatusWarning, resp.GetStatus())
	assert.JSONEq(t, `{"answer":"kept"}`, string(resp.GetData()))
	assert.Equal(t, []string{"warning: first line", "warning: second line"}, resp.GetWarnings())
}
