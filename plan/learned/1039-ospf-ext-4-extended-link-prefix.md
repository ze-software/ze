# 1039 - OSPFv2 Extended Prefix/Link Opaque LSAs (RFC 7684)

## Context

Opaque Type 7 (Extended Prefix) + Type 8 (Extended Link) LSAs as consumers of the
ext-1 opaque carrier, following the ext-2/ext-3 template. TLV containers only: the
Extended Prefix TLV (route type / prefix / A+N flags) and the single Extended Link
TLV (link type / id / data mirroring the Router-LSA link), each nesting sub-TLVs.
This spec delivers the empty containers + a sub-TLV registration hook; it defines
NO SID/label/SRGB. It is the carrier foundation ext-5 (Segment Routing) attaches
Prefix-SID / Adjacency-SID sub-TLVs to.

## Decisions

- Two UNEXPORTED sub-TLV registration hooks for ext-5/SR: `registerPrefixSubTLV(subType uint16, codec extSubTLVCodec)` and `registerLinkSubTLV(...)`, where `extSubTLVCodec{Build,Receive,Render}` carries origination/reception/show callbacks. Origination passes an `extSubTLVContext` (prefix+routeType for the prefix registry; linktype/id/data for the link registry). Type 0 + duplicates rejected; callbacks panic-recovered.
- Route Type derived from the source LSA (stub->intra, self summary->inter, self external->AS-external); Extended Link TLV fields copied verbatim from the matching decoded Router-LSA link.
- Extended Link Opaque LSA carries exactly one Extended Link TLV (RFC 7684 §3.1 SHALL); decode uses the first and counts extras.
- Cross-LSA dedup keeps the STRICTLY-lower Opaque ID (RFC 7684 §2); an equal ID is a refresh and overwrites.

## Consequences

- ext-5 attaches Prefix-SID (prefix registry) and Adjacency-SID (link registry) codecs via the two hooks, plus SRGB/SR-Algorithm via ext-3's registerRITLV; it does not re-open the ext-4 containers.
- Malformed bodies never crash (bound-checked opaqueTLVIterator + fixed-header guards); fuzz targets FuzzOSPFExtPrefixBody/FuzzOSPFExtLinkBody.
- §5 Type-11 reachability is recorded at receive time (`usable = scope != AS || reachable`).

## Gotchas

- The cross-LSA dedup used `cur.opaqueID <= opaqueID` (review ISSUE): the `<=` dropped a same-Opaque-ID REFRESH (same LSA, higher sequence, delivered on a newer install), so updated flags/routeType/usable were silently discarded (e.g. an A-Flag transition or a Type-11 unusable->usable flip never reflected). Must be `<` (strictly-lower wins; equal falls through to overwrite). Regression test TestExtPrefixSameOpaqueIDRefreshUpdates.
- The link consumer keeps NO separate resolved-link store; its gauge is recomputed in refreshExtMetrics from the opaque store, so the withdraw delivery path must call refreshExtMetrics() (it returned early before) or the ze_ospf_ext_link_lsas gauge stays transiently stale.
- Render-codec panics are recovered but not counted (the render chain is free functions with no engine access); accepted NOTE (show path only). Non-IPv4-AF prefix TLVs can over-reject the whole LSA (no live impact; only AF 0 defined); noted for the ext-5 AF path.
- §5 "not stored/acked/reflooded" is consumer-level here: ext-1 stores+refloods opaque bytes for any type; the consumer detects malformed bodies, counts, applies nothing (the MUST "never crash" is met).

## Files

- `internal/plugins/ospf/{ext,ext_prefix,ext_link,ext_subtlv,ext_render}.go` (+ tests), `packet/{ext_prefix,ext_link}.go` (+ tests, fuzz)
- `internal/plugins/ospf/{instance,register,config}.go`, `packet/json.go`, `yang/ze-ospf-conf.yang`
- `test/ospf/ospf-ext-*.ci`, `test/interop/scenarios/ospf-ext-prefix-link-frr/`
- `rfc/short/rfc7684.md`, `docs/guide/ospf.md`, `docs/features.md`, `docs/guide/configuration.md`
