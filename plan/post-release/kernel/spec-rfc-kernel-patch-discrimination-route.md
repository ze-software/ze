# Spec: rfc-kernel-patch-discrimination-route

| Field | Value |
|-------|-------|
| Status | design |
| Scope | tooling |
| Priority | P3 |
| Release reason | Proof tooling: it turns escaped RFC rows into proven ones and changes no shipped behavior, so it waits until after the first release |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-29 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Some RFC requirements Ze meets are produced by the Linux kernel from the packet
alone: the forwarding path discards a TTL-expired datagram, the option parser
refuses a bad Record Route pointer, `__icmp_send` writes the unused word of an
ICMP error. No state Ze installs decides the outcome, so no break of Ze's own
code turns the tagged test red. Today those rows take the `kernel-producer`
escape (`docs/contributing/rfc-conformance-gates.md`, "The escape, and the
precondition behind each reason"), and the ledger publishes them as escaped,
which is debt and never proof.

A break does exist for them, in the kernel. This spec designs the
patched-kernel discrimination route: the tagged unit runs clean on Ze's runtime
kernel, then runs on a kernel built with ONE named patch that breaks the
producing kernel function, and the record is written only when the second run
fails naming that unit. Every `kernel-producer` escape already records
`later-route: patched-kernel`, so the route's population is known before it is
built.

Goal: every `kernel-producer` record can be replaced by a `patched-kernel`
proof record that `./le rfc check` re-verifies, and the escaped count for those
rows falls to zero.

### First population

The seven RFC 792 rows escaped on 2026-10-29, all on
`internal/plugins/vrrp/rfc792_gateway_icmp_integration_linux_test.go`:

| Requirement | Polarity | Tagged unit | Kernel functions |
|-------------|----------|-------------|------------------|
| `RFC792-Format-1` | positive | `TestGatewayICMPDFDiscard` | `net/ipv4/icmp.c::__icmp_send` |
| `RFC792-Format-1` | positive | `TestGatewayICMPTTLDiscard` | `net/ipv4/icmp.c::__icmp_send` |
| `RFC792-Format-1` | positive | `TestGatewayICMPHeaderDiscard` | `net/ipv4/icmp.c::__icmp_send` |
| `RFC792-TimeExceeded-1` | positive | `TestGatewayICMPTTLDiscard` | `net/ipv4/ip_forward.c::ip_forward` |
| `RFC792-ParamProblem-1` | positive | `TestGatewayICMPHeaderDiscard` | `net/ipv4/ip_options.c::__ip_options_compile`, `net/ipv4/ip_input.c::ip_rcv_core` |
| `RFC792-TimeExceeded-1` | negative | `TestGatewayICMPTTLDiscard` | `net/ipv4/ip_forward.c::ip_forward` |
| `RFC792-ParamProblem-1` | negative | `TestGatewayICMPHeaderDiscard` | `net/ipv4/ip_options.c::__ip_options_compile`, `net/ipv4/ip_input.c::ip_rcv_core` |

The two negative rows assert that the control datagram (TTL two, or the
otherwise identical Record Route option with a valid pointer) forwards out
`gw1` over the route Ze's netlink backend installs (`netlinkBackend.AddRoute`).
That route is the control the claim holds constant, never the operand of the
TTL or option decision, so breaking it does not falsify the claim: an
independent judge refused both route breaks on 2026-10-29. Each row takes the
escape, with `AddRoute` named in a `ze-state-excluded` entry.

### Finding the rest

The population is every discrimination record whose `reason` is
`kernel-producer`. `./le rfc discriminate stem <stem>` lists a stem's records
with their `reason`, `kernel-functions` and `later-route` columns, and the files
are `rfc/discrimination/<stem>/<rid>.json`. No second list is kept: a new
`kernel-producer` escape joins the population when it is recorded, and a row
leaves it when its proof record replaces the escape.

## Required Reading

### Architecture Docs
- [ ] `docs/contributing/rfc-conformance-gates.md` - the record, its fields, the two proof routes and the escape
  → Constraint: a record is written only from an observed red that names the tagged unit; a hand-written fingerprint is refused
  → Constraint: a paired break carries solo runs proving each break is needed
  → Decision: `kernel-producer` names `kernel-functions` and `later-route: patched-kernel`; this route consumes both
- [ ] `rfc/discrimination/README.md` - the artifact contract, derived versus authored fields
  → Constraint: a new field is either authored or derived, and the page says which
- [ ] `ai/rules/platform-linux.md` - kernel work and the runtime kernel
  → Constraint: kernel work uses the architecture of the machine the agent is on; the other architecture's kernel is never cross-compiled or emulated locally (`ai/META-DECISIONS.md`, "Test systems and labs")
- [ ] `ai/rules/rfc-compliance.md` - a requirement met through a lower layer still owes a test of what Ze installs, and a discrimination record per tag
  → Constraint: the route proves the unit discriminates the kernel's behavior; it does not change which layer the summary declares

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc792.md` - the first population's summary
  → Constraint: its `Implementation reason` names each kernel function in the spelling a record uses

**Key insights:**
- The guest route already exists for a Linux-only unit: `./le rfc record ... kernel <path>` boots the runtime kernel in QEMU and runs the unit there (`discriminate_guest.go`). The patched route adds a second boot on a patched image.
- The break is in C, outside this repository's Go overlay, so the overlay mechanism of the mutant and revert routes does not apply.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/rfc/discriminate_escape_kernel.go` - checks a `kernel-producer` escape: Linux-only unit carrier, no producer, summary of kind mixed or third-party naming the kernel function, citation inside the claim
- [ ] `internal/le/rfc/discriminate_validate.go` - `validateKernelEscape` and `refuseStrayKernelFields`, the record shape
- [ ] `internal/le/rfc/discriminate_guest.go` - `unitNeedsGuest`, `requireGuestKernel`, `runInGuest`: a Linux-only unit runs in a QEMU guest booting the runtime kernel named by `kernel <path>`
- [ ] `internal/le/rfc/discriminate_action.go` - `recordDiscrimination`, `observeFor`, `sealAndStore`: check, observe, seal, write
- [ ] `internal/le/rfc/rfc.go` - `escapeReasons`, `laterRoutes`, `discriminationRoutes`

**Behavior to preserve:**
- Every existing route, record and escape loads and verifies unchanged.
- A `kernel-producer` escape stays valid until a proof record replaces it.
- `./le rfc check` stays offline: it re-verifies fingerprints and never boots a kernel.

**Behavior to change:**
- A new proof route, `patched-kernel`, joins `discriminationRoutes`; its records count as proven.
- `./le rfc record` gains the route: it builds or takes a patched kernel image, boots both kernels, and writes the record from the observed clean and broken runs.

## Existing Mechanisms (MANDATORY)

The following source was read for the unapproved design refresh below. These
rows describe current mechanisms, not implementation of this proposal.

| Mechanism | Location | What it does today | Proposed disposition |
|-----------|----------|--------------------|----------------------|
| Native recorder | `internal/le/rfc/discriminate_action.go` `recordDiscrimination`, `observeFor`, `sealAndStore` | Collects the cover, applies a Go break, observes, fingerprints the current tree, validates and merges one requirement's records | Extend the same entry point and writer with a route-specific observation and pre-observation snapshot; no second record writer |
| Typed record and offline verifier | `internal/le/rfc/discriminate.go` `DiscriminationRecord`, `readDiscriminationRecords`, `verifyOneDiscrimination` | Rejects unknown JSON fields and verifies unit, claim and Go producer hashes without rerunning tests | Add a typed `kernel-proof` payload only for the new route; keep all existing route-specific refusals |
| Clean/broken observation | `internal/le/rfc/discriminate_observe.go` `observeBreaks`, `judgeRed`, `attribution` | Requires clean green, then a red attributed to the unit; the Go-unit path accepts the unit's FAIL line | Reuse lifecycle and selection, but require exact assertion and prerequisite/control receipts for C proof |
| Guest carrier | `internal/le/rfc/discriminate_guest.go` `runInGuest`, `guestRunArgv` | Compiles each invocation separately and selects one kernel plus an optional image | Extend with one compiled binary observed on two explicit kernel/image pairs |
| Native kernel builder | `internal/appliance/kernelbuilder/driver.go` `Build`; `worker.go` `RunWorker`, `applyPatches`; `recipe.go` `Request`, `writeProvenance` | Builds natively, applies one complete ordered series with zero fuzz, owns and removes its worker workspace; provenance records source pin and basic build dimensions | Extend with one additive proof patch and a complete build receipt; retain production series and cleanup ownership |
| Selected-kernel image packaging | `internal/appliance/instance/prepare.go` `Prepare`; `internal/le/setup/docker/host/build.go` `builder.build`; `internal/le/build/gokrazy/gokrazy.go` `prepareArgs` | `Prepare` accepts `KernelTree`, but image keying and nested packing independently resolve the current runtime tree | Carry one explicit selected tree through both keying and packing, and emit an image receipt |
| Own-system execution guard | `internal/appliance/runtimekernel_existing.go` `ExistingRuntimeKernelTree`; `internal/le/test/qemu/dockerlab_command.go` `dockerLabCommandKernel`, `dockerLabKernelPayload`, `Run.dockerLabCommand` | Checks selected cache identity, exact kernel bytes, one module release/index, image boot payload, acquired payload bytes and running guest identity | Retain every guard; add receipt checks before boot, with no arbitrary-image bypass |
| Proof consumers | `internal/le/rfc/check_baseline.go` `baselineRecordBlobs`; `check_audit.go` `partialCoverRefusals`; `profilecoverage.go` `osProofsOf`; `weekly_source.go` `weeklyProving`; `internal/le/site/rfcledger.go` `rfcProofsByCover` | Baseline freshness, partial scope, OS-state proof credit, endpoint reports and public proof detail use different record projections | Extend each affected projection explicitly; fingerprinted setup code never receives credit for a C break |
| Drift ratchet and diagnostic routing | `internal/le/rfc/check_ratchets.go` `checkDiscriminationRatchet`, `driftedKey`, `driftIsCommitted`, `discriminationDrifted`, `discriminationStaleError`; `check.go` report assembly | Selects one moved Go key, compares HEAD and working-tree behavior, refuses committed drift and reports uncommitted drift; stale diagnostics print Go break/producer | Add typed kernel-proof dependency identity and route-specific comparison/diagnostics; preserve old-route treatment and exclude every stale verdict from proof counts |
| Runtime cache reachability/publication | `internal/appliance/cmd_kernel.go` `runKernel`, `resolveKernel`, `resolveRuntimeKernel`, `defaultKernelBuild`; `cache.go` `kernelCacheVariantFor`, `kernelTreeCachePath` | Normal dimensions only; an existing vmlinuz returns before build, and cold output is copied to the normal variant | Add a registered preparation caller and a typed proof-only publication path; never send a fault build through the normal resolver/key |
| Image cache hit | `internal/le/setup/docker/host/build.go` `builder.build`; `pins.go` `KeyInputs`, `ImageKey` | Keys selected vmlinuz bytes, but not the module output set; an existing image path returns on Stat alone | Key the full actual selected output manifest and typed identity, including native receipt identity when available; proof additionally requires the native receipt. Validate image receipt and actual raw image bytes on hit |
| Consumer fixture ownership and package boundary | Engine `rfc4301_espform_policy_integration_linux_test.go` `newEspPolLab`, `espPolLab.enter`; `discriminate_guest.go` `guestBuildArgv`; dataplane `rfc4301_complete_scope_integration_linux_test.go` and `internal/test/userns/userns_linux.go` `Enter` | Lab locks its caller's thread and enters namespaces directly; selected guest binary compiles one package. The separate dataplane admission test re-executes privately and logs child output | Admit only instrumentation that leaves namespace work on the fixture owner and uses pure assertion leaves; engine prerequisites come from its own fixture, not another package/binary or embedded child logs |
| Function-local fingerprints | `internal/le/rfc/goscope.go` `scopeIndex.keyTexts`, `funcTexts`; `tags.go` `behaviorBytes`; `discriminate.go` seal/verify | Hashes the named declaration's supplied text, not its transitive helper/global/fixture inputs | Preserve old Go-route semantics; new patched-kernel proof additionally binds compiler-derived observer/test input files and population |
| Compiler-selected source and membership collection | `internal/le/build/compile/inputs.go` `DaemonInputs`, `selectedInputs`; `internal/le/go/lint/verifylint.go` `Runner.goList`; `planinputs.go` `queryInputs`, `lintPopulation`; `internal/le/job/inputsnapshot.go` | Compiler JSON supplies source/dependency/embed files; lint population tracks package directories/embed trees; content snapshots detect changed reached inputs | Extend the existing compiler collector with one selected-test-package mode and reuse the content/population mechanisms; retain a raw source/membership/context manifest for offline new-route freshness |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le rfc record id <rid> polarity <p> unit <unit> route patched-kernel kernel <runtime image> kernel-function <path::fn> patch <patch file>`
- The patch is a unified diff against the Linux source at the runtime kernel's release, stored in the tree.

### Transformation Path
1. Shape check: the unit is a Linux-only Go unit, the patch touches only the span of one `kernel-function` the record names, in its file, the release the patch targets equals the runtime kernel's release.
2. Build: the patched kernel is built on the agent's own architecture from the same source and config as the runtime kernel, with only the patch added.
3. Clean run: the unit runs in the QEMU guest on the runtime kernel and passes.
4. Broken run: the unit runs in the guest on the patched kernel and fails naming the unit.
5. Seal: the record carries the unit and claim fingerprints, the kernel release, the patch fingerprint, the kernel function's body fingerprint at that release, and the clean and broken run lines.
6. The `kernel-producer` escape for the same requirement, polarity and unit is removed in the same write.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| le ↔ kernel build | the existing kernel build action, given one extra patch | No |
| le ↔ QEMU guest | `le test qemu run kernel <image>`, as the guest route does today | No |

### Integration Points
- `discriminationRoutes` - the new route registers there, so validation, counts and reports derive from it.
- `observeFor` - the route supplies its own observation runner beside the guest runner.
- `checkDiscriminationRatchet` - a proof record replacing an escape is a gain, never a loss.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | the record is written only by `./le rfc record`, through `sealAndStore` |
| No unintended coupling (components stay isolated) | Yes | the route lives in `internal/le/rfc`; no product package changes |
| No duplicated functionality (extends existing, does not recreate) | Yes | the guest boot is reused; only the image differs |
| Zero-copy preserved where applicable (refs, not copies) | N-A | tooling, no wire path |
| Registration over hardcoding, outbound | Yes | the route is one entry in `discriminationRoutes` and `laterRoutes` already names it |
| Registration over hardcoding, inbound | Yes | reports read the route vocabulary; the searched lists are `discriminationRoutes`, `laterRoutes`, `escapeReasons` |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Ze's runtime kernel source and config can be rebuilt with one extra patch on the agent's architecture | the gokrazy and lab kernel builds exist (`./le build gokrazy`, `./le setup docker kernel`) | the route needs its own kernel build action first | a patched build of the runtime release on the Mac host's architecture | unvalidated |
| A-2 | One patch confined to the named function turns each first-population unit red | the function bodies read on 2026-10-29: `ip_forward` checks `ttl <= 1`, `__ip_options_compile` sets `pp_ptr`, `__icmp_send` fills `un.gateway` | the escape's kernel function is wrong and the record names another | the broken run of each of the seven rows | unvalidated |
| A-3 | The guest already boots an arbitrary kernel image | `requireGuestKernel` and `guestRunArgv` in `discriminate_guest.go` | the guest action needs a parameter for the image | a boot of the patched image through `le test qemu run` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A patch that breaks more than the named function makes a red the claim does not explain | the patch touches lines outside the function's span | the shape check refuses any hunk outside the span |
| R-2 | A kernel bump moves the function, and the record proves a body that no longer runs | the function fingerprint at the new release differs | the record goes stale on a release change and is re-recorded, as a producer change stales a Go record |
| R-3 | The patched build takes long enough to starve shared hosts | build time over the job admission budget | builds run on the Mac only, one at a time, and the image is cached by patch fingerprint |
| R-4 | A negative-polarity row has no single-function break that turns it red without breaking forwarding wholesale | the broken run fails for every unit in the package | the record's citation names the claim span the break targets, and a reviewer reads the break |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Published proof counts: a wrong route publishes a row as proven that nothing proved. No shipped binary changes |
| How is it reverted? | Single commit revert of the route; the escapes it replaced come back from history |
| Who else touches this path? | Any session recording discrimination records in `internal/le/rfc` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le rfc record ... route patched-kernel` | → | the patched-kernel observation runner and `sealAndStore` | `TestPatchedKernelRouteRecordsOnlyAnObservedRed` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A patch whose hunks fall outside the named kernel function | refused before any build, naming the hunk and the function span |
| AC-2 | A patch targeting a release other than the runtime kernel's | refused, naming both releases |
| AC-3 | The unit stays green on the patched kernel | no record is written, and the command answers 2 naming the green run |
| AC-4 | The unit fails on the runtime kernel | no record is written: the clean run must pass |
| AC-5 | Clean on the runtime kernel and red naming the unit on the patched kernel | a `patched-kernel` record is written, the matching `kernel-producer` escape is removed, and `./le rfc discriminate stem <stem>` lists the row as proven |
| AC-6 | The runtime kernel release changes | `./le rfc check` reports the record stale, as a moved producer stales a Go record |
| AC-7 | The seven first-population rows | each carries a `patched-kernel` record and none carries a `kernel-producer` escape |
| AC-8 | A build requested for an architecture other than the host's | refused, naming the host architecture |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPatchedKernelRouteRecordsOnlyAnObservedRed` | `internal/le/rfc/discriminate_kernel_patch_test.go` | AC-3, AC-4, AC-5 with a fake runner | |
| `TestKernelPatchConfinedToTheFunction` | `internal/le/rfc/discriminate_kernel_patch_test.go` | AC-1 | |
| `TestKernelPatchTargetsTheRuntimeRelease` | `internal/le/rfc/discriminate_kernel_patch_test.go` | AC-2, AC-6 | |
| `TestKernelPatchBuildsOnTheHostArchitecture` | `internal/le/rfc/discriminate_kernel_patch_test.go` | AC-8 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| patch hunk line range | inside the function span | the function's last line | the line before the function | the line after the function |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| the seven RFC 792 recordings | `rfc/discrimination/rfc792/` | an author records each row on the Mac and `./le rfc check` verifies it | |

## Files to Modify
- `internal/le/rfc/rfc.go` - the `patched-kernel` route in `discriminationRoutes`
- `internal/le/rfc/discriminate.go` - record fields for the kernel release, patch fingerprint and function fingerprint
- `internal/le/rfc/discriminate_validate.go` - the route's record shape
- `internal/le/rfc/discriminate_action.go` - the route in `observeFor`, the escape replaced on write
- `internal/le/rfc/actions.go` - the `patch` keyword
- `docs/contributing/rfc-conformance-gates.md` - the third proof route
- `rfc/discrimination/README.md` - the new fields

## Files to Create
- `internal/le/rfc/discriminate_kernel_patch.go` - the patch shape check, the build and the two boots
- `internal/le/rfc/discriminate_kernel_patch_test.go` - the unit tests above
- one patch file per first-population row, under the stem's discrimination directory

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | le tooling, no YANG |
| YANG validation constraints | N-A | no YANG |
| YANG custom validators | N-A | no YANG |
| CLI commands/flags | Yes | `internal/le/rfc/actions.go`, the `patch` keyword |
| CLI grammar (keyword before value) | Yes | `patch <path>`, checked by `./le cli grammar` |
| Editor autocomplete | N-A | le completion derives from the action table |
| Functional test for new RPC/API | N-A | no RPC; the recordings are the end-to-end proof |
| Pipe completeness | Yes | the record report answers through the shared renderer |
| Env var registration | N-A | none |
| Doctor check for runtime dependencies | N-A | development tooling, not the daemon |
| Prometheus counters/metrics | N-A | none |
| BGP family surface (new SAFI / capability / attribute) | N-A | none |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | tooling only |
| 2 | Config syntax changed? | No | none |
| 3 | CLI command added/changed? | Yes | `le help rfc record` is derived from the action table |
| 4 | API/RPC added/changed? | No | none |
| 5 | Plugin added/changed? | No | none |
| 6 | Has a user guide page? | No | none |
| 7 | Wire format changed? | No | none |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | the published proven counts change; `docs/features/rfc-status.md` is derived |
| 10 | Test infrastructure changed? | Yes | `docs/contributing/rfc-conformance-gates.md` |
| 11 | Affects daemon comparison? | No | none |
| 12 | Internal architecture changed? | No | none |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | none |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | answered at implementation with `./le spec citation anchors` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | the record examples in `docs/contributing/rfc-conformance-gates.md` |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the route registered, the keyword accepted, the runner a stub
   - Tests: `TestPatchedKernelRouteRecordsOnlyAnObservedRed`
   - Files: `rfc.go`, `actions.go`, `discriminate_action.go`
   - Verify: the route is accepted by validation and refused at observation by the stub
2. **Phase: Patch shape** -- confinement to the function span and the release match
   - Tests: `TestKernelPatchConfinedToTheFunction`, `TestKernelPatchTargetsTheRuntimeRelease`
   - Files: `discriminate_kernel_patch.go`, `discriminate_validate.go`
   - Verify: tests fail, then pass
3. **Phase: Build and boot** -- the patched build on the host architecture and the two guest boots
   - Tests: `TestKernelPatchBuildsOnTheHostArchitecture`, the wiring test with a real runner on the Mac
   - Files: `discriminate_kernel_patch.go`
   - Verify: one first-population row recorded end to end
4. **Phase: Population** -- the seven RFC 792 rows recorded and their escapes removed
   - Tests: `./le rfc check`, `./le rfc discriminate stem rfc792`
   - Files: the patch files and records under `rfc/discrimination/rfc792/`
   - Verify: escaped count for RFC 792 falls by seven, proven count rises by seven

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | the broken run's failure names the tagged unit, not a guest boot failure |
| Attribution | the patch touches only the named function, and a paired patch carries solo runs |
| Rule: platform-linux | no cross-architecture kernel build or emulation |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| the route | `./le help rfc record` lists `patched-kernel` and `patch` |
| seven proof records | `./le rfc discriminate stem rfc792` shows no `kernel-producer` row |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | the patch path stays inside the checkout and the patch applies only to the kernel source tree |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Escape now, patched-kernel proof later | publish the rows as proven by the unit alone; block the rows until the route exists | Thomas's decision of 2026-10-29: the rows publish as debt, and each records that this route could prove it |
| One patch per record, confined to the named function | a patch per kernel file | a narrower break attributes the red to the function the escape named |

## Known Limitations
- The route proves that the tagged unit discriminates the kernel's behavior. It does not move the requirement into Ze's Go: the summary still declares the kernel as the implementer.

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
- [ ] AC-1..AC-N all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] **Commit A:** code + tests + docs + edited spec
- [ ] **Commit B:** `remove plan/post-release/kernel/spec-rfc-kernel-patch-discrimination-route.md` only, in the same `./le commit create` script

## Design refresh for owner review, 2026-11-16 — UNAPPROVED

### Disposition and decision required

**REWORK of the earlier design is proposed.** The metadata remains `design`,
P3, in the post-release bucket, with its original release reason. This refresh
does not authorize implementation, a priority change, a fault patch, a build,
an observation or a record. The main session presents the design and records
the owner's decision before a ready/status/bucket transition.

The concrete scheduling decision is whether to bring forward **only the
generic patched-kernel route and its receipt/packaging dependencies**, so the
selected IPsec work can meet AC-4 in
`plan/release/fixes/spec-ipsec-dual-form-receive-spd-bypass.md`. That AC requires
new-test discrimination before `RFC4301-5.2-2` loses `{partial}`. The IPsec spec
continues to own its exact directional SA permissions, full fragment-context
isolation, port-sensitive fragment enforcement, test changes, observations and
records. Approval of this design alone is not approval of previously refused
fault preparation; that separate decision remains pending.

The seven RFC792 rows in the original First population table and AC-7 remain
later population under this spec. They are not additional release work, are not
removed from the plan, and need no new spec. Generic acceptance below does not
close this whole spec or satisfy its later population criterion.

### Earlier statements that need rework if approved

| Earlier statement | Source-backed correction / proposed disposition |
|-------------------|-------------------------------------------------|
| Build or take a patched kernel during `record` | Choose two already-built matched pairs. The native builder emits receipts first; `record` validates them before any boot and performs no kernel or image build |
| Existing guest boots an arbitrary image (A-3) | A-3 is disproved as stated: `dockerLabCommandKernel` and `dockerLabImageBoot` bind exact kernel bytes and modules to an own-system image. Preserve the guard and provide correctly paired artifacts |
| Existing build takes one extra patch (A-1) | Not currently available: `Request` and `WorkerRequest` have one `PatchesDir`; `applyPatches` reads its complete `series`. Extend that producer instead of replacing the series or mutating normal inputs |
| Resolve the function at the vanilla runtime release | Resolve it after the complete production series. `0004-ip-fragment-xfrm-context.patch` introduces `inet_frag_xfrm_context_init`; `0005-xfrm-fragment-guard.patch` extends the composed source. A vanilla tarball cannot identify every producer |
| A red naming the unit proves the break | Insufficient for this route: `attribution` accepts a Go unit's FAIL line, which can also follow a capability, install, setup or cleanup failure in that unit. Require the intended assertion and all required control/prerequisite receipts |
| Seal after observation against the current tree | `sealDiscrimination` currently takes new hashes after the runs. The new route captures its inputs before compilation/observation and refuses drift before sealing |
| Same release/config or cache name establishes a matched pair | `writeProvenance` omits ordered patches, resolved config, actual compiler and artifact hashes. Eight-hex cache-key components and a backend name do not establish equality; legacy artifacts need new native receipts, not retrospective certification from their names |
| Additive builder support alone makes preparation reachable | `runKernel` and `resolveRuntimeKernel` carry no proof identity; normal warm-hit and publication would ignore the proof request. Add the registered preparation route and distinct proof cache identity defined below |
| vmlinuz digest fully keys a selected image | `builder.build` passes only the vmlinuz digest to `ImageKey` and trusts an existing image path. Bind the complete actual selected output manifest and typed artifact-only/native-build identity; proof additionally requires the native receipt. Validate the image receipt plus actual image SHA256 on every hit |
| Builds run on the Mac only | Apply the existing native-architecture rule, on a suitable host of that architecture, with shared job admission. No foreign-architecture kernel compilation or emulation |
| Single commit revert is the recovery path | Use a reviewed fix-forward change through the repository's commit flow. Never run a forbidden destructive git command; retain old records and valid escapes until replacement succeeds |

### Owner and architecture constraints checked

| Existing ruling / contract | Consequence for this proposal |
|----------------------------|-------------------------------|
| `ai/META-DECISIONS.md`, Goal and scope: kernel-producer is packet-only and escaped debt; OS behavior on Ze-installed state still owes proof | A C location does not make IPsec eligible for `kernel-producer`. Bind the reached Go setup/install producers; neither a halt nor a missing dependency falsifies the packet decision |
| `ai/META-DECISIONS.md`, release timing and design approval | Propose the minimum dependency promotion explicitly. Do not pull RFC792 population forward or mark the design ready without owner review |
| `ai/META-DECISIONS.md`, test changes and native owner-only actions | Any consumer test change follows the independent test-judge route; owner-only authorization remains interactive. This proposal records neither approval |
| `ai/doors/le-tooling.md`, one command registry and main-thread execution | Extend the existing action table and shared renderer. The main thread alone runs builds, tests and recordings |
| `ai/doors/appliance.md`, selected image, pins and host/target separation | Extend existing packaging with an explicit tree; keep checked-in module pins, cleanup and host/target build separation |
| `docs/contributing/rfc-conformance-gates.md`, record and offline freshness | The proof retains the same requirement/polarity/unit cover, requires an observed red, and remains verifiable without network, compilation or boot |

### Alternatives and recommended minimum

| Alternative | How it extends existing mechanisms | Benefit / cost | Recommendation |
|-------------|------------------------------------|----------------|----------------|
| Prebuilt matched normal/fault pairs | Native builder produces each kernel receipt; existing image packer receives its explicit tree and produces an image receipt; the recorder accepts both pairs | Invalid pairs fail cheaply before boots. Build ownership, caching and observation remain separate; caller must prepare the two receipted pairs | Recommended minimum |
| Recorder builds both pairs | `recordDiscrimination` orchestrates the same native builder and selected-tree packer before observing | Fewer caller steps, but adds build admission, cache/error/lifecycle orchestration to record and makes a record attempt expensive before it can be judged | Do not implement in this minimum |

Keeping the existing escape is valid only where its packet-only precondition
holds, and leaves debt. It cannot satisfy the selected IPsec AC-4. A Go
setup/install break remains appropriate for the claim it actually falsifies;
it cannot substitute for an observed C assertion failure.

### Proposed entry and data flow

The following grammar is a proposal, **NOT REGISTERED**. It extends `rfc record`
with `route patched-kernel`; existing `id`, `polarity`, `unit`, `kernel` and
`image` retain their meanings. This route additionally requires `fault-kernel`,
`fault-image`, `patch`, one `kernel-function`, and one `assertion`. Repeated
`state-producer`, `prerequisite` and `control` operands bind the reached Go
producers and the exact prerequisite/control test paths. Kernel and image
receipts are read at fixed adjacent names emitted by their producers, not
accepted as authored hashes. Route-inapplicable operands are refused.

Artifact preparation has its own proposed registered entry point, also
**NOT REGISTERED**: `./le rfc kernel-proof prepare arch <arch> builder
<docker|qemu> patch <tracked-path> kernel-function <linux-path::function>`.
This command builds or validates the two receipted kernel/image pairs and
returns their paths and full identities; it neither boots a guest nor writes a
discrimination record. The separate `record` command consumes that result.
Its concrete request/publication path is defined in the native preparation
refinement below.

1. The native builder resolves the source pin, complete ordered production
   series, fragments and required-symbol manifests. It builds normal and fault
   artifacts under the same immutable builder/toolchain inputs, adding only
   the confined proof patch to the fault build. It never edits production
   patches, the series, config or requirements to make a proof build.
2. During the owned worker build, resolve the named C function in the composed
   baseline source. Validate one additional unified diff against that exact
   source. Reject traversal, symlink escapes, absolute paths, renames, binary
   diffs, empty/no-op patches, ambiguous functions, preprocessor changes and any
   changed line outside that function. Apply with zero fuzz. This section
   defines validation only and contains no fault patch or fault example.
3. The image builder receives the selected tree once. Its key and nested
   `buildgokrazy.prepareArgs`/`instance.Prepare` use that same selection. Emit
   image SHA256 and an image build receipt binding the kernel/modules receipt,
   non-kernel image inputs and packed boot contents. The pair must have equal
   non-kernel image inputs; raw image hashes can differ.
4. `record` validates both full receipts, the allowed one-patch difference,
   current tracked inputs, image binding and native architecture before any
   observation. Before compiling it captures unit, claim, reached state-producer,
   assertion selections and a compiler-derived test-input manifest for the
   actual selected package/dependency/fixture population, as defined below.
   Compile one Linux test binary once under that same recorded build context, including
   coverage needed to prove the declared Go producers were reached. Hash that
   binary; both observations execute those exact bytes and the same selection.
5. Boot normal, run the selected unit, require its PASS and all exact positive
   control and prerequisite PASS receipts, with no skips. Observe installed
   state through the consumer test; coverage alone proves execution, not that
   installed values were correct. Boot fault with the same binary and inputs;
   require the intended assertion's FAIL, while every declared prerequisite and
   positive control still passes. Recheck artifact bytes on acquisition.
6. Use exact full Go test/subtest paths and structured test events for the
   assertion, prerequisites and controls. A parent-unit FAIL, a sibling's red,
   an unstarted assertion, a skip, timeout, panic, boot/capability/install
   failure or cleanup failure is not a proof. Ancestor FAIL events caused only
   by the selected assertion's failure are expected aggregates; they provide no
   attribution themselves. The declared assertion leaf may emit multiple
   failure diagnostics from its mapped full-matrix checks. Reject failure
   diagnostics outside that leaf or its declared mapped checks, every other
   failed path, and contradictory, missing or duplicate terminal receipts.
   Prerequisite/control/setup/read/lifecycle failures still refuse; arbitrary
   log text never substitutes for exact structured event receipts. Consumer
   tests expose one pure assertion leaf for the complete intended matrix and
   distinct pure prerequisite/control leaves. Namespace-sensitive actions stay
   on the fixture owner's locked thread; children assert immutable observations
   only. The consumer-admission refinement below applies before any test edit;
   the consuming spec owns the exact mapping and independent test judgment.
7. Re-read and compare every captured source input, test-input file/population,
   unit/claim/producer snapshot
   and acquired artifact digest before sealing. Refuse drift without writing.
   Extend `sealAndStore` with the validated snapshot; never mint a proof from
   the post-run tree alone. Merge only the matching cover, so another session's
   records survive; an existing escape is replaced only after verification.

### Proposed typed record and receipt contract

Names below are proposed lowercase kebab-case JSON fields. Extend the existing
array layout and strict decoder; do not reinterpret old fields or store opaque
arbitrary maps. A `patched-kernel` record requires exactly one `kernel-proof`
payload and refuses Go `producer`, `producer-sha`, `paired`, `solo`, `reason`,
`kernel-functions`, `ze-state-excluded` and `later-route`. Old routes reject
`kernel-proof` and all new route-only request operands. Existing old records
continue to load and verify without mass rewriting.

| Payload / receipt field | Type | Authorship and verification |
|-------------------------|------|----------------------------|
| `version` | closed schema version | Native derived value; unknown versions refuse |
| `normal`, `fault` | typed build and image receipts | Derived at build time, embedded compactly in the record; compare the complete input manifests, allowing only the named added patch and resulting output hashes to differ |
| Build input manifest | ordered path/digest entries plus typed dimensions | Bind source URL/SHA256/release, target/profile/architecture, production `series` and every ordered patch (currently 0001 through 0005), resolved fragment order/content, all `.require` inputs, full emitted config, firmware if used, builder recipe/worker inputs, immutable container/VM base and package inputs, actual compiler/binutils identity and build options/environment that affect output |
| Build outputs | SHA256 artifact manifest | Bind `vmlinuz`, emitted config, module release, module files/indexes and other packed kernel outputs; no path or release-only identity |
| Image selected input identity | closed `artifact-only` or `native-build` kind, full output-manifest SHA256, and native build-receipt SHA256 exactly for `native-build` | Every image binds the validated actual kernel/modules/config/DTB/overlay manifest. Ordinary legacy selection can be `artifact-only`; this attests only to selected bytes. Both proof-normal/fault require `native-build` with the full native receipt and matching output cross-links; artifact-only never satisfies proof |
| Image outputs | typed manifest and SHA256 | Bind the exact selected input identity, actual packed kernel/module set, pinned non-kernel build inputs and raw image digest; validate all packed members needed by the existing boot guard. The image receipt does not turn an artifact-only input into native build attestation |
| `kernel-function` | source path and unique function name | Authored selection resolved by native build against composed baseline; reject ambiguity |
| `baseline-function`, `fault-function`, `patch` | bounded source snapshots and SHA256, plus tracked patch path | Derived from actual composed source and applied diff. Retain the small baseline/fault function texts and confinement result so verification needs no source extraction/cache; current tracked source pin, production series and patch hashes must still match |
| `state-producers` | ordered Go key/behavior-hash entries | Authored keys, derived pre-observation fingerprints and reached-coverage receipts. These bind setup/install behavior; they are not breaks and must not receive OS producer-break credit |
| `test-inputs` | derived typed selection/build-context, package/fixture directory inventories and full-file SHA256 manifest | Derive from the actual selected binary's source population before compilation; bind helpers, globals/init, test helpers/dependencies, module/toolchain selection inputs and reached fixture/embed inputs. Hash every conservative candidate source file, including currently excluded candidates, as well as membership. Preserve rules and hashes for offline comparison without a compiler/source cache. These are observer dependencies, not state-producer breaks; artifact-only binary SHA does not replace them |
| `binary-sha` | full SHA256 | Derived once after compile; identical in both host acquisition and guest execution receipts |
| `assertion`, `prerequisites`, `controls` | one exact full assertion test path plus exact prerequisite/control paths with terminal receipts | Authored selections resolve within the selected unit/package/binary. The one assertion leaf covers its entire intended matrix; pure leaves assert owner-collected immutable values. Normal all PASS; fault only intended assertion FAIL, controls/prerequisites PASS. Embedded private-child logs and another package's admission test are not receipts |
| `observations` | two typed run receipts | Native commands, elapsed duration, exit/result, boot identity, image/kernel/module/binary digests and bounded decisive output; seal only after successful lifecycle/cleanup |

Large source trees, images and binaries are execution inputs, not permanent
offline dependencies. Keep only receipts, the confined tracked patch, bounded
function snapshots and decisive observations as retained proof. `rfc check`
compares current tracked recipe/source-pin/series/patch/config inputs,
test-input file hashes and package/fixture membership, and
unit/claim/state-producer behavior with these snapshots, validates receipt
cross-links and artifact digests recorded at observation, and never downloads,
builds, extracts Linux, boots, or requires old caches to remain. Cache removal
does not stale a proof; a tracked input change does. A legacy normal artifact
without the new build receipt cannot be attested retrospectively: rebuild it
through the native receipt producer.

### Changed-wire census and consumer disposition

Search scope: repository Go, shell, Python and workflow sources, including
hidden files, excluding git internals, vendor, module caches, temporary/build
outputs and plan prose. Controls were the known `DiscriminationRecord` writer
and `producer-sha` decoder. The search found no shell/Python discrimination
reader/writer in that scope. Followed carriers are the requirement JSON array,
record report, offline verdict, committed-baseline blobs, weekly endpoint JSON
projection and public ledger projection. Re-run this census at implementation
admission because concurrent source changes can add speakers.

| Producer / consumer | Source and observed carrier | Required change or preservation |
|---------------------|-----------------------------|---------------------------------|
| Action declaration / request | `actions.go`; `discriminate_action.go` `recordRequest`, `discriminateRecordAnswer` | Register typed keywords once; validate old/new route operands before builds/boots; help/completion/shared rendering derive from the action table |
| Record mint/merge/write | `discriminate_action.go` `recordDiscrimination`, `sealAndStore`, `mergeDiscrimination`, `renderDiscriminationRecords` | One writer, pre-observation binding, route-specific seal and atomic cover replacement |
| Record decode/shape/freshness | `discriminate.go` `readDiscriminationRecords`, `sealDiscrimination`, `verifyOneDiscrimination`; `discriminate_validate.go` | Strict typed nested shape and old stray-field refusals; offline manifest/snapshot validation |
| Kernel source/recipe/output | `kernelbuilder/recipe.go` `Request`, `writeProvenance`; `driver.go` `Build`; `worker.go` `RunWorker`, `applyPatches` | Emit receipts at producers, add one extra patch operand through native request transports; preserve existing provenance consumers and complete-series behavior |
| Native worker transports and fixture callers | `kernelbuilder/recipe.go` `containerCreateArgs`, `guestBuildCommand`; `tools/kernel-builder/main.go` worker argument parsing; `internal/test/fixture/install_fixture.go` direct `RunWorker` and container argv | Carry the optional extra patch and receipt inputs through both native transports; preserve normal no-extra-patch requests and fixture behavior |
| Consumer observation/receipt boundary | Engine `newEspPolLab`, `espPolLab.enter`, ANY-scope/IPv6-scope and transport-fragment matrix tests; `discriminate_guest.go` `guestBuildArgv`; dataplane complete-scope test and `userns.Enter` | Consumer owns exact old/new assertion/action/fatal-continuation mapping and immutable observation collection. Observer accepts same-binary pure leaf receipts only; engine binds its own fixture's capability/installed state. Dataplane admission remains a separate unit/record |
| Observer source population | `build/compile/inputs.go` `DaemonInputs`, `selectedInputs`; `go/toolchain/gotoolchain.go` `GoTest`, `GoList`; `go/lint/verifylint.go` `Runner.goList`; `go/lint/planinputs.go` `queryInputs`, `lintPopulation.reaches`; `job/inputsnapshot.go` | Extend existing structured source collection for the exact test build context, preserving daemon/lint callers. Snapshot full bytes of selected files and every conservative candidate source, including excluded candidates, plus package/fixture membership and selection metadata; new proof verifier/ratchet consumes their raw typed identities offline |
| Preparation registration and native publication | Existing `internal/le/rfc/extraction/register.go` pattern and `internal/le/register.go`; new `internal/le/rfc/kernel-proof/`; `internal/appliance/cmd_kernel.go` normal resolver and new native proof preparation seam; `cache.go` | Register `rfc kernel-proof prepare` once, link it in le, transport typed proof identity to a distinct cache publisher, validate receipts on hit, preserve ordinary normal caches |
| Image key/pack/payload | setup `docker/host/build.go` `builder.build`, `runGokWith`; `build/gokrazy/gokrazy.go` `prepareArgs`; `instance/prepare.go` `Prepare`; QEMU `dockerlab_command.go` | Same explicit selection at hashing and packing; image receipt; retain selected-byte/module/image/guest checks |
| Image key dimensions and warm-hit consumers | setup `docker/host/pins.go` `KeyInputs`, `ImageKey`; `actions.go` `lockedBuild`, `EnsureImage`; `build.go` `builder.build`; `colima.go` image key record | Key the full actual selected output manifest plus explicit artifact-only/native-build identity and native receipt digest when available; validate image receipt/raw SHA on hit. Ordinary warm legacy kernels remain usable without a native receipt; proof consumers reject artifact-only. Preserve existing callers' locking/lifecycle |
| Selected proof cache reader and cleanup census | `runtimekernel_existing.go` `ExistingRuntimeKernelTree`; `cache.go` `KernelCacheNamespaces`, `evictKeepN`, `cacheEntryArch`; `internal/le/scratch` namespace consumers | Add one closed proof child grammar with receipt checks beside the existing grammar; keep both under protected runtime cache namespace and out of automatic pruning; preserve explicit eviction behavior and ownership requirements |
| Committed-baseline freshness | `check_baseline.go` `baselineDiscrimination`, `baselineRecordBlobs` | Include every tracked kernel-proof manifest/patch/state-producer dependency when reading committed blobs; preserve working-tree versus HEAD drift treatment |
| Stale verdict classification and report | `check_ratchets.go` `checkDiscriminationRatchet`, `driftedKey`, `driftIsCommitted`, `discriminationDrifted`, `discriminationStaleError`; `check.go` discrimination input/report assembly | Consume typed dependency identities at their recorded digest granularity; classify exact HEAD/current inputs without a fallback to the tagged Go unit; name the changed C function and tracked patch/recipe/config dependency. See the ratchet refinement below |
| Audit / partial coverage | `check_audit.go` `partialProofClaims`, `partialCoverRefusals` | Verified `Proves` already admits a new valid route; update mutant/revert-only diagnostic/docs and bind new proof payload in audit freshness |
| OS state-credit and counts | `profilecoverage.go` `DiscriminationVerdict.Proven`, `osProofsOf`, `osVerified`; `implemented.go`; `render_ledger.go` | Count a verified C claim as proof; do not pretend snapshot-only Go state producers were broken. Preserve lower-layer annotation's explicit producer-break requirement |
| Historical/derived record writers | `rename.go`; `evidence_split.go`; `fixture.go`; `selftest_core.go` | Preserve old migrations/fixtures, carry new typed payload without dropping it, and test fresh verification after a supported rename; never manufacture observed run receipts |
| Weekly projection | `weekly_source.go` `weeklyProving`, `weeklyRecordOf` | Preserve existing four-field projection and verified/non-escape predicate; prove new-route gained/lost covers are counted once |
| Public ledger detail | `internal/le/site/rfcledger.go` `rfcProofsByCover`; `rfcevidence.go` | Present C function, patch and matched-observation provenance for this route instead of an empty Go producer; old routes render unchanged |
| Health / discovery / prose | `internal/le/test/health/collect_rfc.go`; `sections.go`; `register.go`; `ai/INDEX.md`; record README and conformance guide | Update route vocabulary, proof-versus-debt wording and discovery; no independent proof counter or inventory |
| Text-only matches | `internal/le/feature/runrecord.go`; `internal/le/doc/check/retired.go`; `internal/component/l2tp/plugins/authradius/coa_transaction.go` | References/comments, not discrimination format speakers; preserve their behavior |

### Proposed generic acceptance and tests

These G-AC rows supplement the original ACs for owner review. They are not
implemented or demonstrated. AC-7 remains the later population obligation.

| ID | Input / condition | Observable outcome | Permitted survivors and reason |
|----|-------------------|--------------------|--------------------------------|
| G-AC-1 | Two natively built pairs with identical baseline manifests and exactly one confined C patch | Same compiled binary passes on normal; intended assertion alone fails on fault; valid typed record verifies and report counts one proven cover | Required valid traffic/control subtests still pass on both kernels; otherwise a broken setup could masquerade as discrimination |
| G-AC-2 | Source, series/order, any production patch, config/require/fragment, actual toolchain/builder input, architecture or non-kernel image-input mismatch | Refuse before boot, name mismatched input, write nothing | Existing valid prebuilt pairs and old proof routes remain usable |
| G-AC-3 | Out-of-function/ambiguous/no-op/directive/path-escape patch, or function missing from composed baseline | Native patch validation refuses before compile; record cannot accept a fabricated receipt | A unique function introduced by production patches is eligible after composition |
| G-AC-4 | Wrong/altered kernel, module/index, image or binary bytes; key/pack selects different trees | Refuse at pair validation or payload acquisition; no bypass and no record | Correct explicit pairs boot through existing identity checks |
| G-AC-5 | Fault produces capability/setup/install/boot/skip/timeout/panic/sibling/cleanup red, missing or contradictory receipts, or only parent FAIL | Refuse; no proof count and no replacement of an existing record | Exact intended assertion FAIL with all required controls/prerequisites PASS remains eligible |
| G-AC-6 | Unit/claim/state-producer/source recipe/production patch/proof patch or observer test-input file/population changes between snapshot, compile, observations or seal | Refuse before write; unchanged observations never seal against later source or a different observer population | Edits outside every bound file/population and declared dependency leave the proof current. Whole-file observer manifests are conservative: a comment-only or other edit inside a bound file may stale the new proof even when the named unit text is unchanged |
| G-AC-7 | Delete execution caches after valid recording; then change one bound file, package/fixture membership or other tracked proof dependency | First proof verification succeeds offline using retained selection/context/inventory; second identifies the exact stale dependency or membership change with existing HEAD/working-tree distinctions | No compiler, go list, network/build/boot or large cache retention is required by the new proof verifier. Unrelated files outside its complete recorded population remain permitted; bound-file comments may conservatively stale it |
| G-AC-8 | Old mutant/revert/escape arrays plus malformed mixed old/new records | Old valid records still verify; old stray-field, unknown-key, duplicate-cover and invalid new-schema cases refuse | Valid kernel-producer escapes remain debt until their own proof succeeds |
| G-AC-9 | Reporting, partial audit, baseline and rename/migration paths consume a new proof | Cover counted once, C provenance retained, stale evidence excluded, state-producer snapshots never receive Go break credit | Existing OS-state proof credit continues to require its actual producer break |
| G-AC-10 | The same tracked recipe/config/production-patch/proof-patch change appears first only in the working tree over unchanged HEAD, then at HEAD with the original proof still present | First case reports exact uncommitted dependency drift without a stale-record violation; second refuses committed drift with the exact dependency and C proof context. Both exclude the stale cover from every proven figure | Unrelated working-tree edits do not change classification of the affected dependency; unchanged old records retain their current ratchet behavior |
| G-AC-11 | Registered prepare action, a warm ordinary normal cache, and a new proof patch; repeat with a valid warm proof pair, then an altered/missing receipt or artifact | First prepares distinct proof-normal/fault entries and matching images; second reuses only fully validated proof artifacts without builds; invalid hit refuses before packing/boot. Ordinary normal cache and shared staging bytes stay unchanged throughout | Valid ordinary normal entries and explicit legacy pairs remain available on their existing routes; they are not retrospectively promoted to proof receipts |
| G-AC-12 | Selected trees with identical vmlinuz but different valid actual module/output manifests, in artifact-only and native-build modes; then alter raw image bytes without changing its path | Module difference gives distinct image identity and packs each selected set in both modes; corrupted image hit refuses on actual SHA/image-receipt check rather than returning the old path | An unchanged complete actual output manifest and typed identity reuses its validated image. Artifact-only remains valid for ordinary images; both proof pairs still require native receipts and all guest guards |
| G-AC-13 | Ordinary resolver returns a valid warm legacy tree with no full native receipt; the same artifact-only image is offered to patched-kernel proof | Ordinary image preparation validates the complete actual output manifest, emits an artifact-only image receipt and reuses the validated image on repeat without rebuilding/migrating the kernel. Proof preparation/record refuses artifact-only before observation | Existing ordinary normal cache/staging stay unchanged; no native receipt migration, overwrite or purge is required. Full-receipted proof-normal/fault entries remain eligible on their distinct path |
| G-AC-14 | A selected consumer needs structured assertion/control/prerequisite receipts | Admit only after a source-backed old/new action and assertion mapping plus independent test judgment proves every matrix row/count/token/order/adjacent control survives, namespace I/O remains on the fixture owner, and original fatal versus continuing failure behavior is preserved. All matrix rows run in a qualifying proof; one pure assertion leaf covers the complete intended matrix | Existing fixture/state/control sequence remains shared; expected assertion violations do not stop later controls. Missing/read/setup/cleanup errors remain nonqualifying prerequisite/lifecycle failures; no implicit approval to change fail-fast behavior |
| G-AC-15 | An engine proof names the dataplane complete-scope test, an embedded private-userns child log, or any second binary as its prerequisite | Refuse consumer admission/observation; engine proof must bind actual capability and installed-state observations from its own existing fixture in the same selected binary | Dataplane admission test keeps its separate unit and own record. A source-feasible same-binary engine prerequisite remains eligible after independent judgment |
| G-AC-16 | Tagged unit text stays unchanged while a shared observer/helper, global/init input, conservative candidate source content, selected-package/dependency membership or reached fixture/embed input changes; specifically, only an existing excluded candidate's build constraint changes so it becomes selectable, with unchanged names and formerly selected bytes | New-route proof stales or pre-seal recording refuses for that exact raw input/population; both uncommitted and committed constraint-only drift receive zero proof credit and their normal report/refusal distinction. Offline comparison needs no compiler to discover the selection change because the excluded candidate's full raw bytes were already bound | Existing mutant/revert/escape records keep their byte layout and fingerprint semantics. Outside-population files do not stale this route; all bound candidate files, selected or excluded, may stale conservatively |

| Proposed test | Owning package / route | Decisive behavior |
|---------------|-----------------------|-------------------|
| `TestPatchedKernelRecordActionReachesObservedProof` | `internal/le/rfc` action dispatch with fake native artifacts/runner | Entry-point wiring, G-AC-1, same binary hash on both observations |
| `TestKernelProofPairRefusesEachInputMismatchBeforeBoot` | `internal/le/rfc` | One mutation per manifest dimension in G-AC-2; boot call count remains zero |
| `TestProofPatchResolvesOnlyInsideComposedFunction` | `internal/appliance/kernelbuilder` | Production-added function resolves; each G-AC-3 refusal independently exercised |
| `TestNativeProofReceiptBindsActualBuildInputsAndOutputs` | `internal/appliance/kernelbuilder` | Ordered production series plus extra patch, resolved config, actual toolchain and output hashes bound at build producer; incomplete legacy provenance refused |
| `TestSelectedKernelIsUsedForImageKeyAndPacking` | `internal/le/setup/docker/host`, `internal/le/build/gokrazy` | Change current runtime selection between steps; explicit pair remains the same at key/pack and receipt |
| `TestKernelProofRejectsAlteredPayloads` | `internal/le/rfc`, `internal/le/test/qemu` | G-AC-4 for each artifact; existing runtime/image guards remain active |
| `TestKernelProofRequiresExactAssertionAndControlReceipts` | `internal/le/rfc` | G-AC-5 table: each false-red category refuses; valid intended red succeeds |
| `TestKernelProofRequiresCompleteSingleAssertionMatrix` | `internal/le/rfc` observer tests plus the consuming spec's judged source mapping | One pure assertion leaf covers all mapped rows; omitted row/count/token/order/control or a receipt from only a filtered case refuses admission; intended assertion failure still permits the following controls |
| `TestKernelProofRefusesCrossPackageOrEmbeddedChildPrerequisite` | `internal/le/rfc` observer/package-selection tests | Engine unit cannot borrow dataplane admission, another binary, or verbose private-child text as a structured prerequisite; same-binary fixture observations remain eligible |
| Consumer ownership/order proof | Consuming IPsec spec, independent test judge before edits/recording | Exact old/new mapping proves all namespace create/enter/socket/netlink/proc/send/read/cleanup actions stay with the original fixture owner, action order and fatal/continue boundaries survive, and pure sequential children do no I/O or per-case fixture reset. If not demonstrated, consumer admission is REWORK |
| `TestKernelProofRefusesDriftAtEachObservationBoundary` | `internal/le/rfc` | Each captured input drifts before normal, fault or seal; no record write |
| `TestKernelProofBindsHelperAndInitializerWithUnchangedUnit` | `internal/le/rfc` plus compiler-input collector fixtures | Helper-only and global/initializer-only changes independently stale/refuse a new proof while unit text and its old behavior hash stay unchanged; old-route behavior remains unchanged |
| `TestKernelProofBindsSourceDependencyAndFixtureMembership` | `internal/le/rfc`, `internal/le/build/compile` | Added/removed package source, new import/dependency selection and fixture/embed additions/removals/content changes invalidate the inventory/context. Paired uncommitted/committed cases change only an existing excluded candidate's build constraint so it becomes selectable: names, tagged unit and formerly selected bytes stay unchanged, yet raw candidate hash stales proof and yields report/refusal respectively with zero proof credit and no compiler/list/cache access. An incomplete or unbounded population refuses admission |
| `TestKernelProofObserverManifestStaysOfflineAndScoped` | `internal/le/rfc` | Without build/source caches or compiler/list/network access, retained selection/context inventories verify unchanged inputs; bound-file comment edits stale conservatively, outside-population file edits stay current |
| `TestKernelProofObserverDriftBeforeSealAndAgainstHead` | `internal/le/rfc` | Helper/init/membership/fixture drift before compile or seal writes nothing; paired uncommitted/committed raw observer-input cases report/refuse respectively and both exclude stale proof |
| `TestKernelProofVerifiesOfflineWithoutBuildCaches` | `internal/le/rfc` | Remove only test-owned caches; verify then independently stale each tracked input |
| `TestKernelProofRawDependencyDriftIsJudgedAgainstHead` | `internal/le/rfc/check_test.go` | Paired G-AC-10 cases for every raw dependency kind: unchanged HEAD plus working edit reports; identical edit committed refuses; both have zero proof credit and exact dependency diagnostics |
| `TestKernelProofCommittedDriftCannotBeHiddenByAnotherWorkingEdit` | `internal/le/rfc/check_test.go` | Committed raw dependency drift beside a different uncommitted dependency, and committed Go producer drift beside another function's edit, still refuse; evaluate all mismatches |
| `TestKernelProofDependencyIdentityUsesItsRecordedDigestKind` | `internal/le/rfc` | Raw bytes, Go behavior and claim text use their own digest rules at HEAD/current; absence and unreadable HEAD stay distinct; no raw dependency falls back to Unit |
| `TestKernelProofStaleDiagnosticsNameCFunctionAndDependency` | `internal/le/rfc` | Refusal and uncommitted report identify C function, proof patch and exact changed input; no empty Go producer/break text |
| `TestKernelProofPrepareIsReachableThroughRegisteredAction` | New `internal/le/rfc/kernel-proof` action/registration tests | Full proposed command dispatches to typed native preparation and selected-image packing; help/completion expose prepare and its keyword operands |
| `TestKernelProofPreparationPreservesWarmNormalCache` | `internal/appliance` proof-preparation tests | With a warm ordinary cache, proof patch reaches native build with distinct role/key; normal cache and shared stage hashes stay unchanged on success, refusal, cancellation and publication race |
| `TestKernelProofCacheHitRequiresReceiptAndActualArtifacts` | `internal/appliance` | Valid hit avoids build; altered module/config/kernel/receipt or missing receipt refuses. Proof-normal and fault identities cannot collide with ordinary normal lookup or each other |
| `TestSelectedRuntimeReaderAcceptsOnlyReceiptedProofGrammar` | `internal/appliance`, `internal/le/test/qemu` | Valid explicit proof pair reaches existing guards; malformed role/hash/path/receipt refuses; legacy normal grammar keeps its existing behavior |
| `TestImageKeyDistinguishesSameKernelDifferentModules` | `internal/le/setup/docker/host` | Same vmlinuz with different actual selected module/config/output manifests gives distinct keys in artifact-only and native-build modes; each pack uses its selected set and native identity cannot collapse into artifact-only |
| `TestImageCacheHitVerifiesReceiptAndRawBytes` | `internal/le/setup/docker/host` | Matching typed identity/full output manifest/image receipt/raw image hit avoids pack; modified raw bytes, missing/malformed image receipt or mismatched selected set refuses in both modes |
| `TestOrdinaryWarmLegacyKernelBuildsAndReusesArtifactOnlyImage` | `internal/le/setup/docker/host` | Warm valid legacy kernel without native receipt builds image from actual full output manifest once, then reuses it; no kernel rebuild/migration or normal cache/staging mutation |
| `TestKernelProofRefusesArtifactOnlyImageIdentity` | `internal/le/rfc`, `internal/appliance` proof preparation boundary | Artifact-only input/image is refused for either proof role before boot, even with correct actual kernel/module/image hashes; full native receipt and exact selected-output cross-links remain mandatory |
| `TestKernelProofPreservesOldRecordsAndStrayFieldRefusals` | `internal/le/rfc` | G-AC-8 on real old fixture shapes and mixed malformed shapes |
| `TestKernelProofSurvivesEveryRecordConsumer` | `internal/le/rfc`, `internal/le/site` | Baseline, partial audit, rename/split/fixture, weekly and ledger projections; no snapshot-as-break credit |
| One real authorized C discrimination | Main-thread own-system native proof through `rfc record` | G-AC-1 end to end, all lifecycle receipts, then offline check; selected IPsec consumer supplies its separately approved assertion/controls |

Use the cheapest tests first. Fake build/boot boundaries prove validation;
they do not satisfy the final native observation. The generic route has no
protocol peer; the IPsec spec retains its existing interop obligations.

### Proposed files, phases and discoverability

| Phase | Files / surface | Outcome and documentation in that phase |
|-------|-----------------|-----------------------------------------|
| 0. Preparation wiring, completed with phases 1 and 2 | New `internal/le/rfc/kernel-proof/register.go`, action/prepare sources and tests, linked by `internal/le/register.go`; new `internal/appliance/runtimekernel_proof.go` and tests; `cache.go`, `runtimekernel_existing.go` | Wire the separate prepare action and typed native seam first; refuse until receipt/build/image dependencies are available. Complete distinct proof publication and validated cache hits with phases 1 and 2; preserve ordinary cache and shared staging. Update `docs/architecture/appliance/build-artifacts.md`, `docs/contributing/rfc-conformance-gates.md` and `ai/INDEX.md` |
| 1. Native receipt and additive patch | `internal/appliance/kernelbuilder/recipe.go`, `driver.go`, `worker.go`; `tools/kernel-builder/main.go`; native Docker/QEMU request transports/tests and existing `internal/test/fixture/install_fixture.go` callers; receipt type in kernelbuilder | Complete derived build receipts and confined extra patch; extend `docs/architecture/appliance/kernel-profiles.md` at the same time; preserve existing instance provenance and normal fixture requests |
| 2. Explicit selected-tree image | `internal/le/setup/docker/host/actions.go`, `build.go`, `pins.go` and warm-hit/key tests; `internal/le/build/gokrazy/gokrazy.go`; existing `internal/appliance/instance` seam/tests | Carry one tree end to end; key full actual selected output manifest and typed identity, adding native receipt identity when available. Ordinary legacy inputs use artifact-only; proof requires native-build. Verify cached image receipt/raw bytes before hit return; update `docs/architecture/appliance/docker-host.md`, `build-artifacts.md` and `kernel-profiles.md` |
| 3. Record wiring and offline contract | `internal/le/rfc/actions.go`, `rfc.go`, `discriminate.go`, `discriminate_action.go`, `discriminate_validate.go`; new `discriminate_kernel_patch.go` and focused tests; existing `internal/le/build/compile/inputs.go` collector and shared toolchain/population seams where needed | Typed route, strict old-route compatibility, one binary, pre-compilation compiler-derived observer source/membership/context snapshot, pair and exact assertion/control selection. Preserve existing daemon/lint collectors; update `rfc/discrimination/README.md`, `docs/contributing/rfc-conformance-gates.md`, `ai/INDEX.md` and collector architecture anchors in `docs/contributing/preview-artifacts.md` only where the shared collector contract changes |
| 4. Observe and preserve guards | `internal/le/rfc/discriminate_guest.go`, `discriminate_observe.go`; `internal/le/test/qemu/dockerlab_command.go` and tests only where needed | Exact event attribution, one-package/binary prerequisite binding and two guarded boots. Enforce complete single-assertion matrix admission; consumer owns independently judged fixture-safe instrumentation. Update `docs/architecture/testing/qemu-integration.md` and `docs/contributing/rfc-conformance-gates.md` with pure-leaf/owner-thread and private-child-log restrictions; consumer updates `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` for its retained fixture/control contract |
| 5. Consumers and generic proof | Census-listed `check.go`, `check_baseline.go`, `check_ratchets.go`, audit/coverage/rename/split/fixture/weekly/site consumers and scoped tests | Every reader retains or presents typed provenance correctly; typed committed/uncommitted drift classification and diagnostics; one authorized native proof and offline freshness. Consumer instrumentation/records belong to the IPsec spec |
| Later population | Original RFC792 patch/record files and AC-7 | Seven original rows remain post-release, with the same native validation and review requirements |

The existing CLI/checklist rows above remain historical design. For this
proposal, CLI grammar/help/completion/shared rendering apply through the action
table; the changed JSON artifact and endpoint/site projections apply through
the census. No YANG, daemon RPC/config, environment variable, product plugin,
Prometheus counter, BGP-family or product feature declaration is added. Native
build dependencies remain development-tool checks at preflight; no daemon
doctor feature is introduced. Any new refusal uses the existing registered
diagnostic mechanism and names the mismatched field and exact repair route.
Source-anchor updates apply to the named architecture/conformance/test pages
in their phase; the remaining SDK/API/config/comparison/route-metadata
documentation categories do not apply because those contracts do not change.

Discovery extends the existing RFC-discrimination row in `ai/INDEX.md`, the
single action declaration and the record README. Regression prevention is the
strict route-specific decoder plus G-AC-2 through G-AC-9; no second inventory or
parallel gate is proposed.

### Refresh risks and assumptions

| ID | Assumption | Basis | If wrong | Validation | State |
|----|------------|-------|----------|------------|-------|
| A-R1 | One confined function patch can expose the selected consumer's intended assertion while controls survive | This is the route's attribution requirement, not evidence that an IPsec fault exists | Generic route may work while a specific claim needs a different justified break | Separate authorized native observation with exact assertion/control receipts | Unvalidated |
| A-R2 | Each selected consumer can expose one pure full-matrix assertion leaf and distinct pure prerequisite/control leaves without changing fixture ownership, sequence or failure semantics | Current selected packet-matrix checks share a parent; `newEspPolLab` locks the caller thread, `enter` directly calls netns.Set, and Go testing Run starts another goroutine. `guestBuildArgv` compiles one package; dataplane admission uses private userns re-execution | Instrumentation may be infeasible for that unit; refuse consumer admission and rework rather than narrow the matrix, split/reset fixtures, borrow another binary or change fail-fast behavior | Source-backed exact old/new action/assertion/fatal-continuation mapping and independent test judgment before test changes, then complete qualifying observation | Unvalidated |
| A-R3 | Receipt inputs can hold an identical immutable toolchain/builder environment for both native builds | Current builders exist but package/image names alone float | Pair refused; no matched proof can be minted | Native worker records actual identities; mismatch test and real pair | Unvalidated |
| A-R4 | Compiler-selected dependency inputs plus conservative package-source/fixture inventories can bound every observer input the selected consumer reaches | Existing compiler collector inventories source/embed closure; lint population supplies membership rules, but arbitrary runtime fixture reads are not all compiler-discoverable | A consumer with an unestablished dynamic fixture/dependency set cannot be admitted | Actual test-mode compiler collection under exact build context, source-backed fixture/input census and independent admission review; helper/init/membership/fixture/offline tests | Unvalidated |

| ID | Risk | Early signal | Mitigation |
|----|------|--------------|------------|
| R-R1 | Post-run source is sealed as if it produced the observed binary | Snapshot and final digest differ | Refuse before write; bind one compiled binary and recheck every boundary |
| R-R2 | A broad fault defeats controls and appears as a claim failure | Prerequisite/control red or absent; parent FAIL only | Exact subtest attribution and complete receipts; no record |
| R-R3 | Image key hashes normal while nested pack resolves fault/current, or vice versa | Selected tree or packed artifact digest differs | One explicit tree passed through key and pack; acquire/hash payload at execution |
| R-R4 | Cache filename or old provenance is mistaken for full attestation | No actual compiler/config/patch/output receipt | Refuse legacy artifacts for this route; rebuild natively |
| R-R5 | Offline verifier quietly needs removed multi-gigabyte inputs | Cache-free test attempts source extraction or artifact access | Retain compact build/observation receipts and function snapshots; compare tracked manifests offline |
| R-R6 | New payload bypasses existing strict schema or loses data at a secondary reader | Mixed-route fixture accepts stray field, or round trip drops payload | Explicit census dispositions and old/new malformed-shape tests |
| R-R7 | Snapshot-only Go setup producers acquire undeserved broken-producer credit | `osProofsOf` receives state-producer keys from C payload | Keep state binding separate from actual broken-producer identity; test credit path |
| R-R8 | Build or proof cleanup removes another session's data | Shared path selected for cleanup | Owned worker/run directories, exact process ownership, joined cleanup; retain only smallest needed evidence in the owning spec |
| R-R9 | Raw input drift is judged against the unchanged tagged Go unit, or only the first mismatch is classified | Committed patch/config drift appears only in the uncommitted list, or another working edit hides it | Typed dependency descriptors, exact recorded/HEAD/current comparison for all mismatches, paired ratchet tests and explicit C diagnostics |
| R-R10 | A warm ordinary cache swallows the extra patch or a proof build overwrites its entry | Native builder not called for new fault input, or normal/staging hashes move | Separate registered prepare API, proof-normal/fault key domain, immutable publication and warm-normal preservation tests |
| R-R11 | Same vmlinuz masks changed modules, or an image path hides corrupted raw bytes | Same image key for different actual module/output manifests, or warm return after file alteration | Full actual output-manifest/typed-identity image key in both modes; check selected artifacts, image receipt and full raw SHA on every cache hit |
| R-R12 | Ordinary legacy kernel is forced into an endless missing-native-receipt repair loop, or artifact-only image is promoted to proof | Repeated normal warm hit never yields usable image, or proof accepts artifact-only identity | Keep ordinary artifact-only preparation valid without native receipt migration; both proof roles require native receipts and exact output cross-links; paired ordinary-success/proof-refusal tests |
| R-R13 | New child subtest moves namespace I/O off the locked fixture thread or resets state between cases | Child calls enter/socket/netlink/proc/send/read/create/cleanup, or a new per-row fixture changes counters/sequence | Parent fixture owner collects immutable observations in original order; only sequential nonparallel pure assertions run in children; reject without the independent mapping/judgment |
| R-R14 | One assertion path represents one filtered case, or a foreign admission log substitutes for the unit's own prerequisite | Matrix/control coverage shrinks, later controls disappear after a violation, or prerequisite comes from another package/private child text | One complete intended-matrix leaf, exact row/count/token/order/control census, preserved fatal/continue behavior, and same-binary actual fixture observations; A-R2 remains unvalidated |
| R-R15 | Function-local unit hash, observed binary SHA or candidate names-only inventory leaves helper/global/fixture/selection drift invisible offline | Unit text stays unchanged but the instrument/helper/init changes, or an existing excluded source becomes selectable through only a build-constraint edit | Bind whole raw bytes of every conservative candidate source, selected or excluded, plus compiler-derived observer inputs, package/fixture membership and build context; snapshot before compile, recheck before seal, verify raw dependencies offline. No old-route semantic change |

### Observer source and population refinement — UNAPPROVED

Schema review confirmed that existing function fingerprints are not recursive.
`scopeIndex.keyTexts`/`funcTexts` select named declaration text;
`behaviorBytes` normalizes only that supplied text. A named test's hash does
not also cover shared instruments such as `espPolStat` or
`sadSelReadDropCounters`, globals/init outside that span, or fixture inputs.
The observed binary SHA binds the two runs but cannot detect later source
drift without rebuilding, which offline verification does not do. Preserve all
old Go-route semantics; add the following binding only to `patched-kernel`.

Derive a typed `test-inputs` population from the **actual selected test binary's
build selection before compilation**, not an authored helper list. Extend the
existing `internal/le/build/compile/inputs.go` compiler-input collector with a
selected-test-package mode; its current `DaemonInputs` is hardcoded to daemon
targets/two architectures and must not be called unchanged for this purpose.
Reuse `selectedInputs`' structured source/dependency/native/embed enumeration
and incomplete/outside-checkout dependency refusals. Use the same activated
toolchain environment, feature/integration/race tags, GOOS/GOARCH, module/vendor
resolution and package selection as `guestBuildArgv`; the path-only current
`Toolchain.GoList` answer alone is not a source inventory. Test-mode selection
must include the tested package's applicable internal/external test files and
their dependency closure, distinguishing the synthesized test main from authored
inputs as the existing lint `Runner.goList` does.

Reuse the package-directory/embed-tree population concepts from lint
`queryInputs`/`lintPopulation` and the job input-snapshot boundary mechanism,
without importing a lint run or creating a second independent source scanner.
The retained inventory records package import identities/directories, selected
source files, dependency edges, module/workspace/vendor metadata including
expected absence, toolchain/stdpatch selection pins, and the exact build
context. Conservatively hash whole repository/vendored source files for that
population, including helpers and global/init declarations. For each
selected/dependency package directory, bind **full raw bytes of every source
candidate in the conservative population**, including candidates currently
excluded by tags/OS/architecture, as well as candidate membership rules and
names. Extend `lintPopulation.reaches`' scope concept, which reaches candidate
source changes even when the compiler did not previously select them;
`selectedInputs` alone enumerates only selected files and is insufficient.
A names-only candidate inventory is expressly insufficient: changing an existing
excluded file's build constraint can make it selectable without adding a name
or changing formerly selected bytes. Its already-bound raw hash must stale the
proof offline. New imports change a bound file, new source imports change
directory membership, and module resolution changes alter bound metadata, so
each invalidates the old closure without a fresh compiler query.

Compiler-selected embeds retain their pattern/root membership and content
inventory. The consumer's source-backed input census additionally binds reached
runtime fixture/data inputs and their directory membership; arbitrary dynamic
reads are not made safe by a compiler listing. If their input set or dependency
selection cannot be established, refuse consumer admission rather than omit it.
Standard-library source identity uses the recorded exact toolchain pin and
tracked stdpatch inputs, not a disposable GOROOT/cache path. Nonstandard source
dependencies that cannot be retained and verified from the checkout/vendor
population are refused by this minimum.

Capture contents and membership before source selection/compilation, ensure
the collector and compiler saw the same recorded context/population, and recheck
before each observation and sealing. Extend the existing typed raw-dependency
list and HEAD blob/membership collection with these files and directory
inventories. Offline verification re-enumerates the retained conservative
directory/pattern rules and compares exact membership/content under the stored
context; it does not re-resolve dependencies, run go list, rebuild, use network,
or require compiler/module/source caches. Added/removed source/dependency or
fixture inputs are stale membership identities at HEAD/current just as changed
raw files are stale dependencies; the same committed/uncommitted ratchet applies.

This deliberately has coarser freshness than a function hash: comments and
other edits inside a bound observer file may stale a new proof even when the
unit's text is unchanged. Files outside all bound populations/dependencies do
not. Observer dependencies are separate from `state-producers`; neither grants
Go producer-break credit. Old mutant/revert/escape layouts, fingerprints and
verification semantics remain unchanged. A-R4 and consumer-specific A-R2 remain
unvalidated; this refinement defines the required design and tests, not an
implemented collector or completed consumer admission.

### Consumer receipt feasibility refinement — UNAPPROVED

Read-only control-feasibility review found an unresolved consumer constraint,
not implemented receipts. Current ANY-scope, IPv6-scope and authenticated
transport-fragment matrix checks perform assertions and adjacent controls in
their parent test. `newEspPolLab` locks that caller's OS thread;
`espPolLab.enter` directly changes its network namespace with netns.Set.
Go testing `T.Run` starts another goroutine, so a child does not inherit that
locked-thread namespace. Existing family wrappers elsewhere do not provide the
new distinct assertion/prerequisite/control receipt contract.

The proposed minimum keeps every namespace-sensitive create, enter, socket,
netlink, proc, send, read and cleanup operation on the existing parent fixture
owner, in its original action order. That owner collects immutable observations;
sequential nonparallel child leaves only assert those values, with no I/O and
no new per-child fixture. One assertion operand remains sufficient only when
that **one pure leaf covers the complete intended matrix**, not one selected
case. Required positive-control and prerequisite observations have distinct pure
leaves, without resetting shared state, packet sequences or counters.

This is conditional consumer admission, not blanket permission to refactor the
tests. The consuming spec must map every old assertion and action to its new
location and order, including exact matrix rows, counts, tokens and adjacent
controls, and map each Fatal/Fatalf versus Errorf continuation boundary.
Expected matrix violations that currently use Errorf must still allow following
controls; missing/read/setup errors stay fatal prerequisite failures and never
earn intended-C-assertion credit. If complete observation collection would
change fail-fast behavior, that change needs its own source-backed mapping and
independent test judgment first. Cleanup failure remains nonqualifying. If this
preservation cannot be demonstrated for a unit, refuse admission and rework its
instrumentation; do not weaken receipts, drop a row or split the shared fixture.

`guestBuildArgv` builds the selected unit's package into one binary.
`dataplane.TestRFC4301KernelAdvertisesCompleteStateScope` therefore cannot run
alongside an engine unit inside that binary. It also uses `userns.Enter`, whose
private re-execution returns verbose output through a parent log; that embedded
text is not a structured subtest receipt. Each engine proof must bind actual
capability and installed-state observations from its own existing engine
fixture. The dataplane capability test remains a separate admission unit with
its own record; it is not a surrogate engine prerequisite. No second-binary
route is added to this minimum. Observer validation, the census, G-AC-14/15 and
the named documentation dispositions carry these restrictions. A-R2 remains
**Unvalidated** until the exact consumer mapping and independent judgment exist.

### Native preparation and image-cache refinement — UNAPPROVED

Native review found two missing producers in the earlier design: there was no
reachable preparation path that preserved normal cache identity, and an image
key based on vmlinuz alone could reuse different modules. Source read for this
refinement: `cmd_kernel.go` `runKernel`, `resolveKernel`,
`resolveRuntimeKernel`, `buildKernelArtifact`, `defaultKernelBuild`;
`cache.go` `kernelCacheVariantFor`, `kernelTreeCachePath`, `evictKeepN`,
`cacheEntryArch`; `runtimekernel_existing.go` `ExistingRuntimeKernelTree`;
`internal/le/setup/docker/host/build.go` `builder.build`, `runGokWith`;
`pins.go` `KeyInputs`, `ImageKey`; `actions.go` `lockedBuild`, `EnsureImage`;
and the registration pattern in `internal/le/rfc/extraction/register.go`.

`resolveRuntimeKernel` currently returns a cached vmlinuz before calling the
builder and publishes cold output to the normal variant from
`kernelCacheVariantFor`. Supplying an extra low-level builder field does not
make either normal lookup or publication proof-aware. Do not pass a fault
request through that normal resolver or rename a fault artifact to fit an old
normal key. The preparation route is an explicit addition beside normal
resolution, sharing native build/profile/requirement mechanisms rather than
changing ordinary normal selection.

| Preparation boundary | Concrete proposed contract |
|----------------------|----------------------------|
| Command registration | New owner package `internal/le/rfc/kernel-proof` registers area `rfc kernel-proof` and `prepare` action through `leroot.Register`/`RegisterActions`, with declared map answer shape, help and completion; one blank import in `internal/le/register.go`, following `rfc/extraction` |
| Authored request | Required native `arch`, explicit `builder`, tracked `patch`, and unique `kernel-function`; root comes from the selected checkout. Version, runtime target/profile, production series and fragments/requirements derive from the owning native declarations, not a replacement private recipe |
| Native callable seam | A typed `appliance.PrepareRuntimeKernelProof` request/result in new `runtimekernel_proof.go` resolves normal inputs and immutable builder/toolchain identities once, and passes normal/fault role plus optional extra patch to `kernelbuilder.Build`. It imports no le package |
| Kernel preparation result | Two typed selected trees and native receipts with normal/fault roles, common baseline/input identity, proof-patch/function identity, full artifact manifests and receipt digests. Each result names exactly what was built or validated on hit |
| Image preparation | The le action calls the extended setup docker-host builder with each explicit selected tree/receipt; the same selection reaches `ImageKey`, `runGokWith`, `buildgokrazy.prepareArgs` and `instance.Prepare`. Return both image paths, SHA256 and receipts in one structured answer |
| Record consumption | Existing proposed `rfc record` operands name these two already-prepared pairs. Preparation does no observation/record write; recording does no preparation/build. All normal/fault preparation remains subject to separate authorization |

Use a closed new child-name grammar under the existing protected runtime cache
namespace: `<version>-runtime-<arch>-runtime-<backend>-proof-<normal|fault>-<full-input-sha256>`.
The full identity is domain-separated by proof role and binds the resolved
immutable toolchain/builder/input manifest; fault additionally binds the patch
and composed-function identity. Proof-normal is also distinct from the ordinary
normal key, so a legacy warm normal artifact cannot be served as a native proof
receipt. Full receipts bind output hashes separately because identical inputs
do not promise reproducible kernel bytes.

`ExistingRuntimeKernelTree` must parse a strict union of its current legacy
normal grammar and this new proof grammar. New proof entries require a valid
matching native receipt, full role/input digest and actual kernel/module/output
manifest checks; reject any extra segment, unknown role or hash mismatch. The
normal resolver never computes a proof key or selects it implicitly. Explicit
proof selection still passes through the existing canonical cache-child,
architecture/source/release, kernel-byte, module-index, packed-image and
running-guest checks. No arbitrary path/image or weakened guard is introduced.
Existing explicit legacy pairs keep their existing execution contract but
cannot be claimed as new receipt-backed proof.

Resolve immutable builder inputs before computing a proof lookup identity and
confirm the worker reports those same actual compiler/package inputs. A proof
cache hit is valid only after its strict receipt matches the requested identity
and every actual selected output digest matches the receipt. A missing or
invalid receipt/artifact refuses with its exact path and repair route; do not
rewrite provenance or silently overwrite/delete the entry. On a miss, use an
owned output/staging directory and a per-key publication lock. Validate all
outputs and receipt before publishing the complete tree with no replacement
of an existing entry. If a concurrent publisher wins, validate that winner;
reuse it only when its receipt and actual artifacts match the requested
identity, otherwise refuse. Preserve every ordinary cache entry and
`tmp/kernel/build`; no normal CLI staging copy, cache eviction or shared input
mutation occurs during preparation. Clean only joined task-owned output.
These proof children remain under `KernelCacheNamespaces` protection; the
existing explicit eviction route remains explicit and still requires ownership
and active-use checks. New proof role/identity parsing must not silently alter
that cleanup contract.

For images, replace the vmlinuz-only `KeyInputs.KernelKey` meaning with a typed
selected identity whose mandatory component is the canonical digest of the
**complete validated actual selected output manifest**: kernel bytes, config,
all module paths/content/symlink identities and indexes, and DTBs/overlays where
packed. Include the explicit artifact-only/native-build kind and, when a full
valid native build receipt is available, its digest. Native-build identity must
cross-link that receipt to the exact actual output manifest; an invalid present
native receipt refuses rather than silently downgrading to artifact-only.
Update every key caller and fixture; ordinary image keying must also bind the
complete module set. Keep the existing pins/source/architecture/size dimensions.

Ordinary image preparation may accept a valid warm legacy selected tree without
a native build receipt. It validates and hashes the complete actual outputs and
emits an image receipt with explicit **artifact-only** selected identity. This
asserts which bytes were selected and packed, not their source/toolchain build
history, and requires no kernel rebuild, normal-cache migration or overwrite.
Native build receipt identity is mandatory only for proof-normal and fault:
their preparation and recording refuse artifact-only inputs/images, even when
all actual output and image hashes match. Ordinary image preparation does not
manufacture the missing native attestation, and its image receipt cannot serve
as a substitute. Both proof roles still use their distinct native proof cache
domain and full build receipts.

An image cache hit now requires the adjacent strict image receipt, matching
typed selected identity/full actual output manifest and image inputs, and a
streamed SHA256 of the actual raw image at that path. Native-build hits also
validate the full native receipt and selected-output cross-links; ordinary
artifact-only hits need no native receipt. Retain all existing packed-kernel
and module guards. Missing/malformed image receipts, different selected outputs
or changed raw bytes refuse before returning an image in either mode. A valid
hit returns without repacking. The explicit selected tree is bound at lookup,
packing and receipt generation, and rechecked before publication/acquisition;
neither a warm hit nor nested gok may re-resolve current runtime selection.
Preserve image locking, partial-output cleanup, Colima/QEMU lifecycle and older
shared image files; no automatic purge follows from the new key scheme.

G-AC-11 proves the full registered request path against a warm ordinary cache,
not just a direct low-level builder call. G-AC-12 holds vmlinuz constant while
changing only valid actual selected module/output manifests in both identity
modes; image keys and packed module sets must differ. G-AC-13 proves ordinary
warm-legacy preparation/reuse succeeds while the same artifact-only identity
is strictly refused for proof. A separate warm-image corruption case changes
actual raw bytes under the same path and must refuse. These cases precede native
expensive build/boot, and their fake boundaries do not replace the authorized
end-to-end native proof required by G-AC-1.

### Ratchet refinement from independent schema review — UNAPPROVED

The independent schema review found that the earlier census omitted the
ratchet's consumers. This is a design correction, not an accepted review gate
or an implementation result. Source read for this refinement:
`check_ratchets.go` `checkDiscriminationRatchet`, `driftedKey`,
`driftIsCommitted`, `discriminationDrifted`, `discriminationStaleError` and
`discriminationInput`; `check_baseline.go` `baselineRecordBlobs`; `check.go`
report assembly; `discriminate.go` `DiscriminationVerdict`; and `check_test.go`
`TestCheckDiscriminationDriftIsJudgedAgainstHead` and
`TestCheckDiscriminationDriftIsJudgedAtUnitGranularity`.

Current `driftedKey` selects `Moved`, the Go producer for producer drift, or the
tagged unit otherwise. `driftIsCommitted` then resolves that key at HEAD and in
the working tree and compares its Go behavior hash. `baselineRecordBlobs`
collects only Unit, Producer and Paired producer paths. Adding raw patch/config
hashes only to the verifier would therefore leave the ratchet reading an
unchanged Go unit. `discriminationStaleError` also assumes every proof has a Go
break and producer. All of these consumers need the typed route together.

For `patched-kernel`, extend the verdict with a typed list of stale dependency
identities rather than placing a raw path in `Moved` or inventing a Go symbol.
The new list is derived from the strict `kernel-proof` input manifest; it is
not an independently authored list. Existing old-route verdict fields and
their semantics remain unchanged.

| Descriptor field | Type / meaning | Comparison rule |
|------------------|----------------|-----------------|
| `kind` | Closed dependency kind: raw file, raw membership inventory, Go behavior, or claim text | Select the exact digest function used when the record was sealed; unknown kinds refuse. Observer whole-file hashes and package/fixture membership use the raw kinds |
| `path` | Safe repository-relative source path, or directory/pattern root for membership | `baselineRecordBlobs` collects file blobs and HEAD tree membership explicitly; compare those with exact current files/inventories using stored conservative rules, without falling back to live files for a missing HEAD blob |
| `symbol` / cover | Go symbol for behavior, or exact claim cover for claim text; absent on raw input | Resolve only the recorded function/claim scope, so an unrelated function or comment cannot explain its drift |
| `recorded-digest` | Digest already bound by the native sealed manifest/snapshot | Compare raw file bytes with full SHA256; membership uses the canonical sorted path/type inventory under its stored rules, including absence. Go behavior uses the existing behavior fingerprint and claim text uses `claimSHA`; never apply `behaviorBytes` to raw source, patch, recipe, series, config or requirement inputs |
| C proof context | Composed kernel source path/function and the tracked proof-patch path from `kernel-proof` | Carry into both stale refusal and uncommitted-drift diagnostics; never print an empty Go Producer or Break |

The emitted resolved config, kernel/module/image outputs and binary are derived
receipt values, not tracked HEAD paths. Validate their receipt cross-links
offline; do not ask git for a generated output or make cache absence a source
drift. Their tracked producing fragments, requirements, recipes, source pin,
series and patches are raw dependencies with explicit repository paths.

For every new-route dependency whose current digest mismatches the recorded
one, independently read its HEAD and current identity at that kind's scope.
The mismatch already makes the working-tree verdict unverified. Extend the
existing ratchet's comparison at this exact dependency scope: when HEAD and
current identities differ, the working edit explains that dependency's drift
and `discriminationDrifted` reports it without a stale-record violation; when
HEAD and current identities are equal, the stale dependency was already wrong
at HEAD, so `checkDiscriminationRatchet` refuses it. Thus unchanged HEAD plus
only a working raw edit reports, while the same changed bytes committed at
HEAD refuse. A matching, freshly observed working-tree record remains eligible
for verification before its producing change and record land together.
Collect and classify all mismatches rather than letting the first uncommitted
one hide another committed dependency. An unrelated working edit to a different
dependency, or outside a recorded Go function/claim, cannot explain the stale
dependency under comparison. A file absent at a known HEAD is an explicit
absent identity, not an empty file: absent at both HEAD and current is committed
absence, absent only at HEAD is a working addition, and absent only at current
is a working deletion. Where git cannot answer, preserve the current
no-committed-attribution policy, report the unavailable comparison explicitly,
and grant no stale proof credit.

`check.go` must supply the expanded HEAD dependency blobs and typed verdicts to
both ratchet and report paths. `checkDiscriminationRatchet`, `driftedKey` and
`driftIsCommitted` gain the route-specific typed dispatch; old routes retain
their existing Go-key path. `discriminationDrifted` and
`discriminationStaleError` render the same exact changed dependency and C proof
context. Proven counts, partial audit, weekly and site consumers continue to
require a verified verdict, so both committed and uncommitted stale cases
receive zero credit. No cache access, build, boot or new execution is needed
to classify this drift.

G-AC-10's tests are a pair over the same sealed proof and same changed bytes,
following the existing `check_test.go` ratchet fixtures: unchanged HEAD plus
working edit must report without refusing, then that exact edit at HEAD must
refuse. Each half asserts the proven count is zero, the diagnostic names the
raw dependency and C function/proof patch, and stale proof is excluded from
downstream counts. Repeat for recipe, fragment/config/require, ordered series,
production patch and proof patch. Add the separate mixed-drift and exact-scope
tests above, including missing versus unreadable HEAD, so neither an always
reporting classifier nor an always refusing classifier can satisfy the design.

### Design challenge and admission state

| Challenge | Answer |
|-----------|--------|
| Simplicity | Prebuilt pairs keep build orchestration out of record and reuse the native builder, image packer, guest lifecycle and single writer; receipt and attribution checks are necessary to distinguish the claimed C red |
| Uniformity | Add one typed route to the existing record/action/verifier flow. Preserve old routes and one cover identity; never add a hand-written proof or separate ledger |
| Performance | Tooling only. Stream artifact SHA256; store bounded manifests/decisive output and small function snapshots, not kernels/source trees in records. No product packet path changes |
| Strongest concern | The minimum spans native build attestation, explicit image selection and exact assertion attribution. Implementing only the route vocabulary would publish apparent proof without the provenance that makes it meaningful |

Admission remains **UNAPPROVED / REWORK** until the main session compares the
generic ACs and changed-wire census with this Task, the preserved mechanisms,
current source and owner rulings, then presents the complete dependency
promotion decision. No design acceptance, native safety approval, implementation
or test result is claimed by this refresh.
