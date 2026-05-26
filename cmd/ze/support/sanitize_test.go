// Design: docs/architecture/core-design.md — config sanitization tests

package support

import (
	"testing"
)

func mustMap(t *testing.T, v any) map[string]any {
	t.Helper()
	m, ok := v.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", v)
	}
	return m
}

func mustSlice(t *testing.T, v any) []any {
	t.Helper()
	s, ok := v.([]any)
	if !ok {
		t.Fatalf("expected []any, got %T", v)
	}
	return s
}

func TestConfigSanitizer_PasswordsRedacted(t *testing.T) {
	tree := map[string]any{
		"hostname": "router1",
		"bgp": map[string]any{
			"neighbor": map[string]any{
				"password":       "secret123",
				"remote-as":      65001,
				"pre-shared-key": "psk-value",
			},
		},
		"tacacs": map[string]any{
			"shared-secret": "tacacs-secret",
			"server":        "10.0.0.1",
		},
		"ssh": map[string]any{
			"private-key": "-----BEGIN RSA PRIVATE KEY-----",
			"port":        22,
		},
	}

	result := sanitizeConfig(tree)

	if result["hostname"] != "router1" {
		t.Errorf("hostname should not be redacted, got %v", result["hostname"])
	}

	bgp := mustMap(t, result["bgp"])
	neighbor := mustMap(t, bgp["neighbor"])
	if neighbor["password"] != redactedValue {
		t.Errorf("password should be redacted, got %v", neighbor["password"])
	}
	if neighbor["remote-as"] != 65001 {
		t.Errorf("remote-as should not be redacted, got %v", neighbor["remote-as"])
	}
	if neighbor["pre-shared-key"] != redactedValue {
		t.Errorf("pre-shared-key should be redacted, got %v", neighbor["pre-shared-key"])
	}

	tacacs := mustMap(t, result["tacacs"])
	if tacacs["shared-secret"] != redactedValue {
		t.Errorf("shared-secret should be redacted, got %v", tacacs["shared-secret"])
	}
	if tacacs["server"] != "10.0.0.1" {
		t.Errorf("server should not be redacted, got %v", tacacs["server"])
	}

	ssh := mustMap(t, result["ssh"])
	if ssh["private-key"] != redactedValue {
		t.Errorf("private-key should be redacted, got %v", ssh["private-key"])
	}
	if ssh["port"] != 22 {
		t.Errorf("port should not be redacted, got %v", ssh["port"])
	}
}

func TestConfigSanitizer_SensitiveMode(t *testing.T) {
	tree := map[string]any{
		"bgp": map[string]any{
			"password": "secret123",
		},
	}

	result := sanitizeConfig(tree)
	bgp := mustMap(t, result["bgp"])
	if bgp["password"] != redactedValue {
		t.Error("sanitizeConfig should redact password")
	}

	bgpRaw := mustMap(t, tree["bgp"])
	if bgpRaw["password"] != "secret123" {
		t.Error("original tree should not be modified")
	}
}

func TestConfigSanitizer_NestedSlices(t *testing.T) {
	tree := map[string]any{
		"servers": []any{
			map[string]any{
				"address":  "10.0.0.1",
				"password": "pass1",
			},
			map[string]any{
				"address":  "10.0.0.2",
				"password": "pass2",
			},
		},
	}

	result := sanitizeConfig(tree)
	servers := mustSlice(t, result["servers"])
	for i, s := range servers {
		srv := mustMap(t, s)
		if srv["password"] != redactedValue {
			t.Errorf("servers[%d].password should be redacted, got %v", i, srv["password"])
		}
		if srv["address"] == redactedValue {
			t.Errorf("servers[%d].address should not be redacted", i)
		}
	}
}

func TestConfigSanitizer_OriginalUnmodified(t *testing.T) {
	tree := map[string]any{
		"password": "original",
		"nested": map[string]any{
			"secret": "nested-original",
		},
	}

	_ = sanitizeConfig(tree)

	if tree["password"] != "original" {
		t.Error("original tree was modified")
	}
	nested := mustMap(t, tree["nested"])
	if nested["secret"] != "nested-original" {
		t.Error("original nested map was modified")
	}
}

func TestIsSensitiveKey(t *testing.T) {
	sensitive := []string{
		"password", "Password", "PASSWORD",
		"plaintext-password",
		"tacacs-password",
		"secret", "shared-secret", "pre-shared-secret", "radius-secret",
		"pre-shared-key", "private-key", "key", "auth-key",
		"passphrase",
		"token", "Token",
		"ssh-password-hash",
	}
	for _, k := range sensitive {
		if !isSensitiveKey(k) {
			t.Errorf("expected %q to be sensitive", k)
		}
	}

	safe := []string{
		"hostname", "address", "port", "remote-as",
		"description", "interface", "vlan-id",
		"community",
		"monkey", "keyboard", "turkey", "donkey-bridge",
		"ssh-host-key-algorithm", "host-key",
	}
	for _, k := range safe {
		if isSensitiveKey(k) {
			t.Errorf("expected %q to be safe, but it was flagged as sensitive", k)
		}
	}
}
