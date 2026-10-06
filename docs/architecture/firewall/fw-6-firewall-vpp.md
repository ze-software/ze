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

## Startup ordering

The firewall declares `StartAfter: ["vpp"]`. When both components are selected,
VPP completes its startup tier before firewall configuration applies ACLs.
The VPP manager starts from `OnStarted`; putting both components in one tier
would make firewall configuration wait for a connection whose manager cannot
start until that configuration returns.

This ordering does not load VPP for an nft-only configuration. Connection
failure still refuses the VPP firewall apply; no timeout or readiness check is
relaxed.
<!-- source: internal/component/firewall/register.go -- init -->
<!-- source: internal/component/vpp/register.go -- runVPPEngine -->

## Read-merge-write ACL bindings

<!-- source: internal/plugins/firewall/vpp/backend_linux.go -- ACL binding merge, orphan cleanup -->

The configuration parser prefixes table names with `ze_` before handing them
to a backend. VPP then builds its ACL tag as `ze/<table>/<chain>`: configuration
table `wan`, chain `input`, produces `ze/ze_wan/input`. Native readback checks
use that normalized identity for installation, restart and orphan removal.
<!-- source: internal/component/firewall/config.go -- parseTable -->
<!-- source: internal/plugins/firewall/vpp/verify.go -- aclTag -->
<!-- source: internal/le/test/deployment/vppevidencescenarios.go -- VPPFirewallACLTag -->

`ACLInterfaceSetACLList` REPLACES the entire ACL vector on an interface. The
backend therefore reads the existing bindings with `ACLInterfaceListDump`, strips
the ze-owned indexes, merges in the new ze ACLs, and writes back. That preserves
an ACL some other system bound to the same interface.

Input and output ACLs go in ONE vector with `nInput` marking the boundary.
Separate per-direction calls overwrite each other. The first implementation made
that mistake and review caught it.

The API's one-octet count represents at most 255 ACL indexes. The merge currently
lacks an oversized-vector refusal, so preservation above that limit is not
established; the pre-existing narrowing defect is recorded in
[`bound-wraps-before-it-refuses`](../../../plan/journal/bound-wraps-before-it-refuses.md).

## Startup adoption and orphan cleanup

Every reconciliation discovers the live ACL tags and their actual VPP indexes.
For each desired owned tag, the backend adopts the lowest existing index and
updates that ACL in place. A same-configuration daemon restart therefore preserves
the ACL identity instead of creating another ACL with the same tag.

The complete discovered owned-index set includes stale tags and legacy duplicate
indexes. The backend reads every live interface's ACL vector, removes those
owned indexes, and merges the desired input and output ACLs in one write.
Foreign ACL order and the input/output boundary are preserved. Only after all
interface writes succeed does it delete detached stale and duplicate ACLs.
This also runs for an empty desired configuration after a daemon restart.

Discovery, binding and deletion failures fail the apply rather than reporting
successful cleanup. An ACL created before an ACL-programming failure is removed
if rollback succeeds; rollback errors remain visible. After a binding failure,
created ACLs remain owned because an earlier interface write may have attached
them; a failed or timed-out write does not prove that VPP left the interface
unchanged. Reconciliation is not an atomic transaction: earlier rule replacements
and interface writes may remain after an error. The next reconciliation
rediscovers live state rather than trusting a successful-apply cache. Desired
ACLs are never deleted and recreated merely to recover their identity.

Native readback requires exactly one occurrence of the owned tag after install
and after the restarted daemon reports its configuration applied. Cleanup still
requires the tag to be absent.

## Traps

<!-- source: internal/plugins/firewall/vpp/binapi_imports.go -- blank-import anchor -->

- **The GoVPP ACL binapi is not vendored by default.** Same trap as the traffic
  backend's policer binapi. The fix is a blank-import anchor file and then
  `go mod vendor`.
- **`resetBackends()` in tests must clear the verifiers map too.** Adding
  `verifiers` to the same mutex-protected state as `backends` means the test
  reset has to cover both.
- **`./le repo generate` rewrites `all.go`.** The codegen script discovers the plugin
  package. A manual edit is overwritten.
- The plugin path is `internal/plugins/firewall/vpp/`, not `firewallvpp/`,
  matching `traffic/vpp/`, `firewall/nft/` and `fib/vpp/`.
