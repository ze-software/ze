// Design: docs/architecture/testing/interop.md -- the cleartext probe of the real VPP ipsec scenario
// Overview: vppevidencerun.go -- runIPsec, which runs this probe after its own checks
// Related: vppevidencempls.go -- the packet-generator pcap pattern and the trace readers reused here
//
// The cleartext probe MEASURES what VPP does with an unprotected packet that
// arrives on an interface whose SPD holds inbound PROTECT, DISCARD and BYPASS
// entries. RFC 4301 Section 5.2 step 3b: "If there is a match and the packet is
// to be discarded or bypassed, do so." and "If there is no match, discard the
// traffic." A packet whose selectors match a PROTECT entry has no SPD-I match.
//
// It is a probe, not a proof: each check passes when VPP's answer was read
// unambiguously, and its detail states what VPP did beside what the RFC requires.
// A check fails only when the measurement itself did not happen: the generator
// sent nothing, or VPP's IPsec input node did not count the frame.
package testdeployment

import (
	"encoding/binary"
	"errors"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/pcap"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/interoplab"
)

// The probe interface and the frames' addresses. The selectors are twins of the
// realVPPCleartext constants in
// internal/component/ike/dataplane/vpp_real_cleartext_integration_test.go, which
// installs them: change both together. pg0 is not used because mpls-label-pop
// creates it later in the same container.
const (
	vppCleartextInterface    = "pg1"
	vppCleartextInterfaceMAC = "02:fe:00:00:45:01"
	vppCleartextAddress      = vppCleartextHost + "/24"
	// vppCleartextHost is the probe interface's own address, the destination of
	// every frame aimed at it.
	vppCleartextHost         = "100.64.45.1"
	vppCleartextReportPrefix = "ze-vpp-cleartext:"
	vppCleartextProbeTest    = "^TestVPPRealCleartextProbe$"
	// vppCleartextESPSPI is an SPI no installed SA holds, so an ESP frame carrying
	// it can only be judged by the DISCARD and BYPASS entries.
	vppCleartextESPSPI = 0x0000beef
)

// The node and the error reasons of VPP's IPv4 IPsec input, as
// foreach_ipsec_input_error spells them (src/vnet/ipsec/ipsec_input.c, FDio/vpp
// v26.06). The IPv4 loop adds every frame it did not hand to an ESP or AH lookup
// to "IPSec policy not matched", the cleartext it passes included, so that reason
// alone never says whether a frame was dropped: the trace does.
const (
	vppIPsecInputNode     = "ipsec4-input-feature"
	vppIPsecInputReceived = "IPSec pkts received"
	vppIPsecInputMatch    = "IPSec policy match"
	vppIPsecInputNoMatch  = "IPSec policy not matched"
	vppIPsecInputBypass   = "IPSec policy bypass" //nolint:gosec // a VPP error reason, not a credential
	vppIPsecInputDiscard  = "IPSec policy discard"
)

// vppCleartextProbeFrame is one frame of the probe and what it measures.
type vppCleartextProbeFrame struct {
	check  string
	source string
	target string
	esp    bool
	// entry names the SPD entry whose selectors the frame matches.
	entry string
	// owed is what RFC 4301 Section 5.2 requires of that frame.
	owed string
}

var vppCleartextProbeFrames = []vppCleartextProbeFrame{
	{check: "probe-cleartext-protect", source: "100.64.46.10", target: vppCleartextHost,
		entry: "the inbound PROTECT entry 100.64.46.0/24",
		owed:  "RFC 4301 Section 5.2 step 3b requires it discarded: it has no SPD-I match"},
	{check: "probe-cleartext-discard", source: "100.64.47.10", target: vppCleartextHost,
		entry: "the inbound DISCARD entry 100.64.47.0/24",
		owed:  "RFC 4301 Section 5.2 step 3b requires it discarded"},
	{check: "probe-cleartext-bypass", source: "100.64.48.10", target: vppCleartextHost,
		entry: "the inbound BYPASS entry 100.64.48.0/24",
		owed:  "RFC 4301 Section 5.2 step 3b requires it passed"},
	// The two ESP frames tell which address pair VPP compares the DISCARD entry
	// against: the first in the orientation the IKE engine writes an inbound
	// selector (source in the remote range), the second with the pair exchanged.
	{check: "probe-esp-discard", source: "100.64.47.10", target: vppCleartextHost, esp: true,
		entry: "the inbound DISCARD entry 100.64.47.0/24, source in its remote range",
		owed:  "the DISCARD entry is owed the match"},
	{check: "probe-esp-discard-reversed", source: "100.64.45.10", target: "100.64.47.1", esp: true,
		entry: "the inbound DISCARD entry 100.64.47.0/24 only with source and destination exchanged",
		owed:  "the DISCARD entry is owed no match"},
}

// vppTraceFate is what VPP's trace shows became of one probe frame.
type vppTraceFate uint8

const (
	vppTraceFateUnspecified vppTraceFate = iota
	vppTraceFateForwarded
	vppTraceFateDropped
)

// runCleartextProbe creates the probe interface, has the IKE dataplane backend
// install the probe's inbound SPD on it, then sends each probe frame and reads
// VPP's verdict on it. A query failure is an operating error.
func (v *VPP) runCleartextProbe(container, work string) ([]VPPCheck, error) {
	index, err := v.createCleartextInterface(container)
	if err != nil {
		return nil, err
	}
	var tb textbuf.Buffer
	probe := tb.Str("/src/").Str(filepath.ToSlash(vppIPsecProbeRel(v.Goarch))).String()
	output, ok := v.containerTextWith(container, []interoplab.EnvironmentVariable{
		{Name: "ZE_VPP_IPSEC_API_SOCKET", Value: "/run/vpp/api.sock"},
		{Name: "ZE_VPP_IPSEC_SW_IF_INDEX", Value: strconv.Itoa(index)},
	}, probe, "-test.run", vppCleartextProbeTest, "-test.v")
	if !ok {
		return []VPPCheck{failVPPCheck("probe-install",
			"the IKE dataplane backend could not install the cleartext probe's inbound SPD", output, nil)}, nil
	}
	spdID, err := parseCleartextProbe(output)
	if err != nil {
		return []VPPCheck{failVPPCheck("probe-install", err.Error(), output, nil)}, nil
	}
	all, err := v.query(container, "show ipsec all")
	if err != nil {
		return nil, err
	}
	if reason := cleartextSPDMissing(all, spdID); reason != "" {
		return []VPPCheck{failVPPCheck("probe-install", reason, all, nil)}, nil
	}
	checks := []VPPCheck{passVPPCheck("probe-install",
		tbDetail("real VPP holds SPD ", spdID, " bound to ", vppCleartextInterface,
			" with an inbound PROTECT, DISCARD and BYPASS entry"))}

	for _, frame := range vppCleartextProbeFrames {
		check, err := v.traceCleartextFrame(container, work, frame)
		if err != nil {
			return nil, err
		}
		checks = append(checks, check)
		if check.Verdict != VPPProofPass {
			return checks, nil
		}
	}
	// The per-entry packet counters, as evidence beside the node's own counts.
	spd, err := v.query(container, "show ipsec spd")
	if err != nil {
		return nil, err
	}
	checks[0].Evidence = tailLines(spd)
	return checks, nil
}

// createCleartextInterface adds the probe's packet-generator interface with an
// address, so ip4-input accepts its frames and the local one is delivered, and
// answers its VPP interface index.
func (v *VPP) createCleartextInterface(container string) (int, error) {
	for _, command := range []string{
		vppCommand("create packet-generator interface", vppCleartextInterface, "hw-addr", vppCleartextInterfaceMAC),
		vppCommand("set interface ip address", vppCleartextInterface, vppCleartextAddress),
		vppCommand("set interface state", vppCleartextInterface, "up"),
	} {
		if _, err := v.query(container, command); err != nil {
			return 0, err
		}
	}
	interfaces, err := v.query(container, "show interface")
	if err != nil {
		return 0, err
	}
	return parseVPPInterfaceIndex(interfaces, vppCleartextInterface)
}

// parseCleartextProbe answers the SPD id the probe test reported.
func parseCleartextProbe(output string) (string, error) {
	if strings.Contains(output, "SKIP") {
		return "", errors.New("the cleartext probe skipped; it must program VPP")
	}
	for line := range strings.SplitSeq(output, "\n") {
		rest, found := strings.CutPrefix(strings.TrimSpace(line), vppCleartextReportPrefix)
		if !found {
			continue
		}
		value, found := strings.CutPrefix(strings.TrimSpace(rest), "spd-id=")
		if !found {
			continue
		}
		if _, err := strconv.ParseUint(value, 10, 32); err != nil {
			continue
		}
		return value, nil
	}
	var tb textbuf.Buffer
	return "", errors.New(tb.Str("the cleartext probe reported no spd-id: ").Str(output).String())
}

// cleartextSPDMissing answers what `show ipsec all` lacks of the probe's SPD, or
// "" when it holds all of it. format_ipsec_policy prints each entry's type with
// the names of foreach_ipsec_spd_policy_type (src/vnet/ipsec/ipsec_spd.h), and
// the binding as "<id> -> <interface>".
func cleartextSPDMissing(all, spdID string) string {
	if !strings.Contains(all, tbDetail(spdID, " -> ", vppCleartextInterface)) {
		return tbDetail("SPD ", spdID, " is not bound to ", vppCleartextInterface)
	}
	for _, kind := range []string{"type ip4-inbound-protect", "type ip4-inbound-discard", "type ip4-inbound-bypass"} {
		if !strings.Contains(all, kind) {
			return tbDetail("real VPP holds no entry of ", kind)
		}
	}
	return ""
}

// traceCleartextFrame sends one probe frame and reads, from VPP alone, whether
// ipsec4-input-feature judged it, what it counted, and whether the frame went on
// to ip4-lookup or was dropped.
func (v *VPP) traceCleartextFrame(container, work string, frame vppCleartextProbeFrame) (VPPCheck, error) {
	file := frame.check + ".pcap"
	if err := writeCleartextPcap(filepath.Join(work, file), frame); err != nil {
		return VPPCheck{}, err
	}
	for _, command := range []string{
		"clear errors",
		"clear trace",
		vppCommand("trace add pg-input", vppTraceCapacity),
		vppCommand("packet-generator new { name", frame.check, "node ethernet-input interface", vppCleartextInterface,
			"pcap", vppMount+"/"+file, "limit 1 }"),
		vppCommand("packet-generator enable-stream", frame.check),
	} {
		if _, err := v.query(container, command); err != nil {
			return VPPCheck{}, err
		}
	}
	generated, streams, err := v.awaitQuery(container, "show packet-generator", true, vppPGGenerateWait,
		func(text string) bool { return pgGenerated(text, frame.check) == 1 })
	if err != nil {
		return VPPCheck{}, err
	}
	trace, err := v.query(container, "show trace")
	if err != nil {
		return VPPCheck{}, err
	}
	counts, err := v.query(container, "show errors")
	if err != nil {
		return VPPCheck{}, err
	}
	if _, err := v.query(container, vppCommand("packet-generator delete", frame.check)); err != nil {
		return VPPCheck{}, err
	}
	return judgeCleartextFrame(frame, generated, streams, trace, counts), nil
}

// judgeCleartextFrame turns VPP's answers about one frame into its check.
func judgeCleartextFrame(frame vppCleartextProbeFrame, generated bool, streams, trace, counts string) VPPCheck {
	if !generated {
		return failVPPCheck(frame.check, "the packet generator did not send the probe frame", streams, nil)
	}
	received := vppErrorCount(counts, vppIPsecInputReceived)
	if received != 1 {
		var tb textbuf.Buffer
		return failVPPCheck(frame.check, tb.Str(vppIPsecInputNode).Str(" counted ").Int(received).
			Str(" frames where the generator sent one, so VPP's IPsec input never judged the probe frame").String(),
			counts, nil)
	}
	nodes := traceNodes(trace)
	fate := traceFate(nodes)
	if fate == vppTraceFateUnspecified {
		return failVPPCheck(frame.check, "the trace shows the probe frame neither reaching ip4-lookup nor dropped", trace, nil)
	}
	kind := "cleartext ICMP echo"
	if frame.esp {
		kind = "an ESP frame with an unknown SPI"
	}
	verb := "forwarded"
	if fate == vppTraceFateDropped {
		verb = "dropped"
	}
	var tb textbuf.Buffer
	tb.Str("real VPP ").Str(verb).Byte(' ').Str(kind).Str(" from ").Str(frame.source).Str(" to ").Str(frame.target).
		Str(", which matches ").Str(frame.entry).Str("; ").Str(vppIPsecInputNode).Str(" counted it as")
	for _, reason := range []string{vppIPsecInputMatch, vppIPsecInputBypass, vppIPsecInputDiscard, vppIPsecInputNoMatch} {
		tb.Str(" '").Str(reason).Str("' ").Int(vppErrorCount(counts, reason))
	}
	tb.Str(". ").Str(frame.owed)
	return passVPPCheck(frame.check, tb.String(), strings.Join(nodes, " -> "))
}

// traceFate answers what the trace shows became of the frame after ip4-input.
// Every feature of the ip4-unicast arc, ipsec4-input-feature among them, runs
// between ip4-input and ip4-lookup, the arc's end node, and an IPsec drop goes to
// error-drop (foreach_ipsec_input_next). A forwarded echo request can still meet
// error-drop later, as its reply, so the first of the two decides. ethernet-input
// hands an IPv4 frame to ip4-input, or to ip4-input-no-checksum when the
// interface already verified the checksum, so either starts the arc.
func traceFate(nodes []string) vppTraceFate {
	input := slices.IndexFunc(nodes, func(node string) bool {
		return node == "ip4-input" || node == "ip4-input-no-checksum"
	})
	if input < 0 {
		return vppTraceFateUnspecified
	}
	for _, node := range nodes[input+1:] {
		if node == "ip4-lookup" {
			return vppTraceFateForwarded
		}
		if node == "error-drop" {
			return vppTraceFateDropped
		}
	}
	return vppTraceFateUnspecified
}

// vppErrorCount answers the sum of the rows `show errors` prints for VPP's IPv4 IPsec input node and
// reason. show_errors (src/vlib/error.c, FDio/vpp v26.06) prints each row as
// "%10lu%=35v%=35s%=10s": the count, the node, the reason and the severity, one
// row per thread, and omits a counter whose value is zero. A missing row is
// therefore VPP's own zero, not an unread counter.
func vppErrorCount(text, reason string) int64 {
	var sum int64
	for line := range strings.SplitSeq(text, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		if fields[1] != vppIPsecInputNode {
			continue
		}
		if strings.Join(fields[2:len(fields)-1], " ") != reason {
			continue
		}
		count, err := strconv.ParseInt(fields[0], 10, 64)
		if err != nil {
			continue
		}
		sum += count
	}
	return sum
}

// writeCleartextPcap writes the one Ethernet frame of a probe frame.
func writeCleartextPcap(path string, frame vppCleartextProbeFrame) error {
	var packet [vppCleartextFrameLen]byte
	if err := cleartextProbePacket(packet[:], frame); err != nil {
		return err
	}
	file, err := os.Create(path) //nolint:gosec // a scratch file the container's VPP reads
	if err != nil {
		return err
	}
	return errors.Join(pcap.WriteFileHeader(file, 65535, pcap.LinkTypeEthernet),
		pcap.WriteRecord(file, time.Now(), packet[:], len(packet)), file.Close())
}

// vppCleartextFrameLen is the length of every probe frame, the Ethernet minimum
// without its FCS. ip4-input reads the datagram's own length from its header, so
// the padding after a shorter datagram is ignored.
const vppCleartextFrameLen = 60

// cleartextProbePacket fills packet, 60 octets, with one probe frame:
//
//	 0                   1                   2                   3
//	 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|        Ethernet: destination, source, type 0x0800  (0-13)    |
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|        IPv4 header, 20 octets, protocol 1 or 50  (14-33)      |
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	| ICMP: type 8, code 0, checksum, identifier, sequence,         |
//	|       8 octets of data  (34-49)                               |
//	|   or                                                          |
//	| ESP: SPI, sequence number, 16 octets standing for the         |
//	|      ciphertext  (34-57)                                      |
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|        zero padding to 60 octets                              |
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
func cleartextProbePacket(packet []byte, frame vppCleartextProbeFrame) error {
	if len(packet) != vppCleartextFrameLen {
		panic("BUG: cleartextProbePacket needs a 60-octet frame")
	}
	destination, err := net.ParseMAC(vppCleartextInterfaceMAC)
	if err != nil {
		return err
	}
	source := net.ParseIP(frame.source).To4()
	if source == nil {
		return errors.New(tbDetail("probe frame ", frame.check, " has no IPv4 source"))
	}
	target := net.ParseIP(frame.target).To4()
	if target == nil {
		return errors.New(tbDetail("probe frame ", frame.check, " has no IPv4 destination"))
	}
	clear(packet)
	copy(packet[0:6], destination)
	copy(packet[6:12], []byte{0x02, 0xfe, 0x00, 0x00, 0x45, 0x02})
	binary.BigEndian.PutUint16(packet[12:14], 0x0800)

	ip := packet[14:34]
	ip[0] = 0x45
	ip[8] = 64
	copy(ip[12:16], source)
	copy(ip[16:20], target)
	payload := packet[34:]
	length := 16
	if frame.esp {
		ip[9] = 50
		binary.BigEndian.PutUint32(payload[0:4], vppCleartextESPSPI)
		binary.BigEndian.PutUint32(payload[4:8], 1)
		length = 24
	} else {
		ip[9] = 1
		payload[0] = 8
		binary.BigEndian.PutUint16(payload[4:6], 0x7a65)
		binary.BigEndian.PutUint16(payload[6:8], 1)
		copy(payload[8:16], "ze-probe")
		binary.BigEndian.PutUint16(payload[2:4], ipv4HeaderChecksum(payload[:length]))
	}
	binary.BigEndian.PutUint16(ip[2:4], uint16(20+length)) //nolint:gosec // 20 plus 16 or 24
	binary.BigEndian.PutUint16(ip[10:12], ipv4HeaderChecksum(ip))
	return nil
}
