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

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/interoplab"
)

const (
	zePeer   = "ze"
	swanPeer = "strongswan"
	frrPeer  = "frr"
	natPeer  = "nat"

	zeIP   = "172.28.0.2"
	swanIP = "172.28.0.3"
	frrIP  = "172.28.0.4"

	// natHost is the NAT box's host octet on the lab network. It sits after FRR so a
	// scenario that wants both keeps its addresses.
	natHost = 5

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
	return l.waitOutput(ctx, peer, []string{"ip", "xfrm", "state"}, xfrmWaitTimeout, "XFRM ESP state", func(output string) bool {
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
	return l.exec(ctx, peer, "ip", "xfrm", "state")
}

// inboundXFRMState answers the ONE state that decapsulates what source sends to target.
//
// The filter matters. `ip xfrm state` prints both directions of a Child SA, so a template
// present on the OUTBOUND state alone satisfies an assertion made over the whole dump
// while the receive path carries nothing. Reception is what RFC 3948 Section 3.1.2
// governs, so the assertions that cite it read this state and no other.
func (l *scenarioLab) inboundXFRMState(ctx context.Context, peer, source, target string) (string, error) {
	output, err := l.exec(ctx, peer, "ip", "xfrm", "state", "list", "src", source, "dst", target)
	if err != nil {
		return "", err
	}
	if !strings.Contains(output, "proto esp") {
		return "", fmt.Errorf("%s carries no inbound ESP state for src %s dst %s: %s", peer, source, target, output)
	}
	return output, nil
}

func (l *scenarioLab) xfrmPolicy(ctx context.Context, peer string) (string, error) {
	output, err := l.exec(ctx, peer, "ip", "xfrm", "policy")
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
	output, err := l.exec(ctx, peer, "ip", "-s", "xfrm", "state")
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
	command := []string{"ping", "-c", strconv.Itoa(count), "-W", "2"}
	if probe.source != "" {
		command = append(command, "-I", probe.source)
	}
	command = append(command, probe.target)
	output := l.execQuiet(ctx, probe.peer, command...)
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
	var tb textbuf.Buffer
	seed := tb.Str("printf '%s\\n%s\\n127.0.0.1\\n%s\\n' ").Str(zeCLIUser).Byte(' ').
		Str(zeCLIPassword).Byte(' ').Str(zeCLIPort).Str(" | ZE_CONFIG_DIR=").
		Str(zeCLIStore).Str(" ze init").String()
	run := tb.Reset().Str("ZE_CONFIG_DIR=").Str(zeCLIStore).Str(" ZE_SSH_PASSWORD=").
		Str(zeCLIPassword).Str(" ze cli -c ").Str(shellQuote(command)).String()
	shell := tb.Reset().Str("[ -d ").Str(zeCLIStore).Str(" ] || ").Str(seed).
		Str(" >/dev/null; ").Str(run).String()
	return l.query(ctx, zePeer, "sh", "-c", shell)
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

// requireSPISetChanged refuses unless the rekey REPLACED at least one SPI.
//
// RFC 7296 Section 2.8 replaces the Child SA's SPI while the selector stays
// identical, so an unchanged set is the evidence the rekey never reached the
// kernel. An empty set after the rekey is refused for the reason above: it would
// satisfy "changed" while proving the readers stopped answering.
func requireSPISetChanged(before, after map[uint32]struct{}) error {
	if len(after) == 0 {
		return fmt.Errorf("the kernel SAD reports no ESP SPI after the rekey, so the change is unproven; before: %v", sortedSPIs(before))
	}
	if sameSPISet(before, after) {
		return fmt.Errorf("the ESP SPI set did not change across the rekey: %v", sortedSPIs(after))
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
