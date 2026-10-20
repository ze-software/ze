package bridge

import (
	"strings"
	"testing"

	// The exabgp.api.* keys are registered by the config component, which
	// writes them from `environment { exabgp { api { ... } } }`; env.Get
	// refuses an unregistered key.
	_ "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/env"
)

// setExaBGPEnv sets one environment variable the way a parent process hands
// it to the bridge, and drops env's cache so the next read sees it.
func setExaBGPEnv(t *testing.T, name, value string) {
	t.Helper()
	t.Setenv(name, value)
	env.ResetCache()
	t.Cleanup(env.ResetCache)
}

// TestAckModeReadsEitherExaBGPSpelling proves the bridge reads exabgp.api.ack
// under both spellings ExaBGP accepts, the dotted one first
// (src/exabgp/environment/config.py, Environment.setup: "Priority: env var
// (dot) > env var (underscore) > INI file > default").
func TestAckModeReadsEitherExaBGPSpelling(t *testing.T) {
	t.Run("underscore", func(t *testing.T) {
		setExaBGPEnv(t, "exabgp_api_ack", "false")
		if NewAckMode().enabled {
			t.Error("exabgp_api_ack=false left the ack enabled")
		}
	})
	t.Run("dot wins", func(t *testing.T) {
		setExaBGPEnv(t, ackEnvKey, "false")
		setExaBGPEnv(t, "exabgp_api_ack", "true")
		if NewAckMode().enabled {
			t.Error("exabgp.api.ack=false lost to exabgp_api_ack=true")
		}
	})
}

// TestAckModeDefaultEnabled verifies the default (env unset) is ack enabled.
//
// VALIDATES: AC-16 default case: ze.bgp is backward-compatible with ExaBGP clients
// expecting done/error lines after each dispatched command.
func TestAckModeDefaultEnabled(t *testing.T) {
	setExaBGPEnv(t, ackEnvKey, "")
	m := NewAckMode()
	if !m.enabled {
		t.Errorf("default ack should be enabled, got disabled")
	}
}

// TestAckModeDisabled verifies explicit `false`/`0`/etc disable the ack.
//
// VALIDATES: AC-17 exabgp.api.ack=false silences done/error emission.
func TestAckModeDisabled(t *testing.T) {
	for _, raw := range []string{"false", "0", "no", "off", "disable", "disabled", "FALSE"} {
		t.Run(raw, func(t *testing.T) {
			setExaBGPEnv(t, ackEnvKey, raw)
			m := NewAckMode()
			if m.enabled {
				t.Errorf("ack with %q should be disabled, got enabled", raw)
			}
		})
	}
}

// TestAckModeExplicitEnabled verifies explicit truthy values keep ack on.
func TestAckModeExplicitEnabled(t *testing.T) {
	for _, raw := range []string{"true", "1", "yes", "on", "enable"} {
		t.Run(raw, func(t *testing.T) {
			setExaBGPEnv(t, ackEnvKey, raw)
			m := NewAckMode()
			if !m.enabled {
				t.Errorf("ack with %q should be enabled", raw)
			}
		})
	}
}

// TestAckWriteAckEmitsDone verifies writeAck emits exactly `done\n`.
//
// VALIDATES: AC-16 emitted frame is a single line, newline-terminated.
func TestAckWriteAckEmitsDone(t *testing.T) {
	var buf strings.Builder
	m := AckMode{enabled: true}
	m.WriteAck(&buf)
	if got := buf.String(); got != "done\n" {
		t.Errorf("writeAck = %q, want %q", got, "done\n")
	}
}

// TestAckWriteAckDisabledNoOutput verifies writeAck is a no-op when disabled.
//
// VALIDATES: AC-17 disabled mode writes zero bytes.
func TestAckWriteAckDisabledNoOutput(t *testing.T) {
	var buf strings.Builder
	m := AckMode{enabled: false}
	m.WriteAck(&buf)
	if got := buf.String(); got != "" {
		t.Errorf("disabled writeAck wrote %q, want empty", got)
	}
}

// TestAckWriteErrorSanitizes verifies multi-line / CR characters in the error
// text are flattened so they cannot inject extra framing.
//
// VALIDATES: Security review item (AC-18 variant): error message cannot inject
// additional lines, in either API version's form.
func TestAckWriteErrorSanitizes(t *testing.T) {
	for _, version := range []APIVersion{APIV4, APIV6} {
		var buf strings.Builder
		m := AckMode{enabled: true}
		m.WriteError(&buf, version, "bad\r\nerror: oops")
		got := buf.String()
		if strings.Count(got, "\n") != 2 {
			t.Errorf("%s: writeError output %q should be the reason line and the marker", version, got)
		}
		if !strings.HasSuffix(got, "\nerror\n") {
			t.Errorf("%s: writeError output %q should end with the error marker", version, got)
		}
	}
}

// TestAckWriteErrorTruncates verifies the error message is bounded in length.
//
// VALIDATES: Security review item: DoS via very long error messages is bounded.
func TestAckWriteErrorTruncates(t *testing.T) {
	var buf strings.Builder
	m := AckMode{enabled: true}
	long := strings.Repeat("x", maxAckMessageLen*2)
	m.WriteError(&buf, APIV4, long)
	if len(buf.String()) > maxAckMessageLen+len("error: \nerror\n") {
		t.Errorf("writeError did not truncate: %d bytes", len(buf.String()))
	}
}

// TestErrorAnswerV4 pins AC-14: a refused line from an API 4 or an undecided
// script answers `error: <why>` then `error`.
//
// VALIDATES: X-13, ExaBGP's _error_detail for a helper that is not API 6
// (`f'error: {message}'`), followed by answer_error_sync's text marker.
// PREVENTS: the one-line `error <why>` the bridge wrote before, which matches
// neither of ExaBGP's forms, so a script reading `error:` never saw the reason.
func TestErrorAnswerV4(t *testing.T) {
	for _, version := range []APIVersion{APIV4, APIVersionAuto} {
		var buf strings.Builder
		m := AckMode{enabled: true}
		m.WriteError(&buf, version, "no peers match selector")
		if got, want := buf.String(), "error: no peers match selector\nerror\n"; got != want {
			t.Errorf("%s: got %q, want %q", version, got, want)
		}
	}
}

// TestErrorAnswerV6 pins AC-15: a refused line from an API 6 script answers
// `{"error": "<why>"}` then `error`, the reason quoted as json.dumps quotes it.
//
// VALIDATES: X-13, `json.dumps({'error': message})` for an API 6 helper: the
// `": "` separator, ensure_ascii's lowercase \u escapes for non-ASCII and a
// surrogate pair past U+FFFF, and `<`, `>`, `&` left as they are (encoding/json
// would write \u003c).
// PREVENTS: an API 6 script reading a text reason it cannot parse as JSON.
func TestErrorAnswerV6(t *testing.T) {
	cases := []struct{ reason, want string }{
		{"no peers match selector", `{"error": "no peers match selector"}`},
		{`bad "word" <x> & \ y`, `{"error": "bad \"word\" <x> & \\ y"}`},
		{"caf\u00e9 \U0001F600 \x01", `{"error": "caf\u00e9 \ud83d\ude00 \u0001"}`},
	}
	for _, tc := range cases {
		var buf strings.Builder
		m := AckMode{enabled: true}
		m.WriteError(&buf, APIV6, tc.reason)
		if got, want := buf.String(), tc.want+"\nerror\n"; got != want {
			t.Errorf("reason %q: got %q, want %q", tc.reason, got, want)
		}
	}
}

// TestErrorAnswerAckDisabledSilent pins AC-16: with ack disabled neither the
// reason nor the marker is written, as both go through ExaBGP's _answer_sync,
// which checks the helper's ack state.
func TestErrorAnswerAckDisabledSilent(t *testing.T) {
	for _, version := range []APIVersion{APIV4, APIVersionAuto, APIV6} {
		var buf strings.Builder
		m := AckMode{enabled: false}
		m.WriteError(&buf, version, "no peers match selector")
		if got := buf.String(); got != "" {
			t.Errorf("%s: disabled ack wrote %q, want nothing", version, got)
		}
	}
}
