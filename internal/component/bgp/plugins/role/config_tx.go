// Design: docs/architecture/config/transaction-protocol.md -- a plugin's verify, apply and rollback of one parsed config
// RFC: rfc/short/rfc9234.md -- the per-peer role config the Section 5 filters read
// Related: config.go -- extractPeerRoleConfigs, the parse of the role leaves
// Related: role.go -- the filter state this installs, and the learned-role prune

package role

import (
	"errors"

	"github.com/ze-software/ze/internal/component/bgp/configjson"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// errRoleInvalidBGPConfigJSON refuses a bgp section the role plugin cannot
// parse. Reading it as "no peer has a role" would drop every peer's export
// rules while the commit reports success.
var errRoleInvalidBGPConfigJSON = errors.New("bgp-role: invalid bgp config JSON")

// roleConfig is every leaf the role plugin reads beyond its capability: each
// peer's or group's role config (strict, export, and the role the OTC filters
// gate by) and the peer name-to-address map. It is one value, so the Section 5
// filters and OnValidateOpen always read one snapshot of it.
//
// The role value itself also reaches the OPEN, and a change to it is refused
// at verify by the capability comparison (sdk.Plugin.DeclareCapabilitiesFrom),
// so a reload that this transaction applies never changes a role in an OPEN.
type roleConfig struct {
	peers    map[string]*peerRoleConfig
	nameToIP map[string]string
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
	for _, section := range sections {
		if section.Root != configRootBGP {
			continue
		}
		if _, ok := configjson.ParseBGPSubtree(section.Data); !ok {
			return roleConfig{}, false, errRoleInvalidBGPConfigJSON
		}
		peers, nameToIP := extractPeerRoleConfigs(section.Data)
		return roleConfig{peers: peers, nameToIP: nameToIP}, true, nil
	}
	return roleConfig{}, false, nil
}

// installRoleConfig makes cfg the config the filters and OnValidateOpen
// read. It prunes nothing (roleConfigTx).
func installRoleConfig(cfg roleConfig) error {
	filterMu.Lock()
	filterPeerConfigs = cfg.peers
	filterNameToIP = cfg.nameToIP
	filterMu.Unlock()
	return nil
}

// installedRoleConfig returns the snapshot installRoleConfig last installed.
// The maps are replaced whole and never written after, so the caller reads
// them without the lock.
func installedRoleConfig() roleConfig {
	filterMu.RLock()
	defer filterMu.RUnlock()
	return roleConfig{peers: filterPeerConfigs, nameToIP: filterNameToIP}
}
