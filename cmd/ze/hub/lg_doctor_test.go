//go:build ze_lg

// Design: docs/architecture/pki/tls-listeners.md -- each looking-glass entry chooses HTTPS or HTTP, one rule for start, reload and doctor

package hub

import (
	"maps"
	"slices"
	"strings"
	"testing"

	zeconfig "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/env"
)

// lgListenerDoctor returns the registered looking-glass listener check, after
// proving it declares each code it reports and that each code is explained.
func lgListenerDoctor(t *testing.T) diagnostic.DoctorCheck {
	t.Helper()
	for _, check := range diagnostic.DoctorChecksForPhase(diagnostic.DoctorPhasePostConfig) {
		if check.Name != "lg-listener-protocol" {
			continue
		}
		for _, code := range []string{codeDoctorLGListenUnreadable, codeDoctorLGTLSUnservable, codeDoctorLGPlainHTTP} {
			if !slices.Contains(check.Codes, code) {
				t.Fatalf("the looking-glass listener check does not declare %s", code)
			}
			meta := diagnostic.Lookup(code)
			if meta == nil {
				t.Fatalf("%s has no explanation for `ze show diagnostic`", code)
			}
			if meta.Title == "" {
				t.Fatalf("%s has an explanation with no title", code)
			}
		}
		return check
	}
	t.Fatal("the looking-glass listener check is absent from the post-config doctor registry")
	return diagnostic.DoctorCheck{}
}

// lgOneFinding fails unless diags is one finding of code at severity, and
// returns it.
func lgOneFinding(t *testing.T, diags []diagnostic.Diagnostic, code string, severity diagnostic.Severity) diagnostic.Diagnostic {
	t.Helper()
	if len(diags) != 1 {
		t.Fatalf("got %d findings, want one: %+v", len(diags), diags)
	}
	if diags[0].Code != code {
		t.Fatalf("finding code = %s, want %s: %+v", diags[0].Code, code, diags[0])
	}
	if diags[0].Severity != severity {
		t.Fatalf("finding severity = %s, want %s: %+v", diags[0].Severity, severity, diags[0])
	}
	return diags[0]
}

func TestDoctorReportsLGListenerProtocol(t *testing.T) {
	// VALIDATES: `ze doctor` reports, through its registered post-config
	// check, each looking-glass answer the operator has to act on, decided by
	// the start path's own planLGTransports: an entry that left tls unset and
	// serves plain HTTP for want of a certificate (a warning naming the entry
	// and the fix), an entry that says `tls true` with no certificate (the
	// refusal ze starts with), and an unreadable ze.looking-glass.listen. A
	// store, a named certificate or `tls false` reports nothing.
	// PREVENTS: a looking glass quietly serving HTTP that doctor calls healthy,
	// and a doctor rule that disagrees with what ze does with the same config.
	check := lgListenerDoctor(t)
	const addr = "127.0.0.1:3443"
	entry := func(leaves map[string]any) map[string]any {
		server := map[string]any{"ip": "127.0.0.1", "port": "3443"}
		maps.Copy(server, leaves)
		return map[string]any{"secure": server}
	}
	enabled := map[string]any{"enabled": "true"}

	t.Run("an unset leaf with no store and no name serves HTTP, with a warning", func(t *testing.T) {
		diags := check.Check(diagnostic.DoctorCheckContext{Tree: lgEntriesTree(enabled, entry(nil))})
		diag := lgOneFinding(t, diags, codeDoctorLGPlainHTTP, diagnostic.SeverityWarning)
		for _, want := range []string{addr, "plain HTTP", "tls leaf is unset"} {
			if !strings.Contains(diag.Message, want) {
				t.Fatalf("message %q does not name %q", diag.Message, want)
			}
		}
		for _, want := range []string{"ze init", "tls false"} {
			if !strings.Contains(diag.Fix, want) {
				t.Fatalf("fix %q does not name %q", diag.Fix, want)
			}
		}
	})

	t.Run("a store keeps the self-signed certificate, so the unset leaf serves HTTPS", func(t *testing.T) {
		diags := check.Check(diagnostic.DoctorCheckContext{Tree: lgEntriesTree(enabled, entry(nil)), Store: lgBlobStorage(t)})
		if len(diags) != 0 {
			t.Fatalf("a looking glass that serves HTTPS was reported: %+v", diags)
		}
	})

	t.Run("a named certificate serves HTTPS without a store", func(t *testing.T) {
		block := map[string]any{"enabled": "true", "certificate": "lg-cert"}
		diags := check.Check(diagnostic.DoctorCheckContext{Tree: lgEntriesTree(block, entry(nil))})
		if len(diags) != 0 {
			t.Fatalf("a looking glass that presents a named certificate was reported: %+v", diags)
		}
	})

	t.Run("tls false asked for HTTP and owes no finding", func(t *testing.T) {
		diags := check.Check(diagnostic.DoctorCheckContext{Tree: lgEntriesTree(enabled, entry(map[string]any{"tls": "false"}))})
		if len(diags) != 0 {
			t.Fatalf("an entry that asked for HTTP was reported: %+v", diags)
		}
	})

	t.Run("tls true with no certificate is the refusal ze starts with", func(t *testing.T) {
		diags := check.Check(diagnostic.DoctorCheckContext{Tree: lgEntriesTree(enabled, entry(map[string]any{"tls": "true"}))})
		diag := lgOneFinding(t, diags, codeDoctorLGTLSUnservable, diagnostic.SeverityError)
		_, startErr := planLGTransports([]lgListener{{addr: addr, tls: zeconfig.LGServerTLSRequired}}, "", false)
		if startErr == nil {
			t.Fatal("the start path must refuse a tls true entry with no certificate")
		}
		if !strings.Contains(startErr.Error(), diag.Fix) {
			t.Fatalf("doctor fix %q is not the fix of the start refusal %v", diag.Fix, startErr)
		}
	})

	t.Run("an unreadable ze.looking-glass.listen is reported", func(t *testing.T) {
		if err := env.Set("ze.looking-glass.listen", "not-an-endpoint"); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = env.Set("ze.looking-glass.listen", "") })
		diags := check.Check(diagnostic.DoctorCheckContext{Tree: lgEntriesTree(enabled, entry(nil))})
		diag := lgOneFinding(t, diags, codeDoctorLGListenUnreadable, diagnostic.SeverityError)
		if !strings.Contains(diag.Message, "not-an-endpoint") {
			t.Fatalf("the finding must name the value: %+v", diag)
		}
		if diag.Actual == nil {
			t.Fatalf("the finding must carry the parse error: %+v", diag)
		}
		if !strings.Contains(diag.Fix, "ze.looking-glass.listen") {
			t.Fatalf("the fix must name the variable to correct: %+v", diag)
		}
	})

	t.Run("a config that starts no looking glass reports nothing", func(t *testing.T) {
		diags := check.Check(diagnostic.DoctorCheckContext{Tree: lgEntriesTree(nil, entry(nil))})
		if len(diags) != 0 {
			t.Fatalf("a dormant block was reported: %+v", diags)
		}
	})
}
