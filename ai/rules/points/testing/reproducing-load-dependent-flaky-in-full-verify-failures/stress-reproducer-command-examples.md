---
kind: fence
level:
stage:
---
```
python3 scripts/dev/stress-repro.py rsvpte --iterations 80         # hunt + capture the stack
python3 scripts/dev/stress-repro.py rsvpte --race                  # data race self-reports its two accesses
python3 scripts/dev/stress-repro.py bgp --burners 32 --parallel 8  # more pressure
python3 scripts/dev/stress-repro.py "bgp plugin" --test 97 --any-failure  # sub-suite, one test, assertion flake
```
