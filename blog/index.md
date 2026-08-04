# The Ze blog

Occasional articles on Ze: design notes, deep dives, and talk write-ups. For what shipped week by week, see the [changelog](../changes/).

- [How Ze keeps BGP traffic away from the garbage collector](how-ze-manages-memory/index.md) (2026-08-04): How Ze reuses buffers, borrows wire data and limits copies so repeated routing work creates little garbage.
- [One BGP UPDATE, many peers](one-bgp-update-many-peers/index.md) (2026-08-04): How Ze completes each peer's policy decision before encoding its UPDATE, then reuses an earlier encoding when the complete result matches.
