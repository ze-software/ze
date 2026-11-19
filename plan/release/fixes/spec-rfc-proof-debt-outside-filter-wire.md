# Spec: rfc-proof-debt-outside-filter-wire

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | tooling |
| Priority | P1 |
| Release reason | A release profile needs every mandatory requirement covered, and a requirement counts as covered only when a recorded break proved its test goes red (`ai/META-DECISIONS.md`, "Goal and scope"). Every row below is a requirement whose proof or audit verdict `./le rfc check` no longer accepts, so the public ledger carries it as unproven until the debt is paid |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-19 |

Scope is tooling: the work re-takes discrimination records, re-judges audit
verdicts and repairs two red tests in `internal/le/rfc`. No product behaviour
changes, so End-to-End User Stories and RFC Documentation are inapplicable and
left out. A product defect an audit exposes is fixed under
`ai/rules/completion.md`, inside this spec, because it blocks the verdict.

## Task

The final `./le rfc check` run of the filter-wire umbrella
(`spec-filter-wire-0-umbrella`) printed 98
violations. The umbrella's RFC records agent classified each against the
commit that caused it. The filter-wire work owns and repays the rows its own
commits caused. This spec owns the rest: proof debt that commits outside the
filtering changes left behind, which the owner put on the release fix list.

Each row is a requirement whose discrimination record no longer verifies,
whose `/ze-rfc-audit` verdict went STALE because the unit it judged changed,
or whose verdict went SHIFTED because only the file around it moved. Two
tooling tests in `internal/le/rfc` are also red, and the commits that touched
their package carry no filter work.

The goal: `./le rfc check` lists none of the rows below, every record was
observed red by `./le rfc record`, every STALE verdict was re-made by an
independent auditor, and both tooling tests pass.

This spec also owns the whole stale-proof backlog that accumulated after
that run (owner decision, 2026-11-19). GitHub CI's `./le rfc check` on
6149245ab7 (run 44453595590) printed 993 violations: 581 discrimination
records that no longer verify (308 unit-changed, 256 producer-changed, 17
producer-gone), 201 SHIFTED audit verdicts over 39 stems, 179 STALE audit
verdicts, and 32 others. The counts are that run's snapshot, recorded to
size the work; the live set is whatever `./le rfc check` names at HEAD, and
the ACs below are settled against it, never against this list. The backlog
grew because no commit was refused for staling a record. Commit 6eec153998
added that refusal (`rfc check` in `./le commit create`), so the set shrinks
from here and only this spec's re-takes, plus records other sessions have
already re-taken, move it. Rows another spec owns stay with that spec, see
AC-33.

This spec also owns the document-author evidence finding below. Its correction
needs a separate design review before implementation; the existing ready work
does not approve a change to the exclusion rules. Runtime decoder tests do not
prove an obligation on a defining document.

This spec also owns the 2026-11-15 source finding for
`RFC4659-3.4-1`: explicit raw UPDATE origination bypasses family admission.
Its existing scope includes product defects exposed by these audits. The
managed six-unit evidence stays bounded to its tested entry points; it does
not prove the raw API. The investigation and any confirmed repair belong
here, outside the active next-hop spec. Recording this finding does not
approve a product design or a diagnostic exception to the RFC.

## Required Reading

### Architecture Docs
- [ ] `docs/contributing/rfc-conformance-gates.md` - every gate this spec clears
  → Decision: a record says a named tagged unit was OBSERVED to fail under a named break; `./le rfc check` replays nothing and compares fingerprints, so a moved unit or producer fingerprint means the red must be observed again ("The discrimination record")
  → Constraint: `./le rfc reseal stem <stem>` re-stamps only a verdict whose unit is byte-identical and shifted; it cannot re-judge changed scope, and it re-stamps only the stems named ("The artifacts", the `rfc/audit` row)
  → Constraint: a STALE verdict is re-made by a judge and stored with `./le rfc audit-stamp stem <stem> from <path> mode rejudge`; an upgrade from `weak`, `wrong` or `partial` to `enforced` over unchanged units needs an `upgrade_reason`
  → Constraint: a unit test file named for a stem carries a tag for it or the `// RFC naming: untagged -- <reason>` marker. A tracked rename uses the commit-owned `file`/`remove` route, not `./le rfc move`, which removes its source immediately ("Moving a tagged test file", "Test file names")
- [ ] `docs/architecture/testing/interop.md` - the interop records run a scenario against FRR
  → Constraint: an interop record rebuilds the artifact the scenario drives so the break takes effect, and a Docker start under load times out rather than proving anything
- [ ] `ai/skills/ze-rfc-audit.md` - the audit a STALE row owes
  → Constraint: the auditor is independent of whoever changed the unit, and reads the RFC's own text in `rfc/full/`
- [ ] `ai/doors/le-tooling.md`, `ai/doors/routing-protocols.md`, `ai/doors/access-security.md`, `ai/doors/bgp-engine.md`, `ai/doors/bgp-plugins.md`, `ai/doors/hub-engine.md` - the subsystems whose tagged units this spec re-proves
  → Constraint: a bare `go test` omits feature tags; run a package through `./le job run label <label> command go test -tags <tags> <package>` (le-tooling, "Boundaries")
  → Constraint: the public conformance ledger is an outside party of every protocol subsystem (routing-protocols, "Who depends on it from outside")
- [ ] `internal/plugins/ospf/instance_enable_test.go` - the descriptive destination for AC-3
  → Constraint: preserve `TestOSPFv3InterfaceEnabledWithConfiguredInstanceID` byte-for-byte on the rename route; a descriptive filename makes no RFC proof claim

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc1661.md`, `rfc2328.md`, `rfc2516.md`, `rfc2545.md`, `rfc4271.md`, `rfc4659.md`, `rfc8907.md`, `rfc1997.md`, `rfc5340.md` - the rows re-judged
  → Constraint: a verdict is judged against `rfc/full/<stem>.txt`, never the summary (`ai/rules/rfc-compliance.md`)

**Key insights:**
- Every row is settled by `./le rfc check` no longer naming it. The classification lives in the umbrella's session state handoff and in the stale class list it produced; the cause commit of each row is in the Acceptance Criteria.
- RFC2328-4.4-3 and RFC2328-D.3-1 owe both a record re-take and an audit re-judge: both their records and their verdicts went stale in b5fdf9a87f.
- RFC8212-3-1 is NOT in this spec: its producer `internal/component/bgp/filterapi/policy.go::AcceptsAtEnd` changed in 2fd7958d6f and fa54cd4285, both `Spec-work: filter-wire-0-umbrella`, so the umbrella owns that interop record.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] umbrella final `./le rfc check` log (98 violations, run 2026-11-02 before the umbrella's last three commits) - names every row below with the reason it no longer verifies
- [ ] `rfc/discrimination/rfc2328/RFC2328-4.4-3.json`, `RFC2328-D.3-1.json` - positive revert records on `transport.go::HandleLinkUp` and `transport.go::SendPacket`, unit-changed by b5fdf9a87f (which edited both test files and `transport.go`)
- [ ] `rfc/discrimination/rfc2545/RFC2545-3-2.json`, `RFC2545-3-3.json` - revert records of `internal/le/interoplab/bgp/check_rfc.go::checkRFC2545NextHops` against `internal/component/bgp/reactor/link_scope.go::sharesNextHopSubnet`: 3-2 positive, 3-3 positive and negative. c782411791 changed the check function, so all three are unit-changed and RFC2545-3-3's partial scope lacks a current verified break
- [ ] `internal/le/interoplab/bgp/check_special.go` - `checkRFC2545NextHops` runs for scenario `bgp-rfc2545-linklocal-nexthop-frr`
- [ ] `internal/plugins/ospf/instance_enable_test.go` - the prepared descriptive destination for AC-3. Historical source: `internal/plugins/ospf/rfc5340_instance_enable_test.go`, added in b5fdf9a87f, named for rfc5340 but carrying no RFC 5340 tag. The historical source stays in place until the generated commit script removes it
- [ ] `internal/le/rfc/check_evidence_test.go::TestPriorEvidenceReadsTheNewestCommitHoldingTheBacklog` - FAILS deterministically in 0.2 s on 2026-11-02: when HEAD^ deleted `rfc/exclusion-evidence-backlog.txt` the prior-evidence reader answers no counts instead of the count read at the newest commit holding the file, and a malformed HEAD^ over a valid ancestor is not walked past (lines 459, 462, 467, 498, 508)
- [ ] `internal/le/rfc/selftest_test.go::TestRFCSelftestRealTreeRowReportsOwnedCheckFailures` - hit its 10-minute limit on a loaded host; no failing assertion was observed, so whether it is a timeout or a real failure is unknown

**Behavior to preserve:**
- Every requirement's level, text, tags and claim prose. A record or verdict is retaken against the unit as it is; no tag, claim or assertion is edited to make a record or verdict pass
- The refusals of `./le rfc check`: it keeps refusing every other stale record and verdict

**Behavior to change:**
- The prior-evidence reader in `internal/le/rfc`, so the newest commit holding the backlog file answers when HEAD^ deleted or malformed it (the defect `TestPriorEvidenceReadsTheNewestCommitHoldingTheBacklog` names), unless the test is shown wrong against the gate's documented contract, which then goes to `ze-test-judge`
- Whatever product defect an audit finds in a re-judged unit

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| `./le rfc record` | `internal/le/rfc`, `docs/contributing/rfc-conformance-gates.md` "Producing a record: the two proof routes" | Applies a break through a Go overlay, runs the tagged unit, writes a record only for a red that names the unit | Uses it unchanged for every record re-take; no record is written by hand |
| `./le rfc check` | `internal/le/rfc`, gates page "The gate's own answer" | Compares record and verdict fingerprints with the tree and names every one that moved | Uses it as the settling command of every AC |
| `./le rfc audit-stamp ... mode rejudge` | gates page "The artifacts", `rfc/audit` row | Replaces a re-made verdict in place with fresh fingerprints | Stores every STALE re-judge |
| `./le rfc reseal stem <stem>` | gates page "The artifacts" | Re-stamps a byte-identical, shifted unit's verdict for the named stems | Clears RFC1997-Well-1, and every SHIFTED verdict of AC-29, one run per stem |
| Commit-time stale-record refusal | `internal/le/rfc/discriminate_commit.go` `judgeStaleRecords`, gates page "The commit carries the record it stales" | Refuses a commit that stales a record the parent held fresh, printing the `./le rfc record` line that re-takes each | Stops the backlog of AC-28 from growing while this spec repays it; its printed re-take lines are the command form AC-28 uses |
| Commit-owned tracked rename | `docs/contributing/committing.md` "The keywords", gates page "Moving a tagged test file" | Selects the byte-identical destination with `file` and retains the source until the generated script removes it through `remove` | The rename route for AC-3; `./le rfc move` removes its source immediately and is not this route |
| `/ze-rfc-audit` skill | `ai/skills/ze-rfc-audit.md` | An independent judge re-reads the RFC and the tests and writes a verdict | Every STALE row is re-judged through it by an agent that did not change the unit |
| `/ze-test-judge` skill | `.claude/skills/ze-test-judge/` | An independent agent judges a test change that could lower what a test proves | Required only if a tooling test or a re-proved test is changed rather than the producer |
| Interop harness | `internal/le/interoplab`, `docs/architecture/testing/interop.md` | Runs one scenario against FRR in Docker | Drives the three RFC 2545 records; run only on a quiet host |
| Job admission | `./le job run`, `docs/contributing/running-commands.md` | Admits and logs one CPU-intensive run at a time | Every scoped package run and every record goes through it |
| Final UPDATE family admission | `internal/component/bgp/reactor/session_ownership.go`, `adjOutWrite.section`; `session_write.go`, `writeUpdateBody` | Checks `SupportsFamily` for non-raw output; explicit raw output observes ownership without applying the projection | Researches both raw forms using existing final admission and useful controls; no replacement or separate writer is approved |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le rfc record`, `./le rfc audit-stamp`, `./le rfc reseal`, each reading `rfc/short/<stem>.md` and the tagged units; the AC-3 tracked rename uses `./le commit create` with `file` and `remove`
- Output: JSON under `rfc/discrimination/<stem>/` and `rfc/audit/<stem>/`

### Transformation Path
1. The tagged unit and producer named in the AC are fingerprinted
2. `./le rfc record` overlays the break, runs the unit, observes the red, writes the record with the new fingerprints
3. The independent auditor writes a pending verdict outside `rfc/audit/`; `audit-stamp mode rejudge` stamps it
4. `./le rfc check` re-reads every record and verdict and names none of these rows

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| le tooling to Docker (interop records) | `internal/le/interoplab` starts FRR and Ze containers | No |
| le tooling to the Go toolchain | overlay build and `go test` of the tagged package | No |

### Integration Points
- `rfc/discrimination/<stem>/<id>.json` and `rfc/audit/<stem>/<id>.json` - the files the AC rewrite, through the tools only

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | every artifact is written by its own `./le rfc` action, none by hand |
| No unintended coupling (components stay isolated) | Yes | no product package gains an import |
| No duplicated functionality (extends existing, does not recreate) | Yes | Existing Mechanisms: every tool is reused |
| Zero-copy preserved where applicable (refs, not copies) | N-A | no encoding code changes |
| Registration over hardcoding, outbound | N-A | nothing new registers |
| Registration over hardcoding, inbound | N-A | no list learns a new name |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Each row's cause commit is the one named in its AC, and none is a filter-wire commit | umbrella stale classification; `git log` of each cause commit read 2026-11-02 | the row belongs to the umbrella and leaves this spec | `git log -1 --format=%B <sha>` carries no `Spec-work: filter-wire-0-umbrella` line | unvalidated |
| A-2 | The RFC 2545 interop records failed only from load (Docker start timed out at load about 21), not from a defect | records agent handoff | a real defect blocks the record and is fixed here | the record on a quiet host either observes the red or shows a real failure | unvalidated |
| A-3 | `TestRFCSelftestRealTreeRowReportsOwnedCheckFailures` timed out only from load | it hit its 10-minute limit with no failing assertion printed | a real failure is fixed here | AC-25 | unvalidated |
| A-4 | Each STALE unit still proves its requirement after the change that staled it | the cause commits are renames, lint fixes, naming, a panic guard and a declaration pass | an auditor's `weak` or `wrong` verdict makes the test or producer owed work in this spec | AC-8 to AC-22 | unvalidated |
| A-5 | The diagnostic contract's intended malformed-input freedom settles valid raw VPNv6 admission | Raw Passthrough Commands documents raw injection; RFC 4659 §3.4 states no diagnostic exception | A design could wrongly treat a sender capability defect as an exclusion; the boundary remains open and no waiver is approved | Read the exact contract and full RFC sentence in design review, preserving level and applicability | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | An interop record is run on a loaded host and times out again | Docker container start timeout | run only when the owner says the host is quiet; never retry the same run hoping it passes |
| R-2 | An auditor re-judges a unit it changed | the auditor's session authored the cause commit | the main thread assigns each STALE row to an agent with no part in its cause commit |
| R-3 | A re-take is written against a unit another session is still changing | `./le rfc check` names the row again after the record | take the record after that session's commit, and re-take it once |
| R-4 | AS width, next-hop refusal or blanket dropping hides the VPNv6 family subject | No useful VPNv6 or same-session IPv4 output even in the negotiated control | Use actual capability sets, matching nonzero AS_PATH width, a healthy third-party next hop and accepting policy; require exact useful controls before interpreting refusal |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | The public ledger shows a requirement as proven on a record that did not observe its red, or a verdict that no judge re-made |
| How is it reverted? | A single commit per record or verdict set, reverted by a fix commit |
| Who else touches this path? | The filter-wire umbrella re-takes its own rows in the same `rfc/discrimination` and `rfc/audit` stems (rfc4271, rfc1997, rfc8212) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le rfc check` | → | record and verdict fingerprint comparison in `internal/le/rfc` | `TestPriorEvidenceReadsTheNewestCommitHoldingTheBacklog` |
| `./le rfc selftest` path over the real tree | → | owned check failure reporting in `internal/le/rfc` | `TestRFCSelftestRealTreeRowReportsOwnedCheckFailures` |

## Acceptance Criteria

Every AC is settled when `./le rfc check` no longer names its row, after the
named action ran. Record ACs observe a red through `./le rfc record`; audit ACs
are re-made by an independent `/ze-rfc-audit` agent and stamped with
`./le rfc audit-stamp stem <stem> from <pending> mode rejudge`.

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Record RFC2328-4.4-3 positive, cause b5fdf9a87f (OSPF) | `./le rfc record id RFC2328-4.4-3 polarity positive unit internal/plugins/ospf/transport/rfc2328_link_test.go::TestOSPFTransportPassesLinkIndicationsToOSPF route revert producer internal/plugins/ospf/transport/transport.go::HandleLinkUp` observes the red; `./le rfc check` no longer names `RFC2328-4.4-3.json` |
| AC-2 | Record RFC2328-D.3-1 positive, cause b5fdf9a87f | `./le rfc record id RFC2328-D.3-1 polarity positive unit internal/plugins/ospf/transport/rfc2328_digest_ip_length_test.go::TestRFC2328DigestCountedInIPLengthNotPacketLength route revert producer internal/plugins/ospf/transport/transport.go::SendPacket` observes the red; `./le rfc check` no longer names `RFC2328-D.3-1.json` |
| AC-3 | Historical misnamed source `internal/plugins/ospf/rfc5340_instance_enable_test.go`, cause b5fdf9a87f; descriptive destination `internal/plugins/ospf/instance_enable_test.go` | Either the file is tagged for the RFC 5340 row it proves (the Instance ID enable behaviour) with a record taken by `./le rfc record`, or the byte-identical descriptive destination is selected by `./le commit create` with `file internal/plugins/ospf/instance_enable_test.go` and `remove internal/plugins/ospf/rfc5340_instance_enable_test.go`, leaving the source in place until the generated script removes it; `./le rfc check` no longer names the file |
| AC-4 | Interop record RFC2545-3-2 positive, cause c782411791, on a quiet host | `./le rfc record id RFC2545-3-2 polarity positive unit internal/le/interoplab/bgp/check_rfc.go::checkRFC2545NextHops route revert producer internal/component/bgp/reactor/link_scope.go::sharesNextHopSubnet` observes scenario `bgp-rfc2545-linklocal-nexthop-frr` red; the RFC2545-3-2 record line leaves `./le rfc check`. Note (2026-11-03, from the stage-filters work): that revert route now fails at assertion 2 (the halt drops the route before FRR sees it), so the recorder refuses it as not naming citation 4; a hand-written mutant that keeps the route is owed. Three encoder breaks stayed green under this checker: `update_build.go` `buildMPReach`, `reactor_wire.go` `announceNextHopOctets`, and `buildMPReach` paired with the `!ll.IsValid()` egress raise in `forward_next_hop.go` `applyEgressNextHopScope`, so the config `update` route builds the next-hop pair elsewhere (candidates: rib `commit.go` `buildMPReachNLRI`, `reactor_api_batch.go` `buildBatchAnnounceUpdate`, `peer_rib_routes.go` `buildRIBRouteUpdate`, `peer_forward_facts.go` `applyFactsNextHop`). FRR 10.7.1 `bgp_path_info_to_ipv6_nexthop` (`bgpd/bgp_zebra.c`) always installs via the link-local once the 32-octet pair arrives, so citation 5 is unreachable by a Ze break |
| AC-5 | Interop record RFC2545-3-3 positive, cause c782411791, quiet host | The same command with `id RFC2545-3-3 polarity positive` observes the red; that record line leaves `./le rfc check` |
| AC-6 | Interop record RFC2545-3-3 negative, cause c782411791, quiet host | The same command with `id RFC2545-3-3 polarity negative` observes the red; that record line and both "partial scope needs a current verified mutant/revert" lines for RFC2545-3-3 leave `./le rfc check` |
| AC-7 | STALE audit RFC2545-3-3, cause c782411791 (`checkRFC2545NextHops` changed) | Re-judged and stamped in stem rfc2545; `./le rfc check` no longer names it |
| AC-8 | STALE audit RFC4659-3.2.1.1-3, cause c782411791 (`checkVPNFRR` changed) | Re-judged and stamped in stem rfc4659; `./le rfc check` no longer names it |
| AC-9 | STALE audit RFC1661-5.1-3, cause 1f4e1e81f1 (l2tp test file rename) | Re-judged and stamped in stem rfc1661; `./le rfc check` no longer names it |
| AC-10 | STALE audit RFC1661-5.2-3, cause 1f4e1e81f1 | Re-judged and stamped; `./le rfc check` no longer names it |
| AC-11 | STALE audit RFC1661-5.2-4, cause 1f4e1e81f1 | Re-judged and stamped; `./le rfc check` no longer names it |
| AC-12 | STALE audit RFC1661-5.3-7, cause 1f4e1e81f1 | Re-judged and stamped; `./le rfc check` no longer names it |
| AC-13 | STALE audit RFC1661-5.4-3, cause 1f4e1e81f1 | Re-judged and stamped; `./le rfc check` no longer names it |
| AC-14 | STALE audit RFC1661-5.4-4, cause 1f4e1e81f1 | Re-judged and stamped; `./le rfc check` no longer names it |
| AC-15 | STALE audit RFC1661-5.4-5, cause 1f4e1e81f1 | Re-judged and stamped; `./le rfc check` no longer names it |
| AC-16 | STALE audit RFC1661-5.8-2, cause 1f4e1e81f1 | Re-judged and stamped; `./le rfc check` no longer names it |
| AC-17 | STALE audit RFC1661-6.4-7, cause 1f4e1e81f1 | Re-judged and stamped; `./le rfc check` no longer names it |
| AC-18 | STALE audit RFC2516-x-9, cause 8c68fcf514 (`TestRFC2516HostSessionMTUDefaultsToThePPPoECeiling`) | Re-judged and stamped in stem rfc2516; `./le rfc check` no longer names it |
| AC-19 | STALE audit RFC4271-5.1.4-1, cause 8f704db860 (`test/plugin/med-locally-set-reaches-peer.ci`) | Re-judged and stamped in stem rfc4271; `./le rfc check` no longer names it |
| AC-20 | STALE audit RFC4271-5.1.4-2, cause f23e06435c (`TestHandleFilterUpdateMEDRemoveIsImportOnly`, lint) | Re-judged and stamped in stem rfc4271; `./le rfc check` no longer names it |
| AC-21 | STALE audit RFC4271-8.2.2-13, cause 898caba3f6 (`fsm.go::handleActive`, panic guard) | Re-judged and stamped in stem rfc4271; `./le rfc check` no longer names it |
| AC-22 | STALE audit RFC8907-8.3-4, cause c44e1bdc8c (`cmd/ze/hub/rfc8907_api_test.go` stream tests) | Re-judged and stamped in stem rfc8907; `./le rfc check` no longer names it |
| AC-23 | STALE audits RFC2328-4.4-3 and RFC2328-D.3-1, cause b5fdf9a87f | Re-judged after AC-1 and AC-2 and stamped in stem rfc2328; `./le rfc check` names neither |
| AC-24 | SHIFTED audit RFC1997-Well-1, cause c782411791 (`checkNoExportBoundary` moved, byte-identical) | `./le rfc reseal stem rfc1997` re-stamps it; `./le rfc check` no longer names it |
| AC-25 | `TestRFCSelftestRealTreeRowReportsOwnedCheckFailures`, which hit its 10-minute limit under load | Run alone on a quiet host through `./le job run label rfc-selftest command go test -count=1 -run TestRFCSelftestRealTreeRowReportsOwnedCheckFailures -timeout 30m ./internal/le/rfc/`. A pass in time records the earlier red as load; a failing assertion is a defect fixed at its producer; a run that still exceeds 10 minutes alone is a defect in the test's cost, fixed so it completes inside its limit without dropping what it asserts |
| AC-26 | `TestPriorEvidenceReadsTheNewestCommitHoldingTheBacklog`, failing deterministically | `./le job run label rfc-prior command go test -count=1 -run TestPriorEvidenceReadsTheNewestCommitHoldingTheBacklog ./internal/le/rfc/` passes after the prior-evidence reader is fixed, and the run is red with the fix reverted |
| AC-28 | Every discrimination record `./le rfc check` names as unit-changed, claim-changed or producer-changed at HEAD, outside AC-33's rows | Each is re-taken by `./le rfc record` on its recorded route against the unit and producer as they are, and observed red; a record whose break no longer reddens its unit is a finding on that test (the test or the producer is fixed here, through `ze-test-judge` where a test changes), never a hand-written record. Settled when `./le rfc check` names no such record |
| AC-29 | Every SHIFTED audit verdict `./le rfc check` names at HEAD | `./le rfc reseal stem <stem>` for each stem it names; settled when `./le rfc check` names no SHIFTED verdict |
| AC-30 | Every STALE audit verdict `./le rfc check` names at HEAD, outside AC-33's rows | Re-made by an independent `/ze-rfc-audit` agent (R-2), stamped with `audit-stamp ... mode rejudge`; a `weak` or `wrong` verdict sends its test or producer back to be fixed here. Settled when `./le rfc check` names no STALE verdict |
| AC-31 | Every record `./le rfc check` names as producer-gone | The function that now holds the removed producer's code is named from `git log -S` of the removal, the record is re-taken against it, and observed red; where no function holds it, the tag's claim no longer has a producer and goes to `ze-test-judge` with the evidence. Settled when `./le rfc check` names no producer-gone record |
| AC-32 | Rows no spec owned at the CI run: RFC4271-5-3 and RFC4271-5-4 (no negative test since 6c22530c23 removed four negative tags, its body carrying `RFC-approval-owed:` for three units), RFC8955-4-3 and RFC9252-3.3-2 (partial scope with no current verified mutant or revert), and RFC4271-9.2-2 and RFC4271-9.2-3, whose owner `spec-bgp-best-route-advertised-to-peers` closed in 56fb8b5663 with both rows' records stale and both verdicts naming a producer the code map no longer holds | RFC 4271 9.2-2 and 9.2-3: records re-taken under AC-28 and verdicts re-judged under AC-30 against their current producers. RFC 4271 5-3 and 5-4: a negative test that takes the case just outside each clause (`ai/META-DECISIONS.md`, "Test changes"), tagged and recorded, plus the owed judge verdicts; `{single-polarity}` is not used where a negative exists. RFC 8955 and 9252: a fresh record for each partial scope. Settled when `./le rfc check` names none of these rows |
| AC-33 | Rows another spec owns | Left to it, and not counted against this spec: RFC7752-3.4-1 and RFC9552-5.5-1 (`spec-bgp-next-hop-auto-rewrites-for-ebgp.md` AC-12); RFC2545-3-3's partial scope (AC-5, AC-6 here and `spec-rfc2545-next-hop-truncated-to-sixteen-bytes.md`); the filter-wire umbrella's rows. A row whose owning spec closes without clearing it falls to this spec |
| AC-34 | Closure | `./le rfc check` at HEAD names no stale record, no SHIFTED and no STALE verdict outside AC-33, and the commit-time refusal of 6eec153998 is still registered (`TestCommitGateIsRegistered`) |
| AC-27 (research amendment; design pending) | Explicit raw UPDATE body/full forms carrying valid labeled VPNv6 NLRI, with and without negotiated AFI 2 / SAFI 128 | The source finding below is decided by final emitted bytes with actual family capabilities and matching AS_PATH width. A useful negotiated VPNv6 route and same-session IPv4 control prevent blanket refusal from passing. Any confirmed capability-admission defect is fixed here after design review; mandatory level and applicability remain unchanged, and independent rejudgment follows observed results. |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPriorEvidenceReadsTheNewestCommitHoldingTheBacklog` | `internal/le/rfc/check_evidence_test.go` | prior evidence read at the newest commit holding the backlog file | |
| `TestRFCSelftestRealTreeRowReportsOwnedCheckFailures` | `internal/le/rfc/selftest_test.go` | the selftest reports owned check failures over the real tree | |
| Every tagged unit named in AC-1 to AC-6 | as named | its record's observed red | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N-A | no numeric input is added | N-A | N-A | N-A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `med-locally-set-reaches-peer` | `test/plugin/med-locally-set-reaches-peer.ci` | re-judged in AC-19, not changed | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bgp-rfc2545-linklocal-nexthop-frr` | `test/interop/scenarios/` | FRR | RFC 2545 Section 3 link-local next hop inclusion, red with `sharesNextHopSubnet` reverted (AC-4 to AC-6) | |

## Files to Modify
- `internal/le/rfc/` - the prior-evidence reader behind AC-26, and the selftest cost if AC-25 finds one
- `rfc/discrimination/rfc2328/RFC2328-4.4-3.json`, `RFC2328-D.3-1.json`, `rfc/discrimination/rfc2545/RFC2545-3-2.json`, `RFC2545-3-3.json` - written by `./le rfc record`
- `rfc/audit/rfc1661/`, `rfc2328/`, `rfc2516/`, `rfc2545/`, `rfc4271/`, `rfc4659/`, `rfc8907/`, `rfc1997/` - the verdict files of AC-7 to AC-24, written by the tools
- `rfc/discrimination/<stem>/` and `rfc/audit/<stem>/` for every stem `./le rfc check` names under AC-28 to AC-32, written only by `./le rfc record`, `reseal` and `audit-stamp`
- The RFC 4271 negative tests of AC-32, in the packages holding the 5-3 and 5-4 positives
- `internal/plugins/ospf/instance_enable_test.go` - the descriptive destination for AC-3; preserve the tag-or-rename choice and retire the historical misnamed source only through the rename commit's `remove`
- Any producer an audit or record shows defective, named in the commit that fixes it

## Files to Create
- None. The AC-3 descriptive destination is already prepared and listed above

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | no config |
| YANG validation constraints | N-A | no config |
| YANG custom validators | N-A | no config |
| CLI commands/flags | N-A | no command changes |
| CLI grammar (keyword before value) | N-A | no command changes |
| Editor autocomplete | N-A | no command changes |
| Functional test for new RPC/API | N-A | no RPC |
| Pipe completeness | N-A | no output |
| Env var registration | N-A | none |
| Doctor check for runtime dependencies | N-A | none |
| Prometheus counters/metrics | N-A | none |
| BGP family surface (new SAFI / capability / attribute) | N-A | none |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | proofs are restored, not widened |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | No | - |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes, if an audit verdict changes level | `rfc/short/<stem>.md` and `docs/features/rfc-status.md` only where a verdict moves a figure |
| 10 | Test infrastructure changed? | Yes, if AC-25 or AC-26 changes gate behaviour | `docs/contributing/rfc-conformance-gates.md` |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | No | - |
| 13 | Route metadata keys added/changed? | No | - |
| 14 | Prometheus counters added/changed? | No | - |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | - |
| 16 | Any changed source file referenced by existing doc source anchors? | Derived | `./le spec citation anchors spec plan/release/fixes/spec-rfc-proof-debt-outside-filter-wire.md`. `docs/architecture/ospf/ospf-ext-15-multi-af.md`, declared by the OSPF files AC-1 to AC-3 touch: unaffected, because no OSPF behaviour changes; a record or a rename leaves the multi-AF design it describes as it is |
| 17 | Existing docs show config/CLI/API examples for this area? | No | - |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the two tooling tests
   - Tests: `TestPriorEvidenceReadsTheNewestCommitHoldingTheBacklog`, `TestRFCSelftestRealTreeRowReportsOwnedCheckFailures`
   - Files: `internal/le/rfc/`
   - Verify: AC-26 red then green with the fix; AC-25 run alone and classified
2. **Phase: Unit records** -- AC-1, AC-2, AC-3
   - Verify: each record observes its red; `./le rfc check` drops the row
3. **Phase: Audits** -- AC-7 to AC-24, one independent `/ze-rfc-audit` agent per stem, rfc2328 after phase 2
   - Verify: stamped with `mode rejudge`; a `weak` or `wrong` verdict sends its test or producer back to be fixed in this spec
4. **Phase: Interop records** -- AC-4, AC-5, AC-6, when the owner reports a quiet host
   - Verify: scenario red with the break, the image rebuilt; `./le rfc check` drops all RFC 2545 lines
5. **Phase: Reseal** -- AC-29, mechanical and first among the backlog phases because it runs no test
   - Verify: `./le rfc check` names no SHIFTED verdict
6. **Phase: Backlog records** -- AC-28 and AC-31, batched by stem (largest first: rfc4271, rfc5176, rfc9494, rfc2545, rfc8277, rfc7311), one `./le rfc record` at a time in the main thread under `./le job run`; unit records before interop and QEMU records, which need a quiet host
   - Verify: per stem, `./le rfc check` names none of that stem's stale records; each stem's re-takes land in a commit of their own
7. **Phase: Backlog audits** -- AC-30, one independent auditor per stem, after that stem's records in phase 6
   - Verify: stamped with `mode rejudge`; `./le rfc check` names no STALE verdict
8. **Phase: Unowned rows** -- AC-32
   - Verify: the RFC 4271 negatives go red under their recorded break; `./le rfc check` names none of the four rows

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N is cleared by name in a `./le rfc check` run pasted in the closing commit |
| Correctness | No record, verdict or fingerprint file was edited by hand |
| Independence | No STALE row was re-judged by the author of its cause commit |
| Rule: `ai/rules/testing.md` | No tag, claim or assertion was weakened to obtain a red or a verdict |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| 26 rows cleared | `./le rfc check` output names none of the ids or files in AC-1 to AC-24 |
| Two tooling tests green | the two `./le job run` commands of AC-25 and AC-26 |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | N-A: tooling over the repository's own files |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup, through `ze-test-judge` |
| Record observes no red | The unit no longer discriminates: an audit verdict and a test or producer fix in this spec |
| Interop scenario times out | Load, not a verdict: wait for a quiet host |
| Audit verdict weak or wrong | Fix the test or the producer in this spec |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `guard-demands-what-the-model-cannot-supply.md` | 2026-11-15 | RFC document-author evidence cannot pass the role-binding model |
| `guard-added-to-one-half-of-a-pair.md` | 2026-11-15 | Explicit raw UPDATE VPNv6 family admission |

## Raw VPNv6 family-admission finding (source only, 2026-11-15)

The authoritative sentence read in `rfc/full/rfc4659.txt`, Section 3.4:
"In order for two PEs to exchange labeled IPv6 VPN NLRIs, they MUST use BGP
Capabilities Negotiation to ensure that they both are capable of properly
processing such NLRIs." Its next sentence specifies capability code 1 and
the AFI/SAFI from Section 3.2. Diagnostic intent supplies no exception in
that sentence. An unsettled raw-operation boundary is a research question,
not a waiver or a reason to exclude this implemented sender capability.

| Producer read | Current source evidence | Scope of the finding |
|---------------|-------------------------|----------------------|
| `internal/component/bgp/reactor/session_write.go`, `Session.SendRawMessage`; `session_aigp.go`, `writeOriginatedRawUpdate` | Recognized UPDATE body and full-packet forms reach `writeUpdateBody` with `raw=true`. A healthy third-party next hop with no AIGP reaches it directly. | Both explicit raw forms need the probe below. Use valid framing, negotiated-width AS_PATH and an accepting policy so another guard cannot decide the result. |
| `internal/component/bgp/reactor/session_ownership.go`, `adjOutWrite.section` | `SupportsFamily` admission is conditional on `!w.raw`; a registered VPNv6 splitter can account for its NLRI without adding a capability check. | Source evidence permits a valid raw VPNv6 announcement toward a recipient that negotiated only IPv4. No wire counterexample has been observed in this amendment. |
| `internal/component/bgp/reactor/session_write.go`, `writeUpdateBody` | Managed output applies `ownership.filter` results. Raw output only observes them and retains its body. | Generic `writeMessageHeld` passes `raw=false` and therefore has family admission; it is a useful control, not another claimed family bypass. |

### Minimal TDD probe owed before a design is accepted

| Working test name | File / entry point | Input, discriminator and useful controls |
|-------------------|--------------------|-----------------------------------------|
| `TestRawVPNv6RequiresNegotiatedFamily` | New descriptive test in `internal/component/bgp/reactor/`; actual `Session.SendRawMessage`, both body and full-packet forms | Use valid registered labeled VPNv6 NLRI, a healthy third-party next hop, accepting final policy and nonzero AS_PATH matching the session width. Compare a real AFI 2 / SAFI 128 negotiation with actual IPv4-only negotiation. Read final frames: negotiated VPNv6 emits the exact useful route; the IPv4-only peer emits no VPNv6 route. A successful useful IPv4 announcement on that same session proves ordinary output remains available. Preserve the six existing OPEN/negotiation/configured-send/receive subjects. |
| `TestGenericVPNv6FamilyAdmissionControl` | The same test carrier; generic `writeMessage` with `raw=false` | With the same valid route and capability sets, confirm the existing final family gate is reached and the useful IPv4 control survives. This distinguishes raw bypass from a broken fixture or unsupported splitter. |

These are research probes, not an approved implementation approach. No
probe, build, proof, audit stamp or ledger update ran for this amendment.
The main thread must observe the discriminating result, complete the
design review for the correction, repair any confirmed defect and obtain
an independent whole-row judgment. `docs/architecture/api/update-syntax.md`,
Raw Passthrough Commands, is the existing operator contract to reconcile in
that implementation. The pending rejudge JSON is retained in the active
next-hop session scratch until canonical disposition supersedes it.

## Document-author evidence finding

The RFC test campaign completed this investigation on 2026-11-15. It changes
no summary, extraction site, audit verdict, requirement level or support claim.
The finding is separate from the campaign's parser fault observations.

| Source | Full obligation | Current evidence defect |
|--------|-----------------|-------------------------|
| RFC 7606 Section 8, `RFC7606-8-1` | "A document that specifies a new BGP attribute MUST provide specifics regarding what constitutes an error for that attribute and how that error is to be handled." | The summary records a gap. Its audit names `message/rfc7606.go::validateAttribute`, although that dispatcher cannot prove what the defining document specifies. The audit records an earlier owner-invalidated exclusion; that history is not authority for restoring it. |
| RFC 5492 Section 4, `RFC5492-4-3` | "Processing of these capability instances is specific to the Capability Code and MUST be described in the document introducing the new capability." | The summary records a gap and no row audit exists. Extraction site `4:2` still says that the row is not applicable. That reason disagrees with the current summary and cannot authorize an exclusion. |

### Cause and correction boundary

`internal/le/rfc/notapplicable_roles.go::boundRoleRefusal` takes the text before
the first BCP 14 keyword as the subject. `subjectRefusal` rejects a subject
containing a `subjectBreakers` word. RFC 7606's restrictive subject contains
`that`. RFC 5492's pre-MUST text contains `of` and `and`; its defining document
appears after MUST in a passive complement. Neither section supplies a separate
scoping sentence accepted by the current model.

`docs/contributing/rfc-conformance-gates.md`, "The exclusion evidence ratchet",
explicitly calls these shapes decided rules. Its suggested gap-and-test route
does not resolve these document obligations: a decoder assertion can prove a
receiver's behavior, but cannot prove what another document contains. The
correction must change that contract through review, not force either sentence
into a runtime requirement or bypass the source checks.

| Existing mechanism | Treatment in the correction |
|--------------------|-----------------------------|
| `boundRoleRefusal` and `subjectRefusal` | Review a narrow document-author form against a general clause parser. Prefer the smallest source-bound representation that preserves the complete condition; do not remove breaker words globally. No alternative is approved here. |
| `check_evidence.go::evidenceReader.roleQuoteGap` and `ownObligationRefusal` | Preserve complete-sentence, cited-section, own-clause and original-level checks. A title, borrowed clause or shortened quote remains insufficient. |
| Native extraction and audit writers | Independently rejudge both rows after the reviewed correction. Reconcile summary, extraction reason and audit together; retain permanent ids and MUST levels. Do not create runtime discrimination records for document content. |

### Acceptance and test plan for the reviewed correction

| Criterion | Observable result and controls |
|-----------|--------------------------------|
| DA-1 | Exact complete RFC 7606 Section 8 and RFC 5492 Section 4 sentences can represent their document obligation through the reviewed evidence route. No arbitrary speaker obligation gains that route. |
| DA-2 | Tests in `internal/le/rfc/notapplicable_roles_test.go` cover the restrictive-subject and passive-document forms separately. Nearby receiver obligations remain applicable, including RFC 5492 Section 4's acceptance of multiple Capabilities Optional Parameters. |
| DA-3 | Tests in `internal/le/rfc/check_evidence_test.go` refuse a missing source, wrong section, incomplete sentence, dropped condition, borrowed obligation, altered level, generic actor or role Ze fills. Existing valid role evidence remains accepted. Each new guard has an observed fault that makes its test fail. |
| DA-4 | An independent RFC auditor reads the defining sentences and current Ze roles, checks the historical owner ruling, and records the resulting disposition through native writers. No whole-RFC completion or runtime behavior is inferred. |
| DA-5 | The correction updates "The exclusion evidence ratchet" in `docs/contributing/rfc-conformance-gates.md` in the same change. The document states the admitted evidence form and its refusal cases. |

Until the design and independent rejudgment are complete, both rows remain
disclosed findings. No decoder test is invented to clear them. The repair needs
no protocol, SDK, schema, daemon command or wire-format change.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| RFC8212-3-1 interop record left to the filter-wire umbrella | listing it here as briefed | its producer `filterapi/policy.go::AcceptsAtEnd` moved in 2fd7958d6f and fa54cd4285, both filter-wire commits |
| One spec for all non-filter rows | one spec per stem | each row is the same mechanical route through the same tools; per-stem phases keep the audits independent |

## Known Limitations
- The other SHIFTED rows of the umbrella's 98-violation run are not classified in this spec; each is cleared by `./le rfc reseal stem <stem>` by whoever owns its cause.

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
- [ ] AC-1..AC-26 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Interop tests for protocol features (AC-4 to AC-6): `./le test integration interop` with `INTEROP_SCENARIO=bgp-rfc2545-linklocal-nexthop-frr`, quiet host only
- [ ] Scoped OSPF transport run for AC-1 and AC-2: `./le job run label ospf-transport command go test -tags ze_ospf ./internal/plugins/ospf/transport/`

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] **Commit A:** records + verdicts + code + edited spec
- [ ] **Commit B:** `remove plan/release/fixes/spec-rfc-proof-debt-outside-filter-wire.md` only
