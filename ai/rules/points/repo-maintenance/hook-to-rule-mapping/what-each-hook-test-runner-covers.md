---
kind: table
level:
stage:
---
| Runner | Covers |
|---|---|
| `scripts/dev/hook-parity-check.py` | Golden exit-code regression for the three consolidated dispatchers. `--bless` regenerates the golden; re-bless only intentionally changed cases. Fixture dirs live under `~/.cache` (a `/tmp` or in-repo path trips `system-tmp`/`throwaway-tests` or the module lint and diverges from the golden). |
| `scripts/dev/hook-fixture-check.py` | Behaviour the golden table cannot isolate: `c_format_alloc`, `validate-spec.sh`, the `commit_helper.py` commit-time gates over git-initialized fixtures, and the 35 `delegation` fixtures. Those 35 pin what no other test reaches. The `Stop` array registration and its order. BOTH ends of the claim lifetime: alive past turn one, released at `SessionEnd`, kept across a resume. The two stop-phrase tiers. The markup filter that must fail toward scanning MORE. Deleting the release line once left the whole suite green. Sections selectable with `--only`. |
