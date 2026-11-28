// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- Child SA volume lifetimes on Linux XFRM.
// Related: register_ipsec_life_volume.go -- the registration.
// Related: tunnel_fixture_ipsec.go -- tunnelIPsecWait, tunnelIPsecPolicies, tunnelIPsecCommand.

package fixture

import (
	"context"
	"errors"
	"fmt"
	"net"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	lifeVolLink = "zevoltest0"

	// Byte-limit case: peer-a (127.0.0.1) -> peer-b (127.0.0.2), life-bytes 131072,
	// lifetime 3600. softBytes is randomized in [117965, 124519] (5% to 10% below 131072).
	// Stopping sends at 125000 bytes sits strictly above the maximum soft threshold
	// (124519) and strictly below the hard ceiling (131072) even if loopback delivers
	// one extra 512-byte packet before the check.
	lifeVolBytesOuterLocal  = "127.0.0.1"
	lifeVolBytesOuterRemote = "127.0.0.2"
	lifeVolBytesInnerLocal  = "198.18.240.1"
	lifeVolBytesInnerRemote = "198.18.240.65"
	lifeVolBytesLocalCIDR   = "198.18.240.0/26"
	lifeVolBytesRemoteCIDR  = "198.18.240.64/26"
	lifeVolBytesStepAt      = 110000
	lifeVolBytesStop        = 125000
	lifeVolBytesPayloadSize = 512
	lifeVolBytesFastBatch   = 16

	// Packet-limit case: peer-a (127.0.0.3) -> peer-b (127.0.0.4), life-packets 200,
	// lifetime 3600. softPackets is randomized in [180, 190] (5% to 10% below 200).
	// Stopping sends at 191 packets sits strictly above the maximum soft threshold (190)
	// and strictly below the hard ceiling (200).
	lifeVolPktsOuterLocal  = "127.0.0.3"
	lifeVolPktsOuterRemote = "127.0.0.4"
	lifeVolPktsInnerLocal  = "198.18.240.129"
	lifeVolPktsInnerRemote = "198.18.240.193"
	lifeVolPktsLocalCIDR   = "198.18.240.128/26"
	lifeVolPktsRemoteCIDR  = "198.18.240.192/26"
	lifeVolPktsStepAt      = 160
	lifeVolPktsStop        = 191
	lifeVolPktsPayloadSize = 64
	lifeVolPktsFastBatch   = 10

	lifeVolUDPPort        = 19999
	lifeVolMaxSendLoops   = 400
	lifeVolResetMaxBytes  = 2048
	lifeVolResetMaxPkts   = 4
	lifeVolPollAttempts   = 400
	lifeVolPollInterval   = 250 * time.Millisecond
	lifeVolRetireAttempts = 400
)

var (
	lifeVolSrcDstPattern   = regexp.MustCompile(`^src\s+(\S+)\s+dst\s+(\S+)`)
	lifeVolESPSPIPattern   = regexp.MustCompile(`proto\s+esp\s+spi\s+(0x[0-9a-fA-F]+)`)
	lifeVolCountersPattern = regexp.MustCompile(`(\d+)\(bytes\),\s*(\d+)\(packets\)`)
)

// xfrmStateCounter holds one kernel ESP state's identity and live lifetime-current counters
// read from `ip -s xfrm state`.
type xfrmStateCounter struct {
	src     string
	dst     string
	spi     string
	bytes   uint64
	packets uint64
}

// volumeRekeyCase names one volume-triggered Child SA rekey run inside tunnelIPsecLifeVolume.
type volumeRekeyCase struct {
	label       string
	outerLocal  string
	outerRemote string
	innerLocal  string
	innerRemote string
	localCIDR   string
	remoteCIDR  string
	byPackets   bool
	stepAt      uint64
	stopAt      uint64
	payloadSize int
	fastBatch   int
}

// parseXFRMStateCounters parses `ip -s xfrm state` output and returns every ESP state whose
// endpoints belong to the undirected pair (addrA, addrB), sorted by SPI.
func parseXFRMStateCounters(output, addrA, addrB string) ([]xfrmStateCounter, error) {
	var states []xfrmStateCounter
	var cur xfrmStateCounter
	inPair := false
	isESP := false
	inCurrent := false
	hasCounters := false

	flush := func() error {
		if !inPair {
			return nil
		}
		if !isESP {
			return nil
		}
		if cur.spi == "" {
			return fmt.Errorf("incomplete ESP state for %s -> %s: missing spi", cur.src, cur.dst)
		}
		if !hasCounters {
			return fmt.Errorf("incomplete ESP state %s (%s -> %s): missing lifetime current counters", cur.spi, cur.src, cur.dst)
		}
		states = append(states, cur)
		return nil
	}

	for line := range strings.SplitSeq(output, "\n") {
		if m := lifeVolSrcDstPattern.FindStringSubmatch(line); m != nil {
			if err := flush(); err != nil {
				return nil, err
			}
			src, dst := m[1], m[2]
			cur = xfrmStateCounter{src: src, dst: dst}
			inPair = (src == addrA && dst == addrB) || (src == addrB && dst == addrA)
			isESP = false
			inCurrent = false
			hasCounters = false
			continue
		}
		if !inPair {
			continue
		}
		if m := lifeVolESPSPIPattern.FindStringSubmatch(line); m != nil {
			cur.spi = m[1]
			isESP = true
			continue
		}
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "lifetime current"):
			inCurrent = true
			continue
		case strings.HasPrefix(trimmed, "lifetime config"), strings.HasPrefix(trimmed, "stats"):
			inCurrent = false
			continue
		}
		if !inCurrent {
			continue
		}
		if !isESP {
			continue
		}
		if m := lifeVolCountersPattern.FindStringSubmatch(line); m != nil {
			b, err := strconv.ParseUint(m[1], 10, 64)
			if err != nil {
				return nil, fmt.Errorf("parse bytes %q: %w", m[1], err)
			}
			p, err := strconv.ParseUint(m[2], 10, 64)
			if err != nil {
				return nil, fmt.Errorf("parse packets %q: %w", m[2], err)
			}
			cur.bytes = b
			cur.packets = p
			hasCounters = true
		}
	}
	if err := flush(); err != nil {
		return nil, err
	}
	slices.SortFunc(states, func(a, b xfrmStateCounter) int {
		if a.spi < b.spi {
			return -1
		}
		if a.spi > b.spi {
			return 1
		}
		return strings.Compare(a.src, b.src)
	})
	return states, nil
}

// xfrmStateSPIs returns the sorted unique SPI list from states.
func xfrmStateSPIs(states []xfrmStateCounter) []string {
	spis := make([]string, 0, len(states))
	for i := range states {
		spis = append(spis, states[i].spi)
	}
	slices.Sort(spis)
	return slices.Compact(spis)
}

// outboundCounter finds the outbound ESP state (src -> dst) among states and returns its
// current byte and packet counts.
func outboundCounter(states []xfrmStateCounter, src, dst string) (bytesCount, packetsCount uint64, found bool) {
	for i := range states {
		if states[i].src != src {
			continue
		}
		if states[i].dst != dst {
			continue
		}
		return states[i].bytes, states[i].packets, true
	}
	return 0, 0, false
}

// replacementStates returns every state in current whose SPI was not in initialSet.
func replacementStates(current []xfrmStateCounter, initialSet map[string]bool) []xfrmStateCounter {
	var fresh []xfrmStateCounter
	for i := range current {
		if initialSet[current[i].spi] {
			continue
		}
		fresh = append(fresh, current[i])
	}
	return fresh
}

// verifyReplacementCountersReset checks that fresh replacement SA states carry reset
// counters well below the retired SA's peak and within the 1-packet tick tolerance.
func verifyReplacementCountersReset(label string, fresh []xfrmStateCounter, peakBytes, peakPackets uint64) (maxBytes, maxPackets uint64, err error) {
	if len(fresh) < 2 {
		return 0, 0, fmt.Errorf("%s: expected 2 replacement ESP states, got %d", label, len(fresh))
	}
	for i := range fresh {
		maxBytes = max(maxBytes, fresh[i].bytes)
		maxPackets = max(maxPackets, fresh[i].packets)
	}
	if maxBytes > lifeVolResetMaxBytes {
		return maxBytes, maxPackets, fmt.Errorf("%s: replacement SA bytes=%d did not reset (peak before rekey=%d)",
			label, maxBytes, peakBytes)
	}
	if maxPackets > lifeVolResetMaxPkts {
		return maxBytes, maxPackets, fmt.Errorf("%s: replacement SA packets=%d did not reset (peak before rekey=%d)",
			label, maxPackets, peakPackets)
	}
	if peakBytes > 0 {
		if maxBytes >= peakBytes {
			return maxBytes, maxPackets, fmt.Errorf("%s: replacement SA bytes=%d not below retired peak=%d",
				label, maxBytes, peakBytes)
		}
	}
	if peakPackets > 0 {
		if maxPackets >= peakPackets {
			return maxBytes, maxPackets, fmt.Errorf("%s: replacement SA packets=%d not below retired peak=%d",
				label, maxPackets, peakPackets)
		}
	}
	return maxBytes, maxPackets, nil
}

// tunnelIPsecLifeVolume proves that both life-bytes and life-packets trigger real
// CREATE_CHILD_SA Child SA rekeys on Linux XFRM, rotating SPIs and resetting counters on
// the replacement SA long before the 3600-second time lifetime.
// An optional argument ("bytes" or "packets") selects a single case so each daemon pair
// binds its own local address cleanly without sharing an IKE UDP socket.
func tunnelIPsecLifeVolume(ctx context.Context, args []string) error {
	selected := ""
	if len(args) > 0 {
		selected = strings.ToUpper(args[0])
	}
	if _, err := tunnelIPsecCommand(ctx, "ip", "link", "add", lifeVolLink, "type", "dummy"); err != nil {
		return err
	}
	defer func() {
		if _, delErr := tunnelIPsecCommand(context.Background(), "ip", "link", "del", lifeVolLink); delErr != nil {
			fmt.Println("CLEANUP-FAILED:", delErr)
		}
	}()
	if _, err := tunnelIPsecCommand(ctx, "ip", "link", "set", lifeVolLink, "up"); err != nil {
		return err
	}

	allCases := []volumeRekeyCase{
		{
			label:       "BYTES",
			outerLocal:  lifeVolBytesOuterLocal,
			outerRemote: lifeVolBytesOuterRemote,
			innerLocal:  lifeVolBytesInnerLocal,
			innerRemote: lifeVolBytesInnerRemote,
			localCIDR:   lifeVolBytesLocalCIDR,
			remoteCIDR:  lifeVolBytesRemoteCIDR,
			byPackets:   false,
			stepAt:      lifeVolBytesStepAt,
			stopAt:      lifeVolBytesStop,
			payloadSize: lifeVolBytesPayloadSize,
			fastBatch:   lifeVolBytesFastBatch,
		},
		{
			label:       "PACKETS",
			outerLocal:  lifeVolPktsOuterLocal,
			outerRemote: lifeVolPktsOuterRemote,
			innerLocal:  lifeVolPktsInnerLocal,
			innerRemote: lifeVolPktsInnerRemote,
			localCIDR:   lifeVolPktsLocalCIDR,
			remoteCIDR:  lifeVolPktsRemoteCIDR,
			byPackets:   true,
			stepAt:      lifeVolPktsStepAt,
			stopAt:      lifeVolPktsStop,
			payloadSize: lifeVolPktsPayloadSize,
			fastBatch:   lifeVolPktsFastBatch,
		},
	}

	var cases []volumeRekeyCase
	for i := range allCases {
		if selected != "" {
			if allCases[i].label != selected {
				continue
			}
		}
		cases = append(cases, allCases[i])
	}
	if len(cases) == 0 {
		return fmt.Errorf("unknown ipsec/life-volume case %q (expected bytes or packets)", selected)
	}

	for i := range cases {
		c := cases[i]
		if _, err := tunnelIPsecCommand(ctx, "ip", "addr", "add", c.innerLocal+"/26", "dev", lifeVolLink); err != nil {
			return err
		}
		if _, err := tunnelIPsecCommand(ctx, "ip", "route", "replace", c.remoteCIDR, "dev", lifeVolLink); err != nil {
			return err
		}
	}

	for i := range cases {
		if err := runVolumeRekeyCase(ctx, cases[i]); err != nil {
			return err
		}
	}
	return nil
}

func runVolumeRekeyCase(ctx context.Context, c volumeRekeyCase) error {
	endpoints := map[string]bool{c.outerLocal: true, c.outerRemote: true}
	readSPIs := func() ([]string, error) {
		return tunnelIPsecSPIs(ctx, endpoints)
	}
	readPolicies := func() ([]string, error) {
		return tunnelIPsecPolicies(ctx, c.localCIDR, c.remoteCIDR)
	}
	readStates := func() ([]xfrmStateCounter, error) {
		out, err := tunnelIPsecCommand(ctx, "ip", "-s", "xfrm", "state")
		if err != nil {
			return nil, err
		}
		return parseXFRMStateCounters(out, c.outerLocal, c.outerRemote)
	}

	firstSPIs, err := tunnelIPsecWait(ctx, lifeVolPollAttempts, lifeVolPollInterval, readSPIs, func(values []string) bool {
		return len(values) >= 2
	})
	if err != nil {
		return fmt.Errorf("%s: tunnel never reached kernel: %w", c.label, err)
	}
	firstPolicies, err := tunnelIPsecWait(ctx, lifeVolPollAttempts, lifeVolPollInterval, readPolicies, func(values []string) bool {
		return len(values) >= 2
	})
	if err != nil {
		return fmt.Errorf("%s: tunnel policies never reached kernel: %w", c.label, err)
	}
	fmt.Println(c.label + "-INSTALLED: " + strings.Join(firstSPIs, " "))

	firstSet := make(map[string]bool, len(firstSPIs))
	for _, spi := range firstSPIs {
		firstSet[spi] = true
	}

	peakBytes, peakPackets, err := driveVolumeTraffic(ctx, c, firstSet, readStates)
	if err != nil {
		return err
	}
	var tb textbuf.Buffer
	tb.Str(c.label).Str("-TRAFFIC: peak-bytes=").Uint(peakBytes).Str(" peak-packets=").Uint(peakPackets)
	fmt.Println(tb.String())

	var replacementSPIs []string
	_, err = tunnelIPsecWait(ctx, lifeVolPollAttempts, lifeVolPollInterval, readSPIs, func(values []string) bool {
		replacementSPIs = replacementSPIs[:0]
		for _, spi := range values {
			if !firstSet[spi] {
				replacementSPIs = append(replacementSPIs, spi)
			}
		}
		return len(replacementSPIs) >= 2
	})
	if err != nil {
		return fmt.Errorf("%s: no replacement states arrived after traffic (peak bytes=%d packets=%d): %w",
			c.label, peakBytes, peakPackets, err)
	}
	fmt.Println(c.label + "-REKEYED: " + strings.Join(replacementSPIs, " "))

	_, err = tunnelIPsecWait(ctx, lifeVolRetireAttempts, lifeVolPollInterval, readSPIs, func(values []string) bool {
		for _, spi := range values {
			if firstSet[spi] {
				return false
			}
		}
		return len(values) >= 2
	})
	if err != nil {
		return fmt.Errorf("%s: retired states stayed in kernel: %v: %w", c.label, firstSPIs, err)
	}
	fmt.Println(c.label + "-RETIRED: " + strings.Join(firstSPIs, " "))

	afterStates, err := readStates()
	if err != nil {
		return fmt.Errorf("%s: read post-rekey counters: %w", c.label, err)
	}
	fresh := replacementStates(afterStates, firstSet)
	newBytes, newPackets, err := verifyReplacementCountersReset(c.label, fresh, peakBytes, peakPackets)
	if err != nil {
		return err
	}
	tb.Reset()
	tb.Str(c.label).Str("-COUNTER-RESET: bytes=").Uint(newBytes).Str(" packets=").Uint(newPackets)
	fmt.Println(tb.String())

	afterPolicies, err := readPolicies()
	if err != nil {
		return fmt.Errorf("%s: read post-rekey policies: %w", c.label, err)
	}
	if !tunnelSameStrings(afterPolicies, firstPolicies) {
		return fmt.Errorf("%s: POLICY-MOVED: before=%v after=%v", c.label, firstPolicies, afterPolicies)
	}
	fmt.Println(c.label + "-POLICY-STABLE: " + strings.Join(afterPolicies, " | "))
	return nil
}

func driveVolumeTraffic(
	ctx context.Context,
	c volumeRekeyCase,
	initialSet map[string]bool,
	readStates func() ([]xfrmStateCounter, error),
) (peakBytes, peakPackets uint64, err error) {
	localIP := net.ParseIP(c.innerLocal)
	remoteIP := net.ParseIP(c.innerRemote)
	if localIP == nil {
		return 0, 0, fmt.Errorf("%s: invalid inner local IP %q", c.label, c.innerLocal)
	}
	if remoteIP == nil {
		return 0, 0, fmt.Errorf("%s: invalid inner remote IP %q", c.label, c.innerRemote)
	}
	conn, err := net.DialUDP("udp4",
		&net.UDPAddr{IP: localIP, Port: 0},
		&net.UDPAddr{IP: remoteIP, Port: lifeVolUDPPort},
	)
	if err != nil {
		return 0, 0, fmt.Errorf("%s: dial UDP %s -> %s: %w", c.label, c.innerLocal, c.innerRemote, err)
	}
	defer conn.Close() //nolint:errcheck // fixture teardown

	payload := make([]byte, c.payloadSize)
	for i := range payload {
		payload[i] = byte(i)
	}

	var curMetric uint64
	for loop := range lifeVolMaxSendLoops {
		if ctx.Err() != nil {
			return peakBytes, peakPackets, ctx.Err()
		}
		batch := 1
		if curMetric < c.stepAt {
			batch = c.fastBatch
		}
		for range batch {
			if _, writeErr := conn.Write(payload); writeErr != nil {
				return peakBytes, peakPackets, fmt.Errorf("%s: send UDP packet: %w", c.label, writeErr)
			}
		}

		states, readErr := readStates()
		if readErr != nil {
			return peakBytes, peakPackets, fmt.Errorf("%s: read XFRM counters: %w", c.label, readErr)
		}
		if len(replacementStates(states, initialSet)) >= 2 {
			return peakBytes, peakPackets, nil
		}

		// Match the initial outbound SA state specifically so its counters are tracked
		// even during a make-before-break transition window.
		var initialOnly []xfrmStateCounter
		for i := range states {
			if initialSet[states[i].spi] {
				initialOnly = append(initialOnly, states[i])
			}
		}
		outBytes, outPackets, ok := outboundCounter(initialOnly, c.outerLocal, c.outerRemote)
		if !ok {
			if len(replacementStates(states, initialSet)) > 0 {
				return peakBytes, peakPackets, nil
			}
			return peakBytes, peakPackets, fmt.Errorf("%s: initial outbound state %s -> %s vanished without replacement (spis=%v)",
				c.label, c.outerLocal, c.outerRemote, xfrmStateSPIs(states))
		}
		peakBytes = max(peakBytes, outBytes)
		peakPackets = max(peakPackets, outPackets)

		if loop == 0 {
			if outBytes == 0 {
				return peakBytes, peakPackets, fmt.Errorf("%s: outbound XFRM state did not count UDP traffic (%s -> %s)",
					c.label, c.innerLocal, c.innerRemote)
			}
			if outPackets == 0 {
				return peakBytes, peakPackets, fmt.Errorf("%s: outbound XFRM state packet counter remained zero (%s -> %s)",
					c.label, c.innerLocal, c.innerRemote)
			}
		}

		if c.byPackets {
			curMetric = outPackets
		} else {
			curMetric = outBytes
		}
		if curMetric >= c.stopAt {
			return peakBytes, peakPackets, nil
		}
	}
	return peakBytes, peakPackets, errors.New(c.label + ": traffic loop hit bound before reaching soft threshold")
}
