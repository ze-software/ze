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
  share-type-definitions-never-pointers
component-vs-plugin-placement-blocking ## Component vs Plugin Placement (BLOCKING)
  the-folder-test-copy-it-in-and-commands-are-live
sdk-is-generic-blocking ## SDK Is Generic (BLOCKING)
  keep-plugin-specific-code-out-of-the-sdk
proximity-principle-blocking ## Proximity Principle (BLOCKING)
  keep-related-code-in-one-folder
  home-a-doctor-check-with-the-dependency-it-checks
yang-is-required-blocking ## YANG Is Required (BLOCKING)
  give-every-rpc-a-yang-registration
  never-put-command-handlers-in-the-engine-core
import-rules-blocking ## Import Rules (BLOCKING)
  infrastructure-must-not-import-plugin-implementations
  which-plugin-imports-are-allowed
plugin-boundary-naming-blocking ## Plugin Boundary Naming (BLOCKING)
  name-a-dispatch-helper-for-what-it-does
5-stage-protocol ## Command Answer Framing (BLOCKING)
  a-command-answer-is-always-a-record-sequence
  the-frame-never-follows-the-payload
  a-command-handler-may-answer-with-rows
onstarted-vs-onallpluginsready-blocking ## OnStarted vs OnAllPluginsReady (BLOCKING)
  which-startup-callback-to-use
exclusive-role-claims-blocking-for-cross-plugin-default ## Exclusive Role Claims (BLOCKING for cross-plugin default overrides)
  declare-a-role-claim-never-announce-it-at-runtime
  why-onallpluginsready-cannot-resolve-a-role
  an-unclaimed-role-reads-false-so-b-keeps-working
  honour-the-per-event-retraction-of-a-claim
peer-up-barrier-blocking-for-plugins-that-register-peers ## Peer-Up Barrier (BLOCKING for plugins that register peers)
  declare-peerupbarrier-when-you-gate-on-peer-up
  count-the-barrier-over-the-delivery-set
  keep-the-barrier-separate-from-the-api-sync-wait
  the-barrier-times-out-and-warns
registration-metadata-feeds-generated-docs ## Registration Metadata Feeds Generated Docs
  the-website-catalog-is-generated-from-registration
  never-hand-write-a-second-plugin-list
optional-dependencies ## Optional Dependencies
  declare-a-soft-dependency-as-optionaldependencies
  detect-an-absent-optional-dependency-at-run-time
family-registration-blocking ## Family Registration (BLOCKING)
  register-every-address-family-a-plugin-handles
runtime-filter-declaration-planned-stage-1-wire-protocol ## Non-CIDR Filter Declaration (BLOCKING for filter plugin authors)
  what-each-family-set-emits-to-a-filter
answer-shape-declaration-stage-1-wire-protocol ## Answer Shape Declaration (stage 1 wire protocol)
  a-plugin-declaration-never-panics-the-daemon
  one-shape-whatever-the-argument
  what-the-engine-checks-and-what-it-cannot
runtime-pipe-alias-declaration-stage-1-wire-protocol ## Runtime Pipe Alias Declaration (stage 1 wire protocol)
  a-pipe-alias-reshapes-an-answer-it-cannot-produce-one
  what-refuses-a-declared-alias-name
modification-accumulator-buffer-arity ## Modification-Accumulator Buffer Arity (BLOCKING for filter plugin authors)
  pass-a-whole-number-of-wire-values
renaming-a-registered-name-blocking ## Renaming a Registered Name (BLOCKING)
  grep-every-consumer-before-renaming
  resolve-every-match-before-committing
  dots-for-subsystem-keys-hyphens-for-plugin-names
directbridge-choosing-the-right-communication-pattern ## DirectBridge: Choosing the Right Communication Pattern (BLOCKING)
  read-directbridge-before-designing-new-plumbing
  never-add-a-direct-call-mechanism-beside-directbridge
  never-use-eventbus-for-request-and-response
  never-call-into-another-component-directly
eventbus-typed-payloads-blocking ## EventBus Typed Payloads (BLOCKING)
  declare-every-new-event-with-a-typed-handle
  plugin-delivery-is-async-unlike-in-process
  subscribe-an-external-plugin-to-another-namespace
  carry-a-correlation-token-on-the-request-event
  assert-every-eventbus-test-stub-at-compile-time
  subscribe-through-the-typed-handle
plugin-self-containment ## Plugin Self-Containment
  a-plugin-owns-its-entire-feature-surface
  what-removal-must-achieve
  move-a-surface-whose-removal-breaks-something-else
  shared-dispatch-carries-selector-scope-not-spelling
  the-wiremethod-prefix-is-a-label-not-ownership
  a-command-stays-central-only-with-no-single-owner
  the-steps-of-carving-a-command-out
  put-a-multi-owner-verb-root-in-a-central-package
  never-delete-the-bare-verb-anchor
  container-merge-or-augment-to-attach-to-an-anchor
  give-a-multi-verb-feature-its-own-module
  a-new-feature-registers-it-never-edits-a-switch
  write-a-removal-compliance-test-and-gate-it
  add-both-halves-of-the-guard-when-you-carve
plugin-process-boundary ## Plugin Process Boundary
  check-isinternal-when-you-call-such-a-function
  refuse-or-warn-by-how-much-value-survives
  judge-the-severity-per-plugin
  add-each-new-dangerous-call-to-the-check
registration-based-dispatch ## Registration-Based Dispatch
  dispatch-subcommands-by-registration-not-switch
  use-subdispatch-for-any-command-group
  the-banned-dispatch-patterns
feature-gate-registration ## Feature-Gate Registration
  no-always-on-package-may-import-a-gated-feature
  extract-a-borrowed-helper-before-gating
  declare-every-gated-package-in-feature-gates-txt
  the-static-tag-lists-are-generated-not-edited
  the-procedure-for-adding-a-feature-gate
  the-seam-shape-for-ssh-and-gnmi
  the-core-level-seam-for-several-start-sites
  gating-a-self-registering-plugin
  mind-both-composition-roots-when-gating-a-plugin
  the-techniques-that-clear-always-on-importers
  a-gated-file-still-pins-a-different-gate
  a-dependent-gate-emits-a-compound-constraint
  shared-contract-leaves-stay-ungated
  tag-dependent-files-and-give-each-an-honest-stub
  the-subsystem-builder-seam-for-hub-construction
  what-a-feature-gate-must-never-do
related-rules ## Related Rules
  pointers-to-related-rules
