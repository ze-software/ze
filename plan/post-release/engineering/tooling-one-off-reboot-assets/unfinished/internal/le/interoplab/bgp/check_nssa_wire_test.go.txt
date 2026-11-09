// Design: docs/architecture/testing/interop.md -- first-epoch NSSA wire proof.
package bgp

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/pcap"
	"github.com/ze-software/ze/internal/le/interoplab"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// These are checker regressions, not native FRR, kernel, VM or conformance proof.
// The normalized FRR database remains empty even when Ze's wire leaks Type 5.
func TestNSSADefaultRequiresFirstEpochWire(t *testing.T) {
	t.Setenv("CLAUDE_CODE_SESSION_ID", "nssa-wire-regression")
	for _, name := range []string{
		"clean", "type5 leak", "missing capture", "malformed capture", "truncated capture",
		"wrong interface", "wrong source", "wrong source MAC", "wrong area", "wrong router",
		"missing default", "maxage default", "wrong default mask", "missing router control",
		"missing hello", "missing initial exchange", "dropped packets", "count mismatch", "missing completion",
	} {
		t.Run(name, func(t *testing.T) {
			lab := &nssaWireRegressionLab{mode: name, at: time.Now()}
			ctx, cancel := context.WithTimeout(t.Context(), 300*time.Millisecond)
			defer cancel()
			err := checkNSSADefault(ctx, &interoplab.CheckContext{
				Source:  interoplab.ScenarioSource{Name: "ospf-stub-nssa-frr"},
				Network: interoplab.Network{IPv4: netip.MustParsePrefix("172.30.0.0/24")},
				Lab:     lab, Checkout: t.TempDir(),
			})
			if name == "clean" {
				if err != nil {
					t.Fatal(err)
				}
				if lab.wireReads == 0 {
					t.Fatal("NSSA checker accepted an empty FRR database without reading actual Ze wire")
				}
			} else if err == nil {
				t.Fatalf("NSSA checker accepted %s while FRR's receiver database stayed empty", name)
			}
		})
	}
}

type nssaWireRegressionLab struct {
	noEvidenceLab
	mode      string
	wireReads int
	at time.Time
}

func (lab *nssaWireRegressionLab) Query(_ context.Context, peer string, command []string, _ []interoplab.EnvironmentVariable) (string, error) {
	text := strings.Join(command, " ")
	switch {
	case strings.Contains(text, "show ip ospf neighbor"):
		return "Neighbor ID Pri State Dead Time Address Interface\n172.30.0.2 1 Full/- 00:00:39 172.30.0.2 eth0:172.30.0.3", nil
	case strings.Contains(text, "show ip route ospf"):
		return `{"0.0.0.0/0":[{"prefix":"0.0.0.0/0","protocol":"ospf","selected":true,"destSelected":true,"installed":true,"nexthops":[{"ip":"172.30.0.2","active":true,"fib":true}]}]}`, nil
	case strings.Contains(text, "show ip ospf database external"):
		return "OSPF Router with ID (172.30.0.3)", nil
	case strings.Contains(text, "/sys/class/net/eth0/address"):
		if peer == peerFRR {
			return "02:00:00:00:00:03\n", nil
		}
		return "02:00:00:00:00:02\n", nil
	case strings.Contains(text, "/tmp/ospf-nssa-capture.exit"):
		if lab.mode == "missing completion" {
			return "137\n", nil
		}
		return "124\n", nil
	case text == "cat /tmp/ospf-nssa-capture.started":
		return lab.at.Add(-120*time.Second).Format(time.RFC3339Nano), nil
	case text == "cat /tmp/ospf-nssa-capture.finished":
		return lab.at.Format(time.RFC3339Nano), nil
	case text == "cat /tmp/ospf-nssa-capture.version":
		return "timeout (GNU coreutils) 9.11\n", nil
	case text == "apk info -e -v coreutils=9.11-r0":
		return "coreutils-9.11-r0\n", nil
	case strings.Contains(text, "/tmp/ospf-nssa-capture.log"):
		count := len(nssaRegressionPackets(lab.mode))
		if lab.mode == "count mismatch" {
			count++
		}
		iface, dropped := "eth0", 0
		if lab.mode == "wrong interface" {
			iface = "eth1"
		}
		if lab.mode == "dropped packets" {
			dropped = 1
		}
		return fmt.Sprintf("tcpdump: listening on %s, link-type EN10MB (Ethernet), snapshot length 65535 bytes\n%d packets captured\n%d packets received by filter\n%d packets dropped by kernel\n", iface, count, count, dropped), nil
	case strings.Contains(text, "base64 /tmp/ospf-nssa.pcap"):
		lab.wireReads++
		if lab.mode == "missing capture" {
			return "", nil
		}
		if lab.mode == "malformed capture" {
			return "not-base64!", nil
		}
		var out bytes.Buffer
		if err := pcap.WriteFileHeader(&out, 65535, pcap.LinkTypeEthernet); err != nil {
			return "", err
		}
		for _, raw := range nssaRegressionPackets(lab.mode) {
			if err := pcap.WriteRecord(&out, lab.at, raw, len(raw)); err != nil {
				return "", err
			}
		}
		data := out.Bytes()
		if lab.mode == "truncated capture" {
			data = data[:len(data)-1]
		}
		return base64.StdEncoding.EncodeToString(data), nil
	case text == "sha256sum /bin/coreutils":
		return strings.Repeat("0", 64) + "  /bin/coreutils\n", nil
	case text == "sha256sum /usr/local/bin/ze":
		return strings.Repeat("1", 64) + "  /usr/local/bin/ze\n", nil
	case strings.Contains(text, "show version"):
		return "FRRouting 10.7.1\n", nil
	case strings.Contains(text, "cat "):
		return "rendered-source-attribution-fixture", nil
	default:
		return "", fmt.Errorf("unexpected NSSA query %s %s", peer, text)
	}
}

func (lab *nssaWireRegressionLab) Exec(context.Context, string, []string, []interoplab.EnvironmentVariable) (interoplab.CommandResult, error) {
	return interoplab.CommandResult{}, nil
}

func (lab *nssaWireRegressionLab) Logs(context.Context, string, int) (interoplab.LogResult, error) {
	return interoplab.LogResult{}, nil
}

func nssaRegressionPackets(mode string) [][]byte {
	ze, frr := types.RouterID{172, 30, 0, 2}, types.RouterID{172, 30, 0, 3}
	area := types.AreaID{0, 0, 0, 2}
	if mode == "wrong area" {
		area = types.AreaID{0, 0, 0, 3}
	}
	frame := func(from types.RouterID, p packet.Packet) []byte {
		p.Header.RouterID, p.Header.AreaID = from, area
		if mode == "wrong router" && from == ze {
			p.Header.RouterID = types.RouterID{172, 30, 0, 99}
		}
		raw := make([]byte, 14+20+p.EncodedLen())
		copy(raw[:6], []byte{1, 0, 0x5e, 0, 0, 5})
		copy(raw[6:12], []byte{2, 0, 0, 0, 0, from[3]})
		if mode == "wrong source MAC" && from == ze {
			raw[11] = 99
		}
		binary.BigEndian.PutUint16(raw[12:14], 0x0800)
		ip := raw[14:]
		ip[0], ip[8], ip[9] = 0x45, 1, 89
		binary.BigEndian.PutUint16(ip[2:4], uint16(len(ip)))
		copy(ip[12:16], from[:])
		copy(ip[16:20], []byte{224, 0, 0, 5})
		if mode == "wrong source" && from == ze {
			ip[15] = 99
		}
		binary.BigEndian.PutUint16(ip[10:12], types.InternetChecksumPair(ip[:20], nil))
		p.WriteTo(ip, 20)
		return raw
	}
	var frames [][]byte
	if mode != "missing hello" {
		for _, id := range []types.RouterID{ze, frr} {
			frames = append(frames, frame(id, packet.Packet{Hello: &packet.Hello{Options: types.OptionNP, HelloInterval: 10, DeadInterval: 40}}))
		}
	}
	if mode != "missing initial exchange" {
		for _, id := range []types.RouterID{ze, frr} {
			frames = append(frames, frame(id, packet.Packet{DBDesc: &packet.DBDesc{Options: types.OptionNP, InterfaceMTU: 1500, Flags: 7, DDSequence: 42}}))
		}
	}
	lsas := []packet.LSA{}
	if mode != "missing router control" {
		lsas = append(lsas, packet.LSA{Header: packet.LSAHeader{Type: types.LSTypeRouter, LinkStateID: types.LinkStateID(ze), AdvertisingRouter: ze, Sequence: types.InitialSequenceNumber}, Router: &packet.RouterLSA{Flags: packet.RouterFlagB, Links: []packet.RouterLink{{LinkID: types.LinkStateID(frr), LinkData: [4]byte(ze), Type: packet.RouterLinkTypeP2P, Metric: 10}}}})
	}
	if mode != "missing default" {
		lsa := packet.LSA{Header: packet.LSAHeader{Type: types.LSTypeNSSA, AdvertisingRouter: ze, Sequence: types.LSSequenceNumber(0x80000001)}, External: &packet.ExternalLSA{Metric: 1}}
		if mode == "maxage default" {
			lsa.Header.Age = 3600
		}
		if mode == "wrong default mask" {
			lsa.External.NetworkMask = [4]byte{255, 255, 255, 0}
		}
		lsas = append(lsas, lsa)
		if mode == "type5 leak" {
			lsa.Header.Type = types.LSTypeASExternal
			lsas = append(lsas, lsa)
		}
	}
	frames = append(frames, frame(ze, packet.Packet{LSUpdate: &packet.LSUpdate{LSAs: lsas}}))
	return frames
}
