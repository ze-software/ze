// Design: docs/architecture/config/transaction-protocol.md -- a plugin's verify, apply and rollback of one parsed config
// RFC: rfc/short/rfc9234.md -- the per-peer role config the Section 5 filters read
// Related: config.go -- extractPeerRoleConfigs, the parse of the role leaves
// Related: role.go -- the filter state this installs, and the learned-role prune

package role

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ze-software/ze/internal/component/bgp/configjson"
	"github.com/ze-software/ze/internal/component/policy"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// errRoleInvalidBGPConfigJSON refuses a bgp section the role plugin cannot
// parse. Reading it as "no peer has a role" would drop every peer's export
// rules while the commit reports success.
var errRoleInvalidBGPConfigJSON = errors.New("bgp-role: the bgp config section it was handed is not valid JSON, " +
	"so no peer's role, strict mode or export set can be read and the commit is refused with the running role config kept; " +
	"fix: check the candidate with `ze config validate <file>`, and report the config if it validates")

// roleConfig is every leaf the role plugin reads beyond its capability: each
// peer's or group's role config (strict, export, and the role the OTC filters
// gate by), the peer name-to-address map, and the key of every configured peer
// and group, with or without a role block. It is one value, so the Section 5
// filters and OnValidateOpen always read one snapshot of it.
//
// The role value itself also reaches the OPEN, and a change to it is refused
// at verify by the capability comparison (sdk.Plugin.DeclareCapabilitiesFrom),
// so a reload that this transaction applies never changes a role in an OPEN.
type roleConfig struct {
	peers    map[string]*peerRoleConfig
	nameToIP map[string]string
	// peerKeys is every key a learned role can be stored under for a peer the
	// config names (configuredPeerKeys). pruneLearnedRoles diffs it against
	// the set of the last prune, so only a commit that removes a peer drops
	// its learned role.
	peerKeys map[string]struct{}
	// upstream is the RFC 7454 Section 9 upstream set (deriveUpstreamSet).
	upstream upstreamSet
	// roleless names the peers and dynamic groups that declare no role
	// (rolelessPeers), for the one warning installRoleConfig logs. No filter
	// reads it.
	roleless []string
}

// roleConfigTx is the role plugin's config transaction. It differs from a
// plain sdk.ConfigTx in one step: the prune of roles learned from OPENs for
// peers the config no longer names. That state is not part of any config
// snapshot, so a rollback could not restore it; the prune therefore runs only
// at Stage 2, where no transaction follows, and at config-committed.
type roleConfigTx struct {
	tx *sdk.ConfigTx[roleConfig]
}

// newRoleConfigTx returns the transaction over the package filter state.
func newRoleConfigTx() *roleConfigTx {
	return &roleConfigTx{tx: sdk.NewConfigTx(roleConfig{}, parseRoleConfig, installRoleConfig)}
}

// register binds the transaction to the plugin's Stage 2 configure and its
// reload verify, apply, rollback and committed callbacks. It MUST be called
// before Run.
func (t *roleConfigTx) register(p *sdk.Plugin) {
	p.OnConfigure(t.configure)
	p.OnConfigVerify(t.tx.Verify)
	p.OnConfigApply(func(_ []sdk.ConfigDiffSection) error { return t.tx.Apply() })
	p.OnConfigRollback(func(_ string) error { return t.tx.Rollback() })
	p.OnConfigCommitted(t.committed)
}

// configure is the Stage 2 delivery: install, then prune at once, because no
// rollback can follow a boot delivery.
func (t *roleConfigTx) configure(sections []sdk.ConfigSection) error {
	if err := t.tx.Configure(sections); err != nil {
		return err
	}
	pruneLearnedRoles()
	return nil
}

// committed prunes the learned roles once the reload that may have removed
// their peers is permanent.
func (t *roleConfigTx) committed(_ string) error {
	pruneLearnedRoles()
	return nil
}

// parseRoleConfig reads the role config from the bgp section of one delivery.
// It changes no state, so a refused verify and a CLI validate leave the
// running filters as they were.
func parseRoleConfig(sections []sdk.ConfigSection) (roleConfig, bool, error) {
	policyRoot, delivered, err := policySection(sections)
	if err != nil {
		return roleConfig{}, false, err
	}
	if !delivered {
		// No policy section: the policy root states no asn-set, so a
		// transit-asns name finds nothing and transitSetASNs refuses it.
		policyRoot = map[string]any{}
	}
	for _, section := range sections {
		if section.Root != configRootBGP {
			continue
		}
		bgpSubtree, ok := configjson.ParseBGPSubtree(section.Data)
		if !ok {
			return roleConfig{}, false, errRoleInvalidBGPConfigJSON
		}
		upstream, err := deriveUpstreamSet(bgpSubtree, policyRoot)
		if err != nil {
			return roleConfig{}, false, err
		}
		roleless, err := rolelessPeers(bgpSubtree)
		if err != nil {
			return roleConfig{}, false, err
		}
		peers, nameToIP := extractPeerRoleConfigs(section.Data)
		return roleConfig{peers: peers, nameToIP: nameToIP, peerKeys: configuredPeerKeys(bgpSubtree),
			upstream: upstream, roleless: roleless}, true, nil
	}
	return roleConfig{}, false, nil
}

// policySection answers the policy root's body, the asn-sets transit-asns
// names; delivered is false when no policy section was delivered, so the
// caller decides what absence means rather than reading a nil map. The policy
// root is a ConfigReads root, delivered whole beside bgp whenever either
// changes (docs/architecture/config/transaction-protocol.md).
func policySection(sections []sdk.ConfigSection) (body map[string]any, delivered bool, err error) {
	for _, section := range sections {
		if section.Root != policy.RootName {
			continue
		}
		var root map[string]any
		if err := json.Unmarshal([]byte(section.Data), &root); err != nil {
			return nil, false, fmt.Errorf("bgp-role: the policy config section is not valid JSON: %w; "+
				"fix: check the candidate with `ze config validate <file>`", err)
		}
		// The section carries the root keyed by its name, as the bgp one does.
		if inner, wrapped := root[policy.RootName].(map[string]any); wrapped {
			return inner, true, nil
		}
		return root, true, nil
	}
	return nil, false, nil
}

// installRoleConfig makes cfg the config the filters and OnValidateOpen
// read, and says in one line which peers declare no role, so none of this
// plugin's checks runs on them. It prunes nothing (roleConfigTx).
func installRoleConfig(cfg roleConfig) error {
	warnPeersWithoutRole(logger(), cfg.roleless)
	filterMu.Lock()
	filterPeerConfigs = cfg.peers
	filterNameToIP = cfg.nameToIP
	filterPeerKeys = cfg.peerKeys
	filterMu.Unlock()
	// The upstream set swaps whole: a reload that adds or removes a provider
	// peer, or edits an asn-set, changes what the next route compares, with
	// no chain rebuilt and no session reset.
	installUpstreamSet(cfg.upstream)
	return nil
}

// installedRoleConfig returns the snapshot installRoleConfig last installed.
// The maps are replaced whole and never written after, so the caller reads
// them without the lock.
func installedRoleConfig() roleConfig {
	filterMu.RLock()
	defer filterMu.RUnlock()
	cfg := roleConfig{peers: filterPeerConfigs, nameToIP: filterNameToIP, peerKeys: filterPeerKeys}
	if set := upstreamASNs.Load(); set != nil {
		cfg.upstream = *set
	}
	return cfg
}

// verifyRoleConfig is the plugin's in-process config verifier, which `ze
// config validate` and the editor run without starting the plugin. It parses
// with the same parseRoleConfig the transaction installs from, so what it
// refuses is what Stage 2 refuses, and it logs the roleless warning the
// install logs: an operator validating a config learns which peers no role
// check covers before the daemon runs it.
func verifyRoleConfig(sections []sdk.ConfigSection) error {
	cfg, _, err := parseRoleConfig(sections)
	if err != nil {
		return err
	}
	warnPeersWithoutRole(verifyLogger(), cfg.roleless)
	return nil
}
