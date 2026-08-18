---
kind: table
level: MUST
stage:
---
| Guard | Refuses |
|-------|---------|
| `check_summary_disposition` | a summary in `rfc/short/` that is in neither `rfc/enrolled.txt` nor `rfc/not-enrolled.txt`. Also a stem in BOTH, a disposition naming a summary that does not exist, and a disposition deleted while the stem never reaches `rfc/enrolled.txt`. Also a `non-normative` reason that judges what ZE owes rather than what the DOCUMENT states. Every summary needs a recorded disposition that distinguishes "the RFC imposes nothing", "nobody extracted it", and "we do not have the text" |
| `check_unproven_support` | a support claim over a summary that declares ZERO gated requirements. A claim is any Status other than `Unsupported` or `Future`, an empty cell included. Two ledgers agreeing on NOTHING is not conformance. It is the cheapest way to look green. Two escapes exist and both are evidence rather than assertion. One is a `non-normative` disposition whose reason states a property of the text. The other is a VALID `manual-walk` extraction sign-off carrying a `register-reason`. The second lets an Informational RFC that invokes RFC 2119 nowhere enrol on an honest zero, and it needs no fabricated MUST |
| `check_gap_count_agreement` | a Remaining cell whose spelled number, sitting immediately before MUST or SHALL, disagrees with the real `{gap}` count. The COUNT is the only fact on that page a machine can own. It says how many annotations exist. It never says their classifications are right, which matters because the paragraph above VOIDS every annotation as authority. A Remaining PROSE derived from those classifications would launder a void judgement into generated text |
