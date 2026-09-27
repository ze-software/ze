# draft-ietf-sidrops-8210bis corrections

Why a requirement row of `rfc/short/draft-ietf-sidrops-8210bis.md` changed level,
text, or citation, or was retired. The summary is the working document; this file
is the record the ratchets read (`internal/le/rfc/check_ratchets.go`).

The rows retired below were copied from the historical RFC 9582 ledger, which
was written against an earlier revision of this draft whose ASPA PDU carried an
AFI. Each was read against the whole of `rfc/drafts/draft-ietf-sidrops-8210bis.txt`
(§1 to §16), against draft-ietf-sidrops-aspa-verification and
against draft-ietf-sidrops-aspa-profile revision 29. No test tagged any of them,
so no tag moved.

Retired 2026-09-27: `DRAFT-IETF-SIDROPS-8210BIS-5.12-6` states no obligation this
draft carries. Read §5, §5.12, §7, §10 and §11.2.3, then the whole text: the ASPA
PDU of Figure 11 has no AFI field, and no sentence tells a router to ignore a PDU
for an unknown AFI. Neither ASPA companion draft states it either.

Retired 2026-09-27: `DRAFT-IETF-SIDROPS-8210BIS-5.12-9` states no obligation this
draft carries. Read §5, §5.12, §7, §10 and §11.2.3, then the whole text: there is
no AFI field for a router to ignore. draft-ietf-sidrops-aspa-verification §6.2
scopes path verification to IPv4 and IPv6 unicast, which is a different rule and
the opposite of applying ASPA to all address families.

Retired 2026-09-27: `DRAFT-IETF-SIDROPS-8210BIS-5.12-8` states nothing beyond
another row of this summary. Read §5.12 and §11.2.3, then the whole text: the
ordering it claims is §5.12, "There are zero or more 32-bit Provider Autonomous
System Number fields in increasing numeric order.", which row
DRAFT-IETF-SIDROPS-8210BIS-5.12-3 quotes with its uniqueness sentence. No sentence
asks a router to verify the order, so the router SHOULD it claimed has no source.
It had no tags.

Retired 2026-09-27: `DRAFT-IETF-SIDROPS-8210BIS-5.12-2` states an obligation this draft does not carry. Read §5.12, the ASPA PDU, then the whole text: it requires unique, ascending provider numbers but never forbids the customer among its providers. draft-ietf-sidrops-aspa-profile §3.3 states it: "The customerASID value MUST NOT appear in any PAS in the providers field." That draft has no summary in rfc/short, so under owner decision D-10 the row is retired, and a journal row asks for it to be enrolled. The row carried no tag.

Retired 2026-09-27: `DRAFT-IETF-SIDROPS-8210BIS-5.12-7` states an obligation this draft does not carry. Read §5.12 and the whole text: no sentence reserves customer AS 0. draft-ietf-sidrops-aspa-profile §3 and §3.2 state it, through "CAS ::= INTEGER (1..4294967295)" and "The customerASID field contains a positive integer". That draft has no summary in rfc/short, so under owner decision D-10 the row is retired, and a journal row asks for it to be enrolled. The row carried no tag.
