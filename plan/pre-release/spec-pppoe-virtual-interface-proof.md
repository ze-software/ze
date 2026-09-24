# Spec: pppoe-virtual-interface-proof

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-21 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**Both PPPoE roles allocate the PPP virtual interface after PADS, and no tagged
test observes the `ppp<N>` unit exist on either side.** The AC allocates it in
`internal/component/l2tp/pppoe/server.go::handlePADR` (`pppoeCreate` then
`ppp.DevPPPSetup`); the Host in `internal/component/l2tp/pppoeclient/dialer.go::Dial`
(`PPPoECreate` then `DevPPPSetup`). The allocation is AF_PPPOX plus `/dev/ppp` ioctls,
which run only under QEMU: `socket_other.go` returns an error off Linux, so no unit
seam reaches it. The proof is an integration-tier tag in the existing harness:
`internal/component/l2tp/pppoeclient/dialer_integration_linux_test.go` drives the
client and `internal/le/qemu/pppoe_accel_linux.go` boots the AC, so the test reads the
`ppp<N>` interface from netlink on both guests after PADS and asserts it is absent
before PADI. One record per polarity through `./le rfc discriminate-record`.

### Requirements this spec covers

| Id | RFC sentence | Producer or absence |
|----|--------------|---------------------|
| RFC2516-3-1 | "Once a PPP session is established, both the Host and the Access Concentrator MUST allocate the resources for a PPP virtual interface." (Section 3) | implemented on both roles; the integration-tier tag is what is missing |
