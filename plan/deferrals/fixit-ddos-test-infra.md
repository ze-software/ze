# Deferrals: fixit-ddos-test-infra

Deferral rows for this source. The aggregate live backlog is folded on
read from `plan/deferrals/` by `/ze-status`; nothing stores it (`ai/rules/deferral-tracking.md`).

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-07-16 | spec-fixit-ddos-test-infra (Design Insights, found while validating A-4) | `test/pppoe/` is ORPHANED DEAD CODE: its 3 `.ci` files use `option=netns:veth=`, which `parseOption` rejects as `unknown option type` (`internal/test/runner/record_parse.go:428-430`), and no suite registers pppoe (`internal/test/cli/register.go:17-36`). They cannot parse and nothing runs them, so the PPPoE Access feature row in `docs/features.md` has no functional test behind it | Found incidentally while researching an unrelated ddos test; fixing it is a distinct piece of work (decide whether to repair the option, re-mark the tests, register a suite, or delete them) and does not belong in the ddos spec | `plan/spec-finish-ci-coverage.md` (skeleton; the feature is `Partial` in `docs/features.md:88` and this is why. Records four options -- repair the directive, re-mark the tests, register a suite, delete them -- without choosing; the choice is the user's) | deferred |
| 2026-07-16 | spec-fixit-ddos-test-infra (AC-10 transit proof) | Proving packets are FORWARDED (not merely admitted) needs a receiver on the far veth end counting arrivals; the current shape can prove the drop but not the forward | Out of scope for the test's rework onto `wait_until`: it needs new test topology, not a wait-primitive change | `plan/spec-finish-ci-coverage.md` (AC-10, tracked in-spec) | deferred |
| 2026-07-19 | spec-fixit-ddos-test-infra functional-proof | QEMU run of the two .ci is the AC-1/4/5/6 proof, deferred to CI | live-server/QEMU constraint, deferred to CI | plan/spec-finish-appliance-qemu-evidence.md | deferred |

