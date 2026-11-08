// Design: docs/features/interfaces.md -- Netlink interface monitor
// Related: monitor_linux.go -- the monitor that owns this subscription

//go:build linux

package ifacenetlink

import (
	"encoding/binary"
	"errors"
	"net/netip"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netlink/nl"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/iface"
	ifaceevents "github.com/ze-software/ze/internal/core/iface/events"
)

// The kernel hands the Router Advertisement options it does not consume
// itself (RDNSS, DNSSL, Prefix Information, PREF64) to userspace as
// RTM_NEWNDUSEROPT on RTNLGRP_ND_USEROPT: net/ipv6/ndisc.c
// ndisc_router_discovery calls ndisc_ra_useropt for each option
// ndisc_is_useropt matches, and only after ipv6_accept_ra(in6_dev) let it
// process the RA. The accept-ra leaf (net.ipv6.conf.<dev>.accept_ra) keeps
// deciding acceptance.

// ndUserOptHeaderLen is sizeof(struct nduseroptmsg), include/uapi/linux/rtnetlink.h.
const ndUserOptHeaderLen = 16

// ndUserOptSourceAddr is NDUSEROPT_SRCADDR, the attribute carrying the RA's
// IPv6 source address.
const ndUserOptSourceAddr = 1

// icmpv6RouterAdvertisement is the ICMPv6 type of a Router Advertisement.
const icmpv6RouterAdvertisement = 134

var (
	errNDUserOptShort  = errors.New("nd user option message shorter than its header")
	errNDUserOptFamily = errors.New("nd user option message is not AF_INET6")
	errNDUserOptLength = errors.New("nd user option message shorter than its options length")
)

// ndUserOpt is one decoded RTM_NEWNDUSEROPT message.
type ndUserOpt struct {
	ifindex  int
	icmpType uint8
	// options are the ND options as received, from the first Type octet.
	options []byte
	// router is the RA's source address; invalid when the kernel sent none.
	router netip.Addr
}

// decodeNDUserOpt decodes the body of one RTM_NEWNDUSEROPT message. The
// header is native-endian, as the kernel writes it.
//
//	struct nduseroptmsg {
//	        unsigned char  nduseropt_family;     off 0
//	        unsigned char  nduseropt_pad1;       off 1
//	        unsigned short nduseropt_opts_len;   off 2
//	        int            nduseropt_ifindex;    off 4
//	        __u8           nduseropt_icmp_type;  off 8
//	        __u8           nduseropt_icmp_code;  off 9
//	        unsigned short nduseropt_pad2;       off 10
//	        unsigned int   nduseropt_pad3;       off 12
//	};                                           options at 16, then attributes
//	                                             at NLMSG_ALIGN(16 + opts_len)
func decodeNDUserOpt(body []byte) (ndUserOpt, error) {
	if len(body) < ndUserOptHeaderLen {
		return ndUserOpt{}, errNDUserOptShort
	}
	if body[0] != unix.AF_INET6 {
		return ndUserOpt{}, errNDUserOptFamily
	}
	optsLen := int(binary.NativeEndian.Uint16(body[2:4]))
	end := ndUserOptHeaderLen + optsLen
	if end > len(body) {
		return ndUserOpt{}, errNDUserOptLength
	}
	msg := ndUserOpt{
		ifindex:  int(int32(binary.NativeEndian.Uint32(body[4:8]))),
		icmpType: body[8],
		options:  body[ndUserOptHeaderLen:end],
	}
	// Each attribute is a 4-octet rtattr header then its payload; the walk
	// is bounded by the message length.
	for at := nlAlign(end); at+4 <= len(body); {
		attrLen := int(binary.NativeEndian.Uint16(body[at : at+2]))
		attrType := binary.NativeEndian.Uint16(body[at+2 : at+4])
		if attrLen < 4 || at+attrLen > len(body) {
			break
		}
		if attrType == ndUserOptSourceAddr && attrLen == 4+16 {
			msg.router = netip.AddrFrom16([16]byte(body[at+4 : at+20]))
		}
		at += nlAlign(attrLen)
	}
	return msg, nil
}

// nlAlign rounds n up to the 4-octet netlink alignment.
func nlAlign(n int) int { return (n + 3) &^ 3 }

// startNDUserOpts joins RTNLGRP_ND_USEROPT and starts the reader. stop
// closes the socket, which ends the reader.
func (m *monitor) startNDUserOpts() error {
	sock, err := nl.Subscribe(unix.NETLINK_ROUTE, unix.RTNLGRP_ND_USEROPT)
	if err != nil {
		return err
	}
	if err := sock.SetReceiveBufferSize(monitorReceiveBufferBytes, false); err != nil {
		sock.Close()
		return err
	}
	m.ndSock = sock
	m.ndDone = make(chan struct{})
	go m.readNDUserOpts(sock)
	return nil
}

// readNDUserOpts reads the ND user option group until stop closes the
// socket. One long-lived goroutine for the monitor's lifetime.
func (m *monitor) readNDUserOpts(sock *nl.NetlinkSocket) {
	defer close(m.ndDone)
	for {
		msgs, from, err := sock.Receive()
		if err != nil {
			select {
			case <-m.stopCh:
				return
			default:
			}
			if errors.Is(err, unix.ENOBUFS) {
				logger().Warn("iface monitor: the kernel dropped RA user options (receive buffer full); RDNSS from the next RA restores them", "error", err)
				continue
			}
			logger().Warn("iface monitor: RA user option receive failed; RDNSS from RAs is no longer read until Ze restarts", "error", err)
			return
		}
		if from.Pid != nl.PidKernel {
			continue
		}
		for _, msg := range msgs {
			if msg.Header.Type != unix.RTM_NEWNDUSEROPT {
				continue
			}
			m.handleNDUserOpt(msg.Data)
		}
	}
}

// handleNDUserOpt emits one nd-user-option event per option of an RA
// message. A message the kernel could not have written is logged and
// dropped.
func (m *monitor) handleNDUserOpt(body []byte) {
	msg, err := decodeNDUserOpt(body)
	if err != nil {
		logger().Warn("iface monitor: RA user option message unreadable", "error", err)
		return
	}
	if msg.icmpType != icmpv6RouterAdvertisement {
		return
	}
	name, ok := m.linkName(msg.ifindex)
	if !ok {
		logger().Warn("iface monitor: RA user option for an unknown link index", "index", msg.ifindex)
		return
	}
	router := ""
	if msg.router.IsValid() {
		router = msg.router.String()
	}
	// The kernel copies one option per message; a longer region is walked
	// by each option's Length, and a Length that does not fit hands the rest
	// on whole, for the consumer's validity check to refuse.
	for rest := msg.options; len(rest) > 0; {
		size := len(rest)
		if len(rest) >= 2 && rest[1] != 0 && int(rest[1])*8 <= len(rest) {
			size = int(rest[1]) * 8
		}
		m.emit(ifaceevents.EventNDUserOption, iface.NDUserOptionPayload{Name: name, Router: router, Option: rest[:size]})
		rest = rest[size:]
	}
}

// linkName answers the device name of ifindex, from the link cache or the
// kernel.
func (m *monitor) linkName(ifindex int) (string, bool) {
	if v, ok := m.linkNames.Load(ifindex); ok {
		name, isName := v.(string)
		return name, isName
	}
	link, err := netlink.LinkByIndex(ifindex)
	if err != nil {
		return "", false
	}
	return link.Attrs().Name, true
}
