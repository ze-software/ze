// Design: docs/architecture/core-design.md -- the development-tool composition root
//
// Package le composes every development tool behind one registered root. A
// standalone cmd/ze binary named le and a tagged ze build call this same root.
package le

import (
	"os"

	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/core/textbuf"
	_ "github.com/ze-software/ze/internal/le/ai/digest"
	_ "github.com/ze-software/ze/internal/le/ai/hooks"
	_ "github.com/ze-software/ze/internal/le/ai/rules"
	_ "github.com/ze-software/ze/internal/le/ai/sync"
	_ "github.com/ze-software/ze/internal/le/ai/tokens"
	_ "github.com/ze-software/ze/internal/le/arch/compoundguard"
	_ "github.com/ze-software/ze/internal/le/arch/enumeration"
	_ "github.com/ze-software/ze/internal/le/arch/fspersistence"
	_ "github.com/ze-software/ze/internal/le/arch/ifaceresolution"
	_ "github.com/ze-software/ze/internal/le/arch/refusebydefault"
	_ "github.com/ze-software/ze/internal/le/arch/tier"
	_ "github.com/ze-software/ze/internal/le/build/gokrazy"
	_ "github.com/ze-software/ze/internal/le/build/gosum"
	_ "github.com/ze-software/ze/internal/le/build/hostdriver"
	_ "github.com/ze-software/ze/internal/le/build/installer"
	_ "github.com/ze-software/ze/internal/le/chaos/run"
	_ "github.com/ze-software/ze/internal/le/chaos/selftest"
	_ "github.com/ze-software/ze/internal/le/cli/catalog"
	_ "github.com/ze-software/ze/internal/le/cli/dispatch"
	_ "github.com/ze-software/ze/internal/le/cli/grammar"
	_ "github.com/ze-software/ze/internal/le/cli/list"
	_ "github.com/ze-software/ze/internal/le/cli/ownership"
	_ "github.com/ze-software/ze/internal/le/cli/stdio"
	_ "github.com/ze-software/ze/internal/le/commit"
	_ "github.com/ze-software/ze/internal/le/completion"
	_ "github.com/ze-software/ze/internal/le/config/claims"
	_ "github.com/ze-software/ze/internal/le/config/coercion"
	_ "github.com/ze-software/ze/internal/le/config/ports"
	_ "github.com/ze-software/ze/internal/le/config/unreadleaves"
	_ "github.com/ze-software/ze/internal/le/data/asndelegation"
	_ "github.com/ze-software/ze/internal/le/data/ietfreference"
	_ "github.com/ze-software/ze/internal/le/data/ipspecialpurpose"
	_ "github.com/ze-software/ze/internal/le/doc/check"
	_ "github.com/ze-software/ze/internal/le/doc/consistency"
	_ "github.com/ze-software/ze/internal/le/doc/index"
	_ "github.com/ze-software/ze/internal/le/doc/ste"
	_ "github.com/ze-software/ze/internal/le/doc/wiring"
	_ "github.com/ze-software/ze/internal/le/doc/yangcontract"
	_ "github.com/ze-software/ze/internal/le/exec"
	_ "github.com/ze-software/ze/internal/le/feature"
	_ "github.com/ze-software/ze/internal/le/go/extract"
	_ "github.com/ze-software/ze/internal/le/go/lint"
	_ "github.com/ze-software/ze/internal/le/go/module"
	_ "github.com/ze-software/ze/internal/le/go/patchstamps"
	_ "github.com/ze-software/ze/internal/le/go/staticcheck"
	_ "github.com/ze-software/ze/internal/le/go/stdpatch"
	"github.com/ze-software/ze/internal/le/go/stdpatch/gen"
	_ "github.com/ze-software/ze/internal/le/go/versionpin"
	_ "github.com/ze-software/ze/internal/le/go/vetplatforms"
	_ "github.com/ze-software/ze/internal/le/job"
	_ "github.com/ze-software/ze/internal/le/lab/pin"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
	leroot "github.com/ze-software/ze/internal/le/le/root"
	_ "github.com/ze-software/ze/internal/le/mrt"
	_ "github.com/ze-software/ze/internal/le/perf"
	_ "github.com/ze-software/ze/internal/le/plugin/boundary"
	_ "github.com/ze-software/ze/internal/le/plugin/coupling"
	_ "github.com/ze-software/ze/internal/le/plugin/declarations"
	_ "github.com/ze-software/ze/internal/le/plugin/gatedtable"
	_ "github.com/ze-software/ze/internal/le/plugin/imports"
	_ "github.com/ze-software/ze/internal/le/repo"
	_ "github.com/ze-software/ze/internal/le/repo/archmap"
	_ "github.com/ze-software/ze/internal/le/repo/bootstraps"
	_ "github.com/ze-software/ze/internal/le/repo/changed"
	_ "github.com/ze-software/ze/internal/le/repo/compiles"
	_ "github.com/ze-software/ze/internal/le/repo/featuretags"
	_ "github.com/ze-software/ze/internal/le/repo/inventory"
	_ "github.com/ze-software/ze/internal/le/repo/packagemap"
	_ "github.com/ze-software/ze/internal/le/repo/rewrite"
	_ "github.com/ze-software/ze/internal/le/repo/workingtree"
	_ "github.com/ze-software/ze/internal/le/rfc"
	_ "github.com/ze-software/ze/internal/le/rfc/skeletons"
	"github.com/ze-software/ze/internal/le/scratch"
	_ "github.com/ze-software/ze/internal/le/session"
	_ "github.com/ze-software/ze/internal/le/setup"
	_ "github.com/ze-software/ze/internal/le/setup/dockerhost"
	_ "github.com/ze-software/ze/internal/le/setup/dockerkernel"
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
	_ "github.com/ze-software/ze/internal/le/test/appliance"
	_ "github.com/ze-software/ze/internal/le/test/bfd"
	_ "github.com/ze-software/ze/internal/le/test/bgp"
	_ "github.com/ze-software/ze/internal/le/test/cymru"
	_ "github.com/ze-software/ze/internal/le/test/deployment"
	_ "github.com/ze-software/ze/internal/le/test/dhcp"
	_ "github.com/ze-software/ze/internal/le/test/dns"
	_ "github.com/ze-software/ze/internal/le/test/editor"
	_ "github.com/ze-software/ze/internal/le/test/enginesteps"
	_ "github.com/ze-software/ze/internal/le/test/exabgp"
	_ "github.com/ze-software/ze/internal/le/test/failsyscall"
	_ "github.com/ze-software/ze/internal/le/test/firewall"
	_ "github.com/ze-software/ze/internal/le/test/fixture"
	_ "github.com/ze-software/ze/internal/le/test/flowexport"
	_ "github.com/ze-software/ze/internal/le/test/functional"
	_ "github.com/ze-software/ze/internal/le/test/fuzz"
	_ "github.com/ze-software/ze/internal/le/test/health"
	_ "github.com/ze-software/ze/internal/le/test/httpd"
	_ "github.com/ze-software/ze/internal/le/test/httpget"
	_ "github.com/ze-software/ze/internal/le/test/install"
	_ "github.com/ze-software/ze/internal/le/test/integration"
	_ "github.com/ze-software/ze/internal/le/test/interopbgp"
	_ "github.com/ze-software/ze/internal/le/test/ipsec"
	_ "github.com/ze-software/ze/internal/le/test/irr"
	_ "github.com/ze-software/ze/internal/le/test/isis"
	_ "github.com/ze-software/ze/internal/le/test/jsonipbytes"
	_ "github.com/ze-software/ze/internal/le/test/l2tp"
	_ "github.com/ze-software/ze/internal/le/test/ldp"
	_ "github.com/ze-software/ze/internal/le/test/lg"
	_ "github.com/ze-software/ze/internal/le/test/localdatacoverage"
	_ "github.com/ze-software/ze/internal/le/test/managed"
	_ "github.com/ze-software/ze/internal/le/test/mcp"
	_ "github.com/ze-software/ze/internal/le/test/mutation"
	_ "github.com/ze-software/ze/internal/le/test/netlab"
	_ "github.com/ze-software/ze/internal/le/test/ospf"
	_ "github.com/ze-software/ze/internal/le/test/ospfv3"
	_ "github.com/ze-software/ze/internal/le/test/peer"
	_ "github.com/ze-software/ze/internal/le/test/peeringdb"
	_ "github.com/ze-software/ze/internal/le/test/pluginexternal"
	_ "github.com/ze-software/ze/internal/le/test/policy"
	_ "github.com/ze-software/ze/internal/le/test/pppoe"
	_ "github.com/ze-software/ze/internal/le/test/qemu"
	_ "github.com/ze-software/ze/internal/le/test/radiusmock"
	_ "github.com/ze-software/ze/internal/le/test/recordplugin"
	_ "github.com/ze-software/ze/internal/le/test/replay"
	_ "github.com/ze-software/ze/internal/le/test/rpki"
	_ "github.com/ze-software/ze/internal/le/test/rsvpte"
	_ "github.com/ze-software/ze/internal/le/test/rtrmock"
	_ "github.com/ze-software/ze/internal/le/test/runner"
	_ "github.com/ze-software/ze/internal/le/test/scale"
	_ "github.com/ze-software/ze/internal/le/test/sensitivity"
	_ "github.com/ze-software/ze/internal/le/test/static"
	_ "github.com/ze-software/ze/internal/le/test/stressrepro"
	_ "github.com/ze-software/ze/internal/le/test/syslog"
	_ "github.com/ze-software/ze/internal/le/test/tacacsmock"
	_ "github.com/ze-software/ze/internal/le/test/textplugin"
	_ "github.com/ze-software/ze/internal/le/test/traffic"
	_ "github.com/ze-software/ze/internal/le/test/ui"
	_ "github.com/ze-software/ze/internal/le/test/unit"
	_ "github.com/ze-software/ze/internal/le/test/vpp"
	_ "github.com/ze-software/ze/internal/le/test/vrrp"
	_ "github.com/ze-software/ze/internal/le/test/weakened"
	_ "github.com/ze-software/ze/internal/le/test/web"
	_ "github.com/ze-software/ze/internal/le/test/wire"
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

// run is the le root handler: every le command and native hook enters here.
// It links the checkout's cache/ (ensureCacheLink), then selects the patched
// standard library (activatePatchedGOROOT), in that order and outside the
// hook contract, except for a shell callback (leroot.ShellCallback), which
// runs none of them. Before dispatching it runs the hooks areas registered to run ahead of every
// command (leroot.RegisterBeforeDispatch), such as the scratch store trim's
// hourly chance to start. A hook never prints and returns nothing, so the
// command's exit code and output are the dispatch's alone.
func run(_ *registry.RuntimeContext, args []string) int {
	program := leaction.InvocationName()
	// A shell callback runs on a keystroke: the repair and the hooks below can
	// print or start work, and the shell reads stdout as candidates.
	if leroot.ShellCallback(args) {
		return leroot.Dispatch(program, args)
	}
	ensureCacheLink()
	activatePatchedGOROOT()
	leroot.RunBeforeDispatch(program)
	return leroot.Dispatch(program, args)
}

// ensureCacheLink makes the checkout's cache/ the link to the per-user cache
// (scratch.EnsureCache), and renames a real cache/ directory aside, before
// activatePatchedGOROOT runs: its gen.CacheDir creates cache/stdpatch, which
// would make a missing cache/ a real directory and grow a second Go build cache
// in the checkout. It covers every le that the `le` script did not start, such
// as bin/le run directly in a git-archive export. It is not a
// leroot.BeforeDispatch hook, because that contract forbids printing, and a
// repair or a refusal is printed on stderr, once, so stdout stays the action's
// own. A correct link costs one Lstat and one Readlink, and no process.
func ensureCacheLink() {
	root, err := lepath.Root()
	if err != nil {
		return // no checkout: no cache/ to link
	}
	result, err := scratch.New(root, os.Environ()).EnsureCache(false)
	var text textbuf.Buffer
	if err != nil {
		text.Str("le: cannot link ").Str(root).Str("/cache to the per-user cache: ").Err(err).
			Str(". Set HOME or XDG_CACHE_HOME, then run `./le scratch links-ensure`\n")
	}
	if err == nil && result.Announce() {
		text.Str(result.Line).Byte('\n')
	}
	if text.Len() == 0 {
		return
	}
	_, _ = os.Stderr.WriteString(text.String()) //nolint:errcheck // stderr is the report channel; it has nowhere to report its own failure
}

// activatePatchedGOROOT selects the patched standard library for this process
// and every child it starts, FIRST, while le runs one goroutine: a
// before-dispatch hook may start goroutines, and os.Setenv must race none
// (docs/contributing/patching-the-standard-library.md). It is not a
// leroot.BeforeDispatch hook, because that contract forbids slow work and the
// first call per toolchain builds the tree. A refusal never stops the
// dispatch: the hooks, ./le spec, the commit flow and ./le go stdpatch goroot
// keep working on the day a Go bump refuses, and gen.Activate keeps the
// error for every action that compiles through gotoolchain.New.
func activatePatchedGOROOT() {
	root, err := lepath.Root()
	if err != nil {
		return // no checkout: no le action here compiles Ze
	}
	// RFC 5216 Section 2.1.1
	_, _ = gen.Activate(root) //nolint:errcheck // kept by stdpatch and returned by gotoolchain.New and ./le go stdpatch goroot
}
