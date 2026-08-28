---
kind: note
level:
stage:
---
Both setcap a **throwaway** binary, run under `sudo` with a per-test network
namespace, assert the host's kernel state is byte-identical before and after,
and exit non-zero (never skip) when Linux, `sudo`, or `setcap` is missing.
Details: `docs/functional-tests.md` "Netns launch mode".
