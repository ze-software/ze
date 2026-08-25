"""Install and verify every tool a Ze dev or test workflow needs.

The dev-setup program. It replaced the `ze-dev-setup` Makefile target and the
script behind it, both of which are gone. Runs standalone:

    PYTHONPATH=scripts python3 -m le.application.setup --check

or through the dispatcher, which calls `action` below with the same options:

    ./le setup --check

Two modes, and the rule that binds them: **they must agree**. A probe-only run
and an install run ask the same questions of the same machine and must reach
the same verdict about what is missing. The shell version let them drift --
install mode reported "Setup complete" with exit 0 on a box where `--check`
exited 1 -- because each branch printed a label and appended to a list by hand.
Here every step returns one `Outcome` carrying both, and the verdict is derived
from the collected outcomes rather than accumulated beside them.
"""

from __future__ import annotations

import argparse
import sys
from collections.abc import Sequence
from dataclasses import dataclass

from le.console import Outcome, Report, State, echo
from le.devtools import system
from le.devtools.editor import LSP_PLUGINS, LspPlugin, missing_lsp_plugins
from le.devtools.install import Installer, detect_package_manager, vendor_go_deps
from le.devtools.probes import probe
from le.devtools.servers import Answer, Health, gopls_health, pyright_health
from le.devtools.tools import ALL_TOOLS, PackageManager, Tool

__all__ = ['Options', 'action', 'add_arguments', 'main', 'options']


@dataclass(frozen=True)
class Options:
    """Everything `action` needs, and nothing about how it was asked for."""

    check: bool = False
    vendor: bool = True


def add_arguments(parser: argparse.ArgumentParser) -> None:
    """Declare the flags. Called by the dispatcher and by `main` alike."""
    parser.add_argument(
        '--check',
        action='store_true',
        help='probe only; change nothing, and exit nonzero if a required tool is missing',
    )
    parser.add_argument(
        '--no-vendor',
        dest='vendor',
        action='store_false',
        help='skip `go mod tidy && go mod vendor` at the end of an install run',
    )


def options(namespace: argparse.Namespace) -> Options:
    """Turn the parsed namespace into the typed value `action` takes.

    The one place argparse's untyped namespace becomes a real type. Everything
    downstream of it is checked.
    """
    return Options(check=bool(namespace.check), vendor=bool(namespace.vendor))


def action(opts: Options) -> int:
    """Probe the machine, install what is missing, report. Returns the exit code."""
    manager = detect_package_manager()
    if manager is None:
        echo('Unsupported platform: no brew (macOS) or apt (Linux) found.')
        _print_manual_list()
        return 1

    echo(f'Ze dev setup (package manager: {manager.value})\n')

    report = Report()
    installer = None if opts.check else Installer(manager)

    for tool in ALL_TOOLS:
        report.add(_visit_tool(tool, manager, installer))

    # Not binaries but behaviours: a language server on PATH is not a working
    # one, and every call fails the same silent way when it is not. Both modes
    # run these -- an install that leaves a mute server is not a setup.
    report.add(_visit_server('gopls-answers', gopls_health()))
    report.add(_visit_server('pyright-answers', pyright_health()))

    # A server that runs and answers is still unreachable if the harness was
    # never told it exists. This is the check that would have caught Python
    # being unanswerable here while Go worked, with both binaries installed.
    for plugin in LSP_PLUGINS:
        report.add(_visit_lsp_plugin(plugin))

    # Machine state that cannot be installed.
    if system.on_linux():
        report.add(_visit_userns(opts))
        report.add(_visit_kvm(opts))
    report.add(_visit_loopback(opts))

    if opts.check:
        return report.check_verdict()

    if opts.vendor:
        echo()
        vendor_go_deps()

    code = report.summarise()
    if code == 0:
        echo('Verify with: make ze-smoke-verify')
    return code


# --- One step per thing the machine must have -----------------------------


def _visit_tool(tool: Tool, manager: PackageManager, installer: Installer | None) -> Outcome:
    """Probe one tool, install it when asked to, and say what happened."""
    if probe(tool):
        return Outcome(tool.name, State.PRESENT)

    if not tool.installable_by(manager):
        return Outcome(tool.name, State.SKIPPED, tool.note or 'no package for this platform')

    if installer is None:
        if tool.required:
            return Outcome(tool.name, State.MISSING, 'REQUIRED')
        return Outcome(tool.name, State.SKIPPED, 'optional, and not installed')

    if not installer.install(tool):
        # apt has already printed why, and the command to run.
        if manager is PackageManager.APT and tool.apt is not None:
            return Outcome(tool.name, State.PENDING, 'not installed')
        if tool.required:
            return Outcome(tool.name, State.MISSING, 'required')
        return Outcome(tool.name, State.SKIPPED, 'optional')

    if probe(tool):
        return Outcome(tool.name, State.INSTALLED)

    # The installer succeeded and the probe still cannot find it, so the tool
    # is on the disk and unusable. This used to count as [installed] and the
    # run ended "Setup complete" with exit 0, while --check on the same box
    # exited 1: the two modes disagreed permanently, and install mode was the
    # one reading green on absence. pipx reaches it every time on a fresh
    # Debian, where ~/.local/bin is on PATH only if it existed at login, and
    # `go install` reaches it through ~/go/bin.
    echo(f'    add {_where_it_landed(tool)}, then re-run')
    return Outcome(tool.name, State.PENDING, 'installed, not on PATH')


def _where_it_landed(tool: Tool) -> str:
    """The directory an install put the tool in, named for a PATH fix."""
    if tool.pipx_install:
        return '~/.local/bin, which pipx uses; run `pipx ensurepath`'
    if tool.go_install:
        return '~/go/bin, which `go install` uses'
    return "the package manager's bin directory"


def _visit_server(name: str, answer: Answer) -> Outcome:
    """Turn a language-server answer into an outcome.

    ABSENT is SKIPPED rather than MISSING: the tool row above installs the
    binary, so reporting it twice would make one missing server two failures.
    BROKEN is MISSING, because installing it again does not repair it and
    somebody has to look.
    """
    if answer.health is Health.OK:
        return Outcome(name, State.PRESENT, answer.detail)
    if answer.health is Health.NA:
        return Outcome(name, State.PRESENT, f'n/a: {answer.detail}')
    if answer.health is Health.ABSENT:
        return Outcome(name, State.SKIPPED, answer.detail)
    return Outcome(name, State.MISSING, answer.detail)


def _visit_lsp_plugin(plugin: LspPlugin) -> Outcome:
    """Whether the harness can reach the language server for these file types.

    PENDING rather than MISSING when it is absent, because nothing this program
    runs can fix it: `claude plugin ...` does not return from inside a session.
    A human runs one command, and PENDING is the state that means exactly that
    (`le/console.py`). It still fails the run, which is the point -- the silent
    version of this cost weeks of whole-file reads.
    """
    name = f'{plugin.plugin}-installed'
    if plugin not in missing_lsp_plugins():
        return Outcome(name, State.PRESENT, ' '.join(plugin.extensions))

    echo(f'  Run: {plugin.install_command}')
    echo(f'    {plugin.why}')
    return Outcome(name, State.PENDING, f'the LSP tool refuses {", ".join(plugin.extensions)}')


def _visit_userns(opts: Options) -> Outcome:
    name = 'userns-unrestricted'
    state = system.userns_state()
    if state is system.Userns.OK:
        return Outcome(name, State.PRESENT)
    if state is system.Userns.NA:
        return Outcome(name, State.PRESENT, 'n/a: no apparmor userns knob')
    if opts.check:
        return Outcome(name, State.MISSING, 'REQUIRED')
    if system.apply_userns():
        return Outcome(name, State.INSTALLED)
    echo('  Could not apply automatically; run manually:')
    system.print_userns_fix()
    return Outcome(name, State.PENDING, 'restriction still in place')


def _visit_kvm(opts: Options) -> Outcome:
    name = 'kvm-access'
    state = system.kvm_state()
    if state is system.Kvm.OK:
        return Outcome(name, State.PRESENT)
    if state is system.Kvm.NA:
        return Outcome(name, State.PRESENT, 'n/a: no /dev/kvm; QEMU uses tcg')
    if state is system.Kvm.PENDING_LOGIN:
        return Outcome(
            name,
            State.PENDING,
            f"in the kvm group; log out and back in, or use: sg {system.KVM_GROUP} -c '<command>'",
        )
    if opts.check:
        return Outcome(name, State.MISSING, 'REQUIRED')
    if system.apply_kvm():
        return Outcome(name, State.PENDING, 'log out and back in to pick up the new group')
    echo('  Could not apply automatically; run manually:')
    system.print_kvm_fix()
    return Outcome(name, State.PENDING, 'not in the kvm group')


def _visit_loopback(opts: Options) -> Outcome:
    name = 'loopback-addresses'
    missing = system.missing_loopback()
    if not missing:
        return Outcome(name, State.PRESENT)
    listed = ', '.join(missing)
    if opts.check:
        return Outcome(name, State.MISSING, f'{listed} (REQUIRED)')
    if system.apply_loopback(missing):
        return Outcome(name, State.INSTALLED, listed)
    echo('  Could not apply automatically; run manually:')
    system.print_loopback_fix(missing)
    return Outcome(name, State.PENDING, listed)


def _print_manual_list() -> None:
    """Every tool and the executables that prove it, for an unsupported host."""
    echo('\nManual installation required. Tools needed:\n')
    for heading, wanted in (('Required:', True), ('\nOptional:', False)):
        echo(heading)
        for tool in ALL_TOOLS:
            if tool.required is not wanted:
                continue
            note = f' ({tool.note})' if tool.note else ''
            echo(f'  {tool.name}: {", ".join(tool.probe)}{note}')


def main(argv: Sequence[str] | None = None) -> int:
    """Standalone entry. Parses, builds options, and calls `action`.

    Holds no logic of its own: the dispatcher reaches `action` without coming
    through here, so anything written here would run on one route only.
    """
    parser = argparse.ArgumentParser(prog='le setup', description=__doc__)
    add_arguments(parser)
    return action(options(parser.parse_args(argv)))


if __name__ == '__main__':
    sys.exit(main())
