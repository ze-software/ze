// Design: ai/rules/architecture.md -- runtime state belongs in the zefs store
//
// direct_fs_persistence enforces the invariant that daemon runtime STATE is
// persisted through ze's managed zefs store (internal/core/statestore ->
// database.zefs), NOT as loose files written with raw os calls. On the gokrazy
// appliance the zefs store is the one integrity-checked, backed-up artifact on
// the writable /perm partition; a loose state file escapes that management and
// silently disappears on reimage. This guard was added after a sweep migrated
// ddos-detect, traffic-usage, ntp, bfd-auth and the config health/pushed hashes
// off loose files.
//
// It scans internal/plugins, internal/component and cmd/ze (non-test) for the
// filesystem WRITE primitives that indicate persistence -- os.WriteFile,
// os.Create, os.OpenFile with a write flag, os.Rename, os.Symlink, os.Link and
// ioutil.WriteFile -- and flags every call site that is not allowlisted. Reads
// (os.ReadFile/os.Open/os.Stat), deletions (os.Remove), temp files
// (os.CreateTemp/os.MkdirTemp) and bare dir creation (os.Mkdir/os.MkdirAll) are
// NOT flagged: the WriteFile/Create/Rename is the load-bearing persistence
// signal, and flagging the rest only adds noise.
//
// Legitimate non-state writers -- kernel sysfs/procfs/dev knobs, ephemeral
// pid/socket/probe files, artifacts produced for external consumers (resolv.conf,
// systemd units, PEM exports, the ze binary itself), and the config storage layer
// that IS the abstraction -- are allowlisted by directory prefix or by file with a
// stated reason. To persist real runtime state, use internal/core/statestore (a
// registered pkg/zefs key), never a raw os write.
//
// Usage:     CGO_ENABLED=0 go run scripts/checks/direct_fs_persistence.go [--json|--selftest]
// Called by: make ze-fs-persistence-check (wired into ze-precommit-verify via
//            scripts/status/verify_run.go) and
//            scripts/checks/direct_fs_persistence_test.go
//
//go:build ignore

package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// scanRoots are the trees walked for runtime code. internal/core (statestore,
// crashlog, audit), internal/appliance and internal/install are deliberately out
// of scope: they are the storage layer, crash-time writers that must survive a
// broken zefs, and build/installer tools, respectively.
var scanRoots = []string{"internal/plugins", "internal/component", "cmd/ze"}

// dirAllowlist exempts whole subsystems whose every write is legitimately a
// non-state file. Prefixes are matched against the repo-relative slash path.
var dirAllowlist = []string{
	"internal/component/config/storage/", // the storage abstraction itself
}

// fileAllowlist maps a repo-relative .go path to the reason its raw filesystem
// writes are legitimate (kernel knob, ephemeral scratch, external artifact, or
// config-storage machinery -- never daemon state that belongs in zefs). Add an
// entry ONLY after confirming the write does not persist runtime state.
var fileAllowlist = map[string]string{
	// --- kernel / device control interfaces (must stay raw) ---
	"internal/plugins/iface/netlink/bridge_linux.go":                 "sysfs bridge stp_state write",
	"internal/plugins/ntp/clock_linux.go":                            "writes the /dev/rtc0 hardware clock",
	"internal/component/host/tuning_linux.go":                        "sysfs cpu governor + procfs IRQ affinity",
	"internal/component/iface/offload_linux.go":                      "sysfs/ethtool offload knobs",
	"internal/component/sysctl/backend_linux.go":                     "/proc/sys sysctl writes",
	"internal/plugins/vrrp/dataplane_linux.go":                       "/proc/sys/net/ipv4/conf arp_ignore/arp_filter/rp_filter knobs for the virtual-MAC dataplane; procfs scalars, no runtime state",
	"internal/plugins/iface/netlink/addr_primary.go":                 "/proc/sys/net/ipv4/conf promote_secondaries knob so deleting a primary IPv4 address does not flush its same-subnet secondaries; procfs scalar, no runtime state",
	"internal/plugins/flowexport/conntrack_setup_appliance_linux.go": "procfs nf_conntrack_acct sysctl on the appliance (ze_appliance conntrack init)",
	"internal/component/vpp/dpdk.go":                                 "sysfs PCI/VFIO/hugepage knobs",
	"internal/component/l2tp/ppp/devppp_linux.go":                    "opens the /dev/ppp kernel device",
	"internal/component/cli/client/main.go":                          "opens /dev/tty (operator terminal)",
	"cmd/ze/ze_core_autoinit.go":                                     "writes /dev/kmsg + creates the /perm/ze store dir (zefs bootstrap)",
	// --- ephemeral scratch (pid/socket/probe/ready files, temp stores) ---
	"internal/plugins/imageserver/register.go": "temp zefs DB served over HTTP (MkdirTemp lifecycle)",
	"internal/plugins/imageserver/handler.go":  "temp database.zefs built for the HTTP response",
	"cmd/ze/hub/pidfile.go":                    "runtime pidfile",
	"cmd/ze/hub/service_ssh.go":                "ephemeral ssh listen-address handoff file",
	"cmd/ze/hub/main.go":                       "test-readiness signal file (path from env)",
	// --- artifacts produced for external consumers (not our state) ---
	"internal/plugins/iface/dhcp/resolv_linux.go":      "system resolv.conf for libc/other daemons",
	"internal/plugins/systemd/main.go":                 "systemd unit file consumed by systemd",
	"internal/plugins/local/cmd_install.go":            "installs the ze binary + config tree into a prefix",
	"internal/plugins/provision/staging.go":            "netboot kernel/initrd/iPXE artifacts for PXE clients",
	"internal/plugins/init/main.go":                    "creates + atomically installs the database.zefs store (zefs bootstrap)",
	"internal/component/pki/store.go":                  "PEM cert/key export for an external IKE daemon",
	"internal/component/support/support.go":            "support bundle archive artifact",
	"internal/component/bgp/reactor/capture_replay.go": "per-peer BGP protocol event capture: a JSONL diagnostic stream the operator hands to a developer, replayed by `ze-test replay` (internal/test/cli/cmd_replay.go). The daemon never reads it back, so it is not runtime state; statestore.Put takes a whole value and cannot carry a bounded, rotating, wire-rate stream",
	"internal/component/vpp/vpp.go":                    "startup.conf consumed by the external VPP process",
	"internal/component/cli/client/transcript.go":      "operator CLI session transcript log",
	"internal/component/config/cli/transcript.go":      "operator CLI session transcript log",
	"internal/component/config/system/resolv_linux.go": "system /etc/resolv.conf for libc/other daemons",
	"internal/component/config/archive/archive.go":     "operator/external config backup artifact",
	"internal/component/config/system/selfupdate.go":   "stages/installs/rolls-back the ze binary (a real executable file); the update-history JSON is persisted via statestore",
	// --- config storage / editing machinery (the layer, not state) ---
	"internal/component/config/provider.go":        "config file save/serialize machinery",
	"internal/component/config/cli/cmd_fmt.go":     "writes formatted config back to the config file",
	"internal/component/config/cli/cmd_migrate.go": "writes migrated config to the output file",
	"internal/component/config/cli/cmd_edit.go":    "creates the config file / ephemeral ssh addr",
	"internal/component/cli/editor.go":             "generic atomic-write util (config editor)",
}

// osWriteFuncs are the os functions that indicate persistence. os.OpenFile is
// handled specially (flagged only with a write flag).
var osWriteFuncs = map[string]bool{
	"WriteFile": true, "Create": true, "Rename": true, "Symlink": true, "Link": true,
}

type finding struct {
	File string `json:"file"`
	Line int    `json:"line"`
	Pkg  string `json:"pkg"`
	Fn   string `json:"fn"`
	Code string `json:"code"`
}

func identName(e ast.Expr) string {
	if id, ok := e.(*ast.Ident); ok {
		return id.Name
	}
	return ""
}

// openFileIsReadOnly reports whether an os.OpenFile call's flag argument (2nd) is
// PROVABLY read-only -- exactly os.O_RDONLY (any package alias) or the literal 0.
// Anything else (a write flag, a variable, a computed or numeric flag) is treated
// as a potential write and flagged, so a persister cannot hide behind a computed
// flag (os.OpenFile(p, mode, 0)).
func openFileIsReadOnly(src []byte, fset *token.FileSet, call *ast.CallExpr) bool {
	if len(call.Args) < 2 {
		return false // unusual shape: not provably read-only -> flag
	}
	a := call.Args[1]
	s := fset.Position(a.Pos()).Offset
	e := fset.Position(a.End()).Offset
	if s < 0 || e > len(src) || s >= e {
		return false
	}
	flags := strings.TrimSpace(string(src[s:e]))
	// A write flag anywhere means it is definitely a write.
	for _, w := range []string{"O_WRONLY", "O_RDWR", "O_CREATE", "O_APPEND", "O_TRUNC"} {
		if strings.Contains(flags, w) {
			return false
		}
	}
	// Provably read-only only if EVERY or-ed term is a known read-safe flag (or 0);
	// a variable or unrecognized term (os.OpenFile(p, mode, 0)) is not provable.
	readSafe := map[string]bool{
		"0": true, "O_RDONLY": true, "O_NONBLOCK": true, "O_CLOEXEC": true,
		"O_SYNC": true, "O_DSYNC": true, "O_NOCTTY": true, "O_NOFOLLOW": true,
		"O_DIRECTORY": true, "O_LARGEFILE": true,
	}
	for _, term := range strings.Split(flags, "|") {
		t := strings.TrimSpace(term)
		if dot := strings.LastIndex(t, "."); dot >= 0 {
			t = t[dot+1:] // strip package qualifier: os.O_RDONLY -> O_RDONLY
		}
		if !readSafe[t] {
			return false
		}
	}
	return true
}

func scanFile(fset *token.FileSet, path string) ([]finding, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	f, err := parser.ParseFile(fset, path, src, 0)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	// Resolve the local names of the os and io/ioutil imports so an aliased
	// import (import fsys "os") is not invisible to the selector match below.
	osNames := map[string]bool{}
	ioutilNames := map[string]bool{}
	for _, imp := range f.Imports {
		p := strings.Trim(imp.Path.Value, `"`)
		name := ""
		if imp.Name != nil {
			name = imp.Name.Name
		}
		switch p {
		case "os":
			if name == "" {
				name = "os"
			}
			osNames[name] = true
		case "io/ioutil":
			if name == "" {
				name = "ioutil"
			}
			ioutilNames[name] = true
		}
	}
	rel := filepath.ToSlash(path)
	lines := strings.Split(string(src), "\n")
	codeAt := func(pos token.Pos) (int, string) {
		ln := fset.Position(pos).Line
		txt := ""
		if ln >= 1 && ln <= len(lines) {
			txt = strings.TrimSpace(lines[ln-1])
		}
		return ln, txt
	}

	var out []finding
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		pkg := identName(sel.X)
		fn := sel.Sel.Name
		switch {
		case osNames[pkg] && fn == "OpenFile":
			// Flag unless the flag argument is provably read-only; a variable or
			// computed flag (os.OpenFile(p, mode, 0)) is treated conservatively.
			if openFileIsReadOnly(src, fset, call) {
				return true
			}
		case osNames[pkg] && osWriteFuncs[fn]:
		case ioutilNames[pkg] && fn == "WriteFile":
		default:
			return true
		}
		ln, txt := codeAt(call.Pos())
		out = append(out, finding{File: rel, Line: ln, Pkg: pkg, Fn: fn, Code: txt})
		return true
	})
	return out, nil
}

func allowlisted(rel string) bool {
	if _, ok := fileAllowlist[rel]; ok {
		return true
	}
	// Test-helper packages (siblings of _test.go: .../testing/, .../mock/) write
	// fixtures, not daemon state.
	if strings.Contains(rel, "/testing/") || strings.Contains(rel, "/mock/") {
		return true
	}
	for _, d := range dirAllowlist {
		if strings.HasPrefix(rel, d) {
			return true
		}
	}
	return false
}

func scan(roots []string) ([]finding, error) {
	var all []finding
	fset := token.NewFileSet()
	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			rel := filepath.ToSlash(path)
			if allowlisted(rel) {
				return nil
			}
			found, serr := scanFile(fset, path)
			if serr != nil {
				return serr
			}
			all = append(all, found...)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].File != all[j].File {
			return all[i].File < all[j].File
		}
		return all[i].Line < all[j].Line
	})
	return all, nil
}

func main() {
	jsonOut := false
	selftest := false
	for _, a := range os.Args[1:] {
		switch a {
		case "--json":
			jsonOut = true
		case "--selftest":
			selftest = true
		}
	}

	if selftest {
		os.Exit(runSelftest())
	}

	findings, err := scan(scanRoots)
	if err != nil {
		fmt.Fprintf(os.Stderr, "direct-fs-persistence: %v\n", err)
		os.Exit(2)
	}

	if jsonOut {
		_ = json.NewEncoder(os.Stdout).Encode(findings)
		if len(findings) > 0 {
			os.Exit(1)
		}
		return
	}

	if len(findings) > 0 {
		fmt.Fprintf(os.Stderr, "direct-fs-persistence: %d raw filesystem write(s) that may persist runtime state:\n", len(findings))
		for _, f := range findings {
			fmt.Fprintf(os.Stderr, "  %s:%d (%s.%s): %s\n", f.File, f.Line, f.Pkg, f.Fn, f.Code)
		}
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Daemon runtime state must persist through the managed zefs store, not loose")
		fmt.Fprintln(os.Stderr, "files: use internal/core/statestore (Put/Get under a registered pkg/zefs key)")
		fmt.Fprintln(os.Stderr, "so appliance state lives inside database.zefs. If this write is a genuine")
		fmt.Fprintln(os.Stderr, "non-state file (kernel knob, ephemeral scratch, external artifact, storage")
		fmt.Fprintln(os.Stderr, "layer), add an allowlist entry with a reason in scripts/checks/direct_fs_persistence.go.")
		os.Exit(1)
	}

	fmt.Println("direct-fs-persistence: OK")
}

func runSelftest() int {
	dir, err := os.MkdirTemp("", "direct-fs-persistence-selftest-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "direct-fs-persistence selftest: %v\n", err)
		return 2
	}
	defer func() { _ = os.RemoveAll(dir) }()

	write := func(rel, content string) string {
		full := filepath.Join(dir, filepath.FromSlash(rel))
		if mkErr := os.MkdirAll(filepath.Dir(full), 0o755); mkErr != nil {
			panic(mkErr)
		}
		if wErr := os.WriteFile(full, []byte(content), 0o644); wErr != nil {
			panic(wErr)
		}
		return full
	}

	fset := token.NewFileSet()
	countCalls := func(path string) int {
		found, ferr := scanFile(fset, path)
		if ferr != nil {
			panic(ferr)
		}
		return len(found)
	}

	var failed []string
	check := func(cond bool, msg string) {
		if !cond {
			failed = append(failed, msg)
		}
	}

	// 1. os.WriteFile + os.Rename persisting state -- both flagged.
	stateWrite := write("state/persist.go", stateWriteFixture)
	check(countCalls(stateWrite) == 2, "os.WriteFile + os.Rename state write not both flagged")

	// 2. statestore usage + reads -- NOT flagged.
	good := write("good/persist.go", goodFixture)
	check(countCalls(good) == 0, "statestore.Put / os.ReadFile / os.Stat wrongly flagged")

	// 3. os.OpenFile O_RDONLY (kernel read) not flagged; O_CREATE|O_WRONLY flagged.
	check(countCalls(write("openread/k.go", openReadFixture)) == 0, "os.OpenFile O_RDONLY wrongly flagged")
	check(countCalls(write("openwrite/k.go", openWriteFixture)) == 1, "os.OpenFile with a write flag not flagged")

	// 4. os.Remove / os.MkdirAll / os.CreateTemp -- NOT flagged (not persistence).
	check(countCalls(write("nonwrite/k.go", nonWriteFixture)) == 0, "os.MkdirAll/Remove/CreateTemp wrongly flagged")

	// 5. os.OpenFile with a variable/computed flag -- flagged (not provably read-only).
	check(countCalls(write("varflag/k.go", varFlagFixture)) == 1, "os.OpenFile with a variable flag not flagged")

	// 6. an aliased os import (import fsys "os") -- still flagged.
	check(countCalls(write("aliased/k.go", aliasedFixture)) == 1, "aliased os.WriteFile not flagged")

	// 7. os.OpenFile O_RDONLY|O_NONBLOCK (a /dev/kmsg-style read) -- NOT flagged.
	check(countCalls(write("rdnonblock/k.go", rdNonBlockFixture)) == 0, "O_RDONLY|O_NONBLOCK read wrongly flagged")

	if len(failed) > 0 {
		fmt.Fprintln(os.Stderr, "direct-fs-persistence selftest FAILED:")
		for _, m := range failed {
			fmt.Fprintf(os.Stderr, "  %s\n", m)
		}
		return 1
	}
	fmt.Println("direct-fs-persistence selftest OK")
	return 0
}

// Fixtures are isolated Go sources fed to scanFile; they are intentionally the
// shapes the guard must (or must not) flag. Kept as consts so the checker's own
// source contains no bare raw-write call for the guard-of-the-guard to trip on.
const stateWriteFixture = `package p
import "os"
func save(path, tmp string, data []byte) error {
	if werr := os.WriteFile(tmp, data, 0o600); werr != nil {
		return werr
	}
	return os.Rename(tmp, path)
}
`

const goodFixture = `package p
import (
	"os"
	"github.com/ze-software/ze/internal/core/statestore"
)
func save(key string, data []byte) (bool, error) { return statestore.Put(key, data) }
func load(path string) ([]byte, error) { return os.ReadFile(path) }
func has(path string) bool { _, serr := os.Stat(path); return serr == nil }
`

const openReadFixture = `package p
import "os"
func read(p string) (*os.File, error) { return os.OpenFile(p, os.O_RDONLY, 0) }
`

const openWriteFixture = `package p
import "os"
func w(p string) (*os.File, error) { return os.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644) }
`

const nonWriteFixture = `package p
import "os"
func c(dir string) error {
	if merr := os.MkdirAll(dir, 0o755); merr != nil {
		return merr
	}
	if rerr := os.Remove(dir); rerr != nil {
		return rerr
	}
	f, terr := os.CreateTemp(dir, "x")
	if terr != nil {
		return terr
	}
	return f.Close()
}
`

const varFlagFixture = `package p
import "os"
func w(p string) (*os.File, error) {
	mode := os.O_WRONLY | os.O_CREATE
	return os.OpenFile(p, mode, 0o644)
}
`

const aliasedFixture = `package p
import fsys "os"
func save(p string, d []byte) error { return fsys.WriteFile(p, d, 0o600) }
`

const rdNonBlockFixture = `package p
import (
	"os"
	"syscall"
)
func read(p string) (*os.File, error) { return os.OpenFile(p, os.O_RDONLY|syscall.O_NONBLOCK, 0) }
`
