// Design: docs/architecture/pki/pki-store.md -- candidate sections are parsed before apply
package pki

import (
	"encoding/base64"
	"encoding/json"
	"testing"
)

// Removal is an explicit empty object; malformed or wrongly wrapped sections
// cannot silently parse as an empty certificate store.
func TestParseJSONRequiresPKIEnvelope(t *testing.T) {
	for _, data := range []string{`{}`, `{"pki":{}}`} {
		cfg, err := ParseJSON(data)
		if err != nil {
			t.Fatalf("valid removal %s: %v", data, err)
		}
		if len(cfg.CACerts) != 0 || len(cfg.Certificates) != 0 {
			t.Fatalf("removal retained certificate entries: %+v", cfg)
		}
	}
	for _, data := range []string{`null`, `[]`, `{"other":{}}`, `{"pki":{},"other":{}}`, `{"pki":"broken"}`} {
		if _, err := ParseJSON(data); err == nil {
			t.Fatalf("malformed candidate %s was accepted", data)
		}
	}
	if _, err := ParseJSON(`{"pki":{"ca":{"test":{"certificate":"not-a-certificate"}}}}`); err == nil {
		t.Fatal("a malformed CA entry disappeared during keyed-list reconstruction")
	}
}

// Candidate verification must reject malformed material even when a leaf-list
// has only one member and the plugin transport represents it as a string.
func TestParseJSONRefusesInvalidSingletonMaterial(t *testing.T) {
	caKey, caDER := testCACertDER(t)
	key, certDER := testDeviceCertDER(t, caKey, caDER)
	caValue := base64.StdEncoding.EncodeToString(caDER)
	certValue := base64.StdEncoding.EncodeToString(certDER)
	keyValue := marshalKeyB64(t, key)
	tree := makePKITree(t, caValue, certValue, keyValue)
	data, err := json.Marshal(tree.ToPluginMap())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseJSON(string(data)); err != nil {
		t.Fatalf("valid certificate configuration was refused: %v", err)
	}
	for _, tc := range []struct {
		group string
		entry string
		leaf  string
	}{
		{group: "ca", entry: "test-ca", leaf: "crl"},
		{group: "certificate", entry: "dev-1", leaf: "intermediate"},
	} {
		t.Run(tc.leaf, func(t *testing.T) {
			tree := makePKITree(t, caValue, certValue, keyValue)
			tree.GetContainer("pki").GetList(tc.group)[tc.entry].SetSlice(tc.leaf, []string{"not-DER"})
			data, err := json.Marshal(tree.ToPluginMap())
			if err != nil {
				t.Fatal(err)
			}
			if _, err := ParseJSON(string(data)); err == nil {
				t.Fatalf("malformed singleton %s disappeared during candidate verification", tc.leaf)
			}
		})
	}
}
