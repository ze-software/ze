// Design: docs/architecture/testing/interop.md -- isolate the peer stimulus from its observation.
// Related: action_test.go -- no-Docker contract; socket_linux.go -- the only syscall adapter.
package main

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"strconv"
	"strings"
)

type action uint8

const (
	actionUnspecified action = iota
	actionUDP
	actionBare
	actionProbe
)

const (
	actionNameUDP  = "udp"
	actionNameBare = "bare"
)

type espForm uint8

const (
	formUnspecified espForm = iota
	formBare
	formUDP
)

// command is produced only by parseCommand. Its zero value is invalid; it names no action.
// All identity fields are private, so CLI validation cannot be bypassed by a consumer.
type command struct {
	action        action
	key           stateKey
	interfaceName string
	sourcePort    uint16
	targetPort    uint16
	echoID        uint16
	echoSequence  uint16
}

type stateKey struct {
	source netip.Addr
	target netip.Addr
	spi    uint32
}

// state is a sanitized projection of the vendor readback, never a reconstruction of an SA.
// selector is the vendor's serialized selector, copied unchanged into the atomic request.
// The zero value identifies no state. No algorithm or key field crosses this boundary.
type state struct {
	key           stateKey
	reqid         uint32
	sequence      uint64
	selector      [56]byte
	encapsulation encapsulation
}

type encapsulation struct {
	form       espForm
	sourcePort uint16
	targetPort uint16
}

// peerIO has no state delete/add method: a form change has only the atomic migration route.
// Implementations MUST finish each operation before returning and retain no caller buffers.
// A successful openPeer caller MUST call close after its last action; close MUST release all sockets.
type peerIO interface {
	close() error
	states(context.Context) ([]state, error)
	migrate(context.Context, state, encapsulation) error
	probe(context.Context, command) (wireObservation, error)
}

// result is the helper's entire public output. It intentionally cannot hold encryption keys.
type result struct {
	Action   string `json:"action"`
	Source   string `json:"source"`
	Target   string `json:"target"`
	SPI      uint32 `json:"spi"`
	ReqID    uint32 `json:"reqid"`
	Form     string `json:"form"`
	Sequence uint64 `json:"sequence"`
}

func parseCommand(argv []string) (command, error) {
	var request command
	if len(argv) == 0 {
		return request, errors.New("usage: ipsec-peerform udp <source> <target> <spi> <sport> <dport>; bare <source> <target> <spi>; probe <interface> <source> <target> <spi> <echo-id> <echo-sequence>")
	}
	identity := argv[1:]
	switch argv[0] {
	case actionNameUDP:
		if len(argv) != 6 {
			return request, errors.New("udp needs source, target, SPI and both UDP ports")
		}
		request.action = actionUDP
		var err error
		request.sourcePort, err = positiveUint16(argv[4])
		if err != nil {
			return command{}, fmt.Errorf("source port: %w", err)
		}
		request.targetPort, err = positiveUint16(argv[5])
		if err != nil {
			return command{}, fmt.Errorf("target port: %w", err)
		}
	case actionNameBare:
		if len(argv) != 4 {
			return request, errors.New("bare needs source, target and SPI")
		}
		request.action = actionBare
	case "probe":
		if len(argv) != 7 {
			return request, errors.New("probe needs interface, source, target, SPI, echo ID and echo sequence")
		}
		request.action = actionProbe
		request.interfaceName = argv[1]
		if request.interfaceName == "" {
			return command{}, errors.New("probe interface is empty")
		}
		if len(request.interfaceName) > 15 {
			return command{}, errors.New("probe interface exceeds Linux IFNAMSIZ")
		}
		if strings.ContainsAny(request.interfaceName, "/\x00\r\n\t ") {
			return command{}, errors.New("probe interface contains an invalid character")
		}
		identity = argv[2:]
		var err error
		request.echoID, err = positiveUint16(argv[5])
		if err != nil {
			return command{}, fmt.Errorf("echo ID: %w", err)
		}
		request.echoSequence, err = positiveUint16(argv[6])
		if err != nil {
			return command{}, fmt.Errorf("echo sequence: %w", err)
		}
	default:
		return request, fmt.Errorf("unknown peer form action %q", argv[0])
	}
	var err error
	request.key.source, err = peerIPv4(identity[0])
	if err != nil {
		return command{}, fmt.Errorf("source: %w", err)
	}
	request.key.target, err = peerIPv4(identity[1])
	if err != nil {
		return command{}, fmt.Errorf("target: %w", err)
	}
	if request.key.source == request.key.target {
		return command{}, errors.New("source and target must be distinct peer addresses")
	}
	spi, err := strconv.ParseUint(identity[2], 0, 32)
	if err != nil {
		return command{}, fmt.Errorf("SPI: %w", err)
	}
	if spi == 0 {
		return command{}, errors.New("SPI must be nonzero")
	}
	request.key.spi = uint32(spi)
	return request, nil
}

func peerIPv4(text string) (netip.Addr, error) {
	address, err := netip.ParseAddr(text)
	if err != nil {
		return netip.Addr{}, err
	}
	if !address.Is4() {
		return netip.Addr{}, errors.New("only IPv4 peer addresses are supported")
	}
	if address.IsUnspecified() {
		return netip.Addr{}, errors.New("unspecified peer address")
	}
	if address.IsMulticast() {
		return netip.Addr{}, errors.New("multicast peer address")
	}
	return address, nil
}

func positiveUint16(text string) (uint16, error) {
	value, err := strconv.ParseUint(text, 10, 16)
	if err != nil {
		return 0, err
	}
	if value == 0 {
		return 0, errors.New("value must be nonzero")
	}
	return uint16(value), nil
}

func runAction(ctx context.Context, request command, peer peerIO) (result, error) {
	states, err := peer.states(ctx)
	if err != nil {
		return result{}, err
	}
	before, err := selectState(states, request.key)
	if err != nil {
		return result{}, err
	}
	answer := result{Source: request.key.source.String(), Target: request.key.target.String(), SPI: request.key.spi, ReqID: before.reqid}
	switch request.action {
	case actionProbe:
		observed, err := peer.probe(ctx, request)
		if err != nil {
			return result{}, err
		}
		if observed.key != request.key {
			return result{}, errors.New("probe observed another SA")
		}
		if !observed.replied {
			return result{}, errors.New("probe has no matching ICMP echo reply")
		}
		if observed.sequence <= before.sequence {
			return result{}, errors.New("wire ESP sequence did not advance beyond the pre-probe state")
		}
		answer.Action, answer.Sequence = "probe", observed.sequence
		answer.Form, err = formName(observed.form)
		if err != nil {
			return result{}, err
		}
		return answer, nil
	case actionUDP, actionBare:
		encap := encapsulation{form: formBare}
		answer.Action = actionNameBare
		if request.action == actionUDP {
			encap = encapsulation{form: formUDP, sourcePort: request.sourcePort, targetPort: request.targetPort}
			answer.Action = actionNameUDP
		}
		if err := peer.migrate(ctx, before, encap); err != nil {
			return result{}, err
		}
		states, err := peer.states(ctx)
		if err != nil {
			return result{}, err
		}
		after, err := selectState(states, request.key)
		if err != nil {
			return result{}, err
		}
		if after.reqid != before.reqid {
			return result{}, errors.New("atomic migration changed the reqid")
		}
		if after.selector != before.selector {
			return result{}, errors.New("atomic migration changed the selector")
		}
		if after.sequence < before.sequence {
			return result{}, errors.New("atomic migration rolled back the outbound replay sequence")
		}
		answer.Form, answer.Sequence = answer.Action, after.sequence
		return answer, nil
	case actionUnspecified:
		return result{}, errors.New("peer form action is unspecified")
	default:
		return result{}, errors.New("peer form action is unknown")
	}
}

func selectState(states []state, key stateKey) (state, error) {
	var selected state
	found := false
	for _, candidate := range states {
		if candidate.key != key {
			continue
		}
		if found {
			return state{}, errors.New("several outbound states match the exact SPI and endpoints")
		}
		selected, found = candidate, true
	}
	if !found {
		return state{}, errors.New("no outbound state matches the exact SPI and endpoints")
	}
	return selected, nil
}

func formName(form espForm) (string, error) {
	switch form {
	case formBare:
		return actionNameBare, nil
	case formUDP:
		return actionNameUDP, nil
	case formUnspecified:
		return "", errors.New("wire form is unspecified")
	default:
		return "", errors.New("wire form is unknown")
	}
}
