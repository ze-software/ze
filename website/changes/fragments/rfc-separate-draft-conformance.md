---
spec: rfc-separate-draft-conformance
topic: RFC Compliance
---
## Before

Seven enrolled IETF Internet-Drafts (`draft-*`, totaling 91 MUST rows: 55 proven, 3 proof pending, 33 not built) were counted inside Ze's published RFC conformance figures alongside published RFCs, and summary validation did not block an Internet-Draft from being listed as a required member of a release profile.

## After

Internet-Drafts (`draft-*`) are now separated from published RFC conformance counts across `internal/le/rfc` and `internal/le/site`. `ImplementedSectionsOf` reports RFC conformance and Draft conformance separately (`ImplementedSections.Drafts`), `rfc/index.json` marks `draft-*` entries out of the RFC `enrolled` subject, and `checkUseCases` refuses listing any `draft-*` summary as a required use-case or release-profile target until it is published as an RFC.

## Corrects

none

## Exposure

none

## Upstream

none

## Asked by

owner
