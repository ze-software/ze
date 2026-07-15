# 1128 -- rib-arch-0 Umbrella: Closure and Cross-Cutting Lessons

## Context

The `spec-rib-arch-*` umbrella indexed 8 independent BGP-engine / RIB follow-ups. All are now
resolved: rib-arch-1 (store-vs-delta decision, 1120), 3 (RFC 5549 inject, 1121),
8 (NLRI rewrite, 1122), 6 (RS/RR fastpath consumer, 1123), 4 (ECMP-to-FIB realtime, 1124),
5 (RFC 9069 BMP Loc-RIB, 1125), 7 (LLGR readvertise wiring, 1126), 2 (binary Raw carrier, 1127).

## Decisions (umbrella-level)

- **"Test gap" / "refactor" framings were wrong twice.** rib-arch-7 was filed as "add a multi-peer
  `.ci`" but was a real unwired feature (the LLGR egress filter never ran on the readvertise rail);
  rib-arch-2 was filed as "length-prefixed binary" but the transport is line-JSON, so the real slice
  was a `[]byte` (base64) carrier. Re-verify the filed premise against `file:line` at design time --
  the umbrella's own split-verification note already flagged that anchors drift.
- **Two items were scoped DOWN with explicit user approval** rather than implemented as filed:
  rib-arch-2's "remove the 9-plugin text path" (ambiguous perf, high blast radius) narrowed to the
  raw carrier; rib-arch-7's live multi-peer BGP `.ci` (blocked on harness establishment) replaced by
  a deterministic reactor wire-output test. Both scope reductions are recorded in the respective
  learned summaries and were user-approved -- not unilateral.

## Consequences

- The BGP filter/forward hot paths gained: intra-source ECMP to the FIB (4), a production
  `Change.Forward` consumer (6), RFC 9069 Loc-RIB monitoring (5), the LLGR egress filter on the
  readvertise rail (7), and a binary raw filter carrier (2) -- each with the common path untouched.

## Gotchas (recurring across the set)

- **A core-package change pulls its whole reverse-dep closure into `ze-lint-changed`,** surfacing
  LATENT pre-existing breakage: rib-arch-4 (touching `locrib`) surfaced 6 stale reactor mocks +
  firewall lint; rib-arch-2 (touching `pkg/plugin/rpc`) surfaced netns/traffic test-infra lint.
  Budget a repair commit whenever a widely-imported package changes.
- **`tmp/ze-verify-failures.json` can disagree with a direct `make ze-lint-changed`** (a stale/partial
  stage record showed lint=0 while direct lint was red). Trust the direct gate; re-run the
  orchestrator to refresh the json before relying on it for a commit.
- **The commit_helper blocks deferral language** ("deferred to", "out of scope") in staged spec/learned
  text unless there is a `plan/deferrals.md` row; reword to factual ("remains a tracked follow-up").
- **The spec validator requires the Functional Tests section to name `.ci` files** and the literal
  `make ze-test` checklist line -- a Go test is not accepted as a `.ci` substitute in that section.

## Files

- Children closed via two-commit closure each; learned summaries 1120-1127.
- This summary + the umbrella spec removal close `spec-rib-arch-0-umbrella.md`.
