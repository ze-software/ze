# Why a protocol implementation cites its source inline

Training knowledge of RFCs is unreliable. Drafts change between versions,
details get conflated across similar RFCs, and wire format specifics (field
offsets, PDU sizes, flag positions) are frequently wrong from memory. The
summaries in `rfc/short/` are the verified source.

An agent has no long-term memory across sessions. When it reads code, an inline
reference to the external spec, API, or upstream project carries the constraint
the code was written against. Without one, every session rediscovers what the
code is implementing, and some of them guess.

Rule: `ai/rules/protocol.md`. The exact-or-reject half has its own record in
`ai/rationale/exact-or-reject.md`.
