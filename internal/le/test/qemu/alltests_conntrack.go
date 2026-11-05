// Design: docs/architecture/testing/qemu-integration.md -- native guest conntrack preparation.
// Related: alltests_deadline.go -- bounded ownership of the preparation child.

package testqemu

import "fmt"

// conntrackSetupTimeout bounds kernel preparation separately from a suite's
// budget. suiteChild retains the same fifteen-second TERM-to-KILL grace.
const conntrackSetupTimeout = "30s"

// guestConntrackSetup owns only inet le_test_conntrack's output chain. The name
// deliberately avoids the ze_ prefix the daemon's firewall reconciler owns.
// Its accept-only rule references ct so the kernel registers tracking hooks;
// loading conntrack alone does not track packets. One atomic batch replaces
// only this chain's rule, so repeated selected runs neither duplicate rules nor
// flush any foreign table. Ze's runtime.require mandates built-in conntrack,
// ctnetlink and nft support: a minimal guest needs no modprobe executable.
const guestConntrackSetup = `set -eu
if [ "$(id -u)" != 0 ] || [ "$(readlink /proc/1/exe)" != /gokrazy/init ]; then
    echo "qemu: conntrack setup requires root inside Ze's system. Fix: invoke qemu all through the Ze-system guest command as root." >&2
    exit 1
fi
case "$(uname -r)" in
    *-ze|*-ze-*) ;;
    *) echo 'qemu: conntrack setup requires Ze runtime kernel. Fix: boot the Ze-system guest with its matching runtime kernel.' >&2; exit 1 ;;
esac
acct=/proc/sys/net/netfilter/nf_conntrack_acct
if [ ! -r "$acct" ] || [ ! -w "$acct" ]; then
    echo 'qemu: Ze runtime kernel exposes no writable conntrack accounting control. Fix: rebuild its runtime profile with the required conntrack features.' >&2
    exit 1
fi
printf '1\n' > "$acct"
if [ "$(cat "$acct")" != 1 ]; then
    echo 'qemu: conntrack accounting did not become enabled. Fix: inspect the Ze guest kernel control before rerunning qemu all.' >&2
    exit 1
fi
nft -f - <<'NFT_CONNTRACK'
add table inet le_test_conntrack
add chain inet le_test_conntrack output { type filter hook output priority -150; policy accept; }
flush chain inet le_test_conntrack output
add rule inet le_test_conntrack output ct state != untracked counter
NFT_CONNTRACK
installed=$(nft --stateless --numeric-priority list chain inet le_test_conntrack output)
compact=$(printf '%s' "$installed" | tr -d '[:space:]')
expected='tableinetle_test_conntrack{chainoutput{typefilterhookoutputpriority-150;policyaccept;ctstate!=untrackedcounter}}'
if [ "$compact" != "$expected" ]; then
    printf 'qemu: conntrack tracking chain readback differs from its accept-only contract: %s. Fix: inspect the reserved le_test_conntrack table before rerunning qemu all.\n' "$installed" >&2
    exit 1
fi
printf 'CONNTRACK-SETUP: acct=%s\n%s\n' "$(cat "$acct")" "$installed"
`

// prepareConntrack establishes the flow source before either Execute path runs
// a suite. The preparation child MUST retain suiteChild's native deadline and
// sole-reaper ownership; a setup failure MUST stop the run before any verdict.
func (a *allTestsRun) prepareConntrack(environ []string) error {
	preparation := *a
	preparation.Timeout = conntrackSetupTimeout
	a.note(banner("guest conntrack preparation"))
	if code := preparation.suiteChild([]string{"sh", "-c", guestConntrackSetup}, environ, nil); code != 0 {
		return fmt.Errorf("qemu: conntrack preparation exited %d; the suites did not start. Fix: inspect the guest setup error above and repeat qemu all", code)
	}
	return nil
}
