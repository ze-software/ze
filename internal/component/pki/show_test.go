// VALIDATES: the local certificate authority export command answers the root
// certificate in PEM, answers nothing else, and the text it prints is directly
// usable as a client trust anchor (AC-8, AC-10).
// PREVENTS: the root private key reaching an operator-facing surface, an export
// that answers an empty certificate when this process loaded no root, and an
// export that silently ignores a token the operator typed after it.
package pki

import (
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/pkg/zefs"
)

// exportedRoot runs the export handler and returns the payload it answered,
// failing the test on any status other than done.
func exportedRoot(t *testing.T, args []string) plugin.Map {
	t.Helper()

	resp, err := handleShowPKILocalCAPEM(nil, args)
	if err != nil {
		t.Fatalf("export handler returned an error: %v", err)
	}
	if resp.Status != plugin.StatusDone {
		t.Fatalf("export status = %v, error %q, want done", resp.Status, resp.Error)
	}
	data, ok := resp.Data.(plugin.Map)
	if !ok {
		t.Fatalf("export payload is %T, want plugin.Map", resp.Data)
	}
	return data
}

func TestExportRootPrintsTheCertificateOnly(t *testing.T) {
	store, _ := newRootStore(t)
	root, err := LoadOrGenerateRoot(store)
	if err != nil {
		t.Fatalf("LoadOrGenerateRoot: %v", err)
	}

	data := exportedRoot(t, nil)

	text, ok := data[fieldPEM].(string)
	if !ok {
		t.Fatalf("export payload has no %s string: %v", fieldPEM, data)
	}

	block, rest := pem.Decode([]byte(text))
	if block == nil {
		t.Fatalf("exported text is not PEM: %q", text)
	}
	if block.Type != pemBlockCertificate {
		t.Fatalf("exported PEM block is %q, want %q", block.Type, pemBlockCertificate)
	}
	if strings.TrimSpace(string(rest)) != "" {
		t.Fatalf("export carries a second PEM block: %q", rest)
	}
	exported, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("parse exported certificate: %v", err)
	}
	if !exported.Equal(root.Certificate()) {
		t.Fatal("the exported certificate is not this daemon's root")
	}

	// The whole response, not only the PEM field: a key that reached any other
	// field would be just as published.
	rendered, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal export payload: %v", err)
	}
	if strings.Contains(string(rendered), pemBlockPrivateKey) {
		t.Fatalf("the export names a private key: %s", rendered)
	}
	storedKey, err := store.ReadFile(zefs.KeyCAKey.Pattern)
	if err != nil {
		t.Fatalf("read stored root key: %v", err)
	}
	keyBlock, _ := pem.Decode(storedKey)
	if keyBlock == nil {
		t.Fatal("the stored root key is not PEM")
	}
	if strings.Contains(string(rendered), base64.StdEncoding.EncodeToString(keyBlock.Bytes)) {
		t.Fatalf("the export carries the root private key: %s", rendered)
	}

	// AC-8 asks for text a client can trust an issuer by, so verify a leaf this
	// root issued against a pool built from the exported text alone.
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM([]byte(text)) {
		t.Fatal("the exported text is not usable as a trust anchor")
	}
	issued, err := root.IssueLeaf("ze-plugin-hub", []string{"127.0.0.1"})
	if err != nil {
		t.Fatalf("IssueLeaf: %v", err)
	}
	leaf, err := x509.ParseCertificate(issued.Certificate[0])
	if err != nil {
		t.Fatalf("parse issued leaf: %v", err)
	}
	if _, err := leaf.Verify(x509.VerifyOptions{
		Roots:     pool,
		KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}); err != nil {
		t.Fatalf("a leaf this root issued does not validate against the exported anchor: %v", err)
	}
}

func TestExportRootRefusesWhenNoRootIsLoaded(t *testing.T) {
	loaded := currentRoot.Load()
	currentRoot.Store(nil)
	t.Cleanup(func() { currentRoot.Store(loaded) })

	resp, err := handleShowPKILocalCAPEM(nil, nil)
	if err != nil {
		t.Fatalf("export handler returned an error: %v", err)
	}
	if resp.Status != plugin.StatusError {
		t.Fatalf("export status = %v, want error when no root is loaded", resp.Status)
	}
	if resp.Data != nil {
		t.Fatalf("a refused export carries a payload: %v", resp.Data)
	}
}

func TestExportRootRefusesAnUnexpectedArgument(t *testing.T) {
	store, _ := newRootStore(t)
	if _, err := LoadOrGenerateRoot(store); err != nil {
		t.Fatalf("LoadOrGenerateRoot: %v", err)
	}

	resp, err := handleShowPKILocalCAPEM(nil, []string{"garbage"})
	if err != nil {
		t.Fatalf("export handler returned an error: %v", err)
	}
	if resp.Status != plugin.StatusError {
		t.Fatalf("export status = %v, want error for an undeclared token", resp.Status)
	}
	if !strings.Contains(resp.Error, "garbage") {
		t.Fatalf("the refusal does not name the token: %q", resp.Error)
	}
}
