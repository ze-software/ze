// Design: docs/architecture/iface/logical-name-resolution.md -- AC-U1 no-direct-resolution guard
//
// Package ifaceresolution enforces the interface-resolution invariant from the
// iface-resolve umbrella (sub-spec 7): no Ze code may resolve a configured
// interface name straight against the kernel. Operator-facing logical names
// must go through the shared iface resolver (iface.Resolve / iface.Addresses /
// iface.Subscribe) or the iface dispatch ops, so the os-name / mac-match
// selectors are honored everywhere instead of forcing name == kernel device.
//
// It scans cmd/, internal/ and pkg/ for direct kernel name->device resolution
// CALLS -- netlink.LinkByName(...), net.InterfaceByName(...), and the
// SIOCGIFINDEX ioctl -- in non-test .go files, and fails for any site outside
// the allowlist below. Each allowlist entry states why that path legitimately
// resolves directly: the resolver/kernel owner itself, a post-resolution
// os-name lookup, a one-shot command with no iface backend loaded, or a
// kernel-sourced device name.

package ifaceresolution

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/lepath"
	"github.com/ze-software/ze/internal/le/leroot"
)

// name is the word this command is typed as, and the prefix its own messages
// carry. The Make target it still is spells the same words:
// ze-iface-resolution-check.
const name = "iface-resolution"

// roots are the trees a consumer of the resolver can live in: the daemon, the
// process entry points, and the public SDK.
var roots = []string{"cmd", "internal", "pkg"}

// scanFloor is the smallest number of files the three roots can hold and still
// be this repository. A walk that reads fewer has not read Ze, and a gate that
// reports nothing over a tree it never read is the failure this gate exists to
// prevent, applied to itself.
//
// It is a floor rather than a count: 4,800 files on 2026-08-26.
const scanFloor = 500

// allowlist maps a path prefix (a file, or a directory ending in "/", relative
// to the repo root with forward slashes) to the reason direct kernel resolution
// is legitimate there. A scanned file whose path has any of these prefixes is
// exempt; every other direct-resolution site fails the gate. This is a fixture,
// not a comment: a new direct-resolution site not covered here fails, which is
// the point -- it forces the author to either migrate to the resolver or
// justify a new exemption here.
var allowlist = map[string]string{
	"internal/component/iface/":                            "the shared resolver and dispatch -- the single owner of logical-name -> device resolution that every other consumer calls instead of the kernel.",
	"internal/plugins/iface/netlink/":                      "the netlink backend -- the single kernel owner the resolver and dispatch delegate to; LinkByName here IS the resolved kernel call.",
	"internal/plugins/traffic/netlink/":                    "the tc kernel adapter; the traffic backend resolves logical->os in its Apply/RestoreOriginal/ListQdiscs methods (resolveOSName) before this adapter runs, so it only ever sees os device names.",
	"internal/plugins/fib/kernel/mplsentry_linux.go":       "resolves the literal \"lo\" loopback device, not a config-sourced name.",
	"internal/plugins/provision/":                          "one-shot bootstrap CLI (ze provision) run at PXE/DHCP provisioning time; no iface backend is loaded and no logical-name config mapping exists yet, so --interface is a raw kernel device.",
	"internal/plugins/imageserver/register.go":             "install/provision image server resolves through iface.Addresses first; when no iface backend is loaded it falls back to the configured raw kernel name, matching the pre-iface bootstrap path.",
	"internal/install/disk/":                               "the disk installer engine (ze-installer initrd PID 1 and `ze install disk`); a self-contained bootstrap context with no iface backend loaded and no logical-name config -- it pins the boot NIC by ze.mac via sysfs (ifaceForMAC) and brings links up via netlink directly, like the provision bootstrap above (docs/architecture/appliance/installer-initrd.md).",
	"internal/plugins/diag/cmd/capture_interface_linux.go": "post-resolution: the code uses the resolved OS name to obtain the *net.Interface that the AF_PACKET capture socket needs.",
	"internal/plugins/ldp/register.go":                     "post-resolution: the code uses the resolved OS name to obtain the *net.Interface that the multicast socket needs.",
	"internal/component/doctor/":                           "one-shot root CLI (ze doctor) with no iface backend loaded; a resolver call would error on every check. Honors no selectors by design.",
	"internal/component/l2tp/ppp/":                         "pppN device names are kernel-assigned per session (created/point-to-point kinds), never config-sourced logical names, so no selector applies (umbrella assumption A-5).",
	"internal/le/deployment/l2tpdiag_linux_ops.go":          "the diagnostic reads the pppN device that PPPIOCNEWUNIT just asked the kernel to allocate; the name is kernel-assigned and no logical selector exists.",
	"internal/le/interoplab/bgp/isis_inject_linux.go":      "the interop driver enters the lab peer network namespace and opens a raw socket on the veth name that the fixture created; no Ze logical selector exists in the lab process.",
	"internal/plugins/vrrp/register.go":                    "waitDevicePresent polls for the macvlan vrrp just asked iface to create, and needs kernel presence rather than a logical-name lookup: resolver.resolve returns a cached Binding without touching the kernel (component/iface/resolve.go), so a hit would not prove the device exists yet.",
	"internal/plugins/vrrp/transport/backend_linux.go":     "post-resolution: the parent goes through iface.Resolve, and this call names the macvlan vrrp itself created (engine.go passes the generated device), a kernel device rather than a config-sourced logical name, to get the *net.Interface its sockets need.",
	"internal/chaos/peer/simulator_actions_iface_linux.go": "chaos fault injector manipulating a raw veth it created inside its own private netns (integration harness); the iface param is an explicit operator/test input, never a ze logical-interface name, and the chaos simulator process has no iface backend loaded.",
	"internal/test/runner/netns_linux.go":                  "functional-test runner bringing the literal \"lo\" loopback up inside a fresh per-test network namespace it just created (Fix B netns launch mode); \"lo\" is a fixed kernel device name, not a config-sourced logical selector, and the test-runner process has no iface backend loaded.",
}

// patterns match a direct kernel name->device resolution CALL. The trailing '('
// keeps comments (LinkByName mentioned in prose) and function-value references
// (var x = net.InterfaceByName) from matching -- only actual calls count.
var patterns = []*regexp.Regexp{
	regexp.MustCompile(`\bnet\.InterfaceByName\(`),
	regexp.MustCompile(`\.LinkByName\(`),
	regexp.MustCompile(`\bSIOCGIFINDEX\b`),
}

// stripComment returns the code portion of a Go source line, dropping a leading
// or trailing "//" comment so a pattern mentioned only in prose (a comment that
// names net.InterfaceByName) does not register as a call. It cuts at the first
// "//" preceded by whitespace -- gofmt always separates a trailing comment from
// code by a space or tab -- so a "://" inside a string literal (preceded by ':',
// not whitespace) is left intact. Returns "" for a full-line comment or blank
// line.
func stripComment(line string) string {
	if strings.HasPrefix(strings.TrimSpace(line), "//") {
		return ""
	}
	for i := 1; i+1 < len(line); i++ {
		if line[i] == '/' && line[i+1] == '/' && (line[i-1] == ' ' || line[i-1] == '\t') {
			return line[:i]
		}
	}
	return line
}

// allowed reports whether rel is exempt. A directory entry (trailing "/")
// matches any file beneath it; a file entry matches only that exact path, so a
// file prefix cannot accidentally exempt a sibling like "register.go2.go".
func allowed(rel string) bool {
	for prefix := range allowlist {
		if strings.HasSuffix(prefix, "/") {
			if strings.HasPrefix(rel, prefix) {
				return true
			}
		} else if rel == prefix {
			return true
		}
	}
	return false
}

// Check scans tree and answers every direct kernel resolution site outside the
// allowlist, sorted by file and line.
//
// floor is the smallest population that counts as having read the tree, and it
// is a parameter because a fixture is not a checkout: le passes scanFloor and a
// test naming a two-file tree passes 0. tracked_build.go already takes its own
// floor this way, for the same reason.
//
// The error is about the SCAN rather than about the tree: a file the walk
// listed and could not read, or a population too small to be the tree the
// caller meant. Either one means the gate did not judge what it was asked to
// judge, and saying so is the whole difference between a clean tree and an
// unread one.
func Check(tree string, floor int) (Findings, error) {
	var findings Findings
	scanned := 0

	for _, root := range roots {
		dir := filepath.Join(tree, root)
		// A root this tree does not hold contributes no code, which is a
		// different fact from a file that cannot be read. The floor below is
		// what keeps that from becoming a gate over nothing.
		if info, statErr := os.Stat(dir); statErr != nil || !info.IsDir() {
			continue
		}

		walkErr := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			rel, relErr := filepath.Rel(tree, path)
			if relErr != nil {
				return relErr
			}
			rel = filepath.ToSlash(rel)
			scanned++
			if allowed(rel) {
				return nil
			}
			sites, scanErr := scanFile(path, rel)
			if scanErr != nil {
				return scanErr
			}
			findings = append(findings, sites...)
			return nil
		})
		if walkErr != nil {
			return nil, fmt.Errorf("scan error in %s: %w", root, walkErr)
		}
	}

	if scanned < floor {
		var tb textbuf.Buffer
		return nil, errScannedTooLittle(tb.Str("only ").Int(int64(scanned)).
			Str(" Go files scanned under cmd, internal and pkg (floor ").Int(int64(floor)).
			Str("): this is not the tree the gate was asked to judge, so it judged almost nothing").String())
	}

	sort.Slice(findings, func(i, j int) bool {
		if findings[i].File != findings[j].File {
			return findings[i].File < findings[j].File
		}
		return findings[i].Line < findings[j].Line
	})
	return findings, nil
}

// errScannedTooLittle is the floor's error, spelled as a type so a caller can
// tell an unread tree from an unreadable file.
type errScannedTooLittle string

func (e errScannedTooLittle) Error() string { return string(e) }

// scanFile answers every direct resolution site in one file.
func scanFile(path, rel string) (Findings, error) {
	file, err := os.Open(path) //nolint:gosec // the path comes from this tool's own walk
	if err != nil {
		return nil, err
	}
	defer file.Close() //nolint:errcheck // read-only

	var findings Findings
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	number := 0
	for scanner.Scan() {
		number++
		line := scanner.Text()
		code := stripComment(line)
		if code == "" {
			continue
		}
		for _, pattern := range patterns {
			if pattern.MatchString(code) {
				findings = append(findings, Finding{File: rel, Line: number, Code: strings.TrimSpace(line)})
				break
			}
		}
	}
	return findings, scanner.Err()
}

// Answer is the `le iface-resolution` command. The tree is the checkout, so the
// command takes no argument.
func Answer(args []string) (any, int) {
	if len(args) > 0 {
		return nil, leroot.RefuseArgument(name, args[0])
	}

	tree, err := lepath.Root()
	if err != nil {
		fmt.Fprintf(os.Stderr, "iface-resolution: %v\n", err) //nolint:errcheck // CLI output
		return nil, 2
	}

	findings, err := Check(tree, scanFloor)
	if err != nil {
		// 2 rather than 1: the script answered 2 for a scan that did not
		// complete, which is a different fact from a tree holding a violation.
		fmt.Fprintf(os.Stderr, "iface-resolution: %v\n", err) //nolint:errcheck // CLI output
		return nil, 2
	}

	if len(findings) > 0 {
		return findings, 1
	}
	return findings, 0
}
