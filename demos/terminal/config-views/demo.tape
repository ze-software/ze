Output artifacts/config-views.webm
Source common.tape

Hide
Type "export STATE=/src/tmp/terminal-demos/state/config-views"
Enter
Type "cd $STATE"
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
Sleep 500ms
Screenshot artifacts/config-views.png

Hide
Type "clear"
Enter
Show
Type "ze config show router.conf bgp peer transit-a"
Enter
Wait+Screen /remote ip 192.0.2.2/
Sleep 7s
Type "clear"
Enter
Type "ze config migrate --format set router.conf 2>/dev/null | ze pipe match 'bgp peer transit-a'"
Enter
Wait+Screen /set bgp peer transit-a/
Sleep 7s
Type "cmp -s router.set roundtrip.set && echo 'canonical output: identical'"
Enter
Wait+Screen /canonical output: identical/
Sleep 5s
Type "ze --plugins | ze pipe match flowspec"
Enter
Wait+Screen /bgp-nlri-flowspec/
Sleep 5s
Type "ze --plugins | ze pipe match flowspec | ze pipe count"
Enter
Sleep 5s

Hide
Type "clear"
Enter
Type "ze-demo card config-views recap"
Enter
Show
Sleep 9s
