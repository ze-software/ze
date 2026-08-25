"""What a Ze dev or test workflow needs, and how each one is installed.

The table is data. Everything that acts on it -- probing, installing,
reporting -- lives elsewhere and reads it, so adding a tool is adding a row.
"""

from __future__ import annotations

import platform
from dataclasses import dataclass
from enum import Enum

__all__ = [
    'APPLIANCE_CHECKS',
    'GRUB_APT_PACKAGE',
    'OPTIONAL_TOOLS',
    'REQUIRED_TOOLS',
    'STATICCHECK_VERSION',
    'ApplianceCheck',
    'PackageManager',
    'Tool',
    'grub_apt_package',
]

STATICCHECK_VERSION = '2026.1'


class PackageManager(Enum):
    """The one package manager this host installs system packages with."""

    BREW = 'brew'
    APT = 'apt'


# GRUB ships one module set per EFI target, and Debian packages each set for
# its own host architecture only: `grub-efi-amd64-bin` is Architecture: amd64,
# so on an arm64 box apt answers "has no installation candidate" and installs
# nothing at all, `grub-mkstandalone` included. Measured in debian:stable-slim
# on both arches: arm64 refuses the amd64 package and takes
# `grub-efi-arm64-bin`; amd64 takes the amd64 one. Either package pulls
# grub-common, which is where `grub-mkstandalone` lives.
#
# So ask for the set this host can build with. `ze appliance iso` picks its
# target from the architecture of the image it packs (`isoGRUBTarget`,
# internal/appliance/cmd_iso.go), so building an ISO for the OTHER architecture
# needs that architecture's set as well, through `dpkg --add-architecture`.
_GRUB_BY_MACHINE = {
    'aarch64': 'grub-efi-arm64-bin',
    'arm64': 'grub-efi-arm64-bin',
    'i386': 'grub-efi-ia32-bin',
    'i686': 'grub-efi-ia32-bin',
}


def grub_apt_package(machine: str) -> str:
    """The GRUB module-set package this host architecture can install.

    Debian names one package per EFI target and builds each only for its own
    architecture, so the answer is the host's, not a preference. Anything not
    listed falls back to amd64: Ze's appliance targets are amd64 and arm64
    (`isoGRUBTarget`, internal/appliance/cmd_iso.go), and a host outside the
    table cannot build for itself whatever this returns.
    """
    return _GRUB_BY_MACHINE.get(machine, 'grub-efi-amd64-bin')


GRUB_APT_PACKAGE = grub_apt_package(platform.machine())


@dataclass(frozen=True)
class Tool:
    """One thing that must be on the machine, and every route to putting it there.

    `probe` is the executable names to look for. `probe_any` says one of them
    is enough; without it every name must be found, which is what a package
    shipping two required binaries needs.

    The install routes are tried in the order `install.py` reads them:
    `go_install` and `pipx_install` work on both platforms and so win over the
    system package manager, which differs per platform.
    """

    name: str
    probe: tuple[str, ...]
    brew: str | None = None
    apt: str | None = None
    go_install: str | None = None
    pipx_install: str | None = None
    required: bool = True
    note: str = ''
    probe_any: bool = False

    def package_for(self, manager: PackageManager) -> str | None:
        """The package name this manager would install, if it has one."""
        return self.brew if manager is PackageManager.BREW else self.apt

    def installable_by(self, manager: PackageManager) -> bool:
        """Whether any route exists to install this tool on this host.

        A tool with no route is neither present nor missing: it is skipped, and
        saying so is the difference between an honest report and one that reds
        on a platform where nothing can be done.
        """
        if self.go_install or self.pipx_install:
            return True
        return self.package_for(manager) is not None


@dataclass(frozen=True)
class ApplianceCheck:
    """A dependency the appliance doctor reports on, and its packages here.

    DRIFT-GUARD: the names come from `applianceDoctorChecks()` in
    internal/appliance/doctor_checks.go, and internal/appliance/
    dev_setup_drift_test.go parses THIS file to hold the two lists to one
    answer. It matches on `name='appliance-...'`, single-quoted, which is how
    ruff formats this package, and it fails rather than passing vacuously when
    that pattern finds nothing. So the quote style here is load-bearing: do not
    rename the field, and do not reformat this table with double quotes.
    """

    name: str
    probe: tuple[str, ...]
    brew: str | None = None
    apt: str | None = None
    note: str = ''


APPLIANCE_CHECKS: tuple[ApplianceCheck, ...] = (
    ApplianceCheck(
        name='appliance-grub',
        probe=('grub-mkstandalone', 'grub2-mkstandalone'),
        brew=None,
        apt=GRUB_APT_PACKAGE,
        note='no first-party Homebrew formula; macOS skips grub (ISO builds are Linux/container-only)',
    ),
    ApplianceCheck(
        name='appliance-xorriso',
        probe=('xorriso',),
        brew='xorriso',
        apt='xorriso',
    ),
    ApplianceCheck(
        name='appliance-e2fsprogs',
        probe=('mkfs.ext4', 'debugfs'),
        brew='e2fsprogs',
        apt='e2fsprogs',
        note='keg-only on macOS; Go code resolves via Cellar glob, no PATH change needed',
    ),
)


REQUIRED_TOOLS: tuple[Tool, ...] = (
    Tool(name='go', probe=('go',), brew='go', apt='golang-go'),
    Tool(name='git', probe=('git',), brew='git', apt='git'),
    Tool(name='protobuf', probe=('protoc',), brew='protobuf', apt='protobuf-compiler'),
    Tool(name='jq', probe=('jq',), brew='jq', apt='jq'),
    Tool(
        name='golangci-lint',
        probe=('golangci-lint',),
        go_install='github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.10.1',
    ),
    Tool(
        name='staticcheck',
        probe=('staticcheck',),
        go_install=f'honnef.co/go/tools/cmd/staticcheck@{STATICCHECK_VERSION}',
    ),
    Tool(
        name='goimports',
        probe=('goimports',),
        go_install='golang.org/x/tools/cmd/goimports@latest',
    ),
    Tool(
        name='gopls',
        probe=('gopls',),
        go_install='golang.org/x/tools/gopls@latest',
        note=(
            'language server behind the agent LSP tool; without it every LSP call'
            ' returns ENOENT and the session reads whole files instead'
            ' (ai/rules/context-economy.md)'
        ),
    ),
    Tool(name='python3', probe=('python3',), brew='python', apt='python3'),
    Tool(
        name='qemu',
        probe=('qemu-system-x86_64', 'qemu-system-aarch64'),
        probe_any=True,
        brew='qemu',
        apt='qemu-system-x86',
    ),
    Tool(
        name='e2fsprogs',
        probe=('mkfs.ext4', 'debugfs'),
        probe_any=True,
        brew='e2fsprogs',
        apt='e2fsprogs',
        note='keg-only on macOS; Go code resolves via Cellar glob',
    ),
    Tool(name='xorriso', probe=('xorriso',), brew='xorriso', apt='xorriso'),
    Tool(
        name='grub',
        probe=('grub-mkstandalone', 'grub2-mkstandalone'),
        probe_any=True,
        brew=None,
        apt=GRUB_APT_PACKAGE,
        note='no first-party Homebrew formula; macOS skips (ISO builds are Linux/container-only)',
    ),
    Tool(name='pipx', probe=('pipx',), brew='pipx', apt='pipx'),
    # uv installs through pipx on BOTH platforms, and the reason is Linux: uv
    # is not in the Debian or Ubuntu repositories, so `apt=None` left a
    # REQUIRED tool with no package there. `installable_by` answers False for
    # that state and the loop prints `[skipped]`, so the tool the evidence SSH
    # probe needs (`uv run --with paramiko`) went missing without ever being
    # counted: check mode said "All required tools present" on a box that had
    # no uv. A guard that reports green on absence is worse than no guard.
    #
    # pipx over brew on macOS, and over the curl installer everywhere: one
    # route is one thing to fix, and `curl | sh` is a supply-chain hole this
    # program would be opening on every dev machine. It must come after the
    # pipx row -- a pipx install is skipped while pipx is not there yet.
    Tool(
        name='uv',
        probe=('uv',),
        pipx_install='uv',
        note='not in apt repos, so pipx is the one route that works on both platforms',
    ),
    Tool(name='ruff', probe=('ruff',), pipx_install='ruff'),
    Tool(
        name='mypy',
        probe=('mypy',),
        pipx_install='mypy',
        note=(
            'the type gate for scripts/le; strict mode, configured in'
            ' pyproject.toml and run by `le lint`'
        ),
    ),
    Tool(
        name='pyright',
        probe=('pyright', 'pyright-langserver'),
        pipx_install='pyright',
        note=(
            'language server for the Python half of the tree; gopls answers a'
            ' symbol question about a .go file, pyright answers the same'
            ' question about a .py one (ai/rules/context-economy.md)'
        ),
    ),
)


OPTIONAL_TOOLS: tuple[Tool, ...] = (
    Tool(
        name='sshpass',
        probe=('sshpass',),
        brew='sshpass',
        apt='sshpass',
        required=False,
        note='SSH-probe fallback only; uv+paramiko is primary',
    ),
    Tool(
        name='docker',
        probe=('docker',),
        brew='docker',
        apt='docker.io',
        required=False,
        note='container appliance/kernel builds',
    ),
    Tool(
        name='colima',
        probe=('colima',),
        brew='colima',
        required=False,
        note='macOS Docker runtime',
    ),
    Tool(
        name='xl2tpd',
        probe=('xl2tpd',),
        brew=None,
        apt='xl2tpd',
        required=False,
        note=(
            'Linux root-only L2TP LAC peer for L2TP PPP evidence tests'
            ' (ze-deployment-l2tp-ppp-test, ze-deployment-gokrazy-l2tp-ppp-test)'
        ),
    ),
    Tool(
        name='ppp',
        probe=('pppd',),
        brew=None,
        apt='ppp',
        required=False,
        note='Linux root-only pppd for the same L2TP PPP/NCP evidence tests',
    ),
)


ALL_TOOLS: tuple[Tool, ...] = REQUIRED_TOOLS + OPTIONAL_TOOLS
