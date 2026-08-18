# Deferrals -- spec-ipsec-esp-dual-form-receive

Source: `plan/spec-ipsec-esp-dual-form-receive.md`. Format: `ai/rules/planning.md`.

Created 2026-08-02. The spec's metadata table has named this shard since it was written, and
the file did not exist, so the spec's Goal Gate "Deferral shard resolved: no live row without
a destination" could not be evaluated at all. Nothing was lost: the rows below are the
residuals the spec's own "What is NOT done" section and the 2026-08-02 audit already record.
This file makes them enforceable.

Reference lists in this file are written as bullets, never as tables. Every pipe-delimited
line here is read by the deferral gate as a six-cell row.

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-08-02 | spec-ipsec-esp-dual-form-receive | Write `test/interop-ipsec/scenarios/22-esp-encap-no-nat/`, the strongSwan proof that an encapsulated ESP form is accepted with no NAT present. The spec's Interop table names it and the directory does not exist | Gated by tier, not by difficulty. `test/interop-ipsec/` has no automated caller, so `scripts/dev/rfc_requirements.py` derives it as unrun and the gate refuses an RFC tag placed there. Writing the scenario is useful before that lands, but it earns no compliance evidence until the tree is wired, which is `plan/spec-rfcgate-2-deferred-unrun-interop-trees.md` AC-1 to AC-3 territory for the ipsec tree | `plan/spec-ipsec-esp-dual-form-receive.md` | deferred |  <!-- doc-links: ignore (interop scenario this document plans; it does not exist in the tree) -->
| 2026-08-03 | spec-ipsec-esp-dual-form-receive | Drive a peer through an ESP form change part way through a live Child SA. `test/interop-ipsec/scenarios/23-esp-form-change/` is WRITTEN and PASSING, and it proves the property the change would exercise: a live Child SA whose peer sends the form Ze's kernel state refuses carries traffic both ways and is neither rekeyed nor deleted. The SWITCH itself is not driven | No trigger exists. strongSwan changes a live SA's form only through MOBIKE (`kernel_netlink_ipsec.c` `update_sa`, driven by `ike_mobike.c`), and Ze advertises no MOBIKE_SUPPORTED: `internal/component/ike/wire/payload_notify.go` defines no notify type 16396. Nothing in swanctl exposes a form change either | `plan/spec-ipsec-11-mobike.md` | deferred |
| 2026-08-02 | spec-ipsec-esp-dual-form-receive | Measure the throughput of the re-presented bare ESP form on a templated security association (risk R-1). The bare form is read off a raw socket and re-presented rather than taking the kernel fast path | Not a correctness gap and not a blocker for AC-4. The path is proven functional by `TestEncapOneStateAcceptsBothForms`. What is unmeasured is its cost, and no consumer of this spec depends on the number | `plan/spec-ipsec-esp-dual-form-receive.md` | deferred |

## Not deferred: what stays in the spec, corrected 2026-08-03

Listed rather than tabled, for the reason given above.

- `test/ipsec/ipsec-esp-form-change.ci` and `test/ipsec/ipsec-esp-encap-no-nat.ci` --
  NOT writable at functional tier, and this is now measured rather than pending. The ipsec
  suite runs unprivileged with `ze.test.ike.dataplane=noop`, the `.ci` framework has no ESP
  injector, and two loopback Ze daemons cannot produce the templated Child SA the dual-form
  path needs. `scripts/evidence/qemu-all-tests.sh` also has no `fsuite ipsec` line, so
  `option=needs-linux` would give dead coverage rather than a way round. The spec's
  "What is NOT done" table carries the full reading. AC-4's proof is
  `TestEncapEstablishedSAServesAPeerFormChange` plus interop scenario 23.
- `test/ipsec/ipsec-esp-form-vpp-reject.ci` -- dropped, and it must stay dropped. AC-5 is
  not-applicable by measurement, so a test asserting a refusal would assert behaviour that
  must not exist.
- User story 3: `show vpn ipsec sa` exposes no ESP-form field. Add the field and its test, or
  strike the row from the spec. Either is a decision, not a deferral.
- `ai/RFC-REQUIREMENTS.md` regeneration, owed after the tagged-test rename. Untouched on
  2026-08-03 by direction: the ledger is stale from another session's in-flight restructure.
- The spec's stale line anchors.

## The public claim now has its evidence, and it did not before

`docs/features/rfc-status.md` stated that the platform limit is lifted and that a peer is
served. On 2026-08-03 that sentence was not merely unproven, it was FALSE: Ze's own inbound
Child SA policy made the kernel drop every refused datagram before the receiver could read it
(`net/ipv4/raw.c` `raw_rcv` runs `xfrm4_policy_check`), so the tunnel carried nothing in that
direction. The per-socket bypass in `espFormReceiver.startLocked` fixes it, interop scenario
23 proves it against strongSwan, and the page now cites that evidence.
