Type "ze-demo card topic-commit-confirmed lab"
Enter
Wait+Screen /====/
Sleep 6s
Type "ze-demo card topic-commit-confirmed overview"
Enter
Wait+Screen /====/
Sleep 6s
Type "sshpass -e ssh ze-demo"
Enter
Wait+Screen /ze#/
Type "show system host"
Enter
Wait+Screen /edge-original/
Sleep 3s
Card topic-commit-confirmed trial
Type "load file absolute merge /src/demos/terminal/topics/commit-confirmed/1-trial-host.conf"
Enter
Wait+Screen /merged from \S*1-trial-host\.conf/
Type "show | compare"
Enter
Wait+Screen /\*\s+system host edge-trial/
Sleep 3s
# Fifteen seconds, not fewer: the reading floor after Confirm within, the
# typing of show system host and its settle must all land before the rollback,
# or the rollback resets the input line and edge-trial is never shown.
Type "commit confirmed 15"
Enter
Wait+Screen /Confirm within/
Type "show system host"
Enter
Wait+Screen /edge-trial/
Sleep 2s
Wait+Screen /automatically rolled back/
Sleep 3s
