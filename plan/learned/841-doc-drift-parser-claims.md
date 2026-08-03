# 841 -- doc-drift-parser-claims

## Context

The text parser architecture docs drifted after route-server parsing moved from token-slice wording to `textparse.Scanner`. The stale docs still described allocation through `strings.Fields`, while the current implementation scans token-by-token and only allocates returned result data structures.

## Decisions

- Kept the doc drift check narrow and table-driven instead of building a generic natural-language validator.
- Scoped forbidden claims to `docs/architecture/api/text-parser.md` so historical references elsewhere do not become false positives.
- Wired source-anchor validation into `make ze-doc-test` because source anchors are the mechanical link between architecture prose and code.
- Updated nearby API docs when they repeated stale text-format examples, not only the primary parser page.

## Consequences

- Future parser rewrites should update source-anchored wording and add or adjust forbidden-claim entries for replaced claims that are likely to regress.
- `make ze-doc-test` now covers doc drift, YANG command validation, and missing source-anchor paths in one user-facing gate.
- Source anchors must point at current files; speculative or removed implementation files should not remain in docs.

## Gotchas

- Stale parser claims can be semantically wrong while all source anchor paths still exist, so source-anchor validation is necessary but not sufficient.
- Forbidden-claim checks should use exact phrases from the retired wording. Broad terms such as `scanner` or `allocation` would create noisy failures.
- Regenerate `ai/CODE-TO-DOCS.md` after changing `<!-- source: ... -->` anchors.

## Files

- `docs/architecture/api/text-parser.md`
- `docs/architecture/api/text-format.md`
- `docs/architecture/api/text-coverage.md`
- `scripts/docvalid/doc_drift.go`
- `scripts/docvalid/scripts_test.go`
- `mk/inventory.mk`
- `docs/contributing/documentation-testing.md`
- `ai/rules/writing.md`
- `ai/CODE-TO-DOCS.md`
