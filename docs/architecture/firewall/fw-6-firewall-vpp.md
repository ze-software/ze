# VPP Firewall Backend

A second firewall backend beside nft, registered as
`firewall.RegisterBackend("vpp", ...)`, translating ze `Match` and `Action` types
to VPP ACL rules.

## ACL-only scope, everything else rejected at commit

<!-- source: internal/plugins/firewall/vpp/verify.go -- backend verifier -->
<!-- source: internal/component/firewall/backend.go -- Verifier, RegisterVerifier, RunVerifier -->

VPP's ACL plugin covers source and destination prefix, port range, protocol,
ICMP type and code, TCP flags, and the permit, deny and reflect verdicts.

Everything else rejects at commit with a message naming the unsupported
expression: NAT (a separate NAT44 plugin), the classifier (mark matching), the
policer (per-rule rate limiting), packet modification (set-mark, set-dscp),
counters, log, and chain traversal. This is exact-or-reject
(`ai/rules/protocol.md`).

`MatchConnState` maps to `PERMIT_REFLECT` for established and related only.
`ConnStateNew` and `ConnStateInvalid` have no VPP ACL equivalent and are
rejected.

The firewall component gained the per-backend verifier for this, matching the
traffic component: `firewall.RegisterVerifier` and `RunVerifier` wired into
`parseAndVerifyFirewallSections`. The YANG `ze:backend` gate handles the
leaf-level annotation, and the verifier handles per-expression rejection.

## Read-merge-write ACL bindings

<!-- source: internal/plugins/firewall/vpp/backend_linux.go -- ACL binding merge, orphan cleanup -->

`ACLInterfaceSetACLList` REPLACES the entire ACL vector on an interface. The
backend therefore reads the existing bindings with `ACLInterfaceListDump`, strips
the ze-owned indexes, merges in the new ze ACLs, and writes back. That preserves
an ACL some other system bound to the same interface.

Input and output ACLs go in ONE vector with `nInput` marking the boundary.
Separate per-direction calls overwrite each other. The first implementation made
that mistake and review caught it.

## Startup orphan cleanup skips the desired set

The first implementation deleted every ze-tagged ACL, including the ones about to
be recreated, which opened a window with no firewall protection. Cleanup builds a
`desiredACLTags` set and skips a match.

## Traps

<!-- source: internal/plugins/firewall/vpp/binapi_imports.go -- blank-import anchor -->

- **The GoVPP ACL binapi is not vendored by default.** Same trap as the traffic
  backend's policer binapi. The fix is a blank-import anchor file and then
  `go mod vendor`.
- **`resetBackends()` in tests must clear the verifiers map too.** Adding
  `verifiers` to the same mutex-protected state as `backends` means the test
  reset has to cover both.
- **`./le repository generate` rewrites `all.go`.** The codegen script discovers the plugin
  package. A manual edit is overwritten.
- The plugin path is `internal/plugins/firewall/vpp/`, not `firewallvpp/`,
  matching `traffic/vpp/`, `firewall/nft/` and `fib/vpp/`.
