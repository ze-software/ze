---
name: ze-review-docs
description: Use when working in the Ze repo and the user asks for ze-review-docs or for a documentation review. Build an inventory for the chosen docs scope, check anchors, links, examples, accuracy, and coverage, then report findings and wait for approval before fixing anything.
---

# Ze Review Docs

This skill reviews Ze documentation against the current codebase.

## Workflow

1. Determine the scope: whole `docs/`, a path, or an area such as guide, architecture, wire, API, config, or plugin docs.
2. Build a simple inventory with path, size, last doc update, and relevant source freshness.
3. Run mechanical checks for stale source anchors, broken internal links, invalid examples, and terminology mismatches.
4. Check factual claims against the real code and flag missing anchors for claims that are still correct.
5. Compare documented features to the current codebase and note coverage gaps.
6. Report issues by category: incorrect, stale, incomplete, missing anchors, broken links, and quality problems.

## Rules

- Do not edit docs before showing the report and getting approval.
- Every factual fix must come from current source, not memory.
