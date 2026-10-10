# IRR topic, beat 2: the IRR configuration, loaded into the running daemon's
# candidate one section at a time, then compared and committed. The caller has
# the SSH editor open at the ze# prompt on the showcase lab. Each wait names
# its section's file or a value only that section adds, so an earlier
# section's output still on screen cannot satisfy it.
Type "load file absolute merge /src/demos/terminal/topics/irr/1-server.conf"
Enter
Wait+Screen /merged from \S*1-server\.conf/
Sleep 3s
Type "load file absolute merge /src/demos/terminal/topics/irr/2-as-set.conf"
Enter
Wait+Screen /merged from \S*2-as-set\.conf/
Sleep 3s
Type "load file absolute merge /src/demos/terminal/topics/irr/3-import.conf"
Enter
Wait+Screen /merged from \S*3-import\.conf/
Sleep 3s
Type "show | compare"
Enter
Wait+Screen /AS-TEST/
Sleep 6s
Type "commit now"
Enter
Wait+Screen /Session committed/
Sleep 3s
