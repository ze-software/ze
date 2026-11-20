Output artifacts/config-graph.webm
Source common.tape

Hide
Type "cd /src/demos/terminal/config-graph"
Enter
Type "clear"
Enter
Type "ze-demo card config-graph intro"
Enter
Show
Wait+Screen /====/
Sleep 250ms
Screenshot artifacts/config-graph.png

Hide
Type "clear"
Enter
Show
Type "ze config show router.conf bgp group transit"
Enter
Wait+Screen /upstream-b/
Sleep 3s
Type "ze config validate router.conf"
Enter
Wait+Screen /configuration valid/
Sleep 2s
Type "clear"
Enter
Type "ze config graph router.conf | ze pipe text | ze pipe match peer/upstream"
Enter
Sleep 2500ms
Type "ze config graph router.conf | ze pipe text | ze pipe match group/transit"
Enter
Sleep 2500ms
Type "ze config graph router.conf | ze pipe text | ze pipe match inherits"
Enter
Sleep 3500ms
Sleep 1s
