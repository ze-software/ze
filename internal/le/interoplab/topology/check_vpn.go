// Design: docs/architecture/testing/interop.md -- the home-CPE lab.
// Related: register_home_lab.go -- the declaration this checks.
// Related: check_lan.go -- execOK, checkMasquerade, checkLease.
package topology

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// ikeWait bounds every wait on the IKE SA: a first exchange, a DNS answer
// after the gate opens, or a move to a new address.
const ikeWait = 90 * time.Second

// wordShow is the verb of a wg(8) read.
const wordShow = "show"

// checkRoadWarrior proves spec-home-cpe-11 AC-8 (spec-wireguard-runtime-proof
// AC-1 and AC-2): the kernel holds the committed WireGuard configuration
// (listen port, the laptop's key, a preshared key, its allowed-ips), wg-client
// handshakes with the CPE's PPP address, reaches lan-res-a across the tunnel
// and fetches from the web server, which sees the CPE's address.
func checkRoadWarrior(ctx context.Context, check *interoplab.CheckContext) error {
	if _, err := waitCPEAddress(ctx, check, subscriberAddress, func(held netip.Addr) bool { return held.String() == subscriberAddress }); err != nil {
		return err
	}
	laptop, err := execOK(ctx, check, nodeWGClient, []string{"wg", wordShow, "wg0", "public-key"})
	if err != nil {
		return err
	}
	dump, err := execOK(ctx, check, nodeCPE, []string{"wg", wordShow, "wg0", "dump"})
	if err != nil {
		return fmt.Errorf("read the CPE's kernel WireGuard state (the ze image needs wireguard-tools for this readback): %w", err)
	}
	if err := wireGuardDumpVerdict(dump, strings.TrimSpace(laptop)); err != nil {
		return err
	}
	if err := checkLease(ctx, check, nodeLANResA, exactly(resAAddress)); err != nil {
		return err
	}
	if _, err := execOK(ctx, check, nodeWGClient, []string{commandPing, "-c", "3", "-W", "2", resAAddress}); err != nil {
		return fmt.Errorf("wg-client did not reach lan-res-a %s across the tunnel: %w", resAAddress, err)
	}
	handshakes, err := execOK(ctx, check, nodeWGClient, []string{"wg", wordShow, "wg0", "latest-handshakes"})
	if err != nil {
		return err
	}
	if fields := strings.Fields(handshakes); len(fields) != 2 || fields[1] == "0" {
		return fmt.Errorf("wg-client reports no completed handshake with the CPE: %q", handshakes)
	}
	page, err := execOK(ctx, check, nodeWGClient, []string{"wget", "-qO-", "-T", "10", "http://" + webAddress + "/"})
	if err != nil {
		return fmt.Errorf("wg-client fetched nothing from the web server across the tunnel: %w", err)
	}
	if !strings.Contains(page, "ze home lab web") {
		return fmt.Errorf("wg-client fetched %q from the web server, not its page", page)
	}
	capture, err := readWebCapture(ctx, check)
	if err != nil {
		return err
	}
	return synSourceVerdict(capture, webAddress+".80:", subscriberAddress)
}

// wireGuardDumpVerdict judges `wg show wg0 dump`: the interface line listens
// on 51820, and the one peer is the laptop's key with a preshared key and
// allowed-ips 10.88.0.2/32 (wg(8): the peer lines carry public key, preshared
// key, endpoint, allowed ips, handshake, rx, tx, keepalive).
func wireGuardDumpVerdict(dump, laptop string) error {
	lines := strings.Split(strings.TrimSpace(dump), "\n")
	if len(lines) != 2 {
		return fmt.Errorf("the CPE's wg0 lists %d lines, want the interface and one peer: %q", len(lines), dump)
	}
	device := strings.Split(lines[0], "\t")
	if len(device) < 3 || device[2] != "51820" {
		return fmt.Errorf("the CPE's wg0 does not listen on 51820: %q", lines[0])
	}
	peer := strings.Split(lines[1], "\t")
	if len(peer) < 4 {
		return fmt.Errorf("the CPE's wg0 peer line is short: %q", lines[1])
	}
	if peer[0] != laptop {
		return fmt.Errorf("the CPE's wg0 peer is %s, want the laptop's key %s", peer[0], laptop)
	}
	if peer[1] == "(none)" {
		return fmt.Errorf("the CPE's wg0 peer holds no preshared key")
	}
	if peer[3] != wgClientInner+"/32" {
		return fmt.Errorf("the CPE's wg0 peer allows %s, want %s/32", peer[3], wgClientInner)
	}
	return nil
}

// checkMgmtTunnel proves spec-home-cpe-11 AC-9 for the P-256 variant: with
// DNS down at CPE boot no IKE SA exists; once DNS answers the SA is
// established with the CPE's certificate, on UDP 4500 (MOBIKE), its
// CHILD_SA narrowed to ::/0 on both sides; an IPv6 ping across vti0 loses
// nothing and each simplex SA's bytes advance on both peers. The Ed25519
// variant is checkMgmtTunnelEd25519.
func checkMgmtTunnel(ctx context.Context, check *interoplab.CheckContext) error {
	if _, err := waitCPEAddress(ctx, check, subscriberAddress, func(held netip.Addr) bool { return held.String() == subscriberAddress }); err != nil {
		return err
	}
	before, err := execOK(ctx, check, nodeSwan, []string{"swanctl", "--list-sas"})
	if err != nil {
		return err
	}
	if sa := parseSwanSA(before); sa.established {
		return fmt.Errorf("an IKE SA was established while DNS was down, so the DNS clause proves nothing: %q", before)
	}
	if _, err := execOK(ctx, check, nodeDNS, []string{"touch", dnsGate}); err != nil {
		return err
	}
	if err := waitSwanSA(ctx, check, subscriberAddress); err != nil {
		return err
	}
	return checkESPAcross(ctx, check, subscriberAddress)
}

// charonEd25519 is charon's signature_scheme_names rendering of SIGN_ED25519,
// as the IPsec lab's x509-ed25519 scenarios read it (ipsec/eddsa.go).
const charonEd25519 = "ED25519"

// swanLogLines bounds the strongSwan log tail the Ed25519 verdict reads: the
// whole of one IKE_SA_INIT and IKE_AUTH exchange at charon's default level.
const swanLogLines = 2000

// checkMgmtTunnelEd25519 proves spec-home-cpe-11 AC-9 for the Ed25519
// variant: every clause of checkMgmtTunnel with the CPE holding an Ed25519
// key, and strongSwan's own account that it verified the CPE's AUTH with
// Ed25519. The SA coming up says both AUTH payloads verified, not with which
// scheme, so the checker reads the line pubkey_authenticator.c process()
// logs only after the signature verified (ipsec/eddsa.go quotes it).
func checkMgmtTunnelEd25519(ctx context.Context, check *interoplab.CheckContext) error {
	if err := checkMgmtTunnel(ctx, check); err != nil {
		return err
	}
	logs, err := check.Lab.Logs(ctx, nodeSwan, swanLogLines)
	if err != nil {
		return fmt.Errorf("read strongSwan's log: %w", err)
	}
	if !logs.Available {
		return errors.New("strongSwan's log is not readable, so the Ed25519 AUTH cannot be judged")
	}
	return swanAuthVerdict(logs.Text, cpeIKEName, charonEd25519)
}

// swanAuthVerdict accepts a strongSwan log holding the line that says it
// verified identity's AUTH with scheme.
func swanAuthVerdict(logs, identity, scheme string) error {
	want := "authentication of '" + identity + "' with " + scheme + " successful"
	for line := range strings.SplitSeq(logs, "\n") {
		if strings.Contains(line, want) {
			return nil
		}
	}
	return fmt.Errorf("strongSwan logs no %q, so it did not verify the CPE's AUTH with %s:\n%s",
		want, scheme, interoplab.OutputExcerpt(logs))
}

// checkAccessReconnect proves spec-home-cpe-11 AC-11: once FreeRADIUS answers
// a new Framed-IP-Address and the LNS tears the session down, the CPE comes
// back with the new address, the IKE SA's CPE address is the new one, ESP
// traffic resumes, and the web server sees the new address as source.
func checkAccessReconnect(ctx context.Context, check *interoplab.CheckContext) error {
	first, err := waitCPEAddress(ctx, check, subscriberAddress, func(held netip.Addr) bool { return held.String() == subscriberAddress })
	if err != nil {
		return err
	}
	if err := waitSwanSA(ctx, check, subscriberAddress); err != nil {
		return err
	}
	if _, err := execOK(ctx, check, nodeRADIUS, []string{"sed", "-i", "s/" + subscriberAddress + "/" + reconnectAddress + "/",
		"/etc/raddb/mods-config/files/authorize"}); err != nil {
		return err
	}
	// FreeRADIUS reads the users file at start; the prestart copy is not
	// redone on a restart, so the edit stands.
	if err := check.Lab.Stop(ctx, nodeRADIUS, 5); err != nil {
		return fmt.Errorf("stop freeradius to reload its users: %w", err)
	}
	if err := check.Lab.Start(ctx, nodeRADIUS); err != nil {
		return fmt.Errorf("start freeradius with the new answer: %w", err)
	}
	session, err := lnsSessionOf(ctx, check, subscriberUser, first)
	if err != nil {
		return err
	}
	var cleared json.RawMessage
	if err := restExecute(ctx, check, nodeLNS, "clear l2tp session id "+strconv.Itoa(session.LocalSID), &cleared); err != nil {
		return err
	}
	if _, err := waitCPEAddress(ctx, check, reconnectAddress, func(held netip.Addr) bool { return held.String() == reconnectAddress }); err != nil {
		return err
	}
	if err := waitSwanSA(ctx, check, reconnectAddress); err != nil {
		return fmt.Errorf("the IKE SA did not follow the CPE to %s (spec-home-cpe-4): %w", reconnectAddress, err)
	}
	if err := checkESPAcross(ctx, check, reconnectAddress); err != nil {
		return err
	}
	return checkMasquerade(ctx, check, nodeLANStatic, reconnectAddress)
}

// swanSA is what one `swanctl --list-sas` answer says about the CPE's SA.
type swanSA struct {
	established bool
	remote      string
	remotePort  string
	installed   bool
	localTS     []string
	remoteTS    []string
}

// parseSwanSA reads `swanctl --list-sas` (swanctl/commands/list_sas.c): an
// IKE_SA header "cpe: #N, ESTABLISHED, IKEv2, ...", its "remote 'id' @
// addr[port]" line, a CHILD_SA header "mgmt: #N, reqid N, INSTALLED, ..." and
// its "local"/"remote" selector lines.
func parseSwanSA(text string) swanSA {
	var sa swanSA
	child := false
	for line := range strings.SplitSeq(text, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "cpe: ") && strings.Contains(trimmed, "ESTABLISHED"):
			sa.established = true
		case strings.HasPrefix(trimmed, "mgmt: ") && strings.Contains(trimmed, "INSTALLED"):
			sa.installed = true
			child = true
		case child && strings.HasPrefix(trimmed, "local "):
			sa.localTS = append(sa.localTS, strings.Fields(trimmed)[1:]...)
		case child && strings.HasPrefix(trimmed, "remote "):
			sa.remoteTS = append(sa.remoteTS, strings.Fields(trimmed)[1:]...)
		case strings.HasPrefix(trimmed, "remote '"):
			_, endpoint, _ := strings.Cut(trimmed, " @ ")
			address, port, _ := strings.Cut(endpoint, "[")
			sa.remote = address
			sa.remotePort = strings.TrimSuffix(port, "]")
		}
	}
	return sa
}

// swanSAVerdict judges the SA against the CPE address it must sit on.
func swanSAVerdict(sa swanSA, cpe string) error {
	if !sa.established {
		return fmt.Errorf("strongSwan holds no established IKE SA with the CPE")
	}
	if sa.remote != cpe {
		return fmt.Errorf("strongSwan's IKE SA sits on CPE address %s, want %s", sa.remote, cpe)
	}
	if sa.remotePort != "4500" {
		return fmt.Errorf("strongSwan's IKE SA sits on port %s, want 4500 (MOBIKE, RFC 4555 Section 3.3)", sa.remotePort)
	}
	if !sa.installed {
		return fmt.Errorf("strongSwan holds no installed CHILD_SA mgmt")
	}
	if len(sa.localTS) != 1 || sa.localTS[0] != "::/0" {
		return fmt.Errorf("the CHILD_SA's local selectors are %v, want ::/0 alone (narrowed)", sa.localTS)
	}
	if len(sa.remoteTS) != 1 || sa.remoteTS[0] != "::/0" {
		return fmt.Errorf("the CHILD_SA's remote selectors are %v, want ::/0 alone (narrowed)", sa.remoteTS)
	}
	return nil
}

// waitSwanSA waits until strongSwan's view of the SA passes swanSAVerdict.
func waitSwanSA(ctx context.Context, check *interoplab.CheckContext, cpe string) error {
	return pollUntil(ctx, ikeWait, "strongSwan holds the CPE's SA on "+cpe, func() error {
		listed, err := execOK(ctx, check, nodeSwan, []string{"swanctl", "--list-sas"})
		if err != nil {
			return err
		}
		return swanSAVerdict(parseSwanSA(listed), cpe)
	})
}

// checkESPAcross pings strongSwan's management address across vti0 and proves
// no loss and, on both peers, that the bytes of each simplex SA (read by its
// own source and destination, never an SPI-keyed sum: vacuity trap 5)
// advanced.
func checkESPAcross(ctx context.Context, check *interoplab.CheckContext, cpe string) error {
	outbound := [2]string{cpe, swanAddress}
	inbound := [2]string{swanAddress, cpe}
	before := map[string]map[[2]string]uint64{}
	for _, peer := range []string{nodeCPE, nodeSwan} {
		counters, err := readXFRMBytes(ctx, check, peer)
		if err != nil {
			return err
		}
		before[peer] = counters
	}
	if _, err := execOK(ctx, check, nodeCPE, []string{commandPing, "-6", "-c", "5", "-W", "2", swanManagement}); err != nil {
		return fmt.Errorf("the CPE's IPv6 ping across vti0 to %s lost packets: %w", swanManagement, err)
	}
	for _, peer := range []string{nodeCPE, nodeSwan} {
		after, err := readXFRMBytes(ctx, check, peer)
		if err != nil {
			return err
		}
		for _, direction := range [][2]string{outbound, inbound} {
			if err := advancedVerdict(before[peer], after, direction); err != nil {
				return fmt.Errorf("%s: %w", peer, err)
			}
		}
	}
	return nil
}

// readXFRMBytes answers each ESP state's current bytes on peer.
func readXFRMBytes(ctx context.Context, check *interoplab.CheckContext, peer string) (map[[2]string]uint64, error) {
	listed, err := execOK(ctx, check, peer, []string{"ip", "-s", "xfrm", "state"})
	if err != nil {
		return nil, err
	}
	return parseXFRMBytes(listed)
}

// parseXFRMBytes reads `ip -s xfrm state`: each state opens with "src A dst
// B", and its "lifetime current:" line is followed by "N(bytes), M(packets)".
// Two states with one source and destination are refused: their bytes could
// not be told apart.
func parseXFRMBytes(text string) (map[[2]string]uint64, error) {
	counters := map[[2]string]uint64{}
	var current [2]string
	inState := false
	lines := strings.Split(text, "\n")
	for index, line := range lines {
		fields := strings.Fields(line)
		if len(fields) == 4 && fields[0] == "src" && fields[2] == "dst" {
			current = [2]string{fields[1], fields[3]}
			inState = true
			continue
		}
		if !inState || strings.TrimSpace(line) != "lifetime current:" || index+1 >= len(lines) {
			continue
		}
		octets, _, found := strings.Cut(strings.TrimSpace(lines[index+1]), "(bytes)")
		if !found {
			return nil, fmt.Errorf("the xfrm state %s to %s carries no byte count: %q", current[0], current[1], lines[index+1])
		}
		value, err := strconv.ParseUint(octets, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("the xfrm state %s to %s byte count %q: %w", current[0], current[1], octets, err)
		}
		if _, twice := counters[current]; twice {
			return nil, fmt.Errorf("two xfrm states run %s to %s, so their bytes cannot be read per direction", current[0], current[1])
		}
		counters[current] = value
	}
	return counters, nil
}

// advancedVerdict proves the state of direction exists after the probe and
// counted more bytes than before it.
func advancedVerdict(before, after map[[2]string]uint64, direction [2]string) error {
	now, found := after[direction]
	if !found {
		return fmt.Errorf("no ESP state runs %s to %s", direction[0], direction[1])
	}
	if now <= before[direction] {
		return fmt.Errorf("the ESP state %s to %s stayed at %d bytes across the ping", direction[0], direction[1], now)
	}
	return nil
}
