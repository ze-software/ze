//go:build ze_bgp

package cli

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/diagnostic"
)

// rfc8212Config builds a one-peer BGP config: the peer's remote AS, whether it
// sits in a group, and the filter block it carries (empty for none).
func rfc8212Config(remoteAS string, inGroup bool, filter string) string {
	peer := `peer edge {
		connection {
			remote { ip 192.0.2.1; }
			local { ip 192.0.2.2; }
		}
		session { asn { remote ` + remoteAS + `; } }
		` + filter + `
	}`
	if inGroup {
		peer = `group transit {
		` + peer + `
	}`
	}
	return `bgp {
	router-id 192.0.2.2;
	session { asn { local 65000; } }
	` + peer + `
}`
}

// rfc8212Warnings returns the RFC 8212 warnings validation reports.
func rfc8212Warnings(t *testing.T, input string) []diagnostic.Diagnostic {
	t.Helper()
	result := runValidation(input, "test.conf")
	if !result.Valid {
		t.Fatalf("config must stay valid, got %+v", result.Diagnostics)
	}
	var warnings []diagnostic.Diagnostic
	for i := range result.Diagnostics {
		if result.Diagnostics[i].Code == rfc8212WarningCode {
			warnings = append(warnings, result.Diagnostics[i])
		}
	}
	return warnings
}

// TestValidateWarnsRFC8212AbsentPolicy proves `ze config validate` names each
// direction RFC 8212 closes on an EBGP peer, in the words of the daemon's
// establishment warning, with the editor command that fixes it, and keeps the
// config valid.
//
// VALIDATES: an EBGP peer with no import and no export policy yields one
// warning per direction; a configured direction and an IBGP peer yield none;
// a group member's fix names the group.
// PREVENTS: a config whose EBGP sessions exchange no routes validating with
// no word about it, the silence that hid the perf lab DUT config's break.
func TestValidateWarnsRFC8212AbsentPolicy(t *testing.T) {
	importOnly := `filter { import { final accept; } }`
	both := `filter { import { final accept; } export { final accept; } }`
	cases := []struct {
		name    string
		input   string
		want    []string
		wantFix []string
	}{
		{
			name:  "ebgp peer without policy",
			input: rfc8212Config("65001", false, ""),
			want: []string{
				"peer edge: RFC 8212: this EBGP peer has no import policy, so Ze uses none of the routes it receives from it; to exchange routes, run `set bgp peer edge filter import final accept`, then `commit now`",
				"peer edge: RFC 8212: this EBGP peer has no export policy, so Ze sends it no routes; to exchange routes, run `set bgp peer edge filter export final accept`, then `commit now`",
			},
			wantFix: []string{"set bgp peer edge filter import final accept", "set bgp peer edge filter export final accept"},
		},
		{
			name:    "ebgp peer with import only",
			input:   rfc8212Config("65001", false, importOnly),
			want:    []string{"peer edge: RFC 8212: this EBGP peer has no export policy, so Ze sends it no routes; to exchange routes, run `set bgp peer edge filter export final accept`, then `commit now`"},
			wantFix: []string{"set bgp peer edge filter export final accept"},
		},
		{
			name:  "ebgp peer with both directions",
			input: rfc8212Config("65001", false, both),
		},
		{
			name:  "ibgp peer without policy",
			input: rfc8212Config("65000", false, ""),
		},
		{
			name:    "group member without export",
			input:   rfc8212Config("65001", true, importOnly),
			want:    []string{"peer edge: RFC 8212: this EBGP peer has no export policy, so Ze sends it no routes; to exchange routes, run `set bgp group transit peer edge filter export final accept`, then `commit now`"},
			wantFix: []string{"set bgp group transit peer edge filter export final accept"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			warnings := rfc8212Warnings(t, tc.input)
			if len(warnings) != len(tc.want) {
				t.Fatalf("got %d RFC 8212 warnings, want %d: %+v", len(warnings), len(tc.want), warnings)
			}
			for i := range warnings {
				w := &warnings[i]
				if w.Severity != diagnostic.SeverityWarning {
					t.Errorf("warning %d: severity %v, want warning", i, w.Severity)
				}
				if w.Message != tc.want[i] {
					t.Errorf("warning %d message:\n got %s\nwant %s", i, w.Message, tc.want[i])
				}
				if w.Help != tc.wantFix[i] {
					t.Errorf("warning %d help: got %q, want %q", i, w.Help, tc.wantFix[i])
				}
				if !strings.Contains(w.Message, "RFC 8212") {
					t.Errorf("warning %d does not cite RFC 8212: %s", i, w.Message)
				}
			}
		})
	}
}
