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

Retired 2026-09-27: `SFLOW-V5-x-8` states an obligation sFlow v5 does not carry. Read §5 and the whole text: sFlow v5 writes its datagram in XDR notation and imports XDR by reference, "The format of the sFlow datagram is specified using the XDR standard [32]". The 4-byte count prefix and the zero padding are RFC 4506 §4.10 and §4.13. RFC 4506 has no summary in rfc/short, so under owner decision D-10 the row is retired, and a journal row asks for it to be enrolled. Its one tag, on TestSFlowSampledHeader in internal/plugins/flowexport/sflow/sflow_v5_flow_test.go, was removed; the comment now names RFC 4506 and the test stays.

Correction 2026-09-28: `SFLOW-V5-x-7` once claimed "4-byte alignment, big-endian" under a heading the document does not have. §5 states only "The format of the sFlow datagram is specified using the XDR standard [32]", which the row now quotes; the alignment and padding rules are RFC 4506 §4.10 and §4.13 (see the `SFLOW-V5-x-8` retirement), so no new row. Same id and level (D-3).

Correction 2026-09-28: `SFLOW-V5-x-9` once claimed per-agent and per-source sequence numbers that are "unsigned 32-bit, wrapping". §5 states no wrap; the row now quotes the datagram sequence_number definition, and the per-source counts are `SFLOW-V5-x-28` and `SFLOW-V5-x-29`. No new row; same id and level.

Correction 2026-09-28: `SFLOW-V5-x-10` once claimed flow_sample "MUST include the actual sampling_rate used by the agent". §5 defines the field as sFlowPacketSamplingRate, which the row now quotes; "the actual sampling rate" is §4.3's sentence, already `SFLOW-V5-x-24`. No new row; same id and level.

Correction 2026-09-28: `SFLOW-V5-x-12` once claimed a "2^30-1" threshold. §5 states only "If ifIndex numbers may be >= 2^24 then the expanded must be used", which the row now quotes. No new row; same id and level.

Correction 2026-09-28: `SFLOW-V5-x-14` once claimed samples "at the configured polling interval". §4.3 defines sFlowCPInterval as "The maximum number of seconds between successive samples", which the row now quotes, so the interval is a maximum. No new row; same id and level.

Correction 2026-09-28: `SFLOW-V5-x-20` once claimed a skip counter initialized in "[1, 2*N-1]". §3.1 states no range; the row now quotes the random skip-counter sentence and its Total_Packets/Total_Samples condition. No new row; same id and level.

Correction 2026-09-28: `SFLOW-V5-x-24` once claimed the actual rate "MUST be reported in each flow sample". §4.3 states the MAY adjustment and "When read, the agent must return the actual sampling rate", which the row now quotes; the flow_sample field is `SFLOW-V5-x-10`. No new row; same id and level.

Correction 2026-09-29: `SFLOW-V5-x-2` drops its `{single-polarity}` marker, which said no reject path existed. The sflow collector validator (internal/plugins/flowexport/sflow/register.go validateSFlowCollectors) now refuses an sflow collector without agent-address, an unspecified agent-address, and two sflow collectors naming two agent addresses, so the row carries a negative. Quote, id and level unchanged.

Correction 2026-09-29: `SFLOW-V5-x-2` quoted the whole sFlowAgentAddress DESCRIPTION of §4.3, four sentences with three obligations of different standing. Its tests prove only the last two sentences: the configured address stays the same while the exported interfaces change, and a manager can use it as a unique key, so the validator refuses a missing, unspecified or second agent address. The row now quotes those two sentences, "The address should be an invariant that does not change as interfaces are reconfigured", and keeps its id, level and tests. The other two sentences are rows of their own, because a row states one verbatim span. `SFLOW-V5-4.3-1` [MUST] quotes "The sFlowAgent address must provide SNMP connectivity to the agent." and is a gap: Ze runs no SNMP agent. The extraction site 4.3:8, which excluded that sentence as binding an SNMP agent role, now maps to SFLOW-V5-4.3-1, because Ze is the sFlow agent the sentence binds. `SFLOW-V5-4.3-2` [SHOULD] quotes "In the case of a multi-homed agent, this should be the loopback address of the agent." and is a gap: nothing checks that the configured address is a loopback address.
