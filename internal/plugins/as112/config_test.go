package as112

import "testing"

// VALIDATES: safe default is disabled.
func TestParseConfig_DefaultsEnabledFalse(t *testing.T) {
	cfg, err := parseConfig(`{}`)
	if err != nil {
		t.Fatalf("parseConfig: unexpected error: %v", err)
	}
	if cfg.Enabled {
		t.Fatal("default Enabled = true, want false")
	}
	if cfg.AddressFamily != addressFamilyBoth {
		t.Fatalf("default AddressFamily = %q, want %q", cfg.AddressFamily, addressFamilyBoth)
	}
}

// VALIDATES: AC-10 -- address-family restriction parses.
func TestParseConfig_AddressFamilyRestriction(t *testing.T) {
	cfg, err := parseConfig(`{"service":{"as112":{"enabled":"true","address-family":"ipv4-only"}}}`)
	if err != nil {
		t.Fatalf("parseConfig: unexpected error: %v", err)
	}
	if !cfg.Enabled {
		t.Fatal("Enabled = false, want true")
	}
	if cfg.AddressFamily != addressFamilyIPv4Only {
		t.Fatalf("AddressFamily = %q, want %q", cfg.AddressFamily, addressFamilyIPv4Only)
	}
}

// VALIDATES: A-3 boundary -- hostname length is bounded (DNS label limit, 63).
func TestParseConfig_HostnameLengthBound(t *testing.T) {
	ok := repeatString("h", maxHostnameLen)
	if _, err := parseConfig(`{"service":{"as112":{"hostname":"` + ok + `"}}}`); err != nil {
		t.Fatalf("hostname at max length (%d) rejected: %v", maxHostnameLen, err)
	}
	tooLong := repeatString("h", maxHostnameLen+1)
	if _, err := parseConfig(`{"service":{"as112":{"hostname":"` + tooLong + `"}}}`); err == nil {
		t.Fatalf("hostname over max length (%d) accepted, want rejected", maxHostnameLen+1)
	}
}

// VALIDATES: an invalid address-family value is rejected at parse time.
func TestParseConfig_AddressFamilyRejectsInvalidValue(t *testing.T) {
	if _, err := parseConfig(`{"service":{"as112":{"address-family":"ipv5-only"}}}`); err == nil {
		t.Fatal("invalid address-family accepted, want rejected")
	}
}

// VALIDATES: facility length is bounded (see maxFacilityLen doc comment).
func TestParseConfig_FacilityLengthBound(t *testing.T) {
	ok := repeatString("f", maxFacilityLen)
	if _, err := parseConfig(`{"service":{"as112":{"facility":"` + ok + `"}}}`); err != nil {
		t.Fatalf("facility at max length (%d) rejected: %v", maxFacilityLen, err)
	}
	tooLong := repeatString("f", maxFacilityLen+1)
	if _, err := parseConfig(`{"service":{"as112":{"facility":"` + tooLong + `"}}}`); err == nil {
		t.Fatalf("facility over max length (%d) accepted, want rejected", maxFacilityLen+1)
	}
}

// VALIDATES: location length is bounded (see maxLocationLen doc comment).
func TestParseConfig_LocationLengthBound(t *testing.T) {
	ok := repeatString("l", maxLocationLen)
	if _, err := parseConfig(`{"service":{"as112":{"location":"` + ok + `"}}}`); err != nil {
		t.Fatalf("location at max length (%d) rejected: %v", maxLocationLen, err)
	}
	tooLong := repeatString("l", maxLocationLen+1)
	if _, err := parseConfig(`{"service":{"as112":{"location":"` + tooLong + `"}}}`); err == nil {
		t.Fatalf("location over max length (%d) accepted, want rejected", maxLocationLen+1)
	}
}

// VALIDATES: AC-14/15 -- allow-from parses a list of prefixes.
func TestParseConfig_AllowFromParses(t *testing.T) {
	cfg, err := parseConfig(`{"service":{"as112":{"allow-from":["10.0.0.0/8","2001:db8::/32"]}}}`)
	if err != nil {
		t.Fatalf("parseConfig: unexpected error: %v", err)
	}
	if len(cfg.AllowFrom) != 2 {
		t.Fatalf("AllowFrom = %v, want 2 entries", cfg.AllowFrom)
	}
}

// VALIDATES: a malformed allow-from prefix is rejected at parse time.
func TestParseConfig_AllowFromRejectsBadPrefix(t *testing.T) {
	if _, err := parseConfig(`{"service":{"as112":{"allow-from":["not-a-prefix"]}}}`); err == nil {
		t.Fatal("malformed allow-from prefix accepted, want rejected")
	}
}

// VALIDATES: AC-3 -- DoT defaults to disabled with port 853, DoH to disabled
// with port 443 and path /dns-query.
func TestParseConfig_SecureDefaults(t *testing.T) {
	cfg, err := parseConfig(`{}`)
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}
	if cfg.Secure.DoTEnabled || cfg.Secure.DoHEnabled {
		t.Fatalf("DoT/DoH default enabled, want disabled: %+v", cfg.Secure)
	}
	if cfg.Secure.DoTPort != 853 {
		t.Fatalf("DoT default port = %d, want 853", cfg.Secure.DoTPort)
	}
	if cfg.Secure.DoHPort != 443 {
		t.Fatalf("DoH default port = %d, want 443", cfg.Secure.DoHPort)
	}
	if cfg.Secure.DoHPath != "/dns-query" {
		t.Fatalf("DoH default path = %q, want /dns-query", cfg.Secure.DoHPath)
	}
}

// VALIDATES: AC-3/AC-4 -- tls and doh containers parse enable, ports, path, and
// cert material.
func TestParseConfig_SecureParses(t *testing.T) {
	cfg, err := parseConfig(`{"service":{"as112":{"tls":{"enabled":"true","listen-port":"8853","cert-file":"/etc/ze/dns.pem","key-file":"/etc/ze/dns.key"},"doh":{"enabled":"true","listen-port":"8443","path":"/q"}}}}`)
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}
	if !cfg.Secure.DoTEnabled || cfg.Secure.DoTPort != 8853 {
		t.Fatalf("DoT = {%v,%d}, want {true,8853}", cfg.Secure.DoTEnabled, cfg.Secure.DoTPort)
	}
	if cfg.Secure.CertFile != "/etc/ze/dns.pem" || cfg.Secure.KeyFile != "/etc/ze/dns.key" {
		t.Fatalf("cert material = {%q,%q}", cfg.Secure.CertFile, cfg.Secure.KeyFile)
	}
	if !cfg.Secure.DoHEnabled || cfg.Secure.DoHPort != 8443 || cfg.Secure.DoHPath != "/q" {
		t.Fatalf("DoH = {%v,%d,%q}", cfg.Secure.DoHEnabled, cfg.Secure.DoHPort, cfg.Secure.DoHPath)
	}
}

// VALIDATES: boundary -- DoT/DoH port 65535 is accepted, 0 is rejected.
func TestParseConfig_SecurePortBoundary(t *testing.T) {
	if _, err := parseConfig(`{"service":{"as112":{"tls":{"listen-port":"65535"}}}}`); err != nil {
		t.Fatalf("DoT port 65535 rejected: %v", err)
	}
	if _, err := parseConfig(`{"service":{"as112":{"tls":{"listen-port":"0"}}}}`); err == nil {
		t.Fatal("DoT port 0 accepted, want rejected")
	}
	if _, err := parseConfig(`{"service":{"as112":{"doh":{"listen-port":"0"}}}}`); err == nil {
		t.Fatal("DoH port 0 accepted, want rejected")
	}
}
