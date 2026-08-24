Output artifacts/config-graph.webm
Source common.tape

Hide
Type "cd /src/demos/terminal/config-graph"
Enter
Type "clear"
Enter
Type "bash /src/demos/terminal/cards.sh config-graph intro"
Enter
Show
Wait+Screen /====/
Sleep 500ms
Screenshot artifacts/config-graph.png

Hide
Type "clear"
Enter
Show
Type "ze config show router.conf bgp group transit"
Enter
Wait+Screen /upstream-b/
Sleep 6s
Type "ze config validate router.conf"
Enter
Wait+Screen /configuration valid/
Sleep 4s
Type "clear"
Enter
Type "ze config graph router.conf | ze pipe text | ze pipe match peer/upstream"
Enter
Sleep 5s
Type "ze config graph router.conf | ze pipe text | ze pipe match group/transit"
Enter
Sleep 5s
Type "ze config graph router.conf | ze pipe text | ze pipe match inherits"
Enter
Sleep 7s
Sleep 2s
