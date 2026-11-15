// Design: docs/architecture/wire/messages.md -- the CLI and YANG model agree on supported message operations

package cli

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	configyang "github.com/ze-software/ze/internal/component/config/yang"
)

// TestDecodeTypesMatchTheModel compares both directions: every modeled decode
// type names a supported message type, and every supported type is modeled.
// Standalone NLRI is a separate decode mode, not a BGP message type.
func TestDecodeTypesMatchTheModel(t *testing.T) {
	want := []string{msgTypeOpen, msgTypeUpdate, msgTypeNotification, msgTypeKeepalive}
	require.ElementsMatch(t, want, toolsModelEnum(t, pathDecode, leafDecodeType))
}

// TestEncodePartsMatchTheModel compares every encoded result part with the
// model's accepted parts, so neither side can add or remove one silently.
func TestEncodePartsMatchTheModel(t *testing.T) {
	want := []string{encodePartMessage, encodePartBody, encodePartNLRI}
	require.ElementsMatch(t, want, toolsModelEnum(t, pathEncode, leafEncodePart))
}

func toolsModelEnum(t *testing.T, path, leaf string) []string {
	t.Helper()
	loader, err := configyang.DefaultLoader()
	require.NoError(t, err)
	for _, argument := range configyang.PathToArgDefs(loader)[path] {
		if argument.Name() == leaf {
			return slices.Collect(argument.EnumValues())
		}
	}
	t.Fatalf("model has no %s leaf at %s", leaf, path)
	return nil
}
