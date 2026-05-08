# 671 -- fw-6-firewall-vpp

## Context

Ze had a single firewall backend (nft, spec-fw-2) and needed a VPP ACL backend
registered as `firewall { backend vpp }`. The VPP lifecycle component
(spec-vpp-1) provided a shared GoVPP connector. The gap was a VPP firewall
backend registering as `firewall.RegisterBackend("vpp", ...)` that translates
ze Match/Action types to VPP ACL rules. The firewall component lacked the
per-backend verifier infrastructure that traffic gained in fw-7, so that needed
adding too.

## Decisions

- **Per-backend Verifier added to firewall component.** Matching the traffic
  pattern from fw-7: `firewall.RegisterVerifier`/`RunVerifier` wired into
  `parseAndVerifyFirewallSections`. The YANG `ze:backend` gate handles
  leaf-level annotation, the verifier handles per-expression rejection.
- **ACL-only scope, everything else rejected at commit.** VPP's ACL plugin
  covers: src/dst prefix, port range, protocol, ICMP type/code, TCP flags,
  permit/deny/reflect. NAT (NAT44 plugin), classifier (mark matching),
  policer (per-rule rate limiting), packet modification (set-mark, set-dscp),
  counters, log, chain traversal all rejected with specific messages pointing
  at deferred destination specs.
- **MatchConnState maps to PERMIT_REFLECT for established/related only.**
  ConnStateNew and ConnStateInvalid have no VPP ACL equivalent and are
  rejected by the verifier.
- **Read-merge-write ACL bindings to preserve foreign ACLs.** VPP's
  `ACLInterfaceSetACLList` replaces the entire ACL vector. The backend reads
  existing bindings via `ACLInterfaceListDump`, strips ze-owned indexes,
  merges in new ze ACLs, and writes back. This preserves non-ze ACL bindings.
- **Plugin path `internal/plugins/firewall/vpp/`** not `firewallvpp/`.
  Matches codebase convention (`traffic/vpp/`, `firewall/nft/`, `fib/vpp/`).
- **`all.go` generated via `make generate`**, not manually edited. The
  codegen script auto-discovers the new package.

## Consequences

- VPP users can configure `firewall { backend vpp }` with src/dst address,
  port, protocol, ICMP type, TCP flags, and accept/drop verdicts. Unsupported
  expressions reject at commit with clear messages.
- The `firewall.RegisterVerifier` pattern is available for any future backend.
- `ai/rules/testing.md` now documents QEMU testing requirements and mandates
  fakeOps-based tests for all VPP backends.
- Real VPP integration tests (`.ci` with running VPP) remain deferred until
  VPP CI infrastructure is built.

## Gotchas

- **VPP `ACLInterfaceSetACLList` replaces the full list in one call.** Input
  and output ACLs must be merged into a single vector with `nInput` marking
  the boundary. Separate per-direction calls overwrite each other. First
  implementation hit this; caught by review.
- **GoVPP ACL binapi not vendored by default.** Same trap as fw-7's policer
  binapi. Fix: blank-import anchor file (`binapi_imports.go`) then
  `go mod vendor`.
- **Startup orphan cleanup must skip desired ACLs.** First implementation
  deleted ALL ze-tagged ACLs including ones about to be re-created, opening
  a window with no firewall protection. Fixed by building a `desiredACLTags`
  set and skipping matches.
- **`resetBackends()` in tests must also clear the verifiers map.** Adding
  `verifiers` to the same mutex-protected state as `backends` requires
  updating the test reset function.
- **`make generate` replaces `all.go`.** Manual edits are overwritten. The
  codegen script discovers plugin packages automatically.

## Files

- `internal/plugins/firewall/vpp/` (13 files): backend, translation, verifier, ops, tests
- `internal/component/firewall/backend.go`: added Verifier type, RegisterVerifier, RunVerifier
- `internal/component/firewall/backend_test.go`: resetBackends clears verifiers
- `internal/component/firewall/engine.go`: RunVerifier wired into parseAndVerifyFirewallSections
- `internal/component/plugin/all/all.go`: generated, includes firewall/vpp
- `vendor/go.fd.io/govpp/binapi/{acl,acl_types}/`: vendored ACL binapi
- `Makefile`: ze-qemu-integration-test includes firewall/vpp
- `ai/rules/testing.md`: Linux-only tests (QEMU) section, VPP Backend Testing Is Mandatory
- `docs/features.md`: VPP Firewall Backend row
- `docs/guide/plugins.md`: firewall-vpp in Infrastructure table
