Output artifacts/launcher.webm
Source common.tape

Hide
Type "bash /src/demos/terminal/cards.sh launcher intro"
Enter
Show
Sleep 10s

Hide
Type "clear"
Enter
Show
Type "ze"
Enter
Wait+Screen /Operations/
Sleep 4s

Type "show"
Wait+Screen /filter: show/
Sleep 4s
Enter
Wait+Screen /ze > show/
Sleep 3s

Type "traceroute"
Wait+Screen /filter: traceroute/
Sleep 6s
Screenshot artifacts/launcher.png

Escape
Sleep 3s
Left
Sleep 3s
Type "doctor"
Wait+Screen /filter: doctor/
Sleep 6s
Escape
Sleep 2s
Escape
Sleep 2s
Escape
Sleep 2s
Wait+Screen /\$ /
Hide
Sleep 2s
Type " true"
Enter
Sleep 1s
Type "clear"
Enter

Type "bash /src/demos/terminal/cards.sh launcher recap"
Enter
Show
Sleep 8s
