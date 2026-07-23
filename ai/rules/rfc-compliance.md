# RFC Compliance (every protocol, not just BGP)

**When:** writing, changing, reviewing, or testing ANY protocol-implementing code, for ANY RFC Ze implements
**Severity:** blocking

## Directives

**Ze aims to be a model of RFC compliance, for EVERY RFC it implements.** Not
just BGP: IS-IS, OSPF, BFD, LDP, RSVP-TE, IKE/IPsec, L2TP, PPPoE, DHCP, NTP,
RADIUS, TACACS+, gNMI, BMP, RPKI, VRRP -- every protocol surface is held to its
own RFCs, and so is anything Ze speaks that has a standard behind it.

You cannot write an RFC-based application and not ensure RFC compliance.
Conformance is a property of the code, checked against the RFC text, and it is
never traded away for convenience, for a green test, or for expedience.

**Conformance is not negotiable and nothing in the repo overrides the RFC: only an explicit instruction from Thomas authorises a deviation.**

When he does authorise one, record it in `plan/learned/` with the RFC section and
the reason, so the next reader finds a decision rather than a bug.

| Situation | What you MUST do |
|-----------|------------------|
| You find code that does not do what the RFC requires | Fix the code. Not later, not in a follow-up spec: a known wire-visible violation is a defect you are now the entry point for (`ai/rules/no-parking.md`) |
| A test pins the non-conformant behaviour | The TEST is wrong. A fixture, golden file, or assertion encoding a violation is not evidence the violation is intended -- it is the violation with a green bar on top. Fix the code, then correct the test and say so |
| A code comment calls the deviation deliberate | A comment is its author's belief, not a decision record (`ai/rules/no-fabrication.md`). Check the RFC text, then `plan/learned/` for a real ruling. Absent one, the RFC wins |
| The RFC requirement is not in `rfc/short/<stem>.md` | An unextracted obligation is still an obligation. Add the checklist row (see Extraction Completeness) -- the gate's silence is not conformance |
| Conforming would change behaviour operators rely on | Say so plainly and ask which way to fix it. Never silently keep the violation, and never present "leave it non-conformant" as an option |
| An exemption genuinely applies (e.g. RFC 7947 route-server transparency) | Gate it on the exact condition the exempting RFC names. An exemption applied unconditionally is a violation for every case it was not written for |

**Before claiming a protocol behaviour is correct, read the RFC text**, not only
the summary and not only the surrounding code. Cite the section you relied on.

Rationale: `ai/rationale/rfc-compliance.md`

## RFC Summaries (`rfc/short/`)

RFC summaries are protocol-only reference documents. They must NOT contain
Ze-specific information: no Ze implementation notes, no Ze file paths, no
"Ze does/does not" statements, no "for ze" sections. Implementation
decisions belong in specs (`plan/`), architecture docs (`docs/architecture/`),
or code comments. A reader should be able to use any `rfc/short/` file
as a standalone protocol reference with no knowledge of Ze.

## Extraction Completeness (BLOCKING when enrolling a summary)

`make ze-rfc-check` verifies that every requirement **listed** in a summary is
covered. It cannot know about an obligation nobody wrote down. A green gate is
bounded by what was extracted, so a missing extraction is invisible to it and to
any audit that only re-checks classifications.

Before enrolling `rfc/short/<stem>.md` in `rfc/enrolled.txt`, walk the RFC's own
text section by section and confirm every MUST / MUST NOT / SHALL / SHALL NOT /
REQUIRED has a checklist row. Fetch the source first if it is absent:
`curl -o rfc/full/rfcNNNN.txt https://www.rfc-editor.org/rfc/rfcNNNN.txt`. A
claim of "verified against the RFC" is not reproducible when `rfc/full/` lacks
the file.

Two signals that an extraction is missing, both seen in practice:

| Signal | Why it matters |
|--------|----------------|
| A `{not-applicable}` whose reason is "ze has no X producer at all" | That admission is often the violation of a separate MUST requiring X to exist. RFC 4271 §5.1.4's "MUST implement a mechanism ... that allows MULTI_EXIT_DISC to be removed" was unextracted, and two requirements cited its absence as their exemption. |
| A section whose siblings are enumerated but one clause is not | RFC 8666 §5's "MUST be ignored on reception" was omitted while §6, §7.1 and §7.2 each had it. An enumeration hole, not a style choice. |

Also verify the requirement TEXT matches the RFC. A misquoted obligation licenses
a justification that never engages it: RFC 4271 §5.1.6 binds a speaker **that
receives** a route with ATOMIC_AGGREGATE, and recording it as an aggregator rule
let the readvertisement path be cited as evidence of non-applicability when it is
the bound path.

## What Keeps RFC Testing Valid (the four ratchets)

`make ze-rfc-check` reads the WORKING TREE to judge coverage, and a tree cannot tell
"never proven" from "stopped being proven". Four comparisons against HEAD supply that
difference. Each fires only on a real downgrade, so a green run means the evidence held,
not that nobody looked.

| Ratchet | Producer | Fires when |
|---------|----------|-----------|
| Enrolment is monotonic | `check_enrolment` | an RFC whose MUSTs were gated stops being gated |
| **Proof is monotonic** | `check_coverage_ratchet` | a requirement loses a polarity it had at HEAD. `{gap}` is NOT an escape: it is the move being blocked |
| **Requirements do not vanish** | `check_retired_requirements` | a requirement id of an enrolled RFC disappears from its summary. Without this, deleting the checklist line is the CHEAPEST route from red to green, cheaper than `{gap}` which costs a public disclosure row, and the ratchet would be pressuring people to hide obligations rather than declare them. Correcting a misquote means editing the TEXT under the same id, which is allowed |
| **Adding an RFC adds checking** | `check_new_summaries` | a summary that is NEW since HEAD declares gated MUSTs and is not in `rfc/enrolled.txt`, fails to parse, or captures zero requirements while `rfc/full/<stem>.txt` has MUST-level keywords |

Summaries that predate HEAD are the existing backlog and are deliberately grandfathered:
a rule that reds the gate on unrelated work gets removed rather than obeyed. Where git
cannot answer, every ratchet judges nothing rather than judging everything.

At edit time the `rfc-tagged-test` guard (`_rfc_tagged_change_err`) blocks a behavior
change to any test carrying an `RFC requirement:` tag, and separately blocks REMOVING the
tag. Removal is checked first and on its own: a tag is a comment, so a behavior comparison
waves its deletion through, after which the test is unguarded and `// test-relax:` alone
buys any later weakening. Scope is the enclosing test function, not the edited hunk (a tag
sits on the doc comment, so a hunk-scoped guard misses exactly the edit it exists to stop)
and not the whole file (which blocked 331 of 3220 untagged helper functions).

**What none of this catches:** a tagged test whose assertions are weakened *in place*
while keeping the same shape. That is `c_test_weakening` and
`scripts/dev/audit-test-relaxation.py`, plus the SHA ratchet
(`check_audit_freshness`) wherever `/ze-rfc-audit` has recorded a verdict. The SHA ratchet
is armed only for RFCs that have an `rfc/audit/<rfc>.json`.

## Before Implementing BGP Features

1. Find RFC in `rfc/` — if missing: `curl -o rfc/full/rfcNNNN.txt https://www.rfc-editor.org/rfc/rfcNNNN.txt`
2. Read relevant sections, note MUST/SHOULD/MAY
3. Check ExaBGP reference

**Priority:** RFC > ExaBGP API compat > ExaBGP implementation

## Wire Format Documentation (MANDATORY)

Never modify protocol code without documenting wire format: ASCII diagram with field offsets, byte offset annotations, RFC section reference.

## RFC MUST Comments (BLOCKING)

Every MUST/MUST NOT enforced in code needs a comment directly above:
```
// RFC NNNN Section X.Y: "quoted requirement"
<enforcing code>
```

Document: validation rules, error conditions, state transitions, timer constraints, message ordering.

## MAY Clauses

ASK user: implement? skip? config option?

## Common RFCs

| Feature | RFC | Location |
|---------|-----|----------|
| BGP-4 base | 4271 | `internal/component/bgp/message/`, `internal/component/bgp/reactor/` |
| MP-BGP | 4760 | `internal/component/bgp/reactor/received_update.go`, `internal/core/bgp/attribute/` |
| 4-byte ASN | 6793 | `internal/core/bgp/capability/capability.go` |
| Add-Path | 7911 | `internal/core/bgp/capability/capability.go` |
| GR | 4724 | `internal/core/bgp/capability/capability.go` |
| Error handling | 7606 | revised error handling |

ExaBGP ref: `/Users/thomas/Code/github.com/exa-networks/exabgp/main/src/exabgp/`
