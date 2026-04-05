# Doc Review

Review, analyze, and improve documentation for accuracy, completeness, and quality.

The user may optionally specify a scope: `/ze-review-docs [path|area|topic]`
- No argument: full inventory scan, then prioritized review
- Path argument: review that specific file or directory (e.g., `/ze-review-docs docs/guide/plugins.md`)
- Area argument: review a documentation area (e.g., `/ze-review-docs guide`, `/ze-review-docs architecture`, `/ze-review-docs wire`)
- `inventory` argument: produce the inventory and gap analysis only, no fixes

See also: `/ze-review` (code quality review), `/ze-review-deep` (exhaustive multi-agent review)

## Steps

### 1. Determine scope

Determine what documentation to review based on the argument:
- No arg or `inventory`: full `docs/` tree
- Path: that file or directory only
- Area keyword: map to directory using this table

| Keyword | Directory |
|---------|-----------|
| `guide` | `docs/guide/` |
| `architecture`, `arch` | `docs/architecture/` |
| `wire` | `docs/architecture/wire/` |
| `api` | `docs/architecture/api/` |
| `config` | `docs/architecture/config/` |
| `plugin`, `plugins` | `docs/plugin-development/` |
| `exabgp` | `docs/exabgp/` |
| `testing` | `docs/architecture/testing/`, `docs/functional-tests.md` |
| `meta` | `docs/architecture/meta/` |

### 2. Build inventory

For each file in scope, collect:

| Field | How |
|-------|-----|
| Path | File path relative to repo root |
| Lines | `wc -l` |
| Source anchors | Count of `<!-- source:` comments |
| Internal links | Count of markdown links to other repo files |
| Last modified | `git log -1 --format=%ci` for the file |
| Last code change | For architecture docs: `git log -1 --format=%ci` for the source files they describe |

Present as a table sorted by staleness (largest gap between doc modification and source code modification first).

### 3. Mechanical checks

Run these checks on every file in scope. These are automatable, binary pass/fail.

**3a. Source anchor validity**

For every `<!-- source: path -- symbol -->` comment in the file:
1. Does the file at `path` exist? (`ls` or `Glob`)
2. Does the symbol exist in that file? (`Grep` for the symbol name)
3. If either is missing: **STALE ANCHOR**

**3b. Internal link validity**

For every markdown link `[text](path)` or `[text](path#anchor)`:
1. Does the target file exist?
2. If `#anchor` is specified: does the heading exist in the target?
3. If either is missing: **BROKEN LINK**

**3c. Code example validity**

For every fenced code block tagged with a language (` ```go `, ` ```bash `, ` ```json `):
- `go`: check that referenced types, functions, and methods exist in the codebase (`Grep`)
- `bash`: check that referenced commands (`ze`, `make ze-*`) exist
- `json`: check that field names match `rules/json-format.md` conventions (kebab-case)

**3d. Terminology consistency**

Check against project naming rules:
- JSON keys must be kebab-case (flag camelCase or snake_case in JSON examples)
- "Ze" naming: `ze-bgp-conf`, `ze-bgp` (flag `exabgp` format names in ze docs)
- Address families must use `afi/safi` format

**3e. Cross-reference completeness (architecture docs only)**

For each architecture doc: does `.claude/INDEX.md` have an entry pointing to it?
For each INDEX.md entry: does the target file exist?

### 4. Accuracy checks

For each file in scope, verify factual claims against current code. This is the highest-value layer.

**4a. Source-anchored claims**

For every source anchor, read the referenced source file and compare the doc's claim to the actual code. Flag:
- Claim describes behavior the code no longer implements
- Claim uses old function/type/field names
- Claim describes a signature that has changed
- Claim shows config syntax the parser no longer accepts

**4b. Un-anchored claims**

For paragraphs making factual claims WITHOUT a source anchor:
1. Identify the claim (syntax, field names, data structures, behavior, data flow)
2. Search the codebase for the relevant code (`Grep`/`Glob`)
3. Compare claim to reality
4. If wrong: **INACCURATE CLAIM**
5. If correct but missing anchor: **MISSING ANCHOR**

**4c. Config syntax verification**

For any doc showing config syntax examples:
1. Read the actual parser (`internal/component/config/`)
2. Verify the syntax is accepted
3. Check field names match YANG schema

**4d. CLI verification**

For any doc showing CLI commands or flags:
1. Read the actual command handler (`cmd/ze/`)
2. Verify flags exist with documented names and types
3. Verify output format matches documented format

### 5. Completeness analysis

Compare documentation against the actual codebase to find gaps.

**5a. Feature coverage**

Run `make ze-inventory` (or `make ze-inventory-json` for structured data). For each item in the inventory:

| Inventory item | Expected documentation |
|----------------|----------------------|
| Each plugin | Entry in `docs/guide/` or `docs/plugin-development/` |
| Each address family | Mentioned in `docs/architecture/wire/nlri.md` or family-specific doc |
| Each RPC | Listed in `docs/architecture/api/commands.md` |
| Each CLI subcommand | Listed in `docs/guide/command-reference.md` |
| Each config root | Covered in `docs/guide/configuration.md` or `docs/architecture/config/syntax.md` |

Flag items present in inventory but absent from documentation as **UNDOCUMENTED FEATURE**.

**5b. Guide coverage**

For each file in `docs/guide/`:
- Does it have a "getting started" example?
- Does it reference the config syntax?
- Does it show realistic (not toy) examples?
- Does it explain prerequisites?

**5c. Architecture coverage**

For each file in `docs/architecture/`:
- Does it explain "why" (design rationale), not just "what" (description)?
- Does it have source anchors tying claims to code?
- Does it describe data flow through the component?

**5d. Route metadata coverage**

For each plugin that sets or reads route metadata:
- Does `docs/architecture/meta/README.md` list the keys?
- Does `docs/architecture/meta/<plugin>.md` exist?

### 6. Quality assessment

For each file in scope, assess readability and usefulness.

| Check | Question |
|-------|----------|
| Audience | Is it clear who this doc is for (user, developer, contributor)? |
| Entry point | Can the reader understand the first paragraph without reading other docs? |
| Ordering | Does it go from general to specific, common to rare? |
| Examples | Are examples realistic and copy-pasteable? |
| Completeness | Does it cover the topic fully, or trail off? |
| Redundancy | Does it duplicate content from another doc without adding value? |
| Navigation | Does it link to related docs for context? |
| Freshness | Does it reference features, syntax, or patterns that no longer exist? |

### 7. Report findings

Present findings in this format:

```
## Doc Review: [scope description]

**Files Reviewed:** [count] | **Source Anchors Checked:** [count] | **Links Checked:** [count]

### Critical (wrong information, will mislead readers)

| # | File | Line | Finding | Evidence | Fix |
|---|------|------|---------|----------|-----|
(sorted by impact)

### Stale (was correct, code has since changed)

| # | File | Line | Finding | Current code | Fix |
|---|------|------|---------|-------------|-----|

### Incomplete (missing documentation for existing features)

| # | Feature/Area | Expected in | What to document |
|---|-------------|-------------|-----------------|

### Missing Anchors (correct claims without source anchors)

| # | File | Line | Claim | Source file | Symbol |
|---|------|------|-------|-------------|--------|

### Broken Links

| # | File | Line | Link target | Status |
|---|------|------|------------|--------|

### Quality Issues

| # | File | Issue | Suggestion |
|---|------|-------|-----------|

### Summary

| Category | Count |
|----------|-------|
| Critical (wrong) | N |
| Stale | N |
| Incomplete | N |
| Missing anchors | N |
| Broken links | N |
| Quality | N |
| **Total** | **N** |
```

### 8. Fix mode (default unless `inventory` argument)

After presenting the report, ask the user which categories or specific findings to fix. When the user selects items to fix:

**For inaccurate/stale claims:**
1. Read the current source code
2. Rewrite the claim to match reality
3. Add or update the source anchor

**For missing anchors:**
1. Verify the claim is still correct
2. Add `<!-- source: path -- symbol -->` after the paragraph

**For broken links:**
1. Find the correct target (file may have been renamed/moved)
2. Update the link

**For incomplete coverage:**
1. Read the source code for the undocumented feature
2. Write documentation following the existing style of the target file
3. Add source anchors for every factual claim

**For quality issues:**
1. Apply the specific suggestion

After each fix, re-run the mechanical check for that file to confirm the fix is valid.

## Rules

- Do NOT fix anything before presenting the report and getting user approval.
- Read the actual source code before writing or correcting any documentation. Never describe code from memory. (`rules/documentation.md` Source Anchors)
- Every factual claim in a fix must have a source anchor.
- Preserve existing document structure and style when making fixes.
- Do not add features, examples, or sections beyond what was identified as missing.
- For large scopes (full `docs/`), use parallel Agent tool calls to review different areas simultaneously.
- When reviewing architecture docs, check `.claude/INDEX.md` entries point to the right file.

## Parallelization

For full-scope reviews, launch up to 4 parallel agents:

| Agent | Scope |
|-------|-------|
| Agent 1 | `docs/guide/` (user-facing guides) |
| Agent 2 | `docs/architecture/` excluding `wire/` and `api/` |
| Agent 3 | `docs/architecture/wire/` + `docs/architecture/api/` |
| Agent 4 | `docs/plugin-development/` + `docs/exabgp/` + root docs (`docs/*.md`) |

Each agent runs steps 3-6 for its scope. Results are merged in step 7.

## What This Skill Does NOT Do

- Does not review `.claude/rules/` (those are instructions for Claude, not user documentation)
- Does not review `plan/` specs or `plan/learned/` summaries
- Does not review `rfc/` (those are reference material, not project docs)
- Does not review code comments or `// Design:` annotations (covered by `/ze-review` and `/ze-review-deep`)
- For change-scoped doc accuracy (do docs match a specific diff?), use `/ze-review-deep docs` instead
