# RSVP-TE Fast Reroute

Ze uses configured RFC 4090 facility-backup LSPs: one bypass protects several
LSPs by label stacking, with local repair at the detecting node. One-to-one
detour backup is not implemented. This describes the implemented path, not a
conformance verdict for every RFC 4090 requirement.

The FRR codecs and repair selection live in `frr.go`, alongside the signaling
engine documented in [`mpls-rsvp-te.md`](mpls-rsvp-te.md).

<!-- source: internal/plugins/rsvpte/frr.go -- encodeFastReroute, encodeSessionAttr, protectionRequest, selectBypass, tryLocalRepair -->

## Decision: bypass paths are configured, not computed

Ze has native OSPF and IS-IS, but RSVP-TE does not consume a traffic-engineering
database or run CSPF. A `bypass` configuration therefore supplies the explicit
route from the point of local repair to a merge point, avoiding the protected
resource.

The point of local repair associates a configured bypass to a protection-desired
transit LSP by matching the bypass merge point to the LSP's next hop (link
protection) or next-next hop (node protection).

Bypass LSPs key into the same LSP table as protected tunnels. The top 4096 tunnel
IDs are reserved for them (`bypassTunnelIDBase = 0xF000`), so a bypass can never
collide with a protected tunnel to the same destination.

A bypass reserves no bandwidth of its own, so it never guarantees a protected
LSP's bandwidth. The point of local repair therefore never sets the RRO
"bandwidth protection" bit (0x04), whatever the head-end asked for (RFC 4090
section 4.4: "If the requested bandwidth is not guaranteed, the PLR MUST NOT set
this flag"). The "node protection" bit (0x08) is set only when the armed bypass
merges at the next-next hop.

All four bits stay clear until the native FIB owner has accepted the bypass's
forwarding context and the protected LSP has a resolved merge-point label.
Arming a bypass at PATH time records a configuration match. RFC 4090 section 6
says: "Until a PLR has a backup path available, the PLR MUST clear the relevant
four flags in the corresponding RRO IPv4 or IPv6 sub-object."

<!-- source: internal/plugins/rsvpte/frr.go -- bypassEstablished, rroProtectionFlags -->

Both a relayed RESV and a periodic RESV refresh read the current bypass state.
They clear stale cached flags when that bypass goes down. The read occurs outside
the protected LSP's lock because the two LSPs have no lock order.

<!-- source: internal/plugins/rsvpte/reservation.go -- acceptReservation, receivedReservation -->
<!-- source: internal/plugins/rsvpte/engine.go -- sendResv -->

## Decision: a transit relays FAST_REROUTE byte for byte

RFC 4090 section 4.1 reserves the FAST_REROUTE object to the head-end. A transit
keeps the object it received (`protectionRequest.Received`) and relays that one,
affinity fields and flags included, and relays none when the head-end signaled
protection through the SESSION_ATTRIBUTE flag alone. The SESSION_ATTRIBUTE is
relayed the same way, as the received bytes, whether or not it requests
protection (`docs/architecture/rsvpte/mpls-rsvp-te.md`, "a transit relays
SESSION_ATTRIBUTE byte for byte").

<!-- source: internal/plugins/rsvpte/frr.go -- protectionFromPath, fastRerouteObject -->

## Decision: local repair moves data and control traffic

`tryLocalRepair` is slotted into `handleLinkDown` exactly where the base engine
tore the LSP down. With a ready bypass, a transit repair programs a two-label
swap and skips teardown; the PLR sends a Notify PathErr (code
25, value 3). The head-end re-optimizes make-before-break on the Notify and
retains the protection request on its replacement LSP. The bypass itself is an
ordinary ingress LSP.

<!-- source: internal/plugins/rsvpte/engine.go -- handleLinkDown -->
<!-- source: internal/plugins/rsvpte/frr.go -- tryLocalRepair, reoptimizeOnNotify -->

The PLR immediately sends the protected PATH through the bypass and refreshes it
on subsequent ticks. The backup keeps SESSION and LSP_ID, uses the PLR's address
for SENDER_TEMPLATE and RSVP_HOP, and starts its ERO at the merge point. This
removes the failed node from a node-protection ERO. The backup clears the three
protection-desired SESSION_ATTRIBUTE bits and does not request another repair.
The original PSB is retained for upstream signaling.

The bypass's own PATH continues to refresh along its configured explicit route.
Only the repaired sender uses the bypass's private forwarding context. Holding
all outgoing PATH messages during repair would also stop the refreshes that keep
the bypass established.

A head-end can also be the PLR. It then needs a second IPv4 address assigned in
its network namespace: Section 6.1.1 requires a different SENDER_TEMPLATE
address for the backup. A configured link prefix is not address ownership.
Without an assigned alternate it does not arm a bypass. With one, repair
installs a two-label ingress push, uses that alternate sender, sets RSVP_HOP
to the outgoing interface, and starts make-before-break re-optimization.
Retiring the old LSP tears the backup sender through the bypass without removing
the replacement's ingress FEC.

The bypass RESV installs its merge-point host route in a private forwarding
table, selected by that bypass's packet mark. Two bypasses to the same merge
point therefore keep separate stacks and output links. PATH, PathTear and
ResvConf select the protected LSP's exact bypass context. Their IP destination
remains the session endpoint or confirmation receiver; the routing lookup uses
the merge point. An absent or unlabeled context is refused, and a terminal
reserved-mark rule prevents fallback to the ordinary routing table.

Native forwarding acceptance precedes an Up state or an advertised label.
Kernel refusal, including a conflicting route, prevents a new reservation from
coming up. If replacing an accepted reservation fails, the previous reservation
remains in place.
Returning backup RESVs update the inner merge label without restoring the
failed swap. A PathErr is translated to the original sender before going
upstream. An alternate merge-point source address is accepted only when the
live native OSPF or IS-IS database attributes it to the same node. A reachable
host prefix alone is not proof of address ownership.

Withdrawing a bypass retires dependent backup senders and their repair stacks
before removing its private route and selector. Shutdown stops the signaling
workers, retires protected LSPs, then removes their bypasses. Other bypasses,
including those with the same merge point, retain their contexts.

At the merge point, a backup sender joins the protected PATH when SESSION,
LSP_ID, the remaining ERO and the sender traffic specification match. The MP
preserves the protected sender downstream, refreshes that PATH periodically,
and sends RESVs to each incoming branch with that branch's FILTER_SPEC.
Branches expire independently. The final branch's teardown releases the shared
label and downstream state.

<!-- source: internal/plugins/rsvpte/register.go -- refreshPaths -->
<!-- source: internal/plugins/rsvpte/transport_linux.go -- rawTransport.SendPath -->
<!-- source: internal/plugins/rsvpte/frr.go -- backupPath, repairedLSP, mergeBackupPath, refreshBackupForwarding -->
<!-- source: internal/plugins/rsvpte/engine.go -- sendPath, sendResv, handlePathErr, handlePathTear -->
<!-- source: internal/plugins/rsvpte/fsm.go -- mergedPath, expiredPSBs -->
<!-- source: internal/plugins/rsvpte/frr.go -- repairSender -->
<!-- source: internal/plugins/rsvpte/reroute.go -- teardownLSP -->
<!-- source: internal/plugins/rsvpte/confirmation.go -- sendResvConf, handleResvConf -->
<!-- source: internal/plugins/rsvpte/peer_identity.go -- samePeer, updatePeerIdentity -->
<!-- source: internal/plugins/rsvpte/reroute.go -- shutdown -->
<!-- source: internal/plugins/fib/kernel/mplscontext_linux.go -- scoped forwarding contexts -->

The MPLS FIB entry carries the full outgoing label stack. Its native owner
acknowledges installation synchronously; a published event alone is not
forwarding evidence. Kernel MTU handling must be checked separately from
small-packet push, swap and pop behavior.

<!-- source: internal/plugins/rsvpte/fib.go -- busFIB programBackup -->
<!-- source: internal/plugins/rsvpte/reservation.go -- acceptReservation, installReservation -->

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
