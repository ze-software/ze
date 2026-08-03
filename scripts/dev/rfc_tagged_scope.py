#!/usr/bin/env python3
"""The single definition of "the tagged unit" -- the text an `RFC requirement:` tag governs.

Spec: plan/spec-rfcgate-3-audit-teeth.md

WHY THIS IS A LEAF MODULE
-------------------------
Two very different programs need the SAME answer to "which text does this tag govern":

  * `.claude/hooks/pretool-writeedit.py` (`c_test_weakening`) widens an edit hunk to its
    enclosing unit so editing the BODY of a tagged test cannot slip past the guard.
  * `scripts/dev/rfc_requirements.py` (`check_audit_freshness`) fingerprints that unit so a
    recorded audit verdict goes stale exactly when what it judged changed.

`reseal_rfc_audits`'s own docstring names the failure a second copy would cause: a
fingerprint rule that drifted would re-seal verdicts against a hash the gate does not
compute. The gate must not import from `.claude/`, and the hook must stay import-light and
runs under `python3 -S`, so a stdlib-only leaf that BOTH import is the only shape that keeps
exactly one definition. This module therefore imports neither of them, and nothing beyond
`re`.

THE TWO TRAPS THIS MODULE'S CALLERS KEEP FALLING INTO
----------------------------------------------------
1. `go_func_scopes` returns CHARACTER OFFSETS, not line numbers. A tag's line number cannot
   be compared against a span; convert with `line_offset` first. Comparing the two directly
   produced a measurement that read as a clean corpus while checking nothing.
2. The spans are NOT a partition of the file. A tag can sit outside every one of them (a
   hoisted table, a tag separated from its func by a blank line, a `func` inside a raw
   string), and the honest answer there is the WHOLE FILE -- more checking, never less
   (`ai/rules/evidence.md`).
"""

import re

# How a path's unit is resolved. `func` = one top-level Go function span; `file` = the whole
# file is the unit. Recorded on the fingerprint so a reader can tell a narrow answer from
# the fallback, and so a file-scoped verdict is never mistaken for an unresolved one.
SCOPE_FUNC = "func"
SCOPE_FILE = "file"

# Shapes the RFC tag scanner reads. Mirrors the `suffix` column of
# `rfc_requirements.CARRIERS`; `TestTaggedScopeCoversEveryCarrier` asserts the two agree, so
# adding a carrier without teaching this module fails the gate rather than silently leaving
# a carrier the edit-time guard does not protect (the state `.py` was in until 2026-07-29).
TAG_CARRIER_SUFFIXES = ("_test.go", ".ci", ".et", "/check.py")

_GO_FUNC_START = re.compile(r"^func\b", re.MULTILINE)
_GO_FUNC_END = re.compile(r"^\}", re.MULTILINE)


def scope_reader(path):
    """How to resolve this path's unit: `func` spans for Go, the whole file for everything else.

    Total on purpose -- there is no "unknown" answer. Go is the only shape with a
    machine-readable unit boundary cheap enough to trust, so every other carrier (`.ci`,
    `.et`, an interop `check.py`) is file-scoped BY DECLARATION rather than by accident.

    That was the `.py` defect: an interop `check.py` fell to whole-file scope only because
    `_GO_FUNC_START` finds no `func` in Python, which is the right answer reached for the
    wrong reason -- and a reason that would have silently changed the day anyone taught the
    span finder about `def`. File scope is strictly MORE sensitive than function scope, so
    declaring it can only over-trigger (a re-read), never under-trigger (a false fresh).
    """
    return "go" if path.endswith(".go") else SCOPE_FILE


def is_tag_carrier(path):
    """True when the RFC tag scanner reads this shape, i.e. a tag here is real evidence.

    The edit-time guard's file predicate. Distinct from `scope_reader`, which answers a
    different question (HOW to resolve a unit) for a wider set: producer `.go` files carry
    no tags but their symbol spans are fingerprinted by an `unimplemented` verdict's `code`
    map.
    """
    return any(path.endswith(suffix) for suffix in TAG_CARRIER_SUFFIXES)


def line_offset(content, line):
    """Character offset of the start of a 1-based line.

    Exists because every consumer needs it and the one that inlined it got it wrong. Clamps
    to the end of content rather than raising: a tag line past EOF means the file shrank
    under us, and the caller's fallback (file scope) is the safe answer, not a crash.
    """
    if line <= 1:
        return 0
    off = 0
    for _ in range(line - 1):
        nl = content.find("\n", off)
        if nl < 0:
            return len(content)
        off = nl + 1
    return off


def doc_comment_start(content, at):
    """Walk back from a `func` offset over its contiguous `//` doc comment."""
    line_start = at
    while line_start > 0:
        prev = content.rfind("\n", 0, line_start - 1) + 1
        if not content[prev:line_start].lstrip().startswith("//"):
            break
        line_start = prev
    return line_start


def go_func_scopes(content):
    """Each top-level func as [doc comment .. closing brace) -- NOT a partition of the file.

    Two boundaries matter, and both were wrong in turn:

    The END is the func's own closing brace, capped at the next func's doc comment. Running
    to the next `func` KEYWORD swallowed the following function's doc comment, where tags
    live, and so treated every function that merely precedes a tagged test as tagged: 331
    of 3220 untagged functions on this repo. Running to the next DOC COMMENT fixed that but
    left the spans contiguous, which quietly re-homed any tag in the gap between one func's
    brace and the next func's doc comment -- a tag separated from its func by a blank line,
    or a table hoisted between two funcs -- onto the PRECEDING function. The gate credits
    such a tag (scan_go_tags accepts one anywhere, rfc_requirements.py) while the hook
    protected the wrong function, and the caller's "tag outside every scope" fallback could
    never fire because no gap existed to fall into.

    Column 0 for the closing brace is gofmt's guarantee for a top-level func. A one-line
    func has none, so the cap keeps its span at the old, safe boundary rather than running
    to the next func's brace.
    """
    starts = [m.start() for m in _GO_FUNC_START.finditer(content)]
    ends = [m.start() for m in _GO_FUNC_END.finditer(content)]
    spans = []
    for i, s in enumerate(starts):
        begin = doc_comment_start(content, s)
        cap = (
            doc_comment_start(content, starts[i + 1])
            if i + 1 < len(starts)
            else len(content)
        )
        brace = next((e for e in ends if e > s), None)
        end = cap if brace is None else min(brace + 2, cap)  # +2: past "}\n"
        spans.append((begin, max(end, s + 1)))
    return spans


_GO_FUNC_DECL = re.compile(
    r"^func\s+(?:\([^)]*\)\s*)?(?P<name>[A-Za-z_][A-Za-z0-9_]*)", re.MULTILINE
)


def func_name_in(text):
    """The name a span DECLARES, or "" when no declaration is visible.

    Reads the SPAN, not the file: `go_func_scopes` starts each span at the doc comment, so the
    declaration line is already inside the text a caller holds. Both Go shapes are read, `func
    Name(` and `func (r Recv) Name(`, and a generic `func Name[T any](` too, because the name
    is taken before the parameter list rather than after it.
    """
    m = _GO_FUNC_DECL.search(text)
    return m.group("name") if m else ""


def go_func_units(content):
    """[(name, text)] for every top-level func span, in file order.

    The by-name view of `go_func_scopes`. A caller that must recover WHICH function a recorded
    fingerprint hashed compares its recorded sha against the sha of each text here, which is an
    exact match and immune to line drift.
    """
    return [
        (func_name_in(content[a:b]), content[a:b]) for a, b in go_func_scopes(content)
    ]


def func_name_at(path, content, line):
    """The name of the top-level func enclosing a 1-based line, or None for file scope.

    The naming half of `unit_at`, and it answers for the SAME span by construction: a key that
    named a function `unit_at` does not resolve to would fingerprint one text and be checked
    against another. None comes back exactly where `unit_at` returns SCOPE_FILE, so a caller
    minting a key writes the bare path there and states "the file is the unit" instead of
    guessing a narrower answer.
    """
    if scope_reader(path) != "go":
        return None
    spans = go_func_scopes(content)
    off = line_offset(content, line)
    hit = [s for s in spans if s[0] <= off < s[1]]
    if len(hit) != 1:
        return None
    return func_name_in(content[hit[0][0] : hit[0][1]]) or None


def func_text(content, name):
    """The text of the ONE top-level func declared `name`, or None.

    None covers two states on purpose: a name no span declares, and a name two spans declare
    (two methods with different receivers can share one name in one file). Both must be refused
    by the caller. Picking either of two same-named functions would fingerprint text nobody
    chose, and the honest answers are "re-read it" and "say which one".
    """
    found = [text for n, text in go_func_units(content) if n == name]
    return found[0] if len(found) == 1 else None


def unit_at(path, content, line):
    """The unit text governing a tag at a 1-based line, and how it was resolved.

    Returns `(text, kind)` where kind is SCOPE_FUNC or SCOPE_FILE. Never invents an answer:
    a line that lands outside every span, or inside more than one, resolves to the WHOLE
    FILE and says so, because a narrower answer there would be a guess and a wrong guess is
    a false FRESH -- the one catastrophic outcome (spec R-2).

    Callers must reject an empty `text` themselves. This function does not raise on empty
    content, because "" is a legitimate reading of an empty file and only the caller knows
    whether hashing it would be a lie.
    """
    if scope_reader(path) != "go":
        return content, SCOPE_FILE
    spans = go_func_scopes(content)
    off = line_offset(content, line)
    hit = [s for s in spans if s[0] <= off < s[1]]
    if len(hit) != 1:
        return content, SCOPE_FILE
    a, b = hit[0]
    return content[a:b], SCOPE_FUNC


def tag_scope(path, content, hunks, tag_re):
    """The text whose RFC tags govern an edit, widened from the hunk to its context.

    An Edit replaces one hunk, and the hunk is all the edit-time guard used to see. A tag
    lives on the line above the function or on a sibling table case, so editing the BODY of
    a tagged test met no tag and slipped past the one check written to stop that.

    Scope is the enclosing top-level `func` plus its doc comment, NOT the whole file: a test
    file holds dozens of functions and typically a handful of tags, and a guard that blocks
    unrelated work is a guard that gets switched off.

    Returns None when there is nothing to widen (no hunks, no tag anywhere, or a Write,
    which already carries the whole file as its own `old`), and the caller then judges the
    hunk exactly as before.

    Falls back to the WHOLE FILE whenever the narrow answer would be a guess: a hunk that is
    not found, a hunk outside every function, or -- importantly -- a file holding a tag that
    no function scope covers. That last case is what keeps this honest against the scanner,
    which credits a tag ANYWHERE in the file.

    `tag_re` is supplied by the caller rather than owned here: the edit-time guard matches a
    deliberately broader pattern than the gate's scanner (it also matches the phrase in
    ordinary prose, which is what makes it widen to file scope for that one file), and
    hard-coding either pattern here would silently change one of the two callers.

    KNOWN LIMIT: the scope cannot follow a call, so an assertion moved into a helper
    function defined outside the tagged test is not covered.
    """
    if not hunks:
        return None
    if not tag_re.search(content):
        return None
    if scope_reader(path) != "go":
        return content  # no function boundaries to narrow to; the file is the test

    spans = go_func_scopes(content)
    for m in tag_re.finditer(content):
        if not any(a <= m.start() < b for a, b in spans):
            return content

    picked = []
    for hunk, replace_all in hunks:
        if not hunk:
            continue
        # With replace_all, EVERY occurrence is rewritten, so every occurrence's scope
        # counts: inspecting only the first would let "change this assertion everywhere"
        # reach a tagged test while the guard looked at an untagged one. Without it the tool
        # itself rejects an ambiguous old_string, so the first occurrence is the only one
        # that can be edited -- and unioning anyway told the author "BLOCKED: RFC-tagged
        # test" when the real problem was a non-unique hunk, a wrong-cause diagnosis on
        # roughly a quarter of ambiguous edits.
        found = False
        start = content.find(hunk)
        while start >= 0:
            found = True
            end_at = start + len(hunk)
            hit = [s for s in spans if s[0] < end_at and start < s[1]]
            if not hit:
                return content  # outside every function: no narrower honest scope
            picked.extend(hit)
            if not replace_all:
                break
            start = content.find(hunk, start + 1)
        if not found:
            return content  # unlocatable hunk: err toward asking
    if not picked:
        return None
    return "\n".join(content[a:b] for a, b in sorted(set(picked)))
