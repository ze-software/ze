// Design: docs/architecture/testing/runner-architecture.md -- browser startup deadlines.
package cli

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestWebMalformedCaseRefusesBeforeStartup ensures the parse cause wins over
// the deliberately unavailable server binary, including an authored skip.
func TestWebMalformedCaseRefusesBeforeStartup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "malformed.wb")
	if err := os.WriteFile(path, []byte("option=skip:reason=example\noption=timeout:value=broken\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	test := discoverWebCase(path, 1)
	ok, err := zeTestRunWebTest(context.Background(), test, zeTestWebBinaries{ze: "/no-such-owned-test-binary"}, 1)
	if ok {
		t.Fatal("malformed case passed")
	}
	if err == nil {
		t.Fatal("malformed case lost its parse error")
	}
	if !strings.Contains(err.Error(), "timeout") {
		t.Fatalf("server startup obscured parse error: %v", err)
	}
}

// TestWebReadinessBoundsInflightHTTP uses a handler that waits for cancellation;
// the readiness limit must cancel that request, not just stop the next poll.
func TestWebReadinessBoundsInflightHTTP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, req *http.Request) {
		<-req.Context().Done()
	}))
	defer server.Close()
	start := time.Now()
	err := zeTestProbeReady(context.Background(), schemeHTTP, strings.TrimPrefix(server.URL, schemeHTTP), "/", 20*time.Millisecond)
	if err == nil {
		t.Fatal("nonresponding server reported ready")
	}
	if elapsed := time.Since(start); elapsed > 250*time.Millisecond {
		t.Fatalf("20ms readiness budget did not bound the request: %s", elapsed)
	}
}
