// Design: docs/architecture/iface/logical-name-resolution.md -- AC-U1 no-direct-resolution guard
//
// iface_resolution enforces the interface-resolution invariant from the
// iface-resolve umbrella (sub-spec 7): no Ze code may resolve a configured
// interface name straight against the kernel. Operator-facing logical names
// must go through the shared iface resolver (iface.Resolve / iface.Addresses /
// iface.Subscribe) or the iface dispatch ops, so the os-name / mac-match
// selectors are honored everywhere instead of forcing name == kernel device.
//
// It scans internal/ for direct kernel name->device resolution CALLS --
// netlink.LinkByName(...), net.InterfaceByName(...), and the SIOCGIFINDEX ioctl
// -- in non-test .go files, and fails for any site outside the allowlist below.
// Each allowlist entry states why that path legitimately resolves directly: the
// resolver/kernel owner itself, a post-resolution os-name lookup, a one-shot
// command with no iface backend loaded, or a kernel-sourced device name.
//
// Usage:     go run scripts/checks/iface_resolution.go [--json]
// Called by: make ze-iface-resolution-check (wired into ze-verify) and
//            scripts/checks/iface_resolution_test.go
//
//go:build ignore

package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

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
	"internal/plugins/diag/cmd/capture_interface_linux.go": "post-resolution: net.InterfaceByName(binding.OsName) after iface.Resolve, to obtain the *net.Interface the AF_PACKET capture socket needs.",
	"internal/plugins/ldp/register.go":                     "post-resolution: net.InterfaceByName(b.OsName) after iface.Resolve, to obtain the *net.Interface the multicast socket needs.",
	"internal/component/doctor/":                           "one-shot root CLI (ze doctor) with no iface backend loaded; a resolver call would error on every check. Honors no selectors by design.",
	"internal/component/l2tp/ppp/":                         "pppN device names are kernel-assigned per session (created/point-to-point kinds), never config-sourced logical names, so no selector applies (umbrella assumption A-5).",
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

type finding struct {
	File string `json:"file"`
	Line int    `json:"line"`
	Code string `json:"code"`
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

func main() {
	jsonOut := false
	for _, a := range os.Args[1:] {
		if a == "--json" {
			jsonOut = true
		}
	}

	var findings []finding
	// Scan the daemon (internal), the process entry points (cmd), and the
	// public SDK (pkg). A direct kernel resolution anywhere a consumer lives
	// must go through the resolver or be allowlisted.
	for _, root := range []string{"cmd", "internal", "pkg"} {
		if fi, statErr := os.Stat(root); statErr != nil || !fi.IsDir() {
			continue
		}
		walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
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
			if allowed(rel) {
				return nil
			}
			f, oerr := os.Open(path)
			if oerr != nil {
				return oerr
			}
			defer f.Close()
			sc := bufio.NewScanner(f)
			sc.Buffer(make([]byte, 1024*1024), 1024*1024)
			ln := 0
			for sc.Scan() {
				ln++
				line := sc.Text()
				code := stripComment(line)
				if code == "" {
					continue
				}
				for _, re := range patterns {
					if re.MatchString(code) {
						findings = append(findings, finding{File: rel, Line: ln, Code: strings.TrimSpace(line)})
						break
					}
				}
			}
			return sc.Err()
		})
		if walkErr != nil {
			fmt.Fprintf(os.Stderr, "iface-resolution: scan error in %s: %v\n", root, walkErr)
			os.Exit(2)
		}
	}

	sort.Slice(findings, func(i, j int) bool {
		if findings[i].File != findings[j].File {
			return findings[i].File < findings[j].File
		}
		return findings[i].Line < findings[j].Line
	})

	if jsonOut {
		_ = json.NewEncoder(os.Stdout).Encode(findings)
		if len(findings) > 0 {
			os.Exit(1)
		}
		return
	}

	if len(findings) > 0 {
		fmt.Fprintf(os.Stderr, "iface-resolution: %d direct kernel resolution site(s) outside the allowlist:\n", len(findings))
		for _, f := range findings {
			fmt.Fprintf(os.Stderr, "  %s:%d: %s\n", f.File, f.Line, f.Code)
		}
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Resolve logical interface names via iface.Resolve / iface.Addresses / iface.Subscribe")
		fmt.Fprintln(os.Stderr, "(or the iface dispatch ops), so os-name / mac-match selectors are honored. If a site")
		fmt.Fprintln(os.Stderr, "must resolve directly, add it to the allowlist in scripts/checks/iface_resolution.go")
		fmt.Fprintln(os.Stderr, "with a reason.")
		os.Exit(1)
	}

	fmt.Println("iface-resolution: OK")
}
