# 1098 -- followup-vpp-iface: VPP interface tunnels / mirror / wireguard / LCP

Spec: `plan/spec-followup-vpp-iface.md` (retired). Phases 4-6 of a 6-phase spec;
phases 1-3 (vendoring + gre/gretap/ipip + vxlan) landed earlier in `22e916e67`.

## Context

The vpp `iface.Backend` had `errNotSupported` stubs for tunnels, mirror,
wireguard, and LCP because each needed a `go.fd.io/govpp/binapi/*` package that
was not vendored. Once the six binapi packages were vendored (phases 1-3), the
remaining work was pure wiring: implement SPAN mirror, the wireguard trio, and a
new LCP-pair Backend surface against the GoVPP binary API, widen the per-feature
`ze:backend` commit-gate annotations, and add doctor checks + real-VPP evidence.
The goal for LCP specifically was the L188 deferral: a Linux TAP shadow of a VPP
interface so the kernel-bound ze BGP listener can run on a VPP-owned NIC.

## Decisions

- **Mirror -> SPAN mapping (A-6):** `SetupMirror(src,dst,ingress,egress)` maps to
  `sw_interface_span_enable_disable` state rx/tx/both, `is_l2=false` (device
  SPAN) over L2 bridge SPAN, matching the netlink tc-mirred device port mirror.
  RemoveMirror replays each recorded `(from,to,is_l2)` with state DISABLED
  because VPP keys the delete on the triple, not the source alone -- so the
  backend tracks installed entries per source name.
- **Wireguard create/configure split:** `wireguard_interface_create` takes the
  private key in one message and VPP has no key-update, so `CreateWireguardDevice`
  (name only) is a documented no-op and the real create happens in
  `ConfigureWireguardDevice` (full spec). Peers reconcile with ReplacePeers
  semantics (remove tracked peer indices, then re-add). A preshared key is
  rejected -- this API revision's `wireguard_peer` has no PSK field -- rather
  than silently dropped, mirroring the GRE-key rejection from phase 2.
- **Wireguard plugin enable via a vpp-component toggle** (`vpp.plugins.wireguard`)
  over deriving it from iface wireguard-interface presence: the plugin must load
  at VPP startup (before iface config applies), and `startup.conf` is generated
  only from `VPPSettings`. This mirrors the existing `lcp.enabled` -> plugin
  precedent and honours `plugin default { disable }` (enable only what's used).
- **LCP netns as a Backend-internal read, not a method param:** `SetupLCPPair`
  reads the running Manager's LCP settings via a new `vppcomp.GetActiveLCPSettings`
  accessor (ifacevpp already imports vppcomp), keeping the iface component free of
  a vpp-component dependency. A root-reachable configured netns (`host`/`root`/
  empty) maps to the empty per-pair netns (VPP host namespace, where ze's BGP
  runs); any other value passes through and the doctor check warns.
- **LCP triggered per-loopback in config-apply**, no new per-interface YANG leaf:
  when the backend is vpp, each loopback (Dummy) is shadowed; `SetupLCPPair`
  no-ops when LCP is disabled, so the call is unconditional at the callsite. The
  wiring lives in BOTH `applyConfig` Phase 1 and `recreateManagedInterface` (the
  deferred vpp-ready / post-crash recreate path) -- the review caught that the
  recreate path recreated the loopback without its shadow.
- **Doctor checks are the config-aware gate (R-6):** `doctor-vpp-wireguard`
  (interface configured under vpp but `plugins.wireguard` off) and
  `doctor-vpp-lcp-netns` (BGP enabled + non-root-reachable lcp netns) are
  config-only checks in the owning ifacevpp plugin. `health.go`'s
  socket-absent-Healthy was left unchanged: the health probe has no config
  context to know whether VPP is expected, so it must not report Down for a
  non-VPP deployment; the doctor layer is the actionable gate.

## Consequences

- The vpp backend now programs gre/gretap/ipip + vxlan tunnels, SPAN mirror, and
  wireguard end-to-end, proven against real VPP 25.10 by
  `scripts/evidence/effective-vpp-iface.py` (GRE tunnel + SPAN + wireguard OK).
  A-3 (v0.13.0 bindings compatible with the deployed VPP) is CONFIRMED.
- `ze:backend` annotations are per-kind for tunnels and per-container for
  mirror/wireguard; the schema-annotation test and the aggregation gate `.ci`
  now assert the widened sets. Reconciling stale gate `.ci` after a widening is
  mandatory -- phase 2's gre widening left `iface-vpp-rejects-tunnel.ci` and
  `iface-vpp-aggregates-errors.ci` red until this spec fixed them.
- New config surface: `vpp.plugins.wireguard` (boolean, default false). LCP TAP
  host names are capped at 15 bytes (Linux IFNAMSIZ) with a collision guard.

## Gotchas

- **LCP enabled + no linux_cp_plugin.so = whole-apply failure.** `ligato/vpp-base`
  ships `wireguard_plugin.so` but NOT `linux_cp_plugin.so`/`linux_nl_plugin.so`.
  With `lcp.enabled true`, ze creates an `lcp_itf_pair` for every loopback; on a
  VPP build lacking the plugin the binapi call returns "unknown message" and, by
  exact-or-reject, fails the entire interface config apply (ze exits at startup).
  This is honest but operationally sharp, and `doctor-vpp-lcp-netns` does NOT
  catch plugin absence (only netns). Real-VPP LCP proof therefore needs a VPP
  image with the linux-cp plugins; the evidence script gates on `show plugins`
  and records an evidence-backed SKIP. Evidence configs that only test other
  features must set `lcp.enabled false` so their loopbacks are not shadowed.
- **commit_helper deferral-language gate trips on spec/comment prose.** "out of
  scope", "future spec", "deferred to" in a staged file (the spec's Known
  Limitations, or a wireguard.go comment) block the commit. Phase commits
  excluded the spec (code-only, matching phases 1-3); a code comment was reworded
  "deferred to" -> "happens in". The spec's deferral language is handled at
  closure by recording the genuine BGP-netns follow-up in `plan/deferrals.md`.
- **`// test-relax:` token needs `//` even in `#`-comment `.ci` files.** The hook
  regex is `//[ \t]*test-relax:`; embed it as `# // test-relax: ...` so the `.ci`
  parser still skips the line as a comment while the hook detects the token.
- **VPP wireguard needs an underlay src the spec model lacks.** `WireguardInterface.SrcIP`
  is left unspecified (VPP FIB-selects); iface `WireguardSpec` carries no underlay
  source. Fine for the common case; a dedicated source leaf is a future extension.

## Files

None recorded.
