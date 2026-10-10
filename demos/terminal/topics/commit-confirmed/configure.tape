# Commit-confirmed topic, beats 2 and 4: a change loaded into the running
# daemon's candidate and committed on probation. Nobody confirms it, so Ze
# rolls it back. The caller has the SSH editor open at the ze# prompt.
Type "show system host"
Enter
Wait+Screen /edge-original/
Sleep 4s
Type "load file absolute merge /src/demos/terminal/topics/commit-confirmed/1-trial-host.conf"
Enter
Wait+Screen /merged from \S*1-trial-host\.conf/
Sleep 3s
Type "show | compare"
Enter
Wait+Screen /\*\s+system host edge-trial/
Sleep 5s
Type "commit confirmed 8"
Enter
Wait+Screen /Confirm within/
Sleep 4s
Wait+Screen /automatically rolled back/
Sleep 4s
