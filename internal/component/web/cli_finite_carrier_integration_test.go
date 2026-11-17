// Design: docs/architecture/web-interface.md -- bounded finite byte carriers.
//go:build integration

package web

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
)

// TestBrowserCLIDownloadFiniteCarrierRefusesMalformed supplies malformed
// finite HTTP carriers to the actual browser consumer. Visible text is valid,
// so any fallback would enable an export of bytes the carrier never supplied.
func TestBrowserCLIDownloadFiniteCarrierRefusesMalformed(t *testing.T) {
	for _, tc := range []struct {
		name  string
		body  string
		count int
	}{
		{name: "unknown-encoding", body: `{"blob":{"encoding":"unknown","content":"unchanged"}}`, count: 9},
		{name: "invalid-base64", body: `{"blob":{"encoding":"base64","content":"!!!!"}}`, count: 3},
		{name: "missing-content", body: `{"blob":{"encoding":"utf-8"}}`, count: 9},
		{name: "missing-blob", body: `{}`, count: 9},
		{name: "base64-byte-count", body: `{"blob":{"encoding":"base64","content":"YQ=="}}`, count: 2},
		{name: "text-byte-count", body: `{"blob":{"encoding":"utf-8","content":"a"}}`, count: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body, err := json.Marshal(map[string]any{
				"output": "finite-carrier-visible-text", "path": "", "mode": terminalModeConfig,
				"output-complete": true, "output-bytes": json.RawMessage(tc.body),
				"output-byte-count": tc.count,
			})
			require.NoError(t, err)
			browser := newDownloadBrowser(t, nil, downloadStreamFixture{response: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, err := w.Write(body)
				require.NoError(t, err)
			})})
			browser.submit("run monitor download malformed-carrier")
			browser.wait(`document.getElementById('cli-output').textContent==='finite-carrier-visible-text'`, "visible finite response")
			require.Equal(t, "true", browser.evaluate(`document.querySelector('[data-cli-local-operator="save"]').disabled`), "malformed byte carriers must not export visible text instead")
			require.Equal(t, "true", browser.evaluate(`(document.getElementById('cli-feedback').textContent+' '+document.getElementById('cli-download-status').textContent).toLowerCase().includes('download')`), "the refusal must explain why no download is available")
			require.Equal(t, "0", browser.evaluate(`window.__downloadProof.clicks.length`))
		})
	}
}

// TestBrowserCLIDownloadFiniteBinaryByteLimit measures the original finite
// bytes at the 4 MiB boundary. One invalid byte expands in display JSON, which
// must neither refuse an in-budget artifact nor admit a byte over the limit.
func TestBrowserCLIDownloadFiniteBinaryByteLimit(t *testing.T) {
	for _, extra := range []int{0, 1} {
		t.Run(string(rune('0'+extra)), func(t *testing.T) {
			original := bytes.Repeat([]byte("x"), 4*1024*1024+extra)
			original[0] = 0xff
			copy(original[len(original)-len("FINITE-LIMIT-LAST"):], "FINITE-LIMIT-LAST")
			wire, err := json.Marshal(command.Blob(original))
			require.NoError(t, err)
			var calls atomic.Int32
			browser := newDownloadBrowser(t, func(context.Context, plugin.CallerIdentity, string) (*plugin.Response, error) {
				calls.Add(1)
				return plugin.NewResponse(plugin.StatusDone, plugin.RawJSON(wire)), nil
			})
			browser.submit("run show data cat meta/finite-limit | text")
			browser.wait(`document.getElementById('cli-output').textContent.endsWith('FINITE-LIMIT-LAST')`, "complete finite boundary result")
			if extra > 0 {
				require.Equal(t, "true", browser.evaluate(`document.querySelector('[data-cli-local-operator="save"]').disabled`))
				require.Equal(t, "true", browser.evaluate(`document.getElementById('cli-download-status').textContent.includes('4 MiB')`))
				require.Equal(t, "0", browser.evaluate(`window.__downloadProof.clicks.length`))
				return
			}
			require.Equal(t, "false", browser.evaluate(`document.querySelector('[data-cli-local-operator="save"]').disabled`), "the byte-exact 4 MiB result must remain downloadable")
			browser.download()
			proof := browser.proof()
			digest := sha256.Sum256(original)
			require.Equal(t, len(original), proof.Size)
			require.Equal(t, hex.EncodeToString(digest[:]), proof.SHA256)
			require.Equal(t, int32(1), calls.Load())
		})
	}
}
