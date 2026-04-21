# No Layering Rationale

Why: `ai/rules/no-layering.md`

## Why "Integration" Means REPLACEMENT, Not ADDITION
Keeping both old and new systems creates maintenance burden, confusion about which to use, and eventual bit-rot of the "temporary" old system that never gets removed.

## Required Behavior
- Delete old code BEFORE writing new
- If deletion breaks things, fix the breakage
- No "temporary" compatibility code
- Ask "am I adding or replacing?" before every change

## Call Out Phrase
If Claude proposes keeping both systems, user should say: "You're layering. Delete the old one."
