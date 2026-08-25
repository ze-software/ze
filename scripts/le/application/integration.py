"""Integration, interop, stress, live and deployment tests.

Ported from mk/test-integration.mk. Twenty-one targets moved; that file's
header lists the ones that stayed and why.

    ./le integration --list                            what each one needs
    ./le integration ze-interop-test                   one of them
    INTEROP_SCENARIO=bgp-ebgp-ipv4-frr ./le integration ze-interop-test

Every gate here needs infrastructure the machine may not have: Docker, a
network namespace, CAP_NET_ADMIN, root, QEMU or internet access. None of them
is part of `make ze-precommit-verify`, and that is by design rather than by
oversight.

**This area names no aggregate run, and `action` refuses one.** Every other
area runs its whole set when given no gate name. Here the set is hours of
Docker, root and QEMU work, so the convenient spelling would be the most
expensive one, and no Make target ever meant "all of these".

**The QEMU kernel reasoning stayed in Make, all of it.** Thirteen targets boot
ze's own runtime kernel through `$(ze-qemu-kernel-guard)`, a shell program that
compares tmp/kernel/vmlinuz against the arch-and-config-keyed cache entry
ze-host names, and `scripts/evidence/qemu_kernel_wiring_test.go` derives its
population from those recipes. The guard, the `: ze-host-build` prerequisite it
needs to name its own cause, the cross-build define the five VM targets share
and the comments explaining each are one unit. Splitting the comments away from
the recipes they judge is how that reasoning went stale the first time, so
nothing about it moved here. `ze-qemu-vpp-hugepages-test` is the one QEMU
target in this module, and it is here because it runs no VM of its own: it
drives `scripts/evidence/effective-vpp-hugepages-qemu.py`, which builds and
boots an appliance image itself and takes no staged kernel.

INTEROP_SCENARIO and IPSEC_INTEROP_SCENARIO moved with their targets, from
`$(VAR)` to the environment. A variable set on the make command line reaches
the recipe's environment, so `make ze-interop-test INTEROP_SCENARIO=x` and
`INTEROP_SCENARIO=x ./le integration ze-interop-test` build the same argv.
VERBOSE and SESSION_TIMEOUT reach ze-stress-bird-test the same way, and they
are passed THROUGH sudo as `VAR=value` arguments because sudo scrubs the
environment it is given.
"""

from __future__ import annotations

import argparse
import os
import sys
from collections.abc import Sequence

from le import gateapp
from le.console import echo
from le.devtools.gate import Gate, GateSet
from le.devtools.toolchain import toolchain

__all__ = ['GATES', 'Options', 'action', 'add_arguments', 'main', 'options']


def _scenario(script: str, variable: str) -> tuple[str, ...]:
    """A lab runner, with the one scenario name its variable selects.

    Empty selects every scenario, which is what an unset make variable did:
    `python3 test/interop/run.py $(INTEROP_SCENARIO)` expanded to the bare
    command and the runner's own default took over.
    """
    chosen = os.environ.get(variable, '').strip()
    return ('python3', script, chosen) if chosen else ('python3', script)


def _integration(*packages: str, timeout: str = '120s') -> tuple[str, ...]:
    """A kernel-facing integration suite, argument for argument.

    `-tags integration` alone, never the feature set: these packages are
    selected by their own `//go:build integration` guard, and the daemon
    features have nothing to say about a netlink call.
    """
    return (
        'go',
        'test',
        '-tags',
        'integration',
        '-count=1',
        '-race',
        '-timeout',
        *(timeout,),
        *packages,
    )


def _evidence(script: str) -> tuple[str, ...]:
    """One deployment-evidence driver under scripts/evidence/."""
    return ('python3', f'scripts/evidence/{script}')


def _sudo_stress(scenario: str) -> tuple[str, ...]:
    """A stress scenario as root, carrying VERBOSE and SESSION_TIMEOUT through sudo.

    The two names are passed as sudo's own `VAR=value` arguments rather than
    exported, because sudo does not forward the environment it is handed. They
    are spelled even when empty, which is what the Make recipe did: `sudo
    VERBOSE= SESSION_TIMEOUT= python3 ...` is the expansion of an unset pair.
    """
    return (
        'sudo',
        f'VERBOSE={os.environ.get("VERBOSE", "")}',
        f'SESSION_TIMEOUT={os.environ.get("SESSION_TIMEOUT", "")}',
        'python3',
        'test/stress/run.py',
        scenario,
    )


GATES = GateSet(
    area='integration',
    gates=(
        # ── Interop ────────────────────────────────────────────────────────
        Gate(
            name='ze-interop-test',
            argv=_scenario('test/interop/run.py', 'INTEROP_SCENARIO'),
            why=(
                'BGP interop against the FRR, BIRD and GoBGP containers, every scenario'
                ' under test/interop/scenarios/. Needs Docker. INTEROP_SCENARIO=<name>'
                ' runs one of them'
            ),
        ),
        Gate(
            name='ze-interop-ipsec-test',
            argv=_scenario('test/interop-ipsec/run.py', 'IPSEC_INTEROP_SCENARIO'),
            why=(
                'IKEv2/IPsec interop against strongSwan. Needs Docker and privileged'
                ' containers. IPSEC_INTEROP_SCENARIO=<name> runs one scenario'
            ),
        ),
        # ── Stress ─────────────────────────────────────────────────────────
        Gate(
            name='ze-stress-bird-test',
            argv=_sudo_stress('04-bulk-ipv4-bird'),
            why=(
                'the BIRD baseline the ze bulk-IPv4 stress numbers are read against.'
                ' Needs root, bird2 and network namespaces'
            ),
        ),
        Gate(
            name='ze-stress-web-test',
            argv=(
                'go',
                'test',
                '-tags',
                'ze_core stress',
                '-race',
                '-count=1',
                '-timeout',
                '300s',
                './internal/component/web/',
                '-run',
                'TestWebConcurrentEditStress',
                '-v',
            ),
            why=(
                '50 or more concurrent editor sessions against the web UI,'
                ' race-instrumented. Evidence tier: the `stress` build tag keeps it out'
                ' of ze-precommit-verify (R-6)'
            ),
        ),
        Gate(
            name='ze-stress-fleet-test',
            argv=(
                'go',
                'test',
                '-tags',
                'ze_core fleetperf',
                '-count=1',
                '-timeout',
                '300s',
                './cmd/ze/hub/',
                '-run',
                'TestFleetManyClientsPerf',
                '-v',
            ),
            why=(
                '128 managed clients against a real hub listener. Evidence tier: the'
                ' `fleetperf` build tag keeps it out of ze-precommit-verify (R-6)'
            ),
        ),
        # ── Live ───────────────────────────────────────────────────────────
        Gate(
            name='ze-live-rpki-test',
            argv=(
                'go',
                'test',
                '-v',
                '-tags',
                'live',
                '-timeout',
                '180s',
                '-count=1',
                './internal/component/bgp/plugins/rpki/...',
                '-run',
                'TestLive',
            ),
            why='the RPKI validator against a real cache. Needs Docker and internet access',
        ),
        # ── Integration (network namespace) ─────────────────────────────────
        Gate(
            name='ze-integration-iface-test',
            argv=_integration('./internal/component/iface/...'),
            why=(
                'the iface component against a real kernel: netlink link, address and'
                ' route programming. Needs CAP_NET_ADMIN'
            ),
        ),
        Gate(
            name='ze-integration-fib-test',
            argv=_integration('./internal/plugins/fib/kernel/...'),
            why=(
                'the kernel FIB backend: what a route looks like once netlink has it.'
                ' Needs CAP_NET_ADMIN'
            ),
        ),
        Gate(
            name='ze-integration-firewall-test',
            argv=_integration('./internal/plugins/firewall/nft/...'),
            why=('the nft firewall backend against a real nftables ruleset. Needs CAP_NET_ADMIN'),
        ),
        Gate(
            name='ze-integration-traffic-test',
            argv=_integration('./internal/plugins/traffic/netlink/...'),
            why=(
                'the traffic-control netlink backend: qdisc and filter programming.'
                ' Needs CAP_NET_ADMIN'
            ),
        ),
        Gate(
            name='ze-integration-gtsm-test',
            argv=_integration(
                './internal/core/network/...', './internal/component/bgp/reactor/...'
            ),
            why=(
                'BGP GTSM and TTL-security, which live in a socket option only a Linux'
                ' kernel can answer for'
            ),
        ),
        Gate(
            name='ze-integration-as112-test',
            argv=_integration('./internal/plugins/as112/...', timeout='60s'),
            why=(
                'the AS112 plugin serving DNS on privileged port 53. Needs'
                ' CAP_NET_BIND_SERVICE or root'
            ),
        ),
        # ── Deployment evidence ─────────────────────────────────────────────
        Gate(
            name='ze-deployment-vpp-test',
            argv=_evidence('effective-vpp.py'),
            why=(
                'ze driving a real VPP daemon, not a fake channel. Needs Docker and a'
                ' privileged container'
            ),
        ),
        Gate(
            name='ze-deployment-vpp-iface-test',
            argv=_evidence('effective-vpp-iface.py'),
            why=(
                'the VPP interface features against a real daemon: tunnels, mirror,'
                ' wireguard, LCP. Needs Docker and a privileged container'
            ),
        ),
        Gate(
            name='ze-evidence-release-candidate-check',
            argv=('bash', 'scripts/evidence/effective-verify.sh'),
            why=(
                'the release-candidate run: the verify gate over a clean checkout in a'
                ' container, so nothing in the developer tree can make it pass. Needs'
                ' Docker and a clean worktree'
            ),
        ),
        Gate(
            name='ze-deployment-l2tp-test',
            argv=_evidence('effective-l2tp-peer.py'),
            why=(
                'the L2TP control session against an external peer. Needs Docker and a'
                ' privileged container'
            ),
        ),
        Gate(
            name='ze-deployment-l2tp-ppp-test',
            argv=_evidence('effective-l2tp-ppp.py'),
            why=(
                'the full L2TP PPP/NCP path on the host. Needs Linux root, xl2tpd, pppd,'
                ' ping and PPPoL2TP kernel support'
            ),
        ),
        Gate(
            name='ze-deployment-docker-l2tp-ppp-test',
            argv=('python3', 'test/interop-l2tp/run.py'),
            why=(
                'the same L2TP PPP/NCP path in a peer-isolated Docker lab. Needs'
                ' PPPoL2TP support in the Docker host kernel'
            ),
        ),
        Gate(
            name='ze-deployment-docker-pppoe-accel-test',
            argv=('python3', 'test/interop-pppoe/run.py'),
            why=(
                "Ze's PPPoE client against a real accel-ppp access concentrator in a"
                ' Docker lab. Needs PPPoE support in the Docker host kernel'
            ),
        ),
        Gate(
            name='ze-deployment-gokrazy-l2tp-ppp-test',
            argv=_evidence('effective-gokrazy-l2tp-ppp.py'),
            why=(
                'L2TP PPP/NCP proven on the gokrazy appliance image rather than on a dev'
                ' host. Needs Linux root, QEMU, xl2tpd, pppd and PPPoL2TP support'
            ),
        ),
        # ── QEMU (the one target that boots no VM of its own) ───────────────
        Gate(
            name='ze-qemu-vpp-hugepages-test',
            argv=_evidence('effective-vpp-hugepages-qemu.py'),
            why=(
                'boot-time hugepage reservation, end to end: build an appliance carrying'
                ' image.hugepages, boot it, then assert `show host kernel` and `show host'
                ' memory` over the Ze CLI. Self-skips when qemu, sshpass, e2fsprogs or go'
                ' are absent; on Linux it needs membership of the kvm group'
                ' (make ze-dev-setup checks it as kvm-access)'
            ),
        ),
    ),
)


def _environment(gate: Gate) -> dict[str, str]:
    """The environment one gate runs under, read off the command it runs.

    Derived rather than declared beside the gate, so a suite that stops being a
    race run stops asking for cgo with no second edit. GOMAXPROCS is
    deliberately absent: the Makefile set it inside GO_TEST and GO_TEST_RACE,
    and no recipe in this file used either one.
    """
    return toolchain().environment(cgo='-race' in gate.argv)


def add_arguments(parser: argparse.ArgumentParser) -> None:
    gateapp.add_arguments(parser, GATES)


def options(namespace: argparse.Namespace) -> gateapp.Options:
    return gateapp.options(namespace)


def action(opts: gateapp.Options) -> int:
    """Run what the options select, and refuse the bare area run.

    Twenty-one suites needing Docker, root, a namespace and QEMU is not
    something a person types by accident and waits out. Name the gate.
    """
    if not opts.names and not opts.listing:
        echo('integration has no aggregate run: name the gate you want.')
        echo(f'  {", ".join(GATES.names())}')
        return 2
    return gateapp.action(opts, GATES, env=_environment)


def main(argv: Sequence[str] | None = None) -> int:
    return gateapp.main(argv, GATES, __doc__, run=action)


Options = gateapp.Options

if __name__ == '__main__':
    sys.exit(main())
