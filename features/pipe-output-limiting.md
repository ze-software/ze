# Pipe Output Limiting

## Meta

| Field | Value |
|-------|-------|
| Name | Pipe Output Limiting |
| Kind | daemon |
| Scope | complete |
| Level | experimental |
| Components | internal/component/command/pipe.go |
| Real-path tests | test/plugin/test-pipe-first-last.ci, test/plugin/rib-pipe-filter.ci |
| Docs | docs/features/pipe-operators.generated.md |
| Doc review | 2026-10-07: every source anchor resolves (pipeFirst, pipeLast, applyFirst, applyLast, collectPipeMeta) and the JSON pipe metadata field is named pipe in internal/component/command/pipe.go |
| Defect review | 2026-10-07: audit found no open immediate spec against first and last; journal rows naming a Component, not each re-verified here: blanket-mechanism-hid-missing-cases.md:9, declared-format-contradicts-payload.md:25, declared-format-contradicts-payload.md:28, documentation-shows-config-the-parser-refuses.md:18, green-that-could-not-have-been-red.md:238, guard-message-teaches-the-violation.md:11, silent-fall-through.md:55 |
| Extra criteria | supported: each journal row naming a Component re-verified as fixed or not a defect = none yet |

## Description

`\| first N` and `\| last N` pipe operators bound output to the first or last N ROWS. They act on rows, so a command whose answer has none refuses them by name rather than truncating a document (client-side truncation otherwise). Commands that register them as pipe filters (e.g. RIB) get server-side early termination: `\| first N` stops the iterator at N, saving both iteration and serialization cost. `\| last N` keeps a trailing window. JSON output includes a `"pipe"` metadata dict recording which data-shaping modifiers were applied. <!-- source: internal/component/command/pipe.go -- pipeFirst, pipeLast, applyFirst, applyLast, collectPipeMeta -->
