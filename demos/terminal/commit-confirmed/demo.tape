Output artifacts/commit-confirmed.webm
Source common.tape

Hide
Type "mkdir -p /src/tmp/terminal-demos/state/commit-confirmed"
Enter
Type "cp /src/demos/terminal/commit-confirmed/identity.conf /src/tmp/terminal-demos/state/commit-confirmed/ze.conf"
Enter
Type "clear"
Enter
Type "bash /src/demos/terminal/cards.sh commit-confirmed intro"
Enter
Show
Sleep 10s

Hide
Type "clear"
Enter
Show
Type "ze config edit -f /src/tmp/terminal-demos/state/commit-confirmed/ze.conf"
Enter
Wait+Screen /ze#/
Type "show system host"
Enter
Wait+Screen /edge-original/
Sleep 4s
Type "set system host edge-trial"
Enter
Sleep 3s
Type "show | compare"
Enter
Wait+Screen /edge-trial/
Sleep 5s
Type "commit confirmed 8"
Enter
Wait+Screen /Confirm within/
Sleep 4s
Type "show system host"
Enter
Wait+Screen /edge-trial/
Sleep 5s
Wait+Screen /automatically rolled back/
Sleep 4s
Type "show system host"
Enter
Wait+Screen /edge-original/
Sleep 5s
Type "set system host edge-confirmed"
Enter
Type "commit confirmed 8"
Enter
Wait+Screen /Confirm within/
Type "confirm"
Enter
Wait+Screen /confirmed and saved permanently/
Sleep 5s
Type "show system host"
Enter
Wait+Screen /edge-confirmed/
Sleep 5s
Screenshot artifacts/commit-confirmed.png

Hide
Type "clear"
Enter
Type "bash /src/demos/terminal/cards.sh commit-confirmed recap"
Enter
Show
Sleep 9s
