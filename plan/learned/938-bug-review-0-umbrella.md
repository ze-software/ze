# bug-review-0-umbrella

## Summary

Completed a review-only bug review over authored plugin and BGP core surfaces. The durable pattern is to start from generated reachability, then split by owner, then reduce findings into implementation specs instead of mixing review and fixes.

## Key decisions

- Use `internal/component/plugin/all/all.go` as the compiled inventory anchor, then reconcile directory-only command roots and registry classes separately.
- Keep production code unchanged during the review. Accepted defects become follow-up `spec-bugfix-*` specs with ACs and regression plans.
- Treat child reports as evidence artifacts: inventory, plugin/system, BGP engine, BGP plugins, then final dedupe.
- Promote only findings with source evidence, reachable trigger, expected/actual behavior, impact, severity, owner, and test plan.

## Results

- Created five review reports and eight fix specs.
- Accepted 10 findings: SYS-001, SYS-002, SYS-003, BENG-001, BENG-002, BENG-003, BENG-004, BENG-005, BPLUG-001, BPLUG-002.
- Kept SYS-004, SYS-005, BPLUG-P1, and BPLUG-P2 as plausible but not promoted.

## Gotchas

- A final report can look complete while canonical spec closure sections still block completion. Close the specs and fill fix-spec audit sections before claiming artifact completion.
- Generated imports prove compilation reachability, not ownership. Directory-only command roots still need command wiring review.

## Verification

- `make ze-spec-status` passed after the bug-review specs moved to `done`.
- ArtifactReview found two closure issues; both were fixed before handoff.

## Files

None recorded.
