// Design: docs/architecture/diagnostics/packet-capture.md -- portable types and helpers

//go:build linux

package cmd

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

var (
	errCaptureIfaceMissing      = errors.New("capture: missing interface name")
	errCaptureCountRequiresVal  = errors.New("capture: count requires a value")
	errCaptureDurRequiresVal    = errors.New("capture: duration requires a value (e.g. 5s)")
	errCaptureSnapRequiresVal   = errors.New("capture: snap-len requires a value")
	errCaptureFormatRequiresVal = errors.New("capture: format requires a value (pcap or text)")
)

const (
	defaultCaptureCount   = 100
	maxCaptureCount       = 10000
	defaultCaptureDur     = 10 * time.Second
	maxCaptureDur         = 60 * time.Second
	minCaptureDur         = time.Second
	defaultCaptureSnapLen = 65535
	minCaptureSnapLen     = 64
	maxCaptureSnapLen     = 65535

	captureFormatPcap = "pcap"
	captureFormatText = "text"

	linkTypeEthernet uint32 = 1
)

var activeCaptures sync.Map

type captureArgs struct {
	iface   string
	filter  string
	count   int
	dur     time.Duration
	snapLen int
	format  string
}

func parseCaptureArgs(args []string) (captureArgs, error) {
	ca := captureArgs{
		count:   defaultCaptureCount,
		dur:     defaultCaptureDur,
		snapLen: defaultCaptureSnapLen,
		format:  captureFormatPcap,
	}

	filterParts := make([]string, 0)
	i := 0
	for i < len(args) {
		switch args[i] {
		case argCount:
			if i+1 >= len(args) {
				return ca, errCaptureCountRequiresVal
			}
			n, err := strconv.Atoi(args[i+1])
			if err != nil || n < 1 || n > maxCaptureCount {
				return ca, fmt.Errorf("capture: count must be 1-%d", maxCaptureCount)
			}
			ca.count = n
			i += 2
		case "duration":
			if i+1 >= len(args) {
				return ca, errCaptureDurRequiresVal
			}
			d, err := time.ParseDuration(args[i+1])
			if err != nil || d < minCaptureDur || d > maxCaptureDur {
				return ca, fmt.Errorf("capture: duration must be %s-%s", minCaptureDur, maxCaptureDur)
			}
			ca.dur = d
			i += 2
		case "snap-len":
			if i+1 >= len(args) {
				return ca, errCaptureSnapRequiresVal
			}
			n, err := strconv.Atoi(args[i+1])
			if err != nil || n < minCaptureSnapLen || n > maxCaptureSnapLen {
				return ca, fmt.Errorf("capture: snap-len must be %d-%d", minCaptureSnapLen, maxCaptureSnapLen)
			}
			ca.snapLen = n
			i += 2
		case "format":
			if i+1 >= len(args) {
				return ca, errCaptureFormatRequiresVal
			}
			switch args[i+1] {
			case captureFormatPcap, captureFormatText:
				ca.format = args[i+1]
			default:
				return ca, fmt.Errorf("capture: unknown format %q (use pcap or text)", args[i+1])
			}
			i += 2
		case "protocol":
			i++
		default:
			if ca.iface == "" {
				ca.iface = args[i]
			} else {
				filterParts = append(filterParts, args[i])
			}
			i++
		}
	}

	if ca.iface == "" {
		return ca, errCaptureIfaceMissing
	}
	if len(filterParts) > 0 {
		ca.filter = joinFilterParts(filterParts)
	}
	return ca, nil
}

func joinFilterParts(parts []string) string {
	var b textbuf.Buffer
	for i, p := range parts {
		if i > 0 {
			b.Byte(' ')
		}
		b.Str(p)
	}
	return b.String()
}

func formatPacketLine(ts time.Time, data []byte) string {
	tsStr := ts.Format("15:04:05.000")
	if len(data) < 14 {
		var b textbuf.Buffer
		b.Str(tsStr).Str(" ??? len=").Int(int64(len(data))).Byte(' ').Hex(truncateBytes(data))
		return b.String()
	}

	etherType := binary.BigEndian.Uint16(data[12:14])
	payload := data[14:]

	if etherType == 0x8100 && len(payload) >= 4 {
		etherType = binary.BigEndian.Uint16(payload[2:4])
		payload = payload[4:]
	}

	switch etherType {
	case 0x0800:
		return formatIPv4Packet(tsStr, payload, data)
	case 0x86DD:
		return formatIPv6Packet(tsStr, payload, data)
	default:
		var ethBytes [2]byte
		binary.BigEndian.PutUint16(ethBytes[:], etherType)
		var b textbuf.Buffer
		b.Str(tsStr).Str(" ETH:0x").Hex(ethBytes[:]).Str(" len=").Int(int64(len(data))).Byte(' ').Hex(truncateBytes(data))
		return b.String()
	}
}

func formatIPv4Packet(tsStr string, ip, raw []byte) string {
	if len(ip) < 20 {
		var b textbuf.Buffer
		b.Str(tsStr).Str(" IPv4 (truncated) len=").Int(int64(len(raw))).Byte(' ').Hex(truncateBytes(raw))
		return b.String()
	}

	proto := ip[9]
	ihl := int(ip[0]&0x0f) * 4
	if ihl < 20 || len(ip) < ihl {
		var b textbuf.Buffer
		b.Str(tsStr).Str(" IPv4 (truncated) len=").Int(int64(len(raw))).Byte(' ').Hex(truncateBytes(raw))
		return b.String()
	}
	srcIP := net.IP(ip[12:16]).String()
	dstIP := net.IP(ip[16:20]).String()

	transport := ip[ihl:]
	return formatTransport(tsStr, proto, srcIP, dstIP, transport, raw)
}

func formatIPv6Packet(tsStr string, ip, raw []byte) string {
	if len(ip) < 40 {
		var b textbuf.Buffer
		b.Str(tsStr).Str(" IPv6 (truncated) len=").Int(int64(len(raw))).Byte(' ').Hex(truncateBytes(raw))
		return b.String()
	}

	srcIP := net.IP(ip[8:24]).String()
	dstIP := net.IP(ip[24:40]).String()

	proto, transport := skipIPv6ExtHeaders(ip)
	return formatTransport(tsStr, proto, srcIP, dstIP, transport, raw)
}

func skipIPv6ExtHeaders(ip []byte) (byte, []byte) {
	nextHdr := ip[6]
	off := 40
	for {
		if off >= len(ip) {
			return nextHdr, nil
		}
		switch nextHdr {
		case 0, 43, 60:
			if off+2 > len(ip) {
				return nextHdr, nil
			}
			extLen := int(ip[off+1])*8 + 8
			nextHdr = ip[off]
			off += extLen
		case 44:
			if off+8 > len(ip) {
				return nextHdr, nil
			}
			nextHdr = ip[off]
			off += 8
		default:
			return nextHdr, ip[off:]
		}
	}
}

func formatTransport(tsStr string, proto byte, srcIP, dstIP string, transport, raw []byte) string {
	switch proto {
	case 6:
		if len(transport) < 14 {
			var b textbuf.Buffer
			b.Str(tsStr).Str(" TCP ").Str(srcIP).Str(" -> ").Str(dstIP).Str(" (truncated) ").Int(int64(len(raw))).Byte(' ').Hex(truncateBytes(raw))
			return b.String()
		}
		srcPort := binary.BigEndian.Uint16(transport[0:2])
		dstPort := binary.BigEndian.Uint16(transport[2:4])
		flags := formatTCPFlags(transport[13])

		var b textbuf.Buffer
		b.Str(tsStr).Str(" TCP ").Str(srcIP).Byte(':').Uint16(srcPort).Str(" -> ").Str(dstIP).Byte(':').Uint16(dstPort).Byte(' ').Str(flags).Byte(' ').Int(int64(len(raw))).Byte(' ').Hex(truncateBytes(raw))
		return b.String()

	case 17:
		if len(transport) < 8 {
			var b textbuf.Buffer
			b.Str(tsStr).Str(" UDP ").Str(srcIP).Str(" -> ").Str(dstIP).Str(" (truncated) ").Int(int64(len(raw))).Byte(' ').Hex(truncateBytes(raw))
			return b.String()
		}
		srcPort := binary.BigEndian.Uint16(transport[0:2])
		dstPort := binary.BigEndian.Uint16(transport[2:4])

		var b textbuf.Buffer
		b.Str(tsStr).Str(" UDP ").Str(srcIP).Byte(':').Uint16(srcPort).Str(" -> ").Str(dstIP).Byte(':').Uint16(dstPort).Byte(' ').Int(int64(len(raw))).Byte(' ').Hex(truncateBytes(raw))
		return b.String()

	default:
		protoName := "PROTO:" + textbuf.StringInt(int64(proto))
		switch proto {
		case 1:
			protoName = "ICMP"
		case 58:
			protoName = "ICMPv6"
		case 47:
			protoName = "GRE"
		case 89:
			protoName = "OSPF"
		}

		var b textbuf.Buffer
		b.Str(tsStr).Byte(' ').Str(protoName).Byte(' ').Str(srcIP).Str(" -> ").Str(dstIP).Byte(' ').Int(int64(len(raw))).Byte(' ').Hex(truncateBytes(raw))
		return b.String()
	}
}

func formatTCPFlags(b byte) string {
	var buf textbuf.Buffer
	buf.Byte('[')
	first := true
	add := func(name string) {
		if !first {
			buf.Byte(',')
		}
		buf.Str(name)
		first = false
	}
	if b&0x02 != 0 {
		add("SYN")
	}
	if b&0x10 != 0 {
		add("ACK")
	}
	if b&0x01 != 0 {
		add("FIN")
	}
	if b&0x04 != 0 {
		add("RST")
	}
	if b&0x08 != 0 {
		add("PSH")
	}
	if b&0x20 != 0 {
		add("URG")
	}
	buf.Byte(']')
	return buf.String()
}

const maxHexBytes = 64

func truncateBytes(data []byte) []byte {
	if len(data) > maxHexBytes {
		return data[:maxHexBytes]
	}
	return data
}

// writePcapPacketWithOrigLen writes one pcap packet record with separate
// captured and original lengths, so Wireshark shows truncation correctly.
func writePcapPacketWithOrigLen(w io.Writer, ts time.Time, data []byte, origLen int) error {
	var hdr [16]byte
	binary.LittleEndian.PutUint32(hdr[0:4], uint32(ts.Unix()))
	binary.LittleEndian.PutUint32(hdr[4:8], uint32(ts.Nanosecond()/1000))
	binary.LittleEndian.PutUint32(hdr[8:12], uint32(len(data)))
	binary.LittleEndian.PutUint32(hdr[12:16], uint32(origLen))
	if _, err := w.Write(hdr[:]); err != nil {
		return err
	}
	_, err := w.Write(data)
	return err
}
