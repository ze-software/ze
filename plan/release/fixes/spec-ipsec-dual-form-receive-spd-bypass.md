# Spec: ipsec-dual-form-receive-spd-bypass

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Priority | P0 |
| Release reason | Security defect in shipped IPsec: on a Child SA moved to UDP 4500 without a NAT, bare ESP from an authenticated peer is decrypted and delivered with no inbound SPD check, so the peer can inject traffic with any inner source and destination. |
| RFC gaps | RFC4301-5.2-2 |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-10 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Close the inbound SPD bypass of the dual-form receive path, keeping both ESP
forms on one Child SA. The fix design is the owner's choice (Decision below).
Nothing is implemented yet.

**The defect, confirmed by an A/B interop run on 2026-11-10.**

1. A Child SA whose IKE SA moved to UDP 4500 is installed with UDP encapsulation
   even when no NAT was detected: `newChildSA`
   (`internal/component/ike/engine/create_child.go`) sets `UDPEncap` when
   `sa.localPort == transport.NATTPort`.
2. The inbound XFRM state therefore carries an ESP-in-UDP template. strongSwan
   6.1.0 (`child_create.c`, `this->child.encap` set from `COND_NAT_ANY`) sends
   bare ESP when it sees no NAT.
3. The kernel refuses the bare form (`XfrmInStateMismatch`). Ze's
   `espFormReceiver` (`internal/component/ike/dataplane/espform_linux.go`,
   `run`, sent by `rawESPFormInjector.inject`) reads it from a raw
   `IPPROTO_ESP` socket carrying a per-socket policy bypass, and re-sends it as
   ESP-in-UDP to Ze's own address through an `IPPROTO_RAW` socket.
4. That datagram goes out `lo`, so after decapsulation `skb->dev` is `lo`.
   Linux sets `disable_policy=1` on loopback (`net/ipv4/devinet.c`, lines 1582
   to 1585 at 7.3), so the input path and the local-input route set
   `IPSKB_NOPOLICY` (`net/ipv4/ip_input.c` line 415, `net/ipv4/route.c` line
   2446), and `__xfrm_policy_check2` (`include/net/xfrm.h` line 1285) returns
   before any policy is consulted.
5. Result: for every bare-ESP datagram on such an SA, the inner packet is
   delivered with no inbound SPD check. The SA's selectors and the
   `unmatched discard` catch-all are not enforced on that traffic.

RFC 4301 Section 5.2, step 4: "Then match the packet against the inbound
selectors identified by the SAD entry to verify that the received packet is
appropriate for the SA via which it was received." Step 5: "If an IPsec system
receives an inbound packet on an SA and the packet's header fields are not
consistent with the selectors for the SA, it MUST discard the packet."

RFC 7296 Section 2.23 is the reason the re-presenting path exists, and the fix
keeps it: "all devices MUST be able to receive and process both
UDP-encapsulated ESP and non-UDP-encapsulated ESP packets at any time."

**Evidence.**

| Run | Result | Log |
|---|---|---|
| interop `icmp-error-payload-check`, as committed (strongSwan sends bare ESP) | FAIL: "the error embedding 10.2.0.1 -> 10.7.0.5, outside the SA's selectors: ze delivered 3 Time Exceeded (Icmp.InTimeExcds), want the error dropped" | `tmp/session/2026-11-09-222726/scratch/ab-plain.log` |
| the same scenario with strongSwan `encap = yes` (ESP-in-UDP, the state's own form, received on the real interface) | PASS | `tmp/session/2026-11-09-222726/scratch/ab-encap.log` |

The kernel sources the diagnosis read are in
`tmp/session/2026-11-09-222726/scratch/kernelsrc73/` and `kernelsrc73b/`.
Those are session scratch: the line references above carry what the fix needs.

The earlier dual-form work already met this blind spot from the other side:
its Phase 5 evidence records that `__xfrm_policy_check2` short-circuits on
loopback, so the QEMU probe could not see the receive-side policy defect
(`plan/release/validation/spec-ipsec-esp-dual-form-receive.md`).

## Decision (owner)

| Option | What it does | Status |
|---|---|---|
| A | A tc-ingress BPF program on the IKE interface adds the UDP header to bare ESP for the watched SPIs, so the kernel receives the state's own form on the real interface and runs the normal inbound policy check. `ike/dataplane` already loads BPF through `cilium/ebpf` (`override_guard_linux.go`), and `internal/plugins/trafficusage/attach_linux.go` already attaches TCX programs | agent-recommended |
| B | Re-present into a Ze-owned TUN device with `disable_policy=0` and `rp_filter=2`, so the policy check runs on that device | candidate |
| C | Clear `disable_policy` on `lo` | rejected: breaks loopback traffic under `unmatched discard` |
| D | Install a template-free state, or stop re-presenting | rejected: loses RFC 7296 Section 2.23 receipt of both forms |

PENDING OWNER: A or B.

## Required Reading

- [ ] `ai/doors/system.md`, `ai/doors/access-security.md`
- [ ] `docs/guide/ipsec.md`, "Both ESP wire forms"
- [ ] `docs/architecture/linux-limitations.md`, `LL-XFRM-STATE-ONE-ESP-FORM`
- [ ] `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md`
- [ ] `plan/release/validation/spec-ipsec-esp-dual-form-receive.md`, Phase 5 Evidence
- [ ] `rfc/short/rfc4301.md` `RFC4301-5.2-2`; `rfc/full/rfc4301.txt` Section 5.2; `rfc/full/rfc7296.txt` Section 2.23

## Current Behavior (MANDATORY)

- [ ] `internal/component/ike/dataplane/espform_linux.go` - `espFormReceiver` watches SPIs, `run` reads refused bare ESP and `rawESPFormInjector.inject` re-sends it through `IPPROTO_RAW` to the local address; `espFormBypassInboundPolicy` exempts the reading socket.
- [ ] `internal/component/ike/engine/create_child.go` - `newChildSA` sets `UDPEncap` on port 4500 with no NAT.
- [ ] `internal/component/ike/dataplane/xfrm_linux.go` - `InstallSA`, the `AcceptBothESPForms && UDPEncap` branch that calls `Watch`.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Dual-form receiver | `ike/dataplane/espform_linux.go` `espFormReceiver` | Re-presents the refused ESP form through loopback | Option A replaces the loopback re-presentation with an ingress rewrite; option B replaces its output device |
| BPF loading in the IKE dataplane | `ike/dataplane/override_guard_linux.go` | Loads and attaches a cgroup program through `cilium/ebpf` | Option A reuses the loader |
| TCX attach | `internal/plugins/trafficusage/attach_linux.go` `tcxAttacher` | Attaches ingress and egress programs to an interface | Option A follows its attach and detach shape |
| Inbound catch-all | `ike/engine/unmatched.go` `installUnmatched` | Final DISCARD entry of the SPD | Unchanged; the fix makes it reach bare ESP again |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Bare ESP from the peer, on the IKE interface, for an SPI whose inbound state carries the ESP-in-UDP template.

### Transformation Path
1. Today: XFRM refuses the bare form; the raw `IPPROTO_ESP` reader takes it, `writeESPForm` adds the UDP header, `IPPROTO_RAW` re-sends it to the local address over `lo`; XFRM decrypts it; the policy check is skipped (`IPSKB_NOPOLICY`).
2. After the fix: the datagram reaches XFRM in the state's form on a device whose policy check runs (option A: the real interface; option B: a Ze-owned TUN).

### Boundaries Crossed
| Boundary | How |
|---|---|
| Ze to kernel | raw sockets today; a BPF program (A) or a TUN device (B) after the fix |

### Integration Points
- `xfrmBackend.InstallSA`, which calls `Watch` for a templated state that asks for both forms.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|---|---|---|---|
| strongSwan sending bare ESP on a 4500-without-NAT Child SA | -> | the dual-form receive path | interop `icmp-error-payload-check` without `encap = yes` |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates |
|---|---|---|
| to be named with the chosen option | `internal/component/ike/dataplane/` | an inner address outside the SA's selectors, received in the refused form, is dropped |

## Acceptance Criteria

- AC-1: interop `icmp-error-payload-check` passes with strongSwan sending bare ESP (no `encap = yes`), and its two red proofs in `plan/release/fixes/spec-rfc4301-architecture-gaps.md` are recorded.
- AC-2: interop `esp-form-change` still passes: one Child SA receives both forms with no rekey.
- AC-3: a negative test injects bare ESP with an inner address outside the SA's selectors on a 4500-without-NAT SA, on a real (non-loopback) interface, and shows it dropped.
- AC-4: `RFC4301-5.2-2` loses its `{partial}` annotation, with discrimination records.
- AC-5: the corrected statements in `docs/guide/ipsec.md`, `LL-XFRM-STATE-ONE-ESP-FORM` and the `espFormBypassInboundPolicy` comment are rewritten to the fixed behavior.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `escape-hatch-scoped-wider-than-its-justification.md` | 2026-11-10 | `espFormReceiver` re-presents bare ESP through lo, so no inbound SPD check runs |

## Files to Modify

- `internal/component/ike/dataplane/espform_linux.go` - the re-presentation path (either option).
- `docs/guide/ipsec.md`, `docs/architecture/linux-limitations.md` - the dual-form description.
- `rfc/short/rfc4301.md` - `RFC4301-5.2-2`.

## Implementation Steps

1. The owner picks option A or B (Decision above).
2. Write the negative test first and show it red on the current path.
3. Implement the chosen option, deleting the loopback re-presentation it replaces (`ai/rules/no-layering.md`).
4. Run the interop scenarios of AC-1 and AC-2 and record the red proofs.

## Checklist

- [ ] Tests written
- [ ] Tests FAIL before the fix
- [ ] Tests PASS after the fix
- [ ] `./le test unit` over `internal/component/ike/dataplane`, then `./le test integration interop-ipsec` for `icmp-error-payload-check` and `esp-form-change`
