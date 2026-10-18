// Design: docs/features/interfaces.md -- DHCPv4 client lifecycle
// Overview: dhcp_v4_linux.go -- DHCPv4 lease state machine
// RFC: rfc/short/rfc2131.md

//go:build linux

package ifacedhcp

import (
	"context"
	"errors"
	"net"
	"time"

	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/dhcpv4/nclient4"

	"github.com/ze-software/ze/internal/core/clock"
)

// v4Outcome is what one DHCPv4 exchange produced.
type v4Outcome uint8

const (
	v4OutcomeUnspecified v4Outcome = iota
	// v4OutcomeAck: a DHCPACK the attempt's matcher accepted.
	v4OutcomeAck
	// v4OutcomeNak: a DHCPNAK the attempt's matcher accepted.
	v4OutcomeNak
	// v4OutcomeNoAnswer: the message went out and nothing acceptable came back
	// within the listen window.
	v4OutcomeNoAnswer
	// v4OutcomeFailed: the message could not be sent (device, socket, builder,
	// or the exchange was canceled by Stop).
	v4OutcomeFailed
)

// v4Delivery is how an extending DHCPREQUEST leaves the host.
type v4Delivery uint8

const (
	v4DeliveryUnspecified v4Delivery = iota
	// v4DeliveryUnicast: RENEWING, from the leased address to the server.
	v4DeliveryUnicast
	// v4DeliveryBroadcast: REBINDING, or RENEWING when no server identifier is
	// known, to 255.255.255.255 over the raw socket.
	v4DeliveryBroadcast
)

// v4Reply is the result of one exchange.
type v4Reply struct {
	outcome v4Outcome
	// msg is the DHCPACK or DHCPNAK. It is nil for no answer and for failure.
	msg *dhcpv4.DHCPv4
	// offerServer is the server identifier of the DHCPOFFER the acquisition
	// answered. It names the leasing server when the ACK omits option 54.
	offerServer net.IP
	// sent is the instant the DHCPREQUEST that msg answers was sent, read on
	// the client's clock. Lease deadlines are measured from it.
	sent time.Time
	err  error
}

// v4Attempt is one extending DHCPREQUEST, fully decided by the lease loop:
// the message, where it goes, which answers count, and how long to listen.
type v4Attempt struct {
	device   string // kernel device, resolved for this attempt
	request  *dhcpv4.DHCPv4
	delivery v4Delivery
	source   net.IP // the leased address, the unicast source
	server   net.IP // the unicast destination
	accept   nclient4.Matcher
	window   time.Duration
}

// v4Exchanger sends DHCPv4 messages and reads the answers. The lease loop owns
// every timing decision; an exchanger performs one acquisition (DORA) or one
// transmission of one extending DHCPREQUEST per call.
type v4Exchanger interface {
	// acquire runs DISCOVER, OFFER, REQUEST and reads the ACK or NAK on device.
	acquire(ctx context.Context, device string, mods []dhcpv4.Modifier) v4Reply
	// extend sends attempt.request once and listens for attempt.window.
	extend(ctx context.Context, attempt v4Attempt) v4Reply
}

// nclient4Exchanger is the production exchanger over the vendored nclient4.
// It opens a fresh client per call on the kernel device it is given, so a
// logical name never reaches a socket.
type nclient4Exchanger struct {
	clock clock.Clock
}

var _ v4Exchanger = nclient4Exchanger{}

func (x nclient4Exchanger) acquire(ctx context.Context, device string, mods []dhcpv4.Modifier) v4Reply {
	client, err := nclient4.New(device)
	if err != nil {
		return v4Reply{outcome: v4OutcomeFailed, err: err}
	}
	defer closeV4Client(client, device)

	offer, err := client.DiscoverOffer(ctx, mods...)
	if err != nil {
		return v4ReplyFromError(err, time.Time{})
	}
	sent := x.clock.Now()
	lease, err := client.RequestFromOffer(ctx, offer, mods...)
	if nak, ok := errors.AsType[*nclient4.ErrNak](err); ok {
		return v4Reply{outcome: v4OutcomeNak, msg: nak.Nak, offerServer: offer.ServerIdentifier(), sent: sent}
	}
	if err != nil {
		return v4ReplyFromError(err, sent)
	}
	return v4Reply{outcome: v4OutcomeAck, msg: lease.ACK, offerServer: offer.ServerIdentifier(), sent: sent}
}

func (x nclient4Exchanger) extend(ctx context.Context, attempt v4Attempt) v4Reply {
	// One transmission per call: the lease loop schedules retransmission
	// (RFC 2131 Section 4.4.5), so the library's own retry is disabled.
	opts := []nclient4.ClientOpt{nclient4.WithRetry(1), nclient4.WithTimeout(attempt.window)}
	destination := nclient4.DefaultServers
	switch attempt.delivery {
	case v4DeliveryUnicast:
		// RFC 2131 Section 4.1
		destination = &net.UDPAddr{IP: attempt.server, Port: nclient4.ServerPort}
		opts = append(opts, nclient4.WithUnicast(&net.UDPAddr{IP: attempt.source, Port: nclient4.ClientPort}))
	case v4DeliveryBroadcast:
		// RFC 2131 Section 4.3.2
	case v4DeliveryUnspecified:
		panic("BUG: iface dhcp v4: extend called with no delivery")
	}
	opts = append(opts, nclient4.WithServerAddr(destination))

	client, err := nclient4.New(attempt.device, opts...)
	if err != nil {
		return v4Reply{outcome: v4OutcomeFailed, sent: x.clock.Now(), err: err}
	}
	defer closeV4Client(client, attempt.device)

	sent := x.clock.Now()
	answer, err := client.SendAndRead(ctx, destination, attempt.request, attempt.accept)
	if err != nil {
		return v4ReplyFromError(err, sent)
	}
	if answer.MessageType() == dhcpv4.MessageTypeNak {
		return v4Reply{outcome: v4OutcomeNak, msg: answer, sent: sent}
	}
	return v4Reply{outcome: v4OutcomeAck, msg: answer, sent: sent}
}

// v4ReplyFromError classifies an nclient4 error: the library's "no response"
// is no answer, anything else (socket, cancel) a failed exchange.
func v4ReplyFromError(err error, sent time.Time) v4Reply {
	if errors.Is(err, nclient4.ErrNoResponse) {
		return v4Reply{outcome: v4OutcomeNoAnswer, sent: sent, err: err}
	}
	return v4Reply{outcome: v4OutcomeFailed, sent: sent, err: err}
}

func closeV4Client(client *nclient4.Client, device string) {
	if err := client.Close(); err != nil {
		loggerPtr.Load().Debug("iface dhcp v4: client close failed", "device", device, "err", err)
	}
}

// v4ExtendAccepts is the matcher for the answers to an extending DHCPREQUEST.
// RENEWING accepts only the leasing server; REBINDING accepts any server.
func v4ExtendAccepts(rebinding bool, server net.IP) nclient4.Matcher {
	answers := nclient4.IsMessageType(dhcpv4.MessageTypeAck, dhcpv4.MessageTypeNak)
	if rebinding {
		// RFC 2131 Section 4.4.5
		return answers
	}
	// RFC 2131 Section 4.4.5
	return nclient4.IsAll(nclient4.IsCorrectServer(server), answers)
}
