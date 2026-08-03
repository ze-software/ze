# 1091 -- layout-4-protocol-skeleton

## Context

Each Ze protocol invented its own module layout (BGP `fsm`/`reactor`/`message`/
`wireu`, BFD `engine`/`packet`/`session`/`transport`, IKE `engine`/`wire`/
`crypto`, IS-IS `adjacency`/`circuit`/`lsdb`), so learning one protocol taught
nothing about the next -- the inverse of holo-routing's fixed per-protocol
skeleton. Child 4 (last) of the spec-layout umbrella: define the standard
skeleton, prove it fits the existing seven protocols, and report divergence
without gating.

## Decisions

- Skeleton = small required set for multi-package protocols (`packet`,
  `transport`, `yang`, an engine home) + per-peer state named by the protocol's
  OWN RFC term (`session`/`adjacency`/`neighbor`/`fsm`) + free domain modules
  (`lsdb`, `spf`, `crypto`, ...), over one flattened vocabulary: the RFC name is
  the discoverable one. BFD is the reference layout.
- Advisory-only, over an enforced gate: the probe shows 29 domain modules and 4
  legacy exceptions across 7 protocols; enforcement would repeat the tiers Path
  B allowlist trap. Report always exits 0; wired as the LAST line of
  `ze-tier-check` so it can never mask the enforcing commands before it.
- Standalone `protocol_skeleton_report.py` over reusing `dep_audit.py` (A-2
  broke benignly): the report needs `os.listdir` + a name classifier, not the
  import graph; zero shared logic, and still no second import parser.
- Protocols declared in an in-script manifest (7 rows) mirrored by the rule
  doc's probe table: "is this dir a protocol" is judgment, not mechanics.

## Consequences

- New protocols have a layout answer before their first subpackage; single-
  package protocols (LDP, RSVP-TE: root + `yang/`) are explicitly fine as-is.
- The skeleton's exceptions are exactly the two the naming glossary already
  documents (BGP historical vocabulary, `ike/wire`), so rule docs stay in sync.
- `ze-tier-check` output gained one advisory summary line per run.

## Gotchas

- A hand-maintained manifest goes stale silently: a moved protocol root
  initially rendered as "single-package" because `os.listdir` of a missing dir
  yielded an empty module list. Missing roots are now flagged in every output
  mode, with regression selftest cases. Any hand-declared path list needs an
  existence check.
- Selftest teeth were proven by mutation (deleting a LEGACY_EXCEPTIONS row made
  the selftest fail), the honest fail-first for a script whose tests ship in
  the same file.

## Files

- `ai/rules/protocol.md` (skeleton + probe mapping + exceptions)
- `scripts/dev/protocol_skeleton_report.py` (classifier, manifest, --selftest)
- `Makefile` (ze-tier-check advisory line), `ai/rules/INDEX.md` (row 78)
