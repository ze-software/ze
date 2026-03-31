# Self-Documenting Code

**BLOCKING:** Code that implements external APIs or protocols MUST reference the upstream spec inline.

## Why

Claude has no long-term memory across sessions. When reading code, inline references to external
specs, APIs, and upstream projects provide the context needed to understand constraints and ensure
continued alignment. Without them, every session must rediscover what the code is implementing.

## Rules

| Situation | Required |
|-----------|----------|
| Implementing an external API | Comment block with upstream repo URL, spec/endpoints file, consuming projects |
| Following an RFC | `// RFC NNNN Section X.Y — see rfc/short/rfcNNNN.md` near the relevant code |
| Matching another project's format | URL to the format definition |
| Using a vendored library | Version and source URL in the import area or vendor manifest |

## Format

Place at file top, after `// Design:` and `// Related:` lines:

```
// Implements the birdwatcher API consumed by Alice-LG.
// Reference: https://github.com/alice-lg/birdwatcher
// API spec: https://github.com/alice-lg/birdwatcher/blob/master/endpoints.go
```

## Not Required

- Internal APIs (ze-to-ze communication)
- Standard library usage
- Well-known protocols where the RFC number in a comment suffices
