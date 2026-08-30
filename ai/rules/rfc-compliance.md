# RFC Compliance (every protocol, not just BGP)

**When:** writing, changing, reviewing, or testing ANY protocol-implementing code, for ANY RFC Ze implements
**Severity:** blocking

## Directives

**Every protocol surface MUST be held to its own RFCs, and so MUST anything Ze speaks that has a standard behind it.** Not just BGP: IS-IS, OSPF, BFD, LDP, RSVP-TE, IKE and IPsec, L2TP, PPPoE, DHCP, NTP, RADIUS, TACACS+, gNMI, BMP, RPKI and VRRP.
**Every MUST and MUST NOT enforced in code MUST carry a comment directly above it naming the RFC section and quoting the requirement (`// RFC NNNN Section X.Y: "quoted requirement"`), covering whichever of the validation rules, error conditions, state transitions, timer constraints and message ordering the code enforces.** Protocol code MUST NOT be changed without documenting the wire format: an ASCII diagram with field offsets, byte offset annotations, and the RFC section reference.
**A MAY clause MUST be put to the user: implement it, skip it, or make it a config option.** You are not authorized to pick.

**Conformance is not negotiable and nothing in the repo overrides the RFC: a deviation MUST NOT be made without an explicit instruction from Thomas, given in answer to the question the next section requires you to put to him.** It MUST NOT be traded away for convenience, for a green test, or for expedience. A test, a golden file, a code comment or an `rfc/audit` verdict that pins non-conformant behavior is the violation with a green bar on top, so fix the code and then correct the test. When Thomas authorizes a deviation, you MUST record it as a row in `plan/journal/<class>.md` carrying the RFC section and the reason.

**Before you claim a protocol behavior is correct, or report that Ze violates an RFC, you MUST read the RFC's own text in `rfc/full/<stem>.txt` or `rfc/drafts/` and quote at least one whole sentence with the section number you read it at.** A `rfc/short/` summary is a derived artifact and never the authority, so a finding that cites only a requirement id, only a summary line, or only its own paraphrase is UNVERIFIED and MUST be labelled so. Fetch a missing text first: `curl -o rfc/full/rfcNNNN.txt https://www.rfc-editor.org/rfc/rfcNNNN.txt`.
**The RFC MUST outrank ExaBGP API compatibility, which MUST outrank the ExaBGP implementation.** An RFC the current one OBSOLETES MUST NOT be read as evidence about what Ze owes; the lineage that matters runs FORWARD, through the documents that UPDATE the current one and its errata. The enrolment walk, the extraction sign-off, the superseded marker and the eight ratchets are in `docs/contributing/rfc-conformance-gates.md`.

## Implement Full Compliance. Ask Thomas Only Before Doing LESS (owner directive, 2026-07-27, clarified 2026-08-01)

**When "implement the RFC fully and prove it fully with tests" is one of the answers on the table, that IS the answer. It MUST be implemented and proven. Thomas has already chosen, so there is nothing to put to him.**

**Asking MUST happen only when you are about to do LESS.** Making Ze more conformant, or better proven, never needs permission: it MUST be done, then reported. The gate exists in one direction only. You are about to do less when you classify a requirement `{gap}`, `{not-applicable}` or `partial`, leave a MUST implemented but unproven by a tagged test, leave one unextracted, defer one to a follow-up spec or a deferral row, lower its level in `rfc/short/`, or close a spec whose RFC rows are anything other than implemented-and-proven.
**The question MUST be "which way do I fix it", and MUST NOT be "MAY I do less".** Quote the requirement id and the RFC section text verbatim, name the producing function, state what full implementation plus a tagged test would cost, and never offer "leave it non-conformant" as an option. Full compliance MUST NOT be placed beside a narrower option for Thomas to pick between.
**Every earlier answer that pointed away from full compliance or full proof is VOID (Thomas, 2026-07-27), wherever it hides: a `plan/learned/` deviation record, a spec `Deviations` row, a `{gap}` in `rfc/short/` or `docs/features/rfc-status.md`, a deferral marked `user-approved-drop`, or a comment calling the deviation deliberate.** A void answer MUST NOT be cited as authority. Finding one while doing something else is not permission to move on: raise it, and record the fresh answer where the stale one lived.
