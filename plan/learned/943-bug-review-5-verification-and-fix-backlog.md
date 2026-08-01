# bug-review-5-verification-and-fix-backlog

## Summary

The final backlog pass is an evidence reducer. Its job is to merge duplicate root causes, preserve rejected-candidate proof, and turn accepted findings into TDD-ready implementation specs.

## Key decisions

- Loaded inventory plus all three area reports before accepting or rejecting any finding.
- Grouped findings by root cause so related bugs share one implementation spec when they enforce the same invariant.
- Accepted plausible findings only when the trigger was concrete and the implementation spec could require proof before code changes.
- Kept rejected candidates in the final report so future reviews do not repeat the same false positives.

## Results

- Created `plan/review-bug-review-final.md`.
- Mapped 10 accepted findings to 8 fix specs.
- Created final fix specs for BENG-004, BENG-005, BPLUG-001, and BPLUG-002.
- Preserved non-promoted SYS-004, SYS-005, BPLUG-P1, and BPLUG-P2 with future routing.

## Gotchas

- A fix spec is incomplete if its Implementation Summary, Audit, Goal Validation, Review Gate, or Pre-Commit Verification section still has template placeholders.
- The final report should not say artifact closure is complete until the parent and child specs are also closed.
- Accepted finding ledgers must map every ID to exactly one implementation spec or explicitly explain the exception.

## Verification

- Final report audit tests passed manually: all child reports loaded, dedupe has root cause, every accepted finding has a fix spec, rejected candidates have proof, inventory coverage is zero-missing, and fix specs include regression plans.

## Files

None recorded.
