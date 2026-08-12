// VALIDATES: a custom-validator refusal never reaches RecoverConfig, so an
// invalid config stops the daemon instead of being replaced on disk by a
// rollback (AC-1, spec-fixit-config-validators-bypassed-at-startup).
// PREVENTS: the route the validation call in LoadConfig opened. RecoverConfig
// (internal/component/config/stamp.go) runs when the config's schema stamp is
// NEWER than this binary's release, walks rollback history, and WRITES the
// first version that loads back over the config file. Before LoadConfig
// validated, a validator refusal was never a LoadConfig error, so that path
// could not be reached by one. After it, an operator who hand-edits a bad
// hostname on a downgraded binary would have had their file overwritten and the
// daemon started on a config they never wrote.

package hub

import (
	"errors"
	"testing"

	zeconfig "github.com/ze-software/ze/internal/component/config"
)

// TestRecoverableLoadErrorDeclinesAValidationRefusal drives the predicate with
// the errors LoadConfig really returns, not with hand-built ones: the refusal
// must come from the walk, so dropping the %w wrap in
// refuseInvalidCustomSections turns this test red.
func TestRecoverableLoadErrorDeclinesAValidationRefusal(t *testing.T) {
	const refused = `plugin {
	internal p1 {
		use no-such-plugin-here
	}
}
`
	_, err := zeconfig.LoadConfig(refused, "test.conf", nil)
	if err == nil {
		t.Fatal("LoadConfig accepted a config the internal-plugin-name validator refuses")
	}
	if !errors.Is(err, zeconfig.ErrCustomValidation) {
		t.Fatalf("a validator refusal must carry ErrCustomValidation: %v", err)
	}
	if recoverableLoadError(err) {
		t.Error("a validator refusal must not be answered by rewriting the config from rollback history")
	}
}

// TestRecoverableLoadErrorAllowsAParseFailure is the other half: version-skew
// recovery must keep working. A syntax error is what a binary older than the
// config's schema stamp actually sees, and it stays recoverable.
func TestRecoverableLoadErrorAllowsAParseFailure(t *testing.T) {
	_, err := zeconfig.LoadConfig("bgp {\n\tthis is not valid syntax\n}\n", "test.conf", nil)
	if err == nil {
		t.Fatal("LoadConfig accepted a syntactically invalid config")
	}
	if !recoverableLoadError(err) {
		t.Errorf("a parse failure must stay recoverable: %v", err)
	}
}
