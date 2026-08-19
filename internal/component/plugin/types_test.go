package plugin

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMapResponseData(t *testing.T) {
	m := Map{"key": "value", "count": 42}
	var _ ResponseData = m

	data, err := json.Marshal(m)
	require.NoError(t, err)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal(data, &parsed))
	assert.Equal(t, "value", parsed["key"])
	assert.Equal(t, float64(42), parsed["count"])
}

func TestStructResponseData(t *testing.T) {
	type sample struct {
		DataMarker
		Name  string `json:"name"`
		Count int    `json:"count"`
	}
	s := sample{Name: "test", Count: 5}
	var _ ResponseData = s

	data, err := json.Marshal(s)
	require.NoError(t, err)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal(data, &parsed))
	assert.Equal(t, "test", parsed["name"])
	assert.Equal(t, float64(5), parsed["count"])
	assert.NotContains(t, parsed, "DataMarker")
}

func TestRouteResultResponseData(t *testing.T) {
	r := RouteResult{Announced: 10, Withdrawn: 2, Warnings: []string{"w1"}}
	var _ ResponseData = r

	data, err := json.Marshal(r)
	require.NoError(t, err)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal(data, &parsed))
	assert.Equal(t, float64(10), parsed["announced"])
	assert.Equal(t, float64(2), parsed["withdrawn"])
}

func TestSliceResponseData(t *testing.T) {
	type item struct {
		DataMarker
		Name string `json:"name"`
	}
	s := Slice[item]{{Name: "a"}, {Name: "b"}}
	var _ ResponseData = s

	data, err := json.Marshal(s)
	require.NoError(t, err)

	var parsed []map[string]any
	require.NoError(t, json.Unmarshal(data, &parsed))
	require.Len(t, parsed, 2)
	assert.Equal(t, "a", parsed[0]["name"])
	assert.Equal(t, "b", parsed[1]["name"])
}

func TestNewErrorResponse(t *testing.T) {
	resp := newErrorResponse("connection refused")
	assert.Equal(t, StatusError, resp.Status)
	assert.Equal(t, "connection refused", resp.Error)
	assert.Nil(t, resp.Data)
}

func TestNewResponse(t *testing.T) {
	resp := NewResponse(StatusDone, Map{"ok": true})
	assert.Equal(t, StatusDone, resp.Status)
	assert.Equal(t, Map{"ok": true}, resp.Data)
	assert.Empty(t, resp.Error)
}

func TestResponseMarshalFormat(t *testing.T) {
	tests := []struct {
		name     string
		resp     *Response
		wantKeys []string
	}{
		{
			name: "done_with_data",
			resp: &Response{
				Serial: "1",
				Status: StatusDone,
				Data:   Map{"count": 42},
			},
			wantKeys: []string{"serial", "status", "data"},
		},
		{
			name: "error_response",
			resp: &Response{
				Serial: "2",
				Status: StatusError,
				Error:  "something went wrong",
			},
			wantKeys: []string{"serial", "status", "error"},
		},
		{
			name: "partial_streaming",
			resp: &Response{
				Serial:  "3",
				Status:  "ack",
				Partial: true,
				Data:    Map{"chunk": 1},
			},
			wantKeys: []string{"serial", "status", "partial", "data"},
		},
		{
			name:     "no_serial_no_data",
			resp:     &Response{Status: StatusDone},
			wantKeys: []string{"status"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.resp)
			require.NoError(t, err)

			var result map[string]any
			require.NoError(t, json.Unmarshal(data, &result))

			for _, key := range tt.wantKeys {
				assert.Contains(t, result, key, "response should contain %s", key)
			}
		})
	}
}

func TestCBOREncodingRemoved(t *testing.T) {
	_, err := parseWireEncoding("cbor")
	require.Error(t, err, "CBOR encoding should not be accepted")
	assert.Contains(t, err.Error(), "invalid wire encoding")
}

// VALIDATES: RawJSON refuses a payload that is not JSON (owner decision,
//
//	2026-08-19). It quoted one until then, which made finished text a
//	valid-looking answer that no format can render.
//
// PREVENTS: a handler pre-rendering its answer and every reader losing
//
//	`| yaml` and `| table` (ai/rules/cli.md).
func TestRawJSONRefusesNonJSON(t *testing.T) {
	_, err := json.Marshal(RawJSON("reload initiated"))
	require.Error(t, err, "text must not marshal as a quoted string")
	assert.Contains(t, err.Error(), "response payload is not JSON")

	// An empty payload is not an error: ExecuteCommandOutput.Data is omitempty,
	// so a command that answers with no data arrives here as "".
	empty, err := json.Marshal(RawJSON(""))
	require.NoError(t, err)
	assert.JSONEq(t, "null", string(empty))

	// A JSON value passes through verbatim, object and array alike.
	obj, err := json.Marshal(RawJSON(`{"peer-count":3}`))
	require.NoError(t, err)
	assert.JSONEq(t, `{"peer-count":3}`, string(obj))
}
