# Handover: fixes from the review of the 2026-11-22 unpushed commits

Thomas asked for a review of the 28 unpushed commits (79c6a40ecb..808ee06cfb),
then ordered the findings fixed. No spec was opened: the fixes were ordered
directly. Nothing is pushed.

## Landed

| Fix | Commit |
|-----|--------|
| LLGR stale routes across a second restart (RFC 9494 4.2) | 26d70dfa0d |
| Config transaction: rollback ends on shutdown, commit has its own bound | 47dc081099 |
| Website: assertions 770170950a changed are restored | 2c7795a098 |
| MCP: tool scope decided on the command line that runs | 4de9d23137 |
| Rules, skills and gates after the rules split | dc9a814656 |
| MRT dump subtype follows what the route is (RFC 8050) | 381e927417 |

## Written and tested, not committed

Each is in the working tree. Tests named here passed in the main checkout.

### 1. NOTIFICATION size capped by the peer's capability (RFC 8654) and three RFC rows

- Files: `internal/component/bgp/reactor/session_write.go`,
  `reactor/rfc8654_notification_limit_test.go` (new),
  `reactor/doctor_codes.go` and `reactor/register_diagnostic.go` (one hunk
  each: the new code `doctor-bgp-notification-data-cut`),
  `internal/component/bgp/cli/rfc8955_decode_test.go`,
  `internal/component/bgp/plugins/role/rfc9234_complex_peering_test.go`,
  `rfc/short/rfc9234.md`, `rfc/short/rfc4360.md`,
  `rfc/discrimination/rfc8654/RFC8654-5-3.json`,
  `rfc/discrimination/rfc8955/RFC8955-7.1-2.json`,
  `docs/architecture/edge-cases/extended-message.md`.
  Remove with the commit: `internal/component/bgp/message/rfc8654_notification_limit_test.go`,
  `rfc/discrimination/rfc9234/RFC9234-6-1.json`.
- Evidence: reactor notification tests, role and traffic-rate tests pass;
  four proof records taken. Re-run
  `./le test unit pkg path ./internal/component/bgp/reactor/ run 'TestRFC8654NotificationToPeerWithoutExtendedMessage|Diagnostic|Doctor'`
  after the doctor-code edit.
- Body: `tmp/session/2026-11-22-063817/scratch/msg-rfc.txt`.
- Blocked: `reactor/doctor_codes.go` is another session's untracked file (see
  "The blocker"). Commit once that refactor lands.
- The RFC9234-6-1 gap text cites "owner ruling 2026-11-22", which the coverage
  ratchet (`ownerRuledMove` in `internal/le/rfc/coverage_ruling.go`) requires
  for a row leaving "proven". The ruling it names is Thomas's order to fix the
  review findings. Thomas has not confirmed that wording.

### 2. gNMI offline checks, IPv6 listener addresses, per-user web sessions

- Files: `internal/core/network/exposure.go` and `exposure_test.go` (new),
  `cmd/ze/hub/{mgmt_guard.go,mgmt_guard_test.go,listener_migrate.go,service_mcp.go,api.go,gnmi_infra.go,gnmi_infra_test.go,service_gnmi.go,aaa_lifecycle.go,web_session_generation_test.go,rfc8907_aaa_lifecycle_test.go}`,
  `internal/component/config/{loader_extract.go,validate_semantic.go,validate_semantic_test.go,gnmi_validate_test.go}`,
  `internal/component/config/cli/{cmd_validate.go,cmd_validate_gnmi_test.go}`,
  `internal/component/gnmi/{server.go,set.go,doctor.go,doctor_test.go,auth_refusal_test.go}`,
  `internal/component/aaa/types.go`, `internal/component/authz/auth.go`,
  `internal/component/ssh/pubkey.go`,
  `internal/component/web/{auth.go,auth_session_revocation_test.go}`,
  docs: `docs/guide/authentication.md`, `docs/features/mcp-integration.md`,
  `docs/guide/web-interface.md`, `docs/architecture/web-interface.md`,
  `docs/comparison.md`, `docs/features.md`.
- Evidence: `internal/core/network`, config (GNMI, Semantic, ServerEndpoint),
  config/cli, gnmi (whole package), web/aaa/authz session and credential
  tests pass. The hub selection passes except two tests that already failed
  at 02:05 in another session's log
  (`TestConfiguredPluginConformanceRejectedReloadRetainsAcceptedTarget`,
  `TestDoReloadRadiusRemovalLateBindFailurePreservesAccounting`).
- Owed: promote `test/draft/ui/doctor-gnmi-mcp-exposure.ci` over
  `test/ui/doctor-gnmi-mcp-exposure.ci` after
  `./le test ui --draft --pattern doctor-gnmi-mcp-exposure`; run
  `./le test functional plugin --pattern mgmt-guard`.
- Judge approvals recorded in `tmp/commit-weakening-approved-1888df8c.md`:
  three config tests and `ui.doctor-gnmi-mcp-exposure`.
- Blocked: almost every hub, aaa, ssh and web file here also carries the other
  session's doctor-code hunks.

### 3. NTP and DDoS detect answer a removal within its grace

- Files: `internal/plugins/ntp/{ntp.go,register.go,bye_test.go}`,
  `internal/plugins/ddos/detect/{detector.go,register.go,bye_test.go}`,
  `docs/architecture/ddos/ddos-detect-enhancements.md`.
- Evidence: both packages pass.
- Blocked: `ddos/detect/register.go` carries a hunk that needs the other
  session's untracked `doctor_codes.go`.

## The blocker

Another session is moving doctor codes out of `internal/core/diagnostic` into
`doctor_codes.go` files in the owning packages. Those files are untracked and
its hunks sit in the same files as all three fixes. Committing those fixes by
whole file would carry that unfinished refactor; committing by hunk cannot be
compile-checked without a clean copy. Once the refactor is committed, each fix
commits by whole file. `tmp/session/2026-11-22-063817/scratch/keephunks.py`
builds a patch that drops hunks whose changed lines are all doctor-code moves,
for `./le commit create ... hunks <path> <patch>`; it was used for 47dc081099.

## Owed across all fixes

- `./le go lint run scope "<pkg>"` was never run on any changed package.
- `./le rfc check` after the last RFC commit.
- `./le site build` and `./le site check` to regenerate `../gh-pages` for
  2c7795a098.
- `./le doc check all` for the moved source anchors.
- Approval rows of landed commits may still sit in the two
  `tmp/commit-*-approved-1888df8c.md` files (the cleanup failed three times on
  a full disk and on the broken build): `gr.TestRFC9494ConsecutiveRestartPreservesLLGRStaleRoutes`,
  `site.TestUpdateCoverageReachesEverySurface`,
  `mcp.TestBearerListAuthenticator_RequiredScopes`. Each failed script printed
  the `./le --name commit-approval-1888df8c commit approval clean ...` line.

## Found and not fixed

Each needs a design of its own; none has a journal row yet.

| Item | Where |
|------|-------|
| RFC 9494 4.2 counts a family as synchronized when the Selection_Deferral_Timer expires; Ze has no such timer | gr plugin; rfc4724 gaps 4.1-5, 4.1-8 |
| A commit-driven reload is not cancelled at daemon stop | hub, `commitReloads.close()` |
| Labeled unicast (SAFI 4) is dumped as RIB_GENERIC with the label stripped | rib storage side-data, mrt |
| The break-glass recovery grant is still revoked by any user's credential change | aaa, `rfc8907_*` tests |
| `checkKeyFilePermissions` refuses every symlinked key file under root | breaks secret mounts made of symlinks |
| Plaintext remote MCP with auth disables MCP with a warning; gNMI refuses to boot | boot guard |
| The refusal-advice commit gate matches message wording | `CheckRefusalAdvice` in `internal/le/commit/refusal_advice.go` |
| rib burst tests exceed fixed 30 s and 60 s deadlines under the race detector | `sdkEventEmitTimeout` in `rib_event_sdk.go`; the 60 s context in `TestDisableValidationBurstRecoversActualSelection` |
| Negatives of RFC4724-3-5 and RFC4360-6-1 miss their clause's condition | need selection deferral, a looping-policy refusal |

## For Thomas

| Question | What each answer changes |
|----------|--------------------------|
| Is "owner ruling 2026-11-22" the right citation for RFC9234-6-1 returning to a gap? | Another wording changes `rfc/short/rfc9234.md` and the approval row together |
| Did you rule the "document-author" not-applicable form (RFC7606-8-1, RFC5492-4-3)? | It is recorded in the spec and the gates page, not in `ai/META-DECISIONS.md` |
| May a subagent run `gopls check`? | The ze-work definition says yes; META-DECISIONS says subagents run no code |
| Which VPP throughput statement is true? | The performance page says throughput backs the guide's numbers; 770170950a had said this lab does not benchmark it |
| The two update-report summaries exceed the 108-character budget | 2026-10-12 (about 165) and 2026-10-25 (about 152) are in your wording, for you to shorten |
| Nine other statements disagree between website pages | Listed in the website agent's report; the earlier text is restored |
