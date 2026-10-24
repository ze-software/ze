# 24 Refuse-by-default: implemented, uncommitted, closure waits on one dependency

State at the end of session 6b51c217 (2026-10-24), which implemented
`plan/release/fixes/spec-refuse-instead-of-defaulting.md` (Status `in-progress`,
claimed by 6b51c217).

## What is true now

| Fact | Detail |
|------|--------|
| Implementation | Every phase ran (check tool, F1 to F4, report clean). 45 of 52 owned journal rows read `fixed: <evidence>`; each fix has a regression test seen red on a behavioural break |
| Commits | **None.** The whole diff sits uncommitted in the shared main checkout, beside other sessions' work. Closing soon matters: an uncommitted diff this size is the work most at risk |
| Map of the diff | The per-phase handoffs in `tmp/session/2026-10-23-6b51c217-dcf5-4a38-936b-93528baa43e4/state/session-state-refuse-instead-of-defaulting-6b51c217-dcf5-4a38-936b-93528baa43e4.md` list every file each phase changed. `tmp/` is not durable: read it before anything cleans it |
| Running at handover | Two agents: doctor fallback removal (owner-approved) and the structured rib graph (owner-approved). Their handoffs append to the state file above. Check both landed before closure |
| Open rows | 7: doctor `collectSchemaListeners` (being fixed) and six that need `spec-plugin-optional-dependency-refusal` (handover 23) |

## Order of the remaining work

1. **Handover 23 first**: `spec-plugin-optional-dependency-refusal` in its own session. Four of the six rows (`getConfigPathPlugins`, MRT/NTP not built, ddos/ExaBGP bridge/flow export undeclared BGP, redistribute/ls_export free strings) close there; in this spec's commit A they are re-homed or marked fixed by it.
2. **Then** in this spec: `cos.Resolve` and `appendDataPlaneWriter` call that spec's not-built refusal (red-first test each). After that `./le arch refuse-by-default report` must list none of the owned producers (AC-7).
3. **Then `/ze-close`** in a fresh session, on the review model.

## Closure checklist (beyond `/ze-close`'s own)

| Item | Detail |
|------|--------|
| Re-insert the doc section | `docs/architecture/testing/verify-freshness-scope.md`: put the section below back before `## Native stage execution`. It was moved out so 895e52ca and a10aef63 could land their paragraphs (e6653e86c9) |
| RFC verdicts | About 25 STALE verdicts on files the `RemoteAS` change touched (`rib_bestchange.go`, bmp tests, rfc8277/8050/6396 tests, reactor RFC tests, `peer_run.go`) need `/ze-rfc-audit` and `./le rfc audit-stamp ... mode rejudge`. One agent ran the unscoped `./le rfc reseal`, which re-stamped shifted verdicts corpus-wide: commit A carries only the `rfc/audit` and `rfc/discrimination` files for units this spec changed |
| Dead exports | `./le repo check` still flags 24 old exports in files this spec touched (13 tied to RFC record hashes, 11 called by other packages' tests). Out of scope: one journal row, owned by a fitting spec, and the commit body names the red it causes (`le-placement-answers`) |
| Test plan row to move | `test/plugin/show-bgp-peer-bare-list.ci` belongs to `spec-web-cli-config-one-off-defects.md` (AC-20, which owns that journal row); remove it from this spec's TDD plan |
| Owner approvals to record in the spec | 2026-10-24: delete doctor's fallback and `TestDoctorGnmiListenerIsProbed` (AC-19 of `spec-daemon-runtime-one-off-defects.md` covered here); the three rib-graph tests assert a structured payload |
| Foreign hunks | About 15 files hold this spec's renames next to other sessions' hunks. Judge each against HEAD per `ai/rules/git-safety.md` before carrying |
| Testing | No whole-tree testing on this machine (`ai/META-DECISIONS.md`, "Test selectively"): scoped package tests, named functional tests, scoped lint only. The full verify runs on another host |
| Pipeline parse-error row | Fix the 2026-10-24 `zero-value-as-valid-answer.md` row (`showPipeline`, `bestPipeline`) together with the two not-built rows (`cos.Resolve`, `appendDataPlaneWriter`): each pipeline's parse error returns a `pipelineRefusal`, so it answers an error status through the existing `commandAnswer` refusal path the graph change added (red-first test each) |

## Owed elsewhere (cannot run on this Mac)

- Linux: `pki-certificate-export-show`, `as112-dot-pki`, `geodns-dot-pki`; `./internal/component/host` (thermal); trafficusage `probeTCX` as root and non-root; l2tp Linux tests; `./internal/plugins/iface/dhcp` (`TestDHCPv6AddressesWithoutDADOutcomeNotReadAsUnique`, red by deleting the `DADReported` block in `checkDAD`); netlink `GetStats`.
- Ze-kernel Docker host: `INTEROP_SCENARIO=addpath-zero-id-frr ./le test integration interop`. It proves acceptance only; its red phase needs `nlri.WriteNLRI` mutated to drop the identifier octets for id 0. Also `bgp-addpath-best-path-pmacct` (the BMP Loc-RIB wire changed).
- `go test -tags docker ./internal/component/gnmi/` (gnmic with its default encoding).

## Contracts this spec changes

| Change | Who notices |
|--------|-------------|
| `pkg/plugin/rpc.RemoteAS` replaces `uint32` for `StructuredEvent.PeerAS`, `PeerInfo.PeerAS`, `Event.GetPeerASN` | in-process plugins; JSON omits `remote.as` while unknown, text says `remote as unknown`, ExaBGP bridge writes ExaBGP's auto `0` |
| `RawJSON.MarshalJSON` refuses a top-level JSON scalar | any command answering a bare string; a refused answer now writes a verdict-error terminator instead of hanging the caller |
| `health.RegisterWhen` | a check that does not apply has no row |
| text OPEN line names the remote AS and carries the OPEN's own AS as `asn` | text event consumers; bgp-rs parses it with `asn.Parse` |
| gNMI serves JSON and JSON_IETF; other encodings `Unimplemented` | gNMI clients (gnmic default now works) |
| LG `routes/protocol/<id>` resolves a listed name or a session address, else 404 | Birdwatcher clients |

## Decisions taken this session (not owner questions)

LLGR nil egress state was already fixed, so its existing tagged tests stand. The BGP close cause reaches the down event as typed fields; the "connection lost" text the 2026-10-13 ruling kept was already gone. A RIB attribute-only change now publishes a best change. VPP duplicate address detection is its own spec, `plan/release/fixes/spec-dhcpv6-dad-on-vpp.md`. Two looking-glass `.ci` files moved from `routes/protocol/10.0.1.1` (a RIB-inject pseudo-peer no list publishes) to `routes/peer/10.0.1.1`.

## Foreign reds seen (not this spec's)

12 reactor RS/OwedKey tests red at HEAD (`spec-bgp-rs-replacement-on-withdrawal`); adj_rib_in `TestLateReflectorClaimsExistingReplayOwner` (row in `test-against-broken-path.md`); `exabgp-bridge-shutdown-external` (row owned by `spec-exabgp-6-api-compatibility`); watchdog wire-method snapshot (row owned by `spec-yang-single-source-of-truth`); config-commit storage-node, PKI, web and `le` answer tests red at HEAD 7630d77dcb; `le-rules-answers` (rendered `ai/rules/testing.md` not committed by 7be7588b60).

## The held doc section (re-insert verbatim)

```markdown
## Stages that judge the change, not the tree

Three arch stages in the population judge what the change under test added rather than the whole tree, because the tree still holds instances other work owns and a tree scope would red every commit until all of them were fixed. `arch enumeration check` judges the change set's packages. `arch compound-guard check` and `arch refuse-by-default check` judge the changed LINES in the unpushed range, committed or not, against the merge base of HEAD and its upstream (or `origin/main`), read from refs only, so the detached worktree `./le verify worktree` runs in answers the same range as its source checkout. A checkout with no such base exits 2, never 0. Each has a `report` action (`./le arch enumeration report`, `./le arch refuse-by-default report`) that lists the tree's instances and exits 0, because a reader deciding what to fix is not a gate. The fast population keeps all three.

`arch refuse-by-default` refuses a changed line that answers a plausible value where the code has no answer: a switch's unmatched value answered with a nil error, a lookup miss or read error answered as a hit, an ADD-PATH path identifier compared with zero, and a state or health answer written from a literal. A shape carrying a `refuse-by-default: guards <Symbol>` annotation is accepted and listed in the stage's output, so a reviewer reads every guard a change names (`docs/contributing/ze-go-style.md`, "A zero value is never an answer").

<!-- source: internal/le/verify/engine/stages.go -- fullStages -->
<!-- source: internal/le/arch/refusebydefault/actions.go -- runCheck, runReport -->
<!-- source: internal/le/arch/compoundguard/actions.go -- runCheck -->
```
