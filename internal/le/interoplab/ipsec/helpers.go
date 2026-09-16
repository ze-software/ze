// Design: docs/architecture/testing/interop.md -- fail-closed strongSwan and Ze queries with bounded observations.
// Related: ipsec.go -- topology and rendered configuration.
// Related: checkers.go -- protocol assertions built from these typed operations.
// RFC: rfc/short/rfc7296.md -- transport mode NAT traversal (Section 2.23.1)
package ipsec

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/interoplab"
)

const (
	zePeer   = "ze"
	swanPeer = "strongswan"
	frrPeer  = "frr"
	natPeer  = "nat"

	// swanConfigPeer is the name every scenario's ze.conf gives the strongswan
	// peer, so it is what ze's own output and metrics call that peer. swanPeer
	// above names the lab container, which is a different string.
	swanConfigPeer = "swan"

	// xfrmCommand is the ip(8) subcommand that reads and writes the kernel's
	// XFRM state and policy databases.
	xfrmCommand = "xfrm"

	zeIP   = "172.28.0.2"
	swanIP = "172.28.0.3"
	frrIP  = "172.28.0.4"

	// natHost is the NAT box's host octet on the lab network. It sits after FRR so a
	// scenario that wants both keeps its addresses.
	natHost = 5
	// natIP is the NAT box's own primary address, natHost on the lab network. It is
	// the one address the box answers for itself rather than forwards, which is what
	// makes it the reference address of the sizing scenario.
	natIP = "172.28.0.5"

	// zePublicIP and swanPublicIP are the two secondary addresses the NAT box owns,
	// and they are the addresses each peer sees the OTHER at.
	//
	// RFC 7296 Section 2.23.1 calls them IPN1 and IPN2. A scenario's nat.conf declares
	// the mapping and this pair is what every checker of a NAT scenario asserts
	// against, so the two cannot drift.
	zePublicIP   = "172.28.0.6"
	swanPublicIP = "172.28.0.7"

	logLinesMax = 10000

	childWaitTimeout    = 30 * time.Second
	selectorWaitTimeout = 90 * time.Second
	xfrmWaitTimeout     = 30 * time.Second
	flowWaitTimeout     = 60 * time.Second
	probeWaitTimeout    = 20 * time.Second

	swanConnection = "ze"
)

var (
	pkiPlaceholder = regexp.MustCompile(`%%PKI_B64:([A-Za-z0-9._-]+)%%`)
	pemHeader      = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9-]*:`)
	spiPattern     = regexp.MustCompile(`proto \w+ spi (0x[0-9a-fA-F]+)`)
	espSPIPattern  = regexp.MustCompile(`proto esp spi (0x[0-9a-fA-F]+)`)
	xfrmBytes      = regexp.MustCompile(`(\d+)\(bytes\)`)
	srcDstPattern  = regexp.MustCompile(`^src (\S+) dst (\S+)`)

	pingLossPattern = regexp.MustCompile(`(\d+)% packet loss`)
)

type scenarioLab struct {
	check   *interoplab.CheckContext
	timeout time.Duration
	state   *scenarioState
}

func newScenarioLab(check *interoplab.CheckContext, timeout time.Duration, state *scenarioState) *scenarioLab {
	return &scenarioLab{check: check, timeout: timeout, state: state}
}

func (l *scenarioLab) exec(ctx context.Context, peer string, command ...string) (string, error) {
	result, err := l.check.Lab.Exec(ctx, peer, command, nil)
	if err != nil {
		return result.Stdout, err
	}
	return result.Stdout, nil
}

func (l *scenarioLab) execQuiet(ctx context.Context, peer string, command ...string) string {
	result, _ := l.check.Lab.Exec(ctx, peer, command, nil)
	return result.Stdout
}

func (l *scenarioLab) query(ctx context.Context, peer string, command ...string) (string, error) {
	return l.check.Lab.Query(ctx, peer, command, nil)
}

func (l *scenarioLab) logs(ctx context.Context, peer string) (string, error) {
	result, err := l.check.Lab.Logs(ctx, peer, logLinesMax)
	if err != nil {
		return "", err
	}
	if !result.Available {
		return "", fmt.Errorf("%s logs were not read", peer)
	}
	if strings.TrimSpace(result.Text) == "" {
		return "", fmt.Errorf("%s logs are empty; checker read no peer output", peer)
	}
	return result.Text, nil
}

func (l *scenarioLab) waitLog(ctx context.Context, peer, needle string, timeout time.Duration) error {
	var tb textbuf.Buffer
	description := tb.Str(peer).Str(" log ").Quoted(needle).String()
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: timeout, Interval: 2 * time.Second, Description: description,
	}, func(probe context.Context) (string, error) {
		return l.logs(probe, peer)
	}, func(logs string) bool {
		return strings.Contains(logs, needle)
	})
	return err
}

func (l *scenarioLab) listSAs(ctx context.Context) (string, error) {
	return l.exec(ctx, swanPeer, "swanctl", "--list-sas")
}

func (l *scenarioLab) waitSA(ctx context.Context, timeout time.Duration) error {
	var tb textbuf.Buffer
	description := tb.Str("strongSwan SA ").Str(swanConnection).String()
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: timeout, Interval: 2 * time.Second, Description: description,
	}, func(probe context.Context) (string, error) {
		return l.listSAs(probe)
	}, func(output string) bool {
		if !strings.Contains(output, "ESTABLISHED") {
			return false
		}
		return strings.Contains(output, swanConnection)
	})
	return err
}

func (l *scenarioLab) waitChild(ctx context.Context, child string) error {
	var tb textbuf.Buffer
	description := tb.Str("strongSwan Child SA ").Str(child).String()
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: childWaitTimeout, Interval: 2 * time.Second, Description: description,
	}, func(probe context.Context) (string, error) {
		return l.listSAs(probe)
	}, func(output string) bool {
		if child == "" {
			return strings.Contains(output, "INSTALLED")
		}
		if !strings.Contains(output, child) {
			return false
		}
		return strings.Contains(output, "INSTALLED")
	})
	return err
}

func (l *scenarioLab) waitChildSelectors(ctx context.Context, local, remote string) error {
	var tb textbuf.Buffer
	localSelector := tb.Str("local  ").Str(local).String()
	remoteSelector := tb.Reset().Str("remote ").Str(remote).String()
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: selectorWaitTimeout, Interval: 2 * time.Second, Description: "strongSwan Child SA selectors",
	}, func(probe context.Context) (string, error) {
		return l.listSAs(probe)
	}, func(output string) bool {
		if !strings.Contains(output, localSelector) {
			return false
		}
		return strings.Contains(output, remoteSelector)
	})
	return err
}

func (l *scenarioLab) waitXFRM(ctx context.Context, peer string) (string, error) {
	return l.waitOutput(ctx, peer, []string{"ip", xfrmCommand, "state"}, xfrmWaitTimeout, "XFRM ESP state", func(output string) bool {
		return strings.Contains(output, "proto esp")
	})
}

func (l *scenarioLab) waitOutput(ctx context.Context, peer string, command []string, timeout time.Duration, description string, ready func(string) bool) (string, error) {
	output, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: timeout, Interval: 2 * time.Second, Description: description,
	}, func(probe context.Context) (string, error) {
		return l.exec(probe, peer, command...)
	}, ready)
	return output, err
}

func (l *scenarioLab) xfrmState(ctx context.Context, peer string) (string, error) {
	return l.exec(ctx, peer, "ip", xfrmCommand, "state")
}

// inboundXFRMState answers the ONE state that decapsulates what source sends to target.
//
// The filter matters. `ip xfrm state` prints both directions of a Child SA, so a template
// present on the OUTBOUND state alone satisfies an assertion made over the whole dump
// while the receive path carries nothing. Reception is what RFC 3948 Section 3.1.2
// governs, so the assertions that cite it read this state and no other.
func (l *scenarioLab) inboundXFRMState(ctx context.Context, peer, source, target string) (string, error) {
	output, err := l.exec(ctx, peer, "ip", xfrmCommand, "state", "list", "src", source, "dst", target)
	if err != nil {
		return "", err
	}
	if !strings.Contains(output, "proto esp") {
		return "", fmt.Errorf("%s carries no inbound ESP state for src %s dst %s: %s", peer, source, target, output)
	}
	return output, nil
}

func (l *scenarioLab) xfrmPolicy(ctx context.Context, peer string) (string, error) {
	output, err := l.exec(ctx, peer, "ip", xfrmCommand, "policy")
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(output) == "" {
		return "", fmt.Errorf("ip xfrm policy printed nothing in %s; checker read no policy state", peer)
	}
	return output, nil
}

// saKey identifies ONE security association by the endpoints and SPI its own
// `ip -s xfrm state` record prints.
//
// RFC 4301 Section 4.1: "An SA is a simplex \"connection\" that affords security
// services to the traffic carried by it." A protected bidirectional flow is two
// SAs, and the RECEIVER chooses the SPI, so the sender's outbound SA and the
// receiver's inbound SA carry the SAME SPI value. Direction can therefore come
// only from source and destination, and a map keyed by SPI alone folds the two
// peers' views of one direction into one entry.
type saKey struct {
	source string
	target string
	spi    string
}

func (l *scenarioLab) xfrmCounters(ctx context.Context, peer string) (map[saKey]uint64, error) {
	output, err := l.exec(ctx, peer, "ip", "-s", xfrmCommand, "state")
	if err != nil {
		return nil, err
	}
	counters, err := parseXFRMCounters(output)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", peer, err)
	}
	return counters, nil
}

// parseXFRMCounters reads the lifetime-current byte count of every SA in one
// peer's dump, keyed by the direction that SA carries.
//
// An SPI printed under no `src ... dst ...` header has no direction, and a
// zero-valued direction would compare equal to a real one, so the dump is
// refused rather than counted (`ai/rules/principles.md`).
func parseXFRMCounters(output string) (map[saKey]uint64, error) {
	counters := make(map[saKey]uint64)
	key := saKey{}
	current := false
	for line := range strings.SplitSeq(output, "\n") {
		if line != "" && line[0] != ' ' && line[0] != '\t' {
			key = saKey{}
			current = false
			if match := srcDstPattern.FindStringSubmatch(line); match != nil {
				key.source = match[1]
				key.target = match[2]
			}
			continue
		}
		if match := spiPattern.FindStringSubmatch(line); match != nil {
			if key.source == "" || key.target == "" {
				return nil, fmt.Errorf("ip -s xfrm state printed spi %s under no src/dst header, so its direction is unknown: %s", match[1], output)
			}
			key.spi = match[1]
			current = false
			continue
		}
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "lifetime current") {
			current = true
			continue
		}
		if strings.HasPrefix(trimmed, "lifetime config") || strings.HasPrefix(trimmed, "stats") {
			current = false
			continue
		}
		if !current || key.spi == "" {
			continue
		}
		for _, match := range xfrmBytes.FindAllStringSubmatch(line, -1) {
			value, err := strconv.ParseUint(match[1], 10, 64)
			if err == nil {
				counters[key] += value
			}
		}
	}
	return counters, nil
}

// espDirection names ONE simplex SA: the peer whose kernel reports it, and the
// endpoints whose traffic it protects. summary is what a reader is told when
// that SA moved no bytes.
type espDirection struct {
	peer    string
	source  string
	target  string
	summary string
}

var (
	zeEncrypts   = espDirection{peer: zePeer, source: zeIP, target: swanIP, summary: "Ze encrypted nothing toward strongSwan"}
	swanDecrypts = espDirection{peer: swanPeer, source: zeIP, target: swanIP, summary: "strongSwan accepted no ESP from Ze"}
	swanEncrypts = espDirection{peer: swanPeer, source: swanIP, target: zeIP, summary: "strongSwan encrypted nothing toward Ze"}
	zeDecrypts   = espDirection{peer: zePeer, source: swanIP, target: zeIP, summary: "Ze decrypted no ESP from strongSwan"}
)

// espBothDirections follows one round trip through the tunnel: Ze encrypts,
// strongSwan decrypts, strongSwan encrypts, Ze decrypts. No single packet
// satisfies all four, which is why they are claimed separately.
var espBothDirections = []espDirection{zeEncrypts, swanDecrypts, swanEncrypts, zeDecrypts}

// directionCounters selects the SAs one peer reports for one direction.
func directionCounters(counters map[saKey]uint64, want espDirection) map[string]uint64 {
	selected := make(map[string]uint64)
	for key, bytes := range counters {
		if key.source == want.source && key.target == want.target {
			selected[key.spi] = bytes
		}
	}
	return selected
}

func sortedCounters(counters map[string]uint64) string {
	spis := make([]string, 0, len(counters))
	for spi := range counters {
		spis = append(spis, spi)
	}
	slices.Sort(spis)
	var out textbuf.Buffer
	for index, spi := range spis {
		if index > 0 {
			out.Byte(' ')
		}
		out.Str(spi).Byte('=').Int(int64(counters[spi]))
	}
	return out.String()
}

// assertESPAdvanced reports whether the ONE SA the direction names moved bytes
// between two snapshots of its peer's dump.
//
// Only SPIs present in BOTH snapshots are compared, so a rekey that retires an
// SA between the two reads does not fail the check. The three failures are told
// apart on purpose: a direction with no SA at all, a direction whose every SA
// was retired, and a direction whose surviving SA did not move.
func assertESPAdvanced(before, after map[saKey]uint64, want espDirection) error {
	beforeBytes := directionCounters(before, want)
	if len(beforeBytes) == 0 {
		return fmt.Errorf("%s: %s reports no SA for src %s dst %s", want.summary, want.peer, want.source, want.target)
	}
	afterBytes := directionCounters(after, want)
	common := make([]string, 0, len(beforeBytes))
	for spi, previous := range beforeBytes {
		latest, ok := afterBytes[spi]
		if !ok {
			continue
		}
		common = append(common, spi)
		if latest > previous {
			return nil
		}
	}
	if len(common) == 0 {
		return fmt.Errorf("%s: no surviving SA for src %s dst %s at %s (before=%s after=%s)",
			want.summary, want.source, want.target, want.peer,
			sortedCounters(beforeBytes), sortedCounters(afterBytes))
	}
	slices.Sort(common)
	return fmt.Errorf("%s: src %s dst %s at %s did not advance (before=%s after=%s, common SPIs %v)",
		want.summary, want.source, want.target, want.peer,
		sortedCounters(beforeBytes), sortedCounters(afterBytes), common)
}

func (l *scenarioLab) checkXFRMCount(ctx context.Context, peer string, expected int) error {
	output, err := l.xfrmState(ctx, peer)
	if err != nil {
		return err
	}
	count := strings.Count(output, "proto esp")
	if count != expected {
		return fmt.Errorf("%s XFRM SA count %d != %d", peer, count, expected)
	}
	return nil
}

// pingLoss reads the loss percentage out of a ping summary.
//
// An absent summary is a FAILURE and never a pass. A run that printed no summary
// measured nothing, so reading the missing match as success would make the check
// answer for a ping that never reported (`ai/rules/principles.md`).
func pingLoss(output string) (int, error) {
	match := pingLossPattern.FindStringSubmatch(output)
	if match == nil {
		return 0, fmt.Errorf("printed no packet-loss summary: %s", output)
	}
	loss, err := strconv.Atoi(match[1])
	if err != nil {
		return 0, fmt.Errorf("printed an unreadable packet-loss percentage %q: %s", match[1], output)
	}
	return loss, nil
}

// requireLosslessPing drives count echo requests from peer to target and refuses
// any loss, and any output whose summary it cannot read.
func (l *scenarioLab) requireLosslessPing(ctx context.Context, peer, target string, count int) error {
	return l.requireLosslessPingFrom(ctx, pingProbe{peer: peer, target: target}, count)
}

// requireLosslessPingFrom is requireLosslessPing with an optional source address, so a
// tunnel-mode flow can be stimulated from the inner address its policy selects on.
func (l *scenarioLab) requireLosslessPingFrom(ctx context.Context, probe pingProbe, count int) error {
	output := l.execQuiet(ctx, probe.peer, pingCommand(probe, count)...)
	loss, err := pingLoss(output)
	if err != nil {
		return fmt.Errorf("ping from %s to %s %w", probe.peer, probe.target, err)
	}
	if loss != 0 {
		return fmt.Errorf("ping from %s to %s lost %d%% of %d packets: %s", probe.peer, probe.target, loss, count, output)
	}
	return nil
}

// espCounters reads each claimed peer's SA byte counters once, in the order the
// directions name them.
func (l *scenarioLab) espCounters(ctx context.Context, wanted []espDirection) (map[string]map[saKey]uint64, error) {
	counters := make(map[string]map[saKey]uint64, len(wanted))
	for _, want := range wanted {
		if _, seen := counters[want.peer]; seen {
			continue
		}
		peerCounters, err := l.xfrmCounters(ctx, want.peer)
		if err != nil {
			return nil, err
		}
		counters[want.peer] = peerCounters
	}
	return counters, nil
}

// verifyTunnelTraffic proves that ESP moved in BOTH directions and that the ping
// which stimulated it completed without loss.
func (l *scenarioLab) verifyTunnelTraffic(ctx context.Context, message string) error {
	return l.verifyESPDirections(ctx, message, espBothDirections)
}

// verifyESPDirections proves that ESP bytes moved on every simplex SA the caller
// claims, and that a ping between the two peers completed without loss.
//
// The ping verdict is necessary and it is NOT sufficient. charon's bypass-lan
// plugin installs a PASS shunt for every locally attached subnet, and a shunt is
// exactly what lets an UNPROTECTED ping succeed, so reachability says nothing
// about protection. The directed counters say what was protected, and the ping
// ties those bytes to a completed round trip.
//
// The claimed set is a parameter because checkESPFormChange cannot claim Ze's
// inbound KERNEL SA. That scenario exists on the two peers disagreeing about ESP
// form, so Ze receives that ESP in userspace and the kernel state correctly stays
// still.
func (l *scenarioLab) verifyESPDirections(ctx context.Context, message string, wanted []espDirection) error {
	return l.verifyESPDirectionsToward(ctx, message, pingProbe{peer: zePeer, target: swanIP}, wanted)
}

// pingProbe is the round trip that stimulates the ESP the caller then measures. The
// source matters behind a NAT and inside a tunnel: a peer reaches the far end at the
// address IT dials, which is the translated one, and a tunnel-mode flow has to leave
// from the inner address the policy selects on.
type pingProbe struct {
	peer   string
	target string
	source string
	// size is the ICMP payload in octets (ping -s), and zero leaves ping's default.
	// A size sets Don't Fragment too (ping -M do), because a sized probe exists to
	// learn whether the path CARRIES that size rather than whether it fragments it.
	size int
}

// verifyESPDirectionsToward is verifyESPDirections with the stimulating round trip
// named by the caller. The default probe pings the peer's own address, which is right
// for every scenario with no middlebox and wrong for every scenario with one.
func (l *scenarioLab) verifyESPDirectionsToward(ctx context.Context, message string, probe pingProbe, wanted []espDirection) error {
	if len(wanted) == 0 {
		return fmt.Errorf("%s: the checker claimed no ESP direction, so nothing was observed", message)
	}
	before, err := l.espCounters(ctx, wanted)
	if err != nil {
		return err
	}
	if err := l.requireLosslessPingFrom(ctx, probe, 4); err != nil {
		return fmt.Errorf("%s: %w", message, err)
	}
	after, err := l.espCounters(ctx, wanted)
	if err != nil {
		return err
	}
	for _, want := range wanted {
		if err := assertESPAdvanced(before[want.peer], after[want.peer], want); err != nil {
			return fmt.Errorf("%s: %w", message, err)
		}
	}
	return nil
}

func (l *scenarioLab) espSPIs(ctx context.Context, peer string) (map[string]struct{}, error) {
	output, err := l.xfrmState(ctx, peer)
	if err != nil {
		return nil, err
	}
	matches := espSPIPattern.FindAllStringSubmatch(output, -1)
	spis := make(map[string]struct{}, len(matches))
	for _, match := range matches {
		spis[match[1]] = struct{}{}
	}
	return spis, nil
}

func sameStrings(first, second map[string]struct{}) bool {
	if len(first) != len(second) {
		return false
	}
	for value := range first {
		if _, ok := second[value]; !ok {
			return false
		}
	}
	return true
}

func newStrings(before, after map[string]struct{}) bool {
	for value := range after {
		if _, ok := before[value]; !ok {
			return true
		}
	}
	return false
}

func (l *scenarioLab) espPolicyPairs(ctx context.Context, peer string) (map[string]struct{}, error) {
	output, err := l.xfrmPolicy(ctx, peer)
	if err != nil {
		return nil, err
	}
	pairs := make(map[string]struct{})
	source := ""
	destination := ""
	hasESP := false
	var tb textbuf.Buffer
	keep := func() {
		if source == "" || !hasESP {
			return
		}
		pair := []string{source, destination}
		slices.Sort(pair)
		key := tb.Reset().Str(pair[0]).Byte('|').Str(pair[1]).String()
		pairs[key] = struct{}{}
	}
	for line := range strings.SplitSeq(output, "\n") {
		if match := srcDstPattern.FindStringSubmatch(line); match != nil {
			keep()
			source = match[1]
			destination = match[2]
			hasESP = false
			continue
		}
		if source != "" && strings.Contains(line, "proto esp") {
			hasESP = true
		}
	}
	keep()
	return pairs, nil
}

func policyPair(first, second string) string {
	values := []string{first, second}
	slices.Sort(values)
	var tb textbuf.Buffer
	return tb.Str(values[0]).Byte('|').Str(values[1]).String()
}

func (l *scenarioLab) waitPolicyPair(ctx context.Context, peer, first, second string, timeout time.Duration) error {
	expected := policyPair(first, second)
	var tb textbuf.Buffer
	description := tb.Str(peer).Str(" ESP policy ").Str(expected).String()
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: timeout, Interval: 2 * time.Second, Description: description,
	}, func(probe context.Context) (map[string]struct{}, error) {
		return l.espPolicyPairs(probe, peer)
	}, func(pairs map[string]struct{}) bool {
		_, ok := pairs[expected]
		return ok && len(pairs) == 1
	})
	return err
}

func (l *scenarioLab) zeCLI(ctx context.Context, command string) (string, error) {
	return l.query(ctx, zePeer, zeCLICommand(command)...)
}

// zeCLICommand builds the argv that asks the ze daemon one question. It is separate
// from the query so a test scripting a lab answers the same argv the checker sends,
// rather than a second spelling of it that can drift (ai/rules/principles.md).
func zeCLICommand(command string) []string {
	var tb textbuf.Buffer
	seed := tb.Str("printf '%s\\n%s\\n127.0.0.1\\n%s\\n' ").Str(zeCLIUser).Byte(' ').
		Str(zeCLIPassword).Byte(' ').Str(zeCLIPort).Str(" | ZE_CONFIG_DIR=").
		Str(zeCLIStore).Str(" ze init").String()
	run := tb.Reset().Str("ZE_CONFIG_DIR=").Str(zeCLIStore).Str(" ZE_SSH_PASSWORD=").
		Str(zeCLIPassword).Str(" ze cli -c ").Str(shellQuote(command)).String()
	shell := tb.Reset().Str("[ -d ").Str(zeCLIStore).Str(" ] || ").Str(seed).
		Str(" >/dev/null; ").Str(run).String()
	return []string{"sh", "-c", shell}
}

func shellQuote(value string) string {
	var tb textbuf.Buffer
	tb.Byte('\'')
	for index := range len(value) {
		if value[index] == '\'' {
			tb.Str("'\\''")
			continue
		}
		tb.Byte(value[index])
	}
	return tb.Byte('\'').String()
}

// assertZeSelectors refuses unless ONE Child SA of `show vpn ipsec sa` carries both
// selectors. The pair is asserted on one Child SA rather than anywhere in the answer,
// because a local half from one SA and a remote half from another describe a tunnel
// that does not exist.
func (l *scenarioLab) assertZeSelectors(ctx context.Context, local, remote string) error {
	records, answer, err := l.zeIKESAs(ctx)
	if err != nil {
		return err
	}
	for _, record := range records {
		child, ok := record["child-sa"].(map[string]any)
		if !ok {
			continue
		}
		if child["ts-local"] != local {
			continue
		}
		if child["ts-remote"] == remote {
			return nil
		}
	}
	return fmt.Errorf("show vpn ipsec sa reports no child sa with ts-local %s and ts-remote %s; answer: %s",
		local, remote, answer)
}

// zeIKEEncryption returns the encryption transform ze negotiated for its IKE SA with
// one peer, read out of the structured answer of `show vpn ipsec sa`.
func (l *scenarioLab) zeIKEEncryption(ctx context.Context, peer string) (string, error) {
	records, answer, err := l.zeIKESAs(ctx)
	if err != nil {
		return "", err
	}
	encryption, err := ikeEncryptionOf(records, peer)
	if err != nil {
		return "", fmt.Errorf("%w; answer: %s", err, answer)
	}
	return encryption, nil
}

// ikeEncryptionOf reads one peer's negotiated IKE encryption transform out of the
// records `show vpn ipsec sa | json` answered. The value is what
// crypto.EncryptionID.String renders for the Transform ID the peers agreed on, so it
// names the wire identity rather than the config keyword that asked for it.
//
// It is a pure predicate so a test drives both polarities with no lab, and it refuses
// four answers rather than returning a string for them: no record for the peer, a
// record whose `encryption` key is absent, one whose value is not a string, and two
// records for the peer that disagree. Each of those would otherwise answer "" or an
// arbitrary half, and an empty transform name compares unequal to every transform, so
// a failure to READ the daemon would read as a verdict ABOUT it
// (ai/rules/principles.md).
func ikeEncryptionOf(records []map[string]any, peer string) (string, error) {
	found := ""
	for _, record := range records {
		if record["peer-name"] != peer {
			continue
		}
		encryption, ok := record["encryption"].(string)
		if !ok {
			return "", fmt.Errorf("the IKE SA for %s carries no encryption transform", peer)
		}
		if encryption == "" {
			return "", fmt.Errorf("the IKE SA for %s reports an empty encryption transform", peer)
		}
		if found != "" && found != encryption {
			return "", fmt.Errorf("the IKE SAs for %s disagree on the encryption transform: %s and %s",
				peer, found, encryption)
		}
		found = encryption
	}
	if found == "" {
		return "", fmt.Errorf("show vpn ipsec sa reports no IKE SA for %s", peer)
	}
	return found, nil
}

// zeChildESPEncryption returns the ESP encryption transform ze reports for the Child
// SA of one peer, read out of the structured answer of `show vpn ipsec sa`.
func (l *scenarioLab) zeChildESPEncryption(ctx context.Context, peer string) (string, error) {
	records, answer, err := l.zeIKESAs(ctx)
	if err != nil {
		return "", err
	}
	encryption, err := childESPEncryptionOf(records, peer)
	if err != nil {
		return "", fmt.Errorf("%w; answer: %s", err, answer)
	}
	return encryption, nil
}

// childESPEncryptionOf reads one peer's Child SA esp-encryption out of the records
// `show vpn ipsec sa | json` answered. The value is what ipsec.EncryptionAlgo.String
// renders for the proposal the peer accepted (engine.PeerInfo.ESPEncryption).
//
// It is a pure predicate with the same refusals as ikeEncryptionOf: no record for the
// peer, a record with no child-sa, a child-sa with no string esp-encryption, an empty
// one, and two Child SAs that disagree each return an error rather than "", because an
// empty transform compares unequal to every name and a failure to READ the daemon
// would read as a verdict ABOUT it (ai/rules/principles.md).
func childESPEncryptionOf(records []map[string]any, peer string) (string, error) {
	found := ""
	for _, record := range records {
		if record["peer-name"] != peer {
			continue
		}
		child, ok := record["child-sa"].(map[string]any)
		if !ok {
			return "", fmt.Errorf("the IKE SA for %s carries no child-sa", peer)
		}
		encryption, ok := child["esp-encryption"].(string)
		if !ok {
			return "", fmt.Errorf("the Child SA for %s carries no esp-encryption transform", peer)
		}
		if encryption == "" {
			return "", fmt.Errorf("the Child SA for %s reports an empty esp-encryption transform", peer)
		}
		if found != "" && found != encryption {
			return "", fmt.Errorf("the Child SAs for %s disagree on the esp-encryption transform: %s and %s",
				peer, found, encryption)
		}
		found = encryption
	}
	if found == "" {
		return "", fmt.Errorf("show vpn ipsec sa reports no IKE SA for %s", peer)
	}
	return found, nil
}

// zeIKESAs returns the IKE SAs `show vpn ipsec sa` reports, and the answer they were
// decoded from.
//
// It reads the command's STRUCTURED answer, which is the only shape a checker can
// assert on. The text rendering is a table whose column order follows the field names
// (ApplyPipes, internal/component/command/pipe.go), so a line-anchored `<field> <value>`
// regex over it matches by accident: adding one field re-sorts the columns and moves
// every nested Child SA key off the line start. That is what it did on 2026-09-06, when
// `behind-nat` took first place from `child-sa` and turned three green scenarios red
// with no daemon behavior changed.
//
// `| json` unwraps the single-key `peers` envelope into the list of SA records
// (unwrapSingleKeyArray, pipe.go), so the answer decodes as a list. A shape that does
// not decode is an error naming the answer, never an empty list: a checker that read
// zero SAs out of an unparsed answer would pass every "no SA reports X" assertion.
func (l *scenarioLab) zeIKESAs(ctx context.Context) ([]map[string]any, string, error) {
	answer, err := l.zeCLI(ctx, "show vpn ipsec sa | json")
	if err != nil {
		return nil, answer, err
	}
	records, err := decodeIKESAs(answer)
	return records, answer, err
}

// decodeIKESAs decodes the answer of `show vpn ipsec sa | json` into its SA records.
//
// It is separate from the query so a test can drive it with an answer the lab really
// produced, including the TEXT rendering the checkers read until 2026-09-06.
func decodeIKESAs(answer string) ([]map[string]any, error) {
	var records []map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(answer)), &records); err != nil {
		return nil, fmt.Errorf("show vpn ipsec sa | json is not a list of IKE SAs: %w; answer: %s", err, answer)
	}
	return records, nil
}

func (l *scenarioLab) reloadZe(ctx context.Context, source string) error {
	if l.state.renderedConfig == "" {
		return errors.New("scenario rendered config path is empty")
	}
	pkiDir := findPKIDir(l.state.root, l.check.Source.Directory)
	if err := renderZeConfig(source, pkiDir, l.state.renderedConfig); err != nil {
		return err
	}
	return l.check.Lab.Signal(ctx, zePeer, "HUP")
}

func (l *scenarioLab) breakLink(ctx context.Context) error {
	_, err := l.exec(ctx, swanPeer, "iptables", "-I", "OUTPUT", "1", "-d", zeIP, "-j", "DROP")
	return err
}

func (l *scenarioLab) restoreLink(ctx context.Context) {
	l.execQuiet(ctx, swanPeer, "iptables", "-D", "OUTPUT", "-d", zeIP, "-j", "DROP")
}

func (l *scenarioLab) frrOutput(ctx context.Context, command string) (string, error) {
	return l.exec(ctx, frrPeer, "vtysh", "-c", command)
}

func (l *scenarioLab) waitFRRSession(ctx context.Context, neighbor string) error {
	var tb textbuf.Buffer
	description := tb.Str("FRR session ").Str(neighbor).String()
	command := tb.Reset().Str("show bgp neighbor ").Str(neighbor).String()
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: l.timeout, Interval: 2 * time.Second, Description: description,
	}, func(probe context.Context) (string, error) {
		return l.frrOutput(probe, command)
	}, func(output string) bool {
		return strings.Contains(output, "BGP state = Established")
	})
	return err
}

func frrHasRoute(output string) bool {
	if strings.TrimSpace(output) == "" {
		return false
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		return false
	}
	if _, ok := payload["paths"]; ok {
		return true
	}
	if _, ok := payload["prefix"]; ok {
		return true
	}
	for _, value := range payload {
		entry, ok := value.(map[string]any)
		if !ok {
			continue
		}
		if _, ok := entry["paths"]; ok {
			return true
		}
		if _, ok := entry["prefix"]; ok {
			return true
		}
	}
	return false
}

func (l *scenarioLab) waitFRRRoute(ctx context.Context, prefix string, present bool) error {
	var tb textbuf.Buffer
	description := tb.Str("FRR route ").Str(prefix).String()
	command := tb.Reset().Str("show bgp ipv4 unicast ").Str(prefix).Str(" json").String()
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: 30 * time.Second, Interval: 2 * time.Second, Description: description,
	}, func(probe context.Context) (string, error) {
		result, execErr := l.check.Lab.Exec(probe, frrPeer, []string{"vtysh", "-c", command}, nil)
		if execErr != nil {
			return "", execErr
		}
		return result.Stdout, nil
	}, func(output string) bool {
		return frrHasRoute(output) == present
	})
	return err
}

// snmpCounters reads one peer's per-namespace IP stack counters from /proc/net/snmp.
//
// The file states each protocol twice: a header line naming the fields, then a value
// line in the same order. The map is keyed "Protocol.Field", so "Udp.InCsumErrors" and
// "Tcp.OutRsts" name themselves at the call site.
//
// A read that produces no pair is an ERROR rather than an empty map. Every delta taken
// from an empty map is zero, and zero is what the negative assertions below expect
// (ai/rules/evidence.md).
func (l *scenarioLab) snmpCounters(ctx context.Context, peer string) (map[string]uint64, error) {
	output, err := l.exec(ctx, peer, "cat", "/proc/net/snmp")
	if err != nil {
		return nil, err
	}
	counters, err := parseSNMPCounters(output)
	if err != nil {
		return nil, fmt.Errorf("%s /proc/net/snmp: %w", peer, err)
	}
	return counters, nil
}

// parseSNMPCounters turns the header-and-value line pairs of /proc/net/snmp into one
// "Protocol.Field" map. The loop is bounded by the file the kernel writes.
func parseSNMPCounters(output string) (map[string]uint64, error) {
	counters := make(map[string]uint64)
	headers := make(map[string][]string)
	var tb textbuf.Buffer
	for line := range strings.SplitSeq(output, "\n") {
		protocol, rest, ok := strings.Cut(line, ": ")
		if !ok {
			continue
		}
		fields := strings.Fields(rest)
		header, seen := headers[protocol]
		if !seen {
			headers[protocol] = fields
			continue
		}
		for i, field := range fields {
			if i >= len(header) {
				break
			}
			value, err := strconv.ParseUint(field, 10, 64)
			if err != nil {
				continue
			}
			counters[tb.Reset().Str(protocol).Byte('.').Str(header[i]).String()] = value
		}
		delete(headers, protocol)
	}
	if len(counters) == 0 {
		return nil, errors.New("no header and value pair was read")
	}
	return counters, nil
}

// counterDelta reports how far one counter moved between two snapshots. The second
// result is false when either snapshot lacks the name, so a misspelled counter cannot
// read as "it did not move".
func counterDelta(before, after map[string]uint64, name string) (uint64, bool) {
	earlier, okEarlier := before[name]
	later, okLater := after[name]
	if !okEarlier || !okLater {
		return 0, false
	}
	if later < earlier {
		return 0, true
	}
	return later - earlier, true
}

// craftedProbe sends exactly one crafted IPv4 datagram from strongSwan through a raw IP
// socket, so the packet takes the kernel routing and XFRM path. A target of zeIP is
// protected by the Child SA; a target of swanIP is the unprotected control that never
// meets an XFRM policy.
//
// --send-ip is written out at the call site because nping otherwise prefers the Ethernet
// layer, and a packet sent there bypasses IPsec and makes every assertion vacuous.
//
// badChecksum asks nping for a deliberately wrong TCP/UDP checksum, which is what a NAT
// leaves behind on a transport-mode flow and what RFC 3948 Section 3.1.2 governs.
func (l *scenarioLab) craftedProbe(ctx context.Context, protocol string, port int, target string, badChecksum bool) (string, error) {
	command := []string{
		"nping", "--send-ip", "--no-capture", "--count", "1", "--rate", "1",
		"--" + protocol, "--dest-port", strconv.Itoa(port),
	}
	if protocol == "tcp" {
		command = append(command, "--flags", "syn")
	}
	if badChecksum {
		command = append(command, "--badsum")
	}
	command = append(command, target)
	output, err := l.exec(ctx, swanPeer, command...)
	if err != nil {
		return output, fmt.Errorf("nping %s to %s badsum=%v on strongSwan: %w: %s", protocol, target, badChecksum, err, output)
	}
	return output, nil
}

// deliverMarker carries nattMarker from strongSwan to Ze over the Child SA and reports
// whether Ze received it byte for byte.
//
// The listener is DETACHED because it outlives the exec that starts it, and the send is
// retried inside the bounded wait so the first datagram cannot race the bind.
func (l *scenarioLab) deliverMarker(ctx context.Context, protocol string, port int) error {
	options := ""
	if protocol == "udp" {
		options = "-u "
	}
	number := strconv.Itoa(port)
	file := "/run/ze-flow-" + protocol + ".txt"
	listen := "rm -f " + file + "; nc " + options + "-l -p " + number + " > " + file + " 2>&1 < /dev/null"
	send := "printf '%s\\n' " + shellQuote(nattMarker) + " | nc " + options + "-w 3 " + zeIP + " " + number
	read := []string{"sh", "-c", "cat " + file + " 2>/dev/null; echo"}

	if err := l.check.Lab.ExecDetached(ctx, zePeer, []string{"sh", "-c", listen}, nil); err != nil {
		return fmt.Errorf("start the %s listener on ze: %w", protocol, err)
	}
	var tb textbuf.Buffer
	description := tb.Str("ze receives the ").Str(protocol).Str(" marker over the Child SA").String()
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: flowWaitTimeout, Interval: 3 * time.Second, Description: description,
	}, func(probe context.Context) (string, error) {
		l.execQuiet(probe, swanPeer, "sh", "-c", send)
		return l.exec(probe, zePeer, read...)
	}, func(received string) bool {
		return strings.Contains(received, nattMarker)
	})
	return err
}

// The four simplex SAs of a scenario whose peers are BOTH translated by the NAT box.
//
// Each end's kernel names the OUTER addresses of its own Child SA, and behind a NAT the
// two ends disagree about them: Ze's states run between its own address and the address
// it dials, and strongSwan's run between its own address and the address it sees Ze at.
// A direction written with the untranslated pair matches no state, which reads as "the SA
// moved nothing" rather than as "the checker looked in the wrong place".
var (
	zeEncryptsNAT   = espDirection{peer: zePeer, source: zeIP, target: swanPublicIP, summary: "Ze encrypted nothing toward the address it dials"}
	swanDecryptsNAT = espDirection{peer: swanPeer, source: zePublicIP, target: swanIP, summary: "strongSwan accepted no ESP from the address it sees Ze at"}
	swanEncryptsNAT = espDirection{peer: swanPeer, source: swanIP, target: zePublicIP, summary: "strongSwan encrypted nothing toward the address it sees Ze at"}
	zeDecryptsNAT   = espDirection{peer: zePeer, source: swanPublicIP, target: zeIP, summary: "Ze decrypted no ESP from the address it dials"}
)

// natESPDirections follows one round trip across the translated path.
var natESPDirections = []espDirection{zeEncryptsNAT, swanDecryptsNAT, swanEncryptsNAT, zeDecryptsNAT}

// natVerdictFields are the three NAT facts `show vpn ipsec sa` reports, and every one of
// them is asserted on ONE IKE SA.
var natVerdictFields = []string{"nat-detected", "behind-nat", "peer-behind-nat"}

// assertNATVerdict refuses unless Ze recorded a NAT on the path AND recorded which side
// each translation is on.
//
// RFC 7296 Section 2.23.1 is written per side: the TSi address is substituted when the
// client is behind a NAT and the TSr address when the server is. A scenario that asserted
// only nat-detected would pass while the two side fields stayed false, and the
// substitution would be a no-op with a green bar over it.
//
// It is the operator-visible half of the verdict: an SA that established with the wrong
// verdict recorded would still pass every reachability assertion.
func (l *scenarioLab) assertNATVerdict(ctx context.Context) error {
	records, answer, err := l.zeIKESAs(ctx)
	if err != nil {
		return err
	}
	for _, record := range records {
		if saFlagsTrue(record, natVerdictFields) {
			return nil
		}
	}
	return fmt.Errorf("show vpn ipsec sa reports no IKE SA with %s all true; answer: %s",
		strings.Join(natVerdictFields, ", "), answer)
}

// saFlagsTrue reports whether one IKE SA record carries every named field as the boolean
// true.
//
// A field the answer does not carry is NOT true. `show vpn ipsec sa` declares all three
// NAT fields on every record (saToMap, internal/component/ike/cmd/show_ipsec.go), so an
// absent one is a changed contract rather than a false verdict, and the caller's error
// carries the whole answer for the reader to see which it was.
func saFlagsTrue(record map[string]any, fields []string) bool {
	for _, field := range fields {
		value, ok := record[field].(bool)
		if !ok {
			return false
		}
		if !value {
			return false
		}
	}
	return true
}

// natInnerAddress installs one inner address on a peer's loopback and the host route that
// steers traffic for the far inner address out of the lab interface.
//
// The tunnel control needs addresses the NAT does NOT translate, because tunnel mode
// carries an inner header the middlebox never sees. Its selectors are therefore the only
// ones in the lab a translation cannot move, which is exactly what makes it the control:
// a substitution that leaked into tunnel mode would change them and nothing else would.
func (l *scenarioLab) natInnerAddress(ctx context.Context, peer, local, remote string) error {
	if _, err := l.exec(ctx, peer, "ip", "address", "replace", local+"/32", "dev", "lo"); err != nil {
		return fmt.Errorf("install inner address %s on %s: %w", local, peer, err)
	}
	if _, err := l.exec(ctx, peer, "ip", "route", "replace", remote+"/32", "dev", "eth0", "src", local); err != nil {
		return fmt.Errorf("install inner route to %s on %s: %w", remote, peer, err)
	}
	return nil
}

// zeDataplaneSA is one record of `show vpn ipsec dataplane sa | json`, decoded
// into the types the kernel holds it in.
//
// It is a TYPED struct rather than a map[string]any on purpose. Decoding into a
// map routes every JSON number through float64, and float64 carries 53 bits of
// mantissa: a uint32 survives that today and the type says nothing about it, so
// the next field that is a uint64 byte counter would round in silence. Naming
// the field uint32 makes the decoder do the conversion, and refuse an answer
// whose number does not fit.
type zeDataplaneSA struct {
	SPI  uint32 `json:"spi"`
	Src  string `json:"src"`
	Dst  string `json:"dst"`
	Mode string `json:"mode"`
}

// dataplaneSAs returns the kernel SAD as ZE reports it, through the Ze CLI in
// the ze container.
//
// This is the reader under test in the dataplane-readback scenario. The other
// reader is iproute2 in the same container, and the scenario exists to prove the
// two agree.
func (l *scenarioLab) dataplaneSAs(ctx context.Context) ([]zeDataplaneSA, string, error) {
	answer, err := l.zeCLI(ctx, "show vpn ipsec dataplane sa | json")
	if err != nil {
		return nil, answer, err
	}
	records, err := decodeZeDataplaneSAs(answer)
	return records, answer, err
}

// decodeZeDataplaneSAs decodes the answer of `show vpn ipsec dataplane sa | json`.
//
// `| json` unwraps the single-key `sas` envelope into the list of records
// (unwrapSingleKeyArray, internal/component/command/pipe.go), so the answer
// decodes as a list. An answer that does not decode is an ERROR naming it, never
// an empty list: a checker that read zero SAs out of an unparsed answer would
// pass every "the two readers agree" assertion it was given.
func decodeZeDataplaneSAs(answer string) ([]zeDataplaneSA, error) {
	var records []zeDataplaneSA
	if err := json.Unmarshal([]byte(strings.TrimSpace(answer)), &records); err != nil {
		return nil, fmt.Errorf("show vpn ipsec dataplane sa | json is not a list of SAs: %w; answer: %s", err, answer)
	}
	return records, nil
}

// decodeZeDataplaneSPIs answers the SPI set the Ze dump names.
func decodeZeDataplaneSPIs(answer string) (map[uint32]struct{}, error) {
	records, err := decodeZeDataplaneSAs(answer)
	if err != nil {
		return nil, err
	}
	spis := make(map[uint32]struct{}, len(records))
	for i := range records {
		spis[records[i].SPI] = struct{}{}
	}
	return spis, nil
}

// spiValues normalizes the SPI forms iproute2 prints into the numbers the Ze
// dump answers.
//
// iproute2 prints an SPI as `0xc1a2b3c4` and Ze answers a JSON number, so the
// two never compare as strings. An SPI that does not parse is an error naming
// it: dropping it would shrink the set silently, and a shrunken set is what an
// agreement assertion cannot tell from agreement.
func spiValues(printed map[string]struct{}) (map[uint32]struct{}, error) {
	values := make(map[uint32]struct{}, len(printed))
	for spi := range printed {
		value, err := strconv.ParseUint(strings.TrimPrefix(spi, "0x"), 16, 32)
		if err != nil {
			return nil, fmt.Errorf("ip xfrm state printed an ESP SPI this lab cannot read: %q: %w", spi, err)
		}
		values[uint32(value)] = struct{}{}
	}
	return values, nil
}

// espSPIValues answers one peer's ESP SPI set, read from iproute2 and normalized
// to the numbers the Ze dump answers.
func (l *scenarioLab) espSPIValues(ctx context.Context, peer string) (map[uint32]struct{}, error) {
	printed, err := l.espSPIs(ctx, peer)
	if err != nil {
		return nil, err
	}
	return spiValues(printed)
}

// requireSameSPISet refuses unless the two readers name the SAME NON-EMPTY set.
//
// The order is the whole point. Two empty sets are equal, so a comparison made
// first would hold over a kernel that holds nothing, which is what a read-only
// dump produces with its entire body deleted. The emptiness of either side is
// therefore an error BEFORE the sets are compared
// (ai/rules/interop-and-goal-validation.md).
func requireSameSPISet(first, second map[uint32]struct{}, firstName, secondName string) error {
	if len(first) == 0 {
		return fmt.Errorf("%s reports no ESP SPI, so an agreement with %s would be vacuous", firstName, secondName)
	}
	if len(second) == 0 {
		return fmt.Errorf("%s reports no ESP SPI, so an agreement with %s would be vacuous", secondName, firstName)
	}
	if !sameSPISet(first, second) {
		return fmt.Errorf("%s and %s disagree on the kernel SAD: %s reports %v, %s reports %v",
			firstName, secondName, firstName, sortedSPIs(first), secondName, sortedSPIs(second))
	}
	return nil
}

// requireSPISetChanged requires a nonempty replacement with every old SPI gone.
// Callers MUST poll through the RFC 7296 Section 2.8 cleanup window before
// treating an overlapping set as a failed rekey.
func requireSPISetChanged(before, after map[uint32]struct{}) error {
	if len(before) == 0 {
		return errors.New("no ESP SPI was observed before rekey")
	}
	if len(after) == 0 {
		return fmt.Errorf("the kernel SAD reports no ESP SPI after the rekey, so the change is unproven; before: %v", sortedSPIs(before))
	}
	for spi := range before {
		if _, remains := after[spi]; remains {
			return fmt.Errorf("old ESP SPI %#x remains after rekey: before %v, after %v", spi, sortedSPIs(before), sortedSPIs(after))
		}
	}
	return nil
}

func sameSPISet(first, second map[uint32]struct{}) bool {
	if len(first) != len(second) {
		return false
	}
	for spi := range first {
		if _, ok := second[spi]; !ok {
			return false
		}
	}
	return true
}

// sortedSPIs renders a set for an error message, in one order, so two failures
// of the same shape read the same.
func sortedSPIs(spis map[uint32]struct{}) []uint32 {
	out := make([]uint32, 0, len(spis))
	for spi := range spis {
		out = append(out, spi)
	}
	slices.Sort(out)
	return out
}

type zeChildCounters struct {
	InboundSPI    uint32  `json:"inbound-spi"`
	OutboundSPI   uint32  `json:"outbound-spi"`
	IfID          uint32  `json:"if-id"`
	BytesOut      *uint64 `json:"bytes-out"`
	BytesIn       *uint64 `json:"bytes-in"`
	PacketsOut    *uint64 `json:"packets-out"`
	PacketsIn     *uint64 `json:"packets-in"`
	CountersKnown bool    `json:"counters-known"`
}

type zeIKESARecord struct {
	Peer  string           `json:"peer-name"`
	Child *zeChildCounters `json:"child-sa"`
}

type readbackSAKey struct {
	source string
	target string
	spi    uint32
	ifID   uint32
}

type espLifetime struct {
	bytes   uint64
	packets uint64
}

var xfrmLifetime = regexp.MustCompile(`(\d+)\(bytes\),\s*(\d+)\(packets\)`)

// readbackLifetimes reads only ESP lifetime-current counters. Other protocols
// cannot satisfy a Child SA lookup, even when they reuse its SPI.
func readbackLifetimes(output string) (map[readbackSAKey]espLifetime, error) {
	result := make(map[readbackSAKey]espLifetime)
	var key readbackSAKey
	var lifetime espLifetime
	var esp, current, known bool
	flush := func() error {
		if !esp {
			return nil
		}
		if key.source == "" || key.target == "" || key.spi == 0 || !known {
			return fmt.Errorf("ESP state has incomplete identity or lifetime-current counters: %s", output)
		}
		if _, duplicate := result[key]; duplicate {
			return fmt.Errorf("duplicate directed ESP state: %+v", key)
		}
		result[key] = lifetime
		return nil
	}
	for line := range strings.SplitSeq(output, "\n") {
		if match := srcDstPattern.FindStringSubmatch(line); match != nil {
			if err := flush(); err != nil {
				return nil, err
			}
			key = readbackSAKey{source: match[1], target: match[2]}
			lifetime = espLifetime{}
			esp, current, known = false, false, false
			continue
		}
		if match := espSPIPattern.FindStringSubmatch(line); match != nil {
			spi, err := strconv.ParseUint(match[1], 0, 32)
			if err != nil {
				return nil, err
			}
			key.spi, esp = uint32(spi), true
		}
		fields := strings.Fields(line)
		for i := 0; i+1 < len(fields); i++ {
			if fields[i] == "if_id" {
				ifID, err := strconv.ParseUint(fields[i+1], 0, 32)
				if err != nil {
					return nil, err
				}
				key.ifID = uint32(ifID)
			}
		}
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "lifetime current") {
			current = true
		} else if strings.HasPrefix(trimmed, "lifetime config") || strings.HasPrefix(trimmed, "stats") {
			current = false
		}
		if current && esp {
			if match := xfrmLifetime.FindStringSubmatch(line); match != nil {
				bytes, err := strconv.ParseUint(match[1], 10, 64)
				if err != nil {
					return nil, err
				}
				packets, err := strconv.ParseUint(match[2], 10, 64)
				if err != nil {
					return nil, err
				}
				lifetime, known = espLifetime{bytes: bytes, packets: packets}, true
			}
		}
	}
	if err := flush(); err != nil {
		return nil, err
	}
	return result, nil
}

func (l *scenarioLab) readbackCounters(ctx context.Context) (zeChildCounters, map[readbackSAKey]espLifetime, error) {
	answer, err := l.zeCLI(ctx, "show vpn ipsec sa | json")
	if err != nil {
		return zeChildCounters{}, nil, err
	}
	var records []zeIKESARecord
	if err := json.Unmarshal([]byte(strings.TrimSpace(answer)), &records); err != nil {
		return zeChildCounters{}, nil, fmt.Errorf("decode Child SA counters: %w; answer: %s", err, answer)
	}
	var child *zeChildCounters
	for i := range records {
		if records[i].Peer == swanConfigPeer && records[i].Child != nil {
			if child != nil {
				return zeChildCounters{}, nil, fmt.Errorf("multiple Child SA records for swan: %s", answer)
			}
			child = records[i].Child
		}
	}
	if child == nil {
		return zeChildCounters{}, nil, fmt.Errorf("no Child SA counters for swan: %s", answer)
	}
	output, err := l.exec(ctx, zePeer, "ip", "-s", xfrmCommand, "state")
	if err != nil {
		return zeChildCounters{}, nil, err
	}
	counters, err := readbackLifetimes(output)
	if err != nil {
		return zeChildCounters{}, nil, err
	}
	if err := requireDirectedCounters(*child, counters); err != nil {
		return zeChildCounters{}, nil, err
	}
	return *child, counters, nil
}

func requireDirectedCounters(child zeChildCounters, kernel map[readbackSAKey]espLifetime) error {
	if !child.CountersKnown {
		return errors.New("the Child SA reports unknown counters over a readable kernel")
	}
	for _, direction := range []struct {
		key     readbackSAKey
		bytes   *uint64
		packets *uint64
	}{
		{readbackSAKey{zeIP, swanIP, child.OutboundSPI, child.IfID}, child.BytesOut, child.PacketsOut},
		{readbackSAKey{swanIP, zeIP, child.InboundSPI, child.IfID}, child.BytesIn, child.PacketsIn},
	} {
		observed, ok := kernel[direction.key]
		if !ok || direction.bytes == nil || direction.packets == nil {
			return fmt.Errorf("missing directed kernel or CLI counters for %+v", direction.key)
		}
		if *direction.bytes != observed.bytes || *direction.packets != observed.packets {
			return fmt.Errorf("directed counter mismatch for %+v: CLI bytes=%d packets=%d, kernel bytes=%d packets=%d",
				direction.key, *direction.bytes, *direction.packets, observed.bytes, observed.packets)
		}
	}
	return nil
}

const dataplaneCountMetric = "ze_ipsec_dataplane_sa_count"
const dataplaneDriftMetric = "ze_ipsec_dataplane_drift"

type dataplaneGauges struct {
	count map[string]float64
	drift map[string]float64
}

func decodeDataplaneGauges(answer string) (dataplaneGauges, error) {
	parser := expfmt.NewTextParser(model.UTF8Validation)
	families, err := parser.TextToMetricFamilies(strings.NewReader(answer))
	if err != nil {
		return dataplaneGauges{}, err
	}
	gauges := dataplaneGauges{count: make(map[string]float64), drift: make(map[string]float64)}
	for _, family := range []struct {
		name   string
		label  string
		values map[string]float64
	}{
		{dataplaneCountMetric, "if_id", gauges.count},
		{dataplaneDriftMetric, "peer", gauges.drift},
	} {
		for _, metric := range families[family.name].GetMetric() {
			if metric.Gauge == nil || len(metric.Label) != 1 || metric.Label[0].GetName() != family.label {
				return dataplaneGauges{}, fmt.Errorf("%s has an unexpected type or label set", family.name)
			}
			label := metric.Label[0].GetValue()
			if _, duplicate := family.values[label]; duplicate {
				return dataplaneGauges{}, fmt.Errorf("%s repeats label %q", family.name, label)
			}
			family.values[label] = metric.Gauge.GetValue()
		}
	}
	return gauges, nil
}

func requireDataplaneGauges(got, want dataplaneGauges) error {
	for _, family := range []struct {
		name string
		got  map[string]float64
		want map[string]float64
	}{
		{dataplaneCountMetric, got.count, want.count},
		{dataplaneDriftMetric, got.drift, want.drift},
	} {
		if len(family.got) != len(family.want) {
			return fmt.Errorf("%s series: got %v, want %v", family.name, family.got, family.want)
		}
		for label, want := range family.want {
			got, present := family.got[label]
			if !present || got != want {
				return fmt.Errorf("%s{%s}: got %v (present %t), want %v", family.name, label, got, present, want)
			}
		}
	}
	return nil
}

func (l *scenarioLab) waitDataplaneGauges(ctx context.Context, want dataplaneGauges) error {
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: probeWaitTimeout, Interval: time.Second, Description: "live Prometheus dataplane gauge transition",
	}, func(probe context.Context) (bool, error) {
		answer, err := l.query(probe, zePeer, "python3", "-c",
			"import urllib.request; print(urllib.request.urlopen('http://127.0.0.1:9273/metrics', timeout=3).read().decode())")
		if err != nil {
			return false, err
		}
		got, err := decodeDataplaneGauges(answer)
		if err != nil {
			return false, err
		}
		if err := requireDataplaneGauges(got, want); err != nil {
			return false, err
		}
		return true, nil
	}, func(ready bool) bool { return ready })
	return err
}

// unreadableDataplaneProbe MUST restore the soft limit before returning. The
// watchdog restores it even if docker exec's caller loses the helper process.
// Its HTTP socket stays open throughout; reconnecting would test accept(), not
// the netlink dump that publishDataplaneGauges performs on its five-second tick.
const unreadableDataplaneProbe = `
import http.client
import json
import os
import resource
import select
import signal
import time

class HeldHTTP(http.client.HTTPConnection):
    sealed = False
    def connect(self):
        if self.sealed:
            raise RuntimeError("metrics connection closed during NOFILE fault")
        super().connect()

http = HeldHTTP("127.0.0.1", 9273, timeout=3)
def get(path):
    http.request("GET", path)
    response = http.getresponse()
    body = response.read().decode()
    if response.status != 200 and not (path == "/health" and response.status == 503):
        raise RuntimeError("HTTP %d: %s" % (response.status, body))
    return body

before = get("/metrics")
http.sealed = True
if http.sock is None:
    raise RuntimeError("exporter did not preserve the warmed HTTP connection")
children = open("/proc/1/task/1/children").read().split()
pids = [int(pid) for pid in children if os.path.basename(os.readlink("/proc/" + pid + "/exe")) == "ze"]
if len(pids) != 1:
    raise RuntimeError("expected one Ze child of tini, found %r" % pids)
pid = pids[0]
original = resource.prlimit(pid, resource.RLIMIT_NOFILE)
watchdog = os.fork()
if watchdog == 0:
    http.close()
    # sleep(poll): watchdog restores the daemon limit at its absolute fault deadline.
    select.select([], [], [], 25)
    try:
        resource.prlimit(pid, resource.RLIMIT_NOFILE, original)
    except ProcessLookupError:
        pass
    os._exit(0)

def interrupted(signum, frame):
    raise RuntimeError("NOFILE probe interrupted by signal %d" % signum)
signal.signal(signal.SIGTERM, interrupted)
signal.signal(signal.SIGINT, interrupted)
try:
    resource.prlimit(pid, resource.RLIMIT_NOFILE, (0, original[1]))
    if resource.prlimit(pid, resource.RLIMIT_NOFILE) != (0, original[1]):
        raise RuntimeError("daemon soft NOFILE limit was not lowered")
    deadline = time.monotonic() + 18
    prefixes = ("ze_ipsec_dataplane_sa_count{", "ze_ipsec_dataplane_drift{")
    while True:
        unknown = get("/metrics")
        if not any(line.startswith(prefixes) for line in unknown.splitlines()):
            break
        if time.monotonic() >= deadline:
            raise RuntimeError("dataplane series survived unreadable kernel: " + unknown)
        # sleep(poll): poll the existing connection until the metrics tick removes both series.
        select.select([], [], [], 0.25)
    health = json.loads(get("/health"))
    ipsec = [row for row in health["components"] if row["name"] == "ipsec"]
    if len(ipsec) != 1:
        raise RuntimeError("unreadable dataplane omitted its health probe: %r" % health)
    reason = ipsec[0].get("reason", "").casefold()
    unreadable = any(term in reason for term in ("unknown", "unreadable", "cannot determine"))
    if ipsec[0]["status"] == "healthy" or "dataplane" not in reason or "drift" in reason or not unreadable:
        raise RuntimeError("unreadable dataplane did not report an unknown observation: %r" % health)
finally:
    resource.prlimit(pid, resource.RLIMIT_NOFILE, original)
    os.kill(watchdog, signal.SIGTERM)
    os.waitpid(watchdog, 0)
    http.close()
print(json.dumps({"before": before, "unknown": unknown}))
`

func (l *scenarioLab) requireUnreadableDataplane(ctx context.Context, wantBefore dataplaneGauges) error {
	answer, err := l.query(ctx, zePeer, "python3", "-c", unreadableDataplaneProbe)
	if err != nil {
		return fmt.Errorf("bounded daemon NOFILE fault: %w", err)
	}
	var observed struct {
		Before  string `json:"before"`
		Unknown string `json:"unknown"`
	}
	if err := json.Unmarshal([]byte(answer), &observed); err != nil {
		return err
	}
	before, err := decodeDataplaneGauges(observed.Before)
	if err != nil {
		return err
	}
	if err := requireDataplaneGauges(before, wantBefore); err != nil {
		return fmt.Errorf("before NOFILE fault: %w", err)
	}
	unknown, err := decodeDataplaneGauges(observed.Unknown)
	if err != nil {
		return err
	}
	if err := requireDataplaneGauges(unknown, dataplaneGauges{}); err != nil {
		return fmt.Errorf("during NOFILE fault: %w", err)
	}
	if err := l.waitDataplaneGauges(ctx, wantBefore); err != nil {
		return err
	}
	return requireLiveDataplaneHealth(ctx, l, true)
}

// pingCommand is the argv of one probe: count echo requests, a two-second wait, the
// source when the probe names one, and the size with Don't Fragment when it names one.
func pingCommand(probe pingProbe, count int) []string {
	command := []string{"ping", "-c", strconv.Itoa(count), "-W", "2"}
	if probe.source != "" {
		command = append(command, "-I", probe.source)
	}
	if probe.size != 0 {
		command = append(command, "-M", "do", "-s", strconv.Itoa(probe.size))
	}
	return append(command, probe.target)
}

// requireLossyPing drives mtuPingCount sized echo requests and refuses unless EVERY
// one is lost: a path that refuses the size answers none of them. Any loss short of
// all of them says the size sometimes passes, which is a different path from the one
// the caller claims, and a summary it cannot read is refused too.
func (l *scenarioLab) requireLossyPing(ctx context.Context, probe pingProbe) error {
	output := l.execQuiet(ctx, probe.peer, pingCommand(probe, mtuPingCount)...)
	loss, err := pingLoss(output)
	if err != nil {
		return fmt.Errorf("ping from %s to %s at %d octets %w", probe.peer, probe.target, probe.size, err)
	}
	if loss != 100 {
		return fmt.Errorf("ping from %s to %s at %d octets lost %d%%, want every packet refused by the clamped path: %s",
			probe.peer, probe.target, probe.size, loss, output)
	}
	return nil
}

// clampForwardedPath installs, on the NAT box, a route to each peer's real address
// carrying mtuClampedPath, so every datagram the box forwards above that size with
// Don't Fragment set is refused with an ICMP frag-needed naming the clamp.
func (l *scenarioLab) clampForwardedPath(ctx context.Context) error {
	for _, real := range []string{zeIP, swanIP} {
		if _, err := l.exec(ctx, natPeer, "ip", "route", "replace", real+"/32", "dev", "eth0", "mtu", strconv.Itoa(mtuClampedPath)); err != nil {
			return fmt.Errorf("clamp the forwarded path to %s on the NAT box: %w", real, err)
		}
	}
	return nil
}

// xfrmInnerAddress is natInnerAddress for a tunnel bound to the xfrm interface: the
// inner route leaves by that interface, which is what selects the if_id-bound policy.
func (l *scenarioLab) xfrmInnerAddress(ctx context.Context, local, remote string) error {
	if _, err := l.exec(ctx, zePeer, "ip", "address", "replace", local+"/32", "dev", "lo"); err != nil {
		return fmt.Errorf("install inner address %s on ze: %w", local, err)
	}
	if _, err := l.exec(ctx, zePeer, "ip", "route", "replace", remote+"/32", "dev", mtuXfrmInterface, "src", local); err != nil {
		return fmt.Errorf("install inner route to %s over %s on ze: %w", remote, mtuXfrmInterface, err)
	}
	return nil
}

// zeShowMTU runs `show mtu | json` in the ze container and decodes the one document
// it answers. A shape that does not decode is an error naming the answer.
func (l *scenarioLab) zeShowMTU(ctx context.Context) (map[string]any, string, error) {
	answer, err := l.zeCLI(ctx, "show mtu | json")
	if err != nil {
		return nil, answer, err
	}
	var run map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(answer)), &run); err != nil {
		return nil, answer, fmt.Errorf("show mtu | json did not answer one document: %w; answer: %s", err, answer)
	}
	return run, answer, nil
}

// zeChildRemoteAddress reads one peer's Child SA remote-address out of
// `show vpn ipsec sa | json`: the endpoint the SA is installed on. No record, no
// child-sa, and a remote-address that is not a string are each an error rather than
// "", because an empty address compares unequal to every target.
func (l *scenarioLab) zeChildRemoteAddress(ctx context.Context, peer string) (string, error) {
	records, answer, err := l.zeIKESAs(ctx)
	if err != nil {
		return "", err
	}
	for _, record := range records {
		if record["peer-name"] != peer {
			continue
		}
		child, ok := record["child-sa"].(map[string]any)
		if !ok {
			return "", fmt.Errorf("the IKE SA for %s carries no child-sa; answer: %s", peer, answer)
		}
		remote, ok := child["remote-address"].(string)
		if !ok || remote == "" {
			return "", fmt.Errorf("the Child SA for %s carries no installed remote-address; answer: %s", peer, answer)
		}
		return remote, nil
	}
	return "", fmt.Errorf("show vpn ipsec sa reports no IKE SA for %s; answer: %s", peer, answer)
}

// rowsOf reads a list of objects out of one key of the `show mtu` document.
func rowsOf(run map[string]any, key string) ([]map[string]any, error) {
	list, ok := run[key].([]any)
	if !ok {
		return nil, fmt.Errorf("show mtu carries no %s list", key)
	}
	rows := make([]map[string]any, 0, len(list))
	for _, item := range list {
		row, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("show mtu %s carries a row that is not an object: %v", key, item)
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// requireMeasurement refuses unless ONE measurement row carries the label, the target
// and the path MTU. The three are asserted on one row rather than anywhere in the
// list, because a label from one row and a figure from another describe a measurement
// that was never made.
func requireMeasurement(run map[string]any, label, target string, pathMTU int) error {
	rows, err := rowsOf(run, "measurements")
	if err != nil {
		return err
	}
	for _, row := range rows {
		if row["label"] != label || row["target"] != target {
			continue
		}
		got, ok := row["path-mtu"].(float64)
		if !ok {
			return fmt.Errorf("the %s measurement of %s carries no path-mtu: %v", label, target, row)
		}
		if int(got) != pathMTU {
			return fmt.Errorf("the %s measurement of %s answers path-mtu %d, want %d (%v)", label, target, int(got), pathMTU, row)
		}
		return nil
	}
	return fmt.Errorf("show mtu carries no %s measurement of %s", label, target)
}

// requireReference refuses unless the reference row names the host and the path
// MTU. The reference is its own row rather than a measurement, so the peers list
// carries only what the tunnels ride.
func requireReference(run map[string]any, host string, pathMTU int) error {
	reference, ok := run["reference"].(map[string]any)
	if !ok {
		return errors.New("show mtu carries no reference row")
	}
	if reference["host"] != host {
		return fmt.Errorf("the reference row names host %v, want %s: %v", reference["host"], host, reference)
	}
	got, ok := reference["path-mtu"].(float64)
	if !ok {
		return fmt.Errorf("the reference row carries no path-mtu: %v", reference)
	}
	if int(got) != pathMTU {
		return fmt.Errorf("the reference %s answers path-mtu %d, want %d: %v", host, int(got), pathMTU, reference)
	}
	return nil
}

// tunnelRowOf reads the one tunnel row named peer.
func tunnelRowOf(run map[string]any, peer string) (map[string]any, error) {
	rows, err := rowsOf(run, "tunnels")
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		if row["peer"] == peer {
			return row, nil
		}
	}
	return nil, fmt.Errorf("show mtu carries no tunnel row for %s", peer)
}

// requireOversizedTunnel pins the sizing of the bound tunnel: measured (not assumed)
// at the clamp, UDP-encapsulated aes256gcm, the xfrm interface at 1500 against the
// ceiling and the recommended value the transform derives.
func requireOversizedTunnel(row map[string]any) error {
	if row["verdict"] != "oversized" {
		return fmt.Errorf("tunnel verdict %v, want oversized: %v", row["verdict"], row)
	}
	if row["assumed"] != false {
		return fmt.Errorf("the tunnel's path is assumed, want measured: %v", row)
	}
	if row["encapsulation"] != true {
		return fmt.Errorf("tunnel encapsulation %v, want true across the NAT: %v", row["encapsulation"], row)
	}
	if row["interface"] != mtuXfrmInterface {
		return fmt.Errorf("tunnel interface %v, want %s: %v", row["interface"], mtuXfrmInterface, row)
	}
	for key, want := range map[string]int{
		"path-mtu":    mtuClampedPath,
		"current-mtu": mtuXfrmInterfaceMTU,
		"ceiling":     mtuGCMUDPCeiling,
		"recommended": mtuGCMUDPRecommended,
		"octets":      mtuXfrmInterfaceMTU - mtuGCMUDPCeiling,
	} {
		got, ok := row[key].(float64)
		if !ok {
			return fmt.Errorf("tunnel row carries no %s: %v", key, row)
		}
		if int(got) != want {
			return fmt.Errorf("tunnel %s %d, want %d: %v", key, int(got), want, row)
		}
	}
	return nil
}

// requireCommand refuses unless the remediation list carries the command.
func requireCommand(run map[string]any, want string) error {
	commands, ok := run["commands"].([]any)
	if !ok {
		return errors.New("show mtu carries no commands list")
	}
	for _, command := range commands {
		if command == want {
			return nil
		}
	}
	return fmt.Errorf("show mtu commands %v lack %q", commands, want)
}

// requireUnderlayAdvice refuses unless the underlay row carries the advice.
func requireUnderlayAdvice(run map[string]any, want string) error {
	underlay, ok := run["underlay"].(map[string]any)
	if !ok {
		return errors.New("show mtu carries no underlay row")
	}
	if underlay["advice"] != want {
		return fmt.Errorf("underlay advice %v, want %s: %v", underlay["advice"], want, underlay)
	}
	return nil
}

// The padded IKE path probe scenario (ike-padded-probe-strongswan) reshapes the NAT
// box so that ICMP cannot see the clamp and only the SA's own channel can.
const (
	// icmpRouteTable is the NAT box's policy-routing table that carries ICMP past
	// the clamp: its routes name no mtu, so a forwarded echo crosses at eth0's 1500.
	icmpRouteTable = "100"
	// mtuUnclampedPath is what the ICMP search then measures to the peer: the box's
	// full link, and the ceiling the IKE probe starts from and refutes.
	mtuUnclampedPath = 1500
	// zeShowMTUOutput is where a detached `show mtu | json` writes its document in
	// the ze container, and zeShowMTUDone the marker the shell writes after it. The
	// run is detached because a probe the peer never answers holds the request
	// window for requestWindowTimeout (30 s) before the SA is deemed failed, which
	// is longer than one docker exec is given.
	zeShowMTUOutput = "/root/show-mtu.json"
	zeShowMTUDone   = "/root/show-mtu.done"
	// showMTUDetachedTimeout bounds the wait for a detached run: two ICMP searches
	// that can each run 45 s on silence, the IKE descent and one full
	// request-window timeout.
	showMTUDetachedTimeout = 300 * time.Second
	// detachedLogTail is how many log lines of each daemon a detached run that
	// never finished names in its error, so the failure says what the daemons
	// were doing rather than only that the marker never appeared.
	detachedLogTail = 60
	// rekeyRaceDelay is how long after the detached run starts that strongSwan is
	// told to rekey the IKE SA. The ICMP search over an unclamped path answers in
	// well under a second, and the IKE descent that follows spends several seconds
	// on retransmit timers, so the rekey lands inside it.
	rekeyRaceDelay = 3 * time.Second
	// probeSAFailedMark is what the measurement row's ike-declined starts with when
	// the SA was deemed failed under an exchange (ikeProbeEnded, mtu/cmd/search.go).
	probeSAFailedMark = "sa-failed at "
	// probeRefusedMark is the ike-declined prefix of a refusal by name.
	probeRefusedMark = "refused: "
)

// peerReassemblyMarks are the per-namespace sysctls bounding the memory the kernel
// spends reassembling fragmented datagrams, high mark first.
var peerReassemblyMarks = []string{"net.ipv4.ipfrag_high_thresh", "net.ipv4.ipfrag_low_thresh"}

// exemptICMPFromClamp installs, on the NAT box, a policy route that carries every
// ICMP datagram it forwards, and every one it sends itself, over routes that name no
// mtu. An echo and its reply then cross at 1500 while every UDP and ESP datagram
// meets clampForwardedPath's 1400: the ICMP search believes 1500 and only a probe
// on the tunnel's own channel meets the clamp.
func (l *scenarioLab) exemptICMPFromClamp(ctx context.Context) error {
	for _, real := range []string{zeIP, swanIP} {
		if _, err := l.exec(ctx, natPeer, "ip", "route", "replace", real+"/32", "dev", "eth0", "table", icmpRouteTable); err != nil {
			return fmt.Errorf("install the unclamped ICMP route to %s on the NAT box: %w", real, err)
		}
	}
	if _, err := l.exec(ctx, natPeer, "ip", "rule", "add", "ipproto", "icmp", "lookup", icmpRouteTable); err != nil {
		return fmt.Errorf("route ICMP past the clamp on the NAT box: %w", err)
	}
	return nil
}

// dropTooBigToward drops, on the NAT box, every Fragmentation Needed the box itself
// would send the target, so a Don't Fragment datagram the clamp refuses vanishes in
// silence: the filtered path of AC-1.
func (l *scenarioLab) dropTooBigToward(ctx context.Context, target string) error {
	_, err := l.exec(ctx, natPeer, "iptables", "-I", "OUTPUT", "1", "-d", target, "-p", "icmp", "--icmp-type", "fragmentation-needed", "-j", "DROP")
	if err != nil {
		return fmt.Errorf("drop Fragmentation Needed toward %s on the NAT box: %w", target, err)
	}
	return nil
}

// dropFragmentsAtPeer makes every fragmented datagram vanish at the strongSwan peer:
// its reassembly memory (net.ipv4.ipfrag_high_thresh, and the low mark it MUST stay
// above) is cut to zero, so the kernel refuses every fragment queue and no datagram
// the NAT box fragmented is ever reassembled or parsed (ReasmFails counts them). A
// drop at a netfilter chain cannot do this: conntrack's defragmentation runs before
// any chain on both the box and the peer, so `iptables -f` matches nothing there and
// the DF-clear copy is answered as if the path carried fragments (measured
// 2026-09-16: 16 exchanges and `prober: ike` under that rule). Seen from Ze the
// path drops fragments either way. Nothing else in the scenario is fragmented:
// the echo crosses whole and ESP stays under the clamp. The function returned
// puts both marks back, high before low, the order the kernel's bounds accept.
func (l *scenarioLab) dropFragmentsAtPeer(ctx context.Context) (restore func(context.Context) error, err error) {
	previous := make([]string, len(peerReassemblyMarks))
	for i, mark := range peerReassemblyMarks {
		value, err := l.exec(ctx, swanPeer, "sysctl", "-n", mark)
		if err != nil {
			return nil, fmt.Errorf("read strongSwan's %s: %w", mark, err)
		}
		previous[i] = strings.TrimSpace(value)
	}
	// Low first: high may not go below low.
	for _, mark := range slices.Backward(peerReassemblyMarks) {
		if _, err := l.exec(ctx, swanPeer, "sysctl", "-w", mark+"=0"); err != nil {
			return nil, fmt.Errorf("cut strongSwan's %s: %w", mark, err)
		}
	}
	return func(ctx context.Context) error {
		for i, mark := range peerReassemblyMarks {
			if _, err := l.exec(ctx, swanPeer, "sysctl", "-w", mark+"="+previous[i]); err != nil {
				return fmt.Errorf("restore strongSwan's %s to %s: %w", mark, previous[i], err)
			}
		}
		return nil
	}, nil
}

// zeShowMTUDetached starts `show mtu | json` in the ze container without waiting
// for it, runs `during` once it is started, and answers the document once the run
// wrote it. A run whose IKE probe is never answered outlasts one docker exec, so
// the shell writes the answer to a file and a marker after it, and this polls for
// the marker.
func (l *scenarioLab) zeShowMTUDetached(ctx context.Context, during func(context.Context) error) (map[string]any, string, error) {
	if _, err := l.exec(ctx, zePeer, "rm", "-f", zeShowMTUOutput, zeShowMTUDone); err != nil {
		return nil, "", fmt.Errorf("clear the previous detached show mtu answer: %w", err)
	}
	argv := zeCLICommand("show mtu | json")
	var tb textbuf.Buffer
	shell := tb.Str("{ ").Str(argv[len(argv)-1]).Str(" ; } > ").Str(zeShowMTUOutput).Str(" 2>&1; echo done > ").Str(zeShowMTUDone).String()
	if err := l.check.Lab.ExecDetached(ctx, zePeer, []string{"sh", "-c", shell}, nil); err != nil {
		return nil, "", fmt.Errorf("start the detached show mtu: %w", err)
	}
	if during != nil {
		if err := during(ctx); err != nil {
			return nil, "", err
		}
	}
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: showMTUDetachedTimeout, Interval: 2 * time.Second, Description: "the detached show mtu to finish",
	}, func(probe context.Context) (string, error) {
		return l.execQuiet(probe, zePeer, "cat", zeShowMTUDone), nil
	}, func(marker string) bool { return strings.TrimSpace(marker) == "done" })
	if err != nil {
		return nil, "", fmt.Errorf("%w; ze log tail: %s; strongSwan log tail: %s", err, l.logTail(ctx, zePeer), l.logTail(ctx, swanPeer))
	}
	answer, err := l.exec(ctx, zePeer, "cat", zeShowMTUOutput)
	if err != nil {
		return nil, answer, fmt.Errorf("read the detached show mtu answer: %w", err)
	}
	var run map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(answer)), &run); err != nil {
		return nil, answer, fmt.Errorf("the detached show mtu | json did not answer one document: %w; answer: %s", err, answer)
	}
	return run, answer, nil
}

// swanRekeyIKE asks strongSwan to rekey the IKE SA now. In IKE_REKEYED charon drops
// a request that is not a Delete (task_manager_v2.c reject_request), which is the
// race R-5 names.
func (l *scenarioLab) swanRekeyIKE(ctx context.Context) error {
	if _, err := l.exec(ctx, swanPeer, "swanctl", "--rekey", "--ike", swanConnection); err != nil {
		return fmt.Errorf("swanctl --rekey --ike %s: %w", swanConnection, err)
	}
	return nil
}

// measurementRowOf answers the ONE measurement row carrying the label and the
// target, or an error naming what the list holds instead.
func measurementRowOf(run map[string]any, label, target string) (map[string]any, error) {
	rows, err := rowsOf(run, "measurements")
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		if row["label"] == label && row["target"] == target {
			return row, nil
		}
	}
	return nil, fmt.Errorf("show mtu carries no %s measurement of %s: %v", label, target, rows)
}

// rowNumber reads one numeric key of a row, refusing a row that lacks it.
func rowNumber(row map[string]any, key string) (int, error) {
	value, ok := row[key].(float64)
	if !ok {
		return 0, fmt.Errorf("the row carries no numeric %s: %v", key, row)
	}
	return int(value), nil
}

// requireIKEMeasured refuses unless the row was measured by the padded IKE
// exchange: prober ike, no ike-declined, path-mtu and ike-confirmed at the size
// given, at least two exchanges (the refuted ask and the size that fit, so a
// DF-clear copy was answered across the box, A-3), and no different-path caveat
// (the SA rides UDP/4500 where ESP rides, AC-7).
func requireIKEMeasured(row map[string]any, pathMTU int) error {
	if row["prober"] != "ike" {
		return fmt.Errorf("the measurement's prober is %v, want ike: %v", row["prober"], row)
	}
	if declined, present := row["ike-declined"]; present {
		return fmt.Errorf("the IKE prober declined: %v", declined)
	}
	for _, key := range []string{"path-mtu", "ike-confirmed"} {
		got, err := rowNumber(row, key)
		if err != nil {
			return err
		}
		if got != pathMTU {
			return fmt.Errorf("the measurement's %s is %d, want %d: %v", key, got, pathMTU, row)
		}
	}
	exchanges, err := rowNumber(row, "exchanges")
	if err != nil {
		return err
	}
	if exchanges < 2 {
		return fmt.Errorf("the measurement spent %d IKE exchanges, want at least two: the ask above the clamp and the size that fit", exchanges)
	}
	if strings.Contains(fmt.Sprint(row["caveats"]), "UDP/500") {
		return fmt.Errorf("the measurement carries the different-path caveat on a NAT-T SA: %v", row["caveats"])
	}
	return nil
}

// requireProbeFailedSA refuses unless the row says the IKE prober was declined
// because the SA was deemed failed under the ONE exchange at the size given: prober
// icmp, ike-declined naming sa-failed and the size, exactly one exchange (no second
// size was tried), and the ICMP figure kept as path-mtu (AC-13).
func requireProbeFailedSA(row map[string]any, size, icmpFigure int) error {
	if row["prober"] != "icmp" {
		return fmt.Errorf("the measurement's prober is %v, want icmp after the SA failed: %v", row["prober"], row)
	}
	declined, ok := row["ike-declined"].(string)
	if !ok {
		return fmt.Errorf("the measurement carries no ike-declined: %v", row)
	}
	want := probeSAFailedMark + strconv.Itoa(size) + " octets"
	if !strings.HasPrefix(declined, want) {
		return fmt.Errorf("ike-declined is %q, want it to start with %q", declined, want)
	}
	exchanges, err := rowNumber(row, "exchanges")
	if err != nil {
		return err
	}
	if exchanges != 1 {
		return fmt.Errorf("the measurement spent %d IKE exchanges, want exactly the one the SA failed under", exchanges)
	}
	pathMTU, err := rowNumber(row, "path-mtu")
	if err != nil {
		return err
	}
	if pathMTU != icmpFigure {
		return fmt.Errorf("the measurement's path-mtu is %d, want the ICMP figure %d kept", pathMTU, icmpFigure)
	}
	return nil
}

// requireCharonParsedProbes refuses unless charon's log shows INFORMATIONAL requests
// parsed and no parse failure or INVALID_SYNTAX: the positive clause proves the log
// was read and the padded requests reached the parser (A-2, R-1).
func requireCharonParsedProbes(logs string) error {
	if !strings.Contains(logs, "parsed INFORMATIONAL request") {
		return errors.New("charon's log shows no INFORMATIONAL request parsed")
	}
	if strings.Contains(logs, "INVALID_SYNTAX") {
		return errors.New("charon answered INVALID_SYNTAX")
	}
	for line := range strings.SplitSeq(logs, "\n") {
		if strings.Contains(line, "pars") && strings.Contains(line, "failed") {
			return fmt.Errorf("charon reported a parse failure: %s", line)
		}
	}
	return nil
}

// requireZeIKEEstablished refuses unless `show vpn ipsec sa` lists an established
// IKE SA for the configured peer.
func (l *scenarioLab) requireZeIKEEstablished(ctx context.Context, peer string) error {
	records, answer, err := l.zeIKESAs(ctx)
	if err != nil {
		return err
	}
	for _, record := range records {
		if record["peer-name"] == peer && record["state"] == "established" {
			return nil
		}
	}
	return fmt.Errorf("ze lists no established IKE SA for %s; answer: %s", peer, answer)
}

// logTail answers the last detachedLogTail lines of one peer's log, or the read
// error's text, for an error message about a run that never finished.
func (l *scenarioLab) logTail(ctx context.Context, peer string) string {
	logs, err := l.logs(ctx, peer)
	if err != nil {
		return "(unreadable: " + err.Error() + ")"
	}
	lines := strings.Split(strings.TrimSpace(logs), "\n")
	if len(lines) > detachedLogTail {
		lines = lines[len(lines)-detachedLogTail:]
	}
	return "\n" + strings.Join(lines, "\n")
}
