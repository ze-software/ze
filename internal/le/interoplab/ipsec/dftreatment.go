// Design: docs/architecture/testing/interop.md -- the outer DF bit of a tunnel, read where strongSwan receives it.
// Related: helpers.go -- inner addresses, the Ze reload and the ping loss summary.
package ipsec

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/interoplab"
)

// The tunnel-df-treatment scenario's inner addresses and probe size. They sit inside
// the Child SA's selectors, 10.2.0.0/24 on Ze's side and 10.1.0.0/24 on strongSwan's.
const (
	dfZeInner   = "10.2.0.1"
	dfSwanInner = "10.1.0.1"
	// dfPingCount is how many echo requests each step sends. Nothing else Ze sends
	// rides ESP in the window, so strongSwan must count exactly this many.
	dfPingCount = 3

	// The nftables table strongSwan counts Ze's ESP in, and the comment of each of
	// its two rules. iptables cannot read the DF bit on these hosts: its one match
	// for an arbitrary header bit is xt_u32, which Ze's kernel does not build
	// (CONFIG_NETFILTER_XT_MATCH_U32 is not set), while an nftables payload
	// expression is part of nf_tables itself.
	dfCounterTable   = "ze_outer_df"
	dfCounterSet     = "outer-df-set"
	dfCounterClear   = "outer-df-clear"
	dfCounterCommand = "nft"
	// dfCounterFamily is the nftables family of the table, inet because Ze's kernel
	// configuration names CONFIG_NF_TABLES_INET, and dfCounterHook names both the
	// chain and the hook it sits on.
	dfCounterFamily = "inet"
	dfCounterHook   = "input"
	// wordRule is the nft(8) object a counting rule is.
	wordRule = "rule"
	// dfBitSet and dfBitClear name the two values of the DF bit, and clear is also
	// the df-bit leaf value that writes it clear.
	dfBitSet   = "set"
	dfBitClear = "clear"
	// dfFragOffDF is the Don't Fragment flag in the 16-bit flags and fragment
	// offset field of the IPv4 header (RFC 791 Section 3.1).
	dfFragOffDF = "0x4000"
)

// dfTreatment is the df-bit leaf one phase of the scenario configures. The zero
// value names no treatment, so a phase that forgot to set one is refused rather
// than read as copy.
type dfTreatment uint8

const (
	dfTreatmentUnspecified dfTreatment = iota
	dfTreatmentClear
	dfTreatmentCopy
)

// leaf answers the df-bit value Ze's config and `show vpn ipsec dataplane sa` carry.
func (t dfTreatment) leaf() (string, error) {
	switch t {
	case dfTreatmentClear:
		return dfBitClear, nil
	case dfTreatmentCopy:
		return "copy", nil
	case dfTreatmentUnspecified:
	}
	return "", fmt.Errorf("DF treatment %d names neither clear nor copy", t)
}

// outerDF answers the DF bit the outer IPv4 header of Ze's ESP packet must carry
// for an inner packet whose own DF bit is innerDF.
//
// RFC 4301 Section 8.1: "This means that it MUST be possible to configure the
// implementation's treatment of the DF bit (set, clear, copy from inner header) for
// each SA."
func (t dfTreatment) outerDF(innerDF bool) (bool, error) {
	switch t {
	case dfTreatmentClear:
		return false, nil
	case dfTreatmentCopy:
		return innerDF, nil
	case dfTreatmentUnspecified:
	}
	return false, fmt.Errorf("DF treatment %d names neither clear nor copy", t)
}

// checkTunnelDFTreatment proves RFC 4301 Section 8.1 on the wire (AC-8 of
// spec-rfc4301-architecture-gaps). Ze initiates with df-bit clear (ze.conf), then
// is reloaded with df-bit copy (ze-copy.conf), and in each phase sends one burst of
// inner packets with DF set and one with DF clear. strongSwan counts every ESP
// packet from Ze by the DF bit of its OUTER IPv4 header, in nftables on its INPUT
// hook, so the bit is read as the peer receives it and not as Ze reports it.
//
// Under clear every packet must arrive with DF clear; under copy each packet must
// carry its inner bit. A single phase could not discriminate: copy and clear agree
// for an inner packet whose DF is clear, and only the DF-set burst tells them apart.
// df-bit set is not a phase: every dataplane refuses it at commit (the Linux kernel
// copies the inner bit or clears it, and no XFRM flag sets it), and that refusal is
// proven by test/parse/ipsec-df-bit-treatment.ci.
//
// Red proof: make xfrmDFFlag (internal/component/ike/dataplane/xfrm_linux.go)
// answer false for DFClear, rebuild the Ze image (the harness does it), and the
// clear phase's DF-set burst arrives with DF set. Make it answer true for DFCopy,
// and the copy phase's DF-set burst arrives with DF clear.
func checkTunnelDFTreatment(ctx context.Context, lab *scenarioLab) error {
	if err := establish(ctx, lab); err != nil {
		return err
	}
	for _, peer := range []string{zePeer, swanPeer} {
		if _, err := lab.waitXFRM(ctx, peer); err != nil {
			return err
		}
	}
	if err := lab.natInnerAddress(ctx, zePeer, dfZeInner, dfSwanInner); err != nil {
		return err
	}
	if err := lab.natInnerAddress(ctx, swanPeer, dfSwanInner, dfZeInner); err != nil {
		return err
	}
	if err := lab.installOuterDFCounters(ctx); err != nil {
		return err
	}

	first, err := lab.waitZeOutboundSPI(ctx, "")
	if err != nil {
		return err
	}
	if err := lab.requireOuterDFPhase(ctx, dfTreatmentClear); err != nil {
		return err
	}

	if err := lab.reloadZe(ctx, filepath.Join(lab.check.Source.Directory, "ze-copy.conf")); err != nil {
		return err
	}
	// The df-bit leaf is part of the peer's configuration, so the reload replaces
	// the Child SA; the copy phase runs on the replacement alone.
	if _, err := lab.waitZeOutboundSPI(ctx, first); err != nil {
		return fmt.Errorf("after the reload to df-bit copy: %w", err)
	}
	return lab.requireOuterDFPhase(ctx, dfTreatmentCopy)
}

// requireOuterDFPhase reads the treatment Ze reports for its outbound SA, then sends
// a DF-set burst and a DF-clear burst and requires strongSwan to count each packet
// with the outer DF bit the treatment derives from its inner bit.
func (l *scenarioLab) requireOuterDFPhase(ctx context.Context, treatment dfTreatment) error {
	leaf, err := treatment.leaf()
	if err != nil {
		return err
	}
	answer, err := l.zeCLI(ctx, "show vpn ipsec dataplane sa | json")
	if err != nil {
		return err
	}
	if err := requireDataplaneDF(answer, leaf); err != nil {
		return err
	}
	for _, innerDF := range []bool{true, false} {
		want, err := treatment.outerDF(innerDF)
		if err != nil {
			return err
		}
		before, err := l.readOuterDFCounts(ctx)
		if err != nil {
			return err
		}
		// ping exits non-zero on any loss, so its error is kept for the refusal
		// rather than returned: the loss summary is what decides.
		output, pingErr := l.exec(ctx, zePeer, dfPingCommand(innerDF)...)
		loss, err := pingLoss(output)
		if err != nil {
			return fmt.Errorf("df-bit %s: ping from %s to %s %w (exec: %v)", leaf, dfZeInner, dfSwanInner, err, pingErr)
		}
		if loss != 0 {
			return fmt.Errorf("df-bit %s: ping from %s to %s lost %g%% of %d packets (exec: %v): %s", leaf, dfZeInner, dfSwanInner, loss, dfPingCount, pingErr, output)
		}
		after, err := l.readOuterDFCounts(ctx)
		if err != nil {
			return err
		}
		if err := judgeOuterDF(before, after, dfPingCount, want); err != nil {
			state, stateErr := l.exec(ctx, zePeer, "ip", xfrmCommand, ipWordState, "list", ipWordSrc, zeIP, xfrmHeaderTarget, swanIP)
			if stateErr != nil {
				return fmt.Errorf("df-bit %s, inner DF %s: %w; ze's outbound XFRM state was not read: %w", leaf, dfBitName(innerDF), err, stateErr)
			}
			return fmt.Errorf("df-bit %s, inner DF %s: %w; ze's outbound XFRM state:\n%s", leaf, dfBitName(innerDF), err, state)
		}
	}
	return nil
}

// dfPingCommand is the argv of one burst from Ze's inner address. iputils ping -M do
// sets the inner DF bit and -M dont clears it (IP_PMTUDISC_DO, IP_PMTUDISC_DONT).
func dfPingCommand(innerDF bool) []string {
	mode := "dont"
	if innerDF {
		mode = "do"
	}
	return []string{cmdPing, "-c", strconv.Itoa(dfPingCount), "-W", "2", "-I", dfZeInner, "-M", mode, dfSwanInner}
}

// dfBitName names a DF bit for a refusal.
func dfBitName(set bool) string {
	if set {
		return dfBitSet
	}
	return dfBitClear
}

// installOuterDFCounters adds strongSwan's two counters of ESP from Ze, one per
// value of the outer DF bit. The input hook sees the ESP packet before XFRM
// decapsulates it, and the decapsulated inner packet is ICMP, so only the outer
// header is ever counted.
func (l *scenarioLab) installOuterDFCounters(ctx context.Context) error {
	rule := func(comparison, comment string) []string {
		return []string{dfCounterCommand, wordAdd, wordRule, dfCounterFamily, dfCounterTable, dfCounterHook,
			"ip", "saddr", zeIP, "ip", "protocol", "esp",
			"ip", "frag-off", "&", dfFragOffDF, comparison, "0",
			"counter", "comment", `"` + comment + `"`}
	}
	steps := [][]string{
		{dfCounterCommand, wordAdd, ipWordTable, dfCounterFamily, dfCounterTable},
		{dfCounterCommand, wordAdd, "chain", dfCounterFamily, dfCounterTable, dfCounterHook,
			"{", "type", "filter", "hook", dfCounterHook, "priority", "0", ";", ipWordPolicy, "accept", ";", "}"},
		rule("!=", dfCounterSet),
		rule("==", dfCounterClear),
	}
	for _, step := range steps {
		if output, err := l.exec(ctx, swanPeer, step...); err != nil {
			return fmt.Errorf("install strongSwan's outer DF counters (%s): %w: %s", strings.Join(step, " "), err, output)
		}
	}
	return nil
}

// outerDFCount is one reading of strongSwan's two counters.
type outerDFCount struct {
	set   uint64
	clear uint64
}

// readOuterDFCounts reads both counters from one listing of the chain.
func (l *scenarioLab) readOuterDFCounts(ctx context.Context) (outerDFCount, error) {
	output, err := l.exec(ctx, swanPeer, dfCounterCommand, "list", "chain", dfCounterFamily, dfCounterTable, dfCounterHook)
	if err != nil {
		return outerDFCount{}, fmt.Errorf("read strongSwan's outer DF counters: %w: %s", err, output)
	}
	setCount, err := parseOuterDFCounter(output, dfCounterSet)
	if err != nil {
		return outerDFCount{}, err
	}
	clearCount, err := parseOuterDFCounter(output, dfCounterClear)
	if err != nil {
		return outerDFCount{}, err
	}
	return outerDFCount{set: setCount, clear: clearCount}, nil
}

// parseOuterDFCounter answers the packet count of the ONE rule an `nft list chain`
// listing carries with comment, printed as `counter packets <n> bytes <m> comment
// "<comment>"`. A missing rule, a rule with no count, or two rules with the comment
// is an error, never zero: a zero read where no rule was found would satisfy the
// "no packet carried the other bit" half of every step.
func parseOuterDFCounter(output, comment string) (uint64, error) {
	var tb textbuf.Buffer
	marker := tb.Str(`comment "`).Str(comment).Byte('"').String()
	found := false
	var packets uint64
	for line := range strings.SplitSeq(output, "\n") {
		if !strings.Contains(line, marker) {
			continue
		}
		if found {
			return 0, fmt.Errorf("strongSwan's chain holds two rules with comment %q:\n%s", comment, output)
		}
		count, err := packetCountOf(line)
		if err != nil {
			return 0, fmt.Errorf("strongSwan's %s rule: %w", comment, err)
		}
		packets = count
		found = true
	}
	if !found {
		return 0, fmt.Errorf("strongSwan's chain holds no rule with comment %q:\n%s", comment, output)
	}
	return packets, nil
}

// packetCountOf answers the number after `packets` on one nft rule line.
func packetCountOf(line string) (uint64, error) {
	fields := strings.Fields(line)
	for i := 0; i+1 < len(fields); i++ {
		if fields[i] != "packets" {
			continue
		}
		value, err := strconv.ParseUint(fields[i+1], 10, 64)
		if err != nil {
			return 0, fmt.Errorf("%q is not a packet count: %w", fields[i+1], err)
		}
		return value, nil
	}
	return 0, fmt.Errorf("the rule prints no packet count: %s", strings.TrimSpace(line))
}

// judgeOuterDF refuses unless exactly sent packets moved the counter of the wanted
// outer DF bit and none moved the other. Exactly, because nothing but the burst
// rides ESP from Ze in the window: a packet over or under says the counter saw
// something other than the burst.
func judgeOuterDF(before, after outerDFCount, sent uint64, wantSet bool) error {
	if after.set < before.set {
		return fmt.Errorf("strongSwan's DF-set counter went back from %d to %d", before.set, after.set)
	}
	if after.clear < before.clear {
		return fmt.Errorf("strongSwan's DF-clear counter went back from %d to %d", before.clear, after.clear)
	}
	setDelta := after.set - before.set
	clearDelta := after.clear - before.clear
	want, other := clearDelta, setDelta
	if wantSet {
		want, other = setDelta, clearDelta
	}
	if want == sent && other == 0 {
		return nil
	}
	return fmt.Errorf("strongSwan received %d ESP packets from ze with the outer DF bit set and %d with it clear, want all %d with it %s",
		setDelta, clearDelta, sent, dfBitName(wantSet))
}

// waitZeOutboundSPI waits until Ze holds exactly one outbound ESP state toward
// strongSwan, other than previous when previous is not empty, and strongSwan holds
// the same SPI inbound, then answers that SPI.
func (l *scenarioLab) waitZeOutboundSPI(ctx context.Context, previous string) (string, error) {
	var spi string
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: selectorWaitTimeout, Interval: 2 * time.Second, Description: "ze's one outbound ESP state toward strongSwan",
	}, func(probe context.Context) (string, error) {
		return l.exec(probe, zePeer, "ip", xfrmCommand, ipWordState, "list", ipWordSrc, zeIP, xfrmHeaderTarget, swanIP)
	}, func(output string) bool {
		var ok bool
		spi, ok = soleOutboundSPI(output, previous)
		return ok
	})
	if err != nil {
		return "", err
	}
	_, err = l.waitOutput(ctx, swanPeer, []string{"ip", xfrmCommand, ipWordState, "list", ipWordSrc, zeIP, xfrmHeaderTarget, swanIP},
		xfrmWaitTimeout, "strongSwan's inbound state for ze's SPI "+spi, func(output string) bool {
			return holdsESPSPI(output, spi)
		})
	if err != nil {
		return "", err
	}
	return spi, nil
}

// soleOutboundSPI answers the SPI of a listing that names exactly one ESP state,
// refusing a listing whose one SPI is previous.
func soleOutboundSPI(output, previous string) (string, bool) {
	matches := espSPIPattern.FindAllStringSubmatch(output, -1)
	if len(matches) != 1 {
		return "", false
	}
	spi := matches[0][1]
	if spi == previous {
		return "", false
	}
	return spi, true
}

// holdsESPSPI reports whether a state listing names spi as an ESP SPI.
func holdsESPSPI(output, spi string) bool {
	for _, match := range espSPIPattern.FindAllStringSubmatch(output, -1) {
		if match[1] == spi {
			return true
		}
	}
	return false
}

// requireDataplaneDF refuses unless Ze reports at least one outbound SA toward
// strongSwan and every one carries df-bit leaf, read back from the dataplane. The
// producer (saInfoToMap, internal/component/ike/cmd/show_dataplane.go) writes
// options-known false and df-bit null for an SA whose options it did not read back,
// so options-known false or absent is refused by name, and so is a null df-bit on
// the outbound row, rather than either reading as a mismatch.
func requireDataplaneDF(answer, leaf string) error {
	records, err := decodeZeDataplaneSAs(answer)
	if err != nil {
		return err
	}
	outbound := 0
	for _, record := range records {
		if record.Src != zeIP {
			continue
		}
		if record.Dst != swanIP {
			continue
		}
		outbound++
		if record.OptionsKnown == nil {
			return fmt.Errorf("ze's outbound SA %s -> %s carries no options-known key; answer: %s", zeIP, swanIP, answer)
		}
		if !*record.OptionsKnown {
			return fmt.Errorf("ze's outbound SA %s -> %s reports options-known false: the dataplane did not read its options back; answer: %s", zeIP, swanIP, answer)
		}
		if record.DFBit == nil {
			return fmt.Errorf("ze's outbound SA %s -> %s reports df-bit null or carries no df-bit key; answer: %s", zeIP, swanIP, answer)
		}
		if *record.DFBit != leaf {
			return fmt.Errorf("ze reports df-bit %q on its outbound SA %s -> %s, want %q; answer: %s", *record.DFBit, zeIP, swanIP, leaf, answer)
		}
	}
	if outbound == 0 {
		return fmt.Errorf("ze reports no outbound SA %s -> %s; answer: %s", zeIP, swanIP, answer)
	}
	return nil
}
