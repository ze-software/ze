---
kind: directive
level:
stage:
---
- **Never loop `make ze-functional-test` / `make ze-verify` to hunt a flake.**
  Use the stress reproducer against the suspected suite.
- **Static-clear the hypothesized site before trusting it.** Read the function
  that PRODUCES the crash (the reslice, the buffer allocation), not a byte-count
  inference (`ai/rules/evidence.md`). The `rsvpte-lsp` "cap-512
  share-registry" diagnosis in `plan/known-failures/` was inference from the
  5448-byte payload size and did not survive reading the producers: the send
  path is `json.Marshal` + `append` with no 512-cap buffer.
- **If it will not reproduce under stress AND the site is statically clear,**
  suspect misattribution (the aggregator tagged another concurrent suite's crash
  to this one) or an already-landed fix, rather than "fixing" a phantom. That is
  the one case a shard may record, and only while you are still driving it
  (`ai/rules/completion.md`). It does not apply once you can name load as
  the cause: that is a mechanism, and it gets fixed.
- A genuine reproduction's log (`tmp/stress-repro/…`) carries the real stack:
  attach it when filing or fixing the bug.
