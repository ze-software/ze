# Handover: RFC 9774 AS_SET deprecation (spec-bgp-rfc9774-as-set-deprecation)

Written 2026-10-18 by the ze-implement agent of session 20123ea5, at its tool budget.
Spec: `plan/spec-bgp-rfc9774-as-set-deprecation.md` (status in-progress, Phase 1/5,
claimed by session 20123ea5 with owner-authorised `ZE_SPEC_WIP_CAP=27`).

## State: code and unit tests written, NOT committed

The work is in the working tree only. It could not be committed because two of the
files it edits carry another session's uncommitted link-local next-hop hunks
(draft-ietf-idr-linklocal-capability) that call `validateLinkLocalNextHopPair` in the
untracked `internal/component/bgp/message/rfc7606_linklocal.go`. Judged against HEAD
those hunks do not build, so carrying them is unsafe (`ai/rules/git-safety.md`), and
`./le commit create` takes whole files. The holder was editing during this session.
Commit this work once that session has committed its link-local hunks, or carry the
file if its referent is then in HEAD.

| File | Mine | Foreign hunks |
|------|------|---------------|
| `internal/component/bgp/message/rfc9774.go` (new) | `ASPathSetSegment` (exported, framed-only scan), `asPathSetVerdict` | none |
| `internal/component/bgp/message/rfc9774_test.go` (new) | `TestValidateASPathRejectsSetSegments`, `TestValidateASPathStructureBeforeSet`, `TestValidateAS4PathRejectsSetSegments` | none |
| `internal/component/bgp/message/rfc7606.go` | `validateASPath`: set scan after the AS 0 scan; doc comment | YES: link-local (`ValidateUpdateRFC7606AddPath` gains `linkLocalNextHop`) |
| `internal/component/bgp/message/rfc7606_test.go` | `TestRFC7606ASPathValidSet` corrected (R-1): now asserts treat-as-withdraw with the RFC 9774 description; claim reworded, still RFC7606-7.2-1 positive | YES: hunk at line 112 |
| `internal/component/bgp/message/rfc7607.go` | `validateAS4PathAttr`: set scan first (outranks AS 0 discard); comments | check before commit |
| `internal/component/bgp/reactor/session_rfc9774.go` (new) | `transmittedPathHoldsSet`, `writeUpdateAsWithdrawal`, `announcedRouteCount`, `mpAddPathSend`, `mpAddPathSendWire` | none |
| `internal/component/bgp/reactor/session_write.go` | one guard in `writeUpdateBody`, before the AIGP guard, `!raw` | check before commit |
| `internal/component/bgp/reactor/session.go` | field `asSetSendLog wellKnownScanLog` | check before commit |
| `internal/component/bgp/reactor/reactor_metrics.go` | `asSetWithdrawn` counter `ze_bgp_as_set_withdrawn_total` (no label) | check before commit |
| `internal/component/bgp/reactor/rfc9774_receive_test.go` (new) | `TestRFC9774ReceiveASSetTreatAsWithdraw`, `...AS4PathSetTreatAsWithdraw`, `...ReannounceWithdraws`, `...SetNotCoalesced` | none |
| `internal/component/bgp/reactor/rfc9774_send_test.go` (new) | `TestRFC9774SendASSetWrittenAsWithdrawal`, `TestRFC9774SendMPASSetWrittenAsWithdrawal`, `TestRFC9774SendRawUntouched`, `BenchmarkWriteUpdateBodyCleanPath` | none |
| `docs/architecture/wire/attributes.md`, `docs/architecture/wire/messages.md`, `docs/guide/monitoring.md`, `docs/guide/rpki.md` | RFC 9774 receive/send behavior, counter, rpki line rewritten | `docs/features/bgp-protocol.md` diff is NOT mine |

## Evidence (scoped, this session)

- `go test -race` over `./internal/component/bgp/message/` and `./internal/component/bgp/reactor/` (full packages, with the receive scan in place): both `ok`. Then `-run RFC9774` over both, with send tests: `ok`.
- Red: `ASPathSetSegment` forced to answer false: `TestRFC7606ASPathValidSet` and all seven `TestRFC9774*` reactor tests went red; restored, green.
- Benchmark under `-race`: `BenchmarkWriteUpdateBodyCleanPath` 0 B/op, 0 allocs/op.
- AC-1 log line observed: `event=treat-as-withdraw attr=2 attr-name=AS_PATH description="RFC 9774 Section 3: AS_SET in AS_PATH"`; AS4_PATH form with `attr=17`.
- Send warning observed: `route withdrawn instead of advertised: RFC 9774 Section 3 forbids ...; remove the set segment ...` with `peer` and `routes`.

## Remaining (all still in scope; nothing was dropped)

1. Commit phases 2-3 (above), once the foreign link-local hunks are in HEAD.
2. Send-guard-only red run: remove the guard call in `writeUpdateBody`, confirm only the send tests go red.
3. A-1 (no aggregation; grep), A-4 (ADD-PATH send framing: no test yet, add one to `rfc9774_send_test.go` with ADD-PATH send negotiated), A-5/A-6 (FRR knob, at interop) are still `unvalidated` in the spec.
4. Counter assertion for AC-9 (`ze_bgp_as_set_withdrawn_total` by route count): unit tests ran with nil metrics; assert it in the `.ci` or with a metrics registry.
5. Functional tests: `test/plugin/rfc9774-as-set-receive-treat-as-withdraw.ci`, `test/encode/rfc9774-as-set-not-advertised.ci`, `test/plugin/rfc9774-as-set-api-not-advertised.ci` (Wiring Test rows 4 and 5, AC-9, AC-11).
6. Enrolment through `/ze-rfc`: `rfc/short/rfc9774.md`, extraction sign-off, then `RFC requirement:` tags WITH polarity (`RFC requirement: RFC9774-3-N positive|negative -- note`) and `./le rfc discriminate-record` per tag. Do NOT add a tag before the summary exists: an unpolarised tag made the Bash hook refuse every command for every session.
7. R-2 notes in `rfc/short/rfc4271.md`, `rfc5065.md`, `rfc6793.md`; regenerate rfc-status; `features/bgp-protocol.md` evidence; `docs/comparison.md` check (Documentation row 11).
8. Interop `test/interop/scenarios/bgp-rfc9774-as-set-frr/`, red with each half reverted, recorded.
9. Owed gates: `./le go lint run`, `./le test unit all`, `./le test functional gating`. `rfc/audit/rfc7606.json` holds a body hash for `TestRFC7606ASPathValidSet`, which the R-1 correction changed.
10. Handoff is `verify`: final commit at status `verification`, then `./le spec release`.

## Continuation 2026-10-18 (second ze-implement agent, session 20123ea5, at budget)

Still NOT committed: `git grep -n validateLinkLocalNextHopPair HEAD -- internal` was empty at
the end of this run. The link-local hunks span more than the two message files:
`reactor/session_validation_receive.go` calls the 6-argument `ValidateUpdateRFC7606AddPath`,
so a HEAD-plus-mine build must keep the whole `internal/component/bgp` working tree.

Done this run (working tree only):

| Item | Result | Evidence |
|------|--------|----------|
| 2. Send-guard-only red | done | guard forced off by `go test -overlay`: only `TestRFC9774Send*` red (6 subtests + MP + ADD-PATH); every `TestRFC9774Receive*` green |
| 3. A-1 | confirmed, with a finding | no aggregation code; but a configured raw `attribute "0x02 0x40 ..."` is written BESIDE the typed AS_PATH (two AS_PATH attributes). Journal row added to `plan/journal/wire-carries-both-arms-of-an-either-or.md` (uncommitted) |
| 3. A-4 | confirmed | `TestRFC9774SendAddPathFraming` (IPv4, IPv6 unicast, IPv4 labeled unicast). Fix: `mpAddPathSend` in `session_rfc9774.go` reads `bgpctx.Registry.Get(s.sendCtxID).AddPath`, the context the splitter uses. Red: forced false, labeled case red only |
| Send guard | changed | `transmittedPathHoldsSet` now walks every attribute with `attribute.NewAttrIterator` and judges every AS_PATH/AS4_PATH occurrence (the duplicate above otherwise slipped a set through). New case "AS_SET in a second AS_PATH". Bench still 0 B/op, 0 allocs/op |
| 4. AC-9 counter | done | never-held subtests use `newSpyRegistry` + `initReactorMetrics`, two prefixes, assert `ze_bgp_as_set_withdrawn_total` == 2 |
| 5a. `test/encode/rfc9774-as-set-not-advertised.ci` | written, green, red proven | expects clean UPDATE, then a withdrawal of 10.0.1.0/24 (static-route rail is not ownership-tracked, so a never-held withdrawal is written; it announces nothing), then EOR. Red with guard reverted: seq=2 receives the set announcement |
| Spec | A-1, A-3, A-4 rows updated | |

How the .ci was run (the tree does not build: another session's DHCP and iface work):
build ze with `go build -overlay` mapping every dirty non-bgp `.go` file to its HEAD content
and every untracked one to "" (`-o bin/rfc9774-20123ea5/ov/ze`, tags `ze_core ze_distro
ze_setup zetest` + feature-gates), build le with tags `ze_le` + feature-gates, then
`PATH=bin/rfc9774-20123ea5:$PATH LE_TEST_NO_BUILD=1 ZE_BIN=.../ov/ze bin/rfc9774-20123ea5/le
test bgp encode --pattern rfc9774 -v`. `bin/rfc9774-20123ea5/` is scratch; delete it when done.

Still open (nothing dropped): 5b `test/plugin/rfc9774-as-set-receive-treat-as-withdraw.ci`,
5c `test/plugin/rfc9774-as-set-api-not-advertised.ci`, items 6, 7, 8, 9, 10 above, A-5/A-6,
the page check for the "every occurrence" guard wording in `docs/architecture/wire/attributes.md`,
and the commit. Owed scoped re-run after any edit: `go test -race -run RFC9774` over
`./internal/component/bgp/message/` and `./internal/component/bgp/reactor/` (reactor green at
the end of this run; message not re-run, unchanged since the first run).
