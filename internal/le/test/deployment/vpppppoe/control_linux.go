//go:build linux

// Design: docs/labs/pppoe-interop.md -- scratch control over an unattached PPPoX socket.
// Related: wire.go -- the bounded control packet encoder.
package testvpppppoe

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"
)

type packetSocket struct {
	fd              int
	index           int
	mac             [6]byte
	discoveryTraces int
}

func runControl(ctx context.Context, device, namespace string) (result controlResult, err error) {
	result = controlResult{Session: sessionID, Interface: device, SocketNamespace: namespace}
	originalNamespace, err := os.Readlink("/proc/thread-self/ns/net")
	if err != nil {
		return result, err
	}
	// Register the owner before namespace entry: restoring that namespace can
	// fail after the callback has already transferred a successfully bound fd.
	raw := packetSocket{fd: -1}
	defer func() {
		if raw.fd >= 0 {
			err = errors.Join(err, unix.Close(raw.fd))
		}
	}()
	if err := openInNamespace(namespace, func() error {
		link, err := net.InterfaceByName(device)
		if err != nil {
			return err
		}
		raw.index = link.Index
		copy(raw.mac[:], link.HardwareAddr)
		fd, err := unix.Socket(unix.AF_PACKET, unix.SOCK_RAW|unix.SOCK_CLOEXEC, int(etherTypeAllNetwork))
		if err != nil {
			return err
		}
		if bindErr := unix.Bind(fd, &unix.SockaddrLinklayer{Ifindex: raw.index, Protocol: etherTypeAllNetwork}); bindErr != nil {
			return errors.Join(bindErr, unix.Close(fd))
		}
		raw.fd = fd
		return nil
	}); err != nil {
		return result, err
	}
	currentNamespace, err := os.Readlink("/proc/thread-self/ns/net")
	if err != nil {
		return result, err
	}
	if currentNamespace != originalNamespace {
		return result, errors.New("control socket creation changed the caller's network namespace")
	}
	result.NamespaceRestored = true
	if err := unix.SetsockoptTimeval(raw.fd, unix.SOL_SOCKET, unix.SO_RCVTIMEO, &unix.Timeval{Sec: 1}); err != nil {
		return result, err
	}
	if err := unix.SetsockoptTimeval(raw.fd, unix.SOL_SOCKET, unix.SO_SNDTIMEO, &unix.Timeval{Sec: 1}); err != nil {
		return result, err
	}
	if err := os.WriteFile(filepath.Join(evidencePath, "control.ready"), []byte("ready\n"), 0o600); err != nil {
		return result, err
	}
	frame, err := raw.discover(ctx, &result)
	if err != nil {
		return result, err
	}
	client := net.HardwareAddr(frame.source[:]).String()
	// RFC 2516 Section 5.4: "When the Access Concentrator receives a PADR packet, it prepares to begin a PPP session."
	if _, err = vppCommand(ctx, "create", "pppoe", "session", "client-ip", "0.0.0.0", "session-id", "17", "client-mac", client, "disable-fib"); err != nil {
		return result, err
	}
	defer func() {
		_, endErr := vppCommand(context.WithoutCancel(ctx), "create", "pppoe", "session", "client-ip", "0.0.0.0", "session-id", "17", "client-mac", client, "del")
		err = errors.Join(err, endErr)
	}()
	sessions, err := vppCommand(ctx, showCommand, "pppoe", "session")
	if err != nil {
		return result, err
	}
	if !strings.Contains(sessions, "session-id 17 ") {
		return result, fmt.Errorf("session absent before PADS: %s", sessions)
	}
	if !strings.Contains(sessions, "client-mac "+client) {
		return result, fmt.Errorf("session has wrong client before PADS: %s", sessions)
	}
	result.SessionBeforePADS = true
	channel := -1
	defer func() {
		if channel >= 0 {
			err = errors.Join(err, unix.Close(channel))
		}
	}()
	if err := openInNamespace(namespace, func() error {
		fd, socketErr := unix.Socket(unix.AF_PPPOX, unix.SOCK_DGRAM|unix.SOCK_CLOEXEC, 0)
		if socketErr != nil {
			return socketErr
		}
		if connectErr := unix.Connect(fd, &unix.SockaddrPPPoE{SID: sessionID, Remote: frame.source[:], Dev: device}); connectErr != nil {
			return errors.Join(connectErr, unix.Close(fd))
		}
		channel = fd
		return nil
	}); err != nil {
		return result, err
	}
	if err := unix.SetsockoptTimeval(channel, unix.SOL_SOCKET, unix.SO_RCVTIMEO, &unix.Timeval{Sec: 1}); err != nil {
		return result, err
	}
	if err := unix.SetsockoptTimeval(channel, unix.SOL_SOCKET, unix.SO_SNDTIMEO, &unix.Timeval{Sec: 1}); err != nil {
		return result, err
	}
	// No /dev/ppp open, PPPIOCATTCHAN, or PPPIOCCONNECT occurs here. The
	// socket itself is the only PPP frame channel whose behavior is measured.
	if err := raw.sendDiscovery(ctx, frame, 0x65, sessionID, frame.tags); err != nil {
		return result, err
	}
	if err := writeControl(ctx, channel, []byte{0xc0, 0x21, 1, 1, 0, 8, 1, 4, 5, 0xd4}); err != nil {
		return result, err
	}
	if err := negotiateControl(ctx, channel, &result); err != nil {
		return result, err
	}
	if err := writeResult(result); err != nil {
		return result, err
	}
	buffer := make([]byte, 4096)
	for ctx.Err() == nil {
		if err := raw.rejectDataOnCP(ctx, buffer); err != nil {
			return result, err
		}
		if _, stopErr := os.Stat(filepath.Join(evidencePath, "stop")); stopErr == nil {
			return result, nil
		} else if !os.IsNotExist(stopErr) {
			return result, stopErr
		}
		length, readErr := controlIO(ctx, func() (int, error) { return unix.Read(channel, buffer) })
		if readErr != nil {
			if errors.Is(readErr, unix.EAGAIN) {
				continue
			}
			return result, readErr
		}
		packet := buffer[:length]
		if len(packet) >= 43 {
			if binary.BigEndian.Uint16(packet[:2]) == 0x0057 {
				if packet[8] == 58 {
					if packet[42] == 133 {
						result.RouterSolicitation = true
					}
				}
				if packet[8] == 17 {
					if len(packet) >= 50 {
						if binary.BigEndian.Uint16(packet[42:44]) == 546 {
							if binary.BigEndian.Uint16(packet[44:46]) == 547 {
								result.DHCPv6 = true
							}
						}
					}
				}
			}
		}
		if err := writeResult(result); err != nil {
			return result, err
		}
	}
	return result, ctx.Err()
}

// rejectDataOnCP drains at most one bounded packet batch, independently of the
// PPPoX reader. A subscriber IP frame on this TAP disproves kernel bypass.
func (socket packetSocket) rejectDataOnCP(ctx context.Context, buffer []byte) error {
	for range 64 {
		if err := ctx.Err(); err != nil {
			return err
		}
		length, _, err := unix.Recvfrom(socket.fd, buffer, unix.MSG_DONTWAIT)
		if err != nil {
			if errors.Is(err, unix.EINTR) {
				continue
			}
			if errors.Is(err, unix.EAGAIN) {
				return nil
			}
			return err
		}
		frame := buffer[:length]
		offset := 14
		if len(frame) < 22 {
			continue
		}
		kind := binary.BigEndian.Uint16(frame[12:14])
		if kind == 0x8100 {
			if len(frame) < 26 {
				continue
			}
			offset = 18
			kind = binary.BigEndian.Uint16(frame[16:18])
		}
		if kind != 0x8864 {
			continue
		}
		if binary.BigEndian.Uint16(frame[offset+2:offset+4]) != sessionID {
			continue
		}
		if binary.BigEndian.Uint16(frame[offset+6:offset+8]) == 0x0021 {
			return errors.New("subscriber IPv4 data reached the Linux control TAP")
		}
	}
	return nil
}

// openInNamespace creates sockets on a locked thread, then returns that thread
// to the caller's namespace before any frame is read or written. Thus the
// socket does not rely on the control process living alongside the TAP.
// The caller MUST register cleanup before calling: open MUST transfer each
// acquired resource to that owner before returning nil, because subsequent
// namespace restoration or handle cleanup can still report an error.
func openInNamespace(name string, open func() error) (err error) {
	if name == "" {
		return open()
	}
	runtime.LockOSThread()
	restored := true
	defer func() {
		if restored {
			runtime.UnlockOSThread()
		}
	}()
	original, err := netns.Get()
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, original.Close()) }()
	target, err := netns.GetFromName(name)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, target.Close()) }()
	if err := netns.Set(target); err != nil {
		return err
	}
	restored = false
	defer func() {
		restoreErr := netns.Set(original)
		restored = restoreErr == nil
		err = errors.Join(err, restoreErr)
	}()
	return open()
}

func writeResult(result controlResult) error {
	body, err := json.Marshal(result)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(evidencePath, "control.state"), body, 0o600)
}

func vppCommand(ctx context.Context, args ...string) (string, error) {
	commandCtx, cancel := context.WithTimeout(ctx, controlCommandBudget)
	defer cancel()
	argv := append([]string{"-s", vppCLIPath}, args...)
	command := exec.CommandContext(commandCtx, vppctlCommand, argv...) //nolint:gosec // The bounded lab helper owns the fixed vppctl executable and command grammar.
	body, err := command.CombinedOutput()
	if err != nil {
		return string(body), fmt.Errorf("vppctl %v: %w: %s", args, errors.Join(err, commandCtx.Err()), body)
	}
	return string(body), nil
}

// negotiateControl is deliberately limited to this lab's no-auth, fixed-IP
// pppd settings. The real product's PPP engine is neither called nor changed.
// Pppd 2.5.5 lcp_ackci and ipcp_reqci supply the peer-side contracts.
func negotiateControl(ctx context.Context, fd int, result *controlResult) error {
	buffer := make([]byte, 4096)
	lcpPeerAck, lcpLocalAck, ipcpPeerAck, ipcpLocalAck := false, false, false, false
	ipcpSent := false
	for ctx.Err() == nil {
		length, err := controlIO(ctx, func() (int, error) { return unix.Read(fd, buffer) })
		if err != nil {
			if errors.Is(err, unix.EAGAIN) {
				continue
			}
			return err
		}
		packet := buffer[:length]
		if len(packet) < 6 {
			return errors.New("truncated PPP control received from pppd")
		}
		protocol := binary.BigEndian.Uint16(packet[:2])
		if protocol != 0xc021 {
			if protocol != 0x8021 {
				return fmt.Errorf("unexpected PPP protocol %04x in fixed lab negotiation", protocol)
			}
		}
		switch packet[2] {
		case 1:
			// RFC 1661 Section 5.2.
			ack, err := controlAck(packet)
			if err != nil {
				return err
			}
			if err := writeControl(ctx, fd, ack); err != nil {
				return err
			}
			if protocol == 0xc021 {
				lcpPeerAck = true
				result.LCPRead = true
				result.LCPWrite = true
				if !lcpLocalAck {
					if err := writeControl(ctx, fd, []byte{0xc0, 0x21, 1, 1, 0, 8, 1, 4, 5, 0xd4}); err != nil {
						return err
					}
				}
			} else {
				ipcpPeerAck = true
			}
		case 2:
			if protocol == 0xc021 {
				if packet[3] == 1 {
					lcpLocalAck = true
				}
			} else {
				if packet[3] == 2 {
					ipcpLocalAck = true
				}
			}
		default:
			return fmt.Errorf("unexpected PPP Configure response %d", packet[2])
		}
		if lcpPeerAck {
			if lcpLocalAck {
				if !ipcpSent {
					if err := writeControl(ctx, fd, []byte{0x80, 0x21, 1, 2, 0, 10, 3, 6, 10, 20, 0, 1}); err != nil {
						return err
					}
					ipcpSent = true
				}
			}
		}
		if ipcpPeerAck {
			if ipcpLocalAck {
				result.IPCPOpen = true
				return nil
			}
		}
	}
	return ctx.Err()
}

func writeControl(ctx context.Context, fd int, packet []byte) error {
	length, err := controlIO(ctx, func() (int, error) { return unix.Write(fd, packet) })
	if err != nil {
		return err
	}
	if length != len(packet) {
		return errors.New("short PPP control datagram write")
	}
	return nil
}

const etherTypeAllNetwork uint16 = unix.ETH_P_ALL<<8 | unix.ETH_P_ALL>>8

func (socket *packetSocket) discover(ctx context.Context, result *controlResult) (discoveryFrame, error) {
	buffer := make([]byte, 4096)
	for ctx.Err() == nil {
		length, err := controlIO(ctx, func() (int, error) {
			length, _, err := unix.Recvfrom(socket.fd, buffer, 0)
			return length, err
		})
		if err != nil {
			if errors.Is(err, unix.EAGAIN) {
				continue
			}
			return discoveryFrame{}, err
		}
		if length >= 14 {
			if binary.BigEndian.Uint16(buffer[12:14]) == 0x8864 {
				return discoveryFrame{}, errors.New("malformed session frame crossed the CP TAP before the discovery barrier")
			}
		}
		frame, err := parseDiscovery(buffer[:length])
		if err != nil {
			continue
		}
		if frame.source == socket.mac {
			continue
		}
		if err := socket.traceDiscovery("receive", buffer[:length]); err != nil {
			return discoveryFrame{}, err
		}
		switch frame.code {
		case 0x09:
			if strings.Contains(string(frame.tags), "bounds-barrier") {
				result.BoundsBarrier = true
				if err := writeResult(*result); err != nil {
					return discoveryFrame{}, err
				}
			}
			tags := append([]byte(nil), frame.tags...)
			name := []byte("Ze VPP mechanism")
			tags = append(tags, 1, 2, 0, byte(len(name)))
			tags = append(tags, name...)
			if err := socket.sendDiscovery(ctx, frame, 0x07, 0, tags); err != nil {
				return discoveryFrame{}, err
			}
		case 0x19:
			return frame, nil
		}
	}
	return discoveryFrame{}, ctx.Err()
}

// RFC 2516 Section 5.4: "The DESTINATION_ADDR field is the unicast Ethernet address of the Host that sent the PADR."
func (socket *packetSocket) sendDiscovery(ctx context.Context, peer discoveryFrame, code byte, sid uint16, tags []byte) error {
	offset := 14
	if len(peer.vlan) > 0 {
		offset = 18
	}
	frame := make([]byte, offset+6+len(tags))
	copy(frame[:6], peer.source[:])
	copy(frame[6:12], socket.mac[:])
	if len(peer.vlan) > 0 {
		copy(frame[12:16], peer.vlan)
		binary.BigEndian.PutUint16(frame[16:18], 0x8863)
	} else {
		binary.BigEndian.PutUint16(frame[12:14], 0x8863)
	}
	frame[offset] = 0x11
	frame[offset+1] = code
	binary.BigEndian.PutUint16(frame[offset+2:offset+4], sid)
	binary.BigEndian.PutUint16(frame[offset+4:offset+6], uint16(len(tags)))
	copy(frame[offset+6:], tags)
	if err := socket.traceDiscovery("send", frame); err != nil {
		return err
	}
	var destination [8]byte
	copy(destination[:], peer.source[:])
	_, err := controlIO(ctx, func() (int, error) {
		return 0, unix.Sendto(socket.fd, frame, 0, &unix.SockaddrLinklayer{Ifindex: socket.index, Halen: 6, Addr: destination})
	})
	return err
}

// traceDiscovery keeps the first sixteen envelopes in the existing helper
// launch log, so a failed VLAN dial retains its actual tag bytes and endpoints.
func (socket *packetSocket) traceDiscovery(direction string, frame []byte) error {
	if socket.discoveryTraces >= 16 {
		return nil
	}
	socket.discoveryTraces++
	_, err := fmt.Fprintf(os.Stderr, "discovery %s ifindex=%d envelope=%x bytes=%d\n", direction, socket.index, frame[:min(len(frame), 24)], len(frame))
	return err
}
