Output artifacts/rbac.webm
Source common.tape
Set WaitTimeout 60s

Hide
Type "cd /src/demos/terminal/rbac"
Enter
Type "export ZE_CONFIG_DIR=/src/tmp/terminal-demos/state/rbac/config NOC_PASSWORD=noc-secret ADMIN_PASSWORD=admin-secret"
Enter
Type "ze-demo run rbac start"
Enter
Wait+Screen /terminal demo ready/
Type "ze-demo card rbac intro"
Enter
Show
Wait+Screen /====/
Sleep 500ms
Screenshot artifacts/rbac.png

Hide
Type "clear"
Enter
Show
Type "ze config show rbac.conf system authorization profile read-only"
Enter
Wait+Screen /match clear/
Sleep 6s
Type "ze config show rbac.conf system authentication user noc profile"
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
Type "ze-demo card rbac deny"
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
Sleep 3s

Hide
Type "ze-demo run rbac stop"
Enter
Wait+Screen /terminal demo stopped/
Type "ze-demo card rbac recap"
Enter
Show
Sleep 9s
