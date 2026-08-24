# Zeledon weekly-update tooling

Zeledon is the voice that posts Ze's weekly updates to Discord `ze-news`. This
directory holds the tooling for that; the workflow itself is documented in the
`ze-weekly-update` skill (`ai/skills/ze-weekly-update.md`).

- **`STYLE.md`** — how to write as Zeledon. Read before drafting any update.
- **`post_weekly.py`** — posts an approved `website/changes/posts/<start>.md`
  to Discord, splitting at section boundaries under Discord's 2000-char limit,
  and archives the exact text sent. Dry-runs by default; `--yes` actually posts.
  A rate-limited message is retried; any other failure stops the post and prints
  the `--resume-from` number that finishes it, since the messages before it are
  already in the channel. See its module docstring for the full CLI.
- **`post_weekly_test.py`** — covers the retry, the resume and the `discord.sh`
  lookup. Run by `make ze-unit-test` through `scripts/dev/python_tests_test.go`.
- **`weekly/`** — local archive of posted updates, one file per week. Gitignored:
  the authoritative copy of the text is the website
  (`website/changes/posts/`, committed and published), so this is kept only so
  `post_weekly.py` can skip weeks it has already posted.

`post_weekly.py` shells out to `discord.sh`, which carries the bot token and so
lives outside this public repo. Unless `DISCORD_SH` is set, `find_discord_sh`
checks `~/Unix/bin/discord.sh`, then `~/bin/discord.sh`, then `PATH`.
