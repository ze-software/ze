// Design: docs/architecture/testing/interop.md -- DHCPv6 client evidence read from Kea's log and ze's eth0.
// Related: check_dhcpv6.go -- the lab I/O that feeds these readings.
package bgp

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// Kea 2.6 logs at INFO one line per DHCPv6 message it receives, one per lease
// it grants or extends, and at every start the server DUID it uses:
//
//	... DHCP6_PACKET_RECEIVED duid=[00:03:00:01:..], [no hwaddr info], tid=0x97a504: RENEW (type 5) received from fe80::.. to ff02::1:2 on interface eth0
//	... DHCP6_LEASE_RENEW duid=[00:03:00:01:..], [no hwaddr info], tid=0x97a504: lease for address fd00:1e:0::50 and iaid=304481633 has been allocated
//	... DHCP6_USING_SERVERID server is using server-id 00:01:00:01:.. and stores in the file /var/lib/kea/kea-dhcp6-serverid
//
// A DHCPv6 client keeps the transaction id across retransmissions, so the
// number of distinct tids of one message type is the number of exchanges.
const (
	keaServerID = "DHCP6_USING_SERVERID server is using server-id "
	keaReceived = "DHCP6_PACKET_RECEIVED "
	keaAlloc    = "DHCP6_LEASE_ALLOC "
	keaRenew    = "DHCP6_LEASE_RENEW "
	keaLeaseFor = "lease for address "

	keaSolicit = "SOLICIT"
	keaRequest = "REQUEST"
	keaRenewal = "RENEW"
	keaRebind  = "REBIND"
)

// The scenario's lease (kea-dhcp6.conf): valid 40 s, T1 10 s. The renewal
// phase watches ze for kea6HoldPeriods lease periods plus a margin, so ze
// renews twelve or thirteen times in it. The bounds on that count are the
// discrimination: a client that never renews draws none; one that renews but
// keeps its first deadlines loses the address after one lease and solicits
// again, which the one-SOLICIT clause refuses; one that answers a stale T1 by
// renewing at once sends about one Renew a second (ze's per-interface rate
// limit), far above kea6RenewalsMax.
const (
	kea6ValidLifetime  = 40 * time.Second
	kea6RenewTimer     = 10 * time.Second
	kea6HoldPeriods    = 3
	kea6HoldMargin     = 10 * time.Second
	kea6RenewalsWanted = 10
	kea6RenewalsMax    = 16
)

// keaEpoch is what Kea logged between one start and the next.
type keaEpoch struct {
	server string
	// exchanges holds, per message type, the transaction ids Kea received.
	exchanges map[string]map[string]bool
	// granted holds, per message type, the transaction ids whose exchange
	// granted or extended the watched address.
	granted map[string]map[string]bool
	clients map[string]bool
}

func newKeaEpoch() *keaEpoch {
	return &keaEpoch{exchanges: map[string]map[string]bool{}, granted: map[string]map[string]bool{}, clients: map[string]bool{}}
}

func (e *keaEpoch) count(kind string) int { return len(e.exchanges[kind]) }

func (e *keaEpoch) grants(kind string) int { return len(e.granted[kind]) }

// keaEpochs reads a Kea log into one epoch per server start, in order: the
// DHCP6_USING_SERVERID line opens an epoch. Lines before the first one, and lines of other shapes, count for nothing. A
// lease line is attributed to the message type its tid was received as.
func keaEpochs(log, address string) []*keaEpoch {
	var epochs []*keaEpoch
	var current *keaEpoch
	kinds := map[string]string{}
	for line := range strings.Lines(log) {
		if rest, found := cutAfter(line, keaServerID); found {
			current = newKeaEpoch()
			current.server, _, _ = strings.Cut(rest, " ")
			epochs = append(epochs, current)
			continue
		}
		if current == nil {
			continue
		}
		if rest, found := cutAfter(line, keaReceived); found {
			duid, tid, message, ok := keaClientFields(rest)
			if !ok {
				continue
			}
			kind, _, _ := strings.Cut(message, " ")
			kinds[tid] = kind
			current.clients[duid] = true
			addTid(current.exchanges, kind, tid)
			continue
		}
		rest, found := cutAfter(line, keaAlloc)
		if !found {
			rest, found = cutAfter(line, keaRenew)
		}
		if !found {
			continue
		}
		_, tid, message, ok := keaClientFields(rest)
		if !ok {
			continue
		}
		leased, isLease := strings.CutPrefix(message, keaLeaseFor)
		if !isLease {
			continue
		}
		if leasedAddress, _, _ := strings.Cut(leased, " "); leasedAddress != address {
			continue
		}
		addTid(current.granted, kinds[tid], tid)
	}
	return epochs
}

// keaClientFields splits "duid=[D], [no hwaddr info], tid=0xT: MESSAGE" into
// its client DUID, transaction id and message.
func keaClientFields(rest string) (duid, tid, message string, ok bool) {
	duid, found := strings.CutPrefix(rest, "duid=[")
	if !found {
		return "", "", "", false
	}
	duid, rest, found = strings.Cut(duid, "]")
	if !found {
		return "", "", "", false
	}
	_, rest, found = strings.Cut(rest, "tid=")
	if !found {
		return "", "", "", false
	}
	tid, message, found = strings.Cut(rest, ": ")
	if !found {
		return "", "", "", false
	}
	return duid, tid, strings.TrimSpace(message), true
}

func cutAfter(line, marker string) (string, bool) {
	_, rest, found := strings.Cut(line, marker)
	return rest, found
}

func addTid(sets map[string]map[string]bool, kind, tid string) {
	if sets[kind] == nil {
		sets[kind] = map[string]bool{}
	}
	sets[kind][tid] = true
}

// kea6RenewalVerdict reads the renewal phase once its window has passed:
// one Kea start, one client DUID, one SOLICIT and one REQUEST exchange that
// granted the address, then between kea6RenewalsWanted and kea6RenewalsMax
// RENEW exchanges, each of which extended it, and no REBIND.
func kea6RenewalVerdict(log, address string) error {
	epochs := keaEpochs(log, address)
	if len(epochs) != 1 {
		return kea6CountError("Kea logged", len(epochs), "starts before the restart, want 1")
	}
	epoch := epochs[0]
	if len(epoch.clients) != 1 {
		return kea6CountError("Kea heard", len(epoch.clients), "client DUIDs, want ze's one")
	}
	if n := epoch.count(keaSolicit); n != 1 {
		return kea6CountError("ze ran", n, "SOLICIT exchanges while its lease was held, want 1")
	}
	if n := epoch.grants(keaRequest); n != 1 {
		return kea6CountError("Kea granted", n, "REQUEST exchanges the address "+address+", want 1")
	}
	renewals := epoch.count(keaRenewal)
	if renewals < kea6RenewalsWanted {
		return kea6CountError("ze ran", renewals, "RENEW exchanges in three lease periods, want at least 10")
	}
	if renewals > kea6RenewalsMax {
		return kea6CountError("ze ran", renewals, "RENEW exchanges in three lease periods, want at most 16 (one per T1)")
	}
	if extended := epoch.grants(keaRenewal); extended != renewals {
		return kea6CountError("Kea extended the lease in", extended, "of "+strconv.Itoa(renewals)+" RENEW exchanges, want every one")
	}
	if n := epoch.count(keaRebind); n != 0 {
		return kea6CountError("ze ran", n, "REBIND exchanges while Kea answered every RENEW, want 0")
	}
	return nil
}

// kea6RebindVerdict reads the rebind phase: Kea started a second time under
// another server DUID, the one client DUID is unchanged, and a REBIND
// exchange extended the address with no SOLICIT. done is false while the log
// is still on its way to an answer.
func kea6RebindVerdict(log, address string) (done bool, err error) {
	epochs := keaEpochs(log, address)
	if len(epochs) < 2 {
		return false, kea6CountError("Kea logged", len(epochs), "starts, want 2 after the restart")
	}
	if len(epochs) > 2 {
		return true, kea6CountError("Kea logged", len(epochs), "starts, want 2")
	}
	before, after := epochs[0], epochs[1]
	if before.server == after.server {
		return true, errors.New("the restarted Kea kept server DUID " + before.server + ", so no rebind is forced")
	}
	if n := after.count(keaSolicit); n != 0 {
		return true, kea6CountError("ze ran", n, "SOLICIT exchanges after the restart, want 0")
	}
	for client := range after.clients {
		if !before.clients[client] {
			return true, errors.New("a client DUID " + client + " Kea had not heard before the restart reached it")
		}
	}
	if after.grants(keaRebind) == 0 {
		return false, errors.New("no REBIND exchange extended " + address + " after the restart")
	}
	return true, nil
}

// kea6RenewAfterRebindVerdict reads the end of the rebind phase: after the
// restart, at least one RENEW exchange extended the address. Kea drops a
// RENEW naming another server, so this one named the new server: ze took the
// identity of the server that answered its REBIND.
func kea6RenewAfterRebindVerdict(log, address string) error {
	epochs := keaEpochs(log, address)
	if len(epochs) != 2 {
		return kea6CountError("Kea logged", len(epochs), "starts, want 2")
	}
	if n := epochs[1].count(keaSolicit); n != 0 {
		return kea6CountError("ze ran", n, "SOLICIT exchanges after the restart, want 0")
	}
	if epochs[1].grants(keaRenewal) == 0 {
		return errors.New("no RENEW to the restarted Kea extended " + address + ": ze did not adopt the server that answered its REBIND")
	}
	return nil
}

// zeV6LeaseVerdict reads `ip -6 -o addr show dev eth0`: address is on the
// interface as a /128 with a finite valid lifetime no longer than lifetime,
// the lease the scenario's server grants.
//
//	2: eth0    inet6 fd00:1e:0::50/128 scope global dynamic \       valid_lft 38sec preferred_lft 28sec
func zeV6LeaseVerdict(output, address string, lifetime time.Duration) error {
	lifetimeSeconds := int(lifetime / time.Second)
	for line := range strings.Lines(output) {
		fields := strings.Fields(line)
		index := indexOf(fields, address+"/128")
		if index < 0 {
			continue
		}
		valid := indexOf(fields, "valid_lft")
		if valid < 0 {
			return errors.New("ze's " + address + " carries no valid_lft")
		}
		if valid+1 >= len(fields) {
			return errors.New("ze's " + address + " carries valid_lft with no value after it")
		}
		seconds, err := strconv.Atoi(strings.TrimSuffix(fields[valid+1], "sec"))
		if err != nil {
			return errors.New("ze installed " + address + " with valid_lft " + fields[valid+1] + ", want a finite lifetime of at most " + strconv.Itoa(lifetimeSeconds) + " s")
		}
		if seconds <= 0 {
			return kea6CountError("ze's "+address+" has valid_lft", seconds, "s, want above 0")
		}
		if seconds > lifetimeSeconds {
			return kea6CountError("ze's "+address+" has valid_lft", seconds, "s, want at most the "+strconv.Itoa(lifetimeSeconds)+" s lease")
		}
		return nil
	}
	return errors.New("ze's eth0 does not hold the leased " + address + "/128")
}

func indexOf(fields []string, want string) int {
	for index, field := range fields {
		if field == want {
			return index
		}
	}
	return -1
}

func kea6CountError(prefix string, count int, suffix string) error {
	var text textbuf.Buffer
	return errors.New(text.Str(prefix).Byte(' ').Int(int64(count)).Byte(' ').Str(suffix).String())
}
