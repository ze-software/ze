---
kind: directive
level: MUST
stage:
---
**A summary whose forward Meta row names a successor MUST carry a
`{superseded: ...}` marker on EVERY requirement line it declares.**
`check_superseded` (`internal/le/rfc/rfc.go`) refuses the summary
otherwise. The row used to be prose nobody read. Seven summaries declare
themselves obsoleted, six of them enrolled, and the gate treated all seven as
current documents. A reader who opened one of their 230 requirement lines saw a
MUST with no sign that the document stating it had been replaced.

**The label MUST be `Obsoleted by` or `Obsoleted-by`, in either capitalisation,
and any OTHER Meta field whose name CONTAINS `obsolet` MUST red the gate rather
than be skipped.** That word is the whole of what the reader recognises: a field
named `Superseded by`, `Replaced by` or `Successor` is skipped in silence today,
and widening the word list is separate work because `_META_FIELD_RE` reads the
first cell of EVERY table row, so a looser word collides with the requirement
tables themselves. No summary uses such a spelling today, so the gap is
prospective; it is stated here rather than left for the next reader to discover
the way the hyphen was.
`parse_successor_stem` reads all four spellings and refuses a fifth,
because reading one of them is how this failed the first time: the corpus's
MAJORITY spelling is the hyphenated one, 28 rows against 18, and a reader that
knew only `Obsoleted by` gave 93 requirements of three enrolled summaries no
obligation at all. A qualifier after the label is kept, which is how
`rfc/short/rfc1334.md` writes `| Obsoleted-by (partial) |` for a document whose
CHAP half moved to RFC 1994 and whose PAP half did not. A reader that SKIPS what
it does not recognise cannot be trusted to have found anything.

**The marker states where the obligation NOW LIVES. It MUST NOT be read, or
written, as saying Ze owes less.** It is a fact about the DOCUMENT. So it
composes with `{gap}`, `{not-applicable}` and `{single-polarity}` rather than
replacing one. A marked requirement stays gated, stays counted in
`ai/RFC-REQUIREMENTS.md`, and stays judged by every ratchet.

| Disposition | Says | Precondition the gate checks |
|-------------|------|------------------------------|
| `restated <ID>; why` | the successor states the same obligation, under that id | the successor's summary is in `rfc/short/` AND declares that id |
| `dropped; why` | the successor states no equivalent obligation | the successor's own text is in `rfc/full/` or `rfc/drafts/` |
| `unextracted <§section>; why` | the successor STATES it, at that section, and its summary declares no row | the successor's own text is in `rfc/full/` or `rfc/drafts/` |
| `unresolved; why` | the successor's text is not in this repository | that text is ABSENT |

**A `dropped` obligation is still owed for as long as Ze speaks the wire format
the obsoleted document defines.** RFC 3768 is the VRRPv2 format keepalived speaks
by default. RFC 9568 removing an obligation says what VRRPv3 requires. It says
nothing about what a VRRPv2 speaker owes on the wire.

**The last two dispositions are DEBT, and the ledger publishes them as debt.**
Draining either one is separable work with its own spec. An `unresolved` line is
drained when somebody fetches and summarises the successor. An `unextracted` line
is drained by an extraction pass over the successor's summary. Marking a line
MUST NOT be treated as closing it.
