// Design: docs/architecture/web-interface.md -- byte-exact finite browser exports.
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

// TestBrowserCLIDownloadFiniteBinaryBlob follows the actual finite HTTP
// handler and embedded browser downloader with the owner's Blob encoding.
// The native anchor observer hashes the real exported Blob; neither a raw
// stream fixture nor a replacement JavaScript exporter supplies the bytes.
func TestBrowserCLIDownloadFiniteBinaryBlob(t *testing.T) {
	original := []byte("FINITE-FIRST\xff\x00\xc3(\nFINITE-LAST")
	wire, err := json.Marshal(command.Blob(original))
	require.NoError(t, err)
	var document map[string]any
	require.NoError(t, json.Unmarshal(wire, &document))
	compact, err := json.Marshal(document)
	require.NoError(t, err)
	for _, tc := range []struct {
		name    string
		pipe    string
		warning string
		want    []byte
	}{
		{name: "default", want: original},
		{name: "text", pipe: " | text", want: original},
		{name: "json", pipe: " | json compact", want: compact},
		{name: "warning", pipe: " | text", warning: "warning: retained detail", want: original},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32
			browser := newDownloadBrowser(t, func(_ context.Context, caller plugin.CallerIdentity, input string) (*plugin.Response, error) {
				calls.Add(1)
				require.Equal(t, "alice", caller.Username)
				require.Equal(t, "show data cat meta/finite-blob", input)
				response := plugin.NewResponse(plugin.StatusDone, plugin.RawJSON(wire))
				if tc.warning != "" {
					response.Status = plugin.StatusWarning
					response.Warnings = []string{tc.warning}
				}
				return response, nil
			})
			browser.submit("run show data cat meta/finite-blob" + tc.pipe)
			browser.wait(`document.getElementById('cli-output').textContent.includes('FINITE-LAST')||document.getElementById('cli-output').textContent.includes('RklOSVRFLUZJUlNU')`, "finite Blob display")
			if tc.warning != "" {
				warning, err := json.Marshal(tc.warning)
				require.NoError(t, err)
				browser.wait(`document.getElementById('cli-output').textContent.includes(`+string(warning)+`)`, "finite warning display")
			}
			browser.download()
			proof := browser.proof()
			digest := sha256.Sum256(tc.want)
			require.Equal(t, len(tc.want), proof.Size)
			require.Equal(t, hex.EncodeToString(digest[:]), proof.SHA256)
			require.Equal(t, int32(1), calls.Load(), "download must not dispatch the finite producer again")
		})
	}
}
