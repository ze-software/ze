# rfc-clause-map-needs-producers

Two BGP MUST-level requirements shipped as violations for months while tests for
those exact clauses passed. Both would have survived a clause-to-test coverage
map, because the tests proved the clause on a *different producer* than the one
that violated it.

| Clause | Violation | Fixed |
|---|---|---|
| RFC 6996 §4 | A private ASN reached an eBGP peer with the strip filter configured | `1bf31e316` |
| RFC 6793 §4.2.2 | AS4_PATH never emitted; AS_TRANS substituted with no way to recover the real ASN, on the normal forward path | `fb3e6f20b` |

## The measurement

At `origin/main`, for RFC 6793 §4.2.2:

- `aspath_transcode_test.go` -- **43** AS4_PATH references
- `aspath_rewrite_test.go` -- **0**

`internal/component/bgp/wireu/` has two producers of the same clause. Transcode
implemented it and was tested to death. Rewrite, reached from the normal eBGP
forward path (`received_update.go:138`), never emitted AS4_PATH at all. A map
built from "which clauses have tests" marks §4.2.2 covered, with 43 citations to
back it up.

RFC 6996 §4 is the same shape one level up: `rewritePrivateASSegments`
(`filter_delta.go:645-669`) implements the clause correctly and is tested.
Nothing tested that the filter was *applied* on the originated egress path,
where `exportFilterForBody` never asked it the question. The clause was tested;
the path was not.

## Why the gate cannot see it

`check_coverage` (`scripts/dev/rfc_requirements.py:570-641`) decides satisfaction
purely from `by_rid` -- the set of `RFC requirement:` tags found in test files.
One positive plus one negative tag *anywhere* satisfies a MUST. Nothing binds a
clause to the production function obliged to satisfy it.

`scan` (`rfc_requirements.py:544-556`) walks the tree reading only `_test.go` and
`.ci`. Production code is never opened.

So tagging `aspath_transcode_test.go` positive+negative for RFC6793-4.2.2 turns
the gate green permanently while `aspath_rewrite.go` violates the same clause.
This is not a hypothetical: it is what the tree looked like this morning.

## The rule

**The unit of coverage is `clause -> producers -> tests per producer`, never
`clause -> tests`.** A clause whose producers are unequally covered is a gap, not
a pass. A single-producer assumption is the fail-open: the map reports "covered"
when it checked one of two.

## The cheap mechanism

Production code already self-declares its obligations. At `origin/main`:

- 870 non-test Go files carry an `// RFC NNNN` comment
- spanning 250 distinct RFCs -- denser than the 174 summaries in `rfc/short/`
- 34 production files cited RFC 6793

`aspath_rewrite.go` names the clause it was violating, in a comment, three times:

```
aspath_rewrite.go:23   // RFC 6793 Section 4: When advertising to non-ASN4 peers...
aspath_rewrite.go:339  // --- AGGREGATOR transcoding (RFC 6793 Section 4.2.2) ---
aspath_rewrite.go:404  // RFC 6793 Section 4.2.2: "set the AS number field in the...
```

The evidence was sitting in a comment inside the violating file. The gate did not
open the file.

Proposed: for an enrolled RFC, every production file citing that RFC must be
reached by at least one test tagged with a clause of that RFC. A citing file no
tagged test covers is an **unproven producer** and belongs on the gap list. This
costs one extra file-kind in the existing `os.walk`, plus coverage data to tie a
tagged test to the functions it reaches. It needs no new annotation from authors:
the comments are already written.

## Honest limits

- It finds asymmetry between **declared** producers only. A producer citing no RFC
  stays invisible. That is strictly better than today (zero producers considered)
  and must be stated as a `{gap}`, not sold as completeness.
- It will surface files citing an RFC as commentary rather than implementation
  (`internal/analyze/bgpconn.go` may be one). Those need an annotation meaning
  "cites for context, implements nothing" -- otherwise the noise trains people to
  dismiss the signal, which is how a ratchet dies.

## Traps for the next agent

- **A high test count on a clause is evidence of attention, not of coverage.** 43
  tests is exactly the number that stops you looking. The instinct to spot-check
  the clauses with *zero* tests first is backwards: a zero is honest, and a
  confident non-zero on a multi-producer clause is the one that ships bugs.
- **Ask "how many producers?" before "how many tests?"** For any clause, grep
  production code for the RFC number before reading the test file. If two files
  implement it and one test file covers it, that asymmetry is the finding.
- **This generalizes past RFCs.** Any coverage artifact keyed on the obligation
  rather than the obliged code has this hole: a doctor check per dependency, a
  validator per config key, a guard per entry point. Where an obligation has more
  than one implementation site, coverage of the obligation is not coverage of the
  sites. See [[1010-verify-producer-before-claiming]] -- same disease, one level
  up: there the claim skipped the producer, here the *gate* does.

## Files

None recorded.
