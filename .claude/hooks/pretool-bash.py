#!/usr/bin/env -S python3 -S
"""Single consolidated PreToolUse check for the Bash tool.

Replaces eleven separate shell hooks that each spawned `bash` + `jq` on every
Bash command. This reads the hook payload once and runs every check in-process,
so a normal command (grep, ls, go test, ...) pays one Python start instead of
~11 shell+jq process spawns.

Ported faithfully from (and replaces, for the Bash matcher only):
    block-worktree-copy.sh   block-destructive-git.sh   block-root-build.sh
    block-pipe-tail.sh       block-system-tmp.sh        block-test-deletion.sh

The commit-time gates (pre-commit-spec-audit / check-deferral-in-diff /
check-deferral-unassigned / check-wiring-at-commit / check-doc-drift) that used
to live here were re-homed to scripts/dev/commit_helper.py creation-time gates
(spec-followup-hooks): here they gated on the literal "git commit" string, which
the sanctioned commit path (bash tmp/commit-<SID>.sh) never contains and
check_destructive_git blocks when it does, so they ran only on a dead path.

NOT folded in (left as their own hook entries because they are not Bash-only):
    block-until-lsp.sh       -- matcher ".*", gates every tool, not just Bash
    golangci-lint            -- matcher "Bash(git commit:*)"
block-system-tmp.sh and block-test-deletion.sh keep their separate Write|Edit
entries; only their Bash behaviour is reproduced here.

Exit codes (Claude Code PreToolUse semantics):
    0 = allow   1 = non-blocking warning   2 = block the command
When several checks fire, all messages are emitted and the most severe code wins.
On an unexpected internal error the hook fails OPEN (exit 0) so a bug here can
never brick every Bash command; the traceback is printed to stderr.
"""

import json
import os
import re
import sys

# ANSI colours, matched to the original shell hooks.
RED = "\033[31m"
YELLOW = "\033[33m"
BOLD = "\033[1m"
RESET = "\033[0m"

# A check returns None to pass, or (code, message) where code is 1 or 2.
#
# A `ze point:` comment directly above a check names the rule point it enforces
# (`<rule-stem>/<slug>` under ai/rules/points/), or `none -- <why>`. Joined by
# `make ze-rules-gate-map`, which fails on a point that does not exist.

# --------------------------------------------------------------------------- #
# Always-run string checks (run on every Bash command).
# --------------------------------------------------------------------------- #


# ze point: none -- the worktree prohibition is in ai/INSTRUCTIONS.md, which is not a rule under ai/rules/
def check_worktree_copy(cmd, _ctx):
    """block-worktree-copy.sh: no cp/mv/rsync/redirect out of .claude/worktrees."""
    worktree = ".claude/worktrees"
    if not cmd or worktree not in cmd:
        return None
    for token in ("cp ", "cp -", "mv ", "mv -", "rsync ", "install "):
        if token in cmd:
            return _worktree_block()
    if re.search(r"\s>", cmd):  # redirection out of a worktree file
        return _worktree_block()
    return None


def _worktree_block():
    return (
        2,
        "❌ Blocked: copying files from worktree to main repo\n"
        "Worktree agents must commit their changes. Use git merge or cherry-pick.\n"
        "Direct file copying overwrites uncommitted work from other sessions.",
    )


# ze point: git-safety/before-destructive-actions/never-run-a-destructive-git-verb
def check_destructive_git(cmd, _ctx):
    """block-destructive-git.sh: refuse destructive git verbs from Bash."""
    if "git restore --staged" in cmd:  # unstaging is safe
        return None
    patterns = (
        "git commit",
        "git push",
        "git reset",
        "git checkout --",
        "git checkout -f",
        "git checkout HEAD",
        "git restore",
        "git revert",
        "git stash drop",
        "git stash clear",
        "git clean",
        "git push --force",
        "git push -f",
        "git merge",
    )
    for pattern in patterns:
        if pattern in cmd:
            return (2, f"❌ Blocked: {pattern} (run manually)")
    return None


# ze point: none -- build hygiene, and no rule states where a Go binary lands
def check_root_build(cmd, _ctx):
    """block-root-build.sh: no `go build` that drops a binary in the repo root."""
    if not re.search(r"(^|[^A-Za-z0-9_])go\s+build([^A-Za-z0-9_]|$)", cmd):
        return None
    if re.search(r"-o\s+bin/", cmd):
        return None
    # This session's own bin/, tmp/s/<session-id>/bin/ (internal/test/sessionpath).
    # Same intent as bin/: a real binary directory, not the repo root -- and it is
    # swept at SessionEnd. The trailing bin/ is required, because ze derives its
    # config/DB dir from a parent dir named bin (internal/core/paths/paths.go).
    if re.search(r"-o\s+tmp/s/[A-Za-z0-9._-]+/bin/", cmd):
        return None
    if re.search(r"go\s+build\s+\./\.\.\.", cmd):
        return None
    if re.search(r"go\s+build\s+(-[A-Za-z0-9_]+\s+)*\./\.\.\.", cmd):
        return None
    return (
        2,
        f"{RED}{BOLD}✘ BLOCKED: go build without -o bin/{RESET}\n\n"
        f"  {RED}→{RESET} Use: go build -o bin/<name> ./cmd/<name>\n"
        f"  {RED}→{RESET} Or: make ze / make chaos / make test-runner",
    )


# ai/rules/commands.md scopes this to EXPENSIVE producers: "Never pipe `make`,
# `go test`, `go build`, `golangci-lint`, `bin/ze*`, or any test/verify/build
# command through `head`, `tail`, `grep`, `awk`, `sed`, `cat`." The reason is
# specific to those: their output is the evidence, truncating it fakes a green,
# and re-running to see the rest costs minutes.
# Matched against a COMMAND WORD, never against the whole statement. Matching
# anywhere made `git diff scripts/dev/foo-check.py | head` an "expensive command",
# so merely READING ABOUT a gate was blocked -- the same false-positive tax the
# per-statement scoping was introduced to remove.
EXPENSIVE_COMMAND = re.compile(
    r"^("
    r"make|"
    r"golangci-lint|"
    r"pytest|"
    r"ze-test|"
    r"(\./)?bin/ze[\w-]*|"
    # "or any test/verify/build command" (ai/rules/commands.md): the repo's own
    # gates, whose output IS the verdict. Everything under scripts/evidence/ counts
    # (QEMU boots, docker interop labs); elsewhere it is by role in the filename, so
    # cheap utilities (spec-session.sh, session-scratch.sh) stay usable.
    r"(\./)?scripts/evidence/[\w./-]+|"
    r"(\./)?scripts/(dev|checks|docvalid|status)/[\w./-]*"
    r"(check|verify|test|audit|lint|stress|repro)[\w./-]*\.(py|sh|go)"
    r")$"
)
# Cheap probes that the role-in-filename heuristic would otherwise catch. These
# are status readers, not gates: `verify-status.sh check` is one line in ~0.00s and
# CLAUDE.md tells every session to run it before committing.
CHEAP_SCRIPTS = {
    "scripts/dev/verify-status.sh",
    "scripts/dev/verify-lock.sh",
    "scripts/dev/verify-summary.sh",
    "scripts/dev/spec-closure-check.py",
}
# `go`/`cargo`-style: expensive only for certain subcommands.
EXPENSIVE_SUBCOMMAND = {"go": {"test", "build", "vet", "run"}}
# Wrappers that delegate to the real command word.
LAUNCHERS = {
    "python3",
    "python",
    "bash",
    "sh",
    "perl",
    "ruby",
    "sudo",
    "time",
    "nice",
    "env",
    "timeout",
}
LOSSY_FILTER = re.compile(r"^\s*(head|tail|grep|egrep|fgrep|awk|sed|cat|less|more)\b")
# `timeout` and `nice` are the two launchers that take operands of their OWN before
# the real command word, so reaching the command means consuming those first.
# Options that take a SEPARATE argument (`timeout -k 5 30 make ...`,
# `nice -n 5 make ...`): the flag and its value both sit in front.
LAUNCHER_OPT_WITH_ARG = {"-k", "--kill-after", "-s", "--signal", "-n", "--adjustment"}
# The duration/niceness operand itself. `timeout` accepts a unit suffix and
# sessions write one (`timeout <n>s make ze-verify`) -- the old bare-`isdigit()`
# test did not match it, so that invocation slipped straight past this gate.
# No duration is named here on purpose: ai/rules/git-safety.md ("Running
# ze-verify") sets the policy, and a number copied into a comment is the drift
# this repo keeps paying for (plan/learned/1359). A bare
# negative niceness (`nice -5 make ...`) is a flag and the operand at once, hence
# the optional leading `-`.
LAUNCHER_OPERAND = re.compile(r"^-?\d+(?:\.\d+)?[smhd]?$")


def _strip_launcher_operands(tokens):
    """Drop a `timeout`/`nice` launcher's own flags and duration/niceness operand.

    Returns `tokens` positioned at the command word the launcher will run.
    """
    while tokens:
        head = tokens[0]
        if head in LAUNCHER_OPT_WITH_ARG:
            tokens = tokens[2:] if len(tokens) > 1 else tokens[1:]
            continue
        if LAUNCHER_OPERAND.match(head):
            return tokens[1:]  # exactly one operand, then the command word
        if head.startswith("-"):
            tokens = tokens[1:]  # a valueless flag (--preserve-status, -v, ...)
            continue
        return tokens  # already at the command word
    return tokens


def _is_expensive(segment):
    """True when this pipeline segment's COMMAND is an expensive producer."""
    tokens = segment.split()
    while tokens and (
        tokens[0] in LAUNCHERS
        or "=" in tokens[0].split("/")[0]
        and not tokens[0].startswith("-")
    ):
        launcher = tokens[0]
        tokens = tokens[1:]
        if launcher in ("timeout", "nice"):
            tokens = _strip_launcher_operands(tokens)
    while tokens and tokens[0].startswith("-"):
        tokens = tokens[1:]
    if not tokens:
        return False
    cmd = tokens[0]
    if cmd.lstrip("./") in CHEAP_SCRIPTS:
        return False
    if cmd in EXPENSIVE_SUBCOMMAND:
        return len(tokens) > 1 and tokens[1] in EXPENSIVE_SUBCOMMAND[cmd]
    return bool(EXPENSIVE_COMMAND.match(cmd))


# ze point: commands/no-pipes-on-expensive-commands/never-pipe-an-expensive-command-read-the-log
# ze point: commands/directives/run-commands-through-make-and-never-poll
def check_pipe_tail(cmd, _ctx):
    """block-pipe-tail.sh: no lossy pipe on an expensive command's output.

    Scoped to the producers ai/rules/commands.md names. The previous form
    blocked EVERY `| tail` (so `git log | tail -3` and `wc -l | tail -1` were
    rejected, teaching sessions to route around the hook) while letting
    `go test ./... | head -50` through, which is the exact case the rule exists
    to stop: a truncated test log reads as a pass.
    """
    # A newline is a statement boundary ONLY when it is not a continuation. Bash
    # continues a pipeline across lines after a trailing `|` or a backslash, and
    # splitting those apart puts the producer in one statement and the filter in
    # the next, so neither half trips the check. Flatten both first.
    flat = re.sub(r"\\\n", " ", cmd)
    flat = re.sub(r"\|[ \t]*\n", "| ", flat)
    # Judge each STATEMENT separately. A compound command routinely mixes a cheap
    # pipeline with an expensive command (`git status | grep x; make ze-foo`), and
    # scanning the whole string would reject it for a pipeline that has nothing to
    # do with the expensive part.
    for statement in re.split(r"&&|\|\||;|\n", flat):
        # `|&` is bash shorthand for `2>&1 |`: a real pipeline, and splitting on
        # a bare `|` leaves the filter segment starting with `&`, which the
        # anchored LOSSY_FILTER never matches.
        segments = re.split(r"\|&?", statement)
        if not any(_is_expensive(seg) for seg in segments[:-1]):
            continue  # nothing expensive is FEEDING a pipe in this statement
        for segment in segments[1:]:
            if not LOSSY_FILTER.match(segment):
                continue  # `| tee`, `| tr`, ... keep the whole stream
            return (
                2,
                "❌ Blocked: piping an expensive command's output through a lossy "
                f"filter ({segment.strip().split()[0]})\n"
                "  -- The truncated output is what you would judge the run by.\n"
                "  -- Use: make ze-verify ZE_VERIFY_LOG=tmp/ze-verify-$$.log\n"
                "  -- Or:  <command> 2>&1 | tee tmp/out-$$.log\n"
                "  -- Then: Read the log with offset/limit",
            )
    return None


# ai/rules/commands.md. A Bash command started with run_in_background
# re-invokes the session when it exits, so a loop that watches one carries no
# information the completion notification does not already carry. The harm is
# not the fork cost commands.md measures: it is the WAKE and its LIFETIME.
# A watcher ticking every few seconds competes with QEMU, Docker and ze-verify
# for the same cores, and it keeps doing so long after its reason expired.
POLL_LOOP_KEYWORD = re.compile(r"\b(while|until)\b")
# `sleep` as a CALL. The boundary class carries `.` and `/` so the Python and
# absolute-path forms count (`time.sleep(5)`, `/bin/sleep 5`); the operand class
# keeps a bare mention out (`echo no-sleep-here`).
SLEEP_CALL = re.compile(r"(?:^|[;&|(\s./])sleep\s*[\d$\"'(]")
# `pgrep` in the loop CONDITION (`until ! pgrep -f qemu`). Matching pgrep
# anywhere would reject `pgrep -f x | while read pid; do ...`, a one-shot loop
# that terminates on its own.
LOOP_PGREP = re.compile(r"\b(while|until)\s+(?:!\s*)?(?:\[\[?\s*)?pgrep\b")
# A `timeout` in front of the loop makes it die on its own, which is the whole
# property this check asks for. `timeout -k 5 300 bash -c '...'` counts. The
# lookbehind keeps a FLAG out: `go test -timeout 300s` bounds a test binary and
# bounds no loop, and crediting it disarmed the gate on a routine compound line.
TIMEOUT_BOUND = re.compile(r"(?<![-\w])timeout\s+(?:-\S+\s+)*\d+(?:\.\d+)?[smhd]?\b")
STATEMENT_SEPARATOR = re.compile(r"&&|\|\||;|\n")
# A loop keyword sitting in a SEARCH argument is text, not a loop. Without this
# the rule became unauditable from Bash: `grep -rn 'until ! pgrep' ai/rules`
# blocked, and the banned pattern is quoted in the rule and its learned summary.
SEARCH_COMMANDS = {
    "grep",
    "egrep",
    "fgrep",
    "rg",
    "ag",
    "git",
    "sed",
    "awk",
    "cat",
    "echo",
    "printf",
}


# ze point: commands/directives/run-commands-through-make-and-never-poll
def check_poll_loop(cmd, _ctx):
    """commands.md: no unbounded wait loop.

    Blocks `while`/`until` paired with a `sleep` call or a `pgrep` condition,
    unless a `timeout` bounds THAT loop. The bound is the escape on purpose: a
    poll that really is the only available signal stays one word away, and that
    word is the property that keeps it from outliving the session's interest.

    The bound is judged per loop occurrence, over the statement the keyword
    opens, never over the whole command. Crediting any earlier `timeout` made
    the guard fail open: `timeout 10 curl x; until ! pgrep -f qemu; do sleep 5;
    done` was accepted, and so was a bounded loop followed by an unbounded one.

    Two boundaries are deliberate. The check reads command TEXT, so a loop
    inside a script file (`nohup bash tmp/watch.sh &`) is out of reach by
    construction. And a loop that bounds itself in its own condition
    (`while [ $SECONDS -lt 300 ]`) is still refused, because the arithmetic is
    not decidable here: add the `timeout` and it passes.
    """
    for match in POLL_LOOP_KEYWORD.finditer(cmd):
        head = cmd[: match.start()]
        cuts = [m.end() for m in STATEMENT_SEPARATOR.finditer(head)]
        prefix = head[cuts[-1] :] if cuts else head
        words = prefix.split()
        if words and words[0].split("/")[-1] in SEARCH_COMMANDS:
            continue  # the keyword is inside a search PATTERN
        tail = cmd[match.start() :]
        end = tail.find("done")
        body = tail if end == -1 else tail[:end]
        if not SLEEP_CALL.search(body) and not LOOP_PGREP.search(tail):
            continue
        if TIMEOUT_BOUND.search(prefix):
            continue
        return (
            2,
            "❌ Blocked: unbounded wait loop (ai/rules/commands.md)\n"
            "  -- A command you started with run_in_background notifies you when it\n"
            "     exits. Waiting for one you launched needs no loop: delete it.\n"
            "  -- A poll that is the only signal must die on its own. Put the bound\n"
            "     immediately in front of the loop, in the same statement:\n"
            "     timeout 300 bash -c 'until [ -f <path> ]; do sleep 30; done'\n"
            "  -- A repeated event belongs to the Monitor tool. Leave persistent\n"
            "     false, or its timeout_ms deadline does not apply.\n"
            "  -- One watcher at a time. Each wake competes with QEMU, Docker and\n"
            "     ze-verify for the same cores.",
        )
    return None


# ze point: testing/temporary-files/use-project-tmp-for-scratch-files
def check_system_tmp(cmd, _ctx):
    """block-system-tmp.sh (Bash branch): no absolute /tmp references."""
    if not cmd:
        return None
    if re.search(r"(^|[\s='\"$(`:,])/tmp(/|\s|$)", cmd):
        return (
            2,
            "❌ Blocked: /tmp access is forbidden\n"
            "Use project tmp/ instead: tmp/<subfolder>/",
        )
    return None


# ze point: testing/directives/write-the-test-first-and-never-weaken-it
def check_test_deletion(cmd, _ctx):
    """block-test-deletion.sh (Bash branch): guard rm/checkout of test files."""
    errors = []
    if re.search(r"(^|[\s]|&&|\|)(rm|git rm)[\s]", cmd):
        if re.search(r"_test\.go", cmd) or re.search(r"\.ci", cmd):
            errors.append(f"Attempting to delete test file via: {cmd}")
        if re.search(r"rm.*-r.*test/", cmd) or re.search(
            r"rm.*-r.*internal/.*test", cmd
        ):
            errors.append(f"Attempting recursive deletion in test directory: {cmd}")
    if re.search(r"git checkout.*(_test\.go|\.ci)", cmd):
        if re.search(r"git checkout (--|[.])", cmd):
            errors.append(f"Attempting to discard test file changes: {cmd}")
    if not errors:
        return None
    lines = [f"{YELLOW}{BOLD}❓ Test deletion - user approval required{RESET}", ""]
    lines += [f"  → {e}" for e in errors]
    lines += ["", f"  {BOLD}Allow this test deletion?{RESET}"]
    return (2, "\n".join(lines))


# Order mirrors the original settings.json hook array. The four commit-gated
# checks (deferral-in-diff, deferral-unassigned, wiring-at-commit, doc-drift)
# were re-homed to scripts/dev/commit_helper.py creation-time gates: they gated
# on the literal "git commit" string, which the sanctioned commit path
# (bash tmp/commit-<SID>.sh) never contains and check_destructive_git blocks
# when it does, so they only ever fired on a dead path. See spec-followup-hooks.
CHECKS = (
    check_worktree_copy,
    check_destructive_git,
    check_root_build,
    check_pipe_tail,
    check_poll_loop,
    check_system_tmp,
    check_test_deletion,
)


def main():
    try:
        payload = json.load(sys.stdin)
    except Exception:
        return 0  # no parsable payload -> nothing to check
    if payload.get("tool_name") != "Bash":
        return 0
    cmd = (payload.get("tool_input") or {}).get("command") or ""
    project_dir = os.environ.get("CLAUDE_PROJECT_DIR")
    if not project_dir:
        project_dir = os.path.abspath(
            os.path.join(os.path.dirname(__file__), "..", "..")
        )
    ctx = {"dir": project_dir}

    worst = 0
    messages = []
    for check in CHECKS:
        try:
            result = check(cmd, ctx)
        except Exception:
            import traceback

            sys.stderr.write(
                f"[pretool-bash] {check.__name__} errored (failing open):\n"
            )
            traceback.print_exc()
            continue
        if result is None:
            continue
        code, message = result
        messages.append(message)
        worst = max(worst, code)

    if messages:
        sys.stderr.write("\n\n".join(messages) + "\n")
    return worst


if __name__ == "__main__":
    try:
        sys.exit(main())
    except Exception:
        import traceback

        traceback.print_exc()
        sys.exit(0)  # fail open: a bug here must never block every Bash command
