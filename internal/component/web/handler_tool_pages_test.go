package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
)

// mockDispatcher returns a CommandDispatcher that records the command and
// returns a fixed output string with no error.
func mockDispatcher(output string) (CommandDispatcher, *string) {
	var captured string
	d := func(command, _, _ string) (string, error) {
		captured = command
		return output, nil
	}
	return d, &captured
}

// workbenchForTools creates a workbench handler with a mock dispatcher for testing.
func workbenchForTools(t *testing.T, dispatch CommandDispatcher) http.HandlerFunc {
	t.Helper()
	renderer, err := NewRenderer()
	require.NoError(t, err)
	schema, schemaErr := config.YANGSchema()
	require.NoError(t, schemaErr)
	tree := config.NewTree()
	return HandleWorkbench(renderer, schema, tree, nil, true, WithDispatch(dispatch))
}

// --- Ping ---

func TestToolPingPageRendersForm(t *testing.T) {
	handler := workbenchForTools(t, nil)
	req := httptest.NewRequest(http.MethodGet, "/show/tools/ping/", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	html := rec.Body.String()
	assert.Contains(t, html, "Ping")
	assert.Contains(t, html, `name="destination"`)
	assert.Contains(t, html, `name="count"`)
	assert.Contains(t, html, `name="timeout"`)
	assert.Contains(t, html, "wb-tool-form")
}

func TestToolPingDispatchesCommand(t *testing.T) {
	dispatch, captured := mockDispatcher("PING 192.0.2.1: 5 packets")
	handler := workbenchForTools(t, dispatch)

	body := "destination=192.0.2.1&count=5&timeout=5"
	req := httptest.NewRequest(http.MethodPost, "/show/tools/ping/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "show ping 192.0.2.1 count 5 timeout 5s", *captured)
	assert.Contains(t, rec.Body.String(), "PING 192.0.2.1")
}

func TestToolPingValidatesInput(t *testing.T) {
	dispatch, captured := mockDispatcher("")
	handler := workbenchForTools(t, dispatch)

	// Empty destination.
	body := "destination=&count=5&timeout=5"
	req := httptest.NewRequest(http.MethodPost, "/show/tools/ping/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Destination is required")
	assert.Empty(t, *captured, "should not dispatch on validation error")
}

func TestToolPingValidatesIPAddress(t *testing.T) {
	dispatch, captured := mockDispatcher("")
	handler := workbenchForTools(t, dispatch)

	body := "destination=not-an-ip&count=5&timeout=5"
	req := httptest.NewRequest(http.MethodPost, "/show/tools/ping/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Invalid IP address")
	assert.Empty(t, *captured, "should not dispatch on invalid IP")
}

func TestToolPingBoundaryCount(t *testing.T) {
	tests := []struct {
		name    string
		count   string
		wantErr bool
	}{
		{"count=0 invalid", "0", true},
		{"count=1 valid", "1", false},
		{"count=100 valid", "100", false},
		{"count=101 invalid", "101", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dispatch, captured := mockDispatcher("ok")
			handler := workbenchForTools(t, dispatch)

			body := "destination=192.0.2.1&count=" + tt.count + "&timeout=5"
			req := httptest.NewRequest(http.MethodPost, "/show/tools/ping/", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req.Header.Set("HX-Request", "true")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if tt.wantErr {
				assert.Contains(t, rec.Body.String(), "Count must be")
				assert.Empty(t, *captured)
			} else {
				assert.NotEmpty(t, *captured)
			}
		})
	}
}

func TestToolPingBoundaryTimeout(t *testing.T) {
	tests := []struct {
		name    string
		timeout string
		wantErr bool
	}{
		{"timeout=0 invalid", "0", true},
		{"timeout=1 valid", "1", false},
		{"timeout=30 valid", "30", false},
		{"timeout=31 invalid", "31", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dispatch, captured := mockDispatcher("ok")
			handler := workbenchForTools(t, dispatch)

			body := "destination=192.0.2.1&count=5&timeout=" + tt.timeout
			req := httptest.NewRequest(http.MethodPost, "/show/tools/ping/", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req.Header.Set("HX-Request", "true")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if tt.wantErr {
				assert.Contains(t, rec.Body.String(), "Timeout must be")
				assert.Empty(t, *captured)
			} else {
				assert.NotEmpty(t, *captured)
			}
		})
	}
}

// --- BGP Decode ---

func TestToolBGPDecodePageRendersForm(t *testing.T) {
	handler := workbenchForTools(t, nil)
	req := httptest.NewRequest(http.MethodGet, "/show/tools/bgp-decode/", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	html := rec.Body.String()
	assert.Contains(t, html, "BGP Decode")
	assert.Contains(t, html, `name="hex"`)
	assert.Contains(t, html, "textarea")
}

func TestToolBGPDecodeDispatchesCommand(t *testing.T) {
	dispatch, captured := mockDispatcher("decoded output")
	handler := workbenchForTools(t, dispatch)

	body := "hex=FFFFFFFF00000017"
	req := httptest.NewRequest(http.MethodPost, "/show/tools/bgp-decode/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "show bgp/decode FFFFFFFF00000017", *captured)
	assert.Contains(t, rec.Body.String(), "decoded output")
}

func TestToolBGPDecodeValidatesHex(t *testing.T) {
	dispatch, captured := mockDispatcher("")
	handler := workbenchForTools(t, dispatch)

	body := "hex=ZZZZ_not_hex"
	req := httptest.NewRequest(http.MethodPost, "/show/tools/bgp-decode/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "hexadecimal")
	assert.Empty(t, *captured)
}

func TestToolBGPDecodeValidatesEmpty(t *testing.T) {
	dispatch, captured := mockDispatcher("")
	handler := workbenchForTools(t, dispatch)

	body := "hex="
	req := httptest.NewRequest(http.MethodPost, "/show/tools/bgp-decode/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Contains(t, rec.Body.String(), "Hex input is required")
	assert.Empty(t, *captured)
}

// --- Metrics Query ---

func TestToolMetricsPageRendersForm(t *testing.T) {
	handler := workbenchForTools(t, nil)
	req := httptest.NewRequest(http.MethodGet, "/show/tools/metrics/", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	html := rec.Body.String()
	assert.Contains(t, html, "Metrics Query")
	assert.Contains(t, html, `name="name"`)
	assert.Contains(t, html, `name="label"`)
}

func TestToolMetricsDispatchesCommand(t *testing.T) {
	dispatch, captured := mockDispatcher("metric_value 42")
	handler := workbenchForTools(t, dispatch)

	body := "name=bgp_peer_up&label=instance%3Dpeer1"
	req := httptest.NewRequest(http.MethodPost, "/show/tools/metrics/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "show metrics-query bgp_peer_up instance=peer1", *captured)
	assert.Contains(t, rec.Body.String(), "metric_value 42")
}

func TestToolMetricsValidatesName(t *testing.T) {
	dispatch, captured := mockDispatcher("")
	handler := workbenchForTools(t, dispatch)

	body := "name="
	req := httptest.NewRequest(http.MethodPost, "/show/tools/metrics/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Contains(t, rec.Body.String(), "Metric name is required")
	assert.Empty(t, *captured)
}

func TestToolMetricsValidatesNameFormat(t *testing.T) {
	dispatch, captured := mockDispatcher("")
	handler := workbenchForTools(t, dispatch)

	body := "name=invalid%21name"
	req := httptest.NewRequest(http.MethodPost, "/show/tools/metrics/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Contains(t, rec.Body.String(), "alphanumeric")
	assert.Empty(t, *captured)
}

func TestToolMetricsValidatesNameLength(t *testing.T) {
	dispatch, captured := mockDispatcher("")
	handler := workbenchForTools(t, dispatch)

	longName := strings.Repeat("a", maxMetricNameLen+1)
	body := "name=" + longName
	req := httptest.NewRequest(http.MethodPost, "/show/tools/metrics/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Contains(t, rec.Body.String(), "maximum length")
	assert.Empty(t, *captured)
}

// --- Capture ---

func TestToolCapturePageRendersForm(t *testing.T) {
	handler := workbenchForTools(t, nil)
	req := httptest.NewRequest(http.MethodGet, "/show/tools/capture/", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	html := rec.Body.String()
	assert.Contains(t, html, "Capture")
	assert.Contains(t, html, `name="tunnel-id"`)
	assert.Contains(t, html, `name="peer"`)
	assert.Contains(t, html, `name="count"`)
}

func TestToolCaptureDispatchesCommand(t *testing.T) {
	dispatch, captured := mockDispatcher("captured 5 packets")
	handler := workbenchForTools(t, dispatch)

	body := "tunnel-id=100&peer=192.0.2.1&count=50"
	req := httptest.NewRequest(http.MethodPost, "/show/tools/capture/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "show capture tunnel-id 100 peer 192.0.2.1 count 50", *captured)
	assert.Contains(t, rec.Body.String(), "captured 5 packets")
}

func TestToolCaptureBoundaryTunnelID(t *testing.T) {
	tests := []struct {
		name     string
		tunnelID string
		wantErr  bool
	}{
		{"tunnel-id=65535 valid", "65535", false},
		{"tunnel-id=65536 invalid", "65536", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dispatch, captured := mockDispatcher("ok")
			handler := workbenchForTools(t, dispatch)

			body := "tunnel-id=" + tt.tunnelID + "&count=100"
			req := httptest.NewRequest(http.MethodPost, "/show/tools/capture/", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req.Header.Set("HX-Request", "true")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if tt.wantErr {
				assert.Contains(t, rec.Body.String(), "Tunnel ID must be")
				assert.Empty(t, *captured)
			} else {
				assert.NotEmpty(t, *captured)
			}
		})
	}
}

func TestToolCaptureBoundaryCount(t *testing.T) {
	tests := []struct {
		name    string
		count   string
		wantErr bool
	}{
		{"count=0 invalid", "0", true},
		{"count=1 valid", "1", false},
		{"count=10000 valid", "10000", false},
		{"count=10001 invalid", "10001", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dispatch, captured := mockDispatcher("ok")
			handler := workbenchForTools(t, dispatch)

			body := "count=" + tt.count
			req := httptest.NewRequest(http.MethodPost, "/show/tools/capture/", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req.Header.Set("HX-Request", "true")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if tt.wantErr {
				assert.Contains(t, rec.Body.String(), "Count must be")
				assert.Empty(t, *captured)
			} else {
				assert.NotEmpty(t, *captured)
			}
		})
	}
}

// --- Output truncation ---

func TestToolOutputTruncation(t *testing.T) {
	// Create output exceeding the 4 MiB cap.
	bigOutput := strings.Repeat("A", relatedOverlayMaxBufBytes+1024)
	dispatch, _ := mockDispatcher(bigOutput)
	handler := workbenchForTools(t, dispatch)

	body := "destination=192.0.2.1&count=1&timeout=1"
	req := httptest.NewRequest(http.MethodPost, "/show/tools/ping/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "truncated")
}
