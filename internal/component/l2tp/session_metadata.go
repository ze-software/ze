// Design: docs/architecture/l2tp/bng-1-radius-attributes.md -- RADIUS attribute metadata store
// Related: handler.go -- AuthMetadata struct, AuthHandler types

package l2tp

import (
	"net"
	"net/netip"
	"sync"

	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
	"github.com/ze-software/ze/internal/component/l2tp/subscriber"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// FramedRoute is a static route learned from RADIUS Framed-Route (attr 22)
// or Framed-IPv6-Route (attr 99). The prefix is the destination network
// and metric is the route priority.
// RFC 2865 Section 5.22, RFC 3162 Section 2.5.
type FramedRoute = l2tpevents.FramedRoute

// RFC 6911 Section 3: Framed-IPv6-Prefix (attr 97).
// RFC 4818 Section 3: Delegated-IPv6-Prefix (attr 123).
// RFC 6911 Section 3.1: Framed-IPv6-Pool (attr 100).

// AuthMetadata carries RADIUS Access-Accept attributes extracted by
// the auth handler. Stored per-session via StoreSessionMetadata so
// downstream consumers (pool handler, reactor, shaper) can read
// subscriber profile without changing the auth respond signature.
//
// RFC 2865 Section 5.8 (Framed-IP-Address), 5.9 (Framed-IP-Netmask),
// 5.11 (Filter-Id), 5.12 (Framed-MTU), 5.27 (Session-Timeout),
// 5.28 (Idle-Timeout).
// RFC 2869 Section 5.16 (Acct-Interim-Interval).
// Framed-Pool: RFC 2865 attribute type 88.
type AuthMetadata struct {
	FramedIP            netip.Addr
	FramedNetmask       net.IPMask
	FramedPool          string
	FramedIPv6Prefix    netip.Prefix // RFC 6911 attr 97
	DelegatedIPv6Prefix netip.Prefix // RFC 4818 attr 123
	FramedIPv6Pool      string       // RFC 6911 attr 100
	SessionTimeout      uint32       // seconds, 0 = not set
	IdleTimeout         uint32       // seconds, 0 = not set
	FramedMTU           uint16       // RFC 2865 attr 12, 68..65535; lowers the pppN IP MTU; 0 = none usable
	FilterID            string
	CoSProfile          string        // "cos:<name>" extracted from Filter-Id; empty if none
	AcctInterimInterval uint32        // seconds, 0 = not set
	FramedRoutes        []FramedRoute // RFC 2865 attr 22 + RFC 3162 attr 99
	// DNSPrimary and DNSSecondary are the subscriber's resolvers from
	// MS-Primary-DNS-Server and MS-Secondary-DNS-Server (RFC 2548 Sections
	// 2.7.6 and 2.7.7, vendor 311 types 28 and 29). The zero Addr means the
	// Access-Accept carried no such attribute, or one that was malformed.
	DNSPrimary   netip.Addr
	DNSSecondary netip.Addr
}

var sessionMeta sync.Map

// StoreSessionMetadata stores RADIUS-extracted metadata for the given
// session. Called by the auth handler after a successful Access-Accept,
// before responding to the PPP session goroutine. Multiple consumers
// (pool, reactor, shaper, acct) read via LoadSessionMetadata;
// ClearSessionMetadata removes the entry on session teardown.
func StoreSessionMetadata(subscriberID string, meta *AuthMetadata) {
	if meta == nil {
		return
	}
	if err := subscriber.RequireID(slogutil.Logger(SubsystemName), subscriberID); err != nil {
		return
	}
	sessionMeta.Store(subscriberID, meta)
}

// LoadSessionMetadata retrieves the RADIUS metadata for the given
// session. Returns nil if no metadata was stored. The entry remains
// in the store so multiple consumers (pool, reactor, shaper, acct)
// can each read it. Call ClearSessionMetadata on session teardown.
func LoadSessionMetadata(subscriberID string) *AuthMetadata {
	// refuse-by-default: guards l2tpReactor.startSessionTimeouts -- nil denies a profile for an invalid ID
	if err := subscriber.RequireID(slogutil.Logger(SubsystemName), subscriberID); err != nil {
		return nil
	}
	v, ok := sessionMeta.Load(subscriberID)
	if !ok {
		return nil
	}
	meta, _ := v.(*AuthMetadata)
	return meta
}

// ClearSessionMetadata removes any stored metadata for the session
// without returning it. Called on session teardown to prevent leaks
// when metadata was stored but never consumed.
func ClearSessionMetadata(subscriberID string) {
	if err := subscriber.RequireID(slogutil.Logger(SubsystemName), subscriberID); err != nil {
		return
	}
	sessionMeta.Delete(subscriberID)
}
