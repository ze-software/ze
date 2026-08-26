// Design: docs/architecture/core-design.md -- le's composition root
//
// One blank import per tool, and that import is the whole wiring. A tool
// removed from this list vanishes from le: from dispatch, from help and from
// completion.
//
// Only one other file holds the same list. letools/zele/tools.go is the tool set
// that a ze binary links when compiled with the ze_le tag. A tool added here
// MUST also be added there. TestTheCrossingLinksEveryToolLeCarries refuses
// mismatched lists and names the import to add.
//
// Every import here names an letools package, and no ze composition root ever
// names one back. That rule is DIRECTIONAL: a tool package under letools/ may
// link the product to enumerate what it registers, which is what a command
// inventory or a CLI-grammar check is for, but le must never OWN a product
// command and ze must never link a dev tool. cmd/le/separation_test.go checks
// both halves rather than trusting this comment.

package main

import (
	_ "github.com/ze-software/ze/letools/aisync"
	_ "github.com/ze-software/ze/letools/archmap"
	_ "github.com/ze-software/ze/letools/changed"
	_ "github.com/ze-software/ze/letools/cidispatch"
	_ "github.com/ze-software/ze/letools/cligrammar"
	_ "github.com/ze-software/ze/letools/commandlist"
	_ "github.com/ze-software/ze/letools/commandownership"
	_ "github.com/ze-software/ze/letools/configclaims"
	_ "github.com/ze-software/ze/letools/configcoercion"
	_ "github.com/ze-software/ze/letools/consistency"
	_ "github.com/ze-software/ze/letools/dashstdio"
	_ "github.com/ze-software/ze/letools/deployment"
	_ "github.com/ze-software/ze/letools/devsetup"
	_ "github.com/ze-software/ze/letools/digest"
	_ "github.com/ze-software/ze/letools/discoveryindex"
	_ "github.com/ze-software/ze/letools/docstocode"
	_ "github.com/ze-software/ze/letools/docvalid"
	_ "github.com/ze-software/ze/letools/docwiring"
	_ "github.com/ze-software/ze/letools/evidence"
	_ "github.com/ze-software/ze/letools/featuretags"
	_ "github.com/ze-software/ze/letools/fspersistence"
	_ "github.com/ze-software/ze/letools/functional"
	_ "github.com/ze-software/ze/letools/fuzz"
	_ "github.com/ze-software/ze/letools/goextract"
	_ "github.com/ze-software/ze/letools/gokrazygosum"
	_ "github.com/ze-software/ze/letools/ianaasn"
	_ "github.com/ze-software/ze/letools/ifaceresolution"
	_ "github.com/ze-software/ze/letools/integration"
	_ "github.com/ze-software/ze/letools/inventory"
	_ "github.com/ze-software/ze/letools/lejob"
	_ "github.com/ze-software/ze/letools/letracked"
	_ "github.com/ze-software/ze/letools/parity"
	_ "github.com/ze-software/ze/letools/perfbench"
	_ "github.com/ze-software/ze/letools/pluginboundary"
	_ "github.com/ze-software/ze/letools/pluginimports"
	_ "github.com/ze-software/ze/letools/portdefaults"
	_ "github.com/ze-software/ze/letools/protocolskeleton"
	_ "github.com/ze-software/ze/letools/pylint"
	_ "github.com/ze-software/ze/letools/qemu"
	_ "github.com/ze-software/ze/letools/repository"
	_ "github.com/ze-software/ze/letools/rfc"
	_ "github.com/ze-software/ze/letools/rules"
	_ "github.com/ze-software/ze/letools/sitefacts"
	_ "github.com/ze-software/ze/letools/specstatus"
	_ "github.com/ze-software/ze/letools/staticcheckmatrix"
	_ "github.com/ze-software/ze/letools/ste"
	_ "github.com/ze-software/ze/letools/testhealth"
	_ "github.com/ze-software/ze/letools/testsensitivity"
	_ "github.com/ze-software/ze/letools/tier"
	_ "github.com/ze-software/ze/letools/tokeneconomy"
	_ "github.com/ze-software/ze/letools/trackedbuild"
	_ "github.com/ze-software/ze/letools/vendorweb"
	_ "github.com/ze-software/ze/letools/webassets"
	_ "github.com/ze-software/ze/letools/weekly"
	_ "github.com/ze-software/ze/letools/workingtree"
	_ "github.com/ze-software/ze/letools/worktree"
	_ "github.com/ze-software/ze/letools/yangglue"
	_ "github.com/ze-software/ze/letools/yangleafmentions"
)
