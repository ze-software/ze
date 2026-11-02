# Spec: dropped-error-4-system-plugins

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | plugin |
| Priority | P1 |
| Release reason | 45 errors in the system plugins and the ExaBGP bridge (TFTP, image and DHCP servers, NTP, the DNS doctor probes, flow export, DDoS detection, VRRP telemetry) are caught and logged only below WARN: an NTP sync that fails, a doctor probe that cannot bind, an ack an ExaBGP script never receives, each with no trace at the default level |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-02 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Child of `plan/release/fixes/spec-dropped-error-0-umbrella.md`, which holds the
resolution policy and the tree-wide acceptance. Door: `ai/doors/system-plugins.md`.

Population: every `error-dropped-below-warn` finding of
`./le arch refuse-by-default report` under `internal/plugins/` and
`internal/exabgp/`, outside the routing-protocol plugins (`ospf`, `isis`,
`ldp`, `rsvpte`, child 6) and the forwarding plugins (`iface`, `fib`,
`firewall`, `traffic`, `policyroute`, child 5). At HEAD on 2026-11-02:
**45 instances in 24 files** (TFTP 10, ExaBGP bridge and runner 11, image
server 4, NTP 5, DHCP server 3, the three DNS doctor probes 6, flow export 3,
DDoS detect 2, VRRP 1).

Each instance is resolved by exactly one answer, judged in this order
(umbrella, Key Design Decisions):

| Order | When | Answer |
|-------|------|--------|
| 1 | Going on is wrong: the caller would act on the failure | Return the error, or fix the behavior the drop hid. A doctor probe (`probeBindable` in `as112`, `dnsresolver`, `geodns`) reports the failure in its check result, since an operator runs `ze doctor` to see exactly that |
| 2 | Going on is right and an operator can act | WARN with what, why and the fix (`ai/rules/user-facing-errors.md`); on a per-packet path (TFTP, DHCP), count it and WARN once per condition |
| 3 | Going on is right and nobody can act (a reply to a TFTP client that left, a close at teardown) | `// refuse-by-default: guards <Symbol> -- <why>` on the line above |

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system-plugins.md`
  → Constraint: every plugin here can run forked; a WARN goes to the plugin's logger, never to a process-local store
  → Constraint: the ExaBGP bridge's contract is ExaBGP's text and JSON; an error made visible to the script changes nothing it reads unless ExaBGP would send it too
  → Constraint: DHCP, TFTP and PXE clients are boot firmware Ze cannot change; no wire change here
- [ ] `docs/architecture/exabgp-bridge.md` - the two runners, ack modes
  → Constraint: an ack write failure means the script's pipe is gone; the bridge's own lifecycle handles that
- [ ] `ai/rules/user-facing-errors.md`
  → Constraint: what, why, fix
- [ ] `docs/contributing/ze-go-style.md` - "Error dropped below WARN"
  → Constraint: return, count, or WARN; the annotation names a guard

**Key insights:**
- The bridge ack writes go to the script's stdin: when that write fails the script has exited, which the runner already reaps, so those are answer 3 naming the reaper as the guard, unless the read shows otherwise.
- TFTP replies to a client that sent a bad request are answer 3; a server-side read failure is answer 2.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/arch/refusebydefault/dropped.go` - the shape
- [ ] `internal/plugins/tftpserver/handler.go` - a failed write of the "write not supported" error reply is logged at Debug

**Behavior to preserve:**
- TFTP, DHCP, NTP and DNS wire behavior; the ExaBGP script's input and output.

**Behavior to change:**
- Each listed instance resolved by one answer.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| refuse-by-default dropped-error shape | `internal/le/arch/refusebydefault/dropped.go` `droppedError` | Lists these 45 | Applies: the acceptance check |
| Guard annotation | `internal/le/arch/refusebydefault/annotation.go` | Accepts a named guard | Applies: answer 3 only |
| Doctor checks | `probeBindable` in `internal/plugins/{as112,dnsresolver,geodns}/doctor.go`, `internal/core/diagnostic` | Report runtime dependencies to `ze doctor` | Extends: a probe failure becomes the check's result (answer 1) |
| Plugin metrics | `docs/plugin-development/metrics.md` | Per-plugin counters | Extends: answer 2 on per-packet paths counts here |
| Bridge script reaper | `internal/plugins/exabgp/bridgerun/script.go` `script.reap` | Notices a script that exited | Applies: the guard an answer-3 ack annotation names, if the read confirms it |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A failed call inside a system plugin or the ExaBGP bridge: a socket write, a sync, a bind probe, a state write, a module load.

### Transformation Path
1. The branch catches the error.
2. It returns it (a doctor probe reports it), counts it and WARNs once, WARNs, or carries the annotation.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Plugin ↔ operator | Plugin log at WARN, `ze doctor`, metrics | No |
| Bridge ↔ ExaBGP script | stdin and stdout of the script | No |

### Integration Points
- The plugin loggers, doctor checks and metric families named above.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Existing loggers and doctor checks |
| No unintended coupling (components stay isolated) | Yes | No new import |
| No duplicated functionality (extends existing, does not recreate) | Yes | No new reporting path |
| Zero-copy preserved where applicable (refs, not copies) | Yes | Per-packet paths count |
| Registration over hardcoding, outbound | N-A | Nothing registers |
| Registration over hardcoding, inbound | N-A | No list learns a name |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The 45 are still the population | Report of 2026-11-02 | Work from the fresh report | Report filtered to this child's paths before the first edit | unvalidated |
| A-2 | A failed ack write means the script has exited and the reaper reports it | Door: the bridge's two runners | An ack lost to a live script is answer 2 | Read `bridgerun/script.go` reap and write loop | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A WARN per TFTP or DHCP packet floods under a misbehaving client | A `test/dhcp` or TFTP `.ci` log grows per packet | Count per packet, WARN once per client and condition |
| R-2 | A doctor probe now reports a failure on a host where binding is refused by design (an unprivileged test run) | A doctor `.ci` turns red | The report is the truth; the `.ci` runs where the probe can bind, or asserts the reported condition |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Log noise, or a doctor check that newly reports a failure |
| How is it reverted? | Single commit revert |
| Who else touches this path? | Specs on TFTP uploads, NTP and the ExaBGP bridge (`spec-exabgp-bridge-refuses-untranslated`) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le arch refuse-by-default report` | → | `droppedError` over this child's paths | Report filtered to this child's paths: zero findings of the shape |
| `ze doctor` on a host where the DNS port cannot be bound | → | `probeBindable` (dnsresolver) | `TestDNSResolverDoctorReportsUnbindablePort` |
| An NTP server that does not answer | → | `syncWorker.doSync` | `TestNTPSyncFailureIsWarned` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `./le arch refuse-by-default report '|' json`, filtered to this child's paths | No `findings` entry with rule `error-dropped-below-warn` |
| AC-2 | A doctor probe that cannot bind | `ze doctor` reports the condition with its fix, for `as112`, `dnsresolver` and `geodns` |
| AC-3 | Each other answer-1 instance | A unit test where the caller observes the error |
| AC-4 | Each file with an answer-2 WARN | A unit test asserts one WARN line names what, why and the fix |
| AC-5 | Each annotation | Names a guard the file can see |
| AC-6 | Package tests and the `test/dhcp`, `test/exabgp` and plugin `.ci` the change reaches | Green |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDNSResolverDoctorReportsUnbindablePort` | `internal/plugins/dnsresolver/doctor_test.go` | AC-2; the same for `as112`, `geodns` | |
| `TestNTPSyncFailureIsWarned` | `internal/plugins/ntp/ntp_test.go` | AC-4 | |
| One test per other answer-1 instance | The package of the instance | AC-3 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| unannotated findings under this child's paths | 0 | 0 | N/A | 1 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `.ci` the change reaches under `test/dhcp`, `test/exabgp`, `test/plugin` | `test/` | Unchanged behavior; doctor reports the bind failure | |

## Files to Modify
- `internal/exabgp/bridge/bridge.go` - 1: `Bridge.Run.func` 512
- `internal/exabgp/bridge/bridge_ack.go` - 4: `AckMode.AnswerLocal` 56; `AckMode.WriteAck` 113; `AckMode.WriteError` 144; `AckMode.WriteErrorMarker` 156
- `internal/exabgp/bridge/bridge_batch.go` - 1: `writeReply` 514
- `internal/exabgp/bridge/bridge_ping.go` - 1: `writePong` 83
- `internal/exabgp/bridge/bridge_status.go` - 1: `statusInfo.write` 201
- `internal/plugins/as112/doctor.go` - 2: `probeBindable` 182,189
- `internal/plugins/ddos/detect/detector.go` - 2: `detector.saveBaseline` 238; `detector.classifyDirection` 544
- `internal/plugins/dhcpserver/register.go` - 3: `closeLogged` 123; `serveRequests` 285; `replyDestination` 379
- `internal/plugins/dnsresolver/doctor.go` - 2: `probeBindable` 147,154
- `internal/plugins/exabgp/bridgerun/script.go` - 3: `script.reap` 452; `script.writeLoop` 481,490
- `internal/plugins/flowexport/conntrack_setup_appliance_linux.go` - 1: `loadConntrackModules` 69
- `internal/plugins/flowexport/conntrack_worker.go` - 1: `conntrackWorker.Start` 120
- `internal/plugins/flowexport/sampling_worker.go` - 1: `samplingWorker.Stop` 141
- `internal/plugins/geodns/doctor.go` - 2: `probeBindable` 190,197
- `internal/plugins/imageserver/config_tx.go` - 3: `prepareZefs` 175; `removeZefsDir` 191; `imageServer.closeServers` 199
- `internal/plugins/imageserver/register.go` - 1: `closeLogged` 315
- `internal/plugins/ntp/ntp.go` - 4: `syncWorker.doSync` 257,263,270; `syncWorker.handleDHCPEvent` 392
- `internal/plugins/ntp/register.go` - 1: `runNTPPlugin` 285
- `internal/plugins/tftpserver/config_tx.go` - 1: `listenInterface` 198
- `internal/plugins/tftpserver/handler.go` - 4: `serveRequests` 274; `handleRRQ.func` 287; `runTransfer` 397; `sendOACKAndWait` 452
- `internal/plugins/tftpserver/panic_guard.go` - 1: `signalPanic` 66
- `internal/plugins/tftpserver/register.go` - 1: `closeLogged` 95
- `internal/plugins/tftpserver/window_send.go` - 3: `serveFile` 33; `serveFile.func` 39; `windowSender.sendWindow` 128
- `internal/plugins/vrrp/telemetry.go` - 1: `publishStateChange` 140

Lines are those of HEAD on 2026-11-02; the report is the current list.

## Files to Create
- Test additions in the packages above; no new source file.

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | No config |
| YANG validation constraints | N-A | No leaf |
| YANG custom validators | N-A | No leaf |
| CLI commands/flags | N-A | No command |
| CLI grammar (keyword before value) | N-A | No command |
| Editor autocomplete | N-A | No command |
| Functional test for new RPC/API | N-A | No RPC |
| Pipe completeness | N-A | No output change |
| Env var registration | N-A | No env var |
| Doctor check for runtime dependencies | Yes | The three existing `probeBindable` checks now report their failure; no new check |
| Prometheus counters/metrics | Yes, if answer 2 counts | Existing plugin families; name each counter used at implementation |
| BGP family surface (new SAFI / capability / attribute) | N-A | None |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | No | |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | No | |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | No | |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | If answer 2 adds a counter | `docs/plugin-development/metrics.md` |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Declared `// Design:` docs: `ai/digests/flow-ddos.md`, `docs/architecture/config/transaction-protocol.md`, `docs/architecture/core-design.md`, `docs/architecture/ddos/cp-survival-5-detect-0-umbrella.md`, `docs/architecture/dns/as112.md`, `docs/architecture/dns/geodns.md`, `docs/architecture/dns/lan-resolver.md`, `docs/architecture/exabgp-bridge.md`, `docs/architecture/flowexport/flow-export-2-flow-records.md`, `docs/architecture/provisioning/dhcp-server.md`, `docs/architecture/provisioning/image-server.md`, `docs/architecture/provisioning/tftp-server.md`, `docs/architecture/vrrp/vrrp-first-hop-redundancy.md`, `docs/features/interfaces.md`. The DNS pages that describe the doctor probes change if the probe's report changes; confirm with `./le spec citation anchors spec plan/release/fixes/spec-dropped-error-4-system-plugins.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | No | |

## Implementation Steps

1. **Phase: Wiring** -- run the report filtered to this child's paths; write the doctor and NTP tests red.
2. **Phase: Doctor probes** -- answer 1 for the three `probeBindable`.
3. **Phase: Classify and resolve** -- every other instance gets its answer and reason in Design Insights; tests first for answers 1 and 2; annotations last.
4. **Phase: Prove** -- AC-1 to AC-6.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every instance has its answer at file:line |
| Correctness | No change to what an ExaBGP script, a TFTP or a DHCP client reads |
| Rule: user-facing-errors | Each WARN and each doctor result names the operation, why and the fix |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Zero findings under this child's paths | `./le arch refuse-by-default report '|' json` filtered by path |
| Package tests | `./le job run label system-plugins command go test -tags <the tags ./le test unit uses> ./internal/plugins/... ./internal/exabgp/...` limited to the changed packages |
| Functional | `./le test dhcp`, `./le test exabgp`, `./le test bgp plugin <name>` for the `.ci` the change reaches |
| Change set judged | `./le verify changed` after the closing commit |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | A WARN that echoes an unbounded client filename or payload |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|

The class row for the whole population is owned by the umbrella.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| A doctor probe reports its failure as the check result | WARN from the probe | `ze doctor` is where an operator looks for exactly this condition |
| The umbrella's three answers, in order, elsewhere | Raise every line to WARN | A WARN does not make a default correct |

## Known Limitations
- None.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row

### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, and `./le verify changed` over the closing commits
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason): N-A, no wire change

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] **Commit A:** code + tests + docs + edited spec
- [ ] **Commit B:** `remove plan/release/fixes/spec-dropped-error-4-system-plugins.md` only, in the same `./le commit create` script
