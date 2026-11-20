// Design: docs/labs/pppoe-interop.md -- discriminate real product observations.
// Related: product_check.go -- combine daemon identity, native FIB and wire traffic.
package testvpppppoe

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"regexp"
	"strconv"
	"strings"
)

type productAccess struct {
	Name  string `json:"name"`
	Index int    `json:"ifindex"`
	Max   int    `json:"max-sessions"`
}

type productSession struct {
	SID     uint16 `json:"sid"`
	MAC     string `json:"mac"`
	Access  string `json:"interface"`
	Service string `json:"service-name"`
	State   string `json:"state"`
	Unit    int    `json:"unit"`
	Name    string `json:"ppp-interface"`
	Index   uint32 `json:"interface-index"`
}

type productNative struct {
	SID          uint16
	MAC          string
	CreationIPv4 string // The native dump's creation value is not negotiated IPCP state.
	Index        uint32
}

type productLink struct{ Name, Address string }

func validateProductSessions(rows []productSession, access string, count int) error {
	if rows == nil {
		return errors.New("daemon sessions is null")
	}
	if len(rows) != count {
		return fmt.Errorf("daemon has %d sessions, want %d", len(rows), count)
	}
	seenSID, seenIndex, seenMAC := make(map[uint16]bool), make(map[uint32]bool), make(map[string]bool)
	for _, row := range rows {
		if row.SID == 0 || row.Index == 0 || row.Index == ^uint32(0) || row.Unit != -1 || !strings.HasPrefix(row.Name, "pppoe_session") || row.Access != access || row.Service != productService || row.State != "session" {
			return fmt.Errorf("daemon session lacks native identity: %+v", row)
		}
		if _, err := strconv.ParseUint(strings.TrimPrefix(row.Name, "pppoe_session"), 10, 32); err != nil {
			return fmt.Errorf("invalid native name %q", row.Name)
		}
		mac, err := net.ParseMAC(row.MAC)
		if err != nil || len(mac) != 6 {
			return fmt.Errorf("invalid subscriber MAC %q", row.MAC)
		}
		if mac[0]&1 != 0 {
			return fmt.Errorf("subscriber MAC is multicast: %s", row.MAC)
		}
		if seenSID[row.SID] || seenIndex[row.Index] || seenMAC[mac.String()] {
			return errors.New("daemon native identities are not unique")
		}
		seenSID[row.SID], seenIndex[row.Index], seenMAC[mac.String()] = true, true, true
	}
	return nil
}

// The pinned upstream format_pppoe_session prints one header and one MAC row
// per native session. Strict cardinality avoids accidentally matching the
// MAC of one subscriber to the SID or address of another.
// vppctl ends each line with CR LF, so a line is cut at either before its
// fields are read: a trailing CR would make the session count unparseable.
func parseProductNative(output string) ([]productNative, error) {
	output = strings.ReplaceAll(output, "\r\n", "\n")
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) == 1 && productNativeEmpty(output) {
		return []productNative{}, nil
	}
	if len(lines) < 3 || !strings.HasPrefix(lines[0], "Number of PPPoE sessions: ") {
		return nil, fmt.Errorf("native session header missing: %s", output)
	}
	count, err := strconv.Atoi(strings.TrimPrefix(lines[0], "Number of PPPoE sessions: "))
	if err != nil || count <= 0 || len(lines) != 1+2*count {
		return nil, fmt.Errorf("native session rows do not match their count: %s", output)
	}
	rows := make([]productNative, 0, count)
	for index := range count {
		header, trailer := strings.Fields(lines[1+2*index]), strings.Fields(lines[2+2*index])
		if len(header) != 13 || header[1] != "sw-if-index" || header[3] != "client-ip4" || header[5] != "client-ip6" || header[7] != "session-id" || header[9] != "encap-if-index" || header[11] != "decap-fib-index" || len(trailer) != 4 || trailer[0] != "local-mac" || trailer[2] != "client-mac" {
			return nil, fmt.Errorf("unrecognized native session row: %s\n%s", lines[1+2*index], lines[2+2*index])
		}
		nativeIndex, err := strconv.ParseUint(header[2], 10, 32)
		if err != nil || nativeIndex == 0 || nativeIndex == uint64(^uint32(0)) {
			return nil, errors.New("invalid native session index")
		}
		sid, err := strconv.ParseUint(header[8], 10, 16)
		if err != nil || sid == 0 {
			return nil, errors.New("invalid native SID")
		}
		address, err := netip.ParseAddr(header[4])
		if err != nil || !address.Is4() {
			return nil, errors.New("invalid native IPv4 address")
		}
		mac, err := net.ParseMAC(trailer[3])
		if err != nil || len(mac) != 6 {
			return nil, errors.New("invalid native subscriber MAC")
		}
		rows = append(rows, productNative{SID: uint16(sid), MAC: mac.String(), CreationIPv4: address.String(), Index: uint32(nativeIndex)})
	}
	return rows, nil
}

func matchProductNative(sessions []productSession, native []productNative) error {
	if len(sessions) != len(native) {
		return fmt.Errorf("daemon/native populations differ: %d/%d", len(sessions), len(native))
	}
	seen := make(map[uint32]bool, len(native))
	for _, session := range sessions {
		found := false
		for _, row := range native {
			if row.Index == session.Index && row.SID == session.SID && row.MAC == session.MAC {
				if seen[row.Index] {
					return errors.New("native session matched more than once")
				}
				seen[row.Index], found = true, true
			}
		}
		if !found {
			return fmt.Errorf("daemon session has no matching native row: %+v", session)
		}
	}
	return nil
}

func parseProductLinks(output string) ([]productLink, error) {
	var devices []struct {
		Name      string `json:"ifname"`
		Addresses []struct {
			Local string `json:"local"`
			// iproute2's JSON names a point-to-point peer "address" (the
			// IFA_ADDRESS beside IFA_LOCAL); only its text form says "peer".
			Peer string `json:"address"`
		} `json:"addr_info"`
	}
	if err := json.Unmarshal([]byte(output), &devices); err != nil {
		return nil, err
	}
	if devices == nil {
		return nil, errors.New("peer address answer is null")
	}
	rows := make([]productLink, 0, len(devices))
	for _, device := range devices {
		if len(device.Addresses) != 1 {
			continue
		}
		address := device.Addresses[0]
		if !strings.HasPrefix(device.Name, "ppp") || address.Peer != serverAddress {
			continue
		}
		parsed, err := netip.ParseAddr(address.Local)
		if err != nil || !parsed.Is4() {
			return nil, fmt.Errorf("peer IPCP address is not IPv4: %s", address.Local)
		}
		bytes := parsed.As4()
		if bytes[0] != 10 || bytes[1] != 20 || bytes[2] != 0 || bytes[3] < 2 || bytes[3] > 102 {
			return nil, fmt.Errorf("peer IPCP address outside configured pool: %s", address.Local)
		}
		rows = append(rows, productLink{Name: device.Name, Address: address.Local})
	}
	return rows, nil
}

func productPopulation(sessions []productSession, native []productNative, links []productLink, count int) error {
	if len(sessions) != count || len(native) != count || len(links) != count {
		return fmt.Errorf("product population differs from %d: daemon=%d native=%d peer=%d", count, len(sessions), len(native), len(links))
	}
	if err := matchProductNative(sessions, native); err != nil {
		return err
	}
	seen := make(map[string]bool, count)
	for _, link := range links {
		if seen[link.Address] {
			return fmt.Errorf("IPCP address reused: %s", link.Address)
		}
		seen[link.Address] = true
	}
	return nil
}

func productOwner(output, address string, sessions []productSession) (productSession, error) {
	var found productSession
	for _, session := range sessions {
		if productHostRoute(output, address, session.Name) != nil {
			continue
		}
		if found.Index != 0 {
			return productSession{}, fmt.Errorf("peer address %s has multiple native FIB owners", address)
		}
		found = session
	}
	if found.Index == 0 {
		return productSession{}, fmt.Errorf("peer address %s has no matching daemon/native FIB owner: %s", address, output)
	}
	return found, nil
}

// productPathShape names which native path types an owned route may carry.
type productPathShape uint8

const (
	productPathUnspecified productPathShape = iota
	// productPathAnyAttached accepts an attached interface or an attached
	// next hop on it: the subscriber's own host route may name the peer.
	productPathAnyAttached
	// productPathAttachedOnly accepts only an attached interface with no
	// next hop, the device route Ze installs for a non-peer gateway.
	productPathAttachedOnly
)

// productHostRoute requires the subscriber's own /32 at preference 0 through
// the named session interface, attached or through an attached next hop.
func productHostRoute(output, address, name string) error {
	host, err := netip.ParseAddr(address)
	if err != nil {
		return fmt.Errorf("subscriber host route has invalid address %q", address)
	}
	return productOwnedPath(output, netip.PrefixFrom(host, host.BitLen()), name, 0, productPathAnyAttached)
}

// The pinned format_fib_entry prints each two-space source header followed
// by that source's four-space path-list and six-space path headers. Only an
// attached interface on that exact source's path counts; the later forwarding
// DPO and another source's paths cannot supply the missing association.
// VPP v25.10 format_fib_path prints the path type name ("attached",
// "attached-nexthop") and the configured preference on the path header, then
// the interface alone on the next line for an attached path, or the next hop
// and the interface for an attached next hop.
func productOwnedPath(output string, destination netip.Prefix, name string, preference int, shape productPathShape) error {
	switch shape {
	case productPathAnyAttached, productPathAttachedOnly:
	case productPathUnspecified:
		return errors.New("BUG: owned path check names no path shape")
	}
	prefix := destination.String()
	preferenceField := "pref=" + strconv.Itoa(preference)
	entries, ownedSources, paths, matches := 0, 0, 0, 0
	inEntry, inSource, needAttached, needNextHop := false, false, false, false
	for line := range strings.SplitSeq(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if before, _, header := strings.Cut(line, " fib:"); header {
			inEntry = strings.TrimSpace(before) == prefix
			inSource, needAttached, needNextHop = false, false, false
			if inEntry {
				entries++
			}
			continue
		}
		if !inEntry {
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(line), "forwarding:") {
			inSource, needAttached, needNextHop = false, false, false
			continue
		}
		if strings.HasPrefix(line, "  ") {
			if !strings.HasPrefix(line, "   ") {
				inSource, needAttached, needNextHop = false, false, false
				if len(fields) >= 2 {
					if strings.HasPrefix(fields[1], "refs:") {
						inSource = fields[0] == "ze-pppoe"
						if inSource {
							ownedSources++
						}
					}
				}
				continue
			}
		}
		if !inSource {
			continue
		}
		if strings.HasPrefix(line, "      path:[") {
			paths++
			needAttached, needNextHop = false, false
			if !productField(line, "ip4") {
				return fmt.Errorf("owned path of %s is not IPv4", prefix)
			}
			if !productField(line, "weight=1") {
				return fmt.Errorf("owned path of %s has another weight", prefix)
			}
			if !productField(line, preferenceField) {
				return fmt.Errorf("owned path of %s lacks preference %d", prefix, preference)
			}
			if productField(line, "attached:") {
				needAttached = true
			}
			if productField(line, "attached-nexthop:") {
				if shape == productPathAttachedOnly {
					return fmt.Errorf("owned path of %s has a next hop where a device route through the session belongs", prefix)
				}
				needNextHop = true
			}
			if !needAttached {
				if !needNextHop {
					return fmt.Errorf("owned path of %s is not an attached interface", prefix)
				}
			}
			continue
		}
		if needAttached {
			needAttached = false
			if !strings.HasPrefix(line, "        ") {
				return errors.New("owned attached interface is missing")
			}
			if len(fields) != 1 {
				return errors.New("owned attached interface is ambiguous")
			}
			if fields[0] == name {
				matches++
			}
			continue
		}
		if needNextHop {
			needNextHop = false
			if !strings.HasPrefix(line, "        ") {
				return errors.New("owned next-hop interface is missing")
			}
			if len(fields) < 2 {
				return errors.New("owned next-hop interface is incomplete")
			}
			hop, err := netip.ParseAddr(fields[0])
			if err != nil {
				return fmt.Errorf("owned next-hop is invalid: %w", err)
			}
			if !hop.Is4() {
				return errors.New("owned next-hop is not IPv4")
			}
			if fields[1] == name {
				matches++
			}
		}
	}
	if entries != 1 {
		return fmt.Errorf("native FIB has %d exact entries for %s", entries, prefix)
	}
	if ownedSources != 1 {
		return fmt.Errorf("native entry %s has %d ze-pppoe sources", prefix, ownedSources)
	}
	if paths != 1 {
		return fmt.Errorf("owned source of %s has %d paths", prefix, paths)
	}
	if matches != 1 {
		return fmt.Errorf("owned path of %s does not name %s", prefix, name)
	}
	if needAttached {
		return errors.New("owned attached path is truncated")
	}
	if needNextHop {
		return errors.New("owned next-hop path is truncated")
	}
	return nil
}

func productField(output, wanted string) bool {
	for field := range strings.FieldsSeq(output) {
		if strings.Trim(field, ",[]()") == wanted {
			return true
		}
	}
	return false
}

// pppd 2.5.5 chap.c::chap_handle_status accepts CHAP Success only after a
// Challenge/Response; chap_print_pkt supplies these independent trace words.
func productAuth(log string, accepted bool) error {
	if !strings.Contains(log, "rcvd [CHAP Challenge") || !strings.Contains(log, "sent [CHAP Response") || !strings.Contains(log, `name = "alice"`) {
		return errors.New("independent peer did not complete the named CHAP challenge/response")
	}
	if accepted {
		if !strings.Contains(log, "<auth chap MD5>") || !strings.Contains(log, "rcvd [CHAP Success") || strings.Contains(log, "rcvd [CHAP Failure") {
			return errors.New("independent peer did not observe CHAP-MD5 acceptance")
		}
		if !strings.Contains(log, "sent [LCP ConfReq") || !strings.Contains(log, "rcvd [LCP ConfAck") || !strings.Contains(log, "rcvd [LCP ConfReq") || !strings.Contains(log, "sent [LCP ConfAck") {
			return errors.New("independent peer lacks bidirectional LCP negotiation")
		}
		return nil
	}
	if !strings.Contains(log, "rcvd [CHAP Failure") || strings.Contains(log, "rcvd [CHAP Success") || strings.Contains(log, "rcvd [IPCP ConfAck") {
		return errors.New("wrong credential was not refused before IPCP")
	}
	return nil
}

func productCaptureEmpty(output string) error {
	seenCapture := false
	for line := range strings.SplitSeq(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		if fields[1] == packetsToken && fields[2] == "captured" {
			if seenCapture || fields[0] != "0" {
				return fmt.Errorf("subscriber IPv4 reached the control TAP: %s", output)
			}
			seenCapture = true
		}
	}
	if !seenCapture {
		return fmt.Errorf("CP capture has no complete zero-packet summary: %s", output)
	}
	return productCaptureNoDrops(output)
}

// productCaptureNoDrops requires a complete zero-drop kernel summary. Callers
// MUST join the capture before using its log as evidence. An interface summary
// is optional, but every reported source MUST be unique and explicitly zero.
func productCaptureNoDrops(output string) error {
	seenKernel, seenInterface := false, false
	for line := range strings.SplitSeq(output, "\n") {
		summary := strings.Join(strings.Fields(line), " ")
		dropSummary := strings.Contains(summary, "packet dropped")
		if strings.Contains(summary, "packets dropped") {
			dropSummary = true
		}
		if !dropSummary {
			continue
		}
		switch summary {
		case "0 packets dropped by kernel":
			if seenKernel {
				return fmt.Errorf("capture repeats its kernel-drop summary: %s", output)
			}
			seenKernel = true
		case "0 packets dropped by interface":
			if seenInterface {
				return fmt.Errorf("capture repeats its interface-drop summary: %s", output)
			}
			seenInterface = true
		default:
			return fmt.Errorf("capture has an unknown, malformed, or nonzero drop summary: %s", output)
		}
	}
	if !seenKernel {
		return fmt.Errorf("capture lacks its kernel-drop summary: %s", output)
	}
	return nil
}

func productNativeEmpty(output string) bool {
	return strings.TrimSpace(output) == "No pppoe sessions configured..."
}

func sameProductOwners(before, after []productSession) error {
	if len(before) != len(after) {
		return errors.New("limit refusal changed the original population")
	}
	for _, owner := range before {
		found := false
		for _, row := range after {
			if owner == row {
				found = true
			}
		}
		if !found {
			return fmt.Errorf("limit refusal displaced an existing subscriber: %+v", owner)
		}
	}
	return nil
}

// Product uses the same exact iputils measured-boundary field as the existing
// independent 1400-byte probe. A count, longer integer or unrelated 1492 text
// cannot stand in for an actual ICMP fragmentation-needed MTU.
func productMTURefusal(output string) error {
	if !regexp.MustCompile(`\bmtu\s*=\s*1492\b`).MatchString(output) {
		return fmt.Errorf("product DF refusal has no measured MTU of 1492: %s", output)
	}
	return nil
}
