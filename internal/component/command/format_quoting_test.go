// Design: docs/architecture/api/commands.md -- YAML output quoting and round-trip fidelity.

package command

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestRenderYAMLQuotesScalarsAndKeysRoundTrip proves that RenderYAML produces
// valid YAML 1.2 that round-trips every string scalar and map key back to the
// exact original string without type coercion or syntax corruption.
//
// VALIDATES: plan/journal/declared-format-contradicts-payload.md (2026-10-26)
// -- values such as `true`, `[REDACTED]`, numbers, timestamps, empty strings,
// leading/trailing whitespace, indicators, and embedded newlines/control
// characters survive yaml.Unmarshal as identical Go strings, both as map
// values, map keys, sequence elements, and top-level scalars.
// PREVENTS: unquoted YAML keys or string scalars changing type or breaking
// document structure when parsed by YAML 1.2 readers.
func TestRenderYAMLQuotesScalarsAndKeysRoundTrip(t *testing.T) {
	trickyStrings := []struct {
		name  string
		value string
	}{
		{"empty", ""},
		{"leading space", " leading"},
		{"trailing space", "trailing "},
		{"spaces only", "   "},
		{"tab only", "\t"},
		{"bool true lower", "true"},
		{"bool true title", "True"},
		{"bool true upper", "TRUE"},
		{"bool false lower", "false"},
		{"bool false title", "False"},
		{"bool false upper", "FALSE"},
		{"yaml11 yes", "yes"},
		{"yaml11 no", "no"},
		{"yaml11 on", "on"},
		{"yaml11 off", "off"},
		{"yaml11 y", "y"},
		{"yaml11 n", "n"},
		{"null lower", "null"},
		{"null title", "Null"},
		{"null upper", "NULL"},
		{"tilde null", "~"},
		{"merge key", "<<"},
		{"value key", "="},
		{"nan", ".nan"},
		{"nan title", ".NaN"},
		{"inf", ".inf"},
		{"pos inf", "+.inf"},
		{"neg inf", "-.inf"},
		{"int zero", "0"},
		{"int decimal", "42"},
		{"int negative", "-42"},
		{"int positive", "+42"},
		{"int octal legacy", "0777"},
		{"int octal 1.2", "0o755"},
		{"int hex", "0xdeadbeef"},
		{"int binary", "0b10101"},
		{"int underscore", "1_000_000"},
		{"int huge", "99999999999999999999999999"},
		{"float", "1.0"},
		{"float leading dot", ".5"},
		{"float trailing dot", "1."},
		{"float exp", "1.5e-3"},
		{"date", "2026-11-23"},
		{"timestamp", "2026-11-23T12:34:56Z"},
		{"redacted brackets", "[REDACTED]"},
		{"flow map", "{a: b}"},
		{"colon space", "a: b"},
		{"trailing colon", "fe80::"},
		{"ipv6", "2001:db8::1"},
		{"hash comment", "foo # bar"},
		{"leading hash", "#comment"},
		{"comma", "a, b"},
		{"anchor", "&anchor"},
		{"alias", "*alias"},
		{"tag", "!tag"},
		{"pipe literal", "|block"},
		{"gt folded", ">fold"},
		{"single quotes", "'single'"},
		{"double quotes", `"double"`},
		{"percent directive", "%YAML"},
		{"at reserved", "@at"},
		{"backtick reserved", "`tick`"},
		{"question mark", "?query"},
		{"dash item", "- item"},
		{"single dash", "-"},
		{"doc start", "---"},
		{"doc end", "..."},
		{"backslash", `a\b\n`},
		{"newline", "line1\nline2"},
		{"crlf", "line1\r\nline2"},
		{"embedded tab", "col1\tcol2"},
		{"nul byte", "nul\x00byte"},
		{"c0 escapes", "bell\a\b\f\v\x1b"},
		{"c0 hex", "soh\x01del\x7f"},
		{"unicode line breaks", "nel\u0085nbsp\u00a0ls\u2028ps\u2029"},
		{"c1 control", "c1\u0090end"},
		{"utf8 printable", "héllo 世界"},
	}

	for _, tc := range trickyStrings {
		t.Run("value/"+tc.name, func(t *testing.T) {
			in := map[string]any{"field": tc.value}
			rendered := RenderYAML(in)

			var got map[string]any
			if err := yaml.Unmarshal([]byte(rendered), &got); err != nil {
				t.Fatalf("yaml.Unmarshal failed for %q: %v\nrendered:\n%s", tc.value, err, rendered)
			}
			val, ok := got["field"]
			if !ok {
				t.Fatalf("missing key 'field' in decoded map %#v\nrendered:\n%s", got, rendered)
			}
			gotStr, isStr := val.(string)
			if !isStr {
				t.Fatalf("value %q decoded as %T (%#v), want string\nrendered:\n%s", tc.value, val, val, rendered)
			}
			if gotStr != tc.value {
				t.Fatalf("value round-trip = %q, want %q\nrendered:\n%s", gotStr, tc.value, rendered)
			}
		})

		t.Run("key/"+tc.name, func(t *testing.T) {
			in := map[string]any{tc.value: "ok"}
			rendered := RenderYAML(in)

			var got map[string]any
			if err := yaml.Unmarshal([]byte(rendered), &got); err != nil {
				t.Fatalf("yaml.Unmarshal failed for key %q: %v\nrendered:\n%s", tc.value, err, rendered)
			}
			if len(got) != 1 {
				t.Fatalf("decoded key count = %d (%#v), want 1\nrendered:\n%s", len(got), got, rendered)
			}
			val, ok := got[tc.value]
			if !ok {
				t.Fatalf("decoded map %#v missing exact string key %q\nrendered:\n%s", got, tc.value, rendered)
			}
			if val != "ok" {
				t.Fatalf("value under key %q = %#v, want \"ok\"\nrendered:\n%s", tc.value, val, rendered)
			}
		})

		t.Run("list-item/"+tc.name, func(t *testing.T) {
			in := map[string]any{"items": []any{tc.value}}
			rendered := RenderYAML(in)

			var got map[string]any
			if err := yaml.Unmarshal([]byte(rendered), &got); err != nil {
				t.Fatalf("yaml.Unmarshal failed for list item %q: %v\nrendered:\n%s", tc.value, err, rendered)
			}
			items, ok := got["items"].([]any)
			if !ok || len(items) != 1 {
				t.Fatalf("items = %#v, want 1-element slice\nrendered:\n%s", got["items"], rendered)
			}
			gotStr, isStr := items[0].(string)
			if !isStr {
				t.Fatalf("list item %q decoded as %T (%#v), want string\nrendered:\n%s", tc.value, items[0], items[0], rendered)
			}
			if gotStr != tc.value {
				t.Fatalf("list item round-trip = %q, want %q\nrendered:\n%s", gotStr, tc.value, rendered)
			}
		})

		t.Run("top-level/"+tc.name, func(t *testing.T) {
			rendered := RenderYAML(tc.value)

			var got any
			if err := yaml.Unmarshal([]byte(rendered), &got); err != nil {
				t.Fatalf("yaml.Unmarshal failed for top-level %q: %v\nrendered:\n%s", tc.value, err, rendered)
			}
			gotStr, isStr := got.(string)
			if !isStr {
				t.Fatalf("top-level %q decoded as %T (%#v), want string\nrendered:\n%s", tc.value, got, got, rendered)
			}
			if gotStr != tc.value {
				t.Fatalf("top-level round-trip = %q, want %q\nrendered:\n%s", gotStr, tc.value, rendered)
			}
		})
	}
}

// TestRenderYAMLKeepsSafePlainScalarsUnquoted proves that ordinary Ze tokens
// (IPv4 addresses, prefixes, kebab-case names, durations, semver) remain clean
// unquoted YAML scalars while still round-tripping as strings.
//
// VALIDATES: quoting is applied only when YAML syntax or type resolution
// requires it, keeping human-readable YAML output clean.
// PREVENTS: over-quoting every string scalar and breaking existing YAML
// readability and callers expecting plain `address: 192.0.2.1`.
func TestRenderYAMLKeepsSafePlainScalarsUnquoted(t *testing.T) {
	safe := []string{
		"192.0.2.1",
		"10.0.0.0/24",
		"established",
		"request mrt dump-rib",
		"1h0m0s",
		"1.2.3",
		"show-bgp-peer",
		"ipv4/unicast",
	}
	for _, value := range safe {
		rendered := RenderYAML(map[string]any{"field": value})
		wantLine := "field: " + value + "\n"
		if rendered != wantLine {
			t.Errorf("RenderYAML(%q) = %q, want unquoted %q", value, rendered, wantLine)
		}
		var got map[string]any
		if err := yaml.Unmarshal([]byte(rendered), &got); err != nil {
			t.Fatalf("yaml.Unmarshal(%q): %v", rendered, err)
		}
		if got["field"] != value {
			t.Errorf("round-trip %q = %#v (%T)", value, got["field"], got["field"])
		}
	}
}

// TestRenderYAMLContainersAndPipeRoundTrip verifies empty containers, nested
// slices, mixed JSON types, and the real `| yaml` pipe path produce valid YAML
// that unmarshals to the exact input structure.
//
// VALIDATES: ApplyPipes `| yaml` produces valid parseable YAML when real
// command JSON carries redacted secrets, multiline text, bool-like strings,
// and empty maps/slices.
// PREVENTS: structural corruption in nested containers or divergence between
// RenderYAML and the `| yaml` pipe operator.
func TestRenderYAMLContainersAndPipeRoundTrip(t *testing.T) {
	payload := `{
		"bool-real": true,
		"bool-string": "true",
		"null-real": null,
		"null-string": "null",
		"empty-map": {},
		"empty-list": [],
		"secret": "[REDACTED]",
		"multiline": "first\nsecond: third",
		"nested-lists": [["a", "true"], []],
		"rows": [
			{"": "empty-key", "a: b": "colon-key", "empty": {}}
		]
	}`

	var want map[string]any
	if err := json.Unmarshal([]byte(payload), &want); err != nil {
		t.Fatalf("fixture JSON: %v", err)
	}

	_, formatFn, errMsg := processPipesChecked("show test | yaml")
	if errMsg != "" {
		t.Fatalf("processPipesChecked: %s", errMsg)
	}
	rendered := formatFn(payload)
	if !strings.Contains(rendered, `"[REDACTED]"`) {
		t.Fatalf("expected [REDACTED] to be quoted in YAML:\n%s", rendered)
	}

	var got map[string]any
	if err := yaml.Unmarshal([]byte(rendered), &got); err != nil {
		t.Fatalf("yaml.Unmarshal failed: %v\nrendered:\n%s", err, rendered)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("YAML round-trip mismatch:\ngot:  %#v\nwant: %#v\nrendered:\n%s", got, want, rendered)
	}
}
