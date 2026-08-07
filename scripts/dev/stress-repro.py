#!/usr/bin/env python3
"""Reproduce load-dependent / flaky-under-concurrency functional-test failures
WITHOUT running the full functional suite.

Some Ze failures only appear under the scheduling and GC pressure of a full
`make ze-functional-test` run (all ~22 suites on all cores). Running one suite
in isolation never triggers them (e.g. the `rsvpte-lsp` boot-frame panic:
`slice bounds out of range [:5448] with capacity 512`, 0/40 in isolation but
~1/4 in full verify). The full suite is far too slow to loop while hunting such
a bug.

This tool recreates that pressure cheaply: it pegs every core with CPU + GC
churn ("burners") and runs MANY concurrent copies of a single target suite in a
loop, capturing the FIRST failure's complete, untruncated output (panic stack,
race report) instead of the 2-line summary the verify aggregator keeps.

It also makes the failure SELF-REPORT:
  * GOTRACEBACK=all -> a panic dumps every goroutine stack, so the goroutine
    racing on the corrupt buffer is captured alongside the crashing one.
  * --race          -> builds a race-instrumented ze; a genuine data race then
    prints the two conflicting accesses (file:line) directly.

Usage:
  python3 scripts/dev/stress-repro.py <suite> [options]

Examples:
  # Hunt the rsvpte boot panic under heavy load, capture the stack:
  python3 scripts/dev/stress-repro.py rsvpte --iterations 80

  # Same, but race-instrumented so a data race self-reports its two accesses:
  python3 scripts/dev/stress-repro.py rsvpte --race --iterations 40

  # A specific test only, lighter load:
  python3 scripts/dev/stress-repro.py rsvpte --test 4 --burners 8 --parallel 2

  # A sub-suite: <suite> and --test are both split on whitespace, so the tokens
  # reach ze-test exactly as you would type them by hand:
  python3 scripts/dev/stress-repro.py "bgp plugin" --test 97 --any-failure

By default only a CRASH (panic / data race / runtime error) counts as a
reproduction. Assertion-failure flakes -- a test whose `expect=` pattern is
merely missed under load -- exit non-zero without any crash signature, so pass
--any-failure to capture those too.

Exit status: 0 = reproduced (details in the saved log), 1 = not reproduced,
2 = setup error (missing binaries, build failure).

See ai/tools/stress-repro.md for the full guide and when to reach for this.
"""

import argparse
import os
import re
import shlex
import subprocess
import sys
import time
from concurrent.futures import ThreadPoolExecutor, as_completed
from multiprocessing import Process

REPO = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))


def _feature_gate_tags():
    """Sorted ze_<feature> build tags from feature-gates.txt (the single source of
    truth). Derived, not hardcoded, so a race build tracks ZE_FEATURES automatically
    when a gate is added -- see ai/rules/plugins.md."""
    tags = set()
    with open(os.path.join(REPO, "feature-gates.txt"), encoding="utf-8") as fh:
        for line in fh:
            line = line.strip()
            if not line or line.startswith("#"):
                continue
            tags.add(line.split()[0])
    return sorted(tags)


# Signatures that mean "the daemon (or runner) crashed", not a normal assert.
CRASH_SIGNATURES = (
    "slice bounds out of range",
    "panic:",
    "fatal error:",
    "DATA RACE",
    "runtime error:",
    "index out of range",
    "invalid memory address",
    "nil pointer dereference",
)


def _burn(deadline):
    """One burner process: CPU spin + unbounded-then-trimmed allocation to keep
    the Go GC and OS scheduler under real pressure (a pure spin loop does not
    churn memory, which is what widens buffer-reuse race windows)."""
    x = 0
    sink = []
    while time.time() < deadline:
        for _ in range(50_000):
            x = (x * 1103515245 + 12345) & 0x7FFFFFFF
        sink.append(bytearray(64 * 1024))
        if len(sink) > 96:  # ~6 MB, then drop half -> steady GC churn
            del sink[: len(sink) // 2]
    return x


def build_race_ze(tags, out_path):
    """Build a race-instrumented, full-feature ze (CGO required for -race)."""
    ldflags = "-X main.version=stress -X main.buildDate=stress"
    cmd = [
        "go",
        "build",
        "-race",
        "-tags",
        tags,
        "-ldflags",
        ldflags,
        "-o",
        out_path,
        "./cmd/ze",
    ]
    env = dict(os.environ, CGO_ENABLED="1")
    print(f"building race ze: {' '.join(cmd)}", flush=True)
    r = subprocess.run(cmd, cwd=REPO, env=env, capture_output=True, text=True)
    if r.returncode != 0:
        sys.stderr.write(r.stdout + r.stderr)
        return False
    return True


def _bin_from_env(key, default):
    """Binary path from the environment, falling back to the repo's bin/<name>.

    Ze env keys are dot/underscore agnostic and case-insensitive (internal/core/env),
    so `ze.bin` and `ZE_BIN` are the same setting; accept both spellings.

    This MUST honour the environment. Under an AI session every canonical binary is
    built session-suffixed (mk/session.mk) and the functional make targets run against
    an isolated pair under tmp/ (mk/test-functional.mk), exporting ZE_BIN/ZE_TEST_BIN
    to point at it. Hardcoding bin/ze made this tool silently stress a STALE binary:
    a fix under test looked "still reproducing" because the run never contained it,
    which is the same false-red class ai/rules/commands.md documents for bare
    `go test` and for launching the runner binary directly.
    """
    for name in (key, key.replace(".", "_").upper()):
        if os.environ.get(name):
            return os.path.abspath(os.environ[name])
    return default


def ensure_binaries(ze_bin, test_bin):
    missing = [p for p in (ze_bin, test_bin) if not os.path.isfile(p)]
    if not missing:
        return True
    print(
        f"missing prebuilt binaries: {missing}\n"
        f"  build them first, e.g.: make ze-functional-test (or make bin/ze-test)",
        file=sys.stderr,
    )
    return False


def run_slug(suite, sel):
    """A single filename component naming this run's suite and selector.

    A selector is whatever ze-test accepts, and ai/rules/testing.md tells you to
    prefer a NAME over a numeric id for anything you keep -- names being stable
    where positions renumber. Test names in that form carry `/`
    (test/web/commit-flow.wb), so joining the raw tokens produced a path with
    directories in it and the tool died on `open()` with FileNotFoundError,
    refusing exactly the selector the rules ask for.
    """
    parts = shlex.split(suite) + shlex.split(sel or "")
    slug = "-".join(re.sub(r"[^A-Za-z0-9._-]+", "-", p).strip("-") for p in parts if p)
    return slug.strip("-") or "run"


def _as_text(stream):
    """Return a subprocess stream as str, whatever subprocess handed us.

    capture_output+text=True yields str on the success path but bytes inside a
    TimeoutExpired, and None when a stream was never produced.
    """
    if stream is None:
        return ""
    if isinstance(stream, (bytes, bytearray)):
        return bytes(stream).decode("utf-8", "replace")
    return stream


# A ze-test invocation that never reached a test at all. Counting one of these as
# a reproduction is worse than useless: it burns the whole run and records a
# usage mistake as a product failure. Seen with `stress-repro.py reload`, where
# the reload tests actually live under the bgp suite (`bgp reload`).
#
# These strings are NOT unique to a usage error: several .ci tests assert them as
# expected output (test/ui/root-namespace.ci and test/ui/pipe-operators.ci both
# do `expect=stderr:contains=unknown command: ...`), and the runner echoes both
# the needle and the daemon's stderr into a failure report. So the check is
# deliberately narrow -- see usage_error_signature.
USAGE_SIGNATURES = (
    "unknown command:",
    "unknown suite:",
    "flag provided but not defined",
)

# ze-test prints its command list ONLY when it never dispatched a suite. Pairing
# this with a USAGE_SIGNATURES match is what distinguishes "bad arguments" from
# "a real failure whose output happens to contain the phrase".
USAGE_BANNER = "\nCommands:\n"


def usage_error_signature(out):
    """Return the usage signature in `out` when the run never reached a test.

    A usage signature ALONE is not enough, and keying on the invocation ordinal
    is not either. Several .ci tests legitimately produce these phrases:
    test/ui/root-namespace.ci asserts `expect=stderr:contains=unknown command:
    traffic-control`, and on failure the runner echoes both the unmet needle and
    ze's stderr into the report. Discarding that as "you mistyped the suite
    name" would throw away the very reproduction the tool exists to capture.

    So require the ze-test USAGE BANNER as well. ze-test prints the signature
    followed by "Commands:" and the full command list only when it never
    dispatched a suite; a run that reached a test never prints it. That is a
    property of "no test ran", not of ordering, so it also holds when a later
    parallel future completes first.
    """
    if USAGE_BANNER not in out:
        return None
    return next((s for s in USAGE_SIGNATURES if s in out), None)


def run_once(suite, sel, ze_bin, test_bin, timeout, extra_tags):
    """One `ze-test <suite> <sel> -v` invocation with prebuilt binaries and
    full-goroutine tracebacks. Returns (returncode, combined_output)."""
    env = dict(os.environ)
    env["ze.bin"] = ze_bin
    env["ze.test.bin"] = test_bin
    env["ZE_TEST_NO_BUILD"] = "1"
    env["GOTRACEBACK"] = "all"  # every goroutine on panic -> the racer shows up
    if extra_tags:
        env["ze.tags"] = extra_tags
    # suite and sel are whitespace-split so a sub-suite ("bgp plugin") and a
    # multi-token selector reach ze-test as separate argv entries.
    # -v goes BEFORE the selector: the runners parse `<suite> [options] [tests...]`
    # and treat anything after the first positional as another test id.
    args = [test_bin, *shlex.split(suite), "-v"]
    if sel:
        args.extend(shlex.split(sel))
    else:
        args.append("--all")
    try:
        r = subprocess.run(
            args, cwd=REPO, env=env, capture_output=True, text=True, timeout=timeout
        )
        return r.returncode, _as_text(r.stdout) + _as_text(r.stderr)
    except subprocess.TimeoutExpired as e:
        # TimeoutExpired carries the raw, UNDECODED streams even though the call
        # above passes text=True: subprocess only decodes on the success path.
        # Concatenating those bytes with a str raised TypeError and killed the
        # whole run, so the reproducer crashed on precisely the failure it exists
        # to catch -- a suite that hangs under load.
        return 124, _as_text(e.stdout) + _as_text(e.stderr) + (
            f"\n[stress-repro: invocation timed out after {timeout}s]\n"
        )


def main():
    ap = argparse.ArgumentParser(
        description="Load-stress reproducer for flaky-under-concurrency test failures"
    )
    ap.add_argument("suite", help="functional suite name (e.g. rsvpte, bgp, ospf)")
    ap.add_argument(
        "--test", dest="sel", default="", help="specific test selector (default: --all)"
    )
    ap.add_argument(
        "--iterations", type=int, default=80, help="max target invocations (default 80)"
    )
    ap.add_argument(
        "--parallel",
        type=int,
        default=0,
        help="concurrent invocations per round (default: NCPU//2)",
    )
    ap.add_argument(
        "--burners",
        type=int,
        default=0,
        help="CPU+GC burner processes (default: 2*NCPU)",
    )
    ap.add_argument(
        "--minutes", type=float, default=20.0, help="wall-clock cap (default 20)"
    )
    ap.add_argument(
        "--timeout",
        type=int,
        default=120,
        help="per-invocation timeout seconds (default 120)",
    )
    ap.add_argument(
        "--race", action="store_true", help="build+use a race-instrumented ze"
    )
    ap.add_argument(
        "--any-failure",
        action="store_true",
        help="treat ANY non-zero exit as a reproduction, not just a crash "
        "signature (needed for assertion-failure flakes, which never panic)",
    )
    ap.add_argument("--tags", default="", help="extra ze.tags for the runner build")
    args = ap.parse_args()

    ncpu = os.cpu_count() or 8
    parallel = args.parallel or max(2, ncpu // 2)
    nburn = args.burners if args.burners else 2 * ncpu

    ze_bin = _bin_from_env("ze.bin", os.path.join(REPO, "bin", "ze"))
    test_bin = _bin_from_env("ze.test.bin", os.path.join(REPO, "bin", "ze-test"))

    if args.race:
        # Mirror the runner's full-feature tag set (ze_core/ze_distro/ze_setup base +
        # ZE_FEATURES, derived from feature-gates.txt) so the command registry (hence
        # the boot dump size) matches a real functional run.
        race_tags = " ".join(
            ["ze_core", "ze_distro", "ze_setup"] + _feature_gate_tags()
        )
        if args.tags:
            race_tags += " " + args.tags
        ze_bin = os.path.join(REPO, "bin", "ze-race")
        if not build_race_ze(race_tags, ze_bin):
            print("race build failed", file=sys.stderr)
            return 2

    if not ensure_binaries(ze_bin, test_bin):
        return 2

    ts = time.strftime("%Y%m%d-%H%M%S")
    outdir = os.path.join(REPO, "tmp", "stress-repro")
    os.makedirs(outdir, exist_ok=True)
    logpath = os.path.join(outdir, f"{run_slug(args.suite, args.sel)}-{ts}.log")

    deadline = time.time() + args.minutes * 60
    print(
        f"stress-repro: suite={args.suite} sel={args.sel or '--all'} "
        f"burners={nburn} parallel={parallel} iterations={args.iterations} "
        f"race={args.race} ncpu={ncpu}",
        flush=True,
    )
    print(f"  log: {logpath}", flush=True)

    burners = [
        Process(target=_burn, args=(deadline,), daemon=True) for _ in range(nburn)
    ]
    for p in burners:
        p.start()

    reproduced = False
    done = 0
    try:
        with open(logpath, "w") as log:
            log.write(
                f"stress-repro {args.suite} {ts}\n"
                f"burners={nburn} parallel={parallel} race={args.race} ncpu={ncpu}\n"
            )
            log.flush()
            with ThreadPoolExecutor(max_workers=parallel) as pool:
                while (
                    done < args.iterations and time.time() < deadline and not reproduced
                ):
                    batch = min(parallel, args.iterations - done)
                    futs = {
                        pool.submit(
                            run_once,
                            args.suite,
                            args.sel,
                            ze_bin,
                            test_bin,
                            args.timeout,
                            args.tags,
                        ): j
                        for j in range(batch)
                    }
                    for fut in as_completed(futs):
                        done += 1
                        rc, out = fut.result()
                        if usage := usage_error_signature(out):
                            log.write(f"\n===== invocation {done} USAGE ERROR =====\n")
                            log.write(out)
                            log.flush()
                            print(
                                f"\nstress-repro: '{args.suite}' never reached a test "
                                f"({usage.strip()}). Not a reproduction.\n"
                                f"Check the suite name -- a sub-suite is passed as one "
                                f'argument, e.g. "bgp reload".',
                                flush=True,
                            )
                            return 2
                        crash = next((s for s in CRASH_SIGNATURES if s in out), None)
                        hit = crash or (args.any_failure and rc != 0)
                        log.write(
                            f"\n===== invocation {done} exit={rc} "
                            f"{'CRASH:' + crash if crash else ('FAIL' if rc else 'ok')}"
                            f" =====\n"
                        )
                        if hit:
                            log.write(out)
                            log.flush()
                            reproduced = True
                            what = repr(crash) if crash else f"non-zero exit {rc}"
                            print(
                                f"\n*** REPRODUCED on invocation {done} "
                                f"(exit {rc}, signature: {what}) ***",
                                flush=True,
                            )
                            if crash:
                                _print_crash_excerpt(out)
                            break
                        else:
                            log.write(out[-500:] + "\n")
                        log.flush()
    finally:
        for p in burners:
            p.terminate()
        for p in burners:
            p.join(timeout=5)

    if reproduced:
        print(f"full capture: {logpath}", flush=True)
        return 0
    print(
        f"not reproduced in {done} invocation(s) under load "
        f"(try --race, more --burners, or higher --parallel). log: {logpath}",
        flush=True,
    )
    return 1


def _print_crash_excerpt(out):
    """Print the lines around the first crash signature so the site is visible
    without opening the full log."""
    lines = out.splitlines()
    idx = next(
        (i for i, ln in enumerate(lines) if any(s in ln for s in CRASH_SIGNATURES)),
        None,
    )
    if idx is None:
        return
    lo = max(0, idx - 2)
    hi = min(len(lines), idx + 40)
    print("--- crash excerpt ---", flush=True)
    for ln in lines[lo:hi]:
        print("  " + ln, flush=True)
    print("--- end excerpt ---", flush=True)


if __name__ == "__main__":
    sys.exit(main())
