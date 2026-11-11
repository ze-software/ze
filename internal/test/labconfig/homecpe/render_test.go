package homecpe

import (
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"errors"
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
	if _, err := ParseValues("name\n"); err == nil {
		t.Error("a lab-values line with no value was accepted")
	}
	if _, err := ParseValues("a 1\na 2\n"); err == nil {
		t.Error("a name given twice was accepted")
	}
}

// TestRenderHomeCPE proves the committed conversion renders with no
// placeholder left, the laptop's public key is the X25519 public key of its
// lab private key, and the issued CPE certificate chains to the issued CA
// with the CPE's IKE identity.
func TestRenderHomeCPE(t *testing.T) {
	root := checkoutRoot(t)
	rendered, err := Render(root, time.Now(), KeyP256)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if strings.Contains(rendered.Config, placeholderOpen) {
		t.Fatalf("the rendered configuration still holds a placeholder")
	}
	listed, err := os.ReadFile(filepath.Join(root, Directory, LabValuesFile))
	if err != nil {
		t.Fatalf("read lab-values: %v", err)
	}
	values, err := ParseValues(string(listed))
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
	if !strings.Contains(rendered.Config, "public-key \""+public+"\"") {
		t.Fatalf("the rendered CPE does not name the laptop's derived public key %s", public)
	}
	issued, err := issueLab(values, time.Now(), KeyP256)
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
	if _, err := cpe.Verify(x509.VerifyOptions{Roots: pool, DNSName: CPEIdentity,
		KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageAny}}); err != nil {
		t.Fatalf("the CPE certificate does not chain to the lab CA as %s: %v", CPEIdentity, err)
	}
	if !ca.MaxPathLenZero {
		t.Fatal("the lab CA carries no path length 0")
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
		algorithm KeyAlgorithm
		want      x509.PublicKeyAlgorithm
	}{
		{"p256", KeyP256, x509.ECDSA},
		{"ed25519", KeyEd25519, x509.Ed25519},
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
	if _, err := issueLab(values, time.Now(), KeyUnspecified); !errors.Is(err, errCPEKeyUnspecified) {
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
