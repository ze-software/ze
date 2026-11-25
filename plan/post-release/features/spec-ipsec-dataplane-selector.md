# Spec: ipsec-dataplane-selector

| Field | Value |
|-------|-------|
| Status | skeleton |
| Priority | P3 |
| Release reason | Adds a capability absent from the supported release; existing supported behavior and its obligations remain required. Thomas brought this spec into the first release on 2026-11-20 (his decision 2) and reversed that the same day by ruling "plan/release/fixes/spec-ipsec-vpp-policy-interface.md will be done after the release": this selector depends on that spec and on the real-VPP ESP proof, so it returns to post-release with them. VPP IPsec is not a first-release feature: until this selector lands no operator can select VPP (it is reachable only through the private test override), so the XFRM-only release stands. |
| Scope | config |
| Depends | spec-ipsec-vpp-policy-interface; spec-ipsec-vpp-dual-form-proof (real-VPP ESP proof) |
| Phase | - |
| Updated | 2026-11-20 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

Moved on 2026-11-20 from `plan/post-release/features/spec-ipsec-dataplane-selector.md`
into `plan/release/fixes/`, by Thomas's decision 2 of that day, and returned here
the same day by his ruling "plan/release/fixes/spec-ipsec-vpp-policy-interface.md
will be done after the release". Status stays `skeleton`: neither ruling designs
anything.

## Task

**An operator cannot choose which IPsec dataplane backend Ze programs. Give the
choice a YANG leaf, and validate it at `ze config verify`.**

Two backends are registered today, `xfrm` and `vpp`, plus `noop` for tests
(`internal/component/ike/dataplane/register.go`, `register_vpp.go`). Nothing an
operator writes reaches `dataplane.Load`. `ikeDataplaneName`
(`internal/component/ike/engine/testport.go`) returns `"xfrm"` unless the private
env var `ze.test.ike.dataplane` is set, and that entry is registered
`Private: true` and described as test infrastructure. `ze-ipsec-conf.yang`
(`internal/component/ike/ipsec/yang/`) has no leaf for it, so `ze config verify`
has nothing to check.

## Why it waits for after the first release

Thomas made VPP IPsec a first-release feature on 2026-11-20 and reversed that the
same day: the VPP policy-interface spec "will be done after the release", and this
selector cannot land before it (AC-4). VPP IPsec is therefore not a first-release
feature, and the release ships XFRM only. The default stays `xfrm`, the production
path. The `vpp` backend sits behind the `ze_vpp` build tag
(`register_vpp.go`), which the default daemon build carries
(`feature-gates.txt`, the `ze_vpp` comment: dropping the tag unlinks GoVPP for
kernel-only deployments). The earlier post-release text said a default build does
not carry it; that was wrong and was corrected on the move into release scope.

## Required Reading

- [ ] `ai/rules/config.md` - the YANG-versus-env-var decision, and the naming rules
- [ ] `ai/patterns/config-option.md` - the structural template for a new leaf and its validator
- [ ] `ai/rules/protocol.md` - a backend that cannot apply the config exactly must reject at verify
- [ ] `plan/post-release/features/spec-ipsec-vpp-policy-interface.md` and `plan/post-release/features/spec-ipsec-vpp-dual-form-proof.md` - live prerequisites: IKE-produced policies, rekey cleanup, inbound cleartext on VPP (its AC-8, C-4 decided "B", the VPP patch 0106, proven on real VPP 2026-11-20), and real-VPP ESP receive evidence

## Current Behavior

**Source files read:** (2026-11-20)
- [ ] `internal/component/ike/engine/testport.go` - `ikeDataplaneName` and the private `ze.test.ike.dataplane` override
- [ ] `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang` - no dataplane leaf

## Data Flow

### Entry Point
- Config: a leaf under `vpn ipsec`, to be designed

### Transformation Path
1. To be designed: the leaf reaches `ikeDataplaneName`, which `runEngine` passes to `dataplane.Load`

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config to IKE engine | to be designed | No |

### Integration Points
- `ikeDataplaneName` (`engine/testport.go`), `dataplane.Load`

## Wiring Test

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| the dataplane leaf under `vpn ipsec` | → | `ikeDataplaneName`, `dataplane.Load` | a `.ci` in `test/ipsec/` (AC-5) |

## Acceptance Criteria

| Id | Criterion |
|----|-----------|
| AC-1 | A YANG leaf under `vpn ipsec` names the dataplane backend, with an `enumeration` covering the backends the build carries |
| AC-2 | `ikeDataplaneName` reads that leaf, and `ze.test.ike.dataplane` stays a private test override that outranks it |
| AC-3 | `ze config verify` rejects a backend name the running build does not carry, naming the build tag that would carry it |
| AC-4 | The selector does not land until `plan/post-release/features/spec-ipsec-vpp-policy-interface.md` AC-1..AC-5 are demonstrated against real VPP and `plan/post-release/features/spec-ipsec-vpp-dual-form-proof.md` supplies its real-VPP ESP-on-wire harness and inherited AC-5 measurement: both ESP forms decrypt on one inbound SA. SA install/read-back alone clears neither prerequisite. Until then VPP remains accessible only through the private test override |
| AC-5 | A `.ci` in `test/ipsec/` asserts each rejection |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| to be named at design | `internal/component/ike/engine/` | AC-2, AC-3 | not written |

## Files to Modify
- `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang` - the leaf (AC-1)
- `internal/component/ike/engine/testport.go` - `ikeDataplaneName` reads it (AC-2)

## Implementation Steps

1. Design the leaf and its verify refusal; present it
2. Implement after both prerequisites meet AC-4

## DO NOT LAND THIS UNTIL TWO THINGS EXIST

The round-5 independent review of `spec-fixit-vpp-ipsec-inoperable` re-ran the
real VPP and ruled that the backend is closable as a defect fix and NOT shippable as a
feature. The main thread endorsed it. Its "Release judgment" section carries the
evidence; the gate it sets is:

Land this selector only after BOTH `plan/post-release/features/spec-ipsec-vpp-policy-interface.md`
and the real-VPP ESP proof owned by `plan/post-release/features/spec-ipsec-vpp-dual-form-proof.md`
meet AC-4 above. That child inherits the 2026-09-05 ruling preserved in
`plan/release/validation/spec-ipsec-esp-dual-form-receive.md`: the harness extension
and dual-form decryption measurement remain required; this selector consumes
that evidence. The split does not relax either prerequisite, and neither did the
brief placement of all three specs in the first release.

The closure review recorded why exposing the backend would make these findings
an operator's problem: the find rate had not fallen (six
wire-visible defects, each found only when somebody looked), zero packets have ever
crossed the backend, it installs no policy IKE produces, and `ListSAs`, `ListPolicies`
and the three-argument `RemovePolicy` all refuse, so an operator cannot read back what it
programmed. `RemoveSA` and `RemovePolicyParams` do each remove one object. The IKE engine
calls both, and so does the OSPF installer (`removeLocked`, `rollback`,
`plugins/ospf/ipsec_install.go`), which operator OSPF config drives. On VPP that path is
inert for a reason of ORDER, not of reach: `installLocked` installs the SA first,
`vppUnsupportedSA` refuses it for its unset `Dir`, so `i.installed` records nothing and
both removals have nothing to send. **Widen that refusal and the OSPF removal path goes
live**, which is one more reason this selector waits. So `Close` is the only removal an
operator can cause today, and it removes everything.
Until then `vpp` stays behind the private `ze.test.ike.dataplane` override.

## Known Limitations

The predecessor `spec-fixit-vpp-ipsec-inoperable` closed on 2026-08-10 after a
real VPP accepted installed SAs. That fulfilled prerequisite is historical
context, and its AC-7 is no longer a landing gate here. It supplied no ESP
traffic evidence and did not supply the interface an IKE-produced policy needs.
The two live prerequisites in Depends and AC-4 remain mandatory.

## Checklist

- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] `./le test unit pkg path ./internal/component/ike/engine/` and `./le test functional ipsec`, run by the main thread
