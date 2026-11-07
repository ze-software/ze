// Design: docs/architecture/testing/interop.md -- the home-CPE lab.
// Related: register_home_lab.go -- the scenarios that run the rendered CPE.
// Related: home_lab_record.go -- the conversion record the conversion scenario checks.
package topology

import (
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"maps"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// The converted CPE's inputs, under the checkout: the model's output with
// placeholders, the VyOS source it came from, the record of the conversion,
// and the lab values for the placeholders.
const (
	cpeDirectory      = "test/interop-topology/cpe"
	cpeConvertedFile  = "ze-cpe.conf"
	cpeSourceFile     = "vyos.conf"
	cpeRecordFile     = "conversion-record.md"
	cpeLabValuesFile  = "lab-values"
	renderedCPEConfig = "ze-cpe.conf"
)

// The two placeholder kinds of the conversion: a secret the owner fills on the
// router, and a value the lab issues at run time.
const (
	placeholderOpen   = "<<"
	placeholderClose  = ">>"
	placeholderSecret = "SECRET:"
	placeholderLab    = "LAB:"
)

// The names the issuer reads from lab-values and the names it issues. Each
// issued name is a placeholder of ze-cpe.conf; wgLaptopPrivate is not, it is
// the wg-client's own key.
const (
	wgCPEPrivate      = "wg-wg0-private-key"
	wgLaptopPSK       = "wg-wg0-laptop-psk"
	wgLaptopPrivate   = "wg-laptop-private-key"
	wgLaptopPublic    = "wg-wg0-laptop-public-key"
	pkiCACertificate  = "pki-LAB-CA-certificate"
	pkiCPECertificate = "pki-CPE-certificate"
	pkiCPEKey         = "pki-CPE-key"
)

// The identities of the lab PKI: the responder vpn.example.net and the CPE's
// local-id, both in ze-cpe.conf and the responder's swanctl.conf.
const (
	responderName = "vpn.example.net"
	cpeIKEName    = "cpe.lab.example.net"
)

// The files the issuer writes for the peers, relative to the prepared
// directory, named by the node declarations that mount them.
const (
	issuedLaptopKey   = "wireguard/laptop.key"
	issuedCPEPublic   = "wireguard/cpe.pub"
	issuedLaptopPSK   = "wireguard/psk"
	issuedCA          = "swanctl/x509ca/lab-ca.pem"
	issuedResponder   = "swanctl/x509/vpn.pem"
	issuedResponderSK = "swanctl/private/vpn-key.pem"
)

// labValues maps a placeholder name to its value.
type labValues map[string]string

// parseLabValues reads lab-values: one "name value" pair per line, with blank
// lines and lines opening with "#" skipped. A line with no value and a name
// given twice are refused, so a typo never leaves a placeholder unfilled in
// silence.
func parseLabValues(text string) (labValues, error) {
	values := labValues{}
	for number, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") {
			continue
		}
		name, value, found := strings.Cut(line, " ")
		value = strings.TrimSpace(value)
		if !found {
			return nil, fmt.Errorf("lab-values line %d names %q with no value; write \"<name> <value>\"", number+1, name)
		}
		if value == "" {
			return nil, fmt.Errorf("lab-values line %d names %q with no value; write \"<name> <value>\"", number+1, name)
		}
		if _, seen := values[name]; seen {
			return nil, fmt.Errorf("lab-values line %d names %q a second time; keep one value per name", number+1, name)
		}
		values[name] = value
	}
	return values, nil
}

// substitutePlaceholders replaces every <<SECRET:name>> and <<LAB:name>> of
// template with its value and changes nothing else. A placeholder of another
// kind, one with no value, and an unterminated one are refused: a file that
// still holds a placeholder is not the configuration the lab means to run.
func substitutePlaceholders(template string, values labValues) (string, error) {
	var out strings.Builder
	rest := template
	for {
		before, after, opened := strings.Cut(rest, placeholderOpen)
		out.WriteString(before)
		if !opened {
			return out.String(), nil
		}
		body, tail, closed := strings.Cut(after, placeholderClose)
		if !closed {
			return "", fmt.Errorf("the configuration holds an unterminated placeholder at %q; close it with %q", clip(placeholderOpen+after), placeholderClose)
		}
		name, err := placeholderName(body)
		if err != nil {
			return "", err
		}
		value, found := values[name]
		if !found {
			return "", fmt.Errorf("placeholder %q has no lab value; add \"%s <value>\" to %s/%s or issue it", body, name, cpeDirectory, cpeLabValuesFile)
		}
		out.WriteString(value)
		rest = tail
	}
}

// placeholderName answers the name of one placeholder's body, and refuses a
// kind that is neither SECRET nor LAB.
func placeholderName(body string) (string, error) {
	if name, found := strings.CutPrefix(body, placeholderSecret); found {
		return name, nil
	}
	if name, found := strings.CutPrefix(body, placeholderLab); found {
		return name, nil
	}
	return "", fmt.Errorf("placeholder %q is neither %s nor %s; the conversion writes only those two kinds", body, placeholderSecret, placeholderLab)
}

// clip shortens a fragment quoted in an error to one line of context.
func clip(fragment string) string {
	const contextOctets = 40
	if len(fragment) > contextOctets {
		return fragment[:contextOctets]
	}
	return fragment
}

// issuedLab is what one run issues: the placeholder values only the run can
// know, and the files the peers mount, by path relative to the prepared
// directory.
type issuedLab struct {
	values labValues
	files  map[string][]byte
}

// issueLab derives the WireGuard public keys from the lab private keys and
// issues a fresh P-256 PKI: a CA with path length 0, the responder's
// certificate for vpn.example.net and the CPE's for cpe.lab.example.net.
// The CPE key is the P-256 variant the umbrella names. Ze signs IKE AUTH with
// Ed25519 since spec-home-cpe-5, and the Ed25519 CPE key of spec-home-cpe-11
// AC-9 is not minted here yet.
func issueLab(values labValues, now time.Time) (issuedLab, error) {
	issued := issuedLab{values: labValues{}, files: map[string][]byte{}}
	if err := issueWireGuard(values, &issued); err != nil {
		return issuedLab{}, err
	}
	if err := issuePKI(now, &issued); err != nil {
		return issuedLab{}, err
	}
	return issued, nil
}

// issueWireGuard derives each X25519 public key from its lab private key
// (RFC 7748 Section 6.1), so no public key is written by hand beside the
// private key it must match.
func issueWireGuard(values labValues, issued *issuedLab) error {
	laptopPublic, err := wireGuardPublic(values, wgLaptopPrivate)
	if err != nil {
		return err
	}
	cpePublic, err := wireGuardPublic(values, wgCPEPrivate)
	if err != nil {
		return err
	}
	psk, found := values[wgLaptopPSK]
	if !found {
		return fmt.Errorf("lab-values names no %s; the wg-client needs the preshared key the CPE holds", wgLaptopPSK)
	}
	issued.values[wgLaptopPublic] = laptopPublic
	issued.files[issuedLaptopKey] = []byte(values[wgLaptopPrivate] + "\n")
	issued.files[issuedCPEPublic] = []byte(cpePublic + "\n")
	issued.files[issuedLaptopPSK] = []byte(psk + "\n")
	return nil
}

// wireGuardPublic answers the base64 public key of the lab private key name.
func wireGuardPublic(values labValues, name string) (string, error) {
	encoded, found := values[name]
	if !found {
		return "", fmt.Errorf("lab-values names no %s; add a base64 X25519 private key for it", name)
	}
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("lab value %s is not base64: %w", name, err)
	}
	private, err := ecdh.X25519().NewPrivateKey(raw)
	if err != nil {
		return "", fmt.Errorf("lab value %s is not a 32-octet X25519 private key: %w", name, err)
	}
	return base64.StdEncoding.EncodeToString(private.PublicKey().Bytes()), nil
}

// issuePKI issues the CA, the responder's certificate and the CPE's. Ze's
// pki leaves take the base64 DER without PEM armour (a certificate) and the
// SEC 1 DER of an EC key; strongSwan reads PEM.
func issuePKI(now time.Time, issued *issuedLab) error {
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("generate the lab CA key: %w", err)
	}
	caTemplate, err := certificateTemplate(now, "ze-home-lab-ca")
	if err != nil {
		return err
	}
	caTemplate.IsCA = true
	caTemplate.BasicConstraintsValid = true
	caTemplate.MaxPathLen = 0
	caTemplate.MaxPathLenZero = true
	caTemplate.KeyUsage = x509.KeyUsageCertSign | x509.KeyUsageCRLSign
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		return fmt.Errorf("issue the lab CA: %w", err)
	}
	ca, err := x509.ParseCertificate(caDER)
	if err != nil {
		return fmt.Errorf("read back the lab CA: %w", err)
	}
	responderDER, responderKey, err := issueLeaf(now, ca, caKey, responderName)
	if err != nil {
		return err
	}
	cpeDER, cpeKey, err := issueLeaf(now, ca, caKey, cpeIKEName)
	if err != nil {
		return err
	}
	issued.values[pkiCACertificate] = base64.StdEncoding.EncodeToString(caDER)
	issued.values[pkiCPECertificate] = base64.StdEncoding.EncodeToString(cpeDER)
	issued.values[pkiCPEKey] = base64.StdEncoding.EncodeToString(cpeKey)
	issued.files[issuedCA] = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER})
	issued.files[issuedResponder] = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: responderDER})
	issued.files[issuedResponderSK] = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: responderKey})
	return nil
}

// issueLeaf issues one end-entity certificate for name, signed by ca, and
// answers its DER and its key's SEC 1 DER.
func issueLeaf(now time.Time, ca *x509.Certificate, caKey *ecdsa.PrivateKey, name string) ([]byte, []byte, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("generate the key of %s: %w", name, err)
	}
	template, err := certificateTemplate(now, name)
	if err != nil {
		return nil, nil, err
	}
	template.DNSNames = []string{name}
	template.KeyUsage = x509.KeyUsageDigitalSignature
	der, err := x509.CreateCertificate(rand.Reader, template, ca, &key.PublicKey, caKey)
	if err != nil {
		return nil, nil, fmt.Errorf("issue the certificate of %s: %w", name, err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, nil, fmt.Errorf("encode the key of %s: %w", name, err)
	}
	return der, keyDER, nil
}

// certificateTemplate is a certificate valid for one day around now, with a
// random 128-bit serial number.
func certificateTemplate(now time.Time, commonName string) (*x509.Certificate, error) {
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, fmt.Errorf("draw a serial number for %s: %w", commonName, err)
	}
	return &x509.Certificate{SerialNumber: serial, Subject: pkix.Name{CommonName: commonName},
		NotBefore: now.Add(-time.Hour), NotAfter: now.Add(24 * time.Hour)}, nil
}

// renderedCPE is the converted configuration with every placeholder filled,
// and the files the run issued for the peers.
type renderedCPE struct {
	config string
	files  map[string][]byte
}

// renderHomeCPE reads the converted configuration and the lab values under
// root, issues the run's values, and substitutes both. A name both listed and
// issued is refused, so lab-values can never shadow what the run derived.
func renderHomeCPE(root string, now time.Time) (renderedCPE, error) {
	directory := filepath.Join(root, filepath.FromSlash(cpeDirectory))
	template, err := os.ReadFile(filepath.Join(directory, cpeConvertedFile)) //nolint:gosec // the checkout root joined with a fixed name
	if err != nil {
		return renderedCPE{}, fmt.Errorf("read the converted CPE configuration: %w", err)
	}
	listed, err := os.ReadFile(filepath.Join(directory, cpeLabValuesFile)) //nolint:gosec // the checkout root joined with a fixed name
	if err != nil {
		return renderedCPE{}, fmt.Errorf("read the lab values: %w", err)
	}
	values, err := parseLabValues(string(listed))
	if err != nil {
		return renderedCPE{}, err
	}
	issued, err := issueLab(values, now)
	if err != nil {
		return renderedCPE{}, err
	}
	for name, value := range issued.values {
		if _, both := values[name]; both {
			return renderedCPE{}, fmt.Errorf("lab-values lists %s, which the run issues; remove it from %s/%s", name, cpeDirectory, cpeLabValuesFile)
		}
		values[name] = value
	}
	config, err := substitutePlaceholders(string(template), values)
	if err != nil {
		return renderedCPE{}, err
	}
	return renderedCPE{config: config, files: issued.files}, nil
}

// prepareHomeCPE renders the CPE configuration and the issued files into
// directory, for the nodes that mount them (node.prepared).
func prepareHomeCPE(root, directory string) error {
	rendered, err := renderHomeCPE(root, time.Now())
	if err != nil {
		return err
	}
	files := map[string][]byte{renderedCPEConfig: []byte(rendered.config)}
	maps.Copy(files, rendered.files)
	for name, content := range files {
		path := filepath.Join(directory, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil { //nolint:gosec // the lab containers read these files as another user
			return fmt.Errorf("create the directory of %s: %w", path, err)
		}
		if err := os.WriteFile(path, content, 0o644); err != nil { //nolint:gosec // the lab containers read these files as another user
			return fmt.Errorf("write %s: %w", path, err)
		}
	}
	return nil
}

// errNoPreparedPeer says a node declares prepared files but the plan holds no
// peer of that name, a declaration defect.
var errNoPreparedPeer = errors.New("the plan holds no peer for a node with prepared files")

// preparePlan runs a scenario's prepare step into a fresh directory under the
// session scratch and mounts each node's prepared files from it. A scenario
// with no prepare step is left as it is.
func preparePlan(root, suffix string, declared *scenario, plan *interoplab.ScenarioPlan) error {
	if declared.prepare == nil {
		return nil
	}
	paths, err := lepath.ResolveSession(root, true)
	if err != nil {
		return fmt.Errorf("resolve the session scratch for %s: %w", plan.Source.Name, err)
	}
	directory := filepath.Join(root, paths.Scratch, "interop-topology", plan.Source.Name+"-"+suffix)
	if err := os.RemoveAll(directory); err != nil {
		return fmt.Errorf("clear the prepared directory %s: %w", directory, err)
	}
	if err := os.MkdirAll(directory, 0o755); err != nil { //nolint:gosec // the lab containers read these files as another user
		return fmt.Errorf("create the prepared directory %s: %w", directory, err)
	}
	if err := declared.prepare(root, directory); err != nil {
		return fmt.Errorf("prepare %s: %w", plan.Source.Name, err)
	}
	for index := range declared.nodes {
		if err := mountPrepared(plan, &declared.nodes[index], directory); err != nil {
			return err
		}
	}
	return nil
}

// mountPrepared adds the prepared files of one node to its peer's mounts.
func mountPrepared(plan *interoplab.ScenarioPlan, declared *node, directory string) error {
	if len(declared.prepared) == 0 {
		return nil
	}
	for index := range plan.Peers {
		if plan.Peers[index].Name != declared.name {
			continue
		}
		for _, file := range declared.prepared {
			plan.Peers[index].Mounts = append(plan.Peers[index].Mounts, interoplab.Mount{
				Source: filepath.Join(directory, filepath.FromSlash(file.source)), Target: file.target, ReadOnly: true})
		}
		return nil
	}
	return fmt.Errorf("%w: %s in %s", errNoPreparedPeer, declared.name, plan.Source.Name)
}
