# Handover 17: ExaBGP 6 Phase 7 and its spin-off fixes, continue on Linux (2026-10-22)

Written on the Mac at the end of session 8385ff13 for the next session on the
Linux host. The long running log is `03-session-2026-10-18-linux-continue.md`
(its later sections carry per-agent detail); this file is the entry point.

Claimed spec: `plan/immediate/spec-exabgp-6-api-compatibility.md` (Phase 7 in
progress). Claim it again on Linux with `./le spec claim spec <file>`.

## Owner decisions still open (ask, do not act)

| # | Decision | Context |
|---|----------|---------|
| 1 | Cap the Go build-cache size, enforced on every `./le` run? | The Mac disk filled three times on 2026-10-21/22: the checkout, shared and ambient Go caches regrow to about 85 GB within hours with many sessions building. Safe `cache-clean` (803356e056) cannot free young entries by design |
| 2 | Shared-build launcher for `bin/` (content-addressed, hard-linked `le-<name>`)? | 77 `le-<name>` launchers of 137 MB each were mostly identical builds. The 24 h idle trim keeps them because each was used that day |

Owner decisions taken this session are in `ai/META-DECISIONS.md`: M11 (third-party
module changes are local diffs, never a fork) and M12 (full YANG coverage runs
after the initial release).

## Landed this session (main, not pushed by this session; origin moved once)

| Area | Commits |
|------|---------|
| Teardown `request peer notify` and bridge teardown octets as hex, `ze:rest-of-line` | ce28145f2f |
| Bridge route-line escapes, merged lists, last NLRI field, SRv6 0xFFFF | 1cfb359fa1 |
| Aggregator bracketed form | a6c1437035 |
| Label, bgp-prefix-sid, origin/mup forms, end-of-line watchdog | add923ae32 |
| MUP DSD/T2ST address field; ParseRouteArgs deleted; ADD-PATH withdraw-only keeps path ids | 371519fe0e, 2fcb0b6519, 27dbbbd0a6 |
| rr/rs key ADD-PATH unicast by prefix and Path Identifier | 751bdc13d7 |
| MP NLRI split errors rendered; one JSON key per family | 15bd174272, d2682e5b19 |
| rr departure as recovery (ze's Path Identifier, GR retention, interop `bgp-addpath-departure-reflector-frr`) | c379d157b1 |
| rr/rs relay peer-up replay and revalidation themselves (owner-relayed pages, no grant borrowing) | cb65125a65 |
| Generic `attribute [ code flags value ]` in update text, config, bridge, migration (RFC 4271 4.3 flags, typed decode) | 6399061087 |
| goyang fork replaced by upstream + vendor patches (M11); grouped-action fix | c07197a98a, a639bb22e9 |
| Safe `cache-clean` (age floor; `all` refuses under builds) | 803356e056 |
| config-yang-reference-types deferred (M12) | 1577888acb |

## Remaining work, in order

1. **ExaBGP Phase 7, old-command corpus (AC-37/X-35).** `TestOldCommands50` is
   4045/4250 and `TestOldCommands42` 2478/2541 (`internal/exabgp/bridge`,
   `-tags ze_bgp`); both stay red until every class closes. Left: IPv6 flow
   `/N/M` offsets (~24), flow components with no value and `mark`/`action`
   (~24), mpls-vpn `split /N`, mcast-vpn (~34), ipv6 multicast (~64), ipv6 mup
   (~19), unicast withdraw differences, extended community `target4`/`origin4`/
   type 0, vpls withdraw with attributes, mup DSD with `next-hop` twice,
   `eor l2vpn vpls`, `route-refresh l2vpn evpn`, `bgp-ls`, and the
   route-refresh/operational judge branch.
2. **AC-38: `TestOldResponses50/42`.** Port the pin's `qa/bin/test_old_responses`
   and its ALLOWED table over `internal/exabgp/bridge/testdata/exabgp-old/responses-*.json`.
3. **End-to-end RPKI revalidation `.ci` through rr/rs** (owed by cb65125a65's
   journal row, top of `plan/journal/silent-fall-through.md`). Written, never
   run: observer `plugin15RPKIRevalidateRelay` in
   `internal/test/fixture/plugin_fixture_15_rpki.go` (registered in
   `plugin_fixture_15.go`), `test/plugin/bgp-rr-rpki-revalidate-relay.ci` and
   `bgp-rs-rpki-revalidate-relay.ci`, uncommitted on the Mac and carried as
   `plan/handover/17-revalidate-relay-wip.patch` (`git apply` it on Linux;
   delete the patch file in the commit that lands the tests). The rr seq=2 hex is a
   placeholder to pin from the first green run. Design and open risks: the
   "2026-10-22 main session: e2e revalidation .ci" section of handover 03.
4. **Defect (journal, open):** plugin-family routes (flow, sr-policy) drop
   parsed config attributes in `buildConfigRouteRequest`
   (`plan/journal/config-route-request-drops-parsed-attributes.md`).
5. **Defect to confirm then journal and fix:** while a route is held
   ineligible by RPKI, a destination receives repeated withdrawals of a prefix
   never advertised to it, one per gate event (`forward_validation.go`
   `RouteEligible`, rr/rs ValidationChange coalescing).
6. **Small:** `cache-clean` JSON keys `free_before`/`free_after` are snake_case
   against `ai/rules/cli.md`.
7. **Then:** ExaBGP Phase 8, spec review, close the spec.
8. **rfc-verdict-fix-bgp:** blocked on a Linux run for the RFC4271-5.1.2-3
   interop records; Linux is now available.
9. **Push:** refused while verification debt is open; a green full verify is
   owed first.

## How to work on Linux

- ExaBGP pin run: `PYTHONPATH= uv run --no-project --python 3.12 --with paramiko
  --with 'exabgp @ git+https://github.com/Exa-Networks/exabgp@685f772e4ede5c818d2ff5c84f4ae040afd37be6'
  ./le --name <n> test exabgp <population> --dut exabgp <case>`.
- 4.2/5.0 sources: `git -C ~/Code/github.com/Exa-Networks/exabgp/main show <tag>:<path>` (4.2.25, 5.0.1).
- Interop runs natively on Linux (the Mac needed the QEMU docker lab with
  `ZE_QEMU_SSH_PORT=2290`).
- A new fixture or `./le` change is not in the shared `bin/le` until
  `./le --update`; agents used `./le --name <x>` and must delete
  `bin/le-<x>` afterwards.
- The staticcheck parts 2/3 red recorded on the Mac was an environment cache
  fault (dead patched GOROOT after a Homebrew Go upgrade, cache entries removed
  mid-run). A green full verify on Linux should clear the carried structural reds.
- Plugin packages need the full `ze_*` tag set from `feature-gates.txt`;
  `-tags ze_bgp` alone fails OSPF/NTP inventory tests that are not real reds.

## Traps met this session

- Agents run out at 100 tool calls; brief each to write its handoff here and
  commit it by about call 85.
- Never pipe `./le commit create` through `tail`: the `script=` line is lost.
- Journal rows are capped at 600 characters and the Fix cell must start
  `fixed: ` or `open: `.
- The shared `.git/index` can lag HEAD after a commit whose index refresh met
  another session's lock; commits use a private index, so do not "fix" it with
  a reset. `git read-tree HEAD` is the owner's call.
- An RFC-tagged test change needs the owner's `./le rfc approve`; after the
  fact, record it with `./le commit debt-discharge ... kind owner`.

## Agent unicast (2026-10-22 Linux)

Out of budget before testing or committing; all edits uncommitted, `go vet`
clean, no test run on them.

| Fix | Release behaviour | Edit |
|-----|-------------------|------|
| trailing `withdraw` flag | API routes go through `add_to_rib`, never the watchdog path; the flag is the internal INTERNAL_WITHDRAW attribute, never packed. All 15 corpus lines were announced | `bridge_attribute.go` reads and ignores it, `routeAttributes.Withdraw` deleted, verb flip removed from `bridge_command.go`, `bridge_route_forms.go`, `bridge_vpls.go`; `bridge_vpls_test.go` case now expects `add` |
| `split /N` in `announce attribute(s) ... nlri` | 5.0 cuts each prefix into /N pieces | `splitPrefixes` in `bridge_route_forms.go`, used by `convertAttributesForm` and `buildRouteCommand` |
| IPv6 `next-hop self` (~45 rows) | 5.0's recorder packed 0.0.0.0 in an AFI 2 MP_REACH, malformed under RFC 2545 Section 3 / RFC 7606; the pin's judge reads them UNREADABLE (4250/4250 at the pin) | `oldSessionReset` in `old_commands_test.go` runs `message.ValidateUpdateRFC7606AddPath` before decoding |

Next, in order: fix the `withdraw` sentence in `docs/architecture/exabgp-bridge.md`
(~line 70) and check `docs/exabgp/exabgp-differences.md`; run the corpus tests
and the scoped `-race` run; then the untouched rows: `family-allowed in-open`
(needs the session's multi-session fact, `selectorExcludes` in
`bridge_selector.go`, `api-multisession.ci` depends on it), extended community
`target4`/`origin4`, `as-path [ ... ] ( ... )` AS_SET, trailing backslash on
`announce route ... \`. Journal row owed: update text answers "watchdog not yet
implemented" while the judge's `ParseUpdateText` accepts the watchdog form.
