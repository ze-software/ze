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
Type "commit confirmed 8"
Enter
Wait+Screen /Confirm within/
Type "show system host"
Enter
Wait+Screen /edge-trial/
Sleep 2s
Wait+Screen /automatically rolled back/
Sleep 3s
