# Transient failure treated as fatal

A step fails for a reason that waiting fixes, and the caller exits instead of
waiting. When the operation had already changed state outside the process, the
abort leaves that state half-changed, and the record that would let anyone
finish it is written only on the success path.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-08-15 | fixit-mirror-clsact-ownership | iface config apply, traffic mirroring | a mirror whose DESTINATION interface is absent aborts the whole commit and unwinds it. `SetupMirror` (`internal/plugins/iface/netlink/mirror_linux.go`) fails at `netlink.LinkByName(dstIface)`, `applyMirror` returns the error, and `applyConfig` (`internal/component/iface/config_apply.go`) answers with `return rollbackPartial()`. The same function refuses to do this one screen away for an absent SOURCE NIC: `absentPhysical` skips it with a warning because "an appliance must not brick because a configured NIC is missing". A capture interface that is an unplugged NIC is that same case on the other side of the mirror, and it is not covered. No test exercises an absent destination | not fixed, and it predates this spec: `applyMirror` returned errors this way before the mirror teardown work. Recorded because the two policies sit in one function and disagree. The fix is to extend the `absentPhysical` treatment to a mirror destination, or to say plainly why a destination is stricter than a source |
| 2026-08-12 | - | scripts/zeledon | Discord rate limited the last of the weekly update's 8 messages. post_weekly.py exited on the first failure, so ze-news held a post with no Coming up section, and the archive that marks a week as posted is written only after every chunk lands, so a re-run would have sent the other 7 again | send() now retries a rate-limited chunk on a fixed backoff, any other failure prints the --resume-from number that finishes the post, and post_weekly_test.py covers both paths plus the discord.sh lookup that had exited not-found on this machine |
