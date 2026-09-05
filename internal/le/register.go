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
	_ "github.com/ze-software/ze/internal/le/ai"
	_ "github.com/ze-software/ze/internal/le/archmap"
	_ "github.com/ze-software/ze/internal/le/buildartifacts"
	_ "github.com/ze-software/ze/internal/le/changed"
	_ "github.com/ze-software/ze/internal/le/cidispatch"
	_ "github.com/ze-software/ze/internal/le/cligrammar"
	_ "github.com/ze-software/ze/internal/le/command/list"
	_ "github.com/ze-software/ze/internal/le/command/ownership"
	_ "github.com/ze-software/ze/internal/le/commit"
	_ "github.com/ze-software/ze/internal/le/config/claims"
	_ "github.com/ze-software/ze/internal/le/config/coercion"
	_ "github.com/ze-software/ze/internal/le/consistency"
	_ "github.com/ze-software/ze/internal/le/dashstdio"
	_ "github.com/ze-software/ze/internal/le/deployment"
	_ "github.com/ze-software/ze/internal/le/digest"
	_ "github.com/ze-software/ze/internal/le/discoveryindex"
	_ "github.com/ze-software/ze/internal/le/doc/check"
	_ "github.com/ze-software/ze/internal/le/doc/wiring"
	_ "github.com/ze-software/ze/internal/le/docstocode"
	_ "github.com/ze-software/ze/internal/le/docvalid"
	_ "github.com/ze-software/ze/internal/le/evidence"
	_ "github.com/ze-software/ze/internal/le/featuretags"
	_ "github.com/ze-software/ze/internal/le/fspersistence"
	_ "github.com/ze-software/ze/internal/le/functional"
	_ "github.com/ze-software/ze/internal/le/fuzz"
	_ "github.com/ze-software/ze/internal/le/goextract"
	_ "github.com/ze-software/ze/internal/le/gokrazygosum"
	_ "github.com/ze-software/ze/internal/le/goversion"
	_ "github.com/ze-software/ze/internal/le/hookcheck"
	_ "github.com/ze-software/ze/internal/le/htmxupgrade"
	_ "github.com/ze-software/ze/internal/le/ianaasn"
	_ "github.com/ze-software/ze/internal/le/ifaceresolution"
	_ "github.com/ze-software/ze/internal/le/integration"
	_ "github.com/ze-software/ze/internal/le/inventory"
	_ "github.com/ze-software/ze/internal/le/job"
	_ "github.com/ze-software/ze/internal/le/journal"
	"github.com/ze-software/ze/internal/le/leroot"
	_ "github.com/ze-software/ze/internal/le/module"
	_ "github.com/ze-software/ze/internal/le/mutation"
	_ "github.com/ze-software/ze/internal/le/netlab"
	_ "github.com/ze-software/ze/internal/le/perfbench"
	_ "github.com/ze-software/ze/internal/le/platformvet"
	_ "github.com/ze-software/ze/internal/le/plugin/boundary"
	_ "github.com/ze-software/ze/internal/le/plugin/imports"
	_ "github.com/ze-software/ze/internal/le/portdefaults"
	_ "github.com/ze-software/ze/internal/le/protocolskeleton"
	_ "github.com/ze-software/ze/internal/le/qemu"
	_ "github.com/ze-software/ze/internal/le/repository"
	_ "github.com/ze-software/ze/internal/le/repository/trackedbuild"
	_ "github.com/ze-software/ze/internal/le/rfc"
	_ "github.com/ze-software/ze/internal/le/rules"
	_ "github.com/ze-software/ze/internal/le/scratch"
	_ "github.com/ze-software/ze/internal/le/session"
	_ "github.com/ze-software/ze/internal/le/setup"
	_ "github.com/ze-software/ze/internal/le/site"
	_ "github.com/ze-software/ze/internal/le/site/facts"
	_ "github.com/ze-software/ze/internal/le/site/wiki"
	_ "github.com/ze-software/ze/internal/le/sourcerewrite"
	_ "github.com/ze-software/ze/internal/le/spec/citation"
	_ "github.com/ze-software/ze/internal/le/spec/session"
	_ "github.com/ze-software/ze/internal/le/spec/status"
	_ "github.com/ze-software/ze/internal/le/staticcheckfeaturematrix"
	_ "github.com/ze-software/ze/internal/le/ste"
	_ "github.com/ze-software/ze/internal/le/stressrepro"
	_ "github.com/ze-software/ze/internal/le/terminaldemo"
	_ "github.com/ze-software/ze/internal/le/testchaos"
	_ "github.com/ze-software/ze/internal/le/testhealth"
	_ "github.com/ze-software/ze/internal/le/testhelper"
	_ "github.com/ze-software/ze/internal/le/testsensitivity"
	_ "github.com/ze-software/ze/internal/le/testunit"
	_ "github.com/ze-software/ze/internal/le/testweakened"
	_ "github.com/ze-software/ze/internal/le/tier"
	_ "github.com/ze-software/ze/internal/le/tokeneconomy"
	_ "github.com/ze-software/ze/internal/le/tracked"
	_ "github.com/ze-software/ze/internal/le/vendorweb"
	_ "github.com/ze-software/ze/internal/le/verify"
	_ "github.com/ze-software/ze/internal/le/verify/deps"
	_ "github.com/ze-software/ze/internal/le/verify/lint"
	_ "github.com/ze-software/ze/internal/le/verify/lock"
	_ "github.com/ze-software/ze/internal/le/verify/status"
	_ "github.com/ze-software/ze/internal/le/verify/summary"
	_ "github.com/ze-software/ze/internal/le/webassets"
	_ "github.com/ze-software/ze/internal/le/weekly"
	_ "github.com/ze-software/ze/internal/le/wikicatalog"
	_ "github.com/ze-software/ze/internal/le/workingtree"
	_ "github.com/ze-software/ze/internal/le/worktree"
	_ "github.com/ze-software/ze/internal/le/yang/glue"
	_ "github.com/ze-software/ze/internal/le/yang/leafmentions"
	_ "github.com/ze-software/ze/internal/le/yang/migration"
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
