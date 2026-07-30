package runner

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// seenRequest records what a captureHeaders server observed on its last request.
type seenRequest struct {
	header http.Header
	host   string
}

// captureHeaders starts a test server that records the headers of every request
// it receives and always answers 200 with the given body.
func captureHeaders(t *testing.T, body string) (*httptest.Server, *seenRequest) {
	t.Helper()
	seen := &seenRequest{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen.header = r.Header.Clone()
		seen.host = r.Host
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(body)); err != nil {
			t.Errorf("write response body: %v", err)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, seen
}

// TestHTTPCheckSendsHeaders verifies that http= header= keys reach the server.
//
// VALIDATES: parsed Headers are applied to the outgoing request by
// executeOneHTTPCheck, so a .ci author's header= is what the server receives.
// PREVENTS: header= parsing correctly but never being wired to the request, so a
// .ci test that sets a mandatory header (MCP-Protocol-Version) silently sends none.
func TestHTTPCheckSendsHeaders(t *testing.T) {
	srv, seen := captureHeaders(t, "ok")
	r := &Runner{}

	chk := httpCheck{
		Seq:    1,
		Method: "get",
		Status: 200,
		Headers: []httpHeader{
			{Name: "MCP-Protocol-Version", Value: "2026-07-28"},
			{Name: "Mcp-Method", Value: "tools/list"},
			{Name: "Mcp-Name", Value: "ze"},
		},
	}
	require.NoError(t, r.executeOneHTTPCheck(context.Background(), srv.Client(), &chk, srv.URL))

	assert.Equal(t, "2026-07-28", seen.header.Get("MCP-Protocol-Version"))
	assert.Equal(t, "tools/list", seen.header.Get("Mcp-Method"))
	assert.Equal(t, "ze", seen.header.Get("Mcp-Name"))
}

// TestHTTPCheckHeaderOverridesContentType verifies precedence between the
// sendfile default Content-Type and an explicit header= for the same field.
//
// VALIDATES: an explicit header=Content-Type: wins over the application/json
// default applied for sendfile bodies, and leaves exactly one value.
// PREVENTS: the default being applied last (so the authored header is ignored),
// or being appended (so the request carries two conflicting Content-Type values).
func TestHTTPCheckHeaderOverridesContentType(t *testing.T) {
	srv, seen := captureHeaders(t, "ok")
	r := &Runner{}

	sendFile := filepath.Join(t.TempDir(), "body.json")
	require.NoError(t, os.WriteFile(sendFile, []byte(`{"jsonrpc":"2.0"}`), 0o600))

	chk := httpCheck{
		Seq:      1,
		Method:   "post",
		Status:   200,
		SendFile: sendFile,
		Headers:  []httpHeader{{Name: "Content-Type", Value: "application/vnd.custom+json"}},
	}
	require.NoError(t, r.executeOneHTTPCheck(context.Background(), srv.Client(), &chk, srv.URL))

	assert.Equal(t, []string{"application/vnd.custom+json"}, seen.header.Values("Content-Type"))
}

// TestHTTPCheckRepeatedHeaderNameAccumulates verifies multi-value field handling.
//
// VALIDATES: repeating header= with the same field name sends every value,
// rather than the last one replacing the earlier ones.
// PREVENTS: a blanket Set() collapsing a legitimately repeated field.
func TestHTTPCheckRepeatedHeaderNameAccumulates(t *testing.T) {
	srv, seen := captureHeaders(t, "ok")
	r := &Runner{}

	chk := httpCheck{
		Seq:    1,
		Method: "get",
		Status: 200,
		Headers: []httpHeader{
			{Name: "X-Ze-Tag", Value: "first"},
			{Name: "X-Ze-Tag", Value: "second"},
		},
	}
	require.NoError(t, r.executeOneHTTPCheck(context.Background(), srv.Client(), &chk, srv.URL))

	assert.Equal(t, []string{"first", "second"}, seen.header.Values("X-Ze-Tag"))
}

// TestHTTPCheckHostHeaderReachesWire verifies the net/http Host special case.
//
// VALIDATES: header=Host: is applied to Request.Host, which is what net/http
// actually writes on the wire.
// PREVENTS: a Host header silently vanishing, because net/http ignores a "Host"
// entry in Request.Header.
func TestHTTPCheckHostHeaderReachesWire(t *testing.T) {
	srv, seen := captureHeaders(t, "ok")
	r := &Runner{}

	chk := httpCheck{
		Seq:     1,
		Method:  "get",
		Status:  200,
		Headers: []httpHeader{{Name: "Host", Value: "lg.example.test"}},
	}
	require.NoError(t, r.executeOneHTTPCheck(context.Background(), srv.Client(), &chk, srv.URL))

	assert.Equal(t, "lg.example.test", seen.host)
}

// TestHTTPWaitSendsHeaders verifies headers on the readiness-poll path.
//
// VALIDATES: executeOneHTTPWait applies the same header= keys, so http=wait can
// poll an endpoint that rejects requests missing a mandatory header.
// PREVENTS: headers being wired into the assertion path only, leaving http=wait
// unable to reach a header-gated endpoint.
func TestHTTPWaitSendsHeaders(t *testing.T) {
	srv, seen := captureHeaders(t, "ready")
	r := &Runner{}

	chk := httpCheck{
		Seq:      1,
		Method:   "get",
		Status:   200,
		Contains: "ready",
		Timeout:  "5s",
		Headers:  []httpHeader{{Name: "MCP-Protocol-Version", Value: "2026-07-28"}},
	}
	require.NoError(t, r.executeOneHTTPWait(context.Background(), srv.Client(), &chk, srv.URL))

	assert.Equal(t, "2026-07-28", seen.header.Get("MCP-Protocol-Version"))
}

// TestExecuteHTTPChecksLoop drives the whole check loop, not just one request.
//
// VALIDATES: executeHTTPChecks still orders by seq, substitutes $PORT, resolves a
// relative sendfile against the tmpfs dir, and carries header= through -- after the
// loop was converted from range-copy to indexing.
// PREVENTS: the indexing conversion silently reusing or skipping an element, and
// the sendfile path rewrite landing somewhere the next iteration re-resolves.
func TestExecuteHTTPChecksLoop(t *testing.T) {
	var order []string
	var sentBodies []string
	var versions []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		order = append(order, r.URL.Path)
		versions = append(versions, r.Header.Get("MCP-Protocol-Version"))
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read request body: %v", err)
		}
		sentBodies = append(sentBodies, string(body))
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("ok")); err != nil {
			t.Errorf("write response body: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	port, err := strconv.Atoi(srv.URL[strings.LastIndex(srv.URL, ":")+1:])
	require.NoError(t, err)

	tmpfs := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpfs, "call.json"), []byte(`{"id":1}`), 0o600))

	rec := newRecord("loop")
	rec.Port = port
	rec.CIFile = filepath.Join(t.TempDir(), "loop.ci")
	rec.TmpfsTempDir = tmpfs
	// Authored out of order on purpose: the loop must sort by seq.
	rec.HTTPChecks = []httpCheck{
		{Seq: 2, Method: "post", URL: "http://127.0.0.1:$PORT/second", Status: 200, SendFile: "call.json",
			Headers: []httpHeader{{Name: "MCP-Protocol-Version", Value: "2026-07-28"}}},
		{Seq: 1, Method: "get", URL: "http://127.0.0.1:$PORT/first", Status: 200,
			Headers: []httpHeader{{Name: "MCP-Protocol-Version", Value: "2026-07-28"}}},
	}

	require.NoError(t, (&Runner{}).executeHTTPChecks(context.Background(), rec))

	assert.Equal(t, []string{"/first", "/second"}, order)
	assert.Equal(t, []string{"", `{"id":1}`}, sentBodies)
	assert.Equal(t, []string{"2026-07-28", "2026-07-28"}, versions)
	// The authored relative path must survive on the Record: only the loop's own
	// defensive copy may be rewritten to an absolute path.
	assert.Equal(t, "call.json", rec.HTTPChecks[0].SendFile)
}

// TestExecuteHTTPWaitsLoop drives the readiness-poll loop.
//
// VALIDATES: executeHTTPWaits substitutes $PORT and applies header= after its own
// range-copy-to-indexing conversion.
// PREVENTS: the wait loop losing per-element state when converted to indexing.
func TestExecuteHTTPWaitsLoop(t *testing.T) {
	var versions []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		versions = append(versions, r.Header.Get("MCP-Protocol-Version"))
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("ready")); err != nil {
			t.Errorf("write response body: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	port, err := strconv.Atoi(srv.URL[strings.LastIndex(srv.URL, ":")+1:])
	require.NoError(t, err)

	rec := newRecord("waitloop")
	rec.Port = port
	rec.CIFile = filepath.Join(t.TempDir(), "waitloop.ci")
	rec.HTTPWaits = []httpCheck{
		{Seq: 1, Method: "get", URL: "http://127.0.0.1:$PORT/ready", Status: 200, Contains: "ready", Timeout: "5s",
			Headers: []httpHeader{{Name: "MCP-Protocol-Version", Value: "2026-07-28"}}},
	}

	require.NoError(t, (&Runner{}).executeHTTPWaits(context.Background(), rec))
	assert.Equal(t, []string{"2026-07-28"}, versions)
}

// TestHTTPCheckWithoutHeadersSetsNone verifies the no-regression case.
//
// VALIDATES: a check with no header= keys still sends only what it sent before
// (the sendfile Content-Type default), adding nothing.
// PREVENTS: applyCheckHeaders injecting a stray field on every request.
func TestHTTPCheckWithoutHeadersSetsNone(t *testing.T) {
	srv, seen := captureHeaders(t, "ok")
	r := &Runner{}

	sendFile := filepath.Join(t.TempDir(), "body.json")
	require.NoError(t, os.WriteFile(sendFile, []byte(`{}`), 0o600))

	chk := httpCheck{Seq: 1, Method: "post", Status: 200, SendFile: sendFile}
	require.NoError(t, r.executeOneHTTPCheck(context.Background(), srv.Client(), &chk, srv.URL))

	assert.Equal(t, []string{"application/json"}, seen.header.Values("Content-Type"))
	assert.Empty(t, seen.header.Values("MCP-Protocol-Version"))
}
