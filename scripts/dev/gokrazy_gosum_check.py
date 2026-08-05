#!/usr/bin/env python3
"""Gate the tracked gokrazy builddir go.sum files against the root module.

`gokrazy/ze/builddir/**/go.sum` are checked in, and until now nothing compared
them with the root `go.sum`. They could drift silently: no build reads them
outside the gokrazy image build, and the source spec's A-6 validation confirmed
no other consumer reads them at all, which is exactly why a drift would go
unnoticed rather than surfacing as a broken build somewhere else.

What drift means here. A go.sum line is `module version hash`. When the same
(module, version) appears in a builddir file AND in the root go.sum with a
DIFFERENT hash, one of the two is recording a module content that the other
would refuse. That is a supply-chain disagreement inside one repository, and the
gokrazy image is the half nothing else exercises.

What this does NOT check, on purpose:

- A (module, version) present only in a builddir file. The builddir modules are
  third-party programs gokrazy packs (dhcp, ntp, randomd, serial-busybox, the
  rtr7 kernel); they legitimately depend on things the root module does not.
- Version SKEW, where root and builddir pin different versions of one module.
  That is normal for independent modules and is not a hash disagreement.

So the check is narrow by construction: it fires only on the one condition that
cannot be legitimate, which is the same (module, version) hashing two ways.
"""

import subprocess
import sys
from collections import defaultdict

BUILDDIR_PREFIX = "gokrazy/ze/builddir/"
ROOT_GOSUM = "go.sum"


def tracked_files(root="."):
    """Return the repository's tracked paths."""
    out = subprocess.run(
        ["git", "-C", root, "ls-files"],
        capture_output=True,
        text=True,
        check=True,
    ).stdout
    return out.split()


def builddir_gosums(files):
    """Return the tracked builddir go.sum paths, sorted."""
    return sorted(
        f for f in files if f.startswith(BUILDDIR_PREFIX) and f.endswith("go.sum")
    )


def parse_gosum(path):
    """Return {(module, version): hash} for one go.sum file.

    A go.sum carries two lines per module, one for the zip and one for the
    `/go.mod`. Both are keyed by the version string as written, so `v1.2.3` and
    `v1.2.3/go.mod` stay distinct keys and are compared separately.
    """
    entries = {}
    with open(path, encoding="utf-8") as handle:
        for line in handle:
            parts = line.split()
            if len(parts) == 3:
                entries[(parts[0], parts[1])] = parts[2]
    return entries


def find_conflicts(root_entries, builddir_files, read=parse_gosum):
    """Return the (module, version) keys hashing two ways.

    Each conflict is (path, module, version, root_hash, builddir_hash).
    """
    conflicts = []
    for path in builddir_files:
        for (module, version), digest in read(path).items():
            root_digest = root_entries.get((module, version))
            if root_digest is not None and root_digest != digest:
                conflicts.append((path, module, version, root_digest, digest))
    return conflicts


def main():
    files = tracked_files()
    builddir = builddir_gosums(files)
    if not builddir:
        # Not an error: the builddir may be retired, or this may run outside a
        # checkout that carries it. Say so rather than passing silently, so a
        # zero-file run cannot look like a clean one.
        print("gokrazy-gosum: no tracked builddir go.sum files, nothing to check")
        return 0

    root_entries = parse_gosum(ROOT_GOSUM)
    conflicts = find_conflicts(root_entries, builddir)

    shared = 0
    per_file = defaultdict(int)
    for path in builddir:
        for key in parse_gosum(path):
            if key in root_entries:
                shared += 1
                per_file[path] += 1

    if conflicts:
        print(
            f"gokrazy-gosum: {len(conflicts)} hash conflict(s) between "
            f"{ROOT_GOSUM} and the tracked builddir go.sum files",
            file=sys.stderr,
        )
        for path, module, version, root_digest, builddir_digest in conflicts:
            print(f"  {module} {version}", file=sys.stderr)
            print(f"    {ROOT_GOSUM}: {root_digest}", file=sys.stderr)
            print(f"    {path}: {builddir_digest}", file=sys.stderr)
        print(
            "\nOne of the two records a module content the other would refuse. "
            "Re-resolve the builddir module, or the root, so both agree.",
            file=sys.stderr,
        )
        return 1

    print(
        f"gokrazy-gosum OK: {len(builddir)} builddir go.sum file(s), "
        f"{shared} entry/entries shared with {ROOT_GOSUM}, no hash conflict"
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())
