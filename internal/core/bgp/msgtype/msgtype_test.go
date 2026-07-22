package msgtype

import "testing"

// TestMessageTypeString pins the human-readable names every log line, MRT
// record, and monitor event renders.
//
// VALIDATES: each RFC-defined type code renders its RFC name, and an unknown
// code renders UNKNOWN(<code>) rather than an empty string.
// PREVENTS: the lift out of internal/component/bgp/message silently changing an
// operator-visible message-type name.
func TestMessageTypeString(t *testing.T) {
	tests := []struct {
		typ  MessageType
		want string
	}{
		{TypeOPEN, "OPEN"},
		{TypeUPDATE, "UPDATE"},
		{TypeNOTIFICATION, "NOTIFICATION"},
		{TypeKEEPALIVE, "KEEPALIVE"},
		{TypeROUTEREFRESH, "ROUTE-REFRESH"},
		{MessageType(0), "UNKNOWN(0)"},
		{MessageType(99), "UNKNOWN(99)"},
	}
	for _, tt := range tests {
		if got := tt.typ.String(); got != tt.want {
			t.Errorf("MessageType(%d).String() = %q, want %q", tt.typ, got, tt.want)
		}
	}
}

// TestMessageTypeCodes pins the RFC wire code of every message type. The codes
// are the wire contract (RFC 4271 Section 4.1, RFC 2918 for ROUTE-REFRESH);
// a renumbering would silently corrupt every BGP message header.
func TestMessageTypeCodes(t *testing.T) {
	codes := map[MessageType]uint8{
		TypeOPEN:         1,
		TypeUPDATE:       2,
		TypeNOTIFICATION: 3,
		TypeKEEPALIVE:    4,
		TypeROUTEREFRESH: 5,
	}
	for typ, want := range codes {
		if uint8(typ) != want {
			t.Errorf("%s = %d, want wire code %d", typ, uint8(typ), want)
		}
	}
}
