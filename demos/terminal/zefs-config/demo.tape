Output artifacts/zefs-config.webm
Source common.tape

Hide
Type "export ZE_CONFIG_DIR=/src/tmp/terminal-demos/state/zefs-config/config ZE_SSH_PASSWORD=secret123 SSHPASS=secret123"
Enter
Type "ze-demo run zefs-config prepare"
Enter
Wait+Screen /terminal demo prepared/
Type "ze-demo card zefs-config intro"
Enter
Show
Wait+Screen /====/
Sleep 500ms
Screenshot artifacts/zefs-config.png

Hide
Type "clear"
Enter
Show
Type "ze init"
Enter
Wait+Screen /username:/
Sleep 1s
Type "admin"
Enter
Wait+Screen /password:/
Sleep 1s
# The password is read without echo, so the recording shows only the prompt.
Type "secret123"
Enter
Wait+Screen /host \[/
Sleep 1s
# Enter accepts the default shown in brackets. The name defaults to the
# hostname, which is ze in the demo container.
Enter
Wait+Screen /port \[/
Sleep 1s
Enter
Wait+Screen /name \[ze\]/
Sleep 1s
Enter
Sleep 4s
Type "ze config list"
Enter
Wait+Screen /\[data\] .*ze\.conf/
Sleep 4s
Type "ze data check"
Enter
Sleep 4s

Hide
Type "ze-demo run zefs-config start"
Enter
Wait+Screen /terminal demo ready/
Type "ze-demo card zefs-config ssh"
Enter
Show
Sleep 9s

Hide
Type "clear"
Enter
Show
Type "sshpass -e ssh ze-demo show bgp"
Enter
Wait+Screen /router-id/
Sleep 7s
Type "sshpass -e ssh ze-demo"
Enter
Wait+Screen /ze#/
Sleep 3s
Type "set environment cli format default table"
Enter
Sleep 4s
Type "show | compare"
Enter
Sleep 6s
Type "commit now"
Enter
Wait+Screen /Session committed/
Sleep 3s
Type "exit"
Enter
Wait+Screen /operational\]/
Sleep 3s
Type "exit"
Enter
Sleep 3s
Type "clear"
Enter
Type "ze cli -c 'show bgp'"
Enter
Wait+Screen /router-id/
Sleep 7s
Type "ze cli -c 'show bgp | text'"
Enter
# The table above prints router-id inside a cell; only the text rendering
# starts a line with it, so the table still on screen cannot satisfy this.
Wait+Screen /(?m)^router-id\s+\S/
Sleep 7s
Type "clear"
Enter
# The operator names the fields they want, in the order they want them.
Type "ze cli -c 'show bgp | display router-id local-as peers-established'"
Enter
Wait+Screen /(?m)^[^$\n]*peers-established/
Sleep 7s
# fill brings the fields display did not name back, in field-name order.
Type "ze cli -c 'show bgp | display router-id | fill alpha'"
Enter
Wait+Screen /peers-configured/
Sleep 7s
Type "clear"
Enter
# peers is an alias: a name for a pipe expression, so the rows come back alone.
Type "ze cli -c 'show bgp | peers'"
Enter
Wait+Screen /address/
Sleep 7s
Type "clear"
Enter
Type "ze cli -c 'show bgp | raw' | head -14"
Enter
# head truncates above router-id, and the typed line carries only single
# quotes, so a double quote appears exactly when JSON output does.
Wait+Screen /"/
Sleep 8s
Type "clear"
Enter
Type "ze cli -c 'show bgp | raw' | ze pipe text"
Enter
Wait+Screen /router-id/
Sleep 7s

Hide
Type "ze-demo run zefs-config stop"
Enter
Wait+Screen /terminal demo stopped/
Type "ze-demo card zefs-config recap"
Enter
Show
Sleep 9s
