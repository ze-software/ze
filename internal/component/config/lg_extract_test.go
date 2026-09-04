package config

import "testing"

// lgTree builds an enabled environment.looking-glass block. Pass tls="" to omit
// the leaf entirely, which is the case the TLS default governs.
func lgTree(tls, token string) *Tree {
	tree := NewTree()
	env := tree.GetOrCreateContainer("environment")
	lg := env.GetOrCreateContainer("looking-glass")
	lg.Set("enabled", "true")
	if tls != "" {
		lg.Set("tls", tls)
	}
	if token != "" {
		lg.Set("token", token)
	}
	srv := NewTree()
	srv.Set("ip", "0.0.0.0")
	srv.Set("port", "8443")
	lg.AddListEntry("server", "default", srv)
	return tree
}

func TestExtractLGConfigTLSDefaultOn(t *testing.T) {
	// VALIDATES: AC-5 -- the looking-glass TLS default is ON. The raw config
	// tree carries no YANG defaults (a leaf the operator did not write is simply
	// absent), so flipping the YANG default alone would change nothing: the
	// extractor must read an absent leaf as true.
	// PREVENTS: a looking glass serving plaintext because the operator wrote no
	// tls leaf, which is the MEDIUM finding this AC closes.
	t.Run("absent leaf reads true", func(t *testing.T) {
		cfg, ok := ExtractLGConfig(lgTree("", ""))
		if !ok {
			t.Fatal("expected an enabled looking-glass config")
		}
		if !cfg.TLS {
			t.Fatal("an absent tls leaf must default to TLS ON")
		}
	})

	t.Run("explicit true reads true", func(t *testing.T) {
		cfg, _ := ExtractLGConfig(lgTree("true", ""))
		if !cfg.TLS {
			t.Fatal("tls true must enable TLS")
		}
	})

	t.Run("explicit false is honored", func(t *testing.T) {
		// The documented opt-out (R-2): an operator with a plaintext
		// birdwatcher client keeps working by writing tls false.
		cfg, _ := ExtractLGConfig(lgTree("false", ""))
		if cfg.TLS {
			t.Fatal("an explicit tls false must serve plaintext")
		}
	})
}

func TestExtractLGSettingsSurviveDisabledBlock(t *testing.T) {
	// VALIDATES: a looking-glass block that does NOT say `enabled true` still
	// yields its tls and token settings. ze.looking-glass.enabled and
	// ze.looking-glass.listen start the server without that leaf, so gating the
	// settings on it threw the operator's instruction away: a block asking for
	// TLS and a bearer token produced a PLAINTEXT, OPEN looking glass.
	// PREVENTS: the environment.mcp defect (spec finding 5) repeated for the
	// looking glass -- config asks for authentication, daemon provides none
	// (ai/rules/protocol.md).
	tree := lgTree("true", "lg-s3cret")
	tree.GetContainer("environment").GetContainer("looking-glass").Set("enabled", "false")

	// The listener question still answers "no": config must not start it.
	if _, ok := ExtractLGConfig(tree); ok {
		t.Fatal("a block with enabled false must not ask for a listener")
	}

	// The settings question answers "here they are".
	cfg, ok := ExtractLGSettings(tree)
	if !ok {
		t.Fatal("settings must be available for any looking-glass block")
	}
	if !cfg.TLS || !cfg.TLSExplicit {
		t.Fatalf("explicit tls true must survive: TLS=%v TLSExplicit=%v", cfg.TLS, cfg.TLSExplicit)
	}
	if cfg.Token != "lg-s3cret" {
		t.Fatalf("token must survive: got %q", cfg.Token)
	}
}

func TestExtractLGSettingsAbsentBlock(t *testing.T) {
	// No block at all means no settings to apply, and the caller must not read
	// a zero-value config as an operator instruction.
	if _, ok := ExtractLGSettings(NewTree()); ok {
		t.Fatal("a tree with no looking-glass block has no settings")
	}
}

func TestExtractLGConfigTLSExplicitFlag(t *testing.T) {
	// VALIDATES: TLSExplicit separates "the operator demands TLS" from "TLS is
	// the default nobody overrode". It is the whole discriminator behind the
	// warned plaintext fallback when no certificate store exists, so a wrong
	// value there either bricks a working looking glass or silently downgrades
	// one the operator explicitly secured.
	absent, _ := ExtractLGConfig(lgTree("", ""))
	if absent.TLSExplicit {
		t.Fatal("an absent tls leaf is the default, not an explicit demand")
	}
	on, _ := ExtractLGConfig(lgTree("true", ""))
	if !on.TLSExplicit {
		t.Fatal("tls true is an explicit demand")
	}
	off, _ := ExtractLGConfig(lgTree("false", ""))
	if !off.TLSExplicit {
		t.Fatal("tls false is an explicit choice too")
	}
}

func TestExtractLGConfigToken(t *testing.T) {
	// VALIDATES: AC-5 -- the optional looking-glass bearer token is extracted so
	// the hub can pass it to the server. Absent means open (the LG stays a
	// public read-only surface by default).
	cfg, _ := ExtractLGConfig(lgTree("", "lg-s3cret"))
	if cfg.Token != "lg-s3cret" {
		t.Fatalf("token: got %q, want %q", cfg.Token, "lg-s3cret")
	}

	open, _ := ExtractLGConfig(lgTree("", ""))
	if open.Token != "" {
		t.Fatalf("an absent token leaf must leave the looking glass open, got %q", open.Token)
	}
}

func TestExtractLGBlockReadsCertificate(t *testing.T) {
	// VALIDATES: the environment.looking-glass certificate leaf reaches
	// LGListenConfig.Certificate, so the hub can name a PKI store entry for the
	// looking-glass HTTPS listener instead of a self-signed certificate.
	// PREVENTS: a public looking glass presenting a certificate no visitor's
	// browser accepts while the operator configured their own chain.
	t.Run("an enabled block yields the name", func(t *testing.T) {
		tree := lgTree("true", "")
		tree.GetContainer("environment").GetContainer("looking-glass").Set("certificate", "lg-cert")

		cfg, ok := ExtractLGConfig(tree)
		if !ok {
			t.Fatal("expected an enabled looking-glass config")
		}
		if cfg.Certificate != "lg-cert" {
			t.Fatalf("certificate = %q, want %q", cfg.Certificate, "lg-cert")
		}
	})

	t.Run("an absent leaf leaves the name empty", func(t *testing.T) {
		// Empty is the "unset" case the self-signed path reads, never an error.
		cfg, _ := ExtractLGConfig(lgTree("true", ""))
		if cfg.Certificate != "" {
			t.Fatalf("an absent certificate leaf must stay empty, got %q", cfg.Certificate)
		}
	})

	t.Run("the name survives enabled false", func(t *testing.T) {
		// A-3: ze.looking-glass.enabled and ze.looking-glass.listen start the
		// server without an `enabled true` leaf, and ExtractLGSettings is what
		// those deployments consult. Gating the certificate on `enabled` would
		// drop it for exactly the deployments most likely to name one.
		tree := lgTree("true", "")
		lg := tree.GetContainer("environment").GetContainer("looking-glass")
		lg.Set("certificate", "lg-cert")
		lg.Set("enabled", "false")

		cfg, ok := ExtractLGSettings(tree)
		if !ok {
			t.Fatal("settings must be available for any looking-glass block")
		}
		if cfg.Certificate != "lg-cert" {
			t.Fatalf("certificate must survive a disabled block: got %q", cfg.Certificate)
		}
	})
}
