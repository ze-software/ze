"""Machine state that is not a binary: a kernel tunable, a group, addresses.

None of these can be installed, so none of them fits the tool table. Each one
answers three questions with the same shape: what state is it in, what command
would change it, and can that command be run right now.
"""

from __future__ import annotations

import getpass
import grp
import os
import platform
import socket
from enum import Enum
from pathlib import Path

from le.console import echo
from le.process import Command, run_privileged

__all__ = [
    'KVM_GROUP',
    'Kvm',
    'Userns',
    'apply_kvm',
    'apply_loopback',
    'apply_userns',
    'kvm_commands',
    'kvm_state',
    'loopback_add_argv',
    'loopback_commands',
    'missing_loopback',
    'userns_commands',
    'userns_state',
]


def on_linux() -> bool:
    return platform.system() == 'Linux'


def on_darwin() -> bool:
    return platform.system() == 'Darwin'


# --- Unprivileged user namespaces -----------------------------------------
#
# Ubuntu 23.10+ ships kernel.apparmor_restrict_unprivileged_userns=1, which
# blocks the user-namespace sandbox Chrome relies on. The agent-browser web
# functional tests then cannot launch Chrome ("No usable sandbox!"), so the
# restriction must be lifted globally.
USERNS_SYSCTL = 'kernel.apparmor_restrict_unprivileged_userns'
USERNS_PROC = Path('/proc/sys/kernel/apparmor_restrict_unprivileged_userns')
USERNS_CONF = '/etc/sysctl.d/60-ze-userns.conf'


class Userns(Enum):
    """The unprivileged-userns restriction state.

    OK          allowed (value 0).
    RESTRICTED  blocked (value 1).
    NA          the kernel has no such knob, so there is nothing to do. A
                non-AppArmor host reaches this.
    """

    OK = 'ok'
    RESTRICTED = 'restricted'
    NA = 'na'


def userns_state() -> Userns:
    if not USERNS_PROC.exists():
        return Userns.NA
    try:
        return Userns.RESTRICTED if USERNS_PROC.read_text().strip() == '1' else Userns.OK
    except OSError:
        return Userns.NA


def userns_commands() -> list[tuple[Command, bytes | None, str | None]]:
    """The commands that lift the restriction persistently.

    The drop-in under /etc/sysctl.d survives a reboot; the `sysctl -w` applies
    the value to the running kernel. Both are needed: one without the other
    leaves the machine right now or right after the next boot, not both.
    """
    return [
        (
            ['tee', USERNS_CONF],
            f'{USERNS_SYSCTL} = 0\n'.encode(),
            f'echo "{USERNS_SYSCTL} = 0" | {{sudo}}tee {USERNS_CONF}',
        ),
        (['sysctl', '-w', f'{USERNS_SYSCTL}=0'], None, None),
    ]


def print_userns_fix() -> None:
    """Print the commands, for when root is out of reach."""
    echo(f'  Run: echo "{USERNS_SYSCTL} = 0" | sudo tee {USERNS_CONF}')
    echo(f'  Run: sudo sysctl -w {USERNS_SYSCTL}=0')


def apply_userns() -> bool:
    """Print, then run, the commands that lift the restriction.

    Returns True only when the restriction is actually cleared. On any failure
    the caller falls back to printing the manual commands.
    """
    for argv, stdin, shown in userns_commands():
        ok, detail = run_privileged(argv, stdin=stdin, shown=shown)
        if not ok:
            echo(f'  FAIL: {detail}')
            return False
    return userns_state() is Userns.OK


# --- KVM device access ----------------------------------------------------
#
# QEMU-backed evidence (appliance boot proofs, the ze-qemu-* targets) runs
# under KVM when it can. /dev/kvm is root:kvm 0660, so the invoking user must
# be in the kvm group; without it qemu does not fall back, it dies with "Could
# not access KVM kernel module: Permission denied" and the caller reports a
# timeout.
#
# Linux only. macOS has no /dev/kvm: QEMU uses the Apple hypervisor (hvf),
# which needs no group.
KVM_DEV = Path('/dev/kvm')
KVM_GROUP = 'kvm'


class Kvm(Enum):
    """Whether QEMU can use KVM as this user.

    OK             /dev/kvm is readable and writable in this process now.
    PENDING_LOGIN  the user IS in the kvm group but this session predates that;
                   group membership is fixed at login.
    NO_GROUP       the device exists and the user is not in the group.
    NA             no /dev/kvm at all (no hardware virt, or a VM without nested
                   virt). QEMU still runs under tcg, only slower, so there is
                   nothing to fix.
    """

    OK = 'ok'
    PENDING_LOGIN = 'pending-login'
    NO_GROUP = 'no-group'
    NA = 'na'


def in_kvm_group() -> bool:
    """True when the user is listed in the kvm group in the group database.

    Deliberately not an access check: after `usermod -aG` the database says yes
    while every already-running process still says no, and telling those two
    apart is the difference between "run this command" and "log back in".
    """
    try:
        return getpass.getuser() in grp.getgrnam(KVM_GROUP).gr_mem
    except KeyError:
        return False


def kvm_state() -> Kvm:
    if not KVM_DEV.exists():
        return Kvm.NA
    if os.access(KVM_DEV, os.R_OK | os.W_OK):
        return Kvm.OK
    return Kvm.PENDING_LOGIN if in_kvm_group() else Kvm.NO_GROUP


def kvm_commands() -> list[Command]:
    return [['usermod', '-aG', KVM_GROUP, getpass.getuser()]]


def print_kvm_fix() -> None:
    echo(f'  Run: sudo usermod -aG {KVM_GROUP} {getpass.getuser()}')
    echo(f"  Then log out and back in, or prefix a command with: sg {KVM_GROUP} -c '<command>'")


def apply_kvm() -> bool:
    """Add the invoking user to the kvm group.

    Returns True when the group database lists the user afterwards. That is NOT
    the same as usable: this process keeps the groups it started with, so the
    caller must still say to log back in.
    """
    for argv in kvm_commands():
        ok, detail = run_privileged(argv)
        if not ok:
            echo(f'  FAIL: {detail}')
            return False
    return in_kvm_group()


# --- Loopback addresses the functional suite binds ------------------------
#
# A `.ci` fixture with two BGP speakers gives each end its own address, and it
# has to: RFC 4271 Section 5.1.3 forbids a peer its own address as NEXT_HOP, so
# a session whose two ends share one address has every originated route
# withheld (`originatedNextHopIsPeerOwn`,
# internal/component/bgp/reactor/forward_next_hop.go).
#
# IPv4 has 127.0.0.0/8 to spend. Linux routes all of it to lo, so only macOS
# needs aliases, and only for the addresses the suite actually uses.
#
# IPv6 has exactly one loopback address, ::1, on every platform. A second one
# is real configuration, and fd00::2 is what the suite uses: fd00::/8 is
# unique-local (RFC 4193), never globally routable, so a fixture that leaks a
# packet toward it can never reach a real destination on a real network. A
# documentation prefix (2001:db8::/32) is globally scoped and would not carry
# that property.
#
# This is setup work rather than runner work because the runner cannot do it:
# SIOCAIFADDR_IN6 returns EPERM to an unprivileged process on darwin, and the
# Linux route needs CAP_NET_ADMIN, while the verify gate runs as an ordinary
# user (internal/test/runner/loopback.go reports the miss and names setup).
#
# Neither addition survives a reboot on either platform. That is deliberate:
# the persistent forms (a launchd plist, a netplan or systemd-networkd unit)
# edit files a developer's machine owns for other reasons. Setup is cheap to
# re-run, and --check says when it is needed.
LOOPBACK_IPV6 = 'fd00::2'

# 127.0.0.2 through 127.0.0.5: the addresses multi-peer fixtures bind today,
# plus the ones docs/guide/chaos-testing.md asks a human to add by hand for FRR
# and BIRD chaos runs (those daemons identify peers by source address, not by
# port).
LOOPBACK_IPV4_DARWIN = tuple(f'127.0.0.{i}' for i in range(2, 6))


def loopback_addresses() -> list[str]:
    """The addresses this host must carry for the functional suite to run.

    Linux is IPv6-only here: 127.0.0.0/8 already routes to lo, so an IPv4 alias
    would be work with no effect.
    """
    if on_darwin():
        return [*LOOPBACK_IPV4_DARWIN, LOOPBACK_IPV6]
    return [LOOPBACK_IPV6]


def loopback_bindable(addr: str) -> bool:
    """Whether a socket can bind `addr` right now.

    A bind, not a scan of the interface list, because a bind is what every
    fixture needs to succeed and the two answers differ: an IPv6 address is
    listed while duplicate-address detection still refuses it. The test runner
    decides the same way (`loopbackBindable`,
    internal/test/runner/loopback.go).
    """
    family = socket.AF_INET6 if ':' in addr else socket.AF_INET
    try:
        with socket.socket(family, socket.SOCK_STREAM) as sock:
            sock.bind((addr, 0))
    except OSError:
        return False
    return True


def missing_loopback() -> list[str]:
    """The subset of `loopback_addresses` this host does not carry."""
    return [addr for addr in loopback_addresses() if not loopback_bindable(addr)]


def loopback_add_argv(addr: str) -> Command:
    """The root command that puts `addr` on the loopback interface."""
    if on_darwin():
        if ':' in addr:
            return ['ifconfig', 'lo0', 'inet6', f'{addr}/128', 'alias']
        return ['ifconfig', 'lo0', 'alias', addr]
    # Linux reaches here for IPv6 only; /128 keeps it a host address, so no
    # route toward the rest of fd00::/8 is created.
    return ['ip', '-6', 'addr', 'add', f'{addr}/128', 'dev', 'lo']


def loopback_commands(missing: list[str]) -> list[Command]:
    return [loopback_add_argv(addr) for addr in missing]


def print_loopback_fix(missing: list[str]) -> None:
    for argv in loopback_commands(missing):
        echo(f'  Run: sudo {" ".join(argv)}')


def apply_loopback(missing: list[str]) -> bool:
    """Print, then run, the commands that add the missing addresses.

    Idempotent by construction: only addresses that failed the bind probe are
    passed in, so a re-run on a configured host runs nothing. Returns True only
    when every address binds afterwards.
    """
    for argv in loopback_commands(missing):
        ok, detail = run_privileged(argv)
        if not ok:
            echo(f'  FAIL: {detail}')
            return False
    return not missing_loopback()
