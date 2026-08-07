---
title: Configuration and YANG
when: adding or changing a config option, YANG module, env var, listener endpoint, or code that reads config values
severity: blocking
related: go-standards, cli, architecture, evidence, plugins
---
directives ## Directives
  manipulate-config-only-by-the-two-approved-methods
  where-the-template-and-rationale-live
config-surface-yang-config-vs-env-var ## Config Surface: YANG Config vs Env Var
  decision-table
  questions-that-decide-yang-config-or-env-var
  default-to-yang-config-env-only-is-the-exception
  yang-config-operator-facing
  settings-that-belong-in-the-config-tree
  examples-of-settings-that-belong-in-yang
  what-yang-config-gives-you
  env-var-only-internal-debug
  settings-that-stay-as-env-vars
  examples-of-settings-that-stay-env-only
  what-env-var-only-settings-lack
  when-both-exist
  keep-the-env-var-as-an-override-after-promotion
  config-value-precedence-order
  document-the-env-var-override-in-the-leaf-description
  promotion-signals
  when-to-promote-an-env-var-to-yang-config
  signals-that-an-env-var-should-become-yang-config
  new-setting-checklist
  before-adding-any-tunable-setting
  checklist-items-for-a-new-tunable-setting
config-naming-conventions ## Config Naming Conventions
  yang-leaves
  yang-leaf-naming-rules
  spell-yang-leaf-names-out-in-full
  abbreviations-allowed-in-yang-leaf-names
  env-vars
  env-var-naming-rules
  match-the-env-var-final-segment-to-the-yang-leaf
  register-an-alias-for-a-legacy-env-var
  the-legacy-env-var-aliases
  hierarchy-env-var-path-mirrors-yang-path
  mirror-the-yang-path-in-the-env-var-path
  yang-path-to-env-var-examples
  alias-the-old-env-var-path-when-the-yang-tree-moves
  go-struct-fields
  go-struct-field-naming-rules
  container-naming
  yang-container-naming-rules
  naming-new-settings-checklist
  checklist-items-for-naming-a-new-setting
config-design ## Config Design
  yang-structure
  when-to-use-grouping-and-when-augment
  use-augment-only-across-components
  listeners
  model-every-listener-with-the-shared-grouping
  which-listener-pattern-to-use
  refine-listener-defaults-per-service
yang-module-structure ## YANG Module Structure
  module-identity
  canonical-module-name-namespace-and-prefix
  component-hyphens-and-reserved-prefixes
  imports-and-shared-vocabulary
  import-the-shared-modules-and-use-their-typedefs
  value-typing
  use-the-shared-typedef-never-a-local-copy
  the-shared-typedef-for-each-concept
  use-ze-validate-only-for-runtime-determined-sets
  units
  state-a-unit-with-the-units-statement-not-the-name
  unit-statement-rules
  give-every-dimensioned-leaf-a-protocol-sane-default
  the-units-section-supersedes-the-name-suffix-rule
  network-endpoints
  model-an-endpoint-as-two-fields-from-a-grouping
  the-grouping-and-port-type-for-each-endpoint-kind
  name-a-local-bind-ip-and-a-remote-target-address
  boolean-toggles-and-flags
  give-every-on-off-setting-the-same-enabled-shape
  boolean-toggle-rules
  defaults-and-enums
  default-and-enum-rules
  layout
  yang-file-layout-rules
  cross-protocol-consistency
  model-equivalent-concepts-the-same-across-protocols
  the-canonical-model-for-each-shared-concept
  diverge-only-for-a-genuine-rfc-term-difference
  grep-the-sibling-protocols-before-adding-a-concept
  command-modules-naming-standardization-deferred
  why-command-module-naming-is-not-converged-yet
  follow-the-majority-command-module-naming-scheme
  yang-mechanical-check
  before-saving-a-yang-edit
  checklist-items-before-saving-a-yang-edit
config-manipulation ## Config Manipulation
  manipulate-config-by-a-parsed-tree-or-set-commands
  the-two-approved-config-manipulation-methods
  forbidden
  never-edit-config-as-raw-text
  why-concatenating-two-configs-is-valid
config-string-coercion ## Config String Coercion
  the-problem
  config-values-arrive-as-json-strings
  how-a-native-type-assertion-silently-drops-config
  the-ddos-detect-feature-that-never-ran
  the-rule
  accept-the-string-form-in-every-config-coercion
  never-assert-a-native-type-on-a-config-value
  the-mechanical-check
  the-check-that-finds-a-missing-string-arm
  allowlist-only-a-genuine-non-config-coercion
examples ## Examples
  what-the-coercion-helper-example-shows
  example-a-string-tolerant-coercion-helper
  what-the-yang-leaf-examples-show
  example-a-dimensioned-leaf-and-a-boolean-toggle
