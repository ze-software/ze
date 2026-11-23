# Declined RFCs and Internet-Drafts

This directory holds RFCs and Internet-Drafts that Ze will **never** implement,
together with their summaries. They are kept here for historical reference only
(for example, to explain why a wire capability code is preserved as unknown or
why `ze exabgp migrate` does not translate a legacy keyword).

| Rule | Reason |
|------|--------|
| Documents here will never be implemented in Ze | Each entry reflects an explicit owner decision; neither has support or any active users |
| A declined document MUST NOT exist under `rfc/full/` or `rfc/drafts/` | Keeping it out of the corpus removes it from the dependency graph (`rfc/graph.tsv`), `./le rfc tree`, and the RFC map |
| A declined document MUST NOT have a summary under `rfc/short/`, an extraction sign-off under `rfc/extraction/`, or `RFC requirement:` test tags | Declined documents carry no conformance obligations and never enter `rfc/enrolled.txt`, `rfc/not-enrolled.txt`, or `docs/features/rfc-status.md` |
| Summaries under `rfc/short/` MUST NOT cite `rfc/declined/` | A declined specification is never a requirement source |

`./le rfc check` enforces these rules (`checkDeclinedSources` in
`internal/le/rfc/check_declined.go`).

## Layout

- `texts/<stem>.txt`: full IETF text, named by document stem without revision.
- `short/<stem>.md`: concise reference summary recording what the document
  proposes, why Ze will never implement it, and how Ze treats its wire codes
  and migrated ExaBGP configuration.

## Declined Documents

| Document | Title | Status | Why Declined | Ze Handling |
|----------|-------|--------|--------------|-------------|
| [draft-ietf-idr-bgp-multisession](short/draft-ietf-idr-bgp-multisession.md) ([text](texts/draft-ietf-idr-bgp-multisession.txt)) | Multisession BGP (`-07`) | Expired 2013-03-16 (never became an RFC) | No adoption or deployment; nothing uses it (`ai/META-DECISIONS.md`) | Never advertises code 68 or Cisco 131; received codes preserved as unknown; `ze exabgp migrate` warns on `multi-session` |
| [draft-ietf-idr-operational-message](short/draft-ietf-idr-operational-message.md) ([text](texts/draft-ietf-idr-operational-message.txt)) | BGP OPERATIONAL Message (`-00`) | Expired 2012-09-01 (never became an RFC) | No IANA capability code or BGP message type ever allocated; no adoption or users (`ai/META-DECISIONS.md`) | Never advertises capability or sends OPERATIONAL messages; `operational` config and bridge commands are refused |
