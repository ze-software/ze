// Design: docs/architecture/isis/isis-9-spf-rib.md -- configuration-safe SPF publication.

package spf

import (
	"net/netip"
	"testing"
	"time"
)

func TestComputerConfigurationOvertakesRun(t *testing.T) {
	for _, change := range []string{"root", "levels"} {
		t.Run(change, func(t *testing.T) {
			src := newStubSource()
			a, b := srcID(1), srcID(2)
			src.bidir(a, b, 10)
			for i := range src.byLevel[Level1] {
				if src.byLevel[Level1][i].Source == b {
					src.byLevel[Level1][i].LSP.TLVs = append(src.byLevel[Level1][i].LSP.TLVs,
						tlv135(netip.MustParsePrefix("192.0.2.0/24"), 5, false))
				}
			}
			resolver := &gatedResolver{entered: make(chan struct{}), block: make(chan struct{})}
			computer := NewComputer(Config{
				Source: src, Resolver: resolver, Root: sysID(1), Levels: []Level{Level1}, Debounce: time.Hour,
			})
			defer computer.Stop()
			completed := 0
			computer.SetOnComplete(func() { completed++ })
			done := make(chan RouteDelta, 1)
			go func() { done <- computer.Run() }()
			<-resolver.entered
			wantRoot, wantLevel := sysID(1), Level1
			if change == "root" {
				wantRoot = sysID(3)
				computer.SetRoot(wantRoot)
			} else {
				wantLevel = Level2
				computer.SetLevels([]Level{wantLevel})
			}
			close(resolver.block)
			if delta := <-done; !delta.Empty() {
				t.Fatalf("obsolete configuration installed routes: %+v", delta)
			}
			if routes := computer.Routes(); len(routes) != 0 {
				t.Fatalf("obsolete configuration became visible: %+v", routes)
			}
			if state := computer.Reachability(); len(state) != 0 || completed != 0 {
				t.Fatalf("obsolete graph was published: reachability=%+v completions=%d", state, completed)
			}
			computer.Run()
			state := computer.Reachability()
			if completed != 1 || len(state) != 1 || state[0].Root != wantRoot || state[0].Level != wantLevel {
				t.Fatalf("current configuration was not published: reachability=%+v completions=%d", state, completed)
			}
			if routes := computer.Routes(); len(routes) != 0 {
				t.Fatalf("new configuration retained old routes: %+v", routes)
			}
		})
	}
}
