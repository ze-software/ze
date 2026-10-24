package cli

import (
	"errors"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
)

// TestEditorReportsPluginVerifyError proves the editor's filter over
// VerifyPluginConfig: only the copy relayed from the bgp extractor is dropped,
// and only when the editor's own pass already refused the config.
//
// VALIDATES: a relayed bgp extraction failure is skipped when the editor
// refused the config and added when it did not; a plugin block extraction
// failure and any other verifier error are always added.
// PREVENTS: the editor listing a bgp failure twice, and the editor calling a
// bgp tree the daemon cannot load valid because the relayed copy was its only
// report.
func TestEditorReportsPluginVerifyError(t *testing.T) {
	cause := errors.New("duplicate remote IP 1.1.1.1 in peer peer2 (already used by peer peer1)")
	relayed := &config.PluginExtractionError{Section: string(config.ConfigTypeBGP), Err: cause}
	pluginBlock := &config.PluginExtractionError{Err: errors.New(`plugin "x": internal plugin requires use`)}
	verifier := errors.New("bgp-rib: some verifier refusal")

	tests := []struct {
		name          string
		err           error
		editorRefused bool
		want          bool
	}{
		{"relayed bgp copy, editor refused", relayed, true, false},
		{"relayed bgp copy, editor found nothing", relayed, false, true},
		{"plugin block failure, editor refused", pluginBlock, true, true},
		{"plugin block failure, editor found nothing", pluginBlock, false, true},
		{"verifier error, editor refused", verifier, true, true},
		{"verifier error, editor found nothing", verifier, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := editorReportsPluginVerifyError(tt.err, tt.editorRefused); got != tt.want {
				t.Fatalf("editorReportsPluginVerifyError = %v, want %v", got, tt.want)
			}
		})
	}
}
