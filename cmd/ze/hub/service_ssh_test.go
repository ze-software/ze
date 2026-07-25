//go:build ze_ssh

package hub

import (
	"testing"

	"github.com/ze-software/ze/internal/component/plugin"
)

// VALIDATES: a Response with Status=="done" (the common success case) never
// produces an error, regardless of its formatted content.
func TestResponseExecErr_DoneIsNil(t *testing.T) {
	resp := &plugin.Response{Status: plugin.StatusDone}
	if err := responseExecErr(resp, "some output"); err != nil {
		t.Fatalf("responseExecErr(done) = %v, want nil", err)
	}
}

// VALIDATES: a Response with Status=="error" and a populated Error field
// produces an error carrying that message -- this is the parse-failure
// shape (e.g. as112's handleAS112Health on a malformed "target" arg).
func TestResponseExecErr_UsesErrorField(t *testing.T) {
	resp := &plugin.Response{Status: plugin.StatusError, Error: "bad input"}
	err := responseExecErr(resp, "irrelevant")
	if err == nil || err.Error() != "bad input" {
		t.Fatalf("responseExecErr = %v, want error \"bad input\"", err)
	}
}

// VALIDATES: a Response with Status=="error" but an EMPTY Error field (the
// operational-failure shape many handlers use -- e.g. as112's
// handleAS112Health returns Status:StatusError with a nil Go error and no
// Error string when a health query legitimately fails, only Data carrying
// {"healthy":false,...}) still produces a non-nil error, using the
// formatted response content so the failure reason isn't silently lost.
// PREVENTS the bug this function fixes: cmd/ze/hub's SetExecutorFactory
// previously always returned (formatted, nil) whenever d.Dispatch itself
// didn't return a Go error, so a Status:StatusError response over the real
// SSH `ze cli -c "..."` path always produced a ZERO exit code -- silently
// breaking any script (a BGP healthcheck probe, in particular) that relies
// on the process exit code to detect an operational failure.
func TestResponseExecErr_EmptyErrorFallsBackToFormatted(t *testing.T) {
	resp := &plugin.Response{Status: plugin.StatusError}
	err := responseExecErr(resp, `{"healthy":false,"target":"127.0.0.1:53"}`)
	if err == nil || err.Error() != `{"healthy":false,"target":"127.0.0.1:53"}` {
		t.Fatalf("responseExecErr = %v, want error using the formatted content", err)
	}
}

// VALIDATES: Status=="error" with both Error and formatted content empty
// still produces SOME non-nil error (a generic fallback), never silently
// succeeding.
func TestResponseExecErr_BothEmptyStillErrors(t *testing.T) {
	resp := &plugin.Response{Status: plugin.StatusError}
	if err := responseExecErr(resp, ""); err == nil {
		t.Fatal("responseExecErr = nil, want a non-nil fallback error")
	}
}
