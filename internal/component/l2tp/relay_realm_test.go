package l2tp

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRelayRealmSyntaxBoundaries checks the realm syntax the daemon refuses.
//
// VALIDATES: spec-home-cpe-14 AC-9 boundaries -- 1 and 253 octets accepted,
// 0 and 254 refused; '@' and whitespace refused, with the fix in the text.
// Method: checkRealmSyntax directly, because the YANG pattern refuses the same
// values earlier on the editor path and would hide this check.
func TestRelayRealmSyntaxBoundaries(t *testing.T) {
	require.NoError(t, checkRealmSyntax("n"))
	require.NoError(t, checkRealmSyntax(strings.Repeat("a", realmLengthMax)))

	err := checkRealmSyntax("")
	require.ErrorIs(t, err, errL2tpRelayRealmInvalid)
	assert.Contains(t, err.Error(), "empty")

	err = checkRealmSyntax(strings.Repeat("a", realmLengthMax+1))
	require.ErrorIs(t, err, errL2tpRelayRealmInvalid)

	err = checkRealmSyntax("cpe@example.net")
	require.ErrorIs(t, err, errL2tpRelayRealmInvalid)
	assert.Contains(t, err.Error(), "l2tp relay-realm example.net", "the error names the realm to write")

	err = checkRealmSyntax("example net")
	require.ErrorIs(t, err, errL2tpRelayRealmInvalid)
}

// TestRelayRealmDuplicateAfterFolding refuses two bindings that differ only in
// case, because the folded match could pick either.
func TestRelayRealmDuplicateAfterFolding(t *testing.T) {
	p := Parameters{
		Remotes:     []Remote{{Name: "lns-a"}},
		RelayRealms: []RealmRelayBinding{{Realm: "example.net", Remote: "lns-a"}, {Realm: "EXAMPLE.net", Remote: "lns-a"}},
	}
	require.ErrorIs(t, validateRelayRealms(&p), errL2tpRelayRealmDuplicate)
}
