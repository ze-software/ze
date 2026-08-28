Output artifacts/bfd-failover.webm
Source common.tape
Set WaitTimeout 60s

Hide
Type "ze-demo run bfd-failover prepare"
Enter
Wait+Screen /BFD failover demo prepared/
Type "ze-demo run bfd-failover start"
Enter
Wait+Screen /BFD and BGP sessions are up/
Type "clear"
Enter
Type "ze-demo card bfd-failover intro"
Enter
Show
Wait+Screen /====/
Sleep 500ms
Screenshot artifacts/bfd-failover.png

Hide
Type "export ZE_CONFIG_DIR=/src/tmp/terminal-demos/state/bfd-failover/config ZE_SSH_PASSWORD=secret123"
Enter
Type "cd /src"
Enter
Type "clear"
Enter
Show
Type "ze config show demos/terminal/bfd-failover/ze.conf bfd"
Enter
Wait+Screen /detect-multiplier 3/
Sleep 6s
Type "clear"
Enter
Type "ze config show demos/terminal/bfd-failover/ze.conf bgp peer edge-peer connection"
Enter
Wait+Screen /mode single-hop/
Sleep 6s
Type "clear"
Enter
Type "ze config show demos/terminal/bfd-failover/ze.conf bgp peer edge-peer timer"
Enter
Wait+Screen /receive-hold-time 300/
Sleep 6s

Hide
Type "ip netns exec bfd-ze ze-demo shell"
Enter
Type "export ZE_CONFIG_DIR=/src/tmp/terminal-demos/state/bfd-failover/config ZE_SSH_PASSWORD=secret123"
Enter
Type "clear"
Enter
Show
Type "ze cli -c 'show bfd sessions' | ze pipe yaml"
Enter
Sleep 6s

Hide
Type "exit"
Enter
Type "clear"
Enter
Show
Type "date -u +%T; ip link set bfd-p down"
Enter
Sleep 4s

Hide
Type "ip netns exec bfd-ze ze-demo shell"
Enter
Type "export ZE_CONFIG_DIR=/src/tmp/terminal-demos/state/bfd-failover/config ZE_SSH_PASSWORD=secret123"
Enter
Type "clear"
Enter
Show
Type "ze cli -c 'show bfd sessions' | ze pipe yaml"
Enter
Sleep 6s
Type "clear"
Enter
Type "ze cli -c 'show bgp peer list' | ze pipe yaml"
Enter
Sleep 6s
Sleep 3s

Hide
Type "exit"
Enter
Show
Type "ip link set bfd-p up"
Enter
Sleep 3s
Hide
Type "ze-demo run bfd-failover restore"
Enter
Wait+Screen /BGP re-established/
Type "ip netns exec bfd-ze ze-demo shell"
Enter
Type "export ZE_CONFIG_DIR=/src/tmp/terminal-demos/state/bfd-failover/config ZE_SSH_PASSWORD=secret123"
Enter
Type "clear"
Enter
Show
Type "ze cli -c 'show bgp peer list' | ze pipe yaml"
Enter
Sleep 6s
Sleep 2s
