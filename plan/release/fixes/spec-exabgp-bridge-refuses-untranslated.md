# Spec: the ExaBGP bridge and migration refuse what they do not translate

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol |
| Priority | P1 |
| Release reason | ExaBGP 6.0, 5.0 and 4.2 compatibility is agreed; a migrated config or helper script that loses a family, a flush, an End-of-RIB or a capability warning in silence runs differently from ExaBGP with no sign to the operator |
| Depends | `spec-exabgp-6-api-compatibility` (closed 2026-10-24; its item 2 defines the error answer a script receives, which this spec uses for every refusal, and it edits the same files) |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-24 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Root cause S7 of the journal backlog (owner order, 2026-10-23). Seven open rows share one
mechanism: the bridge (`internal/exabgp/bridge/`, `internal/plugins/exabgp/`), the migration
(`internal/exabgp/migration/`, `ze config migrate`) and the ExaBGP test runner each translate
the subset of ExaBGP input they know and treat the rest as nothing. Nothing records which
input was consumed, so what was not consumed disappears: a family list that was never
written becomes `ipv4/unicast`, a flush becomes a Warn, an explicit `announce eor` is
swallowed by a claim, a migrated `auto-flush false` landed in a leaf nobody read (fixed 2026-10-23: the migration drops it, as ExaBGP ignores it, and the leaf is gone), the
dead-capability warnings look at the output tree where the capability can never be, and the
test runner drops `option=` lines it does not know.

Why it matters for shipping Ze: the bridge is how existing ExaBGP users move to Ze. A helper
that announces into a family Ze never declared, or waits for a flush that never happens,
fails in production with no message, and the user blames Ze or the script rather than the
translation.

Why a new spec and not an extension of `spec-exabgp-6-api-compatibility.md`: that spec owns
the v6 API vocabulary, versions and answer shapes in eight phases, is in progress under
another session, and none of its acceptance criteria owns these rows (checked 2026-10-23:
no AC names the family fallback, the flush RPC, the EOR claim, `auto-flush`, the migrate
warnings or the runner's option lines). This spec depends on it for the error answer and
lands after its phases that touch the same files.

The fix: every translator consumes input explicitly and refuses the remainder with the
reason and the fix; no fallback stands in for missing input. The check that keeps it: the
migration tracks consumed nodes and refuses the unconsumed at runtime, the bridge is run
over ExaBGP's own API corpus with a test that fails on any line that yields neither a
translation nor an error answer, and the runner refuses an unknown directive as the mock
already does.

**The AS_SET send half (AC-11), moved here from `spec-exabgp-6-api-compatibility.md`.**
Owner, Thomas, 2026-10-24: "Move to bridge-refuses-untranslated". The decision it carries
is the owner's of 2026-10-22: "as_set allow it via the bridge but not directly from the
plugin, so only exabgp compat can make it". ExaBGP 4.2, 5.0 and 6.0 send an AS_PATH with a
set when the script writes one (`as-path [ 65001 ] ( 65002 65003 )`; ExaBGP's `as_path` in
`static/parser.py` reads `(` as an AS_SET). Ze refuses it at two layers today:

| Layer | Today | Owed |
|-------|-------|------|
| Bridge `as-path` (`bridge_attribute.go`, `arityList` reads `[ ]` only) | `as-path ( 3 4 )` refused (`3` an unknown attribute); the `(` of `[ 1 ( 2 3 ) ]` passed through | both forms translated to the set spelling below |
| `update text` `as-path` (`plugins/cmd/update/update_text.go`, `case "as-path"`, a flat `[]uint32`; `attribute.ParseASPathText` reads one sequence) | no spelling for a set: `invalid ASN in as-path: (` | a set spelling that only update text the bridge originates may use; any other sender refused, naming RFC 9774 Section 3 |
| Send guard (`Session.writeUpdateBody`, `reactor/session_write.go`, `!raw && s.transmittedPathHoldsSet(body)`) | withdraws every non-raw UPDATE whose path holds a set, whoever built it | exempts an UPDATE whose route the bridge originated, and only that one |
| Origin grant | none | a Go-only field of `registry.Registration` the bridge plugin sets: not on the SDK wire, so no external plugin can claim it; the dispatcher reads it from the sender (`CommandContext.Sender`, the bridge's process) and the flag rides the built route down to `writeUpdateBody`. Blocked until another session's uncommitted `internal/component/plugin/registry` work (`registry.go`, `config_need.go`, `gated_plugins*.go`) lands, because both edit `Registration` |

RFC 9774 Section 3: "Unless explicitly configured by a network operator to do otherwise
(e.g., during a transition phase), BGP speakers: MUST NOT advertise BGP UPDATE messages
containing AS_SETs or AS_CONFED_SETs". The ExaBGP script an operator migrated and runs is
that configuration; Ze's own API, other plugins and config stay refused, and deleting the
guard is ruled out. The receive half is done in the exabgp-6 spec (`session as-set accept`,
5ac5c3fc0d, 3c4106d89a).

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/exabgp-bridge.md` - bridge translation, families, flush, EOR
  → Constraint: the bridge lives outside the engine; the engine gains no ExaBGP awareness
- [ ] `docs/exabgp/exabgp-migration.md` - what `ze exabgp migrate` carries
  → Constraint: every refusal this spec adds is listed on this page with its fix
- [ ] `docs/exabgp/exabgp-differences.md` - documented differences
  → Decision: OPERATIONAL and multi-session are documented differences (owner rule); the migrate warning for them is a documented-difference notice, not a refusal of the whole config
- [ ] `spec-exabgp-6-api-compatibility` (closed 2026-10-24) - item 2 error answer, item 5 refusals
  → Constraint: a refusal reaches the script as that spec's error answer for the helper's API version; no second answer path
- [ ] `ai/META-DECISIONS.md` - ExaBGP compatibility section
  → Decision: "Where ze is stricter than ExaBGP without an RFC MUST behind it, ExaBGP's behaviour wins"; this answers the EOR row's open owner question

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc4724.md` - End-of-RIB
  → Constraint: RFC 4724 Section 2: "An UPDATE message with no reachable Network Layer Reachability Information (NLRI) and empty withdrawn NLRI is specified as the End-of-RIB marker that can be used by a BGP speaker to indicate to its peer the completion of the initial routing update after the session is established." The section sets no limit of one marker per family per session, so the comment above `claimInitialSyncEOR` (`reactor/peer.go`) that cites one is wrong and is corrected; the claim stays as the dedup between Ze's own two initial-sync producers, and an explicit script request bypasses it, as ExaBGP sends a marker for every `announce eor`

**Key insights:**
- The bridge already refuses an unknown line (`ErrLineNotTranslated` in `Translator.Line`); the drops sit in fallbacks, Warn-only failures and claims, not in the parser
- The test mock (`readExaBGPOption` in `internal/le/interoplab/bgp/exabgp_case.go`) already refuses an option it does not read; the runner beside it (`parseExaBGPCI`) does not

## Current Behavior (MANDATORY)

**Source files read (verified 2026-10-23):**
- [ ] `internal/plugins/exabgp/bridgeplugin/config.go` - `parseConfig` sets `cfg.Families` to `defaultFamily` (`ipv4/unicast`) when the leaf-list is empty
- [ ] `internal/exabgp/migration/migrate.go` - the bridge block (`exabgp { bridge { process ... } }`) is written with process, run, respawn, encoder and on-exit; no `family` leaf is written; watchdog processes are now carried and bound to bgp-watchdog
- [ ] `internal/exabgp/bridge/bridge_muxconn.go` - `formatFlushRequest` sends `ze-bgp:peer-flush` as a plugin RPC; `dispatchPluginRPC` (`plugin/server/dispatch.go`) resolves only engine ops and codec RPC handlers and answers `unknown method` otherwise, while `ze-bgp:peer-flush` is registered as a command RPC in `bgp/plugins/cmd/peer/peer.go`; whether the request now resolves is not re-run today
- [ ] `internal/component/bgp/reactor/reactor_api_forward.go` - `AnnounceEOR` takes `claimInitialSyncEOR` and stands down when the family's marker was already sent, so a script's explicit `announce eor` after initial sync is dropped with no answer
- [ ] `internal/component/config/cli/cmd_migrate.go` - `findUnsupportedFeatures` runs over `result.Tree`, the migrated output, so its three ExaBGP dead-capability warnings cannot fire
- [ ] `internal/test/cli/cmd_exabgp.go` - `parseExaBGPCI` reads `option=file:`, `option=tcp_connections:`, `option=serial`, `option=env:`, `option=allowed:`, `option=allowed-error:` and ignores any other `option=` line
- [ ] `internal/plugins/exabgp/bridgerun/fleet.go` - an event with no BGP message is logged at Debug and dropped
- [ ] `internal/exabgp/bridge/bridge_route_forms.go` - `convertEOR` answers `ipv4/unicast` for a bare `announce eor`, as ExaBGP does (row found fixed)

**Behavior to preserve:**
- `ErrLineNotTranslated` for unknown lines; every translation the exabgp-compat suites prove today
- Watchdog process migration and bgp-watchdog binding

**Behavior to change:** each owned row; listed in the AC table.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- An ExaBGP config through `ze exabgp migrate` or `ze config migrate`; a helper script line through the bridge; a `.ci` case through the exabgp runner

### Transformation Path
1. Migration parses the ExaBGP tree and marks each node it translates in a consumption ledger
2. Any unmarked node is refused with its path, the reason and the fix (or, for a documented difference, a notice naming the differences page)
3. The bridge translates a script line to a Ze command; a command Ze refuses, or a request the bridge cannot complete, answers the script with the error answer of the exabgp-6 spec
4. Events the script subscribed to reach it, or a Warn names the event type the bridge cannot encode

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Script and bridge | text lines and the version's error answer | No |
| Bridge and engine | text commands up, JSON events down | No |
| Migration and config | Ze config tree out; refusal list | No |

### Integration Points
- The exabgp-6 spec's error answer writer (`AckMode` in `bridge_ack.go` as that spec reshapes it)
- `ze-bgp:peer-flush` command registration for the flush request

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | flush goes through the registered RPC; EOR through `AnnounceEOR` |
| No unintended coupling (components stay isolated) | Yes | the engine gains no ExaBGP knowledge; the EOR bypass is a flag on the command, not an ExaBGP check |
| No duplicated functionality (extends existing, does not recreate) | Yes | uses the exabgp-6 error answer and the mock's option refusal shape |
| Zero-copy preserved where applicable (refs, not copies) | N-A | control paths |
| Registration over hardcoding, outbound | Yes | no new registry |
| Registration over hardcoding, inbound | Yes | families come from the ExaBGP config, not from a default; the runner's directive set is the one it reads, refusing the rest |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | ExaBGP sends an EOR for every `announce eor`, including after initial sync | ExaBGP `api_eor` at the pin; read before Phase 3 | the bypass is wrong; refuse with the error answer instead | read ExaBGP source at the pin and quote the function | unvalidated |
| A-2 | The ExaBGP neighbor families are known at migration time | ExaBGP neighbor `family` block, defaults per ExaBGP | the bridge family list must follow ExaBGP's own default | migrate corpus test over `test/exabgp-compat/etc` | unvalidated |
| A-3 | ExaBGP `auto-flush false` holds announcements until the script's `flush` | ExaBGP source at the pin | the leaf maps to another behaviour | read ExaBGP source and quote it | WRONG (2026-10-23): the only reader, `self._have_routes = self.neighbor.flush if update is None else update` in `peer.py`, was deleted by d337e1519 ("always check for new routes in outgoing RIB", first in 4.0.4). 4.2.25, 5.0.1 and f9bfb853b only parse, compare and print it, and send every route at once. `flush adj-rib out` resends the Adj-RIB-Out (`neighbor_rib_resend`), it releases nothing held. Resolved: the migration accepts `auto-flush` and writes nothing, and ze's unwired leaf is deleted |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Migration refusals reject configs that migrate today | exabgp-compat encoding reds | each refusal names its fix; a documented difference is a notice, not a refusal |
| R-2 | Conflict with the exabgp-6 spec's in-flight edits | same files dirty | land after its phases touching `bridge_command.go`, `migrate.go`, `cmd_exabgp.go` |

## Blast Radius
| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | migrated configs refused, or scripts receiving errors where they received silence |
| How is it reverted? | one commit per row group |
| Who else touches this path? | `spec-exabgp-6-api-compatibility` (same files), `spec-plugin-optional-dependency-refusal` (the bridge's undeclared BGP dependency row) |

## Wiring Test (MANDATORY -- NOT deferrable)
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze exabgp migrate` over every config in `test/exabgp-compat/etc` | → | migration consumption ledger | `TestMigrateCorpusConsumesOrRefusesEveryNode` |
| bridge over ExaBGP's API command corpus | → | `Translator.Line` and the error answer | `TestBridgeCorpusEveryLineTranslatedOrAnswered` |
| `.ci` case with an unknown `option=` line | → | `parseExaBGPCI` | `TestParseExaBGPCIRefusesUnknownOption` |
| bridge config with no `family` | → | `parseConfig` | `TestBridgeConfigWithoutFamilyRefused` |
| script `announce eor` after initial sync | → | `AnnounceEOR` | `test/exabgp-compat/api/api-eor.ci` |
| script `announce route ... as-path [ 65001 ] ( 65002 65003 )` | → | bridge `as-path` set translation, update text set spelling under the bridge grant, `writeUpdateBody` exemption | `test/exabgp-compat/api/api-as-set.ci`, interop `bgp-exabgp-bridge-as-set-frr` |
| `send bgp ... update text` with the set spelling from Ze's API | → | update text refusal without the grant | `test/plugin/update-text-as-set-refused.ci` |

## Acceptance Criteria
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | An ExaBGP config holding a node the migration does not translate | migration refuses it, naming the ExaBGP path, why, and the fix; a documented difference (OPERATIONAL, multi-session) gives a notice naming `docs/exabgp/exabgp-differences.md` |
| AC-2 | Every config in `test/exabgp-compat/etc` and ExaBGP's own `qa/conf` at the pin | every input node is either consumed or refused; the test fails on an unconsumed node with no refusal |
| AC-3 | ExaBGP neighbor with families, migrated | the bridge block carries those families; an `exabgp { bridge }` with no family is refused at verify, naming the leaf and the fix |
| AC-4 | `ze config migrate` on an ExaBGP config with `capability multi-session`, `capability operational` or a neighbor `operational` block | the warning fires, read from the input tree |
| AC-5 | Script `flush` with `auto-flush false` | the flush reaches the peer through the registered `ze-bgp:peer-flush`; when it fails, the script receives the error answer |
| AC-6 | Migrated `auto-flush false` | accepted, and changes nothing: the migrated peer sends announcements at once, matching ExaBGP since 4.0.4 (A-3). Done 2026-10-23: `TestMigrateAutoFlushAcceptedAndDropped` |
| AC-7 | Script `announce eor` after the family's initial-sync marker | the marker is sent (ExaBGP behaviour, A-1); the `claimInitialSyncEOR` comment no longer cites a limit RFC 4724 Section 2 does not set |
| AC-8 | Bridge over ExaBGP's API command corpus (the `qa/api` lines and the 4.2 and 5.0 corpora the exabgp-6 spec adopts) | every line yields a Ze command or the error answer; the test fails on a line that yields neither |
| AC-9 | A `.ci` case with `option=update:send-default-route` or any unread `option=` | the runner refuses the case, naming the line and the fix |
| AC-10 | An event of a type a script subscribed to that the bridge cannot encode | a Warn once per type names the event type; an envelope that is routine (no message by design) stays at Debug |
| AC-11 | A script writes `announce route 10.0.0.0/24 next-hop 1.2.3.4 as-path [ 65001 ] ( 65002 65003 )`, and another `as-path ( 3 4 )` | each reaches the peer with the AS_SET segment as written (RFC 9774 Section 3 operator exemption, owner 2026-10-22 and 2026-10-24); the same set sent through `send bgp ... update text` from Ze's API, another plugin or config is refused naming RFC 9774 Section 3; a set on a route the bridge did not originate is still withdrawn by the send guard; the AS_SET row of `oldAllowed` (`old_allowed_test.go`) is deleted and `docs/exabgp/exabgp-differences.md` drops the send paragraph; `rfc/short/rfc9774.md` records the send-half deviation and RFC9774-3-1 gains the exempt polarity with a fresh discrimination record |

## End-to-End User Stories
| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | migrates an ExaBGP config with an IPv6 family | migrate, bridge family list, script announce | `test/exabgp-compat/encoding/` case for an IPv6 neighbor |
| 2 | migrates a config with `auto-flush false` and runs a helper that announces | migration drops the keyword; bridge, wire | new `test/exabgp-compat/api/api-auto-flush.ci`: announcements leave at once, as ExaBGP sends them |
| 3 | runs a helper that sends `announce eor` twice | bridge, `AnnounceEOR`, wire | `test/exabgp-compat/api/api-eor.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestMigrateCorpusConsumesOrRefusesEveryNode` | `internal/exabgp/migration/migrate_test.go` | AC-1, AC-2 | |
| `TestMigrateWritesBridgeFamilies` | same | AC-3 | |
| `TestBridgeConfigWithoutFamilyRefused` | `internal/plugins/exabgp/bridgeplugin/config_test.go` | AC-3 | |
| `TestConfigMigrateWarnsOnInputCapabilities` | `internal/component/config/cli/cmd_migrate_test.go` | AC-4 | |
| `TestBridgeFlushReachesRegisteredRPC` | `internal/exabgp/bridge/bridge_muxconn_test.go` | AC-5 | |
| `TestExplicitEORBypassesInitialSyncClaim` | `internal/component/bgp/reactor/reactor_api_forward_test.go` | AC-7 | |
| `TestBridgeCorpusEveryLineTranslatedOrAnswered` | `internal/exabgp/bridge/bridge_corpus_test.go` | AC-8 | |
| `TestParseExaBGPCIRefusesUnknownOption` | `internal/test/cli/cmd_exabgp_test.go` | AC-9 | |
| `TestFleetWarnsOnUnencodableSubscribedEvent` | `internal/plugins/exabgp/bridgerun/fleet_test.go` | AC-10 | |
| `TestBridgeASPathSetForms` | `internal/exabgp/bridge/bridge_attribute_test.go` | AC-11: `( 3 4 )` and `[ 1 ( 2 3 ) ]` translate to the set spelling | |
| `TestUpdateTextASSetNeedsBridgeGrant` | `internal/component/bgp/plugins/cmd/update/update_text_test.go` | AC-11: set spelling accepted with the grant, refused without, reason names RFC 9774 Section 3 | |
| `TestWriteUpdateBodySendsBridgeOriginatedSet`, `TestWriteUpdateBodyWithdrawsOtherSet` | `internal/component/bgp/reactor/session_write_test.go` | AC-11: exemption by origin only; red with the exemption off | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N-A | no numeric input added | N/A | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `api-eor` | `test/exabgp-compat/api/` | second explicit EOR reaches the peer | |
| `api-auto-flush` | `test/exabgp-compat/api/` | with `auto-flush false`, announcements leave at once (ExaBGP since 4.0.4) | |
| `api-check` | `test/exabgp-compat/api/` | runs once the mock reads `send-default-route`, or is refused by the runner until it does | |
| `cli-exabgp-migrate-untranslated-refused` | `test/parse/` | migrate refuses an untranslated node with its fix | |
| `api-as-set` | `test/exabgp-compat/api/` | AC-11: a script's AS_SET reaches the mock peer as written; green under `--dut exabgp` at the main pin first | |
| `update-text-as-set-refused` | `test/plugin/` | AC-11: the set spelling from Ze's API is refused with the reason | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| exabgp-compat suites against the pinned ExaBGP | `test/exabgp-compat/` | ExaBGP 5.0.13 and ExaBGP main (pins of the exabgp-6 spec) | Ze answers each line as ExaBGP does, the EOR and flush included | |
| `bgp-exabgp-bridge-as-set-frr` | `test/interop/scenarios/` | FRR (`no bgp reject-as-sets`) | AC-11: a bridge script's AS_SET is in FRR's table as written; red with the `writeUpdateBody` exemption off, image rebuilt | |

## Files to Modify
- `internal/exabgp/migration/migrate.go`, `migrate_serialize.go` - consumption ledger, bridge families, refusals
- `internal/plugins/exabgp/bridgeplugin/config.go` - remove the family fallback, refuse empty
- `internal/exabgp/bridge/bridge_muxconn.go`, `bridge_batch.go` - flush through the registered RPC (no auto-flush hold: A-3 was wrong)
- `internal/component/bgp/reactor/reactor_api_forward.go`, `peer.go` - explicit EOR bypass, corrected comment
- `internal/component/config/cli/cmd_migrate.go` - warnings from the input tree
- `internal/test/cli/cmd_exabgp.go` - refuse unknown `option=`
- `internal/plugins/exabgp/bridgerun/fleet.go` - Warn on unencodable subscribed events
- AC-11: `internal/exabgp/bridge/bridge_attribute.go` (set forms), `internal/component/bgp/plugins/cmd/update/update_text.go` and `internal/core/bgp/attribute/text.go` (set spelling, segments carried to the encoder), `internal/component/plugin/registry/registry.go` (the Go-only grant; blocked, see Task), the bridge plugin's registration, `internal/component/bgp/reactor/session_write.go` (exemption), `internal/exabgp/bridge/old_allowed_test.go` (AS_SET row deleted), `internal/component/bgp/yang/ze-bgp-conf.yang` (the as-set leaf sentence on sending), `rfc/short/rfc9774.md`, `docs/exabgp/exabgp-differences.md`, `docs/architecture/exabgp-bridge.md`
- `docs/architecture/exabgp-bridge.md`, `docs/exabgp/exabgp-migration.md`, `docs/exabgp/exabgp-differences.md`

## Files to Create
- `internal/exabgp/bridge/bridge_corpus_test.go`
- `test/exabgp-compat/api/api-auto-flush.ci` and its script
- `test/parse/cli-exabgp-migrate-untranslated-refused.ci`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | the bridge `family` leaf-list exists |
| YANG validation constraints | Yes | `min-elements 1` or a verify refusal on the bridge family list |
| YANG custom validators | No | |
| CLI commands/flags | No | |
| CLI grammar (keyword before value) | N-A | |
| Editor autocomplete | N-A | |
| Functional test for new RPC/API | N-A | no new RPC |
| Pipe completeness | N-A | |
| Env var registration | No | |
| Doctor check for runtime dependencies | No | |
| Prometheus counters/metrics | No | |
| BGP family surface (new SAFI / capability / attribute) | No | the bridge's family list follows the config; `ai/patterns/bgp-family.md` migration section 5b unchanged |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | |
| 2 | Config syntax changed? | Yes | `docs/exabgp/exabgp-migration.md` (family written, refusals) |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | Yes | `docs/architecture/exabgp-bridge.md` |
| 6 | Has a user guide page? | Yes | `docs/exabgp/exabgp-migration.md` |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc4724.md`: explicit EOR after initial sync |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` exabgp runner directives |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | No | |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `docs/architecture/exabgp-bridge.md`, `docs/exabgp/exabgp-differences.md` declared by bridge files; also declared by files named here: `docs/architecture/config/syntax.md` (affected if it shows the bridge block without `family`), `docs/architecture/core-design.md` (unaffected: the engine gains no ExaBGP awareness; the EOR bypass is a flag on an existing command, confirmed in Phase 4), `docs/architecture/testing/ci-format.md` (affected: the exabgp runner refuses unknown `option=` lines). Run `./le spec citation anchors spec plan/release/fixes/spec-exabgp-bridge-refuses-untranslated.md` per phase and name any further declared page |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | migration page examples of the bridge block |

## Implementation Steps

1. **Phase: Wiring** - corpus tests and runner test written red
2. **Phase: runner and migrate warnings** - AC-4, AC-9
3. **Phase: families** - AC-3, user story 1
4. **Phase: EOR and flush** - read ExaBGP at the pin (A-1, A-3), then AC-5 to AC-7
5. **Phase: consumption ledger** - AC-1, AC-2
6. **Phase: bridge corpus and events** - AC-8, AC-10
7. **Phase: AS_SET send half** - AC-11, after the registry work named in Task lands: `api-as-set` green under `--dut exabgp` first; then the grant, the set spelling, the exemption, each test red before; interop red with the exemption off; RFC 9774 records

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Correctness | each behaviour follows ExaBGP at the pin, quoted; RFC outranks only where it says MUST |
| Data flow | no ExaBGP knowledge enters the engine |
| Rule: no-layering | the family fallback is deleted, not kept beside the refusal |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| corpus tests | `TestMigrateCorpusConsumesOrRefusesEveryNode`, `TestBridgeCorpusEveryLineTranslatedOrAnswered` green, red when a refusal is removed |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | script lines that fail translation are answered, never executed partially |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read ExaBGP at the pin and the producer |
| Lint failure | Fix inline |
| Functional test fails | Check the AC |
| Audit finds a missing AC | Back to the relevant phase |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights
- Every drop sits after a successful parse; the parsers already refuse unknown words

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Explicit `announce eor` bypasses the initial-sync claim | refuse it with an error answer | RFC 4724 Section 2 sets no limit and ExaBGP sends it; owner rule: ExaBGP's behaviour wins where Ze is stricter without an RFC MUST |
| Runtime consumption ledger in the migration | a static list of translated keywords | a list is a second declaration that drifts |
| Bridge family list from the config, refused when empty | keep the `ipv4/unicast` default | the default is the defect |
| AS_SET send half lives here (AC-11), moved from `spec-exabgp-6-api-compatibility.md` (owner, Thomas, 2026-10-24: "Move to bridge-refuses-untranslated") | keep it in the exabgp-6 spec | it is a bridge line Ze does not translate today, the mechanism this spec owns; the exabgp-6 spec closes with the receive half done |
| Only bridge-originated routes may carry an AS_SET (owner, 2026-10-22: "as_set allow it via the bridge but not directly from the plugin, so only exabgp compat can make it"), granted by a Go-only `registry.Registration` field | delete the send guard; an operator config leaf; a grant on the SDK wire | deleting the guard is ruled out by the owner; a grant on the wire can be claimed by any external plugin |

## Known Limitations
- AS_SET in `as-path` is AC-11 here (moved 2026-10-24). Other mistranslations that Ze refuses (`clear adj-rib out` without selector, `announce route-refresh` read as a route, `interface-set`) and absent vocabulary (`vpls`, `ping`) were handled by `spec-exabgp-6-api-compatibility` (closed 2026-10-24) and its journal rows, not here

## RFC Documentation (Scope: protocol)

Correct the comment above `claimInitialSyncEOR` and quote RFC 4724 Section 2 above the explicit-EOR path.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `zero-value-as-valid-answer.md` | 2026-09-06 | bridge `parseConfig` falls back to ipv4/unicast; migration writes no family |
| `check-cannot-see-the-change-it-looks-for.md` | 2026-09-06 | `findUnsupportedFeatures` reads the migrated tree |
| `unwired-feature.md` | 2026-10-22 | `auto-flush` written by migrate, read by nothing |
| `unwired-feature.md` | 2026-10-17 | bridge `announce eor` dropped by `claimInitialSyncEOR` |
| `gate-excludes-part-of-its-population.md` | 2026-09-06 | `discoverExaBGPSuite` drops unknown `option=` lines |
| `declared-format-contradicts-payload.md` | 2026-09-19 | bridge drops events that carry no BGP message |
| `conflicting-feature-contracts.md` | 2026-10-21 | migration `TestInterfaceSetExtCommunityRefusals` "dotted as number" red after asdot `asn.Parse` |
| `declared-format-contradicts-payload.md` | 2026-09-06 | `update` event to an API process: empty peer address, zero ASN, no `message` |
| `delegation-hands-the-callee-a-different-argv.md` | 2026-10-17 | bridge `clear adj-rib out` dispatches `clear bgp rib out` with no selector |
| `delegation-hands-the-callee-a-different-argv.md` | 2026-10-17 | bridge reads `announce route-refresh` as a route, sends `-refresh` |
| `parser-refuses-a-form-its-producer-emits.md` | 2026-10-17 | `update text` refuses the bridge's interface-set and rate-limit extended communities |
| `parser-refuses-a-form-its-producer-emits.md` | 2026-10-21 | bridge `as-path` refuses ExaBGP's `( )` AS_SET segment (AC-11) |
| `unwired-feature.md` | 2026-09-05 | `conf-watchdog` against `migrateProcesses` dropping the watchdog |
| `unwired-feature.md` | 2026-10-17 | bridge refuses `announce vpls`, `withdraw vpls` and `ping` |
| `unwired-feature.md` | 2026-10-17 | API-created peer does not attach the creating process |
| `zero-value-as-valid-answer.md` | 2026-10-20 | `convertEOR` expands a bare `announce eor` over every family |

16 rows.

## Rows found already fixed

| Class file | Date | Surface (short) | Evidence (2026-10-23) |
|------------|------|-----------------|-----------------------|
| `zero-value-as-valid-answer.md` | 2026-10-20 | `convertEOR` expands a bare eor over declared families | `convertEOR` (`bridge_route_forms.go`) answers `ipv4/unicast` for a bare `announce eor`, quoting ExaBGP's `api_eor` |
| `gate-excludes-part-of-its-population.md` | 2026-09-19 | `readExaBGPCase` drops `:json:` expectations | `readExaBGPCase` (`internal/le/interoplab/bgp/exabgp_case.go`) attaches each `json:` line to the frame above it and refuses an unknown step kind and an unread option |
| `parser-refuses-a-form-its-producer-emits.md` | 2026-10-14 | bridge `convertAnnounceSRPolicy` leftover tokens refused by `parseSRPolicySection` | 2026-10-24: fixed by the exabgp-6 spec's AC-28, af18a709d8 and 9dea34f1b9; the row is marked fixed |

## Rows examined, not owned

| Class file | Date | Surface (short) | Why |
|------------|------|-----------------|-----|
| `unwired-feature.md` | 2026-09-05 | `conf-watchdog` against `migrateProcesses` | the drop is fixed (`migrate.go` carries processes and binds bgp-watchdog); the open remainder is the watchdog sequence the fixture expects, a different mechanism |
| `silent-fall-through.md` | 2026-10-20 | exabgp bridge reaches BGP undeclared | owned by `spec-plugin-optional-dependency-refusal.md` |

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints

### Goal Gates (MUST pass)
- [ ] AC-1..AC-11 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete
- [ ] Targeted tests pass: the scoped package tests of each changed package under `./le job run`, `./le test functional exabgp-test` for the api and encoding cases named above, and `./le verify current fast`; the full gate (`./le verify current`) once over the closing commits
- [ ] Every A-N confirmed or broken
- [ ] Every item this spec did not do is a spec of its own, named here

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (N-A)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean
- [ ] **Commit A:** code + tests + docs + edited spec + journal rows re-pointed
- [ ] **Commit B:** `remove plan/release/fixes/spec-exabgp-bridge-refuses-untranslated.md` only
