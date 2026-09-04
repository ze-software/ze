// VALIDATES: a pki config leaf accepts the PEM document Ze itself prints, and
// still accepts the base64 DER existing configs hold, for the CA certificate,
// the device certificate, an intermediate and the private key. Also the whole
// distribution path of AC-8: the text `ze show pki local-ca pem` answers is
// pasted verbatim into a `pki ca` block and the resulting pool validates a leaf
// that root issued.
// PREVENTS: an export whose only consumer refuses its output, which forces the
// operator to strip the PEM armor and rejoin the lines by hand, and a broken
// PEM reported as a base64 error about a payload nobody wrote.
package pki

import (
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// pemDocument wraps DER in the PEM armor a tool prints, which is the form an
// operator's clipboard holds.
func pemDocument(t *testing.T, label string, der []byte) string {
	t.Helper()
	out := pem.EncodeToMemory(&pem.Block{Type: label, Bytes: der})
	if out == nil {
		t.Fatalf("encode %s PEM", label)
	}
	return string(out)
}

func TestParseCACertAcceptsPEMAndBase64(t *testing.T) {
	_, caDER := testCACertDER(t)

	cases := []struct {
		name  string
		value string
	}{
		{"pem", pemDocument(t, pemBlockCertificate, caDER)},
		{"base64 der", base64.StdEncoding.EncodeToString(caDER)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := ParseConfig(makePKITree(t, tc.value, "", ""))
			if err != nil {
				t.Fatalf("ParseConfig: %v", err)
			}
			ca, ok := cfg.CACerts["test-ca"]
			if !ok {
				t.Fatal("CA cert 'test-ca' not found")
			}
			if ca.Certificate.Subject.CommonName != "Test CA" {
				t.Fatalf("subject CN = %q, want %q", ca.Certificate.Subject.CommonName, "Test CA")
			}
		})
	}
}

func TestParseDeviceCertAcceptsPEMAndBase64(t *testing.T) {
	caKey, caDER := testCACertDER(t)
	devKey, devDER := testDeviceCertDER(t, caKey, caDER)
	keyDER, err := x509.MarshalPKCS8PrivateKey(devKey)
	if err != nil {
		t.Fatalf("marshal device key: %v", err)
	}

	cases := []struct {
		name string
		cert string
		key  string
	}{
		{
			name: "pem",
			cert: pemDocument(t, pemBlockCertificate, devDER),
			key:  pemDocument(t, pemBlockPrivateKey, keyDER),
		},
		{
			name: "base64 der",
			cert: base64.StdEncoding.EncodeToString(devDER),
			key:  base64.StdEncoding.EncodeToString(keyDER),
		},
		{
			name: "pem certificate with a base64 key",
			cert: pemDocument(t, pemBlockCertificate, devDER),
			key:  base64.StdEncoding.EncodeToString(keyDER),
		},
		{
			name: "base64 certificate with a pem key",
			cert: base64.StdEncoding.EncodeToString(devDER),
			key:  pemDocument(t, pemBlockPrivateKey, keyDER),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := ParseConfig(makePKITree(t, base64.StdEncoding.EncodeToString(caDER), tc.cert, tc.key))
			if err != nil {
				t.Fatalf("ParseConfig: %v", err)
			}
			entry, ok := cfg.Certificates["dev-1"]
			if !ok {
				t.Fatal("certificate 'dev-1' not found")
			}
			if entry.PrivateKey == nil {
				t.Fatal("the private key was not parsed")
			}
			if entry.Certificate.Subject.CommonName != "Test Device" {
				t.Fatalf("subject CN = %q, want %q", entry.Certificate.Subject.CommonName, "Test Device")
			}
		})
	}
}

func TestParseIntermediateAcceptsPEM(t *testing.T) {
	caKey, caDER := testCACertDER(t)
	devKey, devDER := testDeviceCertDER(t, caKey, caDER)

	tree := config.NewTree()
	pkiContainer := tree.GetOrCreateContainer("pki")
	entryTree := config.NewTree()
	entryTree.Set("certificate", base64.StdEncoding.EncodeToString(devDER))
	entryTree.SetSlice("intermediate", []string{pemDocument(t, pemBlockCertificate, caDER)})
	entryTree.GetOrCreateContainer("private").Set("key", marshalKeyB64(t, devKey))
	pkiContainer.AddListEntry("certificate", "dev-1", entryTree)

	cfg, err := ParseConfig(tree)
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}
	entry := cfg.Certificates["dev-1"]
	if len(entry.Intermediates) != 1 {
		t.Fatalf("intermediates parsed = %d, want 1", len(entry.Intermediates))
	}
	if entry.Intermediates[0].Subject.CommonName != "Test CA" {
		t.Fatalf("intermediate CN = %q, want %q", entry.Intermediates[0].Subject.CommonName, "Test CA")
	}
}

// TestPKILeafNamesBothFormsWhenItRefuses: neither accepted form may be reported
// as a failure of the other. A value that opens a PEM block is answered as a
// broken PEM, a value that opens none is answered as broken base64, and each
// message names what the leaf takes.
func TestPKILeafNamesBothFormsWhenItRefuses(t *testing.T) {
	_, caDER := testCACertDER(t)
	armored := pemDocument(t, pemBlockCertificate, caDER)

	cases := []struct {
		name  string
		value string
		want  string
	}{
		{"neither form", "not base64 !!!", "neither a PEM certificate nor base64 DER"},
		{"truncated pem", armored[:len(armored)/2], "PEM block that does not decode"},
		{"a key pasted into the certificate leaf", pemDocument(t, pemBlockPrivateKey, []byte{1, 2, 3}), "not a CERTIFICATE"},
		// The appliance's own cert.pem is leaf then root, so this is the paste
		// an operator makes when they reach for the file instead of the export
		// command. Taking the first block would store the LEAF as the anchor.
		{"leaf and root pasted together", armored + armored, "more than one PEM block"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseConfig(makePKITree(t, tc.value, "", ""))
			if err == nil {
				t.Fatal("a value that is neither form was accepted")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %q, want it to contain %q", err, tc.want)
			}
		})
	}
}

// TestExportedRootPastesIntoAPKICABlock walks AC-8 end to end. The export
// command answers the root, the operator pastes that exact text into a client's
// `pki ca <name> certificate` leaf, the client's config parses, and a leaf the
// hub root issued validates against the pool the store builds from it.
//
// The paste goes through the config TEXT parser rather than a hand-built tree,
// because the operator types a config file and the value carries newlines.
func TestExportedRootPastesIntoAPKICABlock(t *testing.T) {
	store, _ := newRootStore(t)
	root, err := LoadOrGenerateRoot(store)
	if err != nil {
		t.Fatalf("LoadOrGenerateRoot: %v", err)
	}

	exported, ok := exportedRoot(t, nil)[fieldPEM].(string)
	if !ok {
		t.Fatal("the export answered no PEM text")
	}

	// The client config, with the exported text pasted in unedited.
	var b textbuf.Buffer
	b.Str("pki {\n    ca fleet-hub-root {\n        certificate \"").
		Str(exported).
		Str("\";\n    }\n}\n")

	tree, err := config.ParseTreeWithYANG(b.String(), nil)
	if err != nil {
		t.Fatalf("the pasted root does not parse as config: %v", err)
	}
	cfg, err := ParseConfig(tree)
	if err != nil {
		t.Fatalf("the pasted root is refused by the pki parser: %v", err)
	}
	if err := Load(cfg); err != nil {
		t.Fatalf("Load: %v", err)
	}
	t.Cleanup(func() {
		if clearErr := Load(nil); clearErr != nil {
			t.Errorf("clear the pki store: %v", clearErr)
		}
	})

	// The daemon rewrites the config file when the operator commits, so the
	// pasted PEM has to survive a serialize and a re-parse as well as the first
	// load. A value mangled there would work once and break on the next commit.
	schema, err := config.YANGSchema()
	if err != nil {
		t.Fatalf("build the config schema: %v", err)
	}
	parser := config.NewParser(schema)
	rewritten, err := parser.Parse(config.Serialize(tree, schema))
	if err != nil {
		t.Fatalf("the rewritten config does not parse: %v", err)
	}
	rewrittenCfg, err := ParseConfig(rewritten)
	if err != nil {
		t.Fatalf("the rewritten config is refused by the pki parser: %v", err)
	}
	anchor := rewrittenCfg.CACerts["fleet-hub-root"]
	if anchor == nil {
		t.Fatal("the rewritten config carries no fleet-hub-root anchor")
	}
	if !anchor.Certificate.Equal(root.Certificate()) {
		t.Fatal("the rewritten config holds a different certificate than the paste")
	}

	entry := GetCA("fleet-hub-root")
	if entry == nil {
		t.Fatal("the pasted block loaded no CA named fleet-hub-root")
	}
	if !entry.Certificate.Equal(root.Certificate()) {
		t.Fatal("the loaded CA is not the root the export answered")
	}

	issued, err := root.IssueLeaf("ze-managed-hub", []string{"127.0.0.1"})
	if err != nil {
		t.Fatalf("IssueLeaf: %v", err)
	}
	leaf, err := x509.ParseCertificate(issued.Certificate[0])
	if err != nil {
		t.Fatalf("parse the issued leaf: %v", err)
	}
	if _, err := leaf.Verify(x509.VerifyOptions{
		Roots:     CAPool(),
		KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}); err != nil {
		t.Fatalf("a leaf the exported root issued does not validate against the configured pool: %v", err)
	}
}
