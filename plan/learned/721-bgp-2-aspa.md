# 721 -- ASPA Path Verification

## Context

Ze's RPKI plugin validated route origins via ROA (RFC 6811) but had no path verification. ASPA (draft-ietf-sidrops-aspa-verification) extends this by checking that each hop in the AS_PATH is authorized by the customer AS's provider set, detecting route leaks and path manipulation. This required RTR v2 (RFC 9582) for ASPA PDU distribution, a new verification algorithm, and route tracking for re-validation when ASPA data changes.

## Decisions

- Chose per-session RTR version field over package-level const, because v2 negotiation requires fallback to v1 per-session and the old const prevented concurrent sessions at different versions
- Chose ASPA state as informational-only (no accept/reject) over policy-integrated, because policy actions are a separate concern and keeping them out of scope avoids coupling ASPA to adj-rib-in's accept/reject dispatch
- Chose route tracker inside the RPKI plugin over adj-rib-in re-validation, because ASPA re-validation on cache change must not create cross-plugin dependencies (plugin owns its own state)
- Chose flat `map[uint32]map[uint32]struct{}` for ASPA cache over AFI-dimensioned storage, because RFC 9582 MAY allows ignoring AFI and the simpler structure is sufficient for initial deployment
- Chose `atomic.Bool` for `aspaEnabled` over plain bool, because the RTR session goroutine calls `handleASPAChange` which reads the flag concurrently with the SDK event callback goroutine

## Consequences

- RTR sessions now start at v2 and fall back to v1; any future RTR v2 feature (beyond ASPA) can be added without restructuring version handling
- ASPA verification runs on every UPDATE in the structured (DirectBridge) path and on the JSON fallback path; the JSON path cannot detect AS_SET segments (flat `[]uint32` without segment types)
- Route tracker grows with the number of active routes (bounded at 1M); memory-constrained deployments should monitor tracker size
- Future ASPA policy actions (reject Invalid, prefer Valid) need a separate spec that reads the `aspa-state` field from rpki events
- `ROACache.totalLocked()` was O(N), making `ApplyDelta` O(N^2) for 874K VRPs (~13 minutes under lock). Replaced with O(1) running counter. Any future cache methods that add/remove entries must maintain `c.total`

## Gotchas

- `ROACache.ApplyDelta` held the write lock during `totalLocked()` which iterated all map entries per VRP. With 874K real-world VRPs, `cache.Count()` blocked for minutes waiting for the lock. The live test against stayrtr exposed this only after the Docker image was fixed
- stayrtr v0.6.4 does NOT parse `provider_authorizations` from RPKI JSON, so the live ASPA test correctly skips. A future stayrtr version may add support
- Docker image moved from `ghcr.io/bgp/stayrtr` (auth denied) to `docker.io/rpki/stayrtr`
- stayrtr's `-checktime` flag rejects JSON with timestamps older than 24 hours; synthetic test data must use `-checktime=false`
- On macOS (Docker Desktop), `--network host` does not expose host localhost to containers; must use `host.docker.internal` and `-p` port mapping instead

## Files

- `internal/component/bgp/plugins/rpki/aspa_cache.go` -- ASPA record storage
- `internal/component/bgp/plugins/rpki/aspa_verify.go` -- upstream path verification algorithm
- `internal/component/bgp/plugins/rpki/aspa_tracker.go` -- route tracker for re-validation
- `internal/component/bgp/plugins/rpki/rtr_pdu.go` -- ASPA PDU type 11 parser, v2 constants
- `internal/component/bgp/plugins/rpki/rtr_session.go` -- v2 negotiation, ASPA accumulation
- `internal/component/bgp/plugins/rpki/rpki.go` -- plugin integration, tracking, withdrawals
- `internal/component/bgp/plugins/rpki/emit.go` -- aspa-state in event JSON
- `internal/component/bgp/plugins/rpki/rpki_config.go` -- aspa-validation config
- `internal/component/bgp/plugins/rpki/roa_cache.go` -- O(1) totalLocked fix
- `cmd/ze-test/rtr_mock.go` -- --aspa flag for mock RTR v2 server
- `test/plugin/rpki-aspa-*.ci` -- 4 functional tests
