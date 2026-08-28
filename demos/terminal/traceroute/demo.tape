Output artifacts/traceroute.webm
Source common.tape

Hide
Type "export ZE_CONFIG_DIR=/src/tmp/terminal-demos/state/traceroute/config ZE_SSH_PASSWORD=secret123 SSHPASS=secret123"
Enter
Type "ze-demo run traceroute start"
Enter
Wait+Screen /terminal demo ready/
Type "ze-demo card traceroute intro"
Enter
Show
Wait+Screen /====/
Sleep 500ms
Screenshot artifacts/traceroute.png

Hide
Type "clear"
Enter
Show
Type "sshpass -e ssh ze-demo"
Enter
Wait+Screen /ze#/
Sleep 3s
Type "run show traceroute 192.0.2.53"
Enter
Wait+Screen /192.0.2.53/
Sleep 7s
Type "run monitor traceroute 192.0.2.53"
Enter
Wait+Screen /Loss/
Sleep 10s
Sleep 3s
Type "q"
Sleep 3s
Escape
Sleep 3s
Escape
Sleep 3s

Hide
Type "ze-demo run traceroute stop"
Enter
Wait+Screen /terminal demo stopped/
Type "ze-demo card traceroute recap"
Enter
Show
Sleep 9s
