# 1038 - OSPFv3 IPsec AH/ESP authentication (RFC 4552)

## Context

OSPFv3 manual-keyed IPsec AH/ESP as a distinct auth path from the delivered
in-packet RFC 7166 Authentication Trailer. Kernel XFRM SA/SP policy wired onto
the OSPFv3 IPv6 transport, with per-interface SPI/key config. Linux-only; the
kernel path CANNOT be validated on darwin (QEMU/CAP_NET_ADMIN required) - unit,
config, and doctor paths are exercised, the kernel behavior is not.

## Decisions

- ONE shared wildcard transport-mode SA per interface (Src=::, Dst=::) bound by reqid, with the OSPF proto-89 traffic selector (state.Sel = {::/0, ::/0, proto 89}), used for BOTH egress protection and ingress verification (RFC 4552 §7 shared SA/SPI/key). Two identical wildcard states with the same SPI would collide (EEXIST), so it must be a single shared SA, not the naive one-in/one-out pair.
- Interface scoping via the POLICY SELECTOR ifindex (`SPParams.IfIndex` -> XfrmPolicy.Ifindex -> sel.ifindex), NOT `IfID` (XFRMA_IF_ID). IfID needs an xfrm-interface device; regular packets carry if_id=0, so setting it to an ifindex would match NO packets and silently disable IPsec.
- Three interface-scoped proto-89 policies (out/in/fwd), Src=Dst=::/0.
- IPsec installer hooks into ext-15's per-AF v6EngineSet.spawn (one installer per v6 engine), not a single eng6; metric registration is name-idempotent so per-engine registration is safe.
- Install the SA/policy BEFORE the interface FSM starts (outbound protected before the first Hello).

## Consequences

- The shared IKE dataplane gained `SAParams.Sel *SASelector`, `SPParams.UpperProto`, `SPParams.IfIndex`, `Dataplane.RemovePolicyParams`, ProtoESP/AH + Mode consts, and `planStateAlgos` (AEAD / AH-auth-only / ESP-crypt+auth). All additive and zero-valued for IKE (Sel nil, IfIndex 0), so IKE is byte-identical - verified by the full IKE suite staying green.
- Multiple IPsec interfaces on one node require DISTINCT per-interface SPIs (the shared wildcard state's identity is (::, spi, proto)).
- A new `ospfv3` functional-test suite (`internal/test/cli/register.go` + `mk/test-functional.mk`).

## Gotchas

- The ORIGINAL implementation shipped a broken 2-SA model (out dst=ff02::5, in dst=link-local, no traffic selector) that could not carry unicast DBD / ff02::6 / unicast LSU-retransmit, so the adjacency stalled in ExStart/Exchange and never reached Full. Only caught by review because the interop tests were log_skip no-ops (unvalidated Linux feature). Fixed to the wildcard-SA + proto-89-selector model above.
- Residual startup-window gap (socket opens + joins ff02::5 before the inbound require-policy installs in onUp) is DOCUMENTED (NOTE), not closed - closing needs splitting the shared engine's onUp into pre-join/post-open hooks (the v4 engine has no IPsec).
- `xfrmEncName("null")` returns "ecb(cipher_null)"; some kernels expect "cipher_null" - NOTE to verify on the target kernel.
- Diagnostic code `doctor-ospfv3-ipsec` added to a `codes.go` that another session had dirty; add only the one entry, reconcile.

## Files

- `internal/plugins/ospf/{config_ipsec,ipsec_install,ipsec_metrics,doctor_ipsec,doctor_ipsec_linux,doctor_ipsec_other,ipsec_drops_linux,ipsec_drops_other}.go` (+ tests, ipsec_integration_linux_test.go)
- `internal/component/ike/dataplane/{dataplane,xfrm_linux,xfrm_other,vpp}.go` (shared dataplane, IKE-neutral additions)
- `internal/plugins/ospf/{config,instance,register,cmd_show}.go`, `register_multiaf.go` (per-v6-engine installer), `v3/transport/transport.go` (InterfaceSource)
- `internal/core/diagnostic/codes.go`, `internal/test/cli/register.go`, `mk/test-functional.mk`
- `internal/plugins/ospf/yang/ze-ospf-{conf,cmd}.yang`
- `test/ospfv3/ospf-ipsec-*.ci`, `test/interop/scenarios/ospf-ipsec-{frr,ah-frr}/`
- `rfc/short/rfc4552.md`, `docs/guide/ospf.md` (doc reconciliation into main PENDING)
