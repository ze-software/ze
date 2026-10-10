# Commit-confirmed topic, beat 5: the rollback restored the original, and a
# change that is confirmed in time stays. The caller is still in the editor.
Type "show system host"
Enter
Wait+Screen /edge-original/
Sleep 5s
Card topic-commit-confirmed confirmed
Type "load file absolute merge /src/demos/terminal/topics/commit-confirmed/2-confirmed-host.conf"
Enter
Wait+Screen /merged from \S*2-confirmed-host\.conf/
Type "commit confirmed 8"
Enter
Wait+Screen /Confirm within/
Type "commit accept"
Enter
Wait+Screen /Commit accepted/
Sleep 5s
Type "show system host"
Enter
Wait+Screen /edge-confirmed/
Sleep 5s
