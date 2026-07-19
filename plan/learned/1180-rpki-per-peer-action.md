# 1180 -- RPKI Per-Peer Action

## Context

The RPKI validation action (what to do with Invalid / NotFound origin and ASPA Invalid / Unknown
routes) was global-only. The goal was to make it settable per-peer and per-group while keeping the
global option, renaming the config from `rpki { policy { invalid-action } }` to
`rpki { action { invalid } }`. During implementation two latent "config that does nothing" bugs
surfaced: origin `not-found-action` was parsed but never enforced (a global no-op), and the `role`
plugin's per-peer config keying was silently broken on real configs. Both were fixed.

## Decisions

- Per-leaf inheritance (peer > group > global, resolved at config time into a per-peer map keyed by
  remote IP) over container-level replace: a peer overriding one leaf keeps the global others.
- Hard rename cutover (old `policy` becomes a schema error) over dual-support: nothing shipped as
  stable config; old syntax now fails closed (`ze config validate` -> "unknown field in rpki: policy").
- Per-peer map stored in `atomic.Pointer[map[string]peerActionSet]` swapped on reload, over the
  `role` plugin's `sync.RWMutex`: `buildDecisions` reads from ONE goroutine (validationWorker), so a
  lock-free pointer swap suffices and keeps the hot path allocation- and lock-free.
- Key the per-peer map by `connection>remote>ip` via a new shared `configjson.PeerRemoteIP`, over
  copying `role`'s local reader: role read the stale flat `remote/ip` path (see Gotchas).
- Wired `not-found` enforcement (add atomic, store in startSessions, reject/log in buildDecisions)
  rather than leaving it inert -- user chose "fix it too" when the audit found it did nothing.
- Bundled the `role` remote-IP fix with a parallel session's uncommitted local-ASN refactor
  (`spec-fixit-local-asn-config-key`): the files were entangled (would not compile separately), and
  the user explicitly approved bundling after stopping that session.

## Consequences

- Any plugin that identifies peers by IP at runtime MUST key its config by
  `configjson.PeerRemoteIP(peerMap, groupMap)` (reads `connection>remote>ip`), NOT by the
  `ForEachPeer` map key (which is the peer NAME) and NOT by a flat `remote/ip` read.
- New bug class to hunt: "config that does nothing" -- a parsed config leaf with no enforcement path.
  A leaf that is read into a struct field but whose field is never consumed is dead policy. Two were
  found in this one area (origin not-found; role remote-IP keying). Grep new config fields for a
  non-test consumer, same as wiring-completeness for exported symbols.
- `show bgp rpki status` now reports `actions` (effective global) and `peer-actions` (per-peer
  resolved, with per-leaf source) from the SAME atomics/map enforcement uses -- display cannot drift.
- `watchdog` migration to the shared helper was dropped (uncontested, already correct); `role` and a
  future `watchdog` cleanup can still converge on `configjson.PeerRemoteIP`.

## Gotchas

- The config delivered to plugins keys peers by NAME with the address nested at
  `connection>remote>ip` (`Tree.ToMap` emits a keyed YANG list keyed by entry name;
  authradius/config.go documents this). `role.extractRemoteIP` read the flat `m["remote"]["ip"]`
  (pre connection-container reorg), returned "" on real config, and keyed role's map by NAME while
  the OTC filter looked up by IP -> miss -> RFC 9234 OTC config-role filtering silently disabled.
- A green test proves nothing if its assertion is vacuous. `role-otc-ingress-reject.ci` asserted only
  `adj-rib-in total-routes >= 0` (any non-error), not `== 0`, so it passed whether or not the leak was
  rejected. Read the assertion, not just the PASS. It now asserts `== 0` (verified 5/5 non-flaky).
- role's unit fixtures used `{"peer":{"10.0.0.1":{...}}}` (peer keyed by IP, no `connection` wrapper),
  which masked the bug because the code fell back to the map key. Realistic fixtures use
  `{"peer":{"<name>":{"connection":{"remote":{"ip":...}}}}}`.
- Shared worktree with several concurrent Claude sessions repeatedly churned and broke the build
  (branch count swung 35->8->12->18); `scripts/dev/verify-lock.sh` serializes `ze-verify` across
  sessions, so a verify can queue for many minutes behind another session's run.

## Files

- `internal/component/bgp/plugins/rpki/yang/ze-rpki.yang` -- rename + `validation-action` typedef + peer/group augments
- `internal/component/bgp/configjson/traverse.go` (+ `_test.go`) -- `PeerRemoteIP` helper
- `internal/component/bgp/plugins/rpki/rpki_config.go` (+ `_test.go`) -- renamed parse + `parsePeerActions`
- `internal/component/bgp/plugins/rpki/rpki.go` -- `originNotFoundAction`, `perPeerActions`, buildDecisions, statusCommand call
- `internal/component/bgp/plugins/rpki/rpki_status.go` (+ `_test.go`) -- actions/peer-actions serialization
- `internal/component/bgp/plugins/rpki/rpki_action_test.go` -- buildDecisions per-peer + NotFound tests
- `internal/component/bgp/plugins/role/config.go` (+ `_test.go`) -- `extractRemoteIP` -> `configjson.PeerRemoteIP` + RealShape guard
- `test/plugin/rpki-per-peer-action.ci`, `rpki-group-action.ci`; hardened `role-otc-ingress-reject.ci`; migrated `rpki-aspa-policy-*.ci`, `coverage-rpki.ci`, `43-rpki-frr/ze.conf`, `demos/terminal/rpki/ze.conf`
- `docs/guide/rpki.md`, `docs/config-reference.md`, `docs/features.md`
