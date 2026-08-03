# 1240 -- fixit-spec-hygiene-tooling-deferred-operational-cleanup

## Context

Two operational cleanups that `spec-fixit-spec-hygiene-tooling` found but deliberately did not
perform (it was building the checks, not running their output). Filed as a spec so they kept a
destination and stayed visible to the commit gate. Both are chores against repo state, not
design or code.

## Decisions

- **Item 1 -- closed `spec-ipsec-13-rekey-wire`.** Verified genuinely complete before removing
  (not on the flag alone): its Review Gate reads 0 BLOCKER / 0 ISSUE with interop `05-child-rekey`
  re-verified PASS, the CREATE_CHILD_SA rekey code + learned 1069 are committed, and the rekey
  unit tests + `go vet` are green. Repointed the one code design-ref (`ike/engine/msgid.go`) to
  learned 1069, then two-commit-closed it.
- **Item 2 -- `ze-regen-check` green + no un-indexed learned.** The red was NOT un-indexed learned
  files (`ai/LEARNED-FULL-INDEX.md` already covers all 1211 summaries, numbering unique) but two
  stale derived indexes: `ai/rules/CONDENSED.md` (had not absorbed the `platform-linux.md`
  cadence/GPL/pin-table additions from `spec-fixit-supply-chain-hardening`) and `ai/CODE-TO-DOCS.md`.
  Regenerated both (`make ze-regen`) and committed them; `ze-regen-check-readonly` is now green.

## Consequences

- The completed-but-not-closed backlog shrinks by one (ipsec-13 gone from `plan/`).
- `ze-regen-check` (a structural gate) passes on a clean checkout again.

## Gotchas

- Editing an `ai/rules/*.md` rule file (e.g. `platform-linux.md`) stales `ai/rules/CONDENSED.md`,
  which `make ze-discovery-index` does NOT refresh -- only the fuller `make ze-regen` does. Run
  `ze-regen` (not just `ze-discovery-index`) after touching a rule file, or `ze-regen-check` goes
  red on the next clean verify.
- Repointing a `// Design:` code comment stales `ai/DOCS-TO-CODE.md`; regenerate it in the same commit.

## Files

- (chore spec: no feature code) plan/spec-ipsec-13-rekey-wire.md (removed), ike/engine/msgid.go (design-ref repoint), ai/rules/CONDENSED.md + ai/CODE-TO-DOCS.md (regenerated)
