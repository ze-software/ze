---
kind: directive
level: MUST
stage:
rationale: ai/rationale/protocol.md
---
**Each situation below MUST carry the inline reference its row names:**

| Situation | Required |
|-----------|----------|
| Implementing an external API | A comment block with the upstream repo URL, the spec or endpoints file, and the consuming projects |
| Following an RFC | `// RFC NNNN Section X.Y -- see rfc/short/rfcNNNN.md` near the relevant code |
| Matching another project's format | The URL of the format definition |
| Using a vendored library | The version and source URL, in the import area or the vendor manifest |
