package role

import (
	"encoding/binary"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
)

// buildTestAttrs creates raw attributes with ORIGIN + optional OTC.
func buildTestAttrs(otcASN uint32) []byte {
	// ORIGIN attribute: flags=0x40, type=1, len=1, value=0 (IGP)
	attrs := []byte{0x40, 0x01, 0x01, 0x00}
	if otcASN > 0 {
		otc := buildOTCAttr(otcASN)
		attrs = append(attrs, otc[:]...)
	}
	return attrs
}

// buildMultiAttrWithOTC builds ORIGIN + AS_PATH + NEXT_HOP + OTC to test OTC not being first.
func buildMultiAttrWithOTC(otcASN uint32) []byte {
	// ORIGIN: flags=0x40 type=1 len=1 val=0
	attrs := []byte{0x40, 0x01, 0x01, 0x00}
	// AS_PATH (empty) + NEXT_HOP 10.0.0.1
	attrs = append(attrs, 0x40, 0x02, 0x00, 0x40, 0x03, 0x04, 10, 0, 0, 1)
	// OTC
	otc := buildOTCAttr(otcASN)
	attrs = append(attrs, otc[:]...)
	return attrs
}

// buildTestPayload creates a minimal UPDATE payload with given attrs.
// Format: withdrawnLen(2) + withdrawn + attrLen(2) + attrs + nlri.
func buildTestPayload(attrs, nlri []byte) []byte {
	payload := make([]byte, 2+2+len(attrs)+len(nlri))
	// withdrawnLen = 0
	binary.BigEndian.PutUint16(payload[0:2], 0)
	// attrLen
	binary.BigEndian.PutUint16(payload[2:4], uint16(len(attrs)))
	copy(payload[4:], attrs)
	copy(payload[4+len(attrs):], nlri)
	return payload
}

// --- findOTC tests ---

func TestFindOTC(t *testing.T) {
	tests := []struct {
		name          string
		attrs         []byte
		wantASN       uint32
		wantFound     bool
		wantMalformed bool
	}{
		// Basic cases
		{"no_otc", buildTestAttrs(0), 0, false, false},
		{"otc_present", buildTestAttrs(65001), 65001, true, false},
		{"empty_attrs", nil, 0, false, false},
		{"zero_length_attrs", []byte{}, 0, false, false},

		// OTC position: not first attribute
		{"otc_after_multiple_attrs", buildMultiAttrWithOTC(65001), 65001, true, false},

		// OTC only (no preceding attributes)
		{"otc_only", func() []byte {
			otc := buildOTCAttr(65001)
			return otc[:]
		}(), 65001, true, false},

		// Malformed OTC: wrong lengths
		{"malformed_len_0", []byte{0xC0, 35, 0}, 0, false, true},
		{"malformed_len_1", []byte{0xC0, 35, 1, 0x00}, 0, false, true},
		{"malformed_len_2", []byte{0xC0, 35, 2, 0x00, 0x01}, 0, false, true},
		{"malformed_len_3", []byte{0xC0, 35, 3, 0x00, 0x01, 0x02}, 0, false, true},
		{"malformed_len_5", []byte{0xC0, 35, 5, 0x00, 0x01, 0x02, 0x03, 0x04}, 0, false, true},

		// Malformed OTC after valid ORIGIN
		{"malformed_after_origin", []byte{
			0x40, 0x01, 0x01, 0x00, // ORIGIN
			0xC0, 35, 3, 0x00, 0x01, 0x02, // OTC len=3 (malformed)
		}, 0, false, true},

		// Truncated data: header present but value missing
		{"truncated_header_only", []byte{0xC0, 35}, 0, false, false},
		{"truncated_no_value", []byte{0xC0, 35, 4}, 0, false, false},
		{"truncated_partial_value", []byte{0xC0, 35, 4, 0x00, 0x01}, 0, false, false},

		// Extended length flag (bit 0x10 set)
		{"extended_length_otc", func() []byte {
			// flags=0xD0 (Optional+Transitive+ExtLen), type=35, len=0x0004, value
			buf := []byte{0xD0, 35, 0x00, 0x04, 0x00, 0x00, 0xFD, 0xE9} // ASN=65001
			return buf
		}(), 65001, true, false},

		// Extended length with wrong size
		{"extended_length_malformed", []byte{
			0xD0, 35, 0x00, 0x03, 0x00, 0x01, 0x02, // ExtLen OTC with len=3
		}, 0, false, true},

		// Non-OTC attribute with type code 35-adjacent (34, 36)
		{"attr_code_34_not_otc", []byte{0xC0, 34, 4, 0x00, 0x00, 0xFD, 0xE9}, 0, false, false},
		{"attr_code_36_not_otc", []byte{0xC0, 36, 4, 0x00, 0x00, 0xFD, 0xE9}, 0, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			asn, found, malformed := findOTC(tt.attrs)
			assert.Equal(t, tt.wantASN, asn, "ASN mismatch")
			assert.Equal(t, tt.wantFound, found, "found mismatch")
			assert.Equal(t, tt.wantMalformed, malformed, "malformed mismatch")
		})
	}
}

// --- buildOTCAttr tests ---

func TestBuildOTCAttr(t *testing.T) {
	tests := []struct {
		name string
		asn  uint32
	}{
		{"typical", 65001},
		{"min_asn", 1},
		{"max_asn", 4294967295},
		{"zero_asn", 0},
		{"4byte_asn", 100000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			otc := buildOTCAttr(tt.asn)
			assert.Equal(t, byte(0xC0), otc[0], "flags: Optional+Transitive")
			assert.Equal(t, byte(35), otc[1], "type code: OTC")
			assert.Equal(t, byte(4), otc[2], "length: 4 bytes")
			asn := binary.BigEndian.Uint32(otc[3:])
			assert.Equal(t, tt.asn, asn, "ASN value round-trip")
		})
	}
}

// --- appendOTCToAttrs tests ---

func TestAppendOTCToAttrs(t *testing.T) {
	t.Run("appends_to_existing", func(t *testing.T) {
		orig := []byte{0x40, 0x01, 0x01, 0x00}
		result := appendOTCToAttrs(orig, 65001)
		assert.Len(t, result, len(orig)+otcWireLen)
		assert.Equal(t, orig, result[:len(orig)])
		asn, found, _ := findOTC(result)
		assert.True(t, found)
		assert.Equal(t, uint32(65001), asn)
	})

	t.Run("appends_to_empty", func(t *testing.T) {
		result := appendOTCToAttrs(nil, 42)
		assert.Len(t, result, otcWireLen)
		asn, found, _ := findOTC(result)
		assert.True(t, found)
		assert.Equal(t, uint32(42), asn)
	})

	t.Run("does_not_mutate_original", func(t *testing.T) {
		orig := []byte{0x40, 0x01, 0x01, 0x00}
		origCopy := make([]byte, len(orig))
		copy(origCopy, orig)
		_ = appendOTCToAttrs(orig, 65001)
		assert.Equal(t, origCopy, orig, "original slice must not be modified")
	})
}

// --- extractAttrsFromPayload tests ---

func TestExtractAttrsFromPayload(t *testing.T) {
	tests := []struct {
		name    string
		payload []byte
		wantLen int
		wantNil bool
	}{
		{"valid_with_attrs", buildTestPayload(buildTestAttrs(0), nil), 4, false},
		{"valid_with_otc", buildTestPayload(buildTestAttrs(65001), nil), 4 + otcWireLen, false},
		{"empty_attrs", buildTestPayload(nil, nil), 0, false},
		{"too_short", []byte{0x00}, 0, true},
		{"nil_payload", nil, 0, true},
		{"withdrawn_overflows", []byte{0x00, 0xFF, 0x00, 0x00}, 0, true},
		{"with_nlri", buildTestPayload(buildTestAttrs(0), []byte{24, 10, 0, 0}), 4, false},
		{"with_withdrawn", func() []byte {
			withdrawn := []byte{24, 10, 0, 0}
			attrs := buildTestAttrs(0)
			p := make([]byte, 2+len(withdrawn)+2+len(attrs))
			binary.BigEndian.PutUint16(p[0:2], uint16(len(withdrawn)))
			copy(p[2:], withdrawn)
			binary.BigEndian.PutUint16(p[2+len(withdrawn):], uint16(len(attrs)))
			copy(p[4+len(withdrawn):], attrs)
			return p
		}(), 4, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attrs := extractAttrsFromPayload(tt.payload)
			if tt.wantNil {
				assert.Nil(t, attrs)
			} else {
				require.NotNil(t, attrs)
				assert.Len(t, attrs, tt.wantLen)
			}
		})
	}
}

// --- insertOTCInPayload tests ---

func TestInsertOTCInPayload(t *testing.T) {
	t.Run("inserts_otc_and_updates_attrlen", func(t *testing.T) {
		origAttrs := buildTestAttrs(0)
		payload := buildTestPayload(origAttrs, nil)
		modified := insertOTCInPayload(payload, 65001)

		assert.Len(t, modified, len(payload)+otcWireLen)
		newAttrLen := binary.BigEndian.Uint16(modified[2:4])
		assert.Equal(t, uint16(len(origAttrs)+otcWireLen), newAttrLen)

		attrs := extractAttrsFromPayload(modified)
		asn, found, _ := findOTC(attrs)
		assert.True(t, found)
		assert.Equal(t, uint32(65001), asn)
	})

	t.Run("preserves_nlri_section", func(t *testing.T) {
		nlri := []byte{24, 10, 0, 0}
		payload := buildTestPayload(buildTestAttrs(0), nlri)
		modified := insertOTCInPayload(payload, 65001)
		tail := modified[len(modified)-len(nlri):]
		assert.Equal(t, nlri, tail, "NLRI must be preserved")
	})

	t.Run("preserves_withdrawn_section", func(t *testing.T) {
		withdrawn := []byte{24, 192, 168, 0}
		attrs := buildTestAttrs(0)
		payload := make([]byte, 2+len(withdrawn)+2+len(attrs))
		binary.BigEndian.PutUint16(payload[0:2], uint16(len(withdrawn)))
		copy(payload[2:], withdrawn)
		binary.BigEndian.PutUint16(payload[2+len(withdrawn):], uint16(len(attrs)))
		copy(payload[4+len(withdrawn):], attrs)

		modified := insertOTCInPayload(payload, 65001)
		wLen := binary.BigEndian.Uint16(modified[0:2])
		assert.Equal(t, uint16(len(withdrawn)), wLen)
		assert.Equal(t, withdrawn, modified[2:2+wLen])
	})

	t.Run("round_trip", func(t *testing.T) {
		payload := buildTestPayload(buildTestAttrs(0), nil)
		attrs := extractAttrsFromPayload(payload)
		_, found, _ := findOTC(attrs)
		assert.False(t, found, "no OTC initially")

		modified := insertOTCInPayload(payload, 99999)
		attrs = extractAttrsFromPayload(modified)
		asn, found, malformed := findOTC(attrs)
		assert.True(t, found)
		assert.False(t, malformed)
		assert.Equal(t, uint32(99999), asn)
	})

	t.Run("short_payload_unchanged", func(t *testing.T) {
		short := []byte{0x00}
		assert.Equal(t, short, insertOTCInPayload(short, 65001))
	})

	t.Run("nil_payload_unchanged", func(t *testing.T) {
		assert.Nil(t, insertOTCInPayload(nil, 65001))
	})

	t.Run("uint16_overflow_returns_nil", func(t *testing.T) {
		// Build payload with attrs close to uint16 max.
		// attrLen = 65530, adding OTC (7 bytes) = 65537 > 65535.
		largeAttrs := make([]byte, 65530)
		largeAttrs[0] = 0x40 // ORIGIN flags
		largeAttrs[1] = 0x01 // ORIGIN code
		largeAttrs[2] = 0x01 // ORIGIN len
		payload := buildTestPayload(largeAttrs, nil)
		result := insertOTCInPayload(payload, 65001)
		assert.Nil(t, result, "uint16 overflow should return nil")
	})

	t.Run("just_under_overflow_succeeds", func(t *testing.T) {
		// attrLen = 65528, adding OTC (7 bytes) = 65535 = exactly uint16 max.
		attrs := make([]byte, 65528)
		attrs[0] = 0x40
		attrs[1] = 0x01
		attrs[2] = 0x01
		payload := buildTestPayload(attrs, nil)
		result := insertOTCInPayload(payload, 65001)
		require.NotNil(t, result, "exactly at uint16 max should succeed")
		assert.Len(t, result, len(payload)+otcWireLen)
	})
}

// --- checkOTCIngress comprehensive tests ---

func TestCheckOTCIngress(t *testing.T) {
	attrsNoOTC := buildTestAttrs(0)
	attrsOTC65001 := buildTestAttrs(65001)
	malformed := []byte{0x40, 0x01, 0x01, 0x00, 0xC0, 35, 3, 0x00, 0x01, 0x02}

	tests := []struct {
		name       string
		remoteRole string
		remoteASN  uint32
		attrs      []byte
		wantResult int
		wantStamp  uint32
	}{
		// Stamp: no OTC from Provider/Peer/RS
		{"stamp_from_provider", roleProvider, 65001, attrsNoOTC, otcAccept, 65001},
		{"stamp_from_peer", rolePeer, 65002, attrsNoOTC, otcAccept, 65002},
		{"stamp_from_rs", roleRS, 65003, attrsNoOTC, otcAccept, 65003},

		// No stamp: no OTC from Customer/RS-Client (legitimate, not upstream)
		{"no_stamp_from_customer", roleCustomer, 65001, attrsNoOTC, otcAccept, 0},
		{"no_stamp_from_rs_client", roleRSClient, 65001, attrsNoOTC, otcAccept, 0},

		// Reject leak: OTC from Customer/RS-Client
		{"reject_customer_with_otc", roleCustomer, 65001, attrsOTC65001, otcRejectLeak, 0},
		{"reject_rs_client_with_otc", roleRSClient, 65001, attrsOTC65001, otcRejectLeak, 0},

		// Peer OTC: match vs mismatch
		{"accept_peer_otc_matches", rolePeer, 65001, attrsOTC65001, otcAccept, 0},
		{"reject_peer_otc_wrong", rolePeer, 65002, attrsOTC65001, otcRejectLeak, 0},
		{"reject_peer_otc_zero_asn", rolePeer, 0, attrsOTC65001, otcRejectLeak, 0},

		// Provider/RS with OTC already: accept, no re-stamp
		{"accept_provider_has_otc", roleProvider, 65001, attrsOTC65001, otcAccept, 0},
		{"accept_rs_has_otc", roleRS, 65001, attrsOTC65001, otcAccept, 0},

		// Malformed: treat-as-withdraw for all roles
		{"malformed_provider", roleProvider, 65001, malformed, otcTreatWithdraw, 0},
		{"malformed_customer", roleCustomer, 65001, malformed, otcTreatWithdraw, 0},
		{"malformed_peer", rolePeer, 65001, malformed, otcTreatWithdraw, 0},
		{"malformed_rs", roleRS, 65001, malformed, otcTreatWithdraw, 0},
		{"malformed_rs_client", roleRSClient, 65001, malformed, otcTreatWithdraw, 0},

		// No remote role: passthrough
		{"empty_role_no_otc", "", 65001, attrsNoOTC, otcAccept, 0},
		{"empty_role_with_otc", "", 65001, attrsOTC65001, otcAccept, 0},

		// Empty/nil attrs from Provider: still triggers stamp (no OTC found = needs stamping).
		{"nil_attrs_provider", roleProvider, 65001, nil, otcAccept, 65001},
		{"empty_attrs_provider", roleProvider, 65001, []byte{}, otcAccept, 65001},
		// Empty/nil attrs from Customer: no stamp needed.
		{"nil_attrs_customer", roleCustomer, 65001, nil, otcAccept, 0},
		{"empty_attrs_customer", roleCustomer, 65001, []byte{}, otcAccept, 0},

		// Boundary ASNs
		{"stamp_max_asn", roleProvider, 4294967295, attrsNoOTC, otcAccept, 4294967295},
		{"stamp_min_asn", roleProvider, 1, attrsNoOTC, otcAccept, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, stamp := checkOTCIngress(tt.remoteRole, tt.remoteASN, tt.attrs)
			assert.Equal(t, tt.wantResult, result, "result")
			assert.Equal(t, tt.wantStamp, stamp, "stampASN")
		})
	}
}

// --- checkOTCEgress comprehensive tests ---

func TestCheckOTCEgress(t *testing.T) {
	withOTC := buildTestAttrs(65001)
	noOTC := buildTestAttrs(0)

	tests := []struct {
		name     string
		destRole string
		attrs    []byte
		suppress bool
	}{
		// OTC present: suppress to Provider/Peer/RS
		{"otc_provider", roleProvider, withOTC, true},
		{"otc_peer", rolePeer, withOTC, true},
		{"otc_rs", roleRS, withOTC, true},
		// OTC present: allow to Customer/RS-Client
		{"otc_customer", roleCustomer, withOTC, false},
		{"otc_rs_client", roleRSClient, withOTC, false},
		// No OTC: never suppress
		{"no_otc_provider", roleProvider, noOTC, false},
		{"no_otc_customer", roleCustomer, noOTC, false},
		{"no_otc_peer", rolePeer, noOTC, false},
		{"no_otc_rs", roleRS, noOTC, false},
		{"no_otc_rs_client", roleRSClient, noOTC, false},
		// Empty role: never suppress
		{"otc_empty_role", "", withOTC, false},
		{"no_otc_empty_role", "", noOTC, false},
		// Nil/empty attrs: never suppress
		{"nil_attrs", roleProvider, nil, false},
		{"empty_attrs", roleProvider, []byte{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.suppress, checkOTCEgress(tt.destRole, tt.attrs))
		})
	}
}

// --- OTCIngressFilter wrapper tests ---

func TestOTCIngressFilter(t *testing.T) {
	setFilterState(map[string]*peerRoleConfig{
		"10.0.0.1": {role: roleProvider},
		"10.0.0.2": {role: roleCustomer},
		"10.0.0.3": {role: rolePeer},
	}, nil)
	setFilterRemoteRole("10.0.0.1", roleCustomer)
	setFilterRemoteRole("10.0.0.2", roleProvider)
	setFilterRemoteRole("10.0.0.3", rolePeer)
	defer func() {
		setFilterState(nil, nil)
		filterMu.Lock()
		filterRemoteRoles = nil
		filterMu.Unlock()
	}()

	t.Run("no_config_passthrough", func(t *testing.T) {
		src := registry.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.99"), PeerAS: 65099}
		payload := buildTestPayload(buildTestAttrs(65001), nil)
		meta := make(map[string]any)
		accept, modified := OTCIngressFilter(src, payload, meta)
		assert.True(t, accept)
		assert.Nil(t, modified)
		// No role config for 10.0.0.99 -> no src-role in meta.
		_, hasSrcRole := meta["src-role"]
		assert.False(t, hasSrcRole, "no role config -> no src-role in meta")
	})

	t.Run("config_but_no_remote_role", func(t *testing.T) {
		setFilterState(map[string]*peerRoleConfig{
			"10.0.0.50": {role: roleProvider},
		}, nil)
		src := registry.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.50"), PeerAS: 65050}
		meta := make(map[string]any)
		accept, modified := OTCIngressFilter(src, buildTestPayload(buildTestAttrs(0), nil), meta)
		assert.True(t, accept)
		assert.Nil(t, modified)
		// Has role config (provider) -> meta["src-role"] should be set.
		assert.Equal(t, "provider", meta["src-role"], "src-role should reflect our config")
		// Restore original configs and remote roles (setFilterState clears remote roles).
		setFilterState(map[string]*peerRoleConfig{
			"10.0.0.1": {role: roleProvider},
			"10.0.0.2": {role: roleCustomer},
			"10.0.0.3": {role: rolePeer},
		}, nil)
		setFilterRemoteRole("10.0.0.1", roleCustomer)
		setFilterRemoteRole("10.0.0.2", roleProvider)
		setFilterRemoteRole("10.0.0.3", rolePeer)
	})

	t.Run("reject_leak_from_customer", func(t *testing.T) {
		src := registry.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.1"), PeerAS: 65001}
		accept, _ := OTCIngressFilter(src, buildTestPayload(buildTestAttrs(65001), nil), make(map[string]any))
		assert.False(t, accept)
	})

	t.Run("stamp_from_provider", func(t *testing.T) {
		src := registry.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.2"), PeerAS: 65002}
		meta := make(map[string]any)
		accept, modified := OTCIngressFilter(src, buildTestPayload(buildTestAttrs(0), nil), meta)
		assert.True(t, accept)
		require.NotNil(t, modified)
		// Meta set after stamping.
		// Role config for 10.0.0.2 is "customer" -> src-role set.
		assert.Equal(t, "customer", meta["src-role"], "src-role should reflect our config")
		attrs := extractAttrsFromPayload(modified)
		asn, found, _ := findOTC(attrs)
		assert.True(t, found)
		assert.Equal(t, uint32(65002), asn)
	})

	t.Run("accept_peer_otc_matches", func(t *testing.T) {
		src := registry.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.3"), PeerAS: 65003}
		accept, modified := OTCIngressFilter(src, buildTestPayload(buildTestAttrs(65003), nil), make(map[string]any))
		assert.True(t, accept)
		assert.Nil(t, modified)
	})

	t.Run("reject_peer_otc_mismatch", func(t *testing.T) {
		src := registry.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.3"), PeerAS: 65003}
		accept, _ := OTCIngressFilter(src, buildTestPayload(buildTestAttrs(65099), nil), make(map[string]any))
		assert.False(t, accept)
	})

	t.Run("reject_malformed_otc", func(t *testing.T) {
		src := registry.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.2"), PeerAS: 65002}
		malformed := []byte{0x40, 0x01, 0x01, 0x00, 0xC0, 35, 3, 0x00, 0x01, 0x02}
		accept, _ := OTCIngressFilter(src, buildTestPayload(malformed, nil), make(map[string]any))
		assert.False(t, accept)
	})
}

// --- OTCEgressFilter wrapper tests ---

func TestOTCEgressFilter(t *testing.T) {
	setFilterState(map[string]*peerRoleConfig{
		"10.0.0.1": {role: roleProvider, export: []string{"default"}},
		"10.0.0.5": {role: roleProvider, export: []string{"customer", "peer"}},
		"10.0.0.6": {role: roleProvider, export: []string{"default", "unknown"}},
		"10.0.0.7": {role: roleProvider},
	}, nil)
	setFilterRemoteRole("10.0.0.10", roleCustomer)
	setFilterRemoteRole("10.0.0.11", roleProvider)
	setFilterRemoteRole("10.0.0.12", rolePeer)
	setFilterRemoteRole("10.0.0.13", roleRSClient)
	defer func() {
		setFilterState(nil, nil)
		filterMu.Lock()
		filterRemoteRoles = nil
		filterMu.Unlock()
	}()

	noOTC := buildTestPayload(buildTestAttrs(0), nil)
	withOTC := buildTestPayload(buildTestAttrs(65001), nil)

	t.Run("no_source_config_accept", func(t *testing.T) {
		src := registry.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.99")}
		dest := registry.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.10")}
		assert.True(t, OTCEgressFilter(src, dest, noOTC, nil, nil))
	})

	t.Run("no_export_no_otc_accept", func(t *testing.T) {
		src := registry.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.7")}
		dest := registry.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.11")}
		assert.True(t, OTCEgressFilter(src, dest, noOTC, nil, nil))
	})

	t.Run("default_to_customer_accept", func(t *testing.T) {
		src := registry.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.1")}
		dest := registry.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.10")}
		assert.True(t, OTCEgressFilter(src, dest, noOTC, nil, nil))
	})

	t.Run("default_to_provider_suppress", func(t *testing.T) {
		src := registry.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.1")}
		dest := registry.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.11")}
		assert.False(t, OTCEgressFilter(src, dest, noOTC, nil, nil))
	})

	t.Run("default_to_rs_client_accept", func(t *testing.T) {
		src := registry.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.1")}
		dest := registry.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.13")}
		assert.True(t, OTCEgressFilter(src, dest, noOTC, nil, nil))
	})

	t.Run("default_to_peer_suppress", func(t *testing.T) {
		src := registry.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.1")}
		dest := registry.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.12")}
		assert.False(t, OTCEgressFilter(src, dest, noOTC, nil, nil))
	})

	t.Run("explicit_customer_peer_to_customer", func(t *testing.T) {
		src := registry.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.5")}
		dest := registry.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.10")}
		assert.True(t, OTCEgressFilter(src, dest, noOTC, nil, nil))
	})

	t.Run("explicit_customer_peer_to_provider", func(t *testing.T) {
		src := registry.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.5")}
		dest := registry.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.11")}
		assert.False(t, OTCEgressFilter(src, dest, noOTC, nil, nil))
	})

	t.Run("default_unknown_to_untagged", func(t *testing.T) {
		src := registry.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.6")}
		dest := registry.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.14")}
		assert.True(t, OTCEgressFilter(src, dest, noOTC, nil, nil))
	})

	t.Run("default_only_to_untagged_suppress", func(t *testing.T) {
		src := registry.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.1")}
		dest := registry.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.14")}
		assert.False(t, OTCEgressFilter(src, dest, noOTC, nil, nil))
	})

	t.Run("otc_suppress_to_provider", func(t *testing.T) {
		src := registry.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.7")}
		dest := registry.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.11")}
		assert.False(t, OTCEgressFilter(src, dest, withOTC, map[string]any{"src-role": "provider"}, nil))
	})

	t.Run("otc_allow_to_customer", func(t *testing.T) {
		src := registry.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.7")}
		dest := registry.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.10")}
		assert.True(t, OTCEgressFilter(src, dest, withOTC, map[string]any{"src-role": "provider"}, nil))
	})

	t.Run("otc_suppress_to_peer", func(t *testing.T) {
		src := registry.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.7")}
		dest := registry.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.12")}
		assert.False(t, OTCEgressFilter(src, dest, withOTC, map[string]any{"src-role": "provider"}, nil))
	})

	t.Run("otc_suppress_to_rs", func(t *testing.T) {
		setFilterRemoteRole("10.0.0.15", roleRS)
		src := registry.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.7")}
		dest := registry.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.15")}
		assert.False(t, OTCEgressFilter(src, dest, withOTC, map[string]any{"src-role": "provider"}, nil))
	})

	t.Run("otc_allow_to_rs_client", func(t *testing.T) {
		src := registry.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.7")}
		dest := registry.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.13")}
		assert.True(t, OTCEgressFilter(src, dest, withOTC, map[string]any{"src-role": "provider"}, nil))
	})

	t.Run("meta_wrong_type_not_suppressed", func(t *testing.T) {
		// meta["src-role"] with wrong type (int instead of string) must NOT trigger suppression.
		src := registry.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.7")}
		dest := registry.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.11")} // provider
		assert.True(t, OTCEgressFilter(src, dest, withOTC, map[string]any{"src-role": 42}, nil),
			"int src-role is not string -- suppression must not trigger")
		assert.True(t, OTCEgressFilter(src, dest, withOTC, map[string]any{"src-role": true}, nil),
			"bool src-role is not string -- suppression must not trigger")
	})

	// --- src-role metadata tests (config-based suppression) ---

	t.Run("src_role_provider_to_provider_suppress", func(t *testing.T) {
		// Route from provider (our config) to provider dest: suppress, even without OTC in wire.
		src := registry.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.7")}
		dest := registry.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.11")} // provider
		meta := map[string]any{"src-role": "provider"}
		assert.False(t, OTCEgressFilter(src, dest, noOTC, meta, nil),
			"provider->provider must suppress via src-role, even without OTC")
	})

	t.Run("src_role_provider_to_customer_accept", func(t *testing.T) {
		// Route from provider to customer: accept (customers can receive).
		src := registry.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.7")}
		dest := registry.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.10")} // customer
		meta := map[string]any{"src-role": "provider"}
		assert.True(t, OTCEgressFilter(src, dest, noOTC, meta, nil),
			"provider->customer should accept")
	})

	t.Run("src_role_customer_to_provider_accept", func(t *testing.T) {
		// Route from customer to provider: accept (customer routes can go anywhere).
		src := registry.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.99")}  // no config
		dest := registry.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.11")} // provider
		meta := map[string]any{"src-role": "customer"}
		assert.True(t, OTCEgressFilter(src, dest, noOTC, meta, nil),
			"customer->provider should accept")
	})

	t.Run("src_role_peer_to_peer_suppress", func(t *testing.T) {
		// Route from peer to peer: suppress.
		src := registry.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.99")}  // no config
		dest := registry.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.12")} // peer
		meta := map[string]any{"src-role": "peer"}
		assert.False(t, OTCEgressFilter(src, dest, noOTC, meta, nil),
			"peer->peer must suppress via src-role")
	})

	t.Run("src_role_peer_to_rs_suppress", func(t *testing.T) {
		src := registry.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.99")}
		dest := registry.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.15")} // RS
		meta := map[string]any{"src-role": "peer"}
		assert.False(t, OTCEgressFilter(src, dest, noOTC, meta, nil),
			"peer->RS must suppress via src-role")
	})

	t.Run("src_role_rs_to_provider_suppress", func(t *testing.T) {
		src := registry.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.99")}
		dest := registry.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.11")} // provider
		meta := map[string]any{"src-role": "rs"}
		assert.False(t, OTCEgressFilter(src, dest, noOTC, meta, nil),
			"RS->provider must suppress via src-role")
	})
}

// --- Boundary tests ---

func TestOTCBoundaryASN(t *testing.T) {
	for _, asn := range []uint32{1, 65001, 100000, 23456, 4294967295} {
		t.Run("round_trip_"+string(rune(asn)), func(t *testing.T) {
			attrs := buildTestAttrs(asn)
			got, found, malformed := findOTC(attrs)
			require.True(t, found)
			assert.False(t, malformed)
			assert.Equal(t, asn, got)
		})
	}
}

func TestOTCBoundaryLength(t *testing.T) {
	for _, length := range []byte{0, 1, 2, 3, 5, 6, 255} {
		t.Run("malformed_len_"+string(rune(length)), func(t *testing.T) {
			value := make([]byte, length)
			attr := append([]byte{0xC0, 35, length}, value...)
			_, _, malformed := findOTC(attr)
			assert.True(t, malformed, "length %d should be malformed", length)
		})
	}

	t.Run("valid_len_4", func(t *testing.T) {
		attr := []byte{0xC0, 35, 4, 0x00, 0x00, 0xFD, 0xE9}
		_, found, malformed := findOTC(attr)
		assert.True(t, found)
		assert.False(t, malformed)
	})
}

func TestIsUnicastFamily(t *testing.T) {
	for _, f := range []string{"ipv4/unicast", "ipv6/unicast"} {
		assert.True(t, isUnicastFamily(f), "%s should be unicast", f)
	}
	for _, f := range []string{
		"ipv4/multicast", "ipv6/multicast", "ipv4/vpn", "ipv6/vpn",
		"ipv4/flow", "ipv6/flow", "l2vpn/evpn", "l2vpn/vpls",
		"bgp-ls/bgp-ls", "ipv4/mpls-label", "ipv4/mup", "ipv4/mvpn",
		"", "unicast", "ipv4",
	} {
		assert.False(t, isUnicastFamily(f), "%s should not be unicast", f)
	}
}

// --- Loose mode OTC filtering: IBGP peer without role, route has OTC ---

// TestLooseIngressFilter_IBGPNoRole_RouteWithOTC verifies that routes with OTC
// from an IBGP peer (no role configured/negotiated) pass through the ingress filter.
// This is the "loose" case: session established without role, no ingress rules apply.
//
// VALIDATES: No OTC filtering when peer has no role config.
// PREVENTS: OTC routes from IBGP being silently dropped.
func TestLooseIngressFilter_IBGPNoRole_RouteWithOTC(t *testing.T) {
	// No config for the IBGP peer.
	setFilterState(nil, nil)
	defer setFilterState(nil, nil)

	src := registry.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.100"), PeerAS: 65000}
	payload := buildTestPayload(buildTestAttrs(65001), nil) // route has OTC
	accept, modified := OTCIngressFilter(src, payload, make(map[string]any))
	assert.True(t, accept, "IBGP peer without role: route with OTC should pass")
	assert.Nil(t, modified, "no modification expected")
}

// --- IBGP-to-EBGP: route tagged with OTC on IBGP arrives, egress to EBGP peer ---

// TestEgressFilter_IBGPSourceToEBGPDest_WithOTC verifies that a route
// learned from an IBGP peer (no role) that carries OTC (tagged by a remote
// ingress filter) is suppressed when forwarded to an EBGP provider/peer/RS.
//
// Real-world scenario: EBGP customer peer stamps OTC on ingress at the edge.
// Route propagates via IBGP to another router. That router should suppress
// the route to EBGP providers (OTC means "only to customer").
//
// VALIDATES: OTC egress suppression works regardless of whether the source peer has role config.
// PREVENTS: OTC-tagged routes leaking to upstream EBGP peers via IBGP transit.
func TestEgressFilter_IBGPSourceToEBGPDest_WithOTC(t *testing.T) {
	// Source: IBGP peer with no role config (internal mesh).
	// Dest: EBGP provider, peer, RS, customer, rs-client.
	setFilterState(map[string]*peerRoleConfig{
		"10.0.0.100": {role: roleProvider}, // we are provider to 10.0.0.100 but the route doesn't come from here
	}, nil)
	setFilterRemoteRole("10.0.0.200", roleProvider) // EBGP dest: provider
	setFilterRemoteRole("10.0.0.201", roleCustomer) // EBGP dest: customer
	setFilterRemoteRole("10.0.0.202", rolePeer)     // EBGP dest: peer
	setFilterRemoteRole("10.0.0.203", roleRS)       // EBGP dest: RS
	setFilterRemoteRole("10.0.0.204", roleRSClient) // EBGP dest: RS-client
	defer func() {
		setFilterState(nil, nil)
		filterMu.Lock()
		filterRemoteRoles = nil
		filterMu.Unlock()
	}()

	// Source is IBGP peer (no config for this address), route has OTC.
	ibgpSrc := registry.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.50"), PeerAS: 65000}
	withOTC := buildTestPayload(buildTestAttrs(65001), nil)

	// OTC -> Provider: SUPPRESS (RFC 9234: routes with OTC must not propagate to providers).
	dest := registry.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.200")}
	assert.False(t, OTCEgressFilter(ibgpSrc, dest, withOTC, map[string]any{"src-role": "provider"}, nil), "OTC route from IBGP to EBGP provider: suppress")

	// OTC -> Customer: ALLOW (customer is downstream, OTC is fine).
	dest = registry.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.201")}
	assert.True(t, OTCEgressFilter(ibgpSrc, dest, withOTC, map[string]any{"src-role": "provider"}, nil), "OTC route from IBGP to EBGP customer: allow")

	// OTC -> Peer: SUPPRESS.
	dest = registry.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.202")}
	assert.False(t, OTCEgressFilter(ibgpSrc, dest, withOTC, map[string]any{"src-role": "provider"}, nil), "OTC route from IBGP to EBGP peer: suppress")

	// OTC -> RS: SUPPRESS.
	dest = registry.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.203")}
	assert.False(t, OTCEgressFilter(ibgpSrc, dest, withOTC, map[string]any{"src-role": "provider"}, nil), "OTC route from IBGP to EBGP RS: suppress")

	// OTC -> RS-Client: ALLOW (downstream).
	dest = registry.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.204")}
	assert.True(t, OTCEgressFilter(ibgpSrc, dest, withOTC, map[string]any{"src-role": "provider"}, nil), "OTC route from IBGP to EBGP RS-client: allow")
}

// TestEgressFilter_IBGPSourceToEBGPDest_NoOTC verifies routes without OTC
// from IBGP pass through to all EBGP destinations (no suppression).
//
// VALIDATES: Routes without OTC are never suppressed by OTC egress rules.
// PREVENTS: Legitimate routes from IBGP being dropped on egress.
func TestEgressFilter_IBGPSourceToEBGPDest_NoOTC(t *testing.T) {
	setFilterRemoteRole("10.0.0.200", roleProvider)
	setFilterRemoteRole("10.0.0.201", roleCustomer)
	setFilterRemoteRole("10.0.0.202", rolePeer)
	defer func() {
		filterMu.Lock()
		filterRemoteRoles = nil
		filterMu.Unlock()
	}()

	ibgpSrc := registry.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.50"), PeerAS: 65000}
	noOTC := buildTestPayload(buildTestAttrs(0), nil)

	for _, destAddr := range []string{"10.0.0.200", "10.0.0.201", "10.0.0.202"} {
		dest := registry.PeerFilterInfo{Address: netip.MustParseAddr(destAddr)}
		assert.True(t, OTCEgressFilter(ibgpSrc, dest, noOTC, nil, nil),
			"route without OTC from IBGP to %s: should pass", destAddr)
	}
}

// --- Mixed topology: some peers with role, some without ---

// TestMixedTopology_RoleAndNoRolePeers verifies correct filtering when
// some peers have role config and others don't.
//
// VALIDATES: OTC filtering applies only to peers with role, others unaffected.
// PREVENTS: Role config on one peer affecting filtering decisions for other peers.
func TestMixedTopology_RoleAndNoRolePeers(t *testing.T) {
	setFilterState(map[string]*peerRoleConfig{
		"10.0.0.1": {role: roleProvider, export: []string{"default"}},
		// 10.0.0.2 has NO role config (IBGP or legacy peer)
	}, nil)
	setFilterRemoteRole("10.0.0.1", roleCustomer) // EBGP: we are provider, they are customer
	// 10.0.0.2: no remote role
	setFilterRemoteRole("10.0.0.10", roleProvider) // dest with role
	// 10.0.0.20: dest without role
	defer func() {
		setFilterState(nil, nil)
		filterMu.Lock()
		filterRemoteRoles = nil
		filterMu.Unlock()
	}()

	withOTC := buildTestPayload(buildTestAttrs(65001), nil)
	noOTC := buildTestPayload(buildTestAttrs(0), nil)

	// Route from configured peer (provider, export default) to provider dest: suppress (not in default set).
	src1 := registry.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.1")}
	dest1 := registry.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.10")}
	assert.False(t, OTCEgressFilter(src1, dest1, noOTC, nil, nil), "provider src, export default, to provider dest: suppress")

	// Route from unconfigured peer (no role) to provider dest: pass (no src-role, we don't filter).
	src2 := registry.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.2")}
	assert.True(t, OTCEgressFilter(src2, dest1, withOTC, nil, nil), "no-role src to provider: pass (no config = no filtering)")

	// Route from unconfigured peer to provider dest without OTC: same -- pass.
	assert.True(t, OTCEgressFilter(src2, dest1, noOTC, nil, nil), "no-role src, no OTC to provider: pass")

	// Route from unconfigured peer to untagged dest: always pass.
	dest2 := registry.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.20")}
	assert.True(t, OTCEgressFilter(src2, dest2, noOTC, nil, nil), "no-role src to untagged: pass")
}
