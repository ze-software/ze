# 925 -- mpls-rsvp-te-fast-reroute

## Context
Spec `mpls-4-rsvp-te-fast-reroute` implements RSVP-TE Fast Reroute (RFC 4090),
the mpls-3 AC-13 deferral: an LSP survives a link/node failure with sub-second
local repair instead of head-end re-signaling. Scoped to **facility backup**
(Section 3.2, one bypass protects many LSPs by label stacking); **one-to-one
detour backup** (Section 3.1) was split to `spec-mpls-9` with user approval.
Built on the closed mpls-3 engine (PATH/RESV/ERO/RRO/admission/make-before-break).

## Decisions
- **Bypass paths are explicitly configured, not auto-computed.** ze has no
  IGP/CSPF, so a `bypass` config list defines each facility-backup LSP (PLR ->
  merge point along an ERO that avoids the protected resource). The PLR
  auto-associates a configured bypass to a protection-desired transit LSP by
  matching the bypass `merge-point` to the LSP's NHOP (link protection) or NNHOP
  (node protection). Bypass LSPs key into the same `lspTable` via a reserved
  tunnel-id base (`bypassTunnelIDBase = 0xF000`) so they never collide with a
  protected tunnel to the same destination.
- **Local repair is a single in-worker FIB reprogram**, slotted into
  `handleLinkDown` exactly where the base code tore the LSP down: if the matched
  transit LSP has a ready bypass, `tryLocalRepair` programs a 2-label swap and the
  branch `continue`s (skips teardown), sends a Notify PathErr (code 25 value 3),
  and the head-end re-optimizes make-before-break on the Notify (reusing
  `reroute`). No parallel signaling path; the bypass is an ordinary ingress LSP.
- **Node protection needs RRO label recording.** The merge point for node
  protection is the NNHOP, which expects its OWN label, not the NHOP's. So when
  protection is requested the SESSION_ATTRIBUTE sets "label recording desired"
  (0x02); each node records its label after its RRO address subobject (RFC 3209
  4.4.3); the PLR resolves the NNHOP's label from the received RESV RRO
  (`labelForAddr`) into `LSP.BackupLabel`, which `tryLocalRepair` pushes under the
  bypass label. Link protection uses the NHOP's advertised label (`OutLabel`).
- SESSION_ATTRIBUTE (class 207, C-Type 7) was declared but never emitted by base
  RSVP-TE; FRR is the first emitter.

## Consequences
- `mplsfibevents.Entry.OutLabels` already being a slice meant facility backup
  needed NO new data-plane primitive: a swap with `OutLabels:[bypass, protected]`
  flows busFIB -> mpls-fib -> `addMPLSSwap` -> `MPLSDestination{Labels}`. Assumption
  A-1 was confirmed on a live kernel in QEMU.
- The FRR wire codecs + PLR/MP logic live in a dedicated `frr.go`; the `show
  rsvp-te ...` data builders moved to `show_data.go` to keep `register.go` under
  the size limit.
- New counters: `ze_rsvpte_local_repairs_total`, gauges `ze_rsvpte_protected_lsps`
  / `ze_rsvpte_bypass_lsps` (recomputed in the refresh loop + on reconcile).

## Gotchas
- **A plugin engine reading config at RUNTIME must not freeze it at startup.** The
  engine copied `cfg` at construction; `OnConfigApply` reconciled with the fresh
  cfg but never updated `eng.cfg`. Base RSVP-TE got away with it (the engine only
  read cfg.Interfaces/RefreshPeriod, rarely reloaded), but FRR's `selectBypass`
  reads `cfg.Bypasses` on every PATH, so after a reload that added/removed a bypass
  FRR silently stopped arming. Fix: hold the config behind `atomic.Pointer`,
  `setConfig` in OnConfigApply, read via `e.cfg()`. A 4-agent review caught it; no
  test reloaded the engine config (every test set cfg at construction).
- **A setter called only at startup becomes a footgun the first time it runs on reload.** `admission.setInterface` REPLACED the `*interfaceBandwidth` struct (zeroing `ReservedBandwidth`); harmless when called once before any reservation, but the reconcile-on-reload path calls it for every interface on every commit -- so a reload with LSPs up wiped the reserved counter and the link admitted past `MaxReservable` (oversubscription, admission control defeated). Fix: make it read-modify-write (update limits, keep the live reservation). Lesson: when a startup-only mutator gains a reload caller, audit it for state it silently discards. Found only by a deep convergence pass that traced the reconcile diff against LIVE reservations; every test called the setter exactly once.
- **Making a once-immutable field reloadable exposes every runtime read that assumed it valid.** `router-id` was set once at startup; the engine, its keys (`addrToUint32`→`As4()`), and its message encoders all assumed a valid IPv4 Addr. Putting config behind a reloadable atomic pointer meant a reload removing `router-id` (not `mandatory` in YANG) crashed the plugin: `OnStarted` guarded `!RouterID.IsValid()` but `OnConfigApply` did not, and runtime reads (`selectBypass`/`buildPath`) hit `As4()` on the zero Addr. Closed it at three layers: reject a non-IPv4/parse-fail router-id in `parseConfig`; guard `reconcileTunnels` (covers every caller, not just OnStarted); make `setConfig` PRESERVE the router-id (it is the LSR identity — a restart-class value, never adopted at runtime). Lesson: when you make a field reloadable, audit every `As4()`/index/deref that trusted its old immutability, and decide per field whether it is actually reloadable or restart-class.
- **An unbounded wire list that gets re-encoded on relay is a remote-panic vector.**
  `decodeERO` had no hop cap and `encodeERO` no buffer guard (unlike RRO), so a
  crafted transit PATH with ~70 IPv6 ERO hops overflowed the fixed 1500-byte encode
  buffer. The "ERO is bounded" comment asserted a bound nothing enforced. Always
  pair a decode cap with an encode `off+N>len(buf)` guard for relayed objects.
- **A stored key into a reloadable collection must be stable.** Index-derived bypass
  keys re-keyed on a config reorder. Derive the key from a stable identity (a name
  hash here) and detect hash collisions at config time.
- **Build the wire message UNDER the LSP lock, like sendResv/sendPath.** Reading
  `lsp.RSB` after unlock races the refresh goroutine's in-place `LastRefresh` write.
  `-race` missed it because the unit tests never run the refresh loop concurrently.
- **Allocate table-scoped resources OUTSIDE the per-LSP lock** (snapshot under lock,
  release lock, allocate, re-lock to commit) to keep the table->lsp lock order;
  holding lsp.mu across `AllocateLabel` (table mutex) inverts it.
- **SESSION_ATTRIBUTE has two C-Types** (1 LSP_TUNNEL_RA with a 12-byte affinity
  prefix, 7 LSP_TUNNEL without); decode must branch on C-Type or it misreads the
  protection flags from an interop peer.
- **AF_MPLS swaps must use RouteReplace, not RouteAdd.** Local repair re-programs
  the protected LSP's EXISTING swap on the same in-label; `addMPLSSwap` used
  `RouteAdd`, which fails `EEXIST` on a live kernel, so the repair silently
  no-opped. The fake FIB and an isolated-program QEMU test both missed it -- a
  unit test that programs only the backup never exercises the replace. Found in
  review; fixed to `RouteReplace` (the AF_MPLS in-label space is ze's own, unlike
  the shared IP-prefix push path that must guard foreign routes). The QEMU test
  now programs the original swap THEN the backup over the same in-label.
- **A live-kernel swap needs a REACHABLE next hop.** `addMPLSSwap` sets RTA_VIA
  but no LinkIndex; the kernel resolves the output device from the via's IP route,
  so an off-link backup next-hop makes the route install fail. The QEMU test must
  use an on-link backup neighbor.
- **After local repair the protected LSP is correctly torn down by the head-end**
  once it re-optimizes (make-before-break Replaces). So an interop test must check
  the "repaired, retained, in-use" state BEFORE pumping the Notify/reroute to
  convergence -- afterwards the old LSP is legitimately gone.
- The fabric interop harness routes by node address, not by link, and
  `handleLinkDown` needs interfaces: an FRR fabric test gives the PLR two
  interfaces (protected vs bypass subnets) so a link-down matches only the
  protected LSP, and a 4th node (the bypass transit) so the bypass genuinely
  avoids the failed link.
- Same as mpls-3: config booleans arrive as JSON strings ("true"); presence
  containers (`fast-reroute`) arrive as a map; defaults (hop-limit 16) are applied
  in Go, not relied on from YANG.

## Files
- `internal/plugins/rsvpte/frr.go` (new: FAST_REROUTE/SESSION_ATTRIBUTE codecs,
  protectionRequest, selectBypass, tryLocalRepair, reoptimizeOnNotify,
  rroProtectionFlags, bypassKey, updateFRRGauges), `frr_test.go` (new)
- `engine.go` (PLR arming in handlePathTransit; local-repair branch + Notify in
  handleLinkDown; BackupLabel/label-recording in handleResvTransit; Notify
  re-optimization in handlePathErr; recordRoute self-flags + label; labelForAddr)
- `wire.go` (class/flag constants, ParsedMessage FastReroute/SessionAttr decode),
  `build.go` (buildPath FAST_REROUTE/SESSION_ATTRIBUTE; Notify constants; dropped
  the dead buildPathErr ttl param), `fib.go` (ProgramBackup), `fsm.go` (LSP FRR
  fields + PSB.Protection), `register.go` (fast-reroute/bypass parse, setupBypass,
  reconcile, metrics), `show_data.go` (new: show builders incl. showFastReroute),
  `cmd_show.go` (show rsvp-te fast-reroute proxy), `yang/ze-rsvp-te-conf.yang`
- `internal/plugins/fib/kernel/mplsentry_linux.go` (RouteReplace fix),
  `mplsentry_integration_linux_test.go` (live-kernel 2-label swap-then-replace)
- `rfc/short/rfc4090.md` (new), `internal/plugins/rsvpte/interop_test.go`
  (4-node FRR local-repair), `test/rsvpte/rsvpte-frr.ci` (new),
  `internal/test/cli/register.go` (suite description)
- Docs: `docs/guide/rsvp-te.md`, `docs/guide/configuration.md`,
  `docs/guide/command-reference.md`, `docs/features.md`
- Follow-up: `plan/spec-mpls-9-rsvp-te-one-to-one-backup.md` (one-to-one detour)
