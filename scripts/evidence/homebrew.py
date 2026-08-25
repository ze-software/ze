#!/usr/bin/env python3
"""Where Homebrew put things, asked of the machine rather than assumed.

Homebrew has no single install prefix: /opt/homebrew on Apple Silicon,
/usr/local on Intel, and whatever the operator chose for a relocated install.
Every Homebrew path in the evidence scripts was written as the Apple Silicon
literal, so an Intel Mac with e2fsprogs and QEMU properly installed skipped the
install gates with "xorriso not found" and ran the appliance build with debugfs
off PATH.

macOS is the whole subject, and the two default prefixes are offered ONLY
there. `/usr/local` is a directory on essentially every Linux host, so a
resolver that offered it unconditionally would put `/usr/local/share/qemu` and
`/usr/local/sbin` ahead of the distribution's own paths on a box that has never
seen Homebrew. HOMEBREW_PREFIX and the `brew` binary are still honoured
anywhere, so a Linuxbrew install is found by being asked for rather than by
being guessed.

The Go half of the same story is `brewPrefixes` in
internal/appliance/homebrew.go, and scripts/le/application/setup.py carries its own
copy because it must run before anything on the machine is set up.
"""

from __future__ import annotations

import os
import re
import shutil
import sys
from pathlib import Path

# The two defaults Homebrew documents, consulted only after the machine has
# been asked, and only on macOS. They are what makes a PATH that never sourced
# `brew shellenv` still work; offering them elsewhere would hand a Linux box
# its own /usr/local ahead of the distribution paths.
BREW_DEFAULT_PREFIXES = ("/opt/homebrew", "/usr/local")


def brew_prefixes() -> list[Path]:
    """Homebrew prefixes that exist on this host, most authoritative first.

    1. HOMEBREW_PREFIX, exported by `brew shellenv`. The only source that knows
       about a relocated install, so it wins when set.
    2. The `brew` binary, which lives at <prefix>/bin/brew. Its own location
       answers the question on any host that has it. This is the one that fires
       in practice: HOMEBREW_PREFIX is unset in a plain non-login shell.
       It is NOT resolved through its symlinks: on Intel, /usr/local/bin/brew
       links into /usr/local/Homebrew/bin/brew, and following that answers
       /usr/local/Homebrew, which is the wrong prefix on exactly the machines
       this function exists for.
    3. The documented defaults, on macOS ONLY. See the module docstring: a
       Linux box has a /usr/local of its own and it is not Homebrew's.

    Duplicates are dropped, so the usual case returns exactly one path.
    """
    candidates: list[str] = []
    exported = os.environ.get("HOMEBREW_PREFIX")
    if exported:
        candidates.append(exported)
    brew = shutil.which("brew")
    if brew:
        candidates.append(str(Path(brew).parent.parent))
    if sys.platform == "darwin":
        candidates.extend(BREW_DEFAULT_PREFIXES)

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


def brew_files(rel: str) -> list[Path]:
    """Every existing <prefix>/<rel>, in prefix order.

    Returned as a list so a caller can splice it into its own candidate list
    ahead of the Linux paths, which is what all of them do.
    """
    return [p for prefix in brew_prefixes() if (p := prefix / rel).is_file()]


def version_key(path: Path) -> tuple:
    """Sort key for a Cellar version directory, by NUMBER not by spelling.

    `sorted()` on the raw path puts 1.47.10 below 1.47.4, so the string order
    hands back last month's e2fsprogs the first time a formula reaches a
    two-digit patch. The version is the parent of `sub`.

    A segment that is not a number sorts BELOW every number, so 1.47.4 outranks
    1.47.rc1. A Homebrew revision suffix is not that case: 1.47.4_1 splits into
    four numeric segments and simply outranks 1.47.4, which is correct.
    """
    version = path.parent.name
    return tuple(
        (int(part), "") if part.isdigit() else (-1, part)
        for part in re.split(r"[._-]", version)
    )


def brew_keg_dirs(formula: str, sub: str = "sbin") -> list[Path]:
    """Directories holding a keg-only formula's binaries, newest first.

    Homebrew does not link a keg-only formula onto PATH, so <prefix>/bin holds
    nothing for it and only two places do: <prefix>/opt/<formula> is the stable
    symlink kept at the current version, and <prefix>/Cellar/<formula>/<version>
    holds every version installed. Both are searched, because an interrupted
    upgrade leaves a Cellar tree with no opt link and working binaries in it.
    """
    dirs: list[Path] = []
    for prefix in brew_prefixes():
        opt = prefix / "opt" / formula / sub
        if opt.is_dir():
            dirs.append(opt)
        cellar = sorted(
            (prefix / "Cellar" / formula).glob(f"*/{sub}"),
            key=version_key,
            reverse=True,
        )
        dirs.extend(d for d in cellar if d.is_dir())
    return dirs
