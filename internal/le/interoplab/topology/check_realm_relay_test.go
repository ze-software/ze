package topology

import (
	"strings"
	"testing"
)

// verdictWant checks one verdict outcome: want empty means the verdict must
// accept, otherwise it must refuse with an error holding want.
func verdictWant(t *testing.T, name string, err error, want string) {
	t.Helper()
	if want == "" {
		if err != nil {
			t.Errorf("%s: %v", name, err)
		}
		return
	}
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Errorf("%s: err = %v, want %q", name, err, want)
	}
}

// TestLACRealmSessionVerdict proves the realm-relay checkers accept only one
// LAC session, a realm call to the configured remote whose ICCN carried both
// proxy LCP and proxy authentication, and refuse a Service-Name relay (no
// relay keys), a second session (the stray subscriber relayed), another
// realm or remote, and a call that sent either proxy set as false.
// METHOD: one table row per refusal, each changing one field of a good session.
func TestLACRealmSessionVerdict(t *testing.T) {
	text := func(s string) *string { return &s }
	flag := func(b bool) *bool { return &b }
	good := func() lacRelaySession {
		return lacRelaySession{LocalSID: 1, RelayRealm: text(relayedRealm), RelayRemote: text(relayRemote),
			ProxyLCPSent: flag(true), ProxyAuthSent: flag(true)}
	}
	with := func(change func(*lacRelaySession)) []lacRelaySession {
		session := good()
		change(&session)
		return []lacRelaySession{session}
	}
	for _, tc := range []struct {
		name     string
		sessions []lacRelaySession
		want     string
	}{
		{"realm call", []lacRelaySession{good()}, ""},
		{"no session", nil, "holds 0 L2TP sessions"},
		{"stray relayed too", []lacRelaySession{good(), good()}, "holds 2 L2TP sessions"},
		{"service-name relay", []lacRelaySession{{LocalSID: 1}}, "no relay-realm"},
		{"other realm", with(func(s *lacRelaySession) { s.RelayRealm = text("other.example.net") }), "want lab.example.net"},
		{"no remote", with(func(s *lacRelaySession) { s.RelayRemote = nil }), "no relay-remote"},
		{"other remote", with(func(s *lacRelaySession) { s.RelayRemote = text("lns-b") }), "want lns"},
		{"no proxy-lcp-sent", with(func(s *lacRelaySession) { s.ProxyLCPSent = nil }), "no proxy-lcp-sent"},
		{"proxy lcp not sent", with(func(s *lacRelaySession) { s.ProxyLCPSent = flag(false) }), "proxy-lcp-sent false"},
		{"no proxy-auth-sent", with(func(s *lacRelaySession) { s.ProxyAuthSent = nil }), "no proxy-auth-sent"},
		{"proxy auth not sent", with(func(s *lacRelaySession) { s.ProxyAuthSent = flag(false) }), "proxy-auth-sent false"},
	} {
		verdictWant(t, tc.name, lacRealmSessionVerdict(tc.sessions), tc.want)
	}
}

// TestStrayRefusedVerdict proves the stray subscriber counts as refused only
// when pppd logged the LAC's own local refusal: a failure another party sent
// (an LNS the LAC wrongly relayed to) or a success is refused, and a log with
// no answer yet keeps the poll waiting.
// METHOD: pppd 2.5.5 log lines, as chap_handle_status prints them.
func TestStrayRefusedVerdict(t *testing.T) {
	local := "rcvd [CHAP Failure id=0x1 \"no auth handler configured\"]\nCHAP authentication failed: no auth handler configured\nCHAP authentication failed\n"
	for _, tc := range []struct {
		name string
		log  string
		want string
	}{
		{"local refusal", local, ""},
		{"no answer yet", "rcvd [CHAP Challenge id=0x1 <00>, name = \"ze-lac\"]\n", "no CHAP Failure carrying"},
		{"refused by an LNS", "CHAP authentication failed: Access denied\nCHAP authentication failed\n", "no CHAP Failure carrying"},
		{"admitted", "CHAP authentication succeeded: Access granted\n", "CHAP success"},
		{"admitted after a refusal", local + "CHAP authentication succeeded\n", "CHAP success"},
	} {
		verdictWant(t, tc.name, strayRefusedVerdict(tc.log), tc.want)
	}
}

// TestLNSNeverSawVerdict proves an LNS log naming the stray subscriber in a
// CHAP Response is refused, in both LNS formats the checkers read.
func TestLNSNeverSawVerdict(t *testing.T) {
	accelMarker := `name="` + strayUser + `"`
	pppdMarker := `name = "` + strayUser + `"`
	for _, tc := range []struct {
		name, log, marker, want string
	}{
		{"accel-ppp saw only pppd", `recv [CHAP Response id=1 <ab>, name="` + pppdUser + `"]`, accelMarker, ""},
		{"accel-ppp saw the stray", `recv [CHAP Response id=1 <ab>, name="` + strayUser + `"]`, accelMarker, "ze-lac relayed pppd-stray"},
		{"pppd saw only pppd", `rcvd [CHAP Response id=0x1 <ab>, name = "` + pppdUser + `"]`, pppdMarker, ""},
		{"pppd saw the stray", `rcvd [CHAP Response id=0x1 <ab>, name = "` + strayUser + `"]`, pppdMarker, "ze-lac relayed pppd-stray"},
	} {
		verdictWant(t, tc.name, lnsNeverSawVerdict(tc.log, tc.marker, "lns"), tc.want)
	}
}

// TestXL2TPDLNSVerdict proves the xl2tpd checker needs the call xl2tpd
// established from the LAC's address and, in the LNS pppd's log, the
// subscriber's renegotiation, its CHAP Response naming the user, the
// Success, and the IPCP peer address; dropping any one line is refused.
// METHOD: a complete pair of logs, then one row per removed line.
func TestXL2TPDLNSVerdict(t *testing.T) {
	xl2tpd := "xl2tpd[1]: Call established with 192.0.2.2, PID: 9, Local: 1, Remote: 1, Serial: 1\n"
	lines := []string{
		"sent [LCP ConfReq id=0x1 <mru 1492> <auth chap MD5> <magic 0x1>]",
		"rcvd [LCP ConfReq id=0x2 <mru 1492> <magic 0x2>]",
		"sent [LCP ConfAck id=0x2 <mru 1492> <magic 0x2>]",
		"rcvd [LCP ConfAck id=0x1 <mru 1492> <auth chap MD5> <magic 0x1>]",
		"sent [CHAP Challenge id=0x1 <00>, name = \"xl2tpd-lns\"]",
		"rcvd [CHAP Response id=0x1 <ab>, name = \"" + pppdUser + "\"]",
		"sent [CHAP Success id=0x1 \"Access granted\"]",
		"remote IP address " + subscriberAddress,
	}
	without := func(drop string) string {
		kept := make([]string, 0, len(lines))
		for _, line := range lines {
			if !strings.HasPrefix(line, drop) {
				kept = append(kept, line)
			}
		}
		return strings.Join(kept, "\n")
	}
	full := strings.Join(lines, "\n")
	for _, tc := range []struct {
		name, xl2tpd, pppd, want string
	}{
		{"complete", xl2tpd, full, ""},
		{"no call", "xl2tpd[1]: Listening on IP address 192.0.2.3, port 1701\n", full, "no call established"},
		{"call from elsewhere", "xl2tpd[1]: Call established with 192.0.2.9, PID: 9\n", full, "no call established"},
		{"no renegotiation request", xl2tpd, without("rcvd [LCP ConfReq"), "no LCP Configure-Request"},
		{"no renegotiation ack", xl2tpd, without("rcvd [LCP ConfAck"), "no LCP Configure-Ack"},
		{"no response", xl2tpd, without("rcvd [CHAP Response"), "no CHAP Response naming"},
		{"response from another user", xl2tpd, strings.ReplaceAll(full, pppdUser, strayUser), "no CHAP Response naming"},
		{"no success", xl2tpd, without("sent [CHAP Success"), "no CHAP Success"},
		{"no address", xl2tpd, without("remote IP address"), "no IPCP peer address"},
	} {
		verdictWant(t, tc.name, xl2tpdLNSVerdict(tc.xl2tpd, tc.pppd, pppdUser), tc.want)
	}
}
