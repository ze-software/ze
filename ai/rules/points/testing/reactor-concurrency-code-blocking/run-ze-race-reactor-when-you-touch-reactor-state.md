---
kind: note
level:
stage:
---
When touching `internal/component/bgp/reactor/session*.go`, `forward_pool*.go`,
`peer.go`, or any other reactor file that holds locks or shares state across
goroutines, the standard `-race -count=1` unit run is **not enough**. The
bufReader/bufWriter races (`d5843235`, `8dffd422`) lived 47 days because the
schedule that triggered them was rare. Run `make ze-unit-reactor-test-race` (`-race
-count=20`) before claiming the change done.
