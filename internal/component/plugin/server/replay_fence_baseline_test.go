// Design: docs/architecture/api/process-protocol.md -- fence readiness prerequisite.
package server

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func TestExternalReplayFenceDeclarationRequiresReady(t *testing.T) {
	for _, row := range []struct {
		body    string
		refused bool
	}{
		{`{"fences-live-forwards":true}`, true},
		{`{"fences-live-forwards":true,"signals-session-ready":true}`, false},
		{`{"signals-session-ready":true}`, false},
		{`{}`, false},
	} {
		var declaration rpc.DeclareRegistrationInput
		require.NoError(t, json.Unmarshal([]byte(row.body), &declaration))
		declaration.ProtocolVersion = rpc.ProtocolVersion
		err := refuseDriverChecks("external-fence", "external-fence", &declaration)
		if row.refused {
			require.ErrorContains(t, err, "signals-session-ready")
		} else {
			require.NoError(t, err)
		}
	}
}
