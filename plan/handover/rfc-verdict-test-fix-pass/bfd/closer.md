# BFD judge + closer, 2026-09-30

Part 1 committed: 460ae9faf5 "rfc: bfd auth join refused on differing auth, 5882 10.2-1".
- RFC5882-10.2-1 re-judged enforced; RFC5882-4.4-1 re-judged weak (unchanged: narrowKey never widens entry.joined on release; no journal row found for it, parent owns 4.4-1).
- Judge additions: stale timer-merge claim also fixed in api/events.go SessionRequest comment and docs/architecture/bfd.md (auth refusal paragraph + auth_join.go anchor).
- Note (non-blocking): refused-join cases never differ in Auth Type or Meticulous alone; sameAuth compares both.
- Gates run: race bfd/static/ospf ok (job-bfdj-race-14faa4e2.log), full reactor race with feature tags ok (job-bfdj-reactor-e325b7f6.log), scoped lint 0 issues except untracked other-session ospf/lsdb/rfc2328_flood_age_test.go (job-bfdj-lint-f6099843.log), rfc check: no rfc5880-5883 line (bfdj-check2.log), index-update under lock.

Part 2 (close) STOPPED before any closure edit or commit.
- Derived listing: EMPTY. AC-C1..C7 re-checked: C1 empty listing, no Blocked-by; C3 every split/narrowing row has its own row with a verdict (6.7.3-14/6.7.4-8 unimplemented {gap}, 6.7.4-9, 4.2.2.1-1/-2, 4.2.2.2-1, 10.2-1 enforced; 5883-5-2 retired with correction); C4 none; C5 no violation; C6 RFC5880-4.1-1 correction present; C7 none.
- Blocking gates this agent may not run (5-minute rule): ze-close step 3 `./le verify worktree` (verify status STALE since 2026-09-25) and the spec Goal Gate for it; ze-close step 1 interop for the child's D-8 protocol fixes (spec Interop row: FRR bfdd). No child handoff records any interop run. Candidate scenarios: bfd-frr, bfd-simple-password-bird, ospf-bfd-frr, bgp-bfd-strict-frr.
- `./le spec release` must be SKIPPED (session marker holds the parent spec).
- Citers of the spec: only journal Spec cells (bare stem) and the spec itself.
