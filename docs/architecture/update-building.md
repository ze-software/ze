# UPDATE Message Building Architecture

## TL;DR (Read This First)

| Concept | Description |
|---------|-------------|
| **Three Paths** | Receive (zero-copy ingest) → Forward (reflection) → Build (origination) |
| **Receive Path** | conn.Read → WireUpdate(owns buffer) → API/Cache (zero-copy) |
| **Build Path** | Config → *Params → UpdateBuilder.Build*() → Update{[]byte} |
| **Forward Path** | WireUpdate{payload, sourceCtxID} → one built body per destination ContextID |
| **Key Insight** | Zero-copy from read buffer to API; high-volume forwarding uses wire cache |

**When to read full doc:** Understanding message building, *Params design, FlowSpec differences.

---

## Three Paths for UPDATE Messages

### Path 0: Receive Path (Zero-Copy Ingest)

For incoming messages from peers (applies to ALL message types):

```
buf := s.getReadBuffer()  ← Get from appropriate pool (4K/64K)
       ↓
conn.Read(buf)            ← Read directly into pool buffer
       ↓
err, kept := processMessage(buf)  ← Callback returns kept=true if caching
       ↓
if !kept: s.returnReadBuffer(buf) ← Return only if not cached
```

**Buffer pools (size-appropriate):**
`bufMuxStd` and `bufMuxExt` are block-backed multiplexers with a shared byte
budget, not `sync.Pool` instances. Read ownership travels as a `BufHandle`;
`returnReadBuffer` returns pooled handles to the size-appropriate multiplexer
and ignores non-pool handles.

**Files involved:**
- `internal/component/bgp/reactor/session.go` - `Session`, read multiplexers, and `returnReadBuffer()`
- `internal/component/bgp/wireu/wire_update.go` - `WireUpdate` struct with derived accessors
- `internal/component/bgp/reactor/reactor.go` - `notifyMessageReceiver()` takes buf ownership when caching
- `internal/component/bgp/reactor/recent_cache.go` - Returns buf to pool on eviction
<!-- source: internal/component/bgp/reactor/session.go -- Session, getReadBuffer, returnReadBuffer -->
<!-- source: internal/component/bgp/wireu/wire_update.go -- WireUpdate struct -->
<!-- source: internal/component/bgp/reactor/recent_cache.go -- RecentUpdateCache -->

**Key types:**
```go
// internal/component/bgp/wireu/wire_update.go
type WireUpdate struct {
    payload     []byte           // UPDATE body (slice into pool buffer)
    sourceCtxID bgpctx.ContextID
}

// internal/component/bgp/reactor/received_update.go
type ReceivedUpdate struct {
    WireUpdate   *api.WireUpdate  // Slices into poolBuf
    poolBuf      []byte           // Returned to pool on eviction
    SourcePeerIP netip.Addr       // Peer that sent this UPDATE
    ReceivedAt   time.Time        // When received
}

// Derived accessors (zero-copy slices)
// Return (nil, nil) for valid empty, (nil, error) for malformed
func (u *WireUpdate) Withdrawn() ([]byte, error)
func (u *WireUpdate) Attrs() (*AttributesWire, error)
func (u *WireUpdate) NLRI() ([]byte, error)
func (u *WireUpdate) MPReach() (MPReachWire, error)   // nil,nil if attr not present
func (u *WireUpdate) MPUnreach() (MPUnreachWire, error) // nil,nil if attr not present
```
<!-- source: internal/component/bgp/reactor/received_update.go -- ReceivedUpdate struct -->

**Buffer lifecycle (ownership transfer):**
1. Session gets buffer from appropriate pool (`getReadBuffer()`)
2. Session reads message into buffer
3. For UPDATE: creates `WireUpdate` from slice (no copy)
4. **Callback executes FIRST** (buffer always valid during callback)
5. Then cache `Add()` - returns `kept=true` if caching
6. If cached: cache owns buf
7. If not cached: session returns buffer to pool immediately

**Cache API:**
- `Add(update)` - cache takes ownership, returns buf to pool if full/rejected
- `Take(id)` - removes entry, transfers ownership to caller
- `Contains(id)` - check existence without taking ownership
- `Delete(id)` - remove and return buffer to pool
- `ReceivedUpdate.Release()` - caller returns buffer after `Take()`

**Critical ordering:** Callback before cache ensures buffer is valid during callback.
Cache `Take()` prevents use-after-free by transferring ownership.

### Path 1: Build Path (Local Origination)

For routes originating from config or API:

```
Config/API → Domain Object → *Params → UpdateBuilder.Build*() → Update
     │              │              │               │              │
     │              │              │               │              └── Contains raw []byte
     │              │              │               └── Packs to wire format
     │              │              └── Typed struct (UnicastParams, PluginParams, etc.)
     │              └── PluginRoute, StaticRoute, etc.
     └── YAML config, CLI commands
```

The MP_REACH exotic families (MUP, VPLS, MVPN, FlowSpec, SR-Policy) all share one
generic path: the family plugin's config-route parser pre-builds the NLRI and the
family-specific path attributes, which flow through `reactor.PluginRoute` →
`message.PluginParams` → `UpdateBuilder.BuildPlugin()`. There is no per-family
`Build*()` in the message package for these families (see plugin-self-containment).

**Files involved:**
- `internal/component/bgp/reactor/peer_settings.go` - Domain objects (PluginRoute, StaticRoute, etc.)
- `internal/component/bgp/reactor/peer_static_routes.go` - Conversion functions (toPluginParams, etc.)
- `internal/component/bgp/message/update_build.go` - UpdateBuilder, *Params structs, Build*() methods
- `internal/component/bgp/message/update_build_plugin.go` - BuildPlugin + PluginParams (generic exotic-family path)
<!-- source: internal/component/bgp/reactor/peer_settings.go -- PluginRoute, StaticRoute -->
<!-- source: internal/component/bgp/reactor/peer_static_routes.go -- toPluginParams -->
<!-- source: internal/component/bgp/message/update_build_plugin.go -- BuildPlugin, PluginParams -->

SR Policy preserves a native four-octet IPv4 next hop under either NLRI AFI.
Its configured-route parser and command encoder do not enable the generic
IPv4-mapped conversion used by MUP. RFC 9830 Section 2.1 gives SR Policy its own
next-hop family contract; an IPv6 field still requires a global IPv6 address,
optionally followed by link-local, without RFC 8950 capability 5 gating.
<!-- source: internal/component/bgp/plugins/nlri/srpolicy/config.go -- parseConfigRoute -->
<!-- source: internal/component/bgp/plugins/nlri/srpolicy/encode.go -- EncodeRoute -->
<!-- test: internal/component/bgp/plugins/nlri/srpolicy/rfc2545_next_hop_wire_test.go TestSRPolicyConfigNextHopWire -->
<!-- test: internal/component/bgp/plugins/nlri/srpolicy/rfc2545_next_hop_wire_test.go TestSRPolicyEncodeNextHopWire -->

The single-route IPv6 announce writer uses the shared family/field admission.
It refuses loopback, unspecified, multicast, native four-octet and unset inputs
with `ErrNextHopUnencodable`, before writing any bytes. This legacy direct rail
also refuses standalone link-local input: it has no capability-77 permission
input. A valid global address can carry its optional link-local second address;
an explicit mapped address is admitted only as a single sixteen-octet field.
These checks belong to `announceNextHopOctets`, shared by diagnosis and encoding,
so bypassing the caller's validation cannot write a malformed announcement.
<!-- source: internal/component/bgp/reactor/reactor_wire.go -- announceNextHopOctets -->
<!-- test: internal/component/bgp/reactor/rfc2545_announce_nexthop_guard_test.go TestSendAnnounceRefusesUnusableIPv6NextHop -->
The announce and default-originate scope fixtures use an explicitly configured
speaker-owned unicast IPv6 address and connected-prefix snapshot, not the
loopback address as an advertised global next hop.
<!-- test: internal/component/bgp/reactor/rfc2545_peer_send_test.go TestSendAnnounceAppendsLinkLocalWhenSection3Holds -->
<!-- test: internal/component/bgp/reactor/peer_initial_sync_test.go TestDefaultOriginateAppendsLinkLocalWhenSection3Holds -->

The shared `attribute.MPNextHopProfile` declares field widths, plain IPv6 address
roles, mapped-address exceptions and RFC 8950 capability scope in one place.
`ValidNextHopLens` derives its answer from that declaration. Native IPv6
unicast, multicast and labeled fields use 16/32 octets. Their plain IPv4
counterparts also admit four octets, and IPv6 fields require the exact negotiated
capability 5 pair. Both SR Policy AFIs admit 4/16/32 octets without capability 5.
An ordinary IPv6 global slot refuses unset, unspecified, loopback and multicast
addresses; its optional second address must be link-local. RFC 8950 Section 1
also recognizes a single sixteen-octet IPv4-mapped field for IPv6 unicast,
multicast and labeled NLRI. Explicit mapped input is retained without a reverse
capability 5 tuple; native four-octet input and mapped-plus-link-local pairs
remain invalid for those fields. This recognizes the existing control-plane
encoding, not 6PE transport, automatic IPv4 mapping, discovery, LSP installation
or entity adjacency.
VPN keeps its RD-bearing layout and RFC 4659 mapped interpretation; MVPN keeps
its independent IP next-hop family and FlowSpec its ignored zero-hop field.
Unknown and independently defined plugin profiles retain their own contracts.
Configured multicast, labeled and SR Policy builders use the same admission.
`BuildLabeledUnicast` and `BuildPlugin` return nil on refusal; callers must not
pack or send that result. Native IPv4 remains legal where its profile allows it.
Admission checks only serialized inputs: the unicast builder ignores an unused
link-local hint when the emitted next hop is not native IPv6.
<!-- source: internal/component/bgp/message/update_build.go -- BuildUnicast, checkUnicastNextHop -->
<!-- source: internal/component/bgp/reactor/peer.go -- resolveNextHop, linkLocalOnlyNextHopRefused -->
<!-- test: internal/component/bgp/reactor/rfc2545_static_origination_test.go TestStaticOriginateIPv6NextHopAdmission -->
<!-- test: internal/component/bgp/reactor/mapped_plain_next_hop_test.go TestMappedPlainNextHopBuilder -->
<!-- test: internal/component/bgp/reactor/mapped_plain_next_hop_test.go TestMappedPlainNextHopOrigination -->
<!-- test: internal/component/bgp/reactor/mapped_plain_next_hop_test.go TestMappedPlainNextHopForwarding -->

The established batch API, queued initial-sync writer and named-commit service
also use this family/field admission before their direct MP_REACH construction.
The check follows the actual field, not a blanket NLRI-AFI restriction.
The batch API returns an encoding refusal, queued
initial synchronization still sends End-of-RIB, and named commits report
`AnnounceRefused` without counting the rejected route as announced. Named commits
retain the existing `attribute.ErrUnencodableNextHop` classification for an unset
address: presence validation runs before the additional address-role check.
The batch API's legacy inline IPv4 branch keeps its existing invalid-hint remedy:
it leaves the base NEXT_HOP untouched rather than contributing a malformed one.
That remedy does not apply to MP_REACH, whose emitted next-hop field must pass
the family/field check.
<!-- source: internal/core/bgp/attribute/mpnlri.go -- MPNextHopProfile, ValidNextHopLens -->
<!-- source: internal/component/bgp/message/update_build.go -- ValidateFamilyNextHop, ValidateMPNextHop -->
<!-- source: internal/component/bgp/reactor/reactor_api_batch.go -- buildBatchAnnounceUpdate, commitToPeer -->
<!-- source: internal/component/bgp/reactor/peer_rib_routes.go -- buildRIBRouteUpdate -->
<!-- source: internal/component/bgp/rib/commit.go -- buildMPReachNLRI -->
<!-- test: internal/component/bgp/reactor/rfc2545_api_origination_test.go TestAPIBatchRefusesUnusableIPv6NextHop -->
<!-- test: internal/component/bgp/reactor/rfc2545_api_origination_test.go TestQueuedOriginateRefusesUnusableIPv6NextHop -->
<!-- test: internal/component/bgp/reactor/rfc2545_api_origination_test.go TestNamedCommitRefusesUnusableIPv6NextHop -->

The ordinary session writer repeats this shared check on the final next hop
**after** export-policy overrides. Admission checks the original field width
before normalized addresses: stripping a VPN RD cannot authorize that shape in
a plain field. The writer also rechecks the exact RFC 8950 capability 5 pair
and capability 77's standalone-link-local permission, including named commits
that do not use peer resolution. Capability 77 never licenses an invalid pair.
A refusal is route-scoped: initial-sync queues,
separate static groups and withdrawals, and originated forward-queue items
continue to independent usable siblings. The forward worker still flushes
accepted bytes, and API queue acceptance is not a claim of eventual delivery.
Named commits continue usable groups and retain accepted final-writer counts
beside a later refusal. Both input halves report actual announced and withdrawn
routes and UPDATEs after policy and splitting: an announcement-half refusal keeps
`AnnounceRefused`, a withdrawal send error keeps `SendFailed`, and a withdrawal
build refusal keeps `WithdrawRefused`. A complete earlier split message is counted
even when a later section cannot fit. The batch API preserves final splitter
NLRI, attribute and MP-overhead encoding errors rather than replacing them with
a no-negotiated-family warning. Connection failures remain fail-fast; the
first-error contract within one split UPDATE and all-or-nothing legacy
static-group recording are unchanged. Intentional raw injection and pre-filtered
forwarding retain their distinct admission boundaries.
<!-- source: internal/component/bgp/reactor/session_write.go -- writeUpdateGated -->
<!-- source: internal/component/bgp/reactor/peer_send.go -- isRouteScopedSendError -->
<!-- source: internal/component/bgp/rib/commit.go -- Commit -->
<!-- source: internal/component/bgp/reactor/forward_next_hop.go -- payloadNextHop -->
<!-- source: internal/component/bgp/reactor/peer_static_wire.go -- sendStaticRoutes, sendStaticRoutesGrouped, withdrawStaticRoutes -->
<!-- source: internal/component/bgp/reactor/forward_pool.go -- fwdBatchHandler -->
<!-- test: internal/component/bgp/reactor/rfc2545_api_origination_test.go TestOriginatedIPv6NextHopAdmissionAfterExportPolicy -->
<!-- test: internal/component/bgp/reactor/rfc2545_static_origination_test.go TestStaticOriginateContinuesAfterUnusablePolicyNextHop -->

The shared splitter receives an ADD-PATH selector per family. Homogeneous
original builder calls supply their fixed framing through a selector; final
Session and parsed-relay splitting use the destination encoding context's
`AddPath` selector. Legacy IPv4 and each MP section therefore retain their own
negotiated framing in a mixed policy replacement.
<!-- source: internal/component/bgp/message/update_split.go -- Split, SplitCompliant -->
<!-- source: internal/component/bgp/reactor/reactor_api_batch.go -- queueBehindForwards, sendWithdrawals -->
<!-- test: internal/component/bgp/reactor/ordinary_withdraw_result_test.go TestOrdinaryWithdrawalFinalResult -->

IPv6 default origination uses that peer resolution gate too. With automatic local
addressing, next-hop self resolves to the actual connected session endpoint.
An absent or unusable IPv6 endpoint refuses the route; the producer does not
invent `::1`. The ordinary End-of-RIB still closes initial synchronization.
<!-- source: internal/component/bgp/reactor/peer_initial_sync.go -- defaultRouteForAFI, sendDefaultOriginateRoutes -->
<!-- source: internal/component/bgp/reactor/session_connection.go -- connectedLocalAddress -->
<!-- test: internal/component/bgp/reactor/peer_initial_sync_test.go TestDefaultOriginateRefusesUnusableIPv6NextHop -->

Received and policy-written IPv6 next-hop pairs also pass an egress check on
both forwarding rails. Their first address must be IPv6 global unicast and
their second IPv6 link-local unicast. An invalid slot causes a native withdrawal,
not an announcement with the malformed pair retained or silently trimmed.
The existing RFC 4659 48-octet VPN exception still permits its canonical
unspecified first address with a valid link-local second address on qualifying
peering. Capability 77's negotiated single-address form remains separate.
Validation judges effective policy output, not an obsolete received pair, and
preserves a valid legacy sibling in mixed input. These checks validate address
roles and wire forms, not next-hop-entity adjacency.
An effective 16-octet speaker-owned IPv6 global also receives the configured
speaker Link-Local when one connected prefix contains that global and the
recipient. This happens after policy on both rails, including raw export
fallback under `next-hop auto` or `unchanged`. It uses the existing immutable
ownership snapshot and accumulator storage, without rerunning policy.
Third-party Link-Local discovery remains absent; the speaker's own address
cannot substitute for another router's Link-Local.
The global-unicast requirement also covers unpaired ordinary IPv6 globals:
loopback cannot bypass it. The explicit single mapped IPv6 unicast, multicast,
labeled and RD-bearing VPN forms retain their separate control-plane
interpretations. A malformed received four-octet AFI 2 field instead triggers
the existing whole-session reset under RFC 7606 Section 7.11 before forwarding;
its legacy sibling is not published either.
The effective next-hop field length supplies this family, independently of the NLRI AFI and
before addresses are unmapped for identity comparison. Policy replacement updates
that wire-family fact. Native IPv4 next hops, including those for IPv6 SR Policy
NLRI (RFC 9830 Section 2.1), and negotiated standalone link-local IPv6 retain
their separate encodings.
The MP writer requires an existing MP_REACH source. On a legacy-only route,
the extra MP operation recorded alongside an IPv4 next-hop rewrite emits no
attribute and does not trigger IPv6 global-address admission.
For the plain IPv6-role profiles, both rails check the effective field's original
length against its declaration. A native IPv4 or VPN-shaped field cannot replace
a native plain IPv6 field. The same declaration admits extended IPv4 labeled 16/32-octet fields and
both SR Policy AFIs' 32-octet pairs. Addressless
zero/eight-octet raw fields are checked before the no-forwarding-address exit,
so they cannot masquerade as withdrawals. The check preserves wrong-width pairs
before scope trimming could conceal them; the last legal policy replacement
still supersedes obsolete input or an earlier invalid operation. A replacement
of an addressless base remains subject to all address-role, next-hop-self,
peer-identity, capability and link-scope gates. Raw export responses receive the
same check, including the route server's export-policy fallback. When mixed
output is partitioned, the native
width refusal survives materialization even if an unsupported operation would
otherwise leave the original attribute unchanged; it does not withdraw the
independent legacy section. Valid VPN RD forms, SR Policy and MVPN's independent
next-hop families, FlowSpec zero-hop and negotiated capability 77 remain intact.
<!-- source: internal/component/bgp/reactor/forward_next_hop.go -- applyEgressNextHopScope, egressNextHopWithheld -->
<!-- source: internal/component/bgp/reactor/reactor_api_forward.go -- forwardUpdateCore -->
<!-- test: internal/component/bgp/reactor/rfc2545_forward_subnet_test.go TestRFC2545ReceivedPairSecondAddressValidated -->
<!-- test: internal/component/bgp/reactor/rfc2545_forward_subnet_test.go TestRFC2545ReceivedPairFirstAddressValidated -->
<!-- test: internal/component/bgp/reactor/rfc2545_forward_subnet_test.go TestRFC2545EffectivePairPolicyAndMixedSibling -->
<!-- test: internal/component/bgp/reactor/rfc2545_forward_subnet_test.go TestRFC2545ReceivedSingleGlobalAddressValidated -->
<!-- test: internal/component/bgp/reactor/rfc2545_forward_owned_policy_test.go TestRFC2545ForwardPolicyOwnGlobalJointSubnet -->
<!-- test: internal/component/bgp/reactor/rfc2545_forward_subnet_test.go TestSRPolicyNextHopWireFamily -->
<!-- test: internal/component/bgp/reactor/next_hop_family_admission_test.go TestFamilyNextHopOriginationAdmission -->
<!-- test: internal/component/bgp/reactor/next_hop_family_admission_test.go TestFamilyConfiguredBuilderAdmission -->
<!-- test: internal/component/bgp/reactor/next_hop_family_admission_test.go TestFamilyNextHopForwardAdmission -->
<!-- test: internal/component/bgp/reactor/next_hop_family_admission_test.go TestFamilyIndependentNextHopFields -->
<!-- test: internal/component/bgp/reactor/next_hop_family_admission_test.go TestMappedVPNNextHopForwardAdmission -->

FlowSpec origination omits the MP_REACH next-hop even when configuration or the
API supplies an IPv4 or IPv6 address. RFC 8955 Section 4 requires zero next-hop
length, including the IPv6 families covered by RFC 8956. The config builder,
established API batch builder and queued writer each consult `Family.NeedsNextHop()`.
The socket test compares complete UPDATEs for all four registered FlowSpec
families, preserving the NLRI, VPN route distinguisher and traffic-rate action.
<!-- source: internal/component/bgp/message/update_build_plugin.go -- buildMPReachPlugin -->
<!-- source: internal/component/bgp/reactor/reactor_api_batch.go -- buildBatchAnnounceUpdate -->
<!-- source: internal/component/bgp/reactor/peer_rib_routes.go -- buildRIBRouteUpdate -->
<!-- test: internal/component/bgp/reactor/flowspec_origin_wire_test.go TestFlowSpecOriginationOmitsConfiguredNextHop -->

Forwarding applies that family contract after destination policy and before
next-hop withholding. Received FlowSpec next-hop bytes are ignored for forwarding
decisions; `self`, explicit addresses and filter rewrites cannot supply a
FlowSpec next hop. The registered MP_REACH edit handler removes the received
field in the destination-owned output without changing the cached input, rule,
route distinguisher or actions. A genuine legacy-unicast sibling retains its own
next-hop decisions. The route-server proof drives actual TCP sessions and
covering-route authorization through the registered RIB and route-server plugins.
Its plugin server is installed as the shared event bus before engines spawn, as
in the daemon, so authorization recovery can replay a rule whose first forward
ran before the RIB selected it.
<!-- source: internal/component/bgp/reactor/forward_next_hop.go -- payloadNextHop, applyNextHopFamily -->
<!-- source: internal/component/bgp/reactor/filter_delta_handlers.go -- mpReachNextHopHandler -->
<!-- test: internal/component/bgp/reactor/flowspec_forward_wire_test.go TestFlowSpecForwardingOmitsNextHop -->
<!-- test: internal/component/bgp/reactor/flowspec_rs_wire_test.go TestFlowSpecRouteServerOmitsNextHop -->

**Flow example (FlowSpec via the generic plugin path):**
```go
// 1. The flowspec plugin's config parser pre-builds the NLRI + action attrs.
pr, _ := parser(registry.ConfigRouteRequest{Content: tokens, ExtCommunity: ec})

// 2. At send time, convert to params (NLRI + RawAttrs pre-built by the plugin).
params := message.PluginParams{AFI: 1, SAFI: 133, NLRI: pr.NLRI, RawAttrs: rawAttrs}

// 3. Build UPDATE (BuildPlugin owns only ORIGIN/AS_PATH/LOCAL_PREF/MP_REACH).
update := ub.BuildPlugin(params)  // Returns Update{PathAttributes: []byte}

// 4. Send
peer.SendUpdate(update)
```

### Path 2: Forward Path (Route Reflection)

For routes received from peers and forwarded, the wire bytes never enter a route
struct. The received `WireUpdate` carries them, and it carries the ContextID of
the session that produced them:

```
Receive UPDATE → WireUpdate{payload, sourceCtxID} → Forward
                        │                              │
                        │                              └── One build shared by every
                        │                                  destination on that ContextID
                        └── The read buffer, not a copy
```

`forwardUpdateCore` resolves the source context once for the whole fan-out, then
builds one body per `(destCtxID, wire, extended)` key and reuses it for every
destination that lands on the same key. Peers with identical negotiated
capabilities share one ContextID, so route reflection between same-capability
clients builds once and sends the result N times.

**Files involved:**
- `internal/component/bgp/reactor/reactor_api_forward.go` - ForwardUpdate, forwardUpdateCore, the per-context body cache
- `internal/component/bgp/reactor/update_group.go` - GroupKey, the ContextID that decides which peers share a build
- `internal/core/bgp/context/` - EncodingContext, ContextID, Registry
- `docs/architecture/encoding-context.md` - Detailed context system docs
<!-- source: internal/component/bgp/reactor/reactor_api_forward.go -- forwardUpdateCore -->
<!-- source: internal/component/bgp/reactor/update_group.go -- GroupKey, UpdateGroupIndex -->
<!-- source: internal/core/bgp/context/registry.go -- ContextID, Registry -->

`rib.Route` takes no part in this path. It is the value a named commit and the
route API hand to `CommitService`, and it holds NLRI, next hop, attributes and
AS-PATH only.
<!-- source: internal/component/bgp/rib/route.go -- Route struct -->
<!-- source: internal/component/bgp/rib/commit.go -- CommitService, NewCommitService -->

**Flow example (named commit):**
```go
// 1. Build the route from what the operator asked for.
route := rib.NewRouteWithASPath(nlri, nextHop, attrs, asPath)

// 2. Group the routes and build one UPDATE per attribute group.
cs := rib.NewCommitService(peer, encodingContext, grouped)
stats, err := cs.Commit([]*rib.Route{route}, rib.CommitOptions{SendEOR: false})
```

---

## Why Two Paths?

| Concern | Build Path | Forward Path |
|---------|------------|--------------|
| Volume | Low (config rules) | High (millions of routes) |
| Frequency | Once at session start | Continuous |
| Optimization | Pre-pack at config time | Zero-copy forwarding |
| Key structure | *Params structs | WireUpdate payload, cached per destination ContextID |

**The forward path is where scale matters.** Route reflection of millions of routes needs zero-copy. The build path handles low-volume local origination.

### One final ownership and duplicate boundary

Encoded and pre-encoded managed UPDATEs share the final writer's admission. The existing
`adjOut` table is bound to one Session and stores one current owner per native
route and outgoing ADD-PATH presence/ID. Forwarded owners retain stable source
identity and ingress path identity; local/config/API writes establish local
ownership. Explicit local withdrawals retain their authority, while automatic
RIB cleanup and sent replay carry captured ownership rather than impersonating
an operator.

Policy output is filtered per withdrawal path before PATHS-LIMIT mutates its
counts. An original withdrawal requires an existing matching owner; a synthesized
withdrawal may pass when absent, but neither can remove another owner's route.
Surviving legacy and MP sections are preserved. Removing every route from an
ordinary UPDATE produces no message, not an accidental End-of-RIB.

Ordinary UPDATE sizing follows the final policy and next-hop result, after AIGP
stripping. A body that still fits goes directly to the existing writer without
another copy. A larger body is split with the destination's send-size limit and
ADD-PATH framing, without running export policy again. Each final chunk gets its
own ownership admission, PATHS-LIMIT accounting and sent receipt. A later split
error does not leave earlier accepted chunks pending indefinitely: they are
flushed before the error is returned, and a failed flush retires the connection.
<!-- source: internal/component/bgp/reactor/session_write.go -- writeOrdinaryUpdateBody -->

Named-commit message, announced-route and withdrawn-route totals come from the
final writer's accepted output, not the number or shape of original grouped send
calls. Its existing ownership-record walk counts each final route in its actual
direction, so a policy-generated withdrawal is not reported as an announcement.
Suppressed and PATHS-LIMIT-withheld routes contribute no output count. A later
error retains the earlier accepted counts beside the refusal reason. These are
the same buffered-acceptance counts as sent callbacks, not a guarantee of TCP
delivery after a failed flush.
<!-- source: internal/component/bgp/reactor/session_paths_limit.go -- updateSendCounts, commitUpdateSender -->
<!-- source: internal/component/bgp/reactor/reactor_api_batch.go -- commitToPeer -->
<!-- source: internal/component/bgp/reactor/session_ownership.go -- recordSection -->

Duplicate suppression is performed under the final writer lock, after policy
and normalization, not by an API post-send record/forget bridge. Native semantic
keys deliberately omit labels, so local entries also retain exact NLRI evidence
when it cannot be recovered from that key. A label-only advertisement change
sends, an identical repeat suppresses, and a forwarded replacement clears the
local signature. Forwarded entries retain no attribute block.

Explicit diagnostic raw injection uses the same writer and ownership inventory,
but does not duplicate-suppress a requested UPDATE. Its existing AIGP policy,
PATHS-LIMIT admission and Label-Index normalization remain in force. Accountable
final bytes establish local ownership, so a remote source cannot withdraw them.
Opaque bytes accepted by those policies are emitted and flushed before Ze seals
and closes that exact Session with a diagnostic safety reason. They produce no
invented sent-route or AIGP receipt. This is a local sender policy, not an RFC
7606 receiver obligation; it avoids maintaining a live session with unknowable
outbound history. Ordinary forwarding never falls back to opaque injection.
Recognized full-packet UPDATEs retain their existing header reconstruction;
other full packets with uncertain framing or trailing bytes retire after their
literal emission. Successful emission returns success even when followed by
this reset; a failed write or flush retains the actual transport error.

The table follows the ordered buffered frontier. Pending ownership is visible
only to serialized writer admission until a successful flush commits it. Any
write or flush failure invalidates it, seals the failed Session and closes that
connection without recursively taking `writeMu`; a replacement Session is not
retired by the old writer's failure.
Existing sent callbacks still follow successful buffered acceptance, before
flush; they do not prove successful TCP delivery. Their asynchronous RIB
projection is never consulted for ordinary ownership. Only successful flush
commits the frontier and pending AIGP receipts. A failed session's earlier
callback cannot by itself authorize replay or recovery on its replacement.
The callback borrows a typed `wireu.SentOrigin` view; its scalar source/local
fields are copied into the existing event payload. The optional received-ADD-PATH
sidecar owns its backing storage, so returning the writer transaction to its
pool does not recycle event data. No receipt map is created or cloned for an
ordinary forward, and arbitrary caller metadata is passed through unchanged.
<!-- source: internal/component/bgp/reactor/adj_rib_out.go -- adjRIBOut, announceSignature -->
<!-- source: internal/component/bgp/reactor/session_write.go -- writeUpdateGated, writeRawUpdateBody, flushWrites -->


---

## *Params Struct Design

### Consistent Types (Unicast, VPN, LabeledUnicast)

```go
type UnicastParams struct {
    Communities       []uint32  // Typed - packed at Build time
    ExtCommunityBytes []byte    // Raw - complex encoding
    OriginatorID      uint32    // Typed - simple fixed format
    ClusterList       []uint32  // Typed - simple fixed format
}
```

### FlowSpec Exception

```go
type FlowSpecParams struct {
    CommunityBytes    []byte   // Raw - pre-packed by config loader
    ExtCommunityBytes []byte   // Raw - complex encoding
    OriginatorID      uint32   // Typed - simple fixed format
    ClusterList       []uint32 // Typed - simple fixed format
}
```

**Why FlowSpec uses `CommunityBytes []byte`:**
1. FlowSpec routes are config-originated, low-volume
2. Config loader pre-packs once at load time
3. Build path passes through without repacking
4. Negligible optimization, but intentional design

**This does NOT affect route reflection** - received FlowSpec routes forward from their WireUpdate payload like everything else.

---

## Domain Objects vs Params

| Layer | Purpose | Example |
|-------|---------|---------|
| Domain Objects | Store route config | `PluginRoute`, `StaticRoute` |
| *Params | Build UPDATE message | `PluginParams`, `UnicastParams` |
| Update | Wire format container | `Update{PathAttributes []byte}` |

**Conversion functions in `internal/component/bgp/reactor/peer_static_routes.go`:**
```go
func toPluginParams(r PluginRoute, fam family.Family) message.PluginParams
func toStaticRouteUnicastParams(r StaticRoute, nf bool) message.UnicastParams
func toVPNParams(r VPNRoute) message.VPNParams
```
<!-- source: internal/component/bgp/reactor/peer_static_routes.go -- toPluginParams, toStaticRouteUnicastParams -->

The exotic MP_REACH families (MUP/VPLS/MVPN/FlowSpec/SR-Policy) share `PluginRoute`
/ `PluginParams`; their NLRI and family-specific attributes are pre-built by the
family plugin, so the message package carries no per-family domain object or builder
for them. (`message.FlowSpecParams` / `BuildFlowSpec` survive only for the chaos orchestrator
load generator, not the config/API route path.)

---

## Update Struct

The `Update` struct holds raw bytes ready for wire:

```go
type Update struct {
    rawData         []byte  // Full message for passthrough
    WithdrawnRoutes []byte  // Withdrawn prefixes
    PathAttributes  []byte  // Packed attributes
    NLRI            []byte  // Announced prefixes
}
```

All `Build*()` methods produce an `Update` with populated `[]byte` fields.

---

## Scratch Contract (Builder and Splitter)

The `UpdateBuilder` and `Splitter` own a **reusable scratch buffer** that backs every
variable-size `[]byte` they emit. Understanding this contract is mandatory before
modifying any `Build*` method or consuming a returned `*Update`.

<!-- source: internal/component/bgp/message/update_build.go -- UpdateBuilder, scratch, alloc -->
<!-- source: internal/component/bgp/message/update_split.go -- Splitter, Split -->

### Why scratch, not `make`

Every `Update.PathAttributes` and `Update.NLRI` is a wire-facing variable-size byte
slice. Per `ai/rules/architecture.md` ("No `make` where pools exist"), such
allocations must come from a bounded pool. The builder's `scratch` IS that pool: one
buffer per builder, sized to `wire.StandardMaxSize` (4096) on first use, grown on
demand for Extended Message peers.

### Lifetime invariant (MUST understand before using)

| Object | Valid from | Invalidated by |
|--------|-----------|----------------|
| `update.PathAttributes` | return of `Build*` | next `Build*` call on the same builder |
| `update.NLRI` | return of `Build*` | next `Build*` call on the same builder |
| Splitter chunk's `PathAttributes` | fires via `emit(chunk)` | return of `emit` callback (next chunk's build reuses scratch) |

**Consequence:** callers MUST consume the Update (WriteTo, copy out, or hand to
SendUpdate which copies internally) before the next `Build*` on the same builder,
and splitter callers MUST complete `emit(chunk)` before the callback returns.

### Grow semantics (stranded backings are safe)

`alloc(n)` on overflow does `make(newSize) + copy + swap`. Sub-slices returned before
the grow still reference the OLD backing; the new backing holds everything allocated
after. The old backing stays alive (GC-pinned by those sub-slices) until all emitted
Updates are discarded. A single `Update` returned from one `Build*` call may therefore
have `PathAttributes` and `NLRI` pointing to two different arrays -- this is memory-safe
and byte-correct. Treat slices as opaque; never assume they share backing.

### Callback-builder offset protocol

`BuildGroupedUnicast` (and `Splitter.Split`) emit multiple Updates per outer call,
all sharing a subset of scratch. To keep this safe without re-building shared
attributes per chunk:

| Region | Offset | Lifetime |
|--------|-------|----------|
| attrBytes (shared across all chunks in batch) | `scratch[0:A)` | Full outer-call duration |
| per-Update NLRI / chunk PathAttributes | `scratch[A:)` | Until the callback returns |

After each callback returns, the builder resets `off` to **A** (the end of the shared
attribute region), NOT to 0. The next chunk's bytes overwrite the previous chunk's
NLRI region but leave the shared attribute region untouched, so the next emitted
Update still has a valid `PathAttributes` pointing at `scratch[0:A)`.

### Who owns a builder/splitter

| Role | Owner | Scope |
|------|-------|-------|
| UpdateBuilder | Short-lived, one per build-group | Created, used for a batch of builds, discarded |
| Splitter | Long-lived, one per peer (or forward worker) | Created at peer up, retained across sessions |

Builders are cheap to allocate; splitters amortise scratch across millions of
split operations and should NOT be created per-call.

---

## Context-Dependent Encoding

Wire format depends on negotiated capabilities:

| Capability | Effect |
|------------|--------|
| ASN4 | 2-byte vs 4-byte AS numbers in AS_PATH and AGGREGATOR. Without it, a non-mappable AS number goes out as AS_TRANS and the real value goes in AS4_PATH or AS4_AGGREGATOR beside it (RFC 6793 Section 4.2.2) |
| ADD-PATH | Path ID prefix in NLRI |
| Extended Message | >4096 byte messages |

Every builder appends AS_PATH through `UpdateBuilder.appendASPath` and AGGREGATOR
through `UpdateBuilder.appendAggregator`, which add the RFC 6793 companion
attributes. `attribute.AS4PathFor` and `attribute.AS4AggregatorFor` answer
whether one is owed, and the forwarding rails in `wireu` ask the same two
functions.
<!-- source: internal/component/bgp/message/update_build.go -- UpdateBuilder.appendASPath, UpdateBuilder.appendAggregator -->

**Build path:** `UpdateBuilder.Ctx` contains pack context
**Forward path:** `WireUpdate.SourceCtxID()` vs the destination's `sendCtxID` determines zero-copy eligibility

---

## Route Grouping (adj-rib-out → Peer)

When sending routes from adj-rib-out, routes with identical attributes are grouped into single UPDATE messages:

```
adj-rib-out Routes → GroupByAttributesTwoLevel() → ASPathGroups → BuildGrouped*()
       ↓                      ↓                         ↓              ↓
  []*rib.Route         []AttributeGroup           Same AS_PATH    Multiple NLRIs
                             ↓                     per UPDATE       per UPDATE
                        []ASPathGroup
```

**Complexity reduction:** O(routes) → O(routes/capacity)

| Family | Builder Method | Notes |
|--------|---------------|-------|
| IPv4 unicast | `BuildGroupedUnicastWithLimit()` | Uses UnicastParams |
| IPv6/VPN | `sendGroupedMPFamily()` | Packs into MP_REACH_NLRI |

**Files involved:**
- `internal/component/bgp/rib/grouping.go` - `GroupByAttributesTwoLevel()`, `RouteGroup`, `ASPathGroup`
- `internal/component/bgp/reactor/reactor.go` - `sendRoutesWithLimit()`, `sendGroupedIPv4Unicast()`, `sendGroupedMPFamily()`
- `internal/component/bgp/message/update_build.go` - `BuildGroupedUnicastWithLimit()`
- `internal/component/bgp/message/chunk_mp_nlri.go` - `ChunkMPNLRI()` for MP family splitting

**Config:** `group-updates true` (default) in peer settings.

Configured static grouping chooses legacy batching only after resolving each
route's next hop, and only for actual IPv4-unicast routes with native IPv4 next
hops. Extended IPv4 routes, including next-hop self resolving to IPv6, and
labeled/VPN routes use their existing per-route builders instead. Each such
successful send is recorded independently; a refused route does not suppress
its usable siblings. Genuine legacy batches keep their all-or-nothing recording.
<!-- source: internal/component/bgp/reactor/peer_static_wire.go -- sendStaticRoutesGrouped -->
<!-- test: internal/component/bgp/reactor/peer_static_group_next_hop_test.go TestStaticGroupedResolvedNextHopWire -->

The leaf governs every rail that sends several NLRIs to one peer, not the
adj-rib-out alone. `nlriUnitLen` turns it into framing, and the batch API rails
(`AnnounceNLRIBatch`, `WithdrawNLRIBatch`) and the LLGR readvertise rail each
read it there: with `group-updates false` a batch of N prefixes leaves as N
UPDATE messages carrying one prefix each.
<!-- source: internal/component/bgp/reactor/reactor_api_batch.go -- nlriUnitLen, announceBatchToPeers, withdrawBatchFromPeers -->

The announce batch retains an `announceTarget` containing both peer and Session.
Shared build facts include the destination family's ADD-PATH send mode from
`peer.sendCtx`; receive mode does not decide outbound NLRI framing. Every split
chunk goes through that captured Session, with the batch's replay flag, rather
than moving to a replacement connection during the send.
<!-- source: internal/component/bgp/reactor/reactor_api_batch.go -- announceTarget, announceFactsFor, announceBatchToPeers -->
<!-- source: internal/component/bgp/reactor/peer.go -- addPathFor -->
<!-- source: internal/component/bgp/reactor/peer_send.go -- sendUpdateWithSplit -->
<!-- source: internal/component/bgp/reactor/peer_initial_sync.go -- the config-driven sync reads the same leaf -->

---

## Cross-Peer Update Groups

Update groups are an orthogonal optimization that sits above route grouping. Where route grouping packs multiple NLRIs into a single UPDATE for one peer, update groups share a single UPDATE build across multiple peers.

### GroupKey

Each established peer is assigned a GroupKey combining two fields:

| Field | Source | Purpose |
|-------|--------|---------|
| `CtxID` | `peer.sendCtxID` (ContextID) | Encodes all encoding-relevant capability differences: ASN4, ADD-PATH mode, Extended Message, Extended Next Hop, iBGP/eBGP, and ASN values |
| `PolicyKey` | Currently `0` for all peers | Reserved for future per-peer outbound policy differentiation |

Peers with the same GroupKey produce bit-identical UPDATE wire bytes for the same route set and can share a single build.
<!-- source: internal/component/bgp/reactor/update_group.go -- GroupKey struct, CtxID and PolicyKey fields -->

### Group Lifecycle

The reactor maintains an `UpdateGroupIndex` that maps GroupKey to a set of member peers:

| Event | Action |
|-------|--------|
| Peer session established | Reactor calls `updateGroups.Add(peer)` using the peer's `sendCtxID` |
| Peer session closed | Reactor calls `updateGroups.Remove(peer)` before clearing encoding contexts |
| Group becomes empty | Group entry is deleted from the index |

The index is a simple map with no goroutines or channels. It is accessed only from the reactor event loop.
<!-- source: internal/component/bgp/reactor/reactor_notify.go -- Add on established, Remove on closed (before clearEncodingContexts) -->
<!-- source: internal/component/bgp/reactor/update_group.go -- UpdateGroupIndex, Add, Remove -->

### Group-Aware Build Path (AnnounceNLRIBatch / WithdrawNLRIBatch)

When update groups are enabled, the batch API groups peers by build-equivalent parameters (encoding context, next-hop resolution, AS_PATH form, and the peer's `group-updates` leaf) and builds the UPDATE once per parameter set. All peers sharing those parameters receive the same pre-built wire bytes.

The parameter set is `announceFacts`, and both rails use it. It is the argument set of the build AND the map key that forms the group, so a per-peer fact the build reads is a fact the key holds. A peer carrying `group-updates false` therefore cannot share a build with a peer that packs: the two receive a different number of messages from one batch.

The withdraw rail had a `withdrawFacts` of its own, carrying three fields, while a withdrawal was a bare MP_UNREACH_NLRI and nothing but the framing could tell two peers apart. A withdrawal now carries attributes (below), so every per-peer decision that shapes an announce's attribute block shapes a withdrawal's too. `withdrawFactsFor` leaves the attribute-shaping fields zero for a unicast withdrawal, which carries no attributes whatever the peer answers, so those peers still share one build.

When disabled or when each peer has a unique context, the code falls back to per-peer building with no behavior change.
<!-- source: internal/component/bgp/reactor/reactor_api_batch.go -- groupsEnabled check, announceFacts, withdrawFactsFor -->

### Local Duplicate Evidence in the Shared Adj-RIB-Out

RFC 4271 Section 9.2: "A BGP speaker SHOULD NOT advertise a given feasible BGP route from its Adj-RIB-Out if it would produce an UPDATE message containing the same BGP route as was previously advertised."

Every output rail now records through the same final writer. Local advertisements
retain duplicate evidence; forwarded replacements clear it. A second local
announce with identical final bytes sends nothing, but an API build cannot
suppress a route before export policy or independently record a successful send.

| Question | Answer |
|----------|--------|
| What is the key | Registered native semantic identity plus outgoing ADD-PATH presence/ID; VPN RD is included and labels/Compatibility are excluded |
| What is compared | The final normalized attribute signature plus exact native NLRI evidence where the semantic key loses wire bytes; MP_REACH payload and its length octets do not make an individual route depend on batch size |
| What empties it | An admitted withdrawal removes its route; a session retirement invalidates the whole table |
| What is never duplicate-suppressed | An authorized withdrawal, and an admitted batch carrying `NLRIBatch.Replay` |
| What an operator sees | A session debug line naming the peer and suppressed path count, plus the existing per-peer suppression counter |

`Replay` keeps an admitted resend reaching the wire instead of satisfying a
refresh with duplicate suppression. It is not ownership authority. Same-session
resend and ROUTE-REFRESH carry the stored sent-message receipt and preserve the
current owner and revision. Deliberate peer-up replay instead carries the
captured new-Session initial-sync token and explicit local or source/path origin;
it may populate only an empty slot while that initial phase remains open.
See [replay cursor](bgp/replay-cursor.md) for the distinct initial and refresh receipts.

Cursor replay clears prior cursor state before its first group and after its
last group. Each emitted command carries `replay: true`, the captured sent
message-ID receipt and the group's stale level when present. Receipts travel as
lossless decimal strings; stale replay or cleanup cannot overwrite a newer
writer owner. Replay preserves the existing ownership and revision because the
projection ignores replay feedback. ADD-PATH framing remains explicit even for
identifier zero: `formatCursorCommands` emits `path-information` whenever the
stored route has ADD-PATH enabled.
The existing sent event also retains the stable source-Peer incarnation and
received path identity needed when destination encoding removed it. Its scalar
origin is body-common; extra family/announcement ordinals are emitted only for
received ADD-PATH paths. Source attributes are not copied into the writer table.
<!-- source: internal/component/bgp/plugins/rib/rib_replay.go -- resendRoutesWithCursor, formatCursorCommands -->

RFC 2918 Section 4: "Otherwise, the BGP speaker shall re-advertise to that peer the Adj-RIB-Out of the <AFI, SAFI> carried in the message, based on its outbound route filtering policy." That "shall" is why the refresh rail outranks Section 9.2 here. Until 2026-09-14 `sendRoutes` set no marker, so a refresh on an up session sent the RFC 7313 BoRR and EoRR with no UPDATE between them, and RFC 7313 Section 4 has the receiver purge on the EoRR every route the BoRR marked stale: the refresh withdrew the family instead of restoring it. `test/plugin/plugin-refresh.ci` is the recording.

Config-driven initial sync, transaction sends and API origination all establish
local ownership at the same final boundary. The old API-only post-send record
and pre-send forget bridges no longer exist.
<!-- source: internal/component/bgp/reactor/adj_rib_out.go -- adjRIBOut, announceSignature -->
<!-- source: internal/component/bgp/reactor/session_ownership.go -- beginAdjOut, allow, recordSection -->
<!-- source: internal/component/bgp/plugins/rib/rib_replay.go -- resendRoutesWithCursor -->
<!-- source: internal/component/bgp/plugins/rib/rib_commands.go -- sendRoutes -->

### A Withdrawal Names a Route This Connection Advertised

RFC 4271 Section 4.3 identifies a withdrawn route by its destination, "which unambiguously identifies the route in the context of the BGP speaker - BGP speaker connection to which it has been previously advertised."

A connection that has advertised nothing has no route for any withdrawal to name, so the API rail names no route to such a peer. The condition is about the CONNECTION, not about the route: once the connection has carried one UPDATE that makes any destination reachable, every later withdrawal is written, whether or not the peer holds the route named. A key-based rule would be a different rule and a wrong one, because an operator withdrawing a route the peer never received is telling a peer it must not hold it, and RFC 4271 Section 9.1.2 has the receiver ignore a withdrawal for a route it does not have.

What the peer receives is the withdrawal with its routes removed: the path attributes alone, with no MP_UNREACH_NLRI, no Withdrawn Routes and no NLRI. RFC 4271 Section 6.3: "An UPDATE message that contains correct path attributes, but no NLRI, SHALL be treated as a valid UPDATE message." No RFC asks a speaker to SEND it. It is the ExaBGP compatibility contract, and upstream reaches it the same way: `include_withdraw` drops each withdrawn NLRI while it is False and `UpdateCollection.messages` yields the packed attributes regardless. `api-flow` is the recording, and its second frame is that message.

A family whose withdrawal carries no attributes of its own is written NOTHING, because removing the routes leaves an empty UPDATE rather than an attributes-only one. IPv4 unicast and the multiprotocol unicast families are that case, and upstream sends nothing for them too, which `api-fast` records.

The message names no route, so it does not arm the connection either: a second withdrawal is withheld exactly as the first was.

| Question | Answer |
|----------|--------|
| Where the state lives | `Session.advertised`, so a new connection starts unset and there is nothing to clear at teardown |
| What arms it | Any UPDATE carrying NLRI or MP_REACH_NLRI, written at any of the three points a message reaches the socket. A route the RIB forwarded arms it exactly as an API announce does |
| What does not arm it | An End-of-RIB (RFC 4724 Section 2) and a pure withdrawal: neither makes a destination reachable |
| What an operator sees | A warning on `subsystem=bgp.routes` naming the peer, the family, the count and the RFC section; a per-peer counter beside the suppressed count; and `route.ErrWithdrawWithheld` on the command's answer, naming every peer it was withheld from |

The answer is a WARNING and never a failure. The command did what it asked for, so `send bgp <selector> update text ... nlri <family> del <nlri>` still answers `done` with the reason in its warnings. A zero UPDATE count with a bare `done` and no reason is the silent no-op this rail exists to avoid.

ExaBGP has the same asymmetry, reached another way: `include_withdraw` starts False for each session and becomes True only when the first update pass exhausts, and its packing layer drops every withdrawn NLRI while it is False. `api-fast` is the recording: the withdrawal its first burst writes never reaches the wire, and the one its second burst writes does.
<!-- source: internal/component/bgp/reactor/session_write.go -- Session.advertised, noteAdvertised, updateIsReachable -->
<!-- source: internal/component/bgp/reactor/reactor_api_batch.go -- withdrawBatchFromPeers, buildWithheldWithdrawUpdate, logWithdrawWithheld -->

### What a Withdrawal Carries

RFC 4760 Section 4: "An UPDATE message that contains the MP_UNREACH_NLRI is not required to carry any other path attributes." Both shapes are conformant, so the family decides which one Ze sends.

| Family | Withdrawal shape |
|--------|------------------|
| IPv4 unicast | Withdrawn Routes field (RFC 4271 Section 4.3), no path attributes |
| IPv6 unicast | Bare MP_UNREACH_NLRI, no other path attributes |
| Every other family | MP_UNREACH_NLRI plus the block `planBatchAttrs` plans for an announce: the caller's own attributes, the well-known mandatory ORIGIN and AS_PATH, the RFC 4271 Section 5.1.5 LOCAL_PREF toward an internal peer, and the legacy NEXT_HOP where the family carries one |

Unicast is bare because the Withdrawn Routes field carries no attributes and the IPv6 unicast withdrawal is the same withdrawal in the RFC 4760 encoding: the AFI must not decide what `withdraw <prefix> next-hop X local-preference 200` means. ExaBGP splits them at the same seam.

Until 2026-09-06 every withdrawal took the bare shape, so `send bgp <selector> update text <attributes> nlri <family> del <nlri>` was acknowledged and the attributes never reached the wire.
<!-- source: internal/component/bgp/reactor/reactor_api_batch.go -- buildBatchWithdrawUpdate, planBatchAttrs -->

### The Legacy NEXT_HOP Beside MP_REACH_NLRI

RFC 4760 Section 3: "An UPDATE message that carries no NLRI, other than the one encoded in the MP_REACH_NLRI attribute, SHOULD NOT carry the NEXT_HOP attribute." Ze intentionally retains the extra attribute for the compatibility families declared by `family.Family.LegacyNextHop`, matching its ported ExaBGP contract fixtures. This records a compatibility deviation, not conformance credit merely because the requirement says SHOULD NOT. The same section says receiving speakers SHOULD ignore the extra attribute.

Multicast is the family whose answer reads as an exception and is not one. It shares `UnicastParams` with unicast, and `BuildUnicast` writes the attribute only under `isUnicast := p.SAFI == 0 || p.SAFI == attribute.SAFIUnicast`, so the config rail sends MP_REACH_NLRI alone for `ipv4/multicast`. No ported ExaBGP contract fixture pins the other answer, so the RFC's SHOULD NOT stands.

The address must also be an IPv4 address that RFC 4271 Section 6.3 calls syntactically correct, so an IPv6 next hop and `0.0.0.0` each contribute nothing (`legacyNextHopApplies`).

The config rail already sent it -- `message.(*UpdateBuilder).BuildVPN`, and the `nlri/mvpn` and `nlri/mup` config parsers -- while the API rail sent MP_REACH_NLRI alone, so one route reached the wire as two different byte strings depending on whether an operator configured it or announced it.
<!-- source: internal/core/family/family.go -- Family.LegacyNextHop -->
<!-- source: internal/component/bgp/reactor/reactor_api_batch.go -- legacyNextHopApplies -->

### Group-Aware Forward Path (ForwardUpdate)

When forwarding a received UPDATE to multiple peers, the forward path caches the per-context body computation. For peers sharing the same destination context, the context compatibility check and any re-encoding happen once. The cached result is reused for all group members.
<!-- source: internal/component/bgp/reactor/reactor_api_forward.go -- fwdBodyCache, fwdBodyCacheKey -->

### Env Var Gating

Update groups are controlled by `ze.bgp.reactor.update-groups` (boolean, default `true`). The reactor reads this at startup via `NewUpdateGroupIndexFromEnv()`. When false, the `UpdateGroupIndex` reports disabled and all group-aware code paths fall back to per-peer behavior.

ExaBGP migrated configs inject `update-groups false` in the environment reactor block to preserve ExaBGP's per-peer UPDATE semantics.
<!-- source: internal/component/bgp/reactor/update_group.go -- NewUpdateGroupIndexFromEnv, Enabled -->
<!-- source: internal/exabgp/migration/migrate.go -- injectUpdateGroupsDisabled -->

### Relationship to Route Grouping

| Concept | Scope | Config | Purpose |
|---------|-------|--------|---------|
| Route grouping | Within one UPDATE for one peer | `group-updates` per peer | Pack multiple NLRIs with identical attributes into one UPDATE |
| Update groups | Across peers | `ze.bgp.reactor.update-groups` global | Build UPDATE once, send to all peers with same encoding context |

Both optimizations can be active simultaneously and are independent.

---

## Summary

```
┌─────────────────────────────────────────────────────────────────┐
│                    BUILD PATH (Local Origination)               │
│                                                                 │
│  Config → PluginRoute → PluginParams → BuildPlugin()           │
│                ↓              ↓               ↓                 │
│        plugin-built NLRI  (pass-through)  Update{[]byte}        │
│                                                                 │
│  Volume: Low (tens of rules)                                    │
│  Optimization: Plugin pre-builds NLRI + attrs at config time    │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│                    FORWARD PATH (Route Reflection)              │
│                                                                 │
│  Receive → WireUpdate{payload, sourceCtxID} → contexts match?   │
│                        ↓                          ↓             │
│         Route stored in RIB       YES: forward the payload      │
│         (it holds no wire cache)  NO:  rebuild for destCtxID    │
│                                                                 │
│  Volume: High (millions of routes)                              │
│  Optimization: Zero-copy when contexts match                    │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│                    GROUPED SEND PATH (adj-rib-out)              │
│                                                                 │
│  Routes → GroupByAttributesTwoLevel() → BuildGrouped*WithLimit()│
│     ↓              ↓                            ↓               │
│  []*Route    []ASPathGroup              Multiple NLRIs/UPDATE   │
│                                                                 │
│  Volume: Medium (adj-rib-out replay, API announces)             │
│  Optimization: O(routes) → O(routes/capacity) UPDATEs           │
└─────────────────────────────────────────────────────────────────┘
```

---

## UPDATE Size Limiting

UPDATEs must respect max message size (4096 standard, 65535 with Extended Message). Two approaches:

### Option A: Proactive (Build Path)
```go
// Size-aware builder splits at build time
updates, err := ub.BuildGroupedUnicastWithLimit(params, maxSize)
for _, update := range updates {
    peer.SendUpdate(update)
}
```

### Option B: Reactive (Forward/Replay Path)
```go
// Split after building if oversized
update := buildRIBRouteUpdate(route, ...)
peer.sendUpdateWithSplit(update, maxSize, family)
```

> **Wire-Level Split (Implemented)**
>
> The send path uses `Splitter.Split` for oversized UPDATEs and keeps every
> chunk on the Session captured for the build. Same-context
> forwarding uses `wireu.SplitWireUpdate`. Cross-context forwarding re-encodes
> the UPDATE and uses `Splitter.SplitCompliant` to separate mixed NLRI-bearing
> fields. See `plan/learned/DESIGN-HISTORY.md`, "BGP engine: wire encoding and
> RIB" (retired summary 078).

Both forwarding splitters omit an empty MP_UNREACH field when the input also
carries a nonempty legacy withdrawal, nonempty legacy NLRI, or an MP_REACH
attribute. Emitting that empty field separately would invent an End-of-RIB
marker under RFC 4724 Section 2. Genuine standalone markers remain intact.
The parsed splitter still validates the MP envelope and its attribute budget
before deciding whether to omit the empty field.
<!-- test: internal/component/bgp/message/rfc7606_shape_test.go TestSplitCompliantEmptyMPUnreachKeepsMessageMeaning, TestSplitCompliantEmptyMPUnreachStillValidates -->

**Files involved:**
- `internal/component/bgp/message/update_split.go` - `Splitter.Split()` and `Splitter.SplitCompliant()`
- `internal/component/bgp/wireu/split.go` - `SplitWireUpdate()` for same-context forwarding
- `internal/component/bgp/message/chunk_mp_nlri.go` - `ChunkMPNLRI()` for family-aware NLRI parsing
- `internal/component/bgp/reactor/peer_send.go` - `sendUpdateWithSplit()` integration through `Splitter.Split`
- `internal/component/bgp/reactor/forward_body.go` - `buildFwdBody()` selects the forwarding split path and `fwdSplitParsedUpdate()` applies `Splitter.SplitCompliant`
<!-- source: internal/component/bgp/message/update_split.go -- Splitter.Split, Splitter.SplitCompliant -->
<!-- source: internal/component/bgp/wireu/split.go -- SplitWireUpdate -->
<!-- source: internal/component/bgp/reactor/peer_send.go -- sendUpdateWithSplit -->
<!-- source: internal/component/bgp/reactor/forward_body.go -- buildFwdBody, fwdSplitParsedUpdate -->
<!-- source: internal/component/bgp/message/chunk_mp_nlri.go -- ChunkMPNLRI -->

`ChunkMPNLRI` and `SplitMPNLRI` use the family's registered native framing
through `nlrisplit.GetWithdraw`, including negotiated ADD-PATH. The withdrawal
walker also frames announcements without treating the labeled Compatibility
field as an S-bit-terminated label stack. Unsupported families return an error;
there is no CIDR fallback or separate family-size table. Chunking returns
zero-copy slices, while splitting stops after the first complete route that
does not fit instead of rescanning the remaining tail.
See [MP-NLRI ordering](wire/mp-nlri-ordering.md#implementation-notes) and
[native NLRI framing](wire/nlri.md#add-path-decoding-for-plugin-families).
<!-- source: internal/component/bgp/message/chunk_mp_nlri.go -- ChunkMPNLRI, SplitMPNLRI -->

**MP Attribute Ordering:** See [MP-NLRI ordering](wire/mp-nlri-ordering.md) for MP_REACH/MP_UNREACH placement at the end of PathAttributes.

---

## Related Documentation

- `encoding-context.md` - Context system for capability-dependent encoding
- `pool-architecture.md` - Attribute/NLRI deduplication pools
- `message-buffer-design.md` - Passthrough message handling
- `wire/messages.md` - Wire format specification
- `wire/mp-nlri-ordering.md` - MP attribute ordering rationale

## Related Specs

Summaries 057, 059, 078 and 343 were retired on 2026-08-01. Their surviving
knowledge is in `plan/learned/DESIGN-HISTORY.md`, "BGP engine: wire encoding
and RIB".

- DESIGN-HISTORY > Load-bearing invariants - lazy-parsed wire attribute
  storage on the forward path: `AttributesWire` does not own its `packed
  []byte` (057)
- DESIGN-HISTORY > Abandoned approaches - pool handle integration was designed
  and abandoned, not shipped (059)
- DESIGN-HISTORY > Evolution and Load-bearing invariants - buffer pool
  get/return lifecycle, keyed on `cap(buf)` (343)
- DESIGN-HISTORY > Evolution and Load-bearing invariants - wire-level UPDATE
  splitting, its "accept invalid, emit valid" posture and its progress guard
  (078)

---

**Created:** 2026-01-01
**Last Updated: 2026-01-30
