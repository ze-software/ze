#!/usr/bin/env python3
"""In-VM driver for `make ze-netns-qemu-test` (Fix B, spec-netlink-ci-harness).

Runs INSIDE the QEMU Alpine VM as root, cwd=/workspace. Exercises the per-test
network-namespace launch mode end-to-end on a real Linux kernel: it setcaps the
cross-compiled ze binaries, runs a curated firewall subset under ZE_TEST_NETNS
(so `ze` is dropped to a normal user and programs nft inside a throwaway netns),
and asserts the host `nft list tables` is byte-identical before and after.

Why a curated subset and not `firewall --all`: 009-set-element-timeout crashes
the Alpine QEMU kernel. That is a pre-existing environment issue unrelated to
the netns launch mode and is triaged separately -- see
plan/learned/1112-netlink-ci-harness.md. The copp-* suites ARE included: they
configure only `control-plane-protection` (no firewall {} block), which used to
fail "firewall backend not loaded" until ApplyAll learned to load the OS-default
backend on demand -- this run is that fix's Linux regression guard. 004-cli-show
is ALSO included: it drives `ze cli` over the real SSH CLI path (a config-declared
SSH user + `ze init`-provisioned client credentials), which needs the daemon
built with ze_ssh (see the make target; ze init is already in ze_core).

It ALSO runs a policy-routing subset. 005-next-hop needs an interface on the
next-hop's subnet to exist inside the netns (enterTestNetns brings up only
loopback), which the test now provisions via option=netns-link -- this run is
that fix's real-kernel regression guard. 006-reload is EXCLUDED: it is a
separate, still-open reload-reconciliation failure (stale ip rules after SIGHUP),
not the netns-interface bug, and is tracked in plan/handover/21-netlink-suite-recovery.md.

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
    $(ZE_QEMU_TEST_BIN), whose file names carry this session's id under an AI
    session ($(ZE_BIN_SUFFIX), mk/session.mk). So `bin/<name>-linux-<arch>` is
    NOT the built path in general -- hardcoding it makes this script exec a file
    the make target never wrote. The target passes the real paths in through
    these variables; the literal remains the default for a standalone run.
    """
    return os.environ.get(env_key) or f"bin/{name}-linux-{ARCH}"


ZE_QEMU_BIN = _qemu_bin("ZE_QEMU_BIN", "ze")
ZE_QEMU_STRIPPED_BIN = _qemu_bin("ZE_QEMU_STRIPPED_BIN", "ze-stripped")
ZE_QEMU_TEST_BIN = _qemu_bin("ZE_QEMU_TEST_BIN", "ze-test")
# Confirmed host-safe green firewall subset under the netns launch mode.
# Numeric IDs map to NNN-*.ci; the copp-* names select the CoPP suites, which
# exercise the standalone control-plane-protection path (no firewall {} block)
# that the ApplyAll on-demand-backend fix unblocks. ddos-local-withdraw drives the
# ddos-local responder (via the fakeddos injector) through the same on-demand
# backend to prove a cleared mitigation's ze_ddos-local table is swept.
FIREWALL_IDS = [
    "1",
    "2",
    "3",
    "4",
    "5",
    "6",
    "7",
    "8",
    "10",
    "11",
    "12",
    "13",
    "14",
    "15",
    "16",
    "17",
    "copp-bgp",
    "copp-trusted",
    "copp-withdraw",
    "flush-persist",
    "flush-crash",
    "ddos-local-withdraw",
]

# Policy-routing subset, host-safe under the netns launch mode. 006-reload is
# excluded (separate open reload-reconciliation failure, not the netns-interface
# bug this run guards). 005-next-hop provisions eth1 via option=netns-link so its
# next-hop auto-route resolves inside the throwaway netns.
POLICY_IDS = ["1", "2", "3", "4", "5"]

# OSPF interface subset that exercises the netns launch mode's uid-drop path for
# observer tests. Each provisions its dummy link(s) via option=netns-link, loads
# the iface backend on demand (EnsureBackend, v2 and v3), and its ze_api-using
# observer imports ze_api from the tmpfs workdir (the copyTestScripts fix). This
# is the gate for all three infra fixes. Selected by test NAME, not numeric nick:
# nicks are load-order ordinals over the alphabetically-sorted glob, so adding or
# renaming any earlier ospf/*.ci silently renumbers -- and an in-range-but-shifted
# nick runs the WRONG test and still reports green. Names exact-match (selection.go
# indexRecordSelector) so the set stays stable.
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
    rc_firewall = run_suite("firewall", FIREWALL_IDS)
    rc_policy = run_suite("policy", POLICY_IDS)
    rc_ospf = run_suite("ospf", OSPF_IDS)
    rc_ospfv3 = run_suite("ospfv3", OSPFV3_IDS)
    after = host_nft()

    ok = True
    if rc_firewall != 0:
        print(f"FAIL: firewall netns subset returned {rc_firewall}")
        ok = False
    if rc_policy != 0:
        print(f"FAIL: policy netns subset returned {rc_policy}")
        ok = False
    if rc_ospf != 0:
        print(f"FAIL: ospf netns subset returned {rc_ospf}")
        ok = False
    if rc_ospfv3 != 0:
        print(f"FAIL: ospfv3 netns subset returned {rc_ospfv3}")
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
