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
	_ "github.com/ze-software/ze/internal/le/ai/digest"
	_ "github.com/ze-software/ze/internal/le/ai/hooks"
	_ "github.com/ze-software/ze/internal/le/ai/rules"
	_ "github.com/ze-software/ze/internal/le/ai/sync"
	_ "github.com/ze-software/ze/internal/le/ai/tokens"
	_ "github.com/ze-software/ze/internal/le/arch/enumeration"
	_ "github.com/ze-software/ze/internal/le/arch/fspersistence"
	_ "github.com/ze-software/ze/internal/le/arch/ifaceresolution"
	_ "github.com/ze-software/ze/internal/le/arch/tier"
	_ "github.com/ze-software/ze/internal/le/build/gosum"
	_ "github.com/ze-software/ze/internal/le/build/hostdriver"
	_ "github.com/ze-software/ze/internal/le/build/installer"
	_ "github.com/ze-software/ze/internal/le/cli/catalog"
	_ "github.com/ze-software/ze/internal/le/cli/dispatch"
	_ "github.com/ze-software/ze/internal/le/cli/grammar"
	_ "github.com/ze-software/ze/internal/le/cli/list"
	_ "github.com/ze-software/ze/internal/le/cli/ownership"
	_ "github.com/ze-software/ze/internal/le/cli/stdio"
	_ "github.com/ze-software/ze/internal/le/commit"
	_ "github.com/ze-software/ze/internal/le/config/claims"
	_ "github.com/ze-software/ze/internal/le/config/coercion"
	_ "github.com/ze-software/ze/internal/le/config/ports"
	_ "github.com/ze-software/ze/internal/le/config/unreadleaves"
	_ "github.com/ze-software/ze/internal/le/data/asndelegation"
	_ "github.com/ze-software/ze/internal/le/deployment"
	_ "github.com/ze-software/ze/internal/le/doc/check"
	_ "github.com/ze-software/ze/internal/le/doc/consistency"
	_ "github.com/ze-software/ze/internal/le/doc/index"
	_ "github.com/ze-software/ze/internal/le/doc/ste"
	_ "github.com/ze-software/ze/internal/le/doc/wiring"
	_ "github.com/ze-software/ze/internal/le/doc/yangcontract"
	_ "github.com/ze-software/ze/internal/le/functional"
	_ "github.com/ze-software/ze/internal/le/fuzz"
	_ "github.com/ze-software/ze/internal/le/go/extract"
	_ "github.com/ze-software/ze/internal/le/go/lint"
	_ "github.com/ze-software/ze/internal/le/go/module"
	_ "github.com/ze-software/ze/internal/le/go/staticcheck"
	_ "github.com/ze-software/ze/internal/le/go/versionpin"
	_ "github.com/ze-software/ze/internal/le/go/vetplatforms"
	_ "github.com/ze-software/ze/internal/le/integration"
	_ "github.com/ze-software/ze/internal/le/job"
	"github.com/ze-software/ze/internal/le/leroot"
	_ "github.com/ze-software/ze/internal/le/mutation"
	_ "github.com/ze-software/ze/internal/le/netlab"
	_ "github.com/ze-software/ze/internal/le/perfbench"
	_ "github.com/ze-software/ze/internal/le/plugin/boundary"
	_ "github.com/ze-software/ze/internal/le/plugin/declarations"
	_ "github.com/ze-software/ze/internal/le/plugin/imports"
	_ "github.com/ze-software/ze/internal/le/qemu"
	_ "github.com/ze-software/ze/internal/le/repo"
	_ "github.com/ze-software/ze/internal/le/repo/archmap"
	_ "github.com/ze-software/ze/internal/le/repo/changed"
	_ "github.com/ze-software/ze/internal/le/repo/featuretags"
	_ "github.com/ze-software/ze/internal/le/repo/inventory"
	_ "github.com/ze-software/ze/internal/le/repo/packagemap"
	_ "github.com/ze-software/ze/internal/le/repo/rewrite"
	_ "github.com/ze-software/ze/internal/le/repo/trackedbuild"
	_ "github.com/ze-software/ze/internal/le/repo/trackedle"
	_ "github.com/ze-software/ze/internal/le/repo/workingtree"
	_ "github.com/ze-software/ze/internal/le/rfc"
	_ "github.com/ze-software/ze/internal/le/rfc/skeletons"
	_ "github.com/ze-software/ze/internal/le/scratch"
	_ "github.com/ze-software/ze/internal/le/session"
	_ "github.com/ze-software/ze/internal/le/setup"
	_ "github.com/ze-software/ze/internal/le/site"
	_ "github.com/ze-software/ze/internal/le/site/facts"
	_ "github.com/ze-software/ze/internal/le/site/terminaldemo"
	_ "github.com/ze-software/ze/internal/le/site/wiki"
	_ "github.com/ze-software/ze/internal/le/spec/citation"
	_ "github.com/ze-software/ze/internal/le/spec/claim"
	_ "github.com/ze-software/ze/internal/le/spec/current"
	_ "github.com/ze-software/ze/internal/le/spec/journal"
	_ "github.com/ze-software/ze/internal/le/spec/model"
	_ "github.com/ze-software/ze/internal/le/spec/release"
	_ "github.com/ze-software/ze/internal/le/spec/review"
	_ "github.com/ze-software/ze/internal/le/spec/roadmap"
	_ "github.com/ze-software/ze/internal/le/spec/state"
	_ "github.com/ze-software/ze/internal/le/spec/status"
	_ "github.com/ze-software/ze/internal/le/spec/wip"
	_ "github.com/ze-software/ze/internal/le/stressrepro"
	_ "github.com/ze-software/ze/internal/le/testchaos"
	_ "github.com/ze-software/ze/internal/le/testhealth"
	_ "github.com/ze-software/ze/internal/le/testhelper"
	_ "github.com/ze-software/ze/internal/le/testsensitivity"
	_ "github.com/ze-software/ze/internal/le/testunit"
	_ "github.com/ze-software/ze/internal/le/testweakened"
	_ "github.com/ze-software/ze/internal/le/verify"
	_ "github.com/ze-software/ze/internal/le/verify/deps"
	_ "github.com/ze-software/ze/internal/le/verify/evidence"
	_ "github.com/ze-software/ze/internal/le/verify/status"
	_ "github.com/ze-software/ze/internal/le/verify/summary"
	_ "github.com/ze-software/ze/internal/le/web/assets"
	_ "github.com/ze-software/ze/internal/le/web/htmx"
	_ "github.com/ze-software/ze/internal/le/web/vendor"
	_ "github.com/ze-software/ze/internal/le/weekly"
	_ "github.com/ze-software/ze/internal/le/worktree"
	_ "github.com/ze-software/ze/internal/le/yang/glue"
	_ "github.com/ze-software/ze/internal/le/yang/migration"
)

func init() {
	registry.MustRegisterRootHandler("le", run, registry.Meta{
		ShortHelp: "the Ze repository and development commands, in-process",
		Mode:      "offline",
		Section:   registry.SectionTest,
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
