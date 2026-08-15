---
title: Architecture and Design
when: before any design decision, before writing code or a spec, or when deciding where a new package belongs
severity: blocking
related: plugins, performance, protocol, completion, go-standards
---
directives ## Directives
  where-the-architecture-rationale-lives
  load-ze-context-before-any-design-decision
before-writing-code ## Before Writing Code
  complete-before-writing-any-code-tests-or-documentation
  the-checklist-to-complete-before-writing-code
  memory-lifecycle-tracing
  before-any-buffer-pool-allocation-trace-the-full-lifecycle
  sibling-call-site-audit
  grep-every-sibling-call-site-in-the-same-commit
  what-to-check-for-each-sibling-audit-trigger
  example-grep-for-every-call-site
  fix-each-caller-or-say-why-it-is-exempt
design-context ## Design Context
  the-context-to-load-before-a-design-decision
  tier-1-always-read-before-any-design
  the-tier-1-reading-and-what-it-prevents
  tier-2-when-designing-a-specific-artifact
  the-tier-2-reading-for-each-artifact
  tier-3-when-the-design-touches-these-areas
  the-tier-3-reading-for-each-area
  bgp-domain-facts-do-not-assume-from-training-data
  the-bgp-facts-that-training-data-gets-wrong
  anti-patterns
  reuse-the-existing-pattern-before-adding-one
  mechanical-check
  the-questions-to-answer-before-proposing-a-design
design-principles ## Design Principles
  apply-these-design-principles-to-every-decision
  scalability-checklist
  check-abstraction-coupling-and-testability
module-tiers-core-component-plugin ## Module Tiers (core / component / plugin)
  placement-generalizes-the-delete-the-folder-test
  the-tiers
  what-each-tier-is-and-where-it-lives
  the-placement-axes
  the-mechanical-test-for-each-axis
  decision
  where-each-category-of-package-belongs
  non-engine-category-manifest
  record-non-engine-placements-in-the-manifest-file
  what-a-manifest-row-holds
  the-manifest-row-format
  allowed-categories
  what-each-category-means-and-where-it-lives
  the-manifest-classifies-it-does-not-allow
  authoring-rule-read-before-creating-a-package
  decide-the-tier-before-you-pick-a-directory
  pick-the-directory-from-these-rules
  put-a-sub-plugin-under-its-subsystem-namespace
  scope-of-enforcement
  engines-are-mechanical-non-engines-need-a-row
  the-normative-placement-rule
  the-wired-as-a-plugin-signal-is-mechanical
  what-dep-audit-check-enforces
  how-dep-audit-classifies-and-when-it-fails
  disable-ability-compile-out
  axis-b-decides-what-can-be-compiled-out
  listener-services-and-dedicated-seams
  never-import-a-gated-feature-from-always-on-code
  migration-baseline-transitional-not-an-allowlist
  the-migration-baseline-can-only-shrink
data-flow-tracing ## Data Flow Tracing
  these-subsection-names-are-required-in-a-spec
  entry-point
  name-where-data-enters-and-in-what-format
  transformation-path
  trace-each-transformation-stage
  boundaries-crossed
  name-every-boundary-crossed-and-check-for-violations
  integration-points
  check-that-the-integration-points-already-exist
  reference-flows
  the-reference-data-flows-through-ze
  must-answer-before-approving-spec
  the-questions-a-spec-must-answer-about-data-flow
impact-analysis ## Impact Analysis
  by-file-type
  yang-schema-files
  what-a-yang-change-also-touches
  registration-files
  what-a-registration-change-also-touches
  go-source-go-under-internal
  what-a-go-change-also-touches
  functional-test-ci
  what-a-ci-test-change-also-touches
  go-source-to-documentation
  check-code-to-docs-when-you-change-code
  documentation-docs
  what-a-docs-change-also-touches
  spec-plan-spec-md
  what-a-spec-change-also-touches
  quick-grep-patterns
  the-grep-recipes-for-impact-analysis
zefs-persistence-no-loose-state-files ## zefs Persistence (no loose state files)
  the-rule
  persist-runtime-state-in-zefs-not-a-loose-file
  use-statestore-put-and-get-never-os-writefile
  why
  a-loose-state-file-is-lost-at-reimage
  how
  example-of-a-statestore-save-and-restore
  register-the-key-and-share-the-one-store-handle
  legitimate-raw-filesystem-writes-allowlisted
  not-every-file-write-is-state
  the-raw-writes-that-stay-on-the-allowlist
  gate
  the-gate-that-flags-a-raw-filesystem-write
ze-divergences-from-standard-go ## Ze Divergences from Standard Go
  encoding-wire
  where-ze-encoding-diverges-from-standard-go
  architecture-registration
  where-ze-registration-diverges-from-standard-go
  config-schema
  where-ze-config-diverges-from-standard-go
  communication-ipc
  where-ze-ipc-diverges-from-standard-go
  testing
  where-ze-testing-diverges-from-standard-go
  cli-commands
  where-the-ze-cli-diverges-from-standard-go
  scripts-tooling
  where-ze-tooling-diverges-from-standard-go
server-rendered-markup ## Server-Rendered Markup
  keep-markup-in-templ-and-out-of-go
  the-guard-that-owns-each-property
architecture-summary ## Architecture Summary
  system
  the-bgp-and-plugin-layer-diagram
  what-each-layer-of-the-system-does
  negotiated-capabilities-per-peer
  the-negotiated-capability-fields
  the-same-bytes-parse-differently-per-capability-set
  wire-writing
  every-type-implements-bufwriter
  update-structure
  the-parts-of-a-bgp-update
  wireupdate-vs-rib
  wireupdate-transports-the-rib-stores
  forward-pool
  two-tier-model-with-per-destination-peer-workers
  the-peer-pools-and-the-global-shared-pool
  chaos-simulator
  the-chaos-event-buffer-never-drops-an-event
  api-command-syntax
  example-of-the-text-and-binary-command-forms
related ## Related
  where-to-read-more-about-placement
rationale ## Rationale
  incidents-behind-design-context
  designing-before-loading-context-went-wrong
  a-session-reinvented-directbridge
  incident-behind-the-sibling-call-site-audit
  a-one-site-fix-left-three-siblings-broken
