# Spec: ipsec-ipcomp

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Deferral shard | `-` (corrected 2026-08-03: the row named a shard that never existed; not started; the VPP refusal is a design decision in scope, not postponed work. Create `plan/deferrals/ipsec-ipcomp.md` on the first deferral) |
| Updated | 2026-07-31 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Implement IP Payload Compression (IPComp) for IKEv2 Child SAs, as RFC 7296 Section 2.22
defines it. Ze does not implement IPComp today. The goal is a negotiated, configurable,
dataplane-backed feature on both dataplane backends, with the four RFC 7296 Section 2.22
MUST NOT obligations proven by tests.

### Provenance (do not delete)

Four RFC compliance rows were deferred out of the rfcgate-1b RFC 7296 pilot spec into
this spec. The owner decided on 2026-07-31: "Create a spec to fully implement IPComp, do not
do it as part of this session." The pilot spec's phase list calls the four rows work package
WP-11. The 2026-07-30 re-triage renumbered them to work package WP-16. The row identifiers
are the stable name. The package number is not.

A complete read-only design pass over the four rows exists at `tmp/design-wp11.md`, written
2026-07-31. It holds the verbatim RFC text, the producer citations, the identifier
allocation, and the conformance argument. Read it before the design phase of this spec.

### The four compliance rows, quoted verbatim

| Row | Level | Text |
|-----|-------|------|
| `RFC7296-2.22-1` | MUST NOT | `These payloads MUST NOT occur in messages that do not contain SA payloads (§2.22)` |
| `RFC7296-2.22-2` | MUST NOT | `Implementations of this specification MUST NOT accept an IPComp algorithm that was not proposed (§2.22)` |
| `RFC7296-2.22-3` | MUST NOT | `Implementations of this specification MUST NOT accept more than one IPComp algorithm (§2.22)` |
| `RFC7296-2.22-4` | MUST NOT | `Implementations of this specification MUST NOT compress using an algorithm other than one proposed and accepted in the setup of the Child SA (§2.22)` |

The `(§2.22)` citation on each row is load-bearing. `parse_checklist_line`
(`scripts/dev/rfc_requirements.py`) validates that a row identifier's section segment agrees
with its citation.

### Identifier allocation

Section 2.22 has no committed identifier in `rfc/short/rfc7296.md`, so `check_id_allocation`
skips the section and all four rows land at their Appendix A ordinals `-1` through `-4`.
Verified 2026-07-31 with the command below, which returned an empty result and a count of
zero:

    git show HEAD:rfc/short/rfc7296.md | grep -o 'RFC7296-2\.22-[0-9]*' | sort -V | tail -1

Recompute at the moment of landing, and never hardcode. A section with no high-water mark is
a trap: the first row to land sets the mark and blocks every lower ordinal. Land all four
rows in one commit, in ascending order.

### Why this is a feature spec and not four tests

Ze satisfies all four MUST NOT obligations today by non-participation, and RFC 7296 makes
non-participation legal. Offering is a MAY (`rfc/full/rfc7296.txt:3406-3409`). Accepting is a
MAY. Ignoring an unrecognized status notification is mandatory,
and IPCOMP_SUPPORTED is 16387, above the 16383 error ceiling, so it is a status
type. The owner chose the feature over the four tests.

Once Ze negotiates IPComp, the four obligations become active constraints on real code. They
get harder, not easier. The tests written for them must assert over a real negotiation.

## Required Reading

### Architecture Docs
- [ ] `tmp/design-wp11.md` - the read-only design pass over the four rows
  → Constraint: a receive-side rejection of IPCOMP_SUPPORTED violates
    `RFC7296-3.10.1-2` and breaks strongSwan interoperability. Declining is expressed by
    omission from the response, never by an error notification.
  → Constraint: `test/ipsec-interop/` is `TIER_UNRUN`, so a tagged test there is refused.
- [ ] `ai/rules/protocol.md` - backend translation honesty
  → Constraint: a backend that cannot apply the operator's compression config must fail at
    `ze config verify` with a clear error, never approximate it.
- [ ] `ai/rules/architecture.md` - both dataplanes
  → Constraint: a netlink-only feature creates drift. The VPP backend needs the feature or an
    explicit refusal in the same work.
- [ ] `ai/rules/platform-linux.md` - Linux-only code needs QEMU coverage
  → Constraint: the XFRM half is `//go:build linux`, so it needs a QEMU integration test.
- [ ] `ai/rules/repo-maintenance.md` - runtime dependency readiness
  → Constraint: a kernel module dependency needs a registered `ze doctor` check with a
    diagnostic code, a unit test, and a functional test.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc7296.md` - IKEv2. Section 2.22 has no content in the summary today.
  → Constraint: the compression association has no life outside the ESP or AH SA that
    contains it, and it disappears when that SA goes away (`rfc/full/rfc7296.txt:3399-3403`).
  → Constraint: a message that proposes an SA CAN carry multiple IPCOMP_SUPPORTED
    notifications. A message that accepts an SA carries at most one.
  → Constraint: an IPCOMP_SUPPORTED notification is legal in an IKE_AUTH request and response,
    and in a CREATE_CHILD_SA request and response, and nowhere else
    (`rfc/full/rfc7296.txt:7718`, `:7738`, `:7807`, `:7815`).
  → Constraint: the registered transform identifiers are IPCOMP_DEFLATE 2, IPCOMP_LZS 3, and
    IPCOMP_LZJH 4 (`rfc/full/rfc7296.txt:3439-3441`).
- [ ] `rfc/short/rfc3173.md` - IP Payload Compression Protocol. MUST CREATE if absent.
  → Constraint: the CPI and the compression association semantics live here.

**Key insights:** (minimal context to resume after compaction)
- Ze holds one IPComp symbol and reads it nowhere. Everything else is new code.
- The four MUST NOT rows constrain the acceptance decision and the compression decision. They
  do not constrain what Ze proposes.
- Declining a peer offer must never fail the Child SA.

## Current Behavior (MANDATORY)

**Source files read:** (verified in the working tree on 2026-07-31)
- [ ] `internal/component/ike/wire/payload_notify.go` - declares
  `NotifyIPCompSupported uint16 = 16387`. A case-insensitive search for `ipcomp` across
  `internal/`, `pkg/` and `cmd/` returns this line and nothing else. The same search across
  `test/` returns nothing.
- [ ] `internal/component/ike/engine/responder.go` - `(*PeerSession).handleAuthRequest`
  consumes an IKE_AUTH request. Its notify branch at `:387-399` tests two constants,
  `NotifyInitialContact` and `NotifySetWindowSize`. It has no default branch. It stores no
  notification that neither constant matches.
- [ ] `internal/component/ike/engine/responder.go` - `(*PeerSession).buildAuthResponse`
  builds the IKE_AUTH response inner chain. It emits no notification of any type.
- [ ] `internal/component/ike/engine/rekey.go` - `respondChildRekey` builds the
  CREATE_CHILD_SA response. It emits no notification of any type.
- [ ] `internal/component/ike/engine/rekey.go` - `initiateChildRekey` emits REKEY_SA only.
- [ ] `internal/component/ike/engine/auth.go` - `buildAuthRequest` emits INITIAL_CONTACT
  only.
- [ ] `internal/component/ike/engine/child.go` - `installChildSA` programs the dataplane.
  It sets `Proto: protoESP` at `:256`, `:285`, `:316` and `:332`, with `protoESP = 50` at
  `:51`.
- [ ] `internal/component/ike/engine/child.go` - `type ChildSA struct` carries no
  compression state.
- [ ] `internal/component/ike/dataplane/dataplane.go` - `type SAParams struct` has no
  compression field. The `Proto` field at `:166` is documented as ESP (50) or AH (51), and
  those two constants are declared at `:30-31`.
- [ ] `internal/component/ike/dataplane/dataplane.go` - `type Dataplane interface`
  declares `InstallSA`, `RemoveSA`, `InstallPolicy`, `RemovePolicy`, `RemovePolicyParams`,
  `ListSAs` and `Close`.
- [ ] `internal/component/ike/dataplane/xfrm_linux.go` - the Linux backend `InstallSA`.
- [ ] `internal/component/ike/dataplane/vpp.go` - the VPP backend `InstallSA`.
- [ ] `internal/component/ike/ipsec/types.go` - `type ESPProposal struct` carries Number,
  Encryption and Hash. `type ESPGroup struct` at `:350` carries Name, Lifetime, PFS and
  Proposals.
- [ ] `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang` - `list esp-group`, with
  `list proposal` at `:82`. This is the Child SA parameter group.
- [ ] `internal/component/ike/engine/register.go` - the IKE plugin's existing
  `Registration.DoctorChecks` declaration.

**Behavior to preserve:** (unless the user explicitly said to change it)
- Every `test/ipsec/*.ci` stays green. The suite runs inside `ze-verify`
  (`mk/test-functional.mk` lists it in `all_suites`, and `:217` carries its `run_suite`
  line).
- Every scenario under `test/ipsec-interop/scenarios/` stays green. The directory holds
  `01`, `02`, `03`, `04`, `05`, `07`, `08`, `09`, `10` and `11`.
- A peer that offers IPComp while Ze declines still establishes its Child SA.
- A peer that offers no IPComp is unaffected on every code path.
- `show vpn ipsec sa` keeps every field it prints today.

**Behavior to change:** (only what the user asked for)
- Ze offers IPCOMP_SUPPORTED when the operator enables compression on the ESP group.
- Ze accepts at most one offered algorithm as a responder, and only one it proposed.
- Ze programs a compression association in the dataplane when negotiation succeeds.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config: the operator sets a compression container under an ESP group in the config file.
- Wire: an inbound IKE_AUTH or CREATE_CHILD_SA message carries one or more IPCOMP_SUPPORTED
  notification payloads.

### Transformation Path
1. Config parse. The YANG tree yields the compression settings into `ipsec.ESPGroup`
   (`internal/component/ike/ipsec/types.go`).
2. Offer construction. `buildAuthRequest` (`internal/component/ike/engine/auth.go`) and
   `initiateChildRekey` (`internal/component/ike/engine/rekey.go`) append one
   IPCOMP_SUPPORTED payload for each proposed algorithm, each carrying a locally allocated
   CPI and the transform identifier.
3. Offer consumption. `(*PeerSession).handleAuthRequest`
   (`internal/component/ike/engine/responder.go`) and the CREATE_CHILD_SA request path
   collect the offered set.
4. Acceptance. The responder selects at most one algorithm from the intersection of the
   offered set and the locally configured set, and records the peer CPI.
5. Response construction. `buildAuthResponse`
   (`internal/component/ike/engine/responder.go`) and `respondChildRekey`
   (`internal/component/ike/engine/rekey.go`) append exactly one IPCOMP_SUPPORTED payload
   when an algorithm was accepted, and none otherwise.
6. Child SA binding. The negotiated algorithm and the CPI pair are stored on `ChildSA`
   (`internal/component/ike/engine/child.go`).
7. Dataplane. `installChildSA` (`internal/component/ike/engine/child.go`) programs the
   compression association through the `Dataplane` interface
   (`internal/component/ike/dataplane/dataplane.go`).
8. Teardown. The compression association is removed with the Child SA, because it has no life
   outside that SA (`rfc/full/rfc7296.txt:3399-3403`).

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config tree ↔ IKE engine | `ipsec.ESPGroup` fields | No |
| IKE engine ↔ wire codec | `wire.PayloadNotify` with type 16387 | No |
| IKE engine ↔ dataplane | `dataplane.SAParams` gains compression fields | No |
| Dataplane ↔ Linux kernel | XFRM netlink IPComp state and template | No |
| Dataplane ↔ VPP | binary API, or an explicit refusal | No |

### Integration Points
- `wire.PayloadNotify` (`internal/component/ike/wire/payload_notify.go`) already parses and
  writes the payload shape. The codec needs a CPI and transform accessor, not a new payload.
- `dataplane.Dataplane` (`internal/component/ike/dataplane/dataplane.go`) is the single
  seam both backends implement.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Section 2.22 still has no committed identifier when the four rows land | `git show HEAD:rfc/short/rfc7296.md`, empty result, 2026-07-31 | The ordinals shift upward and lose document order | Rerun the allocation command at landing time | unvalidated |
| A-2 | The Linux kernel supports IPComp through XFRM with the `xfrm_ipcomp` and `deflate` modules | Not yet read | The XFRM backend cannot deliver the feature and the whole spec changes shape | Read the vendored netlink library and probe a QEMU guest | unvalidated |
| A-3 | The VPP binary API exposes no IPComp SA type usable from Ze | `internal/component/ike/dataplane/vpp.go`, not yet read for this purpose | The VPP backend implements the feature rather than refusing it | Read the vendored VPP binary API definitions | unvalidated |
| A-4 | `wire.PayloadNotify` carries the CPI and transform identifier without a codec change | `internal/component/ike/wire/payload_notify.go` declares the type constant only | The wire layer needs new accessors and new tests | Read `ReadFrom` and `WriteTo` and write a round-trip test | unvalidated |
| A-5 | strongSwan offers IPComp when its config sets a compression option | Not yet read | The interop scenario cannot exercise the negotiation | Read the strongSwan config reference and build scenario `20-ipcomp` | unvalidated |
| A-6 | `test/ipsec-interop/` remains `TIER_UNRUN` | `scripts/dev/rfc_requirements.py`, `:876-878`, refusal at `:952` and `:1004` | An interop tag becomes legal evidence and the test plan gains an option | Rerun `make ze-rfc-check` at landing time | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | An implementer adds a receive-side rejection of IPCOMP_SUPPORTED, reading four MUST NOT rows as a security rule. That violates `RFC7296-3.10.1-2` and breaks strongSwan interoperability | An interop scenario stops establishing when the peer offers IPComp | Every row test asserts that the SA still establishes in the presence of an offer. Declining is omission from the response, never an error notification |
| R-2 | A declined offer fails the whole Child SA. RFC 7296 states the pattern for a sibling notification: the responder omits the notification from its response and does not reject the Child SA creation (`rfc/full/rfc7296.txt:826-828`) | A peer that offers IPComp gets no Child SA at all | Assert establishment, not merely the absence of an acceptance |
| R-3 | Ze accepts more than one algorithm, or one it did not propose. This is the direct `2.22-2` and `2.22-3` violation | A response carries two IPCOMP_SUPPORTED payloads, or one whose transform is not in the offered set | Select from the intersection, cap the response at one payload, and test both cases with the counter proven to see the payloads it counts |
| R-4 | The CPI outlives its Child SA, or leaks on rekey. RFC 7296 requires the compression association to disappear with the SA (`rfc/full/rfc7296.txt:3399-3403`) | A long-running session accumulates CPI allocations | A CPI allocator owned by the Child SA lifecycle, with a release path proven by a rekey test |
| R-5 | The feature lands on XFRM only, and the VPP backend silently ignores the compression request. `ai/rules/protocol.md` forbids silent approximation | `ze config verify` accepts a compression config that the active backend cannot apply | The VPP backend refuses at verify time with an error naming the backend and the unsupported setting |
| R-6 | The tests pass over an empty sample. A test that asserts an absence over zero collected messages is green either way | None. The test is green | Every absence assertion carries a non-empty count assertion beside it |
| R-7 | The four tagged tests are placed under `test/ipsec-interop/`, which is `TIER_UNRUN`. A tag there is refused by `_refuse_unrun` (`scripts/dev/rfc_requirements.py`, raised at `:1004`) | `make ze-rfc-check` fails naming the file | Place the tags in `_test.go` or in `test/ipsec/*.ci`. Build the interop scenario as untagged coverage |
| R-8 | Engine line numbers move. Other agents edit `internal/component/ike/engine/` concurrently | A tag cites a line holding different code | Every citation in this spec names its function. Relocate by function name before you quote a line |
| R-9 | The XFRM half is Linux-only and ships without QEMU coverage. No `ze-qemu-*-ipsec` target exists today | The feature is unproven on a real kernel | A QEMU integration test and a make target, per `ai/rules/platform-linux.md` |
| R-10 | Compression expands small or already compressed packets. RFC 3173 requires the sender to discard the compressed form when it is not smaller | Throughput drops on a compressed tunnel | Read RFC 3173 during design and record the size decision as an acceptance criterion |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | An IPsec tunnel carries traffic the peer cannot decompress, or a peer that offers IPComp loses its Child SA. Both are total data-path loss for that tunnel |
| How is it reverted? | The negotiation is off by default, so a single commit revert is safe while the default holds. Once an operator enables it and peers negotiate, a revert drops those tunnels until both ends are downgraded |
| Who else touches this path? | the rfcgate-1b RFC 7296 pilot spec (the four rows, and the generic error-notification sender its WP-3 and WP-4 build), `plan/spec-fixit-vpp-ipsec-inoperable.md` (the VPP backend cannot program a security association at all today), `plan/spec-ipsec-remote-access.md` (the same engine and the same Child SA path) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Config file sets compression on an ESP group | → | The compression set reaches `ipsec.ESPGroup` and then `buildAuthRequest` (`internal/component/ike/engine/auth.go`) | `test/ipsec/ipsec-ipcomp-offer.ci` |
| An inbound IKE_AUTH request carries IPCOMP_SUPPORTED | → | `(*PeerSession).handleAuthRequest` (`internal/component/ike/engine/responder.go`) records the offered set | `TestIPCompOfferRecordedOnAuthRequest` |
| An accepted algorithm reaches the dataplane | → | `installChildSA` (`internal/component/ike/engine/child.go`) | `TestInstallChildSAProgramsCompressionAssociation` |
| The VPP backend receives a compression request | → | `(*vppBackend).InstallSA` (`internal/component/ike/dataplane/vpp.go`) | `TestVPPBackendRefusesCompression` |
| `ze doctor` runs on a host without the kernel IPComp module | → | The IKE plugin doctor check (`internal/component/ike/engine/register.go`) | `TestDoctorReportsMissingIPCompModule` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | An ESP group with compression enabled and one algorithm | The IKE_AUTH request carries one IPCOMP_SUPPORTED payload with a non-zero CPI and the configured transform identifier |
| AC-2 | An ESP group with compression enabled and three algorithms | The IKE_AUTH request carries three IPCOMP_SUPPORTED payloads with distinct transform identifiers |
| AC-3 | Compression is not configured | No message Ze sends carries an IPCOMP_SUPPORTED payload, in any exchange |
| AC-4 | A peer offers three algorithms and Ze supports two of them | The response carries exactly one IPCOMP_SUPPORTED payload, and its transform identifier is one Ze proposed |
| AC-5 | A peer offers one algorithm that Ze does not support | The response carries no IPCOMP_SUPPORTED payload, and the Child SA still reaches the established state |
| AC-6 | A peer that offers no IPComp establishes a Child SA | The exchange is byte-identical to the exchange before this spec landed |
| AC-7 | A response accepts an algorithm Ze never proposed | Ze refuses the acceptance, and the failure names the offending transform identifier |
| AC-8 | A response carries two IPCOMP_SUPPORTED payloads | Ze refuses the response, and the failure names the count |
| AC-9 | A Child SA negotiates compression successfully on Linux | The XFRM backend programs an IPComp state and a matching policy template, and the tunnel carries compressed traffic |
| AC-10 | The same config is committed with the VPP backend active | `ze config verify` fails with an error naming the VPP backend and the unsupported compression setting |
| AC-11 | A compressed Child SA is deleted or rekeyed | The compression association is removed, and its CPI is released |
| AC-12 | No message that lacks an SA payload | No such message carries an IPCOMP_SUPPORTED payload, over a full establishment plus a Child rekey plus a liveness probe |
| AC-13 | The kernel IPComp module is absent and compression is configured | `ze doctor` reports a failing check with a registered diagnostic code and a remediation line |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Enables compression on an ESP group and commits | config parse → `ipsec.ESPGroup` → `buildAuthRequest` → wire | `test/ipsec/ipsec-ipcomp-offer.ci` |
| 2 | Brings up a tunnel to a strongSwan peer that offers IPComp | wire → `handleAuthRequest` → acceptance → `installChildSA` → XFRM | `test/ipsec-interop/scenarios/20-ipcomp` (untagged) |
| 3 | Reads the negotiated state | engine state → `show vpn ipsec sa` | `test/ipsec/ipsec-show-sa-ipcomp.ci` |
| 4 | Commits a compression config while VPP is the active backend | config verify → backend capability check → rejection | `test/ipsec/ipsec-ipcomp-vpp-reject.ci` |
| 5 | Runs `ze doctor` before the daemon starts | doctor registry → IKE plugin check → kernel module probe | `TestDoctorReportsMissingIPCompModule` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestIPCompNotifyRoundTrip` | `internal/component/ike/wire/rfc7296_notify_test.go` | The codec writes and reads a CPI and a transform identifier without loss | |
| `TestIPCompOfferRecordedOnAuthRequest` | `internal/component/ike/engine/rfc7296_ipcomp_test.go` | The responder collects the offered set | |
| `TestIPCompAcceptsExactlyOneProposedAlgorithm` | `internal/component/ike/engine/rfc7296_ipcomp_test.go` | AC-4. Tagged `RFC requirement: RFC7296-2.22-3 positive` | |
| `TestIPCompRefusesUnproposedAlgorithm` | `internal/component/ike/engine/rfc7296_ipcomp_test.go` | AC-7. Tagged `RFC requirement: RFC7296-2.22-2 positive` | |
| `TestIPCompRefusesMultipleAcceptances` | `internal/component/ike/engine/rfc7296_ipcomp_test.go` | AC-8. Tagged `RFC requirement: RFC7296-2.22-3 negative` | |
| `TestIPCompNotifyAbsentFromSAFreeMessages` | `internal/component/ike/engine/rfc7296_ipcomp_test.go` | AC-12. Tagged `RFC requirement: RFC7296-2.22-1 positive` | |
| `TestCompressionUsesOnlyTheAcceptedAlgorithm` | `internal/component/ike/engine/rfc7296_ipcomp_test.go` | AC-9 and AC-11. Tagged `RFC requirement: RFC7296-2.22-4 positive` | |
| `TestChildSAReleasesCPIOnTeardown` | `internal/component/ike/engine/rfc7296_ipcomp_test.go` | AC-11 | |
| `TestVPPBackendRefusesCompression` | `internal/component/ike/dataplane/vpp_test.go` | AC-10 | |
| `TestXFRMInstallsCompressionState` | `internal/component/ike/dataplane/xfrm_linux_test.go` | The Linux backend builds the right netlink state | |
| `TestDoctorReportsMissingIPCompModule` | `internal/component/ike/engine/doctor_test.go` | AC-13 | |

Each tagged test needs an anti-vacuity companion. An absence assertion carries a non-empty
count assertion beside it. A counter is proven over a chain that holds the payloads it counts.

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| CPI | 256-65535 (values 0-255 are reserved for well-known transforms) | 65535 | 255 | N/A |
| Transform identifier | 2-4 (DEFLATE, LZS, LZJH per `rfc/full/rfc7296.txt:3439-3441`) | 4 | 1 | 5 |
| IPCOMP_SUPPORTED notification data length | 3 octets (2-octet CPI plus 1-octet transform) | 3 | 2 | 4 |
| Accepted algorithm count in a response | 0-1 (`rfc/full/rfc7296.txt:3426-3428`) | 1 | N/A | 2 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ipsec-ipcomp-offer` | `test/ipsec/ipsec-ipcomp-offer.ci` | The operator enables compression and the daemon offers it | |
| `ipsec-ipcomp-disabled` | `test/ipsec/ipsec-ipcomp-disabled.ci` | Compression stays off by default and no payload is sent | |
| `ipsec-ipcomp-vpp-reject` | `test/ipsec/ipsec-ipcomp-vpp-reject.ci` | The operator commits compression with VPP active and gets a clear error | |
| `ipsec-show-sa-ipcomp` | `test/ipsec/ipsec-show-sa-ipcomp.ci` | The operator reads the negotiated algorithm and CPI | |

The `ipsec` suite runs inside `ze-verify` (`mk/test-functional.mk` and `:217`), so a
`.ci` there earns a verify tier. A `.ci` that drives a crafted IKEv2 inner payload chain needs
a scripted IKEv2 peer, and `internal/test/cli/` has none today. Building one is in scope for
the design phase, and it serves many other rows in the rfcgate-1b RFC 7296 pilot spec.

### QEMU Integration Tests
| Test | Package | Validates | Status |
|------|---------|-----------|--------|
| `TestXFRMIPCompStateIntegration` | `internal/component/ike/dataplane/xfrm_integration_linux_test.go` | The kernel accepts the IPComp state and template in a network namespace | |

The package needs a make target. No `ze-qemu-*` IPsec target exists today.

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `20-ipcomp` | `test/ipsec-interop/scenarios/` | strongSwan | A strongSwan peer with compression enabled negotiates IPComp with Ze, and traffic flows compressed | |
| `21-ipcomp-declined` | `test/ipsec-interop/scenarios/` | strongSwan | A strongSwan peer that offers IPComp against a Ze with compression disabled still establishes its Child SA | |

**These scenarios cannot carry an RFC tag.** `test/ipsec-interop/` is declared `TIER_UNRUN`
(`scripts/dev/rfc_requirements.py`), and a tag there raises `_refuse_unrun`
(`:952`, raised at `:1004`). Nothing runs the suite automatically, so a tag there would be
evidence nothing executes. Compliance evidence for the four rows must be unit tier or
functional tier. Write that reason into each scenario header, so a later reader does not add
a tag.

## Files to Modify
- `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang` - the compression container under
  `list esp-group`
- `internal/component/ike/ipsec/types.go` - `ESPGroup` gains the compression set
- `internal/component/ike/wire/payload_notify.go` - CPI and transform accessors beside the
  existing constant
- `internal/component/ike/engine/auth.go` - `buildAuthRequest` offers
- `internal/component/ike/engine/responder.go` - `handleAuthRequest` collects,
  `buildAuthResponse` accepts at most one
- `internal/component/ike/engine/rekey.go` - `initiateChildRekey` offers,
  `respondChildRekey` accepts at most one
- `internal/component/ike/engine/fsm.go` - the initiator validates the response acceptance
- `internal/component/ike/engine/child.go` - `ChildSA` carries the negotiated state,
  `installChildSA` programs it
- `internal/component/ike/engine/register.go` - the doctor check
- `internal/component/ike/dataplane/dataplane.go` - `SAParams` gains compression
  fields
- `internal/component/ike/dataplane/xfrm_linux.go` - `InstallSA` programs the state
- `internal/component/ike/dataplane/vpp.go` - `InstallSA` refuses
- `internal/core/diagnostic/codes.go` - the doctor diagnostic code
- `mk/test-integration.mk` - the QEMU target
- `rfc/short/rfc7296.md` - the four checklist rows and the Section Index entry
- `docs/features/rfc-status.md` - RFC 7296 row
- `ai/RFC-REQUIREMENTS.md` - regenerated with `make ze-rfc-index`

## Files to Create
- `internal/component/ike/engine/rfc7296_ipcomp_test.go` - the tagged compliance tests
- `internal/component/ike/engine/ipcomp.go` - CPI allocation, the offer and acceptance logic
- `test/ipsec/ipsec-ipcomp-offer.ci` - the operator-facing offer test
- `test/ipsec/ipsec-ipcomp-disabled.ci` - the default-off test
- `test/ipsec/ipsec-ipcomp-vpp-reject.ci` - the backend rejection test
- `test/ipsec/ipsec-show-sa-ipcomp.ci` - the operational read test
- `test/ipsec-interop/scenarios/20-ipcomp/` - the strongSwan negotiation scenario
- `test/ipsec-interop/scenarios/21-ipcomp-declined/` - the strongSwan decline scenario
- `rfc/short/rfc3173.md` - the IP Payload Compression Protocol summary, if absent
- `docs/guide/vpn/ipsec-compression.md` - the operator guide page

### Proposed Config Surface

Expressed as a table, because `ai/rules/spec-no-code.md` forbids code in a spec. The final
shape is a design-phase decision.

| Path | Type | Default | Units | Purpose |
|------|------|---------|-------|---------|
| `vpn ipsec esp-group <name> compression enabled` | boolean | `false` | - | Offer and accept IPComp on Child SAs from this group |
| `vpn ipsec esp-group <name> compression algorithm` | leaf-list, enumeration `deflate`, `lzs`, `lzjh` | `deflate` | - | The proposed set, in preference order |

The names follow `ai/rules/config.md`: kebab-case, no abbreviations, and a positive
boolean named `enabled`. Neither leaf carries a dimension, so neither needs a `units`
statement. `compression` is a container under the existing `esp-group`, because IPComp is a
Child SA property and `esp-group` is the Child SA parameter group
(`internal/component/ike/ipsec/yang/ze-ipsec-conf.yang`).

Design phase must answer two open questions. First, whether a minimum packet size leaf is
needed, which would carry `units bytes`. Second, whether the algorithm enumeration belongs in
`ze-types.yang` beside the other shared IPsec vocabulary.

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang`, the compression container under `list esp-group` |
| YANG validation constraints | Yes | The algorithm leaf-list is an `enumeration`. The enabled leaf is a `boolean` with `default false` |
| YANG custom validators | No | Both leaves take native YANG constraints. No runtime-determined set is involved |
| CLI commands/flags | No | The feature is config-driven. `show vpn ipsec sa` gains fields, and that command exists |
| CLI grammar (keyword before value) | N-A | No new command |
| Editor autocomplete | Yes | Automatic for the enumeration leaf. Confirm during the design phase |
| Functional test for new RPC/API | Yes | `test/ipsec/ipsec-show-sa-ipcomp.ci` |
| Pipe completeness | Yes | `show vpn ipsec sa` already routes through the pipe framework. The new fields must survive `\| json` |
| Env var registration | No | No leaf under `environment/` is added |
| Doctor check for runtime dependencies | Yes | The kernel IPComp module is a runtime dependency. Check in `internal/component/ike/engine/`, code in `internal/core/diagnostic/codes.go`, plus a unit test and a functional test |
| Prometheus counters/metrics | Yes | Compressed and uncompressed byte counters per Child SA. Name them during the design phase |
| BGP family surface (new SAFI / capability / attribute) | N-A | This is IKEv2 and IPsec, not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` |
| 3 | CLI command added/changed? | No | `show vpn ipsec sa` gains fields, and its reference page is covered by row 6 |
| 4 | API/RPC added/changed? | No | No new RPC |
| 5 | Plugin added/changed? | No | The IKE plugin registration shape is unchanged |
| 6 | Has a user guide page? | Yes | `docs/guide/vpn/ipsec-compression.md` (new) |
| 7 | Wire format changed? | Yes | The IKEv2 notification surface. Confirm the target page during the design phase |
| 8 | Plugin SDK/protocol changed? | No | No SDK surface changes |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc7296.md` and `docs/features/rfc-status.md` |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md`, for the QEMU target and any scripted IKEv2 peer |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md`. strongSwan and libreswan both support IPComp |
| 12 | Internal architecture changed? | Yes | The IPsec subsystem architecture page. Confirm the path during the design phase |
| 13 | Route metadata keys added/changed? | No | No route metadata is involved |
| 14 | Prometheus counters added/changed? | Yes | The compression counters named in the Integration Checklist |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | Nothing new registers |
| 16 | Any changed source file referenced by existing doc source anchors? | | Grep `docs/` for anchors naming the files in Files to Modify |
| 17 | Existing docs show config/CLI/API examples for this area? | | Verify every `esp-group` example against the changed YANG |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- register entry points, write failing wiring tests
   - Tests: every row of the Wiring Test table
   - Files: `ze-ipsec-conf.yang`, `internal/component/ike/ipsec/types.go`,
     `internal/component/ike/engine/ipcomp.go` (stub), `internal/component/ike/engine/register.go`
   - Verify: the config leaf parses, reaches `ESPGroup`, and the wiring tests fail because the
     negotiation is a stub
2. **Phase: Wire codec and CPI allocation**
   - Tests: `TestIPCompNotifyRoundTrip`, the CPI boundary tests
   - Files: `internal/component/ike/wire/payload_notify.go`,
     `internal/component/ike/engine/ipcomp.go`
   - Verify: a CPI and a transform identifier survive a write and a read. The allocator refuses
     the reserved range 0 to 255
3. **Phase: Offer construction**
   - Tests: AC-1, AC-2, AC-3
   - Files: `internal/component/ike/engine/auth.go`, `internal/component/ike/engine/rekey.go`
   - Verify: the request carries one payload for each configured algorithm, and none when
     compression is off
4. **Phase: Acceptance and refusal**
   - Tests: AC-4 through AC-8, AC-12, and the four tagged compliance tests
   - Files: `internal/component/ike/engine/responder.go`,
     `internal/component/ike/engine/rekey.go`, `internal/component/ike/engine/fsm.go`
   - Verify: at most one acceptance, only from the proposed set, and a declined offer still
     establishes the Child SA. Run the mutation list in the Critical Review Checklist
5. **Phase: Child SA binding and lifecycle**
   - Tests: AC-11, `TestChildSAReleasesCPIOnTeardown`
   - Files: `internal/component/ike/engine/child.go`
   - Verify: the compression association is created with the Child SA and removed with it,
     across a delete and across a rekey
6. **Phase: Dataplane, both backends**
   - Tests: AC-9, AC-10, `TestXFRMInstallsCompressionState`, `TestVPPBackendRefusesCompression`
   - Files: `internal/component/ike/dataplane/dataplane.go`,
     `internal/component/ike/dataplane/xfrm_linux.go`, `internal/component/ike/dataplane/vpp.go`
   - Verify: the Linux backend programs the state, and the VPP backend refuses at verify time
     with an error naming the backend and the setting
7. **Phase: Doctor, QEMU, and interoperability**
   - Tests: AC-13, `TestXFRMIPCompStateIntegration`, scenarios `20-ipcomp` and
     `21-ipcomp-declined`
   - Files: `internal/component/ike/engine/register.go`, `internal/core/diagnostic/codes.go`,
     `mk/test-integration.mk`, the two scenario directories
   - Verify: `ze doctor --json` reports the check, the QEMU target passes, and both strongSwan
     scenarios pass
8. **Phase: Compliance rows and documentation**
   - Tests: `make ze-rfc-check`, `make ze-doc-test`
   - Files: `rfc/short/rfc7296.md`, `docs/features/rfc-status.md`, `ai/RFC-REQUIREMENTS.md`,
     the documentation checklist rows
   - Verify: the four rows land in one commit at `-1` through `-4`, and the regenerated ledger
     is committed in the same commit

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at `file:line` |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness: acceptance cardinality | The response builder cannot emit two IPCOMP_SUPPORTED payloads, by construction and not by a later check |
| Correctness: proposed set | The accepted transform is drawn from the set Ze sent, never from the set the peer sent |
| Correctness: decline is omission | No code path answers an IPComp offer with an error notification, and no path fails the Child SA |
| Correctness: SA-payload-free messages | No builder for a liveness probe, a Delete, or any other informational message can append the payload |
| Mutation: `buildAuthResponse` emits an unsolicited acceptance | `RFC7296-2.22-1` and `-2` positives redden |
| Mutation: `respondChildRekey` emits an unsolicited acceptance | `RFC7296-2.22-1` and `-2` positives redden. Run this one explicitly. A harness that reaches only the IKE_AUTH producer leaves the CREATE_CHILD_SA producer unproven |
| Mutation: the informational builder emits the payload | `RFC7296-2.22-1` positive reddens. If it stays green, the sample holds no SA-payload-free message |
| Mutation: the acceptance selector reads the peer set instead of the local set | `RFC7296-2.22-2` positive reddens |
| Mutation: the response cap is lifted to two | `RFC7296-2.22-3` positive reddens |
| Mutation: `installChildSA` compresses with an unnegotiated algorithm | `RFC7296-2.22-4` positive reddens |
| Naming | YANG leaves are kebab-case with no abbreviations. The boolean is a positive `enabled`. JSON keys are kebab-case |
| Data flow | The CPI allocator is owned by the Child SA lifecycle, and no other component holds a reference |
| Rule: `ai/rules/protocol.md` | The VPP backend refuses at verify time, not at install time, and the error names the backend and the setting |
| Rule: `ai/rules/platform-linux.md` | The Linux-only backend code has a QEMU integration test and a make target |
| Rule: `ai/rules/rfc-compliance.md` | No answer narrower than full implementation with full proof was chosen. Thomas answered every such question |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| The four rows exist in the summary | `grep -c 'RFC7296-2\.22-' rfc/short/rfc7296.md` returns 4 |
| The four rows are proven | `make ze-rfc-check` passes and `ai/RFC-REQUIREMENTS.md` shows four bound rows |
| The ledger is fresh | `make ze-rfc-index` produces no diff |
| Compression is off by default | `test/ipsec/ipsec-ipcomp-disabled.ci` passes |
| Both backends are covered | `TestXFRMInstallsCompressionState` and `TestVPPBackendRefusesCompression` pass |
| The kernel path is proven | The QEMU target passes |
| Interoperability is proven | Scenarios `20-ipcomp` and `21-ipcomp-declined` pass |
| No interop tag was added | `grep -rn 'RFC requirement:' test/ipsec-interop/` returns nothing |
| The doctor check is reachable | `ze doctor --json` lists the check and `ze explain <code>` answers |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Input validation | A peer CPI of zero, a reserved CPI, an unknown transform identifier, and a notification body whose length is not 3 octets. Each must be refused with a named value |
| Resource exhaustion | A peer that offers many algorithms must not cause an unbounded allocation. Cap the offered set Ze parses |
| Decompression bomb | A compressed payload that expands without bound. Read RFC 3173 and bound the decompressed size |
| Fail closed | An acceptance Ze cannot map to a proposed algorithm must refuse the exchange. It must never reach an uncompressed install that the peer reads as compressed |
| Error leakage | A refusal message must name the transform identifier and the count, and must not print key material |
| Authorization | None. The feature carries no authorization decision |

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

- The four MUST NOT rows constrain the acceptance decision and the compression decision, and
  nothing else. They place no limit on what Ze proposes. A message proposing an SA carries as
  many IPCOMP_SUPPORTED notifications as the sender supports (`rfc/full/rfc7296.txt:3426-3428`).
- The compression association is not an object with its own lifetime. It disappears with the
  ESP or AH SA that contains it, and no Delete payload mentions it
  (`rfc/full/rfc7296.txt:3399-3403`). Model it as a field on `ChildSA`, not as a registry.
- A future reader will meet four MUST NOT rows and reach for a receive-side rejection. That is
  a violation of the status-notification ignore rule (`rfc/full/rfc7296.txt:5625-5628`), not a
  hardening. Every tag must say so.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Implement the feature, rather than prove non-participation with eight tests | Prove non-participation. The four rows are conformant today by non-participation, at a cost of roughly one day | Owner decision, 2026-07-31 |
| Place the compression container under `esp-group` | A per-peer container under `site-to-site peer` | IPComp is a Child SA property, and `esp-group` is the Child SA parameter group (`internal/component/ike/ipsec/yang/ze-ipsec-conf.yang`) |
| Compliance evidence is unit tier or functional tier, never interop tier | Tag the strongSwan scenario | `test/ipsec-interop/` is `TIER_UNRUN` and a tag there is refused (`scripts/dev/rfc_requirements.py`, `:952`) |

## Known Limitations

- The four rows land as one commit. If they cannot land atomically, the first one sets the
  section high-water mark and the rest must take higher ordinals, out of document order.
- The VPP backend cannot program a security association at all today
  (`plan/spec-fixit-vpp-ipsec-inoperable.md`). This spec refuses compression on that backend
  rather than implementing it, and the refusal is correct only while that spec stays open. Any
  work that makes VPP IPsec operable must revisit the refusal.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer
constraints, message ordering, and every MUST/MUST NOT.

Each of the four MUST NOT obligations needs a comment above the code that enforces it. The
four enforcement sites are the response builder cap, the proposed-set intersection, the
informational builders, and the dataplane compression call.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-verify` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: no live row without a destination

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)
