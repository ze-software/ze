Output artifacts/cli-dashboard.webm
Source common.tape

Hide
Type "export ZE_CONFIG_DIR=/src/tmp/terminal-demos/state/cli-dashboard/config ZE_SSH_PASSWORD=secret123 SSHPASS=secret123"
Enter
Type "ze-demo run cli-dashboard start"
Enter
Wait+Screen /terminal demo ready/
Type "ze-demo card cli-dashboard intro"
Enter
Show
Wait+Screen /====/
Sleep 500ms
Screenshot artifacts/cli-dashboard.png

Hide
Type "clear"
Enter
Show
Type "sshpass -e ssh ze-demo"
Enter
Wait+Screen /ze#/
Sleep 3s
Type "exit"
Enter
Wait+Screen /ze>/
Sleep 3s
Type "monitor bgp"
Enter
Wait+Screen /established/
Sleep 8s

Type "s"
Sleep 5s
Down
Sleep 3s
Enter
Wait+Screen /Peer Detail/
Sleep 7s

Escape
Sleep 3s
Type "q"
Wait+Screen /ze>/
Sleep 3s
Type "exit"
Enter
Wait+Screen /\$ /

Hide
Type "ze-demo run cli-dashboard stop"
Enter
Wait+Screen /terminal demo stopped/
Type "ze-demo card cli-dashboard recap"
Enter
Show
Sleep 9s
