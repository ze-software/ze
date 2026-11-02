//go:build ze_ssh

package hub

import (
	"testing"

	"github.com/ze-software/ze/internal/component/plugin"
)

// TestCLIExecutorCarriesTheWarning maps an SSH executor result that warns into
// what the interactive session renders.
//
// VALIDATES: the warning lines reach cli.CommandOutput beside the answer.
// PREVENTS: the interactive SSH session dropping the warning between the
// dispatcher and the model.
func TestCLIExecutorCarriesTheWarning(t *testing.T) {
	execute := cliExecutor(func(string) (*plugin.RenderedResponse, error) {
		return &plugin.RenderedResponse{
			Output:   `{"answer":"kept"}`,
			Response: &plugin.Response{Status: plugin.StatusWarning, Warnings: []string{"warning: a", "warning: b"}},
		}, nil
	})
	output, err := execute("show config dump")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if output.Text != `{"answer":"kept"}` || output.Warning != "warning: a\nwarning: b" {
		t.Errorf("output %+v, want the answer and both warning lines", output)
	}
}
