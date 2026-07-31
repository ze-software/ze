# Spec: fixit-vpp-ipsec-inoperable

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Deferral shard | `plan/deferrals/fixit-vpp-ipsec-inoperable.md` |
| Updated | 2026-07-30 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

Found on 2026-07-30. The work that found it gathered evidence for owner ruling OR-F of
`plan/spec-rfcgate-1b-rfc7296-pilot.md`. The question asked was narrow: can the VPP backend
copy the ECN bits that `RFC7296-2.24-1` and `2.24-2` require. The answer turned out to be
that the backend cannot program a security association at all.

## Task

**The VPP IPsec dataplane backend fails before it encodes anything. Every `InstallSA`,
`RemoveSA`, `InstallPolicy` and `RemovePolicy` returns an error at the first request.**

`vpp.go` declares a CRC of `"00000000"` for all four messages (`vpp.go:187`, `:196`,
`:216`, `:225`). GoVPP resolves a message ID from the name and the CRC together
(`vendor/go.fd.io/govpp/core/connection.go:489` and `:507`). The lookup is a literal string
key, `msgName + "_" + msgCrc`, against the table VPP itself sends at connect
(`vendor/go.fd.io/govpp/adapter/socketclient/socketclient.go:469`). A miss returns
`UnknownMsgError` (`:472`).

The send path returns that error at
`vendor/go.fd.io/govpp/core/request_handler.go:83-87`. The `EncodeMsg` call sits at `:89`,
so the encoder never runs. The message is refused first.

The real CRCs are `c77ebd92` and `9ffac24b` for the SAD messages, and `7bfe69fc` for the
SPD message. They live in the generated binapi at
`gokrazy/modcache/go.fd.io/govpp@v0.13.0/binapi/ipsec/ipsec.ba.go:1687` and `:1798`.

**This is not a wire-format divergence that a careful patch can repair.** The struct is also
misaligned from offset 9 onward, and it omits eleven fields. Both are moot while VPP
refuses the message first.

## Why no test caught it

Three unit files exist: `vpp_test.go`, `vpp_extra_test.go`, `register_vpp_test.go`. None of
them touches the message structs. They cover `vppCryptoAlg`, `vppIntegAlg`, `vppAddress`
and `vppPrefix` only.

A test written against a hand-rolled struct agrees with that struct by construction. The
divergence is between the struct and the message a real VPP expects, and no unit test can
see across that boundary. `ai/rules/testing.md` names the shape: a unit test proves the
algorithm, and only a running functional or interop test proves the daemon.

## Nothing in production selects this backend

`ikeDataplaneName` returns `"xfrm"` unless an override is set
(`internal/component/ike/engine/testport.go:42-50`). Its own comment states that production
always uses XFRM. The only override is `ze.test.ike.dataplane`, registered `Private: true`
and described as test infrastructure (`testport.go:31-37`). OSPF hardcodes
`dataplane.Load("xfrm")` (`internal/plugins/ospf/ipsec_install.go:71`).

The file is also behind `//go:build ze_vpp` (`vpp.go:4`), so a default build omits it.

**This bounds the blast radius, and it does not excuse the defect.** A registered backend
that cannot work is a trap for the first operator who selects it, and `ze config verify`
accepts the selection today.

## The three value bugs, independent of layout

Each is wrong on its own terms, and each would survive a layout repair.

| Site | Sends | Correct | Consequence |
|------|-------|---------|-------------|
| `vpp.go:48` | `Protocol: 1` with the comment `// ESP` | 50 | Ze's OWN constant already agrees with VPP: `dataplane.go:30` declares `ProtoESP uint8 = 50`. The backend disagrees with its own package. `p.Proto` is ignored, so AH is unreachable |
| `vpp.go:292` | `"3des"` maps to 4 | 11 | 4 is `AES_CTR_128`. This programs a different cipher than the operator configured, which `ai/rules/exact-or-reject.md` forbids outright |
| `vpp.go:199-208` | `ipsecSPDEntry` models two prefixes | four address range endpoints | The real `IpsecSpdEntryV2` has no prefix field at all. This is a semantic mismatch, not a misordering |

## The same policy-to-SA gap the XFRM backend carried

Found on 2026-07-31, while the XFRM backend gained the tunnel endpoints its policy template
was missing. That defect left `tmpl src 0.0.0.0 dst 0.0.0.0`, so no state matched the policy
and the tunnel forwarded nothing. The VPP backend has the same class of gap in its own form.

A PROTECT policy must name the SA that protects the traffic. The XFRM backend names it
through the template tunnel endpoints. VPP names it through an SA id, because
`IpsecSpdEntry` and `IpsecSpdEntryV2` carry `SaID` and hold no template addresses at all
(`gokrazy/modcache/go.fd.io/govpp@v0.13.0/binapi/ipsec_types/ipsec_types.ba.go:346`
and `:364`). In the VPP model the tunnel endpoints live on the SAD entry instead.

`InstallPolicy` sends `Policy: 3`, which is `IPSEC_API_SPD_ACTION_PROTECT`
(`ipsec_types.ba.go:244`), together with a hardcoded `SAID: 0` (`vpp.go:104`). `SPParams`
holds no field able to carry an SA id, so no caller can supply one. The policy protects with
SA 0 and resolves to nothing. Zero is again the value that looks like a valid answer.

`InstallPolicy` also drops `p.Mode`. A transport-mode policy and a tunnel-mode policy reach
VPP as the same request.

**Not repaired here, by owner instruction.** The backend programs no SA at all while the
CRCs stay `"00000000"`, so a policy binding would have nothing to bind to. Repair this with
the rest of the message rewrite, and add the new criterion AC-8 below.

## The AEAD key and salt reach VPP as one field

Found on 2026-07-31, while `SAParams.EncKey` gained a documented contract.

`EncKey` carries the cipher key followed by that cipher's salt when `IsAEAD` is true. RFC
4106 Section 8.1 makes AES-GCM KEYMAT four octets longer than the AES key, so AES-GCM-256
gives 36 octets. The Linux XFRM backend is correct: it hands the whole slice to
`rfc4106(gcm(aes))`, which expects exactly that layout
(`internal/component/ike/dataplane/xfrm_linux.go:58-62`).

VPP does not take the two together. `ipsec_sad_entry` carries the GCM salt in its own
field, so the key field must hold the cipher key alone. The backend sends
`CryptoKey: vppKey(p.EncKey)` with all 36 octets and a hardcoded `Salt: 0`
(`internal/component/ike/dataplane/vpp.go:50` and `:56`). VPP would read a 36 octet key
into a field it keys at 32. It would also encrypt with a zero salt while the peer uses
the real one.

**Not repaired here.** The contract is now documented on the field. The split belongs
with the message rewrite rather than in a backend that cannot program an SA at all. The
repair is: take the salt from the last `len(EncKey) - keyBytes` octets, put it in `Salt`,
and pass the remainder as `CryptoKey`. Add the new criterion AC-9 below.

| New criterion | Assertion |
|---------------|-----------|
| AC-9 | An AEAD `InstallSA` sends the cipher key alone in `CryptoKey` and that cipher's salt in `Salt`, both read from `SAParams.EncKey` rather than assumed |

## Required Reading

| Document | Why |
|----------|-----|
| `ai/rules/no-fabrication.md` | A generated binding stub documents a field's existence, never what the foreign system does with it |
| `ai/rules/exact-or-reject.md` | A backend that cannot apply the config exactly must reject at verify |
| `ai/rules/go-standards.md` | The dependency rule, and why vendoring here is additive rather than new |
| `gokrazy/modcache/go.fd.io/govpp@v0.13.0/binapi/ipsec/ipsec.ba.go` | The authoritative message definition |

## Current Behavior (MANDATORY)

Source files read on 2026-07-30:

- [ ] `internal/component/ike/dataplane/vpp.go`
- [ ] `internal/component/ike/dataplane/dataplane.go`
- [ ] `internal/component/ike/engine/testport.go`
- [ ] `vendor/go.fd.io/govpp/core/connection.go`
- [ ] `vendor/go.fd.io/govpp/core/request_handler.go`
- [ ] `vendor/go.fd.io/govpp/adapter/socketclient/socketclient.go`

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point

`InstallSA` on the VPP backend, reached from `engine/child.go` after a Child SA is
negotiated. Format at entry is a `dataplane.SAParams`.

### Transformation Path

The backend translates `SAParams` into `ipsecSAEntry` and wraps it in `ipsecSAAddDel`. It
calls `SendRequest`, which asks `GetMessageID` for the numeric id
(`vendor/go.fd.io/govpp/core/request_handler.go:83`). That resolves through
`connection.go:507` to the socket client's table lookup
(`adapter/socketclient/socketclient.go:469`), keyed on `name + "_" + crc`.

The key never matches, because the CRC is `"00000000"`. `GetMsgID` returns
`UnknownMsgError` (`socketclient.go:472`), `SendRequest` returns it at
`request_handler.go:85-87`, and the flow STOPS. `EncodeMsg` at `:89` is dead code on this
path.

After the repair the same call reaches the encoder, and the generated `Marshal` writes the
layout VPP expects.

### Boundaries Crossed

| Boundary | Direction | Carries |
|----------|-----------|---------|
| engine -> dataplane | out | `SAParams`, the negotiated Child SA |
| dataplane -> govpp | out | The binary-API message, today refused before the encoder runs |
| govpp -> VPP socket | out | Nothing, because the send never happens |

### Integration Points

`plan/spec-rfcgate-1b-rfc7296-pilot.md` owns rows `RFC7296-2.24-1` and `2.24-2`. They stay
`uncertain` until this spec's AC-7 holds.

## Key Design Decisions

| Decision | Over | Because |
|----------|------|---------|
| Delete the hand-rolled types and use the generated binapi | Correcting the CRCs and the field offsets by hand | The generated types carry the correct CRC, the correct layout, and their own `Marshal` and `Size`. Hand-maintaining a foreign wire format is the defect, and correcting it by hand preserves the defect |
| Vendor `binapi/ipsec` and `binapi/ipsec_types` | Adding a new dependency | `go.fd.io/govpp` is ALREADY required. Every transitive dependency of these two packages is already vendored: `api`, `codec`, `ip_types`, `interface_types` and `tunnel_types`. The change is additive to `vendor/`, and `go.mod` does not move |
| Fix the three value bugs in the same work | A follow-up spec | They are in the lines being rewritten. Leaving a known wrong cipher mapping in code you are already editing is parking (`ai/rules/no-parking.md`) |
| Reject a `vpp` dataplane selection at config verify until a real VPP has accepted an SA | Shipping a backend that compiles | `ai/rules/exact-or-reject.md`. A backend nothing has ever exercised must not silently accept an operator's config |

## Blast Radius

`internal/component/ike/dataplane/vpp.go`, plus two additions under `vendor/`. No default
build changes, because the file is `ze_vpp`-gated and XFRM is the default backend.

**One sequencing constraint.** `go mod vendor` regenerates the whole vendor tree. Run it
when no other session holds uncommitted Go work, or their builds will fail in ways that
look like their own defects.

## Risks & Assumptions

| Id | Statement | Basis | Validation |
|----|-----------|-------|------------|
| A-1 | The module is already required, so vendoring is additive | `vendor/modules.txt:336-363` lists 27 binapi packages from the same module | Confirm `go.mod` is unchanged after `go mod vendor` |
| A-2 | The generated types carry correct CRCs | `ipsec.ba.go:1687`, `:1798` | Compare against a live VPP message table |
| R-1 | Nothing has ever run this against a real VPP, so "fixed" cannot be proven by unit tests | No interop lab, no QEMU target, no `.ci` exercises it | The work is not done until something executes it. See Known Limitations |
| R-2 | A correct layout CAN expose further semantic mismatches, above all in the SPD path | The SPD entry is already known to model the wrong concept | Translate the SPD call from ranges, never from prefixes |

## Acceptance Criteria

| Id | Criterion |
|----|-----------|
| AC-1 | The four hand-rolled message types are gone, replaced by the generated binapi types |
| AC-2 | `go.mod` is unchanged, and `vendor/modules.txt` gains exactly the two packages |
| AC-3 | `Protocol` is derived from `p.Proto`, so ESP sends 50 and AH is reachable |
| AC-4 | `"3des"` maps to 11 |
| AC-5 | The SPD call is built from address ranges |
| AC-6 | A `vpp` dataplane selection is rejected at config verify while AC-7 is unmet |
| AC-7 | Something executes this backend against a real VPP and an SA is installed |
| AC-8 | A PROTECT policy names the SA it protects with. `SPParams` carries that identity, and a request that reaches VPP with SA id 0 is rejected rather than sent |
| AC-9 | `InstallPolicy` honors `p.Mode`, so a transport-mode policy and a tunnel-mode policy differ |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | | Feature Code | Test |
|-------------|---|--------------|------|
| `dataplane.Load("vpp")` then `InstallSA` | -> | the generated message send path | a test asserting the resolved message ID is not zero |
| `ze config verify` naming the vpp dataplane | -> | the rejection in AC-6 | a `.ci` asserting the rejection |

## 🧪 TDD Test Plan

### Unit Tests

| Test | Proves |
|------|--------|
| A CRC assertion against the generated constants | AC-1 |
| `Protocol` derived from `p.Proto` for ESP and AH | AC-3 |
| The `3des` mapping | AC-4 |
| The SPD range translation | AC-5 |

### Functional Tests

| Test | Role |
|------|------|
| A `.ci` in `test/ipsec/` asserting the config rejection | AC-6 |

The ipsec suite runs on every push, because `ipsec` is in `all_suites`
(`mk/test-functional.mk`).

## Files to Modify

| File | Change |
|------|--------|
| `internal/component/ike/dataplane/vpp.go` | Delete the hand-rolled types, use the generated ones, fix the three value bugs |
| `vendor/modules.txt` | Two package additions from `go mod vendor` |

## Implementation Steps

1. Wait for a window with no other uncommitted Go work in the tree.
2. Import the two packages, then run `go mod vendor`. Confirm `go.mod` is unchanged.
3. Replace the four type sets. Fix the three value bugs in the same edit.
4. Add the config-verify rejection.
5. Build with `ze_vpp` and run the dataplane tests.

## Goal Gates

`make ze-verify`.

## Quality Gates

`make ze-lint-changed`, `python3 scripts/dev/ste_check.py --check`.

## RFC Documentation (Scope: protocol)

`RFC7296-2.24-1` and `2.24-2` stay `uncertain` in the pilot's Appendix A until AC-7 holds.
Once the backend can program an SA, ECN copying is one assignment:
`Tunnel.EncapDecapFlags` takes `ENCAP_COPY_ECN | DECAP_COPY_ECN`
(`vendor/go.fd.io/govpp/binapi/tunnel_types/tunnel_types.ba.go:42-43`, already vendored).

Do NOT tag either row before AC-7. A tag asserts a proof, and no proof exists while nothing
executes the backend.

## Known Limitations

**AC-7 is the honest bar and it is not cheap.** Nothing in the repository runs Ze's IPsec
against a real VPP today. Until something does, a green unit suite says only that the code
agrees with the generated types. That is a large improvement over agreeing with a
hand-rolled guess, and it is not the same as working.

`ai/rules/qemu-testing.md` describes the shape such a proof takes.

## Checklist

- [ ] Tests written first, run, and Tests FAIL output pasted into the spec
- [ ] The hand-rolled types are gone
- [ ] `go.mod` unchanged
- [ ] The three value bugs fixed
- [ ] The config-verify rejection lands with a `.ci`
- [ ] Tests PASS output pasted into the spec
- [ ] `make ze-verify` green
