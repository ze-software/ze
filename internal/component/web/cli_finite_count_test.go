// Design: docs/architecture/web-interface.md -- bounded finite export carriers.
package web

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
)

// TestWebCLIFiniteBlobCarrierBound observes the original byte count and the
// actual carrier budget at HTTP, including a complete empty result. Display
// JSON expansion must not change either outcome at the published 4 MiB bound.
func TestWebCLIFiniteBlobCarrierBound(t *testing.T) {
	for _, size := range []int{0, 4 * 1024 * 1024, 4*1024*1024 + 1} {
		t.Run(strconv.Itoa(size), func(t *testing.T) {
			original := bytes.Repeat([]byte("x"), size)
			if size > 0 {
				original[0] = 0xff
			}
			wire, err := json.Marshal(command.Blob(original))
			require.NoError(t, err)
			mgr, schema, tree, _ := setupCLITerminalYANGTest(t)
			handler := HandleCLITerminalWithDispatchAuthorizerAndAudit(mgr, schema, tree, func(context.Context, plugin.CallerIdentity, string) (*plugin.Response, error) {
				return plugin.NewResponse(plugin.StatusDone, plugin.RawJSON(wire)), nil
			}, nil, nil)
			recorder := httptest.NewRecorder()
			handler(recorder, authedRequest(t, http.MethodPost, "/cli/terminal", url.Values{
				"command": {"run show data cat meta/finite-bound | text"},
			}))
			var answer struct {
				Count    *int            `json:"output-byte-count"`
				Bytes    json.RawMessage `json:"output-bytes"`
				Complete bool            `json:"output-complete"`
			}
			require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &answer))
			require.True(t, answer.Complete)
			require.NotNil(t, answer.Count, "zero bytes must be a measured result, not an absent count")
			require.Equal(t, size, *answer.Count)
			if size > 4*1024*1024 {
				require.Empty(t, answer.Bytes, "an oversized finite result must not duplicate its bytes in a carrier")
				return
			}
			var carrier struct {
				Blob json.RawMessage `json:"blob"`
			}
			require.NoError(t, json.Unmarshal(answer.Bytes, &carrier))
			exported, err := command.DecodeBlob(carrier.Blob)
			require.NoError(t, err)
			require.Equal(t, original, []byte(exported))
		})
	}
}
