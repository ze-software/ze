//go:build linux

// Design: docs/architecture/testing/interop.md -- peer-local namespace operations.
// Related: action.go -- faked portable contract; frame.go -- outgoing packet evidence.
package main

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"time"

	"github.com/vishvananda/netlink/nl"
	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"
)

// UAPI: https://github.com/torvalds/linux/blob/v7.3/include/uapi/linux/xfrm.h
// MIGRATE_STATE clones under the old state's lock and preserves live replay; there is no fallback.
const migrateStateMessage = 0x29

type linuxPeer struct {
	key    stateKey
	socket *nl.SocketHandle
}

// openPeer acquires a socket in the current container namespace. The caller MUST close it.
func openPeer(request command) (peerIO, error) {
	socket, err := nl.GetNetlinkSocketAt(netns.None(), netns.None(), unix.NETLINK_XFRM)
	if err != nil {
		return nil, fmt.Errorf("open peer XFRM socket: %w", err)
	}
	timeout := unix.NsecToTimeval(time.Second.Nanoseconds())
	if err := socket.SetSendTimeout(&timeout); err != nil {
		socket.Close()
		return nil, err
	}
	if err := socket.SetReceiveTimeout(&timeout); err != nil {
		socket.Close()
		return nil, err
	}
	return &linuxPeer{key: request.key, socket: &nl.SocketHandle{Socket: socket}}, nil
}

// close MUST be called after the last action and releases the vendor-owned socket.
func (p *linuxPeer) close() error {
	p.socket.Close()
	return nil
}

func (p *linuxPeer) execute(ctx context.Context, request *nl.NetlinkRequest, reply uint16, consume func([]byte) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	// Closing the file interrupts vendor Receive even when the namespace becomes
	// quiet. Join a started cancellation callback before returning to the caller.
	closed := make(chan struct{})
	stop := context.AfterFunc(ctx, func() { p.socket.Close(); close(closed) })
	defer func() {
		if !stop() {
			<-closed
		}
	}()
	request.Sockets = map[int]*nl.SocketHandle{unix.NETLINK_XFRM: p.socket}
	var operationErr error
	count := 0
	err := request.ExecuteIter(unix.NETLINK_XFRM, reply, func(message []byte) bool {
		// GETSA carries algorithms and keys. Nothing retains its raw message.
		defer clear(message)
		count++
		operationErr = ctx.Err()
		if operationErr == nil {
			if count > 64 {
				operationErr = errors.New("peer response exceeds the 64-state helper limit")
			}
		}
		if operationErr == nil {
			operationErr = consume(message)
		}
		if operationErr != nil {
			// ExecuteIter otherwise drains the dump after false. Close now so an
			// unbounded response cannot outlive the helper's admission bound.
			p.socket.Close()
			return false
		}
		return true
	})
	if operationErr != nil {
		return operationErr
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return err
}

func (p *linuxPeer) states(ctx context.Context) ([]state, error) {
	var selected []state
	err := p.execute(ctx, stateDumpRequest(), nl.XFRM_MSG_NEWSA, func(message []byte) error {
		if len(message) < nl.SizeofXfrmUsersaInfo {
			return errors.New("short peer state readback")
		}
		info := nl.DeserializeXfrmUsersaInfo(message)
		if info.Family != unix.AF_INET {
			return nil
		}
		if info.Id.Proto != unix.IPPROTO_ESP {
			return nil
		}
		key := stateKey{source: netip.AddrFrom4([4]byte(info.Saddr[:4])), target: netip.AddrFrom4([4]byte(info.Id.Daddr[:4])), spi: nl.Swap32(info.Id.Spi)}
		if key != p.key {
			return nil
		}
		projection, err := projectState(message)
		if err != nil {
			return err
		}
		selected = append(selected, projection)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("read peer states: %w", err)
	}
	return selected, nil
}

func stateDumpRequest() *nl.NetlinkRequest {
	// GETSA dump parses attributes at payload offset zero, unlike a single-SA
	// GETSA. Follow vendor XfrmStateList: an xfrm_usersa_id is not a dump header.
	return nl.NewNetlinkRequest(nl.XFRM_MSG_GETSA, unix.NLM_F_DUMP)
}

// projectState uses vendor decoders for the fixed state, attributes and legacy replay.
// It never formats, returns or reconstructs an algorithm or key. The ESN state flag
// is refused, but a larger non-ESN window may supply the extended replay header.
func projectState(message []byte) (state, error) {
	if len(message) < nl.SizeofXfrmUsersaInfo {
		return state{}, errors.New("short peer state readback")
	}
	info := nl.DeserializeXfrmUsersaInfo(message)
	if info.Flags&nl.XFRM_STATE_ESN != 0 {
		return state{}, errors.New("peer form helper does not support ESN; this scenario requires non-ESN outbound replay metadata")
	}
	if info.Family != unix.AF_INET {
		return state{}, errors.New("peer form helper requires an IPv4 SA")
	}
	if info.Id.Proto != unix.IPPROTO_ESP {
		return state{}, errors.New("peer form helper requires an ESP SA")
	}
	if info.Mode != 1 {
		return state{}, errors.New("peer form helper requires a tunnel-mode SA")
	}
	value := state{
		key:   stateKey{source: netip.AddrFrom4([4]byte(info.Saddr[:4])), target: netip.AddrFrom4([4]byte(info.Id.Daddr[:4])), spi: nl.Swap32(info.Id.Spi)},
		reqid: info.Reqid, encapsulation: encapsulation{form: formBare},
	}
	copy(value.selector[:], info.Sel.Serialize())
	attributes, err := nl.ParseRouteAttr(message[nl.SizeofXfrmUsersaInfo:])
	if err != nil {
		return state{}, err
	}
	replaySeen := false
	encapSeen := false
	directionSeen := false
	for _, attribute := range attributes {
		// Only metadata used by this helper is projected; the attribute namespace is open.
		switch attribute.Attr.Type & 0x3fff {
		case nl.XFRMA_REPLAY_VAL:
			if replaySeen {
				return state{}, errors.New("duplicate peer replay metadata")
			}
			if len(attribute.Value) != nl.SizeofXfrmReplayState {
				return state{}, errors.New("malformed peer replay metadata")
			}
			value.sequence = uint64(nl.DeserializeXfrmReplayState(attribute.Value).OSeq)
			replaySeen = true
		case nl.XFRMA_REPLAY_ESN_VAL:
			if replaySeen {
				return state{}, errors.New("duplicate peer replay metadata")
			}
			sequence, err := extendedWindowSequence(attribute.Value)
			if err != nil {
				return state{}, err
			}
			value.sequence, replaySeen = sequence, true
		case nl.XFRMA_ENCAP:
			if encapSeen {
				return state{}, errors.New("duplicate peer encapsulation metadata")
			}
			encapSeen = true
			if len(attribute.Value) != nl.SizeofXfrmEncapTmpl {
				return state{}, errors.New("malformed peer encapsulation metadata")
			}
			encap := nl.DeserializeXfrmEncapTmpl(attribute.Value)
			if encap.EncapType != unix.UDP_ENCAP_ESPINUDP {
				return state{}, errors.New("unsupported peer encapsulation type")
			}
			if encap.EncapSport == 0 {
				return state{}, errors.New("peer encapsulation source port is zero")
			}
			if encap.EncapDport == 0 {
				return state{}, errors.New("peer encapsulation target port is zero")
			}
			value.encapsulation = encapsulation{form: formUDP, sourcePort: nl.Swap16(encap.EncapSport), targetPort: nl.Swap16(encap.EncapDport)}
		case nl.XFRMA_MARK, nl.XFRMA_SET_MARK, nl.XFRMA_SET_MARK_MASK, nl.XFRMA_IF_ID, nl.XFRMA_OFFLOAD_DEV:
			return state{}, errors.New("peer form helper requires an unmarked, software, interface-independent SA")
		case nl.XFRMA_SA_DIR:
			if directionSeen {
				return state{}, errors.New("duplicate peer SA direction")
			}
			directionSeen = true
			if len(attribute.Value) != 1 {
				return state{}, errors.New("malformed peer SA direction")
			}
			if attribute.Value[0] != 2 {
				return state{}, errors.New("selected peer SA is not outbound")
			}
		default:
		}
	}
	if !replaySeen {
		return state{}, errors.New("peer state readback has no outbound replay metadata")
	}
	return value, nil
}

func (p *linuxPeer) migrate(ctx context.Context, expected state, encap encapsulation) error {
	states, err := p.states(ctx)
	if err != nil {
		return err
	}
	current, err := selectState(states, expected.key)
	if err != nil {
		return err
	}
	if current.reqid != expected.reqid {
		return errors.New("peer SA reqid changed before migration")
	}
	if current.selector != expected.selector {
		return errors.New("peer SA selector changed before migration")
	}
	if current.sequence < expected.sequence {
		return errors.New("peer replay sequence rolled back before migration")
	}
	request, err := migrationRequest(current, encap)
	if err != nil {
		return err
	}
	if err := p.execute(ctx, request, 0, func([]byte) error { return errors.New("atomic migration unexpectedly returned state data") }); err != nil {
		return fmt.Errorf("atomic peer encapsulation migration: %w", err)
	}
	states, err = p.states(ctx)
	if err != nil {
		return err
	}
	after, err := selectState(states, expected.key)
	if err != nil {
		return err
	}
	if after.encapsulation != encap {
		return errors.New("peer readback did not retain the requested encapsulation")
	}
	return nil
}

func migrationRequest(selected state, encap encapsulation) (*nl.NetlinkRequest, error) {
	payload, err := migrationPayload(selected, encap)
	if err != nil {
		return nil, err
	}
	request := nl.NewNetlinkRequest(migrateStateMessage, unix.NLM_F_ACK)
	request.AddRawData(payload[:132])
	request.AddRawData(nl.NewRtAttr(nl.XFRMA_ENCAP, payload[136:]).Serialize())
	return request, nil
}
