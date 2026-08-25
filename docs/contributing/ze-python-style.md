# Ze Python Style

Ze writes its tooling in Python, never in shell. A shell script is fragile, it
is hard to debug, and it fails differently on each machine.
<!-- source: ai/rules/go-standards.md -- Scripts: Python Only -->

This page is the working standard for that Python. It has two companions.
`ze-go-style.md` is the standard for every line of Go, and `writing-style.md`
is the standard for every word.

This page carries the reasoning. The mechanical detail lives in
`pyproject.toml`, in `scripts/le/ruff.toml`, and in `ai/rules/`. When this page
and one of those disagree, the other one wins.

## Why

ExaBGP's Python is a daemon. It reads bytes that a peer on the public internet
sent. A crash is an outage there, and a wrong route is worse than no route.

Ze's Python is not a daemon. It is tooling: gates, checkers, test drivers,
generators, and site builders. The tree holds 468 tracked `.py` files and
175,571 lines of them, measured on 2026-08-25 with `git ls-files '*.py'`.
`scripts/le` is 8,146 of those lines and every other directory is the
remaining 167,425.

That population has two failure modes of its own, and this page exists to stop
them.

| # | Failure | Why it costs more than it looks |
|---|---------|---------------------------------|
| 1 | A gate that reports green and checked nothing | A green run says one of two things, and they look identical: the code is correct, or the code never ran. Every session after it reads the first and inherits the second |
| 2 | A tool that destroys work | A generator, a migrator, or a cleanup step runs unattended. When it stops half way and records nothing, the operator has no way back |

Failure 1 is the one this repository meets most. `make ze-lint` reported
`NOT COVERED BY ANY PASS` for a file that the working tree no longer held,
because `population()` read `git ls-files`, which answers from the index.
<!-- source: scripts/dev/lint_flavors.py -- population -->

Failure 2 is real too. `migrate` in `ensure-links.py` moves the entries of
`tmp/` one at a time with `shutil.move` and holds no undo. Across two devices
`shutil.move` copies and then removes, so a failure at entry N leaves N-1
entries moved and entry N in both places. It failed twice on 2026-08-25.
<!-- source: scripts/dev/ensure-links.py -- migrate -->

So the order is: safety, then performance, then developer experience. Never
trade the first for the second. The third is what makes the first two survive
next year.

> "Simplicity is prerequisite for reliability." (Dijkstra)

## 1. Safety

### 1.1 Never trust what you did not write

ExaBGP says never trust the wire. Ze has no wire here, and the shape is the
same. A gate reads the repository. A checker reads a generated file. A test
driver reads the stdout of a daemon or of `ip`, `docker`, and `go`. None of
that text is yours, and all of it changes under you.

- **Parse what the tool prints today, and pin it with a test.** A regular
  expression written from memory matches nothing and reports zero. Zero then
  reads as a clean result at every layer above it.
- **A count of zero from a parser is a finding, never a pass.** Say so where
  the parse happens, because that is the one layer that knows.
- **Read the population from the thing you judge.** `git ls-files` answers from
  the index, and `os.listdir` answers from the disk. They disagree exactly when
  a session is part way through a change.
- **An external command that fails is data, not an exception.** Capture the
  exit code, capture stderr, and let the caller decide.

The cost of getting this wrong is measured. Four copies of the IPsec interop
scenarios carried a `bytes\s+(\d+)` pattern that matches no iproute2 release.
Every copy read zero, so every "traffic flowed" assertion built on it passed
whatever the tunnel did. iproute2 prints `846(bytes)`, with the number BEFORE
the word. `parse_xfrm_sa_bytes_by_spi` now reads that shape, and `lab_test.py`
pins it against captured `ip -s xfrm state` output.
<!-- source: test/interop-ipsec/lab.py -- parse_xfrm_sa_bytes_by_spi -->

### 1.2 Assert your invariants, never your input

Python deletes every `assert` under `-O`. An assertion is therefore a statement
about OUR code, checked while you develop and while the tests run. It is never
a validation of anything that arrives from outside the process.

| Source of the problem | What to raise |
|---|---|
| The output of an external command | A typed error, or a `Result` carrying the exit code |
| A repository file, a generated file, or a config | `ValueError` naming the file and the line |
| An argument from the command line | An error message and a non-zero exit, never a traceback |
| Our own invariant broken | `assert`, or `raise RuntimeError` on a path that must hold under `-O` |

Ze runs no Python under `-O` today, and nothing is permitted to depend on that.
A tool a colleague runs under `-O` loses every check you wrote as an assertion.

**Assert liberally, inside that boundary.** An invariant that lives only in
your head is one refactor away from being false. Write it where the code can
check it: a precondition, a postcondition, and the negative space around both.

**A test is where `assert` belongs, and this rule does not reach it.** In a
test the assertion IS the verdict, so an interop scenario `check.py` and a
`<tool>_test.py` assert on whatever they read. The `--selftest` bodies of
`migrate_module.py` and `dep_audit.py` are tests too, and that is where every
assertion in those two files sits.

Outside a test, Ze asserts only what Ze owns. `commit_helper.py` holds
`assert callable(runner)` over a table this repository fills itself.
`effective-vpp.py` holds `assert peer.stdout is not None` after the same
function asked for that pipe.
<!-- source: scripts/dev/commit_helper.py -- assert callable(runner) -->
<!-- source: scripts/evidence/effective-vpp.py -- assert peer.stdout is not None -->

```python
# WRONG: the external tool decides whether this holds, and -O deletes the check
assert result.returncode == 0

# RIGHT: the external tool decides, so the caller gets told
if not result.ok:
    return [f'ruff check failed: {result.complaint()}']

# RIGHT: we fill this table, so a miss is our defect
assert callable(runner)
```

### 1.3 Bound everything

Anything an external command, a repository, or an operator can grow has a
limit, a name, and a defined behavior when it is reached.

- **Every subprocess carries a timeout.** A build tool that wedges holds the
  gate, and a gate that never returns is a gate nobody runs.
- **Every timeout is a named constant with its unit in the name.** Ze already
  does this: `SUDO_PROBE_TIMEOUT = 15` and `VET_TIMEOUT = 900` each sit beside
  a comment that says what the number is for.
- **Every loop over external data consumes input each pass.** A `while` that
  waits for a daemon needs a deadline and a message when it expires.
- **A walk over a tree you did not create carries a depth or a count.** A
  symbolic link cycle is not a rare case in `tmp/`.
- **Say what happens on excess.** Fail the gate, drop the entry, or kill the
  child. Never let the number grow while a check that never fires watches it.

<!-- source: scripts/le/process.py -- SUDO_PROBE_TIMEOUT -->
<!-- source: scripts/dev/rfc_requirements.py -- VET_TIMEOUT -->

### 1.4 Handle every error explicitly

- **No bare `except:`.** The tree holds none today, and it stays that way.
- **`except X: pass` carries a comment** that says why the error is expected
  and why silence is safe.
- **Convert at the boundary the error belongs to.** `run` in
  `scripts/le/process.py` is the pattern. It turns `TimeoutExpired` into exit
  code 124 and `OSError` into exit code 127, and it returns a `Result` for
  both. A caller then branches on one thing rather than on a return value and
  two exception types.
- **Never report success you did not verify.** A function that returns `True`
  on a path it did not run lies to its caller.
- **An absent tool is a failure, never a skip.** See section 3.

<!-- source: scripts/le/process.py -- run -->

### 1.5 Keep functions short

One screen is the target, and it is advisory. A function longer than that does
more than one thing and hides the seam where the defect lives. Split at the
seam, never at a line count.

Ze counts nothing here. That is a deliberate difference from ExaBGP, which
holds a 70-line limit with a ratchet. The reason is the population. A fixture
table and a generated report grow with coverage, so a count would push an
author to split one concern across three files.

### 1.6 Smallest possible scope

- Declare a name where it is used, not at the top of the function.
- Prefer an immutable value. `@dataclass(frozen=True)` is the default shape in
  `scripts/le`, and `Options` in `lint.py` is the one to copy: everything the
  action needs, and nothing about how it was asked for.
- Module-level mutable state that a run can grow is a leak with a slow fuse.
  A module-level constant is correct, and a module-level cache needs a cap.
- Export what you mean to export. Every module in `scripts/le` outside its
  package files declares `__all__`.

<!-- source: scripts/le/application/lint.py -- Options -->

### 1.7 Zero warnings

`ruff check`, `ruff format`, and `mypy --strict` are clean in the strict scope,
every commit, with no exceptions. A `# type: ignore` carries the reason on the
same line. A warning you decided to live with is a warning everybody else
learns to skip past. Section 6 gives the two scopes and what each one owes.

## 2. Performance

ExaBGP's performance section is about a hot wire path. Ze's Python has no hot
path. Here performance is wall-clock time in a gate, and every session pays it,
every run.

Three measurements from 2026-08-25 set the scale. `check_doc_links.py` takes
13.3 seconds and reads 19,596 tracked files. `ste_check.py --changed` takes 2.0
seconds. `hook-fixture-check.py` takes 38.3 seconds for 604 checks.

- **Read the tree once.** `sweep_tracked` opens each tracked file one time and
  runs both of its checks over each line. A second pass for a second check
  doubles the cost of the slowest gate in the set.
- **Filter on bytes before you decode.** `sweep_tracked` searches the raw bytes
  for `MARKER_BYTES` first, so a binary or vendored file costs one read and no
  decode.
- **Fork once, not for each file.** `tracked_files` runs `git ls-files` a
  single time. One `git` process for 19,596 files beats 19,596 of them, and the
  same answer holds for `go list`, `ruff`, and `docker`.
- **When a fork is the point, keep the count proportional to behaviors.**
  `hook-fixture-check.py` runs a hook as a subprocess on purpose, because that
  is the only way to drive one through its real entry point. 38.3 seconds for
  604 behavioral checks is the right trade. The same cost for each FILE in the
  tree would not be.
- **Give a slow gate a `--changed` mode.** `ste_check.py` compares against HEAD
  per file, which is what makes 2.0 seconds possible over the same tree.
- **Measure before you claim.** A performance statement with no number is an
  opinion. `/usr/bin/time -f` costs one command.

<!-- source: scripts/dev/check_doc_links.py -- sweep_tracked, tracked_files -->

## 3. A gate is a guard

A gate is code whose purpose is to reject. `ai/rules/evidence.md` governs it,
and this section is that rule applied to Python. Read the rule before you write
one.

**Fail closed, or say something.** On a miss, an empty set, an unmapped input,
or an error, refuse. A gate that neither refuses nor speaks does not exist.

`_mypy` in `lint.py` is the shape to copy. An absent checker returns a failure
rather than a skip. The docstring says why: a gate that reports green because
its checker is absent reads as "checked" when it means "not checked".
<!-- source: scripts/le/application/lint.py -- _mypy -->

**A zero result must never read as a pass.** An exit code cannot tell "every
test passed" from "no test ran". `TestPythonUnitTests` answers that by counting
what each Python file DECLARES and comparing it against the `Ran N tests` line
the run printed. A file that offers cases its run did not reach fails by name.
<!-- source: scripts/dev/python_tests_test.go -- pythonDeclaredTest, pythonRanSummary -->

**Drive the gate from the entry point that triggers it.** A unit test on a
check helper proves the helper is correct. It proves nothing about whether the
caller reaches it with the input that matters. `spawn` in
`hook-fixture-check.py` is the pattern: it runs the hook as a subprocess and
feeds it the same JSON payload the harness sends.
<!-- source: scripts/dev/hook-fixture-check.py -- spawn -->

**Read the population from the same place the thing you judge reads it.** Two
live examples, both measured:

| Gate | What it read | What it judges |
|------|--------------|----------------|
| `population()` (`lint_flavors.py`) | `git ls-files '*.go'`, which answers from the index | files a linter loads from disk. A tracked file deleted in the working tree failed every run until its commit. Now fixed: `population()` skips a path that is not on disk |
| `c_auto_py_format` (`.claude/hooks/posttool-writeedit.py`) | its own three-prefix exclusion list | the scope `pyproject.toml` declares. The hook reformats a legacy file the configuration states it does not format, so an author cannot tell their own change from the reformat |

**A green sweep is evidence only after it has gone red.** Before you record a
gate as working, break the thing it watches and require it to fail. Revert the
fix, delete the check, or empty the input, then run it again. A sweep that
stays green measures nothing, and the number is worse than useless, because it
reads as coverage.

**Say the whole answer in the format that was asked for.** A caller that asks
for JSON and gets prose has to parse the prose. A caller that asks for JSON and
gets exit 0 with no payload reads it as success. Structured output is part of
the contract, and the error path owes the same shape as the success path.

## 4. Developer experience

### Naming

- **Units belong in the name.** `duration_seconds`, `size_bytes`, and
  `_age_seconds` in `session-reap.py` are the shape. A bare `size` invites the
  caller to guess.
  <!-- source: scripts/dev/session-reap.py -- _age_seconds -->
- **`snake_case` for a function and a variable, `PascalCase` for a class,
  `UPPER_SNAKE_CASE` for a constant, a leading underscore for a private name.**
  This is Python's own convention and ExaBGP's, so no reader has to learn it
  twice.
- **Say what the value is, never what type it is.** `ceiling` beats `max_int`.
- **Booleans are positive.** `fix`, `strict_only`, and `ok` are the shape.
- **No abbreviation a newcomer to the file cannot expand.**

### Docstrings and comments

A docstring says what the function is for and what a caller must know. A
comment says WHY, because the code already says what. A comment that repeats
the line below it is noise, and it goes stale and misleads.

Write the reasoning that is not visible. Name the measurement that set a
number, the external behavior that forced the shape, and the reason the obvious
approach fails. `run` in `scripts/le/process.py` and `sweep_tracked` in
`check_doc_links.py` are the two to read first.

### Types

- `from __future__ import annotations` at the top of the file. Every module in
  `scripts/le` outside its package files carries it.
- Modern syntax for the target version: `int | None`, `dict[str, list[int]]`.
  Both configs set `target-version = "py312"`.
- A function in the strict scope is fully annotated, because `mypy --strict`
  refuses it otherwise.
- Prefer a structural answer over a run-time `isinstance` check.

### Commits

- **One change per commit.** A fix, its test, and nothing else.
- **The subject is imperative and specific.**
- **The body says what was wrong, what it caused, and why this fix is right.**
- **A defect fix arrives with the test that fails without it.** Write the test
  first, watch it fail, then fix. A fix without that evidence is a guess.
- Commit only through `scripts/dev/commit_helper.py` (`ai/rules/git-safety.md`).

### Zero technical debt

Do it properly the first time. The cost of a shortcut is paid by whoever debugs
it at 3am with a network down. Work that must wait is written into a spec under
`plan/`, with what is missing and why, and never left as a silent gap.

## 5. Where the codebase already shows this

| Rule | Look at |
|---|---|
| An error from a child process becomes a value | `run` in `scripts/le/process.py` |
| An absent checker fails rather than skips | `_mypy` in `scripts/le/application/lint.py` |
| A ratchet that refuses to stand still | `_ruff_legacy` in `scripts/le/application/lint.py` |
| One read of the tree, with a bytes prefilter | `sweep_tracked` in `scripts/dev/check_doc_links.py` |
| A gate driven through its real entry point | `spawn` in `scripts/dev/hook-fixture-check.py` |
| A test count that cannot be faked by an exit code | `TestPythonUnitTests` in `scripts/dev/python_tests_test.go` |
| A named timeout with the reason beside it | `SUDO_PROBE_TIMEOUT` in `scripts/le/process.py` |
| Options as a frozen dataclass | `Options` in `scripts/le/application/lint.py` |

## 6. Enforcement

`./le lint` is the gate. It runs four stages, `ruff check`, `ruff format`,
`mypy`, and the legacy ratchet, and it reports all of them. A run that stops at
the first red reports one thing for each invocation.

| Scope | Rules | Obligation |
|-------|-------|------------|
| `scripts/le` and the `le` shim | `scripts/le/ruff.toml`: `E`, `F`, `W`, `I`, `UP`, `B`, `SIM`, `RET`, `PTH`, `Q`, `RUF`, with `E501` left to the formatter. Then `ruff format --check`, then `mypy --strict` | Zero findings, from the first commit |
| Everything else | `pyproject.toml`: `E9`, `F`, and `B` only. Defect shapes, and no style at all | At the ceiling in `[tool.le.lint] legacy-max`, and the ceiling falls |

<!-- source: pyproject.toml -- tool.ruff.lint, tool.le.lint -->
<!-- source: scripts/le/ruff.toml -- lint.select -->

Two facts about that split matter, and the header comment of `pyproject.toml`
carries the measurement behind both. The strict rule set reports 59,207
findings against the legacy tree, and 53,625 of those are the quote style
alone. A gate nobody can pass is not a gate. Ruff picks the rule set per file,
by which config file is nearest, so one `ruff check` applies both.

**The legacy ceiling fails in BOTH directions.** Above it, `_ruff_legacy`
prints the per-rule table and refuses. Below it, the run also refuses, and asks
you to lower the number in the same commit. A ceiling nobody lowers is a
ceiling that stops meaning anything.
<!-- source: scripts/le/application/lint.py -- _ruff_legacy -->

Two type checkers run, with one job each. `pyright` answers the editor and the
language server. `mypy` is the gate, in the strict mode ExaBGP has run since
2025-12-17, so a contributor who knows that repository knows this one.
`./le setup` installs `ruff`, `mypy`, and `pyright`.
<!-- source: scripts/le/devtools/tools.py -- Tool -->

Unit tests are wired through `go test`, and there is no `pytest` and no
`unittest discover` here. Name the file `<tool>_test.py`, use `unittest`, put
it beside the tool, and end it with `unittest.main()`. `TestPythonUnitTests`
globs every root in `pythonTestRoots`, so a tool in a NEW directory needs its
root added there first. The full contract is in `ai/rules/testing.md`, under
"Testing Python Tooling".

## 7. Review checklist

- [ ] Every parser of external output is pinned by a test against captured real output
- [ ] No `assert` on anything that came from outside the process
- [ ] Every subprocess call carries a timeout, and every timeout is a named constant
- [ ] No bare `except:`, and no `except X: pass` without a comment
- [ ] Every gate fails closed, and an absent tool fails rather than skips
- [ ] Every gate reads its population from the same place the thing it judges reads it
- [ ] Every new gate has been made to go red on purpose
- [ ] Names carry their units, booleans are positive, no new abbreviations
- [ ] Comments explain why, not what
- [ ] The fix has a test that fails without it
- [ ] `./le lint` passes, and the legacy ceiling is exact

## Where Ze differs from ExaBGP's TigerStyle

The differences follow from one fact: ExaBGP's Python is a protocol daemon, and
Ze's Python is tooling. Each difference is deliberate.

| Subject | ExaBGP | Ze |
|---------|--------|-----|
| What the code reads | Bytes from a peer on the public internet | Repository state, generated files, and the output of external commands |
| Malformed input | Raises `Notify` and closes the session | Fails the gate, names the file, and exits non-zero |
| Function length | A hard limit of 70 lines, held by a ratchet | One screen, advisory. No gate counts the lines |
| Line length | 120 columns | 100 columns in both configs, with `E501` left to the formatter in the strict scope |
| Performance | A hot wire path: batching, `memoryview`, and no copy of the wire | Wall-clock time in a gate: one read of the tree, one fork, and a `--changed` mode |
| Rule scope | One rule set over the whole tree | Two, chosen per file by the nearest config, because the legacy tree reports 59,207 findings under the strict set |
| Style checker | `./qa/bin/check_tiger_style`, with a baseline in `qa/tiger_style.json` | None. `ruff` and `mypy` carry the mechanical half, and the rest is a review responsibility |
| Mutation testing | `./qa/bin/mutmut_run` over the validation modules | None today. The equivalent is the go-red-on-purpose step in section 3 |
| Test runner | `./qa/bin/test_everything`, six suites | `go test`, through `TestPythonUnitTests` |

## Lineage

This standard is a rework of TIGER_STYLE, ExaBGP's Python standard. That
document adapts TigerBeetle's TigerStyle, which in turn adapts NASA's *Power of
Ten: Rules for Developing Safety Critical Code*. The rework is what the two
populations require, and the examples are Ze's own.

Source: `~/Code/github.com/exa-networks/exabgp/.claude/TIGER_STYLE.md`

The Dijkstra quotation comes from that document.
