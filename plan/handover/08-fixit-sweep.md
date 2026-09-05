# Handoff: the fixit sweep of 2026-08-19

**Spec:** none claimed. This session swept `plan/spec-fixit-*` rather than driving one spec.
**Branch:** main
**Goal:** review every fixit spec, fix each real defect at its source, move genuine
improvements to `plan/future/`, and reduce the count of open release specs.

## What a reader needs to know first

**The count barely moved and that is the honest headline.** `plan/spec-fixit-*` went
30 to 29. Nine defects were fixed at source and committed, four of them RFC MUST or
route loss, and three reds at HEAD were cleared. But the mechanism that reduces the
COUNT is closure, and closure ran once. Twenty-two fixit specs are complete or nearly
complete and unclosed. That is the work, and it is mostly not implementation.

## Status

### Done, committed
| SHA | What |
|-----|------|
| `8f0d02e81` | RFC 4724 Section 4 MUST: a peer with no `family` block sent no End-of-RIB. `Negotiate` (`internal/core/bgp/capability/negotiated.go`) now reads a side that advertised no Multiprotocol capability as advertising ipv4/unicast. Cleared a red at HEAD |
| `c299dea31` | RFC 7911 Section 5: `forwardUpdateCore` (`internal/component/bgp/reactor/reactor_api_forward.go`) rebuilt the wire and dropped the source id, so every source's paths keyed under one source. Route loss on the LIVE rail. Found while doing something else |
| `b60d8b92a` | RFC 7911 Section 3: `buildRelayUpdate` refused every ADD-PATH source, so a peer-up replay delivered nothing. Path id now carried on `RawRoute` and `rpc.StoredRoute` with a three-valued framing marker |
| `c5aac686c` | RFC 7296 Section 2.9 MUST: `respondChildRekey` answered a TS set it did not install when a rekey carried no TS payloads |
| `9a4d490d5` | `ApplyAll` (`internal/component/firewall/registry.go`) never recorded what it applied, so `show firewall ruleset` was blind to every plugin-owned table |
| `c6cfb80bd` | `CreateDummy` (`internal/plugins/iface/vpp/ifacevpp.go`) leaked one VPP loopback per apply; `recordMirror` kept a dead index |
| `7f7b74995` | A forced IRR refresh was answered from a one-hour cache and never reached the server |
| `12bc0e9a2` | DNS reply write error discarded; three false claims in the public RFC 1035 ledger corrected |
| `ea1f331da` | `review_gate.py record` wrote findings it never checked against the spec it named |
| `6f3c26068` | IPsec interop lab: one shared CLI-account render, and a snapshot reader that fails closed |
| `c08237575` | Two assertions that could no longer fire. Cleared two more reds at HEAD |
| `6bbfac0fa` | Two problem classes journalled; two specs moved to `plan/future/` |

Closed by the two-commit rule: `c0774fd8a` plus `c0f490baf`, `fixit-appliance-cert-replace-unvalidated`.

### In progress, uncommitted
Three journal rows, held deliberately. Each names a spec, so `commit_helper.py` reads
it as a closure signal and demands a review artifact. They belong WITH their spec's
closure commit, not before it.

- `plan/journal/gate-excludes-part-of-its-population.md`
- `plan/journal/helper-bypassed-by-an-open-coded-copy.md`
- `plan/journal/identifier-reused-after-its-owner-is-gone.md`

A full `./le verify current mode full` started 2026-08-19T13:16:07Z was still running at
handover. Its result rewrites `tmp/ze-verify-failures.json`, which every later commit
is judged against. Read it before assuming a red is yours.

### Remaining, in the order it is worth doing
1. **Closures.** Twenty-two fixit specs sit at `in-progress` with their code landed.
   `/ze-close` is one spec at a time by design. This is the only thing that reduces
   the release count.
2. **Four more VPP leak sites.** GRE, IPIP, VXLAN and WireGuard each send their create
   unconditionally, rebind the name over the old index, and record a deleter closing
   over a stale `SwIfIndex` — the identical shape fixed for loopback in `c6cfb80bd`.
   `ai/rules/completion.md` puts a sibling path with the same defect in scope. The fix
   is proven and mechanical.
3. **Two untriaged fixit specs**, both appeared from other sessions during this one:
   `plugin-concurrency-is-pinned-to-a-ci-constant`, `shutdown-waits-out-a-deadlock`.
4. **`./le integration interop-ipsec`, full run.** Thirteen IPsec scenarios now start an
   SSH listener they did not before (`6f3c26068`). Only that suite proves it changed
   nothing, and it has not run. Do this before closing `fixit-ipsec-interop-cli-credentials`.
5. **`stored-route-relay-hardening` I-2, I-3, I-4, I-6.** Only I-1 landed. I-3 is a
   second route-loss shape: `replayOwned` (`internal/component/bgp/plugins/adj_rib_in/rib.go`)
   is process-global, so a peer given `state` to bgp-adj-rib-in but not bgp-rs is
   replayed by nobody.
6. **`child-rekey-answer-vs-installed-selectors` AC-4**: the xfrm `.ci` and the
   strongSwan narrowing interop scenario. `ai/rules/interop-and-goal-validation.md`
   requires the interop scenario before that spec may claim done.

### Deferred, and why
- `fixit-dns-rfc1035-conformance` stays in `plan/` although the owner ruled it out of
  scope on 2026-08-18. Moving it to `plan/future/` was ATTEMPTED and reverted: the
  citation baseline is shrink-only and keyed on the citer's path, so relocating a spec
  carrying 17 grandfathered rows reports 17 new pairs. Filed as
  `plan/spec-shrink-only-baseline-cannot-see-a-relocation.md`. Do not retry the
  move without fixing that first.
- Two owner decisions, neither blocking anything else:
  - **RFC 4724 ordering.** Does a route a plugin injects AFTER the initial dump belong
    before the End-of-RIB marker? The RFC orders the marker against the initial routing
    update and says nothing about later routes, so the claim in
    `spec-fixit-peer-pending-sync-settles-too-early` is derived, not literal. Live cost:
    `waitForAPISync` (`internal/component/bgp/reactor/peer.go`) burns a fixed 2s waiting
    for a signal only `bgp-rib` ever sends.
  - **vpp-slaac premise.** The spec forbids code until a QEMU test proves RAs never
    reach the LCP tap, and names `test/qemu/`, which does not exist.
- `RFC1035-4.2-1` (zone transfer) annotation is untouched and unresolved. Calling it
  `{not-applicable}` lowers what Ze owes, which `ai/rules/rfc-compliance.md` reserves
  to the owner. Recorded in `plan/deferrals/fixit-dns-rfc1035-conformance.md`.

## Files already handled, do not re-read to rediscover
- `internal/core/bgp/capability/negotiated.go` — `Negotiate`, the implicit-ipv4 default
- `internal/component/bgp/reactor/reactor_api_forward.go` — `forwardUpdateCore`, source id
- `internal/component/bgp/reactor/reactor_api_relay.go` — `buildRelayUpdate`, path id
- `internal/component/bgp/plugins/adj_rib_in/{rib.go,nlri_hex.go}` — storage framing
- `internal/component/ike/engine/rekey.go` — `respondChildRekey`, the TS guard
- `internal/component/firewall/{registry.go,accessor.go,engine.go}` — one applied-state writer
- `internal/plugins/iface/vpp/{ifacevpp.go,mirror.go}` — `CreateDummy`, `recordMirror`
- `internal/component/resolve/irr/{client.go,store/store.go}` — `RefreshPrefixes`
- `internal/core/dnsserver/{handler.go,metrics.go}` — `send`, the write-failure counter
- the retired `scripts/dev/review_gate.py` (current producer: `internal/le/spec/session/review.go`) — `cmd_record`, the three refusals

## The pattern this session found, which matters more than any single fix

**The cheapest route from red to green is to make the check unable to see the thing,
and it presents as a fix.** Four instances, none malicious, each locally reasonable:
a review artifact whose findings were about a different spec; a `.ci` whose failure
branch could no longer fire; a withdraw test asserting an absence its own mutant
satisfied; a test helper whose count fell because assertions moved, not vanished.

The practice: a gate going green must be explained by a change in the thing measured,
never by a change in what the gate can see. `ai/rules/interop-and-goal-validation.md`
already requires proving a test discriminates; nothing checks that it was proven.

Two classes are journalled with the general statement and no case detail:
`plan/journal/reference-checked-claim-unchecked.md` (17 rows, the strongest recurrence
signal in the corpus) and `plan/journal/concurrent-session-corruption.md`.

## Gate friction worth reporting to the owner, who is reviewing gates
- `commit_helper.py`'s structural-red refusal asserts those gates "never fail for flaky
  or environmental reasons". Both of this session's did: `./le verify lint run` lost golangci-lint's
  single-instance lock to a concurrent session, and `./le staticcheck-feature-matrix check`
  exceeded its 25-minute deadline on a loaded box. The message then reports a
  structurally broken tree that is not broken.
- `check_weakened_tests.py` cannot follow a `t.Helper()` extraction or a cross-file
  move, so it demanded four rows for changes that removed no assertion.
- Any `plan/spec-*.md` removal reads as a closure, so relocating a spec needs
  `--review-override`.

## Then
```
./le verify status check
```
FRESH means the 13:16Z run finished and its record is current; judge every red against
`tmp/ze-verify-failures.log` before charging it to your own work. Then pick one spec and
run `/ze-close` on it, and only one.
