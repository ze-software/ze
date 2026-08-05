# The Ze blog

Occasional articles on Ze: design notes, deep dives, and talk write-ups. For what shipped week by week, see the [changelog](../changes/).

- [The proof is the expensive part](the-proof-is-the-expensive-part/index.md) (2026-08-06): Ze uses AI to write code, but an RFC claim only counts after the standard, the tests, the public gap list and the commit process all agree.
- [How Ze keeps BGP traffic away from the garbage collector](how-ze-manages-memory/index.md) (2026-08-04): How Ze reuses buffers, borrows wire data and limits copies so repeated routing work creates little garbage.
- [One BGP UPDATE, many peers](one-bgp-update-many-peers/index.md) (2026-08-04): How Ze completes each peer's policy decision before encoding its UPDATE, then reuses an earlier encoding when the complete result matches.
- [AI slop is the wrong test](ai-slop-is-the-wrong-test/index.md) (2026-08-03): Ze is an AI-written NOS. The useful question is whether the code is constrained, reviewed, tested and measured.
