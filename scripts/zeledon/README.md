# Zeledon weekly-update tooling

Zeledon is the voice that posts Ze's weekly updates to Discord `ze-news`. This
directory holds the tooling for that; the workflow itself is documented in the
`ze-weekly-update` skill (`ai/skills/ze-weekly-update.md`).

- **`STYLE.md`** — how to write as Zeledon. Read before drafting any update.
- **`post_weekly.py`** — posts an approved `gh-pages/changes/posts/<start>.md`
  to Discord, splitting at section boundaries under Discord's 2000-char limit,
  and archives the exact text sent. Dry-runs by default; `--yes` actually posts.
  See its module docstring for the full CLI.
- **`weekly/`** — local archive of posted updates, one file per week. Gitignored:
  the authoritative copy of the text is the website (`gh-pages/changes/posts/`,
  committed and published), so this is kept only so `post_weekly.py` can skip
  weeks it has already posted.

`post_weekly.py` shells out to `discord.sh`, which carries the bot token and so
lives in the private `~/Unix/bin/` rather than this public repo. Override its
path with the `DISCORD_SH` environment variable.
