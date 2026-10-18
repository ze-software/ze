// Design: docs/guide/ipsec.md -- IPsec configuration
// Related: unmatched_evolution.go -- the evolution registered here

package ipsec

import (
	"github.com/ze-software/ze/internal/component/config/migration"

	_ "github.com/ze-software/ze/internal/component/ike/ipsec/yang"
)

func init() {
	if err := migration.RegisterEvolution(unmatchedBypassEvolution()); err != nil {
		panic("BUG: ipsec: register the unmatched evolution: " + err.Error())
	}
}
