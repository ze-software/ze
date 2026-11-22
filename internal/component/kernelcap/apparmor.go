// Design: docs/architecture/doctor-and-health-checks.md -- the kernel capability tier
// Related: probe_linux.go -- remountSysctlWritable and the MPLS label-space write this profile grants
// Related: probe_srv6_linux.go -- the SRv6 forwarding sysctl writes this profile grants
// Related: policy_linux.go -- the denied answer, whose reason names this profile
// Related: ../../le/interoplab/kernelcheck.go -- the probe container that runs under it
//
// Docker confines every container it starts with its docker-default AppArmor
// profile, generated from github.com/moby/profiles/apparmor template.go
// (apparmor/v0.2.2, the version moby master pins at 9fabd6dfbb92 on
// 2026-10-10). Two of its rules stop Ze's throwaway-namespace probes:
// `deny mount,` refuses the private mount namespace in which the probes make
// /proc/sys writable, and `deny @{PROC}/sys/[^k]** w,` refuses the sysctl
// writes that an allowed mount would still meet. AppArmor applies a deny over
// every allow, so no profile that keeps either line can grant the probes. This
// is the profile Ze ships for its probe container instead: docker-default with
// those two rules replaced by exactly the steps the probes take.

package kernelcap

import (
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// ProbeAppArmorProfileName is the AppArmor profile the kernel probe container
// runs under, and the name a denied probe's reason tells the operator to load.
const ProbeAppArmorProfileName = "ze-kernel-probe"

// dockerDefaultHead is docker-default's opening, network, capability and signal
// rules, kept verbatim but for the profile name, which DockerDefaultProfile
// writes in place of NAME. `file,` is kept because the container's stdio and
// its binary are file accesses no profile can name in advance; the write
// denies below then cut it back.
const dockerDefaultHead = `abi <abi/3.0>,
#include <tunables/global>

profile NAME flags=(attach_disconnected,mediate_deleted) {
  #include <abstractions/base>

  network,
  deny network ax25,
  deny network ipx,
  deny network appletalk,
  deny network netrom,
  deny network rose,
  deny network econet,
  deny network atmsvc,
  deny network irda,
  deny network wanpipe,
  deny network isdn,
  deny network caif,
  deny network alg,
  deny network vsock,
  capability,
  file,
  umount,
  signal (receive) peer=unconfined,
  signal (receive) peer=runc,
  signal (receive) peer=crun,
  signal (send,receive) peer=NAME,

  deny @{PROC}/* w,
  deny @{PROC}/{[^1-9/],[^1-9/][^0-9/],[^1-9s/][^0-9y/][^0-9s/],[^1-9/][^0-9/][^0-9/][^0-9/]*}/** w,
`

// dockerDefaultProcDenies is docker-default's two read denies that follow its
// /proc/sys write rules, verbatim.
const dockerDefaultProcDenies = `  deny @{PROC}/sysrq-trigger rwklx,
  deny @{PROC}/kcore rwklx,

`

// dockerDefaultTail is docker-default's /sys denies and its ptrace rule,
// verbatim but for the profile name in place of NAME.
const dockerDefaultTail = `
  deny /sys/[^f]*/** wklx,
  deny /sys/f[^s]*/** wklx,
  deny /sys/fs/[^c]*/** wklx,
  deny /sys/fs/c[^g]*/** wklx,
  deny /sys/fs/cg[^r]*/** wklx,
  deny /sys/firmware/** rwklx,
  deny /sys/devices/virtual/powercap/** rwklx,
  deny /sys/kernel/security/** rwklx,

  ptrace (trace,tracedby,read,readby) peer=NAME,
}
`

// probeMountRules are the three mounts remountSysctlWritable makes, in place of
// docker-default's `deny mount,`: every other mount stays refused, because
// nothing else allows one.
const probeMountRules = `  mount options=(rw, rprivate) -> /,
  mount options=(rw, bind) /proc/sys/ -> /proc/sys/,
  remount options=(rw, bind) /proc/sys/,
`

// DockerDefaultProfile answers docker-default renamed to name, with its
// /proc/sys write rules replaced by procSys and its `deny mount,` replaced by
// mounts. Every other rule stays docker-default's, so a Ze profile differs
// from Docker's only in what its caller states it needs.
func DockerDefaultProfile(name, procSys, mounts string) string {
	var text textbuf.Buffer
	text.Str(strings.ReplaceAll(dockerDefaultHead, "NAME", name))
	text.Str(procSys)
	text.Str(dockerDefaultProcDenies)
	text.Str(mounts)
	text.Str(strings.ReplaceAll(dockerDefaultTail, "NAME", name))
	return text.String()
}

const (
	procSysDenyPrefix   = "  deny @{PROC}/sys/"
	procSysDenyWildcard = "** w,\n"
	procSysDenyLonger   = "{?,/}** w,\n"
)

// ProcSysWriteDenies answers the deny rules that refuse every write under
// /proc/sys but those under keep, relative to /proc/sys. AppArmor has no "deny
// all but", so the denies are a chain over keep: one rule for each position,
// denying any name that differs from keep there. A bracketed class in keep,
// such as [46], is one position that admits each of its characters. A keep
// that ends in a slash grants the whole directory; any other keep is one file,
// and a last rule denies anything longer.
func ProcSysWriteDenies(keep string) string {
	var text textbuf.Buffer
	for i := 0; i < len(keep); {
		width, admits := 1, keep[i:i+1]
		if keep[i] == '[' {
			end := strings.IndexByte(keep[i:], ']')
			if end < 0 {
				panic("BUG: ProcSysWriteDenies keep " + keep + " opens a class it never closes")
			}
			width, admits = end+1, keep[i+1:i+end]
		}
		text.Str(procSysDenyPrefix).Str(keep[:i]).Str("[^").Str(admits).Str("]").Str(procSysDenyWildcard)
		i += width
	}
	if !strings.HasSuffix(keep, "/") {
		text.Str(procSysDenyPrefix).Str(keep).Str(procSysDenyLonger)
	}
	return text.String()
}

type procSysTrieEdge struct {
	ch   byte
	node *procSysTrieNode
}

type procSysTrieNode struct {
	dir      bool
	children []procSysTrieEdge
}

func (n *procSysTrieNode) child(ch byte) *procSysTrieNode {
	for i := range n.children {
		if n.children[i].ch == ch {
			return n.children[i].node
		}
	}
	next := &procSysTrieNode{}
	n.children = append(n.children, procSysTrieEdge{ch: ch, node: next})
	return next
}

// procSysWriteDeniesMulti answers the AppArmor deny rules that refuse every
// write under /proc/sys except the exact relative paths in keeps. It builds a
// byte-trie over the sorted unique paths: at each prefix node with child bytes
// cs it emits `deny @{PROC}/sys/<prefix>[^<cs>]** w,`, and at each file leaf
// (a keep without a trailing `/`) it emits `deny @{PROC}/sys/<prefix>{?,/}** w,`.
// A keep ending in `/` admits the whole directory subtree. An empty keeps slice
// denies every write under /proc/sys.
func procSysWriteDeniesMulti(keeps []string) string {
	if len(keeps) == 0 {
		return procSysDenyPrefix + procSysDenyWildcard
	}
	sorted := slices.Clone(keeps)
	slices.Sort(sorted)
	sorted = slices.Compact(sorted)

	root := &procSysTrieNode{}
	maxLen := 0
	for _, keep := range sorted {
		if keep == "" {
			panic("BUG: procSysWriteDeniesMulti received an empty keep path")
		}
		if strings.ContainsAny(keep, "[]*?{}^-\\,") {
			panic("BUG: procSysWriteDeniesMulti keep " + keep + " holds AppArmor glob metacharacters")
		}
		if len(keep) > maxLen {
			maxLen = len(keep)
		}
		cur := root
		for i := range len(keep) {
			cur = cur.child(keep[i])
		}
		if strings.HasSuffix(keep, "/") {
			cur.dir = true
		}
	}

	var text textbuf.Buffer
	prefix := make([]byte, 0, maxLen)
	emitProcSysTrieDenies(&text, prefix, root)
	return text.String()
}

// emitProcSysTrieDenies emits the deny chain for node. Recursion depth is
// bounded by the length of the longest path in keeps.
func emitProcSysTrieDenies(text *textbuf.Buffer, prefix []byte, node *procSysTrieNode) {
	if node.dir {
		return
	}
	if len(node.children) == 0 {
		text.Str(procSysDenyPrefix).Str(string(prefix)).Str(procSysDenyLonger)
		return
	}
	text.Str(procSysDenyPrefix).Str(string(prefix)).Str("[^")
	for i := range node.children {
		text.Byte(node.children[i].ch)
	}
	text.Str("]").Str(procSysDenyWildcard)
	for i := range node.children {
		emitProcSysTrieDenies(text, append(prefix, node.children[i].ch), node.children[i].node)
	}
}

// probeSysctlKeepPaths returns the /proc/sys-relative paths that Ze's
// throwaway-namespace capability probes write to, sorted. Derived from the
// probes' own ProcPath helpers so the AppArmor grant and the probes cannot
// drift.
func probeSysctlKeepPaths() []string {
	prefix := ProcPath(procSysDir) + "/"
	srv6Paths := srv6ProbeSysctlPaths()
	paths := make([]string, 0, 1+len(srv6Paths))
	paths = append(paths, strings.TrimPrefix(MPLSPlatformLabelsPath(), prefix))
	for _, p := range srv6Paths {
		paths = append(paths, strings.TrimPrefix(p, prefix))
	}
	slices.Sort(paths)
	return paths
}

// ProbeAppArmorProfile answers the text of the ze-kernel-probe profile, ready
// for apparmor_parser.
//
// Under /proc/sys it denies every write except the four sysctls Ze's
// throwaway-namespace probes write (MPLS platform_labels and the three SRv6
// forwarding sysctls), through procSysWriteDeniesMulti over the probes' own
// path helpers so the grant and the probes name one set of paths.
func ProbeAppArmorProfile() string {
	return DockerDefaultProfile(
		ProbeAppArmorProfileName,
		procSysWriteDeniesMulti(probeSysctlKeepPaths()),
		probeMountRules,
	)
}
