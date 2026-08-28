---
kind: directive
level: MUST
stage:
---
- **MUST NOT loop `./le functional` / `./le verify worktree` to hunt a flake.**
  MUST use the stress reproducer against the suspected suite.
- **MUST static-clear the hypothesized site before trusting it.** MUST read the function
  that PRODUCES the crash (the reslice, the buffer allocation), not a byte-count
  inference (`ai/rules/evidence.md`). The `rsvpte-lsp` "cap-512
  share-registry" diagnosis in `plan/known-failures/` was inference from the
  5448-byte payload size and did not survive reading the producers: the send
  path is `json.Marshal` + `append` with no 512-cap buffer.
- **If it will not reproduce under stress AND the site is statically clear,**
  SHOULD suspect misattribution (the aggregator tagged another concurrent suite's crash
  to this one) or an already-landed fix, rather than "fixing" a phantom. That is
  the one case a shard MAY record, and only while you are still driving it
  (`ai/rules/completion.md`). It does not apply once you can name load as
  the cause: that is a mechanism, and it gets fixed.
- A genuine reproduction's log (`tmp/stress-repro/…`) carries the real stack:
  MUST attach it when filing or fixing the bug.
