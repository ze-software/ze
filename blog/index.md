# The Ze blog

Design notes, deep dives, and engineering essays on how Ze is built. For week-by-week shipping notes, read the [changelog](../project/changes/).

- [Reference stays attached to code](reference-from-the-system/index.md) (2026-08-22): How Ze builds its command reference from the binary, and why generated pages still need human judgement.
- [AI coding has not had its Rails moment](ai-coding-has-not-had-its-rails-moment/index.md) (2026-08-10): Rails gave developers and tools a shared project layout. AI coding needs a comparable convention for finding the decisions and evidence that each project must supply for itself.
- [The repository is half the AI harness](the-repository-is-the-ai-harness/index.md) (2026-08-09): A plugin change shows how Ze routes an AI agent from a task index to a rule, an implementation and a rejecting check, and what happens when those checks are wrong.
- [The proof is the expensive part](the-proof-is-the-expensive-part/index.md) (2026-08-06): Following one malformed BGP attribute from the RFC through Ze's tests to its public evidence shows where the proof stops and what still needs human judgement.
- [How Ze reuses memory for BGP UPDATEs](how-ze-manages-memory/index.md) (2026-08-04): Following one BGP UPDATE through receive-buffer reuse, borrowed views, long-lived attributes and independently owned peer output.
- [One BGP UPDATE, many peers](one-bgp-update-many-peers/index.md) (2026-08-04): How Ze finishes each peer's policy decision before reusing a matching UPDATE-body rebuild, with separate ownership for each rebuilt send.
- [AI slop is the wrong test](ai-slop-is-the-wrong-test/index.md) (2026-08-03): Ze is an AI-written network operating system. I remain responsible for what it ships, including the mistakes a model makes and the limits of the tests meant to catch them.
