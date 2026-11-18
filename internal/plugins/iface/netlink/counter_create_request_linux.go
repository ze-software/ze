// Design: docs/features/interfaces.md -- Creation-bound raw counter readers
// Related: counter_create_linux.go -- Native creation and reader lifetime
// Upstream: github.com/vishvananda/netlink v1.3.1, nl.NetlinkRequestData and serialization:
// https://github.com/vishvananda/netlink/blob/v1.3.1/nl/nl_linux.go
// Upstream: Linux v7.3, request flags and link attributes:
// https://github.com/torvalds/linux/blob/v7.3/include/uapi/linux/netlink.h
// https://github.com/torvalds/linux/blob/v7.3/include/uapi/linux/if_link.h

//go:build linux

package ifacenetlink

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/vishvananda/netlink/nl"
	"golang.org/x/sys/unix"
)

// newCounterCreateRequest MUST finish all caller callbacks and copy their bytes
// before the source takes mu. The caller MUST keep input immutable until return.
func newCounterCreateRequest(input *nl.NetlinkRequest) (*nl.NetlinkRequest, error) {
	if input == nil {
		return nil, errors.New("counter creation request is absent")
	}
	if input.Type != unix.RTM_NEWLINK {
		return nil, errors.New("counter creation requires RTM_NEWLINK")
	}
	if input.Flags&unix.NLM_F_CREATE == 0 {
		return nil, errors.New("counter creation requires CREATE")
	}
	if input.Flags&unix.NLM_F_EXCL == 0 {
		return nil, errors.New("counter creation requires EXCL")
	}
	// EXCL and MATCH share a bit; rejecting DUMP's bits would reject EXCL.
	const allowed = unix.NLM_F_REQUEST | unix.NLM_F_ACK | unix.NLM_F_ECHO |
		unix.NLM_F_CREATE | unix.NLM_F_EXCL
	if input.Flags & ^uint16(allowed) != 0 {
		return nil, errors.New("counter creation request has unsupported flags")
	}
	budget := monitorReceiveBufferBytes - unix.NLMSG_HDRLEN
	if len(input.Data) > budget {
		return nil, errors.New("counter creation request exceeds payload count limit")
	}
	if len(input.RawData) > budget {
		return nil, errors.New("counter creation request exceeds byte limit")
	}
	size := len(input.RawData)
	// Both passes have an explicit payload-count and byte budget, including
	// caller payloads whose declared length is zero.
	for _, data := range input.Data {
		if data == nil {
			return nil, errors.New("counter creation payload is absent")
		}
		// The open payload interface can hold a typed nil. Refuse it before
		// calling vendor or caller methods at this cold admission boundary.
		value := reflect.ValueOf(data)
		// Only nil-capable kinds have an IsNil operation.
		//exhaustive:ignore
		switch value.Kind() {
		case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
			if value.IsNil() {
				return nil, errors.New("counter creation payload is absent")
			}
		}
		length := data.Len()
		if length < 0 {
			return nil, errors.New("counter creation payload has a negative length")
		}
		if length > budget-size {
			return nil, errors.New("counter creation request exceeds byte limit")
		}
		size += length
	}
	// This native control request crosses caller ownership. Send MUST receive
	// only the owned copy, because the vendor serializes again under mu.
	request := nl.NewNetlinkRequest(unix.RTM_NEWLINK, allowed)
	request.RawData = make([]byte, 0, size)
	for _, data := range input.Data {
		payload := data.Serialize()
		if len(payload) > budget-len(request.RawData) {
			return nil, errors.New("serialized counter creation exceeds byte limit")
		}
		request.RawData = append(request.RawData, payload...)
	}
	if len(input.RawData) > budget-len(request.RawData) {
		return nil, errors.New("serialized counter creation exceeds byte limit")
	}
	request.RawData = append(request.RawData, input.RawData...)
	if err := validateCounterCreatePayload(request.RawData); err != nil {
		return nil, err
	}
	return request, nil
}

func validateCounterCreatePayload(payload []byte) error {
	if len(payload) < unix.SizeofIfInfomsg {
		return errors.New("short counter creation link header")
	}
	if payload[0] != unix.AF_UNSPEC {
		return errors.New("counter creation requires AF_UNSPEC")
	}
	if int32(nl.NativeEndian().Uint32(payload[4:8])) < 0 {
		return errors.New("counter creation index is negative")
	}
	// Only the primary destination must be the verified source namespace.
	// Nested driver/peer data and backing-link namespace selectors stay opaque.
	for attributes := payload[unix.SizeofIfInfomsg:]; len(attributes) > 0; {
		if len(attributes) < unix.SizeofRtAttr {
			return errors.New("short counter creation attribute")
		}
		length := int(nl.NativeEndian().Uint16(attributes[:2]))
		if length < unix.SizeofRtAttr {
			return errors.New("short counter creation attribute length")
		}
		if length > len(attributes) {
			return errors.New("counter creation attribute exceeds payload")
		}
		aligned := (length + unix.RTA_ALIGNTO - 1) & ^(unix.RTA_ALIGNTO - 1)
		if aligned > len(attributes) {
			return errors.New("counter creation attribute alignment exceeds payload")
		}
		kind := nl.NativeEndian().Uint16(attributes[2:4]) &
			^uint16(unix.NLA_F_NESTED|unix.NLA_F_NET_BYTEORDER)
		if kind == unix.IFLA_NET_NS_PID {
			return fmt.Errorf("counter creation destination namespace attribute %d is unsupported", kind)
		}
		if kind == unix.IFLA_NET_NS_FD {
			return fmt.Errorf("counter creation destination namespace attribute %d is unsupported", kind)
		}
		if kind == unix.IFLA_TARGET_NETNSID {
			return fmt.Errorf("counter creation destination namespace attribute %d is unsupported", kind)
		}
		attributes = attributes[aligned:]
	}
	return nil
}
