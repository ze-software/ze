// Design: docs/functional-tests.md -- portable reload proof through the real subscriber pool handlers
// Related: fakel2tp.go -- this test-only internal plugin's command registrations

package fakel2tp

import (
	"strconv"

	"github.com/ze-software/ze/internal/component/l2tp"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/component/l2tp/subscriber"
	subevents "github.com/ze-software/ze/internal/component/l2tp/subscriber/events"
	"github.com/ze-software/ze/internal/core/opererr"
)

// poolTunnelID is private to these synthetic subscribers. A functional daemon
// using these commands MUST run fakel2tp internally and MUST open no real PPP
// sessions with this tunnel identifier. The pool remains the production one.
const poolTunnelID uint16 = 65000

// poolRequest reads the session's typed selector and optional named pool.
func poolRequest(args []string) (uint16, string, error) {
	if len(args) != 4 {
		return 0, "", poolRequestError()
	}
	if args[0] != "id" {
		return 0, "", poolRequestError()
	}
	if args[2] != "pool" {
		return 0, "", poolRequestError()
	}
	id, err := strconv.ParseUint(args[1], 10, 16)
	if err != nil {
		return 0, "", poolRequestError()
	}
	if id == 0 {
		return 0, "", poolRequestError()
	}
	name := args[3]
	if name == "default" {
		name = ""
	}
	return uint16(id), name, nil
}

func poolRequestError() error {
	return opererr.New("the synthetic subscriber needs a session id and pool name",
		"No address or prefix was requested.",
		"ze request fakel2tp pool address id 1 pool default")
}

// poolAddress invokes the same registered handler both PPP drains invoke.
// This is test-only in-process instrumentation, never a production command or
// a replacement for the Linux IPCP wire and kernel scenarios.
func poolAddress(args []string) (any, error) {
	return poolAddressForAccess(args, subscriber.AccessL2TP)
}

// poolPPPoEAddress gives the same numeric pair its explicit PPPoE identity.
func poolPPPoEAddress(args []string) (any, error) {
	return poolAddressForAccess(args, subscriber.AccessPPPoE)
}

// poolAddressForAccess MUST receive the transport selected by its command;
// no absent or default namespace is accepted from the command arguments.
func poolAddressForAccess(args []string, access subscriber.AccessType) (any, error) {
	id, name, err := poolRequest(args)
	if err != nil {
		return nil, err
	}
	handler := l2tp.GetPoolHandler()
	if handler == nil {
		return nil, opererr.New("the synthetic subscriber has no pool handler",
			"The l2tp-pool plugin did not register.", "ze show l2tp pool")
	}
	subscriberID := subscriber.SessionID(access, uint64(poolTunnelID), id)
	l2tp.StoreSessionMetadata(subscriberID, &l2tp.AuthMetadata{FramedPool: name})
	defer l2tp.ClearSessionMetadata(subscriberID)
	answer := handler(ppp.EventIPRequest{SubscriberID: subscriberID, TunnelID: poolTunnelID, SessionID: id, Family: ppp.AddressFamilyIPv4})
	if !answer.Accept {
		return nil, opererr.New("the synthetic subscriber received no address",
			"The pool refused its request: "+answer.Reason, "ze show l2tp pool")
	}
	return map[string]any{"address": answer.Peer.String(), "gateway": answer.Local.String()}, nil
}

// poolRelease sends the same subscriber topic the real transport teardown
// publishes. The observer MUST confirm pool state after this command answers:
// the bus recipient count does not count synchronous subscribers.
func poolRelease(args []string) (any, error) {
	if len(args) != 4 {
		return nil, poolReleaseError()
	}
	if args[0] != "access" {
		return nil, poolReleaseError()
	}
	if args[2] != "id" {
		return nil, poolReleaseError()
	}
	var access subscriber.AccessType
	// Command tokens are external input; refuse unknown transport names.
	switch args[1] {
	case transportL2TP:
		access = subscriber.AccessL2TP
	case "pppoe":
		access = subscriber.AccessPPPoE
	default:
		return nil, poolReleaseError()
	}
	id, err := strconv.ParseUint(args[3], 10, 16)
	if err != nil {
		return nil, poolReleaseError()
	}
	if id == 0 {
		return nil, poolReleaseError()
	}
	bus := getEventBus()
	if bus == nil {
		return nil, opererr.Wrap(errFakel2tpNoEventBus, "the synthetic subscriber has no event bus",
			"No subscriber teardown was delivered.", "ze show fakel2tp help")
	}
	session := subscriber.Session{
		ID:         subscriber.SessionID(access, uint64(poolTunnelID), uint16(id)),
		AccessType: access,
	}
	switch access {
	case subscriber.AccessL2TP:
		session.TunnelID = poolTunnelID
		session.SessionID = uint16(id)
	case subscriber.AccessPPPoE:
		session.AccessIfIndex = int(poolTunnelID)
		session.PPPoESID = uint16(id)
	}
	if _, err := subevents.SessionDown.Emit(bus, &subevents.SessionDownPayload{
		Session: session, Reason: "synthetic pool identity proof",
	}); err != nil {
		return nil, opererr.Wrap(err, "the synthetic subscriber teardown failed",
			"The pool may still hold its reservation.", "ze show l2tp pool")
	}
	return map[string]any{"id": session.ID}, nil
}

func poolReleaseError() error {
	return opererr.New("the synthetic teardown needs an explicit access type and session id",
		"No subscriber teardown was delivered.",
		"ze request fakel2tp pool release access pppoe id 3")
}

// poolPrefix reaches the production prefix-delegation handler after reload.
func poolPrefix(args []string) (any, error) {
	id, name, err := poolRequest(args)
	if err != nil {
		return nil, err
	}
	handler := l2tp.GetPrefixHandler()
	if handler == nil {
		return nil, opererr.New("the synthetic subscriber has no prefix handler",
			"The l2tp-pool plugin did not register.", "ze show l2tp pool")
	}
	answer := handler(l2tp.PrefixRequest{TunnelID: poolTunnelID, SessionID: id, PoolName: name})
	if !answer.OK {
		return nil, opererr.New("the synthetic subscriber received no delegated prefix",
			"The pool refused its request: "+answer.Reason, "ze show l2tp pool")
	}
	return map[string]any{"prefix": answer.Prefix.String()}, nil
}
