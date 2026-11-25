# VPP Firewall Backend

A second firewall backend beside kernel, registered as
`firewall.RegisterBackend("vpp", ...)`, translating ze `Match` and `Action` types
to VPP ACL rules.

## What VPP programs, everything else rejected at commit

<!-- source: internal/plugins/firewall/vpp/verify.go -- Verify, verifyNATChain, verifyNATSourceMatches, verifyAction, verifyClassifyConstraints -->
<!-- source: internal/component/firewall/backend.go -- Verifier, RegisterVerifier, RunVerifier -->

A filter chain lowers to VPP's ACL plugin, which covers source and destination
prefix, port range, protocol, ICMP type and code, TCP flags, and the permit,
deny and reflect verdicts.

Two things beyond the ACL plugin are accepted and programmed:

- **NAT, in a chain of type `nat`, through VPP's NAT44-ED plugin.** `snat`,
  `dnat` and `masquerade` are accepted there, IPv4 only. `snat` adds its
  address range to the NAT44 pool and marks every VPP interface as inside;
  `masquerade` marks every VPP interface as a NAT44 output interface. Both
  apply to the whole inside interface, so a term carrying either with a
  source-address match is refused naming table, chain and term, with the fix to
  delete the match or use `firewall backend kernel`. `dnat` becomes one
  static mapping and needs a protocol match and a destination port match to
  name it; destination and source address matches are accepted with it. A
  `masquerade` with a port mapping or flags, an IPv6 NAT address, a NAT chain
  term with no NAT action, and `destination-local` with `dnat` are refused. A
  NAT action in a filter chain is refused with a message naming the NAT chain
  it belongs in.
- **`set-mark` and `limit`, in a filter chain, through a VPP classify table.**
  `set-mark` sets the session's metadata and `limit` binds the session to a
  policer. `mark-set` carries no `ze:backend` annotation, so operator firewall
  rules reach the VPP verifier directly. The classify key is an exact IPv4
  header match, so a term carrying either is refused when it has a port range
  rather than one port, or an IPv6 address match, and a term carrying both is
  refused.

<!-- source: internal/plugins/firewall/vpp/nat_linux.go -- applyNATChain, applyMasquerade, applySNAT, buildDNATMapping -->
<!-- source: internal/plugins/firewall/vpp/classify_linux.go -- applyClassifyActions, applyTermClassify -->

Everything else rejects at commit with a message naming the unsupported
expression: mark, connection-mark and DSCP matching, the other packet
modifications (`set-dscp`, connection-mark set, TCP MSS), reject, redirect,
notrack, counters, log, named sets, flowtables, and chain traversal (non-base
chains, jump, goto, return). This is exact-or-reject
(`ai/rules/protocol.md`). A conntrack helper (`system conntrack module`, which
lowers to `ct helper` objects and `ct helper set` assignments in the
`ze_conntrack_helpers` tables) is refused too: VPP has no conntrack helper or
ALG, so the helper would never see a connection. `flow-offload` carries no
`ze:backend` annotation of its own, yet stays refused on VPP because the action
must name a flowtable its table declares and the `flowtable` list carries
`ze:backend "kernel"`, which the commit-time walk refuses before the verifier
runs.

The `destination-local` match (`MatchDestinationLocal`, a destination address
this router holds) is refused as well: a VPP ACL rule matches a fixed prefix
and has no lookup of the router's own addresses. The refusal names the kernel
backend, or `destination-address` with the router's addresses written out.

`MatchConnState` maps to `PERMIT_REFLECT` for established and related only.
`connection-state` carries no `ze:backend` annotation, so operator firewall
rules reach `verifyConnState` directly: established and related are accepted,
while `ConnStateNew` and `ConnStateInvalid` have no VPP ACL equivalent and are
rejected.

The firewall component gained the per-backend verifier for this, matching the
traffic component: `firewall.RegisterVerifier` and `RunVerifier` wired into
`parseAndVerifyFirewallSections`. The YANG `ze:backend` gate handles the
leaf-level annotation, and the verifier handles per-expression rejection.

## Startup ordering

The firewall declares `vpp` as an optional dependency, with a config need on
`firewall backend vpp`. Like the `StartAfter` edge it replaced, the edge orders
the firewall after VPP whenever both components are selected, so VPP completes
its startup tier before firewall configuration applies ACLs.
The VPP manager starts from `OnStarted`; putting both components in one tier
would make firewall configuration wait for a connection whose manager cannot
start until that configuration returns.

This ordering does not load VPP for a kernel-only configuration: the need holds
only when the backend leaf is `vpp`, and then dependency resolution pulls the
VPP component even with no `vpp {}` block. A build without `ze_vpp` refuses
`firewall backend vpp` at validate, commit and boot, naming the tag. Connection
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
- **`./le repo update` rewrites `all.go`.** The codegen script discovers the plugin
  package. A manual edit is overwritten.
- The plugin path is `internal/plugins/firewall/vpp/`, not `firewallvpp/`,
  matching `traffic/vpp/`, `firewall/nft/` and `fib/vpp/`.
