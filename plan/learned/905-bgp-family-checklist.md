# 905: BGP Family Integration Checklist

Spec: post-mortem analysis of SR-Policy, PATHS-LIMIT, and SRv6 Prefix-SID

## Context

Three BGP features (SR-Policy SAFI 73, PATHS-LIMIT capability 76, SRv6 Prefix-SID
attribute 40) each required fix-up commits after the initial implementation. SR-Policy
took 3 commits; PATHS-LIMIT needed separate interop and functional test commits. The
fixes were not edge cases but predictable integration points: JSON serialization,
CLI decode registration, exhaustive switch cases, snapshot test golden lists, and
config surface guards.

The existing rules (wiring-completeness, discovery-updates) were generic. They said
"wire everything" but didn't enumerate the family-specific touchpoints. An agent
following "wire every exported symbol" would still miss that `all_test.go` has a
golden plugin-name list, or that `mpnlri.go` has an exhaustive switch on SAFI.

## Decisions

**Dedicated pattern file over extending the plugin pattern.** The existing
`ai/patterns/plugin.md` had a 7-line NLRI codec section. BGP family integration
touches 12+ distinct areas across the codebase. Extending the plugin pattern would
bury family-specific items in a generic document. A separate `ai/patterns/bgp-family.md`
makes the checklist findable and complete.

**BLOCKING gate in /ze-spec and /ze-implement over advisory guidance.** Advisory
patterns get read and forgotten. The gate in /ze-spec Step 2 (RESEARCH) forces the
pattern to be read before any design work. The gate in /ze-implement Step 3 (AUDIT)
catches specs that predate the pattern. Both are triggered by scope detection (new
SAFI/capability/attribute keywords), not manual opt-in.

**Checklist embedded in spec template over external reference.** The spec template's
"BGP Family Checklist" section copies the integration points into the spec itself,
making them visible during implementation and reviewable during /ze-review. An
external reference would require the agent to remember to consult it.

## Consequences

- New BGP family specs must fill a 18-row integration checklist before implementation.
- /ze-implement refuses to start if the spec involves a BGP extension but lacks the checklist.
- `ai/INDEX.md` now routes NLRI/capability/attribute tasks to `bgp-family.md` (BLOCKING).
- The pattern file documents anti-patterns from real incidents with commit hashes.

## Gotchas

- The SR-Policy `decodeNLRIOnly` bug was a latent defect exposed by the new plugin,
  not caused by it. Cross-plugin impact assessment (section 12 of the checklist) is
  the only way to catch this class of issue.
- Exhaustive switch violations are caught by `golangci-lint` but only if you run
  `make ze-lint-changed`. The checklist includes a proactive grep as defense in depth.

## Files

- `ai/patterns/bgp-family.md` (new) -- 12-section exhaustive checklist
- `ai/skills/ze-spec.md` -- BGP Family Gate added to Step 2 (RESEARCH)
- `ai/skills/ze-implement.md` -- BGP Family Checklist gate added to Step 3 (AUDIT)
- `plan/TEMPLATE.md` -- BGP Family Checklist section added to Integration Checklist
- `ai/INDEX.md` -- NLRI/capability/attribute rows updated to point to bgp-family.md
