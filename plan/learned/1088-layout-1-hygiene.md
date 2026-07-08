# 1088 -- layout-1-hygiene

## Context

Child 1 of the spec-layout umbrella: repo-root clutter (`screenlog.0`, `qos-map.md`,
`AI-NAVIGATION-AUDIT.md`, `test-web`, dead `parked/`) plus a stale "OSPF and IS-IS are
not implemented" Non-Goals claim in `docs/architecture/overview.md` (contradicted by
`internal/plugins/{ospf,isis}`). Goal: clean the root and correct the doc without
touching runtime behavior.

## Decisions

- Relocate rather than delete the still-referenced-by-history files: `qos-map.md` ->
  `docs/research/`, `AI-NAVIGATION-AUDIT.md` -> `plan/audits/`, `test-web` ->
  `scripts/dev/`. Delete `screenlog.0` (+ gitignore `screenlog.*`) and `parked/` (dead
  cbor/reader code, no importers), dropping `./parked/...` from the golangci-lint line.
- Keep `prod.json` at root unchanged (owner decision): it is live appliance-build input
  and its address is private RFC1918, not a routable secret.
- Correct the stale claim in BOTH `overview.md` and `docs/research/bgp-implementations-analysis.md`
  with source anchors (OSPF 245 / IS-IS 94 non-test `.go` files), over deleting the claim
  silently, so the correction is traceable.

## Consequences

- The spec-layout umbrella's remaining children (2 core-import-gate, 3 naming-glossary,
  4 protocol-skeleton) are independent of this and unblocked.
- New rule for file-move specs: a listed "referrer" is not necessarily a reference to the
  file (see Gotchas) -- verify before rewriting.

## Gotchas

- Broken assumption A-1 (the highest-value lesson): the umbrella listed many "referrers"
  for `qos-map.md`/`test-web` to rewrite on move. The audit found ZERO real referrers --
  every grep hit was the `qos-map` config keyword (`ingress-qos-map`/`egress-qos-map`,
  parsed in `internal/component/iface/config.go`) or the learned-summary filenames
  `882-vlan-qos-map.md` / `868-test-web-parallel.md`. Editing those "referrers" as the
  spec directed would have corrupted live config keywords and broken the vlan-qos tests.
  Lesson: when a spec lists referrers for a file move, disambiguate keyword-vs-filename
  (and doc-filename collisions) before trusting the list; grep for the literal path
  (`qos-map\.md`), not the bare token.
- The overview correction's changelog comment ("previously claimed ... not implemented")
  itself tripped the AC-2 grep for the stale phrase; dropped it, kept only the source anchor.

## Files

- `.gitignore`, `Makefile` (drop `./parked/...`)
- `docs/architecture/overview.md`, `docs/research/bgp-implementations-analysis.md`
- relocations: `qos-map.md` -> `docs/research/`, `AI-NAVIGATION-AUDIT.md` -> `plan/audits/`,
  `test-web` -> `scripts/dev/`
- deletions: `screenlog.0`, `parked/` (8 files)
