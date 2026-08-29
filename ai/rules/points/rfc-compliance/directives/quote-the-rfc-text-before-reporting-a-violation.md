---
kind: directive
level: MUST
stage:
---
**A claim that Ze violates an RFC MUST quote the RFC's own text before it is
recorded or acted on.** The quotation comes from `rfc/full/<stem>.txt` or
`rfc/drafts/`, is verbatim, is at least one whole sentence, and carries the
section number it was read at. A finding that cites only a requirement id, only
a `rfc/short/` line, or only its own paraphrase is UNVERIFIED and MUST be
labelled so, whatever else it carries.

**A `rfc/short/` summary is not the source.** It is a derived artifact and it is
the thing under audit, so it can be wrong in the two directions that matter: it
can state an obligation the RFC does not, and it can drop a clause that changes
the obligation's scope. A finding built on the summary alone inherits its error
and gives it the authority of a review.

This binds every place a violation is asserted: a review finding, a spec
premise, a journal row, an audit verdict, a `{gap}` or `{not-applicable}`
annotation, a report to the owner, and a message to another session. It binds
the reader too: a violation claim arriving without its quotation MUST be
re-derived from the RFC text before any work is commissioned on it.

**The failure this prevents is fabrication, not sloppiness.** A requirement id
and a section number are cheap to produce from memory and read as evidence, so
an id that names no such clause, a section number off by one, and a MUST
remembered as stricter than it is are all invisible at review time. Opening the
text costs one read and is the only thing that separates a finding from a
recollection.
