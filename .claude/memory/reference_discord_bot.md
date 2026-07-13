---
name: discord-bot-posting
description: "How to post to Discord via discord.sh bot mode - channels, token, usage"
metadata: 
  node_type: memory
  type: reference
  originSessionId: 4b202cb5-652a-4e3d-ac7a-b78e455f4491
---

Discord bot (client_id 1490376611558588547) is set up in `~/Unix/bin/discord.sh`.

**Usage:** `discord.sh --text "message"` (defaults to #ze-test)

**Channels:**
- `--channel ze-news` (1488605348548837486) -- announcements
- `--channel ze-test` (1490380738837741738) -- testing (default)

**Token:** hardcoded in the script again as the default of
`DISCORD_BOT_TOKEN="${DISCORD_BOT_TOKEN:-<token>}"` (env still overrides). This is
the owner's explicit decision (2026-07-01): it is a limited-rights bot, so baking
it in is acceptable and posting needs no setup. A 2026-06-28 session had removed it
(breaking posting); recovered from the `-Users-thomas-Unix` transcripts and restored.
Do NOT strip the token out again without being asked.

**Weekly updates:** written as "Zeledon" (third-person, no em dashes, no internal
process/spec language). Tooling lives in this repo at `scripts/zeledon/`:
`STYLE.md` (read before drafting), `post_weekly.py` (posts + archives), and the
`weekly/<YYYY-MM-DD>-weekly.md` archive. Follow the `ze-weekly-update` skill:
draft, get Thomas's approval, then `post_weekly.py` posts to ze-news and archives
the exact text. `post_weekly.py` still calls `~/Unix/bin/discord.sh` for the token
(override with the `DISCORD_SH` env var).
