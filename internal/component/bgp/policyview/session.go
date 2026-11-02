// Design: docs/architecture/core-design.md -- Policy Filter Chain, "The BGP route view"
// Related: register.go -- the session fact registrations
// Related: fields_json.go -- the writers the facts share with the route fields
//
// The session facts a filter reads beside the route (spec-filter-wire-6-stage-filters
// D-7): which peer the route came from or goes to, its AS numbers and its
// session classification, and on export the route's source peer. They are
// fields of the same view as the route fields, so an in-process member reads
// them through a Reader and a forked filter receives the ones it declared in
// the filter-update fields object, with no new member of the message header.

package policyview

import (
	"net/netip"
	"strconv"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/format"
	"github.com/ze-software/ze/internal/component/policy"
)

// ImportSession answers the session of a route received from peer: the
// sender's facts, its AS as the known sender, and its classification. The
// reactor builds peer on the received path, so every fact of it is filled
// there, llgr-advertised included (filterapi.PeerFilterInfo.LLGRAdvertised).
func ImportSession(peer filterapi.PeerFilterInfo) Session {
	return Session{
		EBGP:        !peer.IsIBGP,
		Sender:      peer.PeerAS,
		SenderKnown: true,
		Peer:        peer,
		PeerKnown:   true,
		receiving:   true,
	}
}

// ExportSession answers the session of a route sent to dest. source holds the
// facts of the peer the route came from, valid only when sourceKnown: a route
// Ze originated, or one whose rail does not carry its source, has none.
func ExportSession(dest, source filterapi.PeerFilterInfo, sourceKnown bool) Session {
	return Session{
		EBGP:        !dest.IsIBGP,
		Peer:        dest,
		PeerKnown:   true,
		Source:      source,
		SourceKnown: sourceKnown,
	}
}

// sessionFact is one session fact field: how the view answers it from the
// Session, and how the filter-update message writes it.
type sessionFact struct {
	field  policy.Field
	answer func(s *Session) Value
	write  writeFunc
}

// readSessionFact adapts a fact's answer to the reader signature. A fact
// decodes nothing, so it never errors.
func readSessionFact(answer func(s *Session) Value) readFunc {
	return func(v *View) (Value, error) { return answer(&v.session), nil }
}

// peerFacts answers the facts of the session's peer, and false when the
// session holds none (a dry run with no peer, a test view).
func (s *Session) peerFacts() (*filterapi.PeerFilterInfo, bool) {
	return &s.Peer, s.PeerKnown
}

// sourceFacts answers the facts of the route's source on export, and false
// when the session holds none.
func (s *Session) sourceFacts() (*filterapi.PeerFilterInfo, bool) {
	return &s.Source, s.SourceKnown
}

// The fact answers. Each answers absent, never a zero, when the session does
// not hold the fact: an unknown peer, a standalone peer's group, the
// route-server-as of a session that is not a route-server client.

func textFact(facts func(s *Session) (*filterapi.PeerFilterInfo, bool), text func(p *filterapi.PeerFilterInfo) string) func(s *Session) Value {
	return func(s *Session) Value {
		p, ok := facts(s)
		if !ok {
			return Value{}
		}
		name := text(p)
		return Value{present: name != "", text: name}
	}
}

func addressFact(facts func(s *Session) (*filterapi.PeerFilterInfo, bool)) func(s *Session) Value {
	return func(s *Session) Value {
		p, ok := facts(s)
		if !ok {
			return Value{}
		}
		return Value{present: p.Address.IsValid(), addr: p.Address}
	}
}

// asFact answers an AS number fact. Zero is not an AS a session runs with
// (RFC 7607 Section 2: "AS 0 MUST NOT be configured"), so a zero answers
// absent: the reactor fills no AS for that session.
func asFact(facts func(s *Session) (*filterapi.PeerFilterInfo, bool), asn func(p *filterapi.PeerFilterInfo) uint32) func(s *Session) Value {
	return func(s *Session) Value {
		p, ok := facts(s)
		if !ok {
			return Value{}
		}
		number := asn(p)
		return Value{present: number != 0, number: uint64(number), numbered: number != 0}
	}
}

func flagFact(facts func(s *Session) (*filterapi.PeerFilterInfo, bool), flag func(p *filterapi.PeerFilterInfo) bool) func(s *Session) Value {
	return func(s *Session) Value {
		p, ok := facts(s)
		if !ok {
			return Value{}
		}
		return Value{present: true, flag: flag(p)}
	}
}

// answerRouteServerAS answers route-server-as, present only on a session
// that is a route-server client: PeerFilterInfo.RouteServerAS is zero
// otherwise, and that zero means "not a client", never an AS.
func answerRouteServerAS(s *Session) Value {
	p, ok := s.peerFacts()
	if !ok {
		return Value{}
	}
	if !p.RSClient {
		return Value{}
	}
	return asFact((*Session).peerFacts, func(p *filterapi.PeerFilterInfo) uint32 { return p.RouteServerAS })(s)
}

// answerLLGRAdvertised answers llgr-advertised, present only on a received
// route: the reactor fills PeerFilterInfo.LLGRAdvertised on the received path
// from the session's own OPEN, and on no export rail, where false would be a
// zero standing in for an unknown.
func answerLLGRAdvertised(s *Session) Value {
	if !s.receiving {
		return Value{}
	}
	return flagFact((*Session).peerFacts, func(p *filterapi.PeerFilterInfo) bool { return p.LLGRAdvertised })(s)
}

func writeText(buf []byte, _ *View, value Value) ([]byte, bool, error) {
	return format.AppendJSONQuoted(buf, value.text), true, nil
}

func writeAddress(buf []byte, _ *View, value Value) ([]byte, bool, error) {
	buf = append(buf, '"')
	buf = value.addr.AppendTo(buf)
	return append(buf, '"'), true, nil
}

func writeNumber(buf []byte, _ *View, value Value) ([]byte, bool, error) {
	return strconv.AppendUint(buf, value.number, 10), true, nil
}

func writeFlag(buf []byte, _ *View, value Value) ([]byte, bool, error) {
	return strconv.AppendBool(buf, value.flag), true, nil
}

// Text answers a text fact: peer-name, peer-group and their source forms.
func (x Value) Text() string { return x.text }

// Address answers an address fact: peer-address and source-peer-address.
func (x Value) Address() netip.Addr { return x.addr }

// Flag answers a boolean fact: ibgp, rs-client, llgr-advertised and their
// source forms. A present flag is an answer; read Present first.
func (x Value) Flag() bool { return x.flag }
