// Design: docs/architecture/testing/interop.md -- the topology suite.
// Related: check_pppoe_dhcpv6_pd.go -- the WAN half, which hands this file the delegated prefix.
// Related: register_pppoe_dhcpv6_pd.go -- the scenario both halves judge.
package topology

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

const (
	// pdRALateAge is how long after accel-ppp's Reply an RA must have left
	// for its lifetimes to show they count down: a CPE that advertised the
	// lease's whole lifetimes at every RA would be above what remains by
	// this much, far past pdBoundSlackSeconds.
	pdRALateAge = 10 * time.Second
	// pdBoundSlackSeconds absorbs the second accel-ppp's log truncates its
	// timestamp to (the Reply left up to a second after the logged second)
	// and the half second of rounding between that log and tcpdump's clock.
	pdBoundSlackSeconds = 1.5
	// pdAccelLogTimeLayout is the timestamp log_file.c writes between the
	// brackets that open each line (strftime "%Y-%m-%d %H:%M:%S", local time,
	// which is UTC in the Alpine image: it carries no zone file).
	pdAccelLogTimeLayout = "2006-01-02 15:04:05"
	// pdDHCPCDDropped is what dhcpcd's ipv6nd_handlera (src/ipv6nd.c) logs
	// when a router it held as default sends Router Lifetime 0: "%s: %s: no
	// longer a default router (lifetime = 0)", interface then router.
	pdDHCPCDDropped = "no longer a default router (lifetime = 0)"
)

// checkDelegatedLAN proves spec-home-cpe-8 Phases 4 and 5 on lan0, the unit
// that takes subnet-id 1 of delegated: lan0 holds the /64's ::1 with
// lifetimes no longer than the lease (AC-7); the dhcpcd host autoconfigures
// from the RA's PIO, holds Ze as its default router and a route to the
// delegated prefix from the RIO, and pings accel-ppp across Ze (AC-8, AC-13);
// every RA carrying the /64 advertises lifetimes no longer than what remains
// of the delegation, the configured Router Lifetime, and the RIO (RFC 9915
// Section 6.3, AC-9 first row); and once accel-ppp terminates the session,
// the next RA carries Router Lifetime 0, no later one carries more, and
// dhcpcd drops Ze as its default router (RFC 7084 G-5, AC-11).
func checkDelegatedLAN(ctx context.Context, check *interoplab.CheckContext, delegated netip.Prefix) error {
	subnet, err := delegatedSubnet(delegated, pdSubnetID)
	if err != nil {
		return err
	}
	unitAddress := netip.PrefixFrom(subnetHost(subnet, 1), 64)
	if err := pollUntil(ctx, pdWait, pdLANDevice+" holds "+unitAddress.String(), func() error {
		answer, err := check.Lab.Query(ctx, nodeCPE, []string{"ip", "-o", "-6", ipWordAddress, wordShow, ipWordDev, pdLANDevice, "scope", "global"}, nil)
		if err != nil {
			return err
		}
		return unitAddressVerdict(answer, unitAddress)
	}); err != nil {
		return err
	}
	router, err := cpeLANLinkLocal(ctx, check)
	if err != nil {
		return err
	}
	host, err := check.Member(nodeSLAACHost, pdLANLink)
	if err != nil {
		return err
	}
	if err := checkSLAACHost(ctx, check, host.Device, subnet, delegated, router); err != nil {
		return err
	}
	if err := pollUntil(ctx, pdWait, "every RA on "+pdLANDevice+" bounds its lifetimes by the delegation's", func() error {
		return advertisedLifetimes(ctx, check, subnet, delegated)
	}); err != nil {
		return err
	}
	return checkSessionEndWithdrawsRouter(ctx, check, router, host.Device)
}

// checkSLAACHost starts dhcpcd on the LAN host and waits for what it builds
// from the CPE's RAs: an address in subnet, a default route via router, the
// RIO's route to delegated via router, and an IPv6 ping to accel-ppp that
// crosses Ze both ways.
func checkSLAACHost(ctx context.Context, check *interoplab.CheckContext, device string, subnet, delegated netip.Prefix, router netip.Addr) error {
	if _, err := execOK(ctx, check, nodeSLAACHost, []string{"dhcpcd", "-6", "-b", "-j", pdDHCPCDLog, device}); err != nil {
		return fmt.Errorf("start dhcpcd on %s: %w", nodeSLAACHost, err)
	}
	if err := pollUntil(ctx, pdWait, nodeSLAACHost+" holds a SLAAC address in "+subnet.String(), func() error {
		answer, err := check.Lab.Query(ctx, nodeSLAACHost, []string{"ip", "-o", "-6", ipWordAddress, wordShow, ipWordDev, device, "scope", "global"}, nil)
		if err != nil {
			return err
		}
		return slaacVerdict(answer, subnet)
	}); err != nil {
		return err
	}
	if err := pollUntil(ctx, pdWait, nodeSLAACHost+"'s ::/0 via "+router.String(), func() error {
		answer, err := check.Lab.Query(ctx, nodeSLAACHost, []string{"ip", "-6", ipWordRoute, wordShow, "default"}, nil)
		if err != nil {
			return err
		}
		return defaultRouterVerdict(answer, router.String(), device)
	}); err != nil {
		return err
	}
	if err := pollUntil(ctx, pdWait, nodeSLAACHost+"'s route to "+delegated.String()+" from the RIO", func() error {
		answer, err := check.Lab.Query(ctx, nodeSLAACHost, []string{"ip", "-6", ipWordRoute, wordShow, delegated.String()}, nil)
		if err != nil {
			return err
		}
		return rioRouteVerdict(answer, delegated, router, device)
	}); err != nil {
		return err
	}
	return pollUntil(ctx, pdWait, nodeSLAACHost+" pings "+pdAccelTarget+" through the CPE", func() error {
		_, err := execOK(ctx, check, nodeSLAACHost, []string{"ping", "-c", "1", "-W", "2", pdAccelTarget})
		return err
	})
}

// checkSessionEndWithdrawsRouter ends the PPPoE session for good and judges
// the LAN: accel-ppp first stops answering PADI, so the CPE cannot dial a
// second session and bring W back, then terminates the one it holds.
func checkSessionEndWithdrawsRouter(ctx context.Context, check *interoplab.CheckContext, router netip.Addr, device string) error {
	since, err := cpeClock(ctx, check)
	if err != nil {
		return err
	}
	// "--" ends accel-cmd's getopt_long options, so -1 reaches the cli as
	// the delay rather than as an option.
	if _, err := execOK(ctx, check, nodeAccel, []string{"accel-cmd", "--", "pppoe", "set", "PADO-delay", "-1"}); err != nil {
		return fmt.Errorf("stop accel-ppp answering PADI: %w", err)
	}
	if _, err := execOK(ctx, check, nodeAccel, []string{"accel-cmd", "terminate", "username", pdAccelUsername}); err != nil {
		return fmt.Errorf("accel-cmd terminate username %s: %w", pdAccelUsername, err)
	}
	if err := pollUntil(ctx, pdWait, "the RA after the session ended carries Router Lifetime 0", func() error {
		advertisements, err := readLANAdvertisements(ctx, check)
		if err != nil {
			return err
		}
		return zeroLifetimeVerdict(advertisements, since)
	}); err != nil {
		return err
	}
	if err := pollUntil(ctx, pdWait, nodeSLAACHost+" drops "+router.String()+" as its default router", func() error {
		answer, err := check.Lab.Query(ctx, nodeSLAACHost, []string{"ip", "-6", ipWordRoute, wordShow, "default"}, nil)
		if err != nil {
			return err
		}
		if defaultRouterVerdict(answer, router.String(), device) == nil {
			return fmt.Errorf("%s still routes ::/0 via %s on %s:\n%s", nodeSLAACHost, router, device, answer)
		}
		return nil
	}); err != nil {
		return err
	}
	log, err := execOK(ctx, check, nodeSLAACHost, []string{commandCat, pdDHCPCDLog})
	if err != nil {
		return err
	}
	if !strings.Contains(log, device+": "+router.String()+": "+pdDHCPCDDropped) {
		return fmt.Errorf("dhcpcd on %s never logged %s as %q:\n%s", nodeSLAACHost, router, pdDHCPCDDropped, interoplab.OutputExcerpt(log))
	}
	return nil
}

// advertisedLifetimes reads the CPE's RA capture and accel-ppp's DHCPv6 log
// and judges them together (advertisedLifetimesVerdict).
func advertisedLifetimes(ctx context.Context, check *interoplab.CheckContext, subnet, delegated netip.Prefix) error {
	advertisements, err := readLANAdvertisements(ctx, check)
	if err != nil {
		return err
	}
	log, err := check.Lab.Query(ctx, nodeAccel, []string{commandCat, pdAccelLog}, nil)
	if err != nil {
		return err
	}
	replies, err := replyTimes(log, netip.MustParsePrefix(pdDelegatePool))
	if err != nil {
		return err
	}
	return advertisedLifetimesVerdict(advertisements, replies, subnet, delegated)
}

// cpeLANLinkLocal answers lan0's link-local address: the RA's source, so
// the LAN host's default router.
func cpeLANLinkLocal(ctx context.Context, check *interoplab.CheckContext) (netip.Addr, error) {
	answer, err := check.Lab.Query(ctx, nodeCPE, []string{"ip", "-o", "-6", ipWordAddress, wordShow, ipWordDev, pdLANDevice, "scope", "link"}, nil)
	if err != nil {
		return netip.Addr{}, err
	}
	for line := range strings.SplitSeq(answer, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		prefix, err := netip.ParsePrefix(fields[3])
		if err != nil {
			continue
		}
		if prefix.Addr().IsLinkLocalUnicast() {
			return prefix.Addr(), nil
		}
	}
	return netip.Addr{}, fmt.Errorf("%s holds no link-local address to send RAs from:\n%s", pdLANDevice, answer)
}

// cpeClock answers the CPE's clock, which stamps its RA capture.
func cpeClock(ctx context.Context, check *interoplab.CheckContext) (time.Time, error) {
	answer, err := execOK(ctx, check, nodeCPE, []string{"date", "+%s"})
	if err != nil {
		return time.Time{}, err
	}
	seconds, err := strconv.ParseInt(strings.TrimSpace(answer), 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("ze-cpe's clock read %q, not seconds since the epoch: %w", answer, err)
	}
	return time.Unix(seconds, 0), nil
}

// readLANAdvertisements answers every RA the CPE has sent on lan0.
func readLANAdvertisements(ctx context.Context, check *interoplab.CheckContext) ([]advertisement, error) {
	capture, err := check.Lab.Query(ctx, nodeCPE, []string{"tcpdump", "-n", "-tt", "-v", "-r", pdLANCapture}, nil)
	if err != nil {
		return nil, err
	}
	return parseAdvertisements(capture)
}

// delegatedSubnet answers the /64 of delegated that subnetID names: the
// delegated prefix with subnetID in the bits between its length and 64, as
// the delegated-prefix list's subnet-id is defined.
func delegatedSubnet(delegated netip.Prefix, subnetID uint16) (netip.Prefix, error) {
	if !delegated.Addr().Is6() {
		return netip.Prefix{}, fmt.Errorf("delegated prefix %s is not IPv6", delegated)
	}
	room := 64 - delegated.Bits()
	if room < 0 {
		return netip.Prefix{}, fmt.Errorf("delegated prefix %s is longer than a /64", delegated)
	}
	if room < 16 && int(subnetID) >= 1<<room {
		return netip.Prefix{}, fmt.Errorf("subnet-id %d does not fit the %d bits a /%d leaves below /64", subnetID, room, delegated.Bits())
	}
	octets := delegated.Masked().Addr().As16()
	high := binary.BigEndian.Uint64(octets[:8]) | uint64(subnetID)
	binary.BigEndian.PutUint64(octets[:8], high)
	return netip.PrefixFrom(netip.AddrFrom16(octets), 64), nil
}

// subnetHost answers the address of subnet whose interface identifier is
// identifier.
func subnetHost(subnet netip.Prefix, identifier uint64) netip.Addr {
	octets := subnet.Masked().Addr().As16()
	binary.BigEndian.PutUint64(octets[8:], identifier)
	return netip.AddrFrom16(octets)
}

// unitAddressVerdict reads `ip -o -6 address show dev lan0 scope global` and
// accepts want with finite lifetimes no longer than accel-ppp's lease grants
// (RFC 9915 Section 6.3): a lifetime the kernel shows as forever, or above
// the lease, was not taken from the delegation.
func unitAddressVerdict(answer string, want netip.Prefix) error {
	for line := range strings.SplitSeq(answer, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		if fields[3] != want.String() {
			continue
		}
		valid, err := addressLifetime(fields, "valid_lft")
		if err != nil {
			return err
		}
		preferred, err := addressLifetime(fields, "preferred_lft")
		if err != nil {
			return err
		}
		if valid > pdValidLifetime {
			return fmt.Errorf("%s's valid lifetime %ds is above the %ds accel-ppp delegates for (RFC 9915 Section 6.3):\n%s", want, valid, pdValidLifetime, answer)
		}
		if preferred > pdPreferredLifetime {
			return fmt.Errorf("%s's preferred lifetime %ds is above the %ds accel-ppp delegates for (RFC 9915 Section 6.3):\n%s", want, preferred, pdPreferredLifetime, answer)
		}
		return nil
	}
	return fmt.Errorf("%s does not hold %s:\n%s", pdLANDevice, want, answer)
}

// addressLifetime answers the seconds `ip address` prints after word
// ("valid_lft 117sec"), refusing forever.
func addressLifetime(fields []string, word string) (int, error) {
	for index, field := range fields[:len(fields)-1] {
		if field != word {
			continue
		}
		text := fields[index+1]
		seconds, found := strings.CutSuffix(text, "sec")
		if !found {
			return 0, fmt.Errorf("%s is %s, not a lifetime the delegation bounds", word, text)
		}
		return strconv.Atoi(seconds)
	}
	return 0, fmt.Errorf("the address line carries no %s", word)
}

// rioRouteVerdict reads `ip -6 route show <destination>` and accepts a route
// to destination via router on device: the one dhcpcd's inet6_raroutes
// (src/ipv6.c) installs from a Route Information option.
func rioRouteVerdict(answer string, destination netip.Prefix, router netip.Addr, device string) error {
	for line := range strings.SplitSeq(answer, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		if fields[0] != destination.String() {
			continue
		}
		if strings.Join(fields[1:5], " ") == "via "+router.String()+" "+ipWordDev+" "+device {
			return nil
		}
	}
	return fmt.Errorf("no route to %s via %s dev %s:\n%s", destination, router, device, answer)
}

// advertisement is one RA the CPE sent, as `tcpdump -tt -v` prints it
// (icmp6_print and icmp6_opt_print, print-icmp6.c).
type advertisement struct {
	at time.Time
	// routerLifetime is the RA header's Router Lifetime, in seconds.
	routerLifetime uint32
	// prefixes are its Prefix Information options, routes its Route
	// Information options (preferred is zero for a route).
	prefixes []advertisedPrefix
	routes   []advertisedPrefix
}

// advertisedPrefix is one prefix an RA option carries, with its lifetimes in
// seconds.
type advertisedPrefix struct {
	prefix    netip.Prefix
	valid     uint32
	preferred uint32
}

// The tokens tcpdump's icmp6_print and icmp6_opt_print write for an RA.
const (
	tcpdumpRA             = "router advertisement"
	tcpdumpRouterLifetime = "router lifetime "
	tcpdumpPrefixOption   = "prefix info option (3)"
	tcpdumpRouteOption    = "route info option (24)"
	tcpdumpOptionBody     = "): "
	tcpdumpValidTime      = "valid time "
	tcpdumpPreferredTime  = "pref. time "
	tcpdumpRouteLifetime  = "lifetime="
	tcpdumpInfinity       = "infinity"
)

// parseAdvertisements reads `tcpdump -n -tt -v -r` over the RA capture. A
// packet's first line starts at the margin with its epoch timestamp, and its
// continuation lines are indented.
func parseAdvertisements(capture string) ([]advertisement, error) {
	var advertisements []advertisement
	var record []string
	flush := func() error {
		if len(record) == 0 {
			return nil
		}
		defer func() { record = nil }()
		if !strings.Contains(record[0], tcpdumpRA) {
			return nil
		}
		parsed, err := parseAdvertisement(record)
		if err != nil {
			return err
		}
		advertisements = append(advertisements, parsed)
		return nil
	}
	for line := range strings.SplitSeq(capture, "\n") {
		if line == "" {
			continue
		}
		if line[0] != ' ' && line[0] != '\t' {
			if err := flush(); err != nil {
				return nil, err
			}
		}
		record = append(record, line)
	}
	if err := flush(); err != nil {
		return nil, err
	}
	return advertisements, nil
}

// parseAdvertisement reads one RA's lines.
func parseAdvertisement(record []string) (advertisement, error) {
	stamp, _, _ := strings.Cut(record[0], " ")
	seconds, err := strconv.ParseFloat(stamp, 64)
	if err != nil {
		return advertisement{}, fmt.Errorf("RA line %q opens with no epoch timestamp: %w", record[0], err)
	}
	parsed := advertisement{at: time.Unix(0, int64(seconds*float64(time.Second)))}
	lifetimeFound := false
	for _, line := range record[1:] {
		if _, after, found := strings.Cut(line, tcpdumpRouterLifetime); found {
			text, _, _ := strings.Cut(after, ",")
			value, err := lifetimeSeconds(text)
			if err != nil {
				return advertisement{}, err
			}
			parsed.routerLifetime = value
			lifetimeFound = true
			continue
		}
		if strings.Contains(line, tcpdumpPrefixOption) {
			prefix, err := parseOption(line, tcpdumpValidTime, tcpdumpPreferredTime)
			if err != nil {
				return advertisement{}, err
			}
			parsed.prefixes = append(parsed.prefixes, prefix)
			continue
		}
		if strings.Contains(line, tcpdumpRouteOption) {
			route, err := parseOption(line, tcpdumpRouteLifetime, "")
			if err != nil {
				return advertisement{}, err
			}
			parsed.routes = append(parsed.routes, route)
		}
	}
	if !lifetimeFound {
		return advertisement{}, fmt.Errorf("RA at %s carries no Router Lifetime in tcpdump's print:\n%s", parsed.at, strings.Join(record, "\n"))
	}
	return parsed, nil
}

// parseOption reads a PIO or RIO line: the prefix after the option header,
// then the lifetime after validWord and, when preferredWord is not empty, the
// one after it.
func parseOption(line, validWord, preferredWord string) (advertisedPrefix, error) {
	_, body, found := strings.Cut(line, tcpdumpOptionBody)
	if !found {
		return advertisedPrefix{}, fmt.Errorf("RA option line %q has no body", line)
	}
	body = strings.TrimSpace(body)
	text, _, _ := strings.Cut(body, ",")
	prefix, err := netip.ParsePrefix(text)
	if err != nil {
		return advertisedPrefix{}, fmt.Errorf("RA option line %q: %w", line, err)
	}
	option := advertisedPrefix{prefix: prefix}
	option.valid, err = optionLifetime(body, validWord)
	if err != nil {
		return advertisedPrefix{}, fmt.Errorf("RA option line %q: %w", line, err)
	}
	if preferredWord == "" {
		return option, nil
	}
	option.preferred, err = optionLifetime(body, preferredWord)
	if err != nil {
		return advertisedPrefix{}, fmt.Errorf("RA option line %q: %w", line, err)
	}
	return option, nil
}

// optionLifetime answers the lifetime that follows word in body.
func optionLifetime(body, word string) (uint32, error) {
	_, after, found := strings.Cut(body, word)
	if !found {
		return 0, fmt.Errorf("no %q", word)
	}
	text, _, _ := strings.Cut(after, ",")
	return lifetimeSeconds(strings.TrimSpace(text))
}

// lifetimeSeconds reads tcpdump's get_lifetime: "<n>s", or "infinity" for
// 0xffffffff.
func lifetimeSeconds(text string) (uint32, error) {
	if text == tcpdumpInfinity {
		return ^uint32(0), nil
	}
	digits, found := strings.CutSuffix(text, "s")
	if !found {
		return 0, fmt.Errorf("lifetime %q is not <n>s", text)
	}
	value, err := strconv.ParseUint(digits, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("lifetime %q: %w", text, err)
	}
	return uint32(value), nil
}

// replyTimes answers when accel-ppp sent each DHCPv6 Reply delegating a
// prefix from pool, from the timestamp that opens its log line.
func replyTimes(log string, pool netip.Prefix) ([]time.Time, error) {
	var replies []time.Time
	for line := range strings.SplitSeq(log, "\n") {
		if !strings.Contains(line, accelSendReply) {
			continue
		}
		if _, inPool := iaPrefixIn(line, pool); !inPool {
			continue
		}
		at, err := accelLogTime(line)
		if err != nil {
			return nil, err
		}
		replies = append(replies, at)
	}
	return replies, nil
}

// accelLogTime reads the "[YYYY-MM-DD HH:MM:SS]" that opens an accel-ppp log
// line.
func accelLogTime(line string) (time.Time, error) {
	_, after, found := strings.Cut(line, "[")
	if !found {
		return time.Time{}, fmt.Errorf("accel-ppp log line %q opens with no timestamp", line)
	}
	if len(after) < len(pdAccelLogTimeLayout) {
		return time.Time{}, fmt.Errorf("accel-ppp log line %q opens with no timestamp", line)
	}
	at, err := time.ParseInLocation(pdAccelLogTimeLayout, after[:len(pdAccelLogTimeLayout)], time.UTC)
	if err != nil {
		return time.Time{}, fmt.Errorf("accel-ppp log line %q: %w", line, err)
	}
	return at, nil
}

// errNoLateAdvertisement says no RA carrying the /64 left late enough after
// a Reply to tell a counted-down lifetime from the lease's whole one.
var errNoLateAdvertisement = errors.New("no RA carried the delegated /64 ten seconds or more after accel-ppp's Reply")

// advertisedLifetimesVerdict judges every RA carrying subnet's PIO against
// the accel-ppp Reply that preceded it, the moment the delegation's
// lifetimes started to count: the PIO's valid and preferred lifetimes and the
// RIO's lifetime are no longer than what remained of the lease (RFC 9915
// Section 6.3: "the advertised preferred and valid lifetimes MUST NOT exceed
// the corresponding remaining lifetimes of the delegated prefix"), the RA
// carries a RIO for delegated (RFC 7084 L-3), and its Router Lifetime is the
// configured one, since the WAN holds a default router and the unit a prefix
// (the condition matrix's first row). At least one judged RA must have left
// pdRALateAge after its Reply, so an RA that always carries the lease's whole
// lifetimes fails; an RA no Reply precedes within the lease is not judged, so
// a skew between the two clocks fails for want of a judged RA rather than
// passing.
func advertisedLifetimesVerdict(advertisements []advertisement, replies []time.Time, subnet, delegated netip.Prefix) error {
	judged := 0
	late := 0
	for _, advert := range advertisements {
		pio, found := findAdvertised(advert.prefixes, subnet)
		if !found {
			continue
		}
		reply, found := latestReply(replies, advert.at)
		if !found {
			continue
		}
		age := advert.at.Sub(reply)
		if age > pdValidLifetime*time.Second {
			continue
		}
		judged++
		if age >= pdRALateAge {
			late++
		}
		if err := withinRemaining("PIO valid", pio.valid, pdValidLifetime, age, advert.at); err != nil {
			return err
		}
		if err := withinRemaining("PIO preferred", pio.preferred, pdPreferredLifetime, age, advert.at); err != nil {
			return err
		}
		rio, found := findAdvertised(advert.routes, delegated)
		if !found {
			return fmt.Errorf("the RA at %s carries the /64 %s and no Route Information option for the delegated %s (RFC 7084 L-3)", advert.at, subnet, delegated)
		}
		if err := withinRemaining("RIO", rio.valid, pdValidLifetime, age, advert.at); err != nil {
			return err
		}
		if advert.routerLifetime != pdRouterLifetime {
			return fmt.Errorf("the RA at %s carries the /64 %s with the WAN up and Router Lifetime %ds, want the configured %ds", advert.at, subnet, advert.routerLifetime, pdRouterLifetime)
		}
	}
	if judged == 0 {
		return fmt.Errorf("no RA carrying the /64 %s followed one of accel-ppp's %d Replies within its %ds lease", subnet, len(replies), pdValidLifetime)
	}
	if late == 0 {
		return fmt.Errorf("%w (%d judged)", errNoLateAdvertisement, judged)
	}
	return nil
}

// withinRemaining refuses an advertised lifetime above what remained of the
// lease age after the Reply that granted it, less pdBoundSlackSeconds.
func withinRemaining(what string, advertised, lease uint32, age time.Duration, at time.Time) error {
	remaining := float64(lease) - age.Seconds()
	if float64(advertised) <= remaining+pdBoundSlackSeconds {
		return nil
	}
	return fmt.Errorf("the RA at %s advertises a %s lifetime of %ds, %.1fs after accel-ppp's Reply granted %ds, so above the %.1fs that remained (RFC 9915 Section 6.3)", at, what, advertised, age.Seconds(), lease, remaining)
}

// findAdvertised answers the option for prefix.
func findAdvertised(options []advertisedPrefix, prefix netip.Prefix) (advertisedPrefix, bool) {
	for _, option := range options {
		if option.prefix == prefix {
			return option, true
		}
	}
	return advertisedPrefix{}, false
}

// latestReply answers the last Reply logged no later than at.
func latestReply(replies []time.Time, at time.Time) (time.Time, bool) {
	var latest time.Time
	found := false
	for _, reply := range replies {
		if reply.After(at) {
			continue
		}
		if found && !reply.After(latest) {
			continue
		}
		latest = reply
		found = true
	}
	return latest, found
}

// errNoZeroLifetime says the capture holds no RA with Router Lifetime 0 since
// the session was terminated.
var errNoZeroLifetime = errors.New("no RA with Router Lifetime 0 since the session was terminated")

// zeroLifetimeVerdict judges the RAs sent since the session was terminated:
// one with Router Lifetime 0 within one maximum interval and a second, so it
// is the next RA rather than a later periodic one, and none after it with a
// lifetime above 0, since accel-ppp answers no second session. RFC 7084
// Section 4.1 G-5: "it MUST explicitly invalidate itself as an IPv6 default
// router on each of its advertising interfaces by immediately transmitting
// one or more Router Advertisement messages with the "Router Lifetime" field
// set to zero [RFC4861]."
func zeroLifetimeVerdict(advertisements []advertisement, since time.Time) error {
	var first advertisement
	found := false
	for _, advert := range advertisements {
		if advert.at.Before(since) {
			continue
		}
		if !found {
			if advert.routerLifetime != 0 {
				continue
			}
			first = advert
			found = true
			continue
		}
		if advert.routerLifetime != 0 {
			return fmt.Errorf("the RA at %s carries Router Lifetime %ds after the one at %s carried 0, with no session up", advert.at, advert.routerLifetime, first.at)
		}
	}
	if !found {
		return errNoZeroLifetime
	}
	if first.at.Sub(since) > (pdRAIntervalMax+1)*time.Second {
		return fmt.Errorf("the first RA with Router Lifetime 0 left %s after the session was terminated, past the next RA (maximum-interval %ds)", first.at.Sub(since), pdRAIntervalMax)
	}
	return nil
}

// pdLANEvidence answers the state a failed LAN step leaves, for the failure
// report: whether the CPE still holds a WAN default route (the matrix's W
// input), what lan0 holds, what Ze says it holds of the delegation, the RAs
// it sent with their Router Lifetime, PIO and RIO, and what dhcpcd heard and
// installed. The harness's own dump shows only links and addresses. Each
// command's failure is printed in its place: evidence is best effort and
// never replaces the error it follows.
func pdLANEvidence(ctx context.Context, check *interoplab.CheckContext) string {
	probes := []struct {
		peer string
		argv []string
	}{
		{nodeCPE, []string{"ip", "-6", ipWordRoute, wordShow}},
		{nodeCPE, []string{"ip", "-6", ipWordAddress, wordShow, ipWordDev, pdLANDevice}},
		{nodeCPE, []string{"sh", "-c", "for f in /proc/sys/net/ipv6/conf/all/forwarding /proc/sys/net/ipv6/conf/pppoe0/accept_ra /proc/sys/net/ipv6/conf/pppoe0/forwarding /proc/sys/net/ipv6/conf/lan0/forwarding; do echo \"$f $(cat $f)\"; done"}},
		{nodeCPE, interoplab.ZeCLICommand("show interface delegated-prefix")},
		{nodeCPE, interoplab.ZeCLICommand("show interface pppoe-client")},
		{nodeCPE, []string{"tcpdump", "-n", "-tt", "-v", "-r", pdLANCapture}},
		{nodeSLAACHost, []string{"ip", "-6", ipWordRoute, wordShow}},
		{nodeSLAACHost, []string{commandCat, pdDHCPCDLog}},
	}
	var report strings.Builder
	for _, probe := range probes {
		report.WriteString("--- " + probe.peer + ": " + strings.Join(probe.argv, " ") + "\n")
		result, err := check.Lab.Exec(ctx, probe.peer, probe.argv, nil)
		if err != nil {
			report.WriteString("(not run: " + err.Error() + ")\n")
			continue
		}
		report.WriteString(interoplab.OutputExcerpt(result.Stdout))
		if result.Stderr != "" {
			report.WriteString("\nstderr: " + interoplab.OutputExcerpt(result.Stderr))
		}
		report.WriteString("\nexit " + strconv.Itoa(result.ExitCode) + "\n")
	}
	return report.String()
}
