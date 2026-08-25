"""Whether a tool is present, which is not the same as being on PATH.

Two rows in the table need more than `which`, and both learned it the hard way:

  e2fsprogs   is searched by DIRECTORY on every platform. Homebrew links none
              of a keg-only formula onto PATH, and Debian keeps /usr/sbin off a
              non-root user's PATH, so `which` answers nothing on a box where
              the tools are installed and working. Both consumers name the
              directories outright (`E2FS` in mk/build-gokrazy.mk,
              `e2fsSearchDirs` in internal/appliance), so a PATH-based probe
              reported missing what the build then used happily, and the
              install path reported [pending] forever.

  staticcheck must be one exact VERSION. A different one on PATH is a tool that
              runs and disagrees, which is worse than one that is absent.
"""

from __future__ import annotations

import os
import re
import sys
from pathlib import Path

from le.devtools.tools import STATICCHECK_VERSION, Tool
from le.process import run, which

__all__ = [
    'brew_prefixes',
    'cellar_version_key',
    'e2fsprogs_dirs',
    'probe',
    'probe_e2fsprogs',
]

STATICCHECK_PROBE_TIMEOUT = 5


def probe(tool: Tool) -> bool:
    """Whether `tool` is present and usable.

    Dispatches to the two special cases by name. Naming them here rather than
    putting a callable in the table keeps the table pure data, which is what
    lets a test read it without importing any of this.
    """
    if tool.name == 'e2fsprogs':
        return probe_e2fsprogs()
    if tool.name == 'staticcheck':
        return _probe_staticcheck(tool)
    if tool.probe_any:
        return any(which(name) is not None for name in tool.probe)
    return all(which(name) is not None for name in tool.probe)


def _probe_staticcheck(tool: Tool) -> bool:
    """Whether the staticcheck on PATH is the version this repository pins.

    A version mismatch is a false green: the tool runs, reports different
    findings than CI will, and nothing says why.
    """
    executable = which(tool.probe[0])
    if executable is None:
        return False
    result = run([str(executable), '-version'], timeout=STATICCHECK_PROBE_TIMEOUT)
    if not result.ok:
        return False
    expected = rf'staticcheck {re.escape(STATICCHECK_VERSION)}(?: \([^)]+\))?'
    return re.fullmatch(expected, result.out.strip()) is not None


def brew_prefixes() -> list[Path]:
    """Homebrew prefixes that exist on this host, most authoritative first.

    Homebrew has no single prefix: /opt/homebrew on Apple Silicon, /usr/local
    on Intel, and whatever an operator chose for a relocated install. Naming
    only the first made a properly installed e2fsprogs read as missing on an
    Intel Mac, and setup then offered to install what was already there.

    1. HOMEBREW_PREFIX, exported by `brew shellenv`. The only source that knows
       a relocated install.
    2. The `brew` binary, at <prefix>/bin/brew. It is NOT resolved through its
       symlinks: on Intel /usr/local/bin/brew links into
       /usr/local/Homebrew/bin/brew, and following that answers with the wrong
       prefix, on exactly the machines this exists for.
    3. The two documented defaults, for a PATH that never sourced shellenv, and
       on macOS ONLY. /usr/local is a directory on essentially every Linux host
       and it is not Homebrew's there, so offering it unconditionally puts
       /usr/local/sbin ahead of the distribution's own tools.

    The same resolution lives in Go as `brewPrefixes`
    (internal/appliance/homebrew.go) and in Python as `brew_prefixes`
    (scripts/evidence/homebrew.py). scripts/dev/homebrew_prefix_test.py holds
    the copies to one answer.
    """
    candidates: list[str] = []
    exported = os.environ.get('HOMEBREW_PREFIX')
    if exported:
        candidates.append(exported)
    brew = which('brew')
    if brew:
        candidates.append(str(brew.parent.parent))
    if sys.platform == 'darwin':
        candidates.extend(('/opt/homebrew', '/usr/local'))

    prefixes: list[Path] = []
    seen: set[str] = set()
    for candidate in candidates:
        if candidate in seen:
            continue
        seen.add(candidate)
        path = Path(candidate)
        if path.is_dir():
            prefixes.append(path)
    return prefixes


def cellar_version_key(sbin: str) -> tuple[tuple[int, str], ...]:
    """Sort key for a Cellar version directory, by NUMBER not by spelling.

    Plain string order puts 1.47.10 below 1.47.4, so it hands back last month's
    e2fsprogs the first time a formula reaches a two-digit patch.

    A segment that is not a number sorts BELOW every number, so 1.47.4 outranks
    1.47.rc1. A Homebrew revision suffix is not that case: 1.47.4_1 splits into
    four numeric segments and simply outranks 1.47.4, which is correct. This
    keys the same way as `version_key` in scripts/evidence/homebrew.py.
    """
    version = Path(sbin).parent.name
    return tuple(
        (int(part), '') if part.isdigit() else (-1, part) for part in re.split(r'[._-]', version)
    )


def e2fsprogs_dirs() -> list[Path]:
    """Every directory that can hold mkfs.ext4 and debugfs, best first.

    e2fsprogs is keg-only, so Homebrew links none of it onto PATH and a PATH
    lookup answers nothing however well it is installed. Its binaries appear in
    <prefix>/opt/e2fsprogs/sbin, the stable link at the current version, and in
    <prefix>/Cellar/e2fsprogs/<version>/sbin, which is where an interrupted
    upgrade leaves them with no link.

    Split out from `probe_e2fsprogs` so a test can assert WHERE the probe
    looks. The boolean cannot carry that: the list ends with /usr/sbin and
    /sbin, so on any Linux host that has e2fsprogs it answers True with both
    Homebrew branches deleted, and a test on it alone is green either way.
    """
    dirs: list[Path] = []
    for prefix in brew_prefixes():
        dirs.append(prefix / 'opt' / 'e2fsprogs' / 'sbin')
        cellar = sorted(
            (str(p) for p in prefix.glob('Cellar/e2fsprogs/*/sbin')),
            key=cellar_version_key,
            reverse=True,
        )
        dirs.extend(Path(d) for d in cellar)
        dirs.append(prefix / 'sbin')
    dirs.extend((Path('/usr/sbin'), Path('/sbin')))
    return dirs


def probe_e2fsprogs() -> bool:
    """True when ONE directory holds BOTH e2fsprogs tools.

    Both, not either: the image build formats /perm with mkfs.ext4 and then
    injects credentials with debugfs, so a directory carrying only the first
    passes a one-tool probe and the build dies later (mk/build-gokrazy.mk,
    E2FS).
    """
    return any((d / 'mkfs.ext4').is_file() and (d / 'debugfs').is_file() for d in e2fsprogs_dirs())
