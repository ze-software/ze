package ipsec

import (
	"net"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/secret"
	"github.com/ze-software/ze/internal/component/ike/dataplane"
)

// makeESPTree builds a config tree with vpn { ipsec { esp-group <name> { ... } } }.
func makeESPTree(name, lifetime, pfs string, proposals map[string][2]string) *config.Tree {
	tree := config.NewTree()
	vpn := tree.GetOrCreateContainer("vpn")
	ipsec := vpn.GetOrCreateContainer("ipsec")

	espEntry := config.NewTree()
	if lifetime != "" {
		espEntry.Set("lifetime", lifetime)
	}
	if pfs != "" {
		espEntry.Set("pfs", pfs)
	}
	for num, encHash := range proposals {
		prop := config.NewTree()
		prop.Set("encryption", encHash[0])
		if encHash[1] != "" {
			prop.Set("hash", encHash[1])
		}
		espEntry.AddListEntry("proposal", num, prop)
	}
	ipsec.AddListEntry("esp-group", name, espEntry)

	return tree
}

// makeIKETree builds a config tree with vpn { ipsec { ike-group <name> { ... } } }.
func makeIKETree(name string, opts ikeOpts) *config.Tree {
	tree := config.NewTree()
	vpn := tree.GetOrCreateContainer("vpn")
	ipsec := vpn.GetOrCreateContainer("ipsec")

	ikeEntry := config.NewTree()
	if opts.keyExchange != "" {
		ikeEntry.Set("key-exchange", opts.keyExchange)
	}
	if opts.lifetime != "" {
		ikeEntry.Set("lifetime", opts.lifetime)
	}
	if opts.closeAction != "" {
		ikeEntry.Set("close-action", opts.closeAction)
	}
	if opts.dpdAction != "" || opts.dpdInterval != "" || opts.dpdTimeout != "" {
		dpd := ikeEntry.GetOrCreateContainer("dead-peer-detection")
		if opts.dpdAction != "" {
			dpd.Set("action", opts.dpdAction)
		}
		if opts.dpdInterval != "" {
			dpd.Set("interval", opts.dpdInterval)
		}
		if opts.dpdTimeout != "" {
			dpd.Set("timeout", opts.dpdTimeout)
		}
	}
	for num, p := range opts.proposals {
		prop := config.NewTree()
		prop.Set("encryption", p.encryption)
		prop.Set("hash", p.hash)
		prop.Set("dh-group", p.dhGroup)
		ikeEntry.AddListEntry("proposal", num, prop)
	}
	ipsec.AddListEntry("ike-group", name, ikeEntry)

	return tree
}

type ikeOpts struct {
	keyExchange string
	lifetime    string
	closeAction string
	dpdAction   string
	dpdInterval string
	dpdTimeout  string
	proposals   map[string]ikeProposalOpts
}

type ikeProposalOpts struct {
	encryption string
	hash       string
	dhGroup    string
}

func makePeerTree(peerName string, opts peerOpts) *config.Tree {
	tree := config.NewTree()
	vpn := tree.GetOrCreateContainer("vpn")
	ipsec := vpn.GetOrCreateContainer("ipsec")

	if opts.espGroupName != "" {
		espEntry := config.NewTree()
		prop := config.NewTree()
		prop.Set("encryption", "aes128gcm")
		espEntry.AddListEntry("proposal", "1", prop)
		ipsec.AddListEntry("esp-group", opts.espGroupName, espEntry)
	}
	if opts.ikeGroupName != "" {
		ikeEntry := config.NewTree()
		prop := config.NewTree()
		prop.Set("encryption", "aes128gcm")
		prop.Set("hash", "sha256")
		prop.Set("dh-group", "14")
		ikeEntry.AddListEntry("proposal", "1", prop)
		ipsec.AddListEntry("ike-group", opts.ikeGroupName, ikeEntry)
	}

	sts := ipsec.GetOrCreateContainer("site-to-site")
	peerEntry := config.NewTree()
	if opts.ikeGroup != "" {
		peerEntry.Set("ike-group", opts.ikeGroup)
	}
	if opts.espGroup != "" {
		peerEntry.Set("esp-group", opts.espGroup)
	}
	if opts.connType != "" {
		peerEntry.Set("connection-type", opts.connType)
	}
	if opts.localAddr != "" {
		peerEntry.Set("local-address", opts.localAddr)
	}
	if opts.remoteAddr != "" {
		peerEntry.Set("remote-address", opts.remoteAddr)
	}

	auth := peerEntry.GetOrCreateContainer("authentication")
	if opts.authMode != "" {
		auth.Set("mode", opts.authMode)
	}
	if opts.psk != "" {
		auth.Set("pre-shared-secret", opts.psk)
	}
	if opts.pskEncoding != "" {
		auth.Set("pre-shared-secret-encoding", opts.pskEncoding)
	}
	if opts.localID != "" {
		auth.Set("local-id", opts.localID)
	}
	if opts.remoteID != "" {
		auth.Set("remote-id", opts.remoteID)
	}
	if opts.caCert != "" || opts.cert != "" {
		x509 := auth.GetOrCreateContainer("x509")
		if opts.caCert != "" {
			x509.Set("ca-certificate", opts.caCert)
		}
		if opts.cert != "" {
			x509.Set("certificate", opts.cert)
		}
	}

	if opts.vtiBind != "" {
		vti := peerEntry.GetOrCreateContainer("vti")
		vti.Set("bind", opts.vtiBind)
	}

	sts.AddListEntry("peer", peerName, peerEntry)

	return tree
}

type peerOpts struct {
	ikeGroupName string // define this IKE group in the tree
	espGroupName string // define this ESP group in the tree
	ikeGroup     string // peer references this IKE group
	espGroup     string // peer references this ESP group
	connType     string
	localAddr    string
	remoteAddr   string
	authMode     string
	psk          string
	pskEncoding  string // pre-shared-secret-encoding; empty leaves the leaf absent
	localID      string
	remoteID     string
	caCert       string
	cert         string
	vtiBind      string
}

func TestParseESPGroup(t *testing.T) {
	// proposal 10 dropped its hash. parseESPProposal now refuses a hash
	// beside an AEAD cipher, so the old fixture asserted a config shape that is no
	// longer valid (TestParseESPProposalRejectsHashBesideAEAD proves the refusal). No
	// assertion is removed or weakened: the two proposals, their numbers and the
	// aes128gcm reading are all still checked below.
	tree := makeESPTree("ESP-RW", "86400", "disable", map[string][2]string{
		"10": {"aes128gcm", ""},
		"20": {"aes256", "sha512"},
	})

	cfg, err := ParseIPsecConfig(tree)
	if err != nil {
		t.Fatalf("ParseIPsecConfig: %v", err)
	}

	g, ok := cfg.ESPGroups["ESP-RW"]
	if !ok {
		t.Fatal("ESP group ESP-RW not found")
	}
	if g.Lifetime != 86400 {
		t.Errorf("lifetime = %d, want 86400", g.Lifetime)
	}
	if g.PFS != PFSDisable {
		t.Errorf("pfs = %s, want disable", g.PFS)
	}
	if len(g.Proposals) != 2 {
		t.Fatalf("proposals count = %d, want 2", len(g.Proposals))
	}
	if g.Proposals[0].Number != 10 {
		t.Errorf("first proposal number = %d, want 10", g.Proposals[0].Number)
	}
	if g.Proposals[0].Encryption != EncryptionAES128GCM {
		t.Errorf("first proposal encryption = %s, want aes128gcm", g.Proposals[0].Encryption)
	}
	if g.Proposals[1].Number != 20 {
		t.Errorf("second proposal number = %d, want 20", g.Proposals[1].Number)
	}
}

func TestParseESPGroupDefaults(t *testing.T) {
	tree := makeESPTree("DEFAULT", "", "", map[string][2]string{
		"1": {"aes128", "sha256"},
	})

	cfg, err := ParseIPsecConfig(tree)
	if err != nil {
		t.Fatalf("ParseIPsecConfig: %v", err)
	}

	g := cfg.ESPGroups["DEFAULT"]
	if g.Lifetime != 3600 {
		t.Errorf("default lifetime = %d, want 3600", g.Lifetime)
	}
	if g.PFS != PFSEnable {
		t.Errorf("default pfs = %s, want enable", g.PFS)
	}
}

func TestParseIKEGroup(t *testing.T) {
	tree := makeIKETree("IKE-RW", ikeOpts{
		keyExchange: "ikev2",
		lifetime:    "0",
		closeAction: "start",
		dpdAction:   "restart",
		dpdInterval: "10",
		dpdTimeout:  "30",
		proposals: map[string]ikeProposalOpts{
			"10": {encryption: "aes128gcm", hash: "sha256", dhGroup: "14"},
		},
	})

	cfg, err := ParseIPsecConfig(tree)
	if err != nil {
		t.Fatalf("ParseIPsecConfig: %v", err)
	}

	g, ok := cfg.IKEGroups["IKE-RW"]
	if !ok {
		t.Fatal("IKE group IKE-RW not found")
	}
	if g.KeyExchange != KeyExchangeIKEv2 {
		t.Errorf("key-exchange = %s, want ikev2", g.KeyExchange)
	}
	if g.Lifetime != 0 {
		t.Errorf("lifetime = %d, want 0", g.Lifetime)
	}
	if g.CloseAction != CloseActionStart {
		t.Errorf("close-action = %s, want start", g.CloseAction)
	}
	if len(g.Proposals) != 1 {
		t.Fatalf("proposals count = %d, want 1", len(g.Proposals))
	}
	if g.Proposals[0].DHGroup != 14 {
		t.Errorf("dh-group = %d, want 14", g.Proposals[0].DHGroup)
	}
}

func TestParseIKEGroupDPD(t *testing.T) {
	tree := makeIKETree("DPD-TEST", ikeOpts{
		dpdAction:   "restart",
		dpdInterval: "10",
		dpdTimeout:  "30",
		proposals: map[string]ikeProposalOpts{
			"1": {encryption: "aes256", hash: "sha512", dhGroup: "19"},
		},
	})

	cfg, err := ParseIPsecConfig(tree)
	if err != nil {
		t.Fatalf("ParseIPsecConfig: %v", err)
	}

	g := cfg.IKEGroups["DPD-TEST"]
	if g.DPD.Action != DPDActionRestart {
		t.Errorf("dpd action = %s, want restart", g.DPD.Action)
	}
	if g.DPD.Interval != 10 {
		t.Errorf("dpd interval = %d, want 10", g.DPD.Interval)
	}
	if g.DPD.Timeout != 30 {
		t.Errorf("dpd timeout = %d, want 30", g.DPD.Timeout)
	}
}

func TestParseSiteToSitePeerX509(t *testing.T) {
	tree := makePeerTree("mgmt-bridge", peerOpts{
		ikeGroupName: "IKE-RW",
		espGroupName: "ESP-RW",
		ikeGroup:     "IKE-RW",
		espGroup:     "ESP-RW",
		connType:     "initiate",
		remoteAddr:   "mgmt.example.com",
		authMode:     "x509",
		localID:      "DEVICE001",
		remoteID:     "mgmt-bridge",
		caCert:       "exa-vpn-ca",
		cert:         "DEVICE001",
		vtiBind:      "vti0",
	})

	cfg, err := ParseIPsecConfig(tree)
	if err != nil {
		t.Fatalf("ParseIPsecConfig: %v", err)
	}

	peer, ok := cfg.Peers["mgmt-bridge"]
	if !ok {
		t.Fatal("peer mgmt-bridge not found")
	}
	if peer.Auth.Mode != AuthX509 {
		t.Errorf("auth mode = %s, want x509", peer.Auth.Mode)
	}
	if peer.Auth.CACertificate != "exa-vpn-ca" {
		t.Errorf("ca-certificate = %q, want exa-vpn-ca", peer.Auth.CACertificate)
	}
	if peer.Auth.Certificate != "DEVICE001" {
		t.Errorf("certificate = %q, want DEVICE001", peer.Auth.Certificate)
	}
	if peer.Auth.LocalID != "DEVICE001" {
		t.Errorf("local-id = %q, want DEVICE001", peer.Auth.LocalID)
	}
	if peer.Auth.RemoteID != "mgmt-bridge" {
		t.Errorf("remote-id = %q, want mgmt-bridge", peer.Auth.RemoteID)
	}
	if peer.VTIBind != "vti0" {
		t.Errorf("vti bind = %q, want vti0", peer.VTIBind)
	}
	if peer.ConnectionType != ConnectionInitiate {
		t.Errorf("connection-type = %s, want initiate", peer.ConnectionType)
	}
}

func TestParseSiteToSitePeerPSK(t *testing.T) {
	tree := makePeerTree("psk-peer", peerOpts{
		ikeGroupName: "IKE-RW",
		espGroupName: "ESP-RW",
		ikeGroup:     "IKE-RW",
		espGroup:     "ESP-RW",
		authMode:     "pre-shared-secret",
		psk:          "mysecretkey",
	})

	cfg, err := ParseIPsecConfig(tree)
	if err != nil {
		t.Fatalf("ParseIPsecConfig: %v", err)
	}

	peer := cfg.Peers["psk-peer"]
	if peer.Auth.Mode != AuthPreSharedSecret {
		t.Errorf("auth mode = %s, want pre-shared-secret", peer.Auth.Mode)
	}
	if peer.Auth.PSK != "mysecretkey" {
		t.Errorf("psk = %q, want mysecretkey", peer.Auth.PSK)
	}
}

func TestParseSiteToSitePeerVTI(t *testing.T) {
	tree := makePeerTree("vti-peer", peerOpts{
		ikeGroupName: "IKE-RW",
		espGroupName: "ESP-RW",
		ikeGroup:     "IKE-RW",
		espGroup:     "ESP-RW",
		authMode:     "x509",
		vtiBind:      "vti42",
	})

	cfg, err := ParseIPsecConfig(tree)
	if err != nil {
		t.Fatalf("ParseIPsecConfig: %v", err)
	}

	if cfg.Peers["vti-peer"].VTIBind != "vti42" {
		t.Errorf("vti bind = %q, want vti42", cfg.Peers["vti-peer"].VTIBind)
	}
}

func TestParseInvalidEncryption(t *testing.T) {
	tree := makeESPTree("BAD", "", "", map[string][2]string{
		"1": {"des", "sha256"},
	})
	_, err := ParseIPsecConfig(tree)
	if err == nil {
		t.Fatal("expected error for unsupported encryption 'des'")
	}
	if !strings.Contains(err.Error(), "unsupported encryption algorithm") {
		t.Errorf("error = %q, want to contain 'unsupported encryption algorithm'", err)
	}
	if !strings.Contains(err.Error(), "des") {
		t.Errorf("error = %q, want to name the algorithm 'des'", err)
	}
}

func TestParseInvalidDHGroup(t *testing.T) {
	for _, dh := range []string{"0", "99"} {
		tree := makeIKETree("BAD", ikeOpts{
			proposals: map[string]ikeProposalOpts{
				"1": {encryption: "aes128", hash: "sha256", dhGroup: dh},
			},
		})
		_, err := ParseIPsecConfig(tree)
		if err == nil {
			t.Fatalf("expected error for DH group %s", dh)
		}
		if !strings.Contains(err.Error(), "DH group") {
			t.Errorf("dh=%s: error = %q, want to contain 'DH group'", dh, err)
		}
	}
}

func TestParseMissingGroupRef(t *testing.T) {
	tree := makePeerTree("bad-ref", peerOpts{
		ikeGroup: "NONEXISTENT",
		espGroup: "ALSO-MISSING",
		authMode: "x509",
	})

	cfg, err := ParseIPsecConfig(tree)
	if err != nil {
		t.Fatalf("ParseIPsecConfig: %v", err)
	}

	err = cfg.ValidateGroupRefs()
	if err == nil {
		t.Fatal("expected error for missing group reference")
	}
	if !strings.Contains(err.Error(), "not defined") {
		t.Errorf("error = %q, want to contain 'not defined'", err)
	}
}

func TestParseInvalidInterfaceRef(t *testing.T) {
	tree := config.NewTree()
	vpn := tree.GetOrCreateContainer("vpn")
	ipsec := vpn.GetOrCreateContainer("ipsec")
	ipsec.Set("interface", "ethXX")

	cfg, err := ParseIPsecConfig(tree)
	if err != nil {
		t.Fatalf("ParseIPsecConfig: %v", err)
	}

	err = cfg.ValidateInterfaceRef(func(name string) bool {
		return name == "eth0" || name == "pppoe0"
	})
	if err == nil {
		t.Fatal("expected error for interface ethXX")
	}
	if !strings.Contains(err.Error(), "ethXX") {
		t.Errorf("error = %q, want to contain 'ethXX'", err)
	}
}

// makeReloadPeerTree builds a whole config tree carrying one peer, its groups, and the
// members SiteToSitePeer.Equal has to compare through a pointer or a slice: two
// traffic-selector rows, and (for x509) a certificate-url-allow leaf-list.
//
// It does NOT go through makePeerTree. That helper is shared with rfc7296_test.go, whose
// file header carries an RFC-requirement tag outside every func span, and `tag_scope`
// (scripts/dev/rfc_tagged_scope.py) widens tagged-test scope to the WHOLE file when a tag
// sits there. Growing that helper's peerOpts to carry these leaves would drag an
// RFC-tagged file into a change that has nothing to do with RFC 7296 Section 2.15, and the
// commit would owe a test/rfc-changed.md row for it. A local builder costs less.
func makeReloadPeerTree(peerName, authMode string) *config.Tree {
	tree := config.NewTree()
	ipsec := tree.GetOrCreateContainer("vpn").GetOrCreateContainer("ipsec")

	espEntry := config.NewTree()
	espProp := config.NewTree()
	espProp.Set("encryption", "aes128gcm")
	espEntry.AddListEntry("proposal", "1", espProp)
	ipsec.AddListEntry("esp-group", "ESP-1", espEntry)

	ikeEntry := config.NewTree()
	ikeProp := config.NewTree()
	ikeProp.Set("encryption", "aes128gcm")
	ikeProp.Set("hash", "sha256")
	ikeProp.Set("dh-group", "14")
	ikeEntry.AddListEntry("proposal", "1", ikeProp)
	ipsec.AddListEntry("ike-group", "IKE-1", ikeEntry)

	peerEntry := config.NewTree()
	peerEntry.Set("ike-group", "IKE-1")
	peerEntry.Set("esp-group", "ESP-1")
	peerEntry.Set("connection-type", "respond")
	peerEntry.Set("local-address", "192.0.2.10")
	peerEntry.Set("remote-address", "192.0.2.1")

	auth := peerEntry.GetOrCreateContainer("authentication")
	auth.Set("mode", authMode)
	auth.Set("local-id", "ze.example.net")
	auth.Set("remote-id", "peer.example.net")
	switch authMode {
	case "pre-shared-secret":
		auth.Set("pre-shared-secret", "reload-fixture-secret")
	default:
		x509 := auth.GetOrCreateContainer("x509")
		x509.Set("ca-certificate", "test-ca")
		x509.Set("certificate", "test-cert")
		// certificate-url-allow is the one []netip.Prefix a peer config holds, and
		// parseAuthPolicy refuses it unless hash-and-url is on and certificate-url is
		// set, so the three leaves travel together.
		auth.Set("hash-and-url", "true")
		auth.Set("certificate-url", "http://cert.example.net/ze.der")
		auth.AppendValue("certificate-url-allow", "203.0.113.0/24")
		auth.AppendValue("certificate-url-allow", "198.51.100.0/24")
	}

	peerEntry.GetOrCreateContainer("vti").Set("bind", "vti0")

	first := config.NewTree()
	first.GetOrCreateContainer("local").Set("prefix", "10.1.0.0/16")
	first.GetOrCreateContainer("remote").Set("prefix", "10.2.0.0/16")
	peerEntry.AddListEntry("traffic-selector", "1", first)

	second := config.NewTree()
	second.Set("protocol", "6")
	secondLocal := second.GetOrCreateContainer("local")
	secondLocal.Set("prefix", "10.3.0.0/24")
	secondLocal.Set("port", "179")
	second.GetOrCreateContainer("remote").Set("prefix", "10.4.0.0/24")
	peerEntry.AddListEntry("traffic-selector", "2", second)

	ipsec.GetOrCreateContainer("site-to-site").AddListEntry("peer", peerName, peerEntry)
	return tree
}

// VALIDATES: SiteToSitePeer.Equal is TOTAL and still answers "nothing changed" for a
// reload that edited nothing. The two peers come from two independent parses of the same
// configuration, which is exactly what a reload hands the comparison, so every member
// travels through the parser rather than through a struct copy: the *net.IPNet in each
// traffic selector and the []netip.Prefix in the auth policy are equal VALUES behind
// different pointers and different backing arrays.
//
// Both authentication modes run, because they populate DIFFERENT members. x509 is the
// only one that reaches certificate-url-allow, and pre-shared-secret is the mode most
// deployments use, so an instability in the secret's own decode path would otherwise be
// invisible here.
//
// PREVENTS: R-1. A comparison over a whole struct restarts every peer on every commit if
// any member is unstable across a parse, and a tunnel that bounces whenever an operator
// commits anything is a worse defect than the one the totality removes. It also prevents a
// later parser change from reintroducing that instability: a map walk feeding a slice
// member reddens this test the day it lands.
func TestSiteToSitePeerEqualAcrossTwoParses(t *testing.T) {
	for _, authMode := range []string{"x509", "pre-shared-secret"} {
		t.Run(authMode, func(t *testing.T) {
			parse := func() SiteToSitePeer {
				t.Helper()
				cfg, err := ParseIPsecConfig(makeReloadPeerTree("branch-1", authMode))
				if err != nil {
					t.Fatalf("ParseIPsecConfig: %v", err)
				}
				peer, ok := cfg.Peers["branch-1"]
				if !ok {
					t.Fatal("peer branch-1 missing from the parsed config")
				}
				return peer
			}

			first := parse()
			second := parse()

			if len(first.TrafficSelectors) != 2 {
				t.Fatalf("setup: parsed %d traffic selectors, want 2; the fixture must exercise the pointer-bearing member",
					len(first.TrafficSelectors))
			}
			if authMode == "x509" && len(first.Auth.CertificateURLAllow) != 2 {
				t.Fatalf("setup: parsed %d allowed prefixes, want 2; the fixture must exercise the slice member",
					len(first.Auth.CertificateURLAllow))
			}
			if authMode == "pre-shared-secret" && first.Auth.PSK == "" {
				t.Fatal("setup: the pre-shared secret did not reach the parsed peer")
			}
			if !first.Equal(second) {
				t.Errorf("two parses of one configuration compared unequal, so every commit would restart this peer:\nfirst  = %+v\nsecond = %+v",
					first, second)
			}
		})
	}
}

// makeTwoProposalGroupTree is one ike-group and one esp-group, each offering the same TWO
// proposals, added to the tree in the order `numbers` names. Two is the smallest number
// that can show an ORDER, and the order is what TestGroupsEqualWhateverOrderTheyArriveIn
// is about.
func makeTwoProposalGroupTree(numbers [2]string) *config.Tree {
	encryption := map[string]string{"1": "aes256", "2": "aes128"}

	tree := config.NewTree()
	ipsecTree := tree.GetOrCreateContainer("vpn").GetOrCreateContainer("ipsec")

	espEntry := config.NewTree()
	ikeEntry := config.NewTree()
	for _, number := range numbers {
		espProp := config.NewTree()
		espProp.Set("encryption", encryption[number])
		espProp.Set("hash", "sha256")
		espEntry.AddListEntry("proposal", number, espProp)

		ikeProp := config.NewTree()
		ikeProp.Set("encryption", encryption[number])
		ikeProp.Set("hash", "sha256")
		ikeProp.Set("dh-group", "14")
		ikeEntry.AddListEntry("proposal", number, ikeProp)
	}
	ipsecTree.AddListEntry("esp-group", "ESP-1", espEntry)
	ipsecTree.AddListEntry("ike-group", "IKE-1", ikeEntry)

	return tree
}

// VALIDATES: A-1 for the OTHER half of the reload guard. peerConfigChanged
// (engine/reconcile.go) compares the RESOLVED ike-group and esp-group as well as the peer,
// and both comparisons are total. A group that does not parse to one stable value would
// restart every peer naming it on every commit, which is a worse defect than the one the
// total comparison removes.
//
// PREVENTS: the ordering trap. Proposals is a SLICE and reflect.DeepEqual is
// order-sensitive, so the parse must derive the order from the proposal NUMBER and never
// from the order the entries arrived in. parseIKEGroup and parseESPGroup sort on Number for
// that reason; remove either sort and this test reddens. The two trees below declare one
// pair of proposals in opposite order, which is what an operator's edit to an unrelated
// leaf can do to the delivery.
func TestGroupsEqualWhateverOrderTheyArriveIn(t *testing.T) {
	parse := func(numbers [2]string) (IKEGroup, ESPGroup) {
		t.Helper()
		cfg, err := ParseIPsecConfig(makeTwoProposalGroupTree(numbers))
		if err != nil {
			t.Fatalf("ParseIPsecConfig: %v", err)
		}
		return cfg.IKEGroups["IKE-1"], cfg.ESPGroups["ESP-1"]
	}

	firstIKE, firstESP := parse([2]string{"1", "2"})
	secondIKE, secondESP := parse([2]string{"2", "1"})

	if len(firstIKE.Proposals) != 2 || len(firstESP.Proposals) != 2 {
		t.Fatalf("setup: parsed %d IKE and %d ESP proposals, want 2 of each; one proposal cannot show an order",
			len(firstIKE.Proposals), len(firstESP.Proposals))
	}
	if !firstIKE.Equal(secondIKE) {
		t.Errorf("one ike-group parsed to two values, so a commit would restart every peer naming it:\nfirst  = %+v\nsecond = %+v",
			firstIKE, secondIKE)
	}
	if !firstESP.Equal(secondESP) {
		t.Errorf("one esp-group parsed to two values, so a commit would restart every peer naming it:\nfirst  = %+v\nsecond = %+v",
			firstESP, secondESP)
	}
}

func TestIPsecConfigChanged(t *testing.T) {
	old := &IPsecConfig{
		Peers: map[string]SiteToSitePeer{
			"kept":    {Name: "kept", RemoteAddress: "1.1.1.1"},
			"changed": {Name: "changed", RemoteAddress: "2.2.2.2"},
			"removed": {Name: "removed", RemoteAddress: "3.3.3.3"},
		},
	}
	new := &IPsecConfig{
		Peers: map[string]SiteToSitePeer{
			"kept":    {Name: "kept", RemoteAddress: "1.1.1.1"},
			"changed": {Name: "changed", RemoteAddress: "9.9.9.9"},
			"added":   {Name: "added", RemoteAddress: "4.4.4.4"},
		},
	}

	got := new.Changed(old)
	gotSet := make(map[string]bool)
	for _, name := range got {
		gotSet[name] = true
	}

	if gotSet["kept"] {
		t.Error("'kept' should not be in changed list")
	}
	if !gotSet["changed"] {
		t.Error("'changed' should be in changed list")
	}
	if !gotSet["removed"] {
		t.Error("'removed' should be in changed list")
	}
	if !gotSet["added"] {
		t.Error("'added' should be in changed list")
	}
}

// VALIDATES: Changed reports a peer whose ONLY edit is a member the old field list left
// out. It listed IKEGroup, ESPGroup, ConnectionType, LocalAddress, RemoteAddress, VTIBind,
// IfID and Auth, so TrafficSelectors, Mode and TransportRequired were invisible to it.
//
// PREVENTS: half a fix. Changed and peerConfigChanged (engine/reconcile.go) were two
// hand-written field lists over one type, and they did not even name the same eight
// members. Repairing the engine's list and leaving this one would put the same defect one
// caller away.
func TestIPsecConfigChangedSeesEveryPeerMember(t *testing.T) {
	base := SiteToSitePeer{
		Name:          "branch-1",
		RemoteAddress: "192.0.2.1",
		TrafficSelectors: []TrafficSelectorPolicy{
			{Number: "1", LocalPrefix: mustCIDR(t, "10.1.0.0/16"), RemotePrefix: mustCIDR(t, "10.2.0.0/16")},
		},
		Mode:              dataplane.ModeTunnel,
		TransportRequired: true,
	}

	cases := []struct {
		member string
		edit   func(p *SiteToSitePeer)
	}{
		{"TrafficSelectors", func(p *SiteToSitePeer) {
			p.TrafficSelectors = []TrafficSelectorPolicy{
				{Number: "1", LocalPrefix: mustCIDR(t, "10.1.0.0/24"), RemotePrefix: mustCIDR(t, "10.2.0.0/16")},
			}
		}},
		{"Mode", func(p *SiteToSitePeer) { p.Mode = dataplane.ModeTransport }},
		{"TransportRequired", func(p *SiteToSitePeer) { p.TransportRequired = false }},
	}

	for _, tc := range cases {
		t.Run(tc.member, func(t *testing.T) {
			old := &IPsecConfig{Peers: map[string]SiteToSitePeer{"branch-1": base}}
			edited := base
			tc.edit(&edited)
			updated := &IPsecConfig{Peers: map[string]SiteToSitePeer{"branch-1": edited}}

			changed := updated.Changed(old)
			if len(changed) != 1 || changed[0] != "branch-1" {
				t.Errorf("editing %s gave Changed() = %v, want [branch-1]", tc.member, changed)
			}
		})
	}

	unchanged := &IPsecConfig{Peers: map[string]SiteToSitePeer{"branch-1": base}}
	same := &IPsecConfig{Peers: map[string]SiteToSitePeer{"branch-1": base}}
	if got := unchanged.Changed(same); len(got) != 0 {
		t.Errorf("editing nothing gave Changed() = %v, want none", got)
	}
}

func mustCIDR(t *testing.T, prefix string) *net.IPNet {
	t.Helper()
	_, n, err := net.ParseCIDR(prefix)
	if err != nil {
		t.Fatalf("prefix %q: %v", prefix, err)
	}
	return n
}

func TestIPsecConfigChangedRemoteAccess(t *testing.T) {
	old := &IPsecConfig{
		Peers: map[string]SiteToSitePeer{},
		RemoteAccess: &RemoteAccessConfig{
			IKEGroup: "IKE-RA",
			Pool:     VirtualIPPool{Name: "pool1", Range: "10.10.0.0/24"},
			Users:    map[string]EAPUser{"bob": {Name: "bob", Password: "old"}},
		},
	}
	updated := &IPsecConfig{
		Peers: map[string]SiteToSitePeer{},
		RemoteAccess: &RemoteAccessConfig{
			IKEGroup: "IKE-RA",
			Pool:     VirtualIPPool{Name: "pool1", Range: "10.10.0.0/24"},
			Users:    map[string]EAPUser{"bob": {Name: "bob", Password: "new"}},
		},
	}
	got := updated.Changed(old)
	found := false
	for _, name := range got {
		if name == "remote-access" {
			found = true
		}
	}
	if !found {
		t.Error("Changed() should include 'remote-access' when users change")
	}

	same := &IPsecConfig{
		Peers: map[string]SiteToSitePeer{},
		RemoteAccess: &RemoteAccessConfig{
			IKEGroup: "IKE-RA",
			Pool:     VirtualIPPool{Name: "pool1", Range: "10.10.0.0/24"},
			Users:    map[string]EAPUser{"bob": {Name: "bob", Password: "old"}},
		},
	}
	got2 := same.Changed(old)
	for _, name := range got2 {
		if name == "remote-access" {
			t.Error("Changed() should not include 'remote-access' when config is identical")
		}
	}
}

func TestParseNilTree(t *testing.T) {
	cfg, err := ParseIPsecConfig(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if len(cfg.ESPGroups) != 0 || len(cfg.IKEGroups) != 0 || len(cfg.Peers) != 0 {
		t.Error("expected empty config")
	}
}

func TestParseESPLifetimeBoundary(t *testing.T) {
	tree := makeESPTree("GOOD", "86400", "", map[string][2]string{
		"1": {"aes128", "sha256"},
	})
	if _, err := ParseIPsecConfig(tree); err != nil {
		t.Fatalf("lifetime 86400 should be valid: %v", err)
	}

	tree = makeESPTree("BAD", "86401", "", map[string][2]string{
		"1": {"aes128", "sha256"},
	})
	if _, err := ParseIPsecConfig(tree); err == nil {
		t.Fatal("lifetime 86401 should be rejected")
	}
}

func TestParseDPDBoundary(t *testing.T) {
	tree := makeIKETree("OK", ikeOpts{
		dpdInterval: "3600",
		dpdTimeout:  "3600",
		proposals: map[string]ikeProposalOpts{
			"1": {encryption: "aes128", hash: "sha256", dhGroup: "14"},
		},
	})
	if _, err := ParseIPsecConfig(tree); err != nil {
		t.Fatalf("DPD 3600 should be valid: %v", err)
	}

	tree = makeIKETree("BAD-INT", ikeOpts{
		dpdInterval: "3601",
		proposals: map[string]ikeProposalOpts{
			"1": {encryption: "aes128", hash: "sha256", dhGroup: "14"},
		},
	})
	if _, err := ParseIPsecConfig(tree); err == nil {
		t.Fatal("DPD interval 3601 should be rejected")
	}

	tree = makeIKETree("BAD-TO", ikeOpts{
		dpdTimeout: "0",
		proposals: map[string]ikeProposalOpts{
			"1": {encryption: "aes128", hash: "sha256", dhGroup: "14"},
		},
	})
	if _, err := ParseIPsecConfig(tree); err == nil {
		t.Fatal("DPD timeout 0 should be rejected")
	}
}

func TestParseProposalNumberBoundary(t *testing.T) {
	tree := makeESPTree("GOOD", "", "", map[string][2]string{
		"65535": {"aes128", "sha256"},
	})
	if _, err := ParseIPsecConfig(tree); err != nil {
		t.Fatalf("proposal 65535 should be valid: %v", err)
	}

	tree = makeESPTree("BAD", "", "", map[string][2]string{
		"0": {"aes128", "sha256"},
	})
	if _, err := ParseIPsecConfig(tree); err == nil {
		t.Fatal("proposal 0 should be rejected")
	}
}

func TestValidatePKIRefsLocalIDMismatch(t *testing.T) {
	cfg := &IPsecConfig{
		Peers: map[string]SiteToSitePeer{
			"p1": {
				Name: "p1",
				Auth: AuthConfig{
					Mode:          AuthX509,
					CACertificate: "ca1",
					Certificate:   "dev1",
					LocalID:       "WRONG",
				},
			},
		},
	}
	hasCA := func(string) bool { return true }
	hasCert := func(string) bool { return true }
	certCN := func(string) string { return "EXAFO000000400" }

	err := cfg.ValidatePKIRefs(hasCA, hasCert, certCN)
	if err == nil {
		t.Fatal("expected error for local-id/CN mismatch")
	}
	if !strings.Contains(err.Error(), "WRONG") || !strings.Contains(err.Error(), "EXAFO000000400") {
		t.Errorf("error = %q, want to mention both WRONG and EXAFO000000400", err)
	}
}

func TestValidatePKIRefsMatch(t *testing.T) {
	cfg := &IPsecConfig{
		Peers: map[string]SiteToSitePeer{
			"p1": {
				Name: "p1",
				Auth: AuthConfig{
					Mode:          AuthX509,
					CACertificate: "ca1",
					Certificate:   "dev1",
					LocalID:       "EXAFO000000400",
				},
			},
		},
	}
	hasCA := func(string) bool { return true }
	hasCert := func(string) bool { return true }
	certCN := func(string) string { return "EXAFO000000400" }

	if err := cfg.ValidatePKIRefs(hasCA, hasCert, certCN); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestParseInvalidName(t *testing.T) {
	longName := strings.Repeat("a", 256)
	for _, name := range []string{"", "has space", "semi;colon", longName} {
		tree := makeESPTree(name, "", "", map[string][2]string{
			"1": {"aes128", "sha256"},
		})
		_, err := ParseIPsecConfig(tree)
		if err == nil {
			t.Errorf("expected error for ESP group name %q", name)
		}
	}
}

func TestParseNonAEADWithoutHash(t *testing.T) {
	tree := makeESPTree("NO-HASH", "", "", map[string][2]string{
		"1": {"aes128", ""},
	})
	_, err := ParseIPsecConfig(tree)
	if err == nil {
		t.Fatal("expected error for non-AEAD aes128 without hash")
	}
	if !strings.Contains(err.Error(), "hash is required") {
		t.Errorf("error = %q, want to contain 'hash is required'", err)
	}
}

func TestParseAEADWithoutHash(t *testing.T) {
	tree := makeESPTree("AEAD-OK", "", "", map[string][2]string{
		"1": {"aes128gcm", ""},
	})
	_, err := ParseIPsecConfig(tree)
	if err != nil {
		t.Fatalf("AEAD without hash should be valid: %v", err)
	}
}

// --- EAP / Remote Access tests (spec ipsec-4) ---

func makeRemoteAccessTree(opts remoteAccessOpts) *config.Tree {
	tree := config.NewTree()
	vpn := tree.GetOrCreateContainer("vpn")
	ipsec := vpn.GetOrCreateContainer("ipsec")

	// Minimal IKE/ESP groups so config is valid
	ikeEntry := config.NewTree()
	prop := config.NewTree()
	prop.Set("encryption", "aes128gcm")
	prop.Set("hash", "sha256")
	prop.Set("dh-group", "14")
	ikeEntry.AddListEntry("proposal", "1", prop)
	ipsec.AddListEntry("ike-group", "IKE-RA", ikeEntry)

	espEntry := config.NewTree()
	espProp := config.NewTree()
	espProp.Set("encryption", "aes128gcm")
	espEntry.AddListEntry("proposal", "1", espProp)
	ipsec.AddListEntry("esp-group", "ESP-RA", espEntry)

	ra := ipsec.GetOrCreateContainer("remote-access")
	ra.Set("ike-group", "IKE-RA")
	ra.Set("esp-group", "ESP-RA")

	auth := ra.GetOrCreateContainer("authentication")
	if opts.authMode != "" {
		auth.Set("mode", opts.authMode)
	}
	if opts.caCert != "" || opts.cert != "" {
		x509 := auth.GetOrCreateContainer("x509")
		if opts.caCert != "" {
			x509.Set("ca-certificate", opts.caCert)
		}
		if opts.cert != "" {
			x509.Set("certificate", opts.cert)
		}
	}

	if opts.poolName != "" {
		poolEntry := config.NewTree()
		if opts.poolRange != "" {
			poolEntry.Set("range", opts.poolRange)
		}
		if opts.poolRange6 != "" {
			poolEntry.Set("range6", opts.poolRange6)
		}
		for _, dns := range opts.poolDNS {
			poolEntry.Set("dns", dns)
		}
		if opts.poolDomain != "" {
			poolEntry.Set("domain", opts.poolDomain)
		}
		ra.AddListEntry("pool", opts.poolName, poolEntry)
	}

	for _, u := range opts.users {
		userEntry := config.NewTree()
		if u.password != "" {
			userEntry.Set("password", u.password)
		}
		if u.certificate != "" {
			userEntry.Set("certificate", u.certificate)
		}
		ra.AddListEntry("eap-user", u.name, userEntry)
	}

	return tree
}

type remoteAccessOpts struct {
	authMode   string
	caCert     string
	cert       string
	poolName   string
	poolRange  string
	poolRange6 string
	poolDNS    []string
	poolDomain string
	users      []eapUserOpts
}

type eapUserOpts struct {
	name        string
	password    string
	certificate string
}

func TestParseRemoteAccessPool(t *testing.T) {
	tree := makeRemoteAccessTree(remoteAccessOpts{
		authMode:   "eap-mschapv2",
		caCert:     "vpn-ca",
		cert:       "server1",
		poolName:   "roadwarrior",
		poolRange:  "10.10.0.0/24",
		poolDNS:    []string{"8.8.8.8"},
		poolDomain: "vpn.example.com",
	})

	cfg, err := ParseIPsecConfig(tree)
	if err != nil {
		t.Fatalf("ParseIPsecConfig: %v", err)
	}
	if cfg.RemoteAccess == nil {
		t.Fatal("RemoteAccess is nil")
	}
	ra := cfg.RemoteAccess
	if ra.Pool.Name != "roadwarrior" {
		t.Errorf("pool name = %q, want roadwarrior", ra.Pool.Name)
	}
	if ra.Pool.Range != "10.10.0.0/24" {
		t.Errorf("pool range = %q, want 10.10.0.0/24", ra.Pool.Range)
	}
	if len(ra.Pool.DNS) != 1 || ra.Pool.DNS[0] != "8.8.8.8" {
		t.Errorf("pool dns = %v, want [8.8.8.8]", ra.Pool.DNS)
	}
	if ra.Pool.Domain != "vpn.example.com" {
		t.Errorf("pool domain = %q, want vpn.example.com", ra.Pool.Domain)
	}
	if ra.IKEGroup != "IKE-RA" {
		t.Errorf("ike-group = %q, want IKE-RA", ra.IKEGroup)
	}
	if ra.ESPGroup != "ESP-RA" {
		t.Errorf("esp-group = %q, want ESP-RA", ra.ESPGroup)
	}
}

func TestParseEAPMSCHAPv2Auth(t *testing.T) {
	tree := makeRemoteAccessTree(remoteAccessOpts{
		authMode: "eap-mschapv2",
		caCert:   "vpn-ca",
		cert:     "server1",
		poolName: "pool1",
		users: []eapUserOpts{
			{name: "thomas", password: "s3cret"},
		},
	})

	cfg, err := ParseIPsecConfig(tree)
	if err != nil {
		t.Fatalf("ParseIPsecConfig: %v", err)
	}
	if cfg.RemoteAccess == nil {
		t.Fatal("RemoteAccess is nil")
	}
	ra := cfg.RemoteAccess
	if ra.Auth.Mode != AuthEAPMSCHAPv2 {
		t.Errorf("auth mode = %s, want eap-mschapv2", ra.Auth.Mode)
	}
	if ra.Auth.CACertificate != "vpn-ca" {
		t.Errorf("ca-certificate = %q, want vpn-ca", ra.Auth.CACertificate)
	}
	user, ok := ra.Users["thomas"]
	if !ok {
		t.Fatal("user thomas not found")
	}
	if user.Password != "s3cret" {
		t.Errorf("password = %q, want s3cret", user.Password)
	}
}

func TestParseEAPMSCHAPv2EncodedPassword(t *testing.T) {
	encoded, err := secret.Encode("hunter2")
	if err != nil {
		t.Fatalf("secret.Encode: %v", err)
	}
	tree := makeRemoteAccessTree(remoteAccessOpts{
		authMode: "eap-mschapv2",
		poolName: "pool1",
		users: []eapUserOpts{
			{name: "bob", password: encoded},
		},
	})

	cfg, err := ParseIPsecConfig(tree)
	if err != nil {
		t.Fatalf("ParseIPsecConfig: %v", err)
	}
	user := cfg.RemoteAccess.Users["bob"]
	if user.Password != "hunter2" {
		t.Errorf("decoded password = %q, want hunter2", user.Password)
	}
}

func TestParseEAPTLSAuth(t *testing.T) {
	tree := makeRemoteAccessTree(remoteAccessOpts{
		authMode: "eap-tls",
		caCert:   "vpn-ca",
		cert:     "server1",
		poolName: "pool1",
		users: []eapUserOpts{
			{name: "alice", certificate: "alice-cert"},
		},
	})

	cfg, err := ParseIPsecConfig(tree)
	if err != nil {
		t.Fatalf("ParseIPsecConfig: %v", err)
	}
	if cfg.RemoteAccess == nil {
		t.Fatal("RemoteAccess is nil")
	}
	ra := cfg.RemoteAccess
	if ra.Auth.Mode != AuthEAPTLS {
		t.Errorf("auth mode = %s, want eap-tls", ra.Auth.Mode)
	}
	user, ok := ra.Users["alice"]
	if !ok {
		t.Fatal("user alice not found")
	}
	if user.Certificate != "alice-cert" {
		t.Errorf("certificate = %q, want alice-cert", user.Certificate)
	}
}

func TestValidatePoolRange(t *testing.T) {
	tests := []struct {
		name    string
		r       string
		r6      string
		wantErr bool
	}{
		{"valid ipv4", "10.10.0.0/24", "", false},
		{"valid ipv4 /8", "10.0.0.0/8", "", false},
		{"valid ipv4 /30", "10.10.0.0/30", "", false},
		{"invalid ipv4 /31", "10.10.0.0/31", "", true},
		{"invalid ipv4 /7", "10.0.0.0/7", "", true},
		{"valid ipv6", "", "fd00::/64", false},
		{"valid ipv6 /48", "", "fd00::/48", false},
		{"valid ipv6 /126", "", "fd00::/126", false},
		{"invalid ipv6 /127", "", "fd00::/127", true},
		{"invalid ipv6 /47", "", "fd00::/47", true},
		{"invalid cidr", "not-a-cidr", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &IPsecConfig{
				IKEGroups: map[string]IKEGroup{"IKE-RA": {}},
				ESPGroups: map[string]ESPGroup{"ESP-RA": {}},
				RemoteAccess: &RemoteAccessConfig{
					IKEGroup: "IKE-RA",
					ESPGroup: "ESP-RA",
					Pool: VirtualIPPool{
						Name:   "test",
						Range:  tt.r,
						Range6: tt.r6,
					},
					Users: make(map[string]EAPUser),
				},
			}
			err := cfg.ValidateRemoteAccess()
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateRemoteAccess() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateEAPUserPassword(t *testing.T) {
	cfg := &IPsecConfig{
		IKEGroups: map[string]IKEGroup{"IKE-RA": {}},
		ESPGroups: map[string]ESPGroup{"ESP-RA": {}},
		RemoteAccess: &RemoteAccessConfig{
			IKEGroup: "IKE-RA",
			ESPGroup: "ESP-RA",
			Auth:     AuthConfig{Mode: AuthEAPMSCHAPv2},
			Pool:     VirtualIPPool{Name: "pool1"},
			Users: map[string]EAPUser{
				"nopw": {Name: "nopw"},
			},
		},
	}
	err := cfg.ValidateRemoteAccess()
	if err == nil {
		t.Fatal("expected error for mschapv2 user without password")
	}
	if !strings.Contains(err.Error(), "password is required") {
		t.Errorf("error = %q, want to contain 'password is required'", err)
	}
}

func TestValidateEAPTLSUserCertificate(t *testing.T) {
	cfg := &IPsecConfig{
		IKEGroups: map[string]IKEGroup{"IKE-RA": {}},
		ESPGroups: map[string]ESPGroup{"ESP-RA": {}},
		RemoteAccess: &RemoteAccessConfig{
			IKEGroup: "IKE-RA",
			ESPGroup: "ESP-RA",
			Auth:     AuthConfig{Mode: AuthEAPTLS},
			Pool:     VirtualIPPool{Name: "pool1"},
			Users: map[string]EAPUser{
				"nocert": {Name: "nocert"},
			},
		},
	}
	err := cfg.ValidateRemoteAccess()
	if err == nil {
		t.Fatal("expected error for eap-tls user without certificate")
	}
	if !strings.Contains(err.Error(), "certificate is required") {
		t.Errorf("error = %q, want to contain 'certificate is required'", err)
	}
}

func TestDualStackPool(t *testing.T) {
	tree := makeRemoteAccessTree(remoteAccessOpts{
		authMode:   "eap-mschapv2",
		poolName:   "dualstack",
		poolRange:  "10.10.0.0/24",
		poolRange6: "fd00:vpn::/64",
	})

	cfg, err := ParseIPsecConfig(tree)
	if err != nil {
		t.Fatalf("ParseIPsecConfig: %v", err)
	}
	if cfg.RemoteAccess == nil {
		t.Fatal("RemoteAccess is nil")
	}
	pool := cfg.RemoteAccess.Pool
	if pool.Range != "10.10.0.0/24" {
		t.Errorf("range = %q, want 10.10.0.0/24", pool.Range)
	}
	if pool.Range6 != "fd00:vpn::/64" {
		t.Errorf("range6 = %q, want fd00:vpn::/64", pool.Range6)
	}
}
