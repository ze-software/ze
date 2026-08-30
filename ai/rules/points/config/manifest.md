---
title: Configuration and YANG
when: adding or changing a config option, YANG module, env var, listener endpoint, or code that reads config values
severity: blocking
related: go-standards, cli, architecture, evidence, plugins
---
directives ## Directives
  manipulate-config-only-by-the-two-approved-methods
config-surface-yang-config-vs-env-var ## Config Surface: YANG Config vs Env Var
  default-to-yang-config-env-only-is-the-exception
  examples-of-settings-that-belong-in-yang
  examples-of-settings-that-stay-env-only
  config-value-precedence-order
  document-the-env-var-override-in-the-leaf-description
  signals-that-an-env-var-should-become-yang-config
config-naming-conventions ## Config Naming Conventions
  spell-yang-leaf-names-out-in-full
  fix-a-singular-list-name-before-release
  match-the-env-var-final-segment-to-the-yang-leaf
  register-an-alias-for-a-legacy-env-var
  alias-the-old-env-var-path-when-the-yang-tree-moves
config-design ## Config Design
  use-augment-only-across-components
  model-every-listener-with-the-shared-grouping
  refine-listener-defaults-per-service
yang-module-structure ## YANG Module Structure
  component-hyphens-and-reserved-prefixes
  import-the-shared-modules-and-use-their-typedefs
  use-ze-validate-only-for-runtime-determined-sets
  state-a-unit-with-the-units-statement-not-the-name
  give-every-dimensioned-leaf-a-protocol-sane-default
  model-an-endpoint-as-two-fields-from-a-grouping
  name-a-local-bind-ip-and-a-remote-target-address
  give-every-on-off-setting-the-same-enabled-shape
  model-equivalent-concepts-the-same-across-protocols
  diverge-only-for-a-genuine-rfc-term-difference
  grep-the-sibling-protocols-before-adding-a-concept
  follow-the-majority-command-module-naming-scheme
config-manipulation ## Config Manipulation
  manipulate-config-by-a-parsed-tree-or-set-commands
  never-edit-config-as-raw-text
config-string-coercion ## Config String Coercion
  never-assert-a-native-type-on-a-config-value
  how-a-native-type-assertion-silently-drops-config
  accept-the-string-form-in-every-config-coercion
  allowlist-only-a-genuine-non-config-coercion
config-list-shapes ## Config List Shapes
  read-a-leaf-list-or-a-list-through-configvalue
  test-a-leaf-list-reader-at-one-member
