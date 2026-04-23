package vpp

import (
	"bytes"
	"strings"
	"testing"

	"go.fd.io/govpp/adapter/statsclient"

	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
)

func TestNewGovppLogrusLoggerForwardsStructuredFields(t *testing.T) {
	// VALIDATES: govpp logrus entries are bridged into Ze slog output with fields intact.
	// PREVENTS: govpp logs bypassing Ze backends or flattening structured context.
	var buf bytes.Buffer
	logger := slogutil.LoggerWithOutput("vpp", "debug", &buf)

	bridge := newGovppLogrusLogger(logger, "logger", "govpp/test")
	bridge.WithField("connId", 7).Warn("socket closed")

	out := buf.String()
	for _, want := range []string{
		"level=WARN",
		`msg="socket closed"`,
		"subsystem=vpp",
		"logger=govpp/test",
		"connId=7",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("bridged log missing %q in %q", want, out)
		}
	}
}

func TestNewGovppLogrusLoggerRespectsSlogLevel(t *testing.T) {
	// VALIDATES: govpp bridge follows the configured Ze log level.
	// PREVENTS: vendor INFO logs leaking when the VPP subsystem is set to WARN.
	var buf bytes.Buffer
	logger := slogutil.LoggerWithOutput("vpp", "warn", &buf)

	bridge := newGovppLogrusLogger(logger, "logger", "govpp/test")
	bridge.Info("hidden info")
	if got := buf.String(); got != "" {
		t.Fatalf("info log should be filtered by warn logger, got %q", got)
	}

	bridge.Warn("visible warn")
	if got := buf.String(); !strings.Contains(got, `msg="visible warn"`) {
		t.Fatalf("warn log should pass through, got %q", got)
	}
}

func TestSetVPPLoggerRoutesStatsclientLog(t *testing.T) {
	// VALIDATES: SetVPPLogger redirects govpp statsclient logging into Ze's logger.
	// PREVENTS: statsclient logs escaping on stdout instead of the configured backend.
	prevLog := statsclient.Log
	prevDebug := statsclient.Debug
	t.Cleanup(func() {
		statsclient.Log = prevLog
		statsclient.Debug = prevDebug
		SetVPPLogger(slogutil.DiscardLogger())
	})

	var buf bytes.Buffer
	SetVPPLogger(slogutil.LoggerWithOutput("vpp", "info", &buf))

	statsclient.Log.WithField("socket", "/run/vpp/stats.sock").Warn("stats connect failed")

	out := buf.String()
	for _, want := range []string{
		"level=WARN",
		`msg="stats connect failed"`,
		"subsystem=vpp",
		"logger=govpp/statsclient",
		"socket=/run/vpp/stats.sock",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("statsclient log missing %q in %q", want, out)
		}
	}
}
