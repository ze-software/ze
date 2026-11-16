// Design: docs/architecture/api/process-protocol.md -- declared peer-role requirements.
// Related: dissemination.go -- static shape and initialized event resolution.
package registry

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func disseminationDeclaration() rpc.DisseminationDecl {
	return rpc.DisseminationDecl{
		Namespace: "namespace-before-initialization", Role: "selected-output",
		SourceReceive:      []string{"update-received", "state"},
		DestinationReceive: []string{"state", "refresh", "update-sent"}, DestinationSend: []string{"update"},
	}
}

// TestDisseminationDeclarationShape proves incomplete or malformed external
// values are refused without making static registration depend on init order.
// Method: check the valid shape, then remove one required fact per case.
func TestDisseminationDeclarationShape(t *testing.T) {
	for _, defect := range []string{"none", "role", "namespace", "source", "destination receive", "destination send", "receive whitespace", "send control", "receive wildcard", "send wildcard", "duplicate"} {
		t.Run(defect, func(t *testing.T) {
			declaration := disseminationDeclaration()
			declarations := []rpc.DisseminationDecl{declaration}
			switch defect {
			case "none":
			case "role":
				declarations[0].Role = ""
			case "namespace":
				declarations[0].Namespace = ""
			case "source":
				declarations[0].SourceReceive = nil
			case "destination receive":
				declarations[0].DestinationReceive = nil
			case "destination send":
				declarations[0].DestinationSend = nil
			case "receive whitespace":
				declarations[0].SourceReceive = []string{"update state"}
			case "send control":
				declarations[0].DestinationSend = []string{"update\x1b"}
			case "receive wildcard":
				declarations[0].SourceReceive = []string{"*"}
			case "send wildcard":
				declarations[0].DestinationSend = []string{"*"}
			case "duplicate":
				declarations = append(declarations, declaration)
			}
			err := CheckDisseminationDeclarations("configured-process", declarations)
			if defect == "none" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			require.ErrorContains(t, err, "configured-process")
		})
	}
}

// TestDisseminationDeclarationRequiresInitializedEvents proves an unknown
// namespace cannot become runtime authority. Method: a shape valid before
// component init is refused by the later event-resolution boundary.
func TestDisseminationDeclarationRequiresInitializedEvents(t *testing.T) {
	declarations := []rpc.DisseminationDecl{disseminationDeclaration()}
	require.NoError(t, CheckDisseminationDeclarations("configured-process", declarations))
	err := CheckDisseminationEvents("configured-process", declarations)
	require.Error(t, err)
	require.ErrorContains(t, err, "namespace-before-initialization")
}
