# draft-ietf-bess-mup-safi corrections

Correction 2026-09-28: `DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-4` quoted only the first
sentence of its §3.1.3.1 paragraph and dropped the consequence the next two state:
"Otherwise the NLRI is considered as a malformed. A BGP speaker MUST handle such a
malformed NLRI as a". Same requirement id and level, the quote widened to the three
sentences.

Correction 2026-09-28: `DRAFT-IETF-BESS-MUP-SAFI-3.1.4.1-2` quoted only the first
sentence of its §3.1.4.1 paragraph and dropped the same consequence: "if a BGP speaker
receives 3gpp-5g specific BGP Type 2 ST route. Otherwise the NLRI is considered as a
malformed". Same requirement id and level, the quote widened to the three sentences.

Retired 2026-09-28: `DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-8` duplicated a span another row
states: its quote "unknown TLV types MUST be ignored for local processing" is the first
clause of `DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-9`, whose quote carries that clause and the
propagation clause after it. Read §3.1.3.1, §3.1.4.1 (the same sentence repeated for the
Type 2 ST route) and §3.1.5. Merged on the main thread's ruling. The positive tag on
`TestMUPT1STUnknownTLVIgnoredAndPropagated` moved to -9. The negative tag was deleted,
because its assertion is the TLV framing refusal of
`DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-10`. Extraction sites 3.1.3.1:9 and 3.1.4.1:6 now name -9.

Correction 2026-09-28: `DRAFT-IETF-BESS-MUP-SAFI-3.3.3-3` cited Section 3.3.6 as well,
but its quote is the Interwork Segment Discovery sentence of §3.3.3 only: "When a BGP
speaker receives the Interwork Segment Dicovery routes with a MP_REACH_NLRI attribute".
The Direct Segment Discovery sentence of §3.3.6 is now its own row,
`DRAFT-IETF-BESS-MUP-SAFI-3.3.6-5`, and the §3.3.6 nexthop mismatch sentence is
`DRAFT-IETF-BESS-MUP-SAFI-3.3.6-4`.
