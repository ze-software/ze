# 1107 -- test-coverage-gaps

## Context

Umbrella spec from the 2026-07-10 coverage audit (`tmp/test-audit/REPORT.md`),
implemented phase-by-phase across many sessions (W-1..W-9, AC-1..AC-15). Delivered:
runner `.ci` directives + ipsec/appliance/wire suites and the sleep ratchet (W-1/W-2,
`f0fb1c906`); W-3 unit tests + 2 gokrazy bug fixes + mrt/static-vpp/aigp (`d3988511c`,
`a9f201270`); telemetry collector fixture sweep + live `/proc` smoke + a 1024x memory
under-report fix (W-4, `f7df2d71b`..`2e2a8d339`); W-5 functional-gate misses through
real entry points (`02b6ac42d`..`bfd927cb5`); W-6 cmd/core unit deserts
(`6f7bd3b88`..`340fcb15f`); `FuzzFSMEventSequence` (W-7, `7887c3d11`); the fib/vpp +
iface/vpp four-file test reorganization (W-8, `e9b3e1828`); W-9 linux back-fill via
seams + `integration && linux` (`914bbcf67`..`2144d5a6b`). Closed with 5 documented
deferrals (below), each spun into a named fixit spec rather than weakened into a
passing test.

## Decisions

- **New `.ci` runner directives instead of shelling out** (W-1, AC-2): `command=`,
  `expect=output:contains=<needle>:timeout=N`, `expect=event:namespace=:name=`,
  `stream=` + `expect=stream:` drive a CLI command against the live daemon and assert
  its output / a daemon event / monitor-stream content. Parse them BEFORE the generic
  `:`-split so a colon-bearing needle survives (`parseEngineExpectContains`).
- **Two-daemon tests get per-daemon config files** (`a4af6be5c`): the runner keyed
  every config block to one `ze-bgp.conf`, so a second `ze` clobbered the first.
  `zeConfigFileName(rec, block)` gives the first block `ze-bgp.conf` and each further
  distinct block its own file -- the root-cause fix that made two-ze IKE establishment
  testable at all (the "establishment is broken" conclusion was WRONG; it was runner
  config clobber).
- **Strict-four VPP test files by concern** (W-8): both backends consolidated to
  `apply/translate/verify/register_test.go`. Because a package's symbols are already
  unique and there are no build tags, the split is a byte-exact re-bucket (extractor
  script, then `goimports`); the only additions are one backend-registration test per
  package (`registry.Lookup("fib-vpp")` / duplicate-probe on `iface.RegisterBackend`),
  both mutation-verified. Register tests belong in `register_test.go` even when the
  package (iface/vpp) also carries query/monitor/doctor concerns folded into apply.
- **Deferrals are fixit specs, not weakened tests** (`no-workarounds-for-missing-behavior`):
  every strict test that a real product gap blocked was recorded, not softened.

## Consequences

- The `.ci` directive set + ipsec/appliance/wire suites are registered
  (`internal/test/cli/register.go`) and gated (`mk/test-functional.mk`); QEMU fsuite
  list gains the new suites.
- Coverage back-filled across telemetry collectors, cmd/core deserts, VPP backends,
  and linux-only dataplane packages; `integration && linux` tests auto-enroll in QEMU
  via the derived package list (no Makefile edit).
- 5 deferrals remain open under their own specs; see Gotchas for the exact producers.

## Gotchas

- **Startup event subscriptions are namespace-locked to bgp**: `registerSubscriptions`
  -> `DefaultEventNamespace()` (`plugin/server/dispatch.go`), and delivered events
  carry no ns/name envelope (`payloadToJSON`, `dispatch.go`). So the ipsec
  `sa-up`/`child-up` event assertions and the `monitor vpn ipsec` stream test could
  not be written; the `.ci`s prove establishment via `show vpn ipsec sa`/`peer` gated
  on the negotiated aes-cbc algorithm instead. Owner: `spec-fixit-plugin-event-subscription.md`.
- **The `.ci` runner has no "stop a background daemon at step N" primitive**: its
  `Process.Kill()` calls are teardown/timeout only (`runner_exec.go`), so DPD
  liveness-teardown (fires only on peer death) cannot be observed end-to-end. DPD
  timer/probe logic is unit-tested (`engine/dpd_test.go`). Owner:
  `spec-fixit-runner-kill-background.md`.
- **`clear vpn ipsec sa` does not re-establish against a live responder**: the
  initiator re-sends IKE_SA_INIT, the responder holding the stale SA drops it (2 sent
  / 1 accepted). Owner: `spec-fixit-ipsec-clear-reestablish.md`.
- **static/vpp interface-only next-hops error rather than program an index-0 path**:
  address / recursive / blackhole / reject next-hops work; interface-only needs the
  iface/vpp name->sw_if_index resolver threaded into the static backend. Bounded
  follow-up, tracked in this spec's Design Insights.
- **Simultaneous IKE child rekey has a non-deterministic winner**: only the nonce
  winner increments `rekey-count`, so a symmetric-lifetime rekey test flakes. Use
  asymmetric lifetimes (initiator short, responder long) so the initiator always drives.
- **Hook session-id must come from `--session-id`, not argv0 or `$PPID`**: the Claude
  CLI's argv0 is the version path, so a `case */claude` / `base=="claude"` match never
  fires and falls back to an unstable pid that changes every hook subprocess. Both
  `.claude/hooks/lib/session-id.sh` and `.claude/hooks/pretool-writeedit.py` now walk
  the process tree for `--session-id`. A moving session id silently breaks every
  marker-file gate (LSP, session-state).

## Files

None recorded.
