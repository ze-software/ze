package infra_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/infra"
	"github.com/ze-software/ze/internal/component/config/yang"
)

// TestExtractAuthzStoreSurfacesLoaderError: when yang.DefaultLoader refuses
// the module set, ExtractAuthzStore returns that error instead of checking the
// profile match entries against an empty command set. Method: parse a config
// with one profile first, then register a probe module whose `ze:hepl` names
// no declared extension, for this test only, and extract the store.
//
// VALIDATES: authz match validation reports the loader error.
// PREVENTS: every match entry warned as unknown, with the cause hidden.
func TestExtractAuthzStoreSurfacesLoaderError(t *testing.T) {
	tree, err := config.ParseTreeWithYANG(`
system {
    authorization {
        profile noc {
            run {
                default-action deny
                entry 10 {
                    action allow
                    match "peer show"
                }
            }
        }
    }
}
`, nil)
	require.NoError(t, err)

	t.Cleanup(yang.RegisterModuleForTest("ze-probe.yang", `module ze-probe {
    namespace "urn:ze:probe";
    prefix probe;
    import ze-extensions { prefix ze; }
    leaf probe {
        type string;
        ze:hepl "Probe leaf.";
    }
}`))

	store, err := infra.ExtractAuthzStore(tree)
	if !errors.Is(err, yang.ErrUndeclaredExtension) {
		t.Fatalf("ExtractAuthzStore answered (%v, %v), want ErrUndeclaredExtension", store, err)
	}
	if !strings.Contains(err.Error(), "ze:hepl") {
		t.Errorf("error %q does not name ze:hepl", err)
	}
}
