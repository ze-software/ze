// Design: docs/architecture/config/transaction-protocol.md -- a plugin's verify, apply and rollback of one parsed config
// Related: internal.go -- runInternalBridge, which registers these handlers

package bridgeplugin

import (
	"errors"
	"reflect"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/plugins/exabgp/bridgerun"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// registerReloadHandlers binds the bridge's reload verify, apply and
// rollback. The bridge has no live reconfigure path: its scripts run under the
// config they started with (owner decision Q4 = e1). So verify refuses every
// change under exabgp bridge other than a capability leaf, which the composed
// capability check (DeclareCapabilitiesFrom) refuses in its own words, and an
// accepted candidate is one the running bridge already serves: apply and
// rollback have nothing to install.
func (r *bridgeRunner) registerReloadHandlers(p *sdk.Plugin) {
	p.OnConfigVerify(r.verifyReload)
	p.OnConfigApply(func(_ []sdk.ConfigDiffSection) error { return nil })
	p.OnConfigRollback(func(_ string) error { return nil })
}

// verifyReload refuses a candidate whose bridge differs from the running one
// outside the capability leaves, naming each changed leaf path. A delivery
// that carries no exabgp section leaves the bridge unchanged.
func (r *bridgeRunner) verifyReload(sections []sdk.ConfigSection) error {
	for _, s := range sections {
		if s.Root != configRoot {
			continue
		}
		candidate, err := parseConfig(s.Data)
		if err != nil {
			return err
		}
		r.mu.Lock()
		running := r.config
		r.mu.Unlock()
		if refusal := refusePresenceChange(running, candidate); refusal != nil {
			return refusal
		}
		return refuseRunningChange(changedBridgeLeaves(running, candidate))
	}
	return nil
}

// refusePresenceChange builds the verify refusal for a reload that adds or
// removes the whole bridge, or answers nil when neither. Its reason differs
// from refuseRunningChange's: a bridge absent at boot started no script, and
// a removed bridge's scripts keep running until ze restarts.
func refusePresenceChange(running, candidate bridgeConfig) error {
	if running.Present == candidate.Present {
		return nil
	}
	if candidate.Present {
		return errors.New("exabgp bridge cannot be added while ze runs (plugin exabgp-bridge): " +
			"the bridge starts its scripts only when ze starts, so none would run.\n" + sdk.RestartSteps())
	}
	return errors.New("exabgp bridge cannot be removed while ze runs (plugin exabgp-bridge): " +
		"its scripts keep running until ze restarts.\n" + sdk.RestartSteps())
}

// changedBridgeLeaves answers the config path of every non-capability leaf
// that differs between running and candidate, in config order.
func changedBridgeLeaves(running, candidate bridgeConfig) []string {
	var changed []string
	for _, name := range scriptNames(running.Scripts, candidate.Scripts) {
		if !reflect.DeepEqual(findScript(running.Scripts, name), findScript(candidate.Scripts, name)) {
			changed = append(changed, "exabgp bridge process "+name)
		}
	}
	if !slices.Equal(running.Families, candidate.Families) {
		changed = append(changed, "exabgp bridge family")
	}
	return changed
}

// scriptNames answers every script name of a and b, sorted, once each.
func scriptNames(a, b []bridgerun.Script) []string {
	names := make([]string, 0, len(a)+len(b))
	for i := range a {
		names = append(names, a[i].Name)
	}
	for i := range b {
		names = append(names, b[i].Name)
	}
	slices.Sort(names)
	return slices.Compact(names)
}

// findScript answers the script called name, or nil when scripts has none.
func findScript(scripts []bridgerun.Script, name string) *bridgerun.Script {
	for i := range scripts {
		if scripts[i].Name == name {
			return &scripts[i]
		}
	}
	return nil
}

// refuseRunningChange builds the verify refusal for the changed leaf paths, or
// answers nil when none changed. Each line names one leaf, what is wrong and
// why; the restart steps follow, because a restart is the only way to apply
// the change.
func refuseRunningChange(changed []string) error {
	if len(changed) == 0 {
		return nil
	}
	var b strings.Builder
	for _, leaf := range changed {
		b.WriteString(leaf)
		b.WriteString(" cannot change while ze runs (plugin exabgp-bridge): the bridge's scripts keep the config they started with.\n")
	}
	b.WriteString(sdk.RestartSteps())
	return errors.New(b.String())
}
