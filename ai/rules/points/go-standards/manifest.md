---
title: Go Standards
when: writing Go in Ze: naming, env access, context, logging, imports, errors, API contracts, typed-vs-string choices, file layout and cross-references, compatibility shims, or a Go compiler bump
severity: blocking
related: config, cli, performance, repo-maintenance, architecture
---
directives ## Directives
  read-the-ze-style-guide-before-go-design-or-review
  guard-with-early-returns-one-fact-per-guard
  always-follow-these-required-go-patterns
  log-through-slog-never-printf
  never-add-new-third-party-imports-not-already-in-go-mod
  access-env-vars-only-through-internal-core-env
  read-the-config-rule-before-adding-an-env-var
  reset-the-env-cache-after-a-direct-os-setenv
  register-every-env-var-with-mustregister
  use-os-getenv-only-for-system-env-vars
  alias-an-import-when-two-packages-share-a-name
  put-first-party-tooling-in-native-go-packages
  adopt-these-style-patterns-on-touch-not-in-a-sweep
  prefer-guard-clauses-slice-types-and-narrow-constructors
  tag-every-json-field-with-a-kebab-case-name
  never-write-these-forbidden-go-patterns
naming ## Naming
  ze-means-the-with-a-french-accent
  config-naming-lives-in-the-config-rule
  name-a-value-for-what-it-is-not-its-go-type
  pick-a-package-name-from-the-glossary-not-a-synonym
prefer-typed-numeric-over-string ## Prefer Typed Numeric Over String
  use-typed-numeric-identity-on-hot-paths
  make-zero-invalid-and-never-compare-with-string
  convert-to-string-only-at-the-wire-and-human-sinks
  key-a-hot-path-map-with-a-numeric-type
  parse-the-string-once-at-the-boundary
  bind-an-accepted-string-key-to-a-constant
  the-checks-to-run-before-adding-a-string
api-contracts-in-comments ## API Contracts in Comments
  document-every-caller-obligation-in-the-godoc
  write-must-and-put-it-on-both-sides-of-the-pair
  name-the-release-and-the-ordering-in-both-comments
file-modularity ## File Modularity
  keep-one-concern-in-each-go-file
  judge-file-size-against-one-threshold-only
  ask-one-concern-before-you-create-or-extend-a-file
  split-a-file-with-go-extract-and-fix-the-headers
  size-alone-is-not-a-reason-to-split
design-document-references ## Design Document References
  every-go-file-carries-a-design-comment
  write-the-design-line-with-a-topic-annotation
  the-design-line-must-be-the-first-comment-in-every-file
file-cross-references ## File Cross-References
  every-cross-reference-needs-a-back-reference
  when-a-file-needs-no-cross-reference
  point-detail-lines-at-non-obvious-relationships
  keep-a-detail-line-a-reader-would-otherwise-miss
external-commands ## External Commands
  ze-code-must-not-fork-a-system-tool
  an-authorised-command-is-in-the-register
  read-the-kernel-interface-instead
  the-harness-is-not-ze
no-backwards-compatibility ## No Backwards Compatibility
  ze-is-unreleased-so-write-no-compat-code
  internal-code-changes-freely-forever
  the-plugin-api-is-frozen-once-released
  only-the-plugin-api-contract-is-frozen-not-its-code
  keep-exabgp-awareness-out-of-engine-code
go-compiler-upgrade-checklist ## Go Compiler Upgrade Checklist
  compare-noescape-against-the-stdlib-after-a-go-update
