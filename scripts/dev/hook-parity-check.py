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
    "grep x | tail -5",
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
]

_BIG600 = "package x\n" + "\n".join(f"var v{i} = {i}" for i in range(700))
_BIG1100 = "package x\n" + "\n".join(f"var v{i} = {i}" for i in range(1200))

# --- Write|Edit PostToolUse corpus: (label, relpath, on-disk content, payload content or None) ---
POST_CASES = [
    ("small go", "internal/a/small.go", "package a\nfunc F() {}\n", None),
    ("big >600", "internal/a/big.go", _BIG600, None),
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


def run_corpus():
    work = tempfile.mkdtemp(prefix="ze-hook-fixture-")
    env = dict(os.environ, CLAUDE_PROJECT_DIR=work)
    bash = {
        c: feed(
            os.path.join(HOOKS, "pretool-bash.py"),
            {"tool_name": "Bash", "tool_input": {"command": c}},
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
    shutil.rmtree(work, ignore_errors=True)
    return bash, we, post


def bless():
    bash, we, post = run_corpus()
    print("BASH_GOLDEN = " + json.dumps(bash, indent=4, sort_keys=True))
    print("WE_GOLDEN = " + json.dumps(we, indent=4, sort_keys=True))
    print("POST_GOLDEN = " + json.dumps(post, indent=4, sort_keys=True))


def check():
    bash, we, post = run_corpus()
    fails = 0
    for table, got, golden in (
        ("bash", bash, BASH_GOLDEN),
        ("pre write|edit", we, WE_GOLDEN),
        ("post write|edit", post, POST_GOLDEN),
    ):
        for key, want in golden.items():
            if got.get(key) != want:
                fails += 1
                print(f"[DIFF] {table}: {key!r} golden={want} got={got.get(key)}")
        for key in got:
            if key not in golden:
                fails += 1
                print(f"[NEW] {table}: {key!r} = {got[key]} (run --bless)")
    n = len(BASH_GOLDEN) + len(WE_GOLDEN) + len(POST_GOLDEN)
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
    "grep -n x f.log": 0,
    "grep x | tail -5": 2,
    "ls -la": 0,
    "make ze-verify 2>&1 | tee tmp/v.log": 0,
    "make ze-verify | grep X": 2,
    "rm internal/foo_test.go": 2,
    "rm tmp/x": 0,
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
    "Write|scripts os.exit ok": 2,
    "Write|sprintf": 2,
    "Write|strconv format": 2,
    "Write|string concat": 2,
    "Write|system tmp write": 2,
    "Write|utils pkg": 2,
    "Write|version config": 2,
    "Write|yagni": 2,
}
POST_GOLDEN = {
    "Edit|big >1000": 1,
    "Edit|big >600": 1,
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
    "Write|big >600": 1,
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
