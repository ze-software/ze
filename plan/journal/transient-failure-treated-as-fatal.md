# Transient failure treated as fatal

A step fails for a reason that waiting fixes, and the caller exits instead of
waiting. When the operation had already changed state outside the process, the
abort leaves that state half-changed, and the record that would let anyone
finish it is written only on the success path.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-08-12 | - | scripts/zeledon | Discord rate limited the last of the weekly update's 8 messages. post_weekly.py exited on the first failure, so ze-news held a post with no Coming up section, and the archive that marks a week as posted is written only after every chunk lands, so a re-run would have sent the other 7 again | send() now retries a rate-limited chunk on a fixed backoff, any other failure prints the --resume-from number that finishes the post, and post_weekly_test.py covers both paths plus the discord.sh lookup that had exited not-found on this machine |
