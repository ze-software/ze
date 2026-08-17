#!/usr/bin/env python3
"""Suggest a perf run when hot-path code has changed since the last one.

A NUDGE, never a gate: it prints a suggestion and always exits 0, so it can sit
in the dev loop without ever blocking a build. It exists because the real perf
suite (`make ze-evidence-perf-record`) needs Docker and minutes, so it is not run every
edit -- and a throughput/convergence regression in the BGP data plane is exactly
the kind that slips in silently between perf runs.

Deliberately LOCAL, and complementary to CI. The scheduled Docker-free regression
check (.github/workflows/perf-nightly.yml) guards the committed NDJSON history on
every nightly; this nudge runs on the machine doing the work and asks the
developer to refresh that history with a full Docker perf run when hot-path code
changed since the last one. Keeping the heavy Docker sweep local -- rather than a
forge cron on donated shared runners -- is the point: the need is reproducible on
the developer's own machine, so that is where it is detected.

Detection mirrors changed-pkgs.sh, but baselined on the last PERF run rather than
the last verify:

    a perf-sensitive .go file is "uncovered" if it changed in the working tree,
    OR in a commit made since the SHA recorded by the last perf run.

`--record` writes that baseline (current HEAD). The perf make targets call it, so
a real perf run clears the suggestion. With no baseline yet, everything counts as
uncovered -- correct, because perf has never been run here.

Usage:
    perf-suggest.py            # print a suggestion if warranted; exit 0 always
    perf-suggest.py --record   # record the current HEAD as "perf ran here"
"""

from __future__ import annotations

import subprocess
import sys
from pathlib import Path

# Hot-path prefixes the Docker perf DUT actually measures (BGP convergence /
# throughput / p99) plus the harness itself. This list is a suggestion filter,
# not a gate -- a wrong entry only over- or under-suggests, it can never fail a
# build. But the failure that matters for an ADVISORY is UNDER-coverage: a
# silent nudge on a real regression. So the list is anchored to what the sole
# ze perf config (test/perf/configs/ze.conf) exercises, not to a guess.
#
# That config sets `rs-fast-path enable` on every peer, so the measured path
# runs through the route-server plugin's batch forwarder
# (plugins/rs/server_forward.go -> reactor ForwardCached). Omitting plugins/rs/
# left the nudge silent on exactly the throughput regression the run measures
# (found in review). The engine RIB/store/fsm and adj_rib_in sit on the
# convergence path the same config drives.
#
# Extend it when a new data-plane package joins the measured path; the
# alloc-gate benchmarks (mk/alloc-gate.mk) are the narrower, enforced companion.
HOT_PATH_PREFIXES = (
    "internal/component/bgp/reactor/",
    "internal/component/bgp/wireu/",
    "internal/component/bgp/message/",
    "internal/component/bgp/attrpool/",
    "internal/component/bgp/plugins/rs/",  # route-server fast path (config enables it)
    "internal/component/bgp/plugins/rib/",
    "internal/component/bgp/plugins/adj_rib_in/",
    "internal/component/bgp/rib/",  # engine RIB: commit/incoming/update
    "internal/component/bgp/store/",  # per-attribute dedup interning
    "internal/component/bgp/fsm/",  # session establishment / convergence
    "internal/core/bgp/",  # wire, attribute, capability, nlri
    "internal/perf/",
)

MARKER = Path("tmp/.ze-perf-lastrun")


def _git(*args: str) -> str:
    try:
        return subprocess.run(
            ["git", *args], capture_output=True, text=True, check=False
        ).stdout
    except OSError:
        return ""


def _head() -> str:
    return _git("rev-parse", "HEAD").strip()


def _is_hot(path: str) -> bool:
    return path.endswith(".go") and path.startswith(HOT_PATH_PREFIXES)


def record() -> int:
    MARKER.parent.mkdir(parents=True, exist_ok=True)
    MARKER.write_text((_head() or "unknown") + "\n", encoding="utf-8")
    return 0


def _reachable(sha: str) -> bool:
    return (
        subprocess.run(
            ["git", "cat-file", "-e", sha + "^{commit}"],
            capture_output=True,
            check=False,
        ).returncode
        == 0
    )


def _baseline() -> str | None:
    """The commit that committed changes are measured against, in priority order.

    1. The last recorded perf run (the marker) -- the whole point: perf ran here.
    2. Else the merge-base with the upstream branch -- "what this branch has
       developed and never perf-tested". Without this a hot change went silent
       the moment it was committed before any perf run, so the nudge forgot the
       exact work it should flag.
    3. Else nothing: no trusted point, so working-tree changes only. Quiet and
       safe on a detached HEAD or an untracked branch, never noisy on a fresh
       clone.
    """
    if MARKER.exists():
        m = MARKER.read_text(encoding="utf-8").strip()
        if m and m != "unknown" and _reachable(m):
            return m
    upstream = _git("rev-parse", "--abbrev-ref", "@{upstream}").strip()
    if upstream:
        mb = _git("merge-base", "HEAD", upstream).strip()
        if mb and _reachable(mb):
            return mb
    return None


def changed_hot(base: str | None) -> list[str]:
    """Hot-path .go files changed in the working tree, or committed since `base`.

    Takes an already-resolved baseline so suggest() -- which also needs the
    baseline for its message -- resolves _baseline() once instead of running
    merge-base/cat-file twice.
    """
    files: set[str] = set()
    # Working tree: unstaged, staged, untracked.
    for args in (
        ("diff", "--name-only"),
        ("diff", "--cached", "--name-only"),
        ("ls-files", "--others", "--exclude-standard"),
    ):
        files.update(ln for ln in _git(*args).splitlines() if ln)
    if base:
        files.update(
            ln for ln in _git("diff", "--name-only", base + "..HEAD").splitlines() if ln
        )
    return sorted(f for f in files if _is_hot(f))


def suggest() -> int:
    base = _baseline()
    hot = changed_hot(base)
    if not hot:
        return 0

    if MARKER.exists() and base:
        origin = f"last perf run {base[:12]}"
    elif base:
        origin = f"branch merge-base {base[:12]} (perf never recorded here)"
    else:
        origin = "working tree (perf never recorded here)"
    print("", file=sys.stderr)
    print(
        "perf-suggest: BGP data-plane code changed vs " + origin + "."
        " Consider a perf run before relying on these:",
        file=sys.stderr,
    )
    for f in hot[:12]:
        print(f"  {f}", file=sys.stderr)
    if len(hot) > 12:
        print(f"  ... and {len(hot) - 12} more", file=sys.stderr)
    print(
        "  Run:  make ze-evidence-perf-record   (Docker; records the baseline so this clears)\n"
        "  This is advisory -- it never blocks a build.",
        file=sys.stderr,
    )
    return 0


def main() -> int:
    if "--record" in sys.argv[1:]:
        return record()
    return suggest()


if __name__ == "__main__":
    sys.exit(main())
