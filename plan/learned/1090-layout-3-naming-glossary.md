# 1090 -- layout-3-naming-glossary

## Context

Ze's 610 packages use overlapping module vocabulary: every protocol names its
codec/runtime/IO layers differently (`packet` vs `message` vs `wire`; `engine`
vs `reactor`), three sibling packages are named `cli`/`cmd`/`command`, and four
packages carry rib-flavoured names. Child 3 of the spec-layout umbrella added a
package-naming glossary so new packages pick the established term instead of
coining synonyms, and settled the one actively opaque name (`wireu`).

## Decisions

- Glossary lives as a section in `ai/rules/go-standards.md`, over a new rule file:
  one naming doc, discoverable via the existing rules-index row and
  `ai/INDEX.md` plugin row.
- Definitions are DESCRIPTIVE, quoted from `ai/PACKAGE-MAP.md` (generated from
  doc comments), over prescriptive wishes: the glossary documents what the
  terms already mean; only NEW packages are constrained (`packet` and `engine`
  are the preferred terms for new protocol codecs/runtimes; `reactor` and
  `message` are marked BGP-historical).
- `wireu` KEPT, not renamed (user decision): a rename to `wireupdate` would
  rewrite 47 importer files inside the BGP trees the rib-arch spec set is
  concurrently reworking. The expansion ("wire UPDATE") and the decision now
  live in the package's own doc.go, the place a confused reader looks first.
- Rename shortlist closed EMPTY: the other short names (`bgp/plugins/{gr,rs,
  rr,llnh,capa}`) are established BGP jargon local to their namespace.

## Consequences

- New protocol packages have a canonical vocabulary; glossary terms match
  existing doc comments, so no migration pressure exists.
- Changing a `// Package` doc comment invalidates `ai/PACKAGE-MAP.md`; any
  doc.go wording change must run `make ze-discovery-index` or `ze-doc-test`
  fails (bit this spec; 1-line regen).
- The `wireu` rename question is settled with a written rationale; reopening it
  after rib-arch lands is a conscious new decision, not a rediscovery.

## Gotchas

- The spec's Current Behavior claimed `go-standards.md` had forbidden-names rules
  (`utils`/`helpers`); the actual file was 16 lines with none. Same lesson as
  layout-1's A-1: verify a spec's claims about a file against the file before
  building on them.
- `core/routingtable` vs `plugins/routingtable` LOOK like duplicates in the
  package map; `plugins/routingtable/registry.go` shows the plugin wraps
  and re-exports the core package deliberately (single import path for
  consumers). The glossary documents this so nobody "deduplicates" them.

## Files

- `ai/rules/go-standards.md` (Package-Naming Glossary section)
- `internal/component/bgp/wireu/doc.go` (name expansion + keep rationale)
- `ai/PACKAGE-MAP.md` (regenerated: wireu row follows the new doc comment)
