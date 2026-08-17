Output artifacts/zefs-config.webm
Source common.tape

Hide
Type "export ZE_CONFIG_DIR=/src/tmp/terminal-demos/state/zefs-config/config ZE_INIT_INPUT=/src/tmp/terminal-demos/state/zefs-config/init.input ZE_SSH_PASSWORD=secret123 SSHPASS=secret123"
Enter
Type "/src/demos/terminal/zefs-config/run.sh prepare"
Enter
Wait+Screen /terminal demo prepared/
Type "bash /src/demos/terminal/cards.sh zefs-config intro"
Enter
Show
Sleep 11s

Hide
Type "clear"
Enter
Show
Type "ze init < $ZE_INIT_INPUT"
Enter
Sleep 4s
Type "ze config ls"
Enter
Wait+Screen /ze.conf/
Sleep 4s
Type "ze data check"
Enter
Sleep 4s

Hide
Type "/src/demos/terminal/zefs-config/run.sh start"
Enter
Wait+Screen /terminal demo ready/
Type "bash /src/demos/terminal/cards.sh zefs-config ssh"
Enter
Show
Sleep 9s

Hide
Type "clear"
Enter
Show
Type "sshpass -e ssh ze-demo show bgp summary"
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
Type "commit"
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
Type "ze cli -c 'show bgp summary'"
Enter
Wait+Screen /router-id/
Sleep 7s
Screenshot artifacts/zefs-config.png

Hide
Type "/src/demos/terminal/zefs-config/run.sh stop"
Enter
Wait+Screen /terminal demo stopped/
Type "bash /src/demos/terminal/cards.sh zefs-config recap"
Enter
Show
Sleep 9s
