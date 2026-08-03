# Spec: ipsec-esp-dual-form-receive

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Depends | - (both owner decisions answered 2026-08-02, see "Owner decisions, 2026-08-02") |
| Phase | 4/5 |
| Deferral shard | `plan/deferrals/ipsec-esp-dual-form-receive.md` |
| Updated | 2026-08-02 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Let Ze receive both ESP forms on ONE established Child SA, at any time, as RFC 7296
Section 2.23 asks. Today Ze chooses one form per Child SA and programs the Linux XFRM
inbound state for that form alone. A peer that changes form on an established SA is not
served.

The goal has three parts. Find the route that lifts the limit. Cost each route with
evidence. Prove the chosen route against a real kernel.

**This spec is design-time. It selects a route before it writes code.** The four routes
below are candidates, not decisions. Phase 1 is evidence, and it is the only phase that
can run today.

### Ze is CONFORMANT-AS-BOUNDED today. This spec raises a platform limit

**This is not a record of an outstanding violation.** Both obligations are landed and
gated. `RFC7296-2.23-10` (`rfc/short/rfc7296.md`) and `RFC7296-2.23-11` each
carry a tagged positive and a tagged negative test in
`internal/component/ike/engine/rfc7296_natt_bothforms_test.go`. Neither is a `{gap}` and
neither is a `partial`.

Ze receives both forms ACROSS its Child SAs. `TestBfmBothESPFormsAreReachable`
asserts that both forms are actually programmed across the three combinations of NAT
verdict and port. Ze cannot receive both forms WITHIN one Child SA, and
`TestBfmOneFormPerChildSA` pins that boundary as a property the code has.

The limit is disclosed. `docs/features/rfc-status.md` states it in the Remaining
column and names the measurement behind it.

### What a peer must do to expose the limit

Three conditions must hold together.

1. No NAT is present. RFC 7296 requires UDP encapsulation from both devices when a NAT is
   detected (`rfc/full/rfc7296.txt:3550-3551`), so the alternating case needs a clear path.
2. The peer runs its IKE SA on port 4500. A peer can do this with no NAT, and RFC 7296
   permits it (`rfc/full/rfc7296.txt:3540-3542`).
3. The peer changes ESP form on an SA that is already established, rather than choosing
   one form at Child SA creation and keeping it.

Condition 3 is the one nobody has measured. Phase 1 measures it, and that measurement
decides whether this spec is urgent or theoretical.

### Provenance (do not delete)

Thomas ruled on 2026-08-01, in two steps, on the two held rows of
`plan/learned/1313-rfcgate-1b-rfc7296-pilot.md`. The open owner question was OR-WP8-4
(`plan/learned/1313-rfcgate-1b-rfc7296-pilot.md`).

**Step one.** Land both rows in the pilot with tags that record a measured platform limit,
rather than annotate either as a gap. That step is complete.

**Step two.** Create this spec, so the limit does not become permanent.

`ai/rules/rfc-compliance.md` reserves this decision for the owner, because a
classification that lowers what Ze owes is not the implementer's to make. Step two is why
a landed, gated, disclosed limit still gets a spec.

### The two obligations, quoted verbatim

| Row | Level | Text | Locator |
|-----|-------|------|---------|
| `RFC7296-2.23-10` | MUST | `If Network Address Translation Traversal (NAT-T) is supported (that is, if NAT_DETECTION_*_IP payloads were exchanged during IKE_SA_INIT), all devices MUST be able to receive and process both UDP-encapsulated ESP and non-UDP-encapsulated ESP packets at any time` | `rfc/full/rfc7296.txt:3544-3548` |
| `RFC7296-2.23-11` | MUST | `Implementations MUST process received UDP-encapsulated ESP packets even when no NAT was detected` | `rfc/full/rfc7296.txt:3624-3625` |

**The antecedent of `-2.23-10` is TRUE for Ze, so the MUST binds.** Ze emits
`NAT_DETECTION_SOURCE_IP` in its IKE_SA_INIT (`internal/component/ike/engine/initiator.go`)
and consumes the peer's (`responder.go`, `fsm.go`). NAT-T is supported, so the
conditional is satisfied and the obligation is live.

**Two neighbouring sentences bound the problem.** Either side decides on encapsulation
without regard to the other side's choice (`rfc/full/rfc7296.txt:3548-3550`). Both devices
must use encapsulation when a NAT is detected. The second sentence is why
the alternating case needs a NAT-free path.

The `(§2.23)` citation on each row is load-bearing. `parse_checklist_line`
(`scripts/dev/rfc_requirements.py`) validates that a row identifier's section segment
agrees with its citation.

### The four routes, and what each costs

Phase 1 answers all four. No route is chosen before then.

| Route | What it does | Cost | Blocking unknown |
|-------|--------------|------|------------------|
| A. Userspace demultiplex ahead of XFRM | Ze reads port 4500 without `UDP_ENCAP`, strips the 8-octet UDP header, and re-injects bare ESP. ONE template-free state then serves both forms | Medium. A new receive seam, because `Dataplane` models installation only | Does re-injected ESP reach the same state, and does XFRM see the outer addresses it needs? |
| B. VPP dataplane | VPP carries a per-SA UDP-encap flag and a UDP port pair | Large, and gated on another spec | Is VPP's inbound lookup encapsulation-aware? Not established in-repo |
| C. A newer kernel | An encapsulation-aware state lookup upstream | Small to answer. A-5 holds the answer | Does Linux 6.19.11 already behave this way? Ze ships that version |
| D. Establish that no peer does this | Measure real peer behaviour, then size the work by need | Small | Does strongSwan ever change form on an established SA? |

**Route E, accept the bound permanently, is rejected.** It is the current state, and step
two of the owner's ruling rejected permanence.

**Route A is the leading candidate, and the probe already supports half of it.**
`TestEncapKernelBindsOneESPFormPerState` measures that a template-free inbound state
ACCEPTS bare ESP (`encap_integration_linux_test.go`). If userspace can present every
inbound ESP datagram in bare form, one template-free state serves both wire forms. Ze
already reads port 4500 in userspace and discards every datagram that is not IKE
(`internal/component/ike/engine/register.go`, `:544-547`), so the read side exists.
That is a hypothesis with a named test, and it is not a finding.

**Route C is bounded by a fact, not by a wait.** The appliance ships Linux 6.19.11
(`gokrazy/modcache/github.com/rtr7/kernel@v0.0.0-20260403073601-5a996da3a37b/_build/upstream-url.txt`,
with `CONFIG_XFRM_USER=y` at `_build/config.addendum.txt`). The measurement was taken
against a real kernel. Waiting for a newer kernel is not a route, because Ze already runs
a current one. Answer route C by reading the 6.19.11 receive path, and record the answer.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/architecture.md` - both dataplanes
  → Constraint: a netlink-only capability creates drift. The VPP backend needs the
    capability or an explicit refusal in the same work.
- [ ] `ai/rules/platform-linux.md` - Linux-only code needs QEMU coverage
  → Constraint: the receive path is `//go:build linux`, so it needs a QEMU integration
    test. A virtual substitute is mandatory, and "needs hardware" is never an answer.
- [ ] `ai/rules/protocol.md` - backend translation honesty
  → Constraint: a backend that cannot receive the form the operator's peer sends must say
    so, and never approximate.
- [ ] `ai/rules/performance.md` - a userspace receive path is a hot path
  → Constraint: route A puts every encapsulated ESP datagram through userspace. The
    buffer comes from a pool, and the copy count is the design question.
- [ ] `ai/rules/completion.md` - the owner's step two
  → Constraint: a recorded limit is not an addressed limit. This spec exists to close one.
- [ ] `plan/spec-fixit-vpp-ipsec-inoperable.md` - the VPP backend's real state
  → Constraint: route B is gated behind this spec. Its AC-7 says nothing runs Ze's IPsec
    against a real VPP today.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc7296.md` - IKEv2. Rows `-2.23-10` and `-2.23-11`
  → Constraint: port 4500 is reserved for UDP-encapsulated ESP and IKE, and UDP
    encapsulation is forbidden on port 500 (`rfc/full/rfc7296.txt:3535`, `:3544`).
  → Constraint: sending encapsulated ESP is optional on port 4500, and understanding
    received encapsulated ESP is required (`rfc/full/rfc7296.txt:3542-3544`).
- [ ] `rfc/short/rfc3948.md` - UDP Encapsulation of IPsec ESP Packets. MUST CREATE if absent
  → Constraint: the 8-octet UDP header and the non-ESP marker rules live here, and route A
    depends on both.

**Key insights:** (minimal context to resume after compaction)
- Ze is conformant as bounded. The work raises a limit and fixes no violation.
- One Child SA carries one encapsulation boolean, and both directions read it.
- `Dataplane` models installation only. No receive seam exists.
- The VPP backend installs nothing at all, so route B has a prerequisite.

## Current Behavior (MANDATORY)

**Source files read:** (verified in the working tree on 2026-08-01)
- [ ] `internal/component/ike/dataplane/encap_integration_linux_test.go` -
  `TestEncapKernelBindsOneESPFormPerState`, the measurement. It installs TWO states on TWO
  different SPIs, one with an ESP-in-UDP template and one without. It then
  asserts a four-row truth table. A template-free state accepts bare ESP and
  refuses encapsulated ESP. A state with a template does the reverse. A fifth assertion
  (`:225`) is the discriminator: an SPI with no state raises a THIRD counter, so the four
  rows are real readings.
- [ ] `internal/component/ike/dataplane/encap_integration_linux_test.go` - the
  claim that two states on ONE SPI do not help is stated in the doc comment. **No test case
  installs two states on one SPI.** The file's own cases use two distinct SPIs. See A-1.
- [ ] `internal/component/ike/engine/child.go` - the encapsulation decision, one
  expression: `sa.NATDetected || sa.localPort == transport.NATTPort`. It is a DISJUNCTION.
  The port term is an added signal, and it did not replace the NAT verdict. See A-2.
- [ ] `internal/component/ike/engine/child.go` - the `MEASURED KERNEL CONSTRAINT`
  comment, which names the probe.
- [ ] `internal/component/ike/engine/child.go` and `:376-380` - `installChildSA`
  applies the one boolean to the inbound state and the outbound state together. Both use
  `transport.NATTPort` for both ports.
- [ ] `internal/component/ike/dataplane/xfrm_linux.go` - the only place a template is
  built. `state.Encap` stays nil when `p.UDPEncap` is false.
- [ ] `internal/component/ike/dataplane/dataplane.go` - `SAParams` carries
  `UDPEncap`, `UDPEncapSPort` and `UDPEncapDPort`.
- [ ] `internal/component/ike/dataplane/dataplane.go` - the `Dataplane` interface.
  Seven methods, all installation, removal or listing. **No receive method exists**, so a
  userspace receive path is a new seam and not a new backend.
- [ ] `internal/component/ike/transport/encap_linux.go` - `EnableESPInUDP` sets
  `UDP_ENCAP_ESPINUDP`. `internal/component/ike/transport/encap_other.go`
  refuses on every other platform.
- [ ] `internal/component/ike/engine/register.go` - the socket option is applied to the
  port-4500 socket only. Nothing sets it on the port-500 socket.
- [ ] `internal/component/ike/engine/register.go` - `dispatchNATTInbound` reads port
  4500 in userspace and drops every datagram that is not IKE.
- [ ] `internal/component/ike/engine/udpencap.go` - `checkIPsecUDPEncap` reads a cached
  atomic written at listener build time (`register.go`, `:407`). It never reads the
  live socket. `ESPInUDPEnabled` (`transport/encap_linux.go`) has no production caller.
- [ ] `internal/component/ike/dataplane/vpp.go` - the VPP backend hardcodes
  `UDPSrcPort: 0, UDPDstPort: 0` and never reads `p.UDPEncap`. Its hand-rolled
  `ipsecSAEntry` has NO flags field, so the VPP UDP-encap flag cannot be sent.
- [ ] `internal/component/ike/dataplane/vpp.go` - every message declares CRC
  `"00000000"` (also `:237`, `:257`, `:266`), so GoVPP refuses each one at identifier
  resolution and the backend installs nothing.
- [ ] `vendor/github.com/vishvananda/netlink/xfrm_state_linux.go` - `EncapType` has
  exactly two values, both meaning a template is present. **There is no wildcard value**,
  so a single state cannot declare that it accepts either form.

**Behavior to preserve:**
- Ze receives both ESP forms across its Child SAs. `TestBfmBothESPFormsAreReachable` must
  stay green.
- A NAT-traversing tunnel keeps its encapsulated inbound state. A tunnel with no NAT and an
  IKE SA on port 500 keeps its bare inbound state.
- The port-4500 socket keeps decapsulating unless route A replaces that mechanism whole.
- Both tagged RFC pairs keep their polarity. `ai/rules/testing.md` forbids a behavior change
  to a tagged test without the owner's written approval.

**Behavior to change:**
- One established Child SA accepts BOTH ESP forms, rather than one.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- An inbound ESP datagram arrives from the peer, in one of two wire forms.
- Bare ESP: IP protocol 50, the ESP header first, SPI in the first four octets.
- UDP-encapsulated ESP: IP protocol 17 to port 4500, an 8-octet UDP header, then the ESP
  header. A non-zero first four octets after the UDP header distinguish ESP from IKE.

### Transformation Path
1. The kernel routes the datagram by IP protocol. Protocol 50 goes straight to the XFRM
   receive path. Protocol 17 to port 4500 goes to the socket Ze holds.
2. The port-4500 socket carries `UDP_ENCAP_ESPINUDP` today
   (`transport/encap_linux.go`, applied at `engine/register.go`), so the kernel
   strips the UDP header and hands the ESP bytes to the XFRM receive path with an
   encapsulation type set.
3. XFRM looks the state up by destination, SPI, protocol and family. The lookup ignores the
   encapsulation form.
4. XFRM compares the datagram's encapsulation against the state's template. A disagreement
   raises `XfrmInStateMismatch` and drops the packet. This is step 4, and it is the step
   this spec must change or route around.
5. On agreement the packet reaches the crypto check, and the SA decrypts it.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Wire ↔ kernel | IP protocol 50, or UDP port 4500 | No |
| Kernel ↔ userspace | `UDP_ENCAP` socket option today. Route A replaces it with a userspace read | No |
| Engine ↔ dataplane | `dataplane.SAParams`, installation only (`dataplane.go`) | No |
| Engine ↔ transport | `transport.EnableESPInUDP` (`encap_linux.go`) | No |

### Integration Points
- `installChildSA` (`internal/component/ike/engine/child.go`) - the one producer of
  inbound state parameters.
- `createFirstChildSA` (`child.go`) - the one place the encapsulation boolean is
  decided, at `:249`.
- `rekey.go` - a rekey INHERITS the old encapsulation rather than recomputing it, so a
  route that changes the decision must revisit the rekey path.
- `dispatchNATTInbound` (`engine/register.go`) - the existing userspace reader on port
  4500, and route A's insertion point.

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
| A-1 | Two XFRM states on ONE SPI do not let both forms through | Asserted in prose at `encap_integration_linux_test.go` and `child.go`. **No test case installs two states on one SPI.** The probe uses two distinct SPIs | A cheap route exists that nobody costed, and route A is unnecessary | Extend the probe with a two-states-on-one-SPI case. Phase 1 | **confirmed** (2026-08-02), and the REASONED mechanism was wrong |
| A-2 | The encapsulation decision reads the NAT verdict as well as the port | `child.go` is `sa.NATDetected \|\| sa.localPort == transport.NATTPort`, a disjunction. The comment at `child.go` says the port replaces the verdict, which OVERSTATES the code | Prose in the pilot spec, the rfc-status row and this spec describe code that does not exist | Read `child.go`. The spec author did this on 2026-08-01 | **confirmed** (2026-08-01) |
| A-3 | Re-injected bare ESP reaches the same XFRM state as a natively bare datagram | Not established. The probe sends bare ESP from a raw socket (`encap_integration_linux_test.go`) and it is accepted, but it never re-injects a stripped datagram | Route A fails and the leading candidate is gone | Extend the probe: strip a UDP-encapsulated datagram in userspace, re-inject, assert the template-free state accepts it. Phase 1 | **confirmed** (2026-08-02) |
| A-4 | A real peer changes ESP form on an established SA | Not established. RFC 7296 permits it (`rfc/full/rfc7296.txt:3548-3550`). No interop scenario exercises it, and no scenario sets any encapsulation option | The work is theoretical, and its priority drops. It does NOT become unnecessary, because the MUST binds regardless | Drive strongSwan through a form change on one SA. Phase 1 | **confirmed**, and it is NOT the forcing case |
| A-7 | One netns probe per test BINARY is all the harness supports | Discovered 2026-08-02. Every `encapNetns` test passes alone. Run together, the FIRST passes and every later one reads a namespace where its packets never reach XFRM, so no counter moves | The QEMU integration target goes red whenever the package holds more than one namespace probe, and the failure reads as a product defect rather than a harness one | Run the pre-existing probe with `-count=2` in one process. Phase 1 | see Phase 1 Evidence |
| A-5 | Linux 6.19.11 has no encapsulation-aware state lookup | Not established. The appliance ships 6.19.11 (`_build/upstream-url.txt`) and the probe measures a mismatch. The probe ran on the QEMU Alpine kernel, and not on the appliance kernel | Route C is the cheapest answer and the other three are unnecessary | Read the 6.19.11 XFRM receive path, then rerun the probe on the appliance kernel. Phase 1 | **confirmed** (2026-08-02) |
| A-6 | VPP's inbound SA lookup is encapsulation-aware | Not established in-repo. VPP carries `IPSEC_API_SAD_FLAG_UDP_ENCAP = 16` and a per-entry UDP port pair (`gokrazy/modcache/go.fd.io/govpp@v0.13.0/binapi/ipsec_types/ipsec_types.ba.go`), which says the flag exists and not what the lookup does | Route B cannot deliver the capability either, and the drift argument changes | Read VPP's own C source for the ESP receive node. Phase 1 | **confirmed** (2026-08-02), by the opposite mechanism to the one the wording expects |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Route A puts every encapsulated ESP datagram through userspace, and throughput falls | A benchmark against the current kernel decapsulation path | Measure this before you commit to route A. A form change is rare, so a hybrid that stays in the kernel until a mismatch is observed is the fallback |
| R-2 | Route A loses the anti-replay and address checks the kernel performs on the encapsulated form | A conformance read of RFC 3948 against the re-injection path | Keep XFRM as the decryptor. Route A changes the presentation of the datagram and never the cryptography |
| R-3 | The two tagged RFC pairs must change when the limit lifts, and `c_rfc_tagged_test` blocks that edit | The hook refuses the edit | The owner authorizes the change in writing, per `ai/rules/testing.md`. Ask before you edit, never after |
| R-4 | Route B is chosen and inherits a backend that installs nothing | `plan/spec-fixit-vpp-ipsec-inoperable.md` AC-7 is still open | Treat route B as blocked until that spec closes. Record the dependency in Depends |
| R-5 | A-4 comes back negative and the work looks unnecessary | The interop experiment finds no peer that alternates | The MUST binds regardless of peer behavior. A negative A-4 changes PRIORITY, never the obligation. Record it and keep the spec open |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Every IPsec tunnel's receive path. A wrong route silently drops ESP, which is the quietest failure this subsystem has: the tunnel establishes and carries no traffic |
| How is it reverted? | A single commit revert, as long as the change stays inside the receive presentation and no wire format changes. Route A is revertible. A change to the encapsulation DECISION is visible to the peer and is not |
| Who else touches this path? | `plan/learned/1313-rfcgate-1b-rfc7296-pilot.md` owns the two tagged rows. `plan/spec-fixit-vpp-ipsec-inoperable.md` owns the VPP backend. Both must be read before any edit |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A UDP-encapsulated ESP datagram arrives on port 4500 for an SA programmed bare | → | the receive path chosen in phase 1 | `TestEncapOneStateAcceptsBothForms` |
| A bare ESP datagram arrives for an SA whose peer last sent the encapsulated form | → | the same receive path | `TestEncapOneStateAcceptsBothForms` |
| An operator brings up a tunnel and the peer changes ESP form mid-session | → | `installChildSA` and the receive path together | `ipsec-esp-form-change.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | One inbound state for one SPI, and a bare ESP datagram | The datagram reaches the crypto check |
| AC-2 | The SAME inbound state for the SAME SPI, and a UDP-encapsulated ESP datagram | The datagram reaches the crypto check. No `XfrmInStateMismatch` is raised |
| AC-3 | An SPI with no state at all | `XfrmInNoStates` is raised, so AC-1 and AC-2 are real readings and not a counter that never moves |
| AC-4 | An established Child SA, and the peer changes ESP form | Traffic keeps flowing in both directions. The SA is not rekeyed and not deleted |
| AC-5 | The VPP dataplane is selected and cannot receive both forms | `ze config verify` fails with an error naming the backend and the unsupported capability, per `ai/rules/protocol.md` |
| AC-6 | Phase 1 completes | Every assumption A-1 to A-6 is `confirmed` or `broken`, each with a named command or test |
| AC-7 | The route is chosen | The Key Design Decisions table records the chosen route and each rejected route, with its measured cost |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Brings up a tunnel to a peer that sends encapsulated ESP with no NAT | IKE_AUTH → `createFirstChildSA` → `installChildSA` → receive path → decrypt | `ipsec-esp-encap-no-nat.ci` |
| 2 | Keeps that tunnel while the peer changes to bare ESP | established SA → receive path → decrypt | `ipsec-esp-form-change.ci` |
| 3 | Reads which ESP forms the SA accepts | `show vpn ipsec sa` → SA state | `ipsec-show-sa-esp-form.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestChildSAAcceptsBothESPForms` | `internal/component/ike/engine/child_test.go` | An installed Child SA declares both forms permitted | |
| `TestRekeyInheritsBothFormAcceptance` | `internal/component/ike/engine/rekey_test.go` | A rekey does not narrow the SA back to one form (`rekey.go`) | |
| `TestVPPRejectsDualFormWhenUnsupported` | `internal/component/ike/dataplane/vpp_test.go` | The VPP backend fails closed rather than reporting success | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| UDP encapsulation port | 1-65535 | 65535 | 0 | 65536 |
| ESP SPI | 256-4294967295 | 4294967295 | 255 (reserved) | N/A |
| Stripped UDP header length | exactly 8 octets | 8 | 7 | 9 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ipsec-esp-encap-no-nat` | `test/ipsec/ipsec-esp-encap-no-nat.ci` | A peer sends encapsulated ESP with no NAT, and traffic flows | |
| `ipsec-esp-form-change` | `test/ipsec/ipsec-esp-form-change.ci` | The peer changes form on an established SA, and traffic keeps flowing | |
| `ipsec-esp-form-vpp-reject` | `test/ipsec/ipsec-esp-form-vpp-reject.ci` | An operator selects VPP and gets a clear refusal | |

The `ipsec` suite runs inside `ze-verify` (`mk/test-functional.mk` and `:217`), so a
`.ci` there earns a verify tier.

### QEMU Integration Tests

**Mandatory.** The receive path is `//go:build linux`, and `ai/rules/platform-linux.md` makes
a QEMU test blocking for such code.

| Test | Package | Validates | Status |
|------|---------|-----------|--------|
| `TestEncapOneStateAcceptsBothForms` | `internal/component/ike/dataplane/encap_integration_linux_test.go` | ONE state, ONE SPI, both forms accepted | |
| `TestEncapTwoStatesOneSPI` | same file | A-1: whether two states on one SPI change the verdict | |
| `TestEncapReinjectedBareESPAccepted` | same file | A-3: a stripped and re-injected datagram reaches the template-free state | |

**What the dual-form test must assert, precisely.** It reuses the existing harness and
flips exactly one row of the truth table.

1. Install ONE inbound state for ONE SPI, in a fresh network namespace (`encapNetns`,
   `encap_integration_linux_test.go`).
2. Send a bare ESP datagram for that SPI. Assert the kernel raises
   `XfrmInStateProtoError`, which means the state matched and the payload reached the
   crypto check.
3. Send a UDP-encapsulated ESP datagram for the SAME SPI. Assert `XfrmInStateProtoError`
   again. Today this row raises `XfrmInStateMismatch`, and that is the single assertion
   that must flip.
4. Send a datagram for an SPI with no state, on the same goroutine and the same namespace.
   Assert `XfrmInNoStates`. Without this control the two rows above prove nothing.
5. Keep one goroutine and no subtests. `runtime.LockOSThread` binds the namespace to the
   thread, and a subtest gets a different thread that reads the HOST namespace
   (`encap_integration_linux_test.go`).

**The probe's package is already selected automatically.** `ZE_QEMU_INTEGRATION_PKGS`
(`mk/test-integration.mk`) greps for the `integration && linux` build tag, so the file
needs no Makefile edit. **Nothing runs that target automatically**
(`ai/rules/platform-linux.md`), so phase 1 must run it by hand and paste the output.

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `22-esp-encap-no-nat` | `test/ipsec-interop/scenarios/` | strongSwan | A strongSwan peer forced to encapsulate with no NAT exchanges traffic with Ze | |
| `23-esp-form-change` | `test/ipsec-interop/scenarios/` | strongSwan | A strongSwan peer that changes form on one SA keeps traffic flowing. This scenario ALSO answers A-4 | |

**These scenarios cannot carry an RFC tag.** `test/ipsec-interop/` is declared `TIER_UNRUN`
(`scripts/dev/rfc_requirements.py`), so a tag there is refused. Compliance evidence must be
unit tier or functional tier. Write that reason into each scenario header.

The lab runs strongSwan from Alpine 3.21 (`test/ipsec-interop/Dockerfile.strongswan:5`).
No existing scenario sets any encapsulation option, and no scenario directory matches
`*nat*`. Both scenarios are new.

## Files to Modify
- `internal/component/ike/engine/child.go` - the encapsulation decision and the
  inbound and outbound programming
- `internal/component/ike/engine/register.go` - the port-4500 reader and the
  socket option site, for route A
- `internal/component/ike/dataplane/dataplane.go` - the `Dataplane` seam, if
  the chosen route needs a receive method
- `internal/component/ike/dataplane/xfrm_linux.go` - the template decision
- `internal/component/ike/transport/encap_linux.go` - `EnableESPInUDP`, if route A
  stops using the socket option
- `internal/component/ike/engine/rekey.go` - the inherited encapsulation
- `docs/features/rfc-status.md` - RFC 7296, its Remaining column, when the limit
  lifts

## Files to Create
- `test/ipsec/ipsec-esp-form-change.ci` - the end-user proof of a form change
- `test/ipsec-interop/scenarios/23-esp-form-change/` - the strongSwan proof
- `rfc/short/rfc3948.md` - the UDP encapsulation summary, if absent

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | | `internal/component/<name>/yang/` or the owning plugin's `yang/`. Read `ai/rules/config.md` (YANG vs env var) and `ai/rules/config.md` (naming) |
| YANG validation constraints | | Every leaf takes maximum native validation: `range`, `length`, `pattern`, `enumeration`, `type` from `ze-types.yang`. See `ai/patterns/config-option.md` |
| YANG custom validators | | Where native constraints are insufficient: `ze:validate` + `ValidateFn` + `CompleteFn` for completion |
| CLI commands/flags | | `cmd/ze/*/main.go` or subcommand files |
| CLI grammar (keyword before value) | | `ai/rules/cli.md` |
| Editor autocomplete | | Automatic for YANG enum/type leaves. Dynamic values need `CompleteFn` |
| Functional test for new RPC/API | | `test/plugin/*.ci` or `test/decode/*.ci` |
| Pipe completeness | | Route output through `ApplyPipes`/`ProcessPipes` per `ai/rules/cli.md` |
| Env var registration | | YANG leaves under `environment/` need a matching `ze.<name>.<leaf>` via `env.MustRegister()` |
| Doctor check for runtime dependencies | Yes | `doctor-ipsec-udp-encap` (`internal/component/ike/engine/udpencap.go`) reports a cached setsockopt outcome. A route that stops using the socket option makes that check report on a mechanism Ze no longer uses. Revisit the check and its diagnostic code |
| Prometheus counters/metrics | | Observable state: define, register, and list the metric names and labels here |
| BGP family surface (new SAFI / capability / attribute) | N-A | This is IPsec, not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | | `docs/features.md` |
| 2 | Config syntax changed? | | `docs/guide/configuration.md`, `docs/architecture/config/syntax.md` |
| 3 | CLI command added/changed? | | `docs/guide/command-reference.md` |
| 4 | API/RPC added/changed? | | `docs/architecture/api/commands.md` |
| 5 | Plugin added/changed? | | `docs/guide/plugins.md` |
| 6 | Has a user guide page? | | `docs/guide/<topic>.md` |
| 7 | Wire format changed? | | `docs/architecture/wire/*.md` |
| 8 | Plugin SDK/protocol changed? | | `ai/rules/plugins.md`, `docs/architecture/api/process-protocol.md` |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc7296.md` rows `:509` and `:510`, and the `docs/features/rfc-status.md` Remaining column, which today discloses the limit this spec lifts |
| 10 | Test infrastructure changed? | | `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | | `docs/comparison.md` |
| 12 | Internal architecture changed? | | `docs/architecture/core-design.md` or subsystem doc |
| 13 | Route metadata keys added/changed? | | `docs/architecture/meta/README.md`, `docs/architecture/meta/<plugin>.md` |
| 14 | Prometheus counters added/changed? | | `docs/plugin-development/metrics.md` or subsystem telemetry doc |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | | `docs/plugin-overview.md`, `docs/features/plugins.md`, `docs/guide/status.md` |
| 16 | Any changed source file referenced by existing doc source anchors? | | Grep `docs/` for `source: <changed-file>` and update each stale claim |
| 17 | Existing docs show config/CLI/API examples for this area? | | Verify examples against YANG/parser/handler and update stale syntax |

## Implementation Steps

**Phase 1 runs first, and that order is deliberate.** The template puts the wiring phase
first, because a wiring target usually exists at design time. Here it does not. The entry
point depends on which route is chosen, and no route is chosen. Phase 1 produces that
choice. Phase 2 is the wiring phase the template asks for.

1. **Phase 1: Evidence (no feature code)** -- answer every assumption
   - Tests: `TestEncapTwoStatesOneSPI` (A-1), `TestEncapReinjectedBareESPAccepted` (A-3)
   - Files: `internal/component/ike/dataplane/encap_integration_linux_test.go` only
   - Also: read the Linux 6.19.11 XFRM receive path (A-5). Read VPP's ESP receive node
     (A-6). Drive strongSwan through a form change (A-4)
   - Verify: A-1 to A-6 are each `confirmed` or `broken`, with pasted output. Present the
     costed routes to the owner and record the chosen route in Key Design Decisions
2. **Phase 2: Wiring (MANDATORY FIRST once the route is chosen)** -- register the entry
   point and write the failing wiring test
   - Tests: `TestEncapOneStateAcceptsBothForms`
   - Files: the seam the chosen route needs
   - Verify: the entry point exists and is reachable. The wiring test fails because the
     receive path is a stub
3. **Phase 3: The receive path** -- implement the chosen route
   - Tests: `TestChildSAAcceptsBothESPForms`, `TestEncapOneStateAcceptsBothForms`
   - Files: from Files to Modify
   - Verify: tests fail → implement → tests pass
4. **Phase 4: Both dataplanes and both directions** -- the rekey path and the VPP refusal
   - Tests: `TestRekeyInheritsBothFormAcceptance`, `TestVPPRejectsDualFormWhenUnsupported`
   - Files: `rekey.go`, `vpp.go`
   - Verify: a VPP build refuses at config verify rather than reporting success
5. **Phase 5: End to end** -- the `.ci` tests, the interop scenarios, the documentation
   - Tests: `ipsec-esp-form-change.ci`, scenario `23-esp-form-change`
   - Files: `test/ipsec/`, `test/ipsec-interop/scenarios/`, `docs/features/rfc-status.md`
   - Verify: the owner authorizes the tagged-test edits in writing BEFORE they are made

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | The dual-form QEMU test raises `XfrmInStateProtoError` on BOTH rows, and the no-state control still raises `XfrmInNoStates`. Without the control the test proves nothing |
| Correctness | A form change does not rekey, does not delete, and does not reset the replay window |
| Naming | The new seam is named for what it does, and not for the kernel mechanism behind it (`ai/rules/go-standards.md`) |
| Data flow | The cryptography stays in XFRM. A route that moves decryption into userspace is a different spec |
| Rule: `ai/rules/testing.md` | The two tagged RFC pairs are edited only with the owner's written approval, recorded in the spec |
| Rule: `ai/rules/protocol.md` | A backend that cannot receive both forms refuses at config verify, and never reports success |
| Rule: `ai/rules/platform-linux.md` | The QEMU test was RUN, and its output is pasted. The target is not automated |
| Rule: `ai/rules/evidence.md` | Every claim about kernel or VPP behavior cites source that was read, not a comment that asserts it. A-1 exists because a comment asserted a measurement that no test made |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Every assumption answered | Read the Assumptions table. No row is `unvalidated` |
| The dual-form QEMU test exists and passes | `make ze-qemu-integration-test`, output pasted |
| The route decision is recorded | Key Design Decisions names the chosen route and each rejected route |
| The `.ci` tests exist | `ls test/ipsec/ipsec-esp-form-change.ci` |
| The disclosure is updated | `grep -n "form change" docs/features/rfc-status.md` |
| The tagged tests still gate | `make ze-rfc-check` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | A userspace path that strips a UDP header must bound the length before it slices. An 8-octet header on a 7-octet datagram is an attacker-supplied read |
| Anti-replay | The kernel owns the replay window. A route that presents datagrams differently must not let a replayed packet take a second path into the same state |
| Resource exhaustion | An unauthenticated datagram on port 4500 must not allocate per packet. The buffer comes from a pool (`ai/rules/performance.md`) |
| Spoofing | Accepting both forms on one state widens what an off-path attacker can inject. The crypto check still rejects it, and the cost of reaching that check must stay bounded |
| Fail closed | A backend that cannot express dual-form reception refuses, and never installs a state that silently drops (`ai/rules/evidence.md`) |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| Every route is measured impossible | STOP. Report the four measurements to the owner. Do NOT write a `{gap}`, because writing one IS the decision (`ai/rules/rfc-compliance.md`) |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Phase 1 Evidence (2026-08-02)

Every route is answered. Nothing below is argued from a comment.

### The forcing case, measured

`tmp/dualform/baseline-07.log`. Scenario `07-responder-psk` is RED on
`strongSwan accepted no ESP from Ze the responder`. In the same log strongSwan sends
IKE_SA_INIT from `172.28.0.3[500]` with `N(NATD_S_IP) N(NATD_D_IP)`, finds no NAT, and
then sends IKE_AUTH from `172.28.0.3[4500]`.

strongSwan's own source says why, and it is not a misconfiguration.
`ike_natd.c` `process_i` floats the ports when
`has_condition(COND_NAT_ANY) || (peer_cfg->use_mobike(peer_cfg) && supports_extension(EXT_NATT))`,
commented "if the peer supports NAT-T, we switch to port 4500 even if no NAT is
detected". `mobike` defaults to `yes`. `COND_NAT_ANY` stays FALSE, so the ESP form
stays bare (`child_create.c:1776`).

**Ze's root cause is the second term of `child.go`,
`sa.NATDetected || sa.localPort == transport.NATTPort`.** A port float is a MOBIKE
artifact. Ze reads it as an encapsulation signal and encapsulates in BOTH directions,
because `installChildSA` applies that one boolean to the inbound and the outbound state
together.

### Route verdicts

| Route | Verdict | Evidence |
|-------|---------|----------|
| A. Userspace demultiplex | **VIABLE, and the only one buildable today** | `TestEncapReinjectedBareESPAccepted` re-injected a userspace-read datagram with the peer's source preserved and the template-free state took it: `XfrmInStateProtoError`. The kernel already hands those datagrams to userspace when the socket carries no encap type (`net/ipv4/xfrm4_input.c:91-94`, `if (!encap_type) return 1;`) |
| B. VPP | **Capable, and blocked** | VPP's inbound lookup is encapsulation-BLIND, keyed on peer address and SPI (`ipsec_tun_in.c` `ipsec_tun_protect_input_inline`, which picks the ESP offset per packet from the wire and then keys `ipsec4_tunnel_mk_key`). `ipsec_tun.c` `ipsec_tun_register_nodes` points proto-50 AND UDP/4500 at the same node unconditionally. So ONE VPP SA takes both forms. Ze cannot use it: `plan/spec-fixit-vpp-ipsec-inoperable.md` establishes the backend installs no SA at all |
| C. A newer kernel | **CLOSED** | Linux 6.19.11 `net/xfrm/xfrm_input.c:634` is `if ((x->encap ? x->encap->encap_type : 0) != encap_type)`. It is SYMMETRIC, so a template-free state demands `encap_type == 0`. Byte-identical at v6.6 and v6.12. No wildcard encap type exists, and `xfrm_state.c:2233-2237` REFUSES to add, remove, or change an encap template on a live state (`-EINVAL`) |
| D. No peer does this | **REJECTED, and it answers a different question** | strongSwan DOES change form on a live SA, through `kernel_netlink_ipsec.c` `update_sa`, which deletes and re-adds the state and restores the replay counter by hand. It is driven by MOBIKE (`ike_mobike.c:543-545`). But the measured red is an ESTABLISHMENT-time disagreement, not a mid-SA change |

### The kernel limit, now measured rather than reasoned

`TestEncapTwoStatesOneSPI` (QEMU, Alpine aarch64): a second state on one SPI is refused
with `file exists` BOTH with identical addresses AND with a differing source. The prose in
`child.go` and in the sibling probe said the lookup "returns the first match and the
mismatch check then drops the packet". That mechanism is wrong. The second state cannot be
installed at all, because the uniqueness key and the lookup key are the same tuple.

### A-7, a harness defect this work exposed

The QEMU package supports exactly ONE namespace probe per test BINARY. Run alone, all
three probes pass. Run together, the first passes and every later one reports that no
counter moved. It is not caused by the new probes: the PRE-EXISTING
`TestEncapKernelBindsOneESPFormPerState` under `-count=2` passes its first iteration and
fails its second, with no new code involved. `make ze-qemu-integration-test` runs the whole
package, so this must be fixed or the target reads red for a product reason that does not
exist.

### What Phase 2 is blocked on

Phase 1 is complete and the route is chosen. Implementation cannot start, because both
changes the route needs alter the behaviour of TAGGED RFC tests, and
`ai/rules/testing.md` gives that authorization to the owner alone. `// test-relax:` does
NOT satisfy `c_rfc_tagged_test`; only
`// rfc-test-change-approved: <date> <what the user approved>` does, and only the user may
write it. This spec's own Goal Gates already require the authorization in writing BEFORE
the edit.

**Decision 1: authorize the tagged-test edits.** Both live in
`internal/component/ike/engine/rfc7296_natt_bothforms_test.go`.

| Test | Tag | Why the chosen route changes it |
|------|-----|--------------------------------|
| `TestBfmEncapsulatedESPAcceptedWithoutNAT` | `RFC7296-2.23-11 positive` | It asserts EVERY SA carries `UDPEncap` when the SA floated with no NAT. That is exactly the case where ze must now SEND bare, so the outbound half of the assertion has to become a receive-side assertion |
| `TestBfmOneFormPerChildSA` | `RFC7296-2.23-10 negative` | It asserts `inbound.UDPEncap == outbound.UDPEncap`, which is the limit this spec lifts. It becomes the assertion that the SA accepts BOTH forms inbound while sending one |

Neither row is deleted and neither requirement loses a polarity. Both get STRONGER: the
positive stops asserting a template and starts asserting that both forms are received, and
the negative stops pinning a limit and starts pinning the dual-form property.

**Decision 2: how far dual-form reception goes under NAT.** Route A costs a userspace hop
for every encapsulated ESP datagram, because the kernel only hands those to userspace when
the socket carries no encap type.

| Option | Compliance | Cost |
|--------|-----------|------|
| Inbound state always template-free | Both forms at any time, unconditionally | Under NAT every ESP packet crosses userspace, and NAT is the common deployment |
| Inbound template follows the NAT verdict, userspace covers the other form | Both forms whenever no NAT is detected, which is `-2.23-11` exactly | Kernel fast path everywhere. Under NAT the bare form is not received, and RFC 7296 (`rfc/full/rfc7296.txt:3550-3551`) requires both devices to encapsulate there |

`ai/rules/rfc-compliance.md` reserves this for Thomas, because the second option is
narrower than full compliance even though the RFC arguably excludes the case it drops.

## Owner decisions, 2026-08-02

Both blocking questions are answered, and one of them is answered by measurement rather
than by choice.

**Decision 1, the tagged-test edits: AUTHORIZED.** Both tests carry
`// rfc-test-change-approved: 2026-08-02` and both now assert MORE than before. Neither row
is deleted and neither polarity is lost.

**Decision 2, how far dual-form reception goes under NAT: RECEIVE BOTH FORMS ALWAYS.** The
governing text is `rfc/full/rfc7296.txt:3545-3551`. The condition is NAT-T being SUPPORTED,
not a NAT being FOUND, and the words are "at any time". So reception accepts both forms
whenever NAT-T is supported, and only TRANSMISSION follows the NAT verdict.

## Phase 2 Evidence (2026-08-02)

### The per-SA hybrid is NOT buildable, and that is measured

The intended design was the kernel fast path for the EXPECTED form per SA, with userspace
catching the unexpected one. Its two halves ask ONE per-socket option for opposite
settings, and Ze holds ONE port-4500 socket for every SA.

| Measurement | Result | What it decides |
|-------------|--------|-----------------|
| M1 `TestEncapBareESPVisibleToUserspaceWhenStateIsTemplated` | `XfrmInStateMismatch`, and a raw `IPPROTO_ESP` reader saw the datagram: **true** | The under-NAT half WORKS. `ip_protocol_deliver_rcu` calls `raw_local_deliver` before the protocol handler, so userspace sees what XFRM refused |
| M2 `TestEncapEncapsulatedESPHiddenFromUserspaceWhenSocketDecapsulates` | `XfrmInStateMismatch`, userspace read: **false** (`i/o timeout`) | The no-NAT half FAILS while `UDP_ENCAP` is set. The kernel consumes the datagram before any reader sees it |
| A-3 `TestEncapReinjectedBareESPAccepted` (phase 1) | `XfrmInStateProtoError` | The same half WORKS when `UDP_ENCAP` is clear |

M2 and A-3 disagree only on the socket option, so serving both halves per SA would need
`UDP_ENCAP` on and off at once. It is one option on one socket. The choice is therefore
GLOBAL, and only which form the kernel serves is per SA.

### The design as built

| Decision | Value | Why |
|----------|-------|-----|
| `UDP_ENCAP` on the port-4500 socket | stays SET | Keeps the kernel fast path for the encapsulated form, which is every packet of the common NAT deployment |
| Inbound XFRM template | `NATDetected \|\| floated`, unchanged | The kernel serves the form the peer most likely sends, and `-2.23-11` needs a template on a floated no-NAT SA |
| Second inbound form | served beside the kernel | A raw `IPPROTO_ESP` reader takes the refused bare datagram and re-presents it through port 4500, where the kernel strips it and hands XFRM the type the template wants |
| Outbound encapsulation | `NATDetected` ONLY | RFC 7296 makes encapsulation mandatory under NAT and free otherwise. The port float is a MOBIKE artifact, not an encapsulation signal |
| Cost containment | the reader runs only while a TEMPLATED state exists | A deployment whose SAs are all template-free holds no raw socket, so the bare fast path pays nothing |

`TestEncapOneStateAcceptsBothForms` proves the whole path against a real kernel: ONE state,
ONE SPI, encapsulated on the kernel fast path and bare re-presented, both reaching the
crypto check, with the no-state control still raising `XfrmInNoStates`.

### The one cell that stays unreachable, and why no conforming peer reaches it

A template-free inbound state cannot receive the encapsulated form (M2). No conforming peer
produces that combination. RFC 3948 Section 2.1 requires the ESP-in-UDP ports to equal the
IKE ports, and RFC 7296 Section 2.23 forbids encapsulation on port 500
(`rfc/full/rfc7296.txt:3543`). A peer that encapsulates ESP therefore runs its IKE on port
4500, and every SA on port 4500 is templated here. This is an argument from two RFCs, not a
measurement, and it is recorded as such.

### Mutation evidence

`TestEncapOneStateAcceptsBothForms` gates. Mutating the re-presented UDP destination port
to 4501 (`go -overlay`, tree untouched) turns it RED with "expected exactly one counter to
move, got []". Mutating the source address to the local endpoint left it GREEN, which is a
real finding: `__xfrm_state_lookup` keys on destination, SPI, protocol and family, so the
SOURCE is not part of the match. The comment in `espform.go` was corrected to claim only
what the probe proves.

### A defect this work introduced, found by `-count=2` and fixed

`(*espFormReceiver).Forget` panicked on a nil receiver. `xfrmBackend` is built as a bare
`&xfrmBackend{}` literal in ten places, which leaves the receiver nil, and `RemoveSA` and
`Close` reach it on a production path. The methods now define the nil case: `Watch` FAILS
CLOSED with `errNoESPFormReceiver`, and the teardown methods are no-ops.
`TestESPFormReceiverNilIsFailClosed` pins it. `InstallSA` also removes the state it just
added when the receiver refuses, rather than stranding one.

## Design Insights

- **A comment that asserts a measurement is not a measurement.** `child.go` and
  `encap_integration_linux_test.go` both state that two states on one SPI do not
  help. No test case installs two states on one SPI. A-1 exists because of that gap, and
  `ai/rules/evidence.md` predicts exactly this failure.
- **The prose about the encapsulation decision overstates the code.** `child.go` reads
  the NAT verdict AND the port. The comment at `child.go` says the port replaced
  the verdict. Both readings give the same answer today, so nothing is broken, and the
  description is still wrong.
- **`Dataplane` has no receive method.** Every route that changes reception adds a seam
  rather than a backend. That is the single largest cost driver, and it is why route A is
  medium rather than small.
- **Ze already reads port 4500 in userspace.** `dispatchNATTInbound` exists and discards
  non-IKE datagrams. Route A extends a path Ze already owns.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| **Route A is chosen for the receive path** (2026-08-02) | B, C, D, and route E | C is closed by the kernel: the encapsulation check is symmetric, there is no wildcard type, and a live state's template cannot be changed. B is capable but its backend installs nothing and is gated on another spec. D answers a different question, because strongSwan's own float is a MOBIKE artifact and the measured red is at establishment. A is the only route buildable today, and A-3 measured its one unproven mechanism working: a userspace-read datagram re-injected with the peer's source preserved reached the template-free state |
| **The forcing case needs a SECOND change beside route A** (2026-08-02) | Treat this spec as receive-only | The measured red is ze's SEND form. `child.go` treats a port float as an encapsulation signal, so ze encapsulates toward a peer that expects bare. Route A alone fixes what ze ACCEPTS and leaves what ze SENDS wrong, so scenario 07 would stay red. The outbound form must follow the NAT verdict rather than the port |
| Phase 1 is evidence, and it precedes wiring | Start with route A immediately | Four routes are open and none is measured. Committing to a seam before A-1, A-3 and A-5 are answered risks building a seam that a cheaper route makes unnecessary |
| The spec exists even though Ze is conformant as bounded | Close the question with the landed rows | The owner's step two. A landed limit that nobody owns becomes permanent, and `ai/rules/completion.md` names that failure |

## Acceptance Criteria status (2026-08-02)

| AC | Status | Evidence |
|----|--------|----------|
| AC-1 | met | `TestEncapOneStateAcceptsBothForms`, row 3: the re-presented bare datagram raises `XfrmInStateProtoError` |
| AC-2 | met | same test, row 1: the encapsulated form raises `XfrmInStateProtoError` on the SAME state and SPI |
| AC-3 | met | same test, final control: an SPI with no state raises `XfrmInNoStates` |
| AC-4 | **not proven end to end** | No `.ci` drives a form change on an established SA. Scenario `07-responder-psk` proves the establishment-time disagreement is fixed, which is the measured red. The mid-SA change is untested |
| AC-5 | not applicable, by measurement | VPP CAN receive both forms on one SA, so there is nothing to refuse. Recorded in `vpp.go` with the VPP source read. Its backend installs no SA at all, which `plan/spec-fixit-vpp-ipsec-inoperable.md` owns |
| AC-6 | met | A-1 to A-6 are `confirmed`, each with a named test or source read |
| AC-7 | met | Key Design Decisions records route A and every rejected route with its measured cost |

## What is NOT done

Stated plainly rather than left to be discovered (`ai/rules/completion.md`).

| Item | State |
|------|-------|
| `test/ipsec/ipsec-esp-form-change.ci` | NOT written. AC-4 has no functional proof |
| `test/ipsec-interop/scenarios/23-esp-form-change/` | NOT written. A strongSwan-driven mid-SA form change is unproven |
| `test/ipsec/ipsec-esp-encap-no-nat.ci`, `ipsec-esp-form-vpp-reject.ci` | NOT written |
| `ai/RFC-REQUIREMENTS.md` regeneration | NOT run. `make ze-rfc-index` is REQUIRED after moving or renaming a tagged test, and one test was renamed. The main thread directed that the ledger not be regenerated while it is stale from another session |
| Throughput of the re-presented form | Not measured. R-1 stands for the bare form on a templated SA |

## Known Limitations
- Route B cannot be attempted until `plan/spec-fixit-vpp-ipsec-inoperable.md` closes. That
  spec is `ready`, and its AC-7 states that nothing runs Ze's IPsec against a real VPP.
- The interop scenarios cannot carry an RFC tag, because `test/ipsec-interop/` is
  `TIER_UNRUN`. Compliance evidence stays at unit tier or functional tier.
- `make ze-qemu-integration-test` is not automated anywhere. Every QEMU result in this spec
  is produced by hand and pasted.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer
constraints, message ordering, and every MUST/MUST NOT.

The two obligations are RFC 7296 Section 2.23, quoted verbatim in the Task section with
their locators. RFC 3948 governs the UDP encapsulation format itself, and route A depends
on its header rules.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-7 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-verify` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: no live row without a destination
- [ ] `make ze-qemu-integration-test` RUN by hand, output pasted
- [ ] The owner authorized every edit to a tagged RFC test, in writing

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

## Audit 2026-08-02: phases 1 to 4 landed, phase 5 open. NOT ready to close

Read against the code on 2026-08-02, during the closure of
`plan/learned/1313-rfcgate-1b-rfc7296-pilot.md`. This section is a bookkeeping record. It changes no
code and closes nothing.

**AC-1, AC-2, AC-3, AC-6 and AC-7 are landed. AC-5 is not-applicable by measurement and is
recorded as such. AC-4 has no proof of any kind.** Phases 1 to 4 landed, including the rekey
inheritance. Phase 5 is the only open one, and its docs half is done.

**The published claim is stronger than the evidence, and that is the finding that matters.**
`docs/features/rfc-status.md` states that the platform limit is LIFTED and that "a peer that
changes form on that SA is served". Nothing proves the mid-SA change. `TestEncapOneStateAcceptsBothForms`
measures one state accepting both forms, which is necessary and not sufficient: it never
changes form on an ESTABLISHED SA, which is what AC-4 asserts and what the doc sentence
promises a reader.

**Residual work.** Five items, not the two named in "What is NOT done" above.

- Write `test/ipsec/ipsec-esp-form-change.ci`: bring a tunnel up, change the peer's ESP form
  on the established Child SA, assert traffic still flows both ways and the SA was neither
  rekeyed nor deleted. This is the ONLY proof of AC-4, and the only thing that makes the
  `rfc-status.md` sentence true.
- Write `test/ipsec-interop/scenarios/23-esp-form-change/`, the strongSwan-driven mid-SA
  change. Its header must state why it carries no RFC tag.
- Write `test/ipsec-interop/scenarios/22-esp-encap-no-nat/`. The Interop table above names it
  and it does not exist. Neither `22-*` nor `23-*` is present under
  `test/ipsec-interop/scenarios/` (verified 2026-08-02).
- Write `test/ipsec/ipsec-esp-encap-no-nat.ci` (user story 1). Drop the planned
  `ipsec-esp-form-vpp-reject.ci`, since AC-5 is not-applicable by measurement.
- User story 3 has no path at all: `show vpn ipsec sa` exposes no ESP-form field. Either add
  the field and its test, or strike the row.

Plus two bookkeeping items:

- `ai/RFC-REQUIREMENTS.md` regeneration is still owed after the tagged-test rename.
- The spec's line anchors are stale: the encapsulation decision and the inbound and outbound
  programming sites have all moved since they were written. Prefer symbol names over line
  numbers on the next edit (`ai/rules/writing.md`).

**Until AC-4 has a test, either the test lands or the `rfc-status.md` sentence narrows to
what is measured.** Leaving a public claim ahead of its evidence is the one thing here that
misleads a reader rather than merely leaving work open.

The declared deferral shard did not exist when this audit ran. It was created on 2026-08-02
as `plan/deferrals/ipsec-esp-dual-form-receive.md`, so the Goal Gate "Deferral shard
resolved" can be evaluated. The spec stays OPEN.
