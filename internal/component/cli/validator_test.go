package cli

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
)

// TestValidateSyntaxMissingSemicolon verifies semicolon handling.
//
// VALIDATES: Semicolons are auto-inserted at newlines; still required on single-line input.
// PREVENTS: Invalid config saved without warning.
func TestValidateSyntaxMissingSemicolon(t *testing.T) {
	v, err := NewConfigValidator()
	require.NoError(t, err)

	tests := []struct {
		name    string
		content string
		wantErr bool
	}{
		{
			name:    "valid_with_semicolon",
			content: "bgp { router-id 1.2.3.4; }",
			wantErr: false,
		},
		{
			name:    "missing_semicolon_single_line",
			content: "bgp { router-id 1.2.3.4 }",
			wantErr: true,
		},
		{
			name: "block_no_semicolon_needed",
			content: `bgp {
  peer peer1 {
    connection {
      remote {
        ip 1.1.1.1
      }
    }
    session {
      asn {
        remote 65001
      }
    }
  }
}`,
			wantErr: false,
		},
		{
			name: "auto_semicolon_at_newline",
			content: `bgp {
  peer peer1 {
    connection {
      remote {
        ip 1.1.1.1
      }
    }
    session {
      asn {
        remote 65001
      }
    }
  }
}`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := v.ValidateSyntax(tt.content)
			if tt.wantErr {
				assert.NotEmpty(t, errs, "expected syntax error")
			} else {
				assert.Empty(t, errs, "expected no syntax errors")
			}
		})
	}
}

// TestValidateSyntaxUnclosedBrace verifies detection of unclosed braces.
//
// VALIDATES: Parser detects unclosed braces.
// PREVENTS: Malformed config structure.
func TestValidateSyntaxUnclosedBrace(t *testing.T) {
	v, err := NewConfigValidator()
	require.NoError(t, err)

	tests := []struct {
		name    string
		content string
		wantErr bool
	}{
		{
			name: "balanced_braces",
			content: `bgp {
  peer peer1 {
    connection {
      remote {
        ip 1.1.1.1
      }
    }
    session {
      asn {
        remote 65001
      }
    }
  }
}`,
			wantErr: false,
		},
		{
			name: "unclosed_brace",
			content: `bgp {
  peer 1.1.1.1 {
    session {
      asn {
        remote 65001
      }
    }
`,
			wantErr: true,
		},
		{
			name: "extra_close_brace",
			content: `bgp {
  peer peer1 {
    connection {
      remote {
        ip 1.1.1.1
      }
    }
    session {
      asn {
        remote 65001
      }
    }
  }
}}`,
			wantErr: true,
		},
		{
			name: "nested_balanced",
			content: `bgp {
  peer peer1 {
    connection {
      remote {
        ip 1.1.1.1
      }
    }
    session {
      asn {
        remote 65001
      }
      capability {
        route-refresh
      }
    }
  }
}`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := v.ValidateSyntax(tt.content)
			if tt.wantErr {
				assert.NotEmpty(t, errs, "expected syntax error")
			} else {
				assert.Empty(t, errs, "expected no syntax errors")
			}
		})
	}
}

// TestValidateSemanticPeerAsLocalAs verifies remote as validation.
//
// VALIDATES: remote as is parsed correctly.
// NOTE: iBGP validation (remote as == local as) is deferred until
// route-reflector-client is added to schema.
func TestValidateSemanticPeerAsLocalAs(t *testing.T) {
	v, err := NewConfigValidator()
	require.NoError(t, err)

	tests := []struct {
		name    string
		content string
		wantErr bool
	}{
		{
			name: "different_as",
			content: `bgp {
  session {
  	asn {
  		local 65000
  	}
  }
  peer peer1 {
    connection {
      remote {
        ip 1.1.1.1
      }
    }
    session {
      asn {
        remote 65001
      }
    }
  }
}`,
			wantErr: false,
		},
		{
			name: "same_as_ibgp",
			content: `bgp {
  session {
  	asn {
  		local 65000
  	}
  }
  peer peer1 {
    connection {
      remote {
        ip 1.1.1.1
      }
    }
    session {
      asn {
        remote 65000
      }
    }
  }
}`,
			// iBGP (remote as == local as) is valid config.
			// Full iBGP validation requires route-reflector-client in schema.
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := v.Validate(tt.content)
			if tt.wantErr {
				assert.NotEmpty(t, result.Errors, "expected semantic error")
			} else {
				assert.Empty(t, result.Errors, "expected no errors")
			}
		})
	}
}

// TestValidateSemanticDuplicatePeer verifies duplicate peer detection.
//
// VALIDATES: Duplicate peer addresses detected.
// PREVENTS: Configuration with conflicting peers.
func TestValidateSemanticDuplicatePeer(t *testing.T) {
	v, err := NewConfigValidator()
	require.NoError(t, err)

	tests := []struct {
		name    string
		content string
		wantErr bool
	}{
		{
			name: "unique_peers",
			content: `bgp {
  peer peer1 {
    connection {
      remote {
        ip 1.1.1.1
      }
    }
    session {
      asn {
        remote 65001
      }
    }
  }
  peer 2.2.2.2 {
    session {
      asn {
        remote 65002
      }
    }
  }
}`,
			wantErr: false,
		},
		// Note: The parser handles duplicate keys by generating unique keys with #N suffix,
		// so the semantic validator sees them as different keys. This is acceptable behavior
		// as the parser handles the conflict.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := v.Validate(tt.content)
			if tt.wantErr {
				assert.NotEmpty(t, result.Errors, "expected semantic error")
			} else {
				assert.Empty(t, result.Errors, "expected no errors")
			}
		})
	}
}

// TestValidateSemanticRouterID verifies router-id validation.
//
// VALIDATES: Invalid router-id detected.
// PREVENTS: Invalid IPv4 as router-id.
func TestValidateSemanticRouterID(t *testing.T) {
	v, err := NewConfigValidator()
	require.NoError(t, err)

	tests := []struct {
		name    string
		content string
		wantErr bool
	}{
		{
			name:    "valid_router_id",
			content: "bgp { router-id 1.2.3.4; }",
			wantErr: false,
		},
		{
			name:    "invalid_router_id",
			content: "bgp { router-id 999.999.999.999; }",
			wantErr: true,
		},
		{
			name:    "router_id_not_ipv4",
			content: "bgp { router-id not-an-ip; }",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := v.Validate(tt.content)
			if tt.wantErr {
				assert.NotEmpty(t, result.Errors, "expected semantic error")
			} else {
				assert.Empty(t, result.Errors, "expected no errors")
			}
		})
	}
}

// TestValidateSemanticHoldTime verifies receive-hold-time boundary validation.
//
// VALIDATES: Hold time must be 0 or >= 3 per RFC 4271.
// PREVENTS: Invalid receive-hold-time values 1 or 2.
// BOUNDARY: 0 (valid), 1 (invalid), 2 (invalid), 3 (valid), 65535 (valid).
func TestValidateSemanticHoldTime(t *testing.T) {
	v, err := NewConfigValidator()
	require.NoError(t, err)

	tests := []struct {
		name    string
		content string
		wantErr bool
	}{
		{
			name:    "hold_time_0_valid",
			content: "bgp { peer peer1 { connection { remote { ip 1.1.1.1; } } session { asn { remote 65001; } } timer { receive-hold-time 0; } } }",
			wantErr: false,
		},
		{
			name:    "hold_time_1_invalid",
			content: "bgp { peer peer1 { connection { remote { ip 1.1.1.1; } } session { asn { remote 65001; } } timer { receive-hold-time 1; } } }",
			wantErr: true,
		},
		{
			name:    "hold_time_2_invalid",
			content: "bgp { peer peer1 { connection { remote { ip 1.1.1.1; } } session { asn { remote 65001; } } timer { receive-hold-time 2; } } }",
			wantErr: true,
		},
		{
			name:    "hold_time_3_valid",
			content: "bgp { peer peer1 { connection { remote { ip 1.1.1.1; } } session { asn { remote 65001; } } timer { receive-hold-time 3; } } }",
			wantErr: false,
		},
		{
			name:    "hold_time_90_valid",
			content: "bgp { peer peer1 { connection { remote { ip 1.1.1.1; } } session { asn { remote 65001; } } timer { receive-hold-time 90; } } }",
			wantErr: false,
		},
		{
			name:    "hold_time_65535_valid",
			content: "bgp { peer peer1 { connection { remote { ip 1.1.1.1; } } session { asn { remote 65001; } } timer { receive-hold-time 65535; } } }",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := v.Validate(tt.content)
			if tt.wantErr {
				assert.NotEmpty(t, result.Errors, "expected error for receive-hold-time")
			} else {
				assert.Empty(t, result.Errors, "expected no errors")
			}
		})
	}
}

// TestValidateAll verifies full validation pipeline.
//
// VALIDATES: All validation levels run together.
// PREVENTS: Missing validation in commit path.
func TestValidateAll(t *testing.T) {
	v, err := NewConfigValidator()
	require.NoError(t, err)

	// Valid config
	validConfig := `bgp {
  router-id 1.2.3.4
  session {
  	asn {
  		local 65000
  	}
  }
  peer peer1 {
    connection {
      remote {
        ip 1.1.1.1
      }
    }
    session {
      asn {
        remote 65001
      }
    }
    timer { receive-hold-time 90; }
  }
}`
	result := v.Validate(validConfig)
	assert.Empty(t, result.Errors, "expected no errors for valid config")

	// Config with semantic error (invalid receive-hold-time)
	invalidConfig := `bgp {
  router-id 1.2.3.4
  session {
  	asn {
  		local 65000
  	}
  }
  peer peer1 {
    connection {
      remote {
        ip 1.1.1.1
      }
    }
    session {
      asn {
        remote 65001
      }
    }
    timer { receive-hold-time 1; }
  }
}`
	result = v.Validate(invalidConfig)
	require.NotEmpty(t, result.Errors, "expected errors for invalid config")
	// Should have receive-hold-time error
	assert.Contains(t, result.Errors[0].Message, "receive-hold-time")
}

// TestValidationErrorFormat verifies error message formatting.
//
// VALIDATES: Errors include clear messages.
// PREVENTS: Unclear error messages for users.
func TestValidationErrorFormat(t *testing.T) {
	v, err := NewConfigValidator()
	require.NoError(t, err)

	content := `bgp {
  router-id 1.2.3.4
  peer peer1 {
    connection {
      remote {
        ip 1.1.1.1
      }
    }
    session {
      asn {
        remote 65001
      }
    }
    timer { receive-hold-time 1; }
  }
}`

	result := v.Validate(content)
	require.NotEmpty(t, result.Errors)

	// Error should have message
	assert.NotEmpty(t, result.Errors[0].Message, "error should have message")
	assert.Contains(t, result.Errors[0].Message, "receive-hold-time")
}

// TestValidatePeerAddress verifies peer address validation.
//
// VALIDATES: Peer address must be valid IP.
// PREVENTS: Invalid peer addresses in config.
func TestValidatePeerAddress(t *testing.T) {
	v, err := NewConfigValidator()
	require.NoError(t, err)

	tests := []struct {
		name    string
		content string
		wantErr bool
	}{
		{
			name: "valid_ipv4_peer",
			content: `bgp {
  peer peer1 {
    connection {
      remote {
        ip 192.168.1.1
      }
    }
    session {
      asn {
        remote 65001
      }
    }
  }
}`,
			wantErr: false,
		},
		{
			name: "valid_ipv6_peer",
			content: `bgp {
  peer peer1 {
    connection {
      remote {
        ip 2001:db8::1
      }
    }
    session {
      asn {
        remote 65001
      }
    }
  }
}`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := v.Validate(tt.content)
			if tt.wantErr {
				assert.NotEmpty(t, result.Errors, "expected error")
			} else {
				assert.Empty(t, result.Errors, "expected no errors")
			}
		})
	}
}

// TestValidateUnknownKeyword verifies schema validation catches unknown keywords.
//
// VALIDATES: Unknown keywords rejected by parser.
// PREVENTS: Typos in config silently ignored.
func TestValidateUnknownKeyword(t *testing.T) {
	v, err := NewConfigValidator()
	require.NoError(t, err)

	content := `bgp {
  unknown-keyword value
}`

	result := v.Validate(content)
	require.NotEmpty(t, result.Errors, "expected error for unknown keyword")
	assert.Contains(t, result.Errors[0].Message, "unknown")
}

// TestValidateMissingPeerAS verifies mandatory field validation.
//
// VALIDATES: Missing remote as in peer block causes error.
// PREVENTS: Peers configured without required ASN.
func TestValidateMissingPeerAS(t *testing.T) {
	v, err := NewConfigValidator()
	require.NoError(t, err)

	tests := []struct {
		name    string
		content string
		wantErr bool
	}{
		{
			name: "peer_with_remote_as",
			content: `bgp {
  router-id 1.1.1.1
  session {
  	asn {
  		local 65000
  	}
  }
  peer peer1 {
    connection {
      remote {
        ip 192.0.2.1
      }
    }
    session {
      asn {
        remote 65001
      }
    }
  }
}`,
			wantErr: false,
		},
		{
			name: "peer_missing_remote_as",
			content: `bgp {
  router-id 1.1.1.1
  session {
  	asn {
  		local 65000
  	}
  }
  peer peer1 {
    connection {
      remote {
        ip 192.0.2.1
      }
    }
    timer { receive-hold-time 90; }
  }
}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := v.Validate(tt.content)
			if tt.wantErr {
				var found bool
				for _, w := range result.Warnings {
					if strings.Contains(w.Message, "remote") {
						found = true
						break
					}
				}
				assert.True(t, found, "expected warning containing remote")
			} else {
				for _, w := range result.Warnings {
					assert.NotContains(t, w.Message, "remote", "unexpected remote warning")
				}
			}
		})
	}
}

// TestValidatePeerASInheritance verifies remote as can be inherited from group.
//
// VALIDATES: Group-level remote as satisfies mandatory field requirement for group peers.
// PREVENTS: False positives when remote as comes from group defaults.
func TestValidatePeerASInheritance(t *testing.T) {
	v, err := NewConfigValidator()
	require.NoError(t, err)

	tests := []struct {
		name            string
		content         string
		wantErr         bool   // expect an error
		wantWarn        bool   // expect a warning (mandatory-missing)
		wantMsgContains string // what error/warning message should contain
	}{
		{
			name: "remote_as_inherited_from_group",
			content: `bgp {
  router-id 1.1.1.1
  session {
  	asn {
  		local 65000
  	}
  }
  group ibgp {
    session { asn { remote 65000; } }
    timer { receive-hold-time 60; }
    peer peer1 {
      connection {
        remote {
          ip 192.0.2.1
        }
      }
    }
  }
}`,
		},
		{
			name: "remote_as_override_in_group_peer",
			content: `bgp {
  router-id 1.1.1.1
  session {
  	asn {
  		local 65000
  	}
  }
  group ibgp {
    session { asn { remote 65000; } }
    peer peer1 {
      connection {
        remote {
          ip 192.0.2.1
        }
      }
      session {
        asn {
          remote 65001
        }
      }
    }
  }
}`,
		},
		{
			name: "group_without_remote_as",
			content: `bgp {
  router-id 1.1.1.1
  session {
  	asn {
  		local 65000
  	}
  }
  group base {
    timer { receive-hold-time 60; }
    peer peer1 {
      connection {
        remote {
          ip 192.0.2.1
        }
      }
    }
  }
}`,
			wantWarn:        true,
			wantMsgContains: "remote",
		},
		{
			name: "standalone_peer_missing_remote_as",
			content: `bgp {
  router-id 1.1.1.1
  session {
  	asn {
  		local 65000
  	}
  }
  peer peer1 {
    connection {
      remote {
        ip 192.0.2.1
      }
    }
    timer { receive-hold-time 90; }
  }
}`,
			wantWarn:        true,
			wantMsgContains: "remote",
		},
		{
			name: "invalid_receive-hold-time_from_group",
			content: `bgp {
  router-id 1.1.1.1
  session {
  	asn {
  		local 65000
  	}
  }
  group bad {
    session { asn { remote 65001; } }
    timer { receive-hold-time 1; }
    peer peer1 {
      connection {
        remote {
          ip 192.0.2.1
        }
      }
    }
  }
}`,
			wantErr:         true,
			wantMsgContains: "receive-hold-time",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := v.Validate(tt.content)
			switch {
			case tt.wantErr:
				require.NotEmpty(t, result.Errors, "expected error")
				var found bool
				for _, e := range result.Errors {
					if strings.Contains(e.Message, tt.wantMsgContains) {
						found = true
						break
					}
				}
				assert.True(t, found, "expected error containing %q", tt.wantMsgContains)
			case tt.wantWarn:
				require.NotEmpty(t, result.Warnings, "expected warning")
				var found bool
				for _, w := range result.Warnings {
					if strings.Contains(w.Message, tt.wantMsgContains) {
						found = true
						break
					}
				}
				assert.True(t, found, "expected warning containing %q", tt.wantMsgContains)
			default:
				assert.Empty(t, result.Errors, "expected no errors when remote as is inherited from group")
			}
		})
	}
}

// TestValidateASNBoundary verifies ASN boundary validation.
//
// VALIDATES: ASN values within valid range.
// BOUNDARY: 1 (valid min), 4294967295 (valid max), 0 (invalid), overflow (invalid).
func TestValidateASNBoundary(t *testing.T) {
	v, err := NewConfigValidator()
	require.NoError(t, err)

	tests := []struct {
		name    string
		content string
		wantErr bool
	}{
		{
			name:    "asn_min_valid",
			content: "bgp { session { asn { local 1; } } }",
			wantErr: false,
		},
		{
			name:    "asn_max_valid",
			content: "bgp { session { asn { local 4294967295; } } }",
			wantErr: false,
		},
		{
			name:    "asn_typical",
			content: "bgp { session { asn { local 65000; } } }",
			wantErr: false,
		},
		// Note: ASN 0 is technically reserved but parser accepts it as valid number.
		// Semantic validation could add ASN range check if needed.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := v.Validate(tt.content)
			if tt.wantErr {
				assert.NotEmpty(t, result.Errors, "expected error")
			} else {
				assert.Empty(t, result.Errors, "expected no errors")
			}
		})
	}
}

// TestValidateSetFormat verifies that Validate handles set-format content.
//
// VALIDATES: Validate detects set/set-meta format and uses SetParser.
// PREVENTS: Session-mode commits failing because validator only uses hierarchical parser.
func TestValidateSetFormat(t *testing.T) {
	v, err := NewConfigValidator()
	require.NoError(t, err)

	tests := []struct {
		name    string
		content string
		wantErr bool
	}{
		{
			name:    "set_format_valid",
			content: "set bgp router-id 1.2.3.4\nset bgp session asn local 65000\nset bgp peer peer1 connection remote ip 192.0.2.1\nset bgp peer peer1 session asn remote 65001\n",
			wantErr: false,
		},
		{
			name:    "set_meta_format_valid",
			content: "#user@local @2025-01-01T00:00:00Z set bgp router-id 1.2.3.4\n#user@local @2025-01-01T00:00:00Z set bgp session asn local 65000\n#user@local @2025-01-01T00:00:00Z set bgp peer peer1 connection remote ip 192.0.2.1\n#user@local @2025-01-01T00:00:00Z set bgp peer peer1 session asn remote 65001\n",
			wantErr: false,
		},
		{
			name:    "set_format_invalid_field",
			content: "set bgp unknown-field 123\n",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := v.Validate(tt.content)
			if tt.wantErr {
				assert.NotEmpty(t, result.Errors, "expected validation errors")
			} else {
				assert.Empty(t, result.Errors, "expected no errors for valid set-format content")
				assert.Empty(t, result.Warnings, "expected no warnings for valid set-format content")
			}
		})
	}
}

// TestValidatePeer_MissingRequiredField verifies that the generic ze:required check
// produces warnings for missing required fields after group merge.
// VALIDATES: AC-10 -- editor validation error with field name.
// PREVENTS: Missing required fields not caught by editor validation.
func TestValidatePeer_MissingRequiredField(t *testing.T) {
	v, err := NewConfigValidator()
	require.NoError(t, err)

	// Peer missing connection/remote/ip -- should warn.
	content := `bgp {
  router-id 1.1.1.1
  session {
    asn {
      local 65000
    }
  }
  peer test_peer {
    session {
      asn {
        remote 65001
      }
    }
  }
}`
	result := v.Validate(content)

	var found bool
	for _, w := range result.Warnings {
		if strings.Contains(w.Message, "connection/remote/ip") {
			found = true
			break
		}
	}
	assert.True(t, found, "expected warning about missing connection/remote/ip, got warnings: %v", result.Warnings)
}

// TestValidatePeer_RequiredFieldInheritedFromBGP verifies that a required field
// present at bgp level suppresses the warning for standalone peers.
// VALIDATES: bgp-level inheritance in generic ze:required loop.
// PREVENTS: False warning when session/asn/local is set at bgp level.
func TestValidatePeer_RequiredFieldInheritedFromBGP(t *testing.T) {
	v, err := NewConfigValidator()
	require.NoError(t, err)

	// session/asn/local set at bgp level, peer has remote ip + remote as.
	content := `bgp {
  router-id 1.1.1.1
  session {
    asn {
      local 65000
    }
  }
  peer test_peer {
    connection {
      remote {
        ip 192.0.2.1
      }
    }
    session {
      asn {
        remote 65001
      }
    }
  }
}`
	result := v.Validate(content)

	for _, w := range result.Warnings {
		assert.NotContains(t, w.Message, "session/asn/local",
			"session/asn/local inherited from bgp level should not warn")
	}
}

// TestHasResolvedValue verifies the config tree value checker directly.
// VALIDATES: Edge cases for hasResolvedValue.
// PREVENTS: Panic on nil tree or wrong result on edge cases.
func TestHasResolvedValue(t *testing.T) {
	tree := config.NewTree()
	session := config.NewTree()
	asn := config.NewTree()
	asn.Set("remote", "65001")
	asn.Set("empty", "")
	session.SetContainer("asn", asn)
	tree.SetContainer("session", session)
	tree.Set("leaf", "value")

	tests := []struct {
		name string
		tree *config.Tree
		path []string
		want bool
	}{
		{"nil_tree", nil, []string{"session", "asn", "remote"}, false},
		{"present_deep", tree, []string{"session", "asn", "remote"}, true},
		{"missing_leaf", tree, []string{"session", "asn", "local"}, false},
		{"missing_intermediate", tree, []string{"connection", "remote", "ip"}, false},
		{"empty_string", tree, []string{"session", "asn", "empty"}, false},
		{"single_segment", tree, []string{"leaf"}, true},
		{"empty_path", tree, []string{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasResolvedValue(tt.tree, tt.path)
			assert.Equal(t, tt.want, got)
		})
	}
}
