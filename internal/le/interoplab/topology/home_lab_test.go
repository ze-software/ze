package topology

import (
	"errors"
	"net/netip"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/test/labconfig/homecpe"
)

// checkoutRoot is the repository root, four levels above this package.
func checkoutRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", "..", ".."))
	if err != nil {
		t.Fatalf("resolve the checkout root: %v", err)
	}
	return root
}

// TestConversionRecordCoversSource proves the committed record names every
// line of the committed VyOS source, and that the check refuses a source line
// changed without its row, a missing row, an extra row and a dropped row with
// no reason. Method: the real files, then one mutation each.
func TestConversionRecordCoversSource(t *testing.T) {
	if err := conversionRecordCovers(checkoutRoot(t)); err != nil {
		t.Fatalf("the committed record does not cover the committed source:\n%v", err)
	}
	source := "set a 'one'\nset b 'two'\n"
	record := "| # | VyOS | Ze | Class | Note |\n|---|---|---|---|---|\n" +
		"| 1 | `set a 'one'` | `a one` | equivalent | |\n| 2 | `set b 'two'` | dropped: no form | - | |\n"
	rows, err := parseConversionRecord(record)
	if err != nil {
		t.Fatalf("parse a well-formed record: %v", err)
	}
	if err := recordCoverage(source, rows); err != nil {
		t.Fatalf("a covering record was refused: %v", err)
	}
	cases := map[string]struct{ source, record string }{
		"changed line": {"set a 'one'\nset b 'three'\n", record},
		"missing row":  {source + "set c 'four'\n", record},
		"extra row":    {"set a 'one'\n", record},
		"no reason":    {source, strings.Replace(record, "dropped: no form", "dropped:", 1)},
		"no Ze form":   {source, strings.Replace(record, "`a one`", "", 1)},
	}
	for name, mutated := range cases {
		rows, err := parseConversionRecord(mutated.record)
		if err != nil {
			t.Fatalf("%s: parse: %v", name, err)
		}
		if recordCoverage(mutated.source, rows) == nil {
			t.Errorf("%s: the coverage check accepted it", name)
		}
	}
}

// TestHomeCPEScenariosRunTheConvertedFile proves AC-1's last clause: every
// declared scenario with a ze-cpe node runs the rendered conversion, except
// the exempt pppoe-mss-pmtu: child 19's access-up, access-pool,
// access-accounting and child 15's access-wholesale included, which declare
// their CPE with convertedCPE() through accessChain.
func TestHomeCPEScenariosRunTheConvertedFile(t *testing.T) {
	if err := homeCPEScenariosConverted(); err != nil {
		t.Fatalf("a home-CPE scenario runs a CPE file of its own:\n%v", err)
	}
	for _, name := range []string{conversionScenario, lanServicesScenario, roadWarriorScenario, mgmtTunnelScenario, mgmtTunnelEd25519Scenario, accessReconnectScenario} {
		declared := scenarios[name]
		if declared.prepare == nil {
			t.Errorf("%s has no prepare step", name)
		}
	}
}

// TestMSSVerdict judges AC-5 from capture lines: the lowering clause (1460
// arrives as 1400) and the no-raise clause (1300 arrives as 1300) separately,
// with the boundary 1399, 1400 and 1401 of the spec's table.
func TestMSSVerdict(t *testing.T) {
	line := func(port string, mss int) string {
		return "12:00:00.000000 IP 203.0.113.1.40000 > 198.51.100.10." + port +
			": Flags [S], seq 1, win 64240, options [mss " + decimal(uint16(mss)) + ",sackOK], length 0\n"
	}
	cases := []struct {
		name    string
		capture string
		probe   mssProbe
		ok      bool
	}{
		{"lowered", line("9101", 1400), mssProbe{offered: 1460, port: 9101, want: 1400}, true},
		{"not lowered", line("9101", 1460), mssProbe{offered: 1460, port: 9101, want: 1400}, false},
		{"left alone", line("9102", 1300), mssProbe{offered: 1300, port: 9102, want: 1300}, true},
		{"raised", line("9102", 1400), mssProbe{offered: 1300, port: 9102, want: 1300}, false},
		{"1399 kept", line("9103", 1399), mssProbe{offered: 1399, port: 9103, want: 1399}, true},
		{"1400 kept", line("9104", 1400), mssProbe{offered: 1400, port: 9104, want: 1400}, true},
		{"1401 lowered", line("9105", 1400), mssProbe{offered: 1401, port: 9105, want: 1400}, true},
	}
	for _, tc := range cases {
		err := synMSSVerdict(tc.capture, webAddress+"."+decimal(tc.probe.port)+":", tc.probe)
		if (err == nil) != tc.ok {
			t.Errorf("%s: verdict %v, want ok=%v", tc.name, err, tc.ok)
		}
	}
	if err := synMSSVerdict("", webAddress+".9101:", lanMSSProbes[0]); !errors.Is(err, errNoProbeSYN) {
		t.Errorf("an empty capture gave %v, want errNoProbeSYN", err)
	}
}

// TestSYNSourceVerdictWaitsOnAnEmptyCapture proves synSourceVerdict tells a
// capture that holds no SYN yet (errNoSourceSYN, which waitWebSYNFrom polls
// past) from one whose SYN came from the wrong source (a final refusal), and
// accepts the masqueraded one.
// PREVENTS: reading the capture once, the moment curl returned, before
// tcpdump received the SYN from the kernel.
func TestSYNSourceVerdictWaitsOnAnEmptyCapture(t *testing.T) {
	target := webAddress + ".80:"
	syn := func(source string) string {
		return "00:42:30.100000 IP " + source + ".40010 > " + target + " Flags [S], seq 1, win 64240, length 0"
	}
	if err := synSourceVerdict("", target, subscriberAddress); !errors.Is(err, errNoSourceSYN) {
		t.Errorf("an empty capture gave %v, want errNoSourceSYN", err)
	}
	if err := synSourceVerdict(syn("192.168.50.20"), target, subscriberAddress); err == nil || errors.Is(err, errNoSourceSYN) {
		t.Errorf("an unmasqueraded SYN gave %v, want a final refusal", err)
	}
	if err := synSourceVerdict(syn(subscriberAddress), target, subscriberAddress); err != nil {
		t.Errorf("the masqueraded SYN gave %v, want accepted", err)
	}
}

// TestReservationVerdict judges AC-2 and AC-3: a range lease inside
// 192.168.50.100-199, and each reservation exact, the second one in the later
// subnet (vacuity trap 3).
func TestReservationVerdict(t *testing.T) {
	for address, ok := range map[string]bool{"192.168.50.100": true, "192.168.50.199": true,
		"192.168.50.99": false, "192.168.50.200": false} {
		if (inRange(netip.MustParseAddr(address)) == nil) != ok {
			t.Errorf("inRange(%s) disagrees, want ok=%v", address, ok)
		}
	}
	if exactly(resBAddress)(netip.MustParseAddr(resBAddress)) != nil {
		t.Error("the reserved 10.99.0.40 was refused")
	}
	if exactly(resBAddress)(netip.MustParseAddr("192.168.50.100")) == nil {
		t.Error("a first-subnet range lease passed as res-b's reservation")
	}
}

// TestDropCounter reads the default-drop term's counter from nft JSON and
// refuses a ruleset without that term or without its counter.
func TestDropCounter(t *testing.T) {
	ruleset := `{"nftables":[{"metainfo":{}},{"rule":{"expr":[{"counter":{"packets":7,"bytes":420}},` +
		`{"log":{"prefix":"[ipv4-input-default-D]"}},{"drop":null}]}}]}`
	count, err := dropCounter(ruleset, defaultDropPrefix)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if count != 7 {
		t.Fatalf("read %d, want 7", count)
	}
	if _, err := dropCounter(`{"nftables":[]}`, defaultDropPrefix); err == nil {
		t.Error("a ruleset with no default term was accepted")
	}
	if _, err := dropCounter(`{"nftables":[{"rule":{"expr":[{"log":{"prefix":"[ipv4-input-default-D]"}}]}}]}`, defaultDropPrefix); err == nil {
		t.Error("a default term with no counter was read as zero")
	}
}

// TestSwanSAVerdict judges AC-9 and AC-11 from a swanctl --list-sas answer:
// established on 4500 at the CPE address, narrowed to ::/0, and refused on
// another address, on 500, or with the IPv4 selector kept.
func TestSwanSAVerdict(t *testing.T) {
	listed := "cpe: #1, ESTABLISHED, IKEv2, 1_i 2_r*\n" +
		"  local  'vpn.example.net' @ 198.51.100.30[4500]\n" +
		"  remote 'cpe.lab.example.net' @ 203.0.113.1[4500]\n" +
		"  mgmt: #1, reqid 1, INSTALLED, TUNNEL, ESP:AES_GCM_16-128\n" +
		"    local  ::/0\n    remote ::/0\n"
	if err := swanSAVerdict(parseSwanSA(listed), subscriberAddress); err != nil {
		t.Fatalf("a correct SA was refused: %v", err)
	}
	if swanSAVerdict(parseSwanSA(listed), reconnectAddress) == nil {
		t.Error("an SA on the old address passed for the new one")
	}
	if swanSAVerdict(parseSwanSA(strings.ReplaceAll(listed, "[4500]", "[500]")), subscriberAddress) == nil {
		t.Error("an SA on port 500 passed")
	}
	if swanSAVerdict(parseSwanSA(strings.Replace(listed, "local  ::/0", "local  0.0.0.0/0 ::/0", 1)), subscriberAddress) == nil {
		t.Error("an unnarrowed selector passed")
	}
}

// TestParseXFRMBytes reads per-direction bytes keyed by source and
// destination, and refuses two states in one direction (vacuity trap 5).
func TestParseXFRMBytes(t *testing.T) {
	state := "src 203.0.113.1 dst 198.51.100.30\n\tproto esp spi 0x1 reqid 1 mode tunnel\n\tlifetime current:\n\t  840(bytes), 5(packets)\n" +
		"src 198.51.100.30 dst 203.0.113.1\n\tproto esp spi 0x2 reqid 1 mode tunnel\n\tlifetime current:\n\t  0(bytes), 0(packets)\n"
	counters, err := parseXFRMBytes(state)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if counters[[2]string{subscriberAddress, swanAddress}] != 840 {
		t.Fatalf("outbound bytes %v", counters)
	}
	if advancedVerdict(counters, counters, [2]string{swanAddress, subscriberAddress}) == nil {
		t.Error("a direction that did not advance passed")
	}
	if _, err := parseXFRMBytes(state + strings.SplitN(state, "src 198", 2)[0]); err == nil {
		t.Error("two states in one direction were accepted")
	}
}

// TestWireGuardDumpVerdict judges the CPE's kernel readback of AC-8.
func TestWireGuardDumpVerdict(t *testing.T) {
	dump := "priv\tpub\t51820\toff\nLAPTOP\tPSK\t203.0.113.9:4000\t10.88.0.2/32\t0\t0\t0\toff\n"
	if err := wireGuardDumpVerdict(dump, "LAPTOP"); err != nil {
		t.Fatalf("a correct dump was refused: %v", err)
	}
	if wireGuardDumpVerdict(strings.Replace(dump, "\tPSK\t", "\t(none)\t", 1), "LAPTOP") == nil {
		t.Error("a peer with no preshared key passed")
	}
	if wireGuardDumpVerdict(dump, "OTHER") == nil {
		t.Error("another peer key passed")
	}
}

// TestSwanAuthVerdict proves the Ed25519 variant needs strongSwan's own
// account of an Ed25519 verification of the CPE: the ECDSA line the P-256
// variant leaves, and another identity's Ed25519 line, are refused.
func TestSwanAuthVerdict(t *testing.T) {
	const ed = "12[IKE] authentication of 'cpe.lab.example.net' with ED25519 successful\n"
	for _, tc := range []struct {
		name string
		logs string
		want string
	}{
		{"ed25519", "10[IKE] received cert\n" + ed, ""},
		{"ecdsa", "12[IKE] authentication of 'cpe.lab.example.net' with ECDSA_WITH_SHA256_DER successful\n", "did not verify"},
		{"other identity", "12[IKE] authentication of 'vpn.example.net' with ED25519 successful\n", "did not verify"},
		{"empty", "", "did not verify"},
	} {
		err := swanAuthVerdict(tc.logs, homecpe.CPEIdentity, charonEd25519)
		if tc.want == "" {
			if err != nil {
				t.Errorf("%s: %v", tc.name, err)
			}
			continue
		}
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err = %v, want %q", tc.name, err, tc.want)
		}
	}
}
