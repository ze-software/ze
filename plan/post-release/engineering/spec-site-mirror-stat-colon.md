# Spec: site-mirror-stat-colon

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Priority | P3 |
| Release reason | Published Markdown mirrors of the website carry a stray colon after every stat figure; no shipped binary changes |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Design tier | development tier |
| Implementation tier | development tier |
| Updated | 2026-11-12 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**Symptom (2026-11-12).** The Markdown mirror of `/project/activity/` publishes
every stat card as `- Go tests:** 1,870,630:**`, with a colon after the
figure. The mirror published on 2026-11-11, before the size-history work,
already read `- Total added lines:** 5,775,449:**`.

**Cause, read at the producer.** `renderComponent`
(`internal/le/site/mirror.go`) renders a `stat`, `status-row` or
`quality-meter-row` element as a bullet and passes its text through
`mirrorLabelValue`, which is meant to put one colon between the label and the
bold value that follows it. The pattern matches any non-space character
followed by a bold marker, so it also matches the end of the value before its
closing marker, and `labelValueColon` writes a second colon there.

**Why no test saw it.** `TestTheActivityMirrorReadsAsThePublishedMirror`
compares the mirror's shape through `activityMirrorShape`
(`internal/le/site/activity_test.go`), which keeps each bullet's label only.
The fixture `internal/le/site/testdata/published-activity.md` shows the
figures without the colon, a form the converter never writes.

**Goal.** A stat card publishes as `- Label: **value**` with one colon, on
every page whose mirror carries such a card, and a test fails if the colon
after the value comes back.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/le-tooling.md` - the doors this work crosses
  → Constraint: the converter serves every page of the site, so a change to it is judged against every mirror fixture, not one page's
- [ ] `website/AI.md` - what the mirrors are for and which pages publish one

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/site/mirror.go` - `renderComponent`, `mirrorLabelValue`, `labelValueColon`
- [ ] `internal/le/site/activity_test.go` - `activityMirrorShape`

**Behavior to preserve:**
- [to research]

**Behavior to change:**
- The colon is written at the label's boundary only

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Label and value separation | `internal/le/site/mirror.go` `mirrorLabelValue`, `labelValueColon` | Puts a colon and a space before every bold marker that follows a non-space character | Extends: [to research] |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A page's main element, converted by `htmlToMarkdown` during the site build

### Transformation Path
1. [to research]

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Page HTML ↔ Markdown mirror | [to research] | No |

### Integration Points
- [to research]

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| The site build's mirror of a page with a stat card | → | `renderComponent` | [to design] |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| [to design] | `internal/le/site/` | A stat card's mirror line holds one colon, after the label | |

## Files to Modify
- `internal/le/site/mirror.go` - [to design]

## Implementation Steps

1. **Phase: Red test over a stat card's mirror line** -- [to design]
2. **Phase: The colon at the label's boundary only** -- [to design]

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Targeted runs: `./le test unit pkg path ./internal/le/site/`

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `fixture-encodes-an-impossible-state` | 2026-11-12 | Site mirrors publish a stray colon after every stat figure; the activity mirror fixture hides it |
