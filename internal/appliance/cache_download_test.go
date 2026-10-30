// Design: docs/architecture/appliance/build-artifacts.md -- artifact download lifetime.
package appliance

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

// TestDownloadBodyOutlivesResponseHeaders streams an artifact only after the
// download helper returns. Canceling at that return truncates the artifact,
// even though receiving its headers succeeded.
func TestDownloadBodyOutlivesResponseHeaders(t *testing.T) {
	payload := bytes.Repeat([]byte("streamed-kernel-artifact\n"), 400_000)
	releaseBody := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", strconv.Itoa(len(payload)))
		w.WriteHeader(http.StatusOK)
		if err := http.NewResponseController(w).Flush(); err != nil {
			t.Errorf("flush artifact headers: %v", err)
			return
		}
		<-releaseBody
		if _, err := w.Write(payload); err != nil {
			t.Errorf("stream artifact: %v", err)
		}
	}))
	defer server.Close()

	response, err := defaultHTTPGet(server.URL)
	close(releaseBody)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close() //nolint:errcheck // Explicit close below checks the result.
	got, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read artifact after headers returned: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("downloaded artifact differs: got %d bytes, want %d", len(got), len(payload))
	}
	if err := response.Body.Close(); err != nil {
		t.Fatalf("close artifact: %v", err)
	}
	if !errors.Is(response.Request.Context().Err(), context.Canceled) {
		t.Fatalf("closing the body left the request live: %v", response.Request.Context().Err())
	}
}
