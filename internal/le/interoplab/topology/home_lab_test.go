package topology

import (
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

// TestLabValueSubstitution proves substitution replaces both placeholder
// kinds and changes nothing else, and refuses a placeholder with no value, an
// unknown kind and an unterminated one.
func TestLabValueSubstitution(t *testing.T) {
	values := labValues{"pw": "secret-1", "cert": "MIIB"}
	got, err := substitutePlaceholders("a \"<<SECRET:pw>>\";\nb <<LAB:cert>>;\n", values)
	if err != nil {
		t.Fatalf("substitute: %v", err)
	}
	if want := "a \"secret-1\";\nb MIIB;\n"; got != want {
		t.Fatalf("substituted %q, want %q", got, want)
	}
	for _, refused := range []string{"<<SECRET:absent>>", "<<OTHER:pw>>", "x <<SECRET:pw"} {
		if _, err := substitutePlaceholders(refused, values); err == nil {
			t.Errorf("%q was accepted", refused)
		}
	}
	if _, err := parseLabValues("name\n"); err == nil {
		t.Error("a lab-values line with no value was accepted")
	}
	if _, err := parseLabValues("a 1\na 2\n"); err == nil {
		t.Error("a name given twice was accepted")
	}
}

// TestRenderHomeCPE proves the committed conversion renders with no
// placeholder left, the laptop's public key is the X25519 public key of its
// lab private key, and the issued CPE certificate chains to the issued CA
// with the CPE's IKE identity.
func TestRenderHomeCPE(t *testing.T) {
	root := checkoutRoot(t)
	rendered, err := renderHomeCPE(root, time.Now(), cpeKeyP256)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if strings.Contains(rendered.config, placeholderOpen) {
		t.Fatalf("the rendered configuration still holds a placeholder")
	}
	listed, err := os.ReadFile(filepath.Join(root, cpeDirectory, cpeLabValuesFile))
	if err != nil {
		t.Fatalf("read lab-values: %v", err)
	}
	values, err := parseLabValues(string(listed))
	if err != nil {
		t.Fatalf("parse lab-values: %v", err)
	}
	raw, err := base64.StdEncoding.DecodeString(values[wgLaptopPrivate])
	if err != nil {
		t.Fatalf("decode the laptop key: %v", err)
	}
	private, err := ecdh.X25519().NewPrivateKey(raw)
	if err != nil {
		t.Fatalf("laptop key: %v", err)
	}
	public := base64.StdEncoding.EncodeToString(private.PublicKey().Bytes())
	if !strings.Contains(rendered.config, "public-key \""+public+"\"") {
		t.Fatalf("the rendered CPE does not name the laptop's derived public key %s", public)
	}
	issued, err := issueLab(values, time.Now(), cpeKeyP256)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	caDER, err := base64.StdEncoding.DecodeString(issued.values[pkiCACertificate])
	if err != nil {
		t.Fatalf("decode CA: %v", err)
	}
	ca, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatalf("parse CA: %v", err)
	}
	cpeDER, err := base64.StdEncoding.DecodeString(issued.values[pkiCPECertificate])
	if err != nil {
		t.Fatalf("decode CPE certificate: %v", err)
	}
	cpe, err := x509.ParseCertificate(cpeDER)
	if err != nil {
		t.Fatalf("parse CPE certificate: %v", err)
	}
	pool := x509.NewCertPool()
	pool.AddCert(ca)
	if _, err := cpe.Verify(x509.VerifyOptions{Roots: pool, DNSName: cpeIKEName,
		KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageAny}}); err != nil {
		t.Fatalf("the CPE certificate does not chain to the lab CA as %s: %v", cpeIKEName, err)
	}
	if !ca.MaxPathLenZero {
		t.Fatal("the lab CA carries no path length 0")
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

// TestIssueLabCPEKey proves each mgmt-tunnel variant hands the CPE a key of
// its own algorithm (spec-home-cpe-11 AC-9): the issued certificate's public
// key is that algorithm's, the key placeholder parses as Ze's pki leaf parses
// it (SEC 1 for P-256, PKCS #8 for Ed25519) to the private half of that same
// public key, and a scenario that names no algorithm is refused.
func TestIssueLabCPEKey(t *testing.T) {
	values := labValues{}
	for _, name := range []string{wgLaptopPrivate, wgCPEPrivate} {
		values[name] = base64.StdEncoding.EncodeToString(make([]byte, 32))
	}
	values[wgLaptopPSK] = "psk"
	for _, tc := range []struct {
		name      string
		algorithm cpeKeyAlgorithm
		want      x509.PublicKeyAlgorithm
	}{
		{"p256", cpeKeyP256, x509.ECDSA},
		{"ed25519", cpeKeyEd25519, x509.Ed25519},
	} {
		issued, err := issueLab(values, time.Now(), tc.algorithm)
		if err != nil {
			t.Fatalf("%s: issue: %v", tc.name, err)
		}
		certificateDER, err := base64.StdEncoding.DecodeString(issued.values[pkiCPECertificate])
		if err != nil {
			t.Fatalf("%s: decode the CPE certificate: %v", tc.name, err)
		}
		certificate, err := x509.ParseCertificate(certificateDER)
		if err != nil {
			t.Fatalf("%s: parse the CPE certificate: %v", tc.name, err)
		}
		if certificate.PublicKeyAlgorithm != tc.want {
			t.Fatalf("%s: the CPE certificate holds a %v key, want %v", tc.name, certificate.PublicKeyAlgorithm, tc.want)
		}
		keyDER, err := base64.StdEncoding.DecodeString(issued.values[pkiCPEKey])
		if err != nil {
			t.Fatalf("%s: decode the CPE key: %v", tc.name, err)
		}
		if err := keyMatches(keyDER, certificate.PublicKey); err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
	}
	if _, err := issueLab(values, time.Now(), cpeKeyUnspecified); !errors.Is(err, errCPEKeyUnspecified) {
		t.Fatalf("an unspecified CPE key algorithm: err = %v, want errCPEKeyUnspecified", err)
	}
}

// keyMatches parses keyDER the way Ze's pki leaf does and refuses a key that
// is not the private half of public.
func keyMatches(keyDER []byte, public any) error {
	if key, err := x509.ParseECPrivateKey(keyDER); err == nil {
		if !key.PublicKey.Equal(public) {
			return errors.New("the SEC 1 key is not the certificate's")
		}
		return nil
	}
	parsed, err := x509.ParsePKCS8PrivateKey(keyDER)
	if err != nil {
		return errors.New("the key parses neither as SEC 1 nor as PKCS #8")
	}
	switch key := parsed.(type) {
	case ed25519.PrivateKey:
		half, ok := key.Public().(ed25519.PublicKey)
		if !ok {
			return errors.New("the PKCS #8 Ed25519 key answers no Ed25519 public key")
		}
		if !half.Equal(public) {
			return errors.New("the PKCS #8 Ed25519 key is not the certificate's")
		}
		return nil
	case *ecdsa.PrivateKey:
		return errors.New("a P-256 key arrived as PKCS #8, which the responder's EC PRIVATE KEY block cannot carry")
	}
	return errors.New("the PKCS #8 key is neither Ed25519 nor ECDSA")
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
		err := swanAuthVerdict(tc.logs, cpeIKEName, charonEd25519)
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
