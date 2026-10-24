// Design: docs/architecture/api/json-format.md -- the peer object, remote.as
// Related: bridge.go -- StructuredEvent.PeerAS carries it to in-process plugins

package rpc

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
)

// RemoteAS is a peer's AS number as the speaker knows it when an event is
// built. A dynamic peer's AS is learned from the peer's OPEN, so an event
// written before the OPENs are exchanged (the OPEN Ze sends, a NOTIFICATION
// Ze sends in OpenSent, the FSM changes before OpenConfirm) has no AS to name.
// The zero value is that "not yet known" state, so a RemoteAS nobody filled in
// reads as unknown and never as AS 0. Every event writer renders it through
// ASN: the JSON peer object omits remote.as, and a text line says
// `remote as unknown`.
//
// A consumer that renders by reflection gets the same answer from the type
// itself, so none can write the struct's fields: encoding/json writes the
// number or null (a field tagged `omitzero` is omitted while unknown, through
// IsZero), fmt's %v and %s write the number or "unknown" (String), and
// log/slog does the same (LogValue). %d is not covered: fmt formats the
// fields for it, and go vet's printf check reports it.
type RemoteAS struct {
	asn   uint32
	known bool
}

// RemoteASOf names the peer AS a speaker holds. A peer's settings store 0
// while a dynamic peer's AS is not yet learned, and 0 is the one number no
// peer can hold, so it becomes the unknown state here rather than an AS.
//
// RFC 7607 Section 2: "A router MUST NOT initiate a connection claiming to be AS 0."
func RemoteASOf(asn uint32) RemoteAS {
	if asn == 0 {
		return RemoteAS{}
	}
	return RemoteAS{asn: asn, known: true}
}

// ASN returns the peer's AS number, and false while it is not yet known. A
// caller MUST branch on the bool: the number is meaningless without it.
func (a RemoteAS) ASN() (uint32, bool) {
	return a.asn, a.known
}

// remoteASUnknownWord is the word every text surface writes for an AS not yet
// known, as `remote as unknown` does on a text event line.
const remoteASUnknownWord = "unknown"

// String writes the AS number, or "unknown" while it is not yet known.
func (a RemoteAS) String() string {
	if !a.known {
		return remoteASUnknownWord
	}
	var digits [10]byte
	return string(strconv.AppendUint(digits[:0], uint64(a.asn), 10))
}

// LogValue makes a log attribute carry the AS number, or "unknown" while it
// is not yet known.
func (a RemoteAS) LogValue() slog.Value {
	if !a.known {
		return slog.StringValue(remoteASUnknownWord)
	}
	return slog.Uint64Value(uint64(a.asn))
}

// IsZero reports the "not yet known" state. encoding/json calls it for a
// field tagged `omitzero`, which is how the JSON peer object omits remote.as.
func (a RemoteAS) IsZero() bool {
	return !a.known
}

// MarshalJSON writes the AS number, or null while it is not yet known.
func (a RemoteAS) MarshalJSON() ([]byte, error) {
	if !a.known {
		return []byte("null"), nil
	}
	return strconv.AppendUint(nil, uint64(a.asn), 10), nil
}

// UnmarshalJSON reads what MarshalJSON writes: null is not yet known, and a
// number is the AS. AS 0 is refused rather than read as unknown, because a
// document that names it claims an AS no peer can hold.
//
// RFC 7607 Section 2: "A router MUST NOT initiate a connection claiming to be AS 0."
func (a *RemoteAS) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, []byte("null")) {
		*a = RemoteAS{}
		return nil
	}
	// The value is a JSON number, so it is decoded as one. The RFC 5396 asdot
	// spelling is text and never reaches here: 1.2 is a fraction, which a
	// uint32 refuses.
	var asn uint32
	if err := json.Unmarshal(data, &asn); err != nil {
		return fmt.Errorf("peer AS %s is not a 4-octet AS number or null: %w", data, err)
	}
	if asn == 0 {
		return errRemoteASZero
	}
	*a = RemoteAS{asn: asn, known: true}
	return nil
}

// errRemoteASZero refuses a document that names AS 0 as a peer's AS.
var errRemoteASZero = errors.New("peer AS 0 is refused: no peer can hold AS 0 (RFC 7607), write null while the AS is not yet known")
