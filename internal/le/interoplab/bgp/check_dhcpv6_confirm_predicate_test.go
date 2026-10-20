package bgp

import (
	"strings"
	"testing"
)

const keaTestMoved = "fd00:1e:0:1::50"

// keaConfirmLog builds a Kea 2.6 log with the debug packet lines the Confirm
// scenario reads, in Kea's own shape.
type keaConfirmLog struct{ keaLog }

// reply logs Kea's Reply to tid and, under it, the status code option.
func (l *keaConfirmLog) reply(tid, status string) *keaConfirmLog {
	l.text.WriteString("2026-10-20 01:20:00.100 DEBUG [kea-dhcp6.packets/7.2] DHCP6_RESPONSE_DATA duid=[" + keaTestClient + "], [no hwaddr info], tid=" + tid + ": responding with packet REPLY (type 7), packet details: localAddr=[fe80::16]:547 remoteAddr=[fe80::2]:546\n")
	l.text.WriteString("msgtype=7(REPLY), transid=0x" + strings.TrimPrefix(tid, "0x") + "\n")
	l.text.WriteString("type=00002, len=00014: 00:01:00:01:30:8a:ad:10:02:42:ac:1e:00:16\n")
	if status != "" {
		l.text.WriteString("  type=00013, len=00039: " + status + "\n")
	}
	return l
}

// confirm logs ze's Confirm, with a retransmission, and Kea's Reply.
func (l *keaConfirmLog) confirm(tid, status string) *keaConfirmLog {
	l.received(keaTestClient, tid, keaConfirm).received(keaTestClient, tid, keaConfirm)
	return l.reply(tid, status)
}

// solicitRequest logs one Solicit and one Request granted address.
func (l *keaConfirmLog) solicitRequest(solicit, request, address string) *keaConfirmLog {
	l.received(keaTestClient, solicit, keaSolicit)
	l.received(keaTestClient, request, keaRequest)
	l.leased(keaTestClient, request, keaAlloc, address)
	l.reply(request, "")
	return l
}

func (l *keaConfirmLog) reload() *keaConfirmLog {
	l.text.WriteString("2026-10-20 01:21:00.000 INFO  [kea-dhcp6.dhcp6/7.1] DHCP6_DYNAMIC_RECONFIGURATION_SUCCESS dynamic server reconfiguration succeeded with file: /run/kea/kea-dhcp6.conf\n")
	return l
}

const (
	keaTestSuccess   = "Success(0) \"All addresses are on-link\""
	keaTestNotOnLink = "NotOnLink(4) \"Address fd00:1e:0::50 is not on link.\""
)

// replugged is the log of a passing first phase.
func replugged() *keaConfirmLog {
	l := new(keaConfirmLog)
	l.start("00:01:aa")
	l.solicitRequest("0x1", "0x2", keaTestAddress)
	return l.confirm("0x3", keaTestSuccess)
}

// VALIDATES: the Kea timeline reader and the Confirm interop verdicts in both polarities.
// PREVENTS: a checker that passes when ze sends no Confirm, when Kea's answer is not
// the status the phase needs, or when ze re-solicits or renews instead.

// TestKeaStatusName reads Kea's rendering of the status code option.
func TestKeaStatusName(t *testing.T) {
	cases := map[string]string{
		"type=00013, len=00025: " + keaTestSuccess:   keaStatusSuccess,
		"type=00013, len=00025: " + keaTestNotOnLink: keaStatusNotOnLink,
		"type=00013, len=00002":                      "",
		"type=00013, len=00025: Success":             "",
	}
	for text, want := range cases {
		if got := keaStatusName(text); got != want {
			t.Errorf("keaStatusName(%q) = %q, want %q", text, got, want)
		}
	}
}

// TestKea6ReplugVerdict drives the first phase's verdict: one Confirm after
// the bind, answered Success, and nothing else passes.
func TestKea6ReplugVerdict(t *testing.T) {
	cases := []struct {
		name string
		log  func() *keaConfirmLog
		ok   bool
	}{
		{"confirmed", replugged, true},
		{"later phases are not read", func() *keaConfirmLog {
			return replugged().reload().confirm("0x9", keaTestNotOnLink)
		}, true},
		{"no confirm", func() *keaConfirmLog {
			l := new(keaConfirmLog)
			l.start("00:01:aa")
			return l.solicitRequest("0x1", "0x2", keaTestAddress)
		}, false},
		{"two confirms", func() *keaConfirmLog { return replugged().confirm("0x4", keaTestSuccess) }, false},
		{"answered NotOnLink", func() *keaConfirmLog {
			l := new(keaConfirmLog)
			l.start("00:01:aa")
			l.solicitRequest("0x1", "0x2", keaTestAddress)
			return l.confirm("0x3", keaTestNotOnLink)
		}, false},
		{"no reply", func() *keaConfirmLog {
			l := new(keaConfirmLog)
			l.start("00:01:aa")
			l.solicitRequest("0x1", "0x2", keaTestAddress)
			l.received(keaTestClient, "0x3", keaConfirm)
			return l
		}, false},
		{"reply without status", func() *keaConfirmLog {
			l := new(keaConfirmLog)
			l.start("00:01:aa")
			l.solicitRequest("0x1", "0x2", keaTestAddress)
			return l.confirm("0x3", "")
		}, false},
		{"re-solicited", func() *keaConfirmLog {
			return replugged().solicitRequest("0x5", "0x6", keaTestAddress)
		}, false},
		{"renewed", func() *keaConfirmLog {
			l := replugged()
			l.exchange(keaRenewal, 7)
			return l
		}, false},
		{"confirm before the bind", func() *keaConfirmLog {
			l := new(keaConfirmLog)
			l.start("00:01:aa")
			l.confirm("0x3", keaTestSuccess)
			return l.solicitRequest("0x1", "0x2", keaTestAddress)
		}, false},
		{"another client", func() *keaConfirmLog {
			l := replugged()
			l.received("00:03:00:01:ff:ff:ff:ff:ff:ff", "0x8", "INFORMATION-REQUEST")
			return l
		}, false},
	}
	for _, c := range cases {
		if err := kea6ReplugVerdict(c.log().String(), keaTestAddress); (err == nil) != c.ok {
			t.Errorf("%s: err=%v, want ok=%v", c.name, err, c.ok)
		}
	}
}

// TestKea6MovedVerdict drives the second phase's verdict: after the one
// reload, a Confirm answered NotOnLink, then a Solicit and a Request that
// leased the moved address.
func TestKea6MovedVerdict(t *testing.T) {
	moved := func() *keaConfirmLog {
		return replugged().reload().confirm("0x10", keaTestNotOnLink).solicitRequest("0x11", "0x12", keaTestMoved)
	}
	cases := []struct {
		name string
		log  func() *keaConfirmLog
		ok   bool
	}{
		{"moved", moved, true},
		{"no reload", func() *keaConfirmLog {
			return replugged().confirm("0x10", keaTestNotOnLink).solicitRequest("0x11", "0x12", keaTestMoved)
		}, false},
		{"two reloads", func() *keaConfirmLog { return moved().reload() }, false},
		{"no confirm after the reload", func() *keaConfirmLog {
			return replugged().reload().solicitRequest("0x11", "0x12", keaTestMoved)
		}, false},
		{"answered Success", func() *keaConfirmLog {
			return replugged().reload().confirm("0x10", keaTestSuccess).solicitRequest("0x11", "0x12", keaTestMoved)
		}, false},
		{"solicit before the confirm", func() *keaConfirmLog {
			return replugged().reload().solicitRequest("0x11", "0x12", keaTestMoved).confirm("0x10", keaTestNotOnLink)
		}, false},
		{"old address leased again", func() *keaConfirmLog {
			return replugged().reload().confirm("0x10", keaTestNotOnLink).solicitRequest("0x11", "0x12", keaTestAddress)
		}, false},
		{"rebound", func() *keaConfirmLog {
			l := moved()
			l.exchange(keaRebind, 13)
			return l
		}, false},
	}
	for _, c := range cases {
		if err := kea6MovedVerdict(c.log().String(), keaTestAddress, keaTestMoved); (err == nil) != c.ok {
			t.Errorf("%s: err=%v, want ok=%v", c.name, err, c.ok)
		}
	}
}

// TestKeaReloadVerdict reads the reload: done once logged, an error on a
// refused configuration.
func TestKeaReloadVerdict(t *testing.T) {
	if done, err := keaReloadVerdict(replugged().String()); done || err != nil {
		t.Errorf("before the reload: done=%v err=%v, want not done", done, err)
	}
	if done, err := keaReloadVerdict(replugged().reload().String()); !done || err != nil {
		t.Errorf("after the reload: done=%v err=%v, want done", done, err)
	}
	refused := replugged().String() + "2026-10-20 01:21:00.000 ERROR [kea-dhcp6.dhcp6/7.1] DHCP6_DYNAMIC_RECONFIGURATION_FAIL dynamic server reconfiguration failed with file: /run/kea/kea-dhcp6.conf\n"
	if _, err := keaReloadVerdict(refused); err == nil {
		t.Error("a refused reload was not an error")
	}
}

// TestZeV6AbsentVerdict checks the old address is gone under any length.
func TestZeV6AbsentVerdict(t *testing.T) {
	held := "2: eth0    inet6 " + keaTestAddress + "/128 scope global dynamic \\       valid_lft 500sec preferred_lft 400sec\n"
	if err := zeV6AbsentVerdict(held, keaTestAddress); err == nil {
		t.Error("a held /128 read as absent")
	}
	if err := zeV6AbsentVerdict(strings.Replace(held, "/128", "/64", 1), keaTestAddress); err == nil {
		t.Error("a held /64 read as absent")
	}
	if err := zeV6AbsentVerdict(strings.Replace(held, keaTestAddress, keaTestMoved, 1), keaTestAddress); err != nil {
		t.Errorf("only the moved address held: %v", err)
	}
}
