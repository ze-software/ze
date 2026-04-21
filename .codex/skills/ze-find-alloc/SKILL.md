---
name: ze-find-alloc
description: Use when working in the Ze repo and the user asks for ze-find-alloc or for an audit of avoidable encoding allocations. Search production encoding paths for allocation-heavy patterns, classify them by impact, and report where buffer-writing should replace them.
---

# Ze Find Alloc

This is a read-only audit for avoidable allocations in Ze wire-encoding paths.

## Workflow

1. Search production Go files, optionally scoped to a path, while excluding tests and known legitimate allocation sites.
2. Look for allocation-heavy encoding patterns such as `Pack() []byte`, `Encode*() []byte`, `make([]byte, ...)`, inline byte-slice builders, and append-driven wire construction.
3. Classify each finding by impact: hot path, attribute, NLRI, capability, API builder, plugin path, or utility.
4. Report:
   - per-file findings
   - existing `WriteTo` coverage
   - the highest-value migration targets

## Rules

- Read-only.
- Do not report obvious test-only or infrastructure allocations as problems.
