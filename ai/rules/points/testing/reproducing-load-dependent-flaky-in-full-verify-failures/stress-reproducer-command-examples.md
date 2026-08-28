---
kind: fence
level:
stage:
---
```
./le stress-repro run suite rsvpte iterations 80
./le stress-repro run suite rsvpte race
./le stress-repro run suite bgp burners 32 parallel 8
./le stress-repro run suite "bgp plugin" test 97 any-failure
```
