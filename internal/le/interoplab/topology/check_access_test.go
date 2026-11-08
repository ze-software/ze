package topology

import (
	"encoding/binary"
	"errors"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// pcapOf builds a little-endian classic pcap file holding frames.
func pcapOf(frames ...[]byte) []byte {
	capture := make([]byte, pcapHeaderOctets)
	copy(capture, []byte{0xd4, 0xc3, 0xb2, 0xa1})
	for _, frame := range frames {
		record := make([]byte, pcapRecordOctets)
		binary.LittleEndian.PutUint32(record[8:12], uint32(len(frame)))
		binary.LittleEndian.PutUint32(record[12:16], uint32(len(frame)))
		capture = append(capture, record...)
		capture = append(capture, frame...)
	}
	return capture
}

// discoveryFrame builds one untagged PPPoE discovery frame.
func discoveryFrame(code byte, session uint16, tags ...[]byte) []byte {
	frame := make([]byte, ethernetOctets+pppoeHeaderOctets)
	binary.BigEndian.PutUint16(frame[12:14], etherTypeDiscovery)
	frame[14] = 0x11
	frame[15] = code
	binary.BigEndian.PutUint16(frame[16:18], session)
	var length int
	for _, tag := range tags {
		frame = append(frame, tag...)
		length += len(tag)
	}
	binary.BigEndian.PutUint16(frame[18:20], uint16(length))
	return frame
}

// pppoeTag builds one PPPoE tag.
func pppoeTag(kind uint16, value string) []byte {
	tag := make([]byte, pppoeTagOctets, pppoeTagOctets+len(value))
	binary.BigEndian.PutUint16(tag[0:2], kind)
	binary.BigEndian.PutUint16(tag[2:4], uint16(len(value)))
	return append(tag, value...)
}

// framesOf parses the pcap built from frames, failing the test on a parse
// error, so every verdict case also crosses pcapFrames.
func framesOf(t *testing.T, frames ...[]byte) [][]byte {
	t.Helper()
	parsed, err := pcapFrames(pcapOf(frames...))
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

// VALIDATES: AC-1 of spec-home-cpe-19, the verdict half. discoveryVerdict
// passes a capture holding a PADO with a zero-length Service-Name tag and a
// PADS with a non-zero session id; it refuses a PADO naming a service, a PADO
// with no Service-Name tag and a PADS with session 0, and reports a capture
// with no PADO or no PADS as errCaptureEmpty so the checker keeps waiting.
// Method: pcap files built from discovery frames, one per case; the red cases
// are the breaks the spec's red walk names (the relay not answering the empty
// Service-Name), seen at the verdict.
// PREVENTS: a checker that passes a discovery the relay never answered.
func TestAccessChainPADOVerdict(t *testing.T) {
	acName := pppoeTag(0x0102, "ze-lac")
	empty := pppoeTag(pppoeTagServiceName, "")
	named := pppoeTag(pppoeTagServiceName, "internet")
	padi := discoveryFrame(0x09, 0, empty)
	pado := discoveryFrame(pppoeCodePADO, 0, acName, empty)
	pads := discoveryFrame(pppoeCodePADS, 0x0042, empty)
	tests := []struct {
		name    string
		frames  [][]byte
		wantErr string
		waiting bool
	}{
		{"empty Service-Name and PADS pass", framesOf(t, padi, pado, pads), "", false},
		{"named Service-Name refused", framesOf(t, padi, discoveryFrame(pppoeCodePADO, 0, acName, named), pads), "Service-Name of 8 octets", false},
		{"no Service-Name tag refused", framesOf(t, discoveryFrame(pppoeCodePADO, 0, acName), pads), "no Service-Name tag", false},
		{"PADS session 0 refused", framesOf(t, pado, discoveryFrame(pppoeCodePADS, 0, empty)), "SESSION_ID 0", false},
		{"no PADO waits", framesOf(t, padi), "no PADO", true},
		{"no PADS waits", framesOf(t, padi, pado), "no PADS", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := discoveryVerdict(tt.frames)
			checkVerdict(t, err, tt.wantErr, errors.Is(err, errCaptureEmpty), tt.waiting)
		})
	}
}

// l2tpLCPFrame builds an IPv4 UDP datagram from source to port 1701 carrying
// an L2TPv2 data message (L bit set) whose PPP frame is an LCP
// Configure-Request with options.
func l2tpLCPFrame(source string, options ...byte) []byte {
	lcp := append([]byte{lcpConfigureRequest, 1, 0, 0}, options...)
	binary.BigEndian.PutUint16(lcp[2:4], uint16(len(lcp)))
	ppp := append([]byte{0xff, 0x03, 0xc0, 0x21}, lcp...)
	l2tp := append([]byte{0x40, 0x02, 0, 0, 0, 7, 0, 9}, ppp...)
	binary.BigEndian.PutUint16(l2tp[2:4], uint16(len(l2tp)))
	udp := append([]byte{0x06, 0xa5, 0x06, 0xa5, 0, 0, 0, 0}, l2tp...)
	binary.BigEndian.PutUint16(udp[4:6], uint16(len(udp)))
	ip := make([]byte, 20)
	ip[0] = 0x45
	ip[9] = ipProtocolUDP
	from := netip.MustParseAddr(source).As4()
	copy(ip[12:16], from[:])
	frame := make([]byte, ethernetOctets)
	binary.BigEndian.PutUint16(frame[12:14], etherTypeIPv4)
	frame = append(frame, ip...)
	return append(frame, udp...)
}

// VALIDATES: AC-11 of spec-home-cpe-19, the core-link half. lcpMRUVerdict
// passes the LNS's LCP Configure-Request carrying MRU 1492 beside the CHAP
// Authentication-Protocol option; it refuses MRU 1500 and a request with no
// MRU option, ignores the LAC's request (another source), and reports a
// capture with no request from the LNS as errCaptureEmpty.
// Method: pcap files of L2TP data frames built by l2tpLCPFrame.
// PREVENTS: a checker that reads the configured MRU as the negotiated one,
// or the CPE's request as the LNS's.
func TestAccessChainLCPMRUVerdict(t *testing.T) {
	lns := netip.MustParseAddr(lnsCoreAddress)
	chap := []byte{3, 5, 0xc2, 0x23, 5}
	mru1492 := append([]byte{lcpOptionMRU, 4, 0x05, 0xd4}, chap...)
	mru1500 := append([]byte{lcpOptionMRU, 4, 0x05, 0xdc}, chap...)
	tests := []struct {
		name    string
		frames  [][]byte
		wantErr string
		waiting bool
	}{
		{"MRU 1492 passes", framesOf(t, l2tpLCPFrame("192.0.2.2", mru1500...), l2tpLCPFrame(lnsCoreAddress, mru1492...)), "", false},
		{"MRU 1500 refused", framesOf(t, l2tpLCPFrame(lnsCoreAddress, mru1500...)), "MRU 1500, want 1492", false},
		{"no MRU option refused", framesOf(t, l2tpLCPFrame(lnsCoreAddress, chap...)), "no MRU option", false},
		{"another source waits", framesOf(t, l2tpLCPFrame("192.0.2.2", mru1492...)), "no LCP Configure-Request", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := lcpMRUVerdict(tt.frames, lns, sessionMRU)
			checkVerdict(t, err, tt.wantErr, errors.Is(err, errCaptureEmpty), tt.waiting)
		})
	}
}

// VALIDATES: AC-5 of spec-home-cpe-19, the route half. routeViaVerdict
// passes `ip -o route get` output whose route leaves on pppoe0, and refuses
// one leaving on eth0 and one naming no device.
// Method: ip route get lines as fixtures.
// PREVENTS: a ping proving nothing because it left over the lab bridge
// (vacuity trap 4).
func TestAccessChainRouteVerdict(t *testing.T) {
	tests := []struct {
		name    string
		answer  string
		wantErr string
	}{
		{"via pppoe0 passes", "100.64.0.1 dev pppoe0 src 203.0.113.1 uid 0 \\    cache \n", ""},
		{"via eth0 refused", "100.64.0.1 via 172.31.181.1 dev eth0 src 172.31.181.5 uid 0 \\    cache \n", "via eth0, not pppoe0"},
		{"no device refused", "RTNETLINK answers: Network is unreachable\n", "names no device"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checkVerdict(t, routeViaVerdict(tt.answer, poolGateway, cpeDevice), tt.wantErr, false, false)
		})
	}
}

// accountingFixture is a ze_detail file holding one session's records.
func accountingFixture(blocks ...string) string {
	return strings.Join(blocks, "\n")
}

func detailRecord(status, address, input, output string) string {
	var record strings.Builder
	record.WriteString("Sat Nov  7 12:00:00 2026\n")
	record.WriteString("\tAcct-Status-Type = " + status + "\n")
	record.WriteString("\tAcct-Session-Id = \"7-9-0123456789abcdef01234567-1\"\n")
	record.WriteString("\tFramed-IP-Address = " + address + "\n")
	if input != "" {
		record.WriteString("\tAcct-Input-Octets = " + input + "\n")
		record.WriteString("\tAcct-Output-Octets = " + output + "\n")
	}
	return record.String() + "\n"
}

// VALIDATES: AC-8 of spec-home-cpe-19, the verdict half. parseAccountingDetail
// and accountingVerdict pass a session with Start, Interim-Update and Stop,
// each carrying 203.0.113.1, whose Stop counts at least accessVolume octets
// each way; they refuse a Stop whose octets are zero or below the volume, a
// Stop with no octet counters, a record of the session with another address,
// and a counter that does not parse; a file missing the Interim-Update or
// the Stop is errAccountingIncomplete while the checker waits for that stage.
// Method: ze_detail text in FreeRADIUS's detail format as fixtures; the red
// cases are the breaks the spec's red walk names (no interim, octets zero).
// PREVENTS: a checker that passes accounting with no interim, or with
// counters that never moved (vacuity trap 3).
func TestAccessChainDetailVerdict(t *testing.T) {
	address := netip.MustParseAddr(subscriberAddress)
	start := detailRecord(statusStart, subscriberAddress, "", "")
	interim := detailRecord(statusInterim, subscriberAddress, "21000", "21000")
	stop := detailRecord(statusStop, subscriberAddress, "21500", "21400")
	tests := []struct {
		name    string
		text    string
		want    string
		wantErr string
		waiting bool
	}{
		{"start interim stop pass", accountingFixture(start, interim, stop), statusStop, "", false},
		{"start alone passes the start stage", accountingFixture(start), statusStart, "", false},
		{"interim passes the interim stage", accountingFixture(start, interim), statusInterim, "", false},
		{"no interim waits", accountingFixture(start, stop), statusStop, "no Interim-Update", true},
		{"no stop waits", accountingFixture(start, interim), statusStop, "no Stop", true},
		{"no start waits", "", statusStart, "no Accounting Start", true},
		{"zero octets refused", accountingFixture(start, interim, detailRecord(statusStop, subscriberAddress, "0", "0")), statusStop, "Acct-Input-Octets 0 is below", false},
		{"short output refused", accountingFixture(start, interim, detailRecord(statusStop, subscriberAddress, "21500", "100")), statusStop, "Acct-Output-Octets 100 is below", false},
		{"no counters refused", accountingFixture(start, interim, detailRecord(statusStop, subscriberAddress, "", "")), statusStop, "carries no Acct-Input-Octets", false},
		{"other address refused", accountingFixture(start, detailRecord(statusInterim, "100.64.0.2", "1", "1")), statusStop, "carries Framed-IP-Address 100.64.0.2", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			records, err := parseAccountingDetail(tt.text)
			if err != nil {
				t.Fatal(err)
			}
			err = accountingVerdict(records, address, accessVolume, tt.want)
			checkVerdict(t, err, tt.wantErr, errors.Is(err, errAccountingIncomplete), tt.waiting)
		})
	}
	if _, err := parseAccountingDetail(detailRecord(statusStop, subscriberAddress, "lots", "1")); err == nil {
		t.Error("a counter that does not parse was read as a value")
	}
}

// VALIDATES: AC-3 and AC-12 of spec-home-cpe-19, the verdict half.
// loginVerdict passes an accepted CHAP login from 192.0.2.3 naming ze-lns,
// and refuses a reject, a request with no CHAP-Password, one with a
// User-Password, another NAS-Identifier, another NAS-IP-Address and another
// source; parseAuthLog refuses a line missing a field; no line for the user
// is errNoLogin.
// Method: ze_auth_log lines as fixtures.
// PREVENTS: a checker that passes a login RADIUS never checked as CHAP.
func TestAccessChainLoginVerdict(t *testing.T) {
	nas := netip.MustParseAddr(lnsCoreAddress)
	// otherUser is a login for a user the scenario does not dial, which the
	// verdict for subscriberUser skips.
	const otherUser = "other@lab.example.net"
	line := func(verdict, user, chap, password, identifier, address, source string) string {
		return "verdict=" + verdict + " user=" + user + " user-password=" + password + " chap-password=" + chap +
			" nas-identifier=" + identifier + " nas-ip-address=" + address + " source=" + source + "\n"
	}
	good := line("accept", subscriberUser, "present", "absent", lnsNASIdentifier, lnsCoreAddress, lnsCoreAddress)
	tests := []struct {
		name    string
		text    string
		wantErr string
		waiting bool
	}{
		{"accepted CHAP passes", line("accept", otherUser, "present", "absent", "x", "x", "x") + good, "", false},
		{"reject refused", line("reject", subscriberUser, "present", "absent", lnsNASIdentifier, lnsCoreAddress, lnsCoreAddress), "verdict=reject", false},
		{"no CHAP refused", line("accept", subscriberUser, "absent", "present", lnsNASIdentifier, lnsCoreAddress, lnsCoreAddress), "no CHAP-Password", false},
		{"User-Password refused", line("accept", subscriberUser, "present", "present", lnsNASIdentifier, lnsCoreAddress, lnsCoreAddress), "User-Password beside", false},
		{"identifier refused", line("accept", subscriberUser, "present", "absent", "lns", lnsCoreAddress, lnsCoreAddress), "NAS-Identifier lns", false},
		{"NAS-IP refused", line("accept", subscriberUser, "present", "absent", lnsNASIdentifier, "none", lnsCoreAddress), "NAS-IP-Address none", false},
		{"source refused", line("accept", subscriberUser, "present", "absent", lnsNASIdentifier, lnsCoreAddress, "192.0.2.9"), "came from 192.0.2.9", false},
		{"no line waits", line("accept", otherUser, "present", "absent", "x", "x", "x"), "no login", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logins, err := parseAuthLog(tt.text)
			if err != nil {
				t.Fatal(err)
			}
			err = loginVerdict(logins, subscriberUser, lnsNASIdentifier, nas)
			checkVerdict(t, err, tt.wantErr, errors.Is(err, errNoLogin), tt.waiting)
		})
	}
	if _, err := parseAuthLog("verdict=accept user=" + subscriberUser + "\n"); err == nil {
		t.Error("a truncated ze_auth_log line was read as a login")
	}
}

// VALIDATES: the address, peer-device and ping readers of spec-home-cpe-19
// AC-4, AC-5 and AC-7. pppAddressOn answers the one address on pppoe0 and
// refuses none; deviceWithPeer finds the LNS device toward the subscriber;
// pingVerdict refuses a lossy run.
// Method: ip -o -4 address, busybox ping and iputils ping output as fixtures.
// PREVENTS: an address read from another device, and a lossy ping passing.
func TestAccessChainStateReaders(t *testing.T) {
	cpe := "5: pppoe0    inet 203.0.113.1 peer 100.64.0.1/32 scope global pppoe0\\       valid_lft forever\n"
	if address, err := pppAddressOn(cpe, cpeDevice); err != nil || address != netip.MustParseAddr(subscriberAddress) {
		t.Errorf("pppAddressOn = %v, %v; want %s", address, err, subscriberAddress)
	}
	if _, err := pppAddressOn("2: eth1    inet 192.0.2.5/24 scope global eth1\n", cpeDevice); err == nil {
		t.Error("pppAddressOn answered an address pppoe0 does not hold")
	}
	lns := "1: lo    inet 127.0.0.1/8 scope host lo\n7: ppp0    inet 100.64.0.1 peer 203.0.113.1/32 scope global ppp0\n"
	if device, found := deviceWithPeer(lns, netip.MustParseAddr(subscriberAddress)); !found || device != "ppp0" {
		t.Errorf("deviceWithPeer = %q, %v; want ppp0", device, found)
	}
	if _, found := deviceWithPeer(lns, netip.MustParseAddr("203.0.113.9")); found {
		t.Error("deviceWithPeer found a device toward a peer nobody holds")
	}
	if err := pingVerdict("20 packets transmitted, 20 packets received, 0% packet loss\n", pingCount); err != nil {
		t.Error(err)
	}
	if err := pingVerdict("20 packets transmitted, 19 packets received, 5% packet loss\n", pingCount); err == nil {
		t.Error("pingVerdict passed a lossy ping")
	}
	// iputils, the pppd subscriber's pinger, words the summary without "packets".
	if err := pingVerdict("20 packets transmitted, 20 received, 0% packet loss, time 19435ms\n", pingCount); err != nil {
		t.Error(err)
	}
	if err := pingVerdict("20 packets transmitted, 19 received, 5% packet loss, time 19435ms\n", pingCount); err == nil {
		t.Error("pingVerdict passed a lossy iputils ping")
	}
}

// VALIDATES: every file an access-chain node mounts from the checkout
// exists, so a renamed lab file fails here rather than at docker run.
// Method: walk each declared node's shared files under the checkout.
// PREVENTS: a scenario that cannot start because a mount source is gone.
func TestAccessChainSharedFilesExist(t *testing.T) {
	root := filepath.Join("..", "..", "..", "..")
	for _, name := range []string{accessUpScenario, accessPoolScenario, accessAccountingScenario} {
		for _, declared := range scenarios[name].nodes {
			for _, file := range declared.shared {
				if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(file.source))); err != nil {
					t.Errorf("%s node %s mounts %s: %v", name, declared.name, file.source, err)
				}
			}
		}
	}
}

// checkVerdict asserts err is nil when wantErr is empty, and otherwise holds
// wantErr, with waiting saying whether it is the wait-again sentinel.
func checkVerdict(t *testing.T, err error, wantErr string, isWaiting, waiting bool) {
	t.Helper()
	if wantErr == "" {
		if err != nil {
			t.Fatalf("expected pass, got %v", err)
		}
		return
	}
	if err == nil {
		t.Fatalf("expected a refusal containing %q, got nil", wantErr)
	}
	if !strings.Contains(err.Error(), wantErr) {
		t.Errorf("error %q does not contain %q", err, wantErr)
	}
	if isWaiting != waiting {
		t.Errorf("wait-again sentinel = %v, want %v (%v)", isWaiting, waiting, err)
	}
}

// VALIDATES: the two DNS clauses of spec-home-cpe-19 AC-6 (child 13 owns
// them). learnedDNSVerdict passes the CPE's pppoe0 up with 192.0.2.53 then
// 192.0.2.54, and resolvConfVerdict passes the static server alone.
// Method: `show interface pppoe-client | json` answers and resolv.conf texts
// as fixtures, each break being the state the broken product would leave.
// PREVENTS: the clause passing when the client never asked IPCP for DNS
// (the client comes up with dns-servers []), and passing when static
// `system name-server` stops winning (resolv.conf merges the learned servers).
func TestAccessChainDNSVerdicts(t *testing.T) {
	want := []string{dnsPrimary, dnsSecondary}
	learned := `[{"name":"pppoe0","state":"up","source-interface":"eth1","dns-servers":["192.0.2.53","192.0.2.54"]}]`
	clients, err := parsePPPoEClients(learned + "\n")
	if err != nil {
		t.Fatalf("parse a well-formed answer: %v", err)
	}
	if err := learnedDNSVerdict(clients, cpeDevice, want); err != nil {
		t.Fatalf("the learned servers were refused: %v", err)
	}
	for _, testCase := range []struct {
		name    string
		answer  string
		waiting bool
	}{
		// Break: the IPCP Configure-Request no longer carries options 129
		// and 131, so the session comes up having learned nothing.
		{"IPCP DNS request removed", `[{"name":"pppoe0","state":"up","dns-servers":[]}]`, true},
		{"session not up", `[{"name":"pppoe0","state":"ncp","dns-servers":[]}]`, true},
		{"client absent", `[{"name":"pppoe1","state":"up","dns-servers":["192.0.2.53","192.0.2.54"]}]`, true},
		{"no clients", `[]`, true},
		{"primary and secondary swapped", `[{"name":"pppoe0","state":"up","dns-servers":["192.0.2.54","192.0.2.53"]}]`, false},
		{"secondary dropped", `[{"name":"pppoe0","state":"up","dns-servers":["192.0.2.53"]}]`, false},
		{"dns-servers key absent", `[{"name":"pppoe0","state":"up"}]`, false},
	} {
		clients, err := parsePPPoEClients(testCase.answer)
		if err != nil {
			t.Fatalf("%s: parse: %v", testCase.name, err)
		}
		err = learnedDNSVerdict(clients, cpeDevice, want)
		if err == nil {
			t.Errorf("%s: learnedDNSVerdict passed it", testCase.name)
			continue
		}
		if errors.Is(err, errDNSNotLearned) != testCase.waiting {
			t.Errorf("%s: wait-again sentinel = %v, want %v (%v)", testCase.name, !testCase.waiting, testCase.waiting, err)
		}
	}
	if _, err := parsePPPoEClients("error: unknown command\n"); err == nil {
		t.Error("parsePPPoEClients read a refusal as a client list")
	}

	static := []string{cpeStaticNameServer}
	if err := resolvConfVerdict("# Generated by ze\nnameserver 192.0.2.53\n", static); err != nil {
		t.Errorf("the static server alone was refused: %v", err)
	}
	for name, text := range map[string]string{
		// Break: static stops winning, so the learned servers join it.
		"learned servers merged":  "# Generated by ze\nnameserver 192.0.2.53\nnameserver 192.0.2.54\n",
		"learned before static":   "# Generated by ze\nnameserver 192.0.2.54\nnameserver 192.0.2.53\n",
		"learned secondary alone": "# Generated by ze\nnameserver 192.0.2.54\n",
		"not Ze's file":           "nameserver 192.0.2.53\n",
		"empty":                   "",
	} {
		if resolvConfVerdict(text, static) == nil {
			t.Errorf("%s: resolvConfVerdict passed %q", name, text)
		}
	}
	// Break: with a static server that is not the LNS's primary, the
	// learned servers in its place are refused too.
	if resolvConfVerdict("# Generated by ze\nnameserver 192.0.2.53\nnameserver 192.0.2.54\n", []string{"198.51.100.53"}) == nil {
		t.Error("resolvConfVerdict passed the learned servers in place of the static one")
	}
}

// VALIDATES: the CPE's CLI login reads the committed lab-values password the
// rendered ze-cpe.conf gives user admin, and refuses a run with no checkout.
// Method: the committed file, then an empty root.
// PREVENTS: a login whose password drifted from the one the CPE was given.
func TestAccessChainCPECLILogin(t *testing.T) {
	login, err := cpeCLILogin(checkoutRoot(t))
	if err != nil {
		t.Fatalf("cpeCLILogin over the checkout: %v", err)
	}
	if login.User != cpeCLIUser || login.Port != cpeCLIPort || login.Store != cpeCLIStore || login.Password == "" {
		t.Errorf("cpeCLILogin = %+v; want user %s port %s store %s and a password", login, cpeCLIUser, cpeCLIPort, cpeCLIStore)
	}
	// The loopback is not admitted by the CPE's input ruleset (rules 20-a and
	// 20-b take SSH from the LAN prefixes only), so the login dials the LAN
	// gateway, whose own address is the session's source.
	if login.Host.String() != cpeLANGateway {
		t.Errorf("cpeCLILogin dials %s, want the CPE's LAN gateway %s", login.Host, cpeLANGateway)
	}
	template, err := os.ReadFile(filepath.Join(checkoutRoot(t), filepath.FromSlash(cpeDirectory), cpeConvertedFile))
	if err != nil {
		t.Fatalf("read the converted CPE configuration: %v", err)
	}
	if !strings.Contains(string(template), "<<SECRET:"+cpeCLIPasswordValue+">>") {
		t.Errorf("%s gives no field the placeholder %s, so the login's password is not the CPE's", cpeConvertedFile, cpeCLIPasswordValue)
	}
	if _, err := cpeCLILogin(""); err == nil {
		t.Error("cpeCLILogin answered with no checkout")
	}
}
