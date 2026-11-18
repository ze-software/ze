// Design: docs/architecture/core-design.md -- the YANG/handler contract gate.
// Related: contract.go -- the gate these registrations are read by.
//
// The product dispatch root links ExaBGP's CLI owner separately from the bridge
// plugin. Import the same owner so its supplied migration handlers and schema
// participate in the gate, while builds without ze_exabgp retain neither.

//go:build ze_exabgp

package docyangcontract

import (
	_ "github.com/ze-software/ze/internal/plugins/exabgp"
)
