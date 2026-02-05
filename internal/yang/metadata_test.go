// Package yang provides YANG schema loading and validation for ze.
package yang

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestExtractMetadata verifies metadata extraction from YANG modules.
//
// VALIDATES: Module name, namespace, and imports are correctly extracted.
// PREVENTS: Schema discovery showing wrong module names or missing dependencies.
func TestExtractMetadata(t *testing.T) {
	tests := []struct {
		name     string
		yang     string
		wantMod  string
		wantNS   string
		wantImps []string
		wantErr  bool
	}{
		{
			name: "simple_module",
			yang: `module ze-test {
    namespace "urn:ze:test";
    prefix test;
    description "Test module";
}`,
			wantMod:  "ze-test",
			wantNS:   "urn:ze:test",
			wantImps: nil,
		},
		{
			name: "module_with_import",
			yang: `module ze-gr {
    namespace "urn:ze:gr";
    prefix gr;
    import ze-bgp { prefix bgp; }
    description "GR module";
}`,
			wantMod:  "ze-gr",
			wantNS:   "urn:ze:gr",
			wantImps: []string{"ze-bgp"},
		},
		{
			name: "module_with_multiple_imports",
			yang: `module ze-complex {
    namespace "urn:ze:complex";
    prefix complex;
    import ze-bgp { prefix bgp; }
    import ze-types { prefix zt; }
    import ze-extensions { prefix ze; }
    description "Complex module";
}`,
			wantMod:  "ze-complex",
			wantNS:   "urn:ze:complex",
			wantImps: []string{"ze-bgp", "ze-extensions", "ze-types"},
		},
		{
			name:    "invalid_yang",
			yang:    "not valid yang",
			wantErr: true,
		},
		{
			name:    "empty_content",
			yang:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta, err := ParseYANGMetadata(tt.yang)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantMod, meta.Module)
			assert.Equal(t, tt.wantNS, meta.Namespace)
			assert.Equal(t, tt.wantImps, meta.Imports)
		})
	}
}

// TestFormatNamespace verifies namespace formatting for display.
//
// VALIDATES: Namespace URNs are converted to display format (urn:ze:bgp → ze.bgp).
// PREVENTS: Inconsistent namespace display in `ze schema list` output.
func TestFormatNamespace(t *testing.T) {
	tests := []struct {
		ns   string
		want string
	}{
		{"urn:ze:bgp:conf", "ze.bgp.conf"},
		{"urn:ze:graceful-restart", "ze.graceful-restart"},
		{"urn:ze:hostname", "ze.hostname"},
		{"urn:ietf:params:xml:ns:yang:ietf-inet-types", "ietf.params.xml.ns.yang.ietf-inet-types"},
		{"http://example.com/yang", "http://example.com/yang"}, // Non-URN unchanged
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.ns, func(t *testing.T) {
			got := FormatNamespace(tt.ns)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestMetadataFromRealYANG verifies metadata extraction works with real embedded YANG.
//
// VALIDATES: Real ze-bgp YANG can be parsed for metadata.
// PREVENTS: Regression where embedded YANG changes break metadata extraction.
func TestMetadataFromRealYANG(t *testing.T) {
	// Load the real ze-bgp YANG to verify it can be parsed
	loader := NewLoader()
	require.NoError(t, loader.LoadEmbedded())

	// Get the ze-types module (one of the core embedded modules)
	mod := loader.GetModule("ze-types")
	require.NotNil(t, mod, "ze-types module should be available in embedded core modules")

	meta := ExtractMetadata(mod)
	assert.Equal(t, "ze-types", meta.Module)
	assert.Equal(t, "urn:ze:types", meta.Namespace)
}
