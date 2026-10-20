// Design: docs/contributing/rfc-conformance-gates.md -- discrimination routes for interop carriers
// Related: carriers.go -- interopTrees and interopSuiteOf read the catalogs these labs register
//
// Every interop lab declares its own carrier facts in its catalog: the package
// its checkers live in, its scenario directory, its selector variable and the
// action that runs it. This package reads them from interoplab's registry, and
// a registry answers only for the labs a binary links, so the labs are linked
// here: the rfc test binary links nothing else that would. The list is the one
// copy this package keeps, and TestEveryInteropLabRegistersItsCatalogHere
// compares it to every lab package that holds a register_catalog.go.

package rfc

import (
	_ "github.com/ze-software/ze/internal/le/interoplab/bgp"
	_ "github.com/ze-software/ze/internal/le/interoplab/ipsec"
	_ "github.com/ze-software/ze/internal/le/interoplab/l2tp"
	_ "github.com/ze-software/ze/internal/le/interoplab/pppoe"
	_ "github.com/ze-software/ze/internal/le/interoplab/radius"
	_ "github.com/ze-software/ze/internal/le/interoplab/radiussubscriber"
	_ "github.com/ze-software/ze/internal/le/interoplab/rsvpte"
)
