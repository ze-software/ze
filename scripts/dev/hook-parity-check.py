#!/usr/bin/env python3
"""Regression test for the consolidated hook dispatchers.

Three Python dispatchers replaced 58 per-check shell hooks:
  .claude/hooks/pretool-bash.py        <- 11 Bash PreToolUse hooks
  .claude/hooks/pretool-writeedit.py   <- 40 Write|Edit PreToolUse hooks
  .claude/hooks/posttool-writeedit.py  <- 7 cheap Write|Edit PostToolUse hooks

Provenance: each dispatcher was validated to byte-match the exit code of every
original shell hook at migration time (PreToolUse: 3280 per-hook comparisons;
PostToolUse: 22). The originals were then removed, so this test locks in the
verified behaviour with golden exit codes instead of re-running the originals.

Every corpus payload runs inside a FRESH temp project directory (no session
markers, no spec, on-disk fixtures created as needed), which makes every check
deterministic. Resulting exit codes must match the embedded GOLDEN tables.

    python3 scripts/dev/hook-parity-check.py            # assert (CI / pre-commit)
    python3 scripts/dev/hook-parity-check.py --bless     # regenerate GOLDEN tables

Exit 0 = behaviour matches golden, 1 = a dispatcher changed behaviour.
"""

import json
import os
import shutil
import subprocess
import sys
import tempfile

ROOT = os.environ.get("CLAUDE_PROJECT_DIR") or os.path.abspath(
    os.path.join(os.path.dirname(__file__), "..", "..")
)
HOOKS = os.path.join(ROOT, ".claude", "hooks")

# --- Bash PreToolUse corpus ---
BASH_CMDS = [
    "grep -n x f.log",
    "ls -la",
    "go build ./cmd/ze",
    "go build -o bin/ze ./cmd/ze",
    "go build ./...",
    "go build -o tmp/session/2026-08-10-abc123/bin/ze ./cmd/ze",
    "grep x | tail -5",
    "git log --oneline | tail -3",
    "go test ./... | head -50",
    "go test ./... 2>&1 | tee tmp/t.log",
    "bin/ze-test bgp plugin | grep FAIL",
    "git status | grep foo; make ze-rules-index",
    "ls | head; go test ./... | grep FAIL",
    # A newline is a statement boundary only when it is NOT a continuation:
    # bash continues a pipeline after a trailing `|` or a backslash.
    "go test ./... 2>&1 |\n  grep -c FAIL",
    "make ze-verify \\\n  | tail -40",
    "./bin/ze-test bgp plugin | grep FAIL",
    # The same producer in this session's own directory (mk/session.mk
    # ZE_BIN_DIR). Relative is what `make ze-path` prints; absolute is what a
    # subagent told to use absolute paths passes.
    "tmp/session/2026-08-10-abc123/bin/ze-test bgp plugin | grep FAIL",
    "/home/u/ze/tmp/session/2026-08-10-abc123/bin/ze-test bgp plugin | grep FAIL",
    # A directory that merely looks like one: no date, so no session binary.
    "tmp/session/abc123/bin/ze-test bgp plugin | grep FAIL",
    # "any test/verify/build command" (commands.md): the repo's own gates count,
    # cheap utilities in the same directory do not.
    "python3 scripts/dev/hook-parity-check.py | tail -25",
    "python3 scripts/dev/spec-session.sh wip | head -5",
    # Naming a gate script is not running it: the producer is matched in COMMAND
    # position, so reading ABOUT a check stays allowed.
    "git diff scripts/dev/hook-fixture-check.py | head -60",
    # `|&` is bash shorthand for `2>&1 |`, i.e. a real pipeline.
    "make ze-verify |& tail -5",
    # A status reader CLAUDE.md tells every session to run before committing: the
    # role-in-filename heuristic would otherwise call it an expensive gate.
    "scripts/dev/verify-status.sh check | tail -1",
    "cat /tmp/x",
    "cat tmp/x",
    "git reset --hard",
    "git commit -m x",
    "git restore --staged f",
    "git merge main",
    "cp .claude/worktrees/a internal/x",
    "rm internal/foo_test.go",
    "rm tmp/x",
    "make ze-verify | grep X",
    "make ze-verify 2>&1 | tee tmp/v.log",
    # `timeout`/`nice` carry operands of their own before the real command word.
    # A bare-integer test missed every one of these -- including `timeout 240s`,
    # the exact form ai/rules/git-safety.md tells sessions to use.
    "timeout 240s make ze-verify | tail -5",
    "timeout -k 5 30 make ze-verify | tail -5",
    "nice -n 5 make ze-verify | head -20",
    # ... and the launcher must still resolve to the REAL command word: a cheap
    # command behind the same operands stays allowed.
    "timeout 240s ls | head -5",
    # A wait loop is judged on whether it can END (ai/rules/commands.md).
    # Unbounded blocks; a `timeout` in front of the loop passes, and so does a
    # loop that terminates by construction or a one-shot pgrep.
    "until ! pgrep -f qemu; do sleep 5; done",
    "while pgrep -q qemu; do :; done",
    "timeout 600 bash -c 'until [ -f tmp/session/2026-08-10-x/ready ]; do sleep 30; done'",
    "while read -r f; do echo $f; done < tmp/list",
    "pgrep -f qemu",
    # Naming a sleep is not calling one: the word must be a COMMAND, or every
    # search for a sleep in the .ci corpus would be rejected.
    "grep -rn 'time.sleep(' test/plugin",
    # The bound is credited per LOOP, over the statement the keyword opens.
    # Crediting any earlier `timeout` made the guard fail open on a routine
    # compound line, and left a bounded loop covering an unbounded one.
    "timeout 10 curl -s localhost:8080; until ! pgrep -f qemu; do sleep 5; done",
    "timeout 60 bash -c 'until [ -f a ]; do sleep 1; done'; while true; do sleep 5; done",
    "while true; do sleep 5; timeout 10 curl -s localhost; done",
    # A `-timeout` FLAG bounds a test binary, never a loop. A `timeout` with no
    # duration bounds nothing at all.
    "go test -timeout 300s ./... | while read l; do sleep 5; done",
    "timeout bash -c 'until [ -f tmp/session/2026-08-10-x/r ]; do sleep 30; done'",
    # A keyword inside a search PATTERN is text. Without this the rule could not
    # be audited from Bash, since its own summary quotes the banned loop.
    "grep -rn 'until ! pgrep' ai/rules",
    # Precision of the sleep operand, and the Python spelling of the same wait.
    "while read -r line; do echo no-sleep-here; done < tmp/list",
    "python3 -c 'import time; while True: time.sleep(5)'",
    # Ad-hoc scratch is refused at the per-session dir's expense
    # (ai/rules/commands.md). tmp/ is keyed per CHECKOUT, so a fixed name at its
    # root is one file for every session in the tree; a subdirectory, either
    # session layout, and the shared-by-design root names are not.
    "grep -rn foo ai/rules > tmp/notes.txt",
    # The SAME file, spelled three ways. The harness hands agents absolute paths
    # and `./` is what a shell completes to, so a guard anchored on the literal
    # `tmp/` refused one spelling and passed two. @PROJECT@ is substituted with
    # the fixture root at feed time (run_corpus); the golden key keeps the token,
    # so it is stable across runs. The last row is the control that proves the
    # widened CANDIDATE did not widen the REFUSAL: `sub/tmp/` is not this
    # checkout's tmp/, so it is allowed.
    #
    # Three spellings, not every spelling. A path the SHELL builds -- `$PWD/tmp/x`,
    # `~/tmp/x`, a redirect through a symlink to tmp/ -- is out of reach of any
    # regex over command text, exactly as it is for check_pipe_tail. The guard
    # covers what an agent writes, not what bash later resolves.
    "echo probe > ./tmp/notes.txt",
    "echo probe > @PROJECT@/tmp/notes.txt",
    "tee @PROJECT@/tmp/notes.txt",
    "echo probe > sub/tmp/notes.txt",
    'make ze-unit-test-changed > "$dir/unit.log" 2>&1',
    "go test ./... 2>&1 | tee tmp/session/2026-08-10-abc123/t.log",
    "make ze-doc-test > tmp/verify/out.log 2>&1",
    "make ze-doc-test > tmp/ze-verify.log 2>&1",
    "make ze-doc-test > tmp/.ze-verify-duration.txt 2>&1",
    "make ze-doc-test > tmp/commit-abc123.log 2>&1",
    "make ze-doc-test > tmp/delete-abc123.sh 2>&1",
    "make ze-doc-test > tmp/mutation-survivors.md 2>&1",
    "make ze-doc-test > tmp/test-timings.json 2>&1",
    # A redirect quoted as a SEARCH argument is text, so the ban stays auditable
    # from Bash. The unquoted redirect above it is the discriminator: `grep` in
    # command position exempts nothing on its own.
    "grep -rn '> tmp/out.log' ai/rules",
    'bash -c "make ze-doc-test > tmp/out.log"',
    # A heredoc body a non-shell READS is data: writing a document that quotes
    # the banned shape is how this rule gets explained. A shell RUNS what it is
    # fed, and a redirect outside the body is a redirect.
    "cat >> tmp/session/x/state.md <<'EOF'\n  -- never write > tmp/out.log\nEOF",
    "bash <<'EOF'\nmake ze-doc-test > tmp/out.log\nEOF",
    "cat > tmp/out.log <<'EOF'\nhello\nEOF",
]

CLEAN_GO = "package foo\n\nfunc Hello() string {\n\treturn greeting\n}\n"

# --- Write|Edit PreToolUse corpus: (label, repo-relative path, content) ---
WE_CASES = [
    ("clean go", "internal/component/bgp/foo.go", CLEAN_GO),
    (
        "clean test",
        "internal/component/bgp/foo_test.go",
        "package foo\nfunc TestX(t *testing.T){}\n",
    ),
    (
        "os.Exit",
        "internal/component/bgp/handler.go",
        "package h\nfunc f(){ os.Exit(1) }\n",
    ),
    (
        "os.Exit in main ok",
        "cmd/ze/main.go",
        "package main\nfunc main(){ os.Exit(1) }\n",
    ),
    ("panic", "internal/component/bgp/p.go", 'package p\nfunc f(){ panic("boom") }\n'),
    (
        "panic unreachable ok",
        "internal/component/bgp/p.go",
        'package p\nfunc f(){ panic("unreachable") }\n',
    ),
    (
        "sprintf",
        "internal/component/bgp/s.go",
        'package s\nvar x = fmt.Sprintf("%d", n)\n',
    ),
    (
        "errorf ok",
        "internal/component/bgp/s.go",
        'package s\nvar x = fmt.Errorf("%d", n)\n',
    ),
    ("string concat", "internal/component/bgp/c.go", 'package c\nvar x = "a" + b\n'),
    (
        "println",
        "internal/component/bgp/d.go",
        'package d\nfunc f(){ println("hi") }\n',
    ),
    (
        "nolint bad",
        "internal/component/bgp/n.go",
        "package n\nvar x = 1 //nolint:errcheck\n",
    ),
    (
        "nolint good",
        "internal/component/bgp/n.go",
        "package n\nvar x = 1 //nolint:errcheck // needed here\n",
    ),
    (
        "camel json",
        "internal/component/bgp/j.go",
        'package j\ntype T struct { F int `json:"myField"` }\n',
    ),
    (
        "kebab json ok",
        "internal/component/bgp/j.go",
        'package j\ntype T struct { F int `json:"my-field"` }\n',
    ),
    (
        "yagni",
        "internal/component/bgp/y.go",
        "package y\n// might be useful later\nvar x int\n",
    ),
    (
        "ignored err",
        "internal/component/bgp/e.go",
        "package e\nfunc f(){ _, _ = doThing() }\n",
    ),
    (
        "layering",
        "internal/component/bgp/l.go",
        "package l\n// backwards compatibility shim\nvar x int\n",
    ),
    (
        "exabgp",
        "internal/component/bgp/x.go",
        "package x\n// exabgp json format here\nvar x int\n",
    ),
    (
        "go func hot",
        "internal/component/bgp/reactor/peer.go",
        "package r\nfunc f(){\n\tgo func(){ work() }()\n}\n",
    ),
    (
        "go func cold ok",
        "internal/component/bgp/foo.go",
        "package foo\nfunc f(){\n\tgo func(){ work() }()\n}\n",
    ),
    (
        "append encode",
        "internal/component/bgp/message/pack.go",
        "package m\nfunc f(){ buf = append(buf, b) }\n",
    ),
    (
        "banned fmt format (dead hook)",
        "internal/component/bgp/format/text.go",
        'package t\nvar x = fmt.Sprintf("%d", n)\n',
    ),
    (
        "empty default",
        "internal/component/bgp/sw.go",
        "package sw\nfunc f(){\n\tswitch x {\n\tdefault:\n\t}\n}\n",
    ),
    (
        "hardcoded cmds",
        "internal/component/bgp/cmds.go",
        'package c\nvar v = []string{"show", "set", "del", "update", "monitor"}\n',
    ),
    (
        "init register",
        "internal/component/bgp/i.go",
        "package i\nfunc init(){\n\tRegister(thing)\n}\n",
    ),
    (
        "bufhandle",
        "internal/component/bgp/b.go",
        "package b\nvar h = BufHandle{Buf: make([]byte, 8)}\n",
    ),
    (
        "version config",
        "internal/config/v.go",
        'package config\nvar v = "version: 1"\n',
    ),
    (
        "design present ok",
        "internal/component/bgp/g.go",
        "// Design: docs/x.md\npackage g\nvar x int\n",
    ),
    ("design absent", "internal/component/bgp/g2.go", "package g\nvar x int\n"),
    ("utils pkg", "internal/utils/u.go", "package utils\nvar x int\n"),
    ("generated claude", "CLAUDE.md", "# anything\n"),
    # Only the PROJECT ROOT's CLAUDE.md/AGENTS.md are generated. A basename match
    # also caught ~/.claude/CLAUDE.md and any nested one, telling their author to
    # edit an ai/INSTRUCTIONS.md that does not govern them.
    ("nested claude not generated", "docs/CLAUDE.md", "# anything\n"),
    ("claude plans", ".claude/plans/x.md", "plan\n"),
    ("observer sysexit ci", "test/parse/o.ci", "tmpfs=x.run\nsys.exit(1)\n"),
    (
        "observer ok ci",
        "test/parse/o.ci",
        "tmpfs=x.run\nruntime_fail('x')\nsys.exit(1)\n",
    ),
    ("lint exclusion", ".golangci.yml", "issues-exclude:\n  - path: foo\n"),
    ("raw ansi", "internal/component/bgp/a.go", 'package a\nvar x = "\\033[31m"\n'),
    ("scripts os.exit ok", "scripts/tool.go", "package main\nfunc f(){ os.Exit(1) }\n"),
    ("md naming bad", "docs/MyDoc.md", "# x\n"),
    ("system tmp write", "/tmp/scratch.go", "package x\n"),
    (
        "strconv format",
        "internal/component/bgp/sc.go",
        "package sc\nvar x = strconv.FormatUint(n, 10)\n",
    ),
    (
        "and function",
        "internal/component/bgp/af.go",
        "package af\nfunc ParseAndValidate() {}\n",
    ),
    # The Write half of the scratch guard, on the same policy the Bash cases
    # pin: the tmp/ root is refused, the session layout passes, a producer's
    # own folder passes, and the shared-by-design root names pass. The runner
    # sends an ABSOLUTE path here, which is what the Write tool sends too.
    ("scratch tmp root", "tmp/notes.md", "# notes\n"),
    (
        "scratch dated session dir",
        "tmp/session/2026-08-10-abc123/notes.md",
        "# notes\n",
    ),
    ("scratch producer folder", "tmp/evidence/run.log", "boot ok\n"),
    ("scratch shared root name", "tmp/ze-verify.log", "verify ok\n"),
]

# 700 lines is UNDER the only file-size threshold (1000) -- see ai/rules/go-standards.md.
# It stays in the corpus to pin that the removed 600-line tier does not come back.
_BIG700 = "package x\n" + "\n".join(f"var v{i} = {i}" for i in range(700))
_BIG1100 = "package x\n" + "\n".join(f"var v{i} = {i}" for i in range(1200))

# --- Write|Edit PostToolUse corpus: (label, relpath, on-disk content, payload content or None) ---
POST_CASES = [
    ("small go", "internal/a/small.go", "package a\nfunc F() {}\n", None),
    ("big under 1000", "internal/a/big.go", _BIG700, None),
    ("big >1000", "internal/a/huge.go", _BIG1100, None),
    (
        "test no docs",
        "internal/a/foo_test.go",
        "package a\nfunc TestX(t *testing.T){}\n",
        None,
    ),
    (
        "parse no fuzz",
        "internal/component/bgp/message/p.go",
        "package m\nfunc ParseFoo() {}\n",
        None,
    ),
    ("vague names", "internal/a/v.go", "package a\nvar Data x = 1\n", None),
    (
        "numeric no test",
        "internal/a/n.go",
        "package a\nfunc f(){ if y > 5 { return } }\n",
        None,
    ),
    (
        "rfc no header",
        "internal/a/r.go",
        "package a\n// implements RFC 4271 and RFC 4760\nfunc F(){}\n",
        None,
    ),
    ("md deferral", "docs/notes.md", "# notes\n", "this is out of scope for now\n"),
    ("md clean", "docs/clean.md", "# clean\n", "all good here\n"),
    ("py file", "scripts/x.py", "print(1)\n", None),
]


# --- test-weakening corpus: (label, relpath, old, new, mode) ---
# Paths are under pkg/ (NOT internal/) so c_pre_write_go's session-state gate does
# not mask c_test_weakening. In a fresh temp dir only c_test_weakening can fire,
# which isolates its exit code. mode "edit" sends old/new_string; mode "write"
# writes `old` to disk first, then sends `new` as Write content.
WEAKEN_CASES = [
    (
        "skip added",
        "pkg/s/a_test.go",
        "func TestX(t *testing.T){ require.Equal(t, 1, f()) }",
        'func TestX(t *testing.T){ t.Skip("flaky"); require.Equal(t, 1, f()) }',
        "edit",
    ),
    (
        "skip added with relax token",
        "pkg/s/b_test.go",
        "func TestX(t *testing.T){ require.Equal(t, 1, f()) }",
        'func TestX(t *testing.T){ t.Skip("x") // test-relax: feature removed per spec-foo\n require.Equal(t, 1, f()) }',
        "edit",
    ),
    (
        "partial assertion removal",
        "pkg/s/c_test.go",
        "require.Equal(t,1,a); require.Equal(t,2,b); require.NoError(t,err)",
        "require.Equal(t,1,a)",
        "edit",
    ),
    (
        "fatal to nonfatal downgrade",
        "pkg/s/d_test.go",
        "require.Equal(t,1,a)",
        "assert.Equal(t,1,a)",
        "edit",
    ),
    (
        "commented out assertion",
        "pkg/s/e_test.go",
        "require.Equal(t,1,a)",
        "// require.Equal(t,1,a)",
        "edit",
    ),
    (
        "build tag ignore added",
        "pkg/s/f_test.go",
        "package s\nfunc TestX(t *testing.T){ require.Equal(t,1,a) }",
        "//go:build ignore\npackage s\nfunc TestX(t *testing.T){ require.Equal(t,1,a) }",
        "edit",
    ),
    (
        "delete test func",
        "pkg/s/g_test.go",
        "func TestX(t *testing.T){ require.Equal(t,1,a) }",
        "var _ = 1",
        "edit",
    ),
    (
        "benign edit adds assertions",
        "pkg/s/h_test.go",
        "require.Equal(t,1,a)",
        "require.Equal(t,1,a); require.NoError(t,err)",
        "edit",
    ),
    (
        "write overwrite weakens",
        "pkg/s/w_test.go",
        "package s\nfunc TestX(t *testing.T){ require.Equal(t,1,a); require.NoError(t,err) }\n",
        'package s\nfunc TestX(t *testing.T){ t.Skip("x") }\n',
        "write",
    ),
    (
        "write overwrite benign",
        "pkg/s/wb_test.go",
        "package s\nfunc TestX(t *testing.T){ require.Equal(t,1,a) }\n",
        "package s\nfunc TestX(t *testing.T){ require.Equal(t,1,a); require.NoError(t,err) }\n",
        "write",
    ),
]


def feed(prog, payload, env):
    try:
        p = subprocess.run(
            [prog],
            input=json.dumps(payload),
            text=True,
            capture_output=True,
            env=env,
            timeout=30,
        )
        return p.returncode
    except Exception as exc:
        return f"ERR:{exc}"


def _fixture_root():
    # Fixture dirs must evade THREE path-sensitive behaviours so the golden is
    # platform-portable (it was blessed on macOS, where tempfile returns
    # /var/folders/... which happens to dodge all three):
    #   1. c_system_tmp_we blocks any path under /tmp/,
    #   2. c_throwaway_tests blocks .go/.py/.sh under /tmp/ or /var/tmp/,
    #   3. posttool auto-lint (golangci) actually RUNS when the fixture .go sits
    #      inside a Go module, i.e. under the repo tree.
    # A dir under XDG_CACHE_HOME / ~/.cache is none of those: not system-temp and
    # outside the module. os.makedirs is idempotent; run_corpus rmtree's each run.
    base = os.environ.get("XDG_CACHE_HOME") or os.path.join(
        os.path.expanduser("~"), ".cache"
    )
    root = os.path.join(base, "ze-hook-parity")
    os.makedirs(root, exist_ok=True)
    return root


def run_corpus():
    work = tempfile.mkdtemp(prefix="fixture-", dir=_fixture_root())
    env = dict(os.environ, CLAUDE_PROJECT_DIR=work)
    # @PROJECT@ stands for the fixture root, a fresh temp path every run. The
    # delimiters matter: a bare word would rewrite any future command that
    # happened to contain it.
    # The golden is keyed on the token so it stays stable; only the payload the
    # hook sees carries the real absolute path.
    bash = {
        c: feed(
            os.path.join(HOOKS, "pretool-bash.py"),
            {
                "tool_name": "Bash",
                "tool_input": {"command": c.replace("@PROJECT@", work)},
            },
            env,
        )
        for c in BASH_CMDS
    }
    we = {}
    for tool in ("Write", "Edit"):
        for label, rel, content in WE_CASES:
            fp = os.path.join(work, rel) if not rel.startswith("/") else rel
            ti = (
                {"file_path": fp, "content": content}
                if tool == "Write"
                else {"file_path": fp, "old_string": "PH", "new_string": content}
            )
            we[f"{tool}|{label}"] = feed(
                os.path.join(HOOKS, "pretool-writeedit.py"),
                {"tool_name": tool, "tool_input": ti},
                env,
            )
    post = {}
    for tool in ("Write", "Edit"):
        for label, rel, disk, pcontent in POST_CASES:
            fp = os.path.join(work, rel)
            os.makedirs(os.path.dirname(fp), exist_ok=True)
            with open(fp, "w") as fh:
                fh.write(disk)
            body = disk if pcontent is None else pcontent
            ti = (
                {"file_path": fp, "content": body}
                if tool == "Write"
                else {"file_path": fp, "old_string": "PH", "new_string": body}
            )
            post[f"{tool}|{label}"] = feed(
                os.path.join(HOOKS, "posttool-writeedit.py"),
                {"tool_name": tool, "tool_input": ti},
                env,
            )
    weaken = {}
    wprog = os.path.join(HOOKS, "pretool-writeedit.py")
    for label, rel, old_s, new_s, mode in WEAKEN_CASES:
        fp = os.path.join(work, rel)
        if mode == "write":
            os.makedirs(os.path.dirname(fp), exist_ok=True)
            with open(fp, "w") as fh:
                fh.write(old_s)
            ti = {"file_path": fp, "content": new_s}
            tool = "Write"
        else:
            ti = {"file_path": fp, "old_string": old_s, "new_string": new_s}
            tool = "Edit"
        weaken[label] = feed(wprog, {"tool_name": tool, "tool_input": ti}, env)
        if mode == "write" and os.path.isfile(fp):
            os.remove(fp)
    shutil.rmtree(work, ignore_errors=True)
    return bash, we, post, weaken


def bless():
    bash, we, post, weaken = run_corpus()
    print("BASH_GOLDEN = " + json.dumps(bash, indent=4, sort_keys=True))
    print("WE_GOLDEN = " + json.dumps(we, indent=4, sort_keys=True))
    print("POST_GOLDEN = " + json.dumps(post, indent=4, sort_keys=True))
    print("WEAKEN_GOLDEN = " + json.dumps(weaken, indent=4, sort_keys=True))


def check():
    bash, we, post, weaken = run_corpus()
    fails = 0
    for table, got, golden in (
        ("bash", bash, BASH_GOLDEN),
        ("pre write|edit", we, WE_GOLDEN),
        ("post write|edit", post, POST_GOLDEN),
        ("test-weakening", weaken, WEAKEN_GOLDEN),
    ):
        for key, want in golden.items():
            if got.get(key) != want:
                fails += 1
                print(f"[DIFF] {table}: {key!r} golden={want} got={got.get(key)}")
        for key in got:
            if key not in golden:
                fails += 1
                print(f"[NEW] {table}: {key!r} = {got[key]} (run --bless)")
    n = len(BASH_GOLDEN) + len(WE_GOLDEN) + len(POST_GOLDEN) + len(WEAKEN_GOLDEN)
    print(f"hook dispatcher golden check: {n - fails}/{n} match")
    print(
        "OK"
        if fails == 0
        else f"{fails} MISMATCH(es) -- a dispatcher changed behaviour"
    )
    return 1 if fails else 0


# === GOLDEN (regenerate with --bless) ===
BASH_GOLDEN = {
    "cat /tmp/x": 2,
    "cat tmp/x": 0,
    "cp .claude/worktrees/a internal/x": 2,
    "git commit -m x": 2,
    "git merge main": 2,
    "git reset --hard": 2,
    "git restore --staged f": 0,
    "go build -o bin/ze ./cmd/ze": 0,
    "go build ./...": 0,
    "go build ./cmd/ze": 2,
    # A session builds into its own dated directory (mk/session.mk,
    # internal/test/sessionpath). Every other -o target outside bin/ is
    # refused, which "go build ./cmd/ze" above pins.
    "go build -o tmp/session/2026-08-10-abc123/bin/ze ./cmd/ze": 0,
    "grep -n x f.log": 0,
    # A lossy pipe is blocked on an EXPENSIVE producer only (commands.md).
    # `grep x | tail` and `git log | tail` are cheap: blocking them was a false
    # positive that taught sessions to route around the hook.
    "grep x | tail -5": 0,
    "git log --oneline | tail -3": 0,
    "go test ./... | head -50": 2,
    # `tee` keeps the whole stream, so pipe-tail passes it. The 2 is
    # check_scratch_path refusing `tmp/t.log`, a fixed name at the shared tmp/
    # root: one file for every session in this checkout.
    "go test ./... 2>&1 | tee tmp/t.log": 2,
    "bin/ze-test bgp plugin | grep FAIL": 2,
    # Each STATEMENT is judged on its own: a cheap pipeline beside an expensive
    # command is fine, the expensive command's own lossy pipe is not.
    "git status | grep foo; make ze-rules-index": 0,
    "ls | head; go test ./... | grep FAIL": 2,
    "go test ./... 2>&1 |\n  grep -c FAIL": 2,
    "make ze-verify \\\n  | tail -40": 2,
    "./bin/ze-test bgp plugin | grep FAIL": 2,
    # AC-11: the session's own binaries are the same producer as bin/ze-test and
    # are blocked identically, relative or absolute. The date is load-bearing --
    # without it the path names no session directory mk/session.mk can produce.
    "tmp/session/2026-08-10-abc123/bin/ze-test bgp plugin | grep FAIL": 2,
    "/home/u/ze/tmp/session/2026-08-10-abc123/bin/ze-test bgp plugin | grep FAIL": 2,
    "tmp/session/abc123/bin/ze-test bgp plugin | grep FAIL": 0,
    "python3 scripts/dev/hook-parity-check.py | tail -25": 2,
    "python3 scripts/dev/spec-session.sh wip | head -5": 0,
    "git diff scripts/dev/hook-fixture-check.py | head -60": 0,
    "make ze-verify |& tail -5": 2,
    "scripts/dev/verify-status.sh check | tail -1": 0,
    "ls -la": 0,
    "make ze-verify 2>&1 | tee tmp/v.log": 2,
    "make ze-verify | grep X": 2,
    # A launcher's own operands (a suffixed duration, a flag with an argument)
    # sit in front of the command word; the producer behind them is still `make`.
    "timeout 240s make ze-verify | tail -5": 2,
    "timeout -k 5 30 make ze-verify | tail -5": 2,
    "nice -n 5 make ze-verify | head -20": 2,
    # Same launcher shape, cheap producer: nothing to block.
    "timeout 240s ls | head -5": 0,
    "rm internal/foo_test.go": 2,
    "rm tmp/x": 0,
    # An unbounded wait loop is the blocked shape, whether it sleeps or spins.
    "until ! pgrep -f qemu; do sleep 5; done": 2,
    "while pgrep -q qemu; do :; done": 2,
    # Bounded by a `timeout` in front of the loop: it ends on its own.
    "timeout 600 bash -c 'until [ -f tmp/session/2026-08-10-x/ready ]; do sleep 30; done'": 0,
    # Loops that are not waits, and a pgrep that runs once.
    "while read -r f; do echo $f; done < tmp/list": 0,
    "pgrep -f qemu": 0,
    "grep -rn 'time.sleep(' test/plugin": 0,
    # The escape is credited per loop, in the loop's own statement: an earlier
    # bounded command, an earlier bounded LOOP, and a bound inside the body all
    # leave this loop unbounded.
    "timeout 10 curl -s localhost:8080; until ! pgrep -f qemu; do sleep 5; done": 2,
    "timeout 60 bash -c 'until [ -f a ]; do sleep 1; done'; while true; do sleep 5; done": 2,
    "while true; do sleep 5; timeout 10 curl -s localhost; done": 2,
    # A `-timeout` flag is not a bound, and `timeout` with no duration is not one.
    "go test -timeout 300s ./... | while read l; do sleep 5; done": 2,
    "timeout bash -c 'until [ -f tmp/session/2026-08-10-x/r ]; do sleep 30; done'": 2,
    # Searching for the pattern stays possible; the sleep operand stays precise.
    "grep -rn 'until ! pgrep' ai/rules": 0,
    "while read -r line; do echo no-sleep-here; done < tmp/list": 0,
    "python3 -c 'import time; while True: time.sleep(5)'": 2,
    # Scratch placement. Only a fixed name at the tmp/ ROOT is refused; the
    # session layout, any other subdirectory, and the shared-by-design root
    # names pass. The guard's contract is "not at the tmp/ root": it decides on
    # a path's resolved PARENT and names no layout, so no row here pins a
    # directory NAME, and a renamed session root needs no fixture change
    # (spec-session-bin-directory, AC-27).
    "grep -rn foo ai/rules > tmp/notes.txt": 2,
    # One file, three spellings, one verdict -- and a control proving the
    # candidate widened without the refusal widening.
    "echo probe > ./tmp/notes.txt": 2,
    "echo probe > @PROJECT@/tmp/notes.txt": 2,
    "tee @PROJECT@/tmp/notes.txt": 2,
    "echo probe > sub/tmp/notes.txt": 0,
    'make ze-unit-test-changed > "$dir/unit.log" 2>&1': 0,
    "go test ./... 2>&1 | tee tmp/session/2026-08-10-abc123/t.log": 0,
    "make ze-doc-test > tmp/verify/out.log 2>&1": 0,
    "make ze-doc-test > tmp/ze-verify.log 2>&1": 0,
    "make ze-doc-test > tmp/.ze-verify-duration.txt 2>&1": 0,
    "make ze-doc-test > tmp/commit-abc123.log 2>&1": 0,
    "make ze-doc-test > tmp/delete-abc123.sh 2>&1": 0,
    "make ze-doc-test > tmp/mutation-survivors.md 2>&1": 0,
    "make ze-doc-test > tmp/test-timings.json 2>&1": 0,
    # Auditability, and its discriminator: a QUOTED redirect opening a search
    # command is text; the unquoted `> tmp/notes.txt` above still blocks, and
    # quoting a real command to run-shape does not buy an escape.
    "grep -rn '> tmp/out.log' ai/rules": 0,
    'bash -c "make ze-doc-test > tmp/out.log"': 2,
    # Heredoc: data for a non-shell reader, a script for a shell, and the
    # redirect that opens the heredoc is judged on its own.
    "cat >> tmp/session/x/state.md <<'EOF'\n  -- never write > tmp/out.log\nEOF": 0,
    "bash <<'EOF'\nmake ze-doc-test > tmp/out.log\nEOF": 2,
    "cat > tmp/out.log <<'EOF'\nhello\nEOF": 2,
}
WE_GOLDEN = {
    "Edit|and function": 2,
    "Edit|append encode": 2,
    "Edit|banned fmt format (dead hook)": 2,
    "Edit|bufhandle": 2,
    "Edit|camel json": 2,
    "Edit|claude plans": 0,
    "Edit|clean go": 2,
    "Edit|clean test": 2,
    "Edit|design absent": 2,
    "Edit|design present ok": 2,
    "Edit|empty default": 2,
    "Edit|errorf ok": 2,
    "Edit|exabgp": 2,
    "Edit|generated claude": 2,
    "Edit|nested claude not generated": 0,
    "Edit|go func cold ok": 2,
    "Edit|go func hot": 2,
    "Edit|hardcoded cmds": 2,
    "Edit|ignored err": 2,
    "Edit|init register": 2,
    "Edit|kebab json ok": 2,
    "Edit|layering": 2,
    "Edit|lint exclusion": 2,
    "Edit|md naming bad": 0,
    "Edit|nolint bad": 2,
    "Edit|nolint good": 2,
    "Edit|observer ok ci": 0,
    "Edit|observer sysexit ci": 1,
    "Edit|os.Exit": 2,
    "Edit|os.Exit in main ok": 2,
    "Edit|panic": 2,
    "Edit|panic unreachable ok": 2,
    "Edit|println": 2,
    "Edit|raw ansi": 2,
    # The Write surface answers the tmp/ root exactly as the Bash surface does:
    # the root is refused, the session layout and any other subdirectory pass,
    # and so do the shared-by-design root names (c_scratch_path_we).
    "Edit|scratch dated session dir": 0,
    "Edit|scratch producer folder": 0,
    "Edit|scratch shared root name": 0,
    "Edit|scratch tmp root": 2,
    "Edit|scripts os.exit ok": 2,
    "Edit|sprintf": 2,
    "Edit|strconv format": 2,
    "Edit|string concat": 2,
    "Edit|system tmp write": 2,
    "Edit|utils pkg": 2,
    "Edit|version config": 2,
    "Edit|yagni": 2,
    "Write|and function": 2,
    "Write|append encode": 2,
    "Write|banned fmt format (dead hook)": 2,
    "Write|bufhandle": 2,
    "Write|camel json": 2,
    "Write|claude plans": 2,
    "Write|clean go": 2,
    "Write|clean test": 2,
    "Write|design absent": 2,
    "Write|design present ok": 2,
    "Write|empty default": 2,
    "Write|errorf ok": 2,
    "Write|exabgp": 2,
    "Write|generated claude": 2,
    "Write|nested claude not generated": 0,
    "Write|go func cold ok": 2,
    "Write|go func hot": 2,
    "Write|hardcoded cmds": 2,
    "Write|ignored err": 2,
    "Write|init register": 2,
    "Write|kebab json ok": 2,
    "Write|layering": 2,
    "Write|lint exclusion": 2,
    "Write|md naming bad": 1,
    "Write|nolint bad": 2,
    "Write|nolint good": 2,
    "Write|observer ok ci": 0,
    "Write|observer sysexit ci": 1,
    "Write|os.Exit": 2,
    "Write|os.Exit in main ok": 2,
    "Write|panic": 2,
    "Write|panic unreachable ok": 2,
    "Write|println": 2,
    "Write|raw ansi": 2,
    "Write|scratch dated session dir": 0,
    "Write|scratch producer folder": 0,
    "Write|scratch shared root name": 0,
    "Write|scratch tmp root": 2,
    "Write|scripts os.exit ok": 2,
    "Write|sprintf": 2,
    "Write|strconv format": 2,
    "Write|string concat": 2,
    "Write|system tmp write": 2,
    "Write|utils pkg": 2,
    "Write|version config": 2,
    "Write|yagni": 2,
}
WEAKEN_GOLDEN = {
    "skip added": 2,
    "skip added with relax token": 0,
    "partial assertion removal": 2,
    "fatal to nonfatal downgrade": 2,
    "commented out assertion": 2,
    "build tag ignore added": 2,
    "delete test func": 2,
    "benign edit adds assertions": 0,
    "write overwrite weakens": 2,
    "write overwrite benign": 0,
}
POST_GOLDEN = {
    "Edit|big >1000": 1,
    "Edit|big under 1000": 0,
    "Edit|md clean": 0,
    "Edit|md deferral": 1,
    "Edit|numeric no test": 0,
    "Edit|parse no fuzz": 0,
    "Edit|py file": 0,
    "Edit|rfc no header": 0,
    "Edit|small go": 0,
    "Edit|test no docs": 0,
    "Edit|vague names": 0,
    "Write|big >1000": 1,
    "Write|big under 1000": 0,
    "Write|md clean": 0,
    "Write|md deferral": 1,
    "Write|numeric no test": 0,
    "Write|parse no fuzz": 0,
    "Write|py file": 0,
    "Write|rfc no header": 0,
    "Write|small go": 0,
    "Write|test no docs": 0,
    "Write|vague names": 0,
}


if __name__ == "__main__":
    if "--bless" in sys.argv:
        bless()
        sys.exit(0)
    sys.exit(check())
