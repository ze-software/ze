// Design: docs/architecture/core-design.md -- le's composition root
//
// One blank import per tool, and that import is the whole wiring. A tool
// removed from this list vanishes from le: from dispatch, from help and from
// completion, with no other file edited.
//
// Every import here names an letools package, and no ze composition root ever
// names one back. That rule is DIRECTIONAL: a tool package under letools/ may
// link the product to enumerate what it registers, which is what a command
// inventory or a CLI-grammar check is for, but le must never OWN a product
// command and ze must never link a dev tool. cmd/le/separation_test.go checks
// both halves rather than trusting this comment.

package main

import (
	_ "github.com/ze-software/ze/letools/commandlist"
	_ "github.com/ze-software/ze/letools/commandownership"
	_ "github.com/ze-software/ze/letools/configclaims"
	_ "github.com/ze-software/ze/letools/configcoercion"
	_ "github.com/ze-software/ze/letools/consistency"
	_ "github.com/ze-software/ze/letools/docvalid"
	_ "github.com/ze-software/ze/letools/featuretags"
	_ "github.com/ze-software/ze/letools/ianaasn"
	_ "github.com/ze-software/ze/letools/ifaceresolution"
	_ "github.com/ze-software/ze/letools/inventory"
	_ "github.com/ze-software/ze/letools/parity"
	_ "github.com/ze-software/ze/letools/pluginimports"
	_ "github.com/ze-software/ze/letools/portdefaults"
	_ "github.com/ze-software/ze/letools/specstatus"
	_ "github.com/ze-software/ze/letools/vendorweb"
	_ "github.com/ze-software/ze/letools/webassets"
	_ "github.com/ze-software/ze/letools/weekly"
	_ "github.com/ze-software/ze/letools/yangglue"
)
