// Design: docs/architecture/api/birdwatcher-compat.md -- the fixture follows the engine
// Related: server_test.go -- mockDispatch answers every `show bgp rib` command through this file

package lg

import (
	"strconv"
	"strings"
)

// The bgp-rib answers this package's tests are fed.
//
// Each answer is the shape the plugin produces, and each one names the producer
// it copies. A fixture written for what the looking glass wished the engine said
// hid two defects at once: the fake answered `show bgp rib best <family>` with
// `{"routes":[...]}`, while the plugin refused that spelling and, given the right
// one, answers under `best-path`. The best-routes table was empty on every router
// and green in every test. `test/plugin/lg-best-table.ci` drives the real plugin
// and is the check that these copies still match it.

// fakeBestPathAnswer is what `show bgp rib best family ipv4/unicast` answers:
// rows under bestPathEnvelopeKey, each one bestResult
// (internal/component/bgp/plugins/rib/rib_pipeline_best.go), its attributes
// rendered by enrichRouteMapFromEntry.
const fakeBestPathAnswer = `{"best-path":[{"family":"ipv4/unicast","prefix":"10.0.0.0/24",` +
	`"best-peer":"10.0.0.1","attributes":{"next-hop":"10.0.0.1","origin":"igp",` +
	`"as-path":[65001],"local-preference":100}}]}`

// fakeRouteRowsAnswer is what `show bgp rib` answers with no terminal: rows under
// showRowsEnvelopeKey, each one serializeRouteItem
// (internal/component/bgp/plugins/rib/rib_pipeline.go), naming its own peer and
// direction as fields.
const fakeRouteRowsAnswer = `{"routes":[{"peer":"10.0.0.1","direction":"received",` +
	`"family":"ipv4/unicast","prefix":"10.0.0.0/24","next-hop":"10.0.0.1","origin":"igp",` +
	`"as-path":[65001,65002],"local-preference":100,"med":0,` +
	`"community":["65000:100","65001:200"],"large-community":["65000:0:100"]}]}`

// fakeCountAnswer is the `count` terminal: showPipeline answers jsonKeyCount.
const fakeCountAnswer = `{"count":100}`

// fakeHistogramAnswer is the `histogram` terminal (histogramTerminal.drain).
const fakeHistogramAnswer = `{"histogram":{"ipv4/unicast":{"24":100}},"count":100}`

// The pipeline vocabulary parsePipelineArgs and parseBestPipelineArgs accept
// (rib_pipeline.go: scopeKeywords, filterKeywords, terminalKeywords, and
// bestTerminalReason in rib_pipeline_best.go). `peer` is the selector, which
// takes a value as a filter does.
var (
	fakeRIBScopes    = map[string]bool{"advertised": true, "sent": true, "received": true, "sent-received": true}
	fakeRIBFilters   = map[string]bool{"peer": true, "path": true, "aspath": true, "prefix": true, "community": true, "family": true, "match": true, "first": true, "last": true}
	fakeRIBTerminals = map[string]bool{"count": true, "json": true, "histogram": true, "graph": true}
)

// fakeRIBAnswer answers one `show bgp rib ...` command as bgp-rib would.
//
// It refuses what the plugin's parser refuses, with the plugin's error envelope:
// a keyword it does not know, and anything after a terminal. A fake that answers
// every spelling cannot catch a caller that sends the wrong one.
func fakeRIBAnswer(cmd string) string {
	args := strings.Fields(strings.TrimPrefix(cmd, "show bgp rib"))
	best := len(args) > 0 && args[0] == "best"
	if best {
		args = args[1:]
	}

	terminal, refusal := fakeRIBPipeline(args, best)
	if refusal != "" {
		return `{"error":` + strconv.Quote(refusal) + `}`
	}

	switch terminal {
	case "count":
		return fakeCountAnswer
	case "histogram":
		return fakeHistogramAnswer
	}
	if best {
		return fakeBestPathAnswer
	}
	return fakeRouteRowsAnswer
}

// fakeRIBPipeline walks args the way the plugin's parser does and answers the
// terminal it ends in, or the reason it refuses them. The best-path walk takes
// no scope keyword and accepts the `reason` terminal.
func fakeRIBPipeline(args []string, best bool) (terminal, refusal string) {
	for i := 0; i < len(args); i++ {
		keyword := args[i]
		if terminal != "" {
			return "", "filter after terminal: " + keyword
		}
		switch {
		case !best && fakeRIBScopes[keyword]:
		case fakeRIBFilters[keyword]:
			i++
			if i >= len(args) {
				return "", keyword + " requires a value"
			}
		case fakeRIBTerminals[keyword], best && keyword == "reason":
			terminal = keyword
		default:
			return "", "unknown keyword: " + keyword
		}
	}
	return terminal, ""
}
