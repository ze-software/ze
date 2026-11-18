// Design: docs/architecture/web-interface.md -- lossless supplied byte inputs.
package web

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	_ "github.com/ze-software/ze/internal/component/config/storage/cli" // byte operation owner
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/pkg/zefs"
)

func suppliedBytesRequest(t *testing.T, commandPath string, data []byte, file bool) *http.Request {
	t.Helper()
	if !file {
		form := url.Values{webCommandField: {commandPath}, "content": {string(data)}}
		request := httptest.NewRequest(http.MethodPost, "/config/inspect", strings.NewReader(form.Encode()))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		require.NoError(t, readSuppliedInspectionForm(request))
		return request
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField(webCommandField, commandPath))
	part, err := writer.CreateFormFile(webFileField, "bytes.bin")
	require.NoError(t, err)
	_, err = part.Write(data)
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	request := httptest.NewRequest(http.MethodPost, "/config/inspect", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	require.NoError(t, readSuppliedInspectionForm(request))
	return request
}

// TestSuppliedByteInputsReachOwnedEncoder takes native form/file bytes through
// command composition and the real dispatcher, comparing the actual frame.
func TestSuppliedByteInputsReachOwnedEncoder(t *testing.T) {
	const path = "show data encode"
	trees, dispatch := webGoldenInspectionParts(t)
	selected, err := suppliedInspectionSelection(trees, path)
	require.NoError(t, err)
	for _, test := range []struct {
		name string
		data []byte
		file bool
	}{
		{name: "empty file", data: []byte{}, file: true},
		{name: "whitespace file", data: []byte(" \n\t"), file: true},
		{name: "whitespace paste", data: []byte(" \n\t")},
		{name: "binary file", data: []byte{0, 255, 128, 1}, file: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := suppliedBytesRequest(t, path, test.data, test.file)
			line, err := selected.invocation(request)
			require.NoError(t, err)
			response, err := dispatch(context.Background(), plugin.CallerIdentity{Username: "byte-reader"}, line)
			require.NoError(t, err)
			encoded, err := json.Marshal(response.Data)
			require.NoError(t, err)
			var envelope struct {
				Blob json.RawMessage `json:"blob"`
			}
			require.NoError(t, json.Unmarshal(encoded, &envelope))
			got, err := command.DecodeBlob(envelope.Blob)
			require.NoError(t, err)
			want, err := zefs.EncodeNetcapstring(test.data, len(test.data)+len(test.data)/10)
			require.NoError(t, err)
			require.Equal(t, want, []byte(got))
		})
	}
}

// TestSuppliedByteInputsKeepOwnerRefusalsAndConflicts retains configuration
// ownership of empty-source refusal and rejects mixed file/paste submissions.
func TestSuppliedByteInputsKeepOwnerRefusalsAndConflicts(t *testing.T) {
	trees, _ := webGoldenInspectionParts(t)
	config, err := suppliedInspectionSelection(trees, "show config supplied validation")
	require.NoError(t, err)
	_, err = config.invocation(suppliedBytesRequest(t, config.path, []byte{}, true))
	require.Error(t, err)
	encoder, err := suppliedInspectionSelection(trees, "show data encode")
	require.NoError(t, err)
	request := suppliedBytesRequest(t, encoder.path, []byte("one"), true)
	request.PostForm.Set("content", " ")
	_, err = encoder.invocation(request)
	require.Error(t, err)
	require.Contains(t, err.Error(), "without combining both")
}
