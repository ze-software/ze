#!/usr/bin/env python3
"""Run htmx's own upgrade scanner over Ze's htmx-bearing packages.

htmx ships `dist/scripts/upgrade-check.py` in its npm package. It reads markup
and JavaScript and reports every site the htmx 2 to htmx 4 move changes:
removed attributes, renamed events, extension attributes, and the inheritance
carriers a text search does not see, because it builds a DOM and asks whether a
DESCENDANT issues a request.

Ze vendors that scanner at `third_party/web/htmx-upgrade-check.py`, beside the
library bytes it reasons about, and this program is what a gate runs. Vendored
rather than fetched: a gate that downloads its own judge cannot run offline and
cannot be reproduced from what git holds.

WHAT IT SCANS
    Derived, not listed. Every `assets/` directory under `internal/` that holds
    an htmx core file is one htmx-bearing consumer, and the package holding it
    is scanned whole. That is the same derivation `scripts/vendor/check_web.go`
    makes for the drift gate, and it means a fourth interface that embeds htmx
    is scanned the day it does so.

    Extensions: the scanner's own defaults (`.html`, `.js`, ...) plus `.templ`
    and `.go`. Ze writes markup in three places -- templ sources, Go string
    literals (the chaos dashboard), and the captured golden fixtures that are
    the rendered bytes htmx actually sees -- and all three are in scope.

THE EXPLAINED LIST
    `scripts/dev/htmx-upgrade-explained.txt` holds one row per issue class that
    survives the cutover with a reason it does not apply, as
    `<path> | <category> | <reason>`. A row is a claim about Ze's markup, so it
    fails closed both ways: an issue no row explains fails the gate, and a row
    that explains no issue fails it too. A stale exemption is how an allowlist
    becomes a place to hide.

Usage:
    python3 scripts/dev/htmx_upgrade_check.py [--check] [--report] [--root DIR]
"""

import argparse
import importlib.util
import os
import sys

# SCANNER is the vendored htmx scanner, from the repository root.
SCANNER = os.path.join("third_party", "web", "htmx-upgrade-check.py")

# EXPLAINED is the list of issues Ze has judged not to apply.
EXPLAINED = os.path.join("scripts", "dev", "htmx-upgrade-explained.txt")

# CONSUMER_DIR is the directory name a web interface keeps its embedded assets
# in. third_party/web/MANIFEST.md's Consumers table uses it for every row.
CONSUMER_DIR = "assets"

# CONSUMER_ROOT is the tree walked for consumer asset directories.
CONSUMER_ROOT = "internal"

# HTMX_PREFIX and HTMX_SUFFIX identify an htmx core file inside a consumer
# directory. The served name is `htmx.min.js` before the cutover and after it,
# so a name test survives the version change that a version test would not.
HTMX_PREFIX = "htmx"
HTMX_SUFFIX = ".js"

# EXTRA_EXTENSIONS are the file kinds Ze writes markup in that the scanner does
# not scan by default.
EXTRA_EXTENSIONS = [".templ", ".go"]


def load_scanner(root):
    """Import the vendored scanner as a module.

    Importing beats parsing its output: an Issue carries file, line, category
    and message as fields, and one of its messages is itself multi-line.
    """
    path = os.path.join(root, SCANNER)
    if not os.path.isfile(path):
        raise SystemExit(
            f"htmx-upgrade-check: the vendored scanner is missing at {path}; "
            "re-vendor it from the htmx npm package (third_party/web/MANIFEST.md)"
        )

    spec = importlib.util.spec_from_file_location("htmx_upgrade_scanner", path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)

    for name in ("check_file", "collect_files", "DEFAULT_EXTENSIONS"):
        if not hasattr(module, name):
            raise SystemExit(
                f"htmx-upgrade-check: the vendored scanner has no {name}; "
                "its interface changed, so this gate must be re-read against it"
            )

    return module


def htmx_packages(root):
    """Return every package that embeds an htmx core file, from the root.

    A package is named by the directory HOLDING the `assets/` directory, so the
    scan reaches that package's templates, handlers and fixtures, not only the
    library bytes it serves.
    """
    found = []

    base = os.path.join(root, CONSUMER_ROOT)
    for dirpath, dirnames, filenames in os.walk(base):
        dirnames[:] = [
            d
            for d in dirnames
            if d not in ("testdata", "node_modules") and not d.startswith(".")
        ]

        if os.path.basename(dirpath) != CONSUMER_DIR:
            continue

        # testdata is pruned above so a fixture tree cannot enrol itself; the
        # package's own testdata is still scanned, because it is reached
        # through the package directory rather than through this walk.
        if not any(
            f.startswith(HTMX_PREFIX) and f.endswith(HTMX_SUFFIX) for f in filenames
        ):
            continue

        found.append(os.path.relpath(os.path.dirname(dirpath), root))

    found.sort()

    return found


def scan(root, scanner, packages):
    """Return (issues, file_count) over every named package."""
    extensions = list(scanner.DEFAULT_EXTENSIONS) + EXTRA_EXTENSIONS

    paths = [os.path.join(root, p) for p in packages]
    files = scanner.collect_files(paths, extensions)

    issues = []
    for path in files:
        for issue in scanner.check_file(path):
            issues.append(
                (os.path.relpath(path, root), issue.category, issue.line, issue.message)
            )

    return issues, len(files)


def read_explained(root):
    """Return the explained rows as {(path, category): reason}.

    A blank line and a `#` line are comments. Every other line must hold the
    three fields, because a row that cannot be read is a claim nobody can check.
    """
    path = os.path.join(root, EXPLAINED)
    if not os.path.isfile(path):
        raise SystemExit(
            f"htmx-upgrade-check: {EXPLAINED} is missing; it is this gate's "
            "record of what does not apply, and its absence is not an empty list"
        )

    rows = {}
    with open(path, encoding="utf-8") as handle:
        for number, line in enumerate(handle, start=1):
            line = line.strip()
            if not line or line.startswith("#"):
                continue

            fields = [f.strip() for f in line.split("|")]
            if len(fields) != 3 or not all(fields):
                raise SystemExit(
                    f"htmx-upgrade-check: {EXPLAINED}:{number}: a row is "
                    "`<path> | <category> | <reason>`, and every field must carry text"
                )

            rows[(fields[0], fields[1])] = fields[2]

    return rows


def report(issues, files, packages):
    """Print every issue, grouped by file, with the totals under it."""
    for path, category, line, message in issues:
        print(f"{path}:{line}: [{category}] {message}")

    counted = len(
        {(path, category, line, message) for path, category, line, message in issues}
    )
    scanned = ", ".join(packages)
    print(f"\n{counted} issue(s) over {files} file(s) in {scanned}")


def check(root):
    """Run the scanner and judge the result. Returns the exit code."""
    scanner = load_scanner(root)

    packages = htmx_packages(root)
    if not packages:
        # FAIL CLOSED. A run that scanned nothing has proven nothing.
        print(
            f"htmx-upgrade-check: no {CONSUMER_ROOT}/**/{CONSUMER_DIR}/ directory holds an "
            "htmx core file, so this check proved nothing",
            file=sys.stderr,
        )
        return 1

    issues, files = scan(root, scanner, packages)
    if files == 0:
        print(
            f"htmx-upgrade-check: no file was read under {', '.join(packages)}, "
            "so this check proved nothing",
            file=sys.stderr,
        )
        return 1

    explained = read_explained(root)

    unexplained = []
    used = set()
    for path, category, line, message in issues:
        key = (path, category)
        if key in explained:
            used.add(key)
            continue
        unexplained.append((path, category, line, message))

    stale = sorted(set(explained) - used)

    for path, category, line, message in unexplained:
        print(f"{path}:{line}: [{category}] {message}")

    for path, category in stale:
        print(
            f"STALE: {EXPLAINED} explains {category} in {path}, and the scan reports none there"
        )

    if unexplained or stale:
        print(
            f"\n{len(unexplained)} unexplained issue(s) and {len(stale)} stale row(s) "
            f"over {files} file(s) in {', '.join(packages)}",
            file=sys.stderr,
        )
        print("Fix the site, or add its row to " + EXPLAINED, file=sys.stderr)
        return 1

    print(
        f"htmx-upgrade-check: {files} file(s) in {', '.join(packages)} carry no "
        f"unexplained htmx 4 upgrade issue ({len(explained)} explained)"
    )

    return 0


def repo_root():
    """Return the tree holding the working directory, by its go.mod."""
    path = os.path.abspath(os.getcwd())
    while True:
        if os.path.isfile(os.path.join(path, "go.mod")):
            return path
        parent = os.path.dirname(path)
        if parent == path:
            raise SystemExit(
                "htmx-upgrade-check: go.mod not found above the working directory"
            )
        path = parent


def main():
    parser = argparse.ArgumentParser(
        description="Run htmx's own upgrade scanner over Ze's htmx-bearing packages."
    )
    parser.add_argument(
        "--check",
        action="store_true",
        help="judge the tree and exit non-zero on an unexplained issue (default)",
    )
    parser.add_argument(
        "--report",
        action="store_true",
        help="print every issue the scanner reports, explained or not, and exit 0",
    )
    parser.add_argument(
        "--root",
        default="",
        help="repository root to scan (default: the tree holding the working directory)",
    )
    args = parser.parse_args()

    root = args.root or repo_root()

    if args.report:
        scanner = load_scanner(root)
        packages = htmx_packages(root)
        issues, files = scan(root, scanner, packages)
        report(issues, files, packages)
        return 0

    return check(root)


if __name__ == "__main__":
    sys.exit(main())
