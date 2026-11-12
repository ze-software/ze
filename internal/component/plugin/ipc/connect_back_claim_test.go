package ipc

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// hostileClaim is a name a peer that finished TLS can put in its auth request:
// a terminal escape that clears the screen, a newline that forges a second
// row, invalid UTF-8, a bidi override, and enough printable text that a
// three-byte rune straddles the shown bound.
var hostileClaim = "\x1b[2J" + "evil\nrow" + "\xff\xfe" + "‮" + strings.Repeat("a", 30) + "日本語" + strings.Repeat("b", 40)

// assertShowable fails unless shown is valid UTF-8, holds only runes strconv
// prints as themselves (no control character, no bidi override), and is no
// longer than the shown bound plus the elision mark.
func assertShowable(t *testing.T, what, shown string) {
	t.Helper()
	assert.True(t, utf8.ValidString(shown), "%s %q is not valid UTF-8", what, shown)
	for _, r := range shown {
		assert.True(t, strconv.IsPrint(r), "%s %q holds the non-printable rune %U", what, shown, r)
	}
	assert.LessOrEqual(t, len(shown), claimedNameShownMax+len(claimElided), "%s %q passes the bound", what, shown)
}

// TestShownClaimIsSafeToPrint guards shownClaim and escapeClaim: a claimed
// name off the wire becomes a report subject and a doctor record printed bare,
// so the shown form MUST carry no control character and no invalid UTF-8, and
// MUST stay within the bound after escaping, never only before it. Method:
// show hostile claims and check every rune and the length; show a name whose
// three-byte rune lands on the cut and check the cut fell before the rune.
func TestShownClaimIsSafeToPrint(t *testing.T) {
	claims := map[string]string{
		"hostile":           hostileClaim,
		"escape only":       "\x1b[2J",
		"newline":           "a\nb",
		"invalid UTF-8":     "\xff\xfe\xfd",
		"every byte escape": strings.Repeat("\x01", 200),
		"rune on the cut":   strings.Repeat("a", claimedNameShownMax-1) + "日" + "_",
		"quote and slash":   `a"b\c`,
	}
	for what, claim := range claims {
		shown := shownClaim(claim)
		assertShowable(t, what, shown)
		var message textbuf.Buffer
		quoted := writeQuotedClaim(&message, claim).String()
		assertShowable(t, what+" quoted", strings.Trim(quoted, `"`))
	}

	cut := shownClaim(strings.Repeat("a", claimedNameShownMax-1) + "日" + "_")
	assert.Equal(t, strings.Repeat("a", claimedNameShownMax-1)+claimElided, cut,
		"the cut lands before the rune it would split")
	assert.Equal(t, `\x1b[2J`, shownClaim("\x1b[2J"), "an escape character is written as its escape")
	assert.Equal(t, `a\nb`, shownClaim("a\nb"), "a newline is written as its escape")
}

// TestShownClaimKeepsTheNameForm guards the other half: a name of
// PluginNameForm is shown unchanged, so a doctor row and a managed record stay
// keyed by the name the operator configured, and its message form is the
// strconv quote the refusal always wrote. An empty claim shows as two quote
// marks, because the bus refuses an empty subject. Two different claims never
// show the same.
func TestShownClaimKeepsTheNameForm(t *testing.T) {
	for _, name := range []string{"edge-1", "a", strings.Repeat("z", claimedNameShownMax)} {
		require.True(t, ValidPluginName(name), "fixture %q is of the form", name)
		assert.Equal(t, name, shownClaim(name))
		var message textbuf.Buffer
		assert.Equal(t, strconv.Quote(name), writeQuotedClaim(&message, name).String())
	}
	assert.Equal(t, "a_b", shownClaim("a_b"), "a printable name off the form is still recognisable")
	assert.Equal(t, claimEmptyShown, shownClaim(""))
	assert.NotEqual(t, shownClaim(`""`), shownClaim(""), "a claim of two quote marks is not the empty claim")
	assert.NotEqual(t, shownClaim("a\nb"), shownClaim(`a\nb`), "a newline and a backslash-n stay distinct")
}

// TestConnectBackRefusedHostileClaim drives reportConnectBackRefused, the
// producer `show doctor` and `show errors` read, with a hostile claim under
// each cause, an empty claim, and an error carrying a verb as long as the auth
// frame. Method: read the bus and check each event exists (the bus drops an
// oversized or empty field in silence) and that its subject and message are
// printable.
func TestConnectBackRefusedHostileClaim(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)

	causes := []AuthRefusedCause{AuthRefusedName, AuthRefusedToken, AuthRefusedNoSecret}
	for i, cause := range causes {
		reportConnectBackRefused(fmt.Sprint("192.0.2.1:", 4000+i), &AuthRefusal{Claimed: strconv.Itoa(i) + hostileClaim, Cause: cause})
	}
	reportConnectBackRefused("192.0.2.1:4100", &AuthRefusal{Claimed: "", Cause: AuthRefusedName})
	reportConnectBackRefused("192.0.2.1:4200", fmt.Errorf("auth failed: expected method auth, got %q",
		strings.Repeat("\x01", MaxAuthFrameSize)))

	issues := report.Errors(0)
	require.Len(t, issues, len(causes)+2, "every refusal reaches the bus")
	subjects := map[string]bool{}
	for _, issue := range issues {
		assertShowable(t, "subject", issue.Subject)
		assert.True(t, utf8.ValidString(issue.Message), "message %q is not valid UTF-8", issue.Message)
		for _, r := range issue.Message {
			assert.True(t, strconv.IsPrint(r), "message %q holds the non-printable rune %U", issue.Message, r)
		}
		subjects[issue.Subject] = true
	}
	assert.True(t, subjects[claimEmptyShown], "the empty claim is raised under %q: %v", claimEmptyShown, subjects)
	assert.True(t, subjects["192.0.2.1:4200"], "the unnamed refusal is raised under its remote: %v", subjects)
}
