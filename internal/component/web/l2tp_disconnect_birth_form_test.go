// Design: docs/architecture/web-workbench-pages.md -- original session reference in generated actions

//go:build ze_l2tp

package web

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/l2tp"
	"github.com/ze-software/ze/internal/core/subscriberid"
)

// TestL2TPDetailFormCarriesItsSnapshotBirth renders the actual page from the
// shared response projection. The current snapshot's canonical birth reaches
// its hidden precondition; an absent birth cannot offer an actionable button.
func TestL2TPDetailFormCarriesItsSnapshotBirth(t *testing.T) {
	birth, err := subscriberid.ParseL2TPBirth("l2tp-00000000000000000000000000000001")
	require.NoError(t, err)
	for _, guarded := range []bool{true, false} {
		t.Run(map[bool]string{true: "guarded", false: "absent"}[guarded], func(t *testing.T) {
			snapshot := l2tp.SessionSnapshot{LocalSID: 42}
			if guarded {
				snapshot.Birth = birth
			}
			view := l2tpDetailViewOf(l2tpDetailBody{Session: snapshot})
			var output bytes.Buffer
			require.NoError(t, l2tpDetail(view).Render(t.Context(), &output))
			document := parseFinderHTML(t, output.String())
			form := finderElement(t, document, "action", "/l2tp/42/disconnect")
			fields := finderElements(form, "name", "expected-id")
			require.Len(t, fields, 1)
			require.Equal(t, "hidden", finderAttr(fields[0], "type"))
			require.Equal(t, snapshot.Birth.String(), finderAttr(fields[0], "value"))
			button := finderElement(t, form, "type", "submit")
			disabled := false
			for _, attribute := range button.Attr {
				if attribute.Key == "disabled" {
					disabled = true
				}
			}
			require.Equal(t, !guarded, disabled)
			if !guarded {
				require.Contains(t, finderText(form), "Refresh /l2tp")
			}
		})
	}
}
