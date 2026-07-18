Output artifacts/rbac.webm
Source common.tape
Set WaitTimeout 60s

Hide
Type "export ZE_CONFIG_DIR=/src/tmp/terminal-demos/state/rbac/config NOC_PASSWORD=noc-secret ADMIN_PASSWORD=admin-secret"
Enter
Type "/src/demos/terminal/rbac/run.sh start"
Enter
Wait+Screen /terminal demo ready/
Type "bash /src/demos/terminal/cards.sh rbac intro"
Enter
Show
Sleep 11s

Hide
Type "clear"
Enter
Show
Type "ze config show /src/demos/terminal/rbac/rbac.conf system authorization profile read-only"
Enter
Wait+Screen /match clear/
Sleep 6s
Type "ze config show /src/demos/terminal/rbac/rbac.conf system authentication user noc profile"
Enter
Wait+Screen /profile read-only/
Sleep 4s

Hide
Type "export ZE_SSH_PASSWORD=noc-secret"
Enter
Type "clear"
Enter
Show
Type "ze cli --user noc -c 'show version'"
Enter
Sleep 5s

Hide
Type "bash /src/demos/terminal/cards.sh rbac deny"
Enter
Show
Sleep 9s

Hide
Type "clear"
Enter
Show
Type "ze cli --user noc -c 'clear interface counters'"
Enter
Wait+Screen /restricted by access control/
Sleep 6s
Screenshot artifacts/rbac.png
Sleep 3s

Hide
Type "/src/demos/terminal/rbac/run.sh stop"
Enter
Wait+Screen /terminal demo stopped/
Type "bash /src/demos/terminal/cards.sh rbac recap"
Enter
Show
Sleep 9s
