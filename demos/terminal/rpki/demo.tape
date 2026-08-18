Output artifacts/rpki.webm
Source common.tape

Hide
Type "export ZE_CONFIG_DIR=/src/tmp/terminal-demos/state/rpki/config ZE_INIT_INPUT=/src/tmp/terminal-demos/state/rpki/init.input ZE_SSH_PASSWORD=secret123 SSHPASS=secret123"
Enter
Type "/src/demos/terminal/rpki/run.sh prepare"
Enter
Wait+Screen /RPKI demo prepared/
Type "ze init < $ZE_INIT_INPUT >/dev/null"
Enter
Type "/src/demos/terminal/rpki/run.sh start"
Enter
Wait+Screen /RPKI demo ready/
Type "clear"
Enter
Type "bash /src/demos/terminal/cards.sh rpki intro"
Enter
Show
Wait+Screen /====/
Sleep 500ms
Screenshot artifacts/rpki.png

Hide
Type "clear"
Enter
Show
Type "ze cli -c 'show bgp rpki status | no-more'"
Enter
Wait+Screen /vrp-count-ipv4/
Sleep 5s
Type "clear"
Enter
Type "ze cli -c 'show bgp adj-rib-in | no-more'"
Enter
Wait+Screen /11.43.0.0\/24/
Sleep 7s

Hide
Type "/src/demos/terminal/rpki/run.sh stop"
Enter
Wait+Screen /RPKI demo stopped/
Type "clear"
Enter
Type "bash /src/demos/terminal/cards.sh rpki recap"
Enter
Show
Sleep 9s
