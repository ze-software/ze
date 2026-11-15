# The Ze blog

Design notes, deep dives, and engineering essays on how Ze is built. For week-by-week shipping notes, read the [changelog](../project/changes/).

- [Keeping documentation in step with the code](reference-from-the-system/index.md) (2026-08-22): Ze generates public reference from its code and records, with links to the decisions behind them. Operators and future AI sessions can use the same maintained account of the system.
- [AI coding has not had its Rails moment](ai-coding-has-not-had-its-rails-moment/index.md) (2026-08-10): AI coding needs shared conventions for connecting requirements and design decisions to their implementation, and for keeping those relationships current.
- [The repository is half the AI harness](the-repository-is-the-ai-harness/index.md) (2026-08-09): Ze's development rules define what an AI agent must achieve and how to check the result, including the qualities a working demonstration can leave untested.
- [The proof is the expensive part](the-proof-is-the-expensive-part/index.md) (2026-08-06): Ze aims to meet every applicable mandatory RFC requirement. Interoperability labs and tests tied to the RFC text help establish what the running router does and which claims remain unverified.
- [How Ze reuses memory for BGP UPDATEs](how-ze-manages-memory/index.md) (2026-08-04): From the Amiga demo scene to BGP UPDATEs, earlier experience informs how Ze reuses memory and avoids unnecessary allocation in Go.
- [AI slop is human slop](ai-slop-is-the-wrong-test/index.md) (2026-08-03): AI often writes better code than I would, and far more tests per feature. I judge that code by what it does and the evidence behind it, and remain responsible for what Ze releases.
