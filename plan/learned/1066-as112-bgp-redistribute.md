# 1066 -- as112-bgp-redistribute

## Context

The `as112` DNS plugin now originates its four COVERING prefixes (192.175.48.0/24,
192.31.196.0/24, 2620:4f:8000::/48, 2001:4:112::/48 -- NOT the /32,/128 host addresses
bound on `lo`) into BGP as a redistribute source, exactly like `static`/`connected`:
the routes enter the main RIB only when the operator writes
`redistribute { destination bgp { import as112 } }`. This is the EASY path; the
pre-existing hand-authored `bgp { update{nlri} watchdog } + healthcheck` composition
stays as the FULL-CONTROL path. Depended on `spec-redistribute-late-join-replay`
(learned 1062), which had already landed the `ReplayID` late-join mechanism this reuses.
Preserves the AS112 layering rule (learned 1034): as112 never reads `bgp {}` config and
BGP hardcodes no AS112 knowledge.

## Decisions

- Added a **generic** origin-ASN + community capability to the redistribute path
  (`RouteChangeBatch.OriginASN uint32` + `Community []uint32`), NOT an as112-specific one.
  Any source can model itself as a virtual router with its own ASN. The BGP consumer emits
  `origin igp origin-as <asn>` (vs the legacy `origin incomplete`) and
  `community [<hi>:<lo> ...]` only when the fields are set. as112 is the first user; a
  future `import static` originating as an AS is the plausible second.
- **`origin-as` is a NEW reactor primitive, distinct from the verbatim `as-path`** (user
  decision). The existing announce-path `as-path` is sent VERBATIM with no eBGP prepend --
  a deliberate ExaBGP-style exact-path / route-server-transparency feature
  (`buildBatchASPath` returns it unchanged; `writeMandatoryAttrs` copies it byte-for-byte;
  asserted by `TestBuildBatchASPath_Explicit`). So `as-path 112` reaches an eBGP peer as
  `[112]` and is rejected by enforce-first-as. `origin-as <asn>` means "originate with this
  AS, apply the normal export rule": the reactor synthesizes `[asn]` for iBGP and
  `[localAS, asn]` for eBGP. It is carried as a batch DIRECTIVE (`NLRIBatch.OriginAS`,
  `NLRIGroup.OriginAS`), never a wire attribute, so it does not set the wire AS_PATH and the
  reactor's normal per-peer synthesis runs. Default (0) leaves every existing path
  byte-identical (verified: the existing eBGP/iBGP/explicit tests still assert the same
  bytes).
- **Default asn = 112** (user decision; the old as112-3 design defaulted origin to the
  operator's own AS). The redistribute source models an AS112 virtual router, so 112 is
  the natural default; an operator/private ASN is an explicit override.
- Community renders as `<c>>16>:<c&0xFFFF>` (high16:low16), which round-trips through
  `attribute.ParseCommunity` for EVERY uint32 including well-known values (NO_EXPORT
  0xFFFFFF01 -> `65535:65281`, NOPEER 0xFFFFFF04 -> `65535:65284`). Avoids needing a
  uint32->name reverse table on emit.
- OriginASN/Community are **batch-level** (constant per as112 source), threaded through
  the orchestrator's `dispatchEntryToConsumer` into `RouteEntry`. Value-typed; nil/zero
  for every existing producer.
- The as112 producer is a **reconciler** holding announced state (families, asn,
  community). `apply(cfg, serving)` diffs old vs new: withdraw dropped families, (re)add
  new-or-attribute-changed families. So narrowing address-family withdraws only the
  dropped family, and an identical re-apply emits nothing (no route flap). State computed
  under the lock, batches emitted OUTSIDE it (EventBus dispatches synchronously and a
  replay re-emit re-enters the producer).
- Watchdog serving gate: `announce = enabled && (!watchdog || serving)`. `serving` is
  driven at RUNTIME by the DNS server's anycast (non-loopback) listener transitions:
  `server.go listenerChanged` notifies `prod.onServingChanged` on the 0<->1 edge, and the
  producer RE-READS the live serving state via `mgr.serving()` on each `reconcile` (it
  stores NO serving snapshot). The dnsserver reports the edge on bind, Stop, AND a listener
  CRASH (up=false), so a serving loss withdraws the covering prefixes with no config change
  (AC-7) and recovery re-announces. Config and serving are two independent inputs funneled
  through one `reconcile`. This is a listener-liveness gate, NOT a per-query/anycast-path
  health probe -- for that, operators use the hand-authored healthcheck path (two-paths
  story, R-2). `watchdog` defaults true in BOTH code and YANG. Loopback listeners are
  excluded from the aggregate (diagnostic only), so an anycast-bind failure correctly
  withholds.
- Reused the landed `ReplayID` mechanism (1062): as112 subscribes to
  `redistevents.ReplayRequestEvent`; `reemitAll` re-emits the current announced set with
  the echoed `ReplayID` and does NOT mutate state. The spec's `Replay:true` language
  predated the landed token API and was reconciled to `ReplayID` (recorded in the spec
  header).
- Doctor: new neutral check `checkAS112RedistributeOriginCoordination` (+ diagnostic code
  `doctor-as112-redistribute-origin-uncoordinated`) warns on `asn 112 + import as112`
  toward an eBGP public peer -- the redistribute-path sibling of the existing
  `asn.local 112 + replace-as` check. Reads the `config.Tree` generically (no bgp or
  redistribute-plugin import) via `redistributeImportsSourceIntoBGP`.
- YANG: `asn` = `zt:asn` (uint32 1..4294967295), `watchdog` = boolean default true,
  `community` = `type string` (NOT the AA:NN-only `zt:community` pattern, which would
  reject well-known names), validated by `attribute.ParseCommunity` at config time --
  matching the `ze-bgp-conf` community-leaf precedent and learned 1034.
- as112 obtains the EventBus via the P2-typed `ConfigureEventBus` hook; the redistribute
  source is registered in `init()` (sync.Once) so `import as112` resolves during
  `ze config validate` (which imports plugins but starts no engine), matching
  static/connected.

## Consequences

- The origin-ASN/community capability is genuinely reusable: the pipeline stays
  protocol-agnostic (no "as112" spelling in redistevents/consumer/orchestrator). "Origin a
  redistribute source as if it were a neighbor AS" is now a first-class, generic feature.
- Two operator paths, both documented in `docs/guide/as112.md`: redistribute (easy,
  source-level attributes, process-health gate) vs hand-authored update block (full
  per-peer policy, anycast-path probe gate).
- Existing producers (l2tp/connected/static/fakeredist) are byte-for-byte unchanged:
  OriginASN=0 + Community=nil keep `origin incomplete`, no as-path/community, and add no
  per-event allocation (AC-10).
- Late-join delivery works for as112 via the 1062 replay path (a peer establishing after
  emit receives the covering prefixes).

## Gotchas

- The as112 config JSON section stringifies EVERY leaf value (numbers arrive as strings,
  like `enabled` = "true"); `asn` is parsed with `strconv.ParseUint` on an `asString`
  result, matching sibling `geodns`. A float64/json.Number code path would silently ignore
  a custom `asn`.
- `Community []uint32` on the POOLED batch: `ReleaseBatch` must set it to `nil` (drop the
  reference), NEVER `clear()` it. Unlike the pool-owned `Entries` backing array, the
  Community backing array is producer/config-owned; clearing it would corrupt the
  operator's configured community across the pool's next reuse.
- The `update text community` grammar needs BRACKETS for more than one community
  (`community [a b]`); a bare `community a b` parses only the first token and treats the
  rest as new keywords. `formatAnnounce` always brackets (a single community brackets
  fine too).
- With `watchdog false` the producer announces on config-apply regardless of whether the
  DNS server actually bound (an `mgr.apply` failure is logged, not fatal). This is what
  lets a `.ci`/interop test drive the REAL producer without port-53 privilege.
- (from 1062, still applies) A passing `.ci` is not evidence a feature works -- mutation
  test it by disabling the producer emit. Per-peer replay targeting is wire-indistinguishable
  (the reactor suppresses duplicate announces), so it must be unit-tested, not `.ci`-tested.
- **FEATURE-NOT-WIRED (caught in review):** the first cut created the producer in
  `runAS112Plugin` (`newAS112Producer` + `subscribeReplay` + deferred `withdraw`) but the
  `OnConfigure` closure never called `prod.apply(cfg, serving)`, so the real plugin never
  announced at runtime. The producer unit tests call `apply()` directly and the fakeas112
  `.ci` uses a SEPARATE producer, so NEITHER caught it -- only the real-as112 interop
  scenario (and a real-as112 `.ci`) exercise the `OnConfigure -> apply` seam. Lesson: a
  producer wired into a large inline `OnConfigure` closure needs a real-plugin
  functional/interop guard, not just isolated unit tests of the reconcile method.
- **The spec's "no grammar change needed -- the consumer just emits the tokens" key
  insight was WRONG.** It assumed `as-path <asn>` gets the eBGP localAS prepend. The
  announce path sends an explicit as-path VERBATIM (route-server transparency / ExaBGP
  exact-path), confirmed by a code trace and `TestBuildBatchASPath_Explicit` (localAS NOT
  prepended). The eBGP prepend the spec assumed only happens on the FORWARD path
  (learned/re-advertised routes, `RewriteASPath`), never the announce path
  (locally-originated). This forced a new `origin-as` reactor primitive. Lesson: "set an
  origin AS" and "send an exact AS_PATH" are DIFFERENT operations; verify which one the
  announce path actually does before assuming a token gives you the eBGP prepend.
- **A stored serving-state SNAPSHOT races concurrent listener edges (review-caught).** The
  first runtime-serving cut stored a `serving bool` written by two callers (config-apply +
  listener edge); two concurrent opposite-direction edges (a bind-up racing a crash-down)
  could leave the stored value inconsistent with the true listener state -- covering
  prefixes announced-while-not-serving (an RFC 7534 blackhole) that did not self-heal until
  the next edge. Fix: do NOT store a snapshot of state that lives elsewhere -- read it LIVE
  (`servingFn` = `mgr.serving`) on each reconcile from the always-correct listener up-set
  map, so whichever reconcile runs last reads the true aggregate and converges. Lock order
  p.mu->s.mu (servingFn read under p.mu); listenerChanged releases s.mu before notifying so
  there is no inversion; emit stays lock-free. Guarded by a concurrent `-race` regression
  test (`TestAS112Producer_ConcurrentServingConverges`).

## Files

- `internal/core/redistevents/events.go` -- `RouteChangeBatch.OriginASN`, `.Community` (generic, value-typed)
- `internal/core/redistevents/pool.go` -- Acquire zeros both; Release nils Community (never clears it) and zeros OriginASN
- `internal/component/config/redistribute/consumer.go` -- `RouteEntry.OriginASN`, `.Community`
- `internal/component/bgp/redistribute/consumer.go` -- `formatAnnounce` emits `origin igp origin-as <asn>` / community; `originIGP`
- `internal/component/bgp/types/types.go` -- `NLRIGroup.OriginAS`, `NLRIBatch.OriginAS` (batch directive)
- `internal/component/bgp/plugins/cmd/update/update_text.go` -- `origin-as` grammar token (`kwOriginAS`), `parsedAttrs.OriginAS`, threaded into NLRIGroup + NLRIBatch
- `internal/component/bgp/reactor/reactor_api_batch.go` -- `buildBatchASPath`/`writeASPath`/`writeMandatoryAttrs` origin-as branch: `[asn]` iBGP, `[localAS, asn]` eBGP; verbatim `as-path` unchanged
- `internal/component/bgp/reactor/reactor_batch_test.go` -- origin-as iBGP/eBGP + explicit-beats-origin-as unit tests
- `internal/component/bgp/plugins/redistribute_egress/redistribute.go`, `replay.go` -- thread batch OriginASN/Community through `dispatchEntryToConsumer` (both incremental + replay call sites)
- `internal/plugins/as112/config.go` -- `ASN`/`Community`/`Watchdog` parse + validation; `as112DefaultASN`
- `internal/plugins/as112/yang/ze-as112-conf.yang` -- asn/community/watchdog leaves + revision
- `internal/plugins/as112/events/events.go` (new) -- as112 ProtocolID + RouteChange typed handle
- `internal/plugins/as112/eventbus.go` (new) -- atomic EventBus holder
- `internal/plugins/as112/redistribute.go` (new) -- source registration, covering prefixes, producer reconcile/withdraw/reemitAll/emit
- `internal/plugins/as112/register.go` -- registerAS112Sources() in init, ConfigureEventBus hook, producer wiring (applyConfig in OnConfigure, subscribeReplay, deferred withdraw)
- `internal/plugins/as112/server.go` -- listenerChanged aggregates anycast-listener serving state and drives prod.setServing (runtime withdraw on serving loss / re-announce on recovery)
- `internal/component/doctor/checks_as112_coordination.go` -- `checkAS112RedistributeOriginCoordination`, `redistributeImportsSourceIntoBGP`
- `internal/component/doctor/doctor.go` -- register the new check
- `internal/core/diagnostic/codes.go` -- `doctor-as112-redistribute-origin-uncoordinated`
- Tests: `internal/plugins/as112/redistribute_test.go` (config + producer reconcile + replay), `internal/component/bgp/redistribute/consumer_test.go` (origin/community wire), `internal/component/doctor/checks_as112_redistribute_test.go`
- Test producer + functional: `internal/test/plugins/fakeas112/` (new), `internal/test/plugins/all/all.go`, `test/plugin/redistribute-as112-*.ci`
- Interop/lab: `test/interop/scenarios/as112-redistribute-*` (FRR AS_PATH + community + DNS answer; Docker/CI)
- Docs: `docs/guide/as112.md` (two-paths), `docs/guide/{configuration,plugins}.md`, `docs/features.md`, `docs/architecture/core-design.md`, `docs/plugin-overview.md`
