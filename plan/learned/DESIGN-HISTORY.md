# Design History

A chronological-by-subsystem map of how ze's code got to its current shape.
Read this document first when the question is "why is X structured this way?".

The document covers the whole history, and it holds that history in two
different ways. Summaries 001 to 400 were retired on 2026-08-01, and their
surviving knowledge was merged into the sections below. For that era this
document IS the record, not an index to one. For the 888 summaries numbered
401 and above, each section links the files that hold the full detail.

A bare three-digit number at or below 400, such as (059), names a retired
summary. That file is no longer in this directory and nothing here depends on
it. To read the original, `git log --diff-filter=D --name-only -- plan/learned/`
names the commit that deleted the corpus, and `git show <sha>^:<path>` prints the
file.

See also: `METHODOLOGY.md` (how individual summaries are written),
`../../ai/LEARNED-INDEX.md` (curated index by topic).

## How to read this document

Each subsystem section has four fixed parts:

1. **Current shape**: one paragraph describing what exists today.
2. **Evolution**: the phases that produced the current shape.
3. **Abandoned approaches**: designs that were tried and removed. Read the
   entry before you propose one of them a second time.
4. **Load-bearing invariants**: facts about the code that are easy to break
   by accident. Each invariant names the code site that enforces it and the
   reason it exists.

A linked number (401 and above) points at a summary that holds more detail
than the entry here. An unlinked number (400 and below) does not, because the
entry beside it is the record.

---

## Era and phase summary (for orientation)

| Era | Range | Theme |
|-----|-------|-------|
| Foundation | 001-100 | BGP engine port from ExaBGP. Zero-copy, pool dedup, lazy parsing, config migration from ExaBGP syntax to YANG. |
| Buffer-first + Plugins | 101-200 | Pack() to WriteTo(buf, off), UpdateBuilder, hub separation, YANG as sole schema, plugin restructure (`ze.X` in-process plugins, 5-stage startup). |
| Plugin Architecture Maturity | 201-300 | YANG-driven IPC, two-socket RPC, SDK callback pattern, config reload coordinator, bgp-chaos, RIB plugin with per-attribute dedup + refcounted cache, RFC 7606 enforcement. |
| Arch-0 Restructuring + RIB Expansion | 301-400 | Tier-ordered startup, dependencies, UTP (unified text protocol), allocation reduction, 4-component system boundary (Engine, ConfigProvider, PluginManager, Subsystem), backpressure, panic recovery, LLGR, RPKI, editor modes, web UI. |
| Filter Framework + Ops | 401-500 | Prefix limits, filter framework, OTC/role/RPKI decorators, RBAC, per-user drafts with mtime conflict detection, ze-perf benchmarking, interface management, config archive, DNS, zefs blob namespaces. |
| Protocol Expansion | 501-640 | BART RIB, family registry, L2TP + PPP stack, BFD, VPP backend, firewall (nft), gokrazy appliance, sysctl, host inventory, RS fastpath, structured event bus migration (bus absorbed into stream system), TACACS, user login. |

---

## BGP engine: wire encoding and RIB

### Current shape

One reactor per `ze bgp` process, goroutine-per-peer, shared pools for
attribute deduplication. UPDATE reading is lazy via `WireUpdate`
iterators over raw wire bytes. Encoding is buffer-first via
`WriteTo(buf, off) int` into pooled buffers. A peer has two
`EncodingContext`s (recv, send), each identified by a `ContextID uint16`
in a global registry; zero-copy forwarding is a single integer
comparison. Best-path selection lives in the `bgp-rib` plugin, not the
engine; the engine has no Loc-RIB table.

### Evolution

- **001** Foundation plan. Zero-copy
  forwarding via shared encoding context; goroutine-per-peer replaces
  ExaBGP's Python async reactor; per-attribute pools with mutex-based
  typed stores (later replaced by pool handles).
- **003, 124, 174, 176, 332**
  Pool evolution. Single-pool → per-attribute-type pools (22 codes) →
  unified handle layout (bufferBit | poolIdx | flags | slot) → flags
  removed (slot widened to 26 bits) → double-buffer compaction scheduler
  wired into RIB plugin lifecycle. 059 is NOT part of this chain; it was
  designed and abandoned, and it is recorded under Abandoned approaches.
- **173** Plugin RIB NLRI storage splits by family on size. IPv4 unicast
  prefixes are 1 to 5 wire bytes, smaller than a 4-byte pool handle plus
  index, so they are stored directly. IPv6, VPN and EVPN are large enough
  for pool dedup to pay.
- **011** API commits always carry a name. Anonymous commits were rejected
  so one client can hold several concurrent batches. Grouping is decided by
  `rib { group-updates }`, never by the commit command: commit controls
  WHEN, RIB config controls HOW.
- **014** Two-level grouping (non-AS_PATH attributes, then AS_PATH) was
  chosen over folding AS_PATH into one key, so every route in an
  AttributeGroup shares a single `[]Attribute` slice while carrying
  different AS_PATHs.
- **086** IPVPN is deliberately excluded from the `PrefixNLRI` embedding
  that INET and LabeledUnicast share. Its wire order is family, rd, labels,
  prefix, pathID, so embedding would change the wire format.
- **343** Session reads use two fixed pools (4K, 64K) rather than one mixed
  pool. 4K is correct before negotiation because RFC 4271 caps OPEN at 4096;
  64K activates only once Extended Message is negotiated.
- **062** Size limiting exists because `sendInitialRoutes` silently skipped
  oversized UPDATEs, losing routes with no error and no log. Two mechanisms
  remain, and neither replaces the other: proactive limiting in the builder,
  reactive splitting for received UPDATEs.
- **078** Splitter posture is "accept invalid, emit valid". A caller may send
  several MP_REACH for different AFI/SAFIs, which is technically invalid, and
  the splitter normalises to at most one MP_REACH plus one MP_UNREACH per
  UPDATE. It is left unoptimised because it runs only past the size limit.
- **090** In the update grammar one `add` or `del` carries exactly one
  FlowSpec rule, unlike prefix families. FlowSpec components combine with AND
  to define a single rule, so batching disconnected rules has no meaning.
- **081** `rd` and `label` are per-NLRI parameters inside the `nlri` section,
  never global attributes, because an RD belongs to individual prefixes.
  AS_PATH is set-only: `add` and `del` would compromise path integrity.
- **116** The `EncodingCaps` and `SessionCaps` split follows one rule: a
  capability is EncodingCaps if and only if it changes the bytes on the wire.
  `ExtendedMessage` moved on that basis, and `EncodingContext` gained
  `MaxMessageSize()`.
- **253** The `message` package stays free of NLRI-plugin imports because
  family params accept a pre-built `NLRI []byte` rather than typed values.
  SAFI constants stayed in core `nlri` for the same reason: `ParseFamily()`
  needs them.
- **400** For the BGP-LS attribute details the RFCs leave ambiguous, GoBGP's
  `pkg/packet/bgp/bgp.go` was read as the oracle <!-- doc-links: ignore (GoBGP's own tree, not a ze path) -->: 3-byte range padding, V/L
  flag length control, sub-TLV nesting in SR Capabilities.
- **392, 394, 278** Forward congestion. The bounded forward-overflow pool is
  a channel-based token semaphore, chosen over a pre-allocated backing array
  to leave the value-copy item flow unchanged, and it is SOFT-bounded:
  exhausted tokens fall back to unbounded growth. Hard bounding comes only
  from read-throttle and teardown reducing inflow. Read throttle clamps sleep
  to keepalive/6, over /3 or /2, so a throttled source still delivers one
  message inside the hold time, and it uses four fill-level bands rather than
  a linear ramp so a low-ratio source is throttled only at critical fill.
  Backpressure on the forward path is deliberately IMPLICIT: a blocking
  `fwdPool.Dispatch` blocks the ForwardUpdate RPC, which blocks the RS
  worker, which fills its channel, which pauses the source peer. That
  blocking chain IS the signal path.
- **073, 075,
  092, 102,
  103, 114,
  115, 116** Wire
  encoding migration. `Pack() []byte` allocated; replaced by
  `WriteTo(buf []byte, off int) int` with `CheckedWriteTo` variants for
  overflow detection. Session holds `writeBuf []byte` sized 4K pre-OPEN,
  resized to 65535 after Extended Message negotiation.
- **076, 078,
  079,
  204** WireUpdate design. Raw wire
  bytes held, attributes lazy-parsed via AttributesWire. `UpdateSections`
  stores offsets (integers), not data slices.
- **034,
  037,
  038,
  039,
  063,
  112,
  113** Encoding context unification.
  Three Family types (nlri, capability, context) collapsed to one;
  `PackContext` → `WireContext` → `EncodingContext`. Per-peer recv/send
  contexts because ADD-PATH is the only asymmetric capability. Hash
  collision-resistant FNV-64a for registry dedup. `ContextID uint16`
  vs. pointer saves 6 MB at 1M routes.
- **014,
  030,
  032,
  061,
  062,
  097** UpdateBuilder. Routes grouped by
  non-AS_PATH attributes then by AS_PATH; MP_REACH NEXT_HOP lives in
  the attribute, not the base attribute section. Size limits enforced
  at the RIB-to-send boundary via `BuildGroupedUnicastWithLimit`
  (multi-route, splits) and `*WithMaxSize` (single route, errors).
- **093,
  220,
  377,
  450** Split-function zero-copy. `SplitMPNLRI`
  returns subslices of the original buffer. `ChunkMPNLRI` subsumed into
  `iter.Elements`.
- **070,
  071,
  084,
  086** NLRI refactoring.
  `Len()` and `WriteTo()` return payload only; caller prepends path-id
  via `WriteNLRI` helper. `hasPath bool` on NLRI struct removed.
  `Bytes()` (identity/RIB keys) and `Pack()` (wire) are different
  concerns.
- **173,
  253,
  296,
  297,
  301,
  303,
  304,
  317,
  340,
  374,
  384,
  387,
  534,
  607,
  618** RIB plugin arc. RIB commands
  originally engine builtins; moved to plugin-provided via registry
  dispatch. adj-rib-in plugin stores raw wire bytes for zero-copy
  replay. `seqmap` library gives O(log N + K) delta queries. Refcounted
  cache replaced TTL — time-based eviction silently drops routes when
  plugin is slow. RIB show/best moved from separate commands to
  pipeline iterator model. Storage backend: map (default) and BART
  (exact-match trie) for best-path store. `bestPrevStore` interner
  collapses per-route state to `uint16` indices.
- **254** RFC 7606. Severity ordering via
  iota: None / AttributeDiscard / TreatAsWithdraw / SessionReset.
  Numeric comparison gives strength ordering. attribute-discard and
  zero-copy are incompatible — motivated `draft-mangin-idr-attr-discard-00`.
- **275,
  276,
  277,
  278,
  289,
  292,
  316,
  392,
  394,
  424,
  445,
  457,
  630** Forward path evolution. Per-destination-peer worker goroutines,
  MuxConn for concurrent RPCs (`#id <verb> [<json>]\n`), EBGP wire
  variants cached per-UPDATE (ASN4 vs ASN2 prepend), four-layer
  congestion response (bounded overflow, Prometheus metrics, read
  throttle, teardown), fire-and-forget `ForwardUpdatesDirect` typed
  path for internal plugins.
- **239,
  280,
  408,
  442,
  464,
  541,
  548,
  552,
  569,
  570,
  571,
  590,
  591,
  593** Filter and policy framework.
  Capability mode (enable/disable/require/refuse) post-negotiation
  validation. Dynamic event/send type registration from plugins.
  OTC (RFC 9234), prefix-list, as-path, community-match, route-modify
  filter plugins. `apply_mods` attribute modification framework.

### Abandoned approaches

- **AS-PATH as NLRI indexing** (001) — proposed as novel index, listed
  as risk, never carried into production.
- **Span type** (102, 117) — introduced for compact offset storage,
  removed as over-engineered; native `[]byte` subslice suffices.
- **Adj-RIB-Out integration in engine** (060, 068) — reversed.
  Persistence delegated to external API programs (current RIB plugin).
- **`PathAttributes` intermediate struct** (105) — text → struct → wire
  bytes, replaced by `attribute.Builder` for text → wire bytes directly.
- **Cobra + Viper for CLI** (001) — rejected for stdlib `flag.FlagSet`.
- **Freeform config parsing** (001, 166, 281) — extracted content without
  schema; drove the YANG-as-sole-schema redesign.
- **`rib enter-llgr` / `rib depreference-stale` dedicated commands** (407)
  — rejected in favour of composable `attach-community` / `delete-with-community`
  / `mark-stale [level]`.
- **`forward-ebgp` command** (277) — rejected; engine already knows peer
  types, a single branch in `ForwardUpdate` replaces the command.
- **`BuildGroupedUnicast` for RIB routes** (171) — conversion cancelled;
  RIB routes have existing wire bytes (zero-copy forwarding), only
  locally-originated routes use `UpdateBuilder`.
- **Direct-wire in-process delivery, first attempt: an eager
  `StructuredEvent`** (321, 322): it pre-computed the filter result,
  including the NLRIs, at delivery time. Rejected for violating lazy-first:
  the consumer may need none of it.
- **Direct-wire in-process delivery, second attempt: an `UpdateHandle`
  wrapper** (321, 322): a struct holding the raw message plus accessor
  methods. Rejected as an identity wrapper. These are two distinct failures
  with two distinct causes, and the third attempt succeeded by passing the
  raw message and using the existing wire iterators, which was also the
  simplest. Engine delivers `*RawMessage` directly via DirectBridge; the
  plugin walks wire bytes with existing iterators.
- **Pool handle migration for `Route`** (059): moving `Route` from
  `[]byte` to a single pooled handle, interning into a reusable connection
  buffer in RIB mode and allocating per read in API mode, was designed and
  then ABANDONED. Heavy dedup belongs in an API-level route reflector, not
  in the edge speaker. Do not read 059 as a shipped step in the pool chain.
- **The accumulator model in the `update text` parser** (306): `set`,
  `add` and `del` verbs with mid-stream modification, removed for flat
  keyword-value parsing. Attributes must precede every `nlri`, an attribute
  after the first `nlri` is rejected, and a leftover `set` returns a
  migration hint rather than being ignored.
- **`AddPathReceive map[string]bool` on the per-message struct** (100):
  proposed so plugins could see ADD-PATH state. Rejected:
  `AttributesWire.SourceContext()` already reaches the `EncodingContext`,
  which already holds per-family ADD-PATH.
- **`io.Writer` as the wire-encoding interface** (073): rejected for
  `[]byte` plus offset: an extra interface dispatch per call, and no
  alignment with the pooled-buffer pattern. `Pack()` was kept alongside
  `WriteTo` during the migration as an additive step.
- **`WriteTo(buf, off) int` for NLRI splitting** (377): rejected.
  `WriteTo` is the encoding contract, and splitting operates on data already
  in a wire buffer, where returning subslices is simpler and faster. This is
  the standing exception to the buffer-first instinct.
- **Emitting path attributes in non-RFC order to match ExaBGP fixtures**
  (042): added to the UPDATE builder, then reverted. ExaBGP emitted type 25
  before 14, the fixtures were wrong, and the fixtures were rewritten instead
  of the encoder.

### Load-bearing invariants

| Invariant | Site | Why (first occurrence) |
|-----------|------|------------------------|
| A peer has two `EncodingContext`s, never one | `peer.go` (`recvCtx`, `sendCtx`) | ADD-PATH is the only asymmetric capability; single context would conflate recv/send semantics. (038) |
| `ContextID` is `uint16`, not a pointer | `internal/core/bgp/context/` | Saves ~6 MB at 1M routes, single-integer zero-copy comparison. (038, 039) |
| `Bytes()` (identity/RIB key) and `Pack()`/`WriteTo()` (wire) are separate | NLRI types | Wire excludes path-id (caller prepends); RIB keys include path-id for uniqueness. Confusing them corrupted EVPN ADD-PATH once. (070, 071) |
| attribute-discard and zero-copy are incompatible | `rfc7606.go` | Stripping bytes from wire encoding breaks zero-copy forwarding. Solution is a new path attribute, not in-place stripping. (254) |
| NLRI overrun → session reset, not treat-as-withdraw | `rfc7606.go` | The NLRI is not parseable, individual prefix withdrawal is impossible. (254) |
| MP_REACH + MP_UNREACH can carry up to 3 distinct families per UPDATE | RFC 4760 | Per-family wire splitting not done in engine `ForwardUpdate`. (289) |
| Hold time 0 disables throttling entirely | `ReadThrottle.ThrottleSleep` | RFC 4271 §4.4: hold-time 0 means no timers, no safe sleep budget. (394) |
| Best-path lives in bgp-rib plugin, on-demand | `internal/component/bgp/plugins/rib/` | Real-time deferred until export policy exists; engine has no Loc-RIB. (374) |
| Per-source-peer worker pool serializes FIFO | bgp-rr `forwardWorker` | Cache ack protocol requires FIFO message-id order; unbounded goroutines caused ~98% route loss. (269) |
| Engine cache ack is CUMULATIVE: acking id N implicitly acks 1..N-1 | recent-update cache ack | The mechanism behind the ~98% route loss at 522K routes. With concurrent forwarders a later id arriving first evicts earlier entries, so a forwarder must preserve FIFO id order, not merely deliver every id. (269) |
| The route-grouping key must include ORIGINATOR_ID and CLUSTER_LIST | `routeGroupKey` in the UPDATE build path | Omitting them silently merges routes carrying different reflector attributes. `RawAttributes` is deliberately excluded: custom attributes are rare and near-unique. (030) |
| Forwarding compares `updateSize` against the destination peer maximum BEFORE zero-copy | reactor forward path | A 65535-byte UPDATE from an Extended Message peer cannot go verbatim to a 4096-byte peer, so the size check precedes the context-ID comparison. (060) |
| The attribute pool buffer is append-only for writes | attrpool, `freeSlots` | Slot reuse recycles indices but never reclaims byte gaps, so compaction is the only mechanism that returns buffer memory. Slot reuse without a running scheduler grows forever. (332) |
| AS-PATH rewrite byte shift can be positive, zero, OR negative | `RewriteASPath` in wireu | ASN4-to-ASN2 transcoding can shrink ASNs by more than the prepend adds, so a caller assuming the patched wire is never shorter is wrong. (277) |
| Each cached wire variant needs its own lazily-parsed UPDATE struct | eBGP/iBGP branch of forward | One shared struct hands eBGP peers the iBGP bytes. The eBGP patched wire keeps the source ContextID because only AS-PATH content changed, so zero-copy still applies. (277) |
| Forward-pool workers are fire-and-forget for TCP write errors | forward pool worker | A TCP failure independently drives an FSM transition, so propagating a send error reports the same failure twice. (275) |
| Cache `Ack` and `Retain` are independent refcount axes | recent-update cache, `totalConsumers = pendingConsumers + retainCount` | This is why `defer Ack` firing before async workers finish is correct, rather than premature eviction. (275) |
| The route-server worker sends to its channel BLOCKING, with a stop-channel escape only | RS worker dispatch | Every cached UPDATE must be forwarded or released under the cache-consumer protocol, so a non-blocking drop leaks cache entries. (289) |
| Barrier sentinel items must skip the overflow-pool token acquire | overflow dispatch, sentinel is a nil peer | Barriers carry no route data, so taking tokens spends the congestion budget exactly when it matters. (392) |
| BGP Identifier and peer-address tiebreaks compare as 32-bit unsigned, never as strings | best-path comparison in the RIB plugin | "9.0.0.1" sorts above "10.0.0.1" lexicographically while 9 is less than 10 numerically. (374) |
| Deriving NLRI wire hex from a parsed prefix works only for simple families | `prefixToWireHex` in adj-rib-in | For VPN and EVPN it emits bare IPv4 bytes and drops RD and labels. Complex families must use the raw NLRI blob. (296) |
| The adj-rib-in pipeline source buffers per peer rather than yielding one at a time | inbound source stage | The per-peer RIB iterator holds a read lock for the whole callback, so a lazy pull would hold that lock for the life of the query. (384) |
| An NLRI's display form and its map-key form differ and must stay different | `nlri/inet.go` `String()` vs `Key()` | `String()` emits `prefix <cidr>` for the text protocol and drops path-id; `Key()` returns bare CIDR. Unifying them changes every RIB map key. (302) |
| Source IDs are never reused; a deactivated source keeps its slot | `internal/core/source/registry.go` | Recycling makes historical records resolve to the wrong peer. The banded self-describing layout is why bands cannot be reallocated for density. (080) |
| AS_CONFED_SEQUENCE is 3 and AS_CONFED_SET is 4 (RFC 5065) | `attribute/aspath.go` | Ze shipped them swapped and the wire tests encoded the swapped values, so the tests agreed with the bug. Fixing the constant required fixing the fixtures. (027) |
| The two text scanners are deliberately NOT unified | `textparse/scanner.go` `TextScanner` (raw, no quotes) vs the quoted-input splitter | Quoting semantics are incompatible, so sharing goes through `textparse/keywords.go`. Merging the tokenizers is the tempting wrong move. (306) |
| RFC 4271 4.1's 4096-byte maximum message length is deliberately NOT enforced | message header validation | RFC 8654 extended messages allow 65535. An RFC pass reading only RFC 4271 scores this as a missing check and "fixes" it, breaking extended-message peers. (005) |
| CommitManager is per-peer, not global | CommitManager | Routing decisions are per-peer, and a global manager merges batches that must stay separate. Active commits roll back when the peer disconnects. (013) |
| Sending EOR after an API commit is a deliberate extension beyond RFC 4724 | commit `eor` path | `end` flushes routes; `eor` flushes then sends EOR per affected family. RFC 4724 defines EOR for GR initial sync only. This was reviewed and accepted as a batch-completion signal, so an RFC pass must not "correct" it. (013) |
| AS_PATH lives in `route.asPath`, never in `route.attributes` | route grouping | AS_PATH is rewritten per hop while other attributes pass through. The first implementation searched `group.Attributes`, never found it, and gave every route in the group one empty AS_PATH. (014) |
| RFC 7606 validation cannot be decided from UPDATE bytes alone | `ValidateUpdateRFC7606` | It needs `isIBGP` (LOCAL_PREF, ORIGINATOR_ID and CLUSTER_LIST are attribute-discard on EBGP and treat-as-withdraw on IBGP) and `asn4` (AGGREGATOR is 6 bytes with 2-byte ASNs, 8 with 4-byte). Both were added after the fact. (016) |
| Two UPDATE split paths exist and must never serve each other's traffic | `sendUpdateWithSplit` (build, API routes) vs `SplitWireUpdate` (forward, received) | Build owns peer-specific encoding, forward owns zero-copy bytes. Splitting lives in the reactor because only it holds ADD-PATH, ASN4 and max-size context. (082) |
| BGP-LS is the only family a split cannot rescue | MP-NLRI split | Its 2-byte NLRI length lets one NLRI exceed 4096, so split returns an error rather than truncating. Every other family's per-NLRI size derives from its own length byte. (093) |
| `SplitUpdateWithAddPath` takes one `addPath bool` for the whole UPDATE | wire split | When MP_REACH and MP_UNREACH have different ADD-PATH settings, the CALLER must split by family first, or path-ids mis-encode silently. (093) |
| The encoding-context hash includes direction | context registry hashing | Without it a peer's recv and send contexts collapse to one ID, and the two directions become indistinguishable for zero-copy. (112) |
| `PeerIdentity`, `EncodingCaps` and `SessionCaps` are immutable after session creation | capability sub-components | `Negotiated` and both contexts share them by pointer, so mutating one rewrites both directions. (112) |
| `WireWriter` lives in the `context` package, not in `wire` | package placement | `wire -> context -> nlri -> wire` is an import cycle. The surprising location is the fix. (113, 116) |
| Buffers handed back to callers are exempt from buffer-first | UPDATE builder output fields | They must outlive `resetScratch()`. "Finishing the migration" by pooling the outputs produces use-after-free while every test still passes. (220) |
| A multi-route builder reading shared attributes from `routes[0]` requires the caller's grouping key to cover every shared attribute | `mvpnRouteGroupKey` | MVPN grouped on next-hop alone, so two customers' routes with the same next-hop but different Route Targets shared one UPDATE and the second RT was overwritten. That is a VPN isolation failure. (053) |
| `AttributesWire` does NOT own its `packed []byte` | `attribute/wire.go` | Returning the message buffer to the pool while an `AttributesWire` still references it is undefined behavior. Accepted for zero-copy and enforced by convention only. (057) |
| ADD-PATH state is per-AFI/SAFI and asymmetric, never a global bool | `SplitWireUpdate(..., srcCtx)`, `EncodingContext.AddPathFor(family)` | Three sites regressed on this: the splitter's `addPath bool`, a proposed per-message field, and a hardcoded `addPath=false` in the RIB plugin that corrupted stored routes. (078, 100, 340) |
| When source and target contexts disagree on ADD-PATH, NLRI bytes are rewritten, never passed through | `WireNLRI.Pack` | Target has ADD-PATH and source lacks a path id: prepend NOPATH, 4 zero bytes. The reverse case strips 4 bytes. (034, 087) |
| FlowSpec and BGP-LS NLRI never carry a path id, even when ADD-PATH is negotiated | `supportsAddPath(n)` guard in `WriteNLRI` | The original prepended a path id for every type when addPath was true, producing unparseable FlowSpec and BGP-LS. (114) |
| An NLRI exists in two shapes and `WriteTo` must serve both | NLRI `WriteTo` | Constructed-from-components writes fields; parsed-from-wire has empty components and populated cached bytes. Writing only the component path emits nothing for a parsed NLRI. (075) |
| `RDNLRIBase.buildData()` is called from `Bytes()` only, never from `WriteTo()` | `nlri/base.go` | It allocates via `append()`, so routing `WriteTo` through it breaks the zero-alloc contract. The lazy `Bytes()` cache was also a data race until `sync.Once`. (086) |
| Attribute lookup is a linear scan over `attrIndex`, not a map, deliberately | `attribute/wire.go` | At around 15 attributes an O(n) scan matches a map, and the slice preserves wire order, which a map loses. Converting to a map looks like an optimisation and destroys ordering. (111) |
| The RFC 7606 Partial flag (0x20) is NOT preserved for pooled optional-transitive attributes; flags are reconstructed as 0xC0 | per-attribute pools, `RouteEntry.ToWireBytes()` | A deliberate lossy choice, invisible until someone reasons about partial-flag propagation. (176) |
| On implicit withdraw, save pool slot values BEFORE `Release()` | `RouteEntry.Release` call sites | Release invalidates the handle, so reading after it is use-after-free. (176) |
| `returnReadBuffer` selects the pool by `cap(buf)`, never `len(buf)` | session read-buffer return | cap is fixed at allocation while len varies per read, so keying on len returns 64K buffers to the 4K pool. (343) |
| Read-buffer ownership is exclusive: the session or the recent-message cache, never both | session `process()` `kept`, cache `Take()` | The receive callback fires BEFORE `cache.Add()`, so a full cache cannot reject an already-released buffer. `Take()` removes the entry, so two callers cannot both claim one. (343) |
| The UPDATE splitter needs a progress guard | `SplitWireUpdate` `madeProgress` | When the base attributes alone exceed the limit, the slow path loops forever. The guard has no visible purpose until the pathological input arrives, so it reads as removable. (078) |
| FlowSpec NLRI length switches to its 2-byte form at 240, not at 256 | FlowSpec encode and decode | The 2-byte form is `0xF0 \| high_bits` then the low bits, not a big-endian uint16. Easy to misread straight from the RFC. (091, 078) |
| FlowSpec `String()` is parser-compatible only if numeric components omit the `&` AND-prefix and protocol omits any operator prefix | FlowSpec `String()` | The parser infers AND from position, so emitting `&<=65535` makes it drop the value silently. (122) |
| FlowSpec JSON must preserve nested arrays (OR-of-AND) and must not key components by type in a map | FlowSpec JSON encode | Flattening changes the rule's meaning, and map assignment drops all but the last same-type component. Input must be minified, with space-delimited protocol. (192) |
| NLRI `String()` is display output, not a round-trip format | all NLRI `String()` | Reconstruction needs the full command wrapper. Optional fields are emitted only when non-zero. (119) |
| Route Distinguisher string output carries its type prefix | `RouteDistinguisher.String()` | `0:65000:100`, `1:192.0.2.1:100`, `2:65000:1`. Type 0 and Type 2 are otherwise indistinguishable in text. (118) |
| Family-specific attribute keywords must be handled BEFORE the shared common-attribute parser | route and attribute text parsing | The common parser consumes greedily and reports consumed>0, so a family case placed after it is unreachable. MUP's `extended-community` handler was dead for this reason. (105) |
| MP_REACH_NLRI (14) and MP_UNREACH_NLRI (15) cannot be filter members and are rejected in config | attribute filter validation | They carry the NLRI, so filtering them removes routes rather than the attribute. (056) |
| `ContextID` 0 is not a registered context | `APIContextID` | Any operation with no peer session must pre-register a named context, defaulting to ASN4=true. `NewWireUpdate(body, 0)` fails, and API wire bytes are never re-encoded between contexts. (087, 241) |
| A `nil` entry in the `[256]` attribute-parser dispatch table is intentional | `knownAttrParsers` | PMSI, TunnelEncap, AIGP, BGPLS and PrefixSID are known codes with no parser, and they fall through to `OpaqueAttribute`. Reading nil as a bug leads to "fixing" it. (352) |
| The update-text parser's `snapshot()` must deep copy every slice | attribute accumulator | A shallow copy lets a later section mutate an earlier snapshot. (081) |
| BGP-LS TLV 1251 (SRv6 BGP Peer Node SID) is 12 bytes | `attr_srv6.go` | The SID comes from NLRI descriptor TLV 518, not from the attribute. An initial 16-byte SID made it 28. RFC 9514 5.1 and GoBGP both say 12. (400) |
| BGP-LS node-descriptor TLV 256 container unwrapping must be iterative, never recursive | node descriptor parsing | The recursive form let around 16K nested containers overflow the goroutine stack, on attacker-supplied wire data. (400) |
| BGP-LS TLV 518 has two roles and both paths must handle it | NLRI parser and `NodeDescriptor` | It is an SRv6 SID NLRI descriptor (type 6) and a node-descriptor sub-TLV inside TLV 256/257. Handling only the NLRI path loses the SID silently. (400) |
| The RFC 7606 error action must be decided BEFORE callback dispatch, and `fsm.EventUpdateMsg` must fire even for treat-as-withdraw | `reactor/session_read.go` | RFC 4271 8.2.2 requires the FSM event for every UPDATE, and deciding after dispatch means a malformed UPDATE already reached the plugins. (254) |
| The UPDATE grouping key is attribute-hash plus family, and deliberately EXCLUDES PATH-ID | `attribute.Attributes.Hash` | Routes with different path-ids but identical attributes may legally share one UPDATE. Different families may never: IPv4 rides the body, IPv6 rides MP_REACH. (061) |
| Next-hop "self" resolves at the peer, never at the parser, and a session has exactly ONE local address | `reactor/peer.go` `resolveNextHop` | Only the peer knows the negotiated capabilities. An explicitly configured next-hop bypasses validation deliberately, as an operator override. (083) |
| Message IDs are allocated globally across ALL message types, but only UPDATEs enter the recent cache | `reactor/recent_cache.go` | Cache ids are therefore sparse, so a cumulative-ack loop probing consecutive integers wastes work. This is why `seqmap.Since()` is a range query. (304) |
| A cache entry must be inserted BEFORE the event is dispatched | `recent_cache.go` `Add` | Any window produces "plugin holds an id the cache has no entry for", which was the original route-loss bug. (317) |
| The cache tolerates apparent disorder | `recent_cache.go` | Consumer counts may go NEGATIVE between dispatch and `Activate(id,N)`, and out-of-order acks are silent no-ops, because a fast plugin can Decrement before Activate. Clamping at zero looks like hardening and breaks real workloads. (317) |
| `RawMessage.IsAsyncSafe()` is false for RECEIVED UPDATEs | `bgp/types/rawmessage.go` | Their bytes point into a reusable TCP read buffer, so copy before any async hand-off. (321) |
| `sendUpdateWithSplit` takes the caller's computed `addPath` and must not recompute it | `reactor/peer_send.go` | Recomputing reads sendCtx a second time, which is a TOCTOU against the build step. (382) |
| Anything derived from a Go map must be sorted before use | `EncodingContext.Hash()` over the AddPath and ExtendedNextHop maps; `Families()` via `FamilyLess()` | Map order is unspecified, so an unsorted hash makes registry dedup non-deterministic, and unsorted families make EOR order irreproducible. (039, 063) |
| A route parser that falls through on an unrecognised keyword, SAFI, or AFI/prefix mismatch emits malformed wire with no error | `parseSAFI` and the per-family keyword sets | It produced IPv6 bytes under AFI=1 and MUP routes encoded as unicast, with nothing logged. Reject by whitelist, never skip. (017, 046, 047) |
| Zero-copy forwarding requires the original wire bytes to survive from receive time | attribute storage on the receive path | If the RIB ever normalises attributes on ingress, zero-copy breaks even when the two ContextIDs match, and the ContextID comparison still says it is safe. (038) |
| Deferring the RIB insert until after the forward is safe only because `PeerDown()` drains the worker queue before `ClearPeer` | bgp-rr server | Reordering that drain reintroduces withdrawals for routes not yet in the RIB. (284) |

---

## BGP engine: session, FSM, TCP

### Current shape

FSM is pure state transitions + timers. `session.go` orchestrates I/O
and delivers UPDATEs to the reactor's per-peer delivery goroutine.
Read loop uses `bufio.Reader` with 64K buffer and close-on-cancel
(cancel goroutine closes `net.Conn` to unblock `ReadFull`). Writes
are protected by a `writeMu` and batched via a 16K `bufio.Writer`
drained by the forward pool. Listener per local-address; peer lookup
by remote IP. RFC 9234 Role and collision detection in engine;
strict enforcement in the `bgp-role` plugin.

### Evolution

- **015** FSM/Reactor split intentional: FSM
  = pure transitions + timers; Reactor = orchestration + I/O. Follows
  ExaBGP pattern. Reactor bloat was in `peer.go` encoding logic, not
  FSM design — addressed separately.
- **023** RFC 4271 §6.8 collision
  detection: detection at peer/reactor level; two-phase (reject if
  Established, else wait for remote BGP ID in OPEN). OpenSent collision
  NOT handled — only OpenConfirm (the MUST case).
- **049** Per-peer listeners:
  listeners keyed by `netip.Addr`; `LocalAddress` mandatory.
  Self-referential peers and link-local IPv6 rejected.
- **233** `local-address` mandatory,
  validated in Go (`reactor/config.go`), because YANG `mandatory true`
  only checks presence, not format.
- **067** `PeerLifecycleObserver`
  interface; observers registered via `AddPeerObserver`. `OnPeerEstablished`
  fires BEFORE `sendInitialRoutes()` so plugins see Established before
  routes arrive.
- **142** `plugin session ready` mandatory: Ze waits
  for all processes to signal ready before starting peer connections
  (5s timeout, then proceeds with warning). Same mechanism per-session
  on reconnect.
- **244,
  247,
  248,
  250,
  251,
  252,
  351** ReactorInterface split.
  68-method monolith → `ReactorLifecycle` + `BGPReactor` → 5 focused
  sub-interfaces (ReactorIntrospector, ReactorPeerController,
  ReactorConfigurator, ReactorStartupCoordinator, ReactorCacheCoordinator).
  Adapter pattern: `ReactorLifecycle` is implemented by unexported
  `reactorAPIAdapter`, not `*Reactor` directly.
- **272,
  279,
  290,
  316,
  376,
  382** Session I/O pipeline. Close-on-cancel for
  read interruption. `writeMu` added after missing synchronization was
  blamed on "externally synchronized" comments that no caller honored.
  `bufio.Reader` 64K (matches Extended Message). Forward pool batch
  drain. Atomic pointer for `sendCtx` (concurrent readers from plugin
  dispatch without RLock because `peer_initial_sync.go` already holds
  `p.mu.Lock`).
- **288,
  264,
  265** Clock/Dialer/ListenerFactory
  injection. `sim.Clock`, `sim.Dialer`, `sim.ListenerFactory` interfaces.
  VirtualClock for in-process chaos. ChaosClock for `--chaos-seed`
  self-test mode. Grep audit test forbids direct `time.*` and `net.*`
  in reactor/FSM.
- **365** Per-peer panic recovery. `safeRunOnce`
  wraps peer lifecycle; panic becomes error, feeds backoff loop.
  Delivery goroutine recovery exits loop (session tears down anyway).
  4K stack capture via `runtime.Stack`.
- **280** Capability `require` and `refuse` are enforced AFTER negotiation,
  not during the OPEN exchange. Both OPENs are exchanged and negotiated,
  then the requirements are checked and an Unsupported Capability
  NOTIFICATION is sent. The check must be called from BOTH `processOpen()`
  and `handleOpen()`.

### Abandoned approaches

- **`passive bool`** (271) — replaced with independent `connect` and
  `accept` booleans. Defense-in-depth: both checked at reactor.
- **`session reset` command** (170) — removed with API restructure.
- **`CBOR` plugin encoding** (170) — incompatible with line-delimited
  protocol.
- **Field names `Encoder` and `Serial`** on `CommandContext` (229) —
  dead fields, never read; removed.
- **Changing the `asn4` YANG leaf from boolean to string** (280):
  proposed so the leaf could carry `require` and `refuse`. Reverted: it
  broke serializer round-trip and two tests. The `TypeBool` validator was
  extended instead.

### Load-bearing invariants

| Invariant | Site | Why (first occurrence) |
|-----------|------|------------------------|
| `local-address` is mandatory for every peer | `reactor/config.go` | Explicit choice per peer; `auto` is allowed for OS-selected binding but must be explicit. (049, 233) |
| Peer lookup on incoming connection is by remote IP, not listener address | `reactor/listener.go` | Listener address is used only for RFC-compliance validation. (049) |
| Keepalive timer fires via `time.AfterFunc` in independent goroutine | `session.go` | Writes through `sendKeepalive` → `writeMessage`; this is the least obvious concurrent caller. `writeMu` required. (279) |
| `connectionEstablished()` sends an OPEN | session setup | Tests that set up TCP sessions directly must use raw field assignment. (415) |
| Reactor code must use `sim.Clock` interface, not `time.*` | `reactor/*.go` | Chaos and virtual-time tests bypass `time.Now()` silently — audit test forbids direct calls. (288) |
| Collision detection happens at OpenConfirm, not OpenSent | `peer.go collision check` | OpenSent collision is a MAY, never implemented. OpenConfirm is the MUST case. (023) |
| UPDATE delivery uses a bounded channel plus a goroutine PER PEER, never one reactor-wide | per-peer delivery channel on `Peer` | A shared channel reintroduces a cross-peer deadlock: A reads an UPDATE and delivers it to the route server, which forwards to B, whose TCP write blocks because B's receive buffer is full, because B's read goroutine is blocked on the shared channel forwarding back to A. (272) |
| Read interruption uses close-on-cancel, not read deadlines | session and listener read loops | `SetReadDeadline` is a NO-OP on `net.Pipe` and on mock connections, so a deadline-based cancel silently never fires in exactly the tests meant to prove it. (272) |
| The peer-observer slice is copied under a read lock, then iterated with NO lock held | reactor peer-observer notification | An observer commonly calls back into the reactor, so iterating under the lock deadlocks. This is safe only because the slice is read-only after registration. (067) |
| `time.Since(t)` calls `time.Now()` internally, so it defeats clock injection while looking clock-free | reactor elapsed-time code; use `clock.Now().Sub(t)` | It silently broke the chaos virtual clock, and an audit that greps for `time.Now()` does not catch `time.Since`. (341) |
| A config flag that enables a capability must also CONSTRUCT the capability object | config loader capability assembly | route-refresh was configured and both sides reported it supported, but the marker capabilities never reached the OPEN, so negotiation "succeeded" and BoRR/EoRR were never sent. Only a functional test exposed it. (107) |
| A `refuse`d capability must be checked against the peer's RAW advertised OPEN codes, never against the negotiated set | `Negotiated.peerCodes`, `CheckRefusedCodes` | Negotiation is an intersection, so a refused capability never appears in the result and the check silently never fires. (280) |
| `require` overrides `ignore-mismatch`; the stricter of the two wins | negotiation-time `CheckRequired` vs UPDATE-time ignore-mismatch (`internal/component/bgp/reactor/config.go`, session validation) | They are two independent mechanisms acting at different points in the session lifecycle, so wiring ignore-mismatch into the negotiation path would silently defeat `require`. (007) |
| Pause and resume thresholds must be a WIDE hysteresis band | RS `worker.go` high/low-water | 75%/25% oscillated. The shipped read-pause uses 100%/10%, and the wide band is the design. (278, 376) |
| While reads are paused the hold timer is the safety valve, KEEPALIVE continues, and the cancel goroutine must call `Resume()` before closing | `session_flow.go` `waitForResume` | RFC 4271 6.5 bounds a pause. Without Resume-on-cancel the gate blocks shutdown, and `waitForResume` must re-check `closeReason` because a closed `resumeCh` means resume OR shutdown. (376) |
| Session lock order is `s.mu` then `s.writeMu`, and `closeConn` must take `writeMu` for its final Flush inside the `s.mu` section | `reactor/session.go` | Flushing under `s.mu` alone races every concurrent `Send*`. (316) |

---

## Configuration: YANG, parser, editor, reload

### Current shape

YANG is the single source of schema truth. The parser tokenizes a
set+meta line-oriented format (write-through per session), validates
against YANG, and produces a `Tree` + `MetaTree` in memory. Config
travels through the pipeline as `File → Tree → ResolveBGPTree →
map[string]any → reactor.PeersFromTree`. The editor holds `Tree` as
canonical (not raw text). SIGHUP triggers a two-phase verify+apply
coordinator. Plugins declare `wants config <root>` and receive the
subtree at Stage 2; they parse the JSON themselves.

### Evolution

- **008,
  009,
  041** Config migration. Three-
  version chain (v1 ExaBGP main → v2 Ze intermediate → v3 `peer` +
  `template.match`). Heuristic detection (no version field). Named
  semantic transformations over numbered versions.
- **065** Version numbers
  removed from code (API versions, config fields, migration comments).
  Design for machine-transformable migration.
- **050,
  476,
  506,
  628** `environment { }` block. Priority:
  OS env > config block > defaults. Strict validation (unknown keys
  fail at startup). Env vars registered via `env.MustRegister` silently
  overwrite duplicates; known two-site drift (`environment.go` +
  consumer) tracked in memory.
- **166,
  167,
  180,
  181,
  281** YANG as sole schema. `ze:syntax`
  extensions removed — standard YANG `leaf-list` / presence container
  / list handle what `ze:syntax` used to annotate. Freeform parsing
  eliminated. `LegacyBGPSchema()` retained only for ExaBGP migration.
- **151,
  334,
  488,
  556,
  577** YANG reorganisation. Each module lives
  with the package that owns it; `init()` registers via
  `yang.RegisterModule`; `yang_schema.go` hard-codes the module list
  (TWO registrations required — known duplication).
- **293,
  356,
  410,
  551** YANG `ze:validate` extension.
  Registry in `internal/component/config/yang/`; validators in other packages register
  via explicit `RegisterValidators()`. `CompleteFn` provides autocompletion
  in the editor. Runtime-determined sets (plugin families) use
  `ze:validate`; compile-time sets use YANG native constraints.
- **212,
  213,
  214,
  345,
  346** Config reader. Standalone
  binary → inline library. Tokenizer produces `map[string]any` directly
  (JSON roundtrip removed). `ValidateContainer` (flat, per-block) and
  `ValidateTree` (recursive, at load time).
- **175,
  232,
  349,
  369,
  370,
  391,
  427,
  428** Editor. Text surgery (findFullContextPath,
  setValueInConfig) deleted. `Tree` is canonical; `WorkingContent()` =
  `Serialize(tree)`. Per-user change files (`config.conf.change.<user>`)
  replace shared draft. Session identity is `user@origin:unix-ts`.
  Live conflict (same path, two sessions) and stale conflict (committed
  value changed since last set) detected explicitly.
- **222 through
  234,
  342** Config reload. Two-phase
  verify+apply coordinator. Plugins register `WantsConfigRoots`; engine
  sends `config-verify` → all plugins OK → `config-apply`. Any verify
  failure aborts. SIGHUP wired through coordinator; editor triggers
  reload via RPC (not signal).
- **380** Config archive. VyOS-inspired fan-out:
  all locations attempted, errors collected per-location, non-fatal to
  commit. `file://` and `http(s)://` protocols.
- **426,
  456,
  463,
  477** Zefs blob store. Two namespaces:
  `meta/` (instance metadata), `file/active/` (current config).
  `file/draft/` and `file/<date>/` qualifiers planned. No flock (all
  sessions in-process); `Storage.AcquireLock` returns a `WriteGuard`.
- **537,
  538** Transaction protocol. Orchestrator
  migrated from the bus to the stream system during the larger
  bus-removal work. Participating plugins declare via `WantsConfig`;
  orchestrator sends verify → all ok → apply → all ok → commit.

### Abandoned approaches

- **`ConfigVersion` type and numbered migrations** (041) — replaced
  with named `Transformation` registry.
- **`ze:syntax` YANG extensions** (281) — ALL of them were display
  artifacts over standard YANG types. Removed.
- **Flat shared draft** (427) — replaced with per-user change files.
- **Socket locking with flock** (463) — went through three design
  iterations (new RPC protocol → flock on socket → "SSH already exists").
- **`UpdateSections.parsed bool`** (204) — replaced with
  `sections.Valid()`.
- **`freeform` parser mode** (281) — stored word sequences as opaque
  map keys, prevented schema-level validation.
- **Config push from hub to plugins** (160) — pull model: hub notifies,
  plugins query `query config live|edit path`.
- **`SetParser.ValidateValue`** (214) — YANG is the sole validator.
- **A version field in the config file** (008): rejected for migration,
  because the field itself would then need migrating. Version detection is
  structure-based instead. Rejecting the field is what made "no version
  numbers in config" survivable as a project-wide rule.
- **ExaBGP's separate `env` INI file** (050): rejected for the
  `environment { }` block, so one file holds all config. This is why env
  vars are a config surface rather than a deployment surface.
- **Generating a Go schema from YANG at build time** (166): rejected for
  YANG extensions read at load time. Extensions live in the model file and
  need no build step. This is why `ze:` extensions exist instead of codegen.
- **Queuing concurrent reloads** (223): rejected for `TryLock` rejection.
  A queue turns rapid config changes into a reload storm.
- **Junos-style ordered failover for the config archive** (380): rejected
  for VyOS-style fan-out: every location is attempted, errors are collected
  per location, and none fails the commit.
- **`/command` and `/edit` slash-prefixed mode switches** (356, 385):
  chosen on IRC and Slack convention, then removed for bare `run` and
  `edit`, with cross-mode completions.
- **`ze:command` as a boolean marker** (395): it became an extension
  carrying the WireMethod as its argument, because a marker forces a naming
  convention between the tree node and the handler. The command tree was
  also planned as three monolithic YANG files before being split per plugin.
- **libyang** (151): goyang was chosen instead: pure Go, no cgo, and it
  covers the type and range validation actually needed. The schema was
  deliberately kept pragmatic rather than OpenConfig-compatible.

### Load-bearing invariants

| Invariant | Site | Why (first occurrence) |
|-----------|------|------------------------|
| YANG is the single source of schema truth | `internal/component/config/yang/` | `BGPSchema()` removed; `LegacyBGPSchema()` only for ExaBGP migration. (166) |
| Every YANG `environment/<name>` leaf needs `ze.<name>.<leaf>` registered via `env.MustRegister` | plugin `init()` | Env vars are part of the config interface, not follow-up work. (050, CLAUDE.md rule) |
| Every new top-level config block requires TWO registrations | `init()` + `yang_schema.go:YANGSchemaWithPlugins()` | `yang.RegisterModule()` makes module available to loader; explicit call in `yang_schema.go` builds the schema. Parser does not discover. (488, 556, 577) |
| `ApplyConfigDiff` in production re-reads disk via `reloadFn` | `reactor/config.go` | Verify and apply must see the same on-disk state at their respective times. (466, 535) |
| `GetConfigTree` returns live map reference, not a copy | `reactor/config.go` | Mutating outside locks races. (466) |
| `ze config validate` does NOT invoke plugin `OnConfigVerify` | `internal/component/config/cli/cmd_validate.go` | Parser + YANG type check only; plugin-side validation runs only in running daemon. (413, 557, 621, 627) |
| Plugin config tree delivered wrapped as `{"bgp":{...}}` | `server/reload.go` | Plugins must unwrap before accessing their subtree. (185, 538) |
| YANG list = JSON map keyed by list key | type conversion | `list server { key "name"; }` → `"server": {"default": {...}}`, not `[{"name":"default",...}]`. (574) |
| Config values survive JSON roundtrip as strings | type conversion | `"enabled": "true"` not `"enabled": true`; `strconv.ParseUint` needed for numeric. (213, 556, 574) |
| `ze:validate` on a YANG typedef does NOT reach the leaves that reference the typedef; the extension must be repeated on every leaf | leaves using `zt:address-family` | goyang resolves the typedef's type and pattern, but not its custom extensions. 293 asserted the opposite, citing RFC 7950 7.3.4, and 356's empirical finding governs. A leaf that looks validated by its typedef is unvalidated. (356 over 293) |
| Template `match` blocks apply in CONFIG-FILE ORDER, not by specificity | template match application, and the v2-to-v3 migration | An explicit choice to give operators order control, which is why migration must preserve insertion order. Precedence is template.match, then template.group, then the peer block. `inherit` is rejected inside `template`, and `match` is valid only inside `template`. (008, 009) |
| GR `restart-time` has an upper bound of 4095, not 65535 | GR YANG typedef, RFC 4724 capability encoding | The field is 12 bits inside the two-byte capability value, so a YANG `uint16` range accepts unencodable values. (163) |
| Read YANG through the resolved entry tree, never the raw module | `yang.Loader.GetEntry()` after `Resolve()` | A raw `yang.Container` or `yang.Module` has `Mandatory` and other resolved fields unset, so validation built on it silently passes everything. (166, 167) |
| The parser keeps Go schema node types even though YANG owns validation | config parser node kinds | YANG describes WHAT is valid, not HOW syntax is parsed, and syntax-handling modes have no YANG equivalent. This is why full parser replacement stayed blocked. (167) |
| The Bubble Tea model is a value type, so a closure capturing it loses every mutation | editor model | Two structural answers: commands return a result that the `Update()` handler applies, and long-lived state lives behind a shared `*Editor` pointer, with the dirty flag as an `atomic.Bool`. Mutating the model inside a closure looks correct and does nothing. (175, 366) |
| `_default` is the sentinel key for a singleton (unnamed) block | config reader, tokens to map | Code walking the tree without knowing the sentinel treats singletons as a list keyed "_default". (212) |
| `json.Unmarshal` produces `float64` for every number, regardless of the YANG integer type | YANG validator numeric path | A validator switching on `int64` or `uint32` without a `float64` branch rejects every numeric config value. (213) |
| Validation of a dynamically-registered set checks FORMAT only, never membership | address-family validation | Families are registered by plugins at runtime, so a static enumeration is wrong by construction. (215) |
| Removing a config root must send an explicit empty object `{}`, never skip the send | reload coordinator delivery | A plugin that receives nothing cannot distinguish "no change" from "deleted". The first implementation skipped the send, and plugins silently kept stale config. (223) |
| `commit-confirm` and `abort` change committed config and must trigger a reload, exactly like `commit` | editor commit path | Wiring only `commit` leaves the editor and the daemon out of sync. Found in review, not in the spec. (224) |
| A registered plugin whose connection is nil at verify time is an error, not a skip, and liveness is re-checked before apply | reload verify and apply | Skipping a dead plugin produced a green verify over a plugin that never saw the config. (227) |
| Apply errors are aggregated and returned, and the config tree is still updated | reload apply | The reactor has already applied, so the stored tree must reflect the new state even when a plugin failed. (227) |
| A dedicated `HasConfigLoader()` predicate gates the SIGHUP coordinator path | reload entry point | The interface field it would otherwise test is always non-nil in production, so testing the interface alone made every production SIGHUP take the wrong path. (230) |
| Reload re-parses peers through the FULL config pipeline, never the partial tree parser | peer reload | The partial parser populates 6 of 16 peer fields, so comparison reported every peer as changed on every reload. The partial parser is test-only. (230) |
| Startup fails if any `ze:validate` reference has no registered validator | validator registry integrity check | Otherwise that leaf is simply unvalidated, which is the silent-pass failure the extension exists to prevent. (293) |
| The editor validator suppresses "mandatory field missing" while editing, except for `peer-as` | editor validation | A config under edit is always incomplete, so unfiltered checks make the editor permanently red. (293) |
| Config watch channels are capacity 1, drop-oldest and send-newest, collected under the lock and sent outside it | config manager watch | A blocking send stalls the publisher, and sending under the lock deadlocks when a consumer calls back. Only the latest config matters. (326) |
| Use YANG `augment` to extend a container, never a second definition | environment container split across modules | A redefinition REPLACES the container while augment extends it. The two look interchangeable and are not. (334) |
| goyang `Parse()` is order-independent; only `Resolve()` needs every module | YANG loader | This is what makes `init()`-time registration work. Two corollaries: a module registered by `init()` must never also be loaded manually, which is a duplicate-module error, and an augment target must be in the embedded bootstrap set. (334) |
| A list KEY leaf is a schema child of the list, and must never be offered as a completion or accepted as settable | editor completer and set-path validation | The key is already consumed by the path. Schema navigation skips keys silently, so token-level validation must enforce them. (356, 367) |
| A missing list key is detected by testing whether the next token is itself a schema child of the list entry | set-path token validation | If it is, the user omitted the key. Schema-only navigation accepts the shorter path, which is correct for lookup and wrong for a config path. (367) |
| `config false` is inherited by descendants (RFC 7950 7.21.1), so the check must walk every ancestor | set-path rejection | Testing only the target leaf lets a write into a whole read-only subtree. (367) |
| Deleting a nonexistent leaf or list entry is a silent no-op, and list deletion still marks the editor dirty | tree delete | Neither path checks existence, so a no-op delete leaves a dirty editor with nothing to commit. (369) |
| A `file://` archive location must be an absolute URL | archive location parsing | `url.Parse("file://./path")` puts "." in Host and "/path" in Path, so a relative form parses into a target nobody intended. (380) |
| Archive locations are read once from the editor's ORIGINAL content at startup, not from the working draft | editor archive wiring | Adding an `archive { }` block mid-session has no effect until restart. (380) |
| `edit` is disambiguated by arity, not by prefix | editor enter handling | Bare `edit` is a mode switch and `edit <path>` is a config command. A command tree containing `edit` routes to YANG completions unexpectedly. (385) |
| In the set+meta disk format the `^` prefix carries the PREVIOUS value, which is what enables stale-conflict detection | config line metadata | A line without metadata parses identically, so hand-written configs keep working, and dropping prefixes on rewrite silently disables stale detection. (391) |
| Format detection scans EVERY line, not the first | `DetectFormat()` | Metadata prefixes can appear after plain `set` lines, so a first-line test loses authorship and previous-value data. (391) |
| Session IDs are `user@origin:unix-ts`, so two sessions in the same second collide | edit session creation | Tests must vary origin, and adoption matching must use `UserAtOrigin()+":"` because `username+"@"` matched "thomasmore@" for user "thomas". (391) |
| The generic config package holds zero BGP imports | package boundary | The dependency runs one way, from the generic loader into `bgp/config`. Editor, yang, migration and env stay under generic config because they couple through config types, not through BGP. (361) |
| The YANG validator is deliberately permissive on unrecognised types, and leafrefs are not validated offline | YANG validator | Unknown types pass for forward compatibility, and leafref validation needs runtime state. Both are deliberate fail-open holes, not bugs to close blindly. (346) |
| A `bufio.Scanner` buffer must be MaxMessageSize+1, not MaxMessageSize | IPC framing scanner | The bound is exclusive, so a message of exactly the maximum length is truncated silently. (208) |
| Next-hop sits OUTSIDE the nlri section on input and INSIDE it on output | `update` grammar | A deliberate asymmetry: input optimises for typing, where one `nhop` accumulates until changed, and output optimises for determinism, where it is explicit per family. Making them symmetric breaks one of the two properties. (089) |
| Cross-field peer validation runs AFTER template, match and inherit merging, never against the raw parse tree | `validateProcessCapabilities()` | A rule such as "route-refresh requires a process binding with `send { update; }`" is only decidable on the fully resolved peer config. (108) |
| The `add`, `del` or `eor` keyword in an NLRI block is mandatory, not optional sugar | config parser NLRI extraction | It is the structural boundary between family metadata (rd, label) and payload. Making it mandatory removed the last NLRI ambiguity, at the cost of 40 or more encode fixtures. (281) |
| The reactor must not import config; the allowed direction is config into reactor | `SetReloadFunc` and `ReloadFunc` | Reload is injected as a callback purely to avoid the import cycle, and it returns a full `*PeerSettings` so the existing conversion is reused. (342) |

---

## Plugin system: architecture

### Current shape

A plugin is an `init()` function that calls `registry.Register(name,
Registration{...})`. The registration declares: capabilities the plugin
handles, address families, config roots (`WantsConfig`), event types
it emits (`EventTypes`), send types it provides (`SendTypes`),
command handlers (in a `-cmd.yang` module via `ze:command`), and
dependencies. A plugin runs in one of three modes: **Fork** (external
binary or path), **Internal** (same binary, goroutine +
`net.Pipe()` pair), **Direct** (synchronous in-process call via
DirectBridge zero-copy transport). All use the same 5-stage startup
protocol over Socket A (plugin → engine) and Socket B (engine →
plugin). There is ONE wire format, newline-framed
`#<id> <verb> [<json>]\n` (`pkg/plugin/rpc/conn.go`). There is no
first-byte format auto-detection and no second text protocol: both were
built and then deleted (see Abandoned approaches).

### Evolution

- **001** Original plugin protocol:
  JSON events down, text commands up, stdin/stdout.
- **069,
  168** Serial correlation. `#N` numeric
  prefix (plugin → engine), `#abc` alpha prefix (engine → plugin),
  `@serial` response echo. Empty serial (`""`) for unsolicited events.
- **142,
  152,
  172,
  305** 5-stage startup protocol.
  Stage 1 declarations (capabilities, families, schema, commands),
  Stage 2 config delivery, Stage 3 capabilities, Stage 4 registry,
  Stage 5 ready. Tier-ordered per Kahn topological sort; dependencies
  resolved pre-startup.
- **184,
  198,
  210,
  264,
  294,
  459** Plugin invocation modes.
  `ze.X` prefix = internal (goroutine + `io.Pipe` or `net.Pipe`),
  path/cmd = fork. Same protocol both. DirectBridge skips JSON +
  socket I/O for internal plugins (415× faster UPDATE delivery).
  TLS plugin hub server (fleet-config use case).
- **209,
  215,
  395** YANG-driven dispatch. Text
  `RegisterBuiltin()` replaced by YANG RPC metadata extraction.
  `ze:command "wire-method"` YANG extension binds tree nodes to
  handlers. `"bgp "` prefix removed from all commands.
- **218,
  247,
  248,
  253,
  282,
  283,
  329,
  375** Plugin extraction. Watchdog, Hostname,
  FlowSpec, EVPN, VPN, BGP-LS, RouteRefresh, SoftwareVersion, GR, role
  all moved out of engine. `internal/component/bgp/plugins/bgp-*`
  convention. NLRI plugins use `bgp-nlri-*` prefix (9 plugins, 4 tiers).
- **291,
  292,
  298,
  299,
  321,
  322,
  422,
  606** Event delivery performance.
  JSON-RPC `deliver-batch` replaces per-event writes. Persistent reader
  goroutine replaces 5 per-RPC goroutines. Text format opt-in per
  plugin. DirectBridge delivers `*RawMessage` directly; plugin reads
  wire bytes via existing `NLRIIterator`. `ze.EventBus` interface typed.
- **315, 318, 397** Unified framing. A single newline-framed format,
  `#<id> <verb> [<json>]\n`, replaced the dual protocol. The intermediate
  design, a separate text protocol with first-byte auto-detection, was
  built, shipped, and then deleted whole. It is recorded under Abandoned
  approaches. Heredoc `<<EOF` carries multiline content.
- **165** The hub/subsystem split chose process-per-subsystem over
  in-process services, for crash isolation, language freedom, per-process
  resource limits and independent debugging. The same design set all
  plugins as equal peers, with no built-in versus third-party distinction,
  and routed config subtrees by longest-prefix match on handler paths.
- **301** Plugin dependency declarations exist because of one specific
  silent-degradation bug: with bgp-adj-rib-in absent, the route server set
  a permanent `replayDisabled` flag and late-connecting peers silently
  missed routes. Fail-loud at stage 1 replaced the flag, which was removed.
- **329** A plugin that announces routes sends `update text` commands, not
  internal wire builders, which keeps plugins polyglot and keeps wire
  encoding out of them. `nhop set self` resolves the per-peer next hop, so
  no plugin needs next-hop logic.
- **170** Event routing moved from config-driven (`process { receive {
  update; } }`) to API-driven `subscribe` commands, so a plugin declares at
  runtime what it needs. The migration ran both paths in parallel with
  deduplication rather than cutting over.
- **239** OPEN validation moved from engine-side `handleOpen()` to a
  `validate-open` RPC sent synchronously on Socket B, failing fast on the
  first rejection. `Strict` was dropped from the capability declaration
  because the plugin owns enforcement.
- **172** Capability config keys are RFC-scoped or draft-scoped
  (`rfc4724:restart-time`, `draft-walton-bgp-hostname:hostname`) so two
  implementations cannot collide. Each capability self-describes through a
  `ConfigProvider`, so adding one needs no reactor change.
- **188** Plugin auto-loading resolves by FAMILY claim, never by plugin
  name, in two phases: configured plugins first, then a scan for unclaimed
  families. Explicit always wins. The engine also infers Multiprotocol
  capabilities from the declared families.
- **040** Process API config separates WHAT a process receives from HOW it
  is formatted, because conflating them was a real bug: `if pc.Encoder ==
  "text" { pc.ReceiveUpdate = true }` meant JSON-encoder processes received
  nothing.
- **323 through
  328,
  419 through
  425,
  531,
  533** Arch-0 restructuring.
  Four components (originally five): Engine, ConfigProvider,
  PluginManager, Subsystem. Interfaces in `pkg/ze/`. Subsystem ≠
  Plugin (BGP daemon owns TCP/FSM; bgp-rib/rs/gr are plugins).
  BGPHooks eliminated — replaced by typed `EventDispatcher` in
  `bgp/server/`. Bus absorbed into the stream system during config-tx
  protocol work; `ze.EventBus` is now backed by `Server.Emit` and
  `Server.Subscribe`.
- **301** Plugin dependencies
  declared in registration. Two-layer validation: Go registry for
  pre-startup auto-loading of internal plugins; protocol Stage 1 for
  runtime validation of all plugins. Fail-loud on missing dependency.
- **536** Family registry. Runtime-registered
  families (dynamic), AFI/SAFI indexed. `family.Family.String()`
  requires the family to be registered; tests call
  `family.RegisterTestFamilies()`. Single atomic state; old dual
  (mutable + snapshot cache) collapsed.
- **390,
  452,
  484,
  601,
  598,
  600** Auth. SSH server with bcrypt. RBAC
  (Nokia-inspired). AAA registry (`aaa.Default.Register`,
  `BackendRegistry`). TACACS+ plugin. Timing side-channel fixed
  (always run dummy bcrypt for unknown users).

### Abandoned approaches

- **Unified subsystem protocol with full async 5-stage for
  internal handlers** (149) — over-engineered. `init()` self-
  registration works for in-process calls.
- **Plugin YANG opt-in loading** (201) — chicken-and-egg; internal
  plugin YANG always loaded at engine startup.
- **Central `RegisterDefaultHandlers()`** (149) — replaced with
  `init()` self-registration + `LoadBuiltins(d)`.
- **Hooks (BGPHooks)** (328) — replaced by typed `EventDispatcher`.
- **SSH server using own TLS session** (484) — discarded when the
  insight "SSH already provides auth" simplified the design.
- **Standalone `ze-config-reader` binary** (212) — replaced with
  in-process library.
- **`sync.Once` in `OnceValue` callbacks that may error** (079) —
  second call returns cached `(nil, nil)`; must use explicit state.
- **Bus (standalone `internal/bus/`)** (324, 425) <!-- doc-links: ignore (the package this row records as removed) --> — absorbed into
  the stream system during config-tx protocol work. The stream
  system already provided in-process pub/sub with schema validation,
  DirectBridge zero-copy for internal plugins, and TLS delivery
  for external plugins; the bus was the weaker of the two. `ze.EventBus`
  remains as the public interface, backed by `Server.Emit` and
  `Server.Subscribe`.
- **A second, text-mode plugin protocol** (315, 318, 397): designed,
  built and SHIPPED, with a text handshake, `TextConn` and `TextMuxConn`,
  heredoc JSON config, and first-byte format auto-detection. Then deleted
  entirely: about 2500 lines across 9 files, once the single
  `#<id> <verb> [<json>]\n` framing replaced both protocols. None of those
  types exists in the tree today. Do not read auto-detection as live.
- **A negotiated command-answer shape** (spec-record-answers-2-only-encoding,
  2026-08-22): `Process.RecordAnswers()` on the plugin connection and
  `ZE_ANSWER_PROTOCOL` on the SSH exec channel each selected between two
  encodings of one answer. Deleted rather than defaulted, because Ze has never
  been released and a negotiation exists only to carry a shape somebody else
  already speaks. `ProtocolRecordAnswers`, `AnswerProtocolEnv`,
  `declaresRecordAnswers`, `RecordAnswers`, `SetRecordAnswers` and
  `FormatResult` are all absent from the tree.
- **A length-prefixed answer id, and a base-36 outer length on every counted
  field** (spec-record-answers-2-only-encoding, 2026-08-22): designed so a
  reader reaches the kind token by arithmetic, built by phase 3
  (`326ce6e96`) and extended to every field by phase 5 (`46c4d0e1e`), then
  measured and deleted by phase 6 (`9313b7d5e`, `50468ee34`). The measurement is
  the whole entry: a counted id costs 8.1 to 9.2 ns against 3.2 to 3.5 ns for a
  fused digit loop over plain `#42 `, zero allocations either way, and it is two
  bytes wider on every line of every walk. The count bought nothing because the
  reader still had to check the space that closes the field and still had to
  parse the digits, which IS the cost of the plain form. What replaced it is a
  rule rather than a spelling: **a count belongs on a field whose value CAN hold
  the delimiter, and nowhere else.** An id is a bounded digit run and a space
  closes it unambiguously, because a digit cannot be a space. A record payload
  is arbitrary operator bytes and can hold anything, which is why the payload,
  which the spec had left uncounted, is the field that gained the count.
- **Keeping Graceful Restart inside the RIB plugin** (128): rejected. GR
  is capability injection, not route storage. Embedding GR in the reactor
  had been tried before that and rejected as a boundary violation (353).
- **A hand-maintained AFI/SAFI-to-family-name map inside the GR plugin**
  (350): it drifted from the registry, spelling SAFI 4 "mpls" where the
  registry says "mpls-label". That silently broke End-of-RIB matching for
  MPLS-labeled families, and the map was missing five SAFIs. Replaced by
  the family type's own `String()` method.
- **Two pre-tier startup designs** (305): one `runPluginPhase` call per
  tier, where each call overwrites the single `procManager` field and loses
  schema and cleanup visibility for earlier tiers; and a DAG-aware
  coordinator, which was over-engineering for a graph with one dependency
  edge.
- **Replacing BGPHooks without meeting its constraint** (248): BGPHooks
  was a struct of callbacks, deliberately not an interface so that no
  single implementor was forced. It existed to break the import cycle
  between generic plugin infrastructure and BGP. Any replacement must still
  satisfy that constraint.
- **A `family <afi/safi>` prefix on text events** (302): rejected for
  `nlri <family> add/del`, because event grammar and command grammar must
  share a shape. Two NLRI forms remain on purpose: comma-grouped as the
  primary, and keyword-boundary for multi-token NLRIs such as EVPN Type 5,
  which is 12 tokens.
- **Newline as the IPC terminator** (208): rejected at the time for NUL,
  because BGP payloads and JSON strings can contain newlines. The later
  unified framing returned to newline, so anyone proposing newline should
  know the original objection and why it was outgrown: every payload is
  compact `json.Marshal` output.
- **Deduplicating the bgpls, evpn and vpn NLRI decode loops** (231):
  rejected. The three differ in validation, decode, marshaling and family
  declarations, so a shared helper costs more than the duplication. Those
  sites keep `//nolint:dupl` on purpose.
- **Bus-based event delivery** (328): the spec's required replacement for
  BGPHooks, abandoned mid-implementation: the Bus would have had to solve
  per-subscriber format negotiation. The typed EventDispatcher makes direct
  calls instead, for a net 490 fewer lines.
- **Re-export shims letting a core package expose types a plugin now owns**
  (193): `nlri/evpn.go` re-exported EVPN types and created a latent
  `nlri -> evpn -> nlri` cycle, live the moment the plugin imported `nlri`.
  Family plugins own their types.
- **Socket-level tuning for IPC throughput** (294): tried first and
  rejected on measurement. `bufio.Writer` on TCP and 16MB SO_SNDBUF and
  SO_RCVBUF gave no gain, because profiling put the cost in plugin IPC at
  around 27% syscall and 36% goroutine scheduling. That measurement is what
  motivated DirectBridge.
- **SCM_RIGHTS listen-socket handoff** (379): the engine passing a bound
  listener fd to a plugin was fully built, with `SendFD` and `ReceiveFD`,
  Python SDK support and functional tests, and it no longer exists. It was
  fragile by construction: the fd had to arrive between Stage 1 and Stage 2,
  because once a FrameReader starts on Socket B it eats the framing byte and
  the ancillary fd data is lost with NO error. `unix.CloseOnExec(fd)` after
  `ParseUnixRights()` was mandatory, since macOS has no `MSG_CMSG_CLOEXEC`.
- **Splitting `internal/component/plugin/` into 8 sub-packages** (331):
  not achievable. `ServerConfig.RPCProviders` anchors a connected type graph
  (`Handler` to `CommandContext` to `*Server`) that forces handler, startup,
  reload and schema together. Settled at three: `ipc/`, `process/`,
  `server/`.

### Load-bearing invariants

| Invariant | Site | Why (first occurrence) |
|-----------|------|------------------------|
| Internal plugin full 5-stage startup can deadlock | `DirectBridge` startup | Engine blocks waiting for plugin `ready`; plugin blocks waiting for engine config. Decode-only path skips stages. (198, 264) |
| ANY plugin failure aborts the whole startup tier, and that is CORRECT | `StartupCoordinator.PluginFailed` records the first failure and closes the shared `stageCh`, so every plugin waiting in `WaitForStageProgress` gets the same error and is stopped | Owner ruling, 2026-08-11. Raised as a possible defect after one unsupported GRE tunnel in an interface stanza failed the `interface` plugin's configure RPC and took every `bgp-*` plugin down with it. Thomas ruled the blast radius is the point: a router that starts with half its features is worse than one that does not start. A firewall plugin that failed to load while BGP came up anyway would forward traffic unfiltered and look healthy doing it. Fail-closed is the only safe reading, so DO NOT propose making a config-rejection failure survivable |
| `net.Pipe()` writes block until reader is ready | test helpers | Zero-buffering. Tests must start readers before writes OR wrap writes in goroutines. (210, 264, 459) |
| Plugin subprocess stderr consumed by `relayStderrFrom()` | `process.go` | Never reaches test runner's `expect=stderr:contains=`. (451) |
| Strict role (RFC 9234) enforcement stays in plugin | `bgp-role` plugin | Engine is policy-free. (239) |
| Plugin declares what it handles; engine never enumerates | registry API | Families, events, send types all dynamic. (187, 188, 404, 408) |
| Registry lookup is by name (exact, case-sensitive post-normalisation) | `registry.go` | Case normalized at write time, not read time. (187, 412) |
| Auto-linter strips imports between Edits | `auto_linter.sh` hook | `goimports` runs after every Edit/Write; import + first usage must be in same Edit call. (288, 422, many others) |
| Plugin startup needs TWO socket pairs, not one | socketpair creation, `pkg/plugin/rpc` | One socket per direction prevents deadlock. On a single socket both sides can block waiting for the other's response while each holds an unread request. (210) |
| The capability seam is decode-versus-negotiate, not capability-by-capability | `bgp-gr`, `bgp-route-refresh` and `bgp-hostname` versus engine `capability/` | The engine keeps wire parsing and negotiation, and only the CLI and display decode path moves to a plugin. Moving negotiation into a plugin breaks engine=protocol, plugin=policy. (282) |
| A capability-decoding plugin must declare BOTH the decoder function and the capability codes | `registry.Registration` | With only one of the two the plugin registers and is never invoked. It is silent: no error, no log, and the capability renders undecoded. (282) |
| Stopping an internal plugin must close its stdin, not just cancel the context | `process.go` `Stop()` | Context cancellation does not unblock a goroutine already blocked in a pipe `Read()`. (184) |
| No NLRI sub-field token may collide with a top-level event keyword | `textparse/keywords.go` and the rs, rr and persist parsers | The parser finds an NLRI's end by scanning to the next top-level keyword. This holds across all 17 NLRI types by verification, not by construction, and a colliding sub-field word silently truncates parsing. (302) |
| A pre-formatted event cache must key on format AND encoding | event delivery pre-format cache | Keying on format alone hands one plugin another plugin's encoding, silently. (299) |
| The default event format is FormatParsed, and was once FormatHex | `process.go` `Process.Format()` | A missing switch case made FormatHex fall through to FormatParsed. Fixing the fall-through turned every default-format plugin to raw hex. When fixing a switch fall-through, first find who relied on the broken path. (299) |
| The route server deliberately ignores BoRR and EoRR (RFC 7313) subtypes | `bgp/plugins/rs` dispatch matches only `refresh` | A forward-all route server has no refresh cycle to bound, so this is a decision, not an oversight. (263) |
| Every YANG RPC must have a matching registered handler, enforced mechanically | `TestRPCRegistrationTable` | YANG is the authoritative API definition, so a mismatch fails the test rather than surfacing at runtime as an unknown command. (209) |
| Argument-completion requests carry their own short timeout, separate from the command timeout | plugin command dispatch | A user abandons tab-completion long before a 30s command timeout, so reusing that timeout makes completion look broken on any slow plugin. (169) |
| Only a TCP failure retains routes for Graceful Restart | peer-down reason: `tcp-failure`, `notification`, `hold-timer`, `teardown` | A NOTIFICATION shutdown is intentional, so retaining routes would keep withdrawn state alive after a deliberate teardown. (353) |
| Peer state events are delivered sequentially in REVERSE dependency-tier order | event dispatch fan-out | bgp-gr must see peer-down and mark routes stale before bgp-rib acts. Related: OPEN arrives before state-up, because the FSM processes OPEN in OpenConfirm and then transitions. (350) |
| The RPC connection's persistent reader starts lazily via `sync.Once`, not in the constructor | `pkg/plugin/rpc` conn and mux | MuxConn wraps a Conn and takes the reader over, so a reader started at construction races the mux. Two goroutines reading one buffered scanner is a live data race. (292) |
| A write-deadline timeout surfaces as a `net.Error` i/o timeout, NOT as `context.DeadlineExceeded` | deadline writes in `pkg/plugin/rpc` | Callers checking the context error need an explicit translation. Deadline writes default to 30s when the context carries none, because startup RPCs use server-scoped contexts. (292) |
| The dispatcher resolves builtin, then subsystem, then plugin | command dispatcher | This is why unifying duplicate command sets means DELETING the engine builtin and letting the command fall through to the plugin, rather than registering in both places. (245) |
| Route replay goroutines carry a generation counter and converge, rather than taking a snapshot | RS replay | A rapid reconnect otherwise leaves a stale replay goroutine writing into the new session, and a plain snapshot races live delivery. Replay is full pass, then delta loop, then End-of-RIB. (371) |
| A reusable per-worker batch buffer is safe only while the worker is long-lived and serial; a restarted worker must begin with a nil buffer | forward-pool worker | Workers exit on idle timeout and restart, so a buffer parked on shared state hands the restarted worker stale items. (319) |
| Commands that mutate a peer declare `RequiresSelector`, enforced in the dispatcher before the handler | command dispatch | Enforcing inside each handler means one forgotten handler accepts a selectorless mutation across all peers. Wildcard `*` counts as explicit. (368) |
| A subscription delivers FUTURE events only | subscription manager | A plugin subscribing after peers are established sees nothing about existing state, and must query and reconcile. There is no replay-on-subscribe. (170) |
| Responses may arrive out of order, and the presence of a serial decides whether a response is sent at all | serial parsing in the plugin process layer | A command with no `#N` prefix gets no response ever, so a program waiting for one hangs. `Response.Serial` is typed string even for numeric serials, to avoid JSON type ambiguity. (069) |
| A plugin's YANG needs one augment path per template scope it must reach | plugin `yang/` augments; GR needed four | Miss the peer-group or the global variant and config written there is unreachable, with no schema error. (201) |
| The plugin shutdown signal is written synchronously, bypassing the async write queue | `SendShutdown` | The queue may not drain before context cancellation, and the message is then silently lost, so plugins sit until their own timeout. Teardown took 17s instead of 6s. (051) |
| Plugins write logs to stderr only | plugin logging | stdout carries the protocol, so one stray stdout write corrupts framing rather than erroring. (129) |
| A capability that only affects OPEN negotiation belongs in a plugin, not in an engine option | llnh plugin | The engine advertises and records; the plugin acts. Engine-level is right only when the capability changes UPDATE parsing or the FSM. Set by owner directive, reversing the spec mid-implementation. (216) |
| The startup-stage barrier uses one absolute deadline from stage start | `context.WithDeadline(stageStart+timeout)` | `WithTimeout` is relative to the caller, so plugins arriving at different moments get skewed deadlines and the barrier can fire early. (235) |
| Every error path in the plugin server must call `PluginFailed()` and `proc.Stop()` | plugin server error returns | The startup coordinator is a barrier, so one path returning without signalling leaves it waiting forever, with no error and no coordinator-level timeout. (172) |
| The engine must wire the bridge's `DispatchRPC` BEFORE sending the Stage-5 OK | `wireBridgeDispatch` call site | The SDK calls `SetReady()` at the end of Stage 5, so wiring after the loop leaves a window where a ready plugin dispatches into a nil bridge. (294) |
| RPC multiplexing works only if BOTH sides dispatch concurrently | `pkg/plugin/rpc/mux.go` plus per-request dispatch | A multiplexing client against a sequential server just moves the queue into the socket buffer. The symptom is unchanged: silent route drops under load from one heavy peer. (276) |
| The plugin wire format is newline-framed, and that is safe ONLY because every payload is compact `json.Marshal` output | `pkg/plugin/rpc/conn.go` | Compact JSON has no unescaped newline, so pretty-printed JSON silently corrupts framing. Newline was chosen over NUL so `cat`, `grep` and `tail -f` work on a live stream. (397). AMENDED 2026-08-22 by spec-record-answers-2-only-encoding: this still holds for REQUEST lines, and no longer for ANSWER lines. An answer line is framed by the width its own fields state (`answerLineWidth`, `pkg/plugin/rpc/framing.go`), so a counted payload carries a raw `\n` or a trailing `\r` byte for byte. Before that, the uncounted payload forced `replaceNewlines` to rewrite both to spaces INSIDE operator data, silently. `TestCountedValuesCarryNewlinesAndCarriageReturns` holds the round trip. Two consequences a reader must keep: a stated width is attacker-supplied arithmetic and every sum over it can wrap, and the width framing belongs ONLY on the stream that carries answer lines, never on the daemon's rendering |
| An answer line no longer explains itself, so the DOCUMENT owes what the line stopped saying | `docs/architecture/api/process-protocol.md` and `ipc_protocol.md` against `pkg/plugin/rpc/message.go` | Optimizing a wire for parsing spends the legibility it used to carry. `#42 ok status=done type=ndjson key=peers` needed no documentation, because the key names WERE the explanation; `#42 top map 5:peers 0:` is opaque to anyone without the field order. The legibility did not disappear, it MOVED, and the debt was noticed only when the owner asked for it at phase 8. The repayment is a standing rule for this wire: every wire example in an API document carries an in-place decode naming each field, adjacent to the bytes rather than in prose nearby; every count is stated as a BYTE count; every three-letter word is given its meaning where a reader first meets it; and no API page delegates its own subject to another page. `TestAnswerLineTableMatchesDoc` pins the table to the writers. (spec-record-answers-2-only-encoding, owner directive 2026-08-22) |
| Command authorization is fail-OPEN by construction and needs ONE chokepoint covering EVERY dispatch path | `plugin/server/command.go`, where `d.authorizer == nil` allows all | The first implementation checked only when a builtin matched, so subsystem and plugin dispatch bypassed RBAC entirely. Unknown commands are treated as writes. The same guard shape now appears in the gRPC and REST servers. (390) |
| Replay on reconnect must be selective, never bulk | bgp-watchdog `AnnounceInitial` versus `AnnouncePool` | Announcing the whole pool re-announces routes the operator explicitly withdrew. Pool state flips while the peer is down, so the selective set is the correct one at reconnect. (360) |

---

## CLI, web, lookings glass, monitoring

### Current shape

One binary, `ze`, with subcommand dispatch. Interactive CLI over SSH
(Wish library) with Bubble Tea TUI. Dual mode: edit (config editing)
and command (operational RPCs). Shell completion for bash/zsh/fish/nu
with dynamic YANG-driven callbacks (`ze completion words show/run`).
Web UI (HTMX + SSE) provides config view/edit, admin command
execution, CLI modes (form/terminal), live updates. Looking glass
with birdwatcher-compatible API and topology graph. `ze-perf` for
benchmarking; `ze-chaos` for chaos testing; `ze-test` for functional
tests. Prometheus metrics at `/metrics`.

### Evolution

- **004,
  175,
  205,
  232,
  339,
  349,
  356,
  366 through
  370** Editor. Bubble Tea TUI with
  schema-driven completion. `.et` file-based headless test framework.
  Dual-mode (edit/command). Daemon socket probe at startup enables
  reload-on-commit.
- **072,
  199,
  372,
  373,
  381,
  383,
  395,
  440,
  446,
  496,
  518,
  625** CLI. `ze bgp run` + interactive
  merged into `ze cli`. `ze show` (read-only) and `ze run` (all) share
  `BuildCommandTree` from YANG. Shared `cmdutil` package. Pipe
  operators (`| json`, `| table`, `| text`, `| yaml`, `| count`)
  executed server-side where possible.
- **388,
  389,
  396,
  502** CLI commands for metrics/log/
  monitor/signal. `bgp metrics show/list`, `bgp log show/set`,
  `bgp monitor` streaming, `ze status`.
- **266 through
  268,
  307 through
  314,
  357,
  358** Chaos web dashboard. HTMX + SSE.
  ~40-peer active set with adaptive TTL decay. 200 ms SSE debounce.
  Peer grid (alternative to table), donut chart, event toasts, chaos
  pulse, peer filter, convergence trend, chaos rate, trigger buttons.
- **454 through
  475,
  474,
  486,
  498** Main web UI. Server-rendered Go HTML,
  HTMX, SSE. CSP-strict (no `unsafe-eval`). Config view/edit with
  `EditorManager` injected. Admin finder columns, CLI bar (form +
  terminal modes), live update banner, self-signed cert with SAN for
  every interface IP. Looking glass: birdwatcher API, topology graph,
  ASN decorators via `SetDecoratorRegistry`.
- **417,
  433,
  565** `ze-perf` standalone benchmarking
  binary. Cross-implementation (Ze, GoBGP, FRR, BIRD, rustbgpd).
  NDJSON history, stddev-aware regression detection, Docker-orchestrated
  runs.
- **386,
  453,
  482,
  542,
  561** Prometheus metrics. Map-based
  idempotent `PrometheusRegistry`. Per-instance registry (not global)
  for test isolation. Counter metric families invisible until first
  `.Inc()`.

### Abandoned approaches

- **Standalone `ze bgp run` command** (072) — merged into `ze cli`.
- **Separate `rib show in/out/best`** (384, 387) — unified pipeline
  iterator with filters.
- **Custom JS charting for web dashboard** (266) — HTMX + CSS + inline
  SVG only. No JS framework.
- **WebSockets for dashboard** (266, 454) — SSE (server-push only)
  is simpler.
- **Template files for dashboard** (266, 454) — inline Go rendering.
- **`CLICommand` field in RPCRegistration** (395) — YANG `ze:command`
  extension is single source of truth.
- **`bgp` prefix on all commands** (395) — removed. User types
  `peer list`, not `bgp peer list`.
- **Continuous SSE push for the peer-to-peer route flow matrix** (268):
  rejected. At 200 peers the matrix is 200x200, so every tick pushes 40K
  cell updates. It refreshes on tab activation via an HTMX GET instead, and
  cell detail is a separate endpoint.
- **Per-client view modes over SSE** (153): rejected for the chaos
  dashboard. The broker broadcasts identical HTML to every client, so
  per-client mode needs significant broker changes. The grid refreshes by
  HTMX polling and shows ALL peers, which avoids active-set promotion logic.
- **Bubble Tea for the chaos live dashboard** (259): rejected in favour of
  raw ANSI escapes. Bubble Tea takes over stdin and targets interactive
  TUIs, while this dashboard is passive during a run. The rest of the CLI
  still uses Bubble Tea, so the divergence is deliberate.

### Load-bearing invariants

| Invariant | Site | Why (first occurrence) |
|-----------|------|------------------------|
| `EventSource` (SSE) does not support custom headers | `web/htmx.js` | Drives session-cookie auth over Basic Auth. (468) |
| HTMX SSE extension inserts via `innerHTML` | web handlers | Requires pre-rendering through `html/template` for XSS safety. (473) |
| HTMX filter expressions in `hx-trigger` use eval internally | CSP config | Requires `unsafe-eval`; replace with plain JS event listener to keep CSP strict. (454) |
| Pipe operators default table when no format specified | `ProcessPipesDefaultTable` | `| json`, `| table`, `| text`, `| yaml` are format; `| count` is transform. Multiple formatters are an error. (383) |
| `ze config validate` invokes YANG type check only | `internal/component/config/cli/cmd_validate.go` | Plugin `OnConfigVerify` runs only when daemon loads or reloads — parser tests cannot verify plugin-specific rules. (413, others) |
| Bubble Tea owns stdin once the TUI starts, so the editor cannot read piped terminal input normally | editor `load`, paste-mode state | Any multi-line terminal input needs a paste-collecting state with an explicit terminator (Ctrl-D). Pre-TUI stdin detection does not work. (349) |
| Fish single-quotes have NO escape mechanism, so `\'` is literal | `plugins/completion/fish.go` | A `complete -a '...'` containing a regex such as `[^"]*` breaks, leaving `[^` as an unquoted glob, so regexes must live in a named helper. Also, `commandline -opc` at `[-1]` returns the PARTIAL token being typed. (381) |
| Nushell's module system collides with extern command names | `plugins/completion/nushell.go` | `ze.nu` loaded with `use ze.nu` creates module `ze`, which forbids `export extern "ze"`, and `use foo.nu *` does not export externs. Only `source` with a plain `extern` works. (381) |
| The shared completion data source emits `word<TAB>desc` pairs, parsed on the tab by all four shells | `plugins/completion/words.go` | A tab in any YANG Help string silently corrupts completion everywhere. This is safe only because Help is human prose and command names come from `strings.Fields()`. (381) |
| A unit test importing a command package directly proves nothing about registration in the shipped binary | the blank-import site for each command package | Dispatch unit tests pass while every functional test fails with "unknown command". Hit twice in a row: 388 for metrics, 389 for log. (388) |
| `dispatch-command` turns any Go error from a handler into a JSON-RPC error rather than a result | dispatch-command RPC path | A business-logic failure must be a `StatusError` Response with a NIL Go error, so dispatch takes the success path. The `nilerr` linter then forces the error-returning call into a helper that returns a Response. (388) |
| The SSH streaming path does not pass through normal dispatch, so it does not inherit dispatch authorization | SSH streaming executor into `Dispatcher.IsAuthorized` | The executor must hand the username to the streaming handler. Without that explicit check the streaming path fails open. (396) |
| Monitor event delivery is lossy by design | monitor `enqueue()`, `plugin/server/monitor.go` | A slow CLI monitor must never block the engine event path, so enqueue uses select with default, counts drops atomically, and piggybacks the warning on the next event. Do not make it blocking. (396) |
| `SubscriptionManager` is per-Server and keyed by `*Process`, and CLI monitors have no `*process.Process` | `plugin/server/monitor.go` | This is why a second, apparently duplicate `MonitorManager` exists, keyed on client ID. It is not redundant and must not be merged. (396) |
| `ProcessEvent()` writes chaos-dashboard state under a write lock and sets dirty flags ONLY, never doing I/O | `internal/chaos/web/state.go` | HTTP handlers and the SSE goroutine take read locks, and a background goroutine reads the flags every 200ms. I/O inside `ProcessEvent` blocks the event loop and deadlocks against HTTP. This is why the SSE debounce exists. (266) |
| A manual chaos trigger from the web UI emits a standard `EventChaosExecuted` through the normal pipeline | `internal/chaos/web/control.go` | ze-chaos is seed-deterministic and its NDJSON log must stay replayable. Non-chaos control actions (pause, resume, rate) are logged as a separate informational record. Control commands share the event `select`, so there is no priority inversion and no extra mutex. (358) |
| Go forbids a field `Reactor` and a method `Reactor()` on the same struct | any field-to-accessor migration | The package cannot compile in an intermediate state, so the swap is atomic across every construction site. It cost 111 test construction sites across 8 files: count the sites before starting. (229) |
| The SSE partial renderer and the initial page renderer must produce structurally identical markup | chaos `renderStats()` and `writeLayout()` | Two code paths render the same fragment, so updating one leaves the page correct until the first SSE swap. The failure appears seconds after load. (154) |
| New peer-status values are appended, never inserted | chaos `PeerStatus` iota | Inserting shifts existing values and breaks integer comparisons and stored state. Every transition must decrement the state it leaves and increment the one it enters, or counters go negative. (307) |
| The default table formatter is APPENDED at the end of the pipe pipeline, never prepended | `ProcessPipesDefaultTable` | Prepending makes `\| count` count table lines instead of data items. `HasFormatOp` must exclude `count` for the same reason. (383) |

---

## Sub-protocols: BFD, BMP, L2TP, VPP, Firewall, Interface, Host, Gokrazy

### Current shape

- **BMP** (`internal/component/bgp/plugins/bmp`): RFC 7854 receiver +
  sender as DirectBridge plugin. Receiver decodes all 7 message types.
  Sender streams Initiation, Peer Up (real OPEN PDUs), Route Monitoring
  (Adj-RIB-In/Out with per-peer FNV-64a dedup), Route Mirroring
  (verbatim BGP PDUs in TLV type 0), Peer Down, Stats Report, Termination.
  Config via YANG, reconnect with exponential backoff.
- **Interface management** (`internal/component/iface`): backend-split
  (netlink/mock/VPP), monitor via netlink subscription, manage
  (add/remove/addr/unit/create/delete), BGP react on addr events, DHCP
  client for DHCPv4/v6 (DHCPv6 does not install default route — only
  Router Advertisements do), WireGuard via `wgctrl`. Per-protocol backend gate.
- **BFD** (`internal/component/bfd`): RFC 5880 plus RFC 5881/5882/5883
  variants. V4 and V6 transports (IPv6 uses `IPV6_RECVHOPLIMIT`). MD5/SHA1
  authentication. Echo mode. FRR interop scenarios in `test/interop/`.
  BGP client that brings BFD up alongside BGP peer.
- **L2TP + PPP** (`internal/component/l2tp`, `internal/component/l2tp/ppp`):
  RFC 2661 wire, reliable delivery with `seqBefore` unsigned-distance
  comparison, tunnel + session FSM, kernel PPPoL2TP sockets via ioctl,
  LCP/PAP/CHAP/MSCHAPv2 + IPCP/IPv6CP NCPs.
- **VPP backend** (`internal/plugins/fib/vpp`, `internal/component/vpp`):
  GoVPP binapi, AsyncConnect, stats socket for telemetry, PCI
  bind/unbind, classify/policer/QoS for traffic control.
- **Firewall** (`internal/component/firewall`): backend-split (nft on
  Linux). Linux uses netfilter rules via `vishvananda/netlink`.
- **Host inventory** (`internal/component/host`): stdlib parsing of
  `/proc/cpuinfo`, `/proc/meminfo`, `/proc/stat`, sysfs. `/proc/meminfo`
  values in kB converted to bytes at parse time.
- **Gokrazy** (`gokrazy/` + iface-dhcp + ntp): ze owns the appliance
  clock (ze's NTP), DHCP wiring per-interface. `ExtraFileContents`
  seeds config at build time.

### Evolution

Interface: 489,
490, 491,
492, 493,
494,
522,
523,
524,
526,
557,
566,
567,
568,
576,
582,
589,
615.

BFD: 555,
556, 559,
560, 561,
562, 563,
564, 565.

L2TP + PPP: 594,
595, 596,
597, 599,
602,
609, 616,
620, 622.

VPP: 610,
611, 612,
613, 614,
615, 623,
627, 629.

Firewall: 584,
585, 586,
587, 588,
635.

Host + Gokrazy: 577,
578, 579,
580,
581, 583,
631.

BMP: 574,
647.

### Abandoned approaches

- **BFD `Session`, `Engine`, `Clock` generic type names** (555) —
  collided with existing types project-wide; renamed.
- **L2TP signed-subtraction for sequence ordering** (595) —
  `int16(a-b)<0` mis-classifies diff=32768; use unsigned distance.
- **L2TP XOR loop shared between encrypt and decrypt** (594) — chain
  key is `MD5(secret || prev_ciphertext)`, decrypt needs different
  `prev_ciphertext` source than encrypt.
- **MAC on tunnel kinds applied generically** (567) — non-tunnel
  interfaces use MAC from list-level; tunnel-specific MAC clearing.
- **`prometheus/procfs` for host inventory** (631) — pure stdlib
  parsing of `/proc/*` is ~50 lines and clearer.
- **Host `cpu_capacity` hardcoded values** (631) — use ratio (max vs
  lower), not hardcoded 1024/768.
- **Gokrazy `WaitForClock: true`** (580) — ze owns the clock; waiting
  for it causes boot hang.
- **Firewall `DefaultBackendName` exported** (633) — collided with
  iface/traffic; removed export.
- **BMP synthetic 29-byte OPENs** (574, 647) — no capabilities,
  collectors could not analyze negotiated features; replaced with
  event-based OPEN caching.
- **BMP ribout dedup keyed by messageID** (574) — messageID is unique
  per UPDATE, so dedup never fired; replaced with attribute-hash dedup.

### Load-bearing invariants

| Invariant | Site | Why (first occurrence) |
|-----------|------|------------------------|
| `IP_RECVTTL` cmsg arrives as `IP_TTL` | BFD UDP parser | Setsockopt uses `IP_RECVTTL` (enable flag); cmsg type is `IP_TTL` (value carrier). (559) |
| DHCPv6 does not install default route | `iface/dhcp.go` | Default gateway comes from Router Advertisements. (576) |
| `runtime.LockOSThread()` is mandatory before netns switch | iface tests | Without it, scheduler can move goroutine out of the target namespace. (494) |
| `accept_ra` must be `2` not `1` when `forwarding=true` | `iface/sysctl_linux.go` | The kernel ignores Router Advertisements at `accept_ra=1` when forwarding is on. (489, 491) |
| VLAN composite names must fit IFNAMSIZ (15 chars) | `iface/validateIfaceName` | Combined (parent + `.` + vlan-id), not parent alone. (489, 491) |
| `SetKernelWorker` must be called BEFORE `reactor.Start()` | L2TP kernel worker | Reactor goroutine reads `r.kernelErrCh`; write after Start races. (599) |
| `/proc/meminfo` values are in kB, not bytes | host inventory | Convert at parse time; field names carry `-bytes` suffix. (631) |
| `unsafe.Pointer` for RTC ioctl triggers gosec G103 | ntp plugin | Unavoidable for kernel ioctl. Document with `//nolint:gosec` + reason. (577) |
| BMP OPEN cache + dedup cleanup must run before senders-empty check | `bmp/bmp.go:handleStructuredEvent` | Peers establish before collectors connect; early return loses OPEN PDUs. (647) |

---

## Testing infrastructure

### Current shape

Three test flavors:

- **`test/*/*.ci`** — functional: `stdin=config`, `tmpfs=script.py`,
  `cmd=background/foreground`, `expect=bgp:hex/json/text/contains=`,
  `action=sighup/rewrite`. Python plugin scripts via `test/scripts/ze_api.py` (retired, no successor) <!-- doc-links: ignore (deleted 2026-08-28 by eae282592 with no replacement) -->.
- **`test/editor/**/*.et`** — headless TUI replay with keystrokes and
  expectations.
- **Go unit/integration tests** — `_test.go` alongside sources.

Plus fuzz corpora in many packages, `ze-chaos` for chaos, `ze-perf`
for benchmarks.

### Evolution

- **026,
  043,
  044,
  045,
  131,
  132,
  135,
  206,
  339** `.ci` format evolution.
  ExaBGP-inspired runner rewritten in Go. `stdin=`, `tmpfs=`,
  `cmd=background/foreground` for self-contained tests. Test IDs
  alphanumeric (0-9, A-Z, a-z) stable across runs. `ze-test bgp
  parse/ui/encode/decode/plugin`.
- **205,
  428** `.et` framework. Key sequences
  + expected viewport/completions. Extended with `option=session:user=X`,
  `expect=file:path=X:contains=Y`, `restart=` for multi-session and
  persistence tests.
- **255 through
  265** `ze-chaos` tool.
  Seed-deterministic. Multi-peer scenarios, chaos events (flap, timer
  expiry, malformed message), validation model, NDJSON event log,
  property-based testing with shrinking, in-process mode with
  VirtualClock.
- **274** `.ci` diagnostics. Field-
  level JSON diff, named suite failures, parse test reproduction
  commands, full hex in debug commands.
- **338,
  354,
  355,
  393,
  446,
  550,
  558** Test coverage audits.
  Fuzz targets across wire parsers. Observer-exit antipattern
  (`sys.exit(1)` after `daemon shutdown` → test framework sees
  "success") replaced with `runtime_fail(msg)` pattern.
- **608** Concurrent-test flake
  patterns. Locked-write/unlocked-read, subscribe-before-broadcast,
  gate-handler, barrier FIFO, cleanup-drains-work.
- **217** Fix the test harness before the code under test. The ExaBGP suite
  was predicted to reach 33/37 after harness repair and reached 20/37: the
  harness had masked 13 real wire-encoding gaps, so every prior read of that
  suite's signal was wrong.
- **261** A black-box chaos tool can only assert on what it sends and
  receives. Five of ten planned RFC properties were unimplementable from
  outside, because they need internal transitions. This is why in-process
  mode with a virtual clock exists.

### Abandoned approaches

- **`sys.exit(1)` after `daemon shutdown` in Python observers** (550,
  558) — test framework sees the daemon's successful exit, observer
  failure is lost. Use `runtime_fail(msg)` which emits a valid slog
  line at ERROR level.
- **Drops as "acceptable flake"** (562, 604) — race detector missing a
  race does not prove the race does not exist. Fix the code when memory
  model says "race".
- **Count-only assertions** (340, 360, 400, 446, etc.) — parsing can
  produce accidentally-matching counts through data corruption.
  Assert on content (keys, values, wire bytes).
- **`go test ./...` from module root when `tmp/*.go` exists** (557,
  610, 619) — scratch files break unit-test phase. Research subagents
  must use `.txt` or build-tagged directories.
- **Wire tests written as "old implementation versus new implementation"**
  (030): abandoned. Both sides were broken for reflector attributes, so
  the test passed while proving nothing. Replaced with expected-bytes
  assertions taken from the implementation known to be correct.
- **A test fixture whose SHAPE the implementer invented** (396): flat
  JSON, while production ze-bgp nests under `"bgp":{}`. The tests were
  self-consistent, green, and proved nothing, and only independent deep
  review caught it. Fixture shape must be captured from real output.
- **A functional-test daemon with different reload wiring from production**
  (225): SIGHUP tests then exercised only direct-reload and never the
  two-phase coordinator. A test daemon that diverges from production wiring
  produces green tests over an untested path.
- **`net.Pipe()` as the transport for a BGP session** (264): it cannot
  carry one: both peers write OPEN simultaneously and the unbuffered pipe
  deadlocks. Mock BGP connections use TCP loopback pairs. The symptom when
  this is missed is that the first integration test hangs forever.
- **Timer-non-firing and backward-clock faults in ChaosClock** (265):
  deliberately excluded. Both cause permanent FSM stalls and break
  everything, rather than exposing a resilience bug. Only jitter (0.8 to
  1.2x) and sleep extension are injected.
- **A ring buffer for the chaos event log** (364): rejected for an
  unbounded buffer. Dropping route events corrupts convergence counts, so a
  lost event becomes a false validation failure.

### Load-bearing invariants

| Invariant | Site | Why (first occurrence) |
|-----------|------|------------------------|
| `cmd=api` lines in `.ci` files are documentation metadata, not execution directives | `internal/test/runner/` | The route must come from the plugin. (362, 414, 483) |
| `expect=stderr:contains=` only fires inside `ExpectExitCode != nil` branch | `runner_exec.go` | Without `expect=exit:code=`, runner falls through to peer-wait path. (623) |
| Plugin subprocess stderr is consumed by `relayStderrFrom()` | `process.go` | Never reaches test runner's stderr match. (451) |
| Background `.ci` processes do NOT get `ZE_READY_FILE` | `runner_exec.go` | Only foreground path writes daemon.pid + daemon.ready. (623) |
| `parse/` test runner only extracts `stdin=config` and runs `ze validate` | `internal/test/runner/` | `cmd=foreground`, `tmpfs=`, `expect=stdout:contains=` are silently skipped. (449) |
| Python test library (`test/scripts/ze_api.py` (retired, no successor) <!-- doc-links: ignore (deleted 2026-08-28 by eae282592 with no replacement) -->) must track Go protocol | Python SDK | Changing engine RPC without updating Python hangs 129+ tests. (291, 397, 497) |
| Stored test state (registry, `sync.Once`) must Snapshot/Restore | `t.Cleanup` | `Reset` empties all globally registered decoders; fresh registry breaks tests. (240, 533) |
| `net.Pipe()` deadlocks sequential write-then-read | test setup | Zero buffering. Wrap writes in goroutines or start reader first. (210, 264, 459, 609) |
| A `.ci` using `action=sighup` must contain at least one `tmpfs=` block | functional runner sighup path | `daemon.pid` is only written when a tmpfs directory exists, so the signal can never be delivered. (234) |
| A test for JSON-processing code must feed it a JSON string, never a hand-constructed struct | any `parseEvent`-style test | Building the struct skips the function that holds the bug. The RS event parser was wrong for every production event while its unit tests were green. (263) |
| Registry-count assertions use exact equality, not `>=` | RPC registration table test | A `>= N` threshold passes while RPCs silently disappear. (209) |
| In a plugin test, prove the event loop is running before calling any runtime method | two-socket test setup | Send on Socket B first, then call on Socket A. Calling straight into A races the shared reader. (210) |
| Chaos log determinism comes from the writer, not from the event | NDJSON writer | Sequence numbers and `time-offset-ms` are assigned at the serialization boundary, and the diff tool ignores `time-offset-ms`. Relative offsets mean one seed produces one comparable log. (260) |
| `.ci` JSON expectations match on content (NLRI plus action), never on position | runner JSON comparison | Ze sends routes lexicographically, not in config order. Peer and direction fields are excluded as environment-dependent. (121) |
| In a `parse/` `.ci`, `expect=stderr:contains=` switches the test into NEGATIVE mode | parse test runner | A positive parse test that adds a stderr expectation inverts its own verdict. Positive tests use `expect=exit:code=` only. (380) |
| Every `.ci` that starts an SSH listener needs its own port | SSH-based `.ci` tests | Tests run in parallel, so a shared port fails non-deterministically and reads as a flake. (391) |
| A test package needs an explicit import file to fire `init()` registrations | `all_import_test.go` | Blank imports elsewhere do not fire in a test binary, so registry-backed tests see an empty registry. (334) |
| Test runners send SIGTERM plus a grace period, never SIGKILL | runner teardown | SIGKILL bypasses cleanup, so the shutdown behavior under test never runs. (051) |
| Bulk format migrations must exclude fixtures held in a foreign format, and documents quoting the old format | ExaBGP test inputs, spec "before" examples | A repo-wide replace rewrote `test/exabgp/*/input.conf` out of ExaBGP syntax, which is the one thing those files exist to hold. (135) |
| Config-parse success and wire-encoding correctness are independent | ExaBGP compat suite | A config that parses without error routinely produced wrong bytes, so a green parse test is not evidence about the wire. (217) |
| FakeClock and VirtualClock are not interchangeable | `internal/core/clock/` and `internal/chaos/` | FakeClock's timers are inert, for unit tests. VirtualClock keeps a min-heap firing in deadline order on `Advance()`, with same-deadline FIFO. Virtual time never virtualises TCP I/O or goroutine scheduling. (264) |
| An injected clock must reach objects built before injection | `reactor.SetClock()` iterating `r.peers` | It originally set only the reactor-level and recentUpdates clocks, so peers created during plugin load kept the real clock and reconnect backoff ran in real time. (264) |
| Generated test UPDATEs must carry unique prefixes | UPDATE batch helpers | Reusing one prefix makes the RIB dedup the batch to a single entry, so per-item assertions pass or fail for the wrong reason. (284) |
| The `.ci` checker groups expectations by `(conn, seq)` | `internal/test/peer/expect.go` | Messages inside one seq group match in any order, and the groups are strictly sequential. Writing expectations without knowing this produces tests that appear order-insensitive when they are not. (362) |
| Exact `expect=bgp:hex=` assertions are brittle by construction | test/plugin, test/encode | ze auto-includes the extended-message capability, which changes the OPEN byte count, and an iBGP session auto-adds LOCAL_PREF. Verify the actual bytes first, or assert on capability and dispatch instead. (393) |
| A peer left at default connect plus accept fights the test peer for the same port | `.ci` configs that also start ze-peer | Disable connect explicitly rather than tuning timing. (393) |
| A functional test must have exactly one teardown mechanism | test peer plus plugin script | When both tear down, they race. The fix was making the peer send a route after reconnect, as an in-band "job done" signal. (074) |
| The cross-type NLRI consistency test must enumerate EVERY NLRI type | `nlri` consistency tests | EVPN was missing, and that omission is exactly where the ADD-PATH wire corruption lived. A type absent from the table is the type that breaks. (071) |
| Adding a CLI flag means teaching the `.ci` runner to parse it | runner command-line parsing | After `ze bgp decode` changed its default output, every decode test still ran in JSON mode because the runner never parsed `--json`. A `cmd:` versus `cmd=` separator mismatch produces the same silent no-op. (190, 191) |
| An interop scenario's NAME does not say which role ze plays in it | `test/interop-ipsec/scenarios/` | `eap-tls` has ze as the EAP PEER, not the authenticator: its `ze.conf` carries `connection-type initiate` and strongSwan's `swanctl.conf` carries `remote { auth = eap-tls }`. A comment on `indicateSuccess` (`internal/component/ike/eap/eap_tls.go`), which runs only on the authenticator side, cited that scenario as its proof for two versions. Read the pair of config files, never the directory name. (spec-fixit-eap-tls-escape-hatch-kills-the-daemon) |
| A scenario whose feature became unreachable is REPURPOSED, not retired | `test/interop-ipsec/scenarios/eap-tls` | Go 1.27 removed the GODEBUG that let ze export an EAP-TLS MSK on a TLS 1.2 session with no RFC 7627. The scenario can no longer reach a tunnel. Retiring it would have deleted the only test whose peer is not a second copy of ze. It would also have left two spec rows naming a test that no longer exists. It now asserts the completed handshake, the attributed refusal, and that neither end installs an XFRM SA. That is strictly more than it asserted before. When two options are on the table and one needs the owner's word, ask first whether the OTHER one is right on its own. (spec-fixit-eap-tls-escape-hatch-kills-the-daemon) |
| A capability documented in prose and exercised nowhere is one nothing will tell you has gone | `cmd/ze/main.go` header versus scenario eap-tls's `ze-env` | The same escape hatch was written in two places. The scenario SET the variable, so the day Go removed the setting the scenario went red and named the file; the header only DESCRIBED it, and would have outlived the mechanism indefinitely. `TestNoShippedGuidanceNamesARemovedGODEBUG` (`cmd/ze/godebug_guidance_test.go`) is the holder the prose lacked, and it derives its population from the toolchain in use rather than from a list. (spec-fixit-eap-tls-escape-hatch-kills-the-daemon) |

---

## Migration: ExaBGP to Ze

### Current shape

Two separate tools under `internal/exabgp/`, isolated from engine:

- `ze exabgp migrate` — one-shot config converter (ExaBGP → Ze).
- `ze exabgp plugin` — runtime bridge wrapping ExaBGP plugins for use
  with Ze's 5-stage protocol.

Engine code: zero ExaBGP format awareness.

### Evolution

- **001,
  008,
  041,
  096,
  125,
  219** Migration tool. Heuristic
  version detection. Three-version chain. Named transformations.
  Plugin bridge: handles 5-stage internally, switches to JSON
  translation after `ready`.
- **179,
  181,
  183,
  281,
  344** ExaBGP syntax removal from
  engine. `announce { }`, `static { }`, `operational { }` blocks
  deleted. `multi-session`, `operational`, `aigp` capabilities
  rejected during migration.
- **045** BGP-LS decode output deliberately diverges from ExaBGP. Ze emits
  arrays (`remote-router-ids`, `sr-adj`) where ExaBGP emits a single value
  and silently loses duplicate keys for multi-instance TLVs. Ze chose the
  lossless format.
- **056** Ze uses plural JSON keys for community attributes
  (`communities`, `extended-communities`, `large-communities`), which is a
  deliberate divergence from ExaBGP's singular forms. Reverting to "match
  ExaBGP" would be a regression.

### Abandoned approaches

- **ExaBGP tests built with Ze syntax as input** (125) — tests passed
  without exercising migration. Rebuilt with real ExaBGP fixtures.
- **Full `dual-registration` for `daemon-*` RPCs** (228) — replaced
  atomically; ze has no users, no compat needed.

### Load-bearing invariants

| Invariant | Site | Why (first occurrence) |
|-----------|------|------------------------|
| All ExaBGP-aware code lives in `internal/exabgp/` | package boundary | No imports from other packages. (181) |
| Complex NLRI families (FlowSpec, MVPN, MUP) are API-only, not config | config schema | Removed from config for simplicity. (181) |
| ExaBGP text protocol plugins bridged at the boundary | `ze exabgp plugin` | Incompatible with Ze's own RPC framing. That framing was NUL-delimited when this boundary was drawn and is newline-delimited today (`pkg/plugin/rpc/conn.go`); the bridge is required either way. (219, 483) |
| SAFI detection must test for `rd` before `label` | migration SAFI detection | RFC 8277 labeled-unicast (SAFI 4) and RFC 4364 L3VPN (SAFI 128) share label syntax, so a label-first test classifies every VPN route as labeled unicast. (096) |
| ExaBGP fixture configs under the exabgp test tree must be excluded from bulk config migration | `test/exabgp/*/input.conf` | A bulk migration rewrote them into Ze syntax twice, in 135 and again in 138, which made the migration tests pass without exercising migration. (138) |
| The ExaBGP bridge reuses ONE scanner across the startup and JSON phases | bridge phase transition | Creating a fresh scanner at the switch drops bytes already buffered. (125) |
| Two migration systems and two schemas exist, and they must not be merged | `config/migration` (Ze syntax evolution) versus `internal/exabgp` (ExaBGP to Ze) | ExaBGP spells its API block `api { processes [...] }` where Ze uses `process { }`, so running an ExaBGP config through the Ze schema makes the API-block migration silently never execute. (125) |

---

## Cross-cutting: engineering practice

### Current shape

Facts that belong to no single subsystem: Go traps that survive review, the
logging construction rule, how mechanical refactors interact with the
edit-time gates, and what does and does not cross a process boundary.

### Evolution

- **129** Logging gives every subsystem its own logger instance instead of
  calling `slog.SetDefault`, so several subsystems in one process can be
  enabled and disabled independently.
- **288** Optional dependencies are injected by SETTER (`SetClock`,
  `SetDialer`, `SetListenerFactory`, `SetMetricsRegistry`), not by
  constructor parameter. The reason is mechanical: those constructors are
  called from 34 or more test sites, and a constructor change forces every
  call site to change for a dependency most of them do not use.
- **001** The founding plan set a hard product boundary, BGP protocol only,
  no FIB manipulation, matching ExaBGP, and recorded it as a boundary that
  "has never moved". Ze today ships kernel and VPP FIB backends and a
  sysrib, so crossing it was a later deliberate expansion.

### Abandoned approaches

- **Long-lived type aliases as a migration device** (197): tried and
  regretted. An alias hides the coupling: when the original type changes
  shape, every unmigrated consumer breaks silently. Use an alias only as a
  within-one-commit step toward direct imports.

### Load-bearing invariants

| Invariant | Site | Why (first occurrence) |
|-----------|------|------------------------|
| Package-level state touched by plugin runner goroutines needs `sync.Once` or a mutex, never a plain bool guard | builtin command registration in the RIB plugin | "Read-only after startup, no mutex needed" was false: two overlapping startups race the guard and the map it protects. (399) |
| A test file lives with the functions it tests, not with the types it uses | package layout after an extraction | Putting the test beside the types recreates the import cycle the extraction removed. (248) |
| Mechanical moves and new registrations fight the edit-time gates, and the fix is edit ordering | duplicate-symbol and init/Register gates | Adding a type to its new home while the old one still exports it is refused, so delete first. A `Register` call in the same edit as an empty `init()` is refused, so split the edit. (242) |
| Renaming a package to a short common noun collides with local variables at every call site | renames such as `peer`, `runner` | A local `peer` shadows the package import, and the failure appears as unrelated compile errors far from the rename. (133) |
| Bulk `sed` over source and specs has two recurring traps | mechanical field removal, doc rewrites | A line-delete pattern removes the WHOLE registration when the struct literal is one line, so use field-only substitution. A bulk replacement over `.md` corrupts specs that quote the old format as before/after evidence. (133, 395) |
| A package-level `var logger = slogutil.Logger(...)` is a bug | logger construction | Package vars run before `main()` reads config, so config log settings are silently ignored. `LazyLogger()` wraps `sync.Once` to defer construction. (182) |
| CLI flags do not reach forked child processes | `internal/component/bgp/config/loader_create.go` | Env vars are the only reliable channel, which is why the chaos seed and rate are plumbed as `ze.bgp.chaos.seed` in addition to the flag. (265) |
| Creating a set of interconnected new files is blocked by the related-refs hook, which reads the file on disk before applying an edit | `require-related-refs` | Cross-references to files that do not exist yet are rejected, and an Edit that WOULD fix stale refs is blocked by those stale refs. Create empty stubs first. (400) |

---

## Cross-cutting: reading this document in practice

- **"Why is component X structured this way?"** — find X in the
  section index above; the Evolution subsection lists the
  summaries in order. Read the oldest first to see what was being
  rejected; read the newest first to see the current state.
- **"Can I propose approach Y?"** — search "Abandoned approaches"
  for Y. If Y appears, the entry states why it did not work. When the
  entry carries a link, read that summary too before proposing.
- **"Why does the code require invariant Z?"** — search "Load-bearing
  invariants" for Z. The table names the enforcement site and the
  summary that records the reasoning.
- **Still unclear?** — fall back to `../../ai/LEARNED-INDEX.md`
  for the curated topic index, then read the specific summary.

For summaries 401 and above, the numbered file remains the authority and
this document is the map to it. For the retired 001 to 400 band, this
document is the authority, because those files no longer exist.
