---
title: Memory and Encoding
when: before writing buffer, pool, allocation, string-building, or wire-encoding code
severity: blocking
related: architecture, go-standards, repo-maintenance
---
directives ## Directives
  write-wire-encoding-into-pooled-bounded-buffers
the-core-idea ## The Core Idea
  the-strategies-that-remove-allocations
data-lifecycle-wire-to-wire ## Data Lifecycle (Wire to Wire)
  question-any-copy-outside-these-triggers
pool-types ## Pool Types
  size-every-buffer-in-a-pool-the-same
key-wire-abstractions ## Key Wire Abstractions
  the-caller-owns-the-buffer-the-callee-writes-into-it
buffer-first-encoding-mechanical-reference ## Buffer-First Encoding
  questions-to-ask-before-writing-encoding-code
  the-skip-and-backfill-steps
  what-encoding-code-must-not-call
  where-a-plain-allocation-is-still-acceptable
  apply-the-ban-to-text-and-json-generation-files
  audit-and-fix-encoding-allocations
caller-owned-resources-blocking-on-hot-paths ## Caller-Owned Resources (BLOCKING on hot paths)
  allocate-once-at-the-outermost-scope-and-pass-inward
  the-callee-writes-into-what-it-is-given
  when-to-use-sync-pool-vs-caller-passed-buffer
  seed-a-sync-pool-with-the-common-case-size
  trace-a-buffer-lifecycle-before-you-write-code
common-mistakes ## Common Mistakes
  fix-these-common-allocation-mistakes
three-rules ## Three Rules
  never-use-fmt-or-string-on-a-hot-path
banned-patterns ## Banned Patterns
  build-strings-with-textbuf-never-with-plus
  the-constant-expression-exception
  convert-cold-path-concatenation-on-touch
  use-a-standalone-helper-only-for-a-single-value
typed-comparison-rule ## Typed Comparison Rule
  store-an-ip-address-as-netip-addr-not-a-string
  store-both-forms-and-parse-once-at-construction
hot-path-rule ## Hot Path Rule
  apply-the-hot-path-ban-to-these-packages
allowed-fmt-usage ## Allowed fmt Usage
  where-fmt-is-still-allowed
patterns-that-must-not-be-converted ## Patterns That Must NOT Be Converted
  patterns-that-must-stay-as-they-are
textbuf-buffer-canonical-string-builder ## textbuf.Buffer (canonical string builder)
  use-textbuf-buffer-for-all-string-building
  prefer-slice-by-default
  never-slice-a-pooled-buffer-without-releasing-it
  recheck-the-noescape-trick-on-every-go-update
types-own-their-serialization ## Types Own Their Serialization
  give-every-named-type-an-appendto-method
  give-a-domain-concept-its-own-named-type
self-check ## Self-Check
  the-string-building-self-check
conversion-anti-patterns-blocking ## Conversion Anti-Patterns (BLOCKING)
  pick-string-or-slice-by-whether-the-result-escapes
related-documents ## Related Documents
  documents-that-cover-the-rest
