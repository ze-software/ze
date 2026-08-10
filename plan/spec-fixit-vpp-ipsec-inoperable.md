# Spec: fixit-vpp-ipsec-inoperable

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Depends | - |
| Phase | 2/2 |
| Deferral shard | `-` (corrected 2026-08-03: the row named a shard that never existed; nothing deferred; AC-7 is in scope, and the three value bugs are fixed in the same work rather than deferred. Create `plan/deferrals/fixit-vpp-ipsec-inoperable.md` on the first deferral) |
| Updated | 2026-08-10 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

Found on 2026-07-30. The work that found it gathered evidence for owner ruling OR-F of
the rfcgate-1b RFC 7296 pilot spec. The question asked was narrow: can the VPP backend
copy the ECN bits that `RFC7296-2.24-1` and `2.24-2` require. The answer turned out to be
that the backend cannot program a security association at all.

## Task

**The VPP IPsec dataplane backend fails before it encodes anything. Every `InstallSA`,
`RemoveSA`, `InstallPolicy` and `RemovePolicy` returns an error at the first request.**

`vpp.go` declares a CRC of `"00000000"` for all four messages (`vpp.go`, `:196`,
`:216`, `:225`). GoVPP resolves a message ID from the name and the CRC together
(`vendor/go.fd.io/govpp/core/connection.go` and `:507`). The lookup is a literal string
key, `msgName + "_" + msgCrc`, against the table VPP itself sends at connect
(`vendor/go.fd.io/govpp/adapter/socketclient/socketclient.go`). A miss returns
`UnknownMsgError`.

The send path returns that error at
`vendor/go.fd.io/govpp/core/request_handler.go`. The `EncodeMsg` call sits at `:89`,
so the encoder never runs. The message is refused first.

The real CRCs are `c77ebd92` and `9ffac24b` for the SAD messages, and `7bfe69fc` for the
SPD message. They live in the generated binapi at
`gokrazy/modcache/go.fd.io/govpp@v0.13.0/binapi/ipsec/ipsec.ba.go` and `:1798`.

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
(`internal/component/ike/engine/testport.go`). Its own comment states that production
always uses XFRM. The only override is `ze.test.ike.dataplane`, registered `Private: true`
and described as test infrastructure (`testport.go`). OSPF hardcodes
`dataplane.Load("xfrm")` (`internal/plugins/ospf/ipsec_install.go`).

The file is also behind `//go:build ze_vpp` (`vpp.go`), so a default build omits it.

**This bounds the blast radius, and it does not excuse the defect.** A registered backend
that cannot work is a trap for the first operator who selects it.

*Corrected 2026-08-10: this paragraph used to end "and `ze config verify` accepts the
selection today". It does not, because no config surface carries the selection. See
"AC-6 corrected" under Acceptance Criteria.*

## The three value bugs, independent of layout

Each is wrong on its own terms, and each would survive a layout repair.

| Site | Sends | Correct | Consequence |
|------|-------|---------|-------------|
| `vpp.go` | `Protocol: 1` with the comment `// ESP` | 50 | Ze's OWN constant already agrees with VPP: `dataplane.go` declares `ProtoESP uint8 = 50`. The backend disagrees with its own package. `p.Proto` is ignored, so AH is unreachable |
| `vpp.go` | `"3des"` maps to 4 | 11 | 4 is `AES_CTR_128`. This programs a different cipher than the operator configured, which `ai/rules/protocol.md` forbids outright |
| `vpp.go` | `ipsecSPDEntry` models two prefixes | four address range endpoints | The real `IpsecSpdEntryV2` has no prefix field at all. This is a semantic mismatch, not a misordering |

## The same policy-to-SA gap the XFRM backend carried

Found on 2026-07-31, while the XFRM backend gained the tunnel endpoints its policy template
was missing. That defect left `tmpl src 0.0.0.0 dst 0.0.0.0`, so no state matched the policy
and the tunnel forwarded nothing. The VPP backend has the same class of gap in its own form.

A PROTECT policy must name the SA that protects the traffic. The XFRM backend names it
through the template tunnel endpoints. VPP names it through an SA id, because
`IpsecSpdEntry` and `IpsecSpdEntryV2` carry `SaID` and hold no template addresses at all
(`gokrazy/modcache/go.fd.io/govpp@v0.13.0/binapi/ipsec_types/ipsec_types.ba.go`
and `:364`). In the VPP model the tunnel endpoints live on the SAD entry instead.

`InstallPolicy` sends `Policy: 3`, which is `IPSEC_API_SPD_ACTION_PROTECT`
(`ipsec_types.ba.go`), together with a hardcoded `SAID: 0` (`vpp.go`). `SPParams`
holds no field able to carry an SA id, so no caller can supply one. The policy protects with
SA 0 and resolves to nothing. Zero is again the value that looks like a valid answer.

`InstallPolicy` also drops `p.Mode`. A transport-mode policy and a tunnel-mode policy reach
VPP as the same request.

**Not repaired here, by owner instruction.** The backend programs no SA at all while the
CRCs stay `"00000000"`, so a policy binding would have nothing to bind to. Repair this with
the rest of the message rewrite, and add the new criterion AC-8 below.

### Added 2026-08-02: `InstallPolicy` also ignores the `Action` and `Priority` contract

Found while closing the rfcgate-1b RFC 7296 pilot spec. This extends the paragraph
above rather than restating it: the hardcoded `Policy: 3` is named there as the SA-id gap,
and two further fields were added to `SPParams` after this spec was written.

`vppBackend.InstallPolicy` (`internal/component/ike/dataplane/vpp.go`) hardcodes
`Priority: 100` at `:153` and `Policy: 3` at `:158`. It reads neither `p.Action` nor
`p.Priority`.

Both fields are now part of the interface contract, and both carry explicit instructions in
their own declarations:

- `SPParams.Action` (`dataplane.go`) states that a backend which cannot express
  `SPActionBypass` MUST reject the install rather than fall back to protecting, and gives
  the reason: a bypass silently downgraded to a protect policy black-holes the traffic it
  was meant to let through (`ai/rules/protocol.md`). The VPP backend does exactly
  that fall-back, because 3 is PROTECT unconditionally.
- `SPParams.Priority` (`dataplane.go`) states that lower means higher precedence and
  names the two constants callers use, `PriorityIKEBypass` (100) and `PriorityChildSA`
  (2000). A hardcoded 100 gives every Child SA policy the IKE bypass precedence, which
  inverts the ranking the constants exist to express.

**Masked today, and only by an accident of ordering.** `vppUnsupportedSelector(p)` runs
first, at `:142`, and refuses the selectors that would reach these fields. So the defect is
latent rather than active. That mask is not a fix, and it disappears the moment the selector
support widens. Whoever relaxes `vppUnsupportedSelector` inherits this.

Repair it with the rest of the message rewrite. A bypass policy that VPP cannot express must
be REFUSED, never downgraded.

## The AEAD key and salt reach VPP as one field

Found on 2026-07-31, while `SAParams.EncKey` gained a documented contract.

`EncKey` carries the cipher key followed by that cipher's salt when `IsAEAD` is true. RFC
4106 Section 8.1 makes AES-GCM KEYMAT four octets longer than the AES key, so AES-GCM-256
gives 36 octets. The Linux XFRM backend is correct: it hands the whole slice to
`rfc4106(gcm(aes))`, which expects exactly that layout
(`internal/component/ike/dataplane/xfrm_linux.go`).

VPP does not take the two together. `ipsec_sad_entry` carries the GCM salt in its own
field, so the key field must hold the cipher key alone. The backend sends
`CryptoKey: vppKey(p.EncKey)` with all 36 octets and a hardcoded `Salt: 0`
(`internal/component/ike/dataplane/vpp.go` and `:56`). VPP would read a 36 octet key
into a field it keys at 32. It would also encrypt with a zero salt while the peer uses
the real one.

**Not repaired here.** The contract is now documented on the field. The split belongs
with the message rewrite rather than in a backend that cannot program an SA at all. The
repair is: take the salt from the last `len(EncKey) - keyBytes` octets, put it in `Salt`,
and pass the remainder as `CryptoKey`. Add the new criterion AC-9 below.

| New criterion | Assertion |
|---------------|-----------|
| AC-11 | An AEAD `InstallSA` sends the cipher key alone in `CryptoKey` and that cipher's salt in `Salt`, both read from `SAParams.EncKey` rather than assumed |

*Renumbered 2026-08-10: this criterion was written as AC-9, and the Acceptance Criteria
table below already used AC-9 for the policy mode. Two criteria under one id cannot both
be audited, so the AEAD one moved to AC-11.*

## Required Reading

| Document | Why |
|----------|-----|
| `ai/rules/evidence.md` | A generated binding stub documents a field's existence, never what the foreign system does with it |
| `ai/rules/protocol.md` | A backend that cannot apply the config exactly must reject at verify |
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

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point

`InstallSA` on the VPP backend, reached from `engine/child.go` after a Child SA is
negotiated. Format at entry is a `dataplane.SAParams`.

### Transformation Path

The backend translates `SAParams` into `ipsecSAEntry` and wraps it in `ipsecSAAddDel`. It
calls `SendRequest`, which asks `GetMessageID` for the numeric id
(`vendor/go.fd.io/govpp/core/request_handler.go`). That resolves through
`connection.go` to the socket client's table lookup
(`adapter/socketclient/socketclient.go`), keyed on `name + "_" + crc`.

The key never matches, because the CRC is `"00000000"`. `GetMsgID` returns
`UnknownMsgError` (`socketclient.go`), `SendRequest` returns it at
`request_handler.go`, and the flow STOPS. `EncodeMsg` at `:89` is dead code on this
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

The rfcgate-1b RFC 7296 pilot spec owns rows `RFC7296-2.24-1` and `2.24-2`. They stay
`uncertain` until this spec's AC-7 holds.

## Key Design Decisions

| Decision | Over | Because |
|----------|------|---------|
| Delete the hand-rolled types and use the generated binapi | Correcting the CRCs and the field offsets by hand | The generated types carry the correct CRC, the correct layout, and their own `Marshal` and `Size`. Hand-maintaining a foreign wire format is the defect, and correcting it by hand preserves the defect |
| Vendor `binapi/ipsec` and `binapi/ipsec_types` | Adding a new dependency | `go.fd.io/govpp` is ALREADY required. Every transitive dependency of these two packages is already vendored: `api`, `codec`, `ip_types`, `interface_types` and `tunnel_types`. The change is additive to `vendor/`, and `go.mod` does not move |
| Fix the three value bugs in the same work | A follow-up spec | They are in the lines being rewritten. Leaving a known wrong cipher mapping in code you are already editing is parking (`ai/rules/completion.md`) |
| Home the config rejection in `plan/future/spec-ipsec-dataplane-selector.md` | Building a YANG selector here so this spec has something to reject | The rejection needs a selection, and no config surface carries one (see "AC-6 corrected"). Adding the selector is a new feature, and a defect fix does not grow one |
| Refuse an SA whose direction names neither in nor out | Reading an unset direction as outbound | `ai/rules/evidence.md`. VPP selects an inbound SA by its flag alone, so an unflagged inbound SA decrypts nothing and the tunnel reports success while carrying no traffic |

## Blast Radius

`internal/component/ike/dataplane/vpp.go`, plus two additions under `vendor/`. No default
build changes, because the file is `ze_vpp`-gated and XFRM is the default backend.

**One sequencing constraint.** `go mod vendor` regenerates the whole vendor tree. Run it
when no other session holds uncommitted Go work, or their builds will fail in ways that
look like their own defects.

## Risks & Assumptions

| Id | Statement | Basis | Validation |
|----|-----------|-------|------------|
| A-1 | The module is already required, so vendoring is additive | `vendor/modules.txt` lists 27 binapi packages from the same module | Confirm `go.mod` is unchanged after `go mod vendor` |
| A-2 | The generated types carry correct CRCs | `ipsec.ba.go`, `:1798` | Compare against a live VPP message table |
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
| AC-6 | The only selector of the IPsec dataplane is named, and it is the private env override rather than a config leaf. See "AC-6 corrected" below |
| AC-7 | Something executes this backend against a real VPP and an SA is installed |
| AC-8 | A PROTECT policy names the SA it protects with. `SPParams` carries that identity, and a request that reaches VPP with SA id 0 is rejected rather than sent |
| AC-9 | `InstallPolicy` honors `p.Mode`, so a transport-mode policy and a tunnel-mode policy differ |
| AC-10 | Each installed SA states its direction, and a VPP inbound SA carries `IPSEC_API_SAD_FLAG_IS_INBOUND` while an outbound one does not |
| AC-11 | An AEAD `InstallSA` sends the cipher key alone in `CryptoKey` and that cipher's salt in `Salt` |
| AC-12 | A policy names an SPD this backend CREATED and VPP holds, bound to the interface the policy names. VPP creates no SPD of its own, so an entry sent with `spd_id` 0 lands in no database |
| AC-13 | Two SAs that share an SPI and differ in destination get different VPP SAD ids, and each policy resolves the id of the SA it names |
| AC-14 | The policy path refuses a direction that names neither inbound nor outbound, and refuses a policy that names no interface |
| AC-15 | Ze's any-protocol selector reaches VPP as VPP's any-protocol value, and an algorithm this backend cannot name to VPP is refused rather than defaulted |
| AC-16 | Closing the backend REMOVES what it installed: the SPD entries, the interface binding, the SPD and the SAs. A restarted ze leaves no VPP state enforcing policies that name dead SAs |
| AC-17 | A request VPP REFUSES leaves no state behind it. A refused entry add unbinds and deletes the SPD that add created, and a refused SA add is not recorded as installed |
| AC-18 | A tunnel-mode SA asks VPP to copy the ECN congestion indication on encapsulation and on decapsulation (RFC 7296 Section 2.24) |

### AC-6 corrected, 2026-08-10: there is no config selection to reject

AC-6 asked that `ze config verify` reject a `vpp` IPsec dataplane selection. **That
selection does not exist**, so the criterion named a surface Ze does not have.

Verified in the tree on 2026-08-10:

| Claim | Evidence |
|-------|----------|
| No YANG leaf selects the IPsec dataplane | `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang` names no backend, dataplane, xfrm or vpp leaf. `internal/component/ike/yang/ze-ipsec-cmd.yang` has a `dataplane` container, and it holds `show` commands that read the forwarding plane back |
| The only selector is a private env override | `ze.test.ike.dataplane`, registered `Private: true` and described as test infrastructure (`internal/component/ike/engine/testport.go`) |
| It is the one input to the backend choice | `ikeDataplaneName` returns `"xfrm"` unless that variable is set (`testport.go`), and `engine/register.go` passes its result to `dataplane.Load` |

**An operator-facing YANG selector for the IPsec dataplane does not exist.** So
`ze config verify` reads no value it could reject, and the earlier "`ze config verify`
accepts the selection today" in this spec was wrong about the same surface.

Building that selector is a FEATURE, not a defect fix, so it is not built here. It is
specified in `plan/future/spec-ipsec-dataplane-selector.md`, whose AC-4 carries the
rejection AC-6 asked for. That rejection comes off when AC-7 below holds.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | | Feature Code | Test |
|-------------|---|--------------|------|
| `dataplane.Load("vpp")` then `InstallSA` | -> | the generated message send path | a test asserting the resolved message ID is not zero |
| `installChildSA` then `InstallSA` | -> | `SAParams.Dir` to `vppSAFlags` | `TestChildSAStatesCarryTheirDirection` and `TestVPPInstallSAInboundFlag` |

*The second row used to read "`ze config verify` naming the vpp dataplane -> the
rejection in AC-6". There is no such entry point (see "AC-6 corrected"), so it moved to
`plan/future/spec-ipsec-dataplane-selector.md`, which owns the leaf it needs.*

## 🧪 TDD Test Plan

### Unit Tests

| Test | Proves |
|------|--------|
| A CRC assertion against the generated constants | AC-1 |
| `Protocol` derived from `p.Proto` for ESP and AH | AC-3 |
| The `3des` mapping | AC-4 |
| The SPD range translation | AC-5 |
| `TestVPPInstallSAInboundFlag`: an inbound SA carries `IS_INBOUND` and an outbound one does not | AC-10 |
| `TestChildSAStatesCarryTheirDirection`: each of the two states the engine installs names its direction | AC-10 |

### Functional Tests

None. The `.ci` this section used to name asserted a config rejection over a config
surface Ze does not have (see "AC-6 corrected"). It belongs to
`plan/future/spec-ipsec-dataplane-selector.md` AC-5, with the leaf it asserts against.

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

### Decided 2026-08-10: AC-7 holds, so the ECN rows are IMPLEMENTED and TAGGED

This section used to say "`RFC7296-2.24-1` and `2.24-2` stay `uncertain` until AC-7
holds. Do NOT tag either row before AC-7." AC-7 now holds and was independently
reproduced against VPP v26.06, so that instruction is spent. Two facts were checked in
the tree before deciding, because the sentence above them was already stale:

| Claim in the old text | What the tree says |
|-----------------------|--------------------|
| The rows are `uncertain` in the pilot's Appendix A | They are not. `plan/spec-rfcgate-1b-rfc7296-pilot-deferred-eap-identifier.md` names neither row, and `ai/RFC-REQUIREMENTS.md` shows both carrying a positive and a negative tag over the XFRM backend (`rfc7296_ecn_linux_test.go`, `engine/rfc7296_ecn_test.go`) |
| ECN copying is one assignment | Correct. `tunnel_types.Tunnel.EncapDecapFlags` exists in the vendored binapi and takes both flags |

**The decision is to implement it, because full compliance was reachable and small
(`ai/rules/rfc-compliance.md`, rung 2: implement, do not ask).** `InstallSA` now sets
`ecnFullFunctionality` (`vpp.go`), which is
`TUNNEL_API_ENCAP_DECAP_FLAG_ENCAP_COPY_ECN | ..._DECAP_COPY_ECN`. This is not
cosmetic on VPP: zero is `TUNNEL_API_ENCAP_DECAP_FLAG_NONE`, so the tunnel this backend
built until today DISCARDED the congestion indication on decapsulation, which is what
`RFC7296-2.24-2` forbids. The XFRM backend needed no equivalent because Linux copies by
default and ze asks it to disable nothing.

Both rows gain a positive and a negative tag on this backend:
`TestVPPInstallSACopiesECN` and `TestVPPInstallSAECNIsOnTheSAZeInstalls`
(`vpp_message_test.go`). The negative one proves the flags are read off the message the
production path SENT, that the entry is a TUNNEL entry (the class Section 2.24 binds),
and that a mode this backend cannot express is refused rather than sent wearing the
flags. `ai/RFC-REQUIREMENTS.md` is regenerated with them.

**This closes owner ruling OR-F's question**, which is the question that opened this
spec: can the VPP backend copy the ECN bits. It can, and it now does.

### OWNER QUESTION, owed at closure: `AcceptBothESPForms` is implemented and unproven

**Do not answer this in the spec. It is `ai/rules/rfc-compliance.md`'s "which way do I
fix it", and only Thomas answers it.**

`SAParams.AcceptBothESPForms` (`dataplane.go`) carries a MUST. RFC 7296 Section 2.23
requires one inbound SA to receive both the UDP-encapsulated and the bare ESP form,
which `rfc/short/rfc7296.md` holds as `RFC7296-2.23-10` and `RFC7296-2.23-11`. The XFRM
backend serves the second form beside the kernel, because Linux binds one state to one
form (`espform.go`). The VPP backend passes the field through and refuses nothing
(`InstallSA`, `vpp.go`), and what VPP's inbound node graph does with the two forms is
not readable from this tree. The AC-7 run sends no ESP, so it settles nothing about it.

So the requirement is IMPLEMENTED on this backend and UNPROVEN, which is the state
`ai/rules/rfc-compliance.md` says must reach Thomas rather than be classified here.

**The question, in the form the rule requires:** `RFC7296-2.23-10` states that "if
Network Address Translation Traversal (NAT-T) is supported, all devices MUST be able to
receive and process both UDP-encapsulated ESP and non-UDP-encapsulated ESP packets at
any time", and `RFC7296-2.23-11` that "implementations MUST process received
UDP-encapsulated ESP packets even when no NAT was detected". `vppBackend.InstallSA`
installs an inbound SA that claims both and proves neither. Which way do you want it
fixed: build the ESP-on-the-wire harness that sends both forms to a real VPP on one
inbound SA and asserts both decrypt, or refuse an inbound SA on the VPP backend until
that harness exists?

No `{gap}`, `partial` or `{not-applicable}` is written for these rows, and none may be
written until that answer arrives.

## AC-7 is MET: this backend now programs a real VPP

*Corrected 2026-08-10. This section used to say "no target in this checkout runs this
backend against a real VPP, so the criterion cannot be met by any amount of work inside
this spec". **The second clause was false.** `scripts/evidence/effective-vpp.py` has run
a real VPP in privileged Docker since before this spec was written, behind
`make ze-deployment-vpp-test` (`mk/test-integration.mk`), and it asserts through
`vppctl`. The harness class AC-7 needed already existed, and adding an IPsec case to it
was work rather than an impossibility.*

`run_ipsec_evidence` (`scripts/evidence/effective-vpp.py`) is that case. It runs the
`ze_vpp && integration` test binary of `internal/component/ike/dataplane` inside the
container (`TestVPPRealDataplaneInstalls`,
`internal/component/ike/dataplane/vpp_real_integration_test.go`), which installs two SAs
and two policies over the VPP binary API, and it then reads VPP back through `vppctl`.

Measured on **VPP v26.06** (`ligato/vpp-base`):

```
[0] sa 1 (0x1) spi 287454020 (0x11223344) protocol:esp flags:[anti-replay tunnel ]
[1] sa 2 (0x2) spi 1432778632 (0x55667788) protocol:esp flags:[anti-replay tunnel inbound ]
   salt 0xdeadbeef
   crypto alg aes-gcm-256 key 000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f
   integrity alg none key [redacted]
   table-ID:0 [192.0.2.1->198.51.100.1] hop-limit:0 point-to-point
spd 1
 ip4-outbound:
   [1] priority -100 action bypass type ip4-outbound protocol UDP
   [0] priority -2000 action protect type ip4-outbound protocol any sa 1
SPD Bindings:
  1 -> loop0
```

Re-measured on 2026-08-10 after round 4, on the same VPP v26.06. The probe now runs a
CLEANUP half first, which installs its own SA and policy, closes the backend, and asserts
against VPP that neither survives (AC-16):

```
OK: Close removed the SA 0xbadcafe and SPD 1 it installed, so a restart leaves no orphan
OK: real VPP holds both SAs ze installed, and flags one of them inbound
OK: real VPP reports the AEAD cipher key and its salt in their own fields
OK: real VPP holds SPD 1 bound to loop0, with both policies
```

Re-measured on 2026-08-10 after round 5, which added ONE assertion: the ECN copy flags
were printed by this run all along and nothing read them. The same VPP v26.06 reports
them on the tunnel of the SA ze installed:

```
OK: real VPP holds the ECN copy flags RFC 7296 Section 2.24 requires
   table-ID:0 [192.0.2.1->198.51.100.1] hop-limit:0 point-to-point CS0 [resolved ]
   [encap-copy-ecn decap-copy-ecn ] [resolved via fib-entry: 7]
```

### What that run settled

| Was | Now | Evidence |
|-----|-----|----------|
| UNVERIFIED: the ordering `vppPriority` negates for | **MEASURED: DESCENDING.** The negation is correct | The IKE bypass (VPP -100) is listed AHEAD of the Child SA policy (VPP -2000), and two policies added afterwards at VPP 7 and 8 sort ahead of both, highest first |
| UNVERIFIED: the byte order VPP reads `Salt` in | **MEASURED for the API round trip.** The four KEYMAT octets reach VPP's salt in KEYMAT order | KEYMAT ending `de ad be ef` is reported as `salt 0xdeadbeef`, beside the 32 cipher octets alone. Where VPP then places them in the GCM nonce is NOT settled: that needs ESP arriving at a real VPP |
| Unlabeled: "Zero is VPP's any protocol" (`vppSPDEntry`) | **MEASURED AND FALSE.** Zero is IANA protocol 0 | A policy sent with protocol 0 prints as `protocol IP6_HOP_BY_HOP_OPTIONS`; VPP's own CLI with no protocol prints `protocol any`, and so does protocol 255. `vppUpperProto` now maps Ze's any to 255 |
| Unlabeled: "VPP selects an inbound SA by this flag alone" (`vppSAFlags`) | **PARTLY MEASURED.** VPP records the flag on the inbound SA and not on the outbound one | `flags:[anti-replay tunnel inbound ]` against `flags:[anti-replay tunnel ]`. That inbound SELECTION reads nothing else is still unverified, and is labeled at the producer |
| ASSUMED (round 4, first attempt): unbinding the interface and deleting the SPD releases the SA its PROTECT entry names | **MEASURED AND FALSE.** The SA survived, and every request returned retval 0 | The first run of the AC-16 case reported `VPP still holds SA spi=0xbadcafe after Close`. VPP says why in its own output: `show ipsec sa` prints `locks 2` for an SA a PROTECT entry names. An SA is REFCOUNTED, so the entries must be sent back before the SA delete releases the last reference. `removeInstalled` does that, and the case then passed |

**One claim was WITHDRAWN rather than measured.** `InstallSA` used to call
`SAParams.AcceptBothESPForms` MEASURED, citing `ipsec_tun_in.c`, `ipsec_tun.c`,
`ipsec_tun_protect_input_inline`, `ipsec4_tunnel_mk_key` and `ipsec_tun_register_nodes`.
Those files are VPP source and are not in this tree, so no reader here could re-derive
it. The comment now states the claim is unverified and names its proof: a real VPP
receiving BOTH ESP forms on one inbound SA. The AC-7 run sends no ESP, so it does not
settle it.

## Known Limitations

**THE VPP IPSEC BACKEND CANNOT BE DRIVEN BY IKE UNTIL AN INTERFACE IS SUPPLIED.**
This is the limitation that decides whether the backend is usable, so it is stated
first. VPP has no node-wide security policy database: a policy lives in an SPD, and an
SPD acts only on the interfaces it is bound to. `vppPolicyInterface` (`vpp_policy.go`)
therefore refuses `SPParams.IfIndex` 0, and that refusal is correct, because sw_if_index
0 is a real VPP interface. `childPolicyParams` (`internal/component/ike/engine/child.go`)
leaves the field at 0. So this backend installs SAs, and refuses every policy IKE
produces for them.

Nothing was invented to close it, because the three producers read on 2026-08-10 rule
out a small fix. `SPParams.IfIndex` (`dataplane.go`) is an XFRM `sel.ifindex` on Linux
that IKE leaves 0 ON PURPOSE, so filling it in the engine would scope the production
XFRM backend's policies to one interface. The one producer of a non-zero value,
`buildIPsecPolicies` (`internal/plugins/ospf/ipsec_install.go`), passes a KERNEL index,
which names a different interface than the same number does in VPP. And the IKE engine
holds addresses rather than interfaces: `ChildSA` carries `LocalAddr` and `RemoteAddr`
and no interface at all. Supplying the interface therefore needs either a config surface
that does not exist or an address-to-interface resolution with a real design question in
it, so it is homed in `plan/future/spec-ipsec-vpp-policy-interface.md`.

**A Child SA rekey would leave the retired policy in VPP, and the retired SA after
`Close`.** Unreachable today, and
reachable the moment an interface is supplied, so it is recorded here and in
`plan/future/spec-ipsec-vpp-policy-interface.md` rather than fixed. `InstallPolicy`
(`vpp_policy.go`) APPENDS: it sends `ipsec_spd_entry_add_del_v2` with `IsAdd` true and
records the entry in `b.spdEntries`. The XFRM backend UPSERTS instead
(`XfrmPolicyUpdate`, `xfrm_linux.go`), and `removeChildSAExcept`
(`internal/component/ike/engine/child.go`) is built on that: a make-before-break rekey
holds ONE shared policy per direction, so it passes `dropPolicy=false` and never sends
the retired child's policy back. VPP matches an SPD entry by its whole content, and the
replacement child's entry differs in `SaID`: a rekey carries a new SPI (RFC 7296 Section
2.8), which is a new `saIdentity` and a new SAD id (`allocSadID`). So VPP would hold two
entries per direction after one rekey, and the retired one would stay in `b.spdEntries`
and in VPP until `Close`, naming a retired SA whose reference VPP is still counting.

**The retired SA is one step worse: it survives `Close` as well.** Found on 2026-08-10,
after the round-5 review, by reading the chain rather than the call site.
`removeChildSAOutgoing` and `removeChildSAIncoming` call `RemovePolicyParams` only when
`dropPolicy` is true, and they call `RemoveSA` unconditionally. So on a rekey `RemoveSA`
runs while the retired SA's PROTECT entry still holds VPP's reference. `deleteSAD` then
returns retval 0 and VPP keeps the SA, which is the same VPP behavior `removeInstalled`
measured on v26.06. `RemoveSA` reads `err == nil` as success and drops the identity from
`b.sadIDs`. `removeInstalled` sends both SPD entries back, and then iterates `b.sadIDs`
for the SA deletes, where the retired identity no longer is. Nothing sends that delete
again, so the SA is leaked until VPP itself restarts. Recorded in
`plan/journal/false-synchronization-claim.md` and covered by AC-5 of
`plan/future/spec-ipsec-vpp-policy-interface.md`. The `RemoveSA` comment
(`internal/component/ike/dataplane/vpp.go`) names the `dropPolicy=false` exception, in
place of the claim that the caller always removes the policy first.

**A ze that is KILLED leaves its VPP state behind.** `Close` removes the SPD entries,
the interface binding, the SPD and the SAs this backend installed, which covers an
orderly restart. It cannot cover a kill, and nothing readable from VPP can: an
`ipsec_sad_entry_v3` and an `ipsec_spd` carry no name and no owner, so an id another API
client created is indistinguishable from one an earlier ze run created. A record ze
PERSISTED of the ids it allocated would survive a kill, and nothing here writes one. The VPP traffic
backend reclaims its policers at STARTUP because they carry a `ze/` name prefix
(`cleanupStartupOrphans`, `internal/plugins/traffic/vpp/backend_linux.go`), and it states
the same limit for its own anonymous classify tables.

**The AC-7 run programs and reads back. It moves no packet.** Everything above is what
VPP accepted and reports holding. Encryption, decryption, replay and the GCM nonce need
ESP on the wire and a peer, which is a larger harness than this spec builds.

**`make ze-deployment-vpp-test` is not green as a whole, for a reason outside this
spec.** Its IPsec, FIB, MPLS and traffic cases all pass. Its firewall case fails on a
plugin startup deadlock: with `firewall { backend vpp; }` the firewall plugin waits for
VPP to be reachable during its Config stage while the `vpp` plugin waits at the
capability barrier. Reproduced on a FRESH VPP with the firewall case alone, so it is not
this spec's state. Recorded in `plan/journal/plugin-startup-barrier-deadlock.md`.

**Transport mode, state selectors, XFRM interface ids, node-wide policies and SPD
enumeration are all REFUSED rather than approximated.** Each refusal names what VPP
would have been programmed with instead.

## Release judgment

Recorded from the round-5 independent review, which re-ran the real VPP itself. It
decides scope, so it is written here rather than left in a report.

**The spec is CLOSABLE as a defect fix. The backend is NOT shippable as a feature.**
The two verdicts are separate, and the second is what the gating below protects.

| Finding | Why it decides the second verdict |
|---------|-----------------------------------|
| The find rate has not fallen | Six wire-visible defects so far, and each one was found only because somebody looked. Round 5 found the sixth, the rekey policy above |
| Zero packets have ever crossed it | Every run programs VPP and reads it back. Encryption, decryption, replay and the GCM nonce stay unproven |
| It installs no policy IKE produces | `vppPolicyInterface` refuses the `IfIndex` 0 that `childPolicyParams` leaves, which is the headline Known Limitation |
| An operator cannot read back or unpick what it programmed | `ListSAs` (`vpp.go`), `ListPolicies` and the three-argument `RemovePolicy` (`vpp_policy.go`) all refuse. `RemoveSA` and `RemovePolicyParams` each remove ONE object. The IKE engine calls both, and so does the OSPF installer (`removeLocked`, `rollback`, `plugins/ospf/ipsec_install.go`), which operator OSPF config drives. On VPP that path is inert for a reason of ORDER, not of reach: `installLocked` installs the SA first, `vppUnsupportedSA` refuses it for its unset `Dir`, so `i.installed` records nothing and both removals have nothing to send. So an operator reads nothing back, and the only removal an operator can cause is `Close` |

**Gating, recommended by the reviewer and endorsed by the main thread.** Keep the
backend behind the private `ze.test.ike.dataplane` override. Do NOT land
`plan/future/spec-ipsec-dataplane-selector.md` until BOTH
`plan/future/spec-ipsec-vpp-policy-interface.md` and an ESP-on-the-wire harness exist.
The same note is written into that selector spec, so nobody lands it unaware.

## Checklist

- [ ] Tests written first, run, and Tests FAIL output pasted into the spec
- [ ] The hand-rolled types are gone
- [ ] `go.mod` unchanged
- [ ] The three value bugs fixed
- [ ] Each SA states its direction, and the VPP inbound flag follows it
- [ ] The config-verify rejection moved to `plan/future/spec-ipsec-dataplane-selector.md`, which owns the leaf it needs
- [ ] Tests PASS output pasted into the spec
- [ ] `make ze-verify` green

## Implementation Audit

### Requirements from Task

| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Every message is the generated binapi message, resolvable by name+CRC | Done | `internal/component/ike/dataplane/vpp.go`, `vpp_policy.go` | `TestVPPMessageCRCs`, `requireResolvable` on every captured send |
| The three value bugs are fixed | Done | `vppProto`, `vppCryptoAlg` (`vpp.go`), `vppRange` (`vpp_policy.go`) | ESP is 50 from `p.Proto`, `3des` is 11, the selector is an address range |
| A PROTECT policy names the SA that protects it | Done | `spdEntry`, `vpp_policy.go` | Resolves the SAD id through `saIdentity`; SAID 0 and an unknown SA are both refused |
| The SPD chain exists | Done | `ensureSPD`, `freeSPDID`, `spdIDs`, `vpp_policy.go` | `ipsec_spd_add_del` then `ipsec_interface_add_del_spd`; the id is the lowest VPP does not hold and its creation is confirmed by reading the SPD list back |
| The SAD id does not collide across peers | Done | `allocSadID`, `firstFreeSadID`, `vpp.go` | Keyed on the RFC 4301 triple; the counter starts above the highest id VPP holds |
| Neither algorithm mapper defaults | Done | `vppCryptoAlg`, `vppIntegAlg`, `vpp.go` | Both return an error; the two tests that pinned the defaults now assert the refusal |
| The policy path fails closed on direction | Done | `spdEntry`, `vpp_policy.go` | First check, as `vppUnsupportedSA` is on the SA side |
| Something executes this backend against a real VPP | Done | `run_ipsec_evidence`, `scripts/evidence/effective-vpp.py` | See "AC-7 is MET" above for the `vppctl` output |
| No claim is labeled MEASURED that a reader here cannot re-derive | Done | `InstallSA`, `vpp.go` | The `AcceptBothESPForms` claim is withdrawn and relabeled with its proof named. `freeSPDID` now labels its own time-of-check as unverified, and names running VPP as the only evidence |
| Closing the backend leaves VPP as it found it | Done | `Close`, `removeInstalled`, `vpp.go` | Entries, binding, SPD, SAs, in that order; proven against a real VPP and by `TestVPPCloseRemovesWhatItInstalled` |
| A refused request leaves no state behind it | Done | `undoSPD` (`vpp_policy.go`), the `fresh` rollback in `InstallSA` (`vpp.go`) | Two tests each, one for what is undone and one for what must NOT be |
| A tunnel-mode SA copies the ECN indication | Done | `ecnFullFunctionality`, `InstallSA`, `vpp.go` | `RFC7296-2.24-1` and `-2`, tagged on this backend for both polarities |

### Acceptance Criteria

| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestVPPMessageCRCs` | The hand-rolled types are gone |
| AC-2 | Done | `git status` on `go.mod` | Unchanged; `vendor/` gained the two packages |
| AC-3 | Done | `TestVPPInstallSAProtocol` | ESP, AH, and a refusal for anything else |
| AC-4 | Done | `TestVPPCryptoAlg` | `3des` is 11 |
| AC-5 | Done | `TestVPPInstallPolicy`, `TestVPPRangeIPv4`/`IPv6` | Address ranges, not prefixes |
| AC-6 | Changed | "AC-6 corrected" above | The config selection does not exist; homed in `plan/future/spec-ipsec-dataplane-selector.md` |
| AC-7 | Done | `make ze-deployment-vpp-test`, `run_ipsec_evidence` | Real VPP v26.06, `vppctl` output in "AC-7 is MET" |
| AC-8 | Done | `TestVPPInstallPolicy`, `TestVPPInstallPolicyProtectWithoutSARefused` | The policy carries the allocated SAD id; 0, an unknown SA and another peer's SA are refused |
| AC-9 | Done | `TestVPPInstallPolicyModeIsRead` | Transport mode is refused rather than sent as tunnel |
| AC-10 | Done | `TestVPPInstallSAInboundFlag`, and the real-VPP `flags:[... inbound ]` | Both directions |
| AC-11 | Done | `TestVPPInstallSAAEADSaltSplit`, and the real-VPP `salt 0xdeadbeef` | Key and salt in their own fields |
| AC-12 | Done | `TestVPPInstallPolicyCreatesAndBindsSPD`, and the real-VPP `SPD Bindings: 1 -> loop0` | |
| AC-13 | Done | `TestVPPInstallSASameSPIDifferentPeers`, `TestVPPInstallSASkipsSADIDsVPPHolds` | |
| AC-14 | Done | `TestVPPInstallPolicyDirectionRefused`, `TestVPPInstallPolicyWithoutInterfaceRefused` | |
| AC-15 | Done | `TestVPPInstallPolicyAnyProtocol`, `TestVPPCryptoAlgUnknownRefused`, `TestVPPIntegAlgUnknownRefused` | The any-protocol value is `MEASURED` on a real VPP |
| AC-16 | Done | `TestVPPCloseRemovesWhatItInstalled`, `TestVPPCloseReportsARefusedRemovalAndContinues`, `TestVPPCloseSkipsAPolicyAlreadyRemoved`, `TestVPPCloseWithoutInstallsSendsNothing`, and the real-VPP `Close removed the SA 0xbadcafe and SPD 1` | The kill case cannot be covered and is the second Known Limitation |
| AC-17 | Done | `TestVPPInstallPolicyEntryRefusedRemovesTheSPDItCreated`, `TestVPPInstallPolicyEntryRefusedKeepsAnSPDItDidNotCreate`, `TestVPPInstallSARefusedForgetsItsSADID`, `TestVPPInstallSARefusedReinstallKeepsTheEarlierID` | Each pairs what is undone with what must NOT be |
| AC-18 | Done | `TestVPPInstallSACopiesECN`, `TestVPPInstallSAECNIsOnTheSAZeInstalls` | `RFC7296-2.24-1`/`-2`, both polarities; see "RFC Documentation" |

### Tests from TDD Plan

| Test | Status | Location | Notes |
|------|--------|----------|-------|
| A CRC assertion against the generated constants | Done | `TestVPPMessageCRCs` | |
| `Protocol` derived from `p.Proto` | Done | `TestVPPInstallSAProtocol` | |
| The `3des` mapping | Done | `TestVPPCryptoAlg` | |
| The SPD range translation | Done | `TestVPPRangeIPv4`, `TestVPPRangeIPv6`, `TestVPPInstallPolicy` | |
| `TestVPPInstallSAInboundFlag` | Done | `vpp_message_test.go` | |
| `TestChildSAStatesCarryTheirDirection` | Done | `internal/component/ike/engine` | Unchanged this round |
| The real-VPP case | Done | `TestVPPRealDataplaneInstalls`, `run_ipsec_evidence` | New this round |

### Files from Plan

| File | Status | Notes |
|------|--------|-------|
| `internal/component/ike/dataplane/vpp.go` | Done | Split: the policy half is now `vpp_policy.go`, because the single file crossed the 1000-line limit |
| `vendor/modules.txt` | Done | Two packages, `go.mod` unchanged |
| `internal/component/ike/dataplane/vpp_policy.go` | Added | Not in the plan: the SPD chain made one file too large |
| `internal/component/ike/dataplane/vpp_real_integration_test.go` | Added | Not in the plan: AC-7 needed a driver |
| `scripts/evidence/effective-vpp.py` | Added | Not in the plan: AC-7's harness |
| `docs/guide/ipsec.md` | Added | A source anchor named a function that no longer exists |

### Audit Summary

- **Total items:** 18 acceptance criteria
- **Done:** 17
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 1 (AC-6, recorded in "AC-6 corrected")

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Every `InstallSA`, `RemoveSA`, `InstallPolicy` and `RemovePolicy` no longer fails before it encodes anything | functional, against a real VPP | `make ze-deployment-vpp-test`: `run_ipsec_evidence` installs two SAs and two policies over the VPP binary API and `vppctl show ipsec all` reports `sa 1 ... spi 287454020`, `sa 2 ... spi 1432778632 ... inbound`, `spd 1` with both policies, `SPD Bindings: 1 -> loop0` |
| The backend programs what the operator configured, not something near it | functional, against a real VPP | Same run: `crypto alg aes-gcm-256 key 000102...1e1f` is the 32 cipher octets alone, `salt 0xdeadbeef` is the KEYMAT salt, `integrity alg none` is the AEAD answer, and the protect policy reads `protocol any` rather than `IP6_HOP_BY_HOP_OPTIONS` |
| A policy resolves to the SA it names | functional + unit | Real VPP: `action protect ... sa 1` names the SAD id `InstallSA` allocated. `TestVPPInstallSASameSPIDifferentPeers` proves two peers sharing an SPI get different ids and each policy resolves its own |
| Nothing is installed in a shape that reports success while being wrong | unit, each proven to discriminate | Nine refusals, each verified by reverting the behavior and watching the test go red: SPD chain, SAD id allocation, policy direction, upper protocol, policy interface, cipher default, integrity default, unknown-SA protect, RemoveSA by SPI |
| The priority ranking survives translation | functional, against a real VPP | The IKE bypass (VPP -100) is listed ahead of the Child SA policy (VPP -2000) in the same chain, and two later policies at VPP 7 and 8 sort ahead of both: DESCENDING, which is what the negation assumes |

| Nothing this backend installed outlives the process that installed it | functional, against a real VPP | `run_ipsec_evidence`: the probe installs an SA and a policy, closes the backend, and asserts through the VPP API that VPP holds neither. `OK: Close removed the SA 0xbadcafe and SPD 1 it installed`. The FIRST attempt at this failed on the real VPP and is recorded in "What that run settled" |
| A tunnel-mode SA does not discard congestion indications | unit, both polarities, tagged + functional against a real VPP | `TestVPPInstallSACopiesECN` and `TestVPPInstallSAECNIsOnTheSAZeInstalls`, carrying `RFC7296-2.24-1`/`-2`. Reverting the assignment reds the first. Round 5 added the real-VPP half: `run_ipsec_evidence` now asserts `encap-copy-ecn` and `decap-copy-ecn` in `show ipsec sa <index>`, so a real VPP says it accepted the flags and holds them. The `ai/RFC-REQUIREMENTS.md` label stays `unit/verify`: `scan_tree` walks `internal/`, `pkg/` and `test/` only (`scripts/dev/rfc_requirements.py`), so `scripts/evidence/` carries no taggable carrier and a tag there would be invisible |

**Vacuity check.** The real-VPP protocol assertion was proven to discriminate: reverting
`vppUpperProto` to pass the zero through makes `run_ipsec_evidence` fail with
`got [0] priority -2000 action protect type ip4-outbound protocol IP6_HOP_BY_HOP_OPTIONS sa 1`.

**Round 4 vacuity check.** Each of the four fixes was reverted in place and its test
re-run. All four went red, so none of them asserts something the code no longer does:

| Reverted | Test that went red |
|----------|--------------------|
| `Close` calling `removeInstalled` | `TestVPPCloseRemovesWhatItInstalled` |
| the `undoSPD` call on a refused entry add | `TestVPPInstallPolicyEntryRefusedRemovesTheSPDItCreated` |
| the `fresh` rollback of the SAD id | `TestVPPInstallSARefusedForgetsItsSADID` |
| the `EncapDecapFlags` assignment | `TestVPPInstallSACopiesECN` |

The real-VPP AC-16 case discriminated without being asked to: its first run FAILED,
against the real daemon, on a cleanup order that every unit test accepted.

**Round 5 vacuity check.** Round 5 added one assertion, so one revert was run.
`EncapDecapFlags` was set to `TUNNEL_API_ENCAP_DECAP_FLAG_NONE` and the real-VPP probe
re-run: it failed with `FAIL: real VPP SA does not report 'encap-copy-ecn'`. The
assignment was restored and the case passed, exit 0. So the new assertion reads a value
the code produces rather than one VPP prints anyway.

**How that revert was run, and why not through the make target.** The IPsec case is the
FIRST case `effective-vpp.py` runs and it passed on the round-5 tree
(`make ze-deployment-vpp-test`, 2026-08-10, `OK: real VPP holds the ECN copy flags`).
The revert pass could not use the target: another session's in-flight edit of
`internal/component/iface` left `pppoe_client.go` naming an undefined `rtproto`, so
`ensure_linux_binaries` could not build `cmd/ze` and the run aborted before any VPP
case. The IPsec case depends on none of that: it drives the backend over the VPP API
socket from its own test binary. The revert and the restore were therefore driven
through `run_ipsec_evidence` directly, with the same container, the same probe and the
same assertions.

**Hardening note, recorded and not fixed here.** `main` (`scripts/evidence/effective-vpp.py`)
calls `run_ipsec_evidence` FIRST, and the comment above the call states why: the case
drives the backend over the API socket, so it depends on nothing the ze daemon cases set
up. `ensure_linux_binaries` runs before any case and builds `cmd/ze` and `ze-test`
unconditionally, which spends that independence: a `cmd/ze` that does not compile aborts
the run before the IPsec case, exactly as it did on 2026-08-10. Building each binary at
the first case that needs it would keep the ordering's promise. The fix is in
`effective-vpp.py` and touches every case, so it is out of this spec's scope.

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/fixit-vpp-ipsec-inoperable-640fa955-f03a-45e8-a58f-4b367f5859e6.md`, `verdict=clean`, 11 files hash-pinned |
| `review_gate.py check` | clean. `review_gate: OK (0 code files, clean, hashes match ...)` with no file list, and OK again over all 11 immediately before the commit script was prepared |
| Rounds | 6. Round 6 answered a third independent review that read rounds 3 to 5 together, since `git diff` cannot separate them. Every finding was a RECORD defect: one comment asserting an invariant its callers break, three claims about what removes VPP state, one comment naming two refusals of three, one missing manifest consumer. `ai/rules/planning.md` ("Bounding the loop") makes a round of record defects the LAST round, so no round is spent confirming it |
| Rounds reason, for `review_gate.py record --rounds 6` | `--rounds-reason`: rounds 4 and 5 each found a PRODUCT defect, which is what kept the loop open past the usual two. Round 4 found a discarded ECN indication and orphan VPP state after `Close`. Round 5 found the missing policy upsert across a rekey. Round 6 found no product defect |
| Reviewer lenses used | rounds 3, 4, 5 and 6 were all independent and adversarial: false-premise, foreign-system claims, fail-closed guards, resource lifetime, doc anchors, code-versus-spec agreement, release readiness |

### Findings fixed

BLOCKER and ISSUE from the INDEPENDENT rounds only (3 to 6). The defects the spec was
written to fix are in the Implementation Audit above, not here. NOTEs are recorded under
"Recorded, not fixed" below.

| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | BLOCKER (round 3) | The SAD id was the SPI, so two peers sharing an SPI collided on one VPP SA | `InstallSA`, `vpp.go` | `allocSadID`/`firstFreeSadID` keyed on the RFC 4301 triple; `TestVPPInstallSASameSPIDifferentPeers`, `TestVPPInstallSASkipsSADIDsVPPHolds` |
| 2 | BLOCKER (round 3) | No SPD existed and none was bound, so an entry sent with `spd_id` 0 landed in no database | `InstallPolicy` | `ensureSPD` creates it and binds the interface; `TestVPPInstallPolicyCreatesAndBindsSPD`, and `SPD Bindings: 1 -> loop0` on a real VPP |
| 3 | BLOCKER (round 3) | A PROTECT policy named no SA, and `IfIndex` 0 and an unset `Dir` were both accepted | `InstallPolicy` | `spdEntry` refuses each; `TestVPPInstallPolicyProtectWithoutSARefused`, `TestVPPInstallPolicyWithoutInterfaceRefused`, `TestVPPInstallPolicyDirectionRefused` |
| 4 | BLOCKER (round 3) | Both algorithm mappers DEFAULTED silently, so an unknown name programmed a cipher nobody asked for | `vppCryptoAlg`, `vppIntegAlg` | Both return an error, and the two tests that pinned the defaults now assert the refusal |
| 5 | BLOCKER (round 3) | Ze's any-protocol selector reached VPP as protocol 0, which VPP reads as IPv6 hop-by-hop | `vppUpperProto`, `vpp_policy.go` | Mapped to 255, MEASURED on VPP v26.06 by the AC-7 run |
| 6 | ISSUE (round 3) | `InstallSA` labelled a claim about VPP's inbound node graph MEASURED, citing VPP source absent from this tree | `InstallSA`, `vpp.go` | Withdrawn and relabelled with the proof it needs; raised to Thomas as the OWNER QUESTION above |
| 7 | BLOCKER (round 4) | A tunnel-mode SA discarded the ECN indication on decapsulation, which RFC 7296 Section 2.24 forbids | `InstallSA`, `vpp.go` | `ecnFullFunctionality`; `TestVPPInstallSACopiesECN`, and `encap-copy-ecn`/`decap-copy-ecn` on a real VPP |
| 8 | BLOCKER (round 4) | `Close` left the SAs, the SPD and its binding in VPP, so a restart left orphan state | `Close`, `vpp.go` | `removeInstalled`; `TestVPPCloseRemovesWhatItInstalled`, and the real-VPP cleanup half whose FIRST run failed on the SA refcount |
| 9 | ISSUE (round 4) | A refused entry add left the SPD it had just created, and a refused SA add kept its SAD id | `InstallPolicy`, `InstallSA` | `undoSPD` and the `fresh` rollback, each with a paired test for what must NOT be undone |
| 10 | ISSUE (round 5) | `InstallPolicy` APPENDS where XFRM upserts, so a rekey would leave the retired entry, and `RemoveSA` would then leak the retired SA past `Close` | `InstallPolicy`, `RemoveSA` | Unreachable while every IKE policy is refused: homed in AC-5 of `plan/future/spec-ipsec-vpp-policy-interface.md`, recorded in `plan/journal/false-synchronization-claim.md`, and the `RemoveSA` comment now names the `dropPolicy=false` exception |
| 11 | ISSUE (round 5) | Three source anchors in the tests named `vpp.go` for symbols that live in `vpp_policy.go`, and the integration test still called `vppPriority` UNVERIFIED after its own run had measured it | `vpp_message_test.go`, `vpp_real_integration_test.go` | Repointed and relabelled |
| 12 | ISSUE (round 5) | `docs/features.md` and `docs/guide/ipsec.md` described the coverage and named neither limitation that qualifies it | both docs | Both now state that the backend refuses every policy IKE produces and that no config selects it |

### Recorded, not fixed (NOTEs)

Both would edit a file the clean review artifact hash-pins, and neither is a defect in the
product (`ai/rules/planning.md`, a finding in the record is not a finding in the product).

| NOTE | Where it is written down |
|------|--------------------------|
| The `SPParams.IfIndex` comment says "Three refusals keep that inert today". There is a FOURTH on the OSPF-to-VPP policy path: `buildIPsecPolicies` sets no `SAID`, `SPActionProtect` is the zero value of `SPParams.Action`, so `spdEntry` refuses the policy for `SAID == 0` after the mode refusal. An under-count, not a false claim | `plan/future/spec-ipsec-vpp-policy-interface.md`, "Owed on the first touch of `dataplane.go`", which is the spec that widens those refusals |
| Sixteen source anchors in `rfc/short/rfc4552.md` carried `ipsec_install.go:NNN`. The six-line comment this spec added to that file put ten of them six lines early, and `[RFC4552-3-2]`'s range for `ipsecProtoNumber` landed on the tail of `buildIPsecPolicies`. No gate catches it: `check_doc_links.py` strips the `:NNN` and checks the path alone | FIXED, by dropping the line number rather than bumping it. Every one of those anchors already names its symbol, and `ai/rules/writing.md` keeps a line number only when the line IS the fact or a gate pins it. Neither holds here, so the number was pure decay |

### Files round 6 changed

Text only. No behavior changed, and every finding was a claim that did not match the code.

| File | What changed |
|------|--------------|
| `internal/component/ike/dataplane/vpp.go` | `RemoveSA` said the caller removes the policy first. `removeChildSAExcept` does not on a rekey, so the comment now names the `dropPolicy=false` exception and states that the retired SA outlives `Close` |
| `internal/component/ike/dataplane/dataplane.go` | The `SPParams.IfIndex` comment named two refusals. `buildIPsecPolicies` emits `SADirFwd` as well, which `spdEntry` refuses on direction before the mode is read. All three are named, so the file is complete on its own |
| `plan/spec-fixit-vpp-ipsec-inoperable.md` | The leaked SA in Known Limitations, the Release judgment's removal claim corrected (`Close` was not the only thing that removes VPP state), and the `ensure_linux_binaries` hardening note |
| `plan/future/spec-ipsec-vpp-policy-interface.md` | AC-5 now covers the retired SA and `b.sadIDs`, not the SPD entry alone |
| `plan/future/spec-ipsec-dataplane-selector.md` | The DO-NOT-LAND section counted the three refusals and named no removal that works. `RemoveSA` and `RemovePolicyParams` are engine-facing rather than absent, and the gate reads the same either way |
| `feature-gates.txt` | Its header enumerates its consumers and did not name `scripts/evidence/effective-vpp.py`, which reads it in `feature_tags` |
| `plan/journal/false-synchronization-claim.md`, `plan/journal/gate-excludes-part-of-its-population.md` | The two rows owed: the rekey SA leak, and the evidence tier `scan_tree` cannot see |

`docs/features.md` was left alone deliberately. Its "an operator cannot read back or
surgically remove what it programmed" is true on the surface that page describes: no CLI,
config or API path reaches `RemoveSA` or `RemovePolicyParams`, so the operator's only
removal really is `Close`. The two specs corrected above are read by engineers, who need
the mechanism.

### Commit-time facts

`vpp_policy.go`, `vpp_message_test.go`, `vpp_real_integration_test.go` and both
`plan/future/` specs are UNTRACKED. They must appear in the `--file` list, or the commit
lands consumers without their producer. Run `make ze-tracked-build-check` after the
commit script, because the commit carries Go and nothing else compiles what git holds
(`ai/rules/git-safety.md`).

### Files round 5 changed

| File | What changed |
|------|--------------|
| `internal/component/ike/dataplane/dataplane.go` | The `SPParams.IfIndex` comment claimed the Linux and VPP readings never meet. They do, through `defaultDataplaneSource`; it now names the two refusals that keep it inert |
| `internal/component/ike/dataplane/vpp_policy.go` | The same false claim at `vppPolicyInterface`, corrected the same way |
| `internal/component/ike/dataplane/vpp.go` | `allocSadID` and `InstallSA` called the keep-the-earlier-id path "a rekey that reuses the identity"; a rekey carries a new SPI, so it is a new identity |
| `internal/component/ike/dataplane/vpp_message_test.go`, `vpp_real_integration_test.go` | Three anchors named `vpp.go` for symbols in `vpp_policy.go`; the integration test still called `vppPriority` UNVERIFIED after it had been measured by that very run |
| `scripts/evidence/effective-vpp.py` | Asserts the ECN copy flags in `show ipsec sa`, on output the harness already collected and printed |
| `docs/features.md`, `docs/guide/ipsec.md` | Both described the coverage and neither named the limitation that qualifies it: the backend refuses every policy IKE produces |
| `plan/future/spec-ipsec-vpp-policy-interface.md` | The rekey policy leak this spec makes reachable, as AC-5 |
| `plan/future/spec-ipsec-dataplane-selector.md` | The landing gate, so nobody lands the selector unaware |

### Files round 4 changed

| File | What changed |
|------|--------------|
| `internal/component/ike/dataplane/vpp.go` | `Close` removes what it installed (`removeInstalled`); the SAD id is rolled back when VPP refuses the add; the SAD entry carries the ECN flags; `spdEntries` records what Close sends back; `RemoveSA` states the refcount |
| `internal/component/ike/dataplane/vpp_policy.go` | `undoSPD`, `unbindSPD`, `deleteSPD`, `sendSPDEntry`, `forgetSPDEntry`; `ensureSPD` reports what it created; `freeSPDID` labels its time-of-check |
| `internal/component/ike/dataplane/vpp_message_test.go` | The refusing channel, and 10 tests for the above |
| `internal/component/ike/dataplane/vpp_real_integration_test.go` | The AC-16 half: install, close, and assert against a real VPP that nothing survives |
| `scripts/evidence/effective-vpp.py` | Asserts the close-cleanup half ran and passed |
| `plan/future/spec-ipsec-vpp-policy-interface.md` | New: the interface an IKE policy needs, which is a feature rather than a defect fix |
| `docs/functional-tests.md`, `docs/features.md` | The IPsec case of `ze-deployment-vpp-test` was named nowhere outside this spec |
| `ai/RFC-REQUIREMENTS.md` | Regenerated: `RFC7296-2.24-1`/`-2` gain this backend's tags |

### Files round 3 changed

| File | What changed |
|------|--------------|
| `internal/component/ike/dataplane/vpp.go` | SAD id allocation, `RemoveSA` by identity, both algorithm mappers refuse, the withdrawn MEASURED claim, the two labels resolved |
| `internal/component/ike/dataplane/vpp_policy.go` | New: the SPD chain, the policy interface guard, the direction refusal, `vppUpperProto`, and the policy half moved out of `vpp.go` |
| `internal/component/ike/dataplane/dataplane.go` | `SPParams.IfIndex` and `SPParams.SAID` documented for a backend whose identifiers are its own |
| `internal/component/ike/dataplane/vpp_message_test.go` | The dump-answering channel, and the tests for every item above |
| `internal/component/ike/dataplane/vpp_extra_test.go` | The two tests that PINNED the silent cipher defaults now assert the refusal |
| `internal/component/ike/dataplane/vpp_test.go` | Both mappers return an error |
| `internal/component/ike/dataplane/vpp_real_integration_test.go` | New: the AC-7 driver |
| `scripts/evidence/effective-vpp.py` | New: `run_ipsec_evidence`, `ensure_ipsec_probe`, `sw_if_index`; the build tags now derive from `feature-gates.txt`; the firewall fixture matches the YANG grammar |
| `docs/guide/ipsec.md` | The anchor named `vppUnsupportedSelector`, which no longer exists |
| `plan/journal/plugin-list-hardcoded.md`, `plan/journal/plugin-startup-barrier-deadlock.md` | Two finds along the way |

## Implementation Summary

Implementation Audit, Goal Validation and Review Gate are filled above, where the work
that produced them was written. The sections below complete the closure.

### What Was Implemented

- The four hand-rolled VPP messages are gone. `internal/component/ike/dataplane/vpp.go`
  and the new `vpp_policy.go` build the generated `binapi/ipsec` and `binapi/ipsec_types`
  messages, so every send resolves by name and CRC instead of being refused first.
- `vendor/go.fd.io/govpp/binapi/ipsec` and `.../ipsec_types` are vendored. `go.mod` is
  unchanged: the module was already required, and every transitive dependency of the two
  packages was already vendored.
- The three value bugs are fixed: `vppProto` derives the protocol from `p.Proto` (ESP 50,
  AH reachable), `3des` maps to 11, and the selector is an address range rather than a
  prefix pair.
- The SPD chain exists: `ensureSPD` creates the database, binds it to the interface the
  policy names, and `spdEntry` resolves the SAD id of the SA the policy protects with.
- Each SA states its direction, an AEAD key and its salt reach VPP in their own fields,
  and a tunnel-mode SA carries the RFC 7296 Section 2.24 ECN copy flags.
- `Close` removes what the backend installed, in the order VPP's refcount needs: entries,
  binding, SPD, SAs. A refused request undoes what it created.
- Every shape this backend cannot express is REFUSED rather than approximated: transport
  mode, state selectors, XFRM interface ids, node-wide policies, an unknown algorithm, an
  unset direction, a protect policy naming no SA.
- AC-7's harness: `run_ipsec_evidence` (`scripts/evidence/effective-vpp.py`) runs the
  shipped backend against a real VPP v26.06 in Docker under `make ze-deployment-vpp-test`
  and reads VPP back through `vppctl`.

### Bugs Found/Fixed

Each row of "Findings fixed" above is one, with the test that now covers it. Three finds
outside this spec's boundary are recorded rather than fixed: the plugin startup barrier
deadlock (`plan/journal/plugin-startup-barrier-deadlock.md`), the hardcoded build tags in
the evidence script (`plan/journal/plugin-list-hardcoded.md`, fixed), and the rekey SA
leak (`plan/journal/false-synchronization-claim.md`).

### Documentation Updates

- `docs/features.md`: the VPP Data Plane row now names the IPsec evidence and the four
  limits that qualify it. Anchors: `scripts/evidence/effective-vpp.py -- run_ipsec_evidence`,
  `internal/component/ike/dataplane/vpp.go -- vppBackend InstallSA/Close`,
  `internal/component/ike/dataplane/vpp_policy.go -- vppPolicyInterface refuses IfIndex 0`.
- `docs/guide/ipsec.md`: the stale `vppUnsupportedSelector` anchor is repointed to
  `vppUnsupportedSA` and `vppProtectMode`, and the guide now states that the backend
  cannot be driven by IKE and that no configuration selects it.
- `docs/functional-tests.md`: the IPsec case of `make ze-deployment-vpp-test`, what it
  asserts, and the firewall case that is not green.
- `ai/RFC-REQUIREMENTS.md`: regenerated. `RFC7296-2.24-1` and `-2` gain this backend's
  positive and negative tags.
- `rfc/short/rfc4552.md`: sixteen `ipsec_install.go:NNN` anchors lost their line numbers,
  ten of which this spec's own comment had made stale (see "Recorded, not fixed").
- `feature-gates.txt`: its header enumerates its consumers and did not name
  `scripts/evidence/effective-vpp.py`, which reads it in `feature_tags`.
- `make ze-doc-test` was NOT run by this phase: suite runs are centralised in the main
  thread for this closure. Named in the report as owed.

### Deviations from Plan

- AC-6 changed rather than being met: no config surface selects the IPsec dataplane, so
  there is nothing for `ze config verify` to reject. Homed in
  `plan/future/spec-ipsec-dataplane-selector.md`. See "AC-6 corrected".
- The plan named one file. `vpp.go` crossed the 1000-line limit, so the policy half is
  `vpp_policy.go`.
- The plan listed 5 implementation steps and 10 acceptance criteria. Eight more criteria
  (AC-11 to AC-18) were added as each independent round found a wire-visible defect.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | AC-7 was recorded as unmeetable: "no target in this checkout runs this backend against a real VPP, so the criterion cannot be met by any amount of work inside this spec" | `scripts/evidence/effective-vpp.py` had run a real VPP in privileged Docker since before this spec was written, behind `make ze-deployment-vpp-test` | Reading `mk/test-integration.mk` instead of trusting the spec's own sentence | The IPsec case was added to that harness, and AC-7 is met |
| assumption | Unbinding the interface and deleting the SPD was assumed to release the SA its PROTECT entry names | FALSE, and every request returned retval 0. VPP REFCOUNTS an SA: `show ipsec sa` prints `locks 2` while a PROTECT entry names it | The first run of the AC-16 case against a real VPP failed with `VPP still holds SA spi=0xbadcafe after Close` | `removeInstalled` sends the SPD entries back before the SA deletes |
| approach | `InstallSA` labelled its `AcceptBothESPForms` claim MEASURED, citing five VPP source files | Those files are not in this tree, so no reader here could re-derive the claim | Round 3 independent review | The claim is withdrawn, relabelled unverified, and named its proof. Raised to Thomas as the OWNER QUESTION |
| assumption | AC-6 assumed `ze config verify` could reject a `vpp` dataplane selection | No YANG leaf selects the IPsec dataplane at all; the only selector is the private `ze.test.ike.dataplane` override | Reading `ze-ipsec-conf.yang` and `testport.go` on 2026-08-10 | AC-6 is `Changed`, and the rejection is homed with the leaf it needs |
| escalation | A success return from a foreign system was read as proof the object was gone, twice: once for the SPD/SA order above, once in `RemoveSA` dropping `b.sadIDs` on a retval-0 delete VPP ignored | A foreign system's success code says the request was accepted, never that the state changed | The real-VPP run, then round 5 reading the chain | Recorded as a class in `plan/journal/false-synchronization-claim.md` |

## Deferrals Resolved

The metadata shard is `-`: no `plan/deferrals/fixit-vpp-ipsec-inoperable.md` was ever
created, and `ls plan/deferrals/` confirms none exists. Nothing was deferred through that
mechanism. What this spec did NOT build is homed in a spec, not in a deferral row.

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| (no shard, no rows) | done | `ls plan/deferrals/` holds no `fixit-vpp-ipsec-inoperable.md`; the header row records the correction of 2026-08-03 |
| AC-6's config rejection | deferred | `plan/future/spec-ipsec-dataplane-selector.md` AC-4, with the YANG leaf it asserts against |
| The interface a VPP SPD binds to | deferred | `plan/future/spec-ipsec-vpp-policy-interface.md` AC-1 to AC-4 |
| The rekey policy leak and the retired SA under it | deferred | `plan/future/spec-ipsec-vpp-policy-interface.md` AC-5, and `plan/journal/false-synchronization-claim.md` |
| The plugin startup barrier deadlock in the firewall case | deferred | `plan/journal/plugin-startup-barrier-deadlock.md`; reproduced on a fresh VPP, outside this spec |
| `AcceptBothESPForms` implemented and unproven on this backend | open, owner-owed | The OWNER QUESTION under "RFC Documentation". No `{gap}`, `partial` or `{not-applicable}` may be written until Thomas answers |

## Pre-Commit Verification

### Files Exist (ls)

| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/ike/dataplane/vpp.go` | yes | `ls -l`: 30611 bytes |
| `internal/component/ike/dataplane/vpp_policy.go` | yes | `ls -l`: 23938 bytes (untracked, in the commit list) |
| `internal/component/ike/dataplane/vpp_message_test.go` | yes | `ls -l`: 47076 bytes, 36 `func Test` (untracked) |
| `internal/component/ike/dataplane/vpp_real_integration_test.go` | yes | `ls -l`: 10742 bytes (untracked) |
| `scripts/evidence/effective-vpp.py` | yes | `ls -l`: 52092 bytes |
| `vendor/go.fd.io/govpp/binapi/ipsec/ipsec.ba.go` | yes | `ls -l`: 129202 bytes (untracked) |
| `vendor/go.fd.io/govpp/binapi/ipsec_types/ipsec_types.ba.go` | yes | `ls -l`: 17579 bytes (untracked) |
| `plan/future/spec-ipsec-vpp-policy-interface.md`, `plan/future/spec-ipsec-dataplane-selector.md` | yes | `ls -l`: 8520 and 4499 bytes (untracked) |

### AC Verified (grep/test)

Fresh run, 2026-08-10:
`make ze-test-pkg PKG=./internal/component/ike/dataplane RUN='TestVPPMessageCRCs|...'`
-> `ok github.com/ze-software/ze/internal/component/ike/dataplane 1.140s` (exit 0, `-race`,
`ze_vpp` in the tag set), and
`make ze-test-pkg PKG=./internal/component/ike/engine RUN='TestChildSAStatesCarryTheirDirection|TestChildPolicyParams'`
-> `ok ... 1.127s` (exit 0).

| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | The hand-rolled types are gone | `TestVPPMessageCRCs` in the passing run; `grep -c "^func Test" vpp_message_test.go` = 36 |
| AC-2 | `go.mod` unchanged, `vendor/modules.txt` gains two packages | `git diff go.mod` is empty; `git diff vendor/modules.txt` shows exactly `+go.fd.io/govpp/binapi/ipsec` and `+go.fd.io/govpp/binapi/ipsec_types` |
| AC-3, AC-4, AC-5 | Protocol from `p.Proto`, `3des` is 11, ranges not prefixes | `TestVPPInstallSAProtocol`, `TestVPPCryptoAlg`, `TestVPPRangeIPv4`/`IPv6` in the passing run |
| AC-6 | Changed: no config selection exists | `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang` names no backend leaf; `ikeDataplaneName` (`engine/testport.go`) reads only `ze.test.ike.dataplane` |
| AC-7 | A real VPP holds what this backend installed | `run_ipsec_evidence` on VPP v26.06, output quoted in "AC-7 is MET". NOT re-run by this phase: Docker runs are centralised in the main thread |
| AC-8 to AC-15 | Policy names its SA, mode read, direction flagged, salt split, SPD created and bound, ids do not collide, refusals fail closed | Each test named in the Implementation Audit ran in the passing run above |
| AC-16, AC-17 | Close removes what it installed; a refused request leaves nothing | `TestVPPCloseRemovesWhatItInstalled`, `TestVPPCloseReportsARefusedRemovalAndContinues`, `TestVPPCloseSkipsAPolicyAlreadyRemoved`, `TestVPPCloseWithoutInstallsSendsNothing`, `TestVPPInstallSARefusedForgetsItsSADID`, `TestVPPInstallSARefusedReinstallKeepsTheEarlierID` |
| AC-18 | The ECN copy flags | `TestVPPInstallSACopiesECN`, `TestVPPInstallSAECNIsOnTheSAZeInstalls`, both tagged `RFC7296-2.24-1`/`-2` |

### Wiring Verified (end-to-end)

| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `dataplane.Load("vpp")` then `InstallSA` -> the generated message send path | none: the vehicle is `TestVPPRealDataplaneInstalls` (`vpp_real_integration_test.go`) driven by `run_ipsec_evidence` | Read, not inferred: the probe compiles the `ze_vpp && integration` binary of the dataplane package and runs it inside the VPP container, so the shipped backend is what programs VPP. `requireResolvable` asserts a non-zero message id on every captured send |
| `installChildSA` then `InstallSA` -> `SAParams.Dir` to `vppSAFlags` | none: unit vehicles | `TestChildSAStatesCarryTheirDirection` (engine) and `TestVPPInstallSAInboundFlag` (dataplane), both in the passing runs above |
| The AC-6 row of this table | withdrawn | There is no `ze config verify` entry point for a dataplane selection; moved to `plan/future/spec-ipsec-dataplane-selector.md` with the leaf it needs |

### Assumptions Resolved

| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `git diff go.mod` is empty after `go mod vendor`; `vendor/modules.txt` gained exactly the two package lines |
| A-2 | confirmed | The generated CRCs resolve against a live VPP v26.06: every request in `run_ipsec_evidence` was answered, and `UnknownMsgError` appeared nowhere |
| R-1 | closed | Nothing had ever run this backend against a real VPP. `run_ipsec_evidence` now does, on every `make ze-deployment-vpp-test` |
| R-2 | confirmed, and it fired | A correct layout DID expose further semantic mismatches: the SPD entry modelled the wrong concept, the SAD id is not the SPI, protocol 0 is not any, and an SA is refcounted by the policies naming it |

### Documentation Verified

| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `docs/guide/ipsec.md` "refuses a transport-mode install" | `vppUnsupportedSA` (`vpp.go`) refuses `p.Mode != ModeTunnel`; `vppProtectMode` (`vpp_policy.go`) refuses it on the policy side | yes, both producers read |
| `docs/guide/ipsec.md` "cannot be driven by IKE, and no configuration selects it" | `vppPolicyInterface` refuses `IfIndex` 0; `childPolicyParams` (`engine/child.go`) leaves it 0; `ikeDataplaneName` (`engine/testport.go`) reads only the private override | yes |
| `docs/features.md` "an operator cannot read back or surgically remove what it programmed" | `ListSAs`, `ListPolicies` and the three-argument `RemovePolicy` all return `ErrNotSupported`. No CLI, config or API path reaches `RemoveSA` or `RemovePolicyParams`; the OSPF installer does, and that path is inert on VPP by ORDER (see Release judgment) | yes, and the mechanism is corrected in the two specs engineers read |
| `docs/functional-tests.md` IPsec case description | `run_ipsec_evidence` (`scripts/evidence/effective-vpp.py`) asserts each item the paragraph names | yes |
| `ai/RFC-REQUIREMENTS.md` rows for `RFC7296-2.24-1`/`-2` | Regenerated; both rows now carry `vpp_message_test.go` in both polarities | yes, by diff |
| RFC status page (`docs/features/rfc-status.md`) | No row changes: RFC 7296's row already claims the requirement, and this spec adds a second backend's proof rather than a new claim. The file IS modified in this tree, by another session's RFC 5301 work, so it is NOT in this commit | yes, by reading the diff |
| Doctor checks | This spec adds no runtime dependency: the VPP socket, the only external resource, is the existing `vpp` component's, and its doctor checks already exist | yes |

## Core Insight

**A wire format hand-copied from a foreign system is a claim about that system, and the
only thing that can check it is that system.** Every layer of this defect was invisible
from inside the tree: three unit files agreed with a struct that agreed with nothing, the
CRCs were zeros nobody had ever resolved, and the first real VPP run refuted an assumption
about resource lifetime that every unit test had accepted. The repair is not a better
struct. It is deleting the hand-copy in favour of the generated binding, and then running
the result against the daemon, because the generated binding still cannot tell you that
VPP refcounts an SA or that protocol 0 is hop-by-hop rather than any.
