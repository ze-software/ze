# Rationale: the review skill

`ai/skills/ze-review.md` holds the steps a reviewing agent runs. This page
holds the reasons and the history behind some of them. Each section is named by
the step that needs it, so the reason is one read away and the step does not
carry it.

## One diff file for every reviewer

The main thread already knows the scope: it wrote the file list. Left to find
it, each agent runs its own `git diff` and reads the changed files again.
Measured 2026-08-10: three lenses over a 1000-line diff cost 364k tokens
between them.

## Review is not a cost target

Review is the cheapest phase measured, and it prevents the most expensive one.

## Three lenses where they apply

The count is not padding where it applies: on 2026-08-10 each of the three
lenses found a real defect. 94 live files grandfathered as dead, a ratchet that
disarmed itself on any git error, and two false claims in a module docstring.

## Auditing against origin/main

This repository commits directly to main, so `main` is normally HEAD and
auditing against it would compare nothing. The tool refuses that with exit 2
rather than reporting clean.

## A diff bigger than its problem

Auditing the details of over-engineered code ratifies it, and every fix it
drives earns another pass over more of it.

## Wiring first

Wiring verification leads the correctness checks because it catches the
project's most recurring defect class (`plan/learned/RECURRING-PATTERNS.md`).
If new code has no caller in production, nothing else in the review matters.

Unwired code means the feature does not exist from the user's perspective, so
reviewing its correctness, security, or edge cases is premature. That is why a
wiring BLOCKER stops the review until the user acknowledges it.

## The test rewrite check

The test-weakening hook cannot catch a rewrite that drops an old assertion,
because the test keeps the same structural shape. This step is the defense.

## Findings the filter never discards

An unwired symbol is dead code in production. A missing functional test is a
coverage gap. A lost invariant from deleted code is a correctness regression. A
logic bug (wrong condition, wrong variable, off-by-one) is a correctness
defect. A bandaid fix at the wrong depth compounds maintenance cost. A hot-path
allocation is measurable overhead.

## Where the config-surface checks came from

They are failure modes seen building nested config surfaces and cross-module
command trees (the traffic / anomaly / ddos nesting). Each one broke silently,
or only surfaced at daemon startup or on another OS, never at compile time.

## Why later rounds shrink

A full-diff pass every round never converges, because a diff of any size always
yields something new. The agent is then unable to close work that is genuinely
finished.

## Why the always-in-scope list is not restated

A second copy is how the corrected rule and the defective one end up one hop
apart, and an enumeration that falls short by one class reads as permission to
home the class it omitted.

## A regression test per fix

The issue exists because a test was missing, so the fix is incomplete without
one.
