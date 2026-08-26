// Design: docs/architecture/core-design.md -- the tool set a ze_le build links
// Overview: zele.go -- the `ze le` root that dispatches them
//
// This file has one blank import for each tool in cmd/le/register.go.
// A missing import would make a command exist in le but not in the ze crossing.
// TestTheCrossingLinksEveryToolLeCarries compares the files instead of relying on this comment.
//
// The list is duplicated because cmd/le/register.go is le's named composition root.
// Thirty letools/*/register.go headers direct authors to add blank imports there.
// Sharing the list elsewhere would make those instructions false.

package zele

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
