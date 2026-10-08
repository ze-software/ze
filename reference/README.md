# Non-normative reference documents

This tree holds IETF documents that Ze developers read for context. They are
not requirements.

| Rule | Reason |
|------|--------|
| A document here MUST NOT be the source of an `rfc/short/` summary, a requirement row, or an `RFC requirement:` tag | Best-practice and operational guidance is advice to operators. It does not oblige an implementation |
| Only the owner can move a document from here into `rfc/full/` or `rfc/drafts/` | Moving it there makes it a candidate for enrolment, which adds requirements |
| A Standards Track RFC goes in `rfc/full/`, not here, even when its working group writes mostly operational documents | RFC 8212 and RFC 9774 are GROW and IDR documents that update RFC 4271, so they oblige Ze |

## Layout

`ietf/<wg>/<stem>.txt`. The file name is the document stem without a revision,
as in `rfc/full/` and `rfc/drafts/`. The IETF category is not in the name,
because it changes when a draft is published or an RFC is obsoleted.

`ietf/INDEX.tsv` is a snapshot of the IETF datatracker for each document: the
working group, the category (for a draft, the intended status, often unset), the
revision, the location, the RFC that obsoletes it, and the title. When a
document is already present in `rfc/full/` or `rfc/drafts/`, its row names that
location and this tree does not hold a second copy.

## Scope

| Working group | Contents |
|---------------|----------|
| `idr` | Every published BCP and Informational RFC, and every active draft |
| `grow` | Every published BCP and Informational RFC, and every active draft |
| `opsec` | Every published BCP and Informational RFC. The group has no active draft and no Standards Track RFC |

A BCP or Informational RFC that the owner has already reviewed stays in
`rfc/full/`, and its row in `ietf/INDEX.tsv` names that location. Its summary
records the ruling: RFC 7454 and RFC 8195 are declared non-normative, and
RFC 6996 and RFC 7999 are enrolled for the MUST-level obligations they do state.
