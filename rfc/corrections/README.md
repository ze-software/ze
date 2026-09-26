# RFC correction records

One file per document, `rfc/corrections/<stem>.md`, holding the paragraphs that
say why a requirement row of `rfc/short/<stem>.md` changed level, text, or
citation, or was retired. A summary carries what the RFC obliges, so it is the working reference
an implementer opens; the history of what this repository once got wrong lives
here instead.

## Why the record has to exist

`checkLevelRatchet` (`internal/le/rfc/check_ratchets.go`) refuses a row that
leaves the gated MUST-level population (`MUST`, `MUST NOT`, `SHALL`,
`SHALL NOT`, `REQUIRED`) with nothing recorded. Gating is monotonic: the row
keeps its id and its tests, so no other ratchet sees the loss, while every
coverage obligation attached to the row disappears. `loadCorrections` reads this
file, and `correctionAuthorizes` checks the quote against the RFC's own text in
`rfc/full/<stem>.txt` or `rfc/drafts/<stem>.txt`.

Raising a level to a gated one needs no record. The gate never charges for a
conformance improvement.

## Paragraph format

| Part | Requirement |
|------|-------------|
| Opener | `Correction <YYYY-MM-DD>:` on the first line of the paragraph. A leading `>` is allowed |
| The row | The requirement id in backticks. A paragraph naming a neighbour does not authorize this row |
| The proof | At least 24 characters in double quotes, appearing verbatim in the RFC's own text. Line wrapping is ignored; the words are compared |

A paragraph is a run of lines with no blank line in it, so a blank line separates
one correction from the next.

```
Correction 2026-08-15: `RFC7296-2.8-1` was extracted at MUST strength. §2.8.1 states the
collision rule as a recommendation: the redundant SA "SHOULD be closed by the endpoint
that created it". Same requirement id, corrected text and level.
```

A correction is for a row that misquoted the RFC. When the document really does
say MUST, restore the level instead, and read `ai/rules/rfc-compliance.md`,
"Implement Full Compliance", before you write anything that lowers what Ze owes.

## Retiring a row

A row that no sentence of the RFC states is retired rather than kept
(owner decision D-2, 2026-09-26). `checkRetiredRequirements` refuses an id of an
enrolled RFC that `HEAD^` held and the summary no longer carries, unless a
retirement paragraph in this file names it. `retiredIDs` reads every record, and
an id counts only from the record of its own stem.

Retire only after reading the whole RFC for the obligation, not just the section
the row cites. When another document states it, that is a re-attribution, not a
retirement. Before the row goes, move each of its tags to the row that does state
the obligation, or delete a tag whose only claim was this row, and drop the id
from `rfc/extraction/<stem>.json` and `rfc/audit/<stem>.json`. The gates refuse a
tag, a mapping or a verdict that names a missing id.

| Part | Requirement |
|------|-------------|
| Opener | `Retired <YYYY-MM-DD>:` on the first line of the paragraph. A leading `>` is allowed |
| The row | The requirement id in backticks. A paragraph naming a neighbour does not retire this row |
| The search | At least one section reference written `§<n>`, naming the sections read. No quote is asked for, because the row is retired for having no sentence to quote |
| The moves | Where each tag went, in words. No gate reads this part, so the reviewer does |

```
Retired 2026-09-26: `RFC9999-2-3` states no obligation RFC 9999 carries. Read §2 and
§3, then the whole text: no sentence says a speaker counts widgets. Its one tag moved
to `RFC9999-2-1`, which states the obligation the test proves.
```

The two kinds never stand in for each other. A `Correction` paragraph does not
retire a row, and a `Retired` paragraph does not authorize a level change, even
when it quotes the RFC. A retired id is never allocated again: `checkIDAllocation`
refuses any row that carries an id a retirement paragraph names, in the commit
that retires it and in every commit after. So a retirement paragraph is
permanent; deleting it would free the id.
