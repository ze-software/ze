Output artifacts/config-views.webm
Source common.tape

Hide
Type "export STATE=/src/tmp/terminal-demos/state/config-views"
Enter
Type "ze-demo run config-views prepare"
Enter
Wait+Screen /Configuration views prepared/
Type "cd /src/demos/terminal/config-views"
Enter
Type "clear"
Enter
Type "ze-demo card config-views intro"
Enter
Show
Wait+Screen /====/
Sleep 250ms
Screenshot artifacts/config-views.png

Hide
Type "clear"
Enter
Show
Type "ze config show router.conf bgp peer transit-a"
Enter
Wait+Screen /remote ip 192.0.2.2/
Sleep 3500ms
Type "clear"
Enter
Type "ze config migrate format set router.conf 2>/dev/null | ze pipe match 'bgp peer transit-a'"
Enter
Wait+Screen /set bgp peer transit-a/
Sleep 3500ms
Type "cmp -s $STATE/router.set $STATE/roundtrip.set && echo 'canonical output: identical'"
Enter
Wait+Screen /(?m)^canonical output: identical/
Sleep 2500ms
Type "ze show plugin list | ze pipe match flowspec"
Enter
Wait+Screen /bgp-nlri-flowspec/
Sleep 2500ms
Type "ze show plugin list | ze pipe match flowspec | ze pipe count"
Enter
Sleep 2500ms

Hide
Type "clear"
Enter
Type "ze-demo card config-views recap"
Enter
Show
Sleep 4500ms
