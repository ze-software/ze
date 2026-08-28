---
kind: table
level:
stage:
---
| Step | Command / file |
|------|----------------|
| Write the unclassified skeleton | `./le rfc extraction-create STEM=<stem>` |
| Classify every derived site and section by hand | `rfc/extraction/<stem>.json` |
| Re-check the arithmetic | `./le rfc check` |
| Read the published backlog | `ai/RFC-REQUIREMENTS.md`, "Extraction sign-off" |
| Read the counts machine-readably | `./le rfc extraction-status` |
