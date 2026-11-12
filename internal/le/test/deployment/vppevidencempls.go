// Design: docs/architecture/testing/interop.md -- the mpls-label-pop real VPP proof
// Overview: vppevidence.go -- the shared container and ordered scenario dispatch
// Related: vppevidencescenarios.go -- vppMPLSLabelPopConfig and the scenario's fixed values
// Related: vpp_srv6_pg_integration_linux_test.go -- the packet-generator pcap pattern reused here
//
// mpls-label-pop proves that an LDP egress pop reaches VPP's MPLS table 0
// through fib-vpp with no fib block, that VPP forwards a labeled frame by it,
// and that a reload which removes LDP removes both entries. Every verdict
// comes from VPP's own CLI: `show mpls fib`, `show mpls interface` and
// `show trace`, never from a ze log line.
package testdeployment

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/core/pcap"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/interoplab"
)

const (
	vppMPLSPopConfigFile = "mpls-label-pop.conf"
	vppMPLSFIBQuery      = "show mpls fib table 0"
	// vppMPLSLabelMin is the first label outside the reserved range (RFC 3032
	// Section 2.1 reserves 0 to 15). VPP prints a reserved label by name.
	vppMPLSLabelMin   = 16
	vppPGGenerateWait = 5 * time.Second
	vppTraceCapacity  = "8"
)

// vppMPLSFIBEntryLine matches the first line of one entry of `show mpls fib`.
// VPP's mpls_fib_table_show_all prints each entry with format_fib_entry, whose
// first token is format_fib_prefix: for FIB_PROTOCOL_MPLS, "%U:%U" of
// format_mpls_unicast_label and format_mpls_eos_bit, then "/%d" of the length.
// format_mpls_eos_bit writes MPLS_EOS_BITS, "eos" or "neos"
// (src/vnet/mpls/packet.h, FDio/vpp v26.06). A reserved label prints as a
// name, so this pattern only ever matches an allocated label.
var vppMPLSFIBEntryLine = regexp.MustCompile(`^(\d+):(n?eos)/\d+`)

// vppTraceNodeLine matches one node line of `show trace`: the elapsed time,
// then the graph node's registered name.
var vppTraceNodeLine = regexp.MustCompile(`^\d+:\d+:\d+:\d+: ([a-z0-9-]+)$`)

// vppMPLSFIBEntry is one (label, S) entry of MPLS table 0 and the lines VPP
// printed for it, its paths and forwarding chain included.
type vppMPLSFIBEntry struct {
	label uint32
	eos   bool
	block string
}

func (v *VPP) runMPLSLabelPop(container, work string) (VPPScenarioReport, error) {
	checks := make([]VPPCheck, 0, 5)
	if err := v.createMPLSPopInterface(container); err != nil {
		return VPPScenarioReport{}, err
	}
	if err := v.writeConfig(work, vppMPLSPopConfigFile, vppMPLSLabelPopConfig(true)); err != nil {
		return VPPScenarioReport{}, err
	}
	daemon, seen, err := v.startEvidenceDaemon(container, vppMPLSPopConfigFile, 0)
	if err != nil {
		return VPPScenarioReport{}, err
	}
	defer v.stopVPPProcess(daemon, seen)

	var label uint32
	ok, text, err := v.awaitQuery(container, vppMPLSFIBQuery, true, v.vppApplyWait(), func(text string) bool {
		found, single := singlePoppedLabel(text)
		label = found
		return single
	})
	if err != nil {
		return VPPScenarioReport{}, err
	}
	if !ok {
		checks = append(checks, failVPPCheck("installed",
			"real VPP MPLS table 0 holds no single LDP label as both an eos entry with an ip4 lookup path "+
				"and a neos entry with an mpls lookup path", text, seen))
		return finishVPPScenario(VPPScenarioMPLSLabelPop, checks), nil
	}
	checks = append(checks, passVPPCheck("installed",
		tbDetail("real VPP MPLS table 0 holds LDP label ", textbuf.StringUint32(label),
			" as eos (ip4 lookup) and neos (mpls lookup)"), tailLines(text)...))

	ok, text, err = v.awaitQuery(container, "show mpls interface", true, vppQueryWait, vppMPLSInputEnabled)
	if err != nil {
		return VPPScenarioReport{}, err
	}
	if !ok {
		checks = append(checks, failVPPCheck("mpls-input",
			tbDetail("real VPP does not report MPLS enabled on ", VPPMPLSPopInterface), text, seen))
		return finishVPPScenario(VPPScenarioMPLSLabelPop, checks), nil
	}
	checks = append(checks, passVPPCheck("mpls-input",
		tbDetail("real VPP reports MPLS enabled on ", VPPMPLSPopInterface), tailLines(text)...))

	check, err := v.traceMPLSFrame(container, work, label, true)
	if err != nil {
		return VPPScenarioReport{}, err
	}
	checks = append(checks, check)
	if check.Verdict != VPPProofPass {
		return finishVPPScenario(VPPScenarioMPLSLabelPop, checks), nil
	}
	check, err = v.traceMPLSFrame(container, work, VPPMPLSPopUnprogrammedLabel, false)
	if err != nil {
		return VPPScenarioReport{}, err
	}
	checks = append(checks, check)
	if check.Verdict != VPPProofPass {
		return finishVPPScenario(VPPScenarioMPLSLabelPop, checks), nil
	}

	if err := v.reloadEvidenceConfig(container, vppMPLSPopConfigFile, vppMPLSLabelPopConfig(false), daemon); err != nil {
		return VPPScenarioReport{}, err
	}
	ok, text, err = v.awaitQuery(container, vppMPLSFIBQuery, false, vppWithdrawWait, func(text string) bool {
		return labelHasEntry(text, label)
	})
	if err != nil {
		return VPPScenarioReport{}, err
	}
	if !ok {
		checks = append(checks, failVPPCheck("removed",
			tbDetail("real VPP MPLS table 0 still holds label ", textbuf.StringUint32(label),
				" after the reload that removed ldp"), text, seen))
		return finishVPPScenario(VPPScenarioMPLSLabelPop, checks), nil
	}
	checks = append(checks, passVPPCheck("removed",
		tbDetail("real VPP removed both entries of label ", textbuf.StringUint32(label),
			" when a reload removed ldp")))
	return finishVPPScenario(VPPScenarioMPLSLabelPop, checks), nil
}

// createMPLSPopInterface adds the packet-generator interface ze enables MPLS
// input on. It exists before ze starts, so the VPP backend's interface dump
// names it.
func (v *VPP) createMPLSPopInterface(container string) error {
	for _, command := range []string{
		vppCommand("create packet-generator interface", VPPMPLSPopInterface, "hw-addr", VPPMPLSPopInterfaceMAC),
		vppCommand("set interface state", VPPMPLSPopInterface, "up"),
	} {
		if _, err := v.query(container, command); err != nil {
			return err
		}
	}
	return nil
}

// reloadEvidenceConfig replaces the staged configuration of a running daemon
// and signals SIGHUP, the operator's reload. The daemon keeps running, so the
// verdict that follows judges the reload and not a fresh start.
func (v *VPP) reloadEvidenceConfig(container, configFile, content string, daemon *running) error {
	if daemon.exited() {
		return errors.New("the VPP evidence daemon exited before its reload")
	}
	_, err := v.Docker.ExecAttached(v.ctx, interoplab.ContainerExec{
		Container:   container,
		Command:     []string{"tee", filepath.Join(zeRunDir, configFile, "ze.conf")},
		Interactive: true, Stdin: strings.NewReader(content),
		Stdout: io.Discard, Timeout: vppctlTimeout,
	})
	if err != nil {
		return fmt.Errorf("replace the staged %s: %w", configFile, err)
	}
	return daemon.signal(syscall.SIGHUP)
}

// traceMPLSFrame injects one labeled frame on the packet-generator interface
// and judges VPP's own trace of it. A programmed label must reach mpls-lookup
// and then an IPv4 lookup, which only the eos entry's ip4 lookup path leads
// to. An unprogrammed label must reach mpls-lookup and then mpls-drop, the
// node of the drop load-balance a table 0 miss answers with (mpls_fib_create
// sets every unset entry to mpls_fib_drop_dpo_index), and no IPv4 lookup.
func (v *VPP) traceMPLSFrame(container, work string, label uint32, programmed bool) (VPPCheck, error) {
	name := "mpls-pop-control"
	check := "unprogrammed-drop"
	if programmed {
		name = "mpls-pop"
		check = "popped"
	}
	file := name + ".pcap"
	if err := writeMPLSPopPcap(filepath.Join(work, file), label); err != nil {
		return VPPCheck{}, err
	}
	for _, command := range []string{
		"clear trace",
		vppCommand("trace add pg-input", vppTraceCapacity),
		vppCommand("packet-generator new { name", name, "node ethernet-input interface", VPPMPLSPopInterface,
			"pcap", vppMount+"/"+file, "limit 1 }"),
		vppCommand("packet-generator enable-stream", name),
	} {
		if _, err := v.query(container, command); err != nil {
			return VPPCheck{}, err
		}
	}
	generated, streams, err := v.awaitQuery(container, "show packet-generator", true, vppPGGenerateWait,
		func(text string) bool { return pgGenerated(text, name) == 1 })
	if err != nil {
		return VPPCheck{}, err
	}
	trace, err := v.query(container, "show trace")
	if err != nil {
		return VPPCheck{}, err
	}
	if _, err := v.query(container, vppCommand("packet-generator delete", name)); err != nil {
		return VPPCheck{}, err
	}
	labelText := textbuf.StringUint32(label)
	if !generated {
		return failVPPCheck(check, tbDetail("the packet generator did not send its one frame with label ", labelText),
			streams, nil), nil
	}
	nodes := traceNodes(trace)
	if programmed {
		if tracePopped(nodes) {
			return passVPPCheck(check, tbDetail("real VPP traced the frame with label ", labelText,
				" through mpls-lookup to an IPv4 lookup"), strings.Join(nodes, " -> ")), nil
		}
		return failVPPCheck(check, tbDetail("real VPP did not trace the frame with label ", labelText,
			" through mpls-lookup to an IPv4 lookup"), trace, nil), nil
	}
	if traceDropped(nodes) {
		return passVPPCheck(check, tbDetail("real VPP dropped the control frame with unprogrammed label ", labelText,
			" at mpls-lookup"), strings.Join(nodes, " -> ")), nil
	}
	return failVPPCheck(check, tbDetail("real VPP did not drop the control frame with unprogrammed label ",
		labelText, " at mpls-lookup"), trace, nil), nil
}

// writeMPLSPopPcap writes one Ethernet frame: the packet-generator
// interface's MAC as destination, one label stack entry, and an IPv4/UDP
// datagram to the LSR-ID.
func writeMPLSPopPcap(path string, label uint32) error {
	var frame [60]byte
	if err := mplsPopFrame(frame[:], label); err != nil {
		return err
	}
	file, err := os.Create(path) //nolint:gosec // a scratch file the container's VPP reads
	if err != nil {
		return err
	}
	return errors.Join(pcap.WriteFileHeader(file, 65535, pcap.LinkTypeEthernet),
		pcap.WriteRecord(file, time.Now(), frame[:], len(frame)), file.Close())
}

// mplsPopFrame fills frame, 60 octets, with the labeled test frame:
//
//	 0                   1                   2                   3
//	 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|        Ethernet: destination, source, type 0x8847  (0-13)    |
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|                Label                  | TC  |S|       TTL     |  (14-17)
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|        IPv4 header, 20 octets, to the LSR-ID  (18-37)         |
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|        UDP header and payload  (38-59)                        |
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//
// RFC 3032 Section 2.1: "This bit is set to one for the last entry in the
// label stack (i.e., for the bottom of the stack), and zero for all other
// label stack entries." The one entry is the bottom, so S is one and VPP
// looks the label up as its eos entry.
func mplsPopFrame(frame []byte, label uint32) error {
	if len(frame) != 60 {
		panic("BUG: mplsPopFrame needs a 60-octet frame")
	}
	destination, err := net.ParseMAC(VPPMPLSPopInterfaceMAC)
	if err != nil {
		return err
	}
	clear(frame)
	copy(frame[0:6], destination)
	copy(frame[6:12], []byte{0x02, 0xfe, 0x00, 0x00, 0x30, 0x02})
	binary.BigEndian.PutUint16(frame[12:14], 0x8847)
	binary.BigEndian.PutUint32(frame[14:18], label<<12|1<<8|64)

	ip := frame[18:38]
	ip[0] = 0x45
	binary.BigEndian.PutUint16(ip[2:4], 42)
	ip[8] = 64
	ip[9] = 17
	copy(ip[12:16], []byte{192, 0, 2, 10})
	copy(ip[16:20], net.ParseIP(VPPMPLSPopLSRID).To4())
	binary.BigEndian.PutUint16(ip[10:12], ipv4HeaderChecksum(ip))

	udp := frame[38:60]
	binary.BigEndian.PutUint16(udp[0:2], 4000)
	binary.BigEndian.PutUint16(udp[2:4], 4000)
	binary.BigEndian.PutUint16(udp[4:6], 22)
	copy(udp[8:], "ze-label-pop")
	return nil
}

// ipv4HeaderChecksum answers the one's complement checksum of header, whose
// checksum field is zero.
func ipv4HeaderChecksum(header []byte) uint16 {
	var sum uint32
	for i := 0; i+1 < len(header); i += 2 {
		sum += uint32(binary.BigEndian.Uint16(header[i : i+2]))
	}
	for sum > 0xffff {
		sum = sum&0xffff + sum>>16
	}
	return ^uint16(sum)
}

// mplsFIBEntries reads every allocated-label entry of `show mpls fib`. An
// entry's block runs from its first line to the next line VPP printed at
// column zero, the next entry or the next table's header.
func mplsFIBEntries(text string) []vppMPLSFIBEntry {
	var entries []vppMPLSFIBEntry
	var block strings.Builder
	current := -1
	flush := func() {
		if current >= 0 {
			entries[current].block = block.String()
		}
		block.Reset()
	}
	for line := range strings.SplitSeq(text, "\n") {
		if line != "" && line[0] != ' ' && line[0] != '\t' {
			flush()
			current = -1
			match := vppMPLSFIBEntryLine.FindStringSubmatch(line)
			if match != nil {
				label, err := strconv.ParseUint(match[1], 10, 32)
				if err == nil {
					entries = append(entries, vppMPLSFIBEntry{label: uint32(label), eos: match[2] == "eos"})
					current = len(entries) - 1
				}
			}
		}
		if current >= 0 {
			block.WriteString(line)
			block.WriteByte('\n')
		}
	}
	flush()
	return entries
}

// singlePoppedLabel answers the one allocated label that table 0 holds as an
// egress pop, and whether exactly one does. A pop is an eos entry whose path
// is an ip4 deag (look the payload up in IPv4) and a neos entry whose path is
// an mpls deag (look the next label up). format_fib_path writes each path as
// "path:[%d] pl-index:%d %U weight=%d pref=%d %s: ", the third field being
// format_dpo_proto ("ip4", "mpls") and the last fib_path_type_names
// ("deag" for FIB_PATH_TYPE_DEAG).
func singlePoppedLabel(text string) (uint32, bool) {
	eosLookup := map[uint32]bool{}
	neosLookup := map[uint32]bool{}
	for _, entry := range mplsFIBEntries(text) {
		if entry.label < vppMPLSLabelMin {
			continue
		}
		if entry.eos {
			eosLookup[entry.label] = deagPath(entry.block, "ip4")
			continue
		}
		neosLookup[entry.label] = deagPath(entry.block, "mpls")
	}
	var popped []uint32
	for label, ip4 := range eosLookup {
		if !ip4 {
			continue
		}
		if neosLookup[label] {
			popped = append(popped, label)
		}
	}
	if len(popped) != 1 {
		return 0, false
	}
	return popped[0], true
}

// deagPath reports whether block holds a deag path of the payload protocol.
func deagPath(block, proto string) bool {
	for line := range strings.SplitSeq(block, "\n") {
		if !strings.Contains(line, "path:[") {
			continue
		}
		if !strings.Contains(line, " "+proto+" weight=") {
			continue
		}
		if strings.Contains(line, " deag: ") {
			return true
		}
	}
	return false
}

// labelHasEntry reports whether table 0 holds either stack position of label.
func labelHasEntry(text string, label uint32) bool {
	for _, entry := range mplsFIBEntries(text) {
		if entry.label == label {
			return true
		}
	}
	return false
}

// vppMPLSInputEnabled reports whether `show mpls interface` names the
// packet-generator interface as enabled. show_mpls_interface_one prints the
// interface name on one line and "  MPLS enabled" on the next
// (src/vnet/mpls/interface.c, FDio/vpp v26.06).
func vppMPLSInputEnabled(text string) bool {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if strings.TrimSpace(line) != VPPMPLSPopInterface {
			continue
		}
		for _, next := range lines[i+1:] {
			trimmed := strings.TrimSpace(next)
			if trimmed == "" {
				continue
			}
			return trimmed == "MPLS enabled"
		}
	}
	return false
}

// pgGenerated answers the generated count `show packet-generator` reports for
// stream name, or -1 when the stream is absent or its count unreadable.
func pgGenerated(text, name string) int64 {
	for line := range strings.SplitSeq(text, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		if fields[0] != name {
			continue
		}
		count, err := strconv.ParseInt(fields[2], 10, 64)
		if err != nil {
			return -1
		}
		return count
	}
	return -1
}

// traceNodes answers the graph nodes of `show trace`, in the order VPP
// visited them.
func traceNodes(trace string) []string {
	var nodes []string
	for line := range strings.SplitSeq(trace, "\n") {
		match := vppTraceNodeLine.FindStringSubmatch(strings.TrimSpace(line))
		if match != nil {
			nodes = append(nodes, match[1])
		}
	}
	return nodes
}

// tracePopped reports whether an IPv4 lookup follows mpls-lookup. The eos
// entry's ip4 deag path forwards through a lookup DPO, whose node is
// lookup-ip4-dst (src/vnet/dpo/lookup_dpo.c); ip4-lookup is accepted too, so
// a chain that resolves straight into the IPv4 FIB also counts.
func tracePopped(nodes []string) bool {
	lookup := slices.Index(nodes, "mpls-lookup")
	if lookup < 0 {
		return false
	}
	for _, node := range nodes[lookup+1:] {
		if node == "lookup-ip4-dst" {
			return true
		}
		if node == "ip4-lookup" {
			return true
		}
	}
	return false
}

// traceDropped reports whether mpls-drop follows mpls-lookup and no IPv4
// lookup appears anywhere.
func traceDropped(nodes []string) bool {
	if slices.Contains(nodes, "lookup-ip4-dst") {
		return false
	}
	if slices.Contains(nodes, "ip4-lookup") {
		return false
	}
	lookup := slices.Index(nodes, "mpls-lookup")
	if lookup < 0 {
		return false
	}
	return slices.Contains(nodes[lookup+1:], "mpls-drop")
}
