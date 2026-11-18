// Design: docs/architecture/doctor-and-health-checks.md -- saved BGP policy absence.
package bgpconfig

import (
	"fmt"
	"net"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/reactor"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// Each test owns an independent configuration and registration; the actual
// resolver, not an implementation name, grants its ordinary or explicit role.
func policyAbsenceDoctorTree(t *testing.T, remote, final string, explicit bool) *config.Tree {
	t.Helper()
	require.NoError(t, events.RegisterNamespace("bgp", "update", "state", "refresh"))
	snapshot := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snapshot) })
	claims := []string(nil)
	if explicit {
		claims = []string{reactor.SelectedDisseminationRole}
	}
	require.NoError(t, registry.Register(registry.Registration{
		Name: "absence-owner", RunEngine: func(net.Conn) int { return 0 }, CLIHandler: func([]string) int { return 0 }, Claims: claims,
		Dissemination: []rpc.DisseminationDecl{{Namespace: "bgp", Role: reactor.SelectedDisseminationRole,
			SourceReceive: []string{"update-received", "state"}, DestinationReceive: []string{"state", "refresh", "update-sent"}, DestinationSend: []string{"update"}}},
	}))
	filter := "filter { import { final accept; } }"
	if final != "" {
		filter = "filter { import { final accept; } export { final " + final + "; } }"
	}
	tree, err := config.ParseTreeForValidation(fmt.Sprintf(`bgp {
  router-id 192.0.2.254; session { asn { local 65000; } }
  peer source {
   connection { remote { ip 192.0.2.1; } local { ip 192.0.2.254; } }
   session { asn { remote 65001; } }
   filter { import { final accept; } export { final accept; } }
   attach process absence-owner { receive [ update state ]; }
  }
  peer C {
   connection { remote { ip 192.0.2.2; } local { ip 192.0.2.254; } }
   session { asn { remote %s; } } %s
   attach process absence-owner { receive [ state refresh update-sent ]; send [ update ]; }
  }
 }`, remote, filter))
	require.NoError(t, err)
	return tree
}

// Doctor reads the supplied saved configuration, so another tree with policy
// must clear the result instead of retaining the first tree or a log history.
// MUTATION: the policy-absence check returns no finding.
func TestPolicyAbsenceDoctorNamesConfiguredPeerAndClears(t *testing.T) {
	for _, final := range []string{"", "accept", "reject"} {
		t.Run("final-"+final, func(t *testing.T) {
			tree := policyAbsenceDoctorTree(t, "65000", final, false)
			before := tree.ToRunningMap()
			findings := doctorCheckBGPPolicyAbsences(diagnosticContext(tree))
			require.Equal(t, before, tree.ToRunningMap(), "the shared saved tree remains intact")
			if final != "" {
				require.Empty(t, findings)
				return
			}
			require.Len(t, findings, 1)
			require.Equal(t, diagnostic.CodeDoctorBGPLearnedExportNoPolicy, findings[0].Code)
			require.Contains(t, findings[0].Message, "peer C")
			require.Contains(t, findings[0].Message, "no routes learned from other peers")
			require.Equal(t, "set bgp peer C filter export final accept", findings[0].Fix)
			require.Equal(t, diagnostic.SeverityWarning, findings[0].Severity)
			require.NotContains(t, findings[0].Message, "RFC 8212")
			next := tree.Clone()
			export := config.NewTree()
			export.Set("final", "reject")
			next.GetContainer("bgp").GetList("peer")["C"].GetContainer("filter").SetContainer("export", export)
			require.Empty(t, doctorCheckBGPPolicyAbsences(diagnosticContext(next)), "the latest supplied explicit policy clears the saved-config finding")
			require.Len(t, doctorCheckBGPPolicyAbsences(diagnosticContext(tree)), 1, "the unchanged original config remains diagnosed")
		})
	}
}

func TestPolicyAbsenceDoctorKeepsRoleAndExternalDefaults(t *testing.T) {
	for _, remote := range []string{"65000", "65001"} {
		t.Run(remote, func(t *testing.T) {
			findings := doctorCheckBGPPolicyAbsences(diagnosticContext(policyAbsenceDoctorTree(t, remote, "", true)))
			if remote == "65000" {
				require.Empty(t, findings)
				return
			}
			require.Len(t, findings, 1)
			require.Equal(t, diagnostic.CodeDoctorBGPRFC8212NoPolicy, findings[0].Code)
			require.Contains(t, findings[0].Message, "RFC 8212")
			require.Contains(t, findings[0].Message, "no export policy")
		})
	}
}

func TestPolicyAbsenceDoctorRegistrationMatchesItsCodes(t *testing.T) {
	var found bool
	for _, check := range diagnostic.DoctorChecksForPhase(diagnostic.DoctorPhasePostConfig) {
		if check.Name != "bgp-policy-absence" {
			continue
		}
		found = true
		require.ElementsMatch(t, []string{diagnostic.CodeDoctorBGPLearnedExportNoPolicy, diagnostic.CodeDoctorBGPRFC8212NoPolicy}, check.Codes)
		require.Equal(t, []string{doctorDependencyConfigTree}, check.Dependencies)
	}
	require.True(t, found, "the ordinary ze doctor registry must reach the check")
}
