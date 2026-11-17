// Design: docs/architecture/web-interface.md -- actionable HTTP admission refusals.
package web

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/opererr"
)

// TestCLICompletionRefusalPreservesProducerRecovery ensures an actionable
// producer is passed through unchanged while a plain transport error gains context.
func TestCLICompletionRefusalPreservesProducerRecovery(t *testing.T) {
	typed := opererr.New("The command is forbidden.", "The caller has no grant.", "show authorization")
	if got := cliCompletionFailure(typed); got != typed {
		t.Fatalf("completion replaced an actionable producer: %v", got)
	}
	cause := errors.New("connection unavailable")
	refusal := cliCompletionFailure(cause)
	if !errors.Is(refusal, cause) {
		t.Fatalf("completion lost its transport cause: %v", refusal)
	}
	if !strings.HasSuffix(refusal.Error(), "fix: Open /cli and request command completion again") {
		t.Fatalf("completion refusal has no final recovery action: %q", refusal.Error())
	}
}

// TestSubmittedCLIModeRefusalExplainsRecovery keeps the invalid-mode status
// while requiring the browser action on the final diagnostic line.
func TestSubmittedCLIModeRefusalExplainsRecovery(t *testing.T) {
	request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/cli/terminal", strings.NewReader(url.Values{"mode": {"unexpected"}}.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	if _, accepted := submittedCLIMode(response, request); accepted {
		t.Fatal("an invalid submitted CLI mode was accepted")
	}
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d; want %d", response.Code, http.StatusBadRequest)
	}
	if !strings.HasSuffix(response.Body.String(), "fix: Open /cli and select configuration or operational mode\n") {
		t.Fatalf("mode refusal has no final recovery action: %q", response.Body.String())
	}
}

// TestCLIHistoryAuthenticationRefusalExplainsRecovery proves recall does not
// open a history owner before authentication and tells the browser how to sign in.
func TestCLIHistoryAuthenticationRefusalExplainsRecovery(t *testing.T) {
	response := httptest.NewRecorder()
	writeCLIHistory(response, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/cli?history=1&mode=config", http.NoBody), nil)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d; want %d", response.Code, http.StatusUnauthorized)
	}
	if !strings.HasSuffix(response.Body.String(), "fix: Open /login and sign in, then reopen /cli\n") {
		t.Fatalf("history refusal has no final recovery action: %q", response.Body.String())
	}
}

// TestWebRequestAdmissionRefusalExplainsRecovery exercises sealing without
// starting a server and proves refused requests never reach their handler.
func TestWebRequestAdmissionRefusalExplainsRecovery(t *testing.T) {
	var server WebServer
	server.requests.seal()
	response := httptest.NewRecorder()
	server.serveRequest(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("a sealed service admitted a request")
	})).ServeHTTP(response, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/cli", http.NoBody))
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d; want %d", response.Code, http.StatusServiceUnavailable)
	}
	if !strings.HasSuffix(response.Body.String(), "fix: Reconnect to /cli after the web service is available\n") {
		t.Fatalf("admission refusal has no final recovery action: %q", response.Body.String())
	}
}
