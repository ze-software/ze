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
Sleep 250ms
Screenshot artifacts/cli-dashboard.png

Hide
Type "clear"
Enter
Show
Type "sshpass -e ssh ze-demo"
Enter
Wait+Screen /ze#/
Sleep 1500ms
Type "exit"
Enter
Wait+Screen /ze>/
Sleep 1500ms
Type "monitor bgp"
Enter
Wait+Screen /established/
Sleep 4s

Type "s"
Sleep 2500ms
Down
Sleep 1500ms
Enter
Wait+Screen /Peer Detail/
Sleep 3500ms

Escape
Sleep 1500ms
Type "q"
Wait+Screen /ze>/
Sleep 1500ms
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
Sleep 4500ms
