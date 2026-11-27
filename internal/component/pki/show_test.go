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

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/commandtest"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/audit"
	"github.com/ze-software/ze/pkg/zefs"
)

// exportedRoot runs the export handler and returns the payload it answered,
// failing the test on any status other than done.
func exportedRoot(t *testing.T, args []string) plugin.Map {
	t.Helper()

	resp, err := handleShowPKILocalCAPEM(nil, commandtest.Args(args...))
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

	resp, err := handleShowPKILocalCAPEM(nil, commandtest.Args())
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

	resp, err := handleShowPKILocalCAPEM(nil, commandtest.Args("garbage"))
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

// TestShowPKICertBundlePEMRequiresConfirmAndPreservesOutput proves the
// bundle pem handler enforces confirm as defense-in-depth, preserves
// certificate-before-key PEM output when confirmed, leaves certificate-only
// pem unguarded, and registers the expected explicit-authz and audit metadata.
//
// VALIDATES: AC-3-bundle, AC-4, AC-5 at the PKI handler layer: calling
// handleShowPKICertificateBundlePEM without confirm returns an operator error
// with zero private-key material; a confirmed call returns the exact
// cert-before-PKCS#8-key PEM stream; handleShowPKICertificatePEM remains
// unguarded and certificate-only.
// PREVENTS: direct handler invocation bypassing the YANG confirm requirement
// or regressing certificate-only pem or confirmed bundle PEM formatting.
func TestShowPKICertBundlePEMRequiresConfirmAndPreservesOutput(t *testing.T) {
	cfg := testPKIConfig(t)
	if err := Load(cfg); err != nil {
		t.Fatalf("Load: %v", err)
	}

	t.Run("RPC registration declares explicit authz, not read-only, and audit action", func(t *testing.T) {
		var bundleReg, pemReg *pluginserver.RPCRegistration
		rpcs := pluginserver.AllBuiltinRPCs()
		for i := range rpcs {
			switch rpcs[i].WireMethod {
			case "ze-pki:show-certificate-bundle-pem":
				bundleReg = &rpcs[i]
			case "ze-pki:show-certificate-pem":
				pemReg = &rpcs[i]
			}
		}
		if bundleReg == nil {
			t.Fatal("ze-pki:show-certificate-bundle-pem not registered")
		}
		if !bundleReg.NotReadOnly {
			t.Error("bundle pem NotReadOnly = false, want true")
		}
		if !bundleReg.RequireExplicitAuthz {
			t.Error("bundle pem RequireExplicitAuthz = false, want true")
		}
		if bundleReg.AuditAction != audit.ActionPKIPrivateKeyExport {
			t.Errorf("bundle pem AuditAction = %q, want %q", bundleReg.AuditAction, audit.ActionPKIPrivateKeyExport)
		}
		if pemReg == nil {
			t.Fatal("ze-pki:show-certificate-pem not registered")
		}
		if pemReg.NotReadOnly || pemReg.RequireExplicitAuthz || pemReg.AuditAction != "" {
			t.Errorf("cert-only pem should remain unguarded: NotReadOnly=%v RequireExplicitAuthz=%v AuditAction=%q",
				pemReg.NotReadOnly, pemReg.RequireExplicitAuthz, pemReg.AuditAction)
		}
	})

	t.Run("missing confirm refuses with zero private-key bytes", func(t *testing.T) {
		resp, err := handleShowPKICertificateBundlePEM(nil, commandtest.Args("dev-1"))
		if err != nil {
			t.Fatalf("handleShowPKICertificateBundlePEM returned unexpected error: %v", err)
		}
		if resp == nil || resp.Status != plugin.StatusError {
			t.Fatalf("status = %v, want error when confirm is missing", resp)
		}
		if resp.Data != nil {
			t.Fatalf("refused response carried payload: %v", resp.Data)
		}
		if !strings.Contains(strings.ToLower(resp.Error), "confirm") {
			t.Errorf("refusal error %q does not name confirm", resp.Error)
		}
		if strings.Contains(resp.Error, pemBlockPrivateKey) {
			t.Errorf("refusal error leaked private-key marker: %q", resp.Error)
		}
	})

	t.Run("confirmed call returns certificate before PKCS#8 private key", func(t *testing.T) {
		defs := []command.ArgDef{
			commandtest.Must(command.NewStringArg("name", nil, nil, command.ArgOptions{Mandatory: true})),
			commandtest.Must(command.NewFlagArg("confirm", command.ArgOptions{Mandatory: true})),
		}
		validated, valErr := command.ValidateArgs([]string{"confirm"}, defs, map[string]string{"name": "dev-1"})
		if valErr != nil {
			t.Fatalf("ValidateArgs: %v", valErr)
		}
		ctx := &pluginserver.CommandContext{Selectors: map[string]string{"name": "dev-1"}}
		resp, err := handleShowPKICertificateBundlePEM(ctx, validated)
		if err != nil {
			t.Fatalf("handleShowPKICertificateBundlePEM: %v", err)
		}
		if resp == nil || resp.Status != plugin.StatusDone {
			t.Fatalf("status = %v (err %q), want done", resp.Status, resp.Error)
		}
		data, ok := resp.Data.(plugin.Map)
		if !ok {
			t.Fatalf("payload type = %T, want plugin.Map", resp.Data)
		}
		pemText, ok := data[fieldPEM].(string)
		if !ok || pemText == "" {
			t.Fatalf("missing %s string in response: %v", fieldPEM, data)
		}
		certIdx := strings.Index(pemText, "BEGIN CERTIFICATE")
		keyIdx := strings.Index(pemText, "BEGIN PRIVATE KEY")
		if certIdx < 0 {
			t.Fatal("confirmed bundle missing certificate PEM block")
		}
		if keyIdx < 0 {
			t.Fatal("confirmed bundle missing private-key PEM block")
		}
		if certIdx > keyIdx {
			t.Error("certificate PEM block must precede private-key PEM block")
		}
	})

	t.Run("certificate-only pem remains unguarded and omits private key", func(t *testing.T) {
		resp, err := handleShowPKICertificatePEM(nil, commandtest.Args("dev-1"))
		if err != nil {
			t.Fatalf("handleShowPKICertificatePEM: %v", err)
		}
		if resp == nil || resp.Status != plugin.StatusDone {
			t.Fatalf("status = %v (err %q), want done", resp.Status, resp.Error)
		}
		data, ok := resp.Data.(plugin.Map)
		if !ok {
			t.Fatalf("payload type = %T, want plugin.Map", resp.Data)
		}
		pemText, _ := data[fieldPEM].(string)
		if !strings.Contains(pemText, "BEGIN CERTIFICATE") {
			t.Error("cert-only pem missing certificate block")
		}
		if strings.Contains(pemText, pemBlockPrivateKey) {
			t.Error("cert-only pem must never contain a private key")
		}
	})
}
