### `reload` suite -- four config-transaction tests fail only under parallel load

Observed 2026-07-22 (rootless Linux sandbox, 20-way parallel `ze-test bgp reload`).

| id | test | isolation | full suite |
|----|------|-----------|-----------|
| 2 | `commit-transactional` | fails | fails |
| 3 | `commit-verify-reject` | **PASS** | fails |
| 11 | `reload-rapid-sighup` | not isolated | fails |
| 34 | `test-tx-ipsec-eap-tls-requires-ca` | **PASS 3/3** | fails |

Tests 3 and 34 pass deterministically on their own and fail in the full run, so
this is load sensitivity in the reload/config-transaction path, not a product
regression in any of them. Test 2 fails in isolation too and is tracked
separately in `reload-commit-transactional-and-apply-ordering.md`.

**How test 34 was attributed.** It is new in this session. It was
mutation-verified when written (no-op the ike `OnConfigVerify` body, rebuild, the
test flips red; restore, green). It later failed while three review subagents
were saturating the CPU, and passes 3/3 once they finish. Instrumenting the
verify handler during a failing run showed the callback was **never entered** and
the daemon reported `plugin ike verify failed: plugin connection closed
(crashed?)` -- i.e. the plugin's RPC connection is gone before verify is
dispatched, which is a transaction-layer timing problem, not a fault in the
validator under test.

**Next step.** Per `ai/rules/flaky-under-load.md` the tool for this is
`scripts/dev/stress-repro.py reload`, which recreates the pressure cheaply and
captures the first failure's complete log. That has NOT been run. The shared
signature to look for is what closes the plugin connection between SIGHUP and
the verify dispatch: all four tests drive a config transaction, and the two that
pass alone both fail the same way.
