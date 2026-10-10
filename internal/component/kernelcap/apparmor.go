// Design: docs/architecture/doctor-and-health-checks.md -- the kernel capability tier
// Related: probe_linux.go -- remountSysctlWritable and the label-space write this profile grants
// Related: policy_linux.go -- the denied answer, whose reason names this profile
// Related: ../../le/interoplab/kernelcheck.go -- the probe container that runs under it
//
// Docker confines every container it starts with its docker-default AppArmor
// profile, generated from github.com/moby/profiles/apparmor template.go
// (apparmor/v0.2.2, the version moby master pins at 9fabd6dfbb92 on
// 2026-10-10). Two of its rules stop the MPLS transit MTU probe: `deny mount,`
// refuses the private mount namespace in which the probe makes /proc/sys
// writable, and `deny @{PROC}/sys/[^k]** w,` refuses the label-space write that
// an allowed mount would still meet. AppArmor applies a deny over every allow,
// so no profile that keeps either line can grant the probe. This is the profile
// Ze ships for its probe container instead: docker-default with those two rules
// replaced by exactly the steps the probe takes.

package kernelcap

import (
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// ProbeAppArmorProfileName is the AppArmor profile the kernel probe container
// runs under, and the name a denied probe's reason tells the operator to load.
const ProbeAppArmorProfileName = "ze-kernel-probe"

// probeProfileHead is docker-default's opening, network, capability and signal
// rules, kept verbatim but for the profile name. `file,` is kept because the
// container's stdio and the ze binary are file accesses the probe cannot name
// in advance; the write denies below then cut it back.
const probeProfileHead = `abi <abi/3.0>,
#include <tunables/global>

profile ze-kernel-probe flags=(attach_disconnected,mediate_deleted) {
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
  signal (send,receive) peer=ze-kernel-probe,

  deny @{PROC}/* w,
  deny @{PROC}/{[^1-9/],[^1-9/][^0-9/],[^1-9s/][^0-9y/][^0-9s/],[^1-9/][^0-9/][^0-9/][^0-9/]*}/** w,
`

// probeProfileTail is the three mounts remountSysctlWritable makes, in place of
// docker-default's `deny mount,` (every other mount stays refused, because
// nothing else allows one), then docker-default's remaining denies verbatim.
const probeProfileTail = `  deny @{PROC}/sysrq-trigger rwklx,
  deny @{PROC}/kcore rwklx,

  mount options=(rw, rprivate) -> /,
  mount options=(rw, bind) /proc/sys/ -> /proc/sys/,
  remount options=(rw, bind) /proc/sys/,

  deny /sys/[^f]*/** wklx,
  deny /sys/f[^s]*/** wklx,
  deny /sys/fs/[^c]*/** wklx,
  deny /sys/fs/c[^g]*/** wklx,
  deny /sys/fs/cg[^r]*/** wklx,
  deny /sys/firmware/** rwklx,
  deny /sys/devices/virtual/powercap/** rwklx,
  deny /sys/kernel/security/** rwklx,

  ptrace (trace,tracedby,read,readby) peer=ze-kernel-probe,
}
`

// ProbeAppArmorProfile answers the text of the ze-kernel-probe profile, ready
// for apparmor_parser.
//
// Under /proc/sys it denies every write but the probe's own label-space write.
// AppArmor has no "deny all but", so the denies are a chain over that path:
// one rule for each position, denying any name that differs from the path
// there, and one denying anything longer. The chain is derived from
// MPLSPlatformLabelsPath, so the grant and the probe name one path.
func ProbeAppArmorProfile() string {
	keep := strings.TrimPrefix(MPLSPlatformLabelsPath(), ProcPath("sys")+"/")

	var text textbuf.Buffer
	text.Str(probeProfileHead)
	for i := range len(keep) {
		text.Str("  deny @{PROC}/sys/").Str(keep[:i]).Str("[^").Byte(keep[i]).Str("]** w,\n")
	}
	text.Str("  deny @{PROC}/sys/").Str(keep).Str("{?,/}** w,\n")
	text.Str(probeProfileTail)
	return text.String()
}
