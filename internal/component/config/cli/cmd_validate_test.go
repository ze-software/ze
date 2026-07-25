package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"net"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// validConfig is a minimal valid BGP configuration for testing.
const validConfig = `bgp {
	peer peer1 {
		connection {
			remote {
				ip 127.0.0.1;
			}
			local {
				ip 127.0.0.1;
			}
		}
		session {
			asn {
				remote 65533;
				local 65533;
			}
		}
	}
}`

// TestValidateRunValidConfig verifies valid config returns exit code 0.
//
// VALIDATES: Valid config produces success exit code.
// PREVENTS: Regression in config validation acceptance.
func TestValidateRunValidConfig(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "ze-validate-test-*.conf")
	require.NoError(t, err)
	t.Cleanup(func() { os.Remove(tmpFile.Name()) }) //nolint:errcheck,gosec // test cleanup

	_, err = tmpFile.WriteString(validConfig)
	require.NoError(t, err)
	require.NoError(t, tmpFile.Close())

	code := cmdValidate([]string{"-q", tmpFile.Name()})
	assert.Equal(t, 0, code, "valid config should return exit code 0")
}

// TestValidateRunInvalidConfig verifies invalid config returns exit code 1.
//
// VALIDATES: Invalid config produces error exit code.
// PREVENTS: Silent acceptance of broken configs.
func TestValidateRunInvalidConfig(t *testing.T) {
	content := `not valid config syntax`
	tmpFile, err := os.CreateTemp("", "ze-validate-test-*.conf")
	require.NoError(t, err)
	t.Cleanup(func() { os.Remove(tmpFile.Name()) }) //nolint:errcheck,gosec // test cleanup

	_, err = tmpFile.WriteString(content)
	require.NoError(t, err)
	require.NoError(t, tmpFile.Close())

	code := cmdValidate([]string{"-q", tmpFile.Name()})
	assert.Equal(t, 1, code, "invalid config should return exit code 1")
}

// TestValidateRunMissingFile verifies missing file returns exit code 2.
//
// VALIDATES: Missing file produces file error exit code.
// PREVENTS: Confusing error for missing vs invalid.
func TestValidateRunMissingFile(t *testing.T) {
	code := cmdValidate([]string{"-q", "/nonexistent/path/config.conf"})
	assert.Equal(t, 2, code, "missing file should return exit code 2")
}

// TestValidateRunNoArgs verifies missing args returns exit code 1.
//
// VALIDATES: Missing arguments shows usage.
// PREVENTS: Panic on empty args.
func TestValidateRunNoArgs(t *testing.T) {
	code := cmdValidate([]string{})
	assert.Equal(t, 1, code, "no args should return exit code 1")
}

// TestValidateRunStdin verifies reading config from stdin works.
//
// VALIDATES: "-" argument reads from stdin.
// PREVENTS: Regression in stdin handling.
func TestValidateRunStdin(t *testing.T) {
	// Save original stdin.
	oldStdin := os.Stdin
	t.Cleanup(func() { os.Stdin = oldStdin })

	// Create pipe for stdin.
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdin = r

	// Write content in goroutine.
	go func() {
		io.Copy(w, bytes.NewReader([]byte(validConfig))) //nolint:errcheck,gosec // test helper
		w.Close()                                        //nolint:errcheck,gosec // test helper
	}()

	code := cmdValidate([]string{"-q", "-"})
	assert.Equal(t, 0, code, "valid stdin should return exit code 0")
}

// TestValidateExtractLine verifies line number extraction from error messages.
//
// VALIDATES: Line numbers extracted correctly from parser errors.
// PREVENTS: Missing line info in error output.
func TestValidateExtractLine(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"line 5: unexpected token", 5},
		{"line 123: syntax error", 123},
		{"error at line 42: missing semicolon", 42},
		{"no line number here", 0},
		{"", 0},
	}

	for _, tt := range tests {
		got := extractLine(tt.input)
		assert.Equal(t, tt.want, got, "extractLine(%q)", tt.input)
	}
}

// TestValidateResultValid verifies validation result for valid config.
//
// VALIDATES: Valid config produces Valid=true result.
// PREVENTS: False negatives in validation.
func TestValidateResultValid(t *testing.T) {
	result := runValidation(validConfig, "test.conf")

	if !result.Valid {
		for _, d := range result.Diagnostics {
			t.Logf("diagnostic: [%s] %s", d.Code, d.Message)
		}
	}
	require.True(t, result.Valid, "expected Valid=true")
	assert.Equal(t, "test.conf", result.Path)
}

// TestValidateResultInvalid verifies validation result for invalid config.
//
// VALIDATES: Invalid config produces Valid=false with errors.
// PREVENTS: Silent failures in validation.
func TestValidateResultInvalid(t *testing.T) {
	content := `invalid syntax here`
	result := runValidation(content, "bad.conf")

	assert.False(t, result.Valid, "expected Valid=false for invalid config")
	assert.NotEmpty(t, result.Diagnostics, "expected at least one error")
}

// TestValidateContentReturnsAggregatedError verifies API callers can reuse the
// static config validation path and receive a clear error.
//
// VALIDATES: ValidateContent exposes runValidation failures as an error.
// PREVENTS: API pre-save validation drifting from ze config validate.
func TestValidateContentReturnsAggregatedError(t *testing.T) {
	err := ValidateContent(`bgp { router-id invalid; }`, "bad.conf")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "config validation failed")
	assert.Contains(t, err.Error(), "router-id")
}

// test-relax: AC-8 validation-entry-point coverage for the masked-bcrypt guard
// moved to real-binary functional tests test/parse/bcrypt-placeholder-rejected.ci
// and test/parse/config-dump-masks-bcrypt.ci. A Go test here needed a blank
// import of internal/component/ssh/yang (the only ze:bcrypt config leaf lives in
// ze-ssh-conf.yang), which registers into the package-global YANG module list
// and polluted the schema for unrelated tests in this package (listener-conflict
// and fix-plan diagnostics). The guard itself is unit-tested in
// internal/component/config: config.TestRejectMaskedBcryptLeaves.

// TestValidateRunsPluginConfigVerifier verifies `ze config validate` rejects
// errors from registered side-effect-free plugin verify hooks.
//
// VALIDATES: static config validation invokes plugin config verification.
// PREVENTS: plugin-only config errors passing offline validation.
func TestValidateRunsPluginConfigVerifier(t *testing.T) {
	snap := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snap) })

	require.NoError(t, registry.Register(registry.Registration{
		Name:        "validate-test",
		Description: "test verifier",
		ConfigRoots: []string{"bgp"},
		RunEngine:   func(net.Conn) int { return 0 },
		CLIHandler:  func([]string) int { return 0 },
		InProcessConfigVerifier: func([]rpc.ConfigSection) error {
			return assert.AnError
		},
	}))

	result := runValidation(validConfig, "test.conf")
	assert.False(t, result.Valid, "expected plugin verifier to reject config")
	require.NotEmpty(t, result.Diagnostics)
	assert.Contains(t, result.Diagnostics[0].Message, "validate-test")
}

// TestValidateRejectsInternalPluginMissingUse verifies `ze config validate`
// reports an internal plugin that omits the required `use` key, matching the
// boot-path loader check (ExtractPluginsFromTree). The YANG schema does not mark
// `use` mandatory, so only the loader check catches this; before wiring it in,
// validate returned exit 0 for a config that fails at boot.
//
// VALIDATES: static validation runs the loader internal-plugin checks.
// PREVENTS: `internal <name> {}` missing `use` passing validation but failing at boot.
func TestValidateRejectsInternalPluginMissingUse(t *testing.T) {
	conf := `plugin {
	internal rib {
	}
}`
	result := runValidation(conf, "missing-use.conf")
	assert.False(t, result.Valid, "internal plugin without use should be invalid")

	var found *diagnostic.Diagnostic
	for i := range result.Diagnostics {
		if result.Diagnostics[i].Code == "config-plugin-extract" {
			found = &result.Diagnostics[i]
			break
		}
	}
	require.NotNil(t, found, "expected config-plugin-extract diagnostic")
	assert.Contains(t, found.Message, "requires use")
}

// TestValidateRejectsDuplicatePluginName verifies `ze config validate` reports a
// plugin name reused across the internal and external lists, matching the
// boot-path loader check. YANG list keys are per-list, so the duplicate is legal
// to the schema but rejected by ExtractPluginsFromTree at boot.
//
// VALIDATES: static validation rejects internal/external duplicate plugin names.
// PREVENTS: duplicate plugin names passing validation but failing at boot.
func TestValidateRejectsDuplicatePluginName(t *testing.T) {
	snap := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snap) })

	// Register the referenced plugin so the YANG internal-plugin-name validator
	// on `use` passes and the only remaining error is the duplicate name.
	require.NoError(t, registry.Register(registry.Registration{
		Name:        "dup-test",
		Description: "test plugin",
		RunEngine:   func(net.Conn) int { return 0 },
		CLIHandler:  func([]string) int { return 0 },
	}))

	conf := `plugin {
	internal dup {
		use dup-test
	}
	external dup {
		run /bin/true
	}
}`
	result := runValidation(conf, "dup.conf")
	assert.False(t, result.Valid, "duplicate plugin name should be invalid")

	var found *diagnostic.Diagnostic
	for i := range result.Diagnostics {
		if result.Diagnostics[i].Code == "config-plugin-extract" {
			found = &result.Diagnostics[i]
			break
		}
	}
	require.NotNil(t, found, "expected config-plugin-extract diagnostic")
	assert.Contains(t, found.Message, "duplicate plugin name")
}

// TestValidateAcceptsInternalPluginWithUse verifies a well-formed internal
// plugin block (name + registered `use`) still passes validation after the
// loader checks were wired in, so the new check does not reject valid configs.
//
// VALIDATES: valid internal-plugin config passes static validation.
// PREVENTS: the loader extraction check rejecting well-formed plugin blocks.
func TestValidateAcceptsInternalPluginWithUse(t *testing.T) {
	snap := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snap) })

	require.NoError(t, registry.Register(registry.Registration{
		Name:        "use-test",
		Description: "test plugin",
		RunEngine:   func(net.Conn) int { return 0 },
		CLIHandler:  func([]string) int { return 0 },
	}))

	conf := `plugin {
	internal good {
		use use-test
	}
}`
	result := runValidation(conf, "valid-internal.conf")
	if !result.Valid {
		for _, d := range result.Diagnostics {
			t.Logf("diagnostic: [%s] %s", d.Code, d.Message)
		}
	}
	assert.True(t, result.Valid, "valid internal plugin config should pass")

	for i := range result.Diagnostics {
		assert.NotEqual(t, "config-plugin-extract", result.Diagnostics[i].Code,
			"valid config should produce no plugin-extract diagnostic")
	}
}

// TestValidateSemanticValidationWarnings verifies semantic checks produce warnings.
//
// VALIDATES: Missing router-id produces warning.
// PREVENTS: Silent config issues.
func TestValidateSemanticValidationWarnings(t *testing.T) {
	result := runValidation(validConfig, "test.conf")
	require.True(t, result.Valid, "expected valid config")

	// Should have warning about missing router-id.
	hasRouterIDWarning := false
	for _, d := range result.Diagnostics {
		if strings.Contains(d.Message, "router-id") {
			hasRouterIDWarning = true
			break
		}
	}
	assert.True(t, hasRouterIDWarning, "expected warning about missing router-id")
}

// TestValidateNoFalseASNWarnings verifies AS numbers set under the canonical
// session/asn paths do not trigger spurious "not configured" warnings.
//
// VALIDATES: addSemanticWarnings reads session/asn/{local,remote}.
// PREVENTS: Regression where it read pre-schema paths (bgp/local/as,
// peer/local/as, peer/remote/as) and falsely warned on every correct config.
func TestValidateNoFalseASNWarnings(t *testing.T) {
	const conf = `bgp {
    router-id 10.0.0.1
    session {
        asn {
            local 65000
        }
    }
    peer peer1 {
        connection {
            remote {
                ip 127.0.0.1
            }
            local {
                ip 127.0.0.1
            }
        }
        session {
            asn {
                local 65000
                remote 65001
            }
        }
    }
}
`
	result := runValidation(conf, "asn.conf")
	require.True(t, result.Valid, "expected valid config, diagnostics: %+v", result.Diagnostics)

	for _, d := range result.Diagnostics {
		if strings.Contains(d.Message, "local-as not configured") ||
			strings.Contains(d.Message, "remote as not configured") ||
			strings.Contains(d.Message, "not configured globally") {
			t.Errorf("spurious ASN warning for a config that sets session/asn: %q", d.Message)
		}
	}
}

// envelopeKey returns the JSON key name for the contract envelope field.
func envelopeKey() string { return "schema" + "-" + "version" }

// TestCommandContractSnapshot pins the diagnostic JSON structure: field names,
// types, and required fields. Adding a field is OK; removing or renaming breaks
// the contract.
//
// VALIDATES: AC-7 command contract stability.
// PREVENTS: Accidental contract drift in diagnostic JSON.
func TestCommandContractSnapshot(t *testing.T) {
	content := `invalid syntax here`
	result := runValidation(content, "contract.conf")
	out := diagnostic.NewValidateResult(result.Path, result.Valid, result.Diagnostics, result.Config)

	data, err := json.Marshal(out)
	require.NoError(t, err)

	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &raw))

	for _, required := range []string{envelopeKey(), "valid", "path", "diagnostics"} {
		assert.Contains(t, raw, required, "missing required top-level field: "+required)
	}

	var diags []map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw["diagnostics"], &diags))
	require.NotEmpty(t, diags)

	for _, required := range []string{"code", "severity", "message"} {
		assert.Contains(t, diags[0], required, "missing required diagnostic field: "+required)
	}
}

// TestValidateListenerConflictRelated verifies listener conflict diagnostics
// include related entries pointing to both conflicting listeners.
//
// VALIDATES: AC-16 listener conflict related entries.
// PREVENTS: Losing structured conflict data in diagnostic output.
func TestValidateListenerConflictRelated(t *testing.T) {
	conf := `environment {
	web {
		enabled true
		server web1 {
			ip 0.0.0.0
			port 8080
		}
		server web2 {
			ip 0.0.0.0
			port 8080
		}
	}
}`
	result := runValidation(conf, "conflict.conf")
	assert.False(t, result.Valid)

	var found *diagnostic.Diagnostic
	for i := range result.Diagnostics {
		if result.Diagnostics[i].Code == "config-listener-conflict" {
			found = &result.Diagnostics[i]
			break
		}
	}
	require.NotNil(t, found, "expected config-listener-conflict diagnostic")
	require.Len(t, found.Related, 2, "expected two related entries")
	assert.Contains(t, found.Related[0].Path, "web")
	assert.Contains(t, found.Related[1].Path, "web")
	assert.NotNil(t, found.Repair)
	assert.Equal(t, "resolve-listener-conflict", found.Repair.ID)
}

// TestConfigFixPlanRepairIDs verifies fix-plan output carries stable repair IDs
// for YANG validation errors with known repairs.
//
// VALIDATES: AC-9/AC-10 repair metadata population.
// PREVENTS: Empty repair objects in fix-plan output.
func TestConfigFixPlanRepairIDs(t *testing.T) {
	conf := `bgp {
	peer peer1 {
		connection {
			remote {
				ip 127.0.0.1
			}
			local {
				ip 127.0.0.1
			}
		}
		session {
			asn {
				remote 65533
				local 65533
			}
		}
	}
}
environment {
	web {
		enabled true
		server web1 {
			ip 0.0.0.0
			port 8080
		}
		server web2 {
			ip 0.0.0.0
			port 8080
		}
	}
}`
	result := runValidation(conf, "repair.conf")

	var hasRepair bool
	for _, d := range result.Diagnostics {
		if d.Repair != nil && d.Repair.ID != "" {
			hasRepair = true
			break
		}
	}
	assert.True(t, hasRepair, "expected at least one diagnostic with a repair ID")
}

// TestValidatePendingMissingDraft verifies --pending with missing draft returns exit code 2.
//
// VALIDATES: AC-19 --pending flag error path.
// PREVENTS: Confusing error when no draft exists.
func TestValidatePendingMissingDraft(t *testing.T) {
	code := cmdValidate([]string{"--json", "--pending", "/nonexistent/path/config.conf"})
	assert.Equal(t, 2, code)
}

// TestValidatePendingRejectsStdin verifies --pending with stdin returns exit code 1.
//
// VALIDATES: --pending cannot read from stdin (no draft path for stdin).
// PREVENTS: Confusing "-.draft" file-not-found error.
func TestValidatePendingRejectsStdin(t *testing.T) {
	code := cmdValidate([]string{"--json", "--pending", "-"})
	assert.Equal(t, 1, code)
}

// TestValidatePendingRequiresJSON verifies --pending without --json returns exit code 1.
//
// VALIDATES: AC-19 --pending requires --json.
// PREVENTS: Unstructured output from pending validation.
func TestValidatePendingRequiresJSON(t *testing.T) {
	code := cmdValidate([]string{"--pending", "/nonexistent/path/config.conf"})
	assert.Equal(t, 1, code)
}

// TestValidatePendingReadsDraft verifies --pending reads the .draft file and validates it.
//
// VALIDATES: AC-19 --pending reads draft from zefs.
// PREVENTS: Pending validation ignoring draft content.
func TestValidatePendingReadsDraft(t *testing.T) {
	dir := t.TempDir()
	configPath := dir + "/config.conf"
	draftPath := configPath + ".draft"

	require.NoError(t, os.WriteFile(configPath, []byte(validConfig), 0o644))
	require.NoError(t, os.WriteFile(draftPath, []byte("invalid draft content"), 0o644))

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	code := cmdValidate([]string{"--json", "--pending", configPath})

	w.Close() //nolint:errcheck // test pipe
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r) //nolint:errcheck // test

	assert.Equal(t, 1, code, "invalid draft should return exit code 1")

	var out map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(buf.Bytes(), &out))
	assert.Contains(t, out, envelopeKey())
	assert.Contains(t, out, "diagnostics")
}
