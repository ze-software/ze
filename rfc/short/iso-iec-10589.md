# ISO/IEC 10589 - Information technology

## Meta

| Field | Value |
|-------|-------|
| Title | Information technology -- Telecommunications and information exchange between systems -- Intermediate System to Intermediate System intra-domain routeing information exchange protocol |
| Enrolment | source-restricted |
| Enrolment reason | The IS-IS base protocol, published by ISO/IEC and not freely redistributable. Ze implements it: internal/plugins/isis carries the adjacency FSM, DIS election, the link-state database with CSNP/PSNP flooding, SPF and the FIB install, and clause 7.3.16.4 c) own-LSP conflict handling is implemented in reoriginateAboveClaim (internal/plugins/isis/own_lsp_conflict.go). It can never be enrolled: checkEnrolment requires the standard's own text at rfc/full/iso-iec-10589.txt before an enrolment is legal, and the ISO copyright forbids putting it there. This is a DECISION rather than debt, which is what separates it from 'blocked': no fetch discharges it. The nine IS-IS RFCs that extend this base ARE extracted and enrolled -- rfc1195, rfc5301, rfc5303, rfc5304, rfc5305, rfc5308, rfc5310, rfc3787 and rfc2966 -- so what is unbounded here is the base standard's own clauses and nothing that cites them. |
| Support | drafts 70 |
| Support name | ISO/IEC 10589 |
| Support area | IS-IS base protocol |
| Support status | Experimental |
| Support coverage | Base IS-IS protocol reference, paired with the IS-IS RFC rows above. |
| Support remaining | - |

## Overview

The IS-IS base protocol: the intra-domain routeing information exchange protocol used with the connectionless-mode network service. It defines the LSP, IIH and SNP PDUs, the level-1 and level-2 hierarchy, the adjacency state machine and the SPF computation that RFC 1195 extends to carry IP reachability.

## Compliance Checklist

No requirement is extracted yet. The `Enrolment reason` above says why, and
names the fetch that unblocks it. An empty checklist gates nothing: it does not
claim conformance and it is not evidence of any.
