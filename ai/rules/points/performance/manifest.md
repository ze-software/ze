---
title: Memory and Encoding
when: before writing buffer, pool, allocation, string-building, or wire-encoding code
severity: blocking
related: architecture, go-standards, repo-maintenance
---
directives ## Directives
  write-wire-encoding-into-pooled-bounded-buffers
the-core-idea ## The Core Idea
  why-every-allocation-on-the-update-path-costs
  the-strategies-that-remove-allocations
data-lifecycle-wire-to-wire ## Data Lifecycle (Wire to Wire)
  the-wire-to-wire-buffer-path
  ownership-rules
  who-owns-the-buffer-at-each-stage
  when-copies-happen
  copies-are-deliberate-never-accidental
  when-a-copy-is-deliberate
  question-any-copy-outside-these-triggers
pool-types ## Pool Types
  the-pools-ze-runs
  pool-strategy-by-goroutine-shape
  the-pool-shape-choice-is-load-bearing
  pick-the-pool-shape-from-the-goroutine-shape
  size-every-buffer-in-a-pool-the-same
key-wire-abstractions ## Key Wire Abstractions
  the-wire-abstractions-and-their-lifetimes
  bufwriter-interface
  what-every-wire-producing-type-implements
  the-bufwriter-signature
  context-dependent-types-add-a-second-method
  the-writetowithcontext-signature
  the-caller-owns-the-buffer-the-callee-writes-into-it
buffer-first-encoding-mechanical-reference ## Buffer-First Encoding: Mechanical Reference
  the-encoding-pools-and-their-sizes
  pattern-get-write-put
  get-from-the-pool-write-then-put-it-back
  pattern-skip-and-backfill-hot-path
  when-to-use-skip-and-backfill
  the-skip-and-backfill-steps
  skip-and-backfill-avoids-the-double-traversal
  banned-in-encoding-code
  what-encoding-code-must-not-call
  when-make-byte-is-ok
  where-a-plain-allocation-is-still-acceptable
  before-writing-encoding-code
  questions-to-ask-before-writing-encoding-code
  audit-and-fix-encoding-allocations
  text-json-format-generation
  apply-the-ban-to-text-and-json-generation-files
caller-owned-resources-blocking-on-hot-paths ## Caller-Owned Resources (BLOCKING on hot paths)
  the-most-common-allocation-mistake
  the-principle
  allocate-once-at-the-outermost-scope-and-pass-inward
  the-callee-writes-into-what-it-is-given
  anti-pattern-per-call-allocation-in-a-loop
  loop-allocation-versus-writeto
  anti-pattern-helper-that-allocates-internally
  sprintf-versus-an-append-based-formatter
  anti-pattern-sub-function-allocates-what-caller-already-has
  a-helper-allocating-its-own-scratch-buffer
  sync-pool-for-reusable-scratch-buffers
  use-sync-pool-when-the-caller-cannot-supply-a-buffer
  a-sync-pool-scratch-buffer
  when-to-use-sync-pool-vs-caller-passed-buffer
  choosing-between-a-pool-and-a-caller-buffer
  seed-a-sync-pool-with-the-common-case-size
  tracing-data-lifecycle-before-writing-code
  answer-these-before-writing-allocation-code
  trace-a-buffer-lifecycle-before-you-write-code
decision-tree-where-should-this-buffer-come-from ## Decision Tree: Where Should This Buffer Come From?
  the-buffer-source-decision-tree
common-mistakes ## Common Mistakes
  fix-these-common-allocation-mistakes
three-rules ## Three Rules
  never-use-fmt-or-string-on-a-hot-path
decision-tree-before-writing-any-fmt-sprintf ## Decision Tree: Before Writing Any `fmt.Sprintf`
  before-writing-any-fmt-sprintf-or-fprintf-errorf
  the-sprintf-replacement-decision-tree
banned-patterns ## Banned Patterns
  string-concatenation-with-is-banned-in-new-code
  build-strings-with-textbuf-never-with-plus
  the-constant-expression-exception
  convert-cold-path-concatenation-on-touch
  replacements-for-plus-concatenation
  concatenation-in-a-loop-compounds-the-cost
  concatenation-versus-a-textbuf-chain
  use-a-standalone-helper-only-for-a-single-value
  fmt-patterns
  replacements-for-fmt-sprintf-calls
  strings-patterns
  replacements-for-the-strings-package
  other
  replacements-for-parse-back-and-padding-patterns
typed-comparison-rule ## Typed Comparison Rule
  store-an-ip-address-as-netip-addr-not-a-string
  string-storage-anti-patterns-and-their-fixes
  store-both-forms-and-parse-once-at-construction
  a-struct-holding-both-string-and-typed-address
hot-path-rule ## Hot Path Rule
  apply-the-hot-path-ban-to-these-packages
  which-paths-count-as-hot
  use-the-appendto-pattern-instead
allowed-fmt-usage ## Allowed fmt Usage
  where-fmt-is-still-allowed
patterns-that-must-not-be-converted ## Patterns That Must NOT Be Converted
  patterns-that-must-stay-as-they-are
textbuf-buffer-canonical-string-builder ## textbuf.Buffer (canonical string builder)
  use-textbuf-buffer-for-all-string-building
  how-textbuf-buffer-stays-off-the-heap
  recheck-the-noescape-trick-on-every-go-update
  allocation-tiers
  tier-0-zero-allocations
  zero-allocation-buffer-examples
  tier-1-one-allocation
  one-allocation-buffer-examples
  never-slice-a-pooled-buffer-without-releasing-it
  choosing-an-init
  how-to-init-and-extract-for-each-pattern
  methods-all-return-buffer-for-chaining
  the-buffer-methods
  string-vs-slice
  use-slice-when-the-string-is-consumed-immediately
  prefer-slice-by-default
  pick-string-or-slice-by-result-lifetime
  slice-versus-string-examples
  reusing-a-buffer-with-reset
  reuse-one-buffer-across-loop-iterations
  a-buffer-reset-between-loop-iterations
  pooled-buffers-use-get-and-release
  a-pooled-buffer-in-a-loop
  standalone-functions-for-single-value-returns
  the-standalone-string-helpers
  when-to-use-the-append-into-buffer-helpers
  the-append-into-buffer-helpers
  which-textbuf-form-to-use-for-each-pattern
  banned
  string-building-anti-patterns-and-their-fixes
  keybuilder-for-grouping-keys-with-separators
  embed-strings-builder-in-a-key-builder
  a-key-builder-type
types-own-their-serialization ## Types Own Their Serialization
  give-every-named-type-an-appendto-method
  give-a-domain-concept-its-own-named-type
  appendto-pattern-for-types
  an-appendto-method-implementation
self-check ## Self-Check
  before-submitting-code-that-builds-strings
  the-string-building-self-check
conversion-anti-patterns-blocking ## Conversion Anti-Patterns (BLOCKING)
  these-errors-recur-during-mechanical-textbuf-conversion
  multiple-buffers-in-one-function
  several-buffers-where-one-would-do
  string-where-slice-suffices
  a-needless-string-copy-on-immediate-use
  where-slice-is-the-right-extraction
  where-string-is-the-right-extraction
  slice-from-stack-allocated-buffer-the-escape-trap
  why-slice-from-a-stack-buffer-dangles
  a-dangling-slice-returned-from-a-stack-buffer
  pick-string-or-slice-by-whether-the-result-escapes
  unnecessary-scratch-buffer-when-output-buffer-exists
  a-needless-scratch-buffer
related-documents ## Related Documents
  documents-that-cover-the-rest
