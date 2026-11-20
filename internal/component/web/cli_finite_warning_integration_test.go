// Design: docs/architecture/web-interface.md -- warnings stay outside finite exports.
//go:build integration

package web

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
)

// TestBrowserCLIDownloadFiniteWarningKeepsSelectedBytes observes the real
// terminal HTTP response, warning display and native download Blob together.
// A warning cannot replace or prefix the selected binary or JSON answer.
func TestBrowserCLIDownloadFiniteWarningKeepsSelectedBytes(t *testing.T) {
	original := []byte("WARNING-FIRST\xff\x00\xc3(\nWARNING-LAST")
	wire, err := json.Marshal(command.Blob(original))
	require.NoError(t, err)
	var document map[string]any
	require.NoError(t, json.Unmarshal(wire, &document))
	compact, err := json.Marshal(document)
	require.NoError(t, err)
	for _, format := range []struct {
		name string
		pipe string
		want []byte
	}{
		{name: "default", want: original},
		{name: "text", pipe: " | text", want: original},
		{name: "json", pipe: " | json compact", want: compact},
	} {
		t.Run(format.name, func(t *testing.T) {
			var calls atomic.Int32
			browser := newDownloadBrowser(t, func(_ context.Context, caller plugin.CallerIdentity, input string) (*plugin.Response, error) {
				calls.Add(1)
				require.Equal(t, "alice", caller.Username)
				require.Equal(t, "show data cat meta/finite-warning-blob"+format.pipe, input, "the terminal dispatches the line as typed")
				response := plugin.NewResponse(plugin.StatusWarning, plugin.RawJSON(wire))
				response.Warnings = []string{"warning: the snapshot excludes expired records", "run show data cat meta/finite-warning-blob"}
				return response, nil
			})
			browser.submit("run show data cat meta/finite-warning-blob" + format.pipe)
			browser.wait(`document.getElementById('cli-output').textContent.includes('warning: the snapshot excludes expired records')&&document.getElementById('cli-output').textContent.includes('run show data cat meta/finite-warning-blob')`, "visible finite warning and recovery action")
			browser.download()
			proof := browser.proof()
			digest := sha256.Sum256(format.want)
			require.Equal(t, len(format.want), proof.Size)
			require.Equal(t, hex.EncodeToString(digest[:]), proof.SHA256)
			require.Equal(t, 1, proof.Clicks, "the real download control must activate its native anchor")
			require.Equal(t, int32(1), calls.Load(), "download must not dispatch the command again")
		})
	}
}
