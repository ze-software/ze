// Design: docs/architecture/web-interface.md -- typed discard audit refusals.
package web

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/opererr"
)

// TestDiscardAuditDetailKeepsItsTypedRefusal exercises the post-discard review
// boundary: native recovery and joined causes survive, while a healthy review
// returns a nil refusal and excludes retained changes from the audit detail.
func TestDiscardAuditDetailKeepsItsTypedRefusal(t *testing.T) {
	cause := errors.New("discard review read failed")
	closeFailure := errors.New("discard review release failed")
	native := opererr.Wrap(cause, "Discard audit review is unavailable.",
		"The remaining configuration changes could not be inspected.", "ze show doctor")
	for _, test := range []struct {
		name string
		err  error
	}{
		{name: "plain cause", err: cause},
		{name: "native refusal", err: native},
		{name: "joined causes", err: errors.Join(native, closeFailure)},
		{name: "healthy review"},
	} {
		t.Run(test.name, func(t *testing.T) {
			mgr := newTestEditorManager(t)
			require.NoError(t, mgr.SetValue("alice", []string{"bgp"}, "router-id", "192.0.2.10"))
			session, err := mgr.GetOrCreate("alice")
			require.NoError(t, err)
			session.editor = &reviewFailureEditor{Editor: session.editor, err: test.err}
			const retained = "- bgp router-id 1.2.3.4\n+ bgp router-id 192.0.2.10\n"
			const dropped = "+ bgp description discarded\n"
			var detail string
			var refusal *opererr.Error
			detail, refusal = discardAuditDetail(mgr, "alice", retained+dropped)
			if test.err == nil {
				require.Nil(t, refusal, "a healthy read must not become a typed-nil error interface")
				assert.Equal(t, dropped, detail)
				return
			}
			require.NotNil(t, refusal)
			assert.Empty(t, detail, "a failed review cannot supply successful discard audit detail")
			assert.NotEmpty(t, refusal.Problem())
			assert.NotEmpty(t, refusal.Why())
			assert.NotEmpty(t, refusal.Fix())
			assert.ErrorIs(t, refusal, cause)
			if test.err == native {
				assert.Same(t, native, refusal)
				assert.Equal(t, native.Error(), refusal.Error())
			}
			if errors.Is(test.err, closeFailure) {
				assert.ErrorIs(t, refusal, closeFailure)
				assert.Contains(t, refusal.Error(), closeFailure.Error())
			}
		})
	}
}
