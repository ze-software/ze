---
kind: directive
level: MUST
stage:
---
**A change to `internal/component/bgp/reactor/session*.go`,
`forward_pool*.go`, `peer.go`, or any other reactor file that holds locks or
shares state across goroutines MUST run
`go test -race -count=20 ./internal/component/bgp/reactor/...` before it is
claimed done.** The standard `-race -count=1` unit run is not enough: a schedule
rare enough to need twenty runs has hidden a reactor race for weeks.
