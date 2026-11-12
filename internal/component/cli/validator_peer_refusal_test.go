// Design: docs/guide/config-editor.md -- refusals and their fix lines
// Related: validator.go -- peerIssue, refusalIssue, formatPeerError

package cli

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/yang"
)

// TestFormatPeerErrorPassesTheRefusalThrough proves the peer walk passes the
// schema's enum refusal through as the operator error, with the peer in its
// path and the `set` of that path as its fix, and never the peer framing
// formatPeerError writes.
// Method: the peer walk's own validator judges a peer map holding
// `route-refresh requre`; its enum refusal is shown through peerIssue for a
// standalone peer and for a group member, and each text is compared.
//
// VALIDATES: the formatPeerError pass-through (validatePeer -> peerIssue ->
// refusalIssue) for ErrTypeEnum.
// PREVENTS: `peer peer1: "route-refresh" - ...` hiding the accepted values and
// the fix the refusal carries.
// Break: delete the HoldsOperatorError arm of peerIssue; both cases then read
// the formatPeerError text and go red.
func TestFormatPeerErrorPassesTheRefusalThrough(t *testing.T) {
	v, err := NewConfigValidator()
	require.NoError(t, err)
	peer := map[string]any{"session": map[string]any{"capability": map[string]any{"route-refresh": "requre"}}}
	var refusal *yang.ValidationError
	errs := v.yangValidator.ValidateTree(peerYANGPath, peer)
	for i := range errs {
		if errs[i].Type == yang.ErrTypeEnum {
			refusal = &errs[i]
		}
	}
	require.NotNil(t, refusal, "the walk refuses the enum value: %v", errs)
	require.True(t, refusal.HoldsOperatorError(), "the walk's enum refusal holds the operator error")

	cases := []struct {
		name, group, problem, fix string
	}{
		{name: "standalone", problem: routeRefreshProblem, fix: routeRefreshRootFix},
		{name: "in a group", group: "g1",
			problem: strings.Replace(routeRefreshProblem, "bgp peer", "bgp group g1 peer", 1),
			fix:     strings.Replace(routeRefreshRootFix, "bgp peer", "bgp group g1 peer", 1)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			issue := v.peerIssue(7, severityError, "peer1", tc.group, refusal)
			text := issue.Text(nil)
			assert.Equal(t, 7, issue.Line)
			assert.Equal(t, severityError, issue.Severity)
			assert.True(t, strings.HasPrefix(text, tc.problem), "text %q", text)
			assert.Equal(t, tc.fix, lastLine(text))
			assert.NotContains(t, text, `peer peer1: "route-refresh" -`, "not the peer framing")
		})
	}
}
