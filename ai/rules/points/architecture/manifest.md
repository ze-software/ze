---
title: Architecture and Design
when: before any design decision, before writing code or a spec, or when deciding where a new package belongs
severity: blocking
related: plugins, performance, protocol, completion, go-standards
---
directives ## Directives
  load-ze-context-before-any-design-decision
  where-the-architecture-rationale-lives
before-writing-code ## Before Writing Code
  the-checklist-to-complete-before-writing-code
  before-any-buffer-pool-allocation-trace-the-full-lifecycle
  grep-every-sibling-call-site-in-the-same-commit
  what-to-check-for-each-sibling-audit-trigger
  fix-each-caller-or-say-why-it-is-exempt
design-context ## Design Context
  the-tier-1-reading-and-what-it-prevents
  the-tier-2-reading-for-each-artifact
  the-tier-3-reading-for-each-area
  reuse-the-existing-pattern-before-adding-one
  the-questions-to-answer-before-proposing-a-design
design-principles ## Design Principles
  apply-these-design-principles-to-every-decision
  check-abstraction-coupling-and-testability
module-tiers-core-component-plugin ## Module Tiers (core / component / plugin)
  the-normative-placement-rule
  where-each-category-of-package-belongs
  pick-the-directory-from-these-rules
  put-a-sub-plugin-under-its-subsystem-namespace
  record-non-engine-placements-in-the-manifest-file
  the-manifest-classifies-it-does-not-allow
  never-import-a-gated-feature-from-always-on-code
data-flow-tracing ## Data Flow Tracing
  these-subsection-names-are-required-in-a-spec
  name-where-data-enters-and-in-what-format
  trace-each-transformation-stage
  name-every-boundary-crossed-and-check-for-violations
  check-that-the-integration-points-already-exist
  the-questions-a-spec-must-answer-about-data-flow
impact-analysis ## Impact Analysis
  what-a-yang-change-also-touches
  what-a-registration-change-also-touches
  what-a-go-change-also-touches
  what-a-ci-test-change-also-touches
  check-code-to-docs-when-you-change-code
  what-a-docs-change-also-touches
  what-a-spec-change-also-touches
zefs-persistence-no-loose-state-files ## zefs Persistence (no loose state files)
  use-statestore-put-and-get-never-os-writefile
  register-the-key-and-share-the-one-store-handle
  the-raw-writes-that-stay-on-the-allowlist
  the-gate-that-flags-a-raw-filesystem-write
server-rendered-markup ## Server-Rendered Markup
  keep-markup-in-templ-and-out-of-go
architecture-summary ## Architecture Summary
  the-same-bytes-parse-differently-per-capability-set
  every-type-implements-bufwriter
  wireupdate-transports-the-rib-stores
  the-peer-pools-and-the-global-shared-pool
  the-chaos-event-buffer-never-drops-an-event
related ## Related
  where-to-read-more-about-placement
