// Design: docs/architecture/wire/messages.md — BGP message types
// RFC: rfc/short/rfc4271.md — message header format (Section 4.1)

// Package msgtype owns the BGP message-type code (the 1-octet Type field of
// the RFC 4271 header) and its RFC-defined values.
//
// It lives in internal/core because always-on consumers outside the BGP engine
// classify raw BGP messages by type -- the MRT recorder writes a BGP4MP record
// per message and only distinguishes UPDATE from everything else -- and must
// keep compiling when the engine is compiled out (//go:build ze_bgp). The BGP
// codec (internal/component/bgp/message) owns everything else about the header;
// only the type vocabulary is shared.
package msgtype

import (
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// MessageType is the RFC 4271 Section 4.1 Type field: a 1-octet unsigned
// integer indicating the message type.
type MessageType uint8

// RFC 4271 Section 4.1 - Message type codes.
// Types 1-4 are defined in RFC 4271, type 5 (ROUTE-REFRESH) in RFC 2918.
const (
	TypeOPEN         MessageType = 1 // RFC 4271 Section 4.1
	TypeUPDATE       MessageType = 2 // RFC 4271 Section 4.1
	TypeNOTIFICATION MessageType = 3 // RFC 4271 Section 4.1
	TypeKEEPALIVE    MessageType = 4 // RFC 4271 Section 4.1
	TypeROUTEREFRESH MessageType = 5 // RFC 2918
)

// typeNames maps a message type to the lowercase word that names it, indexed by
// the RFC 4271 type code.
//
// It is the ONLY place in Ze that spells the five names. The uppercase form
// String answers is derived from it at startup, and every surface that takes a
// name from an operator reads it through FromText, so the word `send bgp raw`
// accepts and the word a log line prints cannot drift. The same five words are
// the `type` enumeration of ze-raw-cmd.yang.
//
// Index 0 is the empty string: RFC 4271 defines no type 0, so no name is owed
// for one, and FromText refuses an empty name for that reason.
var typeNames = [...]string{
	TypeOPEN:         "open",
	TypeUPDATE:       "update",
	TypeNOTIFICATION: "notification",
	TypeKEEPALIVE:    "keepalive",
	TypeROUTEREFRESH: "route-refresh",
}

// typeNamesUpper is typeNames in the case String answers, built once at startup
// so String costs an index rather than an allocation.
var typeNamesUpper = func() [len(typeNames)]string {
	var upper [len(typeNames)]string
	for code, name := range typeNames {
		upper[code] = strings.ToUpper(name)
	}
	return upper
}()

// String returns a human-readable name for the message type.
func (t MessageType) String() string {
	if int(t) >= len(typeNamesUpper) || typeNamesUpper[t] == "" {
		var b textbuf.Buffer
		return b.Reset().Str("UNKNOWN(").Int(int64(t)).Byte(')').String()
	}
	return typeNamesUpper[t]
}

// LowerString returns the lowercase name a CLI and a YANG enum use, or the
// empty string for a code no RFC defines.
func (t MessageType) LowerString() string {
	if int(t) >= len(typeNames) {
		return ""
	}
	return typeNames[t]
}

// FromText answers the MessageType a lowercase name spells, and whether the
// name is one of them. The match is exact, so a caller that accepts a
// mixed-case word lowercases it first.
func FromText(name string) (MessageType, bool) {
	if name == "" {
		return 0, false
	}
	for code, known := range typeNames {
		if known == name {
			return MessageType(code), true //nolint:gosec // G115: the index is bounded by a six-entry array
		}
	}
	return 0, false
}

// TextNames answers every lowercase name, in type-code order, for an error
// message or a help line that must offer the whole set.
func TextNames() []string {
	names := make([]string, 0, len(typeNames))
	for _, name := range typeNames {
		if name != "" {
			names = append(names, name)
		}
	}
	return names
}
