Output artifacts/vrrp-failover.webm
Source common.tape
Set WaitTimeout 60s

Hide
Type "/src/demos/terminal/vrrp-failover/run.sh prepare"
Enter
Wait+Screen /VRRP lab prepared/
Type "/src/demos/terminal/vrrp-failover/run.sh start"
Enter
Wait+Screen /Ze is master/
Type "clear"
Enter
Type "bash /src/demos/terminal/cards.sh vrrp-failover intro"
Enter
Show
Sleep 11s

Hide
Type "export ZE_PID=/src/tmp/terminal-demos/state/vrrp-failover/ze.pid ZE_CONFIG_DIR=/src/tmp/terminal-demos/state/vrrp-failover/config ZE_SSH_PASSWORD=secret123"
Enter
Type "cd /src"
Enter
Type "clear"
Enter
Show
Type "ze config show demos/terminal/vrrp-failover/ze.conf interface ethernet eth0 unit 0 ipv4 vrrp group gateway"
Enter
Wait+Screen /priority 200/
Sleep 6s
Type "grep -E 'interface|virtual_router_id|priority|192.0.2.1' /src/demos/terminal/vrrp-failover/keepalived.conf"
Enter
Wait+Screen /priority 100/
Sleep 6s

Hide
Type "ip netns exec vrrp-ze bash"
Enter
Type "export ZE_CONFIG_DIR=/src/tmp/terminal-demos/state/vrrp-failover/config ZE_SSH_PASSWORD=secret123"
Enter
Type "clear"
Enter
Show
Type "ze cli -c 'show vrrp' | ze pipe yaml"
Enter
Sleep 6s
Hide
Type "exit"
Enter
Type "clear"
Enter
Show
Type "ip -n vrrp-ze -o addr show | grep 192.0.2.1 | tr -s ' ' | cut -d' ' -f2,4"
Enter
Wait+Screen /zv4-2-10/
Type "ip -n vrrp-ze -o link show zv4-2-10 | grep -o '00:00:5e:00:01:0a'"
Enter
Wait+Screen /00:00:5e:00:01:0a/
Sleep 6s

Type "clear"
Enter
Type "cat /src/demos/terminal/vrrp-failover/failover-proof.sh"
Enter
Wait+Screen /set -euo pipefail/
Sleep 7s
Type "clear"
Enter
Type "bash -x /src/demos/terminal/vrrp-failover/failover-proof.sh"
Enter
Wait+Screen /2 packets transmitted, 2 received/
Sleep 8s
Screenshot artifacts/vrrp-failover.png
Sleep 2s
