// Design: docs/architecture/appliance/on-device-installer.md -- download with retry and integrity check
// Related: download.go -- downloadToFile, downloadToDisk

package disk

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The installer fetches its image from an operator-supplied URL, which may
// carry userinfo for a private mirror. Every retry writes a Warn line and the
// final failure is a formatted error, so the credential reaches the installer
// console twice (plan/journal/secret-echoed-to-the-client.md, 2026-08-18).
const (
	imageFakeCredential = "DUMMY-VALUE-NOT-A-CREDENTIAL-4471bc"
	imageFakeTail       = "4471bc"
	imageFakeUserinfo   = "operator:" + imageFakeCredential + "@"
)

func imageURLWithUserinfo(rawURL string) string {
	return strings.Replace(rawURL, "//", "//"+imageFakeUserinfo, 1)
}

// captureSlog installs a default logger over a buffer while fn runs.
// download.go logs through the package-level slog functions.
func captureSlog(t *testing.T, fn func()) string {
	t.Helper()
	var buf bytes.Buffer
	original := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	defer slog.SetDefault(original)
	fn()
	return buf.String()
}

func requireNoImageCredential(t *testing.T, surface, published string) {
	t.Helper()
	if strings.Contains(published, imageFakeCredential) {
		t.Errorf("%s holds the whole credential: %s", surface, published)
	}
	if strings.Contains(published, imageFakeTail) {
		t.Errorf("%s holds the tail of the credential: %s", surface, published)
	}
}

// shortRetryDelay collapses the retry backoff for a test that has to reach
// the retry log. The entry points are downloadToFile and downloadToDisk, and
// the Warn line lives in their retry loop rather than in the inner attempt.
func shortRetryDelay(t *testing.T) {
	t.Helper()
	original := retryDelay
	retryDelay = time.Millisecond
	t.Cleanup(func() { retryDelay = original })
}

// TestInstallerDownloadNeverPublishesTheURLUserinfo drives the real download
// against a server that refuses, and requires the credential inside the URL
// to reach neither the retry log nor the error the installer prints.
func TestInstallerDownloadNeverPublishesTheURLUserinfo(t *testing.T) {
	shortRetryDelay(t)

	refusing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer refusing.Close()

	rawURL := imageURLWithUserinfo(refusing.URL + "/ze.img")
	dest := filepath.Join(t.TempDir(), "ze.img")

	var err error
	logged := captureSlog(t, func() { err = downloadToFile(rawURL, dest) })
	if err == nil {
		t.Fatal("the download did not fail, so nothing was published to assert on")
	}
	requireNoImageCredential(t, "the returned error", err.Error())
	requireNoImageCredential(t, "the log line", logged)

	logged = captureSlog(t, func() { err = downloadToDisk(rawURL, dest, "") })
	if err == nil {
		t.Fatal("the stream did not fail, so nothing was published to assert on")
	}
	requireNoImageCredential(t, "the returned error", err.Error())
	requireNoImageCredential(t, "the log line", logged)
}

// TestInstallerDownloadStillNamesAURLWithNoUserinfo is the opposite polarity.
// An installer that hid every URL would satisfy the test above and leave the
// operator with no way to tell which mirror refused.
func TestInstallerDownloadStillNamesAURLWithNoUserinfo(t *testing.T) {
	refusing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer refusing.Close()

	shortRetryDelay(t)
	rawURL := refusing.URL + "/ze.img"
	err := downloadToFile(rawURL, filepath.Join(t.TempDir(), "ze.img"))
	if err == nil {
		t.Fatal("the download did not fail")
	}
	if !strings.Contains(err.Error(), rawURL) {
		t.Errorf("the error does not name the mirror: %s", err)
	}
}
