// Design: docs/architecture/core-design.md -- runtime state belongs in the zefs store
//
// Package fspersistence enforces the invariant that daemon runtime STATE is
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
// pid/socket/probe files, artifacts produced for external consumers
// (resolv.conf, systemd units, PEM exports, the ze binary itself), and the
// config storage layer that IS the abstraction -- are allowlisted by directory
// prefix or by file with a stated reason. To persist real runtime state, use
// internal/core/statestore (a registered pkg/zefs key), never a raw os write.

package fspersistence

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ze-software/ze/internal/le/population"
)

// scanRoots are the trees walked for runtime code. internal/core (statestore,
// crashlog, audit), internal/appliance and internal/install are deliberately
// outside this walk: they are the storage layer, crash-time writers that must
// survive a broken zefs, and build/installer tools, respectively.
var scanRoots = []string{"internal/plugins", "internal/component", "cmd/ze"}

// dirAllowlist exempts whole subsystems whose every write is legitimately a
// non-state file. Prefixes are matched against the repo-relative slash path.
//
// It carries the reason as a value rather than a trailing comment. The
// exemption accounting then prints WHY beside a rule it asks about.
var dirAllowlist = map[string]string{
	"internal/component/config/storage/": "the storage abstraction itself",
}

// segmentAllowlist exempts a path SEGMENT wherever it appears. A test-helper
// package writes fixtures rather than daemon state.
// "/mock/" sat beside "/testing/" until 2026-09-02 and suppressed nothing: no
// mock directory exists under the scan roots, so the rule cannot fire. It
// comes back, with a reason written then, the day one does.
var segmentAllowlist = map[string]string{
	"/testing/": "test-helper package: writes fixtures, not daemon state",
}

// exemptionRules is every rule the three tables hold, under one key space. The
// accounting then asks which of them still suppress a write.
//
// The three key shapes cannot collide. A file key ends in .go. A directory key
// ends in "/" and starts with a root name. A segment key starts with "/".
func exemptionRules() map[string]string {
	rules := make(map[string]string, len(fileAllowlist)+len(dirAllowlist)+len(segmentAllowlist))
	for _, table := range []map[string]string{fileAllowlist, dirAllowlist, segmentAllowlist} {
		maps.Copy(rules, table)
	}
	return rules
}

// fileAllowlist maps a repo-relative .go path to the reason its raw filesystem
// writes are legitimate (kernel knob, ephemeral scratch, external artifact, or
// config-storage machinery -- never daemon state that belongs in zefs). Add an
// entry ONLY after confirming the write does not persist runtime state.
//
// An entry is owed only where it SUPPRESSES a write. Four entries were removed
// on 2026-09-02 because their files no longer hold a write primitive at all:
// config/cli/cmd_fmt.go, config/cli/cmd_migrate.go, imageserver/handler.go and
// imageserver/register.go. Each still names a real file, so an existence check
// would have passed every one of them.
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
	"internal/component/command/pipe_save.go":                        "`| save <path>` writes ONE answer to a path the operator typed, in the operator's own process. It is not daemon state and it never goes through the storage layer: the point of the operator is to put a rendering where the operator asked for it. It is refused where the daemon expands the chain, so a remote caller cannot reach this write (see the file header)",
	"cmd/ze/ze_core_autoinit.go":                                     "writes /dev/kmsg + creates the /perm/ze store dir (zefs bootstrap)",
	// --- ephemeral scratch (pid/socket/probe/ready files, temp stores) ---
	"cmd/ze/hub/pidfile.go":     "runtime pidfile",
	"cmd/ze/hub/service_ssh.go": "ephemeral ssh listen-address handoff file",
	"cmd/ze/hub/main.go":        "test-readiness signal file (path from env)",
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
	"internal/component/config/provider.go":     "config file save/serialize machinery",
	"internal/component/config/cli/cmd_edit.go": "creates the config file / ephemeral ssh addr",
	"internal/component/cli/editor.go":          "generic atomic-write util (config editor)",
}

// osWriteFuncs are the os functions that indicate persistence. os.OpenFile is
// handled separately, and is flagged only with a write flag.
var osWriteFuncs = map[string]bool{
	"WriteFile": true, "Create": true, "Rename": true, "Symlink": true, "Link": true,
}

// readSafeFlags are the os.OpenFile flag terms that cannot open a file for
// writing. A term outside this set is not provable, so the call is flagged.
var readSafeFlags = map[string]bool{
	"0": true, "O_RDONLY": true, "O_NONBLOCK": true, "O_CLOEXEC": true,
	"O_SYNC": true, "O_DSYNC": true, "O_NOCTTY": true, "O_NOFOLLOW": true,
	"O_DIRECTORY": true, "O_LARGEFILE": true,
}

// writeFlags are the os.OpenFile flag terms that prove the call opens for
// writing.
var writeFlags = []string{"O_WRONLY", "O_RDWR", "O_CREATE", "O_APPEND", "O_TRUNC"}

// scanFloor is the least non-test Go files the walk must read before the gate
// believes it saw the tree. This checkout carried 3152 on 2026-08-26, so the
// floor fires on a tree that was never read rather than on one that shrank.
const scanFloor = 500

// Check walks tree's runtime code and answers every raw filesystem write that
// is not allowlisted.
//
// floor is a parameter rather than a constant because a fixture tree holds a
// handful of files: le passes scanFloor and a test passes 0.
func Check(tree string, floor int) (Findings, error) {
	findings, _, err := check(tree, floor)
	return findings, err
}

// CheckCheckout is Check plus the accounting over the exemption rules
// themselves, for a caller that is judging the real repository.
//
// The two are separate for the reason floor is a parameter. Over a fixture, a
// rule that suppresses nothing means the tree does not hold that code. Over the
// checkout it means an exemption that has stopped doing anything. Only the
// caller knows which tree it handed over.
func CheckCheckout(tree string, floor int) (Findings, error) {
	findings, matched, err := check(tree, floor)
	if err != nil {
		return nil, err
	}
	coverage, err := population.Exemptions("fs-persistence allowlist", exemptionRules(), matched)
	if err != nil {
		return nil, err
	}
	if len(coverage.Unexcused) != 0 {
		return nil, fmt.Errorf("%w: %s -- delete the rule, or correct the path it was meant to name",
			ErrDeadAllowlistEntry, strings.Join(coverage.Unexcused, ", "))
	}
	return findings, nil
}

// ErrDeadAllowlistEntry names an exemption rule that suppressed no raw write in
// the tree the walk just read.
var ErrDeadAllowlistEntry = errors.New("an fs-persistence allowlist rule suppresses nothing")

func check(tree string, floor int) (Findings, map[string]bool, error) {
	var all Findings
	read := 0
	matched := make(map[string]bool, len(fileAllowlist))
	fset := token.NewFileSet()

	for _, root := range scanRoots {
		base := filepath.Join(tree, filepath.FromSlash(root))
		err := filepath.WalkDir(base, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			relPath, relErr := filepath.Rel(tree, path)
			if relErr != nil {
				return relErr
			}
			rel := filepath.ToSlash(relPath)
			read++
			// An exempt file is scanned rather than skipped, and its writes are
			// then dropped. Skipping would make "this rule did work" mean only
			// "a file exists under it", which stays true for a rule whose file
			// stopped making the raw write it was excused for.
			found, scanErr := ScanFile(fset, path, rel)
			if scanErr != nil {
				return scanErr
			}
			if rule, exempt := AllowlistedBy(rel); exempt {
				if len(found) != 0 {
					matched[rule] = true
				}
				return nil
			}
			all = append(all, found...)
			return nil
		})
		if err != nil {
			return nil, nil, err
		}
	}

	if read < floor {
		return nil, nil, fmt.Errorf("the walk read %d non-test Go files under %s, below the floor of %d: this tree was not read", read, tree, floor)
	}

	sort.Slice(all, func(i, j int) bool {
		if all[i].File != all[j].File {
			return all[i].File < all[j].File
		}
		return all[i].Line < all[j].Line
	})
	return all, matched, nil
}

// Allowlisted reports whether a repo-relative path's raw writes are declared
// legitimate.
func Allowlisted(rel string) bool {
	_, exempt := AllowlistedBy(rel)
	return exempt
}

// AllowlistedBy answers the rule that exempts rel, and whether one does.
//
// It names WHICH rule because the rules are a population of their own. One that
// suppresses no write is an exemption nobody rechecked. It keeps declaring a raw
// write legitimate for whatever code arrives at that path next. The walk is the
// only producer that can tell.
func AllowlistedBy(rel string) (string, bool) {
	if _, ok := fileAllowlist[rel]; ok {
		return rel, true
	}
	for segment := range segmentAllowlist {
		if strings.Contains(rel, segment) {
			return segment, true
		}
	}
	for prefix := range dirAllowlist {
		if strings.HasPrefix(rel, prefix) {
			return prefix, true
		}
	}
	return "", false
}

// ScanFile answers every raw filesystem write in one file. rel is the name the
// finding carries, which is the repo-relative path a reader can open.
func ScanFile(fset *token.FileSet, path, rel string) ([]Finding, error) {
	src, err := os.ReadFile(path) //nolint:gosec // repository path
	if err != nil {
		return nil, err
	}
	file, err := parser.ParseFile(fset, path, src, 0)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", rel, err)
	}

	osNames, ioutilNames := importNames(file)
	lines := strings.Split(string(src), "\n")

	var out []Finding
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		pkg := identName(selector.X)
		fn := selector.Sel.Name
		switch {
		case osNames[pkg] && fn == "OpenFile":
			// Flag unless the flag argument is provably read-only; a variable
			// or computed flag (os.OpenFile(p, mode, 0)) is treated
			// conservatively.
			if openFileIsReadOnly(src, fset, call) {
				return true
			}
		case osNames[pkg] && osWriteFuncs[fn]:
		case ioutilNames[pkg] && fn == "WriteFile":
		default:
			return true
		}
		line := fset.Position(call.Pos()).Line
		code := ""
		if line >= 1 && line <= len(lines) {
			code = strings.TrimSpace(lines[line-1])
		}
		out = append(out, Finding{File: rel, Line: line, Pkg: pkg, Fn: fn, Code: code})
		return true
	})
	return out, nil
}

// importNames answers the local names of the os and io/ioutil imports, so an
// aliased import (import fsys "os") is not invisible to the selector match.
func importNames(file *ast.File) (osNames, ioutilNames map[string]bool) {
	osNames = map[string]bool{}
	ioutilNames = map[string]bool{}
	for _, imported := range file.Imports {
		path := strings.Trim(imported.Path.Value, `"`)
		name := ""
		if imported.Name != nil {
			name = imported.Name.Name
		}
		switch path {
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
	return osNames, ioutilNames
}

// identName answers the name of an identifier expression, and the empty string
// for anything else.
func identName(expr ast.Expr) string {
	if ident, ok := expr.(*ast.Ident); ok {
		return ident.Name
	}
	return ""
}

// openFileIsReadOnly reports whether an os.OpenFile call's flag argument (the
// second) is PROVABLY read-only -- exactly os.O_RDONLY (any package alias) or
// the literal 0. Anything else (a write flag, a variable, a computed or numeric
// flag) is treated as a potential write and flagged, so a persister cannot hide
// behind a computed flag.
func openFileIsReadOnly(src []byte, fset *token.FileSet, call *ast.CallExpr) bool {
	if len(call.Args) < 2 {
		return false // unusual shape: not provably read-only, so it is flagged
	}
	arg := call.Args[1]
	start := fset.Position(arg.Pos()).Offset
	end := fset.Position(arg.End()).Offset
	if start < 0 || end > len(src) || start >= end {
		return false
	}
	flags := strings.TrimSpace(string(src[start:end]))

	// A write flag anywhere means it is definitely a write.
	for _, flag := range writeFlags {
		if strings.Contains(flags, flag) {
			return false
		}
	}
	// Provably read-only only if EVERY or-ed term is a known read-safe flag (or
	// 0); a variable or unrecognized term is not provable.
	for term := range strings.SplitSeq(flags, "|") {
		trimmed := strings.TrimSpace(term)
		if dot := strings.LastIndex(trimmed, "."); dot >= 0 {
			trimmed = trimmed[dot+1:] // strip package qualifier: os.O_RDONLY -> O_RDONLY
		}
		if !readSafeFlags[trimmed] {
			return false
		}
	}
	return true
}
