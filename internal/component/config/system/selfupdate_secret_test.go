//go:build ze_distro

// Design: docs/architecture/appliance/self-update.md -- download, verify, stage, restart logic
// Related: selfupdate.go -- selfUpdater.check, fetchManifest, downloadBinary

package system

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The self-updater reads the same operator-supplied URL as the update checker
// and publishes the same two surfaces, so it owes the same property
// (plan/journal/secret-echoed-to-the-client.md, 2026-08-18). The constants and
// the helpers are shared with update_secret_test.go, which carries no build
// tag and is compiled beside this file.

// TestSelfUpdateNeverPublishesTheURLUserinfo drives the real manifest check
// against a server that refuses and requires the credential inside the URL to
// reach neither the stored status nor the log line.
func TestSelfUpdateNeverPublishesTheURLUserinfo(t *testing.T) {
	refusing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer refusing.Close()

	updater := newSelfUpdater(withUserinfo(refusing.URL+"/version.json"), 86400, SelfUpdateConfig{}, nil)
	updater.running = "26.01.01"

	logged := captureStderr(t, func() { updater.check(context.Background()) })

	status := updater.Status()
	if status.LastError == "" {
		t.Fatal("the check did not fail, so nothing was published to assert on")
	}
	requireNoCredential(t, "UpdateStatus.LastError", status.LastError)
	requireNoCredential(t, "the log line", logged)
}

// TestSelfUpdateStillNamesAURLWithNoUserinfo is the opposite polarity.
func TestSelfUpdateStillNamesAURLWithNoUserinfo(t *testing.T) {
	refusing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer refusing.Close()

	updater := newSelfUpdater(refusing.URL+"/version.json", 86400, SelfUpdateConfig{}, nil)
	updater.running = "26.01.01"

	updater.check(context.Background())

	if !strings.Contains(updater.Status().LastError, refusing.URL) {
		t.Errorf("LastError does not name the endpoint: %s", updater.Status().LastError)
	}
}
