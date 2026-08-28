Output artifacts/traffic-anomaly.webm
Source common.tape
Set WaitTimeout 60s

Hide
Type "export ZE_CONFIG_DIR=/src/tmp/terminal-demos/state/traffic-anomaly/config ZE_SSH_PASSWORD=secret123"
Enter
Type "ze-demo run traffic-anomaly prepare"
Enter
Wait+Screen /Traffic lab prepared/
Type "ze-demo run traffic-anomaly start"
Enter
Wait+Screen /Traffic monitor attached/
Type "clear"
Enter
Type "ze-demo card traffic-anomaly intro"
Enter
Show
Wait+Screen /====/
Sleep 500ms
Screenshot artifacts/traffic-anomaly.png

Hide
Type "cd /src"
Enter
Type "clear"
Enter
Show
Type "ze config show demos/terminal/traffic-anomaly/ze.conf traffic usage"
Enter
Wait+Screen /interface traffic0/
Sleep 6s
Type "clear"
Enter
Type "ze cli -c 'show traffic usage name traffic0' | ze pipe yaml"
Enter
Sleep 6s

Type "ip netns exec traffic-peer ping -c 4 10.77.0.1"
Enter
Wait+Screen /4 packets transmitted, 4 received/
Sleep 4s
Type "ip netns exec traffic-peer curl -s -o /dev/null http://10.77.0.1:8080/payload.txt"
Enter
Sleep 3s

Type "clear"
Enter
Type "ze cli -c 'show traffic usage name traffic0' | ze pipe yaml"
Enter
Sleep 8s
Sleep 2s
