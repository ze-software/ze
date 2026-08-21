---
kind: note
level: MUST NOT
stage:
---
`Plugin.Run` sets `Protocol: []string{rpc.ProtocolRecordAnswers}` on the Stage 3 `declare-capabilities` for every plugin. There is no field to set and no way to opt out, so a plugin MUST NOT be written to assume the single-line command frame.
