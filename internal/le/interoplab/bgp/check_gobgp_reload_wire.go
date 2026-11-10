// Design: docs/architecture/testing/interop.md -- complete original native reload history.
package bgp

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/pcap"
	"github.com/ze-software/ze/internal/le/interoplab"
)

// finish MUST await the shared capture's joined terminal marker before decoding
// its file. prepareGoBGPReloadPeers MUST activate that capture before gobgpd.
func (proof *gobgpReloadProof) finish(ctx context.Context) error {
	var result error
	if err := proof.exec(ctx, "capture-stop", []string{"pkill", "-INT", "-x", cmdTcpdump}); err != nil {
		result = errors.Join(result, err)
	}
	if err := gobgpReloadCapture.waitCompleted(ctx, proof.check.Lab, peerGoBGP, "0", 5*time.Second); err != nil {
		result = errors.Join(result, err)
	}
	log, err := proof.query(ctx, "capture-log", peerGoBGP, []string{cmdCat, gobgpReloadCapture.log()})
	result = errors.Join(result, err)
	if !packetCaptureActivated(log) {
		result = errors.Join(result, errors.New("GoBGP capture never activated"))
	}
	packets, err := packetCaptureComplete(log, gobgpReloadCapture.packets)
	result = errors.Join(result, err)
	for _, path := range []string{gobgpReloadCapture.started(), gobgpReloadCapture.finished(), gobgpReloadCapture.version(), gobgpReloadCapture.exit()} {
		_, err := proof.query(ctx, path, peerGoBGP, []string{cmdCat, path})
		result = errors.Join(result, err)
	}
	text, err := proof.query(ctx, "capture-base64", peerGoBGP, []string{cmdBase64, gobgpReloadCapture.file()})
	result = errors.Join(result, err)
	if err != nil {
		return result
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(text))
	result = errors.Join(result, err)
	if err != nil {
		return result
	}
	result = errors.Join(result, os.WriteFile(filepath.Join(proof.directory, "wire.pcap"), raw, 0o600))
	digest := sha256.Sum256(raw)
	proof.evidence["capture-sha256"] = hex.EncodeToString(digest[:])
	if result != nil {
		return result
	}
	if proof.baseline.IsZero() {
		return errors.New("initial barrier did not establish a reload observation fence; native capture retained as setup evidence only")
	}
	return requireGoBGPReloadWire(raw, proof.check.Network, proof.baseline, packets)
}

func requireGoBGPReloadWire(raw []byte, network interoplab.Network, baseline time.Time, packets int) error {
	gobgp := netip.MustParseAddr(networkHostAddress(network, 5))
	// Reuse the native SYN/ACK/first-byte/complete-frame checks, not a second
	// permissive reassembler. GoBGP occupies the normalizer's local-speaker role.
	history, err := delayOpenHistory(raw, gobgp)
	if err != nil {
		return err
	}
	if len(history) != 4 {
		return fmt.Errorf("native GoBGP history has %d TCP connections, want four original sessions", len(history))
	}
	seen := make(map[netip.Addr]bool)
	count := 0
	for _, connection := range history {
		count += connection.Packets
		peer := connection.Flow.TargetAddr
		known := false
		for _, host := range []uint8{2, 4, 3, 15} {
			if peer == netip.MustParseAddr(networkHostAddress(network, host)) {
				known = true
			}
		}
		if !known {
			return fmt.Errorf("unknown peer %s in native GoBGP history", peer)
		}
		if seen[peer] {
			return errors.New("native GoBGP history replaced an original connection")
		}
		seen[peer] = true
		if !connection.Closed.IsZero() {
			return errors.New("native GoBGP original connection closed during proof")
		}
		for direction, messages := range [][]delayOpenMessage{connection.Ze, connection.FRR} {
			opens, keepalives := 0, 0
			for _, message := range messages {
				switch message.Type {
				case bgpOpen:
					opens++
				case bgpKeepalive:
					keepalives++
				case bgpUpdate:
					if direction != 1 {
						continue
					}
					switch peer {
					case netip.MustParseAddr(networkHostAddress(network, 2)), netip.MustParseAddr(networkHostAddress(network, 4)):
						if !message.First.Before(baseline) {
							return errors.New("source UPDATE arrived after the baseline fence")
						}
					}
				case 5:
					if !message.First.Before(baseline) {
						return errors.New("ROUTE-REFRESH arrived during reload window")
					}
				case bgpNotification:
					return errors.New("NOTIFICATION in original native reload history")
				default:
					return fmt.Errorf("unknown BGP message type %d", message.Type)
				}
			}
			if opens != 1 {
				return errors.New("native history did not retain exactly one original OPEN per direction")
			}
			if keepalives == 0 {
				return errors.New("native history omitted original session KEEPALIVE")
			}
		}
	}
	if count != packets {
		return fmt.Errorf("native pcap has %d packets, completed capture reports %d", count, packets)
	}
	streams, _, err := pcap.Reassemble(bytes.NewReader(raw), 179)
	if err != nil {
		return err
	}
	for index := range streams {
		stream := &streams[index]
		if err := requireGoBGPReloadFrames(stream, gobgp, network); err != nil {
			return err
		}
	}
	return nil
}

func requireGoBGPReloadFrames(stream *pcap.Stream, gobgp netip.Addr, network interoplab.Network) error {
	var buffer [65535]byte
	for offset := 0; offset < len(stream.Bytes); {
		length, err := readExtendedFrame(bytes.NewReader(stream.Bytes[offset:]), buffer[:])
		if err != nil {
			return err
		}
		frame := buffer[:length]
		if frame[18] == bgpOpen {
			if err := requireGoBGPReloadOpen(frame, stream.Flow.SourceAddr, gobgp, network); err != nil {
				return err
			}
		}
		if frame[18] == bgpUpdate {
			update, err := parsedEmptyMPUpdate(frame[19:])
			if err != nil {
				return err
			}
			if stream.Flow.SourceAddr == gobgp {
				for _, receiver := range []uint8{3, 15} {
					if stream.Flow.TargetAddr != netip.MustParseAddr(networkHostAddress(network, receiver)) {
						continue
					}
					withdrawn, err := gobgpReloadIPv4Prefixes(update.withdrawn)
					if err != nil {
						return err
					}
					for _, prefix := range withdrawn {
						if prefix == netip.MustParsePrefix(gobgpReloadPrefixes[7]) {
							return errors.New("GoBGP emitted a withdrawal for a never-sent destination")
						}
					}
					announced, err := gobgpReloadIPv4Prefixes(update.nlri)
					if err != nil {
						return err
					}
					for _, prefix := range announced {
						if prefix == netip.MustParsePrefix(gobgpReloadPrefixes[7]) {
							return errors.New("GoBGP advertised the never-sent control")
						}
					}
				}
			}
		}
		offset += length
	}
	return nil
}

func requireGoBGPReloadOpen(frame []byte, source, gobgp netip.Addr, network interoplab.Network) error {
	if len(frame) < 29 {
		return errors.New("short original GoBGP reload OPEN")
	}
	if frame[18] != bgpOpen {
		return errors.New("original identity frame is not an OPEN")
	}
	if frame[19] != 4 {
		return errors.New("original OPEN is not BGP version four")
	}
	if int(frame[28])+29 != len(frame) {
		return errors.New("invalid original OPEN option length")
	}
	asn := uint16(65005)
	if source != gobgp {
		found := false
		for _, peer := range []struct {
			host uint8
			asn  uint16
		}{{2, 65001}, {4, 65004}, {3, 65002}, {15, 65015}} {
			if source == netip.MustParseAddr(networkHostAddress(network, peer.host)) {
				asn = peer.asn
				found = true
			}
		}
		if !found {
			return errors.New("unknown original OPEN sender")
		}
	}
	if binary.BigEndian.Uint16(frame[20:22]) != asn {
		return errors.New("original OPEN AS differs from native speaker identity")
	}
	if netip.AddrFrom4([4]byte(frame[24:28])) != source {
		return errors.New("original OPEN router ID differs from native speaker identity")
	}
	for options := frame[29:]; len(options) > 0; {
		if len(options) < 2 {
			return errors.New("truncated original OPEN parameter")
		}
		length := int(options[1])
		if length+2 > len(options) {
			return errors.New("truncated original OPEN parameter value")
		}
		if options[0] == 2 {
			for capabilities := options[2 : 2+length]; len(capabilities) > 0; {
				if len(capabilities) < 2 {
					return errors.New("truncated original OPEN capability")
				}
				length := int(capabilities[1])
				if length+2 > len(capabilities) {
					return errors.New("truncated original OPEN capability value")
				}
				// A one-sided peer capability does not negotiate ADD-PATH.
				// GoBGP's original OPEN must offer none in either direction.
				if capabilities[0] == 69 {
					if source == gobgp {
						return errors.New("GoBGP offered ADD-PATH outside bounded successor scope")
					}
				}
				if capabilities[0] == 65 {
					if length != 4 {
						return errors.New("invalid original four-octet AS capability")
					}
					if binary.BigEndian.Uint32(capabilities[2:6]) != uint32(asn) {
						return errors.New("original four-octet AS capability changes native identity")
					}
				}
				capabilities = capabilities[2+length:]
			}
		}
		options = options[2+length:]
	}
	return nil
}

func gobgpReloadIPv4Prefixes(raw []byte) ([]netip.Prefix, error) {
	var prefixes []netip.Prefix
	for len(raw) > 0 {
		bits := int(raw[0])
		if bits > 32 {
			return nil, errors.New("non-IPv4 prefix in bounded successor UPDATE")
		}
		octets := (bits + 7) / 8
		if len(raw) < octets+1 {
			return nil, errors.New("truncated IPv4 prefix in successor UPDATE")
		}
		var address [4]byte
		copy(address[:], raw[1:1+octets])
		prefix := netip.PrefixFrom(netip.AddrFrom4(address), bits)
		if prefix != prefix.Masked() {
			return nil, errors.New("noncanonical IPv4 prefix in successor UPDATE")
		}
		prefixes = append(prefixes, prefix)
		raw = raw[1+octets:]
	}
	return prefixes, nil
}
