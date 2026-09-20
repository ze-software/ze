// Design: docs/architecture/config/system-update.md -- periodic version check against remote manifest
// Related: update.go -- UpdateChecker.check, fetchVersion

package system

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// The update-check URL is operator-supplied and may carry userinfo, so the
// credential is inside the URL rather than in a config leaf. Every leaf mask
// this repository built is the wrong shape for it
// (plan/journal/secret-echoed-to-the-client.md, 2026-08-18).
//
// Two surfaces publish it. UpdateStatus.LastError reaches the operator's
// terminal through `show firmware` (internal/plugins/update-cmd/cmd/show.go),
// and the Warn line reaches the log. Both are asserted here, on a distinctive
// TAIL as well as on the whole value.
const (
	urlFakeCredential = "DUMMY-VALUE-NOT-A-CREDENTIAL-4471bc"
	urlFakeTail       = "4471bc"
	urlFakeUserinfo   = "operator:" + urlFakeCredential + "@"
	// A userinfo with no password half. net/http's own stripPassword leaves
	// this shape whole, so it is the case Ze has to answer for itself.
	urlFakeTokenOnly = urlFakeCredential + "@"
)

// withUserinfo inserts the fake userinfo into a `http://host:port/...` URL.
func withUserinfo(rawURL string) string {
	return strings.Replace(rawURL, "//", "//"+urlFakeUserinfo, 1)
}

// withBareToken inserts a userinfo that is a token alone, with no password.
func withBareToken(rawURL string) string {
	return strings.Replace(rawURL, "//", "//"+urlFakeTokenOnly, 1)
}

// captureStderr swaps os.Stderr for a pipe while fn runs. slogutil.Logger
// reads os.Stderr when it builds the handler, and check() builds the logger on
// every call, so the swap catches the line check() writes.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	original := os.Stderr
	os.Stderr = writer
	fn()
	os.Stderr = original
	if closeErr := writer.Close(); closeErr != nil {
		t.Fatalf("close pipe writer: %v", closeErr)
	}
	out, readErr := io.ReadAll(reader)
	if readErr != nil {
		t.Fatalf("read pipe: %v", readErr)
	}
	return string(out)
}

// requireNoCredential fails when either the whole fake value or its tail is
// present in what a surface published.
func requireNoCredential(t *testing.T, surface, published string) {
	t.Helper()
	if strings.Contains(published, urlFakeCredential) {
		t.Errorf("%s holds the whole credential: %s", surface, published)
	}
	if strings.Contains(published, urlFakeTail) {
		t.Errorf("%s holds the tail of the credential: %s", surface, published)
	}
}

// TestUpdateCheckNeverPublishesTheURLUserinfo drives the real check loop
// against a server that refuses, and against an address nothing listens on,
// and requires the credential inside the URL to reach neither the stored
// status the CLI prints nor the log line.
func TestUpdateCheckNeverPublishesTheURLUserinfo(t *testing.T) {
	refusing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer refusing.Close()

	// A server that is closed before the check runs gives a transport error,
	// which is the path where net/http writes the URL into the error itself.
	dead := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	deadURL := dead.URL
	dead.Close()

	cases := []struct {
		name   string
		rawURL string
	}{
		{name: "the server answers an error status", rawURL: withUserinfo(refusing.URL + "/version.json")},
		{name: "the transport fails", rawURL: withUserinfo(deadURL + "/version.json")},
		{name: "the userinfo is a bare token", rawURL: withBareToken(refusing.URL + "/version.json")},
		{name: "a bare token meets a transport failure", rawURL: withBareToken(deadURL + "/version.json")},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			checker := newUpdateChecker(tc.rawURL, 86400)
			checker.running = "26.01.01"

			logged := captureStderr(t, func() { checker.check(context.Background()) })

			status := checker.Status()
			if status.LastError == "" {
				t.Fatal("the check did not fail, so nothing was published to assert on")
			}
			requireNoCredential(t, "UpdateStatus.LastError", status.LastError)
			requireNoCredential(t, "the log line", logged)
		})
	}
}

// TestUpdateCheckStillNamesAURLWithNoUserinfo is the opposite polarity. A
// checker that hid every URL would satisfy the test above and leave the
// operator unable to tell which endpoint failed.
func TestUpdateCheckStillNamesAURLWithNoUserinfo(t *testing.T) {
	refusing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer refusing.Close()

	checker := newUpdateChecker(refusing.URL+"/version.json", 86400)
	checker.running = "26.01.01"

	logged := captureStderr(t, func() { checker.check(context.Background()) })

	if !strings.Contains(checker.Status().LastError, refusing.URL) {
		t.Errorf("LastError does not name the endpoint: %s", checker.Status().LastError)
	}
	if !strings.Contains(logged, refusing.URL) {
		t.Errorf("the log line does not name the endpoint: %s", logged)
	}
}
