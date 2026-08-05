# 1346 -- A Directive You Only Want To Wait On Is Still An Assertion

## Context

The relay-shape spec sat open for two weeks after its code landed. Closing it
meant re-measuring its own tables rather than reading them, and four of its
claims had decayed: the `{gap}` count it asserted was one higher than the file
now holds, its `ze-rfc-check` tag count was two weeks old, its Wiring row cited
line numbers the file no longer has, and its interop scenario 47, recorded as
"4/4 stable", failed one run in four. None of the drift was caused by anyone
touching this spec's code. The tree moved underneath a document nobody re-read.

## Decisions

- Held the review gate to the SPEC STEM over the work-name the author used.
  `spec_closure_stem` (`scripts/dev/commit_helper.py`) derives the artifact key
  from the removed spec path, so Runs 4 and 5, filed as
  `withdraw-only-relay-shape` and `relay-withdraw-attribute-gate`, were reviews
  of nothing as far as closure was concerned: the gate read BLOCKED while the
  spec's prose read as reviewed.
- Fixed interop 47's flake with `option=linger:value=true` over adding a retry or
  loosening the assertion. The two trailing `expect=bgp:...KEEPALIVE` rules were
  a hold-open device that also asserted the next frame's type; linger holds the
  session without asserting anything.
- Overturned a reviewer's ISSUE on the RFC text rather than writing code for it.
  A rebuilt body with `nlriOverride = []byte{}` carries the source's complete
  attribute set and no NLRI, and RFC 4271 Section 6.3 makes that shape explicitly
  valid: "An UPDATE message that contains correct path attributes, but no NLRI,
  SHALL be treated as a valid UPDATE message." RFC 7606 Section 5.2 escalates
  only "if any path attribute errors are encountered in such an UPDATE message".
- Added `test/plugin/relay-withdraw-reflector.ci` over leaving the RFC 4456
  reflection pair proven at daemon level by nightly interop alone.
- Corrected the deferral row's Status to `done` in commit A and left the shard
  removal to the next commit, over hand-writing a script that routes around the
  gate.

## Consequences

- A spec that sits open owes a re-measurement, not a re-read. Every number in a
  closure table is a claim about a tree that has since moved, and the cheap ones
  (`grep -c '{gap'`, a tag count, a scenario's pass rate) are the ones that rot
  silently.
- `review_gate.py record --spec <name>` takes an arbitrary string. Nothing warns
  that the name is not the spec stem, and the mismatch surfaces only at
  `commit_helper.py create`, which is the last step before committing.
- A deferral row's Status must be one of `done`, `cancelled`, `resolved`.
  `deferral_shard_removal_problems` reads the shard from HEAD, so correcting a
  non-vocabulary status and removing the shard cannot happen in one prepared run:
  the correction has to land first.
- Three reviewers over one surface found defects in each other's output.
  Reviewer C's two ISSUEs were both in the FIX delta A and B produced, one of
  them a narrowing that the same change reintroduced into a sibling file.

## Gotchas

- **A `.ci` or `inject.msg` directive you only want to WAIT on is an assertion.**
  `expect=bgp:...KEEPALIVE` used as a hold-open device fails hard on any real
  message arriving first. Scenario 47 lost that race because FRR re-advertises
  the injector's own prefixes back through ze; the peer then exited and
  `check.py` failed on a missing route, pointing at nothing the scenario asserts.
  **This spec hit the same trap twice** -- its own Mistake Log records it for the
  `.ci` ("declaring the peers' End-of-RIB opts INTO the race") and nobody carried
  it across to the interop scripts. Use `option=linger:value=true`.
- Scenarios 52, 53 and 54 still use the trailing-KEEPALIVE idiom. They passed
  here, and none of them has a peer that re-advertises back through ze, which is
  what makes 47 the one that loses.
- Running `make ze-plugin-test` and `make ze-interop-test` at the same time in one
  tree reddens three unrelated `.ci` tests. Only `ze-verify*` takes the repo-wide
  lock; nothing stops these two from contending for CPU and docker.
- A new `<!-- source: -->` anchor in a doc regenerates `ai/CODE-TO-DOCS.md`. When
  that generated index is already dirty from another session, committing it
  carries their mappings for code that is not in HEAD. Naming the file and the
  symbol in prose satisfies `ai/rules/writing.md` without touching the index.
- An `rfc-test-change-approved:` marker is demanded by the write hook for ANY
  edit to an RFC-tagged test file, a docstring-only one included. The hook reads
  the replacement text alone, so a marker already at the top of the file does not
  count.

## Files

- `plan/deferrals/fixit-otc-src-role-meta-fallback.md` -- Status to `done`
- `test/plugin/relay-withdraw-reflector.ci` -- new, the verify-tier witness for
  the RFC 4456 producer
- `test/interop/scenarios/47-rfc7606-relay-shape-frr/inject.msg` -- `option=linger`
- `test/interop/scenarios/54-relay-withdraw-reflector-frr/check.py`, `ze.conf` --
  the FRR measurement narrowed to the sessions that took it
- `internal/component/bgp/plugins/role/otc.go`,
  `internal/component/bgp/wireu/aspath_slot.go`,
  `internal/component/bgp/wireu/aspath_slot_test.go`,
  `internal/test/peer/message.go` -- comments and one RFC quote corrected
- `docs/features/rfc-status.md` -- the RFC 9234 coverage gap closed
- `ai/RFC-REQUIREMENTS.md` -- regenerated for the shifted tag lines
