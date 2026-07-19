# Error Messages

**BLOCKING.** Every error, log line, and failure output you write must let a
human or an agent see what failed, why, and what to do next, without opening the
source. The error is the corrective signal: if it does not point at the fix, the
reader cannot act and an agent cannot self-correct.

## The contract: what / why / next

An error must answer three questions:

1. **What failed** -- the specific operation plus the identifying subject:
   `file:line`, config key, field name, command, NLRI, port, diagnostic code.
   Never a bare "operation failed" or "invalid input".
2. **Why -- the evidence** -- the offending value AND the expected one. Quote
   values with `%q` so empty, whitespace, or look-alike values are visible:
   `expected exit code %d, got %d`, `unknown field %q (want one of ...)`.
3. **What to do next** -- the corrective action, or a stable handle the reader
   can act on: a directive to add, a flag to set, a make target to run, or a
   registered `doctor-*` diagnostic code that `ze explain` expands.

If the next step needs more than one line, attach a diagnostic code (below)
rather than truncating the guidance.

**Leg 3 must be TRUE, not merely present.** A remediation that names a command
must name one that actually produces the promised effect. A command that looks
plausible but does not do what the message claims is worse than no advice: the
reader trusts it, follows it, and loses the time twice, then stops trusting the
tool's output at all. Verify the producer before you print the instruction -- if
the message says "re-run X to refresh Y", read the code that writes Y and confirm
X writes it (a lint target does not rewrite a verify record; only a verify run
does). This is the `doctor-vpp-lcp-netns` class of bug: advice that cannot work.

Scope of leg 3: it is mandatory on machine-facing surfaces (doctor, startup,
config apply/verify, readiness, plugin load -- the diagnostic-code surfaces
below). For internal errors that get wrapped upward, legs 1 and 2 plus a
wrapped cause (`%w`) are the requirement; add the corrective action whenever
a clear next step exists, but a deep internal error need not invent one.

## Format: humans scan, agents parse

| Rule | Why |
|------|-----|
| Lowercase start, no trailing punctuation, single line | Go convention; errors get wrapped, joined, and grepped |
| One **stable leading phrase** per failure kind (e.g. `reject=syslog pattern found:`) | Agents and log scanners match on it; do not reword per call site |
| Wrap the cause and add context: `fmt.Errorf("parse %s: %w", path, err)` | Preserves `errors.Is/errors.As` chains; each layer adds what it knows |
| Name the subject and the value, not just the type | "invalid value" with no value is unactionable |
| Truncate large blobs (bodies, dumps, hex) before embedding | A 10 MB error is unreadable for both humans and agents |
| No `fmt.Sprintf`/`fmt.Errorf` on hot paths -- see `ai/rules/no-sprintf-alloc.md` | Boundary and one-shot errors may use `fmt.Errorf`; hot paths use append builders |

## Fail closed, never vacuously

When a check, assertion, validation, or translation cannot be evaluated, return
an error that says so. Never return success, `nil`, or skip. A silent skip is a
false pass and a silent data loss, and it removes the corrective signal entirely.
See `plan/learned/839-ci-runner-false-pass-audit.md` and
`ai/rules/exact-or-reject.md`.

## Machine-facing failures: carry a diagnostic code

User-facing and runtime failures (doctor, startup, config apply, readiness, plugin
load) must carry a registered code in `internal/core/diagnostic/codes.go` with
title, description, examples, and remediation, explainable via
`ze explain <code>`. Return the code plus structured fields, not a pre-formatted
sentence -- see `ai/rules/derive-not-hardcode.md`. The diagnostic code is what
makes the corrective action machine-readable for an agent.

## Mechanical check

Before returning or logging an error, ask:

1. Does it name the specific subject (path/key/field/value), not just the operation?
2. Could a reader who has never seen this code take the next step from this line alone?
3. If the next step needs more than one line, is there a diagnostic code carrying it?
4. Is the leading phrase stable and greppable, or did I reword a shared failure?

Any "no" -- add the subject, the value, the corrective action, or the code.

## Banned

| Pattern | Fix |
|---------|-----|
| `errors.New("failed")`, `"invalid input"`, `"unexpected error"` | Name what, the value, and the expected |
| Dropping the cause inside `if err != nil` (`return errors.New("parse failed")`) | Wrap: `fmt.Errorf("parse %s: %w", name, err)` |
| Reporting a value as invalid without printing it | Include `%q` of the offending value |
| Rewording a stable error phrase per call site | Keep one phrase so it stays greppable |
| Returning `nil`/skip when a check cannot run | Return an error; fail closed |
| A user-facing failure with no diagnostic code or remediation | Register a `doctor-*` code, make it `ze explain`-able |
