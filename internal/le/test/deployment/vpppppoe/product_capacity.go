// Design: docs/labs/pppoe-interop.md -- a capacity refusal on the physical wire.
// Related: product_discovery.go -- the independently joined successful capture.
package testvpppppoe

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"

	"github.com/ze-software/ze/internal/core/pcap"
	"github.com/ze-software/ze/internal/le/interoplab"
)

func productHostUniq(index int) string { return fmt.Sprintf("%08x", index+1) }

// A separate capture retains the first hundred clients' successful exchange.
// Only this owned process writes the capacity artifact, and its join precedes
// parsing. The fixed stem and bounded filter never accept a caller path.
func startProductCapacity(ctx context.Context, lab interoplab.CheckerLab) error {
	command := `rm -f /evidence/product-capacity.done /evidence/product-capacity.log /tmp/product-capacity.pcap; timeout --signal=INT 120s tcpdump -U -n -s 512 -i eth0 -c 128 -w /tmp/product-capacity.pcap 'ether proto 0x8863 or (vlan 100 and ether proto 0x8863)' > /evidence/product-capacity.log 2>&1 & child=$!; echo "$child" > /evidence/product-capacity.pid; wait "$child"; status=$?; echo "$status" > /evidence/product-capacity.done`
	if err := lab.ExecDetached(ctx, vppPeer, []string{"sh", "-c", command}, nil); err != nil {
		return err
	}
	_, err := waitText(ctx, lab, []string{readFileCommand, "/evidence/product-capacity.log"}, "listening on eth0")
	return err
}

func finishProductCapacity(ctx context.Context, lab interoplab.CheckerLab) error {
	log, err := joinProductCapture(ctx, lab, "product-capacity")
	if err != nil {
		return err
	}
	if err := productCaptureNoDrops(log); err != nil {
		return err
	}
	encoded, err := query(ctx, lab, vppPeer, "base64", "/tmp/product-capacity.pcap")
	if err != nil {
		return err
	}
	if len(encoded) > 128<<10 {
		return errors.New("capacity capture exceeds its packet/snapshot bound")
	}
	wire, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return err
	}
	local, err := interfaceMAC(ctx, lab)
	if err != nil {
		return err
	}
	_, client := productClient(100)
	if err := productCapacityRefusal(wire, local, client, productHostUniq(100)); err != nil {
		return err
	}
	// BuildPADSError also handles other resource errors. This existing producer
	// reason identifies AllocSID exhaustion, rather than a later native failure.
	daemon, err := query(ctx, lab, vppPeer, readFileCommand, "/evidence/product-ze.log")
	if err != nil {
		return err
	}
	if !productCapacityReason(daemon) {
		return errors.New("correlated refusal has no actual session-ID exhaustion producer decision")
	}
	peer, err := query(ctx, lab, clientPeer, readFileCommand, productLog(100))
	if err != nil {
		return err
	}
	if !strings.Contains(peer, "PADS: System-Error:") {
		return errors.New("101st pppd did not receive the captured resource refusal")
	}
	return nil
}

func productCapacityReason(log string) bool {
	for line := range strings.SplitSeq(log, "\n") {
		if !strings.Contains(line, "pppoe: session ID exhausted") {
			continue
		}
		for field := range strings.FieldsSeq(line) {
			if field == "reason=session-id-exhausted" {
				return true
			}
		}
	}
	return false
}

// pppd2.5.5 discovery.c sends Host-Uniq in PADI/PADR and accepts a PADS
// System-Error only from the chosen AC for that Host-Uniq. Ze's BuildPADSError
// echoes it with AC-Name, zero SID and the empty AC-System-Error tag. The error
// PADS intentionally has no Service-Name; service is checked on PADO/PADR.
func productCapacityRefusal(wire []byte, local, client, transaction string) error {
	unique, err := hex.DecodeString(transaction)
	if err != nil {
		return err
	}
	if len(unique) != 4 {
		return errors.New("capacity transaction is not the configured four-octet Host-Uniq")
	}
	reader, err := pcap.NewReader(bytes.NewReader(wire))
	if err != nil {
		return err
	}
	if reader.LinkType() != pcap.LinkTypeEthernet {
		return errors.New("capacity capture is not Ethernet")
	}
	stage := 0
	var record pcap.Record
	for {
		err := reader.Next(&record)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		frame := record.Data
		if record.OriginalLen != len(frame) {
			return errors.New("capacity Ethernet frame is truncated")
		}
		if len(frame) < 20 {
			return errors.New("capacity Ethernet header is truncated")
		}
		source, destination := net.HardwareAddr(frame[6:12]).String(), net.HardwareAddr(frame[:6]).String()
		if source != client {
			if destination != client {
				continue
			}
		}
		offset, kind := 14, binary.BigEndian.Uint16(frame[12:14])
		if kind == 0x8100 {
			if len(frame) < 24 {
				return errors.New("capacity VLAN header is truncated")
			}
			if binary.BigEndian.Uint16(frame[14:16])&0xfff != 100 {
				return errors.New("capacity VLAN is not the selected access")
			}
			offset, kind = 18, binary.BigEndian.Uint16(frame[16:18])
		}
		if kind != 0x8863 {
			return errors.New("capacity frame is not PPPoE discovery")
		}
		packet := frame[offset:]
		if packet[0] != 0x11 {
			return errors.New("capacity PPPoE version/type differs")
		}
		length := int(binary.BigEndian.Uint16(packet[4:6]))
		if length > len(packet)-6 {
			return errors.New("capacity PPPoE payload is truncated")
		}
		tags, err := productCapacityTags(packet[6 : 6+length])
		if err != nil {
			return err
		}
		if !bytes.Equal(tags[0x0103], unique) {
			return errors.New("capacity frame does not belong to the current 101st dial")
		}
		sid := binary.BigEndian.Uint16(packet[2:4])
		if sid != 0 {
			return errors.New("capacity dial was admitted instead of refused")
		}
		switch packet[1] {
		case 0x09:
			if source != client {
				return errors.New("capacity PADI has a foreign source")
			}
			if destination != "ff:ff:ff:ff:ff:ff" {
				return errors.New("capacity PADI is not broadcast")
			}
			if string(tags[0x0101]) != productService {
				return errors.New("capacity PADI requested another service")
			}
			if stage == 0 {
				stage = 1
			}
		case 0x07:
			if stage < 1 {
				return errors.New("capacity PADO precedes the actual PADI")
			}
			if source != local {
				return errors.New("capacity PADO has a foreign access MAC")
			}
			if destination != client {
				return errors.New("capacity PADO answers another client")
			}
			if err := productDiscoveryTags(packet[6 : 6+length]); err != nil {
				return err
			}
			if stage < 2 {
				stage = 2
			}
		case 0x19:
			if stage < 2 {
				return errors.New("capacity PADR has no actual chosen PADO")
			}
			if source != client {
				return errors.New("capacity PADR has a foreign source")
			}
			if destination != local {
				return errors.New("capacity PADR selected another access concentrator")
			}
			if string(tags[0x0101]) != productService {
				return errors.New("capacity PADR requested another service")
			}
			if stage < 3 {
				stage = 3
			}
		case 0x65:
			if stage < 3 {
				return errors.New("capacity PADS did not answer the actual PADR")
			}
			if source != local {
				return errors.New("capacity refusal has a foreign access MAC")
			}
			if destination != client {
				return errors.New("capacity refusal answers another client")
			}
			if string(tags[0x0102]) != "ze-vpp-bng" {
				return errors.New("capacity refusal has a foreign AC-Name")
			}
			refused, present := tags[0x0202]
			if !present {
				return errors.New("capacity PADS has no AC-System-Error")
			}
			if len(refused) != 0 {
				return errors.New("capacity PADS did not carry Ze's exact resource refusal")
			}
			if _, wrong := tags[0x0201]; wrong {
				return errors.New("capacity PADS is a service refusal")
			}
			stage = 4
		default:
			return fmt.Errorf("unexpected capacity discovery code 0x%02x", packet[1])
		}
	}
	if stage != 4 {
		return errors.New("101st peer has no complete correlated PADI/PADO/PADR/resource-refusal exchange")
	}
	return nil
}

func productCapacityTags(payload []byte) (map[uint16][]byte, error) {
	tags := make(map[uint16][]byte)
	for len(payload) != 0 {
		if len(payload) < 4 {
			return nil, errors.New("capacity tag header is truncated")
		}
		kind, length := binary.BigEndian.Uint16(payload[:2]), int(binary.BigEndian.Uint16(payload[2:4]))
		if length > len(payload)-4 {
			return nil, errors.New("capacity tag payload is truncated")
		}
		if _, repeated := tags[kind]; repeated {
			return nil, errors.New("capacity tag is repeated")
		}
		tags[kind] = payload[4 : 4+length]
		payload = payload[4+length:]
	}
	return tags, nil
}
