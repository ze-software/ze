#!/usr/bin/env -S python3 -S
"""Single consolidated PreToolUse check for Write/Edit/MultiEdit/NotebookEdit.

Replaces 40 separate shell hooks that each spawned bash + several jq calls on
every file edit. Parses the payload once and runs every check in-process.

Each check gates on the SAME tool set and file-path predicates as its original
shell hook, so behaviour is identical; only the process count changes (40 -> 1).
Exit-code parity against all 40 originals is enforced by
scripts/dev/hook-parity-check.py.

Exit codes: 0 allow, 1 non-blocking warning, 2 block. Most severe wins when
several checks fire. Fails OPEN (exit 0) on an unexpected internal error.
"""

import datetime
import glob
import importlib.util
import os
import re
import sys
import time

RED = "\033[31m"
YELLOW = "\033[33m"
BOLD = "\033[1m"
RESET = "\033[0m"

PROJECT_DIR = os.environ.get("CLAUDE_PROJECT_DIR") or os.path.abspath(
    os.path.join(os.path.dirname(__file__), "..", "..")
)

# textbuf type serializer reference (internal/core/textbuf)
# UPDATE when textbuf serializer API changes.
_TEXTBUF_REF = (
    "  textbuf type serializers (internal/core/textbuf):\n"
    "  String:      StringUint(u64) StringUint8 StringUint16 StringUint32\n"
    "               StringInt(i64) StringAddr(netip.Addr) StringPrefix(netip.Prefix)\n"
    "               StringHex([]byte) StringHexUpper([]byte) StringMAC([]byte)\n"
    "               HostPort(host,port) Join([]string,sep)\n"
    "               StrInt StrUint IntStr UintStr StrIntStr StrUintStr\n"
    "  Chain (.):   Str Byte Uint Uint8 Uint16 Uint32 Int Addr Prefix\n"
    "               Hex HexUpper MAC Float(v,prec) Float2 Bool Quoted Err\n"
    "               Join PadRight PadLeft Repeat HostPort HostPortN Colored\n"
    "  Append:      Uint Int Addr Prefix Hex HexUpper MAC\n"
    "  Extract:     .String() (1 alloc)  .Slice() (0-copy, freezes)  .Bytes() (raw)\n"
    "  Reuse:       var tb textbuf.Buffer; ... tb.Reset() between uses"
)


# --------------------------------------------------------------------------- #
# Rule bindings
#
# A `ze point:` comment directly above a check names the rule point that check
# enforces, spelled `<rule>/<section>/<slug>` under ai/rules/points/: the rule,
# the `##` section directory, and the point file, always three components. A
# check may name several, and one that enforces no written point says
# `none -- <why>`.
# `make ze-rules-gate-map` joins these against the points on disk: it reports
# which points are gated and which are not, and it FAILS on a reference to a
# point that does not exist, which is what a reworded rule looks like.
# --------------------------------------------------------------------------- #

# --------------------------------------------------------------------------- #
# Shared helpers
# --------------------------------------------------------------------------- #


def std_content(ti):
    """CONTENT = .tool_input.content // .tool_input.new_string (jq // = null-only)."""
    c = ti.get("content")
    if c is None:
        c = ti.get("new_string")
    return c or ""


def grep_lines(text, pattern, ignorecase=False, invert=False):
    flags = re.IGNORECASE if ignorecase else 0
    out = []
    for i, line in enumerate(text.split("\n"), 1):
        m = bool(re.search(pattern, line, flags))
        if m != invert:
            out.append((i, line))
    return out


def grep_any(text, pattern, ignorecase=False):
    flags = re.IGNORECASE if ignorecase else 0
    return re.search(pattern, text, flags | re.MULTILINE) is not None


def filter_out(pairs, pattern, ignorecase=False):
    flags = re.IGNORECASE if ignorecase else 0
    return [(n, l) for (n, l) in pairs if not re.search(pattern, l, flags)]


def isfile(path):
    return os.path.isfile(path)


# --- path identity, asked of the filesystem ---
#
# `os.path.realpath` resolves symlinks. It does NOT resolve CASE, and this
# repository is developed on a case-insensitive volume, where
# `<repo>/AI/rules/points/<rule>/<section>/<slug>.md` and
# `<repo>/ai/rules/Points/.../<Slug>.md` both open the very file a check exists
# to protect. Every comparison built from realpath STRINGS therefore permitted a
# Write that destroyed the point it refused under the canonical spelling. Two
# checks had it, and it is the same defect class as the depth bug beside it: one
# keyed on depth, one on case.
#
# `os.path.normcase` is not the fix. It lowercases on Windows and is the
# IDENTITY on every POSIX platform, this one included, so it would leave the
# hole exactly where it is. Identity is asked of the filesystem instead
# (`st_dev`, `st_ino`), which answers truly on a case-insensitive volume and on
# a case-sensitive one alike -- and on a case-sensitive volume `AI/rules` names
# nothing, so refusing it would be a false block rather than a fix.


def _same_path(a, b):
    """Whether two paths name the same file or directory on disk.

    Falls back to a string comparison when either side is absent: a path with
    no inode cannot be compared by identity, and the strings are then the only
    evidence there is.
    """
    try:
        return os.path.samestat(os.stat(a), os.stat(b))
    except OSError:
        return a == b


def _on_disk_name(directory, name):
    """`name` as `directory` spells it, or `name` when nothing there matches.

    Only reached once the parent has already matched by identity, so this is
    about NAMING the file in a refusal and routing the message, never about the
    verdict.
    """
    try:
        entries = os.listdir(directory)
    except OSError:
        return name
    if name in entries:
        return name
    folded = name.lower()
    return next((e for e in entries if e.lower() == folded), name)


def _tail_under(base, path):
    """The components of `path` below `base`, in their on-disk spelling.

    None when `path` is not under `base`. The walk compares by identity at every
    step, so a case variant of any segment resolves to the same answer as the
    canonical spelling. Bounded: a path deeper than 24 components below `base`
    is not a rule point and is not worth walking.
    """
    parts = []
    current = path
    while not _same_path(current, base):
        head, tail = os.path.split(current)
        if not tail or head == current or len(parts) >= 24:
            return None
        parts.append(tail)
        current = head
    out = []
    where = base
    for name in reversed(parts):
        name = _on_disk_name(where, name)
        out.append(name)
        where = os.path.join(where, name)
    return out


# --- session id + state file ---
#
# The session id is resolved by the ONE shared resolver, .claude/hooks/lib/
# session_id.py -- the SAME code the shell hooks run (via the lib/session-id.sh
# shim) to WRITE the markers this file READS (.lsp-invoked-<sid>, .source-read-<sid>,
# .session-<sid>, and the per-session directory holding the digest). One
# implementation, so the two ends
# cannot drift; a disagreement fails CLOSED, blocking work already done (incident
# 2026-07-16, spec-fixit-session-id-collision). state_file() below is the local
# path-naming twin of lib/state-file.sh's _state_file().

_SID_MODULE_PATH = os.path.join(
    os.path.dirname(os.path.abspath(__file__)), "lib", "session_id.py"
)
_sid_spec = importlib.util.spec_from_file_location("ze_session_id", _SID_MODULE_PATH)
_ze_session_id = importlib.util.module_from_spec(_sid_spec)
_sid_spec.loader.exec_module(_ze_session_id)

# Which `tmp/` paths a session may write, shared with pretool-bash.py so a path
# refused in a redirect cannot land through the Write tool. Resolved relative to
# THIS FILE for the reason above.
_SCRATCH_MODULE_PATH = os.path.join(
    os.path.dirname(os.path.abspath(__file__)), "lib", "scratch_path.py"
)
_scratch_spec = importlib.util.spec_from_file_location(
    "ze_scratch_path", _SCRATCH_MODULE_PATH
)
_scratch_path = importlib.util.module_from_spec(_scratch_spec)
_scratch_spec.loader.exec_module(_scratch_path)

# "The tagged unit" has exactly ONE definition, shared with the RFC coverage gate
# (scripts/dev/rfc_tagged_scope.py). Resolved relative to THIS FILE, never through
# PROJECT_DIR: CLAUDE_PROJECT_DIR can point at a fixture tree while the hook itself is always
# two directories below the repo root.
_SCOPE_MODULE_PATH = os.path.join(
    os.path.dirname(os.path.abspath(__file__)),
    "..",
    "..",
    "scripts",
    "dev",
    "rfc_tagged_scope.py",
)


def _load_rfc_scope():
    """The shared tagged-scope leaf, or None when it cannot be loaded.

    A hook that raises on import blocks every edit in the repository, so this cannot be a
    bare import. The caller's None branch degrades toward MORE checking, not less.
    """
    try:
        spec = importlib.util.spec_from_file_location(
            "ze_rfc_tagged_scope", _SCOPE_MODULE_PATH
        )
        mod = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(mod)
        return mod
    except Exception:
        return None


_rfc_scope = _load_rfc_scope()


def session_id():
    """Resolve this session's id via the ONE shared resolver (session_id.py)."""
    return _ze_session_id.session_id()


def session_dir(sid):
    """This session's directory, `tmp/session/<YYYY-MM-DD>-<sid>`.

    The glob-then-name rule of `.claude/hooks/lib/session-dir.sh`: take the
    directory that already carries this id, whatever its date, and name a new
    one with today's date only on a miss. Recomputing from today would move a
    live session's directory at midnight. Creates nothing -- this hook only
    reads.
    """
    root = os.path.join(PROJECT_DIR, "tmp", "session")
    for d in sorted(glob.glob(os.path.join(root, f"????-??-??-{sid}"))):
        if os.path.isdir(d):
            return d
    return os.path.join(root, f"{datetime.date.today().isoformat()}-{sid}")


def state_file(sid):
    """Where this session's per-spec digest is, for the gates that require one.

    The digest LANDS at `<session-dir>/state/` (lib/state-file.sh `_state_file`).
    A digest written before that move sits flat under `tmp/session/`, and this
    is a reader, so it accepts the flat file when the session has one and no
    other. Reading both is what stops these gates blocking every session whose
    digest predates the move; only the WRITER is single-location.
    """
    marker = os.path.join(PROJECT_DIR, "tmp/session", f".session-{sid}")
    spec = ""
    if os.path.isfile(marker):
        try:
            with open(marker) as fh:
                spec = fh.readline().strip()
        except Exception:
            spec = ""
    if spec and spec != "unassigned":
        stem = re.sub(r"\.md$", "", re.sub(r"^spec-", "", spec))
        name = f"session-state-{stem}-{sid}.md"
    else:
        name = f"session-state-{sid}.md"
    current = os.path.join(session_dir(sid), "state", name)
    if os.path.isfile(current):
        return current
    legacy = os.path.join(PROJECT_DIR, "tmp/session", name)
    if os.path.isfile(legacy):
        return legacy
    return current


# --------------------------------------------------------------------------- #
# Content-pattern checks (Go production code)
# --------------------------------------------------------------------------- #


def _go_we(ctx):
    return ctx["tool"] in ("Write", "Edit") and ctx["fp"].endswith(".go")


# ze point: architecture/design-principles/apply-these-design-principles-to-every-decision
def c_and_functions(ctx):
    if not _go_we(ctx) or re.search(r"_test\.go$", ctx["fp"]):
        return None
    hits = grep_lines(
        ctx["content"], r"^func[ \t]+(\([^)]+\)[ \t]+)?[A-Z][a-zA-Z]*And[A-Z]"
    )
    if hits:
        names = []
        for n, l in hits[:4]:
            m = re.search(
                r"func[ \t]+(?:\([^)]+\)[ \t]+)?([A-Z][a-zA-Z]*And[A-Z][a-zA-Z]*)", l
            )
            names.append(f"  L{n}: {m.group(1) if m else l.strip()}")
        detail = "\n".join(names)
        fix = (
            "\n  Split into two functions, one per responsibility.\n"
            "  e.g. ParseAndValidate -> Parse + Validate (called sequentially by caller)"
        )
        return (
            1,
            f"{RED}{BOLD}❌ BLOCKED: Single responsibility violation{RESET} (function with 'And' in name)\n{detail}{fix}",
        )
    return None


# ze point: cli/cli-patterns/return-exit-codes-and-write-errors-to-stderr
def c_os_exit(ctx):
    fp = ctx["fp"]
    if (
        not _go_we(ctx)
        or re.search(r"_test\.go$", fp)
        or re.search(r"/main\.go$", fp)
        or re.search(r"/register\.go$", fp)
        or "/scripts/" in fp
    ):
        return None
    hits = filter_out(
        grep_lines(ctx["content"], r"os\.Exit\("), r"//.*os\.Exit", ignorecase=True
    )
    if hits:
        lines = [f"  L{n}: {l.strip()}" for n, l in hits[:4]]
        detail = "\n".join(lines)
        fix = (
            "\n  Return an error instead. os.Exit() is only allowed in:\n"
            "    main.go, register.go, scripts/, _test.go\n"
            '  Pattern: return fmt.Errorf("context: %w", err)  -- let main() handle exit'
        )
        return (
            2,
            f"{RED}{BOLD}❌ BLOCKED: os.Exit() in handler{RESET}\n{detail}{fix}",
        )
    return None


# ze point: go-standards/directives/never-write-these-forbidden-go-patterns
def c_panic(ctx):
    fp = ctx["fp"]
    if not _go_we(ctx) or re.search(r"_test\.go$", fp) or "/scripts/" in fp:
        return None
    hits = grep_lines(ctx["content"], r"(^|[^A-Za-z0-9_])panic[ \t]*\(")
    hits = filter_out(
        hits,
        r'panic[ \t]*\([ \t]*"(unreachable|not implemented|unimplemented|TODO|BUG|impossible)',
    )
    if hits:
        lines = [f"  L{n}: {l.strip()}" for n, l in hits[:4]]
        detail = "\n".join(lines)
        fix = (
            "\n  Return an error instead of panicking.\n"
            "  Allowed panic strings (auto-excluded from this check):\n"
            '    panic("unreachable"), panic("not implemented"),\n'
            '    panic("TODO"), panic("BUG"), panic("impossible")'
        )
        return (
            2,
            f"{RED}❌ Return error, don't panic(){RESET}\n{detail}{fix}",
        )
    return None


# ze point: none -- no point states the raw-ANSI ban; the palette is in docs/architecture/cli/color-system.md
def c_raw_ansi(ctx):
    fp = ctx["fp"]
    if (
        not _go_we(ctx)
        or re.search(r"_test\.go$", fp)
        or re.search(r"textbuf\.go$", fp)
        or re.search(r"helpfmt\.go$", fp)
    ):
        return None
    if not ctx["content"]:
        return None
    hits = filter_out(
        grep_lines(
            ctx["content"],
            r"\\033\[|\\x1b\[|\\e\[|\\u001b\[|\\U0000001b\[",
            ignorecase=True,
        ),
        r"//.*\\033",
    )
    if hits:
        lines = [f"  L{n}: {l.strip()}" for n, l in hits[:4]]
        detail = "\n".join(lines)
        fix = (
            "\n  Use internal/core/textbuf helpfmt constants instead of raw escapes.\n"
            "  Raw ANSI is only allowed in: textbuf.go, helpfmt.go, _test.go"
        )
        return (
            2,
            f"{RED}{BOLD}✘ BLOCKED: raw ANSI escape code in {fp}{RESET}\n{detail}{fix}",
        )
    return None


# ze point: go-standards/directives/never-write-these-forbidden-go-patterns
def c_legacy_log(ctx):
    fp = ctx["fp"]
    if not _go_we(ctx) or re.search(r"_test\.go$", fp) or "/scripts/" in fp:
        return None
    content = ctx["content"]
    a = grep_any(
        content,
        r'^[ \t]*(import[ \t]+)?(_[ \t]+|[a-zA-Z][a-zA-Z0-9_]*[ \t]+)?"log"[ \t]*$',
    )
    b = grep_any(
        content,
        r"(^|[^A-Za-z0-9_])log\.(Print|Printf|Println|Fatal|Fatalf|Fatalln|Panic|Panicf|Panicln)([^A-Za-z0-9_]|$)",
    )
    if a or b:
        fix = (
            '\n  Replace "log" import with "log/slog" and use structured logging:\n'
            '    log.Printf("msg: %v", err)   -> slog.Error("msg", "error", err)\n'
            '    log.Println("starting")       -> slog.Info("starting")\n'
            "    log.Fatalf(...)               -> return fmt.Errorf(...)  (let main handle exit)\n"
            "  Allowed in: _test.go, scripts/"
        )
        return (2, f"{RED}❌ Use slog, not log package{RESET}{fix}")
    return None


# ze point: performance/three-rules/never-use-fmt-or-string-on-a-hot-path
# ze point: performance/directives/write-wire-encoding-into-pooled-bounded-buffers
def c_sprintf_new(ctx):
    fp = ctx["fp"]
    if not _go_we(ctx) or re.search(r"_test\.go$", fp) or not ctx["content"]:
        return None
    h1 = filter_out(
        filter_out(
            grep_lines(ctx["content"], r"fmt\.(Sprintf|Fprintf|Printf)\("),
            r"fmt\.Fprintf\(os\.(Stdout|Stderr)",
        ),
        r"//.*fmt\.(Sprintf|Fprintf|Printf)",
    )
    h2 = filter_out(
        grep_lines(ctx["content"], r"strconv\.Format(Uint|Int)\("),
        r"//.*strconv\.Format",
    )
    if h1 or h2:
        lines = []
        for n, l in (h1 + h2)[:6]:
            lines.append(f"  L{n}: {l.strip()}")
        detail = "\n".join(lines)
        fix = (
            "\n  Replacements (ai/rules/performance.md):\n"
            '    fmt.Sprintf("%s: %v", x, err)    -> var tb textbuf.Buffer; tb.Str(x).Str(": ").Err(err).String()\n'
            "    fmt.Sprintf(\"%s:%d\", s, n)        -> var tb textbuf.Buffer; tb.Str(s).Byte(':').Int(int64(n)).String()\n"
            '    fmt.Sprintf("%d", n)              -> textbuf.StringInt(int64(n))  or  textbuf.StringUint(uint64(n))\n'
            '    fmt.Sprintf("%q", s)              -> var tb textbuf.Buffer; tb.Quoted(s).String()\n'
            "    strconv.FormatUint(v, 10)         -> textbuf.StringUint(v)\n"
            "    strconv.FormatInt(v, 10)          -> textbuf.StringInt(v)\n"
            "  ALLOWED (do NOT change):\n"
            '    fmt.Errorf("context: %w", err)    -- error wrapping is the intended use\n'
            "    fmt.Fprintf(os.Stdout, ...)        -- CLI output\n"
            '    fmt.Sprintf("%v", anyVal)          -- arbitrary-type; no textbuf path\n'
            '    fmt.Sprintf("%T", val)             -- reflect-based type name\n'
        ) + _TEXTBUF_REF
        return (
            2,
            f"{RED}{BOLD}✘ BLOCKED: banned format primitive in {fp}{RESET}\n{detail}{fix}",
        )
    return None


# ze point: performance/banned-patterns/build-strings-with-textbuf-never-with-plus
def c_string_concat(ctx):
    fp = ctx["fp"]
    if not _go_we(ctx) or re.search(r"_test\.go$", fp) or not ctx["content"]:
        return None
    hits = grep_lines(ctx["content"], r'("[^"]*"\s*\+\s*[^"=]|[^"=]\s*\+\s*"[^"]*")')
    hits = filter_out(hits, r"^\s*//")
    hits = filter_out(hits, r"(^[0-9]+:)?\s*const\s")
    hits = filter_out(hits, r'"[^"]*"\s*\+\s*"[^"]*"')
    hits = filter_out(hits, r'//.*"[^"]*"\s*\+')
    hits = filter_out(hits, r"filepath\.(Join|Dir|Base)")
    hits = filter_out(hits, r"path\.(Join|Dir|Base)")
    if hits:
        lines = []
        for n, l in hits[:6]:
            lines.append(f"  L{n}: {l.strip()}")
        detail = "\n".join(lines)
        fix = (
            "\n  Replacements (ai/rules/performance.md):\n"
            "    a + \"/\" + b              -> var tb textbuf.Buffer; tb.Str(a).Byte('/').Str(b).String()\n"
            "    \"#\" + id                 -> var tb textbuf.Buffer; tb.Byte('#').Str(id).String()\n"
            '    "prefix:" + s            -> var tb textbuf.Buffer; tb.Str("prefix:").Str(s).String()\n'
            "    s + strconv.Itoa(n)      -> var tb textbuf.Buffer; tb.Str(s).Int(int64(n)).String()\n"
            '    "KEY=" + val             -> var tb textbuf.Buffer; tb.Str("KEY=").Str(val).String()\n'
            "  Use ONE buffer per function, .Reset() between uses.\n"
            "  Single-char prefix: use .Byte('#') not .Str(\"#\")\n"
        ) + _TEXTBUF_REF
        return (
            2,
            f"{RED}{BOLD}BLOCKED: string + concatenation in {fp}{RESET}\n{detail}{fix}",
        )
    return None


# Debug-marker print. Matches fmt.Print* AND fmt.Fprint* (the `F?`): a debug
# line is just as likely to be Fprintf(os.Stderr, "DEBUG: ...") as Printf, and
# `fmt\.Print` alone never matches "fmt.Fprintf" because of the F.
#
# There is deliberately NO blanket `fmt\.Fprint.*os\.Stderr` rule. It was
# removed after measuring it against the tree: it flagged 1118 committed stderr
# writes across 123 files, of which 1117 were legitimate CLI output (usage text,
# interactive prompts, error messages) and the single "hit" was a false positive
# -- a diff header, `fmt.Fprintf(os.Stderr, "--- %s (original)\n", path)`, caught
# only because "---" was in the marker list. Precision was zero, so the rule
# blocked real work and never caught a real debug print. Writing to stderr is
# what a CLI DOES; it is not evidence of a debug statement. "---" is dropped for
# the same reason. The markers below are the actual tells.
_DEBUG_MARKER = r'fmt\.F?Print.*"(DEBUG|debug|TRACE|trace|>>>|<<<|\*\*\*|XXX|FIXME)'


# ze point: go-standards/directives/log-through-slog-never-printf
def c_temp_debug(ctx):
    fp = ctx["fp"]
    if (
        not _go_we(ctx)
        or re.search(r"_test\.go$", fp)
        or "cmd/" in fp
        or "/scripts/" in fp
        or re.search(r"/register\.go$", fp)
    ):
        return None
    content = ctx["content"]
    if (
        grep_lines(content, r"^[ \t]*println[ \t]*\(")
        or grep_lines(content, _DEBUG_MARKER, ignorecase=True)
        or filter_out(
            grep_lines(content, r'fmt\.Println[ \t]*\([ \t]*"[^"]{1,50}"[ \t]*\)'),
            r"error|fail|warn|usage|help|version",
            ignorecase=True,
        )
    ):
        found = []
        for n, l in grep_lines(content, r"^[ \t]*println[ \t]*\(")[:2]:
            found.append(f"  L{n}: println(...)  -> use slog or remove")
        for n, l in grep_lines(content, _DEBUG_MARKER, ignorecase=True)[:2]:
            found.append(f"  L{n}: debug print -> remove")
        for n, l in filter_out(
            grep_lines(content, r'fmt\.Println[ \t]*\([ \t]*"[^"]{1,50}"[ \t]*\)'),
            r"error|fail|warn|usage|help|version",
            ignorecase=True,
        )[:2]:
            found.append(f'  L{n}: fmt.Println("...") -> use slog or remove')
        detail = "\n".join(found[:4]) if found else ""
        fix = (
            "\n  Remove debug statements. Use slog for permanent logging.\n"
            "  Allowed in: _test.go, cmd/, scripts/, register.go\n"
            "  Operator-facing CLI output on os.Stderr is NOT a debug statement."
        )
        return (
            2,
            f"{RED}{BOLD}❌ BLOCKED: Temporary debug statement{RESET}\n{detail}{fix}",
        )
    return None


# ze point: quality/linting/fix-lint-issues-never-disable-a-linter
def c_nolint(ctx):
    if not _go_we(ctx):
        return None
    bad_lines = []
    for n, line in grep_lines(ctx["content"], r"//[ \t]*nolint"):
        if not re.search(r"//[ \t]*nolint:[a-zA-Z,]+[ \t]+//", line):
            bad_lines.append((n, line))
    if bad_lines:
        lines = [f"  L{n}: {l.strip()}" for n, l in bad_lines[:4]]
        detail = "\n".join(lines)
        fix = (
            "\n  Required format: //nolint:lintername // justification reason\n"
            "  Example: //nolint:gosec // args are test-controlled, not user input"
        )
        return (
            2,
            f"{RED}{BOLD}❌ BLOCKED: nolint without justification{RESET}\n{detail}{fix}",
        )
    return None


# ze point: cli/json-format/name-json-keys-in-lowercase-kebab-case
# ze point: go-standards/directives/tag-every-json-field-with-a-kebab-case-name
def c_json_kebab(ctx):
    fp = ctx["fp"]
    if not _go_we(ctx) or re.search(r"_test\.go$", fp):
        return None
    camel = grep_lines(ctx["content"], r'`.*json:"[a-z]+[A-Z]')
    snake = grep_lines(ctx["content"], r'`.*json:"[a-z]+_[a-z]')
    if camel or snake:
        hits = (camel + snake)[:4]
        lines = [f"  L{n}: {l.strip()}" for n, l in hits]
        detail = "\n".join(lines)
        fix = (
            "\n  JSON tags must use kebab-case to match YANG/config naming:\n"
            '    camelCase "peerAddr"  -> "peer-addr"\n'
            '    snake_case "peer_addr" -> "peer-addr"\n'
            "  See ai/rules/cli.md"
        )
        return (
            2,
            f"{RED}{BOLD}❌ BLOCKED: Non-kebab-case JSON tags{RESET}\n{detail}{fix}",
        )
    return None


# ze point: architecture/design-principles/apply-these-design-principles-to-every-decision
def c_yagni(ctx):
    fp = ctx["fp"]
    if not _go_we(ctx) or re.search(r"_test\.go$", fp):
        return None
    pats = [
        "in case we need",
        "might be useful",
        "for future use",
        "someday",
        "just in case",
        "maybe later",
        "could be extended",
        "placeholder for",
        "reserved for future",
        "not yet implemented.*TODO",
    ]
    for p in pats:
        hits = grep_lines(ctx["content"], p, ignorecase=True)
        if hits:
            lines = [f"  L{n}: {l.strip()}" for n, l in hits[:3]]
            detail = "\n".join(lines)
            fix = (
                "\n  Remove speculative code/comments. Build only what is needed now.\n"
                "  If future work is planned, track it in a spec, not in source comments."
            )
            return (
                2,
                f"{RED}{BOLD}❌ BLOCKED: YAGNI violation{RESET}\n{detail}{fix}",
            )
    return None


# ze point: go-standards/directives/never-write-these-forbidden-go-patterns
def c_ignored_errors(ctx):
    if not _go_we(ctx):
        return None
    content = ctx["content"]
    h1 = grep_lines(
        content,
        r"^[ \t]*_[ \t]*=[ \t]*[A-Za-z0-9_]+\.(Close|Write|Read|Flush|Sync|Remove|Mkdir|Chmod)[ \t]*\(",
    )
    h2 = grep_lines(content, r"^[ \t]*_[ \t]*,[ \t]*_[ \t]*=")
    if h1 or h2:
        lines = [f"  L{n}: {l.strip()}" for n, l in (h1 + h2)[:4]]
        detail = "\n".join(lines)
        fix = (
            "\n  Handle the error explicitly:\n"
            "    _ = f.Close()           -> if err := f.Close(); err != nil { ... }\n"
            "    _, _ = io.Copy(w, r)    -> n, err := io.Copy(w, r)"
        )
        return (
            2,
            f"{RED}❌ Handle errors: if err != nil {{ }}{RESET}\n{detail}{fix}",
        )
    return None


# ze point: no-layering/directives/delete-the-old-before-implementing-the-new
# ze point: no-layering/directives/never-keep-the-old-path-beside-the-new-one
def c_layering(ctx):
    fp = ctx["fp"]
    if not _go_we(ctx) or re.search(r"_test\.go$", fp):
        return None
    pats = [
        r"backwards.?compatib",
        r"backward.?compatib",
        r"legacy.?(code|format|shim|layer|path|support)",
        r"fallback.?to.?(old|legacy|previous|pre[-_])",
        r"hybrid.?(approach|system|layer)",
        r"gradual.?migration",
        r"temporary.?shim",
        r"compat.?layer",
        r"deprecated.?but.?kept",
    ]
    for p in pats:
        hits = grep_lines(ctx["content"], p, ignorecase=True)
        if hits:
            lines = [f"  L{n}: {l.strip()}" for n, l in hits[:3]]
            detail = "\n".join(lines)
            fix = (
                "\n  Ze does not maintain compatibility layers. Replace old code directly.\n"
                "  No shims, no gradual migrations, no deprecated-but-kept paths.\n"
                "  If breaking a format, update all consumers in the same change."
            )
            return (
                2,
                f"{RED}{BOLD}❌ BLOCKED: Layering/compatibility pattern{RESET}\n{detail}{fix}",
            )
    return None


# ze point: go-standards/no-backwards-compatibility/keep-exabgp-awareness-out-of-engine-code
def c_exabgp(ctx):
    fp = ctx["fp"]
    if not _go_we(ctx) or "/exabgp/" in fp or "cmd/ze/exabgp" in fp:
        return None
    content = ctx["content"]
    # Exception: `exabgp-compat` is the established test-fixtures directory
    # (test/exabgp-compat/). Referencing that path from tests -- globs,
    # filepath.Join, ReadDir, comments -- is test plumbing, not engine ExaBGP
    # format/naming logic, so neutralize that one directory token before
    # scanning. Real exabgp format/JSON references, the ExaBGPCompat type, and
    # `internal/exabgp` imports below are still blocked.
    content = re.sub(r"exabgp-compat", "fixtures", content, flags=re.IGNORECASE)
    errs = False
    for p in [
        r"exabgp.*format",
        r"ExaBGP.*JSON",
        r"exabgp.*json",
        r'"neighbor".*"announce"',
        r"exabgp.*compat",
        r"ExaBGPCompat",
    ]:
        if grep_any(content, p, ignorecase=True):
            errs = True
            break
    if grep_any(content, r'".*internal/exabgp"') and not re.search(r"cmd/ze/", fp):
        errs = True
    if errs:
        fix = (
            "\n  ExaBGP references are only allowed in:\n"
            "    internal/exabgp/   (migration/compat package)\n"
            "    cmd/ze/exabgp/     (CLI subcommand)\n"
            "  The engine must not reference ExaBGP formats or naming."
        )
        return (
            2,
            f"{RED}{BOLD}❌ BLOCKED: ExaBGP in engine{RESET}{fix}",
        )
    return None


# ze point: goroutine-lifecycle/directives/keep-every-goroutine-a-long-lived-worker
def c_goroutine(ctx):
    fp = ctx["fp"]
    if not _go_we(ctx) or re.search(r"_test\.go$", fp):
        return None
    hot = bool(
        re.search(r"reactor", fp)
        or re.search(r"/event", fp)
        or re.search(r"/dispatch", fp)
        or re.search(r"/hub/", fp)
        or re.search(r"/wire/", fp)
        or re.search(r"/message/", fp)
    )
    if not hot:
        return None
    hits = grep_lines(ctx["content"], r"^[ \t]*go func\(")
    if hits:
        lines = [f"  L{n}: {l.strip()}" for n, l in hits[:4]]
        detail = "\n".join(lines)
        fix = (
            "\n  No per-event goroutines in hot paths (reactor, event, dispatch, hub, wire, message).\n"
            "  Use long-lived worker goroutines started at component init.\n"
            "  See ai/rules/goroutine-lifecycle.md"
        )
        return (
            2,
            f"{RED}{BOLD}❌ BLOCKED: Per-event goroutine in hot path{RESET}\n{detail}{fix}",
        )
    return None


# ze point: performance/directives/write-wire-encoding-into-pooled-bounded-buffers
# ze point: performance/buffer-first-encoding-mechanical-reference/audit-and-fix-encoding-allocations
def c_encoding_alloc(ctx):
    fp = ctx["fp"]
    if not _go_we(ctx) or re.search(r"_test\.go$", fp):
        return None
    enc = [
        "/message/update_build",
        "/message/update_split",
        "/message/pack",
        "/reactor_wire",
        "/reactor/forward_build",
        "/bgp/nlri/base",
        "/bgp/nlri/inet",
        "/bgp/nlri/rd",
    ]
    if not any(_glob_match(fp, "*" + p + "*") for p in enc):
        return None
    content = ctx["content"]
    reasons = []
    make_allow = r"return make|Pool|New.*func|nlriBytes[ \t]*:=[ \t]*make|owned[ \t]*:=[ \t]*make|//[ \t]*pool-fallback"
    a_hits = filter_out(
        filter_out(
            grep_lines(content, r"append[ \t]*\("),
            r"(args|strings|labels|families|errors|ERRORS|names|fields|parts)",
            ignorecase=True,
        ),
        r"//.*append",
        ignorecase=True,
    )
    for n, l in a_hits[:2]:
        reasons.append(
            f"  L{n}: append() -> use caller-owned buffer with WriteTo(buf, off)"
        )
    m_hits = filter_out(
        grep_lines(content, r"make[ \t]*\([ \t]*\[[ \t]*\][ \t]*byte"),
        make_allow,
        ignorecase=True,
    )
    for n, l in m_hits[:2]:
        reasons.append(f"  L{n}: make([]byte) -> use pool.Get() or caller-owned buffer")
    b_hits = filter_out(
        grep_lines(content, r"\.[ \t]*Bytes[ \t]*\([ \t]*\)"),
        r"(rd\.Bytes|spec\.|json\.|\.String\(\)\.Bytes)",
        ignorecase=True,
    )
    for n, l in b_hits[:2]:
        reasons.append(f"  L{n}: .Bytes() -> use WriteTo(buf, off) pattern")
    p_hits = grep_lines(content, r"\.[ \t]*Pack[ \t]*\([ \t]*\)")
    for n, l in p_hits[:2]:
        reasons.append(f"  L{n}: .Pack() -> use WriteTo(buf, off) pattern")
    bf_hits = grep_lines(
        content, r"func[ \t]+build[A-Za-z0-9_]+\(.*\)[ \t]*\([ \t]*\[[ \t]*\][ \t]*byte"
    )
    for n, l in bf_hits[:2]:
        reasons.append(
            f"  L{n}: build*() returning []byte -> use WriteTo(buf, off) pattern"
        )
    len_hits = filter_out(
        grep_lines(content, r"\.Len\(\)"),
        r"(CheckedWriteTo|WriteAttrToWithLen|// .*Len)",
        ignorecase=True,
    )
    if len_hits:
        wa = [
            l
            for _, l in grep_lines(content, r"WriteAttrTo\(")
            if not re.search(r"WriteAttrToWithLen|WriteAttrToWithContext", l)
        ]
        if wa:
            for n, l in len_hits[:2]:
                reasons.append(
                    f"  L{n}: .Len() with WriteAttrTo -> use WriteAttrToWithLen"
                )
    if reasons:
        detail = "\n".join(reasons[:6])
        fix = (
            "\n  Encoding hot paths must be zero-alloc. Use:\n"
            "    WriteTo(buf []byte, off int) int   -- write into caller-owned buffer\n"
            "    pool.Get() / pool.Release()         -- for buffers that must be allocated\n"
            "  See ai/rules/performance.md, ai/rules/performance.md"
        )
        return (
            2,
            f"{RED}{BOLD}❌ BLOCKED: per-call allocation in encoding hot path{RESET}\n{detail}{fix}",
        )
    return None


# ze point: performance/hot-path-rule/apply-the-hot-path-ban-to-these-packages
def c_format_alloc(ctx):
    # ENABLED 2026-07-09 (spec-followup-hooks). Previously a deliberate no-op:
    # the original block-format-alloc.sh used a bash-4 `declare -A` table that
    # the macOS bash-3.2 shebang could not run, so the guard silently exited 0
    # and never fired. The list is now corrected (bgp/attribute/text.go was
    # removed with the attribute package in 3e66070f8; bgp/format/json.go added)
    # and comment lines are exempt exactly like c_sprintf_new (:321,:325).
    # Incremental value over c_sprintf_new (which already bans fmt.Sprintf/
    # Fprintf/Printf + strconv.Format* everywhere): the strings.Join / Builder /
    # NewReplacer / ReplaceAll bans that keep the format files buffer-first.
    fp = ctx["fp"]
    if not _go_we(ctx) or re.search(r"_test\.go$", fp) or not ctx["content"]:
        return None
    fmt_files = [
        "/bgp/reactor/filter_format.go",
        "/bgp/format/text.go",
        "/bgp/format/text_json.go",
        "/bgp/format/text_update.go",
        "/bgp/format/text_human.go",
        "/bgp/format/summary.go",
        "/bgp/format/codec.go",
        "/bgp/format/decode.go",
        "/bgp/format/json.go",
    ]
    if not any(fp.endswith(f) for f in fmt_files):
        return None
    banned = [
        r"fmt\.Sprintf\(",
        r"fmt\.Fprintf\(",
        r"strings\.Join\(",
        r"strings\.Builder",
        r"strings\.NewReplacer",
        r"strings\.ReplaceAll\(",
        r"strconv\.FormatUint\(",
        r"strconv\.FormatInt\(",
    ]
    for p in banned:
        hits = filter_out(grep_lines(ctx["content"], p), r"//.*" + p)
        if hits:
            lines = [f"  L{n}: {l.strip()}" for n, l in hits[:4]]
            detail = "\n".join(lines)
            fix = (
                "\n  Format files stay buffer-first (ai/rules/performance.md):\n"
                "    strings.Join/Builder/NewReplacer/ReplaceAll -> textbuf.Buffer helpers\n"
                "    fmt.Sprintf/Fprintf -> textbuf.Buffer chain (see sprintf-new fixups)\n"
                "  Comment lines naming these primitives are exempt."
            )
            return (
                2,
                f"{RED}{BOLD}✘ BLOCKED: banned format primitive in {fp}{RESET}\n{detail}{fix}",
            )
    return None


# ze point: config/directives/manipulate-config-only-by-the-two-approved-methods
def c_silent_ignore(ctx):
    fp = ctx["fp"]
    if (
        not _go_we(ctx)
        or re.search(r"_test\.go$", fp)
        or "cmd/" in fp
        or "internal/test/" in fp
        or "/scripts/" in fp
    ):
        return None
    content = ctx["content"]
    errs = False
    for p in [
        r"continue[ \t]*//[ \t]*ignore",
        r"return[ \t]*nil[ \t]*//[ \t]*ignore",
        r"//[ \t]*silently[ \t]*ignore",
        r"//[ \t]*skip[ \t]*unknown",
    ]:
        m = grep_lines(content, p, ignorecase=True)
        if m and not any(
            re.search(r"(forbidden|wrong|bad|dont|do not)", l, re.IGNORECASE)
            for _, l in m
        ):
            errs = True
    # empty default: case
    lines = content.split("\n")
    in_def = False
    for ln in lines:
        if re.match(r"^[ \t]*default:[ \t]*$", ln):
            in_def = True
            continue
        if in_def:
            if re.match(r"^[ \t]*$", ln) or re.match(r"^[ \t]*//", ln):
                continue
            if re.match(r"^[ \t]*}[ \t]*$", ln):
                errs = True
            in_def = False
    if "/config/" in fp and grep_any(content, r"default:[ \t]*(//|break[ \t]*$)"):
        errs = True
    if errs:
        found = []
        for p in [
            r"continue[ \t]*//[ \t]*ignore",
            r"return[ \t]*nil[ \t]*//[ \t]*ignore",
            r"//[ \t]*silently[ \t]*ignore",
            r"//[ \t]*skip[ \t]*unknown",
        ]:
            for n, l in grep_lines(content, p, ignorecase=True)[:2]:
                found.append(f"  L{n}: {l.strip()}")
        detail = "\n".join(found[:4]) if found else "  (empty default: case detected)"
        fix = (
            "\n  Never silently drop data. Handle every case explicitly:\n"
            '    return fmt.Errorf("unknown kind %q", kind)\n'
            '    default: slog.Warn("unhandled", "kind", kind)\n'
            "  Allowed in: _test.go, cmd/, internal/test/, scripts/"
        )
        return (
            2,
            f"{RED}{BOLD}❌ BLOCKED: Silent ignore pattern{RESET}\n{detail}{fix}",
        )
    return None


# ze point: evidence/directives/derive-every-string-from-the-canonical-registry
def c_hardcoded_commands(ctx):
    fp = ctx["fp"]
    if not _go_we(ctx) or re.search(r"_test\.go$", fp) or not ctx["content"]:
        return None
    rx = r'"(show|set|del|update|validate|monitor|clear|help|config|bgp|cli|schema|plugin|doctor|version|signal|completion|status|init)"'
    in_blk = False
    words = 0
    hit = False
    for line in ctx["content"].split("\n"):
        if re.match(r"^[ \t]*//", line):
            continue
        if not in_blk and re.search(r"\[\]string\{", line):
            in_blk = True
            words = 0
        if in_blk:
            words += len(re.findall(rx, line))
            if "}" in line:
                if words >= 4:
                    hit = True
                in_blk = False
    if hit:
        fix = (
            "\n  Derive command lists from the registry, never hardcode.\n"
            "  Use the registration pattern: iterate registered commands at runtime.\n"
            "  See ai/rules/evidence.md"
        )
        return (
            2,
            f"{RED}{BOLD}✘ BLOCKED: possible hardcoded command list in {fp}{RESET}{fix}",
        )
    return None


# ze point: go-standards/directives/never-write-these-forbidden-go-patterns
def c_init_register(ctx):
    fp = ctx["fp"]
    if not _go_we(ctx) or re.search(r"_test\.go$", fp):
        return None
    base = os.path.basename(fp)
    if base == "register.go" or _glob_match(base, "register_*.go"):
        return None
    content = ctx["content"]
    if grep_any(content, r"RegisterRPCs\("):
        return None
    if not grep_any(content, r"^func init\(\)"):
        return None
    # init body: sed -n '/^func init()/,/^func /p' | head -30
    body = []
    started = False
    for line in content.split("\n"):
        if not started:
            if re.search(r"^func init\(\)", line):
                started = True
                body.append(line)
        else:
            body.append(line)
            if re.search(r"^func ", line):
                break
    body = body[:30]
    btext = "\n".join(body)
    for p in [
        r"Register",
        r"Add.*Handler",
        r"Subscribe",
        r"Hook",
        r"global.*=",
        r"default.*=",
    ]:
        if grep_any(btext, p, ignorecase=True):
            fix = (
                "\n  Registration belongs in register.go (or register_*.go), not init().\n"
                "  Move Register/Subscribe/AddHandler/Hook calls to register.go.\n"
                "  Global assignments belong in var blocks, not init().\n"
                "  See ai/patterns/registration.md"
            )
            return (
                2,
                f"{RED}{BOLD}❌ BLOCKED: Implicit behavior in init(){RESET}{fix}",
            )
    return None


# --------------------------------------------------------------------------- #
# Other file-type / filename / filesystem checks
# --------------------------------------------------------------------------- #


# ze point: quality/linting/fix-lint-issues-never-disable-a-linter
def c_lint_exclusions(ctx):
    fp = ctx["fp"]
    if ctx["tool"] not in ("Write", "Edit"):
        return None
    if (
        not re.search(r"\.golangci", fp)
        and "golangci.yml" not in fp
        and "golangci.yaml" not in fp
    ):
        return None
    content = ctx["content"]
    errs = False
    if grep_any(
        content, r"exclude-rules:|exclude:|issues-exclude:|skip-files:|skip-dirs:"
    ):
        if grep_any(content, r"^[ \t]*-[ \t]*(path|text|linters|source):"):
            errs = True
    # `grep -qE 'disable:...' | grep -vE '#.*disable'` -> pipeline of grep -q always succeeds(rc0) since
    # grep -q produces no stdout; the second grep -v on empty input -> rc1 -> overall false. Faithful: never fires.
    if errs:
        fix = (
            "\n  Fix the code to satisfy the linter, do not add exclusions.\n"
            "  If the lint is genuinely wrong, use //nolint:name // reason on the line."
        )
        return (
            2,
            f"{RED}{BOLD}❌ BLOCKED: Adding linter exclusions{RESET}{fix}",
        )
    return None


# ze point: config/directives/manipulate-config-only-by-the-two-approved-methods
def c_version_config(ctx):
    fp = ctx["fp"]
    if ctx["tool"] not in ("Write", "Edit"):
        return None
    if (
        "/config/" not in fp
        and not fp.endswith(".conf")
        and not re.search(r"config.*\.go$", fp)
    ):
        return None
    for p in [
        r"version[ \t]*[=:][ \t]*[0-9]",
        r"Version[ \t]*[=:][ \t]*[0-9]",
        r'"version"[ \t]*:',
        r"version[ \t]+[0-9]+[ \t]*;",
        r"config.?version",
        r"schema.?version",
    ]:
        if grep_any(ctx["content"], p, ignorecase=True):
            fix = (
                "\n  Ze config is YANG-modeled and unversioned.\n"
                "  Schema evolution uses YANG augment/deprecate, not version numbers.\n"
                "  See ai/rules/config.md"
            )
            return (
                2,
                f"{RED}{BOLD}❌ BLOCKED: Version in config{RESET}{fix}",
            )
    return None


# ze point: performance/common-mistakes/fix-these-common-allocation-mistakes
def c_fake_bufhandle(ctx):
    fp = ctx["fp"]
    if not fp.endswith(".go"):
        return None
    tool = ctx["tool"]
    if tool == "Write":
        content = ctx["ti"].get("content") or ""
    elif tool in ("Edit", "MultiEdit"):
        content = ctx["ti"].get("new_string") or ""
    else:
        return None
    bad = grep_lines(content, r"BufHandle\{[^}]*Buf:[ \t]*make")
    if not bad:
        return None
    bad = [
        (n, l)
        for (n, l) in bad
        if not re.match(r"^[ \t]*[0-9]*:[ \t]*//", f"{n}:{l}")
        and "noPoolBufID" not in l
    ]
    # the comment filter operates on "N:line"; emulate grep -v '^\s*[0-9]*:\s*//'
    bad2 = []
    for n, l in grep_lines(content, r"BufHandle\{[^}]*Buf:[ \t]*make"):
        s = f"{n}:{l}"
        if re.match(r"^[ \t]*[0-9]*:[ \t]*//", s):
            continue
        if "noPoolBufID" in l:
            continue
        bad2.append((n, l))
    if bad2:
        lines = [f"  L{n}: {l.strip()}" for n, l in bad2[:4]]
        detail = "\n".join(lines)
        fix = (
            "\n  Use pool.Get() to obtain BufHandles, never construct with make().\n"
            "  Only noPoolBufID-tagged constructions are allowed (pool bootstrap).\n"
            "  See ai/rules/performance.md"
        )
        return (
            2,
            f"{RED}{BOLD}BLOCKED: fake BufHandle construction in {fp}{RESET}\n{detail}{fix}",
        )
    return None


# ze point: testing/observer-exit-antipattern-in-ci-tests-blocking/fail-a-ci-observer-with-runtime-fail-not-sys-exit
def c_observer_sys_exit(ctx):
    fp = ctx["fp"]
    if not fp.endswith(".ci"):
        return None
    tool = ctx["tool"]
    if tool == "Write":
        content = ctx["ti"].get("content") or ""
    elif tool in ("Edit", "MultiEdit"):
        content = ctx["ti"].get("new_string") or ""
    else:
        return None
    if not grep_any(content, r"sys\.exit\(1\)"):
        return None
    if grep_any(content, r"runtime_fail"):
        return None
    if grep_any(content, r"^(expect|reject)=stderr"):
        return None
    fix = (
        "\n  Use runtime_fail instead of sys.exit(1), or add expect=stderr/reject=stderr.\n"
        "  sys.exit(1) without these makes the observer exit non-deterministically."
    )
    return (
        1,
        f"{YELLOW}{BOLD}WARN: observer-exit antipattern in {fp}{RESET}{fix}",
    )


# ze point: architecture/zefs-persistence-no-loose-state-files/persist-runtime-state-in-zefs-not-a-loose-file
def c_direct_fs_state(ctx):
    # Edit-time nudge (the authoritative block is `make ze-fs-persistence-check`,
    # scripts/checks/direct_fs_persistence.go): runtime STATE must persist through
    # the managed zefs store, not a loose file. Non-blocking -- kernel/proc,
    # ephemeral, external-artifact and storage-layer writes are legitimate.
    fp = ctx["fp"]
    if not _go_we(ctx) or re.search(r"_test\.go$", fp) or not ctx["content"]:
        return None
    if not re.search(r"/(internal/plugins|internal/component|cmd/ze)/", fp):
        return None
    hits = grep_lines(
        ctx["content"],
        r"os\.(WriteFile|Create|Rename|Symlink|Link)\(|os\.OpenFile\([^)]*O_(CREATE|WRONLY|RDWR|APPEND|TRUNC)",
    )
    if not hits:
        return None
    fix = (
        "\n  If this persists runtime STATE (baseline, snapshot, sequence number,"
        "\n  hash, cache), use internal/core/statestore (a registered pkg/zefs key)"
        "\n  or your storage.Storage handle, not a loose file -- on the appliance only"
        "\n  database.zefs is managed/backed-up. Kernel/proc, ephemeral, external and"
        "\n  storage-layer writes are fine (allowlisted). Enforced by"
        "\n  `make ze-fs-persistence-check`; see ai/rules/architecture.md."
    )
    return (
        1,
        f"{YELLOW}{BOLD}WARN: raw filesystem write in {fp}{RESET}{fix}",
    )


# ze point: repo-maintenance/canonical-sources-and-sync-direction/edit-the-canonical-source-not-the-generated-file
# ze point: repo-maintenance/canonical-sources-and-sync-direction/sync-generated-files-from-their-canonical-source
def c_generated_files(ctx):
    """Block edits to THIS project's generated CLAUDE.md / AGENTS.md.

    Matched by full path, not basename: only the two at the project root are
    generated from ai/INSTRUCTIONS.md. A basename match also caught the user's
    hand-maintained ~/.claude/CLAUDE.md and any CLAUDE.md in another checkout,
    telling their author to edit an ai/INSTRUCTIONS.md that does not govern them.
    """
    if ctx["tool"] not in ("Write", "Edit"):
        return None
    base = os.path.basename(ctx["fp"])
    if base not in ("CLAUDE.md", "AGENTS.md"):
        return None
    generated = os.path.realpath(os.path.join(PROJECT_DIR, base))
    # A relative file_path resolves against the CWD, which is not the project dir
    # for every caller; join it to PROJECT_DIR so the check cannot fail OPEN there.
    given = (
        ctx["fp"] if os.path.isabs(ctx["fp"]) else os.path.join(PROJECT_DIR, ctx["fp"])
    )
    if os.path.realpath(given) != generated:
        return None
    fix = (
        f"\n  {base} is auto-generated. Edit the canonical source instead:\n"
        "    ai/INSTRUCTIONS.md  (then run the sync script)\n"
        "  See ai/rules/repo-maintenance.md"
    )
    return (2, f"BLOCKED: {base} is generated{fix}")


# The generated files that sit DIRECTLY in ai/rules/, and how each is rebuilt.
# A rendered rule (any other `*.md` there) comes from ai/rules/points/<stem>/.
_RULES_ARTIFACTS = {
    "INDEX.md": ("scripts/dev/rules_index.py", "make ze-rules-index"),
    "TRIGGERS.md": ("scripts/dev/rules_condensed.py", "make ze-rules-condensed"),
    "CORE.md": ("scripts/dev/rules_condensed.py", "make ze-rules-condensed"),
}


# ze point: repo-maintenance/canonical-sources-and-sync-direction/keep-shared-rules-in-ai-rules-and-render-them
# ze point: repo-maintenance/canonical-sources-and-sync-direction/sync-generated-files-from-their-canonical-source
def c_rendered_rules(ctx):
    """Block edits to the generated files sitting directly in ai/rules/.

    Every `ai/rules/<rule>.md` is RENDERED from `ai/rules/points/<rule>/` by
    scripts/dev/rules_points.py, so an edit there is overwritten by the next
    `make ze-rules-render` and silently lost. The three all-caps artifacts beside
    them (INDEX.md, TRIGGERS.md, CORE.md) are generated too, from the rendered
    rules; nothing guarded them before this check existed.

    Matched by full path against PROJECT_DIR, for the reason c_generated_files
    records: a basename or suffix match would also catch a rule file in another
    checkout and send its author to a points directory that does not govern them.
    Points themselves are the canonical source and are always permitted -- they
    sit two or more components below ai/rules, never one, so the depth test lets
    them through by construction (spec AC-8).

    Matched by IDENTITY, never by comparing realpath strings: see `_same_path`.
    `<repo>/AI/rules/performance.md` opened the generated rule and exited 0.

    Fails CLOSED: when the path cannot be resolved the answer is a refusal, never
    a permit, because the file this could not classify might be a rendered rule
    (`ai/rules/evidence.md`).

    NotebookEdit is deliberately absent from the tool set. `main()` reads
    `file_path` and NotebookEdit sends `notebook_path`, so that branch could
    never be reached; and a notebook is not markdown, so nothing under ai/rules/
    is a notebook. Listing it asserted coverage the entry point cannot deliver.
    """
    if ctx["tool"] not in ("Write", "Edit", "MultiEdit"):
        return None
    fp = ctx["fp"]
    if not fp:
        return None
    try:
        given = fp if os.path.isabs(fp) else os.path.join(PROJECT_DIR, fp)
        resolved = os.path.realpath(given)
        rules_dir = os.path.realpath(os.path.join(PROJECT_DIR, "ai", "rules"))
        tail = _tail_under(rules_dir, resolved)
    except (OSError, ValueError):
        return (
            2,
            f"{RED}{BOLD}❌ BLOCKED: cannot resolve {fp} against the project"
            f" directory{RESET}\n  Refusing rather than permitting: it may be a"
            "\n  generated file under ai/rules/. Pass an absolute path.",
        )
    if tail is None or len(tail) != 1:
        return None
    # Named from the ON-DISK spelling, never the one the payload typed. A
    # symlink from outside is correctly blocked either way, and the refusal has
    # to name the file the author must actually edit. A case-insensitive
    # filesystem opens INDEX.md for `ai/rules/index.md`.  <!-- doc-links: ignore (the lowercase spelling an author types; the file on disk is INDEX.md) -->
    # The fix line has to send them to rules_index.py.
    base = tail[0]
    if not base.endswith(".md"):
        return None

    if base in _RULES_ARTIFACTS:
        script, target = _RULES_ARTIFACTS[base]
        fix = (
            f"\n  ai/rules/{base} is generated by {script}."
            f"\n  Edit the rule's point under ai/rules/points/<rule>/, then run:"
            f"\n    make ze-rules-render && {target}"
            '\n  See ai/rules/repo-maintenance.md, "Canonical Sources and Sync Direction"'
        )
        return (2, f"{RED}{BOLD}❌ BLOCKED: ai/rules/{base} is generated{RESET}{fix}")

    stem = base[: -len(".md")]
    if stem.isupper():
        fix = (
            f"\n  ai/rules/{base} sits beside the rendered rules and is not one."
            "\n  Find the generator that owns it before editing; run `make ze-regen`"
            "\n  to rebuild every generated file. See ai/rules/repo-maintenance.md"
        )
        return (2, f"{RED}{BOLD}❌ BLOCKED: ai/rules/{base} is generated{RESET}{fix}")

    fix = (
        f"\n  ai/rules/{base} is RENDERED from ai/rules/points/{stem}/ by"
        "\n  scripts/dev/rules_points.py. An edit here is lost at the next render."
        f"\n  Edit the point that carries the instruction:"
        f"\n    ai/rules/points/{stem}/<section>/<slug>.md   one block of the rule"
        f"\n    ai/rules/points/{stem}/manifest.md           title, When, Severity,"
        "\n                                                 sections and reading order"
        "\n  Then run: make ze-rules-render"
        '\n  See ai/rules/repo-maintenance.md, "Rule Placement"'
    )
    return (2, f"{RED}{BOLD}❌ BLOCKED: ai/rules/{base} is generated{RESET}{fix}")


# ze point: never-destroy-work/forbidden-without-explicit-permission/ask-before-deleting-or-overwriting-user-work
def c_point_overwrite(ctx):
    """Refuse a Write that would replace an EXISTING rule point.

    A point file is the canonical source of one instruction, and a Write
    replaces the whole file. Writing over a slug that is already taken deletes
    that instruction at the moment of the write, and every gate downstream of it
    runs afterwards: `make ze-rules-render` reports the manifest duplicate, and
    `make ze-rules-render-check` reports the rendered drift, but the bytes are
    already gone by then and only git holds them. That is what happened to one
    point of ai/rules/points/repo-maintenance/, recovered only from git.

    A SIBLING of c_rendered_rules rather than a branch inside it. That check
    answers "is this a GENERATED file", and its verdict for every path under
    ai/rules/points/ is permit-by-construction: the early return on the dirname
    is exactly what makes spec AC-8 true. This one answers the opposite
    question, about the canonical source, and its verdict depends on the TOOL
    and on whether the file already exists. Folding two opposite polarities into
    one function would make the AC-8 early return something to work around.

    Edit stays permitted: it is targeted, it fails when its old_string does not
    match, and it cannot silently drop a body. MultiEdit is refused on the ONE
    shape that can: an empty `old_string` matches at position 0 and replaces the
    whole file, which is a Write wearing another name. That branch is unreachable
    in a harness whose tool set carries no MultiEdit, and it is written because
    the previous docstring asserted MultiEdit was safe without anything proving
    it. A Write to a NEW path stays permitted, because that is how a point is
    authored (`ai/rules/never-destroy-work.md` bans destroying work, not
    creating it).

    TWO shapes are canonical, because the tree is at a fixed depth of two:
    ai/rules/points/<rule>/manifest.md carries the whole spine, and
    ai/rules/points/<rule>/<section>/<slug>.md carries one instruction. A path at
    any other depth is not a canonical source, so it is left alone.

    Matched by IDENTITY, never by comparing realpath strings: see `_same_path`.
    A Write through `<repo>/AI/rules/points/...` landed on the real point and
    exited 0, which is this check's own failure mode with different capitals.
    """
    tool = ctx["tool"]
    if tool == "Write":
        replaces = True
    elif tool == "MultiEdit":
        replaces = any(
            not (e or {}).get("old_string") for e in (ctx["ti"].get("edits") or [])
        )
    else:
        replaces = False
    if not replaces:
        return None
    fp = ctx["fp"]
    if not fp:
        return None
    try:
        given = fp if os.path.isabs(fp) else os.path.join(PROJECT_DIR, fp)
        resolved = os.path.realpath(given)
        points_dir = os.path.realpath(
            os.path.join(PROJECT_DIR, "ai", "rules", "points")
        )
        tail = _tail_under(points_dir, resolved)
    except (OSError, ValueError):
        return (
            2,
            f"{RED}{BOLD}❌ BLOCKED: cannot resolve {fp} against the project"
            f" directory{RESET}\n  Refusing rather than permitting: it may be a"
            "\n  rule point under ai/rules/points/. Pass an absolute path.",
        )
    if tail is None:
        return None
    if len(tail) == 2 and tail[1] == "manifest.md":
        # ai/rules/points/<rule>/manifest.md
        stem, rel = tail[0], "/".join(tail)
        what = (
            "the sections, the reading order, the title, the trigger and the severity"
        )
    elif len(tail) == 3:
        # ai/rules/points/<rule>/<section>/<slug>.md
        stem, rel = tail[0], "/".join(tail)
        what = "one instruction of the rule"
    else:
        return None
    if not os.path.isfile(resolved):
        return None

    fix = (
        f"\n  ai/rules/points/{rel} already exists and carries {what}."
        "\n  A Write replaces the whole file, so the instruction in it would be"
        "\n  gone before any gate ran. Two routes, both non-destructive:"
        f"\n    Edit ai/rules/points/{rel}   change the point that is there"
        f"\n    Write ai/rules/points/{stem}/<section>/<a-free-slug>.md   then add"
        "\n      the slug to that section in the rule's manifest.md"
        "\n  See ai/rules/never-destroy-work.md and docs/contributing/rule-authoring.md"
    )
    return (
        2,
        f"{RED}{BOLD}❌ BLOCKED: Write over an existing rule point{RESET}{fix}",
    )


# RFC 2119 / RFC 8174 keywords and the lowercase spellings that are refused in
# their place. Kept in sync with RFC_LEVELS / LOWER_MODAL in
# scripts/dev/rules_lint.py: that pass refuses the same file at gate time, and a
# keyword one accepts and the other does not would make them disagree.
RFC_KEYWORD_RE = re.compile(
    r"\b(?:MUST NOT|SHALL NOT|SHOULD NOT|NOT RECOMMENDED|MUST|SHALL|REQUIRED"
    r"|SHOULD|RECOMMENDED|MAY|OPTIONAL)\b"
)
LOWER_MODAL_RE = re.compile(r"(?<![\w-])(must|shall|should|may)\b(?![-\w])")
POINT_FENCE_RE = re.compile(r"^```.*?^```", re.M | re.S)
POINT_CODE_SPAN_RE = re.compile(r"`[^`]*`")


def _point_visible(text):
    """The words a point STATES: fenced blocks and code spans are quoted, not stated."""
    return POINT_CODE_SPAN_RE.sub("", POINT_FENCE_RE.sub("", text))


# ze point: rule-format/every-directive-states-a-level/every-directive-states-its-rfc-2119-level
def c_rule_point_rfc_language(ctx):
    """Refuse a directive point that does not state its obligation in RFC 2119 language.

    A rule exists to settle what an agent owes. A directive whose weight a reader
    infers from tone is a directive two readers weigh differently, and the corpus
    carried 509 of them before this check landed. The capitalised keyword is the
    whole fix: it says whether the instruction binds, recommends, or permits, and
    it says it in the one vocabulary every reader here already shares.

    Scoped to `kind: directive`. A `table` is usually a lookup and a `note` is
    usually context; forcing MUST into a two-column glossary would add a word
    without adding an obligation.

    Two shapes, one refusal each. A Write carries the WHOLE point, so the missing
    keyword is decidable and refused. An Edit carries a fragment, so the keyword
    may legitimately sit in the untouched part of the file -- what is decidable
    there is the lowercase modal being INTRODUCED, which is refused for both
    tools. `make ze-rules-lint` reads the finished file and owns the rest.
    """
    fp = ctx["fp"]
    content = ctx["content"]
    if ctx["tool"] not in ("Write", "Edit", "MultiEdit") or not content:
        return None
    try:
        given = fp if os.path.isabs(fp) else os.path.join(PROJECT_DIR, fp)
        resolved = os.path.realpath(given)
        points_dir = os.path.realpath(
            os.path.join(PROJECT_DIR, "ai", "rules", "points")
        )
        tail = _tail_under(points_dir, resolved)
    except (OSError, ValueError):
        return None
    if tail is None or len(tail) != 3:
        return None

    whole = ctx["tool"] == "Write"
    if whole:
        is_directive = re.search(r"^kind:[ \t]*directive[ \t]*$", content, re.M)
    else:
        try:
            head = open(resolved, encoding="utf-8", errors="replace").read(400)
        except OSError:
            return None
        is_directive = re.search(r"^kind:[ \t]*directive[ \t]*$", head, re.M)
    if not is_directive:
        return None

    rel = "/".join(tail)
    visible = _point_visible(content)
    lower = sorted(set(LOWER_MODAL_RE.findall(visible)))
    if lower:
        words = ", ".join(repr(w) for w in lower)
        return (
            2,
            f"{RED}{BOLD}❌ BLOCKED: lowercase obligation word in a rule "
            f"directive{RESET}"
            f"\n  ai/rules/points/{rel} states {words} in lowercase."
            "\n  A directive says what an agent owes, so it says it in RFC 2119"
            "\n  language: MUST, MUST NOT, SHOULD, SHOULD NOT, MAY."
            "\n  Capitalise the keyword, or rewrite the sentence to carry no"
            "\n  modal at all. ai/rules/writing.md bans the hedging spelling.",
        )
    if whole and not RFC_KEYWORD_RE.search(visible):
        return (
            2,
            f"{RED}{BOLD}❌ BLOCKED: rule directive states no RFC 2119 level{RESET}"
            f"\n  ai/rules/points/{rel} declares `kind: directive` and states no"
            "\n  capitalised keyword, so nothing in it says whether the"
            "\n  instruction binds, recommends, or permits."
            "\n  Use MUST / MUST NOT for an obligation, SHOULD / SHOULD NOT for a"
            "\n  strong default, MAY for a permission, and set `level:` to the"
            "\n  strongest one the body states."
            "\n  A block that states no obligation is `kind: note` or"
            "\n  `kind: table`, not `kind: directive`."
            "\n  See ai/rules/rule-format.md 'Every directive states a level'",
        )
    return None


# ze point: architecture/directives/load-ze-context-before-any-design-decision
def c_utils_package(ctx):
    fp = ctx["fp"]
    if ctx["tool"] != "Write" or not fp.endswith(".go"):
        return None
    errs = False
    if "/utils/" in fp or "/helpers/" in fp or "/common/" in fp or "/misc/" in fp:
        errs = True
    if grep_any(ctx["content"], r"^package[ \t]+(utils|helpers|common|misc)[ \t]*$"):
        errs = True
    if errs:
        fix = (
            "\n  No utils/helpers/common/misc packages. Place code where it belongs:\n"
            "    Domain logic    -> internal/component/<name>/ or internal/plugins/<name>/\n"
            "    Shared infra    -> internal/core/<name>/\n"
            "    Test helpers    -> internal/test/<name>/"
        )
        return (
            2,
            f"{RED}{BOLD}❌ BLOCKED: Forbidden package pattern{RESET}{fix}",
        )
    return None


# ze point: testing/no-throw-away-tests/put-each-test-in-the-suite-that-runs-its-format
# ze point: testing/temporary-files/use-project-tmp-for-scratch-files
def c_throwaway_tests(ctx):
    fp = ctx["fp"]
    if ctx["tool"] != "Write":
        return None
    errs = False
    if re.search(r"^/tmp/", fp) or re.search(r"^/var/tmp/", fp):
        if re.search(r"\.(go|py|sh)$", fp):
            errs = True
    if re.search(r"test_.*\.(go|py|sh)$", fp) or re.search(
        r"_test_.*\.(go|py|sh)$", fp
    ):
        if (
            not re.search(r"^.*/internal/", fp)
            and not re.search(r"^.*/test/", fp)
            and not re.search(r"^.*/cmd/", fp)
        ):
            errs = True
    if re.search(r"/main\.go$", fp) and not re.search(r"^.*/cmd/", fp):
        errs = True
    if errs:
        fix = (
            f"\n  File: {fp}\n"
            "  Tests belong in the source tree, not /tmp:\n"
            "    Go tests     -> internal/<pkg>/<name>_test.go  (next to source)\n"
            "    Functional   -> test/<suite>/<name>.ci\n"
            "    main.go      -> cmd/<binary>/main.go only"
        )
        return (
            2,
            f"{RED}{BOLD}❌ BLOCKED: Throwaway test file{RESET}{fix}",
        )
    return None


# ze point: none -- plan-file location is a Claude-only rule (.claude/rules/planning.md); only ai/rules/ has points
def c_claude_plans(ctx):
    fp = ctx["fp"]
    if ctx["tool"] != "Write":
        return None
    if re.search(r"\.claude/plans/", fp):
        return (
            2,
            "❌ BLOCKED: Do not use .claude/plans/\n"
            "  Write plans to .claude/plan/ze-plan-<name> instead.",
        )
    if re.search(r"^/Users/[^/]+/\.claude/plan/", fp):
        return (
            2,
            "❌ BLOCKED: Do not write plans to ~/.claude/plan/\n"
            "  Write to project .claude/plan/ze-plan-<name> instead.",
        )
    if re.search(r"\.claude/plan/", fp):
        if not re.search(r"^ze-plan-", os.path.basename(fp)):
            return (
                2,
                f"❌ BLOCKED: Plan file must be named 'ze-plan-<name>'\n"
                f"  Got: {os.path.basename(fp)}",
            )
    return None


# ze point: none -- no point states the file-naming convention (.md kebab-case, .go underscores, .sh kebab-case)
def c_enforce_naming(ctx):
    fp = ctx["fp"]
    if ctx["tool"] != "Write" or isfile(fp):
        return None
    base = os.path.basename(fp)
    errs = False
    kind = ""
    if fp.endswith(".md"):
        if re.match(
            r"^(README|INDEX|CLAUDE|LICENSE|CONTRIBUTING|CHANGELOG)\.md$", base
        ):
            return None
        if re.search(r"[A-Z]", base) or "_" in base:
            errs = True
            kind = "  .md files: lowercase kebab-case (e.g. my-feature.md)\n  Exceptions: README, INDEX, CLAUDE, LICENSE, CONTRIBUTING, CHANGELOG"
    if fp.endswith(".go"):
        if "-" in base:
            errs = True
            kind = "  .go files: no hyphens, use underscores (e.g. my_feature.go)"
        if re.search(r"[A-Z]", base) and not re.match(r"^[a-z_]+_[A-Z]", base):
            errs = True
            kind = "  .go files: lowercase (e.g. my_feature.go, not MyFeature.go)"
    if fp.endswith(".sh"):
        if re.search(r"[A-Z]", base) or "_" in base:
            errs = True
            kind = "  .sh files: lowercase kebab-case (e.g. my-script.sh)"
    if errs:
        return (
            1,
            f"{RED}{BOLD}❌ BLOCKED: Naming convention violation{RESET}\n  File: {base}\n{kind}",
        )
    return None


# --- what a spec is ABOUT ---
#
# The kinds mark-source-read.sh records, spelled identically at both ends: it
# writes tmp/session/.source-read-<KIND>-<SID> for a Read it accepts, and
# c_design_without_lsp below asks for the kind the spec under edit is about.
# The two lists are one contract; the fixtures in
# scripts/dev/hook-fixture-check.py (sections mark-source-read and design-gate)
# drive the writer and the reader over the SAME fixture project so they cannot
# drift apart silently, and design-gate-contract-both-ends-agree walks a path
# list through BOTH so a divergence is a named red rather than a hole.
_SOURCE_KINDS = ("go", "py", "sh", "make", "yang")

# A spec states its own subject in "## Files to Modify" and "## Files to Create".
# Each pattern maps one path to the kind of source that grounds a claim about it.
# EVERY kind the spec names must have been read: a spec listing a reactor `.go`
# beside an `mk/*.mk` claims things about both, and letting the cheaper one
# satisfy the gate would hand the author the choice of what counts as evidence.
# A section that yields NO kind never passes silently: the gate degrades to the
# older any-source bar and SAYS it did.
#
# THE KIND IS THE EXTENSION, WITH NO DIRECTORY ANCHOR (2026-08-08). These
# patterns must accept exactly what mark-source-read.sh records, because the
# author's way past a block is to READ THE FILE THE SPEC NAMES. While this end
# demanded `py` for any `*.py` and the writer recorded `py` only under `scripts/`
# and `.claude/hooks/`, 11 open specs (test/interop/interop.py,
# test/ipsec-interop/lab.py, tools/kernel-builder/build.py) and 2 more for `sh`
# (packaging/deb/preinstall.sh) had no readable route to their own marker: the
# sanctioned exit was reading an unrelated scripts/*.py, which manufactures the
# evidence the gate exists to demand. Anchoring THIS end to those directories
# instead would have made those 13 specs subjectless and dropped them to the
# weaker any-source bar -- a gate that stops asking, rather than one that can be
# answered. Dropping the `internal|pkg|cmd` anchor on `.go` costs nothing and
# buys the same property for Go: 6 specs naming only `test/**.go` demanded no
# kind at all before, and now name `go`.
_SUBJECT_PATTERNS = (
    (r"\.go$", "go"),
    (r"\.py$", "py"),
    (r"\.sh$", "sh"),
    (r"(?:^|/)Makefile$", "make"),
    (r"\.mk$", "make"),
    (r"\.yang$", "yang"),
)

# A path-shaped token, backticked or bare, in a bullet or in a table cell. The
# lists on disk are written all three ways, so reading only the backticked form
# derived NOTHING for 52 of the 232 open specs, and each of those dropped to the
# weaker bar with nothing said. A `/` is required, which keeps a prose mention of
# a bare `foo.go` from becoming a subject.
_PATH_TOKEN = re.compile(r"[A-Za-z0-9_.\-/]*/[A-Za-z0-9_.\-]+|Makefile\b")


def _subject_lines(body):
    """The part of each line that states a FILE, with description prose dropped.

    A `## Files to Modify` table is `| path | what changes |`, and the second
    cell is prose: "mirrors `scripts/dev/foo.py`" in a description used to make <!-- doc-links: ignore (example path in a docstring, deliberately absent) -->
    `py` a requirement of a spec that modifies no Python. Heading depth already
    keeps a `### Checklist` row from becoming a subject; this is the same hole
    one row further in. Only the first cell of a table row is a path column, so
    only the first cell is scanned. A bullet is left whole: its paths are not
    positional, and several specs list two files in one bullet.
    """
    for line in body.splitlines():
        line = line.strip()
        if line.startswith("|"):
            cells = line.split("|")
            yield cells[1] if len(cells) > 1 else ""
        else:
            yield line


def _spec_subject_kinds(text):
    """The source kinds a spec is about: its Files to Modify AND Files to Create.

    Both, because a spec whose new code is all under Files to Create is about
    that code exactly as much (`plan/spec-anomaly-0-umbrella.md` lists two docs
    to modify and its Go under Create).

    Each section ends at the next heading of ANY depth. Stopping only at `## `
    swallowed `### Integration Checklist` and `### Documentation Checklist`,
    whose rows name files the spec does not modify: 10 specs on disk gain a kind
    that way, and under the every-kind rule a checklist row would then decide
    what the author must read.

    Measured 2026-08-08 over the 240 open specs: 110 name one kind, 74 name
    several, and 56 name none the gate can read (a placeholder `internal/...`, or
    a bare directory). Those 56 take the weaker bar and are TOLD so. Every kind
    those 184 specs demand is reachable by reading a path the spec itself lists,
    which is the property `_SUBJECT_PATTERNS` and `mark-source-read.sh` exist to
    hold jointly. The tail is the price: 3 specs name four kinds and
    `plan/spec-release-distribution.md` names five, each on its own 30-minute
    clock (`plan/learned/HOOK-FRICTION.md`, `c_design_without_lsp`).
    """
    kinds = set()
    for m in re.finditer(
        r"^## Files to (?:Modify|Create)[^\n]*\n(.*?)(?=^#{2,6} |\Z)",
        text,
        re.M | re.S,
    ):
        for line in _subject_lines(m.group(1)):
            for token in _PATH_TOKEN.findall(line):
                path = token.strip().strip("`")
                for pattern, kind in _SUBJECT_PATTERNS:
                    if re.search(pattern, path):
                        kinds.add(kind)
                        break
    return kinds


def _spec_text(ctx):
    """The spec as the gate can see it: what is on disk, plus what is being written.

    An Edit hands over only its replacement text, so the Files to Modify section
    usually lives on disk; a Write of a new spec has no file yet and carries the
    whole document in the payload. A MultiEdit carries neither `content` nor
    `new_string`, and its `edits` list is the only place its text exists: reading
    just the first two made a MultiEdit-authored spec subjectless, which is the
    quiet degradation this gate must not have.
    """
    text = ""
    try:
        with open(ctx["fp"], encoding="utf-8", errors="replace") as fh:
            text = fh.read()
    except OSError:
        pass
    parts = [text, ctx["content"] or ""]
    for edit in ctx["ti"].get("edits") or []:
        if isinstance(edit, dict):
            parts.append(str(edit.get("new_string") or ""))
    return "\n".join(parts)


# ze point: evidence/no-fabrication/investigate-source-in-session-before-writing-a-spec
# ze point: evidence/no-fabrication/read-the-producing-code-before-claiming-behavior
def c_design_without_lsp(ctx):
    if ctx["tool"] not in ("Edit", "Write", "MultiEdit", "NotebookEdit"):
        return None
    rel = ctx["fp"]
    if rel.startswith(PROJECT_DIR + "/"):
        rel = rel[len(PROJECT_DIR) + 1 :]
    if not (_glob_match(rel, "plan/design-*.md") or _glob_match(rel, "plan/spec-*.md")):
        return None
    sid = session_id()
    # A spec/design write requires that the implementation was investigated this
    # session. Reading the function that PRODUCES a behavior is the verification
    # we want before authoring a spec that claims something about it
    # (ai/rules/evidence.md).
    #
    # WHICH source counts is the spec's OWN subject, and EVERY kind it names must
    # have been read. Three properties are load-bearing, and each one was a hole
    # first:
    #   * The LSP tool is gopls, so `.lsp-invoked` is evidence of kind `go` and of
    #     nothing else. An LSP-only session used to satisfy a Python spec.
    #   * Every kind, not any kind. A spec naming a reactor `.go` beside an
    #     `mk/*.mk` claims things about both, and any-of let the author pick the
    #     cheap file as the evidence for the expensive one.
    #   * Each kind's own freshness. Taking the newest over all kinds let a fresh
    #     `.mk` read carry a Go read that had gone stale.
    #   * The kind a spec DEMANDS must be a kind a Read can RECORD. The two ends
    #     are `_SUBJECT_PATTERNS` here and the `case` in mark-source-read.sh, and
    #     while they disagreed the sanctioned way past a block was to read a file
    #     the spec does not name. Both are keyed on the extension alone now.
    sess = os.path.join(PROJECT_DIR, "tmp/session")

    def _mtime(name):
        try:
            return os.stat(os.path.join(sess, name)).st_mtime
        except OSError:
            return None

    lsp = _mtime(f".lsp-invoked-{sid}")
    any_source = _mtime(f".source-read-{sid}")
    evidence = {k: _mtime(f".source-read-{k}-{sid}") for k in _SOURCE_KINDS}
    if lsp is not None:
        evidence["go"] = max(t for t in (evidence["go"], lsp) if t is not None)

    fresh = int(os.environ.get("LSP_FRESHNESS_SECONDS", "1800"))
    now = time.time()
    # What this says is what mark-source-read.sh MEASURES, and no more. It used to
    # say "read enough of it to have learned something", which reads as a floor
    # under every Read and is a bar the writer does not hold: a whole-file read
    # passes at any length, deliberately, and that is the operator's cheapest
    # honest route past a block. A refusal that asserts a property its producer
    # does not enforce is the shield that stops the next reader asking
    # (ai/rules/evidence.md).
    how = (
        "  Reading the source a spec is ABOUT satisfies this, and the kind is the\n"
        "  file's extension: .go, .py, .sh, .yang, the Makefile or a .mk -- anywhere in\n"
        "  the tree -- or, for Go only, the LSP tool. What counts as reading it: the\n"
        "  WHOLE file, at any length, or a window of at least 20 lines. A Read that\n"
        "  showed nothing counts as nothing, so re-reading a file whole a second time\n"
        "  renews nothing -- the harness answers that with 'file unchanged' and shows\n"
        "  you no lines. Read a window of it at an offset instead."
    )

    kinds = _spec_subject_kinds(_spec_text(ctx))
    if kinds:
        missing = [k for k in sorted(kinds) if evidence.get(k) is None]
        stale = [
            k
            for k in sorted(kinds)
            if evidence.get(k) is not None and now - evidence[k] > fresh
        ]
        if missing or stale:
            subject = ", ".join(sorted(kinds))
            detail = ""
            if missing:
                detail += "  Never read this session: %s\n" % ", ".join(missing)
            if stale:
                detail += "  Read, but longer ago than %ds: %s\n" % (
                    fresh,
                    ", ".join(stale),
                )
            return (
                2,
                "❌ Blocked [design-without-lsp]: this spec's own subject was not\n"
                "  investigated this session.\n"
                "  Its Files to Modify / Files to Create name %s, and EVERY kind they\n"
                "  name must be read: the author does not choose which one counts\n"
                "  (ai/rules/evidence.md).\n" % subject + detail + how,
            )
        return None

    # The subject could not be read. This is the ONLY permissive path left, so it
    # is never silent: the older any-source bar still applies, and the operator is
    # told the gate degraded and what to write to restore it.
    have = [t for t in list(evidence.values()) + [any_source] if t is not None]
    if not have:
        return (
            2,
            "❌ Blocked [design-without-lsp]: no implementation investigated this session\n"
            "  before a spec/design write.\n"
            "  Before specing a gap, READ the source that PRODUCES the behavior you are\n"
            "  claiming, not its caller (ai/rules/evidence.md, Behavioral claims).\n"
            + how,
        )
    if now - max(have) > fresh:
        return (
            2,
            "❌ Blocked [design-without-lsp]: implementation investigation is stale (> %ds).\n"
            % fresh
            + "  Re-read the source that produces the behavior, or use the LSP tool,\n"
            "  before editing the spec/design file.",
        )
    return (
        1,
        f"{YELLOW}⚠ design-without-lsp: no subject read from '## Files to Modify' or\n"
        "  '## Files to Create', so this spec was checked against the WEAKER bar: any\n"
        "  implementation source, of any kind. List the files this spec modifies or\n"
        f"  creates, each with its path, and the gate will ask for the kinds they name.{RESET}",
    )


# ze point: planning/spec-metadata-blocking/update-spec-status-at-each-transition
def c_source_edit_spec(ctx):
    fp = ctx["fp"]
    if ctx["tool"] not in ("Write", "Edit", "MultiEdit"):
        return None
    if not fp:
        return None
    if not (
        _glob_match(fp, "*/internal/*.go")
        or _glob_match(fp, "*/pkg/*.go")
        or _glob_match(fp, "*/cmd/*.go")
        or _glob_match(fp, "*/test/*")
        or _glob_match(fp, "*/plan/learned/*")
    ):
        return None
    sid = session_id()
    marker = os.path.join(PROJECT_DIR, "tmp/session", f".session-{sid}")
    if not os.path.isfile(marker):
        return None
    try:
        with open(marker) as fh:
            selected = fh.readline().strip()
    except Exception:
        return None
    if not selected or selected == "unassigned":
        return None
    spec_path = os.path.join(PROJECT_DIR, "plan", selected)
    if not os.path.isfile(spec_path):
        return None
    status = ""
    try:
        with open(spec_path) as fh:
            for line in fh:
                if re.match(r"^\| *Status *\|", line):
                    parts = line.split("|")
                    if len(parts) > 2:
                        status = parts[2].strip()
                    break
    except Exception:
        return None
    if status in ("skeleton", "design", "ready"):
        return (
            2,
            f"{RED}{BOLD}BLOCKED:{RESET} spec {selected} is `{status}` (flip to in-progress first)",
        )
    return None


# ze point: none -- session state is a Claude-only rule (.claude/rules/post-compaction.md), outside the corpus
def c_pre_write_go(ctx):
    fp = ctx["fp"]
    if ctx["tool"] not in ("Write", "Edit"):
        return None
    if not re.search(r"^.*/internal/.*\.go$", fp):
        return None
    sid = session_id()
    sstate = state_file(sid)
    selected = ""
    # The claim marker lives under tmp/session/ (lib/state-file.sh _claim_spec,
    # and scripts/dev/spec-session.sh). This read used the legacy .claude/ path,
    # which has one reader and zero writers repo-wide, so `selected` was always
    # empty and the "state file must mention the claimed spec" branch below was
    # dead. Note state_file() above already resolves the correct path, so this
    # one function was reading two different marker locations.
    marker = os.path.join(PROJECT_DIR, "tmp", "session", f".session-{sid}")
    if os.path.isfile(marker):
        try:
            with open(marker) as fh:
                selected = fh.readline().strip()
        except Exception:
            selected = ""
        if selected == "unassigned":
            selected = ""
    errs = False
    if not os.path.isfile(sstate):
        errs = True
    if selected:
        spec_path = os.path.join(PROJECT_DIR, "plan", selected)
        if os.path.isfile(spec_path):
            if not os.path.isfile(sstate) or not _file_contains(sstate, selected):
                errs = True
    if errs:
        return (2, f"{RED}No session state ({sstate}) - see post-compaction.md{RESET}")
    return None


# ze point: none -- same Claude-only rule as c_pre_write_go (.claude/rules/post-compaction.md)
def c_require_docs_read(ctx):
    fp = ctx["fp"]
    if ctx["tool"] != "Write":
        return None
    if not re.search(r"plan/spec-.*\.md$", fp):
        return None
    sid = session_id()
    sstate = state_file(sid)
    if not os.path.isfile(sstate):
        return (
            1,
            f"{RED}{BOLD}BLOCKED: Docs not verified{RESET}\n"
            f"  Session state file missing: {sstate}\n"
            "  Read the relevant docs/architecture files first to create session state.\n"
            "  See .claude/rules/post-compaction.md",
        )
    return None


# ze point: go-standards/design-document-references/every-go-file-carries-a-design-comment
def c_require_design_ref(ctx):
    fp = ctx["fp"]
    if ctx["tool"] not in ("Write", "Edit") or not fp.endswith(".go"):
        return None
    base = os.path.basename(fp)
    if (
        re.search(r"_test\.go$", base)
        or re.search(r"_gen\.go$", base)
        or base in ("register.go", "embed.go", "doc.go")
    ):
        return None
    if ctx["tool"] == "Write":
        content = ctx["ti"].get("content") or ""
        if "// Design:" in content:
            return None
        if grep_any(content[:500], r"Code generated|DO NOT EDIT"):
            return None
    else:  # Edit
        if isfile(fp) and _file_contains(fp, "// Design:"):
            return None
        if "// Design:" in (ctx["ti"].get("new_string") or ""):
            return None
        if isfile(fp):
            try:
                with open(fp) as fh:
                    if re.search(r"Code generated|DO NOT EDIT", fh.read(500)):
                        return None
            except Exception:
                pass
    fix = (
        "\n  Every non-test .go file needs a // Design: comment in the file header.\n"
        "  Format: // Design: <path-to-design-doc> -- <brief description>\n"
        "  Example: // Design: docs/architecture/bgp/reactor.md -- FSM state machine\n"
        "  Exempt: _test.go, _gen.go, register.go, embed.go, doc.go, generated files"
    )
    return (
        2,
        f"{RED}{BOLD}✘ BLOCKED: Missing // Design: comment{RESET}{fix}",
    )


# ze point: go-standards/file-cross-references/keep-file-cross-references-bidirectional
# ze point: go-standards/file-cross-references/update-cross-references-when-a-file-moves
def c_require_related_refs(ctx):
    fp = ctx["fp"]
    if ctx["tool"] not in ("Write", "Edit") or not fp.endswith(".go"):
        return None
    base = os.path.basename(fp)
    if (
        re.search(r"_test\.go$", base)
        or re.search(r"_gen\.go$", base)
        or base in ("register.go", "embed.go", "doc.go")
    ):
        return None
    directory = os.path.dirname(fp)
    if ctx["tool"] == "Write":
        content = ctx["ti"].get("content") or ""
    else:
        content = ""
        try:
            with open(fp) as fh:
                content = fh.read()
        except OSError:
            content = ""
        old = ctx["ti"].get("old_string", "")
        new = ctx["ti"].get("new_string", "")
        if ctx["ti"].get("replace_all", False):
            content = content.replace(old, new)
        else:
            content = content.replace(old, new, 1)
    xref = r"// (Detail|Overview|Related):"
    # Check 1: siblings referencing this file need a back-ref
    ref_from = ""
    try:
        for f in os.listdir(directory):
            if not f.endswith(".go") or f.endswith("_test.go") or f.endswith("_gen.go"):
                continue
            try:
                with open(os.path.join(directory, f)) as fh:
                    if re.search(
                        r"// (Detail|Overview|Related): " + re.escape(base) + " ",
                        fh.read(),
                    ):
                        ref_from = f
                        break
            except Exception:
                continue
    except Exception:
        pass
    if ref_from and not re.search(xref, content):
        fix = (
            f"\n  {ref_from} references {base} with // Related: or // Detail:\n"
            f"  Add a back-reference to {base}:\n"
            f"    // Related: {ref_from} -- <brief description>"
        )
        return (
            2,
            f"{RED}{BOLD}✘ BLOCKED: Missing cross-reference comment{RESET}{fix}",
        )
    # Check 2: stale refs
    for m in re.finditer(r"// (?:Detail|Overview|Related): ([^ ]*\.go)", content):
        ref = m.group(1)
        if not os.path.isfile(os.path.join(directory, ref)):
            return (
                2,
                f"{RED}{BOLD}✘ BLOCKED: Stale cross-reference{RESET}\n"
                f"  References {ref} but file does not exist in {os.path.basename(directory)}/\n"
                "  Update or remove the // Related: / // Detail: comment.",
            )
    return None


# ze point: testing/directives/write-the-test-first-and-never-weaken-it
def c_require_test_first(ctx):
    fp = ctx["fp"]
    if ctx["tool"] not in ("Write", "Edit") or not fp.endswith(".go"):
        return None
    if (
        re.search(r"_test\.go$", fp)
        or re.search(r"_gen\.go$", fp)
        or re.search(r"\.pb\.go$", fp)
        or "/cmd/" in fp
    ):
        return None
    test_file = fp[:-3] + "_test.go"
    if ctx["tool"] == "Write" and not isfile(fp):
        if not isfile(test_file):
            return (
                1,
                f"{RED}{BOLD}❌ BLOCKED: TDD - Write test first{RESET}\n"
                f"  Write the test file before the implementation:\n"
                f"    {test_file}\n"
                "  See ai/rules/testing.md",
            )
    return None


# ze point: architecture/design-context/reuse-the-existing-pattern-before-adding-one
def c_check_existing_patterns(ctx):
    fp = ctx["fp"]
    if ctx["tool"] != "Write":
        return None
    if (
        not re.search(r"^.*/internal/.*\.go$", fp)
        or isfile(fp)
        or re.search(r"_test\.go$", fp)
    ):
        return None
    pkg_dir = os.path.dirname(fp)
    if not os.path.isdir(pkg_dir):
        return None
    content = ctx["ti"].get("content") or ""
    types = re.findall(r"type[ \t]+([A-Z][a-zA-Z0-9]*)[ \t]+struct", content)[:5]
    funcs = [
        m
        for m in re.findall(r"^func[ \t]+([A-Z][a-zA-Z0-9]*)\(", content, re.MULTILINE)
    ][:5]
    go_files = [
        os.path.join(pkg_dir, f)
        for f in (os.listdir(pkg_dir) if os.path.isdir(pkg_dir) else [])
        if f.endswith(".go") and not f.endswith("_test.go")
    ]

    def defined(pattern):
        for gf in go_files:
            try:
                with open(gf) as fh:
                    head = "".join([next(fh) for _ in range(3)]) if False else ""
            except Exception:
                pass
            try:
                with open(gf) as fh:
                    txt = fh.read()
                if re.search(r"//go:build", "\n".join(txt.split("\n")[:3])):
                    continue
                if re.search(pattern, txt):
                    return True
            except Exception:
                continue
        return False

    for t in types:
        if defined(r"type[ \t]+" + re.escape(t) + r"[ \t]+struct"):
            return (
                2,
                f"{RED}❌ Duplicate: Type '{t}' ALREADY EXISTS in this package:{RESET}",
            )
    for fn in funcs:
        if defined(r"^func[ \t]+" + re.escape(fn) + r"[ \t]*\("):
            return (
                2,
                f"{RED}❌ Duplicate: Function '{fn}' ALREADY EXISTS in this package:{RESET}",
            )
    return None


# ze point: none -- the function is a no-op that always returns None, so it enforces nothing
def c_check_existing_tests(ctx):
    # Warning-only hook: prints to stderr but always exit 0. No effect on exit code.
    return None


# ze point: testing/temporary-files/use-project-tmp-for-scratch-files
def c_system_tmp_we(ctx):
    fp = ctx["fp"]
    if fp.startswith("/tmp/") or fp == "/tmp":
        return (
            2,
            "❌ Blocked: writing to /tmp is forbidden\n"
            "  Write project files under the project directory.\n"
            "  Tests: internal/<pkg>/<name>_test.go or test/<suite>/<name>.ci",
        )
    return None


# The Write half of check_scratch_path (.claude/hooks/pretool-bash.py). The
# check above sends every session to the project `tmp/`, whose ROOT is keyed per
# CHECKOUT: a fixed name there is one file for every session in this tree, and
# nothing removes it. Both surfaces call one module, so a path a redirect is
# refused cannot land through the Write tool instead.
# ze point: commands/write-ad-hoc-scratch-under-your-per-session-dir/write-ad-hoc-scratch-under-this-session-s-private-directory
def c_scratch_path_we(ctx):
    """ai/rules/commands.md: ad-hoc scratch belongs under this session's own dir."""
    fp = ctx["fp"]
    if not fp or not _scratch_path.is_ad_hoc_root_file(fp, PROJECT_DIR):
        return None
    return (
        2,
        f"{RED}{BOLD}❌ Refused: ad-hoc scratch at the tmp/ root: {fp}{RESET}\n"
        "  -- tmp/ is keyed per CHECKOUT, so that name is one file for every "
        "session in this tree, and nothing removes it.\n"
        "  -- Write it under this session's own directory instead:\n"
        "     dir=$(scripts/dev/session-scratch.sh)\n"
        "  -- A subdirectory passes, and so do the root names that are shared by "
        "design: ze-verify*, commit-*, delete-*, mutation*, test-timings*\n"
        "  -- ai/rules/commands.md, 'Write Ad-Hoc Scratch Under Your Per-Session Dir'",
    )


# Intentionally BROADER than the original shell-hook (which only blocked outright
# deletion on Edit). c_test_weakening also catches the quiet ways a failing test
# gets neutered instead of the code being fixed: adding t.Skip, dropping *some*
# assertions, downgrading require->assert, commenting assertions out, build-tag
# 'ignore', and the same via Write/MultiEdit overwrite. Rule: ai/rules/testing.md
#
# The carriers that comment with `#` rather than `//`. `.py` earns its place through
# `_carries_rfc_tag`, not through `is_test`: a tagged interop `check.py` reaches this
# check when `_rfc_tagged_change_err` declines. `.et` is here for the same reason.
# Neither is the broad parity `_behavior_bytes` has, which runs on every carrier it
# names.
_HASH_COMMENT_CARRIERS = (".ci", ".et", ".py")


_ASSERT_PAT = r"(?:t\.(?:Error|Errorf|Fatal|Fatalf|Fail|FailNow)|assert\.|require\.)"
_FATAL_PAT = r"(?:t\.(?:Fatal|Fatalf|FailNow)|require\.)"
_SKIP_PAT = r"\b[A-Za-z_]\w*\.Skip(?:Now|f)?[ \t]*\("
_IGNORE_TAG = r"//(?:go:build ignore\b|[ \t]*\+build ignore\b)"

# What a `.ci` / `.et` run can actually FAIL on, or stop exercising. The rest of
# those files is `option=`/`stdin=`/`tmpfs=` setup, embedded ze config, and observer
# plumbing, none of which decides a verdict.
#
# This replaced a count of non-comment LINES. That count made the guard fire on
# every mechanical improvement -- three blind `time.sleep` calls collapsed into one
# `wait_until` barrier removes two lines and no coverage -- while a hunk that
# swapped two real expectations for two tautologies kept its line count and passed
# untouched. It was refusing the fix and admitting the damage. Of the 755 `test-relax:`
# tokens in the working tree on 2026-08-10, 542 sat on a `.ci`/`.et` carrier this arm
# judges, and 362 of those excused a timing refactor it would now allow. That is what a
# guard which cries wolf buys: a corpus of justifications nobody reads.
#
# EVERY arm is anchored at statement position, and the text is comment-stripped
# before it is counted. Both are load-bearing. A bare `\bassert\b` matched prose and
# string literals, so deleting an `expect=` and adding the comment "we no longer
# assert the first line" balanced the count and passed. Measured at HEAD on
# 2026-08-10, a bare `\bassert\b` matched 49 comment lines across 47 tracked `.ci`
# files, every one of them able to pay for a deletion. `cmd=` is counted because the old
# line counter counted it: a deleted `cmd=` stops a command running, which is
# coverage removed even when the surviving expectations still match.
#
# `runtime_fail` is listed BEFORE `fail` so it is consumed whole and counted once.
# `\b` does not hold between `_` and `f`, so a bare `fail(` can never match inside
# it, and `api.fail(` still does.
#
# KNOWN BOUNDS, both deliberate. This counts; it does not interpret.
#
#  - Wrapping a live assertion in `if False:` keeps the count and kills the assertion.
#    Catching that needs a Python parse of an embedded script fragment, which an
#    edit-time hook does not have: the hunk is not a parseable program.
#  - Shrinking the embedded FIXTURE is invisible here. Deleting a `neighbor` stanza
#    from a `stdin=ze-bgp` block changes the scenario while every `cmd=` and `expect=`
#    survives. The line counter this replaced did catch it, by accident, along with
#    every reformat and every simplification. Restoring a fixture-line count would
#    restore the false-positive engine that produced 755 unread tokens, so the trade
#    is taken knowingly and the class is named here rather than left for someone to
#    discover.
#
# Both are owned by `scripts/dev/audit-test-relaxation.py` over the diff, and by the
# human reading it. Neither is owned by silence.
_CI_COVERAGE = re.compile(
    r"^[ \t]*(?:expect|reject|cmd)="
    r"|^[ \t]*assert\b"
    r"|^[ \t]*(?:\w+\.)?(?:runtime_fail|fail)[ \t]*\("
    r"|\b(?:wait_until|dispatch_until)[ \t]*\(",
    re.MULTILINE,
)
# `reject=` counted a SECOND time, on its own. Rewriting `reject=out:text=error` as
# `expect=out:text=error` inverts the assertion -- the run now demands the error it
# used to refuse -- while the combined count is unchanged. Only a separate tally of
# the negative expectations sees it.
_CI_REJECT = re.compile(r"^[ \t]*reject=", re.MULTILINE)
# An expectation whose needle is empty is not checked: `validateFileContent`
# (internal/test/runner/runner_validate.go) guards on `check.Contains != ""`, so
# emptying the needle silently drops the content assertion. The combined counter
# cannot see it, because the line is still there.
#
# It does NOT always leave nothing behind: `expect=file:path=X:contains=` still
# asserts X is readable, via `validateOnePathCheck`. So this reports a needle that
# STOPPED being checked, which is a weakening either way, and it does not claim the
# whole line went inert. Before this check existed, 14 lines in the tracked corpus
# ended in a bare `key=`; the shipped pattern matches none of them, because each
# turned out to be a needle that merely CONTAINS an `=` (`contains=ExecStart=`).
_CI_EMPTY_NEEDLE = re.compile(
    r"^[ \t]*(?:expect|reject)=[ \t]*$"
    r"|^[ \t]*(?:expect|reject)=.*:[A-Za-z_][\w-]*=[ \t]*$",
    re.MULTILINE,
)


def _strip_comments(text, hashed):
    """`text` with its comments removed, quote-aware. Never touches the file on disk.

    Two simpler versions of this were wrong in opposite directions, and each one
    reintroduced the defect the other fixed.

    A to-end-of-line strip is not symmetric: `//` inside a Go STRING literal is not a
    comment, so a fixture whose value is `"//go:build linux\\n" + "t.Fatal(x)"` lost
    its `t.Fatal` from the OLD side alone, and deleting that whole entry read as no
    change. A whole-line-only strip leaves a TRAILING comment in place, and a trailing
    comment can then PAY for deleted coverage: `expect=out:text=two  # we now
    wait_until(x)` restores the count that deleting the other expectation lowered.
    257 lines across 88 tracked `.ci` and `.et` files carry a trailing comment, so that
    shape is idiomatic rather than hypothetical.

    So: scan, and cut at the first comment marker that is not inside a quote. Quote
    state resets per line, which mis-reads a Go raw string spanning lines. That is
    the residue, and it is bounded: a multi-line raw string in a test fixture is rare,
    and the error is symmetric across old and new when it happens.
    """
    marker = "#" if hashed else "//"
    return "\n".join(_strip_line(line, marker) for line in text.split("\n"))


def _strip_line(line, marker):
    """One line, comment removed. Quote-aware, unless the quotes do not balance.

    A `.ci` value is not a quoted language, so a lone apostrophe in English prose --
    or a regex like `pattern="maximum": ?"?100`, of which 11 exist at HEAD -- left the
    scanner inside a quote for the rest of the line and stopped it cutting anything.
    That reopened the exact hole the quote-awareness was added to close: the trailing
    comment paid for the deleted expectation again, just on quirkier lines.

    So an unterminated quote means the line is not quoted text, and the second pass
    cuts at the first marker. A line whose quotes DO balance keeps its protection,
    which is what stops `"//go:build linux"` reading as a comment.

    RESIDUE, and it is the deliberate half of the trade: on a line whose quotes do
    NOT balance, the fallback can cut inside a real string. `u := "http://a";
    require.NoError(t, err) // don't` strips at the `//` in the URL, so deleting that
    line reads as no change. Measured at HEAD, three tracked lines take the fallback
    while losing a counted construct, and all three are whole-line comments, where the
    strip is correct. The alternative -- keeping quote state on an unbalanced line --
    let a trailing comment PAY for a deleted expectation, which is the failure this
    whole function exists to prevent.
    """
    quote = ""
    cut = -1
    i = 0
    while i < len(line):
        ch = line[i]
        if quote:
            if ch == "\\":
                i += 2
                continue
            if ch == quote:
                quote = ""
        elif ch in "\"'`":
            quote = ch
        elif cut < 0 and line.startswith(marker, i):
            cut = i
        i += 1
    if quote:  # unbalanced: not quoted text, so honour the first marker
        first = line.find(marker)
        return line if first < 0 else line[:first]
    return line if cut < 0 else line[:cut]


# An assertion that cannot fail. Introducing one is the in-place gutting that no
# count-based heuristic sees: `assert 'established' in resp` -> `assert True` keeps
# every line and every assertion count identical. Deliberately narrow. A real test
# does not assert a constant, so this fires almost only on the thing it names.
_TAUTOLOGY = re.compile(
    r"\bassert[ \t]+(?:True|1)[ \t]*(?:$|[,#])"
    r"|\bassert[ \t]+not[ \t]+(?:False|0)[ \t]*(?:$|[,#])"
    # `assert 1 == 1`, `assert x == x`: the same expression on both sides.
    r"|\bassert[ \t]+(\S+)[ \t]*==[ \t]*\1[ \t]*(?:$|[,#])"
    r"|\b(?:require|assert)\.(?:True|Truef)\([^,]+,[ \t]*true[ \t]*[,)]"
    r"|\b(?:require|assert)\.(?:False|Falsef)\([^,]+,[ \t]*false[ \t]*[,)]",
    re.MULTILINE,
)


def _test_weakening_errs(old, new, fp):
    """(blocking, advisory) -- the ways `new` weakens the test in `old`.

    The split is the whole design, and it is answering a measured failure. The
    arms fall into two kinds:

      blocking  something BAD APPEARED. A test function is gone, a skip was
                added, a needle now matches anything, an assertion cannot fail.
                Each is one-directional: there is no innocent edit that produces
                it, so refusing is right and the noise is near zero.

      advisory  a COUNT WENT DOWN. Fewer `t.Run` cases, fewer table rows, fewer
                assertions, fewer `.ci` expectations.

    A count cannot tell "I deleted a check" from "I replaced three checks with one
    better check", and the second is what ordinary refactoring IS. So the counting
    arms fired on good work, constantly, and every firing cost the author a
    `test-relax:` token. That is where 780 of them came from: reading all 402 that
    no one had triaged, 146 say in their own words that the coverage still exists
    and only 19 record a real loss. Three of every four tokens in the corpus
    excuse an improvement.

    So a count drop now REPORTS and lets the work through; only the blocking arms
    refuse. Nothing that catches real damage was given up, because a count falling
    was never evidence of damage on its own.

    The counting arms read COMMENT-STRIPPED text. `_ASSERT_PAT` matches `require.`
    and `assert.` wherever they appear, prose included, so a comment that MENTIONS
    an assertion was counted as one -- and deleting that comment then read as
    deleting an assertion. The `test-relax:` justifications this gate produces are
    full of such prose, which made the gate refuse its own cleanup: removing a token
    whose reason says "the require.Equal on port" needed a fresh token to remove.

    Two arms keep the raw text on purpose. `commenting out assertions` exists to
    find assertions inside comments, and `emptying an expectation's needle` reads a
    `.ci` needle that may legitimately begin with `#`.
    """
    errs = []
    soft = []
    hashed = fp.endswith(_HASH_COMMENT_CARRIERS)
    old_s = _strip_comments(old, hashed)
    new_s = _strip_comments(new, hashed)
    if new.strip() == "" and old.strip():
        errs.append("replacing test content with empty string")
    if grep_any(old, r"^func (Test|Fuzz|Benchmark)") and not grep_any(
        new, r"^func (Test|Fuzz|Benchmark)"
    ):
        errs.append("deleting Test/Fuzz/Benchmark function")
    if old_s.count("t.Run(") > new_s.count("t.Run("):
        soft.append(
            f"removing t.Run cases ({old_s.count('t.Run(')} -> {new_s.count('t.Run(')})"
        )
    old_tbl = len(re.findall(r"\{[ \t]*(?:name|Name)[ \t]*:", old_s))
    new_tbl = len(re.findall(r"\{[ \t]*(?:name|Name)[ \t]*:", new_s))
    if old_tbl > new_tbl:
        soft.append(f"removing table-driven cases ({old_tbl} -> {new_tbl})")
    old_as = len(re.findall(_ASSERT_PAT, old_s))
    new_as = len(re.findall(_ASSERT_PAT, new_s))
    if old_as > new_as and new.strip():
        soft.append(f"removing assertions ({old_as} -> {new_as})")
    old_fatal = len(re.findall(_FATAL_PAT, old_s))
    new_fatal = len(re.findall(_FATAL_PAT, new_s))
    if old_fatal > new_fatal and new_as >= old_as:
        soft.append(
            f"downgrading fatal assertions to non-fatal ({old_fatal} -> {new_fatal} require/Fatal)"
        )
    old_skip = len(re.findall(_SKIP_PAT, old_s))
    new_skip = len(re.findall(_SKIP_PAT, new_s))
    if new_skip > old_skip:
        errs.append(f"adding t.Skip ({old_skip} -> {new_skip}); the test stops running")
    if re.search(_IGNORE_TAG, new) and not re.search(_IGNORE_TAG, old):
        errs.append("adding 'ignore' build tag; file dropped from the build")
    old_cmt = len(re.findall(r"//[^\n]*" + _ASSERT_PAT, old))
    new_cmt = len(re.findall(r"//[^\n]*" + _ASSERT_PAT, new))
    if new_cmt > old_cmt:
        errs.append(f"commenting out assertions ({old_cmt} -> {new_cmt})")
    if fp.endswith((".ci", ".et")):
        old_ci = len(_CI_COVERAGE.findall(old_s))
        new_ci = len(_CI_COVERAGE.findall(new_s))
        if old_ci > new_ci:
            soft.append(
                f"removing expectations ({old_ci} -> {new_ci} expect=/reject=/cmd=/assert/fail)"
            )
        old_rej = len(_CI_REJECT.findall(old_s))
        new_rej = len(_CI_REJECT.findall(new_s))
        if old_rej > new_rej:
            soft.append(
                f"removing negative expectations ({old_rej} -> {new_rej} reject=)"
            )
        # On the RAW text, never the comment-stripped one. A needle may legitimately
        # BEGIN with `#` -- `expect=stdout:contains=# tcp.bind`, and 9 tracked lines
        # are that form -- and stripping the
        # comment turns it into `...contains=`, which reads as empty. That misfired
        # in both directions at once: adding such a line was refused as an emptied
        # needle, and genuinely emptying one was allowed because both sides looked
        # equally empty.
        old_empty = len(_CI_EMPTY_NEEDLE.findall(old))
        new_empty = len(_CI_EMPTY_NEEDLE.findall(new))
        if new_empty > old_empty:
            errs.append(
                f"emptying an expectation's needle ({old_empty} -> {new_empty}); it now matches anything"
            )
    old_taut = len(_TAUTOLOGY.findall(old_s))
    new_taut = len(_TAUTOLOGY.findall(new_s))
    if new_taut > old_taut:
        errs.append(
            f"introducing an assertion that cannot fail ({old_taut} -> {new_taut})"
        )
    return errs, soft


_RFC_TAG = re.compile(r"RFC requirement:\s*[A-Za-z0-9][A-Za-z0-9.\-]*-\d+")
_RFC_APPROVED = re.compile(r"rfc-test-change-approved:[ \t]*\S")
_GO_LINE_COMMENT = re.compile(r"//.*$", re.MULTILINE)
_CI_LINE_COMMENT = re.compile(r"^[ \t]*#.*$", re.MULTILINE)
_WS = re.compile(r"\s+")
# One Go import line: an optional alias (`bgpctx`, `_`, `.`) then a quoted path. The
# `import (` and `)` brackets count too, so growing a one-line import into a block passes.
_GO_IMPORT_LINE = re.compile(
    r'^(?:import\s*\(|\)|import\s+)?(?:[A-Za-z_.][\w.]*\s+)?"[^"]+"$'
)
_GO_IMPORT_DELIM = re.compile(r"^(?:import\s*\(|\))$")


def _import_only_go_edit(old, new, fp):
    """True when both sides of a .go edit are nothing but import lines.

    An import cannot weaken an assertion, so an edit made ONLY of them is never the
    thing this guard exists to catch. Without this, adding a test to a file that already
    holds a tagged one always costs an operator approval: new tests need new imports, the
    import block sits outside every function so `_enclosing_tagged_scope` widens to the
    whole file, and `_behavior_bytes` then sees real code change (HOOK-FRICTION.md,
    2026-08-01). That is the path `ai/rules/testing.md` tells contributors to
    take ("ADD a new test case or function"), so the guard was charging the honest route.

    EVERY non-blank line on BOTH sides must match, which is what keeps it from becoming a
    hole: an assertion smuggled into the same edit as an import leaves a line that is not
    import-shaped, and the edit blocks as before. Comments are stripped first, so a doc
    comment rewritten beside the imports does not defeat it.

    One side may be empty. Deleting an import cannot remove an assertion: if the import is
    still used the package stops compiling, which is loud, and if it is unused the deletion
    changes no behaviour. Deleting a TEST is unaffected, because a function body is not
    import-shaped. Both sides empty returns False, since `seen` never gets set.
    """
    if not fp.endswith(".go"):
        return False
    seen = False
    for side in (old, new):
        for line in _GO_LINE_COMMENT.sub("", side).splitlines():
            line = line.strip()
            if not line:
                continue
            seen = True
            if not (_GO_IMPORT_LINE.match(line) or _GO_IMPORT_DELIM.match(line)):
                return False
    return seen


def _behavior_bytes(text, fp):
    """The part of a test that decides what it asserts: code minus comments and layout.

    Used to tell a reformat, a comment edit, or a re-tag from a change to what the test
    actually checks. Deliberately crude -- it only has to answer "did the assertions move".

    The comment syntax is chosen by shape, and `#` covers three of the four carriers: a `.ci`,
    a `.et` and an interop `check.py` all comment with `#`, which is also where they carry
    their RFC tags. Using the Go `//` stripper on a `check.py` would leave every `#` comment in
    the compared bytes, so re-wording a Python comment would read as a behaviour change and
    block -- the over-blocking that gets a guard switched off.
    """
    hash_comments = fp.endswith((".ci", ".et", ".py"))
    stripped = (
        _CI_LINE_COMMENT.sub("", text)
        if hash_comments
        else _GO_LINE_COMMENT.sub("", text)
    )
    return _WS.sub("", stripped)


def _enclosing_tagged_scope(fp, hunks):
    """The text whose RFC tags govern an edit, widened from the hunk to its context.

    The span logic itself lives in scripts/dev/rfc_tagged_scope.py, which
    scripts/dev/rfc_requirements.py's audit fingerprint reads too. Exactly ONE definition of
    "the tagged unit" exists: a second copy that drifted would let the gate re-seal a verdict
    against a hash this guard does not compute, and the two would then disagree about which
    text an obligation covers (spec-rfcgate-3-audit-teeth.md AC-22).

    Only the FILE READ and the fail-closed degradation stay here. `_RFC_TAG` also stays here
    and is passed in: it is deliberately broader than the gate's scanner (it matches the
    phrase in ordinary prose too, which is what makes this widen to file scope for one file
    in the tree), and moving it into the shared leaf would silently change the gate.
    """
    if not hunks:
        return None
    try:
        with open(fp, encoding="utf-8", errors="replace") as fh:
            content = fh.read()
    except OSError:
        return None
    if _rfc_scope is None:
        # The leaf is committed beside this hook, so an ImportError means a broken checkout,
        # not a supported configuration. Degrade toward MORE checking: hand back the whole
        # file so a tagged test still blocks. Returning None instead would silently restore
        # the hunk-only scope this guard exists to widen -- a fail-OPEN on the one path where
        # a missed block ships an unproven compliance claim.
        return content if _RFC_TAG.search(content) else None
    return _rfc_scope.tag_scope(fp, content, hunks, _RFC_TAG)


# ze point: testing/rfc-tagged-tests-blocking/never-edit-an-rfc-tagged-test-to-match-the-code
def _rfc_tagged_change_err(old, new, fp, tag_scope=None):
    """Describe an unapproved behavior change to a test carrying an `RFC requirement:` tag.

    An RFC-tagged test is the only thing standing between a regression and a shipped
    protocol violation: it encodes an obligation Ze publicly claims to meet
    (docs/features/rfc-status.md), and `make ze-rfc-check` counts it as the proof. Editing
    it to match the code inverts the whole point -- ai/rules/testing.md already says fix the
    code, not the test; here that is enforced rather than asked.

    Scoped to BEHAVIOR-bearing edits: reformatting and comment/tag edits pass, because a
    hook that blocks gofmt gets disabled, and then it protects nothing.

    A rename DOES block, since an identifier is code and this check cannot tell a rename
    from a rewrite without parsing Go. That is the deliberate side the error falls on: a
    spurious block costs one question, a missed one ships an unproven compliance claim.
    Renaming a test that carries a standards obligation is worth a beat of thought anyway.

    NOT satisfied by a row in `test/weakened.md`: that row is self-service, and an agent
    writing its own justification is not user approval. It is exactly the loophole this
    closes.
    """
    scope = old if tag_scope is None else tag_scope
    if not _RFC_TAG.search(scope):
        return None
    if _RFC_APPROVED.search(new):
        return None
    # Removing the TAG is checked before anything else, because a tag is a comment and the
    # behavior comparison below would wave it through as "comments only" -- after which the
    # test is unguarded and a `test/weakened.md` row buys any later weakening. Deleting the
    # proof marker is the cheapest way to retire a compliance claim, so it is not a
    # comment edit; it is the edit this guard exists for.
    dropped = sorted(set(_RFC_TAG.findall(old)) - set(_RFC_TAG.findall(new)))
    if dropped:
        return dropped
    # Import-only edits pass, and this sits AFTER the tag-removal check on purpose: a tag
    # lives in a comment, comments are stripped before the import test, so a hunk that
    # deleted a tag while touching imports would otherwise read as import-only.
    if _import_only_go_edit(old, new, fp):
        return None
    # Behavior is judged on the HUNK, never on the widened scope: the scope answers "is
    # this test load-bearing for a compliance claim", the hunk answers "did the assertions
    # move". Judging the scope would make every reformat inside a tagged function a block.
    if _behavior_bytes(old, fp) == _behavior_bytes(new, fp):
        return None  # comments/whitespace only
    tags = sorted(set(_RFC_TAG.findall(scope)))
    return tags


def _carries_rfc_tag(fp):
    """True when `fp` is a shape the RFC tag scanner reads AND actually holds a tag.

    C-4, spec-rfcgate-3-audit-teeth.md: `is_test` below covers `_test.go` and a `/test/` `.ci`
    and nothing else, so when plan/spec-rfcgate-2-evidence.md admitted interop `check.py`
    evidence, two files started carrying RFC obligations that this guard could not see at all.
    The gate counted their tags as the proof behind a public compliance claim while the
    edit-time guard let any edit through.

    Deliberately narrower than "any file": the carrier list comes from the shared leaf (which
    a gate test holds against `CARRIERS`), and the file must really contain a tag. A path
    predicate alone would drag every `.go` and every `.et` in the repository into this check
    for nothing.
    """
    if _rfc_scope is None or not _rfc_scope.is_tag_carrier(fp):
        return False
    try:
        with open(fp, encoding="utf-8", errors="replace") as fh:
            return bool(_RFC_TAG.search(fh.read()))
    except OSError:
        return False


DRAFT_SEGMENT = "test/draft/"


def _is_draft(fp):
    """True when `fp` sits in the draft incubator.

    `test/draft/` is gitignored and skipped by every repo-wide gate, so what
    lives there is not a test yet: it claims no evidence, proves no RFC
    obligation, and appears in no coverage ledger. Neither the weakening
    heuristic nor the RFC-tag guard has anything to protect there, and the
    draft workflow ends in exactly two moves, promote it or delete it
    (ai/rules/testing.md). Guarding a draft makes the incubator the one
    directory an agent can fill and never empty.
    """
    norm = fp.replace(os.sep, "/")
    return norm.startswith(DRAFT_SEGMENT) or "/" + DRAFT_SEGMENT in norm


# --------------------------------------------------------------------------- #
# The weakening hatch: test/weakened.md
#
# The justification for a weakened test lives in ONE file the commit carries, not
# in a comment beside the test. A comment stayed in the test file forever and
# explained a diff its later readers could no longer see: 601 of them piled up
# across 413 files, nobody could read them, so writing one cost nothing
# (plan/spec-weakened-per-commit.md).
#
# The same module answers at commit time (scripts/dev/commit_helper.py), so a test
# name and a row are resolved once rather than twice.
# --------------------------------------------------------------------------- #

_WEAKENED_MODULE_PATH = os.path.join(
    os.path.dirname(os.path.abspath(__file__)),
    "..",
    "..",
    "scripts",
    "dev",
    "check_weakened_tests.py",
)

_UNLOADED = object()
_weakened_mod = _UNLOADED


def _weakened_module():
    """`scripts/dev/check_weakened_tests.py`, loaded on first use and kept, or None.

    Lazy, and only a refusal asks for it: this hook runs on every Edit, and that
    module executes a second one when it loads.

    A hook that raises on import blocks every edit in the repository, so this
    cannot be a bare import. The caller's None branch fails CLOSED: without the
    module the hatch cannot be read, and an unreadable hatch is not an open one.
    """
    global _weakened_mod
    if _weakened_mod is _UNLOADED:
        try:
            spec = importlib.util.spec_from_file_location(
                "ze_check_weakened_tests", _WEAKENED_MODULE_PATH
            )
            mod = importlib.util.module_from_spec(spec)
            spec.loader.exec_module(mod)
            _weakened_mod = mod
        except Exception:
            _weakened_mod = None
    return _weakened_mod


def _weakened_rows(mod):
    """(rows, problems) -- what `test/weakened.md` accepts, and what stops it being read.

    Read at CALL time under PROJECT_DIR, which CLAUDE_PROJECT_DIR can point at a
    fixture tree.

    `errors="replace"` is what keeps this fail-CLOSED, and it is the same reader
    the delegate uses (`_read_contract`, `scripts/dev/check_weakened_tests.py`).
    The dispatcher catches an exception from a check and fails OPEN, so a
    `UnicodeDecodeError` raised here would let the weakening edit through with no
    row at all. Only OSError is a state this hook reports; a byte it cannot decode
    is replaced and the row it sat in fails to match, which refuses.
    """
    try:
        with open(
            os.path.join(PROJECT_DIR, mod.WEAKENED_PATH),
            encoding="utf-8",
            errors="replace",
        ) as fh:
            text = fh.read()
    except OSError:
        return [], [f"{mod.WEAKENED_PATH} does not exist yet; write it, header first"]
    return mod.parse_weakened_file(text)


def _edited_file_pair(fp, tool, ti, old, new):
    """(old, new) as WHOLE FILES, so a weakening carries the name of its test.

    A weakening is named after the top-level func that holds it, and an Edit hunk
    is usually the assertion alone: it carries no `func` line, so the hunk on its
    own names the FILE. The commit gate compares whole files and asks for the func
    name, so this hook reconstructs what the edit will leave on disk. Without that
    the two gates would ask for two different rows for one weakening.

    The hunk pair comes back when the file cannot be read, or when a hunk is not
    in it: each means the reconstruction would be a guess, and a hunk still names
    the test whenever it carries the declaration.
    """
    if tool == "Write":
        return old, new  # `old` is already the file on disk
    try:
        with open(fp, encoding="utf-8", errors="replace") as fh:
            text = fh.read()
    except OSError:
        return old, new
    edits = (ti.get("edits") or []) if tool == "MultiEdit" else [ti]
    after = text
    for e in edits:
        hunk = e.get("old_string", "")
        if not hunk or hunk not in after:
            return old, new
        after = after.replace(
            hunk, e.get("new_string", ""), -1 if e.get("replace_all") else 1
        )
    return text, after


def _weakened_names(mod, fp, file_old, file_new):
    """The tests this edit weakens, named the way `test/weakened.md` names them.

    `weakened_units` is the shared namer, and it is given the SAME detector the
    commit gate gives it, count arms included. The count arms do not refuse here
    (see the notice in `c_test_weakening`), and they are still named, because the
    commit gate asks for a row for every kind it finds.
    """
    names = []
    for name, _ in mod.weakened_units(fp, file_old, file_new, _test_weakening_errs):
        if name not in names:
            names.append(name)
    return names or [os.path.splitext(os.path.basename(fp))[0]]


def _weakened_hatch(fp, tool, ti, old, new):
    """(names with no row, why the file could not be read, the file's path).

    An empty pair of lists opens the hatch: every test this edit weakens is named
    in `test/weakened.md`, with a reason.
    """
    mod = _weakened_module()
    if mod is None:
        # No name to ask a row for, because the module that resolves one is the
        # module that did not load. The refusal stands: an unreadable hatch is not
        # an open one, and the checkout is what needs fixing.
        return (
            [],
            [
                "scripts/dev/check_weakened_tests.py did not import, so the hatch "
                "could not be read"
            ],
            "test/weakened.md",
        )
    rows, problems = _weakened_rows(mod)
    file_old, file_new = _edited_file_pair(fp, tool, ti, old, new)
    package = os.path.basename(os.path.dirname(fp))
    missing = []
    for name in _weakened_names(mod, fp, file_old, file_new):
        # `row_matches` is the ONE definition of "this row names that test", the
        # `package.TestName` qualifier included. A copy here would let the hook
        # accept a row the commit gate refuses. It is public for that reason:
        # the hook is a real caller, not a module poking at its neighbour.
        weak = mod.Weakened(fp, package, name, [])
        if not any(mod.row_matches(row.name, weak) for row in rows):
            missing.append(name)
    return missing, problems, mod.WEAKENED_PATH


# ze point: testing/directives/write-the-test-first-and-never-weaken-it
# ze point: testing/fix-code-not-tests/fix-the-code-when-a-test-fails-not-the-test
def c_test_weakening(ctx):
    fp = ctx["fp"]
    if _is_draft(fp):
        return None
    # `.et` sits beside `.ci` here because it is the same kind of artifact: an editor
    # functional test, judged by `expect=` lines, under `test/`. It was absent until
    # 2026-08-10, so `c_test_weakening` returned None for all 164 of them and the whole
    # guard was inert over the editor suite. None carries an `RFC requirement:` tag, so
    # `_carries_rfc_tag` was not letting them in by the side door either.
    is_test = bool(
        re.search(r"_test\.go$", fp) or (fp.endswith((".ci", ".et")) and "/test/" in fp)
    )
    # A tagged carrier the `is_test` predicate misses still gets the RFC-tagged branch below,
    # but NOT the generic weakening heuristic: `_test_weakening_errs` counts Go/`.ci` shapes
    # and would mis-read a Python scenario, and widening two rules at once when only one has a
    # demonstrated hole is how a guard earns its reputation for over-blocking.
    if not is_test and not _carries_rfc_tag(fp):
        return None
    tool = ctx["tool"]
    hunks = []
    if tool == "Edit":
        old = ctx["ti"].get("old_string", "")
        new = ctx["ti"].get("new_string", "")
        hunks = [(old, bool(ctx["ti"].get("replace_all")))]
    elif tool == "MultiEdit":
        edits = ctx["ti"].get("edits") or []
        old = "\n".join(e.get("old_string", "") for e in edits)
        new = "\n".join(e.get("new_string", "") for e in edits)
        # Per hunk, not the join: the joined text appears nowhere in the file, so widening
        # on it would find no context and silently restore the old narrow behavior.
        hunks = [(e.get("old_string", ""), bool(e.get("replace_all"))) for e in edits]
    elif tool == "Write":
        # Only an overwrite of an existing test file can weaken it.
        if not isfile(fp):
            return None
        try:
            with open(fp, encoding="utf-8", errors="replace") as fh:
                old = fh.read()
        except OSError:
            return None
        new = ctx["ti"].get("content", "") or ""
    else:
        return None
    # RFC-tagged tests are checked FIRST, before the `test/weakened.md` hatch below, and
    # deliberately: a row there is self-service, and an agent writing its own justification
    # is not user approval. Letting it run first would leave the loophole open.
    tags = _rfc_tagged_change_err(
        old, new, fp, tag_scope=_enclosing_tagged_scope(fp, hunks)
    )
    if tags:
        named = "\n".join(f"  - {t}" for t in tags)
        return (
            2,
            f"{RED}{BOLD}BLOCKED: RFC-tagged test - ask the user before changing it{RESET}\n"
            f"  {os.path.basename(fp)} enforces RFC obligations:\n{named}\n"
            "  These are the proof behind a public compliance claim\n"
            "  (docs/features/rfc-status.md), counted by `make ze-rfc-check`.\n"
            "  Editing the test to match the code inverts that: the obligation stops being\n"
            "  proven while still being advertised.\n"
            "  Fix the CODE. If you believe the test is genuinely wrong, STOP and show the\n"
            "  user the RFC text next to the test -- do not edit first and explain after.\n"
            "  A row in test/weakened.md does NOT authorize this: it is your own\n"
            "  justification, not the user's approval.\n"
            "  Once the USER has approved, record what they approved on the changed test:\n"
            "    // rfc-test-change-approved: <date> <what the user approved and why>\n"
            "  PUT IT IN THE LINES YOU ARE WRITING. This check reads only the replacement\n"
            "  text of THIS edit, so the same marker at the top of the file does not\n"
            "  satisfy it and the edit is refused again.\n"
            "  (auditable: grep -rn 'rfc-test-change-approved:')\n"
            "  Reformatting and comment/tag edits are never blocked, and neither is a .go\n"
            "  edit made only of import lines. A rename is, because this check cannot tell\n"
            "  one from a rewrite -- approve it the same way.",
        )
    # Documented, auditable escape hatch. Forces a written reason instead of a
    # silent edit. It reads `test/weakened.md`, which the commit carries and which
    # the commit gate reads again, and it opens only on a row naming the test THIS
    # edit weakens. A row for another test buys nothing, or the hatch would be
    # per-file rather than per-weakening (see `_weakened_hatch`).
    errs, soft = _test_weakening_errs(old, new, fp)
    if not errs:
        if not soft:
            return None
        # A count fell and nothing one-directional happened. Say so and let the
        # edit through: code 0 leaves the dispatcher's verdict alone (it takes the
        # max), so this is a notice and not an obligation.
        #
        # It used to refuse here, and refusing is what built the corpus. Three of
        # every four `test-relax:` tokens excuse an edit that made a test BETTER,
        # because consolidating three assertions into one table, or three blind
        # sleeps into one barrier, lowers a count exactly as deleting a check does.
        # The token then cost a reviewed line and bought nothing a reader could use.
        notice = "\n".join(f"  - {s}" for s in soft)
        return (
            0,
            f"{YELLOW}notice: this edit lowers a test count{RESET}\n"
            f"  In {os.path.basename(fp)}:\n{notice}\n"
            "  Allowed, and this hook asks for no row. A count falling is what\n"
            "  consolidating cases or replacing a poll loop with a barrier looks\n"
            "  like, and it reads the same as deleting a check, so it cannot be the\n"
            "  refusal on its own.\n"
            "  Check yourself that the coverage moved rather than went. The commit\n"
            "  gate records every weakening kind, count drops included, so the commit\n"
            "  carrying this edit still needs a row in test/weakened.md naming this\n"
            "  test. Say there which of the two happened.",
        )
    missing, unreadable, weakened_path = _weakened_hatch(fp, tool, ctx["ti"], old, new)
    if not missing and not unreadable:
        return None
    # The soft findings ride along on a refusal. They are not the reason for it,
    # but an author reading "commenting out assertions" is better served knowing
    # the count moved too than having that fact withheld because it no longer
    # blocks on its own.
    detail = "\n".join(f"  - {e}" for e in errs + soft)
    blocked = "".join(f"\n  {p}" for p in unreadable)
    rows_to_write = "\n".join(
        f"    | {name} | <what left the suite, and why this is correct without it> |"
        for name in missing
    )
    advice = (
        (
            f"\n  If the coverage is genuinely gone, accept it in {weakened_path}:\n"
            f"{rows_to_write}\n"
            "  Two columns, under the header `| Test | Reason |`: the test this edit\n"
            "  weakens, and the reason a reviewer can act on.\n"
            "  WRITE THE ROW FIRST, then make this edit again. This hook reads the\n"
            "  file from disk, so a row you have not written yet buys nothing, and\n"
            "  neither does a row naming another test.\n"
            "  The file is replaced per commit. Delete the rows of the last commit,\n"
            "  write the rows of this one, and commit it with the change."
        )
        if missing
        else ""
    )
    return (
        2,
        f"{YELLOW}{BOLD}❓ Test weakening blocked - fix the code, not the test{RESET}\n"
        f"  Detected in {os.path.basename(fp)}:\n{detail}\n"
        "  A red test means the CODE is wrong by default. Diagnose the failure and\n"
        "  fix the source. Only weaken a test for a removed feature or for coverage\n"
        f"  that moved somewhere else.{blocked}{advice}",
    )


# --------------------------------------------------------------------------- #


def _glob_match(text, pattern):
    """fnmatch-style match used by bash `case`/`[[ == glob ]]`."""
    import fnmatch

    return fnmatch.fnmatch(text, pattern)


def _file_contains(path, needle):
    try:
        with open(path, encoding="utf-8", errors="replace") as fh:
            return needle in fh.read()
    except Exception:
        return False


# ze point: plugins/registration-based-dispatch/dispatch-subcommands-by-registration-not-switch
def c_switch_dispatch(ctx):
    """ai/rules/plugins.md: no switch-based subcommand dispatch."""
    fp = ctx["fp"]
    if not _go_we(ctx) or re.search(r"_test\.go$", fp) or not ctx["content"]:
        return None
    hits = filter_out(
        grep_lines(ctx["content"], r"switch\s+args\[0\]"),
        r"//.*nolint",
    )
    if hits:
        detail = "\n".join(f"  L{n}: {l.strip()}" for n, l in hits[:6])
        fix = (
            "\n  Use subdispatch.New() + Register() instead of switch on args.\n"
            "  Rule: ai/rules/plugins.md"
        )
        return (
            2,
            f"{RED}{BOLD}❌ BLOCKED: switch-based command dispatch in {fp}{RESET}\n{detail}{fix}",
        )
    return None


# ze point: testing/ci-sleep-justification/justify-every-sleep-in-a-ci-test
def c_ci_sleep_justification(ctx):
    # Edit-time nudge; the authoritative BLOCK is check_ci_sleep_justification in
    # scripts/dev/verify_wiring_docs.py (run by the inventory make gate). Every
    # time.sleep( in a .ci functional test must carry a comment on the line above
    # it (or trailing it) saying why it is there / why it was not converted to a
    # deterministic wait. A blind sleep hides real races and hides why it was left.
    # See ai/rules/testing.md. Non-blocking: an Edit fragment may not
    # show the comment that already sits above the sleep in the file, so warn only.
    fp = ctx["fp"]
    if not fp.endswith(".ci") or "/test/" not in fp:
        return None
    tool = ctx["tool"]
    if tool == "Write":
        new = ctx["ti"].get("content") or ""
    elif tool == "MultiEdit":
        new = "\n".join(
            (e.get("new_string") or "") for e in (ctx["ti"].get("edits") or [])
        )
    elif tool == "Edit":
        new = ctx["ti"].get("new_string") or ""
    else:
        return None
    lines = new.split("\n")
    bad = []
    for i, line in enumerate(lines):
        if "time.sleep(" not in line:
            continue
        if line.strip().startswith("#"):
            continue  # the sleep is itself commented out
        after = line.split("time.sleep(", 1)[1]
        if "#" in after:
            continue  # trailing comment on the same line
        prev = lines[i - 1].strip() if i > 0 else ""
        if prev.startswith("#"):
            continue  # comment on the line directly above
        bad.append((i + 1, line.strip()))
    if not bad:
        return None
    detail = "\n".join(f"  +{n}: {l}" for n, l in bad[:4])
    fix = (
        "\n  Add a `#` comment on the line directly above each sleep explaining why it\n"
        "  is not a deterministic wait: poll interval, deliberate timer, needs-linux\n"
        "  effect, or no queryable readiness signal. Enforced at commit by the\n"
        "  inventory gate (scripts/dev/verify_wiring_docs.py);\n"
        "  see ai/rules/testing.md."
    )
    return (
        1,
        f"{YELLOW}{BOLD}WARN: unjustified time.sleep( in {fp}{RESET}\n{detail}{fix}",
    )


# A path plus a line number is a citation that is wrong as soon as anybody
# edits the file, and the repo carried 15039 of them before this check existed.
# The path and the symbol survive an edit, so those are what a citation names
# (ai/rules/writing.md). Two things keep their numbers: a fenced code
# block, where the number is quoted output rather than a citation, and
# rfc/full/*.txt, because a published RFC never changes.
_LINE_REF = re.compile(
    r"(?<![A-Za-z0-9_])([A-Za-z0-9_./-]*\.(?:go|py|sh|md|mk|yang|ci|et|json|txt"
    r"|c|h|cc|cpp|rs|java|ts|js))"
    r":\d+(?:-\d+)?"
)

# A forge permalink's line anchor, `#L1903` or `#L1712-1716`. Same defect as a
# bare line number, and it survived the first sweep: this corpus cites FRR,
# BIRD, GoBGP and OpenBSD that way. The URL resolves to the file without it, and
# the citation should name the SYMBOL.
_LINE_ANCHOR = re.compile(r"https?://\S*?(#L\d+(?:[-L]\d+)?)")
_LINE_REF_PROSE = (".md",)
# The harness passes an absolute path, so each root is matched with its leading
# separator. The bare form is accepted too, so a relative path from a test or a
# script reaches the same verdict.
_LINE_REF_ROOTS = ("ai/", "docs/", "plan/", ".claude/")

# A line reference is legitimate exactly when a GENERATOR maintains it, because
# then it is refreshed with the tree instead of rotting in place. A hand-typed
# one is wrong the moment the file moves, and `rfc/requirements/rfc7606.md` is
# the working example: its `file.go:line` entries are derived from
# `RFC requirement:` tags on every `make ze-rfc-index`. One such file exists per
# RFC, and `ai/RFC-REQUIREMENTS.md` is the index over them.
#
# Both marker forms in this repo are a GENERATED declaration near the top, one
# an HTML comment and one a prose line. The file on disk is consulted as well as
# the new text, so an Edit that does not include the header still gets the right
# verdict.
_GENERATED_MARK = re.compile(r"GENERATED|generated by", re.I)


def _declares_generated(text):
    head = "\n".join((text or "").splitlines()[:10])
    return bool(_GENERATED_MARK.search(head)) and "do not edit" in head.lower()


def _in_prose_root(fp, new=""):
    if _declares_generated(new):
        return False
    try:
        with open(fp, encoding="utf-8", errors="replace") as fh:
            if _declares_generated(fh.read(4000)):
                return False
    except OSError:
        pass
    return any(f"/{r}" in fp or fp.startswith(r) for r in _LINE_REF_ROOTS)


# ze point: writing/detail-budget/write-only-what-changes-the-next-action
# ze point: evidence/no-fabrication/cite-a-line-number-only-when-a-generator-maintains-it
def c_line_number_ref(ctx):
    fp = ctx["fp"]
    if not fp.endswith(_LINE_REF_PROSE):
        return None
    tool = ctx["tool"]
    if tool == "Write":
        new = ctx["ti"].get("content") or ""
    elif tool == "MultiEdit":
        new = "\n".join(
            (e.get("new_string") or "") for e in (ctx["ti"].get("edits") or [])
        )
    elif tool == "Edit":
        new = ctx["ti"].get("new_string") or ""
    else:
        return None
    # Checked AFTER `new` is in hand: a Write that turns a file into a generated
    # artifact is judged on what it writes, not on what the file used to be.
    if not _in_prose_root(fp, new):
        return None
    bad, fence = [], False
    for n, line in enumerate(new.split("\n"), 1):
        if line.lstrip().startswith("```"):
            fence = not fence
            continue
        if fence:
            continue
        for m in _LINE_REF.finditer(line):
            if m.group(1).startswith("rfc/full/"):
                continue
            bad.append((n, m.group(0)))
        for m in _LINE_ANCHOR.finditer(line):
            bad.append((n, m.group(1)))
    if not bad:
        return None
    detail = "\n".join(f"  +{n}: {ref}" for n, ref in bad[:4])
    fix = (
        "\n  Cite the file and the SYMBOL, not the line: `session.go`, `handleOpen()`.\n"
        "  A line number is right only when the line itself IS the fact, and then it\n"
        "  belongs in a fenced block as quoted output. See ai/rules/writing.md.\n"
        "  A forge permalink is the same case: link the file and NAME the function,\n"
        "  [bgp_io.c `bgp_write`](https://.../bgp_io.c), never a bare #L anchor.\n"
        "  Name the symbol BEFORE dropping an anchor. Two citations into one file\n"
        "  collapse into the same link once their anchors go.\n"
        "  A line number is allowed only where a GENERATOR keeps it current: the\n"
        "  file must declare `GENERATED ... do not edit` in its first ten lines,\n"
        "  as rfc/requirements/rfc7606.md does. Derived or absent, no third option.\n"
        "  Sweep an existing file with: scripts/dev/line_refs.py --apply"
    )
    return (
        2,
        f"{RED}{BOLD}❌ BLOCKED: line-number citation in prose{RESET}\n{detail}{fix}",
    )


CHECKS = (
    c_line_number_ref,
    c_generated_files,
    c_rendered_rules,
    c_point_overwrite,
    c_rule_point_rfc_language,
    c_design_without_lsp,
    c_claude_plans,
    c_source_edit_spec,
    c_pre_write_go,
    c_check_existing_patterns,
    c_legacy_log,
    c_panic,
    c_ignored_errors,
    c_nolint,
    c_os_exit,
    c_layering,
    c_silent_ignore,
    c_yagni,
    c_and_functions,
    c_init_register,
    c_utils_package,
    c_temp_debug,
    c_encoding_alloc,
    c_format_alloc,
    c_sprintf_new,
    c_string_concat,
    c_raw_ansi,
    c_hardcoded_commands,
    c_switch_dispatch,
    c_require_design_ref,
    c_require_related_refs,
    c_test_weakening,
    c_json_kebab,
    c_goroutine,
    c_version_config,
    c_lint_exclusions,
    c_exabgp,
    c_observer_sys_exit,
    c_ci_sleep_justification,
    c_direct_fs_state,
    c_fake_bufhandle,
    c_require_docs_read,
    c_require_test_first,
    c_throwaway_tests,
    c_enforce_naming,
    c_check_existing_tests,
    c_system_tmp_we,
    c_scratch_path_we,
)


def main():
    try:
        import json

        payload = json.load(sys.stdin)
    except Exception:
        return 0
    tool = payload.get("tool_name")
    if tool not in ("Write", "Edit", "MultiEdit", "NotebookEdit"):
        return 0
    ti = payload.get("tool_input") or {}
    ctx = {
        "tool": tool,
        "ti": ti,
        "fp": ti.get("file_path") or "",
        "content": std_content(ti),
        "transcript": payload.get("transcript_path") or "",
    }

    worst = 0
    messages = []
    for check in CHECKS:
        try:
            r = check(ctx)
        except Exception:
            import traceback

            sys.stderr.write(
                f"[pretool-writeedit] {check.__name__} errored (failing open):\n"
            )
            traceback.print_exc()
            continue
        if r is None:
            continue
        code, msg = r
        messages.append(msg)
        worst = max(worst, code)
    if messages:
        sys.stderr.write("\n".join(messages) + "\n")
    return worst


if __name__ == "__main__":
    try:
        sys.exit(main())
    except Exception:
        import traceback

        traceback.print_exc()
        sys.exit(0)
