// Design: docs/guide/bgp-role.md -- peers that declare no role
// Overview: roleless.go -- rolelessPeers, the one declaration of the set this check reports
// Related: register.go -- the DoctorChecks entry that installs checkRolelessPeers
//
// The check reads the config the doctor runner loaded through the same
// sections the engine delivers to this plugin (config.BuildPluginConfigSections
// over the pruned tree), parsed by the same rolelessPeers the config-load
// warning reads, so `ze doctor` and the warning name one set of peers.

package role

import (
	"github.com/ze-software/ze/internal/component/bgp/configjson"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// doctorPeerNoRoleCode names a peer or a dynamic group that declares no
// RFC 9234 role (internal/core/diagnostic/codes.go).
const doctorPeerNoRoleCode = "doctor-bgp-peer-no-role"

// checkRolelessPeers enumerates the peers and the dynamic groups that declare
// no RFC 9234 role. Such a peer is accepted, but the checks a role inserts (the
// RFC 9234 Section 5 OTC procedures, the RFC 7454 Section 9 upstream check) run
// only on a peer that declares a relationship, so the operator is told which
// sessions no relationship was stated for, and the gap is a decision they can
// see rather than a silence.
//
// A WARNING, not an error: the config loads and the daemon starts on it. The
// plugin names the same peers in one aggregated line when it installs the
// config; this check is the door an operator opens on purpose.
//
// A config the check cannot read is reported as unknown, never as a config
// with no roleless peer.
func checkRolelessPeers(ctx registry.DoctorCheckContext) []rpc.DoctorCheckDiagnostic {
	tree, ok := ctx.Tree.(*config.Tree)
	if !ok || tree == nil {
		return nil
	}
	if tree.GetContainer(configRootBGP) == nil {
		return nil
	}
	names, err := rolelessPeersFromTree(tree)
	if err != nil {
		return []rpc.DoctorCheckDiagnostic{rolelessDiagnostic(textbuf.New().
			Str("bgp-role: which peers declare no RFC 9234 role is unknown, because the bgp configuration could not be read: ").
			Err(err).Str(". Check the configuration with `ze config validate <config-file>`, then rerun `ze doctor`.").String())}
	}
	if len(names) == 0 {
		return nil
	}
	return []rpc.DoctorCheckDiagnostic{rolelessDiagnostic(textbuf.New().
		Str("peers and dynamic groups declare no RFC 9234 role, so no check a role inserts runs on them: ").
		Join(names, ", ").
		Str(". Add `role { import <role> }` to each peer or group that should carry one, or leave it out deliberately.").String())}
}

// rolelessPeersFromTree names the roleless peers of the config tree, read from
// the bgp section the engine delivers this plugin.
//
// It prunes a CLONE: PruneInactive modifies in place, and the doctor runner
// shares one tree across every later check.
func rolelessPeersFromTree(tree *config.Tree) ([]string, error) {
	schema, err := config.YANGSchema()
	if err != nil {
		return nil, err
	}
	clone := tree.Clone()
	config.PruneInactive(clone, schema)

	sections, err := config.BuildPluginConfigSections(clone.ToPluginMap(), []string{configRootBGP})
	if err != nil {
		return nil, err
	}
	for _, section := range sections {
		if section.Root != configRootBGP {
			continue
		}
		bgpSubtree, ok := configjson.ParseBGPSubtree(section.Data)
		if !ok {
			return nil, errRoleInvalidBGPConfigJSON
		}
		return rolelessPeers(bgpSubtree)
	}
	// The bgp container is present (checkRolelessPeers) and prunes to nothing
	// only when every node under it is inactive: no peer runs, none is named.
	return nil, nil
}

// rolelessDiagnostic is a warning under doctorPeerNoRoleCode.
func rolelessDiagnostic(message string) rpc.DoctorCheckDiagnostic {
	return rpc.DoctorCheckDiagnostic{
		Code:     doctorPeerNoRoleCode,
		Severity: string(diagnostic.SeverityWarning),
		Message:  message,
	}
}
