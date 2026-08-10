| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-08-09 | problem-journal | doc gates | 82 of 1611 source anchors named a symbol absent from the file they cite, all green | spec to verify the symbol list each anchor already carries |
| 2026-08-10 | doc-claims-are-checked-not-just-resolved | doc gates | `claim_is_declared` in `scripts/dev/code_to_docs.py` resolves a dotted claim by its member alone, so an anchor claiming `Peer.Shutdown` passes against `func (s *Session) Shutdown()`: the reference resolves and the receiver is unchecked | not fixed, needs receiver-aware resolution and its own measurement |
