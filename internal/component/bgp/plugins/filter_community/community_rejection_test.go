// Design: docs/architecture/core-design.md -- A Redistribution Rule Never Gates What BGP Learns
// Related: community_filter.go -- the adapter shared by both ingress members

package filter_community

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// TestCommunityRewritePreservesRejection proves that a rejected ingress
// decision cannot become accept or modify when the plugin adapts its answer.
// Method: a negative decision with no rewrite, an unchanged body, and a
// community edit must produce only FilterReject. The existing positive
// rewrite test continues to prove accepted routes retain their edits.
func TestCommunityRewritePreservesRejection(t *testing.T) {
	original := buildPayload(append(buildOriginAttr(), buildCommunityAttr(0xFDE80001)...))
	for _, tc := range []struct {
		name     string
		modified []byte
	}{
		{name: "no-rewrite"},
		{name: "unchanged-body", modified: original},
		{name: "community-edit", modified: buildPayload(append(buildOriginAttr(), buildCommunityAttr(0xFDE80002)...))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, err := rewriteVerdict(false, original, tc.modified)
			require.NoError(t, err)
			require.Equal(t, &sdk.FilterUpdateOutput{Action: sdk.FilterReject}, out,
				"a rejected route must not become accepted or carry edits")
		})
	}
}
