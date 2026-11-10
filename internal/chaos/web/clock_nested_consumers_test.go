// Design: docs/architecture/chaos-web-dashboard.md -- one run clock at every reached consumer.
// Related: clock_consumers_test.go -- the controlled clock and the top-level routes.
package web

import (
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/chaos/peer"
)

// TestDashboardNestedClockConsumers covers the consumers the top-level route
// table reaches only through a wrapper: the sidebar stats uptime, each
// time-dependent viz rendered inside a panel slot of the grid, and the same viz
// answered by the panel-content poll. The clock is frozen months away from the
// wall clock, so a consumer that reads time.Now renders a different age or
// duration, and a consumer that samples the clock itself breaks the one-sample
// count.
// MUTATION: time.Now() in writeUptime fails the stats rows; time.Now() in
// writePanelSlotAt or renderVizToBuffer fails every panel row; a second
// d.state.clock.Now() in writePanelGrid fails the sample count.
func TestDashboardNestedClockConsumers(t *testing.T) {
	c := newConsumerClock()
	d, mux := clockConsumerDashboard(t, c)
	base := c.at
	d.state.StartTime = base
	d.ProcessEvent(peer.Event{Type: peer.EventEstablished, PeerIndex: 0, Time: base.Add(5 * time.Second)})
	d.ProcessEvent(peer.Event{Type: peer.EventChaosExecuted, PeerIndex: 0, Time: base.Add(5 * time.Second), ChaosAction: "tcp-disconnect"})
	for _, elapsed := range []time.Duration{20 * time.Second, 80 * time.Second} {
		seconds := strconv.Itoa(int(elapsed.Seconds()))
		age := formatElapsed(elapsed-5*time.Second) + " ago"
		duration := `class="stat-value">` + FormatDuration(elapsed) + "</span>"
		// The closing quote stops a longer wall-clock duration from matching a prefix.
		timeline := `--timeline-duration:` + seconds + `"`
		for _, entry := range []struct{ path, want string }{
			{"/sidebar/stats", `id="uptime" data-start="` + seconds + `"` + streamPanel + `>` + FormatDuration(elapsed) + `</span>`},
			{"/viz/panels?p0=events", age},
			{"/viz/panels?p0=chaos-timeline", duration},
			{"/viz/panels?p0=peer-timeline", timeline},
			{"/viz/panel-content?panel=1&viz=chaos-timeline", duration},
			{"/viz/panel-content?panel=2&viz=peer-timeline", timeline},
		} {
			t.Run(entry.path+"/"+elapsed.String(), func(t *testing.T) {
				c.reset(base.Add(elapsed))
				body := serveClockConsumer(t, mux, http.MethodGet, entry.path)
				if !strings.Contains(body, entry.want) {
					t.Errorf("render does not contain controlled value %q", entry.want)
				}
				if got := c.samples(); got != 1 {
					t.Errorf("render sampled clock %d times, want once", got)
				}
			})
		}
	}
}

// wallClockReads are the time package names that read or schedule on the wall
// clock. A chaos producer reaches time only through its injected clock.Clock.
var wallClockReads = []string{"After", "AfterFunc", "NewTicker", "NewTimer", "Now", "Since", "Tick", "Until"}

// TestChaosProducersReadNoWallClock parses every non-test Go file of the
// dashboard and report producers and refuses any reference to a wall-clock
// function of the time package, under whatever name the file imports it. The
// behavior tests above prove the routes they request; this proves a consumer no
// route table names yet cannot read the wall clock either.
// MUTATION: time.Now() added to any producer file, writeUptime included, names
// that file and line; a dot import of time is refused because it cannot be judged.
func TestChaosProducersReadNoWallClock(t *testing.T) {
	scanned := 0
	for _, dir := range []string{".", filepath.Join("..", "report")} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			name := entry.Name()
			if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				continue
			}
			scanned++
			for _, finding := range wallClockReferences(t, filepath.Join(dir, name)) {
				t.Errorf("%s reads the wall clock; use the injected clock.Clock", finding)
			}
		}
	}
	if scanned == 0 {
		t.Fatal("no producer file was scanned, so no wall-clock read could be found")
	}
}

// wallClockReferences answers each position in path that names a wall-clock
// function of the time package.
func wallClockReferences(t *testing.T, path string) []string {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	timeName := ""
	for _, spec := range file.Imports {
		if spec.Path.Value != `"time"` {
			continue
		}
		timeName = "time"
		if spec.Name != nil {
			timeName = spec.Name.Name
		}
	}
	if timeName == "." {
		return []string{path + ": a dot import of time"}
	}
	if timeName == "" || timeName == "_" {
		return nil
	}
	var found []string
	ast.Inspect(file, func(node ast.Node) bool {
		selector, ok := node.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		pkg, ok := selector.X.(*ast.Ident)
		if !ok {
			return true
		}
		if pkg.Name != timeName {
			return true
		}
		if slices.Contains(wallClockReads, selector.Sel.Name) {
			found = append(found, fset.Position(selector.Pos()).String()+": time."+selector.Sel.Name)
		}
		return true
	})
	return found
}
