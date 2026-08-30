---
kind: directive
level: MUST
stage:
rationale: ai/rationale/protocol.md
---
**Code that implements an external API, follows an RFC, matches another project's format, or uses a vendored library MUST carry the upstream reference inline, at the top of the file after the `// Design:` and `// Related:` lines.** An external API names the upstream repository URL, the spec or endpoints file, and the consuming projects. RFC code carries `// RFC NNNN Section X.Y -- see rfc/short/rfcNNNN.md` beside the relevant code. A foreign format names the URL of the format definition, and a vendored library names its version and source URL. An internal ze-to-ze API, standard library usage, and a well-known protocol whose RFC number already sits in a comment need none.
