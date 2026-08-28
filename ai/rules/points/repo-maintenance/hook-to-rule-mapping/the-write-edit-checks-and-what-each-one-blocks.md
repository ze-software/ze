---
kind: table
level:
stage:
---
| Check | Enforces | Triggers on | What it does |
|---|---|---|---|
| `writeLineCitation` | `evidence.md`, `writing.md` | governed prose | Blocks source line-number citations outside quoted code and generated files. BLOCKING. |
| `writeGenerated` | `repo-maintenance.md` | generated root instructions and tool-specific `.claude/` content | Blocks writes to generated root instructions and warns when shared agent guidance is written into a tool-specific tree. |
| `writeRenderedRule` | `repo-maintenance.md` | rendered files directly under `ai/rules/` | Blocks edits to rendered rules and directs the author to the canonical point source. BLOCKING. |
| `writePointOverwrite` | `never-destroy-work.md` | Write or replacement MultiEdit on an existing rule point | Blocks replacement of an existing canonical point file. BLOCKING. |
| `writePointLanguage` | `rule-format.md` | canonical directive points | Blocks lowercase obligation words and a new directive with no RFC 2119 level. BLOCKING. |
| `writeDesignEvidence` | `evidence.md` | design and spec files | Blocks a design or spec write until this session has read producing source or invoked LSP. BLOCKING. |
| `writeSpecStatus` | `planning.md` | source Go edits | Blocks implementation while the selected spec has the wrong lifecycle status. BLOCKING. |
| `writeGoPatterns` | `architecture.md`, `cli.md`, `go-standards.md`, `performance.md`, `quality.md`, `plugins.md`, `goroutine-lifecycle.md` | production Go writes and edits | Applies the native forbidden-pattern checks for handlers, panic, legacy logging, allocating formatting, nolint, init registration, switch dispatch, anonymous goroutines, and fake buffer handles. |
| `writeFilePatterns` | `architecture.md`, `commands.md`, `config.md`, `quality.md`, `testing.md` | writes and edits selected by path or file type | Applies native path, package-name, scratch, lint-exclusion, config-version, and CI observer checks. |
| `writeWeakening` | `testing.md` | tests and test-harness evidence | Runs the native weakening analyser and blocks unauthorised evidence changes. BLOCKING. |
| `writeCISleep` | `testing.md` | `test/**/*.ci` | Blocks `time.sleep(` without a recognised justification marker. BLOCKING. |
