// Design: docs/architecture/web-interface.md -- supplied completion inputs.
package web

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestSuppliedCompletionHTTPInputs takes ordinary editing text through the
// generated file/paste controls and the real registered completion handler.
func TestSuppliedCompletionHTTPInputs(t *testing.T) {
	const commandPath = "show config supplied completion"
	const configuration = "bgp { router-id 192.0.2.1; }"
	renderer, _, _, _ := webGoldenParts(t)
	trees, dispatch := webGoldenInspectionParts(t)
	manager := &EditorManager{}
	for _, file := range []bool{false, true} {
		form := url.Values{webCommandField: {commandPath}, "configuration": {configuration}, "input": {"set router"}, "context": {"bgp"}, "ghost": {boolTrue}}
		request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/config/inspect", strings.NewReader(form.Encode()))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		if file {
			var body bytes.Buffer
			writer := multipart.NewWriter(&body)
			for key, values := range form {
				if key != "configuration" {
					require.NoError(t, writer.WriteField(key, values[0]))
				}
			}
			part, err := writer.CreateFormFile("configuration-file", "caller.conf")
			require.NoError(t, err)
			_, err = part.Write([]byte(configuration))
			require.NoError(t, err)
			require.NoError(t, writer.Close())
			request = httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/config/inspect", &body)
			request.Header.Set("Content-Type", writer.FormDataContentType())
		}
		request = request.WithContext(withUsername(request.Context(), "completion-reader"))
		response := httptest.NewRecorder()
		HandleSuppliedInspection(renderer, manager, dispatch, trees, nil)(response, request)
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		require.Contains(t, response.Body.String(), "-id")
		require.NotContains(t, response.Body.String(), "could not be completed")
	}
}

// TestSuppliedCompletionGeneratedControls inspects the actual authenticated GET
// response so field metadata alone cannot stand in for usable rendered inputs.
func TestSuppliedCompletionGeneratedControls(t *testing.T) {
	renderer, _, _, _ := webGoldenParts(t)
	trees, _ := webGoldenInspectionParts(t)
	query := url.Values{webCommandField: {"show config supplied completion"}}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/config/inspect?"+query.Encode(), http.NoBody)
	request = request.WithContext(withUsername(request.Context(), "completion-reader"))
	response := httptest.NewRecorder()
	HandleSuppliedInspectionForm(renderer, trees)(response, request)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	for _, name := range []string{"configuration", "configuration-file", "input", "input-file", "context", "ghost"} {
		require.Contains(t, response.Body.String(), `name="`+name+`"`)
	}
	require.NotContains(t, response.Body.String(), `name="content"`)
}
