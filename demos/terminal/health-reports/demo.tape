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
Type "exit"
Enter
Wait+Screen /ze>/
Type "exit"
Enter
Wait+Screen /\$ /
Type "sshpass -e ssh ze-demo 'show health'"
Enter
Wait+Screen /ipsec/
Wait+Screen /status\s+down/
Sleep 6s
Type "sshpass -e ssh ze-demo 'show warnings source bgp'"
Enter
Wait+Screen /prefix-stale/
Wait+Screen /count\s+1/
Sleep 6s
Type "sshpass -e ssh ze-demo 'request peer 127.0.0.2 teardown 4'"
Enter
Wait+Screen /peer\s+127\.0\.0\.2/
Wait+Screen /subcode\s+4/
Sleep 4s
Type "sshpass -e ssh ze-demo 'show errors source bgp'"
Enter
Wait+Screen /notification-sent/
Sleep 6s
Sleep 3s
Sleep 3s
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
