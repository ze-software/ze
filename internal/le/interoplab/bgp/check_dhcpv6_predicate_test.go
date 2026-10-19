package bgp

import (
	"strconv"
	"strings"
	"testing"
)

const (
	keaTestAddress = "fd00:1e:0::50"
	keaTestClient  = "00:03:00:01:52:00:77:52:d0:12"
)

// keaLog builds a Kea 2.6 INFO log in Kea's own shape. Each start line opens
// an epoch under its server DUID; each exchange is a received packet and,
// when granted, the lease line Kea writes for it.
type keaLog struct{ text strings.Builder }

func (l *keaLog) start(server string) *keaLog {
	l.text.WriteString("2026-10-19 00:44:48.738 INFO  [kea-dhcp6.dhcp6/7.1] DHCP6_USING_SERVERID server is using server-id " + server + " and stores in the file /var/lib/kea/kea-dhcp6-serverid\n")
	l.text.WriteString("2026-10-19 00:44:48.740 INFO  [kea-dhcp6.dhcp6/7.1] DHCP6_STARTED Kea DHCPv6 server version 2.6.2 started\n")
	return l
}

func (l *keaLog) received(client, tid, kind string) *keaLog {
	l.text.WriteString("2026-10-19 00:45:57.115 INFO  [kea-dhcp6.packets/7.2] DHCP6_PACKET_RECEIVED duid=[" + client + "], [no hwaddr info], tid=" + tid + ": " + kind + " (type 5) received from fe80::1 to ff02::1:2 on interface eth0\n")
	return l
}

func (l *keaLog) leased(client, tid, message, address string) {
	l.text.WriteString("2026-10-19 00:45:57.116 INFO  [kea-dhcp6.leases/7.2] " + message + "duid=[" + client + "], [no hwaddr info], tid=" + tid + ": lease for address " + address + " and iaid=304481633 has been allocated\n")
}

// exchange logs one granted exchange of kind with a fresh tid.
func (l *keaLog) exchange(kind string, serial int) *keaLog {
	tid := "0x" + strconv.Itoa(serial)
	message := keaRenew
	if kind == keaRequest {
		message = keaAlloc
	}
	l.received(keaTestClient, tid, kind)
	if kind != keaSolicit {
		l.leased(keaTestClient, tid, message, keaTestAddress)
	}
	return l
}

// bound logs one Solicit (sent twice, one tid), one Request, then renewals.
func (l *keaLog) bound(renewals int) *keaLog {
	l.received(keaTestClient, "0x1", keaSolicit).received(keaTestClient, "0x1", keaSolicit)
	l.exchange(keaRequest, 2)
	for index := range renewals {
		l.exchange(keaRenewal, 100+index)
	}
	return l
}

func (l *keaLog) String() string { return l.text.String() }

// VALIDATES: the Kea log reader and the DHCPv6 interop verdicts on both sides of every bound.
// PREVENTS: a checker that passes when ze stops renewing, renews without extending, or re-solicits.

// TestKeaEpochsCountsExchangesByTid checks the reader every verdict rests
// on: a retransmission is one exchange, a lease line counts for the type its
// tid arrived as and only for the watched address, and a start opens a new
// epoch under its own server DUID.
func TestKeaEpochsCountsExchangesByTid(t *testing.T) {
	log := new(keaLog).start("00:01:aa").bound(2)
	log.leased(keaTestClient, "0x100", keaRenew, "fd00:1e:0::500")
	log.start("00:01:bb").exchange(keaRebind, 200)
	epochs := keaEpochs(log.String(), keaTestAddress)
	if len(epochs) != 2 {
		t.Fatalf("epochs = %d, want 2", len(epochs))
	}
	first, second := epochs[0], epochs[1]
	cases := []struct {
		name      string
		got, want int
	}{
		{"solicit exchanges", first.count(keaSolicit), 1},
		{"request grants", first.grants(keaRequest), 1},
		{"renew exchanges", first.count(keaRenewal), 2},
		{"renew grants of the address", first.grants(keaRenewal), 2},
		{"rebind grants after the restart", second.grants(keaRebind), 1},
		{"clients", len(first.clients), 1},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s = %d, want %d", c.name, c.got, c.want)
		}
	}
	if first.server != "00:01:aa" || second.server != "00:01:bb" {
		t.Errorf("servers = %q, %q, want 00:01:aa, 00:01:bb", first.server, second.server)
	}
}

// TestKea6RenewalVerdict checks the renewal phase on both sides of each
// bound: 9 renewals fail and 10 pass, 16 pass and 17 fail, and a second
// SOLICIT, an unextended RENEW, a REBIND or a second client each fail.
func TestKea6RenewalVerdict(t *testing.T) {
	unextended := new(keaLog).start("00:01:aa").bound(11)
	unextended.received(keaTestClient, "0x999", keaRenewal)
	resolicit := new(keaLog).start("00:01:aa").bound(12)
	resolicit.received(keaTestClient, "0x998", keaSolicit)
	rebind := new(keaLog).start("00:01:aa").bound(12).exchange(keaRebind, 997)
	stranger := new(keaLog).start("00:01:aa").bound(12)
	stranger.received("00:03:00:01:ff:ff:ff:ff:ff:ff", "0x996", keaSolicit)
	restarted := new(keaLog).start("00:01:aa").bound(12).start("00:01:bb")
	cases := []struct {
		name string
		log  string
		ok   bool
	}{
		{"nine renewals", new(keaLog).start("00:01:aa").bound(9).String(), false},
		{"ten renewals", new(keaLog).start("00:01:aa").bound(10).String(), true},
		{"sixteen renewals", new(keaLog).start("00:01:aa").bound(16).String(), true},
		{"seventeen renewals", new(keaLog).start("00:01:aa").bound(17).String(), false},
		{"a renew Kea did not extend", unextended.String(), false},
		{"a second solicit", resolicit.String(), false},
		{"a rebind", rebind.String(), false},
		{"a second client", stranger.String(), false},
		{"a restart", restarted.String(), false},
	}
	for _, c := range cases {
		if err := kea6RenewalVerdict(c.log, keaTestAddress); (err == nil) != c.ok {
			t.Errorf("%s: err=%v, want ok=%v", c.name, err, c.ok)
		}
	}
}

// TestKea6RebindVerdict checks the rebind phase: a granted REBIND under a
// new server DUID passes and a RENEW after it completes the phase; the same
// DUID, a SOLICIT after the restart, a new client, or no granted RENEW fail.
func TestKea6RebindVerdict(t *testing.T) {
	base := func(second string) *keaLog { return new(keaLog).start("00:01:aa").bound(12).start(second) }
	cases := []struct {
		name          string
		log           *keaLog
		done, ok      bool
		renewedAfter  bool
		renewVerdicts bool
	}{
		{"waiting for the restart", new(keaLog).start("00:01:aa").bound(12), false, false, false, false},
		{"waiting for the rebind", base("00:01:bb"), false, false, false, false},
		{"rebound then renewed", base("00:01:bb").exchange(keaRebind, 300).exchange(keaRenewal, 301), true, true, true, true},
		{"rebound, no renew yet", base("00:01:bb").exchange(keaRebind, 300), true, true, false, true},
		{"same server DUID", base("00:01:aa").exchange(keaRebind, 300), true, false, false, false},
		{"solicit after the restart", base("00:01:bb").exchange(keaSolicit, 300).exchange(keaRebind, 301), true, false, false, false},
	}
	for _, c := range cases {
		done, err := kea6RebindVerdict(c.log.String(), keaTestAddress)
		if done != c.done || (err == nil) != c.ok {
			t.Errorf("%s: done=%v err=%v, want done=%v ok=%v", c.name, done, err, c.done, c.ok)
		}
		if !c.renewVerdicts {
			continue
		}
		if err := kea6RenewAfterRebindVerdict(c.log.String(), keaTestAddress); (err == nil) != c.renewedAfter {
			t.Errorf("%s: renew after rebind err=%v, want ok=%v", c.name, err, c.renewedAfter)
		}
	}
	stranger := base("00:01:bb")
	stranger.received("00:03:00:01:ff:ff:ff:ff:ff:ff", "0x400", keaRebind)
	stranger.exchange(keaRebind, 401)
	if done, err := kea6RebindVerdict(stranger.String(), keaTestAddress); !done || err == nil {
		t.Errorf("a new client after the restart: done=%v err=%v, want a failure", done, err)
	}
}

// TestZeV6LeaseVerdict checks the eth0 reading: a /128 with a finite lifetime
// up to the lease passes; forever, zero, above 40 s, another prefix length or
// absence fail.
func TestZeV6LeaseVerdict(t *testing.T) {
	line := func(prefix, valid string) string {
		return "2: eth0    inet6 " + prefix + " scope global dynamic \\       valid_lft " + valid + " preferred_lft 28sec\n" +
			"2: eth0    inet6 fd00:1e:0::2/64 scope global \\       valid_lft forever preferred_lft forever\n"
	}
	cases := []struct {
		name   string
		output string
		ok     bool
	}{
		{"leased", line(keaTestAddress+"/128", "38sec"), true},
		{"at the lease", line(keaTestAddress+"/128", "40sec"), true},
		{"above the lease", line(keaTestAddress+"/128", "41sec"), false},
		{"zero", line(keaTestAddress+"/128", "0sec"), false},
		{"forever", line(keaTestAddress+"/128", "forever"), false},
		{"wrong length", line(keaTestAddress+"/64", "38sec"), false},
		{"absent", line("fd00:1e:0::51/128", "38sec"), false},
	}
	for _, c := range cases {
		if err := zeV6LeaseVerdict(c.output, keaTestAddress); (err == nil) != c.ok {
			t.Errorf("%s: err=%v, want ok=%v", c.name, err, c.ok)
		}
	}
}
