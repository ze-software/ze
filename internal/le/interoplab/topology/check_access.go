// Design: docs/architecture/testing/interop.md -- the access chain.
// Related: register_access_chain.go -- the declaration these check.
// Related: check_access_verdict.go -- the parsers and verdicts they apply.
// Related: check_access_capture.go -- the capture parsers they apply.
package topology

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// The ping the access chain sends across the session: pingCount echo
// requests of pingPayload octets, each one IPv4 datagram of pingDatagram
// octets (a 20-octet IPv4 header and an 8-octet ICMP header). accessVolume is what the CPE sends,
// and what the LNS answers, in octets: mid-range, so a counter reported as
// zero or as a small constant falls below it (vacuity trap 3).
const (
	pingCount    = 20
	pingPayload  = 1000
	pingDatagram = pingPayload + 28
	accessVolume = uint64(pingCount * pingDatagram)
)

// The argv words the access checkers repeat across their state reads.
const (
	ipWordAddress = "address"
	ipWordRoute   = "route"
	ipWordDev     = "dev"
	commandCat    = "cat"
	commandPing   = "ping"
)

// sessionUpTimeout bounds discovery, the L2TP dial, LCP, CHAP through
// RADIUS and IPCP, end to end.
const sessionUpTimeout = 120 * time.Second

// checkAccessUp proves access-up on state, each clause from a different
// producer (vacuity trap 5): the CPE's address and resolver, the LAC's
// discovery capture and L2TP tables, FreeRADIUS's login record, the LNS's
// session table and core capture, and the LNS's PPP interface counters.
func checkAccessUp(ctx context.Context, check *interoplab.CheckContext) error {
	subscriber := netip.MustParseAddr(subscriberAddress)
	// AC-4: 203.0.113.1 lies outside the pool, so only the Access-Accept's
	// Framed-IP-Address can have put it on the CPE.
	if _, err := waitCPEAddress(ctx, check, "Framed-IP-Address "+subscriberAddress,
		func(address netip.Addr) bool { return address == subscriber }); err != nil {
		return err
	}
	if err := checkDiscovery(ctx, check); err != nil {
		return err
	}
	if err := checkLACRelays(ctx, check); err != nil {
		return err
	}
	if err := waitLogin(ctx, check, subscriberUser); err != nil {
		return err
	}
	session, err := lnsSessionOf(ctx, check, subscriberUser, subscriber)
	if err != nil {
		return err
	}
	if err := checkNegotiated(ctx, check, session); err != nil {
		return err
	}
	if err := waitResolvers(ctx, check); err != nil {
		return err
	}
	return checkPingAcross(ctx, check, nodeLNS, subscriber, pingCount)
}

// checkAccessPool proves access-pool: the subscriber's Access-Accept carries
// no Framed-IP-Address (access-pool/users), so the CPE's address comes from
// the LNS pool, and the pool shows it allocated to that session.
func checkAccessPool(ctx context.Context, check *interoplab.CheckContext) error {
	pool := netip.MustParsePrefix(poolNetwork)
	gateway := netip.MustParseAddr(poolGateway)
	address, err := waitCPEAddress(ctx, check, "an address from "+poolNetwork,
		func(address netip.Addr) bool { return pool.Contains(address) && address != gateway })
	if err != nil {
		return err
	}
	if err := waitLogin(ctx, check, subscriberUser); err != nil {
		return err
	}
	if _, err := lnsSessionOf(ctx, check, subscriberUser, address); err != nil {
		return err
	}
	var allocation struct {
		Allocated uint32 `json:"allocated"`
		Sessions  []struct {
			Address string `json:"address"`
		} `json:"sessions"`
	}
	if err := restExecute(ctx, check, nodeLNS, "show l2tp pool", &allocation); err != nil {
		return err
	}
	if allocation.Allocated != 1 {
		return fmt.Errorf("ze-lns's pool reports %d addresses allocated, want 1 (the CPE's %s)", allocation.Allocated, address)
	}
	for _, entry := range allocation.Sessions {
		if entry.Address == address.String() {
			return nil
		}
	}
	return fmt.Errorf("ze-lns's pool lists no session holding %s, the address the CPE holds", address)
}

// checkAccessAccounting proves access-accounting: FreeRADIUS's ze_detail
// holds the session's Start, then an Interim-Update at the pinned 60-second
// cadence, then, once the CPE is stopped (its PPPoE client sends PADT, the
// LAC ends the call, the LNS ends the session), the Stop with the octets the
// ping carried.
func checkAccessAccounting(ctx context.Context, check *interoplab.CheckContext) error {
	subscriber := netip.MustParseAddr(subscriberAddress)
	if _, err := waitCPEAddress(ctx, check, "Framed-IP-Address "+subscriberAddress,
		func(address netip.Addr) bool { return address == subscriber }); err != nil {
		return err
	}
	if err := waitAccounting(ctx, check, statusStart, 30*time.Second); err != nil {
		return err
	}
	if err := checkPingAcross(ctx, check, nodeLNS, subscriber, pingCount, "-s", strconv.Itoa(pingPayload)); err != nil {
		return err
	}
	// RFC 2869 Section 5.16: "The value MUST NOT be smaller than 60." The
	// first interim is due 60 seconds after the Start.
	if err := waitAccounting(ctx, check, statusInterim, 100*time.Second); err != nil {
		return err
	}
	if err := check.Lab.Stop(ctx, nodeCPE, 15); err != nil {
		return fmt.Errorf("stop ze-cpe so its PPPoE client sends PADT: %w", err)
	}
	return waitAccounting(ctx, check, statusStop, 60*time.Second)
}

// waitCPEAddress waits until the CPE's PPPoE device holds one IPv4 address
// accept takes. The kernel holds it only once IPCP is Opened.
func waitCPEAddress(ctx context.Context, check *interoplab.CheckContext, want string, accept func(netip.Addr) bool) (netip.Addr, error) {
	var address netip.Addr
	answer, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: sessionUpTimeout, Interval: 2 * time.Second,
		Description: "ze-cpe " + cpeDevice + " holding " + want,
	}, func(probe context.Context) (string, error) {
		return check.Lab.Query(probe, nodeCPE, []string{"ip", "-o", "-4", ipWordAddress, "show", ipWordDev, cpeDevice}, nil)
	}, func(answer string) bool {
		held, err := pppAddressOn(answer, cpeDevice)
		if err != nil {
			return false
		}
		address = held
		return accept(held)
	})
	if err != nil {
		return netip.Addr{}, fmt.Errorf("ze-cpe's %s never held %s (last answer %q): %w", cpeDevice, want, answer, err)
	}
	return address, nil
}

// checkDiscovery judges AC-1 over ze-lac's access capture.
func checkDiscovery(ctx context.Context, check *interoplab.CheckContext) error {
	var verdict error
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: 10 * time.Second, Interval: time.Second, Description: "ze-lac's PADO and PADS in its access capture",
	}, func(probe context.Context) ([][]byte, error) {
		return readCapture(probe, check, nodeLAC, accessCapture)
	}, func(frames [][]byte) bool {
		verdict = discoveryVerdict(frames)
		return !errors.Is(verdict, errCaptureEmpty)
	})
	if err != nil {
		return waitFailure("ze-lac's access capture never held the discovery", verdict, err)
	}
	return verdict
}

// checkLACRelays judges AC-2 on the LAC: one tunnel to the LNS carrying one
// session, the kernel PPPoE session on the access device, and no PPP device
// with an address, because a relaying LAC terminates nothing.
func checkLACRelays(ctx context.Context, check *interoplab.CheckContext) error {
	var tunnels []struct {
		Peer         string `json:"peer"`
		SessionCount int    `json:"session-count"`
	}
	if err := restExecute(ctx, check, nodeLAC, "show l2tp tunnels", &tunnels); err != nil {
		return err
	}
	if len(tunnels) != 1 {
		return fmt.Errorf("ze-lac holds %d L2TP tunnels, want 1 to ze-lns", len(tunnels))
	}
	// `show l2tp tunnels` reports the peer as the address and UDP port the
	// tunnel's control channel reaches (l2tp TunnelSnapshot.PeerAddr, a
	// netip.AddrPort), so the LNS is its core address on the L2TP port.
	want := netip.AddrPortFrom(netip.MustParseAddr(lnsCoreAddress), l2tpPort)
	peer, err := netip.ParseAddrPort(tunnels[0].Peer)
	if err != nil {
		return fmt.Errorf("ze-lac's tunnel peer %q is not an address and port: %w", tunnels[0].Peer, err)
	}
	if peer != want {
		return fmt.Errorf("ze-lac's tunnel goes to %s, want ze-lns at %s", peer, want)
	}
	if tunnels[0].SessionCount != 1 {
		return fmt.Errorf("ze-lac's tunnel to ze-lns carries %d sessions, want 1", tunnels[0].SessionCount)
	}
	access, err := check.Member(nodeLAC, accessLink)
	if err != nil {
		return err
	}
	table, err := check.Lab.Query(ctx, nodeLAC, []string{commandCat, "/proc/net/pppoe"}, nil)
	if err != nil {
		return fmt.Errorf("read ze-lac's PPPoE session table: %w", err)
	}
	if !sessionOnDevice(table, access.Device) {
		return fmt.Errorf("ze-lac's /proc/net/pppoe holds no session on %s, the access link:\n%s", access.Device, table)
	}
	addresses, err := check.Lab.Query(ctx, nodeLAC, []string{"ip", "-o", "-4", ipWordAddress, "show"}, nil)
	if err != nil {
		return fmt.Errorf("read ze-lac's addresses: %w", err)
	}
	if pppAddressFromAny(addresses) {
		return fmt.Errorf("ze-lac holds an address on a PPP device, so it terminated the session instead of relaying it:\n%s", addresses)
	}
	return nil
}

// pppAddressFromAny reports whether `ip -o -4 address show` lists any
// address on a ppp device.
func pppAddressFromAny(answer string) bool {
	for line := range strings.SplitSeq(answer, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && strings.HasPrefix(fields[1], "ppp") {
			return true
		}
	}
	return false
}

// waitLogin judges AC-3 and AC-12 from FreeRADIUS's own login record.
func waitLogin(ctx context.Context, check *interoplab.CheckContext, user string) error {
	nas := netip.MustParseAddr(lnsCoreAddress)
	var verdict error
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: 20 * time.Second, Interval: time.Second, Description: "FreeRADIUS's login record for " + user,
	}, func(probe context.Context) ([]authLogin, error) {
		text, err := check.Lab.Query(probe, nodeRADIUS, []string{commandCat, authLogPath}, nil)
		if err != nil {
			return nil, err
		}
		return parseAuthLog(text)
	}, func(logins []authLogin) bool {
		verdict = loginVerdict(logins, user, lnsNASIdentifier, nas)
		return !errors.Is(verdict, errNoLogin)
	})
	if err != nil {
		return waitFailure("FreeRADIUS never logged a login for "+user, verdict, err)
	}
	return verdict
}

// l2tpSession is one session of `show l2tp sessions` and `show l2tp session
// id <id>`: mru and auth-method are null before LCP opens.
type l2tpSession struct {
	LocalSID int     `json:"local-sid"`
	Username string  `json:"username"`
	Assigned string  `json:"assigned-addr"`
	MRU      *int    `json:"mru"`
	Auth     *string `json:"auth-method"`
}

// lnsSessionOf answers the LNS's one session, and refuses one that does not
// name user or does not hold address.
func lnsSessionOf(ctx context.Context, check *interoplab.CheckContext, user string, address netip.Addr) (l2tpSession, error) {
	var sessions []l2tpSession
	if err := restExecute(ctx, check, nodeLNS, "show l2tp sessions", &sessions); err != nil {
		return l2tpSession{}, err
	}
	if len(sessions) != 1 {
		return l2tpSession{}, fmt.Errorf("ze-lns holds %d L2TP sessions, want 1", len(sessions))
	}
	if sessions[0].Username != user {
		return l2tpSession{}, fmt.Errorf("ze-lns's session names user %q, want %s", sessions[0].Username, user)
	}
	if sessions[0].Assigned != address.String() {
		return l2tpSession{}, fmt.Errorf("ze-lns's session holds %q, want %s, the address the CPE holds", sessions[0].Assigned, address)
	}
	return sessions[0], nil
}

// checkNegotiated judges AC-11: the LNS session detail reports the
// negotiated MRU 1492 and CHAP-MD5, the CPE's device MTU is 1492, and the
// LNS's own LCP Configure-Request on the core link carried MRU 1492.
func checkNegotiated(ctx context.Context, check *interoplab.CheckContext, session l2tpSession) error {
	var detail l2tpSession
	if err := restExecute(ctx, check, nodeLNS, "show l2tp session id "+strconv.Itoa(session.LocalSID), &detail); err != nil {
		return err
	}
	if detail.MRU == nil {
		return errors.New("ze-lns's session detail reports mru null, so LCP is not open")
	}
	if *detail.MRU != sessionMRU {
		return fmt.Errorf("ze-lns's session detail reports mru %d, want %d", *detail.MRU, sessionMRU)
	}
	if detail.Auth == nil {
		return errors.New("ze-lns's session detail reports auth-method null, so LCP is not open")
	}
	if *detail.Auth != "chap-md5" {
		return fmt.Errorf("ze-lns's session detail reports auth-method %s, want chap-md5", *detail.Auth)
	}
	device, err := check.Lab.Query(ctx, nodeCPE, []string{"ip", "-o", "link", "show", ipWordDev, cpeDevice}, nil)
	if err != nil {
		return fmt.Errorf("read ze-cpe's %s: %w", cpeDevice, err)
	}
	if !strings.Contains(device, " mtu "+strconv.Itoa(sessionMRU)+" ") {
		return fmt.Errorf("ze-cpe's %s does not report MTU %d:\n%s", cpeDevice, sessionMRU, device)
	}
	frames, err := readCapture(ctx, check, nodeLNS, coreCapture)
	if err != nil {
		return err
	}
	return lcpMRUVerdict(frames, netip.MustParseAddr(lnsCoreAddress), sessionMRU)
}

// The CLI account the converted ze-cpe.conf declares (`system authentication
// user admin`, `environment ssh server main` on `::` port 22, which answers
// every address the CPE holds), and the lab-values name its password placeholder reads.
// cpeCLIStore is the account's own client store: the lab's fixture account
// (interoplab.ZeCLIStore) does not exist on the CPE.
const (
	cpeCLIUser          = "admin"
	cpeCLIPasswordValue = "user-admin-password"
	cpeCLIPort          = "22"
	cpeCLIStore         = "/tmp/ze-cli-store-admin"
)

// cpeCLILogin answers the CPE's CLI account, its password read from the
// lab-values file under root that the prepare step rendered into ze-cpe.conf.
func cpeCLILogin(root string) (interoplab.ZeCLILogin, error) {
	if root == "" {
		return interoplab.ZeCLILogin{}, errors.New("reading ze-cpe's CLI password needs the checkout's lab-values; the caller declared no checkout")
	}
	listed, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(cpeDirectory), cpeLabValuesFile)) //nolint:gosec // the checkout root joined with fixed names
	if err != nil {
		return interoplab.ZeCLILogin{}, fmt.Errorf("read the lab values for ze-cpe's CLI password: %w", err)
	}
	values, err := parseLabValues(string(listed))
	if err != nil {
		return interoplab.ZeCLILogin{}, err
	}
	password, found := values[cpeCLIPasswordValue]
	if !found {
		return interoplab.ZeCLILogin{}, fmt.Errorf("%s/%s lists no %s, the password ze-cpe.conf gives user %s; add it", cpeDirectory, cpeLabValuesFile, cpeCLIPasswordValue, cpeCLIUser)
	}
	// The CLI dials the CPE's LAN gateway address, never its loopback: the
	// converted input ruleset (ze-cpe.conf `table v4-input`, rules 20-a and
	// 20-b) admits SSH only from the LAN prefixes and drops the rest, as the
	// VyOS ruleset it came from does, and a session to 192.168.50.1 leaves
	// from 192.168.50.1 itself.
	return interoplab.ZeCLILogin{User: cpeCLIUser, Password: password, Host: netip.MustParseAddr(cpeLANGateway), Port: cpeCLIPort, Store: cpeCLIStore}, nil
}

// waitResolvers judges AC-6 in two clauses, each from its own producer.
// First the CPE's PPPoE client reports, in `show interface pppoe-client`,
// that it is up on pppoe0 with the two servers the LNS pool offered in IPCP
// (RFC 1877), primary first: only an IPCP exchange that requested them puts
// them there. Then resolv.conf holds the converted configuration's static
// `system name-server` alone, because static servers win over every learned
// one (child 13, RFC 3646 Section 6 precedent): the learned secondary
// 192.0.2.54 in the file would mean the precedence broke. The client reports
// up only after it handed the servers to resolv.conf's writer, so the file
// read after the first clause already reflects the lease.
func waitResolvers(ctx context.Context, check *interoplab.CheckContext) error {
	login, err := cpeCLILogin(check.Checkout)
	if err != nil {
		return err
	}
	var verdict error
	_, _, err = interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: 20 * time.Second, Interval: time.Second, Description: "ze-cpe's show interface pppoe-client listing the IPCP DNS servers",
	}, func(probe context.Context) ([]pppoeClientStatus, error) {
		answer, err := check.Lab.Query(probe, nodeCPE, interoplab.ZeCLICommandAs(login, "show interface pppoe-client | json"), nil)
		if err != nil {
			return nil, err
		}
		return parsePPPoEClients(answer)
	}, func(clients []pppoeClientStatus) bool {
		verdict = learnedDNSVerdict(clients, cpeDevice, []string{dnsPrimary, dnsSecondary})
		return !errors.Is(verdict, errDNSNotLearned)
	})
	if err != nil {
		return waitFailure("ze-cpe's PPPoE client never reported the IPCP DNS servers", verdict, err)
	}
	if verdict != nil {
		return fmt.Errorf("ze-cpe: %w", verdict)
	}
	resolv, err := check.Lab.Query(ctx, nodeCPE, []string{commandCat, "/tmp/resolv.conf"}, nil)
	if err != nil {
		return fmt.Errorf("read ze-cpe's /tmp/resolv.conf: %w", err)
	}
	if err := resolvConfVerdict(resolv, []string{cpeStaticNameServer}); err != nil {
		return fmt.Errorf("ze-cpe's /tmp/resolv.conf: %w", err)
	}
	return nil
}

// checkPingAcross judges AC-5: the CPE's eth0 holds no IPv4 address, its
// route to the LNS's PPP address leaves on the PPPoE device, it pings that
// address with no loss, and the LNS's PPP interface toward the subscriber
// received at least the pings sent, so the pings crossed the LAC's bridge.
func checkPingAcross(ctx context.Context, check *interoplab.CheckContext, lns string, subscriber netip.Addr, count int, extra ...string) error {
	// Exec, not Query: an eth0 with no IPv4 address is the answer this clause
	// wants, and `ip` prints nothing for it with exit 0, which Query refuses as
	// unmeasured (EmptyAnswerError). Exec still fails on a non-zero exit, so a
	// missing device or a dead container is not read as "no address".
	eth0, err := check.Lab.Exec(ctx, nodeCPE, []string{"ip", "-o", "-4", ipWordAddress, "show", ipWordDev, "eth0"}, nil)
	if err != nil {
		return fmt.Errorf("read ze-cpe's eth0: %w", err)
	}
	if strings.TrimSpace(eth0.Stdout) != "" {
		return fmt.Errorf("ze-cpe's eth0 holds an IPv4 address, so a ping could bypass the session:\n%s", eth0.Stdout)
	}
	route, err := check.Lab.Query(ctx, nodeCPE, []string{"ip", "-o", ipWordRoute, "get", poolGateway}, nil)
	if err != nil {
		return fmt.Errorf("read ze-cpe's route to %s: %w", poolGateway, err)
	}
	if err := routeViaVerdict(route, poolGateway, cpeDevice); err != nil {
		return err
	}
	device, before, err := lnsReceived(ctx, check, lns, subscriber)
	if err != nil {
		return err
	}
	argv := append([]string{commandPing, "-c", strconv.Itoa(count), "-W", "2"}, extra...)
	result, err := check.Lab.Exec(ctx, nodeCPE, append(argv, poolGateway), nil)
	if err != nil {
		return fmt.Errorf("ping %s from ze-cpe: %w", poolGateway, err)
	}
	if err := pingVerdict(result.Stdout, count); err != nil {
		return err
	}
	_, after, err := lnsReceived(ctx, check, lns, subscriber)
	if err != nil {
		return err
	}
	if after < before+uint64(count) {
		return fmt.Errorf("%s's %s received %d packets during %d pings, so the replies did not come from the session", lns,
			device, after-before, count)
	}
	return nil
}

// lnsReceived answers the PPP device of LNS node lns toward subscriber and its
// received-packet counter.
func lnsReceived(ctx context.Context, check *interoplab.CheckContext, lns string, subscriber netip.Addr) (string, uint64, error) {
	addresses, err := check.Lab.Query(ctx, lns, []string{"ip", "-o", "-4", ipWordAddress, "show"}, nil)
	if err != nil {
		return "", 0, fmt.Errorf("read %s's addresses: %w", lns, err)
	}
	device, found := deviceWithPeer(addresses, subscriber)
	if !found {
		return "", 0, fmt.Errorf("%s holds no PPP device with peer %s:\n%s", lns, subscriber, addresses)
	}
	counter, err := check.Lab.Query(ctx, lns, []string{commandCat, "/sys/class/net/" + device + "/statistics/rx_packets"}, nil)
	if err != nil {
		return "", 0, fmt.Errorf("read %s's %s receive counter: %w", lns, device, err)
	}
	packets, err := strconv.ParseUint(strings.TrimSpace(counter), 10, 64)
	if err != nil {
		return "", 0, fmt.Errorf("%s's %s receive counter %q does not parse: %w", lns, device, counter, err)
	}
	return device, packets, nil
}

// waitAccounting waits until ze_detail holds the session's records up to
// want, and judges them (AC-8).
func waitAccounting(ctx context.Context, check *interoplab.CheckContext, want string, timeout time.Duration) error {
	subscriber := netip.MustParseAddr(subscriberAddress)
	var verdict error
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: timeout, Interval: 2 * time.Second, Description: "FreeRADIUS's ze_detail holding the session's " + want,
	}, func(probe context.Context) ([]accountingRecord, error) {
		text, err := check.Lab.Query(probe, nodeRADIUS, []string{commandCat, accountingDetailPath}, nil)
		if err != nil {
			return nil, err
		}
		return parseAccountingDetail(text)
	}, func(records []accountingRecord) bool {
		verdict = accountingVerdict(records, subscriber, accessVolume, want)
		return !errors.Is(verdict, errAccountingIncomplete)
	})
	if err != nil {
		return waitFailure("FreeRADIUS's ze_detail never held the session's "+want, verdict, err)
	}
	return verdict
}

// waitFailure names a wait that timed out, with the last verdict when a
// probe ran far enough to reach one: a probe that never answered leaves it
// nil, and the wait's own error then says why.
func waitFailure(what string, verdict, err error) error {
	if verdict == nil {
		return fmt.Errorf("%s: %w", what, err)
	}
	return fmt.Errorf("%s: %w: %w", what, verdict, err)
}

// readCapture answers the frames of a capture file inside peer's container.
func readCapture(ctx context.Context, check *interoplab.CheckContext, peer, path string) ([][]byte, error) {
	encoded, err := check.Lab.Query(ctx, peer, []string{"base64", path}, nil)
	if err != nil {
		return nil, fmt.Errorf("read %s's capture %s: %w", peer, path, err)
	}
	capture, err := base64.StdEncoding.DecodeString(strings.Join(strings.Fields(encoded), ""))
	if err != nil {
		return nil, fmt.Errorf("decode %s's capture %s: %w", peer, path, err)
	}
	return pcapFrames(capture)
}

// restExecute runs one command through peer's REST surface (the api-server
// block every access-chain Ze configuration carries) and decodes its data
// into out. A refused command is an error naming its answer.
func restExecute(ctx context.Context, check *interoplab.CheckContext, peer, command string, out any) error {
	body, err := json.Marshal(map[string]string{"command": command})
	if err != nil {
		return fmt.Errorf("encode %s's command %q: %w", peer, command, err)
	}
	answer, err := check.Lab.Query(ctx, peer, []string{"wget", "-qO-",
		"--header=Authorization: Bearer secret", "--header=Content-Type: application/json",
		"--post-data=" + string(body), "http://127.0.0.1:17012/api/v1/execute"}, nil)
	if err != nil {
		return fmt.Errorf("%s: %s: %w", peer, command, err)
	}
	var response struct {
		Status string          `json:"status"`
		Error  string          `json:"error"`
		Data   json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal([]byte(answer), &response); err != nil {
		return fmt.Errorf("decode %s's answer to %s: %w: %s", peer, command, err, answer)
	}
	if response.Status != "done" {
		return fmt.Errorf("%s refused %s (status %q): %s", peer, command, response.Status, response.Error)
	}
	if err := json.Unmarshal(response.Data, out); err != nil {
		return fmt.Errorf("decode %s's data for %s: %w: %s", peer, command, err, response.Data)
	}
	return nil
}
