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

import importlib.util
import json
import os
import re
import shlex
import sys

# ANSI colours, matched to the original shell hooks.
RED = "\033[31m"
YELLOW = "\033[33m"
BOLD = "\033[1m"
RESET = "\033[0m"

# Which `tmp/` paths a session may write, shared with pretool-writeedit.py.
# Resolved relative to THIS FILE, never through CLAUDE_PROJECT_DIR: that
# variable can point at a fixture tree while the hook itself always sits in
# .claude/hooks/.
_scratch_spec = importlib.util.spec_from_file_location(
    "ze_scratch_path",
    os.path.join(os.path.dirname(os.path.abspath(__file__)), "lib", "scratch_path.py"),
)
_scratch_path = importlib.util.module_from_spec(_scratch_spec)
_scratch_spec.loader.exec_module(_scratch_path)

# Canonical session-id validator. The payload value must be accepted before it is
# copied into a shell command.
_sid_spec = importlib.util.spec_from_file_location(
    "ze_session_id",
    os.path.join(os.path.dirname(os.path.abspath(__file__)), "lib", "session_id.py"),
)
_ze_session_id = importlib.util.module_from_spec(_sid_spec)
_sid_spec.loader.exec_module(_ze_session_id)

# A check returns None to pass, or (code, message) where code is 1 or 2.
#
# A `ze point:` comment directly above a check names the rule point it enforces
# (`<rule-stem>/<slug>` under ai/rules/points/), or `none -- <why>`. Joined by
# `make ze-rules-gate-map-report`, which fails on a point that does not exist.

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
    # This session's own bin/, tmp/session/<YYYY-MM-DD>-<session-id>/bin/
    # (mk/session.mk, internal/test/sessionpath). Same intent as bin/: a real
    # binary directory, not the repo root, and one the operator can identify by
    # date. The trailing bin/ is required, because ze derives its config/DB dir
    # from a parent dir named bin (internal/core/paths/paths.go).
    if re.search(r"-o\s+tmp/session/\d{4}-\d{2}-\d{2}-[A-Za-z0-9._-]+/bin/", cmd):
        return None
    if re.search(r"go\s+build\s+\./\.\.\.", cmd):
        return None
    if re.search(r"go\s+build\s+(-[A-Za-z0-9_]+\s+)*\./\.\.\.", cmd):
        return None
    return (
        2,
        f"{RED}{BOLD}✘ BLOCKED: go build without -o bin/{RESET}\n\n"
        f"  {RED}→{RESET} Use: go build -o bin/<name> ./cmd/<name>\n"
        f"  {RED}→{RESET} Or: make ze-build / make ze-chaos-build / make test-runner",
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
    # The same binaries in this session's own directory,
    # tmp/session/<YYYY-MM-DD>-<session-id>/bin/ze* (mk/session.mk ZE_BIN_DIR,
    # internal/test/sessionpath). `make ze-session-binary-path` prints a path relative to the
    # checkout, and an agent told to use absolute paths passes the whole thing,
    # so both spellings reach this check and both name the same producer.
    r"([\w./-]*/)?tmp/session/\d{4}-\d{2}-\d{2}-[A-Za-z0-9._-]+/bin/ze[\w-]*|"
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
# sessions write one (`timeout <n>s make ze-precommit-verify`) -- the old bare-`isdigit()`
# test did not match it, so that invocation slipped straight past this gate.
# No duration is named here on purpose: ai/rules/git-safety.md ("Running
# ze-precommit-verify") sets the policy, and a number copied into a comment is the drift
# this repo keeps paying for. A bare
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
                "  -- Use: make ze-precommit-verify ZE_VERIFY_LOG=tmp/ze-verify-$$.log\n"
                '  -- Or:  dir=$(scripts/dev/session-scratch.sh); <command> 2>&1 | tee "$dir/out.log"\n'
                "  -- Then: Read the log with offset/limit",
            )
    return None


# One machine, several sessions, one checkout. Every heavy target is sized for
# the WHOLE box, so two agents starting one at the same moment oversubscribe it
# (plan/spec-shared-machine-job-admission.md). `make` routes a job through
# scripts/dev/ze-run.sh, which admits one and queues the rest; a raw invocation
# reaches the machine with nothing in front of it.
#
# The escape. A case that genuinely needs the raw form states its reason in the
# command, and the assignment is the whole cost. It exists so that nobody
# reaches for a shape this check cannot see -- a script file, a renamed binary
# (spec risk R-4). The reason is REQUIRED and lands in the transcript, so the
# escape is auditable by reading the session rather than by trusting it.
RAW_ADMIT_VAR = "ZE_ADMIT_RAW"
# golangci-lint subcommands that read configuration and run no analysis. `run`
# is the one that claims the box; `golangci-lint config verify` is how the
# linter ceiling is checked and it costs milliseconds. An absent or unknown
# subcommand counts as heavy, so a new analysis verb is refused by default.
GOLANGCI_CHEAP_SUBCOMMANDS = {
    "config",
    "version",
    "help",
    "linters",
    "cache",
    "completion",
}
# The functional runner, in the three places it is built: `bin/`, this session's
# own directory (mk/session.mk ZE_BIN_DIR), or on PATH. The arch suffix is the
# QEMU pair's spelling (`bin/ze-test-linux-arm64`). `bin/ze` is NOT here: only
# the suite runner is a heavy job.
ZE_TEST_BINARY = re.compile(
    r"^(\./)?("
    r"bin/|"
    r"([\w./-]*/)?tmp/session/\d{4}-\d{2}-\d{2}-[A-Za-z0-9._-]+/bin/"
    r")?ze-test(-[\w.-]+)?$"
)
PYTHON_INTERPRETER = re.compile(r"^(\./)?([\w./-]*/)?python[\d.]*$")
# The two shapes scripts/dev/python_tests_test.go globs (`pythonTestGlobs`).
PYTHON_TEST_FILE = re.compile(r"(^|/)(test_[\w.-]*|[\w.-]*_test)\.py$")


def _go_test_target(args):
    """Name the `make ze-unit-pkg-test` invocation that runs the same packages."""
    packages = []
    run = ""
    index = 0
    while index < len(args):
        arg = args[index]
        if arg == "-run" and index + 1 < len(args):
            run = args[index + 1]
            index += 2
            continue
        if arg.startswith("-run="):
            run = arg[len("-run=") :]
            index += 1
            continue
        if not arg.startswith("-") and (
            arg == "." or "/" in arg or arg.endswith("...")
        ):
            packages.append(arg)
        index += 1
    target = "make ze-unit-pkg-test PKG=" + (" ".join(packages) or "<package>")
    if run:
        target += " RUN=" + run.strip("'\"")
    return target


def _ze_test_suite(args):
    """The suite name a `ze-test` argument list selects.

    The runner takes `<domain> <suite>` for BGP (`bgp plugin`) and a bare suite
    everywhere else (`editor`, `web`), with test ids and flags after it.
    """
    positional = [a for a in args if not a.startswith("-") and not a.isdigit()]
    if positional[:1] == ["bgp"] and len(positional) > 1:
        return positional[1]
    return positional[0] if positional else "<suite>"


_SEGMENT_SEPARATORS = frozenset({"&&", "||", ";", "|", "|&", "&"})


def _command_segments(cmd):
    """The command TEXT of each statement, split quote-aware.

    Splitting the raw string on separators counts a newline inside a quoted
    argument as a new statement, so a paragraph of prose that happens to begin
    with a banned verb reads as an invocation of it. That is not hypothetical:
    it refused the commit whose message DESCRIBES this check, and rewording the
    message is how a phrase silently leaves the prose that most needs it. The
    same shape is recorded for check_destructive_git in
    plan/journal/gate-fires-outside-its-population.md.

    shlex collapses a quoted argument, newlines and all, into ONE token, so a
    separator inside quotes can never open a segment. An unbalanced quote is
    not a command this guard can judge, and it falls back to the raw split
    rather than failing open.
    """
    try:
        tokens = shlex.split(cmd, comments=False, posix=True)
    except ValueError:
        return [
            seg
            for st in re.split(r"&&|\|\||;|\n", cmd)
            for seg in re.split(r"\|&?", st)
        ]
    segments, current = [], []
    for token in tokens:
        if token in _SEGMENT_SEPARATORS:
            segments.append(current)
            current = []
            continue
        current.append(token)
    segments.append(current)
    return [shlex.join(seg) for seg in segments if seg]


def _raw_job(segment):
    """(what, replacement) when this pipeline segment starts a raw heavy job."""
    tokens = segment.split()
    while tokens:
        head = tokens[0]
        if "=" in head.split("/")[0] and not head.startswith("-"):
            name, _, value = head.partition("=")
            if name == RAW_ADMIT_VAR and value.strip("'\""):
                return None  # a declared reason admits the raw form
            tokens = tokens[1:]
            continue
        if PYTHON_INTERPRETER.match(head):
            for arg in tokens[1:]:
                if arg.startswith("-"):
                    continue
                if PYTHON_TEST_FILE.search(arg):
                    return (
                        f"python test `{arg}`",
                        "make ze-unit-pkg-test PKG=./scripts/dev "
                        "RUN=TestPythonUnitTests",
                    )
                break
            return None
        if head in LAUNCHERS:
            tokens = tokens[1:]
            if head in ("timeout", "nice"):
                tokens = _strip_launcher_operands(tokens)
            continue
        break
    if not tokens:
        return None
    word = tokens[0]
    base = word.split("/")[-1]
    if base == "go":
        if len(tokens) > 1 and tokens[1] == "test":
            return ("`go test`", _go_test_target(tokens[2:]))
        return None
    if base == "golangci-lint":
        subcommand = next((t for t in tokens[1:] if not t.startswith("-")), "")
        if subcommand in GOLANGCI_CHEAP_SUBCOMMANDS:
            return None
        return (
            "`golangci-lint`",
            "make ze-lint-changed   (make ze-lint for the whole tree)",
        )
    if ZE_TEST_BINARY.match(word):
        return (
            f"the functional runner `{word}`",
            f"make ze-functional-{_ze_test_suite(tokens[1:])}-test",
        )
    return None


# ze point: commands/directives/heavy-jobs-are-admitted-by-make-never-typed-raw
def check_raw_test_invocation(cmd, _ctx):
    """commands.md: a heavy job is admitted by `make`, never typed raw.

    Refuses `go test`, `golangci-lint`, the `ze-test` runner and a Python test
    file run by hand. Each refusal names the `make` target that runs the same
    work through the admission point, so the queued path is the one on screen.

    The command word is what decides, judged per statement and per pipeline
    segment. A `make` target is never refused, whatever its arguments spell
    (`make ze-qemu-debug RUN='bin/ze-test-linux-arm64 bgp parse 91 -v'`), and
    neither is a search whose PATTERN quotes a banned verb.

    Two boundaries are deliberate. Cheap subcommands stay usable, so
    `golangci-lint config verify` passes and only analysis is refused. And the
    check reads command TEXT, so a heavy job inside a script file or a
    `bash -c` string is out of reach by construction, exactly as it is for
    check_poll_loop; that is what the declared-reason escape is for, rather
    than a shape nobody can audit.
    """
    for segment in _command_segments(cmd):
        verdict = _raw_job(segment)
        if verdict is None:
            continue
        what, replacement = verdict
        return (
            2,
            f"❌ Blocked: {what} run raw, outside job admission "
            "(ai/rules/commands.md)\n"
            "  -- One machine, several sessions: make routes a heavy job\n"
            "     through scripts/dev/ze-run.sh, which runs one and queues\n"
            "     the rest. Typed raw, it lands on the box unadmitted.\n"
            f"  -- Use: {replacement}\n"
            "  -- No target fits? Queue the raw command yourself:\n"
            "     scripts/dev/ze-run.sh <label> <command>\n"
            "  -- A one-off that must not queue states its reason:\n"
            f'     {RAW_ADMIT_VAR}="bisecting one 2s case" <command>',
        )
    return None


# ai/rules/commands.md. A Bash command started with run_in_background
# re-invokes the session when it exits, so a loop that watches one carries no
# information the completion notification does not already carry. The harm is
# not the fork cost commands.md measures: it is the WAKE and its LIFETIME.
# A watcher ticking every few seconds competes with QEMU, Docker and ze-precommit-verify
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
            "     ze-precommit-verify for the same cores.",
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
            "Use this session's own scratch dir instead:\n"
            "  dir=$(scripts/dev/session-scratch.sh)   # <session-dir>/scratch/\n"
            '  <command> > "$dir/<name>"',
        )
    return None


# The /tmp block above sends every session somewhere, and that somewhere was the
# `tmp/` ROOT, which is keyed per CHECKOUT: a fixed name there collides with a
# sibling session's identical name and is never cleaned. This check closes the
# loop by naming the per-session directory at the moment the path is chosen. It
# REFUSES the command (owner decision, 2026-08-10): one session wrote 351 ad-hoc
# files to the root in a day while the warning was the whole enforcement
# (plan/journal/guard-message-teaches-the-violation.md).
#
# Which paths are refused is decided in .claude/hooks/lib/scratch_path.py, and
# c_scratch_path_we (.claude/hooks/pretool-writeedit.py) calls the same module:
# a path this surface refuses must not land through the Write tool.
#
# The path group accepts a LEADING DIRECTORY PREFIX, so `./tmp/x` and the
# absolute `/home/.../<checkout>/tmp/x` are candidates too. All three spell the
# same file, and the harness hands agents absolute paths, so anchoring on the
# literal `tmp/` was a door in the guard: the Write surface refused the absolute
# form while this one passed it. Widening the CANDIDATE cannot widen the
# REFUSAL, because is_ad_hoc_root_file decides on the resolved parent -- a
# `foo/tmp/x` or a `/elsewhere/tmp/x` resolves to a parent that is not this
# checkout's tmp/ and is allowed. EXPENSIVE_COMMAND carries the same prefix for
# the same reason.
_SCRATCH_WRITE = re.compile(
    r"(?:>>?\s*|\btee\s+(?:-a\s+)?)(?P<q>['\"]?)"
    r"(?P<path>[\w./-]*tmp/[^\s;|&)'\"]+)(?P=q)"
)


def _quoted_spans(cmd):
    """The character ranges of `cmd` that sit inside a quoted string.

    An UNTERMINATED quote opens no span, so a stray quote cannot silence the
    guard over the rest of the command.
    """
    spans = []
    quote = ""
    start = 0
    i = 0
    while i < len(cmd):
        char = cmd[i]
        if char == "\\" and (quote == "" or quote == '"'):
            i += 2
            continue
        if quote == "":
            if char in "'\"":
                quote, start = char, i
        elif char == quote:
            spans.append((start, i))
            quote = ""
        i += 1
    return spans


def _statement_command(cmd, pos):
    """The command word of the statement `pos` sits in, without its directory."""
    head = cmd[:pos]
    cuts = [m.end() for m in STATEMENT_SEPARATOR.finditer(head)]
    words = (head[cuts[-1] :] if cuts else head).split()
    return words[0].split("/")[-1] if words else ""


# A heredoc body is DATA on the command's stdin, so a `>` in it redirects
# nothing -- unless the reader is a shell, which runs what it is fed. The reader
# is the command word of the statement that opens the heredoc.
HEREDOC_OPEN = re.compile(r"<<-?\s*(?P<q>['\"]?)(?P<tag>[A-Za-z_]\w*)(?P=q)")
SHELL_COMMANDS = {"bash", "sh", "dash", "ksh", "zsh", "eval", "source"}


def _heredoc_text_spans(cmd):
    """The character ranges of `cmd` that a non-shell reads as heredoc DATA.

    A heredoc whose delimiter never appears runs to the end of the command, and
    one fed to a shell yields no span at all: `bash <<'EOF' ... > tmp/x ... EOF`
    is a write.
    """
    spans = []
    for match in HEREDOC_OPEN.finditer(cmd):
        if _statement_command(cmd, match.start()) in SHELL_COMMANDS:
            continue
        newline = cmd.find("\n", match.end())
        if newline == -1:
            continue
        body = newline + 1
        close = re.compile(
            r"^[ \t]*" + re.escape(match.group("tag")) + r"[ \t]*$", re.M
        )
        found = close.search(cmd, body)
        spans.append((body, found.start() if found else len(cmd)))
    return spans


def _is_text_not_a_write(cmd, pos, quoted, heredocs):
    """True when the redirect at `pos` writes nothing, so the guard passes it.

    Two shapes qualify. A heredoc body a non-shell reads is data, which is what
    lets a session quote this rule in a document it writes with `cat >> file`.
    And a QUOTED redirect opening a search command is a search argument, so
    `grep -rn '> tmp/out.log' ai/rules` reads the rule instead of breaking it;
    without it the ban could not be audited from Bash (check_poll_loop keeps the
    same property for its own keyword).

    The search shape needs BOTH conditions. `grep foo ai/rules > tmp/notes.txt`
    writes for real and its redirect is unquoted, so the command word alone
    cannot tell the two apart, and `bash -c "... > tmp/x"` stays refused for the
    reason F22 refuses a quoted wait loop.
    """
    if any(a <= pos < b for a, b in heredocs):
        return True
    if not any(a < pos < b for a, b in quoted):
        return False
    return _statement_command(cmd, pos) in SEARCH_COMMANDS


# ze point: commands/write-ad-hoc-scratch-under-your-per-session-dir/write-ad-hoc-scratch-under-this-session-s-private-directory
def check_scratch_path(cmd, ctx):
    """ai/rules/commands.md: ad-hoc scratch belongs under this session's own dir."""
    if not cmd:
        return None
    offenders = []
    quoted = _quoted_spans(cmd)
    heredocs = _heredoc_text_spans(cmd)
    for match in _SCRATCH_WRITE.finditer(cmd):
        path = match.group("path")
        if path in offenders or _is_text_not_a_write(
            cmd, match.start(), quoted, heredocs
        ):
            continue
        if _scratch_path.is_ad_hoc_root_file(path, ctx["dir"]):
            offenders.append(path)
    if not offenders:
        return None
    return (
        2,
        f"{RED}{BOLD}❌ Refused: the command writes ad-hoc scratch at the "
        f"tmp/ root: {', '.join(offenders)}{RESET}\n"
        "  -- tmp/ is keyed per CHECKOUT, so that name is one file for every "
        "session in this tree, and nothing removes it.\n"
        '  -- Use: dir=$(scripts/dev/session-scratch.sh); <command> > "$dir/out.log"\n'
        "  -- A subdirectory passes, and so do the root names that are shared by "
        "design: ze-verify*, commit-*, delete-*, mutation*, test-timings*\n"
        "  -- ai/rules/commands.md, 'Write Ad-Hoc Scratch Under Your Per-Session Dir'",
    )


DRAFT_DIR = "test/draft/"


def _draft_only(cmd):
    """True when every test path `cmd` names sits in the draft incubator.

    A test path is a token that carries a `test/` segment or a `_test.go` name,
    and it is normalized before it is matched. So `internal/x/y_test.go` counts <!-- doc-links: ignore (example path, deliberately absent) -->
    with no `test/` directory above it, and `test/draft/../plugin/live.ci` counts
    as the live test it reaches. The incubator root is itself a draft, which
    keeps `rm -r test/draft/` free of approval.

    `test/draft/` is gitignored and invisible to every repo-wide gate, so what
    lives there is not a test yet: it earns no coverage and proves no obligation.
    The workflow that creates a draft ends in exactly two moves, promote it or
    delete it (ai/rules/testing.md), and guarding the delete leaves the incubator
    the one directory an agent can fill but never empty.

    Requires EVERY named path to be a draft. A command mixing a draft with a real
    test still blocks, because the real one is the reason this guard exists.
    """
    targets = [
        os.path.normpath(t)
        for t in re.findall(r"[^\s'\"]*(?:test/|_test\.go)[^\s'\"]*", cmd)
    ]
    return bool(targets) and all(
        t == "test/draft" or t.startswith(DRAFT_DIR) for t in targets
    )


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
    if not errors or _draft_only(cmd):
        return None
    lines = [f"{YELLOW}{BOLD}❓ Test deletion - user approval required{RESET}", ""]
    lines += [f"  → {e}" for e in errors]
    lines += ["", f"  {BOLD}Allow this test deletion?{RESET}"]
    return (2, "\n".join(lines))


# The trees pretool-writeedit.py owns. settings.json wires that hook to
# `Write|Edit|MultiEdit|NotebookEdit` and Bash is absent from the matcher, so a
# shell write here runs NONE of its checks -- not c_design_without_lsp, not the
# plan-file placement check, none of them.
GOVERNED_TREE = r"(?:plan/|ai/rules/)"

# Tier one: the write is a shell VERB. Binding on the verb and not on the path
# is what keeps `grep plan/spec-x.md`, `sed -n '1,40p' plan/spec-x.md` and
# `commit_helper.py --file plan/spec-x.md` free -- the sanctioned commit path
# names these paths constantly and would otherwise refuse itself.
_GOVERNED_SHELL_WRITE = re.compile(
    # Each verb is anchored at a word start, or `cp` matches inside "mcp" and
    # `mv` inside any word ending in it -- "mcp spec" once blocked a plain grep.
    # The span is newline-free for the same class of reason: without it a verb
    # on one line reached a path on the NEXT command's line.
    r">>?[ \t]*[\"']?" + GOVERNED_TREE
    + r"|(?:^|[\s;&|])(?:sed|perl)[ \t]+(?:[^|;&\n]*[ \t])?-i\b[^|;&\n]*" + GOVERNED_TREE
    + r"|(?:^|[\s;&|])tee[ \t]+(?:-a[ \t]+)?[\"']?" + GOVERNED_TREE
    + r"|(?:^|[\s;&|])(?:mv|cp)[ \t]+[^|;&\n]*[ \t][\"']?" + GOVERNED_TREE
)

# Tier two: the write is inside an interpreter payload, where the shell sees no
# verb at all. This tier OVER-MATCHES on purpose. A literal-path form
# (`Path("plan/x.md").write_text`) has almost no false positives and misses the
# shape that produced this finding: a loop that writes `Path(src)` with the path
# in a variable. Matching the interpreter, a governed path and a write primitive
# anywhere in the payload catches that, at the cost of refusing a payload that
# merely READS plan/ and writes to scratch. That cost is deliberate and the
# escape below is its answer.
_GOVERNED_SCRIPT_WRITE = re.compile(
    r"(?:python3?|perl|ruby)\b[\s\S]*" + GOVERNED_TREE + r"[\s\S]*"
    r"(?:open\([^)]*[\"'][wa]|write_text\(|\.write\(|writelines\(|truncate\()"
)

# The escape, spelled like RAW_ADMIT_VAR above: a required reason, in the
# command, landing in the transcript. A false positive costs one assignment; a
# false negative costs the guard.
GOVERNED_ADMIT_VAR = "ZE_ADMIT_GOVERNED_WRITE"


# ze point: commands/directives/bash-must-not-edit-a-governed-document
def check_governed_doc_edit(cmd, _ctx):
    """Refuse a shell write to a tree pretool-writeedit.py guards.

    Auto mode tells an agent to prefer Bash for file changes, so this bypass was
    the DEFAULT route rather than an unusual one, and the guard never ran once.

    It is partial by construction: deciding whether an arbitrary shell command
    rewrites a document is undecidable, and a path assembled from variables in a
    payload this pattern cannot read still slips through. It is not fail-open
    for that -- what it misses is exactly what is missed today, and the shapes
    that occur in practice are the five below.

    The advice names Edit and not Write, because c_point_overwrite in
    pretool-writeedit.py refuses a Write over an existing rule point: offering
    both would bounce the author between two guards.
    """
    if not (_GOVERNED_SHELL_WRITE.search(cmd) or _GOVERNED_SCRIPT_WRITE.search(cmd)):
        return None
    if re.search(rf"\b{GOVERNED_ADMIT_VAR}=[\"']?\S", cmd):
        return None
    return (
        2,
        "\n".join(
            [
                f"{RED}{BOLD}❌ Blocked: shell write to plan/ or ai/rules/{RESET}",
                "",
                "  .claude/hooks/pretool-writeedit.py guards these trees, and",
                "  settings.json wires it to Write|Edit|MultiEdit|NotebookEdit",
                "  only. A shell write runs none of its checks.",
                "",
                "  → Edit the file with the Edit tool. Prefer Edit over Write:",
                "    a Write over an existing rule point is refused separately.",
                "  → Reading is untouched: grep, cat and `sed -n` stay free.",
                "  → A payload that only READS these trees and writes elsewhere",
                "    is refused too; that is the cost of catching a path built",
                "    from a variable. State the reason and it lands:",
                f'      {GOVERNED_ADMIT_VAR}="reads plan/, writes scratch" <command>',
            ]
        ),
    )


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
    check_raw_test_invocation,
    check_poll_loop,
    check_system_tmp,
    check_scratch_path,
    check_test_deletion,
    check_governed_doc_edit,
)


def main():
    try:
        payload = json.load(sys.stdin)
    except Exception:
        return 0  # no parsable payload -> nothing to check
    if payload.get("tool_name") != "Bash":
        return 0
    tool_input = payload.get("tool_input")
    if not isinstance(tool_input, dict):
        return 0
    cmd = tool_input.get("command") or ""
    project_dir = os.environ.get("CLAUDE_PROJECT_DIR")
    if not project_dir:
        project_dir = os.path.abspath(
            os.path.join(os.path.dirname(__file__), "..", "..")
        )
    ctx = {"dir": project_dir}
    parent_sid = ""
    if payload.get("agent_id") or os.environ.get("CLAUDE_CODE_FORK_SUBAGENT"):
        parent_sid = _ze_session_id._hook_session_id(payload)

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
    if worst < 2 and parent_sid and isinstance(tool_input.get("command"), str):
        updated_input = dict(tool_input)
        updated_input["command"] = (
            f"export CLAUDE_CODE_SESSION_ID={parent_sid}; {tool_input['command']}"
        )
        json.dump(
            {
                "hookSpecificOutput": {
                    "hookEventName": "PreToolUse",
                    "updatedInput": updated_input,
                }
            },
            sys.stdout,
        )
        sys.stdout.write("\n")
    return worst


if __name__ == "__main__":
    try:
        sys.exit(main())
    except Exception:
        import traceback

        traceback.print_exc()
        sys.exit(0)  # fail open: a bug here must never block every Bash command
