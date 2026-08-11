package config

import "testing"

// gnmiBlockTree builds an environment.gnmi block. Pass enabled="" to omit the
// leaf entirely, and host="" to omit the server list, which is the case the
// 0.0.0.0:9339 default governs.
func gnmiBlockTree(enabled, host, token string) *Tree {
	tree := NewTree()
	env := tree.GetOrCreateContainer("environment")
	gnmi := env.GetOrCreateContainer("gnmi")
	if enabled != "" {
		gnmi.Set("enabled", enabled)
	}
	if token != "" {
		gnmi.Set("token", token)
	}
	if host != "" {
		srv := NewTree()
		srv.Set("ip", host)
		srv.Set("port", "9339")
		gnmi.AddListEntry("server", "main", srv)
	}
	return tree
}

func TestExtractGNMISettingsSurviveDisabledBlock(t *testing.T) {
	// VALIDATES: an environment.gnmi block that does NOT say `enabled true`
	// still yields its token and TLS settings. ze.gnmi.enabled starts the
	// server without that leaf (ze.gnmi.listen only names an address), so
	// gating the settings on it threw the operator's instruction away: a block
	// asking for a token
	// produced an UNAUTHENTICATED gNMI Set surface, or a boot refusal telling
	// the operator to set the token they had already written.
	// PREVENTS: the environment.mcp defect (spec finding 5) and the
	// environment.looking-glass defect repeating on the third service
	// (ai/rules/protocol.md).
	tree := gnmiBlockTree("false", "127.0.0.1", "gnmi-s3cret")
	gnmi := tree.GetContainer("environment").GetContainer("gnmi")
	tls := gnmi.GetOrCreateContainer("tls")
	tls.Set("cert", "/etc/ze/gnmi.pem")
	tls.Set("key", "/etc/ze/gnmi.key")

	// The listener question still answers "no": config must not start it.
	if _, ok := ExtractGNMIConfig(tree); ok {
		t.Fatal("a block with enabled false must not ask for a listener")
	}

	// The settings question answers "here they are".
	cfg, ok := ExtractGNMISettings(tree)
	if !ok {
		t.Fatal("settings must be available for any gnmi block")
	}
	if cfg.Token != "gnmi-s3cret" {
		t.Fatalf("token must survive: got %q", cfg.Token)
	}
	if cfg.TLS.Cert != "/etc/ze/gnmi.pem" || cfg.TLS.Key != "/etc/ze/gnmi.key" {
		t.Fatalf("tls must survive: cert=%q key=%q", cfg.TLS.Cert, cfg.TLS.Key)
	}
}

func TestExtractGNMISettingsAbsentBlock(t *testing.T) {
	// No block at all means no settings to apply, and the caller must not read a
	// zero-value config as an operator instruction.
	if _, ok := ExtractGNMISettings(NewTree()); ok {
		t.Fatal("a tree with no gnmi block has no settings")
	}
	if _, ok := ExtractGNMISettings(nil); ok {
		t.Fatal("a nil tree has no settings")
	}
}

func TestExtractGNMIConfigStillGatesOnEnabled(t *testing.T) {
	// VALIDATES: the split changes only WHICH question each function answers.
	// ExtractGNMIConfig keeps meaning "config asks for a gNMI listener", which
	// is what the offline validators and the doctor bind probes read.
	if _, ok := ExtractGNMIConfig(gnmiBlockTree("", "0.0.0.0", "")); ok {
		t.Fatal("a block with no enabled leaf must not ask for a listener")
	}
	cfg, ok := ExtractGNMIConfig(gnmiBlockTree("true", "0.0.0.0", "tok"))
	if !ok {
		t.Fatal("enabled true must ask for a listener")
	}
	if cfg.Token != "tok" || len(cfg.Servers) != 1 {
		t.Fatalf("enabled block must carry its own servers and token: %+v", cfg)
	}
}
