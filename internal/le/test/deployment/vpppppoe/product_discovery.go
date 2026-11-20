// Design: docs/labs/pppoe-interop.md -- discovery on the physical VPP access wire.
// Related: product_check.go -- this is observed wire state, not a PADS/native timing claim.
package testvpppppoe

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/pcap"
	"github.com/ze-software/ze/internal/le/interoplab"
)

// The capture file lives in the container's own /tmp: tcpdump drops to its
// own user and chowns the savefile, which the 9p /evidence share refuses
// ("Couldn't change ownership of savefile"). It is read back in place.
func startProductDiscovery(ctx context.Context, lab interoplab.CheckerLab) error {
	command := `rm -f /evidence/product-discovery.done /evidence/product-discovery.log /tmp/product-discovery.pcap; timeout --signal=INT 600s tcpdump -U -n -s 512 -i eth0 -c 2048 -w /tmp/product-discovery.pcap 'ether proto 0x8863 or (vlan 100 and ether proto 0x8863)' > /evidence/product-discovery.log 2>&1 & child=$!; echo "$child" > /evidence/product-discovery.pid; wait "$child"; status=$?; echo "$status" > /evidence/product-discovery.done`
	if err := lab.ExecDetached(ctx, vppPeer, []string{"sh", "-c", command}, nil); err != nil {
		return err
	}
	_, err := waitText(ctx, lab, []string{readFileCommand, "/evidence/product-discovery.log"}, "listening on eth0")
	return err
}

func finishProductDiscovery(ctx context.Context, lab interoplab.CheckerLab, sessions []productSession) error {
	log, err := joinProductCapture(ctx, lab, "product-discovery")
	if err != nil {
		return err
	}
	if err := productCaptureNoDrops(log); err != nil {
		return err
	}
	encoded, err := query(ctx, lab, vppPeer, "base64", "/tmp/product-discovery.pcap")
	if err != nil {
		return err
	}
	if len(encoded) > 2<<20 {
		return errors.New("discovery capture exceeds its packet/snapshot bound")
	}
	wire, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return err
	}
	mac, err := interfaceMAC(ctx, lab)
	if err != nil {
		return err
	}
	return productDiscovery(wire, mac, sessions)
}

// joinProductCapture joins only the capture PID published by its owned peer.
// The stem is internal, never a user-provided path or a process-name selector.
func joinProductCapture(ctx context.Context, lab interoplab.CheckerLab, stem string) (string, error) {
	if stem != "product-cp" && stem != "product-discovery" && stem != "product-capacity" {
		return "", errors.New("unknown product capture")
	}
	prefix := "/evidence/" + stem
	text, err := query(ctx, lab, vppPeer, readFileCommand, prefix+".pid")
	if err != nil {
		return "", err
	}
	pid, err := strconv.ParseUint(strings.TrimSpace(text), 10, 31)
	if err != nil || pid < 2 {
		return "", fmt.Errorf("owned capture has invalid PID %q", text)
	}
	if _, err := query(ctx, lab, vppPeer, "kill", "-INT", strconv.FormatUint(pid, 10)); err != nil {
		return "", err
	}
	status, _, err := interoplab.Wait(ctx, productWait(stem+" join"), func(ctx context.Context) (string, error) {
		return query(ctx, lab, vppPeer, readFileCommand, prefix+".done")
	}, func(text string) bool { return strings.TrimSpace(text) != "" })
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(status) != "0" && strings.TrimSpace(status) != "130" {
		return "", fmt.Errorf("owned capture exited unsuccessfully: %q", status)
	}
	return query(ctx, lab, vppPeer, readFileCommand, prefix+".log")
}

func productDiscovery(wire []byte, local string, sessions []productSession) error {
	reader, err := pcap.NewReader(bytes.NewReader(wire))
	if err != nil {
		return err
	}
	if reader.LinkType() != pcap.LinkTypeEthernet {
		return errors.New("physical discovery capture is not Ethernet")
	}
	owners := make(map[string]uint16, len(sessions))
	for _, session := range sessions {
		owners[session.MAC] = session.SID
	}
	stages := make(map[string]int, len(sessions))
	var record pcap.Record
	frameIndex := 0
	for {
		if err := reader.Next(&record); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return err
		}
		frameIndex++
		frame := record.Data
		if record.OriginalLen != len(frame) || len(frame) < 20 {
			return errors.New("discovery capture contains a truncated Ethernet frame")
		}
		source, destination := net.HardwareAddr(frame[6:12]).String(), net.HardwareAddr(frame[:6]).String()
		offset, kind := 14, binary.BigEndian.Uint16(frame[12:14])
		if kind == 0x8100 {
			if len(frame) < 24 || binary.BigEndian.Uint16(frame[14:16])&0xfff != 100 {
				return errors.New("discovery capture contains an unexpected VLAN")
			}
			offset, kind = 18, binary.BigEndian.Uint16(frame[16:18])
		}
		if kind != 0x8863 {
			return errors.New("discovery capture contains an unexpected EtherType")
		}
		packet := frame[offset:]
		length := int(binary.BigEndian.Uint16(packet[4:6]))
		if packet[0] != 0x11 || length > len(packet)-6 {
			return errors.New("discovery capture contains an invalid PPPoE header")
		}
		code, sid := packet[1], binary.BigEndian.Uint16(packet[2:4])
		client := source
		if code == 0x07 || code == 0x65 {
			client = destination
		}
		wanted, tracked := owners[client]
		if !tracked {
			continue
		}
		switch code {
		case 0x09:
			if sid != 0 || destination != "ff:ff:ff:ff:ff:ff" {
				return fmt.Errorf("PADI has a nonzero SID or nonbroadcast destination: %s", describeDiscoveryFrame(frameIndex, source, destination, sid, stages[client]))
			}
			if stages[client] == 0 {
				stages[client] = 1
			}
		case 0x07:
			if stages[client] < 1 || source != local || sid != 0 {
				return fmt.Errorf("PADO did not answer the real PADI from the access MAC %s: %s", local, describeDiscoveryFrame(frameIndex, source, destination, sid, stages[client]))
			}
			if err := productDiscoveryTags(packet[6 : 6+length]); err != nil {
				return err
			}
			if stages[client] < 2 {
				stages[client] = 2
			}
		case 0x19:
			if stages[client] < 2 || destination != local || sid != 0 {
				return fmt.Errorf("PADR did not select the observed access concentrator %s: %s", local, describeDiscoveryFrame(frameIndex, source, destination, sid, stages[client]))
			}
			if stages[client] < 3 {
				stages[client] = 3
			}
		case 0x65:
			if stages[client] < 3 || source != local || sid != wanted || sid == 0 {
				return fmt.Errorf("PADS wire SID/MAC differs from the real daemon/native owner (SID %d, access MAC %s): %s", wanted, local, describeDiscoveryFrame(frameIndex, source, destination, sid, stages[client]))
			}
			if err := productDiscoveryTags(packet[6 : 6+length]); err != nil {
				return err
			}
			stages[client] = 4
		default:
			return fmt.Errorf("unexpected discovery code 0x%02x during successful admission", code)
		}
	}
	for client := range owners {
		if stages[client] != 4 {
			return fmt.Errorf("subscriber %s has no complete physical PADI/PADO/PADR/PADS exchange", client)
		}
	}
	if len(owners) == 0 {
		return errors.New("discovery proof has no successful subscriber")
	}
	return nil
}

func productDiscoveryTags(payload []byte) error {
	name, service := false, false
	for len(payload) != 0 {
		if len(payload) < 4 {
			return errors.New("discovery tag header is truncated")
		}
		kind, length := binary.BigEndian.Uint16(payload[:2]), int(binary.BigEndian.Uint16(payload[2:4]))
		if length > len(payload)-4 {
			return errors.New("discovery tag value is truncated")
		}
		value := string(payload[4 : 4+length])
		if kind == 0x0102 {
			if name || value != "ze-vpp-bng" {
				return errors.New("PADO/PADS AC-Name differs from configured Ze")
			}
			name = true
		}
		if kind == 0x0101 {
			if service || value != productService {
				return errors.New("PADO/PADS Service-Name differs from configured Ze")
			}
			service = true
		}
		payload = payload[4+length:]
	}
	if !name || !service {
		return errors.New("PADO/PADS lacks the configured AC-Name or Service-Name")
	}
	return nil
}

// describeDiscoveryFrame names one captured discovery frame for a refusal: its
// position in the capture, its addresses and SID, and how far its
// subscriber's exchange had got (0 none, 1 PADI, 2 PADO, 3 PADR).
func describeDiscoveryFrame(index int, source, destination string, sid uint16, stage int) string {
	return fmt.Sprintf("frame %d src %s dst %s sid %d, exchange at stage %d", index, source, destination, sid, stage)
}
