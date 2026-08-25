Output artifacts/health-reports.webm
Source common.tape
Set WaitTimeout 60s

Hide
Type "export ZE_CONFIG_DIR=/src/tmp/terminal-demos/state/health-reports/config ZE_INIT_INPUT=/src/tmp/terminal-demos/state/health-reports/init.input ZE_SSH_PASSWORD=secret123 SSHPASS=secret123"
Enter
Type "/src/demos/terminal/health-reports/run.sh prepare"
Enter
Wait+Screen /Health reports demo prepared/
Type "ze init < $ZE_INIT_INPUT >/dev/null"
Enter
Type "/src/demos/terminal/health-reports/run.sh start"
Enter
Wait+Screen /Health reports demo ready/
Type "clear"
Enter
Type "bash /src/demos/terminal/cards.sh health-reports intro"
Enter
Show
Wait+Screen /====/
Sleep 500ms
Screenshot artifacts/health-reports.png

Hide
Type "clear"
Enter
Show
Type "sshpass -e ssh ze-demo"
Enter
Wait+Screen /stale prefix data/
Sleep 5s
Type "run show health"
Enter
Wait+Screen /ipsec/
Wait+Screen /Ze Editor/
Sleep 6s
Type "run show warnings source bgp"
Enter
Wait+Screen /prefix-stale/
Wait+Screen /Ze Editor/
Sleep 6s
Type "run request peer 127.0.0.2 teardown 4"
Enter
Wait+Screen /Ze Editor/
Sleep 4s
Type "run show errors source bgp"
Enter
Wait+Screen /notification-sent/
Sleep 6s
Escape
Sleep 3s
Escape
Sleep 3s
Escape
Sleep 3s

Hide
Type "/src/demos/terminal/health-reports/run.sh stop"
Enter
Wait+Screen /Health reports demo stopped/
Type "clear"
Enter
Type "bash /src/demos/terminal/cards.sh health-reports recap"
Enter
Show
Sleep 9s
