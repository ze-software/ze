---
title: Memory and Encoding
when: before writing buffer, pool, allocation, string-building, or wire-encoding code
severity: blocking
related: architecture, go-standards, repo-maintenance
---
directives ## Directives
  write-wire-encoding-into-pooled-bounded-buffers
common-mistakes ## Common Mistakes
  fix-these-common-allocation-mistakes
three-rules ## Three Rules
  never-use-fmt-or-string-on-a-hot-path
hot-path-rule ## Hot Path Rule
  apply-the-hot-path-ban-to-these-packages
