#!/usr/bin/env python3
"""In-VM driver for `make ze-netns-qemu-test` (Fix B, spec-netlink-ci-harness).

Runs INSIDE the QEMU Alpine VM as root, cwd=/workspace. Exercises the per-test
network-namespace launch mode end-to-end on a real Linux kernel: it setcaps the
cross-compiled ze binaries, runs a curated firewall subset under ZE_TEST_NETNS
(so `ze` is dropped to a normal user and programs nft inside a throwaway netns),
and asserts the host `nft list tables` is byte-identical before and after.

Why a curated subset and not `firewall --all`: 009-set-element-timeout crashes
the Alpine QEMU kernel. That is a pre-existing environment issue unrelated to
the netns launch mode and is triaged separately. The copp-* suites ARE included:
they configure only `control-plane-protection` (no firewall {} block), which
used to fail "firewall backend not loaded" until ApplyAll learned to load the
OS-default backend on demand -- this run is that fix's Linux regression guard.
004-cli-show
is ALSO included: it drives `ze cli` over the real SSH CLI path (a config-declared
SSH user + `ze init`-provisioned client credentials), which needs the daemon
built with ze_ssh (see the make target; ze init is already in ze_core).

It ALSO runs the policy-routing suite in full. 005-next-hop needs an interface on
the next-hop's subnet to exist inside the netns (enterTestNetns brings up only
loopback), which the test now provisions via option=netns-link -- this run is
that fix's real-kernel regression guard.

Every subset below selects by test NAME. A numeric nick is a load-order ordinal
over the alphabetically-sorted .ci glob (runner.GenerateNick, record.go, is a
bare counter; EncodingTests.Discover sorts the glob then adds in that order), so
adding or renaming any earlier file silently renumbers every test after it -- and
an in-range-but-shifted nick runs the WRONG test while still reporting green.
That had already happened here: firewall nick "17" resolved to
command-owner-firewall-root.ci, not to any 017-*.ci. Names exact-match
(indexRecordSelector, selection.go), so a named set stays stable, and
assert_named below refuses to run a set that has drifted. See ai/rules/testing.md
"A numeric id is a position, not an identity".

The 9p workspace mount is security_model=none (no xattr), so file capabilities
cannot be set there; ze is copied to a tmpfs dir first. That dir must be
world-traversable because the credential-dropped ze (uid 1000) execs it, and a
pinned ze.config.dir gives the daemon and any `ze cli` a shared writable state
path independent of the (VM-only) binary location.
"""

import os
import subprocess
import sys

ARCH = os.environ.get("QEMU_GOARCH") or (
    "arm64" if os.uname().machine in ("aarch64", "arm64") else "amd64"
)
CAPS = "cap_net_admin,cap_net_raw,cap_net_bind_service+ep"
CAPDIR = "/tmp/zebin"
STATE = "/tmp/zestate"


def _qemu_bin(env_key, name):
    """Path of a binary mk/test-integration.mk cross-compiled for the VM.

    Those are built as $(ZE_QEMU_BIN) / $(ZE_QEMU_STRIPPED_BIN) /
    $(ZE_QEMU_TEST_BIN), which sit in this session's own directory under an AI
    session ($(ZE_BIN_DIR), mk/session.mk). So `bin/<name>-linux-<arch>` is
    NOT the built path in general -- hardcoding it makes this script exec a file
    the make target never wrote. The target passes the real paths in through
    these variables; the literal remains the default for a standalone run.
    """
    return os.environ.get(env_key) or f"bin/{name}-linux-{ARCH}"


ZE_QEMU_BIN = _qemu_bin("ZE_QEMU_BIN", "ze")
ZE_QEMU_STRIPPED_BIN = _qemu_bin("ZE_QEMU_STRIPPED_BIN", "ze-stripped")
ZE_QEMU_TEST_BIN = _qemu_bin("ZE_QEMU_TEST_BIN", "ze-test")
# Confirmed host-safe green firewall subset under the netns launch mode: every
# test/firewall/*.ci except 009-set-element-timeout (see the module docstring --
# it crashes the Alpine QEMU kernel). The copp-* names exercise the standalone
# control-plane-protection path (no firewall {} block) that the ApplyAll
# on-demand-backend fix unblocks. ddos-local-withdraw drives the ddos-local
# responder (via the fakeddos injector) through the same on-demand backend to
# prove a cleared mitigation's ze_ddos-local table is swept.
# command-owner-firewall-root is an offline help-path test that entered this set
# as the numeric nick "17" (there is no 017-*.ci); it touches no nft state and is
# kept so the enrolled set is unchanged by the numeric-to-name conversion.
FIREWALL_IDS = [
    "001-boot-apply",
    "002-reload",
    "003-coexistence",
    "004-cli-show",
    "005-match-in-set-addr",
    "006-dscp-ipv6-rejected",
    "007-setdscp-inet",
    "008-match-in-set-port",
    "010-byte-rate-limit",
    "011-snat-addr-range",
    "012-icmp-type",
    "013-iface-wildcard",
    "014-nat-exclude",
    "015-masquerade-ports",
    "016-masquerade-flags",
    "command-owner-firewall-root",
    "copp-bgp",
    "copp-trusted",
    "copp-withdraw",
    "flush-persist",
    "flush-crash",
    "ddos-local-withdraw",
]

# Policy-routing suite in full, host-safe under the netns launch mode.
# 005-next-hop provisions eth1 via option=netns-link so its next-hop auto-route
# resolves inside the throwaway netns.
# 006-reload was excluded while it was believed to be a still-open
# reload-reconciliation failure. It is not: 94b07348d showed both of its verdict
# assertions matched the base-chain `policy accept` declaration rather than a rule
# line, so phase 1 was vacuous and phase 2 could never pass -- the reload had
# worked all along. Its config uses only accept/drop actions, so it programs no ip
# rule and asserts none (translate.go emits ipRuleSpec only for table/next-hop
# actions), which is what the old "stale ip rules after SIGHUP" rationale claimed.
# Its interface match lowers to nftables iifname, a string match that needs no
# link in the netns -- exactly like 002/003/004, which pass here without one.
POLICY_IDS = [
    "001-boot-apply",
    "002-set-table",
    "003-tcp-flags",
    "004-tcp-mss",
    "005-next-hop",
    "006-reload",
]

# OSPF interface subset that exercises the netns launch mode's uid-drop path for
# observer tests. Each provisions its dummy link(s) via option=netns-link, loads
# the iface backend on demand (EnsureBackend, v2 and v3), and its ze_api-using
# observer imports ze_api from the tmpfs workdir (the copyTestScripts fix). This
# is the gate for all three infra fixes.
#   ospf-nbma, ospf-ptmp, ospf-show                     (v2 raw-socket + IPv4 read)
#   ospf-multiaf, ospf-multiaf-reconcile, ospf-multiaf-show,
#   ospf-multiaf-v4-route                               (v3 AF engines, EnsureBackend v3)
#   ospf-instance-demux                                 (RFC 6549 repeated leaf-list
#      `instance-id 0; instance-id 5;`; guards the config-parser fix that makes
#      repeated leaf-list statements accumulate so both instances carry eth0)
OSPF_IDS = [
    "ospf-instance-demux",
    "ospf-multiaf",
    "ospf-multiaf-reconcile",
    "ospf-multiaf-show",
    "ospf-multiaf-v4-route",
    "ospf-nbma",
    "ospf-ptmp",
    "ospf-show",
]
# OSPFv3 subset (all boot a top-level IPv6 OSPFv3 config; the v6 engine opens a raw
# proto-89 socket on the netns-provisioned link, IPv6 link-local auto on link-up):
#   ospfv3-vlink  virtual-link wiring survives boot
#   ospfv3-nbma   NBMA interface (network-type nbma + static neighbor) boots clean
#   ospfv3-ptmp   point-to-multipoint interface boots clean
OSPFV3_IDS = ["ospfv3-vlink", "ospfv3-nbma", "ospfv3-ptmp"]


# PPPoE access concentrator (test/pppoe/). Every test provisions a veth PAIR via
# option=netns-link:peer=, so the netns launch mode is what makes them runnable
# at all. They are NOT in DEFAULT_SUITES because handlePADR opens an
# AF_PPPOX/PX_PROTO_OE socket before it sends PADS
# (internal/component/l2tp/pppoe/server.go) and the stock Alpine kernel has no
# CONFIG_PPPOE: `make ze-qemu-pppoe-test` selects them and boots ze's runtime
# kernel, which does.
PPPOE_IDS = ["pppoe-basic", "pppoe-concurrent-l2tp", "pppoe-vlan"]

SUITE_REGISTRY = {
    "firewall": FIREWALL_IDS,
    "policy": POLICY_IDS,
    "ospf": OSPF_IDS,
    "ospfv3": OSPFV3_IDS,
    "pppoe": PPPOE_IDS,
}

# What `make ze-netns-qemu-test` runs on the stock Alpine kernel. A caller that
# needs a different set names it in ZE_NETNS_QEMU_SUITES, which is how a suite
# with a kernel requirement of its own gets its own target rather than becoming
# everybody's precondition.
DEFAULT_SUITES = ("firewall", "policy", "ospf", "ospfv3")


def _selected_suites():
    names = os.environ.get("ZE_NETNS_QEMU_SUITES", "").split() or list(DEFAULT_SUITES)
    unknown = [n for n in names if n not in SUITE_REGISTRY]
    if unknown:
        sys.exit(
            f"ZE_NETNS_QEMU_SUITES names {unknown} which this script has no id set for; "
            f"known suites: {sorted(SUITE_REGISTRY)}"
        )
    return tuple((n, SUITE_REGISTRY[n]) for n in names)


SUITES = _selected_suites()


def assert_named(suite, ids):
    """Refuse to run a subset whose selectors are not real test names.

    ze-test's positional selector matches a record's Nick, Name, or CIFile
    (indexRecordSelector, internal/test/runner/selection.go), and a missing one
    makes the whole run exit non-zero ("test %q not found" -> RunCISubcommand
    returns 1), so an unmatched NAME is already fail-closed at the runner. This
    guard exists for the failure the runner CANNOT see: a numeric selector, which
    it resolves happily as a load-order ordinal and which therefore silently
    re-points to a different test the moment an alphabetically-earlier .ci file is
    added, renamed, or deleted. That is a gate reporting success for tests it
    never ran, so it fails the run here rather than shrinking or shifting the set
    in silence (ai/rules/evidence.md, ai/rules/testing.md).

    Checked before any test runs so the diagnosis is not buried under suite
    output. The suite directory is test/<suite>/ for all four (registerCIRoot,
    internal/test/cli/register.go) and this script runs at the repo root.
    """
    bad = []
    for name in ids:
        if name.isdigit():
            bad.append(f"{name!r}: numeric nick (a position, not an identity)")
        elif not os.path.isfile(os.path.join("test", suite, f"{name}.ci")):
            bad.append(f"{name!r}: no test/{suite}/{name}.ci")
    if bad:
        print(f"SELECTION FAIL: {suite} subset does not name real tests:", flush=True)
        for line in bad:
            print(f"  - {line}", flush=True)
        sys.exit(f"{suite} subset invalid; refusing to run a silently smaller gate")
    print(f"selection OK: {suite} names {len(ids)} test(s)", flush=True)


def sh(cmd, **kw):
    print(f"+ {cmd}", flush=True)
    return subprocess.run(cmd, shell=True, **kw)


def setcap_binaries():
    os.makedirs(CAPDIR, exist_ok=True)
    for name, src in (("ze", ZE_QEMU_BIN), ("ze-stripped", ZE_QEMU_STRIPPED_BIN)):
        dst = f"{CAPDIR}/{name}"
        if sh(f"cp {src} {dst} && chmod 0755 {dst}").returncode != 0:
            sys.exit(f"copy {src} failed")
        if sh(f"setcap {CAPS} {dst}").returncode != 0:
            sys.exit(f"setcap {dst} failed (no xattr support?)")
    sh(f"getcap {CAPDIR}/ze")


def prepare_state():
    sh(
        f"rm -rf {STATE} && mkdir -p {STATE} && chown 1000:1000 {STATE} && chmod 0755 {STATE}"
    )
    prepare_dev_ppp()


def prepare_dev_ppp():
    """Let the credential-dropped ze open /dev/ppp.

    A PPPoE AC builds the kernel PPP channel through /dev/ppp after PADR and
    before PADS (ppp.DevPPPSetup, called from handlePADR). The node is root-owned
    0600, and DAC is not a capability ze holds: the netns mode grants
    cap_net_admin, cap_net_raw and cap_net_bind_service, none of which override
    file permissions. Without this every PADR dies "open /dev/ppp: permission
    denied" and no PADS is ever sent.

    Widening the mode here, on a throwaway VM, is the honest fix. Adding
    cap_dac_override to CAPS would let every suite in every netns run open any
    root-owned file, which is a much larger grant for one device node. A host
    that has no /dev/ppp (no CONFIG_PPP) is left alone: the suites that need it
    fail loudly on their own assertion rather than on a setup step.

    "Throwaway" is the whole justification, so it is CHECKED and not merely
    asserted in this docstring. The VM boots fresh from an ISO every run and its
    /dev is gone at poweroff; a developer's Linux box keeps the widened node, and
    a world-writable /dev/ppp lets any local user open PPP channels. Run outside
    the VM this refuses rather than widening (`ai/rules/evidence.md`: fail closed
    or say something).
    """
    if not os.path.exists("/dev/ppp"):
        return
    if not in_throwaway_vm():
        sys.exit(
            "refusing to chmod 0666 /dev/ppp: this is not the QEMU evidence VM, and the "
            "widened node would persist for every local user. Run: make ze-qemu-pppoe-test"
        )
    sh("chmod 0666 /dev/ppp")


def in_throwaway_vm():
    """True only inside the Alpine evidence VM qemu-run.py boots.

    The repo reaches that VM as a virtio-9p share mounted at /workspace
    (`-virtfs ... mount_tag=workspace`, scripts/evidence/qemu-run.py), which this
    script's own cwd depends on. No developer host mounts its checkout over 9p,
    so the mount is a property of the environment rather than a variable a caller
    can set, which an env-var marker would not be.
    """
    if sys.platform != "linux":
        return False
    try:
        with open("/proc/mounts", encoding="utf-8") as f:
            mounts = f.read().splitlines()
    except OSError:
        return False
    for line in mounts:
        fields = line.split()
        if len(fields) >= 3 and fields[1] == "/workspace" and fields[2] == "9p":
            return True
    return False


def host_nft():
    return subprocess.run(
        ["nft", "list", "tables"], capture_output=True, text=True
    ).stdout


def run_suite(suite, ids):
    env = {
        **os.environ,
        "ZE_TEST_NO_BUILD": "1",
        "ZE_QEMU": "1",
        "ZE_BIN": f"{CAPDIR}/ze",
        "ZE_STRIPPED_BIN": f"{CAPDIR}/ze-stripped",
        "ZE_TEST_BIN": ZE_QEMU_TEST_BIN,
        "ZE_TEST_NETNS": "1",
        "ZE_TEST_UID": "1000",
        "ZE_TEST_GID": "1000",
        "ze.config.dir": STATE,
    }
    cmd = [ZE_QEMU_TEST_BIN, suite, "-p", "1", *ids]
    print(f"+ {' '.join(cmd)}", flush=True)
    return subprocess.run(cmd, env=env).returncode


def main():
    # Before anything else: a drifted selector must stop the run, not shrink it.
    for suite, ids in SUITES:
        assert_named(suite, ids)

    setcap_binaries()
    prepare_state()

    # Regression guard for the config-file chown fix: a hardened umask makes the
    # runner's root-created config files 0600, so a credential-dropped ze can only
    # read them if the runner chowns them to the target uid (not merely relies on
    # world-read). Set it AFTER the CAPDIR/STATE setup (which must stay
    # world-traversable so the uid-dropped ze can exec the binary) so only the
    # ze-test run inherits it. This proves the chown fix holds under umask 077.
    os.umask(0o077)

    before = host_nft()
    # Driven off SUITES so a suite can never be enrolled in the guard/selection
    # above yet dropped from the run or from the verdict below.
    codes = [(suite, run_suite(suite, ids)) for suite, ids in SUITES]
    after = host_nft()

    ok = True
    for suite, rc in codes:
        if rc != 0:
            print(f"FAIL: {suite} netns subset returned {rc}")
            ok = False
    if before != after:
        print("HOST-SAFETY FAIL: host nft tables changed during the netns run")
        print(f"--- before ---\n{before}\n--- after ---\n{after}")
        ok = False
    else:
        print("HOST-SAFE: host nft tables unchanged")

    print("netns-qemu:", "PASS" if ok else "FAIL")
    return 0 if ok else 1


if __name__ == "__main__":
    sys.exit(main())
