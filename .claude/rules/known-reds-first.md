# Fix known reds by targeted reruns; the full verify is the final check

Owner decision, 2026-10-17: "as we are fixing more issues, we should stop the
global test and focus only on what we know fails until it all pass ... then
rerun the full test ... make sure it is our approach to testing always".

| When | Do |
|------|----|
| A run (full verify, CI, nightly) leaves reds | Write them into one list of known reds. Do not start another full verify |
| Fixing | Rerun only the failing test, suite or stage, alone and under the load that broke it, until it passes. Each fix is reviewed and committed on its own |
| Every listed red passes its targeted rerun | Run the full verify once, on a quiet machine with no agent running tests beside it |
| That full verify leaves new reds | They become the new list; repeat |

A full verify while known reds are still open, or with agents loading the
machine, spends hours re-measuring what is already known.
