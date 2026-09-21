# RFC correction records

One file per document, `rfc/corrections/<stem>.md`, holding the paragraphs that
say why a requirement row of `rfc/short/<stem>.md` changed level, text, or
citation. A summary carries what the RFC obliges, so it is the working reference
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
