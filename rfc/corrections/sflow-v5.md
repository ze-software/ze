# sFlow Version 5 corrections

Why a requirement row of `rfc/short/sflow-v5.md` changed level, text, or citation.
The summary is the working document; this file is the record the level
ratchet reads (`checkLevelRatchet`, `internal/le/rfc/check_ratchets.go`).

Correction 2026-09-26: `SFLOW-V5-x-2` was extracted at MUST strength and cited "Agent
Architecture", a heading the document does not have. The obligation is in the description
of sFlowAgentAddress in §4.3, and the document states the loopback choice and the stable,
unique address as recommendations: "this should be the loopback address of the agent", and
"The address should be an invariant that does not change as interfaces are reconfigured".
The only "must" in that description is SNMP connectivity, which the row never claimed.
Same requirement id, corrected text, level and citation.

Correction 2026-09-26: `SFLOW-V5-x-16` was extracted at MUST strength and cited
"Implementation Guidance", a heading the document does not have. §5 introduces the
maximum-value sentinel for an unavailable counter in a list governed by a recommendation:
"The following values should be used for fields that are unknown (unless otherwise
indicated in the structure definitions)." The list item itself carries no keyword. Same
requirement id, corrected text, level and citation. The MUST that sits beside it, that a
counter is always available or always unavailable within a session, is SFLOW-V5-x-32.

Retired 2026-09-27: `SFLOW-V5-x-8` states an obligation sFlow v5 does not carry. Read §5 and the whole text: sFlow v5 writes its datagram in XDR notation and imports XDR by reference, "The format of the sFlow datagram is specified using the XDR standard [32]". The 4-byte count prefix and the zero padding are RFC 4506 §4.10 and §4.13. RFC 4506 has no summary in rfc/short, so under owner decision D-10 the row is retired, and a journal row asks for it to be enrolled. Its one tag, on TestSFlowSampledHeader in internal/plugins/flowexport/sflow/flow_test.go, was removed; the comment now names RFC 4506 and the test stays.
