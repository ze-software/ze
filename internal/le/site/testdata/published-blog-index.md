# The Ze blog

Design notes, deep dives, and engineering essays on how Ze is built. For week-by-week shipping notes, read the [changelog](../project/changes/).

- [Reference stays attached to code](reference-from-the-system/index.md) (2026-08-22): Ze's command and configuration declarations also feed its reference pages. That leaves the writing for the reasons behind the design, with a route back to the code when either needs to change.
- [AI coding has not had its Rails moment](ai-coding-has-not-had-its-rails-moment/index.md) (2026-08-10): Teaching Claude where to keep designs and how to find them changed the code it produced. Ze's repository became a meta-system for programming according to the project's own decisions.
- [The repository is half the AI harness](the-repository-is-the-ai-harness/index.md) (2026-08-09): Ze's repository teaches an arriving agent how to use earlier project decisions. Its guidance and checks help with the next change, provided somebody also looks after the rules.
- [The proof is the expensive part](the-proof-is-the-expensive-part/index.md) (2026-08-06): Claude can produce a routing feature quickly. My network change-control instincts make me spend much longer establishing what it must do, and whether anything has demonstrated that it does it.
- [How Ze reuses memory for BGP UPDATEs](how-ze-manages-memory/index.md) (2026-08-04): Moving ExaBGP's ideas to Go would still leave plenty of unnecessary copying. How I chose the representations and lifetimes for BGP data in Ze.
- [One BGP UPDATE, many peers](one-bgp-update-many-peers/index.md) (2026-08-04): Ze's new forwarding path could rebuild the same UPDATE a hundred times. Why I chose to compare completed peer decisions, reuse the rebuild and keep the final copy.
- [AI slop is the wrong test](ai-slop-is-the-wrong-test/index.md) (2026-08-03): AI often writes better code than I would, and considerably more tests per feature. I have little patience for calling it all slop when human-written software has given us so much to complain about.
