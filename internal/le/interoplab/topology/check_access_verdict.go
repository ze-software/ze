// Design: docs/architecture/testing/interop.md -- the access chain.
// Related: check_access.go -- the checkers that read this state.
package topology

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"strconv"
	"strings"
)

// accountingRecord is one block of FreeRADIUS's ze_detail file: the fields
// the accounting verdict reads, each parsed once from its "Name = value" line.
type accountingRecord struct {
	status    string
	sessionID string
	framedIP  netip.Addr
	// inputOctets and outputOctets join Acct-Input-Octets with
	// Acct-Input-Gigawords (RFC 2869 Section 5.1), and the output pair the
	// same way; hasOctets says the record carried both octet counters.
	inputOctets  uint64
	outputOctets uint64
	hasOctets    bool
}

// parseAccountingDetail reads the ze_detail file: a timestamp line, then one
// "\tName = value" line per attribute, a blank line ending each record. A
// counter that does not parse is refused rather than read as zero.
func parseAccountingDetail(text string) ([]accountingRecord, error) {
	var records []accountingRecord
	var current accountingRecord
	var inputLow, inputHigh, outputLow, outputHigh uint64
	var hasInput, hasOutput bool
	for line := range strings.SplitSeq(text+"\n\n", "\n") {
		name, value, found := strings.Cut(strings.TrimSpace(line), " = ")
		if !found {
			if current.status != "" {
				current.inputOctets = inputHigh<<32 | inputLow
				current.outputOctets = outputHigh<<32 | outputLow
				current.hasOctets = hasInput && hasOutput
				records = append(records, current)
			}
			current = accountingRecord{}
			inputLow, inputHigh, outputLow, outputHigh = 0, 0, 0, 0
			hasInput, hasOutput = false, false
			continue
		}
		var err error
		switch name {
		case "Acct-Status-Type":
			current.status = value
		case "Acct-Session-Id":
			current.sessionID = strings.Trim(value, "\"")
		case "Framed-IP-Address":
			current.framedIP, err = netip.ParseAddr(value)
		case "Acct-Input-Octets":
			inputLow, err = strconv.ParseUint(value, 10, 32)
			hasInput = err == nil
		case "Acct-Output-Octets":
			outputLow, err = strconv.ParseUint(value, 10, 32)
			hasOutput = err == nil
		case "Acct-Input-Gigawords":
			inputHigh, err = strconv.ParseUint(value, 10, 32)
		case "Acct-Output-Gigawords":
			outputHigh, err = strconv.ParseUint(value, 10, 32)
		}
		if err != nil {
			return nil, fmt.Errorf("ze_detail line %q does not parse: %w", line, err)
		}
	}
	return records, nil
}

// The Acct-Status-Type values FreeRADIUS writes (RFC 2866 Section 5.1, RFC
// 2869 Section 2.1), and the stage a checker waits for.
const (
	statusStart   = "Start"
	statusInterim = "Interim-Update"
	statusStop    = "Stop"
)

// errAccountingIncomplete says the detail file does not hold the record the
// verdict waits for yet, so a checker keeps waiting.
var errAccountingIncomplete = errors.New("the accounting record the check waits for is not in ze_detail yet")

// accountingVerdict judges AC-8 over the parsed records: one Acct-Session-Id
// has a Start, at least one Interim-Update and a Stop, each carrying
// Framed-IP-Address address, and the Stop's input and output octets are each
// at least volume. want names the last status the checker waits for, so a
// file that stops before it is errAccountingIncomplete and anything else
// short is refused.
func accountingVerdict(records []accountingRecord, address netip.Addr, volume uint64, want string) error {
	var start *accountingRecord
	for index := range records {
		if records[index].status == statusStart && records[index].framedIP == address {
			start = &records[index]
			break
		}
	}
	if start == nil {
		return fmt.Errorf("%w: no Accounting Start with Framed-IP-Address %s", errAccountingIncomplete, address)
	}
	if want == statusStart {
		return nil
	}
	var interims int
	var stop *accountingRecord
	for index := range records {
		record := &records[index]
		if record.sessionID != start.sessionID {
			continue
		}
		if record.framedIP != address {
			return fmt.Errorf("Acct-Session-Id %s: a %s record carries Framed-IP-Address %s, want %s",
				start.sessionID, record.status, record.framedIP, address)
		}
		if record.status == statusInterim {
			interims++
		}
		if record.status == statusStop {
			stop = record
		}
	}
	if interims == 0 {
		return fmt.Errorf("%w: Acct-Session-Id %s has no Interim-Update", errAccountingIncomplete, start.sessionID)
	}
	if want == statusInterim {
		return nil
	}
	if stop == nil {
		return fmt.Errorf("%w: Acct-Session-Id %s has no Stop", errAccountingIncomplete, start.sessionID)
	}
	if !stop.hasOctets {
		return fmt.Errorf("Acct-Session-Id %s: the Stop carries no Acct-Input-Octets and Acct-Output-Octets", start.sessionID)
	}
	if stop.inputOctets < volume {
		return fmt.Errorf("Acct-Session-Id %s: the Stop's Acct-Input-Octets %d is below the %d octets the CPE sent",
			start.sessionID, stop.inputOctets, volume)
	}
	if stop.outputOctets < volume {
		return fmt.Errorf("Acct-Session-Id %s: the Stop's Acct-Output-Octets %d is below the %d octets the LNS answered",
			start.sessionID, stop.outputOctets, volume)
	}
	return nil
}

// authLogin is one line FreeRADIUS's ze_auth_log module writes: the fields
// the login verdict compares, each a "key=value" word.
type authLogin struct {
	verdict       string
	user          string
	userPassword  string
	chapPassword  string
	nasIdentifier string
	nasIPAddress  string
	source        string
}

// authLogKeys are the words every ze_auth_log line carries; a line missing
// one is refused, so a truncated or reformatted line is never read as a
// partial login.
var authLogKeys = []string{"verdict", "user", "user-password", "chap-password", "nas-identifier", "nas-ip-address", "source"}

// parseAuthLog reads every ze_auth_log line into an authLogin.
func parseAuthLog(text string) ([]authLogin, error) {
	var logins []authLogin
	for line := range strings.SplitSeq(text, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		fields := map[string]string{}
		for word := range strings.FieldsSeq(line) {
			key, value, found := strings.Cut(word, "=")
			if found {
				fields[key] = value
			}
		}
		for _, key := range authLogKeys {
			if _, present := fields[key]; !present {
				return nil, fmt.Errorf("ze_auth_log line %q has no %s field", line, key)
			}
		}
		logins = append(logins, authLogin{verdict: fields["verdict"], user: fields["user"],
			userPassword: fields["user-password"], chapPassword: fields["chap-password"],
			nasIdentifier: fields["nas-identifier"], nasIPAddress: fields["nas-ip-address"], source: fields["source"]})
	}
	return logins, nil
}

// errNoLogin says FreeRADIUS has not answered the user's login yet.
var errNoLogin = errors.New("FreeRADIUS has logged no login for the user yet")

// loginVerdict judges AC-3 and AC-12: FreeRADIUS accepted user from the LNS
// at nas (its source address and its NAS-IP-Address) naming itself
// identifier, and the request carried a CHAP-Password and no User-Password,
// so the method the LNS negotiated with the CPE is the one the server
// checked.
func loginVerdict(logins []authLogin, user, identifier string, nas netip.Addr) error {
	for index := range logins {
		login := &logins[index]
		if login.user != user {
			continue
		}
		if login.verdict != "accept" {
			return fmt.Errorf("FreeRADIUS answered %s with verdict=%s, want accept", user, login.verdict)
		}
		if login.chapPassword != "present" {
			return fmt.Errorf("the Access-Request for %s carried no CHAP-Password, so FreeRADIUS did not check CHAP", user)
		}
		if login.userPassword != "absent" {
			return fmt.Errorf("the Access-Request for %s carried a User-Password beside its CHAP-Password", user)
		}
		if login.nasIdentifier != identifier {
			return fmt.Errorf("the Access-Request for %s named NAS-Identifier %s, want %s", user, login.nasIdentifier, identifier)
		}
		if login.nasIPAddress != nas.String() {
			return fmt.Errorf("the Access-Request for %s carried NAS-IP-Address %s, want the LNS source-address %s", user, login.nasIPAddress, nas)
		}
		if login.source != nas.String() {
			return fmt.Errorf("the Access-Request for %s came from %s, want the LNS source-address %s", user, login.source, nas)
		}
		return nil
	}
	return fmt.Errorf("%w: %s", errNoLogin, user)
}

// routeViaVerdict judges the route half of AC-5 over `ip -o route get
// <target>`: the route leaves on device, the PPPoE session, and nothing
// else, so a ping that answers crossed the session rather than eth0.
func routeViaVerdict(answer, target, device string) error {
	fields := strings.Fields(answer)
	for index := range fields {
		if fields[index] != ipWordDev {
			continue
		}
		if index+1 == len(fields) {
			break
		}
		if fields[index+1] != device {
			return fmt.Errorf("ze-cpe routes %s via %s, not %s, so a reply would not prove the session: %s",
				target, fields[index+1], device, strings.TrimSpace(answer))
		}
		return nil
	}
	return fmt.Errorf("ze-cpe's route to %s names no device: %q", target, strings.TrimSpace(answer))
}

// pingVerdict judges busybox ping's summary: every one of sent packets was
// received, "N packets transmitted, N packets received, 0% packet loss".
func pingVerdict(output string, sent int) error {
	want := strconv.Itoa(sent) + " packets transmitted, " + strconv.Itoa(sent) + " packets received, 0% packet loss"
	if !strings.Contains(output, want) {
		return fmt.Errorf("ze-cpe's ping did not report %q:\n%s", want, output)
	}
	return nil
}

// pppAddressOn answers the IPv4 address `ip -o -4 address show dev <device>`
// lists for device, and refuses an answer with none or several.
func pppAddressOn(answer, device string) (netip.Addr, error) {
	var found []netip.Addr
	for line := range strings.SplitSeq(answer, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		if fields[1] != device {
			continue
		}
		if fields[2] != "inet" {
			continue
		}
		prefix, _, _ := strings.Cut(fields[3], "/")
		address, err := netip.ParseAddr(prefix)
		if err != nil {
			return netip.Addr{}, fmt.Errorf("%s lists an address that does not parse: %q", device, line)
		}
		found = append(found, address)
	}
	if len(found) != 1 {
		return netip.Addr{}, fmt.Errorf("%s holds %d IPv4 addresses, want one: %q", device, len(found), answer)
	}
	return found[0], nil
}

// deviceWithPeer answers the device `ip -o -4 address show` lists with
// "peer <peer>/32": the LNS's PPP interface toward the subscriber.
func deviceWithPeer(answer string, peer netip.Addr) (string, bool) {
	want := peer.String() + "/32"
	for line := range strings.SplitSeq(answer, "\n") {
		fields := strings.Fields(line)
		for index := 0; index+1 < len(fields); index++ {
			if fields[index] == "peer" && fields[index+1] == want {
				return fields[1], true
			}
		}
	}
	return "", false
}

// pppoeClientStatus is one object of `show interface pppoe-client | json`
// (iface.PPPoEClientStatus): the keys the DNS verdict reads. DNSServers is a
// pointer so an answer that lacks the key is told apart from an empty list,
// which the command always writes when nothing was learned.
type pppoeClientStatus struct {
	Name       string    `json:"name"`
	State      string    `json:"state"`
	DNSServers *[]string `json:"dns-servers"`
}

// pppoeClientStateUp is the state `show interface pppoe-client` reports once
// IPCP is Opened. The client records it after it handed the learned servers
// to resolv.conf's writer, so resolv.conf read after it already reflects them.
const pppoeClientStateUp = "up"

// errDNSNotLearned says the PPPoE client has not reported its session up
// with the servers IPCP offered: a state the wait passes over, unlike a
// refusal of what it did report.
var errDNSNotLearned = errors.New("the PPPoE client has not reported its session up with the IPCP DNS servers")

// parsePPPoEClients reads the JSON array `show interface pppoe-client | json`
// answers, one object per configured client.
func parsePPPoEClients(answer string) ([]pppoeClientStatus, error) {
	var clients []pppoeClientStatus
	if err := json.Unmarshal([]byte(strings.TrimSpace(answer)), &clients); err != nil {
		return nil, fmt.Errorf("show interface pppoe-client did not answer a JSON array: %w: %q", err, answer)
	}
	return clients, nil
}

// learnedDNSVerdict judges RFC 1877 on the client's own report: the client
// called name is up and lists want, in order, as the servers IPCP learned
// (child 13 AC-11). A client absent, not up, or up with no servers yet
// answers errDNSNotLearned; up with other servers is refused outright.
func learnedDNSVerdict(clients []pppoeClientStatus, name string, want []string) error {
	for _, client := range clients {
		if client.Name != name {
			continue
		}
		if client.State != pppoeClientStateUp {
			return fmt.Errorf("%w: %s reports state %q", errDNSNotLearned, name, client.State)
		}
		if client.DNSServers == nil {
			return fmt.Errorf("%s's answer carries no dns-servers key, which the command always writes", name)
		}
		if len(*client.DNSServers) == 0 {
			return fmt.Errorf("%w: %s is up with dns-servers [], so IPCP learned none", errDNSNotLearned, name)
		}
		if !slices.Equal(*client.DNSServers, want) {
			return fmt.Errorf("%s learned dns-servers %v through IPCP, want %v, primary first", name, *client.DNSServers, want)
		}
		return nil
	}
	return fmt.Errorf("%w: show interface pppoe-client lists no client %s", errDNSNotLearned, name)
}

// resolvConfVerdict judges the precedence rule (RFC 3646 Section 6, which
// Ze applies to every learned source): with static name servers configured,
// resolv.conf lists exactly those, in order, and no learned server.
func resolvConfVerdict(text string, static []string) error {
	var want strings.Builder
	want.WriteString("# Generated by ze\n")
	for _, server := range static {
		want.WriteString("nameserver ")
		want.WriteString(server)
		want.WriteByte('\n')
	}
	if text != want.String() {
		return fmt.Errorf("resolv.conf reads %q, want %q: the static system name-server alone, with no server learned through IPCP", text, want.String())
	}
	return nil
}
