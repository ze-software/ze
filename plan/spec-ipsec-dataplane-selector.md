# Spec: ipsec-dataplane-selector

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | config |
| Depends | spec-ipsec-vpp-policy-interface; spec-ipsec-esp-dual-form-receive (real-VPP ESP proof) |
| Phase | - |
| Updated | 2026-08-10 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

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

This is a feature, not a defect. Ze applies exactly what the operator configured
today, because the operator can configure nothing here.

## Why it is not urgent

The default is the backend that works. `xfrm` is the production path, and the
`vpp` backend is behind the `ze_vpp` build tag, so a default build does not carry
it. An operator who wants VPP has no way to ask for it, which is a missing
feature rather than a wrong answer.

## Required Reading

| Document | Why |
|----------|-----|
| `ai/rules/config.md` | The YANG-versus-env-var decision, and the naming rules |
| `ai/patterns/config-option.md` | The structural template for a new leaf and its validator |
| `ai/rules/protocol.md` | A backend that cannot apply the config exactly must reject at verify |
| `plan/spec-ipsec-vpp-policy-interface.md` and `plan/spec-ipsec-esp-dual-form-receive.md` | Live prerequisites: IKE-produced policies, rekey cleanup, and real-VPP ESP receive evidence |

## Acceptance Criteria

| Id | Criterion |
|----|-----------|
| AC-1 | A YANG leaf under `vpn ipsec` names the dataplane backend, with an `enumeration` covering the backends the build carries |
| AC-2 | `ikeDataplaneName` reads that leaf, and `ze.test.ike.dataplane` stays a private test override that outranks it |
| AC-3 | `ze config verify` rejects a backend name the running build does not carry, naming the build tag that would carry it |
| AC-4 | The selector does not land until `plan/spec-ipsec-vpp-policy-interface.md` AC-1..AC-5 are demonstrated against real VPP and `plan/spec-ipsec-esp-dual-form-receive.md` supplies its real-VPP ESP-on-wire harness and AC-5 measurement: both ESP forms decrypt on one inbound SA. SA install/read-back alone clears neither prerequisite. Until then VPP remains accessible only through the private test override |
| AC-5 | A `.ci` in `test/ipsec/` asserts each rejection |

## DO NOT LAND THIS UNTIL TWO THINGS EXIST

The round-5 independent review of `spec-fixit-vpp-ipsec-inoperable` re-ran the
real VPP and ruled that the backend is closable as a defect fix and NOT shippable as a
feature. The main thread endorsed it. Its "Release judgment" section carries the
evidence; the gate it sets is:

Land this selector only after BOTH `plan/spec-ipsec-vpp-policy-interface.md`
and the real-VPP ESP proof owned by `plan/spec-ipsec-esp-dual-form-receive.md`
meet AC-4 above. The latter spec's 2026-09-05 ruling owns the harness extension
and dual-form decryption measurement; this selector consumes that evidence.

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
