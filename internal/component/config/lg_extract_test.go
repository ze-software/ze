package config

import "testing"

// lgTree builds an enabled environment.looking-glass block with one server
// entry. Pass tls="" to omit the entry's tls leaf entirely, which is the case
// the TLS default governs.
func lgTree(tls, token string) *Tree {
	tree := NewTree()
	env := tree.GetOrCreateContainer("environment")
	lg := env.GetOrCreateContainer("looking-glass")
	lg.Set("enabled", "true")
	if token != "" {
		lg.Set("token", token)
	}
	srv := NewTree()
	srv.Set("ip", "0.0.0.0")
	srv.Set("port", "8443")
	if tls != "" {
		srv.Set("tls", tls)
	}
	lg.AddListEntry("server", "default", srv)
	return tree
}

// lgServerTLS returns the tls state of the one server entry lgTree builds.
func lgServerTLS(t *testing.T, cfg LGListenConfig) LGServerTLS {
	t.Helper()
	if len(cfg.Servers) != 1 {
		t.Fatalf("got %d server entries, want 1", len(cfg.Servers))
	}
	return cfg.Servers[0].TLS
}

func TestExtractLGConfigTLSDefaultOn(t *testing.T) {
	// VALIDATES: AC-5 -- a looking-glass server entry asks for TLS by default.
	// The raw config tree carries no YANG defaults (a leaf the operator did not
	// write is simply absent), so the YANG default alone would change nothing:
	// the extractor must read an absent leaf as a request for TLS.
	// PREVENTS: a looking glass serving plaintext because the operator wrote no
	// tls leaf, which is the MEDIUM finding this AC closes.
	t.Run("absent leaf reads true", func(t *testing.T) {
		cfg, ok := ExtractLGConfig(lgTree("", ""))
		if !ok {
			t.Fatal("expected an enabled looking-glass config")
		}
		if got := lgServerTLS(t, cfg); got != LGServerTLSInherited {
			t.Fatalf("an absent tls leaf must default to TLS ON: got state %d, want LGServerTLSInherited", got)
		}
	})

	t.Run("explicit true reads true", func(t *testing.T) {
		cfg, _ := ExtractLGConfig(lgTree("true", ""))
		if got := lgServerTLS(t, cfg); got != LGServerTLSRequired {
			t.Fatalf("tls true must enable TLS: got state %d, want LGServerTLSRequired", got)
		}
	})

	t.Run("explicit false is honored", func(t *testing.T) {
		// The documented opt-out (R-2): an operator with a plaintext
		// birdwatcher client keeps working by writing tls false.
		cfg, _ := ExtractLGConfig(lgTree("false", ""))
		if got := lgServerTLS(t, cfg); got != LGServerTLSOff {
			t.Fatalf("an explicit tls false must serve plaintext: got state %d, want LGServerTLSOff", got)
		}
	})
}

func TestExtractLGConfigTLSPerServerEntry(t *testing.T) {
	// VALIDATES: each server entry carries its own tls leaf, read in list
	// order with the entry it was written on, so one looking glass asks for
	// HTTP on one endpoint and HTTPS on another.
	// PREVENTS: one entry's leaf deciding its sibling's protocol, and a tls
	// leaf written on the block, where the schema no longer has one, deciding
	// any entry's.
	tree := lgTree("false", "")
	lg := tree.GetContainer("environment").GetContainer("looking-glass")
	secure := NewTree()
	secure.Set("ip", "0.0.0.0")
	secure.Set("port", "443")
	lg.AddListEntry("server", "secure", secure)
	demanded := NewTree()
	demanded.Set("ip", "0.0.0.0")
	demanded.Set("port", "9443")
	demanded.Set("tls", "true")
	lg.AddListEntry("server", "demanded", demanded)
	// The schema refuses this leaf; a raw tree that holds it anyway reads as
	// if it were absent.
	lg.Set("tls", "false")

	cfg, ok := ExtractLGConfig(tree)
	if !ok {
		t.Fatal("expected an enabled looking-glass config")
	}
	if len(cfg.Servers) != 3 {
		t.Fatalf("got %d server entries, want 3", len(cfg.Servers))
	}
	want := []struct {
		port string
		tls  LGServerTLS
	}{
		{"8443", LGServerTLSOff},
		{"443", LGServerTLSInherited},
		{"9443", LGServerTLSRequired},
	}
	for i, w := range want {
		if cfg.Servers[i].Port != w.port {
			t.Fatalf("entry %d: port %q, want %q", i, cfg.Servers[i].Port, w.port)
		}
		if cfg.Servers[i].TLS != w.tls {
			t.Errorf("entry %d (port %s): tls state %d, want %d", i, w.port, cfg.Servers[i].TLS, w.tls)
		}
	}

	endpoints := cfg.Endpoints()
	if len(endpoints) != 3 {
		t.Fatalf("Endpoints: got %d, want 3", len(endpoints))
	}
	if endpoints[1].Listen() != "0.0.0.0:443" {
		t.Errorf("Endpoints[1]: got %q, want 0.0.0.0:443", endpoints[1].Listen())
	}
}

func TestExtractLGConfigDefaultEndpointInheritsTLS(t *testing.T) {
	// VALIDATES: a block that names no server yields the default endpoint with
	// the tls leaf unset, never the zero state.
	// PREVENTS: the synthesized endpoint reaching the hub as
	// LGServerTLSUnspecified, which the transport plan treats as a Ze defect.
	tree := NewTree()
	lg := tree.GetOrCreateContainer("environment").GetOrCreateContainer("looking-glass")
	lg.Set("enabled", "true")

	cfg, ok := ExtractLGConfig(tree)
	if !ok {
		t.Fatal("expected an enabled looking-glass config")
	}
	if got := lgServerTLS(t, cfg); got != LGServerTLSInherited {
		t.Fatalf("the default endpoint: got %d, want LGServerTLSInherited", got)
	}
	if cfg.Servers[0].Listen() != "0.0.0.0:8443" {
		t.Fatalf("the default endpoint: got %q, want 0.0.0.0:8443", cfg.Servers[0].Listen())
	}
}

func TestExtractLGSettingsSurviveDisabledBlock(t *testing.T) {
	// VALIDATES: a looking-glass block that does NOT say `enabled true` still
	// yields its server entries, with their tls leaf, and its token.
	// ze.looking-glass.enabled starts the server without that leaf, so gating
	// the settings on it threw the operator's instruction away: a block asking
	// for TLS and a bearer token produced a PLAINTEXT, OPEN looking glass.
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
	if got := lgServerTLS(t, cfg); got != LGServerTLSRequired {
		t.Fatalf("explicit tls true must survive: got state %d, want LGServerTLSRequired", got)
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
	// VALIDATES: the extractor separates "the operator demands TLS" from "TLS
	// is the default nobody overrode". It is the whole discriminator behind the
	// warned fallback to HTTP when no certificate store exists, so a wrong
	// value there either bricks a working looking glass or silently downgrades
	// one the operator explicitly secured.
	absent, _ := ExtractLGConfig(lgTree("", ""))
	if lgServerTLS(t, absent) != LGServerTLSInherited {
		t.Fatal("an absent tls leaf is the default, not an explicit demand")
	}
	on, _ := ExtractLGConfig(lgTree("true", ""))
	if lgServerTLS(t, on) != LGServerTLSRequired {
		t.Fatal("tls true is an explicit demand")
	}
	off, _ := ExtractLGConfig(lgTree("false", ""))
	if lgServerTLS(t, off) != LGServerTLSOff {
		t.Fatal("tls false is an explicit choice too")
	}
	// The schema admits true and false only. A raw value that is neither is
	// never read as plain HTTP.
	junk, _ := ExtractLGConfig(lgTree("maybe", ""))
	if lgServerTLS(t, junk) != LGServerTLSRequired {
		t.Fatal("a written value that is not false must demand TLS, never serve in the clear")
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
