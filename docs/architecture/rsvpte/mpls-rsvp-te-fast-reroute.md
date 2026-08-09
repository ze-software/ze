# RSVP-TE Fast Reroute

Ze implements RFC 4090 **facility backup** (section 3.2): one bypass LSP protects
many protected LSPs by label stacking, and a link or node failure is repaired
locally instead of by head-end re-signaling. One-to-one detour backup (section
3.1) is not implemented.

The codecs and the point-of-local-repair logic live in one file on top of the
base engine documented in [`mpls-rsvp-te.md`](mpls-rsvp-te.md).

<!-- source: internal/plugins/rsvpte/frr.go -- encodeFastReroute, encodeSessionAttr, protectionRequest, selectBypass, tryLocalRepair -->

## Decision: bypass paths are configured, not computed

Ze has no IGP and no CSPF, so a `bypass` config list defines each facility-backup
LSP as an explicit route from the point of local repair to a merge point that
avoids the protected resource.

The point of local repair associates a configured bypass to a protection-desired
transit LSP by matching the bypass merge point to the LSP's next hop (link
protection) or next-next hop (node protection).

Bypass LSPs key into the same LSP table as protected tunnels. The top 4096 tunnel
IDs are reserved for them (`bypassTunnelIDBase = 0xF000`), so a bypass can never
collide with a protected tunnel to the same destination.

## Decision: local repair is one FIB reprogram in the existing worker

`tryLocalRepair` is slotted into `handleLinkDown` exactly where the base engine
tore the LSP down. When the matched transit LSP has a ready bypass, the repair
programs a two-label swap, skips the teardown, and sends a Notify PathErr (code
25, value 3). The head-end re-optimizes make-before-break on the Notify, reusing
the base reroute path. There is no parallel signaling path: the bypass is an
ordinary ingress LSP.

The dataplane needed no new primitive. The MPLS FIB entry already carried a label
**slice**, so a swap with two out-labels flows through the existing bus and
programs an MPLS destination with a stack.

<!-- source: internal/plugins/rsvpte/fib.go -- busFIB programBackup -->

## Decision: node protection requires label recording

The merge point for node protection is the next-next hop, which expects **its
own** label rather than the next hop's. So when protection is requested the
SESSION_ATTRIBUTE sets the label-recording-desired flag, each node records its
label after its RRO address subobject (RFC 3209 section 4.4.3), and the point of
local repair resolves the next-next hop's label out of the received RESV.
`tryLocalRepair` pushes that label under the bypass label. Link protection uses
the next hop's advertised label instead.

<!-- source: internal/plugins/rsvpte/engine.go -- recordRoute, labelForAddr -->

## Trap: a plugin engine that reads config at runtime must not freeze it

The engine copied the config at construction. `OnConfigApply` reconciled with the
fresh config but never updated that copy. The base engine survived it because it
only read a few rarely-reloaded fields. Fast Reroute reads the bypass list on
every PATH, so after a reload that added or removed a bypass it silently stopped
arming.

The config now sits behind an atomic pointer, written by `setConfig` from
`OnConfigApply` and read through an accessor.

<!-- source: internal/plugins/rsvpte/engine.go -- cfgPtr, setConfig -->

## Trap: a startup-only mutator gains a reload caller

`admission.setInterface` **replaced** the per-interface bandwidth struct, zeroing
the reserved counter. That was harmless while it ran once before any reservation
existed. The reconcile-on-reload path calls it for every interface on every
commit, so a reload with LSPs up wiped the reserved counter and the link admitted
past its maximum: admission control defeated.

It is now read-modify-write, updating the limits and keeping the live
reservation. When a startup-only mutator gains a reload caller, audit it for
state it silently discards.

<!-- source: internal/plugins/rsvpte/admission.go -- setInterface, interfaceBandwidth -->

## Trap: making a field reloadable exposes every read that assumed it valid

`router-id` was set once at startup, so the engine, its keys, and its encoders
all assumed a valid IPv4 address. Putting the config behind a reloadable pointer
meant a reload that removed `router-id`, which is not mandatory in the YANG,
crashed the plugin: `OnStarted` guarded an invalid router ID and `OnConfigApply`
did not, and runtime reads dereferenced the zero address.

It is closed at three layers: `parseConfig` rejects a non-IPv4 or unparsable
router ID; `reconcileTunnels` carries the guard so it covers every caller;
`setConfig` **preserves** the router ID, because it is the LSR identity and is a
restart-class value rather than a reloadable one.

When you make a field reloadable, audit every read that trusted its old
immutability and decide per field whether it is reloadable or restart-class.

## Trap: an unbounded wire list that is re-encoded on relay

The explicit-route decoder had no hop cap and the encoder had no buffer guard,
unlike the RRO path. A crafted transit PATH with about seventy IPv6 hops
overflowed the fixed encode buffer. A comment asserted the object was bounded and
nothing enforced the bound. Pair a decode cap with an encode bounds check for
every object that is relayed.

## Trap: SESSION_ATTRIBUTE has two C-Types

C-Type 1 (LSP_TUNNEL_RA) carries a 12-byte affinity prefix; C-Type 7 (LSP_TUNNEL)
does not. Decode must branch on the C-Type or it reads the protection flags from
the wrong offset for an interop peer.

## Trap: a stored key into a reloadable collection must be stable

Index-derived bypass keys re-keyed on a config reorder. The key is derived from a
name hash instead, and a hash collision is detected at config time.

## Concurrency rules this code follows

- Build the wire message **under** the LSP lock, as `sendResv` and `sendPath` do.
  Reading the reservation state block after unlock races the refresh goroutine's
  in-place write. The race detector missed it because no unit test ran the
  refresh loop concurrently.
- Allocate table-scoped resources **outside** the per-LSP lock: snapshot under
  the lock, release it, allocate, then re-lock to commit. Holding the LSP lock
  across a label allocation inverts the table-to-LSP lock order.

## Trap: an interop test must observe the repaired state before convergence

After local repair the head-end correctly tears the protected LSP down once it
re-optimizes. A test must assert "repaired, retained, in use" **before** it pumps
the Notify and reroute to convergence; afterwards the old LSP is legitimately
gone.

The fabric harness routes by node address rather than by link, and link-down
matching needs interfaces. So a Fast Reroute fabric test gives the point of local
repair two interfaces, on the protected and bypass subnets, and adds a fourth
node as the bypass transit, so the bypass genuinely avoids the failed link.
