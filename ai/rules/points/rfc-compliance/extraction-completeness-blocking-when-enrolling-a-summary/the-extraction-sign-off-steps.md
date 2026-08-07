---
kind: table
level:
stage:
---
| Step | Command / file |
|------|----------------|
| Write the unclassified skeleton | `make ze-rfc-extract STEM=<stem>` |
| Classify every derived site and section by hand | `rfc/extraction/<stem>.json` |
| Re-check the arithmetic | `make ze-rfc-check` |
| Read the published backlog | `ai/RFC-REQUIREMENTS.md`, "Extraction sign-off" |
| Read the counts machine-readably | `make ze-rfc-extraction-status` |
