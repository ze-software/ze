// Design: docs/architecture/web-interface.md -- modeled empty admin inputs.
package web

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	_ "github.com/ze-software/ze/internal/component/config/storage/cli" // the native encoder owner
	"github.com/ze-software/ze/pkg/zefs"
)

// TestAdminExecuteKeepsModeledEmptyBytes posts supplied bytes through the
// actual admin HTTP endpoint and native dispatcher, then compares the owner's
// complete frame. Empty permitted content must survive blank-control admission.
func TestAdminExecuteKeepsModeledEmptyBytes(t *testing.T) {
	renderer, err := NewRenderer()
	require.NoError(t, err)
	trees, dispatch := webGoldenInspectionParts(t)
	server := httptest.NewServer(HandleAdminExecute(renderer, dispatch, trees))
	defer server.Close()

	for _, test := range []struct {
		name string
		data []byte
	}{
		{name: "empty content", data: []byte{}},
		{name: "binary and whitespace content", data: []byte{0, 255, 128, 1, ' ', '\n', '\t'}},
	} {
		t.Run(test.name, func(t *testing.T) {
			form := url.Values{"content": {base64.StdEncoding.EncodeToString(test.data)}}
			request, err := http.NewRequestWithContext(t.Context(), http.MethodPost,
				server.URL+"/admin/show/data/encode?format=json", strings.NewReader(form.Encode()))
			require.NoError(t, err)
			request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			response, err := server.Client().Do(request)
			require.NoError(t, err)
			defer func() { require.NoError(t, response.Body.Close()) }()
			require.Equal(t, http.StatusOK, response.StatusCode)
			var result struct {
				Output string `json:"output"`
				Error  bool   `json:"error"`
			}
			require.NoError(t, json.NewDecoder(response.Body).Decode(&result))
			require.False(t, result.Error, result.Output)
			var envelope struct {
				Blob json.RawMessage `json:"blob"`
			}
			require.NoError(t, json.Unmarshal([]byte(result.Output), &envelope))
			got, err := command.DecodeBlob(envelope.Blob)
			require.NoError(t, err)
			want, err := zefs.EncodeNetcapstring(test.data, len(test.data)+len(test.data)/10)
			require.NoError(t, err)
			require.Equal(t, want, []byte(got))
		})
	}
}
