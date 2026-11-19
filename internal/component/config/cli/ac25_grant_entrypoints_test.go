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
	configuration := func(routerID, sourceProcess, sourceReceive, destinationReceive string) string {
		return fmt.Sprintf(`plugin {
 internal ac25-table { use bgp-rib; }
 internal ac25-other { use bgp-rib; }
}
bgp {
 router-id %s;
 session { asn { local 65000; } }
 peer source {
  connection { local { ip 192.0.2.254; } remote { ip 192.0.2.1; } }
  session { asn { remote 65001; } }
  filter { import { final accept; } export { final reject; } }
  attach process %s { receive [ %s ]; }
 }
 peer destination {
  connection { local { ip 192.0.2.254; } remote { ip 192.0.2.2; } }
  session { asn { remote 65002; } }
  filter { import { final reject; } export { final accept; } }
  attach process ac25-table { send [ update ]; %s }
 }
}
`, routerID, sourceProcess, sourceReceive, destinationReceive)
	}
	const sourceGrants = "update-received state"
	const destinationGrants = "receive [ state refresh update-sent ];"
	base := configuration("192.0.2.254", "ac25-table", sourceGrants, destinationGrants)
	for _, question := range []struct {
		name               string
		sourceProcess      string
		sourceReceive      string
		destinationReceive string
		refusal            string
	}{
		{
			name: "source state only", sourceProcess: "ac25-table", sourceReceive: "state", destinationReceive: destinationGrants,
			refusal: "peer \"source\" process \"ac25-table\" has incomplete source grants; complete with `attach process ac25-table { receive [ update-received state ]; }`",
		},
		{
			name: "destination send only", sourceProcess: "ac25-table", sourceReceive: sourceGrants,
			refusal: "peer \"destination\" process \"ac25-table\" has incomplete destination grants; complete with `attach process ac25-table { receive [ state refresh update-sent ]; send [ update ]; }`",
		},
		{
			name: "split instances", sourceProcess: "ac25-other", sourceReceive: sourceGrants, destinationReceive: destinationGrants,
			refusal: "peer \"destination\" destination process \"ac25-table\" has no complete source on this same instance; attach a source peer with `attach process ac25-table { receive [ update-received state ]; }`",
		},
		{
			name: "complete instance", sourceProcess: "ac25-table", sourceReceive: sourceGrants, destinationReceive: destinationGrants,
		},
	} {
		t.Run(question.name, func(t *testing.T) {
			candidate := configuration("192.0.2.253", question.sourceProcess, question.sourceReceive, question.destinationReceive)
			file := filepath.Join(t.TempDir(), "ac25.conf")
			require.NoError(t, os.WriteFile(file, []byte(candidate), 0o600))
			validationError := ValidateContent(candidate, file)
			wantCode := exitOK
			if question.refusal != "" {
				wantCode = exitInvalid
				require.ErrorContains(t, validationError, question.refusal)
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
