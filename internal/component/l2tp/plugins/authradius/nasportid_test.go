// NAS-Port-Id (RFC 2869 Section 5.17) template resolution and emission.
//
// VALIDATES: the operator-configured nas-port-id-format resolves to the same text
// in the Access-Request and in every Accounting-Request of one session, and an
// unresolvable format is refused at config time rather than emitted half-expanded.
// PREVENTS: a NAS-Port-Id that differs between auth and accounting (billing cannot
// correlate the records), and a literal "{unknown}" reaching the RADIUS server.

package l2tpauthradius

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/component/radius"
)

func TestNASPortIDResolve(t *testing.T) {
	facts := nasPortIDFacts{nasID: "lns1", tunnelID: 1027, sessionID: 42}

	cases := []struct {
		name   string
		format string
		want   string
	}{
		{"empty", "", ""},
		{"literal only", "slot0/port1", "slot0/port1"},
		{"nas-id", "{nas-id}", "lns1"},
		{"tunnel and session", "{tunnel-id}:{session-id}", "1027:42"},
		{"all three", "{nas-id}-{tunnel-id}-{session-id}", "lns1-1027-42"},
		{"mixed literal", "l2tp:{tunnel-id}.{session-id}/0", "l2tp:1027.42/0"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := resolveNASPortID(tc.format, facts); got != tc.want {
				t.Fatalf("resolveNASPortID(%q) = %q, want %q", tc.format, got, tc.want)
			}
		})
	}
}

// An empty nas-identifier leaves the {nas-id} placeholder empty; the resolved
// value is then whatever the literals say. A format that resolves to the empty
// string must not be emitted (checked in the emission tests below).
func TestNASPortIDResolveEmptyNASID(t *testing.T) {
	got := resolveNASPortID("{nas-id}", nasPortIDFacts{tunnelID: 1, sessionID: 2})
	if got != "" {
		t.Fatalf("resolveNASPortID with empty nas-id = %q, want empty", got)
	}
}

func TestValidateNASPortIDFormatAccepts(t *testing.T) {
	for _, format := range []string{"", "static", "{nas-id}", "{tunnel-id}:{session-id}", "a{nas-id}b{tunnel-id}c"} {
		if err := validateNASPortIDFormat(format); err != nil {
			t.Fatalf("validateNASPortIDFormat(%q) = %v, want nil", format, err)
		}
	}
}

func TestValidateNASPortIDFormatRejects(t *testing.T) {
	cases := []struct {
		name   string
		format string
	}{
		{"unknown placeholder", "{svlan}"},
		{"unterminated brace", "{tunnel-id"},
		{"bare open brace", "port{"},
		{"empty placeholder", "{}"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateNASPortIDFormat(tc.format)
			if err == nil {
				t.Fatalf("validateNASPortIDFormat(%q) = nil, want an error", tc.format)
			}
			// The error must name the placeholders that ARE supported, derived
			// from the same list the resolver uses (ai/rules/evidence.md).
			for _, name := range nasPortIDPlaceholders {
				if !strings.Contains(err.Error(), name) {
					t.Fatalf("error %q does not name supported placeholder %q", err, name)
				}
			}
		})
	}
}

// RFC 2869 Section 5.17: NAS-Port-Id "is only used in Access-Request and
// Accounting-Request packets" and carries UTF-8 text.
func TestAccessRequestCarriesNASPortID(t *testing.T) {
	req := ppp.EventAuthRequest{TunnelID: 1027, SessionID: 42, Username: "alice", Method: ppp.AuthMethodPAP}

	attrs, ok := buildAccessRequestAttrs(req, "lns1", nil, "{nas-id}:{tunnel-id}.{session-id}")
	if !ok {
		t.Fatal("a PAP request MUST build an Access-Request")
	}
	pkt := &radius.Packet{Code: radius.CodeAccessRequest, Attrs: attrs}
	vals := pkt.FindAllAttr(radius.AttrNASPortID)
	if len(vals) != 1 {
		t.Fatalf("NAS-Port-Id count: got %d, want 1", len(vals))
	}
	if got := string(vals[0]); got != "lns1:1027.42" {
		t.Fatalf("NAS-Port-Id = %q, want %q", got, "lns1:1027.42")
	}
}

// No format configured: the Access-Request is byte-identical to today's.
func TestAccessRequestOmitsNASPortIDWhenUnset(t *testing.T) {
	req := ppp.EventAuthRequest{TunnelID: 1027, SessionID: 42, Username: "alice", Method: ppp.AuthMethodPAP}

	attrs, ok := buildAccessRequestAttrs(req, "lns1", nil, "")
	if !ok {
		t.Fatal("a PAP request MUST build an Access-Request")
	}
	pkt := &radius.Packet{Code: radius.CodeAccessRequest, Attrs: attrs}
	if vals := pkt.FindAllAttr(radius.AttrNASPortID); len(vals) != 0 {
		t.Fatalf("NAS-Port-Id present with no format configured: %q", vals)
	}
}

func TestAcctRequestCarriesNASPortID(t *testing.T) {
	acct := newRADIUSAcct()
	sess := &acctSession{tunnelID: 1027, sessionID: 42, username: "alice", acctSessID: "1027-42-1", nasPortID: "lns1:1027.42"}

	for _, status := range []uint8{radius.AcctStatusStart, radius.AcctStatusInterimUpdate, radius.AcctStatusStop} {
		pkt := acct.buildAcctPacket(sess, "lns1", nil, status, 0)
		vals := pkt.FindAllAttr(radius.AttrNASPortID)
		if len(vals) != 1 {
			t.Fatalf("status %d: NAS-Port-Id count: got %d, want 1", status, len(vals))
		}
		if got := string(vals[0]); got != "lns1:1027.42" {
			t.Fatalf("status %d: NAS-Port-Id = %q, want %q", status, got, "lns1:1027.42")
		}
	}
}

// The Access-Request and the Accounting-Requests of one session MUST carry the
// same NAS-Port-Id text, or the billing system cannot join the records.
func TestNASPortIDSameInAuthAndAcct(t *testing.T) {
	const format = "{nas-id}:{tunnel-id}.{session-id}"

	req := ppp.EventAuthRequest{TunnelID: 7, SessionID: 9, Username: "bob", Method: ppp.AuthMethodPAP}
	authAttrs, ok := buildAccessRequestAttrs(req, "lns1", nil, format)
	if !ok {
		t.Fatal("a PAP request MUST build an Access-Request")
	}
	authPkt := &radius.Packet{Attrs: authAttrs}

	acct := newRADIUSAcct()
	sess := &acctSession{
		tunnelID:   7,
		sessionID:  9,
		username:   "bob",
		acctSessID: "7-9-1",
		nasPortID:  resolveNASPortID(format, nasPortIDFacts{nasID: "lns1", tunnelID: 7, sessionID: 9}),
	}
	acctPkt := acct.buildAcctPacket(sess, "lns1", nil, radius.AcctStatusStart, 0)

	got := string(authPkt.FindAttr(radius.AttrNASPortID))
	want := string(acctPkt.FindAttr(radius.AttrNASPortID))
	if got == "" || got != want {
		t.Fatalf("NAS-Port-Id auth=%q acct=%q, want equal and non-empty", got, want)
	}
}

func TestAcctRequestOmitsNASPortIDWhenUnset(t *testing.T) {
	acct := newRADIUSAcct()
	sess := &acctSession{tunnelID: 1027, sessionID: 42, username: "alice", acctSessID: "1027-42-1"}
	pkt := acct.buildAcctPacket(sess, "lns1", nil, radius.AcctStatusStart, 0)
	if vals := pkt.FindAllAttr(radius.AttrNASPortID); len(vals) != 0 {
		t.Fatalf("NAS-Port-Id present with no format configured: %q", vals)
	}
}

// A format that resolves to the empty string (every placeholder empty, no
// literals) yields no attribute: RFC 2869 Section 5.17 gives NAS-Port-Id a
// minimum Length of 3, which is Type + Length + at least one text octet.
func TestNASPortIDEmptyResolutionOmitted(t *testing.T) {
	acct := newRADIUSAcct()
	sess := &acctSession{
		tunnelID:   1027,
		sessionID:  42,
		username:   "alice",
		acctSessID: "1027-42-1",
		nasPortID:  resolveNASPortID("{nas-id}", nasPortIDFacts{tunnelID: 1027, sessionID: 42}),
	}
	pkt := acct.buildAcctPacket(sess, "", nil, radius.AcctStatusStart, 0)
	if vals := pkt.FindAllAttr(radius.AttrNASPortID); len(vals) != 0 {
		t.Fatalf("NAS-Port-Id emitted for an empty resolution: %q", vals)
	}
}

func nasPortIDConfigTree(format string) map[string]any {
	radiusBlock := map[string]any{
		"nas-identifier": "lns1",
		"server": []any{
			map[string]any{"name": "radius1", "address": "10.0.0.1", "shared-key": "secret123"},
		},
	}
	if format != "" {
		radiusBlock["nas-port-id-format"] = format
	}
	return map[string]any{
		"l2tp": map[string]any{"auth": map[string]any{"radius": radiusBlock}},
	}
}

func TestParseConfigNASPortIDFormat(t *testing.T) {
	cfg, err := parseConfigFromTree(nasPortIDConfigTree("{nas-id}:{tunnel-id}.{session-id}"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.NASPortIDFormat != "{nas-id}:{tunnel-id}.{session-id}" {
		t.Fatalf("NASPortIDFormat = %q", cfg.NASPortIDFormat)
	}
}

func TestParseConfigNASPortIDFormatAbsent(t *testing.T) {
	cfg, err := parseConfigFromTree(nasPortIDConfigTree(""))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.NASPortIDFormat != "" {
		t.Fatalf("NASPortIDFormat = %q, want empty", cfg.NASPortIDFormat)
	}
}

// A format naming a placeholder ze cannot resolve is refused when the config is
// parsed. Accepting it would emit the literal text to the RADIUS server, which
// no operator asked for and no billing system can undo.
func TestParseConfigRejectsUnknownPlaceholder(t *testing.T) {
	_, err := parseConfigFromTree(nasPortIDConfigTree("{interface}:{svlan}.{cvlan}"))
	if err == nil {
		t.Fatal("parseConfigFromTree accepted an unknown placeholder")
	}
	if !strings.Contains(err.Error(), "nas-port-id-format") {
		t.Fatalf("error does not name the leaf: %v", err)
	}
}

// A template that can only ever resolve to nothing is refused at commit time.
// The operator asked for {nas-id} and set no nas-identifier, so every packet
// would silently carry no NAS-Port-Id at all.
func TestParseConfigRejectsNASIDPlaceholderWithoutNASIdentifier(t *testing.T) {
	tree := nasPortIDConfigTree("{nas-id}:{tunnel-id}")
	l2tpBlock, _ := tree["l2tp"].(map[string]any)
	authBlock, _ := l2tpBlock["auth"].(map[string]any)
	radiusBlock, _ := authBlock["radius"].(map[string]any)
	delete(radiusBlock, "nas-identifier")

	_, err := parseConfigFromTree(tree)
	if err == nil {
		t.Fatal("parseConfigFromTree accepted {nas-id} with no nas-identifier")
	}
	if !strings.Contains(err.Error(), "nas-identifier") {
		t.Fatalf("error does not name the missing leaf: %v", err)
	}
}

// A template longer than a RADIUS attribute value is refused at commit time
// rather than expanded per packet and dropped. Boundary: 253 commits, 254 does
// not (RFC 2865 Section 5).
func TestParseConfigNASPortIDFormatLengthBoundary(t *testing.T) {
	for _, tc := range []struct {
		length int
		accept bool
	}{{253, true}, {254, false}} {
		_, err := parseConfigFromTree(nasPortIDConfigTree(strings.Repeat("x", tc.length)))
		if tc.accept && err != nil {
			t.Fatalf("%d-character format rejected: %v", tc.length, err)
		}
		if !tc.accept && err == nil {
			t.Fatalf("%d-character format accepted", tc.length)
		}
	}
}

// RFC 2865 Section 5: an attribute Length is one octet and covers Type and
// Length too, so a value is at most 253 octets. Boundary: 1 and 253 are carried,
// 0 and 254 are not. An over-long resolution is dropped, never truncated.
func TestNASPortIDLengthBoundary(t *testing.T) {
	cases := []struct {
		name    string
		length  int
		emitted bool
	}{
		{"zero octets", 0, false},
		{"one octet", 1, true},
		{"253 octets (last valid)", 253, true},
		{"254 octets (first invalid)", 254, false},
	}
	sess := &acctSession{tunnelID: 1, sessionID: 2, username: "alice", acctSessID: "1-2-1"}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			acct := newRADIUSAcct()
			sess.nasPortID = strings.Repeat("x", tc.length)
			pkt := acct.buildAcctPacket(sess, "lns1", nil, radius.AcctStatusStart, 0)
			vals := pkt.FindAllAttr(radius.AttrNASPortID)
			if tc.emitted && len(vals) != 1 {
				t.Fatalf("%d-octet value: got %d attributes, want 1", tc.length, len(vals))
			}
			if !tc.emitted && len(vals) != 0 {
				t.Fatalf("%d-octet value: got %d attributes, want 0", tc.length, len(vals))
			}
			if tc.emitted && len(vals[0]) != tc.length {
				t.Fatalf("value length: got %d, want %d", len(vals[0]), tc.length)
			}
		})
	}
}
