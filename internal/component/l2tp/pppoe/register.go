// Design: docs/architecture/l2tp/bng-5-pppoe.md -- component registration

package pppoe

import (
	"github.com/ze-software/ze/internal/component/config"
	_ "github.com/ze-software/ze/internal/component/l2tp/pppoe/yang"
)

func init() {
	if err := config.RegisterServiceBlock(SubsystemName, "PPPoE subsystem"); err != nil {
		panic("BUG: pppoe: service block registration: " + err.Error())
	}
	if err := config.RegisterScopedTreeValidator("pppoe-backend", SubsystemName, validateBackendGate); err != nil {
		panic("BUG: register PPPoE backend validation: " + err.Error())
	}
	if err := config.RegisterScopedTreeValidator("pppoe-authentication-policy", SubsystemName, validateAuthPolicy); err != nil {
		panic("BUG: register PPPoE authentication policy: " + err.Error())
	}
}

// validateAuthPolicy runs the same parser as startup and reload against the
// original tree, before defaults can conceal a conflicting explicit policy.
func validateAuthPolicy(tree *config.Tree) error {
	_, err := ExtractParameters(tree.ToMap())
	return err
}
