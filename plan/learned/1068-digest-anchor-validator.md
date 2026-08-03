# 1068 -- Digest Anchor Validator and Subsystem Digest Expansion

## Context

The `ai/digests/` subsystem flow digests introduced in [[1067-generated-discovery-indexes]]
are hand-maintained (not generated), so their `file:line` anchors rot silently when code
moves: nothing checked them. There were also only 6 digests (BGP core), leaving most of the
network OS (OSPF, IS-IS, MPLS, FIB, VPP, iface, firewall, IKE, subscriber, DNS, API, web,
MCP, hub, telemetry, DDoS, AAA) with no fast-orientation layer. Goal: make the checkable part
of a digest mechanical (so anchors cannot go stale unnoticed) and expand coverage to every
major subsystem.

## Decisions

- Added `scripts/dev/digest_check.py`: validates every backtick `file:line` anchor in
  `ai/digests/*.md`. Strict for lined anchors (must resolve to exactly one file, line in
  range); full repo-relative links (`docs/...`, `plan/...`) must exist; bare no-line mentions
  (`register.go`, `176-topic.md`) are treated as informal and skipped. This mirrors the
  guarantee the per-file `// Design:` targets already get.
- Anchors are subsystem-relative (`peer.go`, not the full path), so each digest declares
  its subtree(s) with a machine-readable `<!-- digest-base: dir1 dir2 -->` header. Resolution
  tries bases in declared order and the first base that holds the file wins; a basename that
  is ambiguous within one base must be qualified (`storage/familyrib.go`). Chose this over
  rewriting every anchor to a full path so the digests stay readable.
- Wired the gate where doc checks run: `make ze-digest-check` folded into `make ze-doc-test`,
  and `verify_wiring_docs.py` selects it when a digest OR a `.go` under a declared base changes
  (a code edit can shift the cited line). Enforcement is the pre-commit developer loop plus
  the changed-file router; there is still no CI (see [[1067-generated-discovery-indexes]]).
- Deduplicated the discovery-index source predicate: `feeds_discovery_index` (commit_helper)
  and `is_discovery_source` (verify_wiring_docs) carried identical path rules with a
  "keep in sync" comment; both now call `scripts/dev/discovery_sources.py`, each supplying its
  own file text (HEAD-only for the commit gate, working-tree plus HEAD for the router).
- Expanded from 6 to 23 digests: 17 new subsystem digests written by tracing real code, each
  anchor opened and verified, each validated against `ze-digest-check` before landing.

## Consequences

- A digest that references a deleted file or an out-of-range line now fails `ze-doc-test`, so
  the anchors cannot rot silently. Prose truth is still not mechanically checkable, so the
  digest and `ai/digests/README.md` both say to verify `file:line` before relying on a detail.
- The 17 traces each surfaced real doc-vs-code drift or unwired features, recorded in each
  digest's gotchas. Notable ones worth follow-up: forked/multi-process deployment silently
  drops OSPF/IS-IS routes from the kernel (nil Loc-RIB singleton); the managed-config server
  side has zero production call sites; the IKE responder path and rekey wire messages are not
  implemented; the DDoS FlowSpec responder announce/withdraw are stubs; RADIUS is a no-op
  admin-auth backend; several `admin-distance` YANG leaves are dead.
- `ai/INDEX.md` front door, `ai/rules/repo-maintenance.md`, and
  `docs/contributing/documentation-testing.md` all point at the digest layer and its gate.

## Gotchas

- The resolver fails closed on cross-base ambiguity: a bare basename that exists under more
  than one declared base is reported as ambiguous (it does NOT pick the first base, which
  could silently validate against the wrong same-named file). Qualify with enough path or a
  full repo-relative anchor; confirm with `digest_check.py --list`. An adversarial review
  caught the earlier first-base-wins behavior validating `config-pipeline.md`'s bare
  `loader.go` against `bgp/config/loader.go` (LoadReactorFile) instead of the intended
  `config/loader.go` (PruneInactive).
- Comma-joined line lists in one backtick (`` `x.go,49` ``, `` `x.go,20-24` ``) expand
  to one validated anchor per element. A bare `:14,49` continuation (no filename) is still
  skipped, since it cannot be tied to a file mechanically.
- Character-only edits to a digest (for example the no-em-dash normalization) do not change
  line numbers, so anchors stay valid; adding or removing digest lines never invalidates
  anchors either (they point at the referenced .go file, not positions in the digest).

## Files

- `scripts/dev/digest_check.py` (+ `digest_check_test.py`)
- `scripts/dev/discovery_sources.py` (+ `discovery_sources_test.py`); `commit_helper.py`,
  `verify_wiring_docs.py` now import it
- `mk/inventory.mk` (`ze-digest-check`, folded into `ze-doc-test`)
- `ai/digests/*.md` (6 existing gained `digest-base` headers; 17 new digests; `README.md` index)
- `ai/INDEX.md`, `ai/rules/repo-maintenance.md`, `docs/contributing/documentation-testing.md`
