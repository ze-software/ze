//go:build ze_bgp

// Design: docs/guide/bgp-policy.md -- learned-export absence at config validate.
package cli

import (
	"fmt"
	"net"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// Run the actual validate entry point with an alias's complete source and
// destination grants. Configured reject remains valid and generates no absence.
// MUTATION: config's carrier ignores the shared learned-export absence.
func TestValidateReportsLearnedExportAbsenceAndConfiguredSurvivor(t *testing.T) {
	for _, final := range []string{"", "accept", "reject"} {
		t.Run("final-"+final, func(t *testing.T) {
			require.NoError(t, events.RegisterNamespace("bgp", "update", "state", "refresh"))
			snapshot := registry.Snapshot()
			t.Cleanup(func() { registry.Restore(snapshot) })
			require.NoError(t, registry.Register(registry.Registration{
				Name: "validate-ordinary-owner", RunEngine: func(net.Conn) int { return 0 }, CLIHandler: func([]string) int { return 0 },
				Dissemination: []rpc.DisseminationDecl{{Namespace: "bgp", Role: "bgp-selected-output",
					SourceReceive: []string{"update-received", "state"}, DestinationReceive: []string{"state", "refresh", "update-sent"}, DestinationSend: []string{"update"}}},
			}))
			export := ""
			if final != "" {
				export = "filter { export { final " + final + "; } }"
			}
			input := fmt.Sprintf(`bgp {
    router-id 192.0.2.254; session { asn { local 65000; } }
    peer source {
     connection { remote { ip 192.0.2.1; } local { ip 192.0.2.254; } }
     session { asn { remote 65001; } }
     filter { import { final accept; } export { final accept; } }
     attach process validate-ordinary-owner { receive [ update state ]; }
    }
    peer C {
     connection { remote { ip 192.0.2.2; } local { ip 192.0.2.254; } }
     session { asn { remote 65000; } } %s
     attach process validate-ordinary-owner { receive [ state refresh update-sent ]; send [ update ]; }
    }
   }`, export)
			result := runValidation(input, "learned-export.conf")
			require.True(t, result.Valid, "%+v", result.Diagnostics)
			found := 0
			for _, finding := range result.Diagnostics {
				if finding.Code != "doctor-bgp-learned-export-no-policy" {
					continue
				}
				found++
				require.Contains(t, finding.Message, "peer C")
				require.Contains(t, finding.Message, "no routes learned from other peers")
				require.Equal(t, "set bgp peer C filter export final accept", finding.Help)
				require.Equal(t, finding.Help, finding.Fix)
				require.NotContains(t, finding.Message, "RFC 8212")
			}
			want := 0
			if final == "" {
				want = 1
			}
			require.Equal(t, want, found)
		})
	}
}
