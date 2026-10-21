# Implemented draft not enrolled in the RFC ledger

Ze implements a requirement of a document that has no `rfc/short/<stem>.md`
summary, so no requirement row, tag or discrimination record can name it and
`./le rfc check` never sees the code.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-10-21 | spec-exabgp-6-api-compatibility | `rfc/drafts/draft-simpson-idr-flowspec-redirect-02.txt` Section 3; `rib_flowspec_redirect.go`, reactor `discardRedirectAction` | The redirect-to-IP EBGP check ("MUST also, by default, enforce the following check", "MUST be possible to disable this additional validation check on a per-EBGP session basis") and the next-hop reset are implemented and tested, but the draft has no `rfc/short` summary, so they carry no requirement rows. No rows were invented. | open: enrol the draft (walk its text into an `rfc/short` summary), then tag the tests |
