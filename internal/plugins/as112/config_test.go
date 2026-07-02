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
