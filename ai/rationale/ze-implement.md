# Rationale: the implement skill

`ai/skills/ze-implement.md` holds the steps an implementing agent runs. This
page holds the reasons behind some of them. Each section is named by the step
that needs it, so the reason is one read away and the step does not carry it.

## Pages before the spawn

An agent sent to discover a contract that a page already states pays twice for
one answer.

## Resolving symbols before the spawn

What you buy is paying the resolution ONCE rather than once per agent, and it
is worth buying. When this was measured, the symbol map of
`internal/component/ike/engine/fsm.go` cost 1,297 bytes against 44,164 for the
file (34.1x), and an agent given a bare file name that resolves nothing reads
all of it.

## Reading the handoff instead of the source

Re-reading a source file the handoff already digests is the cost this
decomposition removes.

## Closure in its own skill

Closure instructions reached at the tail of a long skill get partially
followed. Closure used to be steps 11-16 of a 16-step skill, reached with the
context already full of implementation detail. The measurable result: across
161 specs the closure tables were byte-identical to the template in 65-75% of
in-progress specs, while sections authors added when they needed them were
untouched in 0%. `plan/templates/TEMPLATE-CLOSURE.md` was split out of
`plan/templates/TEMPLATE.md` for exactly this reason, and `/ze-close` is the
same fix applied to the instructions. `ai/skills/ze-close.md` points here
too.

## Status first

Other sessions check spec status to avoid collisions. A spec that stays in
`design` or `ready` during implementation lies about its state.

## The BGP Family Checklist

This gate exists because SR-Policy shipped incomplete (3 commits) due to
missing integration points that the generic wiring rules did not catch.

## Style in the phase that wrote the code

A style defect caught in the phase that produced it costs one edit. The same
defect caught in step 7 costs a re-read of every phase before it.

## Why a round has a scope

"Stop only when a pass finds nothing anywhere" has no state in which it stops,
which is why finished work fails to close. Each fix is new code that needs a
fresh review, and that review covers the fix and the sibling call sites it
touched, not the whole diff again, inside the five rounds the planning rule
allows. A round that reviews only the fixes
of the round before it, and the sibling call sites they touched, shrinks each
time, so the loop reaches a clean round.

## Why green means the diff was read

A gate covers what somebody thought to check, so a defect on a surface no gate
reads passes every target. A report that calls the work green before anybody
has read it makes the Review Gate establish what was already announced:
reporting "green" and leaving the reading to the Review Gate inverts the order.
