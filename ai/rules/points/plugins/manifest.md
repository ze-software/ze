---
title: Plugins
when: creating or changing a plugin: its registration, placement, transport, command surface, process boundary, dispatch table, or a feature gate
severity: blocking
related: repo-maintenance, cli, evidence
---
directives ## Directives
  all-plugins-must-follow-these-patterns
  summary-of-the-plugin-directives
cross-boundary-value-types-blocking ## Cross-Boundary Value Types (BLOCKING)
  cross-a-boundary-with-value-types-only
  the-value-type-rule-per-surface
  share-type-definitions-never-pointers
architecture ## Architecture
  where-each-plugin-layer-lives
component-vs-plugin-placement-blocking ## Component vs Plugin Placement (BLOCKING)
  pointer-to-the-command-ownership-rationale
  three-directories-three-roles
  what-each-directory-holds-and-depends-on
  the-folder-test-copy-it-in-and-commands-are-live
  plugin-layout
  the-plugin-directory-tree
  hand-write-the-yang-and-generate-the-go-glue
  what-goes-where
  where-each-plugin-artifact-belongs
sdk-is-generic-blocking ## SDK Is Generic (BLOCKING)
  keep-plugin-specific-code-out-of-the-sdk
  what-a-generic-sdk-requires
proximity-principle-blocking ## Proximity Principle (BLOCKING)
  keep-related-code-in-one-folder
  pointer-to-the-user-facing-removal-test
  what-proximity-requires
  home-a-doctor-check-with-the-dependency-it-checks
yang-is-required-blocking ## YANG Is Required (BLOCKING)
  give-every-rpc-a-yang-registration
  which-registrations-require-yang
  never-put-command-handlers-in-the-engine-core
import-rules-blocking ## Import Rules (BLOCKING)
  infrastructure-must-not-import-plugin-implementations
  which-plugin-imports-are-allowed
plugin-boundary-naming-blocking ## Plugin Boundary Naming (BLOCKING)
  name-a-dispatch-helper-for-what-it-does
  banned-dispatch-helper-names
5-stage-protocol ## 5-Stage Protocol
  the-startup-handshake-stages
  what-happens-after-the-handshake-completes
onstarted-vs-onallpluginsready-blocking ## OnStarted vs OnAllPluginsReady (BLOCKING)
  plugins-load-across-several-phases
  which-startup-callback-to-use
  the-bgp-rpki-reference-for-cross-plugin-dispatch
exclusive-role-claims-blocking-for-cross-plugin-default ## Exclusive Role Claims (BLOCKING for cross-plugin default overrides)
  declare-a-role-claim-never-announce-it-at-runtime
  how-a-role-claim-is-declared-and-resolved
  why-stage-2-delivery-is-early-enough
  why-onallpluginsready-cannot-resolve-a-role
  an-unclaimed-role-reads-false-so-b-keeps-working
  the-bgp-rs-role-claim-reference
peer-up-barrier-blocking-for-plugins-that-register-peers ## Peer-Up Barrier (BLOCKING for plugins that register peers)
  declare-peerupbarrier-when-you-gate-on-peer-up
  how-the-peer-up-barrier-is-counted-and-released
  count-the-barrier-over-the-delivery-set
  keep-the-barrier-separate-from-the-api-sync-wait
  the-barrier-times-out-and-warns
  why-bgp-rs-declares-the-barrier
registration-fields ## Registration Fields
  the-registration-struct-fields
registration-metadata-feeds-generated-docs ## Registration Metadata Feeds Generated Docs
  the-website-catalog-is-generated-from-registration
  never-hand-write-a-second-plugin-list
optional-dependencies ## Optional Dependencies
  declare-a-soft-dependency-as-optionaldependencies
  hard-versus-optional-dependencies
  both-dependency-fields-validate-the-same-way
  startup-order-covers-optional-edges-too
  the-owner-owns-the-fallback-when-a-dep-is-absent
  detect-an-absent-optional-dependency-at-run-time
family-registration-blocking ## Family Registration (BLOCKING)
  register-every-address-family-a-plugin-handles
  what-family-registration-guarantees
runtime-filter-declaration-planned-stage-1-wire-protocol ## Runtime Filter Declaration (planned -- stage 1 wire protocol)
  declare-route-filters-at-stage-1
  the-filter-declaration-fields
  non-cidr-families-blocking-for-filter-plugin-authors
  what-each-family-set-emits-to-a-filter
  pointer-to-the-non-cidr-filter-contract
modification-accumulator-buffer-arity ## Modification-Accumulator Buffer Arity (BLOCKING for filter plugin authors)
  pass-a-whole-number-of-wire-values
  what-one-op-may-carry-per-list-attribute
  who-checks-the-arity-and-what-a-violation-costs
renaming-a-registered-name-blocking ## Renaming a Registered Name (BLOCKING)
  a-registered-name-lives-in-many-loose-strings
  the-bgp-subsystem-rename-that-broke-logging
  grep-every-consumer-before-renaming
  where-each-consumer-of-a-name-lives
  mechanical-check-before-committing-the-rename
  the-rename-grep-command
  resolve-every-match-before-committing
  dots-for-subsystem-keys-hyphens-for-plugin-names
new-plugin-checklist ## New Plugin Checklist
  the-new-plugin-checklist
  what-make-generate-populates-for-you
invocation-modes ## Invocation Modes
  how-each-invocation-mode-is-spelled
transport ## Transport
  transport-and-auth-per-plugin-type
  how-an-external-plugin-authenticates-on-connect
directbridge-choosing-the-right-communication-pattern ## DirectBridge: Choosing the Right Communication Pattern (BLOCKING)
  read-directbridge-before-designing-new-plumbing
  pointer-to-the-plugin-system-design-history
  which-communication-pattern-to-use
  never-add-a-direct-call-mechanism-beside-directbridge
  never-use-eventbus-for-request-and-response
  never-call-into-another-component-directly
structured-event-delivery-directbridge ## Structured Event Delivery (DirectBridge)
  internal-plugins-receive-structured-events
  what-each-structured-event-carries
  read-attributes-and-nlris-without-copying
eventbus-typed-payloads-blocking ## EventBus Typed Payloads (BLOCKING)
  declare-every-new-event-with-a-typed-handle
  plugin-delivery-is-async-unlike-in-process
  subscribe-an-external-plugin-to-another-namespace
  carry-a-correlation-token-on-the-request-event
  assert-every-eventbus-test-stub-at-compile-time
  the-eventbus-stub-assertion-line
  why-the-stub-assertion-is-needed
  subscribe-through-the-typed-handle
plugin-self-containment ## Plugin Self-Containment
  a-plugin-owns-its-entire-feature-surface
  self-containment-is-the-load-bearing-invariant
  the-removal-test
  what-deleting-a-plugin-must-do
  what-removal-must-achieve
  move-a-surface-whose-removal-breaks-something-else
  what-this-forbids
  anti-patterns-that-fail-the-removal-test
  what-shared-code-may-do
  shared-dispatch-carries-selector-scope-not-spelling
  finding-the-owner-follow-the-code-not-the-wire-method
  the-wiremethod-prefix-is-a-label-not-ownership
  trace-the-handler-s-real-dependencies
  worked-examples-of-finding-a-command-s-owner
  a-command-stays-central-only-with-no-single-owner
  where-a-plugin-s-surface-lives
  where-each-surface-of-a-plugin-lives
  keep-only-generic-commands-in-a-central-verb
  how-to-carve-a-command-into-its-owner
  the-steps-of-carving-a-command-out
  unowned-verb-roots-multi-owner-verbs
  put-a-multi-owner-verb-root-in-a-central-package
  never-delete-the-bare-verb-anchor
  container-merge-or-augment-to-attach-to-an-anchor
  the-guard-test-keeps-a-command-out-of-the-anchor
  dedicated-feature-modules
  give-a-multi-verb-feature-its-own-module
  registration-over-hardcoding-the-cli-client-too
  the-cli-client-registers-views-too
  the-per-feature-field-anti-pattern-in-cli-model
  the-client-side-view-registry-is-the-correct-shape
  a-new-feature-registers-it-never-edits-a-switch
  mechanical-check
  write-a-removal-compliance-test-and-gate-it
  the-bgp-show-schema-guard-and-its-owner-half
  add-both-halves-of-the-guard-when-you-carve
plugin-process-boundary ## Plugin Process Boundary
  the-problem
  a-plugin-runs-internal-or-external
  a-direct-call-silently-no-ops-when-external
  confirmed-instances-all-fixed
  the-known-same-process-effect-calls
  the-rule
  check-isinternal-when-you-call-such-a-function
  refuse-or-warn-by-how-much-value-survives
  judge-the-severity-per-plugin
  the-mechanical-check
  what-the-plugin-boundary-check-scans
  add-each-new-dangerous-call-to-the-check
registration-based-dispatch ## Registration-Based Dispatch
  rule
  dispatch-subcommands-by-registration-not-switch
  how-to-apply
  use-subdispatch-for-any-command-group
  banned-patterns
  the-banned-dispatch-patterns
  why-registration-based-dispatch-not-switch-case
  what-switch-based-dispatch-costs
  what-registration-based-dispatch-buys
  example-sub-dispatcher-registration
  a-sub-dispatcher-example
feature-gate-registration ## Feature-Gate Registration
  what-a-compile-out-able-feature-means
  read-this-before-touching-a-feature-gate
  pointers-to-the-companion-rules
  the-one-invariant
  no-always-on-package-may-import-a-gated-feature
  extract-a-borrowed-helper-before-gating
  feature-gates-txt-is-the-single-source-of-truth
  declare-every-gated-package-in-feature-gates-txt
  a-single-feature-manifest-example
  the-manifest-line-format
  every-consumer-derives-from-the-manifest
  the-static-tag-lists-are-generated-not-edited
  procedure-add-a-feature-gate
  the-procedure-for-adding-a-feature-gate
  the-manifest-is-the-only-declaration-point
  the-registration-shapes
  the-listener-service-registration-shape
  the-seam-shape-for-ssh-and-gnmi
  the-core-level-seam-for-several-start-sites
  gating-a-self-registering-plugin
  a-multi-package-manifest-example
  mind-both-composition-roots-when-gating-a-plugin
  extract-then-gate-at-subsystem-scale
  the-techniques-that-clear-always-on-importers
  traps-that-appear-only-at-subsystem-scale
  a-gated-file-still-pins-a-different-gate
  a-dependent-gate-emits-a-compound-constraint
  a-tag-may-mix-independent-and-dependent-packages
  shared-contract-leaves-stay-ungated
  tag-dependent-files-and-give-each-an-honest-stub
  the-subsystem-builder-seam-for-hub-construction
  banned
  what-a-feature-gate-must-never-do
related-rules ## Related Rules
  pointers-to-related-rules
