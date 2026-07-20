# CI validation moved from Codeberg Woodpecker to GitHub Actions

Ze is pushed to both codeberg.org (origin) and github.com/ze-software/ze. Validation
now runs on **GitHub Actions** (`.github/workflows/verify.yml` + `evidence-nightly.yml`
+ `perf-nightly.yml`); all three `.woodpecker/*.yml` were removed. The shape is pinned
by `scripts/dev/github_workflows_test.go` (7 guards).

## Why the forge, not the test, was the lever (reusable)

A whole test tier (`ze-integration-test`: six CAP_NET_ADMIN / CAP_NET_BIND_SERVICE
netns/nft suites, `mk/test-integration.mk`) could not run in CI on Codeberg. Its only
Woodpecker lever for capabilities is `privileged: true`, which is a BLOCKING linter
error on an untrusted SHARED instance and aborts the ENTIRE pipeline (the lint runs
before the `when:` match, so it kills every workflow, verify included). GitHub's
`ubuntu-latest` gives passwordless root, so `sudo -E env "PATH=$PATH" make
ze-integration-test` gets those capabilities natively. **When a CI capability wall
blocks a test tier, the runner/forge is the fix, not weakening the test.** This
resolved spec-fixit-ci-schedule-evidence AC-2, which the fuzz-only fallback could not.

## Two GitHub-vs-Woodpecker gotchas worth remembering

- **GitHub cron lives IN the workflow** (`on: schedule: - cron:`); merging to the
  default branch CREATES the schedule. Woodpecker's cron is a separate repo setting
  that nothing in the repo records, so a Woodpecker "nightly" can silently never fire.
  (GitHub caveat: scheduled workflows are disabled after 60 days of repo inactivity;
  `workflow_dispatch` is the re-arm.)
- **agent-browser after root -> non-root**: the old Woodpecker container ran as root;
  GitHub runs as `runner`. `npm install -g` needs a user-writable npm prefix, so use
  `actions/setup-node` (NOT apt `nodejs npm`, whose /usr prefix EACCESes). agent-browser
  is load-bearing: the `.wb` web suite hard-fails without it (`internal/component/web/
  testing/runner.go` execs it), failing all of `make ze-verify`.

## Meta-lesson: editing a canonical source stales its generated mirror

Editing `ai/rules/qemu-testing.md` silently staled the GENERATED `ai/rules/CONDENSED.md`
(built by `rules_condensed.py`), tripping `ze-rules-condensed-check` -> the structural
`ze-regen-check-readonly` gate -> a local `make ze-verify` red. Regenerate derived files
after editing their source. Compounding trap: that generator + `CONDENSED.md` were
ANOTHER session's UNCOMMITTED feature (absent from `HEAD:Makefile`), so the red bit only
locally, never on a clean CI checkout -- regenerate to keep the shared tree consistent,
but do NOT commit another session's untracked feature files.

Also: the test-deletion hook (`.claude/hooks/pretool-bash.py` `check_test_deletion`)
hard-blocks any agent `rm` of a `*_test.go` path (exit 2, no override); the USER must
run the deletion.
