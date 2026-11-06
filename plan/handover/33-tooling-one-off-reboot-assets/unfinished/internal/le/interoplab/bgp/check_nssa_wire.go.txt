// Design: docs/architecture/testing/interop.md -- NSSA first-epoch wire proof.
package bgp

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/pcap"
	"github.com/ze-software/ze/internal/le/interoplab"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// nssaFirstEpochProof adopts the capture armed by prepareScenario, never starts
// a replacement exchange. A successful adoption MUST be paired with finish;
// finish MUST await the entire joined observation window before judging absence.
type nssaFirstEpochProof struct {
	check *interoplab.CheckContext
	directory string
}

func adoptNSSAFirstEpochCapture(_ context.Context, check *interoplab.CheckContext) (*nssaFirstEpochProof, error) {
	if check.Source.Name != "ospf-stub-nssa-frr" { return nil, errors.New("NSSA first-epoch proof needs its original scenario") }
	if !check.Network.IPv4.IsValid() { return nil, errors.New("NSSA first-epoch proof has no selected network") }
	if !check.Network.IPv4.Addr().Is4() { return nil, errors.New("NSSA first-epoch proof needs an IPv4 topology") }
	directory, err := retainedProofDirectory(check)
	if err != nil { return nil, err }
	return &nssaFirstEpochProof{check: check, directory: directory}, nil
}

func (proof *nssaFirstEpochProof) finish(ctx context.Context, nativeErr error) (resultErr error) {
	check, capture := proof.check, nssaPacketCapture
	// This is an intentional finite observation window, not a readiness sleep.
	// Cancellation or a forced kill cannot shorten it into a negative verdict.
	resultErr = errors.Join(nativeErr, capture.waitCompleted(ctx, check.Lab, "ze", "124", 125*time.Second))
	diagnostic, cancel := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
	defer cancel()
	evidence := make(map[string]string)
	query := func(key, peer string, command []string) string {
		text, err := check.Lab.Query(diagnostic, peer, command, nil)
		evidence[key] = text
		if err != nil { evidence[key+" error"] = err.Error(); resultErr = errors.Join(resultErr, err) }
		return text
	}
	log := query("capture-log", "ze", []string{cmdCat, capture.log()})
	if !packetCaptureActivated(log) { resultErr = errors.Join(resultErr, errors.New("NSSA capture was not activated on eth0")) }
	captured, err := packetCaptureComplete(log, nssaPacketCapture.packets)
	resultErr = errors.Join(resultErr, err)
	started, startErr := time.Parse(time.RFC3339Nano, strings.TrimSpace(query("capture-started", "ze", []string{cmdCat, capture.started()})))
	finished, endErr := time.Parse(time.RFC3339Nano, strings.TrimSpace(query("capture-finished", "ze", []string{cmdCat, capture.finished()})))
	resultErr = errors.Join(resultErr, startErr, endErr)
	if startErr == nil && endErr == nil && (finished.Sub(started) < 120*time.Second || finished.Sub(started) > 126*time.Second || finished.After(time.Now())) {
		resultErr = errors.Join(resultErr, fmt.Errorf("NSSA capture has invalid bounded epoch %s through %s", started, finished))
	}
	version := query("timeout-version", "ze", []string{cmdCat, capture.version()})
	firstLine, _, _ := strings.Cut(version, "\n")
	if firstLine != "timeout (GNU coreutils) 9.11" { resultErr = errors.Join(resultErr, errors.New("NSSA capture did not use the selected GNU timeout 9.11")) }
	packageVersion := query("timeout-package", "ze", []string{"apk", "info", "-e", "-v", "coreutils=9.11-r0"})
	if strings.TrimSpace(packageVersion) != "coreutils-9.11-r0" { resultErr = errors.Join(resultErr, errors.New("NSSA capture has the wrong coreutils package")) }
	for _, binary := range []struct{ key, path string }{{"timeout-binary-sha256", "/bin/coreutils"}, {"ze-binary-sha256", "/usr/local/bin/ze"}} {
		text := query(binary.key, "ze", []string{"sha256sum", binary.path})
		fields := strings.Fields(text)
		if len(fields) != 2 || fields[1] != binary.path || len(fields[0]) != 64 {
			resultErr = errors.Join(resultErr, fmt.Errorf("invalid native SHA256 attribution for %s", binary.path))
		} else if _, err := hex.DecodeString(fields[0]); err != nil { resultErr = errors.Join(resultErr, err) }
	}
	zeMAC := query("ze-eth0-mac", "ze", []string{cmdCat, "/sys/class/net/eth0/address"})
	frrMAC := query("frr-eth0-mac", peerFRR, []string{cmdCat, "/sys/class/net/eth0/address"})
	for _, source := range []struct{ key, peer, path string }{{"rendered-ze.conf", "ze", zeMountedConfig}, {"rendered-frr.conf", peerFRR, "/etc/frr/frr.conf"}} {
		text := query(source.key, source.peer, []string{cmdCat, source.path})
		digest := sha256.Sum256([]byte(text))
		evidence[source.key+" sha256"] = hex.EncodeToString(digest[:])
	}
	query("native-frr-version", peerFRR, []string{cmdVtysh, "-c", frrShowVersion})
	encoded, readErr := check.Lab.Query(diagnostic, "ze", []string{"base64", capture.file()}, nil)
	resultErr = errors.Join(resultErr, readErr)
	if readErr != nil { evidence["capture-read error"] = readErr.Error() }
	raw, decodeErr := base64.StdEncoding.DecodeString(strings.TrimSpace(encoded))
	resultErr = errors.Join(resultErr, decodeErr)
	var observed nssaWireObservation
	if decodeErr == nil {
		digest := sha256.Sum256(raw)
		evidence["capture-sha256"] = hex.EncodeToString(digest[:])
		resultErr = errors.Join(resultErr, os.WriteFile(filepath.Join(proof.directory, "wire.pcap"), raw, 0o600))
		if startErr == nil && endErr == nil {
			observed, err = observeNSSAFirstEpoch(raw, check.Network, zeMAC, frrMAC, started, finished)
			resultErr = errors.Join(resultErr, err)
			if err == nil && observed.Packets != captured { resultErr = errors.Join(resultErr, fmt.Errorf("NSSA pcap has %d records, tcpdump captured %d", observed.Packets, captured)) }
		}
	}
	// The original session scratch and actual network identify this current proof.
	// It neither reconstructs missing historical originals nor proves a VM/kernel run.
	verdict := "passed"
	if resultErr != nil { verdict = resultErr.Error() }
	data, marshalErr := json.MarshalIndent(map[string]any{"scenario": check.Source.Name, "network": check.Network, "verdict": verdict, "first-epoch": observed, "evidence": evidence}, "", "  ")
	resultErr = errors.Join(resultErr, marshalErr)
	if marshalErr == nil { resultErr = errors.Join(resultErr, os.WriteFile(filepath.Join(proof.directory, "proof.json"), append(data, '\n'), 0o600)) }
	return resultErr
}

type nssaWireObservation struct {
	Packets int `json:"packets"`
	Hello [2]bool `json:"hello"`
	InitialExchange [2]bool `json:"initial-exchange"`
	RouterControl bool `json:"router-control"`
	Default bool `json:"type7-default"`
}

func observeNSSAFirstEpoch(raw []byte, network interoplab.Network, zeText, frrText string, started, finished time.Time) (nssaWireObservation, error) {
	var observed nssaWireObservation
	if !network.IPv4.IsValid() { return observed, errors.New("NSSA observer has no selected network") }
	if !network.IPv4.Addr().Is4() { return observed, errors.New("NSSA observer needs an IPv4 topology") }
	var macs [2][6]byte
	for index, text := range []string{zeText, frrText} {
		mac, err := net.ParseMAC(strings.TrimSpace(text))
		if err != nil { return observed, fmt.Errorf("NSSA observer MAC: %w", err) }
		if len(mac) != 6 { return observed, fmt.Errorf("NSSA observer has invalid eth0 MAC %q", text) }
		copy(macs[index][:], mac)
	}
	if macs[0] == macs[1] { return observed, errors.New("NSSA topology gives both peers the same MAC") }
	var addresses [2]netip.Addr
	for index, host := range []uint8{2, 3} {
		address, err := netip.ParseAddr(networkHostAddress(network, host))
		if err != nil { return observed, err }
		if !address.Is4() { return observed, errors.New("NSSA observer has no selected IPv4 peer address") }
		addresses[index] = address
	}
	reader, err := pcap.NewReader(bytes.NewReader(raw))
	if err != nil { return observed, err }
	if reader.LinkType() != pcap.LinkTypeEthernet { return observed, errors.New("NSSA capture is not Ethernet") }
	var record pcap.Record
	previous := started
	for {
		if err := reader.Next(&record); err != nil {
			if errors.Is(err, io.EOF) { break }
			return observed, err
		}
		observed.Packets++
		if observed.Packets >= nssaPacketCapture.packets { return observed, errors.New("NSSA capture reached its packet bound") }
		if len(record.Data) != record.OriginalLen { return observed, errors.New("NSSA capture is truncated") }
		if record.Timestamp.Before(previous) { return observed, errors.New("NSSA capture timestamps are unordered") }
		if record.Timestamp.Before(started) { return observed, errors.New("NSSA packet predates this capture epoch") }
		if record.Timestamp.After(finished) { return observed, errors.New("NSSA packet follows this capture epoch") }
		previous = record.Timestamp
		p, direction, err := nssaWirePacket(record.Data, addresses, macs)
		if err != nil { return observed, fmt.Errorf("NSSA record %d: %w", observed.Packets, err) }
		if p.Hello != nil {
			if !p.Hello.Options.Has(types.OptionNP) { return observed, errors.New("NSSA Hello lacks NSSA capability") }
			if p.Hello.Options.Has(types.OptionE) { return observed, errors.New("NSSA Hello advertises external capability") }
			observed.Hello[direction] = true
		}
		if p.DBDesc != nil && p.DBDesc.Flags&4 != 0 {
			if !observed.Hello[direction] { return observed, errors.New("NSSA initial exchange predates its source Hello") }
			observed.InitialExchange[direction] = true
		}
		if p.LSUpdate == nil { continue }
		for _, lsa := range p.LSUpdate.LSAs {
			if !lsa.VerifyChecksum() { return observed, errors.New("NSSA capture has an invalid LSA checksum") }
			var router packet.RouterLSA
			var external packet.ExternalLSA
			// The v2 codec admits only these base and opaque LS types; v3's
			// distinct scope-typed values are outside this capture's grammar.
			//exhaustive:ignore
			switch lsa.Header.Type {
			case types.LSTypeRouter:
				router, err = lsa.DecodeRouter()
			case types.LSTypeNetwork:
				_, err = lsa.DecodeNetwork()
			case types.LSTypeSummaryNetwork, types.LSTypeSummaryASBR:
				_, err = lsa.DecodeSummary()
			case types.LSTypeASExternal, types.LSTypeNSSA:
				external, err = lsa.DecodeExternal()
			case types.LSTypeOpaqueLink, types.LSTypeOpaqueArea, types.LSTypeOpaqueAS:
				// Opaque extension bodies are not base LSA bodies.
			default:
				return observed, errors.New("NSSA capture has an unrecognized LSA type")
			}
			if err != nil { return observed, err }
			if direction != 0 { continue }
			if lsa.Header.Type == types.LSTypeASExternal { return observed, fmt.Errorf("Ze flooded Type 5 into NSSA: %+v", lsa.Header) }
			ze, foreign := addresses[0].As4(), addresses[1].As4()
			if lsa.Header.AdvertisingRouter != types.RouterID(ze) { continue }
			if lsa.Header.Age.IsMaxAge() { continue }
			if lsa.Header.Type == types.LSTypeNSSA && lsa.Header.LinkStateID == (types.LinkStateID{}) {
				if external.NetworkMask == ([4]byte{}) && external.Metric < packet.ExternalMetricMax { observed.Default = true }
			}
			if lsa.Header.Type == types.LSTypeRouter && lsa.Header.LinkStateID == types.LinkStateID(ze) {
				for _, link := range router.Links {
					if router.Flags&packet.RouterFlagB != 0 && link.Type == packet.RouterLinkTypeP2P && link.LinkID == types.LinkStateID(foreign) && link.LinkData == ze && link.Metric == 10 { observed.RouterControl = true }
				}
			}
		}
	}
	for _, control := range [...]bool{observed.Hello[0], observed.Hello[1], observed.InitialExchange[0], observed.InitialExchange[1], observed.RouterControl, observed.Default} {
		if !control { return observed, fmt.Errorf("NSSA first epoch lacks positive Hello/exchange/router/Type7 default controls: %+v", observed) }
	}
	return observed, nil
}

// nssaWirePacket dissects only the Ethernet/IPv4 envelope. OSPF framing, body
// decoding and checksums remain the existing production packet codec's job.
func nssaWirePacket(raw []byte, addresses [2]netip.Addr, macs [2][6]byte) (packet.Packet, int, error) {
	if len(raw) < 34 { return packet.Packet{}, 0, errors.New("NSSA Ethernet IPv4 frame is truncated") }
	if binary.BigEndian.Uint16(raw[12:14]) != 0x0800 { return packet.Packet{}, 0, errors.New("NSSA Ethernet frame is not IPv4") }
	ip := raw[14:]
	header := int(ip[0]&15)*4
	length := int(binary.BigEndian.Uint16(ip[2:4]))
	if ip[0]>>4 != 4 { return packet.Packet{}, 0, errors.New("NSSA IP version is not IPv4") }
	if header < 20 { return packet.Packet{}, 0, errors.New("NSSA IPv4 header length is invalid") }
	if header > len(ip) { return packet.Packet{}, 0, errors.New("NSSA IPv4 header is truncated") }
	if length != len(ip) { return packet.Packet{}, 0, errors.New("NSSA IPv4 packet length mismatches capture") }
	if length < header { return packet.Packet{}, 0, errors.New("NSSA IPv4 length excludes its header") }
	if ip[9] != 89 { return packet.Packet{}, 0, errors.New("NSSA IPv4 packet is not OSPF") }
	if binary.BigEndian.Uint16(ip[6:8])&0x3fff != 0 { return packet.Packet{}, 0, errors.New("NSSA IPv4 packet is fragmented") }
	if !types.InternetChecksumPairValid(ip[:header], nil) { return packet.Packet{}, 0, errors.New("NSSA IPv4 checksum is invalid") }
	source := netip.AddrFrom4([4]byte(ip[12:16]))
	target := netip.AddrFrom4([4]byte(ip[16:20]))
	direction := -1
	for index, address := range addresses { if source == address { direction = index } }
	if direction < 0 { return packet.Packet{}, 0, errors.New("NSSA source IP belongs to another topology") }
	if [6]byte(raw[6:12]) != macs[direction] { return packet.Packet{}, 0, errors.New("NSSA source MAC belongs to another topology") }
	multicast := netip.AddrFrom4([4]byte{224, 0, 0, 5})
	if target == multicast {
		if [6]byte(raw[:6]) != ([6]byte{1, 0, 0x5e, 0, 0, 5}) { return packet.Packet{}, 0, errors.New("NSSA multicast target MAC mismatch") }
	} else {
		if target != addresses[1-direction] { return packet.Packet{}, 0, errors.New("NSSA target IP belongs to another topology") }
		if [6]byte(raw[:6]) != macs[1-direction] { return packet.Packet{}, 0, errors.New("NSSA target MAC belongs to another topology") }
	}
	p, err := packet.DecodePacket(ip[header:])
	if err != nil { return packet.Packet{}, 0, err }
	if int(p.Header.Length) != len(ip)-header { return packet.Packet{}, 0, errors.New("NSSA OSPF framing excludes captured bytes") }
	if p.Header.InstanceID != 0 { return packet.Packet{}, 0, errors.New("NSSA OSPF instance is not zero") }
	if p.Header.AuType != packet.AuTypeNull { return packet.Packet{}, 0, errors.New("NSSA OSPF authentication differs from fixture") }
	if p.Header.AreaID != (types.AreaID{0, 0, 0, 2}) { return packet.Packet{}, 0, errors.New("NSSA OSPF area belongs to another topology") }
	if p.Header.RouterID != types.RouterID(source.As4()) { return packet.Packet{}, 0, errors.New("NSSA OSPF router belongs to another topology") }
	if !p.VerifyChecksum() { return packet.Packet{}, 0, errors.New("NSSA OSPF checksum is invalid") }
	return p, direction, nil
}
