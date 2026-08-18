Output artifacts/ospf-adjacency.webm
Source common.tape
Set WaitTimeout 60s

Hide
Type "/src/demos/terminal/ospf-adjacency/run.sh prepare"
Enter
Wait+Screen /OSPF lab prepared/
Type "/src/demos/terminal/ospf-adjacency/run.sh start"
Enter
Wait+Screen /OSPF adjacency converged/
Type "clear"
Enter
Type "bash /src/demos/terminal/cards.sh ospf-adjacency intro"
Enter
Show
Wait+Screen /====/
Sleep 500ms
Screenshot artifacts/ospf-adjacency.png

Hide
Type "export ZE_CONFIG_DIR=/src/tmp/terminal-demos/state/ospf-adjacency/config ZE_SSH_PASSWORD=secret123"
Enter
Type "cd /src"
Enter
Type "clear"
Enter
Show
Type "ze config show demos/terminal/ospf-adjacency/ze.conf ospf"
Enter
Wait+Screen /router-id 172.31.0.2/
Sleep 6s

Hide
Type "ip netns exec ospf-ze bash"
Enter
Type "export ZE_CONFIG_DIR=/src/tmp/terminal-demos/state/ospf-adjacency/config ZE_SSH_PASSWORD=secret123"
Enter
Type "clear"
Enter
Show
Type "ze cli -c 'show ospf neighbor detail'"
Enter
Sleep 7s
Type "clear"
Enter
Type "ze cli -c 'show ospf database router'"
Enter
Sleep 7s
Type "clear"
Enter
Type "ze cli -c 'show ospf route'"
Enter
Sleep 7s
Sleep 2s
