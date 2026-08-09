package config

import "testing"

// webTree builds an environment.web block. Pass certificate="" to omit the leaf
// entirely, which is the self-signed case.
func webTree(enabled, certificate string) *Tree {
	tree := NewTree()
	envBlock := tree.GetOrCreateContainer("environment")
	web := envBlock.GetOrCreateContainer("web")
	web.Set("enabled", enabled)
	if certificate != "" {
		web.Set("certificate", certificate)
	}
	srv := NewTree()
	srv.Set("ip", "0.0.0.0")
	srv.Set("port", "3443")
	web.AddListEntry("server", "default", srv)
	return tree
}

func TestExtractWebConfigCertificate(t *testing.T) {
	// VALIDATES: AC-1 wiring row -- the environment.web.certificate leaf reaches
	// WebListenConfig.Certificate, which is what cmd/ze/hub selects the PKI TLS
	// path on. Without this the leaf parses and is silently dropped.
	t.Run("leaf reaches the struct", func(t *testing.T) {
		cfg, ok := ExtractWebConfig(webTree("true", "lan"))
		if !ok {
			t.Fatal("expected an enabled web config")
		}
		if cfg.Certificate != "lan" {
			t.Fatalf("Certificate = %q, want %q", cfg.Certificate, "lan")
		}
	})

	t.Run("absent leaf leaves it empty", func(t *testing.T) {
		// AC-2: an empty Certificate is what selects the self-signed path, so it
		// must never be filled in with a default.
		cfg, ok := ExtractWebConfig(webTree("true", ""))
		if !ok {
			t.Fatal("expected an enabled web config")
		}
		if cfg.Certificate != "" {
			t.Fatalf("Certificate = %q, want empty", cfg.Certificate)
		}
	})
}

func TestExtractWebSettingsSurviveDisabledBlock(t *testing.T) {
	// VALIDATES: a web block that does NOT say `enabled true` still carries the
	// operator's certificate choice to a listener started by something else.
	// PREVENTS: the third instance of the enabled-gate shape, where a service
	// extractor parses its settings PAST the `enabled` check and so discards
	// them whenever something else starts the listener. cmd/ze/hub/main.go sets
	// webEnabled from the --web flag, ze.web.listen, and ze.web.enabled, none of
	// which consult this block; gating `certificate` on `enabled` would silently
	// serve a self-signed certificate to an operator who asked for their own chain.
	tree := webTree("false", "lan")

	if _, ok := ExtractWebConfig(tree); ok {
		t.Fatal("ExtractWebConfig must report false: config starts no listener")
	}

	cfg, ok := ExtractWebSettings(tree)
	if !ok {
		t.Fatal("ExtractWebSettings must report true: the block exists")
	}
	if cfg.Certificate != "lan" {
		t.Fatalf("Certificate = %q, want %q -- settings must survive the enabled gate", cfg.Certificate, "lan")
	}
}

func TestExtractWebSettingsAbsentBlock(t *testing.T) {
	// A tree with no environment.web block reports present=false, so the hub can
	// tell "operator wrote no web block" from "operator wrote enabled false".
	tree := NewTree()
	if _, ok := ExtractWebSettings(tree); ok {
		t.Fatal("ExtractWebSettings must report false when no web block exists")
	}
}
