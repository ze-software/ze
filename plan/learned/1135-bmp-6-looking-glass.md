# 1135 -- BMP receiver looking glass integration

## Context

Operators running ze as a BMP receiver (route collector) had no way to browse
collected routes. The BMP receiver parsed Route Monitoring messages but
discarded the BGP UPDATE payload after logging. The looking glass only showed
peers from real BGP sessions. This spec added BMP route storage, display, and
LG integration while keeping BMP routes completely isolated from best-path
selection and the FIB.

## Decisions

- **ProtocolID separation over StaleLevel**: BMP routes stored under
  `bmpProtocolID` in the two-level `ribInPool` map (added in spec-rib-4).
  `gatherCandidatesLocked` iterates only `bgpPeers`, so BMP routes are
  automatically excluded from best-path with zero filter code. Using
  StaleLevel would have conflicted with GR/LLGR staleness semantics.

- **Global RegisterRouteInjector over PluginServerAccessor extension**: the
  RIB plugin registers its inject handler via a package-level atomic in the
  `rpc` package. Simpler than adding methods to the PluginServerAccessor
  interface, avoids import cycles, same thread-safety via `atomic.Value`.

- **Composite peer key `<router>:<peer-address>`**: encodes the BMP router
  identity into the peer key. Multiple BMP routers reporting the same
  downstream peer get separate entries. Colon separator is unambiguous
  (router address includes port, peer address is bare IP).

- **Separate LG endpoints**: `/api/looking-glass/protocols/bmp` and
  `/routes/bmp/{name}` are distinct from the BGP endpoints. BMP peer
  metadata comes from `bmp peers` (richer than BGP summary), and routes
  come from `bgp rib show-protocol bmp` (reusing the RIB pipeline).

## What Worked

- The two-level `ribInPool` map from spec-rib-4-extraction was designed
  exactly for this use case. Adding BMP storage required only registering
  `bmpProtocolID` and initializing an inner map.

- DirectBridge typed handler (InjectWireRoute) follows the ForwardCached
  precedent exactly: type + Set + Has + call, atomic bool guard.

- Restricting `newInboundSource` to `bgpPeers` was a one-line change that
  cleanly separated BGP and BMP display paths.

## Mistakes

- Initial `processRouteMonitoring` lacked a nil guard on `bp.plugin`,
  causing test panics. BMP session tests create `BMPPlugin{}` with nil
  plugin. Fixed by checking nil before calling InjectWireRoute.

- LG BMP routes handler initially used `prefix` keyword to filter by peer
  name, which filtered by network prefix instead. Fixed by adding peer
  selector support to `show-protocol` command.

## Patterns

- **Cross-plugin typed handler via global registration**: when two internal
  plugins need a zero-copy data path and the Server struct cannot hold
  a direct reference, use a package-level atomic registration in the shared
  `rpc` package. The producer registers at startup, consumers read at call
  time. Thread-safe, no import cycles, no interface changes.

## Files

None recorded.
