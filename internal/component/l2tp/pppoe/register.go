// Design: docs/architecture/l2tp/bng-5-pppoe.md -- component registration

package pppoe

import (
	"github.com/ze-software/ze/internal/component/config"
	_ "github.com/ze-software/ze/internal/component/l2tp/pppoe/yang"
)

func init() {
	if err := config.RegisterTreeValidator("pppoe-authentication-policy", validateAuthPolicy); err != nil {
		panic("BUG: register PPPoE authentication policy: " + err.Error())
	}
}

// validateAuthPolicy runs the same parser as startup and reload against the
// original tree, before defaults can conceal a conflicting explicit policy.
func validateAuthPolicy(tree *config.Tree) error {
	_, err := ExtractParameters(tree.ToMap())
	return err
}
