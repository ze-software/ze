# The configure fragment leaves this same SSH candidate open after rollback.
Type "show system host"
Enter
Wait+Screen /edge-original/
Type "show | compare"
Enter
Wait+Screen /ze#/
Sleep 2s
Type "show | changes"
Enter
Wait+Screen /No pending changes/
Sleep 4s
Card topic-commit-confirmed confirmed
Type "load file absolute merge /src/demos/terminal/topics/commit-confirmed/2-confirmed-host.conf"
Enter
Wait+Screen /merged from \S*2-confirmed-host\.conf/
Type "show | compare"
Enter
Wait+Screen /edge-confirmed/
Type "commit confirmed 8"
Enter
Wait+Screen /Confirm within/
Type "commit accept"
Enter
Wait+Screen /Commit accepted/
# Keep the accepted change visible beyond the same eight-second deadline.
Sleep 9s
Type "show system host"
Enter
Wait+Screen /edge-confirmed/
Sleep 3s
Type "exit"
Enter
Wait+Screen /operational\]/
Type "exit"
Enter
Wait+Screen /\$ /
Hide
Type "ze-demo run commit-confirmed await confirmed"
Enter
Wait+Screen /Topic ready/
Type "clear"
Enter
Show
Type "ze cli -c \"show config history $ZE_CONFIG_DIR/ze.conf | yaml | no-more\""
Enter
Wait+Screen /revision:/
Sleep 4s
Type "ze-demo card topic-commit-confirmed recap"
Enter
Wait+Screen /====/
Sleep 5s
