//go:build ze_bgp

// Design: docs/architecture/api/architecture.md -- complete explicit transit grants.
// Related: ../../cli/editor_commit.go -- validate the actual staged candidate.
package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	editorcli "github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/config/storage"
)

// TestAC25ParsedGrantRefusalsReachValidateAndCommit asks whether the real file
// validator and editor candidate commit refuse the same three partial transit
// graphs, with the operator's exact diagnosis and no changed active content.
// The complete graph must validate and publish a candidate through the same
// path. This test does not start a daemon or claim startup or peer-wire proof.
func TestAC25ParsedGrantRefusalsReachValidateAndCommit(t *testing.T) {
	configuration := func(routerID, sourceProcess, sourceAllow, destinationSend string) string {
		return fmt.Sprintf(`bus {
 endpoint bgp-rib { internal bgp-rib; receive { allow none; } send { allow none; } }
 endpoint ac25-table { internal bgp-rib; }
 endpoint ac25-other { internal bgp-rib; }
}
bgp {
 router-id %s;
 session { asn { local 65000; } }
 peer source {
  connection { local { ip 192.0.2.254; } remote { ip 192.0.2.1; } }
  session { asn { remote 65001; } }
  filter { import { final accept; } export { final reject; } }
  bus {
   send { to [ /bus/endpoint/%s ]; allow [ %s ]; }
   receive { from none; }
  }
 }
 peer destination {
  connection { local { ip 192.0.2.254; } remote { ip 192.0.2.2; } }
  session { asn { remote 65002; } }
  filter { import { final reject; } export { final accept; } }
  bus {
   %s
   receive { from [ /bus/endpoint/ac25-table ]; allow [ bgp:update ]; }
  }
 }
}
`, routerID, sourceProcess, sourceAllow, destinationSend)
	}
	const sourceGrants = "bgp:update bgp:state"
	const destinationGrants = "send { to [ /bus/endpoint/ac25-table ]; allow [ bgp:state bgp:refresh bgp:update ]; message bgp:update { tag direction { value [ sent ]; } } }"
	const destinationPartial = "send { to [ /bus/endpoint/ac25-table ]; allow [ bgp:state ]; }"
	base := configuration("192.0.2.254", "ac25-table", sourceGrants, destinationGrants)
	for _, question := range []struct {
		name            string
		sourceProcess   string
		sourceAllow     string
		destinationSend string
		refusal         string
		fixLine         string
	}{
		{
			name: "source state only", sourceProcess: "ac25-table", sourceAllow: "bgp:state", destinationSend: destinationGrants,
			refusal: `participant "source" endpoint "ac25-table" has incomplete source grants for dissemination role "bgp-selected-output"`,
			fixLine: `bus { endpoint ac25-table { receive { allow [ bgp:update bgp:state ]; } } }`,
		},
		{
			name: "destination send only", sourceProcess: "ac25-table", sourceAllow: sourceGrants, destinationSend: destinationPartial,
			refusal: `participant "destination" endpoint "ac25-table" has incomplete destination grants for dissemination role "bgp-selected-output"`,
			fixLine: `bus { endpoint ac25-table { receive { allow [ bgp:state bgp:refresh bgp:update ]; } send { allow [ bgp:update ]; } } }`,
		},
		{
			name: "split instances", sourceProcess: "ac25-other", sourceAllow: sourceGrants, destinationSend: destinationGrants,
			refusal: `participant "destination" destination endpoint "ac25-table" has no complete source on this same instance for dissemination role "bgp-selected-output"`,
			fixLine: `bus { endpoint ac25-table { receive { allow [ bgp:update bgp:state ]; } } }`,
		},
		{
			name: "complete instance", sourceProcess: "ac25-table", sourceAllow: sourceGrants, destinationSend: destinationGrants,
		},
	} {
		t.Run(question.name, func(t *testing.T) {
			candidate := configuration("192.0.2.253", question.sourceProcess, question.sourceAllow, question.destinationSend)
			file := filepath.Join(t.TempDir(), "ac25.conf")
			require.NoError(t, os.WriteFile(file, []byte(candidate), 0o600))
			validationError := ValidateContent(candidate, file)
			wantCode := exitOK
			if question.refusal != "" {
				wantCode = exitInvalid
				require.ErrorContains(t, validationError, question.refusal)
				require.ErrorContains(t, validationError, question.fixLine)
			} else {
				require.NoError(t, validationError)
			}
			require.Equal(t, wantCode, cmdValidate([]string{"-q", file}), "the normal file command must enforce the same grant graph")

			store := storage.CreateMemory()
			t.Cleanup(func() { require.NoError(t, store.Close()) })
			const name = "ac25.conf"
			require.NoError(t, store.WriteFile(name, []byte(base), 0o600))
			ed, err := editorcli.NewEditorWithStorage(store, name)
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, ed.Close()) })
			ed.SetSession(editorcli.NewEditSession("ac25", "web"))
			loaded, err := ed.ParseLoad(candidate, nil)
			require.NoError(t, err)
			require.NoError(t, ed.LoadReplace(nil, loaded))
			validated := false
			ed.SetPreCommitValidate(func(staged string) error {
				validated = true
				return ValidateContent(staged, name)
			})
			result, output, commitError := ed.CommitSessionCandidate(time.Now())
			require.True(t, validated, "commit must reach the real validator with its staged candidate")
			active, err := storage.ReadActiveConfig(store, name)
			require.NoError(t, err)
			require.Equal(t, []byte(base), active, "candidate commit must not change active configuration")
			mirror, err := store.ReadFile(name)
			require.NoError(t, err)
			require.Equal(t, []byte(base), mirror, "candidate commit must not overwrite the active file")
			published, _, present, err := storage.ReadCandidateConfig(store, name)
			require.NoError(t, err)
			if question.refusal != "" {
				require.ErrorContains(t, commitError, question.refusal)
				require.Nil(t, result)
				require.Empty(t, output)
				require.False(t, present, "a refused candidate must not be published")
				require.True(t, ed.Dirty(), "refusal must retain the user's pending correction")
				return
			}
			require.NoError(t, commitError)
			require.NotNil(t, result)
			require.Empty(t, result.Conflicts)
			require.Positive(t, result.Applied, "the complete control must actually stage its changed router-id")
			require.True(t, present)
			require.Equal(t, output, string(published))
			require.NoError(t, ValidateContent(string(published), name), "the published complete candidate must retain its valid grants")
		})
	}
}
