package sysctl

import (
	"testing"
)

func TestKnownKeyRegistration(t *testing.T) {
	// VALIDATES: AC-15 -- Known keys contributed by plugins at init, retrievable via Lookup/All.
	// PREVENTS: Keys silently lost after registration.
	ResetRegistry()
	t.Cleanup(ResetRegistry)

	MustRegister(KeyDef{
		Name:        "net.ipv4.conf.all.forwarding",
		Type:        TypeBool,
		Description: "Enable IPv4 forwarding globally",
		Platform:    PlatformLinux,
	})

	k, ok := Lookup("net.ipv4.conf.all.forwarding")
	if !ok {
		t.Fatal("registered key not found via Lookup")
	}
	if k.Name != "net.ipv4.conf.all.forwarding" {
		t.Errorf("Name: got %q, want %q", k.Name, "net.ipv4.conf.all.forwarding")
	}
	if k.Type != TypeBool {
		t.Errorf("Type: got %v, want %v", k.Type, TypeBool)
	}
	if k.Description != "Enable IPv4 forwarding globally" {
		t.Errorf("Description: got %q", k.Description)
	}
	if k.Platform != PlatformLinux {
		t.Errorf("Platform: got %v, want %v", k.Platform, PlatformLinux)
	}

	all := All()
	if len(all) != 1 {
		t.Fatalf("All() returned %d keys, want 1", len(all))
	}
	if all[0].Name != "net.ipv4.conf.all.forwarding" {
		t.Errorf("All()[0].Name: got %q", all[0].Name)
	}
}

func TestKnownKeyRegistrationMultiple(t *testing.T) {
	// VALIDATES: Multiple keys from different contributors register correctly.
	// PREVENTS: Second registration overwriting first.
	ResetRegistry()
	t.Cleanup(ResetRegistry)

	MustRegister(KeyDef{
		Name:        "net.ipv4.conf.all.forwarding",
		Type:        TypeBool,
		Description: "IPv4 forwarding",
		Platform:    PlatformLinux,
	})
	MustRegister(KeyDef{
		Name:        "net.ipv6.conf.all.forwarding",
		Type:        TypeBool,
		Description: "IPv6 forwarding",
		Platform:    PlatformLinux,
	})

	all := All()
	if len(all) != 2 {
		t.Fatalf("All() returned %d keys, want 2", len(all))
	}

	if _, ok := Lookup("net.ipv4.conf.all.forwarding"); !ok {
		t.Error("ipv4 forwarding key not found")
	}
	if _, ok := Lookup("net.ipv6.conf.all.forwarding"); !ok {
		t.Error("ipv6 forwarding key not found")
	}
}

func TestDuplicateRegistration(t *testing.T) {
	// VALIDATES: AC-15 -- Duplicate registration panics (programming bug, not runtime error).
	// PREVENTS: Silent overwrite of a key definition from another contributor.
	ResetRegistry()
	t.Cleanup(ResetRegistry)

	MustRegister(KeyDef{
		Name:        "net.ipv4.conf.all.forwarding",
		Type:        TypeBool,
		Description: "IPv4 forwarding",
		Platform:    PlatformLinux,
	})

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic on duplicate registration, got nil")
		}
	}()

	MustRegister(KeyDef{
		Name:        "net.ipv4.conf.all.forwarding",
		Type:        TypeBool,
		Description: "duplicate",
		Platform:    PlatformLinux,
	})
}

func TestLookupUnknown(t *testing.T) {
	// VALIDATES: AC-9 -- Unknown keys are simply not in the known registry.
	// PREVENTS: False positive lookups.
	ResetRegistry()
	t.Cleanup(ResetRegistry)

	if _, ok := Lookup("net.nonexistent.key"); ok {
		t.Error("expected Lookup to return false for unknown key")
	}
}

func TestTemplateKeyRegistration(t *testing.T) {
	// VALIDATES: Per-interface key templates with <iface> placeholder.
	// PREVENTS: Template keys not matching concrete interface names.
	ResetRegistry()
	t.Cleanup(ResetRegistry)

	MustRegister(KeyDef{
		Name:        "net.ipv4.conf.<iface>.forwarding",
		Type:        TypeBool,
		Description: "Per-interface IPv4 forwarding",
		Platform:    PlatformLinux,
		Template:    true,
	})

	// Direct lookup by template name works.
	if _, ok := Lookup("net.ipv4.conf.<iface>.forwarding"); !ok {
		t.Error("template key not found by template name")
	}

	// MatchTemplate resolves a concrete key against templates.
	k, ok := MatchTemplate("net.ipv4.conf.eth0.forwarding")
	if !ok {
		t.Fatal("MatchTemplate did not resolve concrete key")
	}
	if k.Name != "net.ipv4.conf.<iface>.forwarding" {
		t.Errorf("MatchTemplate returned %q, want template name", k.Name)
	}

	// Non-matching concrete key returns false.
	if _, ok := MatchTemplate("net.ipv4.tcp_syncookies"); ok {
		t.Error("MatchTemplate should not match a non-template key")
	}
}

func TestValidate(t *testing.T) {
	// VALIDATES: AC-11 -- Known key invalid value rejected.
	// PREVENTS: Invalid values passing through for known keys.
	ResetRegistry()
	t.Cleanup(ResetRegistry)

	MustRegister(KeyDef{
		Name:        "net.ipv4.conf.all.rp_filter",
		Type:        TypeIntRange,
		Min:         0,
		Max:         2,
		Description: "Reverse path filter",
		Platform:    PlatformLinux,
	})
	MustRegister(KeyDef{
		Name:        "net.ipv4.conf.all.forwarding",
		Type:        TypeBool,
		Description: "IPv4 forwarding",
		Platform:    PlatformLinux,
	})

	tests := []struct {
		name    string
		key     string
		value   string
		wantErr bool
	}{
		{"bool true", "net.ipv4.conf.all.forwarding", "1", false},
		{"bool false", "net.ipv4.conf.all.forwarding", "0", false},
		{"bool invalid", "net.ipv4.conf.all.forwarding", "2", true},
		{"bool non-numeric", "net.ipv4.conf.all.forwarding", "yes", true},
		{"range low", "net.ipv4.conf.all.rp_filter", "0", false},
		{"range high", "net.ipv4.conf.all.rp_filter", "2", false},
		{"range over", "net.ipv4.conf.all.rp_filter", "3", true},
		{"range under", "net.ipv4.conf.all.rp_filter", "-1", true},
		{"range non-numeric", "net.ipv4.conf.all.rp_filter", "abc", true},
		{"unknown key", "net.unknown.key", "anything", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.key, tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate(%q, %q) error = %v, wantErr %v", tt.key, tt.value, err, tt.wantErr)
			}
		})
	}
}
