#!/usr/bin/env python3
# Design: docs/contributing/testing.md -- the build flavors each gate covers
#
# lint_flavors.py -- lint the builds that the two golangci-lint passes in
# `_ze-lint-impl` cannot reach, and prove that no tracked Go file is left
# unreported on.
#
# golangci-lint analyzes ONE build for each run: one GOOS, one GOARCH, one tag
# set. The Makefile runs two, and 245 tracked own-source Go files sat outside
# both of them on a Linux host (measured 2026-08-24, `--coverage` prints the
# number for the tree in front of you). A file outside the analyzed build is not
# merely unchecked: every pass exits 0 and reads as clean over it.
#
# Four causes put a file there, and each needs a different answer:
#
#   1. The package set. `ZE_LINT_PKGS` named four roots and the tree has more.
#      Answered in the Makefile, which now lints ./... -- not here.
#   2. The compile-out stubs. A `//go:build !ze_<feature>` file is the code an
#      operator reaches when the feature is OFF, and `.golangci.yml` turns every
#      feature ON. The `compile-out` flavor row drops every gate and keeps
#      ze_core alone, which is the one subtractive mechanism (see tagless_config).
#   3. Personality and capability tags. `ze_installer`, `ze_distro`, `debug`,
#      `race` and the rest. One flavor row each, below.
#   4. GOOS and GOARCH. `--build-tags` cannot express either, and pass 1 uses the
#      HOST's, so which files a Linux machine leaves blind differs from a Mac.
#      One flavor row for each target that a tracked file names.
#
# WHY A SCOPE IS DERIVED RATHER THAN WRITTEN DOWN. A flavor's package list is
# computed from the tree: the packages holding files that the base passes do not
# load. A hand-written list drifts the moment somebody adds a `//go:build debug`
# file in a new package, and the drift is silent -- which is the defect class
# this whole script exists to close (plan/journal/gate-excludes-part-of-its-
# population.md). Deriving it costs one `go list` for each flavor, about 2.5s.
#
# FAIL LOUD ON AN EMPTY POPULATION. A flavor whose scope is empty is SKIPPED
# rather than run: golangci-lint exits 5 with "no go files to analyze" when no
# package in its set has a buildable file, and a pass that cannot run is not a
# pass that found nothing. An empty scope means the base passes already cover
# that flavor's files -- on a darwin host the `darwin` flavor is empty for
# exactly that reason -- and `--coverage` is what proves the file is covered
# rather than lost.
#
# Usage:
#   lint_flavors.py                     lint every flavor, then assert coverage
#   lint_flavors.py --scope <pkgs>      the same, restricted to these packages
#   lint_flavors.py --coverage          print the blind files, run no linter
#   lint_flavors.py --list              print the flavor table
#
# The linter's own ceilings arrive from the Makefile through ZE_LINT_FLAVOR_RUN:
# GOMEMLIMIT in the environment, and `-j` in the arguments this script passes on
# to every run it makes. It sets neither itself, because a second opinion about
# the share a lint may take is how a job takes twice it
# (plan/spec-shared-machine-job-admission.md).

import argparse
import os
import re
import subprocess
import sys

ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
CONFIG = os.path.join(ROOT, ".golangci.yml")

# The template asks `go list` for every file golangci-lint analyzes in a
# package: the build's own files, and the test files it compiles beside them.
LIST_TEMPLATE = (
    "{{$d:=.Dir}}"
    "{{range .GoFiles}}{{$d}}/{{.}}\n{{end}}"
    "{{range .CgoFiles}}{{$d}}/{{.}}\n{{end}}"
    "{{range .TestGoFiles}}{{$d}}/{{.}}\n{{end}}"
    "{{range .XTestGoFiles}}{{$d}}/{{.}}\n{{end}}"
)


def config_tags():
    """Return the build tags .golangci.yml declares, in file order."""
    tags, inside = [], False
    with open(CONFIG, encoding="utf-8") as handle:
        for line in handle:
            if line.startswith("  build-tags:"):
                inside = True
                continue
            if inside:
                if line.startswith("    - "):
                    tags.append(line.strip()[2:])
                    continue
                break
    if not tags:
        sys.exit(
            "lint_flavors: .golangci.yml declares no build-tags; the file's shape changed"
        )
    return tags


def feature_gate_tags():
    """Return every per-feature compile-out tag, derived from the config.

    `ze_core` is not one of them: it selects the daemon rather than a feature,
    and a build without it is a different program (the `setup-standalone` row).
    Everything else in `.golangci.yml`'s list is generated from
    `feature-gates.txt` by `make generate`, so reading it here is reading the
    manifest one hop away and cannot drift from it.
    """
    return [tag for tag in config_tags() if tag != "ze_core"]


# BASE_PASSES are the two golangci-lint runs `_ze-lint-impl` performs. They are
# declared here so a flavor's scope can subtract what they already load; this
# script never runs them. lint_flavors_test.py reads the Makefile and fails when
# the recipe and this table stop agreeing.
BASE_PASSES = [
    {"name": "host", "goos": None, "goarch": None, "tags": []},
    {
        "name": "linux-integration",
        "goos": "linux",
        "goarch": None,
        "tags": ["integration"],
    },
]

# FLAVORS is the table. `tags` are ADDED to the build-tags `.golangci.yml`
# carries: `--build-tags` extends that list rather than replacing it (measured,
# golangci-lint v2.10.1), which is why no row here repeats a feature gate and
# why no row can turn one OFF.
#
# A row that must turn a tag OFF names it in `without`. That row is run against
# a copy of `.golangci.yml` carrying no build-tags at all (see tagless_config),
# and the whole tag set is then passed on the command line. It is the one
# subtractive mechanism golangci-lint v2.10.1 offers.
FLAVORS = [
    {
        "name": "darwin",
        "goos": "darwin",
        "goarch": None,
        "tags": [],
        "why": "every `!linux` and `darwin` file. Pass 1 uses the HOST GOOS, so on a "
        "Linux machine both base passes are GOOS=linux and 101 files are blind",
    },
    {
        "name": "freebsd",
        "goos": "freebsd",
        "goarch": None,
        "tags": [],
        "why": "the FreeBSD TCP-MD5 socket option, and the `!linux && !darwin` fallbacks in "
        "sysctl and hostload",
    },
    {
        "name": "openbsd",
        "goos": "openbsd",
        "goarch": None,
        "tags": [],
        "why": "`!linux && !freebsd && !darwin`, which is the generic no-TCP-MD5 fallback",
    },
    {
        "name": "dragonfly",
        "goos": "dragonfly",
        "goarch": None,
        "tags": [],
        "why": "a unix GOOS that is none of linux, darwin, freebsd, openbsd or netbsd. "
        "internal/core/privilege/drop_other.go names exactly that set, and the five rows "
        "above all fail one of its terms. dragonfly is picked over solaris, illumos and "
        "aix because Go ships the same unix build for all four and dragonfly is the one "
        "with a BSD row beside it to compare against",
    },
    {
        "name": "wasip1",
        "goos": "wasip1",
        "goarch": "wasm",
        "tags": [],
        "why": "the `//go:build !unix` fallbacks in internal/core/crashlog and pkg/zefs. "
        "windows and plan9 cannot lint them -- internal/core/slogutil imports log/syslog "
        "with no build constraint and that package has no implementation on either -- but "
        "wasip1 is not unix and log/syslog does build there, so the whole import graph "
        "type-checks. Ze does not ship a WASI binary; this row is a lens on the !unix "
        "files, not a target claim",
    },
    {
        "name": "linux-arm64",
        "goos": "linux",
        "goarch": "arm64",
        "tags": [],
        "why": "the netlink int-width files, selected by filename suffix rather than by a "
        "constraint. The appliance ships on arm64",
    },
    {
        "name": "linux-other-arch",
        "goos": "linux",
        "goarch": "riscv64",
        "tags": [],
        "why": "`linux && !amd64 && !arm64`, the generic netlink fallback",
    },
    {
        "name": "capability",
        "goos": "linux",
        "goarch": None,
        "tags": [
            "debug",
            "race",
            "live",
            "stress",
            "maprib",
            "fleetperf",
            "zetest",
            "gokrazy",
            "ze_test",
            "ze_perf",
            "ze_analyze",
            "ze_chaos",
            "ze_le",
            "integration",
        ],
        "why": "every capability tag that is not a personality. They coexist in one build: "
        "each names an added file rather than a different program. ze_le is the odd one "
        "and belongs here for the same reason: it adds cmd/ze/ze_le_register.go, whose "
        "one blank import carries le's development commands into ze under a single root "
        "(letools/zele). No shipped build sets it, so no other pass loads that file",
    },
    {
        "name": "distro",
        "goos": "linux",
        "goarch": None,
        "tags": ["ze_distro"],
        "why": "bin/ze, the daemon `make ze-build` builds",
    },
    {
        "name": "appliance",
        "goos": "linux",
        "goarch": None,
        "tags": ["ze_appliance"],
        "why": "the binary gokrazy packs into the appliance image",
    },
    {
        "name": "setup",
        "goos": "linux",
        "goarch": None,
        "tags": ["ze_setup"],
        "why": "ze-host, the `ze appliance ...` build driver",
    },
    {
        "name": "personalities",
        "goos": "linux",
        "goarch": None,
        "tags": ["ze_distro", "ze_appliance", "ze_setup"],
        "why": "the files that assert what happens when the personality tags are combined. "
        "No single-personality row selects them",
    },
    {
        "name": "installer",
        "goos": "linux",
        "goarch": None,
        "tags": ["ze_installer", "ze_installer_fault"],
        "why": "cmd/ze-installer and internal/install/disk, the installer initrd's PID 1. "
        "It is compiled, vetted and unit-tested; until 2026-08-24 no LINT pass had "
        "ever loaded it",
    },
    {
        "name": "installer-nofault",
        "goos": "linux",
        "goarch": None,
        "tags": ["ze_installer"],
        "why": "the `!ze_installer_fault` file, which the row above excludes by turning the "
        "fault-injection tag ON",
    },
    {
        "name": "tinygo",
        "goos": "linux",
        "goarch": None,
        "tags": ["tinygo"],
        "why": "the TinyGo pprof stub. The tag selects a no-op where the gc build starts a "
        "pprof server, and nothing else in the tree carries it",
    },
    {
        "name": "setup-standalone",
        "goos": "linux",
        "goarch": None,
        "tags": ["ze_setup"],
        "without": ["ze_core"],
        "why": "bin/ze-setup, whose dispatch is `ze_setup && !ze_core`. It is a disjoint "
        "cmd/ze program, so the row above (which keeps ze_core) cannot reach it",
    },
    {
        "name": "compile-out",
        "goos": "linux",
        "goarch": None,
        "tags": [],
        "without": feature_gate_tags(),
        "why": "every `//go:build !ze_<feature>` stub: the code an operator reaches when a "
        "feature is OFF -- the `not built` message, the no-op registration, the alternative "
        "dispatch. `.golangci.yml` turns every gate ON, so this row drops all of them and "
        "keeps ze_core alone. One row reaches every stub because the gates are independent: "
        "a build with none of them on satisfies every negated term at once",
    },
]


def build_constraint(path):
    """Return the file's //go:build expression, or None when it carries none."""
    try:
        with open(
            os.path.join(ROOT, path), encoding="utf-8", errors="replace"
        ) as handle:
            head = handle.read(4096)
    except OSError:
        return None
    found = re.search(r"^//go:build (.*)$", head, re.M)
    return found.group(1).strip() if found else None


# RESIDUE is what no flavor reaches, with the reason for each. A file listed here
# is NOT excused: the reason states what would have to change, and `--coverage`
# fails when a residue entry stops being blind, so an entry cannot outlive its
# cause. Nothing is added here to make a red run green.
RESIDUE = {
    "examples/plugin/go/main.go": "a separate Go module (`module example/acme-monitor`), so `./...` cannot reach it and "
    "golangci-lint would need its own run in that directory. It has no tracked go.sum, so "
    "the module does not resolve without `go mod tidy`, and this repository vendors its "
    "dependencies rather than fetching them. Needs an owner decision on how the example "
    "module joins the build",
    "tools.go": "the tools.go idiom: a `//go:build tools` file whose imports are PROGRAMS, "
    "kept so `go mod tidy` pins the build tools. golangci-lint reports "
    "'is a program, not an importable package' for each one, and so would any type checker. "
    "It stops being blind when the pins move to go.mod's `tool` directives, which is its own "
    "change",
}


def run(cmd, env=None, cwd=ROOT):
    """Run a command and return (returncode, stdout, stderr)."""
    proc = subprocess.run(
        cmd, cwd=cwd, env=env, capture_output=True, text=True, check=False
    )
    return proc.returncode, proc.stdout, proc.stderr


def effective_tags(pas):
    """Return every build tag this pass compiles with, config tags included."""
    without = set(pas.get("without", ()))
    return [tag for tag in config_tags() if tag not in without] + pas["tags"]


def go_list(pas, patterns):
    """Return {package directory: {file, ...}} for one pass over these patterns.

    Paths are repository-relative. `-e` keeps a package that does not type-check
    in the answer: this asks which files a build SELECTS, and a package that
    fails to compile still has to be linted rather than silently dropped.
    """
    env = dict(os.environ)
    if pas["goos"]:
        env["GOOS"] = pas["goos"]
    if pas["goarch"]:
        env["GOARCH"] = pas["goarch"]
    tags = " ".join(effective_tags(pas))
    code, out, err = run(
        ["go", "list", "-e", "-tags", tags, "-f", LIST_TEMPLATE] + patterns, env
    )
    if code != 0 and not out:
        sys.exit(
            f"lint_flavors: go list failed for flavor {pas['name']}: {err.strip()}"
        )
    packages = {}
    prefix = ROOT + "/"
    for line in out.splitlines():
        line = line.strip()
        if not line.startswith(prefix):
            continue
        path = line[len(prefix) :]
        packages.setdefault(os.path.dirname(path), set()).add(path)
    return packages


def files_of(packages):
    """Flatten a go_list answer into one file set."""
    return {path for files in packages.values() for path in files}


def population():
    """Return every tracked Go file a lint pass is expected to reach.

    Three groups are outside it. `vendor/` is other people's code.
    `gokrazy/modcache/` is a tracked third-party module cache. A `//go:build
    ignore` file belongs to no build by its own declaration, which is how every
    program under scripts/checks/ lives beside its siblings in one directory.

    A fourth is outside it for a different reason: `git ls-files` answers from
    the INDEX, so a file deleted in the working tree is still tracked. No pass
    can lint a file that is not on disk, and reporting one as blind would fail
    every run between the deletion and its commit.
    """
    _, out, _ = run(["git", "ls-files", "*.go"])
    files = []
    for path in out.split():
        if path.startswith("vendor/") or path.startswith("gokrazy/modcache/"):
            continue
        if not os.path.exists(os.path.join(ROOT, path)):
            continue
        constraint = build_constraint(path)
        if constraint and re.search(r"\bignore\b", constraint):
            continue
        files.append(path)
    return set(files)


def scopes(patterns):
    """Return [(flavor, [package directory, ...]), ...] and the files every pass loads.

    A flavor's scope holds the packages carrying a file no EARLIER pass loads,
    base passes first. Order decides which flavor is charged with a file that
    two of them select; it never decides whether the file is covered.
    """
    seen = set()
    for pas in BASE_PASSES:
        seen |= files_of(go_list(pas, patterns))
    answer = []
    for flavor in FLAVORS:
        packages = go_list(flavor, patterns)
        scope = sorted(name for name, files in packages.items() if files - seen)
        seen |= files_of(packages)
        answer.append((flavor, scope))
    return answer, seen


def tagless_config():
    """Write a copy of .golangci.yml carrying no build tags, and return its path.

    A flavor that must turn a tag OFF cannot say so on the command line:
    `--build-tags` only adds. Starting from a config with an EMPTY tag list makes
    the flavor's own `tags` the whole set, which is the one subtractive mechanism
    golangci-lint v2.10.1 offers.

    Derived on every run rather than generated into the tree: a second tracked
    config would have to be kept in step with the first one by hand, and the
    linter set is exactly the thing that must not differ between two passes.
    `relative-path-mode` is set because golangci-lint reports paths relative to
    the config file, and a flavor's findings must name the same path the base
    passes do.

    The name carries the pid because several sessions share one checkout, and
    `main` removes the file on every exit path.
    """
    scratch = os.path.join(ROOT, "tmp", "lint-flavors")
    os.makedirs(scratch, exist_ok=True)
    path = os.path.join(scratch, f"notags-{os.getpid()}.yml")
    lines, dropping = [], False
    with open(CONFIG, encoding="utf-8") as handle:
        for line in handle.read().splitlines():
            if line.startswith("  build-tags:"):
                dropping = True
                continue
            if dropping:
                if line.startswith("    - "):
                    continue
                dropping = False
            lines.append(line)
            if line.startswith("run:"):
                lines.append("  relative-path-mode: gitroot")
    with open(path, "w", encoding="utf-8") as handle:
        handle.write("\n".join(lines) + "\n")
    return path


def lint(flavor, scope, extra_args):
    """Run one golangci-lint pass. Returns the exit code."""
    env = dict(os.environ)
    if flavor["goos"]:
        env["GOOS"] = flavor["goos"]
    if flavor["goarch"]:
        env["GOARCH"] = flavor["goarch"]
    command = ["golangci-lint", "run"] + extra_args
    if flavor.get("without"):
        command += ["-c", tagless_config()]
    tags = effective_tags(flavor) if flavor.get("without") else flavor["tags"]
    if tags:
        command += ["--build-tags", ",".join(tags)]
    command += ["./" + name for name in scope]
    target = f"GOOS={flavor['goos'] or 'host'}"
    if flavor["goarch"]:
        target += f" GOARCH={flavor['goarch']}"
    added = ",".join(flavor["tags"]) or "none"
    removed = ",".join(flavor.get("without", ())) or "none"
    print(
        f"Running ze linter ({flavor['name']}: {target}, tags add {added} drop {removed}, "
        f"{len(scope)} packages)...",
        flush=True,
    )
    return subprocess.run(command, cwd=ROOT, env=env, check=False).returncode


def report_coverage(seen):
    """Print the files no pass loads. Returns the exit code."""
    blind = sorted(population() - seen)
    unexpected = [path for path in blind if path not in RESIDUE]
    healed = [path for path in RESIDUE if path not in blind]
    for path in blind:
        reason = RESIDUE.get(path, "NOT COVERED BY ANY PASS")
        print(f"  {path}: {reason}")
    if unexpected:
        print(
            f"lint_flavors: {len(unexpected)} tracked Go file(s) are linted by nothing. "
            "Add the flavor that selects them, or state the reason in RESIDUE "
            "(scripts/dev/lint_flavors.py).",
            file=sys.stderr,
        )
        return 1
    if healed:
        print(
            f"lint_flavors: {len(healed)} RESIDUE entr(y|ies) are now linted: "
            f"{', '.join(healed)}. Delete the entry -- a stated remainder that is no longer "
            "a remainder hides the next one.",
            file=sys.stderr,
        )
        return 1
    print(
        f"lint_flavors: every tracked Go file is linted, except the {len(blind)} "
        "stated above."
    )
    return 0


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--scope",
        default="./...",
        help="package patterns to lint (default the whole tree)",
    )
    parser.add_argument(
        "--coverage",
        action="store_true",
        help="print the files no pass loads and run no linter",
    )
    parser.add_argument("--list", action="store_true", help="print the flavor table")
    args, extra_args = parser.parse_known_args()

    if args.list:
        for flavor in BASE_PASSES + FLAVORS:
            added = ",".join(flavor["tags"]) or "-"
            removed = ",".join(flavor.get("without", ())) or "-"
            print(
                f"{flavor['name']:<18} GOOS={flavor['goos'] or 'host':<8} "
                f"GOARCH={flavor['goarch'] or 'host':<8} add={added} drop={removed}"
            )
        return 0

    patterns = args.scope.split()
    table, seen = scopes(patterns)

    if args.coverage:
        return report_coverage(seen)

    failed = []
    try:
        for flavor, scope in table:
            if not scope:
                continue
            if lint(flavor, scope, extra_args) != 0:
                failed.append(flavor["name"])
    finally:
        derived = os.path.join(ROOT, "tmp", "lint-flavors", f"notags-{os.getpid()}.yml")
        if os.path.exists(derived):
            os.remove(derived)

    # Coverage is asserted only for a whole-tree run. Over a scoped set the
    # question has no answer: a file outside the scope is absent because the
    # caller said so, not because no flavor selects it.
    if patterns == ["./..."] and report_coverage(seen) != 0:
        failed.append("coverage")

    if failed:
        print(f"lint_flavors: failed: {', '.join(failed)}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
