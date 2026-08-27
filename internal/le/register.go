// Design: docs/architecture/core-design.md -- the development-tool composition root
//
// Package le composes every development tool behind one registered root. A
// standalone cmd/ze binary named le and a tagged ze build call this same root.
package le

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/ze-software/ze/internal/component/command/registry"
	_ "github.com/ze-software/ze/internal/le/aisync"
	_ "github.com/ze-software/ze/internal/le/archmap"
	_ "github.com/ze-software/ze/internal/le/buildartifacts"
	_ "github.com/ze-software/ze/internal/le/changed"
	_ "github.com/ze-software/ze/internal/le/cidispatch"
	_ "github.com/ze-software/ze/internal/le/cligrammar"
	_ "github.com/ze-software/ze/internal/le/commandlist"
	_ "github.com/ze-software/ze/internal/le/commandownership"
	_ "github.com/ze-software/ze/internal/le/configclaims"
	_ "github.com/ze-software/ze/internal/le/configcoercion"
	_ "github.com/ze-software/ze/internal/le/consistency"
	_ "github.com/ze-software/ze/internal/le/dashstdio"
	_ "github.com/ze-software/ze/internal/le/deployment"
	_ "github.com/ze-software/ze/internal/le/devsetup"
	_ "github.com/ze-software/ze/internal/le/digest"
	_ "github.com/ze-software/ze/internal/le/discoveryindex"
	_ "github.com/ze-software/ze/internal/le/doccheck"
	_ "github.com/ze-software/ze/internal/le/docstocode"
	_ "github.com/ze-software/ze/internal/le/docvalid"
	_ "github.com/ze-software/ze/internal/le/docwiring"
	_ "github.com/ze-software/ze/internal/le/evidence"
	_ "github.com/ze-software/ze/internal/le/featuretags"
	_ "github.com/ze-software/ze/internal/le/fspersistence"
	_ "github.com/ze-software/ze/internal/le/functional"
	_ "github.com/ze-software/ze/internal/le/fuzz"
	_ "github.com/ze-software/ze/internal/le/goextract"
	_ "github.com/ze-software/ze/internal/le/gokrazygosum"
	_ "github.com/ze-software/ze/internal/le/hookcheck"
	_ "github.com/ze-software/ze/internal/le/htmxupgrade"
	_ "github.com/ze-software/ze/internal/le/ianaasn"
	_ "github.com/ze-software/ze/internal/le/ifaceresolution"
	_ "github.com/ze-software/ze/internal/le/integration"
	_ "github.com/ze-software/ze/internal/le/inventory"
	_ "github.com/ze-software/ze/internal/le/journal"
	_ "github.com/ze-software/ze/internal/le/lejob"
	_ "github.com/ze-software/ze/internal/le/letracked"
	_ "github.com/ze-software/ze/internal/le/lintgate"
	"github.com/ze-software/ze/internal/le/leroot"
	_ "github.com/ze-software/ze/internal/le/parity"
	_ "github.com/ze-software/ze/internal/le/perfbench"
	_ "github.com/ze-software/ze/internal/le/platformvet"
	_ "github.com/ze-software/ze/internal/le/pluginboundary"
	_ "github.com/ze-software/ze/internal/le/pluginimports"
	_ "github.com/ze-software/ze/internal/le/portdefaults"
	_ "github.com/ze-software/ze/internal/le/protocolskeleton"
	_ "github.com/ze-software/ze/internal/le/pylint"
	_ "github.com/ze-software/ze/internal/le/qemu"
	_ "github.com/ze-software/ze/internal/le/repository"
	_ "github.com/ze-software/ze/internal/le/rfc"
	_ "github.com/ze-software/ze/internal/le/rules"
	_ "github.com/ze-software/ze/internal/le/scratch"
	_ "github.com/ze-software/ze/internal/le/sitefacts"
	_ "github.com/ze-software/ze/internal/le/speccitation"
	_ "github.com/ze-software/ze/internal/le/specstatus"
	_ "github.com/ze-software/ze/internal/le/staticcheckmatrix"
	_ "github.com/ze-software/ze/internal/le/ste"
	_ "github.com/ze-software/ze/internal/le/terminaldemo"
	_ "github.com/ze-software/ze/internal/le/testchaos"
	_ "github.com/ze-software/ze/internal/le/testhealth"
	_ "github.com/ze-software/ze/internal/le/testsensitivity"
	_ "github.com/ze-software/ze/internal/le/testunit"
	_ "github.com/ze-software/ze/internal/le/tier"
	_ "github.com/ze-software/ze/internal/le/tokeneconomy"
	_ "github.com/ze-software/ze/internal/le/trackedbuild"
	_ "github.com/ze-software/ze/internal/le/vendorweb"
	_ "github.com/ze-software/ze/internal/le/verifydeps"
	_ "github.com/ze-software/ze/internal/le/verifyworktree"
	_ "github.com/ze-software/ze/internal/le/weakened"
	_ "github.com/ze-software/ze/internal/le/webassets"
	_ "github.com/ze-software/ze/internal/le/weekly"
	_ "github.com/ze-software/ze/internal/le/workingtree"
	_ "github.com/ze-software/ze/internal/le/worktree"
	_ "github.com/ze-software/ze/internal/le/yangglue"
	_ "github.com/ze-software/ze/internal/le/yangleafmentions"
)

func init() {
	registry.MustRegisterRootHandler("le", run, registry.Meta{
		Description: "the Ze repository and development commands, in-process",
		Mode:        "offline",
		Section:     registry.SectionTest,
	})
}

func run(_ *registry.RuntimeContext, args []string) int {
	return leroot.Dispatch(invocationName(), args)
}

func invocationName() string {
	name := filepath.Base(os.Args[0])
	name = strings.TrimSuffix(name, ".exe")
	if name == "le" {
		return "le"
	}
	return "ze le"
}
