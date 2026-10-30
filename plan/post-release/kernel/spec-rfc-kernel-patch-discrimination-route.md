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
